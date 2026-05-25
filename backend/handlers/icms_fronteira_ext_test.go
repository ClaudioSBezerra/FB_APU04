package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// ── Handler creation guards ──────────────────────────────────────────────────

func TestIcmsFronteiraRegrasListHandler_Creation(t *testing.T) {
	if IcmsFronteiraRegrasListHandler(nil) == nil { t.Error("expected non-nil handler") }
}
func TestIcmsFronteiraRegraCreateHandler_Creation(t *testing.T) {
	if IcmsFronteiraRegraCreateHandler(nil) == nil { t.Error("expected non-nil handler") }
}
func TestIcmsFronteiraRegraDeleteHandler_Creation(t *testing.T) {
	if IcmsFronteiraRegraDeleteHandler(nil) == nil { t.Error("expected non-nil handler") }
}
func TestIcmsFronteiraRegrasImportarHandler_Creation(t *testing.T) {
	if IcmsFronteiraRegrasImportarHandler(nil) == nil { t.Error("expected non-nil handler") }
}
func TestIcmsFronteiraExportCSVHandler_Creation(t *testing.T) {
	if IcmsFronteiraExportCSVHandler(nil) == nil { t.Error("expected non-nil handler") }
}
func TestIcmsFronteiraExportXLSXHandler_Creation(t *testing.T) {
	if IcmsFronteiraExportXLSXHandler(nil) == nil { t.Error("expected non-nil handler") }
}
func TestIcmsFronteiraExportHTMLHandler_Creation(t *testing.T) {
	if IcmsFronteiraExportHTMLHandler(nil) == nil { t.Error("expected non-nil handler") }
}
func TestIcmsFronteiraExtratoImportarHandler_Creation(t *testing.T) {
	if IcmsFronteiraExtratoImportarHandler(nil) == nil { t.Error("expected non-nil handler") }
}
func TestIcmsFronteiraExtratoListHandler_Creation(t *testing.T) {
	if IcmsFronteiraExtratoListHandler(nil) == nil { t.Error("expected non-nil handler") }
}
func TestIcmsFronteiraExtratoDeleteHandler_Creation(t *testing.T) {
	if IcmsFronteiraExtratoDeleteHandler(nil) == nil { t.Error("expected non-nil handler") }
}
func TestIcmsFronteiraContestacaoListHandler_Creation(t *testing.T) {
	if IcmsFronteiraContestacaoListHandler(nil) == nil { t.Error("expected non-nil handler") }
}
func TestIcmsFronteiraContestacaoCreateHandler_Creation(t *testing.T) {
	if IcmsFronteiraContestacaoCreateHandler(nil) == nil { t.Error("expected non-nil handler") }
}
func TestIcmsFronteiraContestacaoUpdateHandler_Creation(t *testing.T) {
	if IcmsFronteiraContestacaoUpdateHandler(nil) == nil { t.Error("expected non-nil handler") }
}
func TestIcmsFronteiraContestacaoDeleteHandler_Creation(t *testing.T) {
	if IcmsFronteiraContestacaoDeleteHandler(nil) == nil { t.Error("expected non-nil handler") }
}

// ── Method not allowed (405) ─────────────────────────────────────────────────

