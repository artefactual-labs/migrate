package updateindexcmd

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/artefactual-labs/migrate/internal/application"
	"github.com/artefactual-labs/migrate/internal/cmd/rootcmd"
	"github.com/artefactual-labs/migrate/internal/storage_service"
)

func TestLoadUUIDsFromTargetLocation(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.URL.Path, "/api/v2/file/")
		assert.Equal(t, r.URL.Query().Get("current_location__uuid"), "location-uuid")
		assert.Equal(t, r.URL.Query().Get("package_type"), "AIP")
		_, _ = io.WriteString(w, `{
			"meta": {"next": "", "total_count": 2},
			"objects": [
				{"uuid":"2faa61dc-ed33-49f4-8b36-954f203bab4a"},
				{"uuid":"daa05a8c-ec53-4c27-968a-6e6bdba905ce"}
			]
		}`)
	}))
	t.Cleanup(func() { srv.Close() })

	cfg := &Config{
		RootConfig:          rootcmd.New(nil, io.Discard, io.Discard),
		AllInTargetLocation: true,
	}
	app := &application.App{
		Config: &application.Config{
			StorageService: application.StorageServiceConfig{
				Locations: application.StorageServiceLocationConfig{
					MoveTargetLocationID: "location-uuid",
				},
			},
		},
		StorageClient: storage_service.NewAPI(srv.Client(), srv.URL, "user", "key"),
	}

	uuids, err := cfg.loadUUIDs(context.Background(), app)
	assert.NilError(t, err)
	assert.Equal(t, len(uuids), 2)
	assert.Equal(t, uuids[0].String(), "2faa61dc-ed33-49f4-8b36-954f203bab4a")
	assert.Equal(t, uuids[1].String(), "daa05a8c-ec53-4c27-968a-6e6bdba905ce")
}

func TestSummaryRecordResult(t *testing.T) {
	t.Parallel()

	stats := &summary{}
	messages := []string{
		"Updated Elasticsearch fields: location, filePath",
		"Dry run: would update Elasticsearch fields: filePath",
		"Elasticsearch update not needed: location and filePath already matched target",
		"Elasticsearch update skipped: package status is MOVING at location /api/v2/location/location-uuid/",
		"something unexpected",
	}

	for _, msg := range messages {
		stats.recordResult(msg)
	}

	assert.Equal(t, stats.updated, 1)
	assert.Equal(t, stats.wouldUpdate, 1)
	assert.Equal(t, stats.noChange, 1)
	assert.Equal(t, stats.skipped, 1)
	assert.Equal(t, stats.failed, 1)
}
