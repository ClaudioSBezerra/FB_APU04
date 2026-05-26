package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// ── IcmsFronteiraSegmentosHandler ────────────────────────────────────────────

func TestIcmsFronteiraSegmentosHandler_Creation(t *testing.T) {
	if IcmsFronteiraSegmentosHandler(nil) == nil {
		t.Error("expected non-nil handler")
	}
}

func TestIcmsFronteiraSegmentosHandler_MethodNotAllowed(t *testing.T) {
	h := IcmsFronteiraSegmentosHandler(nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/icms-fronteira/segmentos", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("want 405, got %d", rr.Code)
	}
}

func TestIcmsFronteiraSegmentosHandler_GetUnauthorized(t *testing.T) {
	h := IcmsFronteiraSegmentosHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/segmentos?uf=PE", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rr.Code)
	}
}

func TestIcmsFronteiraSegmentosHandler_PostUnauthorized(t *testing.T) {
	h := IcmsFronteiraSegmentosHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/icms-fronteira/segmentos", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rr.Code)
	}
}

// ── IcmsFronteiraCompanySegmentosHandler ─────────────────────────────────────

func TestIcmsFronteiraCompanySegmentosHandler_Creation(t *testing.T) {
	if IcmsFronteiraCompanySegmentosHandler(nil) == nil {
		t.Error("expected non-nil handler")
	}
}

func TestIcmsFronteiraCompanySegmentosHandler_MethodNotAllowed(t *testing.T) {
	h := IcmsFronteiraCompanySegmentosHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/company-segmentos", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("want 405, got %d", rr.Code)
	}
}

func TestIcmsFronteiraCompanySegmentosHandler_Unauthorized(t *testing.T) {
	h := IcmsFronteiraCompanySegmentosHandler(nil)
	req := httptest.NewRequest(http.MethodPut, "/api/icms-fronteira/company-segmentos", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rr.Code)
	}
}

// ── IcmsFronteiraSegmentoItemHandler ─────────────────────────────────────────

func TestIcmsFronteiraSegmentoItemHandler_Creation(t *testing.T) {
	if IcmsFronteiraSegmentoItemHandler(nil) == nil {
		t.Error("expected non-nil handler")
	}
}

func TestIcmsFronteiraSegmentoItemHandler_MethodNotAllowed(t *testing.T) {
	h := IcmsFronteiraSegmentoItemHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/icms-fronteira/segmentos/item", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("want 405, got %d", rr.Code)
	}
}

func TestIcmsFronteiraSegmentoItemHandler_PutUnauthorized(t *testing.T) {
	h := IcmsFronteiraSegmentoItemHandler(nil)
	req := httptest.NewRequest(http.MethodPut, "/api/icms-fronteira/segmentos/item", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rr.Code)
	}
}

func TestIcmsFronteiraSegmentoItemHandler_DeleteUnauthorized(t *testing.T) {
	h := IcmsFronteiraSegmentoItemHandler(nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/icms-fronteira/segmentos/item?codigo=1&uf=PE", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rr.Code)
	}
}
