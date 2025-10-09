package ssmock

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/artefactual-labs/migrate/internal/storage_service"
)

const defaultMoveDelay = 150 * time.Millisecond

// Server provides an in-memory simulation of the Archivematica Storage Service
// API used by migrate.
type Server struct {
	cfg       *Config
	state     *serverState
	mu        sync.RWMutex
	srv       *http.Server
	ln        net.Listener
	moveDelay time.Duration

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	started bool
}

type Option func(*Server)

// Snapshot provides an immutable view of the server state for assertions.
type Snapshot struct {
	Locations        map[string]storage_service.Location
	Packages         map[string]storage_service.Package
	PackageLocations map[string]string
	LocationOrder    []string
}

// WithMoveDelay adjusts how long the simulator keeps packages in the MOVING
// state before completing a move operation.
func WithMoveDelay(d time.Duration) Option {
	return func(s *Server) {
		s.moveDelay = d
	}
}

// NewServer builds a new simulator instance from the provided configuration.
func NewServer(cfg *Config, opts ...Option) (*Server, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}
	state, err := newStateFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	srv := &Server{
		cfg:       cfg,
		state:     state,
		moveDelay: defaultMoveDelay,
	}
	for _, opt := range opts {
		opt(srv)
	}
	return srv, nil
}

// NewServerFromFile loads a TOML configuration file and returns a simulator.
func NewServerFromFile(path string, opts ...Option) (*Server, error) {
	cfg, err := LoadConfig(path)
	if err != nil {
		return nil, err
	}
	return NewServer(cfg, opts...)
}

// Start begins serving HTTP requests in the background.
func (s *Server) Start() error {
	if s.started {
		return errors.New("server already started")
	}
	ln, err := net.Listen("tcp", s.cfg.Server.Listen)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	s.ln = ln
	s.ctx, s.cancel = context.WithCancel(context.Background())

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/api/v2/file/", s.handleFile)
	mux.HandleFunc("/api/v2/location/", s.handleLocation)
	mux.HandleFunc("/_internal/replicate", s.handleReplicate)

	s.srv = &http.Server{Handler: mux}
	s.started = true

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.srv.Serve(s.ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("ssmock: server stopped: %v", err)
		}
	}()
	return nil
}

// Run starts the server and blocks until the provided context is cancelled.
func (s *Server) Run(ctx context.Context) error {
	if err := s.Start(); err != nil {
		return err
	}
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return s.Shutdown(shutdownCtx)
}

// Addr returns the address the server is listening on.
func (s *Server) Addr() string {
	if s.ln == nil {
		return ""
	}
	return s.ln.Addr().String()
}

