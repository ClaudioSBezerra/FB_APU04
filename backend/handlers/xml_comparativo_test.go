package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// ---------------------------------------------------------------------------
// descricaoModelo
// ---------------------------------------------------------------------------

func TestDescricaoModelo_KnownModels(t *testing.T) {
	cases := []struct {
		cod  string
		want string
	}{
		{"55", "NF-e (XML importável)"},
		{"65", "NFC-e (XML importável)"},
		{"57", "CT-e (XML importável)"},
		{"01", "NF Modelo 1 (papel)"},
		{"06", "NF Energia Elétrica"},
	}
	for _, tc := range cases {
		got := descricaoModelo(tc.cod)
		if got != tc.want {
			t.Errorf("descricaoModelo(%q) = %q, want %q", tc.cod, got, tc.want)
		}
	}
}

func TestDescricaoModelo_UnknownModel(t *testing.T) {
	got := descricaoModelo("99")
	if got != "Modelo 99" {
		t.Errorf("descricaoModelo(\"99\") = %q, want \"Modelo 99\"", got)
	}
}

// ---------------------------------------------------------------------------
// validaTipoComparativo
// ---------------------------------------------------------------------------

func TestValidaTipoComparativo_Saidas(t *testing.T) {
	ind, tab, ok := validaTipoComparativo("saidas")
	if !ok || ind != "1" || tab != "nfe_saidas" {
		t.Errorf("got (%q,%q,%v), want (\"1\",\"nfe_saidas\",true)", ind, tab, ok)
	}
}

func TestValidaTipoComparativo_Entradas(t *testing.T) {
	ind, tab, ok := validaTipoComparativo("entradas")
	if !ok || ind != "0" || tab != "nfe_entradas" {
		t.Errorf("got (%q,%q,%v), want (\"0\",\"nfe_entradas\",true)", ind, tab, ok)
	}
}

func TestValidaTipoComparativo_Invalid(t *testing.T) {
	_, _, ok := validaTipoComparativo("qualquercoisa")
	if ok {
		t.Error("expected ok=false for invalid tipo")
	}
}

func TestValidaTipoComparativo_Empty(t *testing.T) {
	_, _, ok := validaTipoComparativo("")
	if ok {
		t.Error("expected ok=false for empty tipo")
	}
}

// ---------------------------------------------------------------------------
// Handler MethodNotAllowed (guards before DB access)
// ---------------------------------------------------------------------------

func TestResumoComparativoHandler_MethodNotAllowed(t *testing.T) {
	handler := ResumoComparativoHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/xml/comparativo/resumo", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestLacunasHandler_MethodNotAllowed(t *testing.T) {
	handler := LacunasHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/xml/comparativo/lacunas", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestModelosEFDHandler_MethodNotAllowed(t *testing.T) {
	handler := ModelosEFDHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/xml/comparativo/modelos", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// Unauthorized (GET sem JWT no contexto — retorna 401 antes de tocar no DB)
// ---------------------------------------------------------------------------

func TestResumoComparativoHandler_Unauthorized(t *testing.T) {
	handler := ResumoComparativoHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/xml/comparativo/resumo?tipo=saidas", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestLacunasHandler_Unauthorized(t *testing.T) {
	handler := LacunasHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/xml/comparativo/lacunas?tipo=saidas", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestModelosEFDHandler_Unauthorized(t *testing.T) {
	handler := ModelosEFDHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/xml/comparativo/modelos", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// LacunasMensalHandler guards (method + auth — both return before DB access)
// ---------------------------------------------------------------------------

func TestLacunasMensalHandler_MethodNotAllowed(t *testing.T) {
	handler := LacunasMensalHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/xml/comparativo/lacunas-mensal", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestLacunasMensalHandler_Unauthorized(t *testing.T) {
	handler := LacunasMensalHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/xml/comparativo/lacunas-mensal?tipo=saidas", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// LacunasExportHandler guards
// ---------------------------------------------------------------------------

func TestLacunasExportHandler_MethodNotAllowed(t *testing.T) {
	handler := LacunasExportHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/xml/comparativo/lacunas/export", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestLacunasExportHandler_Unauthorized(t *testing.T) {
	handler := LacunasExportHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/xml/comparativo/lacunas/export?tipo=saidas", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}
