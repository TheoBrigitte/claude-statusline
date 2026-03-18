// Package status fetches and caches the Claude API operational status.
package status

import (
	"encoding/json"
	"fmt"
	"io"
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
)

var cacheRelPath = filepath.Join(".local", "state", "claude-status", "api_status.txt")

type apiStatusResponse struct {
	Status struct {
		Description string `json:"description"`
	} `json:"status"`
}

// Get returns the Claude API operational status as an emoji indicator.
//
// It uses a file-based cache at ~/.local/state/claude-status/api_status.txt
// to avoid hitting the status API on every invocation. If the cache file
// exists and was modified less than 10 minutes ago, the cached value is
// returned directly. Otherwise, it fetches a fresh status from the API,
// writes it to the cache file for subsequent calls, and returns it.
//
// Any filesystem errors are silently ignored — the function falls back
// to a live API fetch if the cache is unavailable.
func Get() string {
	var status string

	// Try to read from the file-based cache first.
	if home, err := os.UserHomeDir(); err == nil {
		statusFileFullPath := filepath.Join(home, cacheRelPath)
		if err := os.MkdirAll(filepath.Dir(statusFileFullPath), 0o755); err == nil { // nolint:gosec // G301: cache directory, world-readable is fine
			statusFile, err := os.OpenFile(statusFileFullPath, os.O_RDWR|os.O_CREATE, 0o644) //nolint:gosec // G302: cache file, world-readable is fine
			if err == nil {
				if info, err := statusFile.Stat(); err == nil {
					if time.Since(info.ModTime()) < cacheDuration {
						// Cache is fresh — return it without hitting the API.
						if cached, err := io.ReadAll(statusFile); err == nil {
							return strings.TrimSpace(string(cached))
						}
					} else {
						// Cache is stale — truncate and schedule a write-back
						// after the live fetch completes via deferred closure.
						if err = statusFile.Truncate(0); err == nil {
							if _, err = statusFile.Seek(0, 0); err == nil {
								defer statusFile.Close() //nolint:errcheck
								defer func() {
									statusFile.WriteString(status) //nolint:errcheck,gosec
								}()
							}
						}
					}
				}
			}
		}
	}

	client := &http.Client{Timeout: 5 * time.Second}
	status = Fetch(client, apiURL)
	return status
}

// Fetch performs the HTTP request and interprets the response as a status indicator.
func Fetch(client *http.Client, url string) string {
	resp, err := client.Get(url)
	if err != nil {
		return StatusERR + fmt.Sprintf("request: %v", err.Error())
	}
	defer resp.Body.Close() //nolint:errcheck
	var r apiStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return StatusERR + fmt.Sprintf("reponse: %v", err.Error())
	}
	if strings.Contains(strings.ToLower(r.Status.Description), "operational") {
		return StatusOK
	}
	return StatusWARN + " degraded"
}