func TestIcmsFronteiraRegrasListHandler_MethodNotAllowed(t *testing.T) {
	h := IcmsFronteiraRegrasListHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/icms-fronteira/regras", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed { t.Errorf("want 405, got %d", rr.Code) }
}
func TestIcmsFronteiraRegraCreateHandler_MethodNotAllowed(t *testing.T) {
	h := IcmsFronteiraRegraCreateHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/regras", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed { t.Errorf("want 405, got %d", rr.Code) }
}
func TestIcmsFronteiraRegraDeleteHandler_MethodNotAllowed(t *testing.T) {
	h := IcmsFronteiraRegraDeleteHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/regras/some-id", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed { t.Errorf("want 405, got %d", rr.Code) }
}
func TestIcmsFronteiraRegrasImportarHandler_MethodNotAllowed(t *testing.T) {
	h := IcmsFronteiraRegrasImportarHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/regras/importar", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed { t.Errorf("want 405, got %d", rr.Code) }
}
func TestIcmsFronteiraExportCSVHandler_MethodNotAllowed(t *testing.T) {
	h := IcmsFronteiraExportCSVHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/icms-fronteira/exportar/csv", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed { t.Errorf("want 405, got %d", rr.Code) }
}
func TestIcmsFronteiraExportXLSXHandler_MethodNotAllowed(t *testing.T) {
	h := IcmsFronteiraExportXLSXHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/icms-fronteira/exportar/xlsx", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed { t.Errorf("want 405, got %d", rr.Code) }
}
func TestIcmsFronteiraExportHTMLHandler_MethodNotAllowed(t *testing.T) {
	h := IcmsFronteiraExportHTMLHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/icms-fronteira/exportar/pdf", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed { t.Errorf("want 405, got %d", rr.Code) }
}
func TestIcmsFronteiraExtratoImportarHandler_MethodNotAllowed(t *testing.T) {
	h := IcmsFronteiraExtratoImportarHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/extrato/importar", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed { t.Errorf("want 405, got %d", rr.Code) }
}
func TestIcmsFronteiraExtratoListHandler_MethodNotAllowed(t *testing.T) {
	h := IcmsFronteiraExtratoListHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/icms-fronteira/extrato", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed { t.Errorf("want 405, got %d", rr.Code) }
}
func TestIcmsFronteiraExtratoDeleteHandler_MethodNotAllowed(t *testing.T) {
	h := IcmsFronteiraExtratoDeleteHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/extrato", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed { t.Errorf("want 405, got %d", rr.Code) }
}
func TestIcmsFronteiraContestacaoListHandler_MethodNotAllowed(t *testing.T) {
	h := IcmsFronteiraContestacaoListHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/icms-fronteira/contestacoes", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed { t.Errorf("want 405, got %d", rr.Code) }
}
func TestIcmsFronteiraContestacaoCreateHandler_MethodNotAllowed(t *testing.T) {
	h := IcmsFronteiraContestacaoCreateHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/contestacoes", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed { t.Errorf("want 405, got %d", rr.Code) }
}
func TestIcmsFronteiraContestacaoUpdateHandler_MethodNotAllowed(t *testing.T) {
	h := IcmsFronteiraContestacaoUpdateHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/contestacoes/id", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed { t.Errorf("want 405, got %d", rr.Code) }
}
func TestIcmsFronteiraContestacaoDeleteHandler_MethodNotAllowed(t *testing.T) {
	h := IcmsFronteiraContestacaoDeleteHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/contestacoes/id", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed { t.Errorf("want 405, got %d", rr.Code) }
}

// ── Valid JWT → panics at nil DB (covers claims + GetEffectiveCompanyID stmt) ─

func TestIcmsFronteiraRegrasListHandler_WithAuth(t *testing.T) {
	func() {
		defer func() { recover() }()
		h := AuthMiddleware(IcmsFronteiraRegrasListHandler(nil), "")
		req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/regras", nil)
		req.Header.Set("Authorization", "Bearer "+makeTestJWT(t, "user"))
		h(httptest.NewRecorder(), req)
	}()
}

func TestIcmsFronteiraExtratoListHandler_WithAuth(t *testing.T) {
	func() {
		defer func() { recover() }()
		h := AuthMiddleware(IcmsFronteiraExtratoListHandler(nil), "")
		req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/extrato", nil)
		req.Header.Set("Authorization", "Bearer "+makeTestJWT(t, "user"))
		h(httptest.NewRecorder(), req)
	}()
}

func TestIcmsFronteiraContestacaoListHandler_WithAuth(t *testing.T) {
	func() {
		defer func() { recover() }()
		h := AuthMiddleware(IcmsFronteiraContestacaoListHandler(nil), "")
		req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/contestacoes", nil)
		req.Header.Set("Authorization", "Bearer "+makeTestJWT(t, "user"))
		h(httptest.NewRecorder(), req)
	}()
}

func TestIcmsFronteiraExportCSVHandler_WithAuth(t *testing.T) {
	func() {
		defer func() { recover() }()
		h := AuthMiddleware(IcmsFronteiraExportCSVHandler(nil), "")
		req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/exportar/csv?regime=todos", nil)
		req.Header.Set("Authorization", "Bearer "+makeTestJWT(t, "user"))
		h(httptest.NewRecorder(), req)
	}()
}

