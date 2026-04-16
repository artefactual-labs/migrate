package application

import (
	"testing"

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
