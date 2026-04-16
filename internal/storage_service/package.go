package storage_service

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
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

type PackageListResponse struct {
	Meta struct {
		Limit      int    `json:"limit"`
		Next       string `json:"next"`
		Offset     int    `json:"offset"`
		Previous   string `json:"previous"`
		TotalCount int    `json:"total_count"`
	} `json:"meta"`
	Objects []Package `json:"objects"`
}

func (s *PackageService) GetByID(ctx context.Context, id string) (*Package, error) {
	var pkg *Package
	path := fmt.Sprintf("/api/v2/file/%s/", id)
	err := s.client.Call(ctx, http.MethodGet, path, nil, &pkg)
	return pkg, err
}

func (s *PackageService) ListByLocation(ctx context.Context, locationID string) ([]Package, error) {
	const pageSize = 1000

	offset := 0
	result := []Package{}
	for {
		values := url.Values{}
		values.Set("current_location__uuid", locationID)
		values.Set("package_type", "AIP")
		values.Set("limit", strconv.Itoa(pageSize))
		values.Set("offset", strconv.Itoa(offset))

		var res PackageListResponse
		path := fmt.Sprintf("/api/v2/file/?%s", values.Encode())
		if err := s.client.Call(ctx, http.MethodGet, path, nil, &res); err != nil {
			return nil, err
		}

		result = append(result, res.Objects...)
		offset += len(res.Objects)
		if res.Meta.Next == "" || len(res.Objects) == 0 {
			break
		}
	}

	return result, nil
}

func (s *PackageService) Move(ctx context.Context, packageID, locationID string) error {
	path := fmt.Sprintf("/api/v2/file/%s/move/", packageID)
	p := url.Values{}
	p.Set("location_uuid", locationID)
	return s.client.Call(ctx, http.MethodPost, path, p.Encode(), nil)
}

type FixityResponse struct {
	Success   bool           `json:"success"`
	Message   string         `json:"message"`
	Timestamp string         `json:"timestamp"`
	Failures  FixityFailures `json:"failures"`
}

type FixityFailures struct {
	Files struct {
		Missing   []string `json:"missing"`
		Changed   []string `json:"changed"`
		Untracked []string `json:"untracked"`
	} `json:"files"`
}

func (s *PackageService) CheckFixity(ctx context.Context, id string) (*FixityResponse, error) {
	res := &FixityResponse{}
	path := fmt.Sprintf("/api/v2/file/%s/check_fixity/", id)
	err := s.client.Call(ctx, http.MethodGet, path, nil, res)
	return res, err
}