// WaitReady polls the health endpoint until the server responds or the context
// is cancelled.
func (s *Server) WaitReady(ctx context.Context) error {
	if !s.started {
		return errors.New("server not started")
	}
	url := fmt.Sprintf("http://%s/healthz", s.Addr())
	client := &http.Client{Timeout: 200 * time.Millisecond}
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		res, err := client.Do(req)
		if err == nil {
			res.Body.Close() //nolint:errcheck
			if res.StatusCode == http.StatusOK {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if !s.started {
		return nil
	}
	if s.cancel != nil {
		s.cancel()
	}
	err := s.srv.Shutdown(ctx)
	s.wg.Wait()
	s.started = false
	return err
}

// Snapshot clones the current simulator state for assertions. The returned
// structure is safe to read without further locking.
func (s *Server) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snap := Snapshot{
		Locations:        make(map[string]storage_service.Location, len(s.state.locations)),
		Packages:         make(map[string]storage_service.Package, len(s.state.packages)),
		PackageLocations: make(map[string]string, len(s.state.packages)),
		LocationOrder:    append([]string(nil), s.state.locationOrder...),
	}

	for id := range s.state.locations {
		cloned, _ := s.state.cloneLocation(id)
		snap.Locations[id] = *cloned
	}
	for id, pkg := range s.state.packages {
		cloned, _ := s.state.clonePackage(id)
		snap.Packages[id] = *cloned
		snap.PackageLocations[id] = pkg.locationID
	}
	return snap
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	remainder := strings.TrimPrefix(r.URL.Path, "/api/v2/file/")
	if remainder == r.URL.Path {
		http.NotFound(w, r)
		return
	}
	if strings.HasSuffix(remainder, "/move/") {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		id := strings.TrimSuffix(remainder, "/move/")
		if id == "" {
			http.NotFound(w, r)
			return
		}
		s.handleMove(w, r, id)
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	id := strings.TrimSuffix(remainder, "/")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	s.mu.RLock()
	pkg, ok := s.state.clonePackage(id)
	s.mu.RUnlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, pkg)
}

func (s *Server) handleMove(w http.ResponseWriter, r *http.Request, id string) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, fmt.Sprintf("invalid form: %v", err), http.StatusBadRequest)
		return
	}
	dest := r.FormValue("location_uuid")
	if dest == "" {
		http.Error(w, "location_uuid is required", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	pkgState, ok := s.state.packages[id]
	if !ok {
		s.mu.Unlock()
		http.NotFound(w, r)
		return
	}
	if _, ok = s.state.locations[dest]; !ok {
		s.mu.Unlock()
		http.Error(w, "unknown destination location", http.StatusBadRequest)
		return
	}
	if pkgState.locationID == dest && pkgState.pkg.Status == "UPLOADED" {
		s.mu.Unlock()
		w.WriteHeader(http.StatusOK)
		return
	}
	if pkgState.pkg.Status == "MOVING" && pkgState.pendingID == dest {
		s.mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
		return
	}
	pkgState.pkg.Status = "MOVING"
	pkgState.pendingID = dest
	s.mu.Unlock()

	s.scheduleMove(id)
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) scheduleMove(id string) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		timer := time.NewTimer(s.moveDelay)
		defer timer.Stop()
		select {
		case <-timer.C:
			// Check if context is cancelled before acquiring the lock
			select {
			case <-s.ctx.Done():
				return
			default:
			}

			s.mu.Lock()
			pkgState, ok := s.state.packages[id]
			if ok && pkgState.pendingID != "" {
				dest := pkgState.pendingID
				prev := pkgState.locationID
				pkgState.previousID = prev
				pkgState.locationID = dest
				pkgState.pendingID = ""
				pkgState.pkg.Status = "UPLOADED"
				destURI := locationResource(dest)
				currentBefore := pkgState.pkg.CurrentLocation
				pkgState.pkg.CurrentLocation = destURI
				if currentBefore != "" && currentBefore != destURI {
					pkgState.pkg.CurrentLocation = fmt.Sprintf("%s|%s", destURI, currentBefore)
				}
				pkgState.pkg.CurrentPath = fmt.Sprintf("/%s", pkgState.pkg.UUID)
				pkgState.pkg.CurrentFullPath = pkgState.pkg.CurrentPath
				if loc, ok := s.state.locations[dest]; ok {
					base := strings.TrimRight(loc.location.Path, "/")
					if base != "" {
						pkgState.pkg.CurrentFullPath = fmt.Sprintf("%s/%s", base, pkgState.pkg.UUID)
					}
				}
			}
			s.mu.Unlock()
		case <-s.ctx.Done():
			return
		}
	}()
}

func (s *Server) handleLocation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	remainder := strings.TrimPrefix(r.URL.Path, "/api/v2/location/")
	if remainder == r.URL.Path {
		http.NotFound(w, r)
		return
	}
	id := strings.TrimSuffix(remainder, "/")
	if id == "" {
		id = remainder
	}
	if id == "" {
		http.NotFound(w, r)
		return
	}

	s.mu.RLock()
	loc, ok := s.state.cloneLocation(id)
	s.mu.RUnlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, loc)
}

type replicateRequest struct {
	AIPUUID             string `json:"aip_uuid"`
	SourceLocationUUID  string `json:"source_location_uuid"`
	ReplicaLocationUUID string `json:"replica_location_uuid"`
}

type replicateResponse struct {
	Status      string `json:"status"`
	ReplicaUUID string `json:"replica_uuid,omitempty"`
	ReplicaURI  string `json:"replica_uri,omitempty"`
	Message     string `json:"message,omitempty"`
}

