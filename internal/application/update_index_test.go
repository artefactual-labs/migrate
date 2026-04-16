package application

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/artefactual-labs/migrate/internal/storage_service"
	"gotest.tools/v3/assert"
)

func TestBuildIndexFilePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		location    string
		currentPath string
		indexedPath string
		want        string
	}{
		{
			name:        "uses package current path when available",
			location:    "/mnt/aip_store_s3/",
			currentPath: "/daa0/5a8c/ec53/file.7z",
			indexedPath: "/old/root/file.7z",
			want:        "/mnt/aip_store_s3/daa0/5a8c/ec53/file.7z",
		},
		{
			name:        "falls back to indexed basename",
			location:    "/mnt/aip_store_s3",
			currentPath: "",
			indexedPath: "/old/root/daa0/5a8c/ec53/file.7z",
			want:        "/mnt/aip_store_s3/file.7z",
		},
		{
			name:        "returns location path when nothing else available",
			location:    "/mnt/aip_store_s3/",
			currentPath: "",
			indexedPath: "",
			want:        "/mnt/aip_store_s3",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := buildIndexFilePath(tc.location, tc.currentPath, tc.indexedPath)
			assert.Equal(t, got, tc.want)
		})
	}
}

func TestUpdateIndexMessageFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		current   string
		target    string
		indexed   string
		rebuilt   string
		want      []string
		wantNoop  bool
	}{
		{
			name:    "reports both fields changed",
			current: "Old Location",
			target:  "New Location",
			indexed: "/old/path/file.7z",
			rebuilt: "/new/path/file.7z",
			want:    []string{"location", "filePath"},
		},
		{
			name:    "reports filepath changed",
			current: "Same Location",
			target:  "Same Location",
			indexed: "/old/path/file.7z",
			rebuilt: "/new/path/file.7z",
			want:    []string{"filePath"},
		},
		{
			name:     "reports noop",
			current:  "Same Location",
			target:   "Same Location",
			indexed:  "/same/path/file.7z",
			rebuilt:  "/same/path/file.7z",
			wantNoop: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var updated []string
			if tc.current != tc.target {
				updated = append(updated, "location")
			}
			if tc.indexed != tc.rebuilt {
				updated = append(updated, "filePath")
			}

			if tc.wantNoop {
				assert.Equal(t, len(updated), 0)
				return
			}

			assert.DeepEqual(t, updated, tc.want)
		})
	}
}

func TestPackageReadyForIndexUpdate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		pkg    *storage_service.Package
		target string
		want   bool
	}{
		{
			name: "ready when uploaded in target location",
			pkg: &storage_service.Package{
				Status:          "UPLOADED",
				CurrentLocation: "/api/v2/location/location-uuid/",
			},
			target: "location-uuid",
			want:   true,
		},
		{
			name: "not ready when still moving",
			pkg: &storage_service.Package{
				Status:          "MOVING",
				CurrentLocation: "/api/v2/location/location-uuid/",
			},
			target: "location-uuid",
			want:   false,
		},
		{
			name: "not ready when in another location",
			pkg: &storage_service.Package{
				Status:          "UPLOADED",
				CurrentLocation: "/api/v2/location/other-location/",
			},
			target: "location-uuid",
			want:   false,
		},
		{
			name:   "not ready when package is nil",
			pkg:    nil,
			target: "location-uuid",
			want:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := packageReadyForIndexUpdate(tc.pkg, tc.target)
			assert.Equal(t, got, tc.want)
		})
	}
}

func TestFormatUpdateIndexMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		fields []string
		dryRun bool
		want   string
	}{
		{
			name:   "real update message",
			fields: []string{"location", "filePath"},
			want:   "Updated Elasticsearch fields: location, filePath",
		},
		{
			name:   "dry run message",
			fields: []string{"filePath"},
			dryRun: true,
			want:   "Dry run: would update Elasticsearch fields: filePath",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := formatUpdateIndexMessage(tc.fields, tc.dryRun)
			assert.Equal(t, got, tc.want)
		})
	}
}

func TestUpdateIndexADryRunDoesNotWriteToElasticsearch(t *testing.T) {
	t.Parallel()

	var updateCalls atomic.Int32

	esServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/aips/_search":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"hits": {
					"total": 1,
					"hits": [{
						"_index": "aips",
						"_type": "_doc",
						"_id": "doc-1",
						"_score": 1,
						"_source": {
							"uuid": "aip-uuid",
							"filePath": "/old/root/example.7z",
							"location": "Old Location"
						}
					}]
				}
			}`))
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/aips/_update/"):
			updateCalls.Add(1)
			t.Fatalf("unexpected Elasticsearch update request in dry-run mode: %s", r.URL.Path)
		default:
			t.Fatalf("unexpected Elasticsearch request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer esServer.Close()

	ssServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/location/location-uuid":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"description": "Target Location",
				"path": "/mnt/target",
				"uuid": "location-uuid"
			}`))
		case "/api/v2/file/aip-uuid/":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"uuid": "aip-uuid",
				"status": "UPLOADED",
				"current_location": "/api/v2/location/location-uuid/",
				"current_path": "tree/example.7z",
				"current_full_path": "/mnt/target/tree/example.7z"
			}`))
		default:
			t.Fatalf("unexpected Storage Service request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer ssServer.Close()

	app := &App{
		Config: &Config{
			Elastic: ElasticConfig{
				Host:    esServer.URL,
				Version: "v6",
			},
			StorageService: StorageServiceConfig{
				Locations: StorageServiceLocationConfig{
					MoveTargetLocationID: "location-uuid",
				},
			},
		},
		StorageClient: storage_service.NewAPI(http.DefaultClient, ssServer.URL, "user", "key"),
	}

	result, err := app.UpdateIndexA(context.Background(), UpdateIndexActivityParams{
		UUID:   "aip-uuid",
		DryRun: true,
	})
	assert.NilError(t, err)
	assert.Equal(t, result.Message, "Dry run: would update Elasticsearch fields: location, filePath")
	assert.Equal(t, updateCalls.Load(), int32(0))
}