func TestIcmsFronteiraExportXLSXHandler_WithAuth(t *testing.T) {
	func() {
		defer func() { recover() }()
		h := AuthMiddleware(IcmsFronteiraExportXLSXHandler(nil), "")
		req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/exportar/xlsx?regime=st", nil)
		req.Header.Set("Authorization", "Bearer "+makeTestJWT(t, "user"))
		h(httptest.NewRecorder(), req)
	}()
}

func TestIcmsFronteiraExportHTMLHandler_WithAuth(t *testing.T) {
	func() {
		defer func() { recover() }()
		h := AuthMiddleware(IcmsFronteiraExportHTMLHandler(nil), "")
		req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/exportar/pdf?regime=difal", nil)
		req.Header.Set("Authorization", "Bearer "+makeTestJWT(t, "user"))
		h(httptest.NewRecorder(), req)
	}()
}

func TestIcmsFronteiraRegraCreateHandler_WithAuth(t *testing.T) {
	func() {
		defer func() { recover() }()
		h := AuthMiddleware(IcmsFronteiraRegraCreateHandler(nil), "")
		req := httptest.NewRequest(http.MethodPost, "/api/icms-fronteira/regras", nil)
		req.Header.Set("Authorization", "Bearer "+makeTestJWT(t, "user"))
		h(httptest.NewRecorder(), req)
	}()
}

func TestIcmsFronteiraContestacaoCreateHandler_WithAuth(t *testing.T) {
	func() {
		defer func() { recover() }()
		h := AuthMiddleware(IcmsFronteiraContestacaoCreateHandler(nil), "")
		req := httptest.NewRequest(http.MethodPost, "/api/icms-fronteira/contestacoes", nil)
		req.Header.Set("Authorization", "Bearer "+makeTestJWT(t, "user"))
		h(httptest.NewRecorder(), req)
	}()
}

func TestIcmsFronteiraExtratoDeleteHandler_WithAuth(t *testing.T) {
	func() {
		defer func() { recover() }()
		h := AuthMiddleware(IcmsFronteiraExtratoDeleteHandler(nil), "")
		req := httptest.NewRequest(http.MethodDelete, "/api/icms-fronteira/extrato?periodo=01/2026", nil)
		req.Header.Set("Authorization", "Bearer "+makeTestJWT(t, "user"))
		h(httptest.NewRecorder(), req)
	}()
}

func TestIcmsFronteiraContestacaoUpdateHandler_WithAuth(t *testing.T) {
	func() {
		defer func() { recover() }()
		h := AuthMiddleware(IcmsFronteiraContestacaoUpdateHandler(nil), "")
		req := httptest.NewRequest(http.MethodPut, "/api/icms-fronteira/contestacoes/some-id", nil)
		req.Header.Set("Authorization", "Bearer "+makeTestJWT(t, "user"))
		h(httptest.NewRecorder(), req)
	}()
}

func TestIcmsFronteiraContestacaoDeleteHandler_WithAuth(t *testing.T) {
	func() {
		defer func() { recover() }()
		h := AuthMiddleware(IcmsFronteiraContestacaoDeleteHandler(nil), "")
		req := httptest.NewRequest(http.MethodDelete, "/api/icms-fronteira/contestacoes/some-id", nil)
		req.Header.Set("Authorization", "Bearer "+makeTestJWT(t, "user"))
		h(httptest.NewRecorder(), req)
	}()
}

func TestIcmsFronteiraRegraDeleteHandler_WithAuth(t *testing.T) {
	func() {
		defer func() { recover() }()
		h := AuthMiddleware(IcmsFronteiraRegraDeleteHandler(nil), "")
		req := httptest.NewRequest(http.MethodDelete, "/api/icms-fronteira/regras/some-id", nil)
		req.Header.Set("Authorization", "Bearer "+makeTestJWT(t, "user"))
		h(httptest.NewRecorder(), req)
	}()
}

func TestIcmsFronteiraExtratoImportarHandler_WithAuth(t *testing.T) {
	func() {
		defer func() { recover() }()
		h := AuthMiddleware(IcmsFronteiraExtratoImportarHandler(nil), "")
		req := httptest.NewRequest(http.MethodPost, "/api/icms-fronteira/extrato/importar", nil)
		req.Header.Set("Authorization", "Bearer "+makeTestJWT(t, "user"))
		h(httptest.NewRecorder(), req)
	}()
}

