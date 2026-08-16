package status

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func serving(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, body) //nolint:errcheck
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFetch(t *testing.T) {
	t.Run("operational returns green", func(t *testing.T) {
		srv := serving(t, `{"status":{"description":"All Systems Operational"}}`)

		got, err := Fetch(srv.Client(), srv.URL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != StatusOK {
			t.Errorf("got %q, want %q", got, StatusOK)
		}
	})

	t.Run("degraded returns warning", func(t *testing.T) {
		srv := serving(t, `{"status":{"description":"Partially Degraded Service"}}`)

		got, err := Fetch(srv.Client(), srv.URL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != StatusWARN+" degraded" {
			t.Errorf("got %q, want %q", got, StatusWARN+" degraded")
		}
	})

	t.Run("case insensitive operational match", func(t *testing.T) {
		srv := serving(t, `{"status":{"description":"all systems OPERATIONAL"}}`)

		got, err := Fetch(srv.Client(), srv.URL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != StatusOK {
			t.Errorf("got %q, want %q", got, StatusOK)
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		srv := serving(t, `not json`)

		got, err := Fetch(srv.Client(), srv.URL)
		assertShortError(t, got, err, "response:")
	})

	t.Run("missing description returns error", func(t *testing.T) {
		srv := serving(t, `{"status":{}}`)

		got, err := Fetch(srv.Client(), srv.URL)
		assertShortError(t, got, err, "response:")
	})

	t.Run("non-2xx response returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, `{"status":{"description":"All Systems Operational"}}`) //nolint:errcheck
		}))
		defer srv.Close()

		got, err := Fetch(srv.Client(), srv.URL)
		assertShortError(t, got, err, "unexpected status")
	})

	t.Run("connection error returns error", func(t *testing.T) {
		got, err := Fetch(&http.Client{}, "http://127.0.0.1:1")
		assertShortError(t, got, err, "request:")
	})
}

// assertShortError checks a failed fetch reports StatusERR on its own. The
// indicator is rendered into a width-constrained status line, so the failure
// detail belongs in the error rather than in the returned string.
func assertShortError(t *testing.T, got string, err error, wantDetail string) {
	t.Helper()
	if err == nil {
		t.Fatalf("got %q with no error, want an error", got)
	}
	if got != StatusERR {
		t.Errorf("indicator = %q, want exactly %q with no detail appended", got, StatusERR)
	}
	if !strings.Contains(err.Error(), wantDetail) {
		t.Errorf("error = %q, want it to mention %q", err, wantDetail)
	}
}

func TestReadCache(t *testing.T) {
	write := func(t *testing.T, contents string, age time.Duration) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "api_status.txt")
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
		when := time.Now().Add(-age)
		if err := os.Chtimes(path, when, when); err != nil {
			t.Fatal(err)
		}
		return path
	}

	t.Run("fresh non-empty cache is a hit", func(t *testing.T) {
		got, ok := readCache(write(t, StatusOK+"\n", time.Minute))
		if !ok || got != StatusOK {
			t.Errorf("got (%q, %v), want (%q, true)", got, ok, StatusOK)
		}
	})

	t.Run("stale cache is a miss", func(t *testing.T) {
		if _, ok := readCache(write(t, StatusOK, cacheDuration+time.Minute)); ok {
			t.Error("stale cache reported as a hit")
		}
	})

	// Regression: the cache file used to be created empty with a fresh
	// timestamp, which counted as a hit and forced a live HTTP request on
	// every render for a full cache duration.
	t.Run("fresh but empty cache is a miss", func(t *testing.T) {
		if _, ok := readCache(write(t, "", time.Second)); ok {
			t.Error("empty cache reported as a hit")
		}
		if _, ok := readCache(write(t, "  \n", time.Second)); ok {
			t.Error("blank cache reported as a hit")
		}
	})

	t.Run("missing cache is a miss", func(t *testing.T) {
		if _, ok := readCache(filepath.Join(t.TempDir(), "absent.txt")); ok {
			t.Error("missing cache reported as a hit")
		}
	})
}

func TestWriteCacheRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "api_status.txt")

	writeCache(path, StatusOK)
	got, ok := readCache(path)
	if !ok || got != StatusOK {
		t.Fatalf("got (%q, %v), want (%q, true)", got, ok, StatusOK)
	}

	// A later write must replace the previous value rather than append to it.
	writeCache(path, StatusERR)
	if got, _ := readCache(path); got != StatusERR {
		t.Errorf("got %q after rewrite, want %q", got, StatusERR)
	}
}
