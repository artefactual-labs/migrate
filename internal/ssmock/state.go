package ssmock

import (
	"fmt"
	"time"

	"github.com/artefactual-labs/migrate/internal/storage_service"
)

type serverState struct {
	locations     map[string]*locationState
	locationOrder []string
	packages      map[string]*packageState
}

type locationState struct {
	location storage_service.Location
}

type packageState struct {
	pkg        storage_service.Package
	locationID string
	pendingID  string
	previousID string
}

func newStateFromConfig(cfg *Config) (*serverState, error) {
	st := &serverState{
		locations: make(map[string]*locationState, len(cfg.Locations)),
		packages:  make(map[string]*packageState),
	}

	for _, loc := range cfg.Locations {
		resURI := locationResource(loc.ID)
		description := loc.Description
		if description == "" {
			description = fmt.Sprintf("Location %s", loc.ID)
		}
		purpose := loc.Purpose
		if purpose == "" {
			purpose = "AS"
		}

		locCopy := storage_service.Location{
			Description:  description,
			Enabled:      true,
			Path:         loc.Path,
			Pipeline:     append([]string(nil), loc.Pipeline...),
			Purpose:      purpose,
			Quota:        loc.Quota,
			RelativePath: loc.Relative,
			ResourceURI:  resURI,
			Space:        loc.Space,
			Used:         0,
			UUID:         loc.ID,
		}

		st.locations[loc.ID] = &locationState{location: locCopy}
		st.locationOrder = append(st.locationOrder, loc.ID)

		for _, pkg := range loc.Packages {
			if _, exists := st.packages[pkg.ID]; exists {
				return nil, fmt.Errorf("duplicate package id %q", pkg.ID)
			}

			status := pkg.Status
			if status == "" {
				status = "UPLOADED"
			}
			currentPath := pkg.CurrentPath
			if currentPath == "" {
				currentPath = fmt.Sprintf("/%s", pkg.ID)
			}
			currentFullPath := pkg.CurrentFullPath
			if currentFullPath == "" {
				if loc.Path != "" {
					currentFullPath = fmt.Sprintf("%s/%s", loc.Path, pkg.ID)
				} else {
					currentFullPath = currentPath
				}
			}
			storedDate := pkg.StoredDate
			if storedDate == "" {
				storedDate = time.Now().UTC().Format(time.RFC3339)
			}

			replicas := make([]string, len(pkg.Replicas))
			for i, replica := range pkg.Replicas {
				replicas[i] = packageResource(replica)
			}

			replicatedPackage := pkg.ReplicatedPackage
			if replicatedPackage != "" {
				replicatedPackage = packageResource(replicatedPackage)
			}

			st.packages[pkg.ID] = &packageState{
				locationID: loc.ID,
				previousID: loc.ID,
				pkg: storage_service.Package{
					UUID:              pkg.ID,
					CurrentFullPath:   currentFullPath,
					CurrentLocation:   locationResource(loc.ID),
					CurrentPath:       currentPath,
					Encrypted:         pkg.Encrypted,
					OriginPipeline:    pkg.OriginPipeline,
					PackageType:       pkg.PackageType,
					RelatedPackages:   nil,
					Replicas:          replicas,
					ReplicatedPackage: replicatedPackage,
					ResourceUri:       packageResource(pkg.ID),
					Size:              pkg.Size,
					Status:            status,
					StoredDate:        storedDate,
				},
			}

			st.locations[loc.ID].location.Used += int(pkg.Size)
		}
	}

	return st, nil
}

func (s *serverState) clonePackage(id string) (*storage_service.Package, bool) {
	pkgState, ok := s.packages[id]
	if !ok {
		return nil, false
	}
	clone := pkgState.pkg
	if pkgState.pkg.Replicas != nil {
		clone.Replicas = append([]string(nil), pkgState.pkg.Replicas...)
	}
	if pkgState.pkg.RelatedPackages != nil {
		clone.RelatedPackages = append([]string(nil), pkgState.pkg.RelatedPackages...)
	}
	return &clone, true
}

func (s *serverState) cloneLocation(id string) (*storage_service.Location, bool) {
	locState, ok := s.locations[id]
	if !ok {
		return nil, false
	}
	clone := locState.location
	if locState.location.Pipeline != nil {
		clone.Pipeline = append([]string(nil), locState.location.Pipeline...)
	}
	used := 0
	for _, pkg := range s.packages {
		if pkg.locationID == id {
			used += int(pkg.pkg.Size)
		}
	}
	clone.Used = used
	return &clone, true
}

func packageResource(id string) string {
	return fmt.Sprintf("/api/v2/file/%s/", id)
}

func locationResource(id string) string {
	return fmt.Sprintf("/api/v2/location/%s/", id)
}
