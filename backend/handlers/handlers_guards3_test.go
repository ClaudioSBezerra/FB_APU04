package handlers

// handlers_guards3_test.go — terceira extensão de cobertura
// Cobre: funções puras de ai_reports.go, guards de admin handlers (UUID inválido),
// e handlers de AI restantes.

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// ─── Admin handlers — guard de parâmetro UUID ─────────────────────────────────

func TestPromoteUserHandler_MissingID(t *testing.T) {
	// Sem parâmetro id → 400, antes de acessar DB
	handler := PromoteUserHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/promote", nil)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("PromoteUserHandler missing id: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestPromoteUserHandler_InvalidUUID(t *testing.T) {
	// ID presente mas não é UUID válido → 400, antes de acessar DB
	handler := PromoteUserHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/promote?id=not-a-uuid", nil)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("PromoteUserHandler invalid UUID: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestDeleteUserHandler_MissingID(t *testing.T) {
	// Sem parâmetro id → 400, antes de acessar DB
	handler := DeleteUserHandler(nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/users", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("DeleteUserHandler missing id: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestDeleteUserHandler_InvalidUUID(t *testing.T) {
	// ID presente mas inválido → 400, antes de acessar DB
	handler := DeleteUserHandler(nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/users?id=bad-uuid", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("DeleteUserHandler invalid UUID: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// ─── AI Reports handlers — NoAuth ────────────────────────────────────────────

func TestGetExecutiveSummaryHandler_NoAuth(t *testing.T) {
	handler := GetExecutiveSummaryHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/ai/summary", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("GetExecutiveSummaryHandler no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestGetDailyInsightHandler_NoAuth(t *testing.T) {
	handler := GetDailyInsightHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/ai/insight", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("GetDailyInsightHandler no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestGetSavedAIReportHandler_NoAuth(t *testing.T) {
	handler := GetSavedAIReportHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/ai/reports/123", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("GetSavedAIReportHandler no auth: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

// ─── Funções puras ai_reports.go ─────────────────────────────────────────────

func TestBuildDadosBrutosJSON_NonEmpty(t *testing.T) {
	r := &ApuracaoResumo{
		CompanyName:      "Test Corp",
		CNPJ:             "12345678000199",
		Periodo:          "01/2026",
		FaturamentoBruto: 100000.0,
		IbsProjetado:     5000.0,
		CbsProjetado:     3000.0,
	}
	result := buildDadosBrutosJSON(r)
	if result == "" {
		t.Error("buildDadosBrutosJSON: expected non-empty JSON")
	}
	if len(result) < 10 {
		t.Errorf("buildDadosBrutosJSON: JSON too short: %q", result)
	}
}

func TestBuildFallbackNarrative_ReturnsString(t *testing.T) {
	r := &ApuracaoResumo{
		CompanyName:      "Test Corp",
		CNPJ:             "12345678000199",
		Periodo:          "01/2026",
		FaturamentoBruto: 50000.0,
		IcmsSaida:        4500.0,
		IcmsEntrada:      1000.0,
		IcmsAPagar:       3500.0,
		IbsProjetado:     2000.0,
		CbsProjetado:     1500.0,
	}
	result := buildFallbackNarrative(r)
	if result == "" {
		t.Error("buildFallbackNarrative: expected non-empty narrative")
	}
	if !containsStr(result, "01/2026") {
		t.Errorf("buildFallbackNarrative: expected periodo in result, got %q", result[:100])
	}
}

func TestBuildFallbackNarrative_WithPreviousPeriod(t *testing.T) {
	r := &ApuracaoResumo{
		Periodo:             "02/2026",
		PeriodoAnterior:     "01/2026",
		FaturamentoBruto:    60000.0,
		FaturamentoAnterior: 50000.0, // > 0 to trigger comparativo
	}
	result := buildFallbackNarrative(r)
	if !containsStr(result, "aumento") && !containsStr(result, "reducao") {
		t.Errorf("buildFallbackNarrative with previous period: expected comparison text, got %q", result[:200])
	}
}

func TestBuildFallbackInsight_WithLargeVariation(t *testing.T) {
	r := &ApuracaoResumo{
		Periodo:             "02/2026",
		PeriodoAnterior:     "01/2026",
		FaturamentoBruto:    60000.0,
		FaturamentoAnterior: 40000.0, // 50% increase → abs > 10
	}
	insight := buildFallbackInsight(r)
	if insight.Texto == "" {
		t.Error("buildFallbackInsight large variation: expected non-empty texto")
	}
	if insight.Tipo == "" {
		t.Error("buildFallbackInsight large variation: expected non-empty tipo")
	}
}

func TestBuildFallbackInsight_WithDecline(t *testing.T) {
	r := &ApuracaoResumo{
		Periodo:             "02/2026",
		PeriodoAnterior:     "01/2026",
		FaturamentoBruto:    30000.0,
		FaturamentoAnterior: 50000.0, // 40% decline
	}
	insight := buildFallbackInsight(r)
	if insight.Tipo != "alerta" {
		t.Errorf("buildFallbackInsight decline: expected tipo='alerta', got %q", insight.Tipo)
	}
}

func TestBuildFallbackInsight_WithoutPreviousPeriod(t *testing.T) {
	// FaturamentoAnterior=0 → should use fallback path
	r := &ApuracaoResumo{
		Periodo:          "01/2026",
		FaturamentoBruto: 50000.0,
		IbsProjetado:     3000.0,
		CbsProjetado:     2000.0,
	}
	insight := buildFallbackInsight(r)
	if insight.Texto == "" {
		t.Error("buildFallbackInsight no previous period: expected non-empty texto")
	}
}

func TestBuildExecutiveSummaryPrompt_ReturnsString(t *testing.T) {
	r := &ApuracaoResumo{
		CompanyName:      "Empresa Teste",
		CNPJ:             "12345678000199",
		Periodo:          "03/2026",
		FaturamentoBruto: 200000.0,
		TotalEntradas:    100000.0,
		IcmsSaida:        18000.0,
		IcmsEntrada:      5000.0,
		IcmsAPagar:       13000.0,
		IbsProjetado:     8000.0,
		CbsProjetado:     6000.0,
	}
	result := buildExecutiveSummaryPrompt(r)
	if result == "" {
		t.Error("buildExecutiveSummaryPrompt: expected non-empty prompt")
	}
	if !containsStr(result, "Empresa Teste") {
		t.Errorf("buildExecutiveSummaryPrompt: expected company name in prompt")
	}
}

// containsStr is a helper to check substring presence.
func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		findSubstring(s, sub))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