// ── No claims in context → 401 (direct call, no middleware) ─────────────────

func TestIcmsFronteiraRegrasListHandler_NoAuth(t *testing.T) {
	h := IcmsFronteiraRegrasListHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/regras", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized { t.Errorf("want 401, got %d", rr.Code) }
}
func TestIcmsFronteiraRegraCreateHandler_NoAuth(t *testing.T) {
	h := IcmsFronteiraRegraCreateHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/icms-fronteira/regras", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized { t.Errorf("want 401, got %d", rr.Code) }
}
func TestIcmsFronteiraRegraDeleteHandler_NoAuth(t *testing.T) {
	h := IcmsFronteiraRegraDeleteHandler(nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/icms-fronteira/regras/some-id", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized { t.Errorf("want 401, got %d", rr.Code) }
}
func TestIcmsFronteiraRegrasImportarHandler_NoAuth(t *testing.T) {
	h := IcmsFronteiraRegrasImportarHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/icms-fronteira/regras/importar", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized { t.Errorf("want 401, got %d", rr.Code) }
}
func TestIcmsFronteiraExportCSVHandler_NoAuth(t *testing.T) {
	h := IcmsFronteiraExportCSVHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/exportar/csv", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized { t.Errorf("want 401, got %d", rr.Code) }
}
func TestIcmsFronteiraExportXLSXHandler_NoAuth(t *testing.T) {
	h := IcmsFronteiraExportXLSXHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/exportar/xlsx", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized { t.Errorf("want 401, got %d", rr.Code) }
}
func TestIcmsFronteiraExportHTMLHandler_NoAuth(t *testing.T) {
	h := IcmsFronteiraExportHTMLHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/exportar/pdf", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized { t.Errorf("want 401, got %d", rr.Code) }
}
func TestIcmsFronteiraExtratoImportarHandler_NoAuth(t *testing.T) {
	h := IcmsFronteiraExtratoImportarHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/icms-fronteira/extrato/importar", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized { t.Errorf("want 401, got %d", rr.Code) }
}
func TestIcmsFronteiraExtratoListHandler_NoAuth(t *testing.T) {
	h := IcmsFronteiraExtratoListHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/extrato", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized { t.Errorf("want 401, got %d", rr.Code) }
}
func TestIcmsFronteiraExtratoDeleteHandler_NoAuth(t *testing.T) {
	h := IcmsFronteiraExtratoDeleteHandler(nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/icms-fronteira/extrato", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized { t.Errorf("want 401, got %d", rr.Code) }
}
func TestIcmsFronteiraContestacaoListHandler_NoAuth(t *testing.T) {
	h := IcmsFronteiraContestacaoListHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/contestacoes", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized { t.Errorf("want 401, got %d", rr.Code) }
}
func TestIcmsFronteiraContestacaoCreateHandler_NoAuth(t *testing.T) {
	h := IcmsFronteiraContestacaoCreateHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/icms-fronteira/contestacoes", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized { t.Errorf("want 401, got %d", rr.Code) }
}
func TestIcmsFronteiraContestacaoUpdateHandler_NoAuth(t *testing.T) {
	h := IcmsFronteiraContestacaoUpdateHandler(nil)
	req := httptest.NewRequest(http.MethodPut, "/api/icms-fronteira/contestacoes/id", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized { t.Errorf("want 401, got %d", rr.Code) }
}
func TestIcmsFronteiraContestacaoDeleteHandler_NoAuth(t *testing.T) {
	h := IcmsFronteiraContestacaoDeleteHandler(nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/icms-fronteira/contestacoes/id", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized { t.Errorf("want 401, got %d", rr.Code) }
}

func TestColLetter(t *testing.T) {
	cases := map[int]string{0: "A", 1: "B", 25: "Z", 26: "AA", 27: "AB", 51: "AZ", 52: "BA"}
	for i, want := range cases {
		if got := colLetter(i); got != want {
			t.Errorf("colLetter(%d) = %q, want %q", i, got, want)
		}
	}
}
