package proxy

import (
	"testing"

	"github.com/inboxes-cig/bastion/internal/db"
)

func TestMatchLocation(t *testing.T) {
	loc := func(path string, enabled bool) db.SiteLocation {
		return db.SiteLocation{PathPrefix: path, Enabled: enabled}
	}
	locs := []db.SiteLocation{
		loc("/api", true),
		loc("/api/v1", true),
		loc("/static", true),
		loc("/off", false),
		loc("/", true),
	}

	tests := []struct {
		name string
		path string
		want string
	}{
		{"exact", "/api", "/api"},
		{"child", "/api/users", "/api"},
		{"longest prefix", "/api/v1/users", "/api/v1"},
		{"segment boundary", "/apix", "/"},
		{"subpath", "/static/app.js", "/static"},
		{"disabled skipped", "/off", "/"},
		{"root fallback", "/anything", "/"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := matchLocation(locs, tc.path)
			if got == nil {
				t.Fatalf("matchLocation(%q) = nil, want %q", tc.path, tc.want)
			}
			if got.PathPrefix != tc.want {
				t.Fatalf("matchLocation(%q) = %q, want %q", tc.path, got.PathPrefix, tc.want)
			}
		})
	}
}

func TestMatchLocationNoMatch(t *testing.T) {
	locs := []db.SiteLocation{{PathPrefix: "/api", Enabled: true}}
	if got := matchLocation(locs, "/other"); got != nil {
		t.Fatalf("matchLocation = %+v, want nil", got)
	}
}
