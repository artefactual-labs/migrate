package ssmock

import (
	"sort"

	"github.com/pelletier/go-toml/v2"
)

type snapshotTOML struct {
	Locations []snapshotLocation `toml:"location"`
}

type snapshotLocation struct {
	ID       string            `toml:"id"`
	Packages []snapshotPackage `toml:"packages,omitempty"`
}

type snapshotPackage struct {
	ID     string `toml:"id"`
	Status string `toml:"status,omitempty"`
}

func (snap Snapshot) MarshalTOML() ([]byte, error) {
	var orderedLocations []string
	if len(snap.LocationOrder) > 0 {
		orderedLocations = append([]string(nil), snap.LocationOrder...)
	} else {
		orderedLocations = make([]string, 0, len(snap.Locations))
		for id := range snap.Locations {
			orderedLocations = append(orderedLocations, id)
		}
		sort.Strings(orderedLocations)
	}

	packagesByLocation := make(map[string][]snapshotPackage, len(snap.Locations))
	for pkgID, locID := range snap.PackageLocations {
		pkg, ok := snap.Packages[pkgID]
		if !ok {
			continue
		}
		packagesByLocation[locID] = append(packagesByLocation[locID], snapshotPackage{
			ID:     pkg.UUID,
			Status: pkg.Status,
		})
	}

	for id, pkgs := range packagesByLocation {
		sort.Slice(pkgs, func(i, j int) bool {
			return pkgs[i].ID < pkgs[j].ID
		})
		packagesByLocation[id] = pkgs
	}

	out := snapshotTOML{Locations: make([]snapshotLocation, 0, len(orderedLocations))}
	for _, id := range orderedLocations {
		loc := snapshotLocation{ID: id}
		if pkgs := packagesByLocation[id]; len(pkgs) > 0 {
			loc.Packages = pkgs
		}
		out.Locations = append(out.Locations, loc)
	}

	data, err := toml.Marshal(out)
	if err != nil {
		return nil, err
	}
	return data, nil
}
