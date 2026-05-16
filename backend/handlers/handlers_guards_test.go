package handlers

// handlers_guards_test.go — cobertura adicional para alcançar >= 30% de statements
// em ./handlers/...
//
// Estratégia:
//   1. Funções puras (sem DB): validateReadOnlySQL, isValidUUID, parsePgArray,
//      GetAllowedOrigins, BackupDir, jsonErr, GenerateToken
//   2. Guards de método (nil db seguro — handler retorna antes de tocar DB):
//      ResetCompanyDataHandler, RefreshViewsHandler, ResetDatabaseHandler,
//      ReassignUserHandler, AIQueryHandler, ApuracaoPainelHandler,
//      CreditosPerdidosHandler, XMLSaneamentoCCLASSTRIBHandler,
//      XMLSaneamentoCSVHandler, XMLFornecedoresCCLASSTRIBHandler,
//      CreateManagerHandler, CreateFornSimplesHandler, DeleteFornSimplesHandler,
//      ImportCFOPsHandler
//   3. Guards de autenticação — método correto + sem claims no contexto → 401
//      (não chama GetEffectiveCompanyID, portanto nil db é seguro)
//   4. Caminhos adicionais do GetClientIP (X-Real-IP)
//   5. RecordFailure no rateLimiter
//   6. SecurityMiddleware: preflight OPTIONS e origem não permitida

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ─── 1. Funções puras ────────────────────────────────────────────────────────

func TestValidateReadOnlySQL_AllowsSelect(t *testing.T) {
	cases := []struct {
		name string
		sql  string
	}{
		{"simple select", "SELECT * FROM nfe_entradas"},
		{"select with where", "SELECT id, valor FROM parceiros WHERE cnpj = '12345678000199'"},
		{"select with join", "SELECT a.id, b.nome FROM nfe_entradas a JOIN parceiros b ON a.emit_cnpj = b.cnpj"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateReadOnlySQL(tc.sql); err != nil {
				t.Errorf("validateReadOnlySQL(%q): expected nil error, got %v", tc.sql, err)
			}
		})
	}
}

func TestValidateReadOnlySQL_BlocksMutations(t *testing.T) {
	cases := []struct {
		name string
		sql  string
	}{
		{"INSERT", "INSERT INTO users (email) VALUES ('x@y.com')"},
		{"UPDATE", "UPDATE users SET role = 'admin' WHERE id = '1'"},
		{"DELETE", "DELETE FROM users WHERE id = '1'"},
		{"DROP", "DROP TABLE users"},
		{"ALTER", "ALTER TABLE users ADD COLUMN foo TEXT"},
		{"CREATE", "CREATE TABLE evil (id SERIAL)"},
		{"TRUNCATE", "TRUNCATE TABLE users"},
		{"GRANT", "GRANT ALL ON users TO PUBLIC"},
		{"REVOKE", "REVOKE ALL ON users FROM PUBLIC"},
		{"case insensitive insert", "insert into users values (1)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateReadOnlySQL(tc.sql); err == nil {
				t.Errorf("validateReadOnlySQL(%q): expected error for mutation, got nil", tc.sql)
			}
		})
	}
}

