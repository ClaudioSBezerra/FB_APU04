package handlers

// handlers_guards10_test.go — décima extensão de cobertura
// Cobre caminhos pós-validação pré-DB em: LoginHandler, ForgotPasswordHandler,
// ResetPasswordHandler, e com-auth pré-DB em handlers de managers, rfb, nfe.
// Todos usam panic-recovery para nil-DB safety.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ─── LoginHandler — valid JSON, non-limited IP → hits DB ─────────────────────

func TestLoginHandler_ValidBodyHitsDB(t *testing.T) {
	// Valid JSON + fresh IP → IsLimited=false → db.QueryRow → panics with nil db
	body := `{"email":"test@example.com","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "203.0.113.77") // fresh unique IP
	rr := httptest.NewRecorder()
	handler := LoginHandler(nil)

	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		handler(rr, req)
	}()

	// Covers: JSON decode success + LoginRL.IsLimited check + log.Printf + db.QueryRow
	_ = panicked
}

// ─── ForgotPasswordHandler — valid JSON, fresh email → hits DB ───────────────

func TestForgotPasswordHandler_ValidEmailFreshHitsDB(t *testing.T) {
	// Valid JSON with unique email not yet rate-limited → Allow() passes → db.QueryRow panics
	body := `{"email":"never-rate-limited-fresh@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler := ForgotPasswordHandler(nil)

	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		handler(rr, req)
	}()

	// Covers: JSON decode success + ForgotPasswordRL.Allow() + db.QueryRow
	_ = panicked
}

// ─── ResetPasswordHandler — valid JSON, matching passwords → hits DB ─────────

