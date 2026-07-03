package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Guards (método + auth) de FiscalExecutionRunHandler. Não tocam o banco: o
// check de método (405) e o de claims (401) acontecem antes de qualquer uso
// do *sql.DB, então passar nil é seguro (mesma convenção de
// icms_fronteira_st_itens_guards_test.go).
func TestFiscalExecution_Guards(t *testing.T) {
	h := FiscalExecutionRunHandler(nil)

	t.Run("wrong method returns 405", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/fiscal/execute", nil)
		rr := httptest.NewRecorder()
		h(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
		}
	})

	t.Run("no auth returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/fiscal/execute", nil)
		rr := httptest.NewRecorder()
		h(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("got %d, want %d", rr.Code, http.StatusUnauthorized)
		}
	})
}
