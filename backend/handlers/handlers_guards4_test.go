package handlers

// handlers_guards4_test.go — quarta extensão de cobertura
// Cobre: funções puras (nullDate, nullStr, Atoi, extractChave),
// guards de método e auth para handlers de jobs, reports, managers,
// erp_bridge, rfb_credentials, environment, dashboard, etc.

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// ─── Funções puras erp_bridge_batch.go ────────────────────────────────────────

func TestNullDate_Empty(t *testing.T) {
	got := nullDate("")
	if got != nil {
		t.Errorf("nullDate('') = %v, want nil", got)
	}
}

func TestNullDate_Whitespace(t *testing.T) {
	got := nullDate("   ")
	if got != nil {
		t.Errorf("nullDate('   ') = %v, want nil", got)
	}
}

func TestNullDate_Valid(t *testing.T) {
	got := nullDate("2026-01-15")
	if got == nil {
		t.Error("nullDate('2026-01-15') = nil, want non-nil")
	}
	if s, ok := got.(string); !ok || s != "2026-01-15" {
		t.Errorf("nullDate('2026-01-15') = %v, want '2026-01-15'", got)
	}
}

func TestNullStr_Empty(t *testing.T) {
	got := nullStr("")
	if got != nil {
		t.Errorf("nullStr('') = %v, want nil", got)
	}
}

func TestNullStr_Valid(t *testing.T) {
	got := nullStr("CNPJ123")
	if got == nil {
		t.Error("nullStr('CNPJ123') = nil, want non-nil")
	}
	if s, ok := got.(string); !ok || s != "CNPJ123" {
		t.Errorf("nullStr('CNPJ123') = %v, want 'CNPJ123'", got)
	}
}

// ─── Função pura upload.go (Atoi) ─────────────────────────────────────────────

func TestAtio_ValidNumber(t *testing.T) {
	got := Atoi("42")
	if got != 42 {
		t.Errorf("Atio('42') = %d, want 42", got)
	}
}

func TestAtio_Zero(t *testing.T) {
	got := Atoi("0")
	if got != 0 {
		t.Errorf("Atio('0') = %d, want 0", got)
	}
}

func TestAtio_Invalid(t *testing.T) {
	got := Atoi("not-a-number")
	if got != 0 {
		t.Errorf("Atio('not-a-number') = %d, want 0", got)
	}
}

func TestAtio_Empty(t *testing.T) {
	got := Atoi("")
	if got != 0 {
		t.Errorf("Atio('') = %d, want 0", got)
	}
}

// ─── Job handlers ─────────────────────────────────────────────────────────────

