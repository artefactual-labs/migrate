package ssmock

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/artefactual-labs/migrate/internal/storage_service"
)

func testConfig() *Config {
	return &Config{
		Server: ServerConfig{Listen: "127.0.0.1:0"},
		Locations: []LocationConfig{
			{
				ID:          "loc-1",
				Description: "Source",
				Packages: []PackageConfig{
					{ID: "pkg-1", Size: 2048},
				},
			},
			{ID: "loc-2", Description: "Destination"},
			{ID: "loc-3", Description: "Replica"},
		},
	}
}

func TestGetPackage(t *testing.T) {
	t.Parallel()

	srv := StartTestServer(t, testConfig(), WithMoveDelay(5*time.Millisecond))
	baseURL := fmt.Sprintf("http://%s", srv.Addr())

	resp, err := http.Get(fmt.Sprintf("%s/api/v2/file/%s/", baseURL, "pkg-1")) //nolint:noctx
	if err != nil {
		t.Fatalf("get package: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var pkg storage_service.Package
	if err := json.NewDecoder(resp.Body).Decode(&pkg); err != nil {
		t.Fatalf("decode package: %v", err)
	}
	if pkg.UUID != "pkg-1" {
		t.Fatalf("unexpected uuid: %s", pkg.UUID)
	}
	if pkg.Status != "UPLOADED" {
		t.Fatalf("unexpected status: %s", pkg.Status)
	}
}

func TestMovePackage(t *testing.T) {
	t.Parallel()

	srv := StartTestServer(t, testConfig(), WithMoveDelay(5*time.Millisecond))
	baseURL := fmt.Sprintf("http://%s", srv.Addr())

	form := url.Values{}
	form.Set("location_uuid", "loc-2")
	resp, err := http.PostForm(fmt.Sprintf("%s/api/v2/file/%s/move/", baseURL, "pkg-1"), form)
	if err != nil {
		t.Fatalf("move package: %v", err)
	}
	resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		pkg := fetchPackage(t, baseURL, "pkg-1")
		if pkg.Status == "UPLOADED" && strings.Contains(pkg.CurrentLocation, "loc-2") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("package did not move before timeout")
}

func TestReplicatePackage(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	srv := StartTestServer(t, cfg, WithMoveDelay(5*time.Millisecond))
	baseURL := fmt.Sprintf("http://%s", srv.Addr())

	// Move to destination first to satisfy source location requirement.
	form := url.Values{}
	form.Set("location_uuid", "loc-2")
	resp, err := http.PostForm(fmt.Sprintf("%s/api/v2/file/%s/move/", baseURL, "pkg-1"), form)
	if err != nil {
		t.Fatalf("move package: %v", err)
	}
	resp.Body.Close() //nolint:errcheck

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		pkg := fetchPackage(t, baseURL, "pkg-1")
		if pkg.Status == "UPLOADED" && strings.Contains(pkg.CurrentLocation, "loc-2") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	pkg := fetchPackage(t, baseURL, "pkg-1")
	if !strings.Contains(pkg.CurrentLocation, "loc-2") {
		t.Fatalf("package not in expected source location before replicate")
	}

	payload := map[string]string{
		"aip_uuid":              "pkg-1",
		"source_location_uuid":  "loc-2",
		"replica_location_uuid": "loc-3",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/_internal/replicate", baseURL), bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("replicate request: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var replicateResp replicateResponse
	if err := json.NewDecoder(resp.Body).Decode(&replicateResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if replicateResp.Status != "success" {
		t.Fatalf("unexpected replication status: %s", replicateResp.Status)
	}

	replica := fetchPackage(t, baseURL, replicateResp.ReplicaUUID)
	if !strings.Contains(replica.CurrentLocation, "loc-3") {
		t.Fatalf("replica stored in wrong location: %s", replica.CurrentLocation)
	}
	orig := fetchPackage(t, baseURL, "pkg-1")
	matched := false
	for _, uri := range orig.Replicas {
		if strings.Contains(uri, replicateResp.ReplicaUUID) {
			matched = true
			break
		}
	}
	if !matched {
		t.Fatalf("original package replicas not updated: %v", orig.Replicas)
	}
}

func fetchPackage(t *testing.T, baseURL, id string) *storage_service.Package {
	t.Helper()
	resp, err := http.Get(fmt.Sprintf("%s/api/v2/file/%s/", baseURL, id)) //nolint:noctx
	if err != nil {
		t.Fatalf("fetch package: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
	var pkg storage_service.Package
	if err := json.NewDecoder(resp.Body).Decode(&pkg); err != nil {
		t.Fatalf("decode package: %v", err)
	}
	return &pkg
}

func TestSnapshot(t *testing.T) {
	t.Parallel()

	srv := StartTestServer(t, testConfig())
	baseURL := fmt.Sprintf("http://%s", srv.Addr())

	form := url.Values{}
	form.Set("location_uuid", "loc-2")
	resp, err := http.PostForm(fmt.Sprintf("%s/api/v2/file/%s/move/", baseURL, "pkg-1"), form)
	if err != nil {
		t.Fatalf("move package: %v", err)
	}
	resp.Body.Close() //nolint:errcheck

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		snap := srv.Snapshot()
		if snap.PackageLocations["pkg-1"] == "loc-2" {
			pkg := snap.Packages["pkg-1"]
			if pkg.Status == "UPLOADED" && strings.Contains(pkg.CurrentLocation, "loc-2") {
				if snap.Locations["loc-1"].Used == 0 && snap.Locations["loc-2"].Used == int(pkg.Size) {
					return
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("snapshot did not reflect move before timeout")
}
