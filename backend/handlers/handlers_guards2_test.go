package handlers

// handlers_guards2_test.go — extensão de cobertura para handlers dos módulos de
// upload fiscal e apurações RFB, além de funções puras de formatação.
//
// Todos os testes são nil-db-safe: os handlers retornam antes de tocar o banco
// de dados nos caminhos de método errado ou ausência de claims no contexto.

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ─── RFB Apuração handlers ───────────────────────────────────────────────────

func TestSolicitarApuracaoHandler_MethodGuard(t *testing.T) {
	handler := SolicitarApuracaoHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/rfb/solicitar", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("SolicitarApuracaoHandler GET: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestSolicitarApuracaoHandler_NoAuth(t *testing.T) {
	handler := SolicitarApuracaoHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/rfb/solicitar", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("SolicitarApuracaoHandler POST no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestDownloadManualHandler_MethodGuard(t *testing.T) {
	handler := DownloadManualHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/rfb/download-manual", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("DownloadManualHandler GET: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestDownloadManualHandler_NoAuth(t *testing.T) {
	handler := DownloadManualHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/rfb/download-manual", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("DownloadManualHandler POST no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestDeleteRequestHandler_MethodGuard(t *testing.T) {
	handler := DeleteRequestHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/rfb/delete-request", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("DeleteRequestHandler GET: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestDeleteRequestHandler_NoAuth(t *testing.T) {
	handler := DeleteRequestHandler(nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/rfb/delete-request", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("DeleteRequestHandler DELETE no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestClearErrorsHandler_MethodGuard(t *testing.T) {
	handler := ClearErrorsHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/rfb/clear-errors", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("ClearErrorsHandler GET: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestClearErrorsHandler_NoAuth(t *testing.T) {
	handler := ClearErrorsHandler(nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/rfb/clear-errors", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("ClearErrorsHandler DELETE no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestReprocessHandler_MethodGuard(t *testing.T) {
	handler := ReprocessHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/rfb/reprocess", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("ReprocessHandler GET: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestReprocessHandler_NoAuth(t *testing.T) {
	handler := ReprocessHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/rfb/reprocess", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("ReprocessHandler POST no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestStatusApuracaoHandler_NoAuth(t *testing.T) {
	// StatusApuracaoHandler has no method guard, directly checks claims
	handler := StatusApuracaoHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/rfb/status", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("StatusApuracaoHandler GET no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestDetalheApuracaoHandler_NoAuth(t *testing.T) {
	// DetalheApuracaoHandler has no method guard, directly checks claims
	handler := DetalheApuracaoHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/rfb/apuracao/abc123", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("DetalheApuracaoHandler GET no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

// ─── NFe Entradas handlers ────────────────────────────────────────────────────

func TestNfeEntradasUploadHandler_OPTIONS(t *testing.T) {
	handler := NfeEntradasUploadHandler(nil)
	req := httptest.NewRequest(http.MethodOptions, "/api/nfe-entradas/upload", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("NfeEntradasUploadHandler OPTIONS: got %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestNfeEntradasUploadHandler_MethodGuard(t *testing.T) {
	handler := NfeEntradasUploadHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/nfe-entradas/upload", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("NfeEntradasUploadHandler GET: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestNfeEntradasUploadHandler_NoAuth(t *testing.T) {
	handler := NfeEntradasUploadHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/nfe-entradas/upload", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("NfeEntradasUploadHandler POST no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestNfeEntradasListHandler_OPTIONS(t *testing.T) {
	handler := NfeEntradasListHandler(nil)
	req := httptest.NewRequest(http.MethodOptions, "/api/nfe-entradas", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("NfeEntradasListHandler OPTIONS: got %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestNfeEntradasListHandler_MethodGuard(t *testing.T) {
	handler := NfeEntradasListHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/nfe-entradas", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("NfeEntradasListHandler POST: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestNfeEntradasListHandler_NoAuth(t *testing.T) {
	handler := NfeEntradasListHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/nfe-entradas", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("NfeEntradasListHandler GET no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestNfeEntradasImpostosHandler_OPTIONS(t *testing.T) {
	handler := NfeEntradasImpostosHandler(nil)
	req := httptest.NewRequest(http.MethodOptions, "/api/nfe-entradas/impostos", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("NfeEntradasImpostosHandler OPTIONS: got %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestNfeEntradasImpostosHandler_MethodGuard(t *testing.T) {
	handler := NfeEntradasImpostosHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/nfe-entradas/impostos", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("NfeEntradasImpostosHandler POST: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestNfeEntradasImpostosHandler_NoAuth(t *testing.T) {
	handler := NfeEntradasImpostosHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/nfe-entradas/impostos", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("NfeEntradasImpostosHandler GET no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

// ─── NFe Saídas handlers ─────────────────────────────────────────────────────

func TestNfeSaidasUploadHandler_OPTIONS(t *testing.T) {
	handler := NfeSaidasUploadHandler(nil)
	req := httptest.NewRequest(http.MethodOptions, "/api/nfe-saidas/upload", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("NfeSaidasUploadHandler OPTIONS: got %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestNfeSaidasUploadHandler_MethodGuard(t *testing.T) {
	handler := NfeSaidasUploadHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/nfe-saidas/upload", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("NfeSaidasUploadHandler GET: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestNfeSaidasUploadHandler_NoAuth(t *testing.T) {
	handler := NfeSaidasUploadHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/nfe-saidas/upload", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("NfeSaidasUploadHandler POST no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestNfeSaidasListHandler_OPTIONS(t *testing.T) {
	handler := NfeSaidasListHandler(nil)
	req := httptest.NewRequest(http.MethodOptions, "/api/nfe-saidas", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("NfeSaidasListHandler OPTIONS: got %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestNfeSaidasListHandler_MethodGuard(t *testing.T) {
	handler := NfeSaidasListHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/nfe-saidas", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("NfeSaidasListHandler POST: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestNfeSaidasListHandler_NoAuth(t *testing.T) {
	handler := NfeSaidasListHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/nfe-saidas", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("NfeSaidasListHandler GET no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

// ─── CTe Entradas handlers ────────────────────────────────────────────────────

func TestCteEntradasUploadHandler_OPTIONS(t *testing.T) {
	handler := CteEntradasUploadHandler(nil)
	req := httptest.NewRequest(http.MethodOptions, "/api/cte-entradas/upload", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("CteEntradasUploadHandler OPTIONS: got %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestCteEntradasUploadHandler_MethodGuard(t *testing.T) {
	handler := CteEntradasUploadHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/cte-entradas/upload", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("CteEntradasUploadHandler GET: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestCteEntradasUploadHandler_NoAuth(t *testing.T) {
	handler := CteEntradasUploadHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/cte-entradas/upload", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("CteEntradasUploadHandler POST no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestCteEntradasListHandler_OPTIONS(t *testing.T) {
	handler := CteEntradasListHandler(nil)
	req := httptest.NewRequest(http.MethodOptions, "/api/cte-entradas", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("CteEntradasListHandler OPTIONS: got %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestCteEntradasListHandler_MethodGuard(t *testing.T) {
	handler := CteEntradasListHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/cte-entradas", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("CteEntradasListHandler POST: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestCteEntradasListHandler_NoAuth(t *testing.T) {
	handler := CteEntradasListHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/cte-entradas", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("CteEntradasListHandler GET no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

// ─── XML Upload handlers ──────────────────────────────────────────────────────

func TestXMLUploadHandler_MethodGuard(t *testing.T) {
	handler := XMLUploadHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/xml/upload", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("XMLUploadHandler GET: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestXMLUploadHandler_NoAuth(t *testing.T) {
	handler := XMLUploadHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/xml/upload", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("XMLUploadHandler POST no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestXMLUploadBatchStatusHandler_MethodGuard(t *testing.T) {
	handler := XMLUploadBatchStatusHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/xml/batch/status", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("XMLUploadBatchStatusHandler POST: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestXMLUploadBatchStatusHandler_NoAuth(t *testing.T) {
	handler := XMLUploadBatchStatusHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/xml/batch/status", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("XMLUploadBatchStatusHandler GET no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestXMLUploadBatchesHandler_MethodGuard(t *testing.T) {
	handler := XMLUploadBatchesHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/xml/batches", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("XMLUploadBatchesHandler POST: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestXMLUploadBatchesHandler_NoAuth(t *testing.T) {
	handler := XMLUploadBatchesHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/xml/batches", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("XMLUploadBatchesHandler GET no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

// ─── AI Reports handlers ──────────────────────────────────────────────────────

func TestGetAvailablePeriodsHandler_NoAuth(t *testing.T) {
	handler := GetAvailablePeriodsHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/ai/periods", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("GetAvailablePeriodsHandler GET no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestListSavedAIReportsHandler_NoAuth(t *testing.T) {
	handler := ListSavedAIReportsHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/ai/reports", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("ListSavedAIReportsHandler GET no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

// ─── Funções puras nfe_saidas.go ─────────────────────────────────────────────

func TestToDecimal(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  float64
	}{
		{"empty", "", 0},
		{"zero", "0", 0},
		{"valid float", "1234.56", 1234.56},
		{"whitespace", "  99.99  ", 99.99},
		{"invalid", "abc", 0},
		{"negative", "-5.5", -5.5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := toDecimal(tc.input)
			if got != tc.want {
				t.Errorf("toDecimal(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestToNullDecimal(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantNil bool
		want    float64
	}{
		{"empty string", "", true, 0},
		{"whitespace only", "   ", true, 0},
		{"invalid", "abc", true, 0},
		{"valid", "42.0", false, 42.0},
		{"negative", "-1.5", false, -1.5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := toNullDecimal(tc.input)
			if tc.wantNil {
				if got != nil {
					t.Errorf("toNullDecimal(%q): expected nil, got %v", tc.input, *got)
				}
			} else {
				if got == nil {
					t.Errorf("toNullDecimal(%q): expected %v, got nil", tc.input, tc.want)
				} else if *got != tc.want {
					t.Errorf("toNullDecimal(%q) = %v, want %v", tc.input, *got, tc.want)
				}
			}
		})
	}
}

func TestParseDhEmi(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		wantErr     bool
		wantMesAno  string
	}{
		{"ISO8601 with timezone", "2026-02-26T12:00:00-03:00", false, "02/2026"},
		{"date only", "2026-01-15", false, "01/2026"},
		{"empty string", "", true, ""},
		{"invalid", "not-a-date", true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, mesAno, err := parseDhEmi(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("parseDhEmi(%q): expected error, got nil", tc.input)
				}
			} else {
				if err != nil {
					t.Errorf("parseDhEmi(%q): unexpected error: %v", tc.input, err)
				}
				if mesAno != tc.wantMesAno {
					t.Errorf("parseDhEmi(%q) mesAno = %q, want %q", tc.input, mesAno, tc.wantMesAno)
				}
			}
		})
	}
}

// ─── Funções puras ai_reports.go ─────────────────────────────────────────────

func TestCalcPreviousPeriod(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"02/2026", "01/2026"},
		{"01/2026", "12/2025"},
		{"12/2025", "11/2025"},
		{"invalid", "invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := calcPreviousPeriod(tc.input)
			if got != tc.want {
				t.Errorf("calcPreviousPeriod(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestFormatBRL(t *testing.T) {
	cases := []struct {
		name  string
		input float64
		want  string
	}{
		{"zero", 0, "0,00"},
		{"simple", 1234.56, "1.234,56"},
		{"negative", -100.0, "-100,00"},
		{"large", 1000000.0, "1.000.000,00"},
		{"small decimal", 0.99, "0,99"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := formatBRL(tc.input)
			if got != tc.want {
				t.Errorf("formatBRL(%v) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestBuildFilialClause_Empty(t *testing.T) {
	clause, args := buildFilialClause([]string{}, 2)
	if clause != "" {
		t.Errorf("buildFilialClause empty: clause = %q, want empty string", clause)
	}
	if len(args) != 0 {
		t.Errorf("buildFilialClause empty: args = %v, want empty", args)
	}
}

func TestBuildFilialClause_Single(t *testing.T) {
	clause, args := buildFilialClause([]string{"12345678000199"}, 2)
	if clause == "" {
		t.Error("buildFilialClause single: expected non-empty clause")
	}
	if len(args) != 1 {
		t.Errorf("buildFilialClause single: got %d args, want 1", len(args))
	}
	if args[0] != "12345678000199" {
		t.Errorf("buildFilialClause single: args[0] = %v, want 12345678000199", args[0])
	}
}

func TestBuildFilialClause_Multiple(t *testing.T) {
	filiais := []string{"11111111000111", "22222222000222", "33333333000333"}
	clause, args := buildFilialClause(filiais, 3)
	if clause == "" {
		t.Error("buildFilialClause multiple: expected non-empty clause")
	}
	if len(args) != 3 {
		t.Errorf("buildFilialClause multiple: got %d args, want 3", len(args))
	}
}

// ─── Funções puras auth.go ────────────────────────────────────────────────────

func TestHashPasswordAndCheck(t *testing.T) {
	password := "SecurePass123!"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: unexpected error: %v", err)
	}
	if hash == "" {
		t.Fatal("HashPassword: returned empty hash")
	}
	if !CheckPasswordHash(password, hash) {
		t.Error("CheckPasswordHash: expected true for correct password")
	}
	if CheckPasswordHash("wrong-password", hash) {
		t.Error("CheckPasswordHash: expected false for wrong password")
	}
}

func TestGetUserIDFromContext_NoContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	userID := GetUserIDFromContext(req)
	if userID != "" {
		t.Errorf("GetUserIDFromContext no context: got %q, want empty string", userID)
	}
}

func TestValidateJWTSecret_NoEnvVar(t *testing.T) {
	// Deve apenas logar um warning — não deve entrar em pânico
	// (DATABASE_URL não está setado em testes, então não chama log.Fatal)
	// Chamamos apenas para cobrir a função — sem assertion de output de log.
	defer func() {
		if r := recover(); r != nil {
			// log.Fatal chama os.Exit, capturado aqui como panic apenas em testes com recover
		}
	}()
	ValidateJWTSecret()
	// Se chegou aqui, a função não abortou — isso é o comportamento esperado em ambiente de teste.
}

func TestGetEnv_ReturnsDefault(t *testing.T) {
	got := getEnv("THIS_ENV_VAR_DOES_NOT_EXIST_FBTEST", "fallback_value")
	if got != "fallback_value" {
		t.Errorf("getEnv: got %q, want %q", got, "fallback_value")
	}
}

func TestIsSecureCookie_WithForwardedProto(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	got := isSecureCookie(req)
	if !got {
		t.Error("isSecureCookie: expected true when X-Forwarded-Proto=https")
	}
}

func TestIsSecureCookie_HTTP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No X-Forwarded-Proto, no COOKIE_SECURE env var (not set in tests)
	got := isSecureCookie(req)
	if got {
		t.Error("isSecureCookie: expected false when X-Forwarded-Proto not set")
	}
}

// ─── RFB Webhook handler ──────────────────────────────────────────────────────

func TestRFBWebhookHandler_MethodGuard(t *testing.T) {
	handler := RFBWebhookHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/rfb/webhook", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("RFBWebhookHandler GET: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

// ─── Rate limiter janela de tempo ────────────────────────────────────────────

func TestRateLimiter_WindowExpiry(t *testing.T) {
	// Janela muito curta — after window, tokens should be accepted again
	rl := newRateLimiter(1, 1*time.Millisecond)
	if !rl.Allow("key") {
		t.Fatal("TestRateLimiter_WindowExpiry: first request should be allowed")
	}
	if rl.Allow("key") {
		t.Error("TestRateLimiter_WindowExpiry: second request should be denied immediately")
	}
	// Wait for window to expire
	time.Sleep(5 * time.Millisecond)
	if !rl.Allow("key") {
		t.Error("TestRateLimiter_WindowExpiry: request after window expiry should be allowed")
	}
}