func TestIsValidUUID(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid uuid v4", "550e8400-e29b-41d4-a716-446655440000", true},
		{"valid uuid uppercase", "550E8400-E29B-41D4-A716-446655440000", true},
		{"empty string", "", false},
		{"too short", "550e8400-e29b-41d4", false},
		{"no dashes", "550e8400e29b41d4a716446655440000", false},
		{"invalid chars", "gggggggg-e29b-41d4-a716-446655440000", false},
		{"random word", "hello", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isValidUUID(tc.input)
			if got != tc.want {
				t.Errorf("isValidUUID(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestParsePgArray(t *testing.T) {
	cases := []struct {
		name  string
		input interface{}
		want  []string
	}{
		{"nil", nil, []string{}},
		{"empty string", "", []string{}},
		{"empty braces", "{}", []string{}},
		{"single element", "{abc}", []string{"abc"}},
		{"two elements", "{abc,def}", []string{"abc", "def"}},
		{"byte slice", []byte("{x,y,z}"), []string{"x", "y", "z"}},
		{"unknown type", 42, []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parsePgArray(tc.input)
			if len(got) != len(tc.want) {
				t.Errorf("parsePgArray(%v) = %v (len %d), want %v (len %d)",
					tc.input, got, len(got), tc.want, len(tc.want))
				return
			}
			for i, v := range got {
				if v != tc.want[i] {
					t.Errorf("parsePgArray(%v)[%d] = %q, want %q", tc.input, i, v, tc.want[i])
				}
			}
		})
	}
}

func TestGetAllowedOrigins_ContainsLocalhost(t *testing.T) {
	origins := GetAllowedOrigins()
	if !origins["http://localhost:3000"] {
		t.Error("GetAllowedOrigins: expected http://localhost:3000 to be allowed")
	}
	if !origins["http://localhost:5173"] {
		t.Error("GetAllowedOrigins: expected http://localhost:5173 to be allowed")
	}
}

func TestBackupDir_ReturnsString(t *testing.T) {
	// BackupDir returns either "/backups" or "./backups" — both are non-empty strings
	dir := BackupDir()
	if dir == "" {
		t.Error("BackupDir: expected non-empty string")
	}
	if dir != "/backups" && dir != "./backups" {
		t.Errorf("BackupDir: unexpected value %q", dir)
	}
}

func TestJsonErr_SetsStatusAndContentType(t *testing.T) {
	rr := httptest.NewRecorder()
	jsonErr(rr, http.StatusBadRequest, "test error")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("jsonErr: got status %d, want %d", rr.Code, http.StatusBadRequest)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("jsonErr: Content-Type = %q, want application/json", ct)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "test error") {
		t.Errorf("jsonErr: body %q does not contain 'test error'", body)
	}
}

func TestGenerateToken_ReturnsNonEmptyToken(t *testing.T) {
	token, err := GenerateToken("user-123", "admin")
	if err != nil {
		t.Fatalf("GenerateToken: unexpected error: %v", err)
	}
	if token == "" {
		t.Error("GenerateToken: returned empty token")
	}
	// JWT has 3 parts separated by '.'
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Errorf("GenerateToken: expected 3 JWT parts, got %d", len(parts))
	}
}

// ─── 2 & 3. Guards de método e autenticação ──────────────────────────────────

// methodGuardCase é uma tabela de testes para handlers com guarda de método.
type methodGuardCase struct {
	name           string
	method         string
	expectedStatus int
}

func TestResetCompanyDataHandler_MethodGuard(t *testing.T) {
	cases := []methodGuardCase{
		{"GET not allowed", http.MethodGet, http.StatusMethodNotAllowed},
		{"POST not allowed", http.MethodPost, http.StatusMethodNotAllowed},
	}
	handler := ResetCompanyDataHandler(nil)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/api/admin/reset-company", nil)
			rr := httptest.NewRecorder()
			handler(rr, req)
			if rr.Code != tc.expectedStatus {
				t.Errorf("got %d, want %d", rr.Code, tc.expectedStatus)
			}
		})
	}
}

