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

// makeTestJWTWithModules creates a signed JWT carrying a "modules" claim.
func makeTestJWTWithModules(t *testing.T, role string, modules []string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"user_id": "test-user-id",
		"role":    role,
		"modules": modules,
		"exp":     time.Now().Add(30 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(getJWTSecret())
	if err != nil {
		t.Fatalf("makeTestJWTWithModules: failed to sign token: %v", err)
	}
	return signed
}

func TestAuthMiddleware_ModuleEnforcement(t *testing.T) {
	tests := []struct {
		name           string
		token          string
		path           string
		expectedStatus int
	}{
		{
			name:           "UserWithModuleAllowed",
			token:          "modules:fronteira",
			path:           "/api/icms-fronteira/resumo",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "UserWithoutModuleBlocked",
			token:          "modules:notas",
			path:           "/api/icms-fronteira/resumo",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "UserEmptyModulesBlocked",
			token:          "modules:",
			path:           "/api/reforma/parametros",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "LegacyTokenWithoutClaimAllowed",
			token:          "legacy",
			path:           "/api/icms-fronteira/resumo",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "AdminBypassesModules",
			token:          "admin",
			path:           "/api/icms-fronteira/resumo",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "UnmappedPathNotBlocked",
			token:          "modules:",
			path:           "/api/auth/me",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var tokenStr string
			switch tc.token {
			case "legacy":
				tokenStr = makeTestJWT(t, "user")
			case "admin":
				tokenStr = makeTestJWT(t, "admin")
			case "modules:":
				tokenStr = makeTestJWTWithModules(t, "user", []string{})
			default:
				tokenStr = makeTestJWTWithModules(t, "user", []string{tc.token[len("modules:"):]})
			}
			handler := AuthMiddleware(innerOKHandler, "")
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.Header.Set("Authorization", "Bearer "+tokenStr)
			rr := httptest.NewRecorder()
			handler(rr, req)
			if rr.Code != tc.expectedStatus {
				t.Errorf("%s: got status %d, want %d", tc.name, rr.Code, tc.expectedStatus)
			}
		})
	}
}

func TestModuleForAPIPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/api/icms-fronteira/st-itens", "fronteira"},
		{"/api/reforma/modulo1/creditos", "reforma"},
		{"/api/auditoria-efd", "auditoria"},
		{"/api/fiscal/comparacao", "pacotefiscal"},
		{"/api/pacotefiscal/xml/upload", "pacotefiscal"},
		{"/api/xml/painel/entradas-informativos", "painel"}, // prefixo mais específico vence
		{"/api/xml/upload", "notas"},
		{"/api/nfe-entradas", "notas"},
		{"/api/mercadorias", "simulador"},
		{"/api/auth/login", ""},
		{"/api/config/aliquotas", ""},
		{"/api/erp-bridge/pending", ""},
	}
	for _, tc := range tests {
		if got := ModuleForAPIPath(tc.path); got != tc.want {
			t.Errorf("ModuleForAPIPath(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}
