package version

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// GitHubRepo is the repository used for the update check.
const GitHubRepo = "Inboxes-CIG/Bastion"

// Version is injected at build time via -ldflags.
var Version = "dev"

type Info struct {
	Current         string    `json:"current"`
	Latest          string    `json:"latest"`
	UpdateAvailable bool      `json:"update_available"`
	URL             string    `json:"url"`
	CheckedAt       time.Time `json:"checked_at"`
}

type cacheFile struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest"`
	URL       string    `json:"url"`
}

// Check returns the current version and the latest GitHub release (cached 24h).
func Check(cachePath string) Info {
	info := Info{Current: Version, CheckedAt: time.Now()}

	if cachePath != "" {
		if b, err := os.ReadFile(cachePath); err == nil {
			var c cacheFile
			if json.Unmarshal(b, &c) == nil && time.Since(c.CheckedAt) < 24*time.Hour {
				info.Latest = c.Latest
				info.URL = c.URL
				info.CheckedAt = c.CheckedAt
				info.UpdateAvailable = newer(c.Latest, Version)
				return info
			}
		}
	}

	req, err := http.NewRequest("GET", "https://api.github.com/repos/"+GitHubRepo+"/releases/latest", nil)
	if err != nil {
		return info
	}
	req.Header.Set("User-Agent", "Bastion/"+Version)
	req.Header.Set("Accept", "application/vnd.github+json")
	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return info
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return info
	}
	var rel struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if json.NewDecoder(resp.Body).Decode(&rel) != nil {
		return info
	}
	info.Latest = strings.TrimPrefix(rel.TagName, "v")
	info.URL = rel.HTMLURL
	info.UpdateAvailable = newer(info.Latest, Version)

	if cachePath != "" {
		c := cacheFile{CheckedAt: info.CheckedAt, Latest: info.Latest, URL: info.URL}
		if b, err := json.Marshal(c); err == nil {
			_ = os.WriteFile(cachePath, b, 0o644)
		}
	}
	return info
}

func newer(latest, current string) bool {
	if latest == "" || current == "" || current == "dev" {
		return false
	}
	l := parse(latest)
	c := parse(current)
	for i := 0; i < 3; i++ {
		if l[i] > c[i] {
			return true
		}
		if l[i] < c[i] {
			return false
		}
	}
	return false
}

func parse(v string) [3]int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")
	var out [3]int
	for i := 0; i < 3 && i < len(parts); i++ {
		out[i], _ = strconv.Atoi(parts[i])
	}
	return out
}
