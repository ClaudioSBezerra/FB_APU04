package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

// Guards de método/autenticação — nil db é seguro porque os 3 handlers
// retornam antes de tocar o banco (mesmo padrão já usado em todo o pacote).

func TestCNPJPublicoEnriquecerHandler_MethodGuard(t *testing.T) {
	handler := CNPJPublicoEnriquecerHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/fornecedores-clientes/enriquecer", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestCNPJPublicoEnriquecerHandler_NoAuth(t *testing.T) {
	handler := CNPJPublicoEnriquecerHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/fornecedores-clientes/enriquecer", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestCNPJPublicoJobStatusHandler_MethodGuard(t *testing.T) {
	handler := CNPJPublicoJobStatusHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/fornecedores-clientes/jobs/abc", nil)
	req = req.WithContext(context.WithValue(req.Context(), ClaimsKey, jwt.MapClaims{}))
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestCNPJPublicoJobStatusHandler_NoAuth(t *testing.T) {
	handler := CNPJPublicoJobStatusHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/fornecedores-clientes/jobs/abc", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestCNPJPublicoJobStatusHandler_Cancelar_MethodGuard(t *testing.T) {
	handler := CNPJPublicoJobStatusHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/fornecedores-clientes/jobs/abc/cancelar", nil)
	req = req.WithContext(context.WithValue(req.Context(), ClaimsKey, jwt.MapClaims{}))
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestCNPJPublicoJobStatusHandler_Cancelar_NoAuth(t *testing.T) {
	handler := CNPJPublicoJobStatusHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/fornecedores-clientes/jobs/abc/cancelar", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestCNPJPublicoImportarExcelHandler_MethodGuard(t *testing.T) {
	handler := CNPJPublicoImportarExcelHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/fornecedores-clientes/importar-excel", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestCNPJPublicoImportarExcelHandler_NoAuth(t *testing.T) {
	handler := CNPJPublicoImportarExcelHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/fornecedores-clientes/importar-excel", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestFindColuna(t *testing.T) {
	header := []string{"", "CGC_EMITENTE", "ANO", "VLR_TOTAL"}
	if got := findColuna(header, "cnpj", "cgc"); got != 1 {
		t.Errorf("cnpj: got %d, want 1", got)
	}
	if got := findColuna(header, "ano"); got != 2 {
		t.Errorf("ano: got %d, want 2", got)
	}
	if got := findColuna(header, "valor", "vlr"); got != 3 {
		t.Errorf("valor: got %d, want 3", got)
	}
	if got := findColuna(header, "inexistente"); got != -1 {
		t.Errorf("inexistente: got %d, want -1", got)
	}
}

func TestParseValorPlanilha(t *testing.T) {
	cases := []struct {
		in     string
		want   float64
		wantOk bool
	}{
		{"352808.09", 352808.09, true},
		{"17497674.170000002", 17497674.170000002, true},
		{"352.808,09", 352808.09, true},
		{"1755,6", 1755.6, true},
		{"R$ 100.50", 100.50, true},
		{"", 0, false},
		{"abc", 0, false},
	}
	for _, c := range cases {
		got, ok := parseValorPlanilha(c.in)
		if ok != c.wantOk {
			t.Errorf("parseValorPlanilha(%q) ok = %v, want %v", c.in, ok, c.wantOk)
			continue
		}
		if ok && got != c.want {
			t.Errorf("parseValorPlanilha(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestCNPJPublicoRelatorioHandler_MethodGuard(t *testing.T) {
	handler := CNPJPublicoRelatorioHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/fornecedores-clientes/relatorio", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestCNPJPublicoRelatorioHandler_NoAuth(t *testing.T) {
	handler := CNPJPublicoRelatorioHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/fornecedores-clientes/relatorio", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}
