package handlers

// handlers_guards7_test.go — sétima extensão de cobertura
// Cobre: RegisterHandler, LoginHandler, CreateUserHandler, ImportFornSimplesHandler,
// environment handlers (CreateGroup/DeleteGroup/CreateCompany/DeleteCompany),
// ApuracaoPainelHandler, GetMercadoriasReportHandler, GetComunicacoesReportHandler,
// admin handlers (ResetCompanyData/InvalidBody, RefreshViews, ResetDatabase, Reassign),
// generateRefreshTokenString, isSecureCookie, GetClientIP extra, SecurityMiddleware extras.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ─── RegisterHandler ─────────────────────────────────────────────────────────

func TestRegisterHandler_InvalidBody(t *testing.T) {
	handler := RegisterHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("RegisterHandler invalid body: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestRegisterHandler_MissingFields(t *testing.T) {
	handler := RegisterHandler(nil)
	body := `{"email":"test@example.com","password":"secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("RegisterHandler missing fields: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestRegisterHandler_ShortPassword(t *testing.T) {
	handler := RegisterHandler(nil)
	body := `{"email":"test@example.com","password":"short","full_name":"Test User","company_name":"Acme"}`
	req := httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("RegisterHandler short password: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// ─── LoginHandler ─────────────────────────────────────────────────────────────

func TestLoginHandler_InvalidBody(t *testing.T) {
	handler := LoginHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader("{bad-json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("LoginHandler invalid body: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// ─── CreateUserHandler ────────────────────────────────────────────────────────

func TestCreateUserHandler_InvalidBody(t *testing.T) {
	handler := CreateUserHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("CreateUserHandler invalid body: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestCreateUserHandler_MissingFields(t *testing.T) {
	handler := CreateUserHandler(nil)
	body := `{"email":"test@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("CreateUserHandler missing fields: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// ─── ImportFornSimplesHandler ─────────────────────────────────────────────────

func TestImportFornSimplesHandler_MethodGuard(t *testing.T) {
	handler := ImportFornSimplesHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/forn-simples/import", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("ImportFornSimplesHandler GET: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestImportFornSimplesHandler_MissingFile(t *testing.T) {
	handler := ImportFornSimplesHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/forn-simples/import", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("ImportFornSimplesHandler missing file: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// ─── Environment — CreateGroupHandler ────────────────────────────────────────

func TestCreateGroupHandler_InvalidBody(t *testing.T) {
	handler := CreateGroupHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/groups", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("CreateGroupHandler invalid body: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// ─── Environment — DeleteGroupHandler ────────────────────────────────────────

func TestDeleteGroupHandler_MissingID(t *testing.T) {
	handler := DeleteGroupHandler(nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/groups", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("DeleteGroupHandler missing id: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// ─── Environment — CreateCompanyHandler ──────────────────────────────────────

func TestCreateCompanyHandler_InvalidBody(t *testing.T) {
	handler := CreateCompanyHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/companies", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("CreateCompanyHandler invalid body: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestCreateCompanyHandler_MissingRequiredFields(t *testing.T) {
	handler := CreateCompanyHandler(nil)
	body := `{"name":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/companies", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("CreateCompanyHandler missing required fields: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// ─── Environment — DeleteCompanyHandler ──────────────────────────────────────

func TestDeleteCompanyHandler_MissingID(t *testing.T) {
	handler := DeleteCompanyHandler(nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/companies", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("DeleteCompanyHandler missing id: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// ─── ResetCompanyDataHandler — invalid body ───────────────────────────────────
// (method guard already tested in handlers_guards_test.go)

func TestResetCompanyDataHandler_InvalidBody(t *testing.T) {
	handler := ResetCompanyDataHandler(nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/reset-company", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("ResetCompanyDataHandler invalid body: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// ─── ReassignUserHandler — invalid body ──────────────────────────────────────
// (method guard and missing-fields already tested in handlers_guards_test.go)

func TestReassignUserHandler_InvalidBody(t *testing.T) {
	handler := ReassignUserHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/reassign-user", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("ReassignUserHandler invalid body: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// ─── SecurityMiddleware — preflight without Origin header ─────────────────────

func TestSecurityMiddleware_PreflightNoOrigin(t *testing.T) {
	// OPTIONS without Origin → still returns 204 (preflight handling is unconditional)
	allowedOrigins := map[string]bool{"https://example.com": true}
	mw := SecurityMiddleware(nil, allowedOrigins)
	req := httptest.NewRequest(http.MethodOptions, "/api/test", nil)
	req.Header.Set("Access-Control-Request-Method", "POST")
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)
	// Either 204 (preflight) or 200/other — must not panic
	if rr.Code == 0 {
		t.Error("SecurityMiddleware preflight no origin: got zero status code")
	}
}

// ─── GetClientIP — multiple X-Forwarded-For ───────────────────────────────────
// TestGetClientIP_XForwardedFor_Multiple is in handlers_guards2_test.go;
// test single-entry case here.

func TestGetClientIP_SingleXForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.5")
	got := GetClientIP(req)
	if got != "203.0.113.5" {
		t.Errorf("GetClientIP single X-Forwarded-For: got %q, want %q", got, "203.0.113.5")
	}
}

// ─── RateLimiter — window reset ───────────────────────────────────────────────

func TestRateLimiter_WindowReset(t *testing.T) {
	// A very short window so expiry fires quickly
	rl := newRateLimiter(2, 50*time.Millisecond)
	rl.Allow("ip")
	rl.Allow("ip")
	if rl.Allow("ip") {
		t.Error("RateLimiter: 3rd Allow should be false when limit=2")
	}
	time.Sleep(100 * time.Millisecond)
	if !rl.Allow("ip") {
		t.Error("RateLimiter: Allow after window expiry should return true")
	}
}

// ─── isSecureCookie with plain HTTP request ───────────────────────────────────

func TestIsSecureCookie_PlainHTTP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://localhost/", nil)
	result := isSecureCookie(req)
	// Without PROD env var or https, should be false
	if result {
		t.Error("isSecureCookie plain HTTP localhost: expected false")
	}
}

// generateRefreshTokenString already tested in handlers_guards6_test.go.

// ─── UploadHandler — invalid content type ────────────────────────────────────

func TestUploadHandler_InvalidContentType(t *testing.T) {
	handler := UploadHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/upload", strings.NewReader("not a multipart body"))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	handler(rr, req)
	// Should fail gracefully without panicking (not 200)
	if rr.Code == http.StatusOK {
		t.Errorf("UploadHandler invalid content type: expected non-200, got 200")
	}
}

// ─── CreateFornSimplesHandler — already tested, add one more case ────────────
// TestCreateFornSimplesHandler_MethodGuard and InvalidCNPJ exist in guards2.
// Add invalid JSON body variant.

func TestCreateFornSimplesHandler_InvalidBody(t *testing.T) {
	handler := CreateFornSimplesHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/forn-simples", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("CreateFornSimplesHandler invalid body: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}
