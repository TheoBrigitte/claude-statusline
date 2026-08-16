// Package status fetches and caches the Claude API operational status.
package status

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	StatusOK   = "🟢"
	StatusWARN = "🟡"
	StatusERR  = "🔴"

	apiURL        = "https://status.claude.com/api/v2/status.json"
	cacheDuration = 10 * time.Minute
	httpTimeout   = 5 * time.Second
)

var cacheRelPath = filepath.Join(".local", "state", "claude-status", "api_status.txt")

type apiStatusResponse struct {
	Status struct {
		Description string `json:"description"`
	} `json:"status"`
}

// Get returns the Claude API operational status as an emoji indicator.
//
// It uses a file-based cache at ~/.local/state/claude-status/api_status.txt to
// avoid hitting the status API on every invocation: a cache file written less
// than 10 minutes ago is returned as-is. Otherwise the status is fetched from
// the API and written back to the cache.
//
// Failures are cached too. The status line runs on every prompt render, so an
// unreachable API must not cost an HTTP timeout each time; the indicator
// recovers on the next cache expiry.
//
// Filesystem errors are silently ignored — without a usable cache the function
// still returns a live status.
func Get() string {
	path, pathErr := cachePath()
	if pathErr == nil {
		if cached, ok := readCache(path); ok {
			return cached
		}
	}

	status, _ := Fetch(&http.Client{Timeout: httpTimeout}, apiURL)

	if pathErr == nil {
		writeCache(path, status)
	}
	return status
}

// cachePath returns the cache file path, creating its parent directory.
func cachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(home, cacheRelPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // G301: cache directory, world-readable is fine
		return "", err
	}
	return path, nil
}

// readCache returns the cached status, and whether it is usable: the file has
// to exist, be younger than cacheDuration, and hold a non-empty value.
//
// The emptiness check matters on the very first run. Creating the file and
// treating it as a hit because its timestamp is recent would serve an empty
// status for a full cache duration, turning every render into a live fetch.
func readCache(path string) (string, bool) {
	info, err := os.Stat(path)
	if err != nil || time.Since(info.ModTime()) >= cacheDuration {
		return "", false
	}
	contents, err := os.ReadFile(path) //nolint:gosec // G304: path is derived from the user's home directory
	if err != nil {
		return "", false
	}
	status := strings.TrimSpace(string(contents))
	return status, status != ""
}

// writeCache stores status for subsequent invocations. Errors are ignored: a
// cache that cannot be written only costs an extra fetch next time.
func writeCache(path, status string) {
	_ = os.WriteFile(path, []byte(status), 0o644) //nolint:gosec // G306: cache file, world-readable is fine
}

// Fetch performs the HTTP request and interprets the response as a status
// indicator.
//
// The indicator is always short enough to sit in a status line; when the
// request or the response is unusable it is StatusERR and the detail is
// carried by the returned error rather than rendered on screen.
func Fetch(client *http.Client, url string) (string, error) {
	resp, err := client.Get(url)
	if err != nil {
		return StatusERR, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return StatusERR, fmt.Errorf("response: unexpected status %s", resp.Status)
	}

	var r apiStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return StatusERR, fmt.Errorf("response: %w", err)
	}
	if r.Status.Description == "" {
		return StatusERR, fmt.Errorf("response: missing status description")
	}

	if strings.Contains(strings.ToLower(r.Status.Description), "operational") {
		return StatusOK, nil
	}
	return StatusWARN + " degraded", nil
}