func TestGetJobParticipantsHandler_MethodGuard(t *testing.T) {
	handler := GetJobParticipantsHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/123/participants", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("GetJobParticipantsHandler POST: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestGetJobParticipantsHandler_NoAuth(t *testing.T) {
	handler := GetJobParticipantsHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/123/participants", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("GetJobParticipantsHandler GET no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestListJobsHandler_MethodGuard(t *testing.T) {
	handler := ListJobsHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("ListJobsHandler POST: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestListJobsHandler_NoAuth(t *testing.T) {
	handler := ListJobsHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/jobs", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("ListJobsHandler GET no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestGetJobStatusHandler_MethodGuard(t *testing.T) {
	handler := GetJobStatusHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/status", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("GetJobStatusHandler POST: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestGetJobStatusHandler_NoAuth(t *testing.T) {
	handler := GetJobStatusHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/status", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("GetJobStatusHandler GET no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestCancelJobHandler_OPTIONS(t *testing.T) {
	handler := CancelJobHandler(nil)
	req := httptest.NewRequest(http.MethodOptions, "/api/jobs/123/cancel", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("CancelJobHandler OPTIONS: got %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestCancelJobHandler_MethodGuard(t *testing.T) {
	handler := CancelJobHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/123/cancel", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("CancelJobHandler GET: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestCancelJobHandler_NoAuth(t *testing.T) {
	handler := CancelJobHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/123/cancel", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("CancelJobHandler POST no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

// ─── Auth handlers — method guards ────────────────────────────────────────────

func TestRefreshHandler_MethodGuard(t *testing.T) {
	handler := RefreshHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/refresh", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("RefreshHandler GET: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestRefreshHandler_MissingCookie(t *testing.T) {
	// POST without refresh_token cookie → 401
	handler := RefreshHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("RefreshHandler POST no cookie: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestLogoutHandler_MethodGuard(t *testing.T) {
	handler := LogoutHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/logout", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("LogoutHandler GET: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestLogoutHandler_ValidPost(t *testing.T) {
	// POST with no auth header — should succeed (clears session gracefully)
	handler := LogoutHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("LogoutHandler POST: got %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestChangePasswordHandler_NoAuth(t *testing.T) {
	// No claims in context → 401
	handler := ChangePasswordHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/change-password", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("ChangePasswordHandler no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

// ─── Dashboard handler ────────────────────────────────────────────────────────

func TestGetDashboardProjectionHandler_NoAuth(t *testing.T) {
	handler := GetDashboardProjectionHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/projection", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("GetDashboardProjectionHandler no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

// ─── Report handlers ─────────────────────────────────────────────────────────

func TestGetMercadoriasReportHandler_NoAuth(t *testing.T) {
	handler := GetMercadoriasReportHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/reports/mercadorias", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("GetMercadoriasReportHandler no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestGetTransporteReportHandler_NoAuth(t *testing.T) {
	handler := GetTransporteReportHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/reports/transporte", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("GetTransporteReportHandler no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestGetEnergiaReportHandler_NoAuth(t *testing.T) {
	handler := GetEnergiaReportHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/reports/energia", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("GetEnergiaReportHandler no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestGetComunicacoesReportHandler_NoAuth(t *testing.T) {
	handler := GetComunicacoesReportHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/reports/comunicacoes", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("GetComunicacoesReportHandler no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

// ─── Manager handlers ─────────────────────────────────────────────────────────

func TestListManagersHandler_NoAuth(t *testing.T) {
	handler := ListManagersHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/managers", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("ListManagersHandler no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestUpdateManagerHandler_MethodGuard(t *testing.T) {
	handler := UpdateManagerHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/managers/123", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("UpdateManagerHandler GET: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestUpdateManagerHandler_NoAuth(t *testing.T) {
	handler := UpdateManagerHandler(nil)
	req := httptest.NewRequest(http.MethodPut, "/api/managers/123", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("UpdateManagerHandler PUT no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestDeleteManagerHandler_MethodGuard(t *testing.T) {
	handler := DeleteManagerHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/managers/123", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("DeleteManagerHandler GET: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestDeleteManagerHandler_NoAuth(t *testing.T) {
	handler := DeleteManagerHandler(nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/managers/123", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("DeleteManagerHandler DELETE no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

// ─── Environment handlers ─────────────────────────────────────────────────────

func TestGetEnvironmentsHandler_NoAuth(t *testing.T) {
	handler := GetEnvironmentsHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/environments", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("GetEnvironmentsHandler no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

// GetGroupsHandler and GetCompaniesHandler access DB directly without auth guards
// and thus cannot be tested with nil db — skipped here (requires DB integration test).

// ─── RFB Credentials handlers ─────────────────────────────────────────────────

func TestGetRFBCredentialHandler_NoAuth(t *testing.T) {
	handler := GetRFBCredentialHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/rfb/credentials", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("GetRFBCredentialHandler no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestSaveRFBCredentialHandler_NoAuth(t *testing.T) {
	handler := SaveRFBCredentialHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/rfb/credentials", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("SaveRFBCredentialHandler no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestDeleteRFBCredentialHandler_NoAuth(t *testing.T) {
	handler := DeleteRFBCredentialHandler(nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/rfb/credentials", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("DeleteRFBCredentialHandler no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

// ─── Upload handler ────────────────────────────────────────────────────────────

func TestUploadHandler_OPTIONS(t *testing.T) {
	handler := UploadHandler(nil)
	req := httptest.NewRequest(http.MethodOptions, "/api/upload", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("UploadHandler OPTIONS: got %d, want %d", rr.Code, http.StatusOK)
	}
}

// ─── Environment/Group/Company handlers (Unauthorized guard) ──────────────────

func TestUpdateCompanyHandler_MethodNotAllowed(t *testing.T) {
	handler := UpdateCompanyHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/config/companies?id=1", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("UpdateCompanyHandler GET: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}
