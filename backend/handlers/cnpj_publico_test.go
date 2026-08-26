package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