func (s *Server) handleReplicate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	defer r.Body.Close() //nolint:errcheck
	var req replicateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid body: %v", err), http.StatusBadRequest)
		return
	}
	if req.AIPUUID == "" || req.SourceLocationUUID == "" || req.ReplicaLocationUUID == "" {
		http.Error(w, "aip_uuid, source_location_uuid and replica_location_uuid are required", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	pkgState, ok := s.state.packages[req.AIPUUID]
	if !ok {
		writeJSON(w, &replicateResponse{Status: "missing", Message: "aip not found"})
		return
	}
	if pkgState.locationID != req.SourceLocationUUID {
		writeJSON(w, &replicateResponse{Status: "missing", Message: "aip not present in source location"})
		return
	}
	if _, ok := s.state.locations[req.ReplicaLocationUUID]; !ok {
		http.Error(w, "unknown replica location", http.StatusBadRequest)
		return
	}

	origURI := packageResource(pkgState.pkg.UUID)
	for _, candidate := range s.state.packages {
		if candidate.locationID == req.ReplicaLocationUUID && candidate.pkg.ReplicatedPackage == origURI {
			writeJSON(w, &replicateResponse{
				Status:      "noop",
				ReplicaUUID: candidate.pkg.UUID,
				ReplicaURI:  candidate.pkg.ResourceUri,
			})
			return
		}
	}

	replicaID := s.nextReplicaID(pkgState.pkg.UUID)
	replicaURI := packageResource(replicaID)

	replica := storageCopy(pkgState.pkg)
	replica.UUID = replicaID
	replica.ResourceUri = replicaURI
	replica.CurrentLocation = locationResource(req.ReplicaLocationUUID)
	replica.ReplicatedPackage = origURI
	replica.Replicas = nil
	replica.Status = "UPLOADED"
	replica.CurrentPath = fmt.Sprintf("/%s", replicaID)
	replica.CurrentFullPath = replica.CurrentPath
	if loc, ok := s.state.locations[req.ReplicaLocationUUID]; ok {
		base := strings.TrimRight(loc.location.Path, "/")
		if base != "" {
			replica.CurrentFullPath = fmt.Sprintf("%s/%s", base, replicaID)
		}
	}

	s.state.packages[replicaID] = &packageState{
		pkg:        replica,
		locationID: req.ReplicaLocationUUID,
		previousID: req.ReplicaLocationUUID,
	}

	if !slices.Contains(pkgState.pkg.Replicas, replicaURI) {
		pkgState.pkg.Replicas = append(append([]string(nil), pkgState.pkg.Replicas...), replicaURI)
	}

	writeJSON(w, &replicateResponse{
		Status:      "success",
		ReplicaUUID: replicaID,
		ReplicaURI:  replicaURI,
	})
}

func (s *Server) nextReplicaID(base string) string {
	current := base
	for range 1024 {
		next, err := incrementUUIDLastSegment(current)
		if err != nil {
			break
		}
		if _, exists := s.state.packages[next]; !exists {
			return next
		}
		current = next
	}
	return uuid.NewString()
}

func incrementUUIDLastSegment(id string) (string, error) {
	u, err := uuid.Parse(id)
	if err != nil {
		return "", err
	}

	const segmentSize = 6
	lastBytes := u[16-segmentSize:]

	buf := make([]byte, 8)
	copy(buf[8-segmentSize:], lastBytes)
	value := binary.BigEndian.Uint64(buf)
	value = (value + 1) & ((1 << (segmentSize * 8)) - 1)
	binary.BigEndian.PutUint64(buf, value)
	copy(u[16-segmentSize:], buf[8-segmentSize:])

	return u.String(), nil
}

func storageCopy(pkg storage_service.Package) storage_service.Package {
	clone := pkg
	if pkg.Replicas != nil {
		clone.Replicas = append([]string(nil), pkg.Replicas...)
	}
	if pkg.RelatedPackages != nil {
		clone.RelatedPackages = append([]string(nil), pkg.RelatedPackages...)
	}
	return clone
}

func methodNotAllowed(w http.ResponseWriter, method string) {
	w.Header().Set("Allow", method)
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, fmt.Sprintf("encode response: %v", err), http.StatusInternalServerError)
	}
}
