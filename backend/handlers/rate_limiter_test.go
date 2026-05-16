package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiter_Allow(t *testing.T) {
	rl := newRateLimiter(3, time.Minute)
	for i := 1; i <= 3; i++ {
		if !rl.Allow("key") {
			t.Errorf("TestRateLimiter_Allow: attempt %d should be allowed but was denied", i)
		}
	}
}

func TestRateLimiter_ExceedsLimit(t *testing.T) {
	rl := newRateLimiter(3, time.Minute)
	for i := 0; i < 3; i++ {
		rl.Allow("key")
	}
	// 4th attempt must be denied
	if rl.Allow("key") {
		t.Error("TestRateLimiter_ExceedsLimit: 4th attempt should be denied but was allowed")
	}
}

func TestRateLimiter_Reset(t *testing.T) {
	rl := newRateLimiter(3, time.Minute)
	for i := 0; i < 3; i++ {
		rl.Allow("key")
	}
	// Limit reached — next Allow returns false
	if rl.Allow("key") {
		t.Error("TestRateLimiter_Reset: expected 4th Allow to be denied before Reset")
	}
	rl.Reset("key")
	// After Reset, Allow should return true
	if !rl.Allow("key") {
		t.Error("TestRateLimiter_Reset: expected Allow to be permitted after Reset")
	}
}

func TestRateLimiter_IsLimited(t *testing.T) {
	rl := newRateLimiter(3, time.Minute)
	for i := 0; i < 3; i++ {
		rl.Allow("key")
	}
	if !rl.IsLimited("key") {
		t.Error("TestRateLimiter_IsLimited: expected IsLimited to return true after exceeding limit")
	}
}

func TestGetClientIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	got := GetClientIP(req)
	want := "1.2.3.4"
	if got != want {
		t.Errorf("TestGetClientIP_XForwardedFor: got %q, want %q", got, want)
	}
}

func TestGetClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// httptest.NewRequest sets RemoteAddr to "192.0.2.1:1234" by default
	// GetClientIP strips the port → "192.0.2.1"
	got := GetClientIP(req)
	want := "192.0.2.1"
	if got != want {
		t.Errorf("TestGetClientIP_RemoteAddr: got %q, want %q", got, want)
	}
}
