package storage_service

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

type PackageService struct {
	client *Client
}

type Package struct {
	UUID              string   `json:"uuid"`
	CurrentFullPath   string   `json:"current_full_path"`
	CurrentLocation   string   `json:"current_location"`
	CurrentPath       string   `json:"current_path"`
	Encrypted         bool     `json:"encrypted"`
	OriginPipeline    string   `json:"origin_pipeline"`
	PackageType       string   `json:"package_type"`
	RelatedPackages   []string `json:"related_packages"`
	Replicas          []string `json:"replicas"`
	ReplicatedPackage string   `json:"replicated_package"`
	ResourceUri       string   `json:"resource_uri"`
	Size              uint64   `json:"size"`
	Status            string   `json:"status"`
	StoredDate        string   `json:"stored_date"`
}

func (s *PackageService) GetByID(ctx context.Context, id string) (*Package, error) {
	var pkg *Package
	path := fmt.Sprintf("/api/v2/file/%s/", id)
	err := s.client.Call(ctx, http.MethodGet, path, nil, &pkg)
	return pkg, err
}

func (s *PackageService) Move(ctx context.Context, packageID, locationID string) error {
	path := fmt.Sprintf("/api/v2/file/%s/move/", packageID)
	p := url.Values{}
	p.Set("location_uuid", locationID)
	return s.client.Call(ctx, http.MethodPost, path, p.Encode(), nil)
}