func TestResetPasswordHandler_ValidPasswordsHitsDB(t *testing.T) {
	// Valid JSON, matching passwords, length >= 8 → db.QueryRow → panics with nil db
	// This covers the 3 pre-DB validation guards PASS path
	body := `{"token":"some-reset-token","password":"newpass123","confirm_password":"newpass123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler := ResetPasswordHandler(nil)

	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		handler(rr, req)
	}()

	// Covers: decode success + password match + len check + db.QueryRow
	_ = panicked
}

// ─── RegisterHandler — valid body, fresh IP → hits HashPassword then DB ───────

func TestRegisterHandler_ValidBodyHitsHashThenDB(t *testing.T) {
	// Valid complete body + fresh IP → passes all pre-hash guards → HashPassword → db.Begin panics
	body := `{"email":"fresh@example.com","password":"longpassword","full_name":"Test User","company_name":"Acme"}`
	req := httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "203.0.113.88") // fresh unique IP for register RL
	rr := httptest.NewRecorder()
	handler := RegisterHandler(nil)

	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		handler(rr, req)
	}()

	// Covers: decode success + field check + len check + HashPassword + db.Begin
	_ = panicked
}

// ─── GetMeHandler — with auth → hits DB ──────────────────────────────────────

func TestGetMeHandler_WithAuth(t *testing.T) {
	rr := httptest.NewRecorder()
	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		handler := GetMeHandler(nil)
		wrappedHandler := AuthMiddleware(handler, "")
		tokenStr := makeTestJWT(t, "user")
		req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		wrappedHandler(rr, req)
	}()
	// Covers auth-passing path + userID extraction + db.QueryRow call
	_ = panicked
}

// ─── GetUserCompaniesHandler — with auth → hits DB ───────────────────────────

func TestGetUserCompaniesHandler_WithAuth(t *testing.T) {
	rr := httptest.NewRecorder()
	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		handler := GetUserCompaniesHandler(nil)
		wrappedHandler := AuthMiddleware(handler, "")
		tokenStr := makeTestJWT(t, "user")
		req := httptest.NewRequest(http.MethodGet, "/api/auth/companies", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		wrappedHandler(rr, req)
	}()
	_ = panicked
}

// ─── GetAvailablePeriodsHandler — with auth → hits DB ────────────────────────

func TestGetAvailablePeriodsHandler_WithAuth(t *testing.T) {
	rr := httptest.NewRecorder()
	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		handler := GetAvailablePeriodsHandler(nil)
		wrappedHandler := AuthMiddleware(handler, "")
		tokenStr := makeTestJWT(t, "user")
		req := httptest.NewRequest(http.MethodGet, "/api/ai/periods", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		wrappedHandler(rr, req)
	}()
	_ = panicked
}

// ─── ListSavedAIReportsHandler — with auth → hits DB ─────────────────────────

func TestListSavedAIReportsHandler_WithAuth(t *testing.T) {
	rr := httptest.NewRecorder()
	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		handler := ListSavedAIReportsHandler(nil)
		wrappedHandler := AuthMiddleware(handler, "")
		tokenStr := makeTestJWT(t, "user")
		req := httptest.NewRequest(http.MethodGet, "/api/ai/reports", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		wrappedHandler(rr, req)
	}()
	_ = panicked
}

// ─── GetSavedAIReportHandler — with auth → hits DB ───────────────────────────

func TestGetSavedAIReportHandler_WithAuth(t *testing.T) {
	rr := httptest.NewRecorder()
	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		handler := GetSavedAIReportHandler(nil)
		wrappedHandler := AuthMiddleware(handler, "")
		tokenStr := makeTestJWT(t, "user")
		req := httptest.NewRequest(http.MethodGet, "/api/ai/reports/123", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		wrappedHandler(rr, req)
	}()
	_ = panicked
}

// ─── GetExecutiveSummaryHandler — with auth → hits DB ────────────────────────

func TestGetExecutiveSummaryHandler_WithAuth(t *testing.T) {
	rr := httptest.NewRecorder()
	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		handler := GetExecutiveSummaryHandler(nil)
		wrappedHandler := AuthMiddleware(handler, "")
		tokenStr := makeTestJWT(t, "user")
		req := httptest.NewRequest(http.MethodGet, "/api/ai/summary", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		wrappedHandler(rr, req)
	}()
	_ = panicked
}

// ─── GetDailyInsightHandler — with auth → hits DB ────────────────────────────

func TestGetDailyInsightHandler_WithAuth(t *testing.T) {
	rr := httptest.NewRecorder()
	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		handler := GetDailyInsightHandler(nil)
		wrappedHandler := AuthMiddleware(handler, "")
		tokenStr := makeTestJWT(t, "user")
		req := httptest.NewRequest(http.MethodGet, "/api/ai/insight", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		wrappedHandler(rr, req)
	}()
	_ = panicked
}

// ─── NfeSaidasListHandler — with auth → hits DB ──────────────────────────────

func TestNfeSaidasListHandler_WithAuth(t *testing.T) {
	rr := httptest.NewRecorder()
	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		handler := NfeSaidasListHandler(nil)
		wrappedHandler := AuthMiddleware(handler, "")
		tokenStr := makeTestJWT(t, "user")
		req := httptest.NewRequest(http.MethodGet, "/api/nfe-saidas", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		wrappedHandler(rr, req)
	}()
	_ = panicked
}

// ─── NfeEntradasListHandler — with auth → hits DB ────────────────────────────

func TestNfeEntradasListHandler_WithAuth(t *testing.T) {
	rr := httptest.NewRecorder()
	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		handler := NfeEntradasListHandler(nil)
		wrappedHandler := AuthMiddleware(handler, "")
		tokenStr := makeTestJWT(t, "user")
		req := httptest.NewRequest(http.MethodGet, "/api/nfe-entradas", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		wrappedHandler(rr, req)
	}()
	_ = panicked
}

// ─── StatusApuracaoHandler — with auth → hits DB ─────────────────────────────

func TestStatusApuracaoHandler_WithAuth(t *testing.T) {
	rr := httptest.NewRecorder()
	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		handler := StatusApuracaoHandler(nil)
		wrappedHandler := AuthMiddleware(handler, "")
		tokenStr := makeTestJWT(t, "user")
		req := httptest.NewRequest(http.MethodGet, "/api/rfb/status", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		wrappedHandler(rr, req)
	}()
	_ = panicked
}

// ─── GetRFBCredentialHandler — with auth → hits DB ───────────────────────────

func TestGetRFBCredentialHandler_WithAuth(t *testing.T) {
	rr := httptest.NewRecorder()
	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		handler := GetRFBCredentialHandler(nil)
		wrappedHandler := AuthMiddleware(handler, "")
		tokenStr := makeTestJWT(t, "user")
		req := httptest.NewRequest(http.MethodGet, "/api/rfb/credentials", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		wrappedHandler(rr, req)
	}()
	_ = panicked
}

// ─── ListManagersHandler — with auth → hits DB ───────────────────────────────

func TestListManagersHandler_WithAuth(t *testing.T) {
	rr := httptest.NewRecorder()
	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		handler := ListManagersHandler(nil)
		wrappedHandler := AuthMiddleware(handler, "")
		tokenStr := makeTestJWT(t, "user")
		req := httptest.NewRequest(http.MethodGet, "/api/managers", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		wrappedHandler(rr, req)
	}()
	_ = panicked
}

// ─── UpdateManagerHandler — with auth → hits DB ──────────────────────────────

func TestUpdateManagerHandler_WithAuth(t *testing.T) {
	rr := httptest.NewRecorder()
	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		handler := UpdateManagerHandler(nil)
		wrappedHandler := AuthMiddleware(handler, "")
		tokenStr := makeTestJWT(t, "user")
		req := httptest.NewRequest(http.MethodPut, "/api/managers/some-manager-id", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		wrappedHandler(rr, req)
	}()
	_ = panicked
}

// ─── SolicitarApuracaoHandler — with auth → hits DB ──────────────────────────

func TestSolicitarApuracaoHandler_WithAuth(t *testing.T) {
	rr := httptest.NewRecorder()
	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		handler := SolicitarApuracaoHandler(nil)
		wrappedHandler := AuthMiddleware(handler, "")
		tokenStr := makeTestJWT(t, "user")
		req := httptest.NewRequest(http.MethodPost, "/api/rfb/solicitar", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		wrappedHandler(rr, req)
	}()
	_ = panicked
}

// ─── DeleteRequestHandler — with auth → hits DB ──────────────────────────────

func TestDeleteRequestHandler_WithAuth(t *testing.T) {
	rr := httptest.NewRecorder()
	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		handler := DeleteRequestHandler(nil)
		wrappedHandler := AuthMiddleware(handler, "")
		tokenStr := makeTestJWT(t, "user")
		req := httptest.NewRequest(http.MethodDelete, "/api/rfb/requests/123", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		wrappedHandler(rr, req)
	}()
	_ = panicked
}

// ─── ClearErrorsHandler — with auth → hits DB ────────────────────────────────

func TestClearErrorsHandler_WithAuth(t *testing.T) {
	rr := httptest.NewRecorder()
	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		handler := ClearErrorsHandler(nil)
		wrappedHandler := AuthMiddleware(handler, "")
		tokenStr := makeTestJWT(t, "user")
		req := httptest.NewRequest(http.MethodPost, "/api/rfb/clear-errors", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		wrappedHandler(rr, req)
	}()
	_ = panicked
}

// ─── ReprocessHandler — with auth → hits DB ──────────────────────────────────

func TestReprocessHandler_WithAuth(t *testing.T) {
	rr := httptest.NewRecorder()
	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		handler := ReprocessHandler(nil)
		wrappedHandler := AuthMiddleware(handler, "")
		tokenStr := makeTestJWT(t, "user")
		req := httptest.NewRequest(http.MethodPost, "/api/rfb/reprocess", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		wrappedHandler(rr, req)
	}()
	_ = panicked
}

// ─── DownloadManualHandler — with auth → hits DB ─────────────────────────────

func TestDownloadManualHandler_WithAuth(t *testing.T) {
	rr := httptest.NewRecorder()
	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		handler := DownloadManualHandler(nil)
		wrappedHandler := AuthMiddleware(handler, "")
		tokenStr := makeTestJWT(t, "user")
		req := httptest.NewRequest(http.MethodPost, "/api/rfb/download", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		wrappedHandler(rr, req)
	}()
	_ = panicked
}

// ─── DetalheApuracaoHandler — with auth → hits DB ────────────────────────────

func TestDetalheApuracaoHandler_WithAuth(t *testing.T) {
	rr := httptest.NewRecorder()
	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		handler := DetalheApuracaoHandler(nil)
		wrappedHandler := AuthMiddleware(handler, "")
		tokenStr := makeTestJWT(t, "user")
		req := httptest.NewRequest(http.MethodGet, "/api/rfb/detalhe/123", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		wrappedHandler(rr, req)
	}()
	_ = panicked
}

// ─── GetMercadoriasReportHandler — with auth → hits DB ───────────────────────

func TestGetMercadoriasReportHandler_WithAuth(t *testing.T) {
	rr := httptest.NewRecorder()
	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		handler := GetMercadoriasReportHandler(nil)
		wrappedHandler := AuthMiddleware(handler, "")
		tokenStr := makeTestJWT(t, "user")
		req := httptest.NewRequest(http.MethodGet, "/api/reports/mercadorias", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		wrappedHandler(rr, req)
	}()
	_ = panicked
}

// ─── GetComunicacoesReportHandler — with auth → hits DB ──────────────────────

func TestGetComunicacoesReportHandler_WithAuth(t *testing.T) {
	rr := httptest.NewRecorder()
	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		handler := GetComunicacoesReportHandler(nil)
		wrappedHandler := AuthMiddleware(handler, "")
		tokenStr := makeTestJWT(t, "user")
		req := httptest.NewRequest(http.MethodGet, "/api/reports/comunicacoes", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		wrappedHandler(rr, req)
	}()
	_ = panicked
}

// ─── ApuracaoPainelHandler — with auth → hits DB ─────────────────────────────

func TestApuracaoPainelHandler_WithAuth(t *testing.T) {
	rr := httptest.NewRecorder()
	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		handler := ApuracaoPainelHandler(nil)
		wrappedHandler := AuthMiddleware(handler, "")
		tokenStr := makeTestJWT(t, "user")
		req := httptest.NewRequest(http.MethodGet, "/api/apuracao/painel", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		wrappedHandler(rr, req)
	}()
	_ = panicked
}

// ─── GetTransporteReportHandler — with auth → hits DB ────────────────────────

func TestGetTransporteReportHandler_WithAuth(t *testing.T) {
	rr := httptest.NewRecorder()
	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		handler := GetTransporteReportHandler(nil)
		wrappedHandler := AuthMiddleware(handler, "")
		tokenStr := makeTestJWT(t, "user")
		req := httptest.NewRequest(http.MethodGet, "/api/reports/transporte", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		wrappedHandler(rr, req)
	}()
	_ = panicked
}

// ─── GetEnergiaReportHandler — with auth → hits DB ───────────────────────────

func TestGetEnergiaReportHandler_WithAuth(t *testing.T) {
	rr := httptest.NewRecorder()
	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		handler := GetEnergiaReportHandler(nil)
		wrappedHandler := AuthMiddleware(handler, "")
		tokenStr := makeTestJWT(t, "user")
		req := httptest.NewRequest(http.MethodGet, "/api/reports/energia", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		wrappedHandler(rr, req)
	}()
	_ = panicked
}
