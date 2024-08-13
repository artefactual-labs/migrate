package storage_service

import (
	"fmt"
	"net/http"
)

type LocationService struct {
	client *Client
}

type Location struct {
	Description  string   `json:"description"`
	Enabled      bool     `json:"enabled"`
	Path         string   `json:"path"`
	Pipeline     []string `json:"pipeline"`
	Purpose      string   `json:"purpose"`
	Quota        string   `json:"quota"`
	RelativePath string   `json:"relative_path"`
	ResourceURI  string   `json:"resource_uri"`
	Space        string   `json:"space"`
	Used         int      `json:"used"`
	UUID         string   `json:"uuid"`
}

func (s *LocationService) Get(id string) (*Location, error) {
	var loc *Location
	path := fmt.Sprintf("/api/v2/location/%s", id)
	err := s.client.Call(http.MethodGet, path, nil, &loc)
	return loc, err
}
