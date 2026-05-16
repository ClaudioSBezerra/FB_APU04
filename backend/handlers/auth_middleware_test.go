package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// makeTestJWT creates a signed JWT with the given role using getJWTSecret().
func makeTestJWT(t *testing.T, role string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"user_id": "test-user-id",
		"role":    role,
		"exp":     time.Now().Add(30 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(getJWTSecret())
	if err != nil {
		t.Fatalf("makeTestJWT: failed to sign token: %v", err)
	}
	return signed
}

// innerOKHandler is a simple next handler that always returns 200.
var innerOKHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func TestAuthMiddleware_RejectCases(t *testing.T) {
	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
	}{
		{
			name:           "NoHeader",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "MalformedHeader",
			authHeader:     "InvalidToken",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "InvalidJWT",
			authHeader:     "Bearer nao.e.jwt.valido",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler := AuthMiddleware(innerOKHandler, "admin")
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rr := httptest.NewRecorder()
			handler(rr, req)
			if rr.Code != tc.expectedStatus {
				t.Errorf("TestAuthMiddleware_%s: got status %d, want %d", tc.name, rr.Code, tc.expectedStatus)
			}
		})
	}
}

func TestAuthMiddleware_ValidJWTWrongRole(t *testing.T) {
	// JWT with role="user", middleware requires "admin" → 403
	tokenStr := makeTestJWT(t, "user")
	handler := AuthMiddleware(innerOKHandler, "admin")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("TestAuthMiddleware_ValidJWTWrongRole: got status %d, want %d", rr.Code, http.StatusForbidden)
	}
}

func TestAuthMiddleware_PassesThrough(t *testing.T) {
	// JWT with role="admin", middleware requires "admin" → 200 from inner handler
	tokenStr := makeTestJWT(t, "admin")
	handler := AuthMiddleware(innerOKHandler, "admin")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("TestAuthMiddleware_PassesThrough: got status %d, want %d", rr.Code, http.StatusOK)
	}
}
