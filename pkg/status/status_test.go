package status

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetch(t *testing.T) {
	t.Run("operational returns green", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"status":{"description":"All Systems Operational"}}`) // nolint:errcheck
		}))
		defer srv.Close()

		got := Fetch(srv.Client(), srv.URL)
		if got != StatusOK {
			t.Errorf("got %q, want %q", got, StatusOK)
		}
	})

	t.Run("degraded returns warning", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"status":{"description":"Partially Degraded Service"}}`) // nolint:errcheck
		}))
		defer srv.Close()

		got := Fetch(srv.Client(), srv.URL)
		if got != StatusWARN+" degraded" {
			t.Errorf("got %q, want %q", got, StatusWARN+" degraded")
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `not json`) // nolint:errcheck
		}))
		defer srv.Close()

		got := Fetch(srv.Client(), srv.URL)
		if !strings.HasPrefix(got, StatusERR) {
			t.Errorf("got %q, want prefix %q", got, StatusERR)
		}
		if !strings.Contains(got, "reponse:") {
			t.Errorf("got %q, want 'reponse:' in error message", got)
		}
	})

	t.Run("connection error returns error", func(t *testing.T) {
		client := &http.Client{}
		got := Fetch(client, "http://127.0.0.1:1")
		if !strings.HasPrefix(got, StatusERR) {
			t.Errorf("got %q, want prefix %q", got, StatusERR)
		}
		if !strings.Contains(got, "request:") {
			t.Errorf("got %q, want 'request:' in error message", got)
		}
	})

	t.Run("case insensitive operational match", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"status":{"description":"all systems OPERATIONAL"}}`) // nolint:errcheck
		}))
		defer srv.Close()

		got := Fetch(srv.Client(), srv.URL)
		if got != StatusOK {
			t.Errorf("got %q, want %q", got, StatusOK)
		}
	})
}
