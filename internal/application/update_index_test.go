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
