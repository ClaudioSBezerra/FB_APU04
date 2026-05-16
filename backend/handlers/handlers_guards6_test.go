package handlers

// handlers_guards6_test.go — sexta extensão de cobertura
// Cobre: handlers erp_bridge.go (guards antes de DB), handlers auth.go
// (ForgotPasswordHandler, RegisterHandler, ForgotPasswordHandler),
// upload.go (CheckDuplicityHandler, UploadHandler), environment.go guards,
// e handlers adicionais.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ─── ERP Bridge handlers — no-auth (erpBridgeGetCompany returns 401) ─────────

func TestERPBridgeConfigHandler_NoAuth(t *testing.T) {
	handler := ERPBridgeConfigHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/erp-bridge/config", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("ERPBridgeConfigHandler GET no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestERPBridgeRunsHandler_NoAuth(t *testing.T) {
	handler := ERPBridgeRunsHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/erp-bridge/runs", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("ERPBridgeRunsHandler GET no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestERPBridgeRunHandler_NoAuth(t *testing.T) {
	handler := ERPBridgeRunHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/erp-bridge/runs/123", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("ERPBridgeRunHandler GET no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestERPBridgeServidoresHandler_NoAuth(t *testing.T) {
	handler := ERPBridgeServidoresHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/erp-bridge/servidores", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("ERPBridgeServidoresHandler GET no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestERPBridgeRegistrarServidoresHandler_MethodGuard(t *testing.T) {
	handler := ERPBridgeRegistrarServidoresHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/erp-bridge/servidores/registrar", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("ERPBridgeRegistrarServidoresHandler GET: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestERPBridgeRegistrarServidoresHandler_NoAuth(t *testing.T) {
	handler := ERPBridgeRegistrarServidoresHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/erp-bridge/servidores/registrar", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("ERPBridgeRegistrarServidoresHandler POST no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestERPBridgeGenerateAPIKeyHandler_MethodGuard(t *testing.T) {
	handler := ERPBridgeGenerateAPIKeyHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/erp-bridge/generate-api-key", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("ERPBridgeGenerateAPIKeyHandler GET: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestERPBridgeGenerateAPIKeyHandler_NoAuth(t *testing.T) {
	handler := ERPBridgeGenerateAPIKeyHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/erp-bridge/generate-api-key", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("ERPBridgeGenerateAPIKeyHandler POST no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestERPBridgeCredentialsHandler_MethodGuard(t *testing.T) {
	handler := ERPBridgeCredentialsHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/erp-bridge/credentials", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("ERPBridgeCredentialsHandler POST: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestERPBridgeCredentialsHandler_MissingKey(t *testing.T) {
	handler := ERPBridgeCredentialsHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/erp-bridge/credentials", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("ERPBridgeCredentialsHandler GET no key: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestERPBridgeTriggerHandler_MethodGuard(t *testing.T) {
	handler := ERPBridgeTriggerHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/erp-bridge/trigger", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("ERPBridgeTriggerHandler GET: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestERPBridgeTriggerHandler_NoAuth(t *testing.T) {
	handler := ERPBridgeTriggerHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/erp-bridge/trigger", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("ERPBridgeTriggerHandler POST no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestERPBridgePendingHandler_NoAuth(t *testing.T) {
	handler := ERPBridgePendingHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/erp-bridge/pending", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("ERPBridgePendingHandler GET no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestERPBridgeHeartbeatHandler_MethodGuard(t *testing.T) {
	handler := ERPBridgeHeartbeatHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/erp-bridge/heartbeat", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("ERPBridgeHeartbeatHandler GET: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestERPBridgeHeartbeatHandler_MissingKey(t *testing.T) {
	handler := ERPBridgeHeartbeatHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/erp-bridge/heartbeat", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("ERPBridgeHeartbeatHandler POST no key: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

// ─── Auth handlers — early JSON/body guards ───────────────────────────────────

func TestForgotPasswordHandler_InvalidBody(t *testing.T) {
	// Rate limiter allows, but JSON decode fails → 400
	handler := ForgotPasswordHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password",
		strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("ForgotPasswordHandler invalid body: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestResetPasswordHandler_InvalidBody(t *testing.T) {
	// JSON decode fails → 400
	handler := ResetPasswordHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password",
		strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("ResetPasswordHandler invalid body: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestResetPasswordHandler_PasswordMismatch(t *testing.T) {
	// Passwords don't match → 400 (before DB)
	handler := ResetPasswordHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password",
		strings.NewReader(`{"token":"abc","password":"pass1234","confirm_password":"different"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("ResetPasswordHandler password mismatch: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestResetPasswordHandler_ShortPassword(t *testing.T) {
	// Password < 8 chars → 400 (before DB)
	handler := ResetPasswordHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password",
		strings.NewReader(`{"token":"abc","password":"short","confirm_password":"short"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("ResetPasswordHandler short password: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// ─── Upload handlers ──────────────────────────────────────────────────────────

func TestCheckDuplicityHandler_OPTIONS(t *testing.T) {
	handler := CheckDuplicityHandler(nil)
	req := httptest.NewRequest(http.MethodOptions, "/api/check-duplicity", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("CheckDuplicityHandler OPTIONS: got %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestCheckDuplicityHandler_NoAuth(t *testing.T) {
	// No claims in context → GetUserIDFromContext returns "" → 401
	handler := CheckDuplicityHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/check-duplicity?cnpj=123&dt_ini=2026-01-01", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("CheckDuplicityHandler no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestUploadHandler_InvalidBody(t *testing.T) {
	// POST without multipart form → invalid file → 400 or 500 (before DB)
	handler := UploadHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/upload",
		strings.NewReader("not-multipart"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	// Handler will fail trying to get form file — status should not be 200
	if rr.Code == http.StatusOK {
		t.Errorf("UploadHandler invalid body: got %d, want non-200", rr.Code)
	}
}

// ─── Environment handlers — missing id guard ──────────────────────────────────

func TestDeleteEnvironmentHandler_MissingID(t *testing.T) {
	// No id param → 400, before DB access
	handler := DeleteEnvironmentHandler(nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/environments", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("DeleteEnvironmentHandler missing id: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// ─── CreateEnvironment/UpdateEnvironment — invalid JSON body → 400 ────────────

func TestCreateEnvironmentHandler_InvalidBody(t *testing.T) {
	handler := CreateEnvironmentHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/environments",
		strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("CreateEnvironmentHandler invalid body: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestUpdateEnvironmentHandler_InvalidBody(t *testing.T) {
	handler := UpdateEnvironmentHandler(nil)
	req := httptest.NewRequest(http.MethodPut, "/api/environments/123",
		strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("UpdateEnvironmentHandler invalid body: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// ─── GetMeHandler — no claims ────────────────────────────────────────────────

func TestGetMeHandler_NoAuth(t *testing.T) {
	handler := GetMeHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("GetMeHandler no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestGetUserCompaniesHandler_NoAuth(t *testing.T) {
	handler := GetUserCompaniesHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/companies", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("GetUserCompaniesHandler no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

// ─── generateRefreshTokenString ───────────────────────────────────────────────

func TestGenerateRefreshTokenString_NonEmpty(t *testing.T) {
	tok := generateRefreshTokenString()
	if tok == "" {
		t.Error("generateRefreshTokenString: returned empty string")
	}
	// Should be 64 hex chars (32 bytes)
	if len(tok) != 64 {
		t.Errorf("generateRefreshTokenString: got len %d, want 64", len(tok))
	}
}

func TestGenerateRefreshTokenString_Unique(t *testing.T) {
	t1 := generateRefreshTokenString()
	t2 := generateRefreshTokenString()
	if t1 == t2 {
		t.Error("generateRefreshTokenString: two calls returned same value (should be unique)")
	}
}

// ─── GetUserCompanyID (deprecated wrapper, pure delegation) ──────────────────

// GetUserCompanyID calls GetEffectiveCompanyID(db, userID, "")
// With nil DB it will panic if actually reached — we only test the concept
// indirectly via other covered paths.

// ─── GetUserIDFromContext — with valid claims ─────────────────────────────────

func TestGetUserIDFromContext_WithClaims(t *testing.T) {
	// Create a request with a valid JWT in context
	tokenStr := makeTestJWT(t, "user")
	handler := AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := GetUserIDFromContext(r)
		if userID != "test-user-id" {
			t.Errorf("GetUserIDFromContext: got %q, want 'test-user-id'", userID)
		}
		w.WriteHeader(http.StatusOK)
	}), "")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rr := httptest.NewRecorder()
	handler(rr, req)
}

// ─── clearRefreshCookie indirectly via LogoutHandler ─────────────────────────
// Already covered by TestLogoutHandler_ValidPost in handlers_guards4_test.go
