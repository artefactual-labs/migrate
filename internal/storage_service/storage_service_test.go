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