func TestRefreshViewsHandler_MethodGuard(t *testing.T) {
	// nil db: method guard returns before touching DB
	handler := RefreshViewsHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/refresh-views", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("RefreshViewsHandler GET: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestResetDatabaseHandler_MethodGuard(t *testing.T) {
	// nil db: method guard returns before touching DB
	handler := ResetDatabaseHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/reset-database", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("ResetDatabaseHandler GET: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestReassignUserHandler_MethodGuard(t *testing.T) {
	// nil db: method guard returns before touching DB
	handler := ReassignUserHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/reassign-user", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("ReassignUserHandler GET: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestReassignUserHandler_MissingFields(t *testing.T) {
	// POST with valid JSON but missing required fields → 400, before DB
	handler := ReassignUserHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/reassign-user",
		strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("ReassignUserHandler missing fields: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestAIQueryHandler_MethodGuard(t *testing.T) {
	// nil db: method guard returns before touching DB
	handler := AIQueryHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/ai/query", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("AIQueryHandler GET: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestAIQueryHandler_NoAuth(t *testing.T) {
	// POST without claims in context → 401, before DB
	handler := AIQueryHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/query",
		strings.NewReader(`{"pergunta":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("AIQueryHandler no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestApuracaoPainelHandler_MethodGuard(t *testing.T) {
	handler := ApuracaoPainelHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/apuracao/painel", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("ApuracaoPainelHandler POST: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestApuracaoPainelHandler_NoAuth(t *testing.T) {
	// GET without claims → 401, before DB
	handler := ApuracaoPainelHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/apuracao/painel", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("ApuracaoPainelHandler GET no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestCreditosPerdidosHandler_MethodGuard(t *testing.T) {
	handler := CreditosPerdidosHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/apuracao/creditos-perdidos", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("CreditosPerdidosHandler POST: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestCreditosPerdidosHandler_NoAuth(t *testing.T) {
	handler := CreditosPerdidosHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/apuracao/creditos-perdidos", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("CreditosPerdidosHandler GET no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestXMLSaneamentoCCLASSTRIBHandler_MethodGuard(t *testing.T) {
	handler := XMLSaneamentoCCLASSTRIBHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/xml/reports/saneamento", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("XMLSaneamentoCCLASSTRIBHandler POST: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestXMLSaneamentoCCLASSTRIBHandler_NoAuth(t *testing.T) {
	handler := XMLSaneamentoCCLASSTRIBHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/xml/reports/saneamento", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("XMLSaneamentoCCLASSTRIBHandler GET no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestXMLSaneamentoCSVHandler_MethodGuard(t *testing.T) {
	handler := XMLSaneamentoCSVHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/xml/reports/saneamento/csv", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("XMLSaneamentoCSVHandler POST: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestXMLSaneamentoCSVHandler_NoAuth(t *testing.T) {
	handler := XMLSaneamentoCSVHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/xml/reports/saneamento/csv", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("XMLSaneamentoCSVHandler GET no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestXMLFornecedoresCCLASSTRIBHandler_MethodGuard(t *testing.T) {
	handler := XMLFornecedoresCCLASSTRIBHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/xml/reports/fornecedores-cclasstrib", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("XMLFornecedoresCCLASSTRIBHandler POST: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestXMLFornecedoresCCLASSTRIBHandler_NoAuth(t *testing.T) {
	handler := XMLFornecedoresCCLASSTRIBHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/xml/reports/fornecedores-cclasstrib", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("XMLFornecedoresCCLASSTRIBHandler GET no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestCreateManagerHandler_MethodGuard(t *testing.T) {
	handler := CreateManagerHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/managers", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("CreateManagerHandler GET: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestCreateManagerHandler_NoAuth(t *testing.T) {
	// POST without claims → 401, before DB
	handler := CreateManagerHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/managers",
		strings.NewReader(`{"nome_completo":"Test","cargo":"CEO","email":"t@t.com"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("CreateManagerHandler no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestCreateFornSimplesHandler_MethodGuard(t *testing.T) {
	// nil db: method guard returns before touching DB
	handler := CreateFornSimplesHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/forn-simples", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("CreateFornSimplesHandler GET: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestCreateFornSimplesHandler_InvalidCNPJ(t *testing.T) {
	// POST with CNPJ < 14 digits → 400, before DB
	handler := CreateFornSimplesHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/forn-simples",
		strings.NewReader(`{"cnpj":"12345"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("CreateFornSimplesHandler invalid CNPJ: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestDeleteFornSimplesHandler_MethodGuard(t *testing.T) {
	// nil db: method guard returns before touching DB
	handler := DeleteFornSimplesHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/forn-simples", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("DeleteFornSimplesHandler GET: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestDeleteFornSimplesHandler_MissingCNPJ(t *testing.T) {
	// DELETE without ?cnpj → 400, before DB
	handler := DeleteFornSimplesHandler(nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/forn-simples", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("DeleteFornSimplesHandler missing CNPJ: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestImportCFOPsHandler_MethodGuard(t *testing.T) {
	// nil db: method guard returns before touching DB
	handler := ImportCFOPsHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/cfop/import", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("ImportCFOPsHandler GET: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestImportCFOPsHandler_OPTIONS(t *testing.T) {
	// OPTIONS is handled specially (preflight) — returns 200
	handler := ImportCFOPsHandler(nil)
	req := httptest.NewRequest(http.MethodOptions, "/api/cfop/import", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("ImportCFOPsHandler OPTIONS: got %d, want %d", rr.Code, http.StatusOK)
	}
}

// ─── 4. GetClientIP — caminho X-Real-IP ──────────────────────────────────────

func TestGetClientIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Real-IP", "9.8.7.6")
	got := GetClientIP(req)
	want := "9.8.7.6"
	if got != want {
		t.Errorf("GetClientIP X-Real-IP: got %q, want %q", got, want)
	}
}

func TestGetClientIP_XForwardedFor_Multiple(t *testing.T) {
	// Multiple IPs in X-Forwarded-For — GetClientIP uses LAST (rightmost, set by proxy)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "1.1.1.1, 2.2.2.2, 3.3.3.3")
	got := GetClientIP(req)
	want := "3.3.3.3"
	if got != want {
		t.Errorf("GetClientIP multiple X-Forwarded-For: got %q, want %q", got, want)
	}
}

// ─── 5. RecordFailure ─────────────────────────────────────────────────────────

func TestRateLimiter_RecordFailure(t *testing.T) {
	rl := newRateLimiter(2, time.Minute)
	rl.RecordFailure("ip")
	rl.RecordFailure("ip")
	// 2 failures recorded — IsLimited should return true
	if !rl.IsLimited("ip") {
		t.Error("TestRateLimiter_RecordFailure: expected IsLimited=true after 2 failures")
	}
}

func TestRateLimiter_RecordFailure_BelowLimit(t *testing.T) {
	rl := newRateLimiter(3, time.Minute)
	rl.RecordFailure("ip")
	// 1 failure — still under limit
	if rl.IsLimited("ip") {
		t.Error("TestRateLimiter_RecordFailure: expected IsLimited=false with 1 failure under limit 3")
	}
}

// ─── 6. SecurityMiddleware ────────────────────────────────────────────────────

func TestSecurityMiddleware_AllowedOrigin(t *testing.T) {
	allowed := map[string]bool{"http://localhost:3000": true}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux := SecurityMiddleware(next, allowed)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("SecurityMiddleware allowed origin: got %d, want %d", rr.Code, http.StatusOK)
	}
	if rr.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("SecurityMiddleware: expected CORS origin header, got %q",
			rr.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestSecurityMiddleware_DisallowedOrigin(t *testing.T) {
	allowed := map[string]bool{"http://localhost:3000": true}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux := SecurityMiddleware(next, allowed)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://evil.example.com")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("SecurityMiddleware disallowed origin: got %d, want %d", rr.Code, http.StatusOK)
	}
	if rr.Header().Get("Access-Control-Allow-Origin") == "http://evil.example.com" {
		t.Error("SecurityMiddleware: evil origin should NOT be in ACAO header")
	}
}

func TestSecurityMiddleware_Preflight_OPTIONS(t *testing.T) {
	allowed := map[string]bool{"http://localhost:3000": true}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux := SecurityMiddleware(next, allowed)

	req := httptest.NewRequest(http.MethodOptions, "/api/auth/login", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("SecurityMiddleware preflight: got %d, want %d", rr.Code, http.StatusNoContent)
	}
	if rr.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("SecurityMiddleware preflight: expected Access-Control-Allow-Methods header")
	}
}

func TestSecurityMiddleware_SecurityHeaders(t *testing.T) {
	allowed := map[string]bool{}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux := SecurityMiddleware(next, allowed)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	headers := map[string]string{
		"X-Frame-Options":        "DENY",
		"X-Content-Type-Options": "nosniff",
	}
	for h, want := range headers {
		got := rr.Header().Get(h)
		if got != want {
			t.Errorf("SecurityMiddleware: %s = %q, want %q", h, got, want)
		}
	}
}

// ─── 7. AuthMiddleware — token revogado ──────────────────────────────────────

func TestAuthMiddleware_RevokedToken(t *testing.T) {
	// Gera um token válido, coloca na blacklist, verifica que retorna 401
	tokenStr := makeTestJWT(t, "admin")
	tokenBlacklist.Store(tokenStr, time.Now().Add(30*time.Minute))
	defer tokenBlacklist.Delete(tokenStr)

	handler := AuthMiddleware(innerOKHandler, "admin")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("TestAuthMiddleware_RevokedToken: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
	tokenBlacklist.Delete(tokenStr)
}

// ─── 8. AuthMiddleware — requiredRole vazio permite qualquer role ─────────────

func TestAuthMiddleware_EmptyRequiredRole(t *testing.T) {
	// requiredRole="" — qualquer role autenticada passa
	tokenStr := makeTestJWT(t, "user")
	handler := AuthMiddleware(innerOKHandler, "")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("TestAuthMiddleware_EmptyRequiredRole: got %d, want %d", rr.Code, http.StatusOK)
	}
}
