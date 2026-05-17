package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizePath(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"/api/health", "/api/health"},
		{"/api/users/123", "/api/users/:n"},
		{"/api/users/123/items", "/api/users/:n/items"},
		{"/api/runs/550e8400-e29b-41d4-a716-446655440000", "/api/runs/:id"},
		{"/api/runs/550e8400-e29b-41d4-a716-446655440000/status", "/api/runs/:id/status"},
		{"/api/foo/123/bar/456", "/api/foo/:n/bar/:n"},
		{"/api/foo/123/", "/api/foo/:n"},
		{"/", "/"},
		{"/metrics", "/metrics"},
		{"/api/xml/upload-batches/abc-def/status", "/api/xml/upload-batches/abc-def/status"},
	}

	for _, c := range cases {
		got := normalizePath(c.input)
		if got != c.want {
			t.Errorf("normalizePath(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestMetricsMiddleware(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := MetricsMiddleware(inner)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestStatusRecorderWriteHeader(t *testing.T) {
	rr := httptest.NewRecorder()
	sr := &statusRecorder{ResponseWriter: rr, status: http.StatusOK}
	sr.WriteHeader(http.StatusNotFound)
	if sr.status != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", sr.status)
	}
}
