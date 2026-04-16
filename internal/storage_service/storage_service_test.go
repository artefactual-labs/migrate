package storage_service_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/artefactual-labs/migrate/internal/storage_service"
)

func TestAPI(t *testing.T) {
	t.Parallel()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{
			"uuid":"9607cd13-99cd-46c9-82e6-4d7ef86ccaf7",
			"status":"ok"
		}`)
	}))
	t.Cleanup(func() { srv.Close() })

	c := srv.Client()
	client := storage_service.NewAPI(c, srv.URL, "user", "key")

	pkg, err := client.Packages.GetByID(t.Context(), "9607cd13-99cd-46c9-82e6-4d7ef86ccaf7")
	assert.NilError(t, err)
	assert.Equal(t, pkg.UUID, "9607cd13-99cd-46c9-82e6-4d7ef86ccaf7")
}

func TestListPackagesByLocation(t *testing.T) {
	t.Parallel()

	requests := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		assert.Equal(t, r.URL.Path, "/api/v2/file/")
		assert.Equal(t, r.URL.Query().Get("current_location__uuid"), "location-uuid")
		assert.Equal(t, r.URL.Query().Get("package_type"), "AIP")

		switch r.URL.Query().Get("offset") {
		case "0":
			_, _ = io.WriteString(w, `{
				"meta":{"next":"http://example.test/api/v2/file/?offset=2","total_count":3},
				"objects":[
					{"uuid":"11111111-1111-1111-1111-111111111111"},
					{"uuid":"22222222-2222-2222-2222-222222222222"}
				]
			}`)
		case "2":
			_, _ = io.WriteString(w, `{
				"meta":{"next":"","total_count":3},
				"objects":[
					{"uuid":"33333333-3333-3333-3333-333333333333"}
				]
			}`)
		default:
			t.Fatalf("unexpected offset %q", r.URL.Query().Get("offset"))
		}
	}))
	t.Cleanup(func() { srv.Close() })

	client := storage_service.NewAPI(srv.Client(), srv.URL, "user", "key")
	pkgs, err := client.Packages.ListByLocation(t.Context(), "location-uuid")
	assert.NilError(t, err)
	assert.Equal(t, requests, 2)
	assert.Equal(t, len(pkgs), 3)
	assert.Equal(t, pkgs[0].UUID, "11111111-1111-1111-1111-111111111111")
	assert.Equal(t, pkgs[2].UUID, "33333333-3333-3333-3333-333333333333")
}
