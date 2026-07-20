package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// ─── detectDeclaredLineCount (função pura, sem DB) ──────────────────────────

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sped.txt")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return path
}

func TestDetectDeclaredLineCount_ValidTrailer(t *testing.T) {
	path := writeTempFile(t, "|0000|017|...|\n|C100|...|\n|9999|13|\n")
	got := detectDeclaredLineCount(path, 0)
	if got != "13" {
		t.Errorf("got %q, want %q", got, "13")
	}
}

func TestDetectDeclaredLineCount_MissingTrailer(t *testing.T) {
	path := writeTempFile(t, "|0000|017|...|\n|C100|...|\n")
	got := detectDeclaredLineCount(path, 0)
	if got != "not_found" {
		t.Errorf("got %q, want %q", got, "not_found")
	}
}

func TestDetectDeclaredLineCount_FileDoesNotExist(t *testing.T) {
	got := detectDeclaredLineCount(filepath.Join(t.TempDir(), "missing.txt"), 0)
	if got != "not_found" {
		t.Errorf("got %q, want %q", got, "not_found")
	}
}

func TestDetectDeclaredLineCount_BelowMinCountIgnored(t *testing.T) {
	// minCount=100 (padrão do SPED Fiscal): contagem pequena não deve ser
	// aceita como trailer válido, para evitar falso positivo em arquivo pequeno.
	path := writeTempFile(t, "|9999|13|\n")
	got := detectDeclaredLineCount(path, 100)
	if got != "not_found" {
		t.Errorf("got %q, want %q (count below minCount should be rejected)", got, "not_found")
	}
}

func TestDetectDeclaredLineCount_MalformedTrailerIgnored(t *testing.T) {
	path := writeTempFile(t, "|9999|abc|\n")
	got := detectDeclaredLineCount(path, 0)
	if got != "not_found" {
		t.Errorf("got %q, want %q (non-numeric count should be rejected)", got, "not_found")
	}
}

func TestDetectDeclaredLineCount_IgnoresFalsePositiveInDescription(t *testing.T) {
	// Só conta se a linha COMEÇAR com "|9999|" — uma menção no meio de outro
	// campo não deve ser confundida com o trailer real.
	path := writeTempFile(t, "|C100|obs|Referencia 9999|999|\n|9999|42|\n")
	got := detectDeclaredLineCount(path, 0)
	if got != "42" {
		t.Errorf("got %q, want %q", got, "42")
	}
}

// ─── UploadEFDContribuicoesHandler — guards (nil db seguro) ─────────────────

func TestUploadEFDContribuicoesHandler_OPTIONS(t *testing.T) {
	handler := UploadEFDContribuicoesHandler(nil)
	req := httptest.NewRequest(http.MethodOptions, "/api/efd-contribuicoes/upload", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("OPTIONS: got %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestUploadEFDContribuicoesHandler_MethodGuard(t *testing.T) {
	cases := []string{http.MethodGet, http.MethodPut, http.MethodDelete}
	handler := UploadEFDContribuicoesHandler(nil)
	for _, method := range cases {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/efd-contribuicoes/upload", nil)
			rr := httptest.NewRecorder()
			handler(rr, req)
			if rr.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s: got %d, want %d", method, rr.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}

func TestUploadEFDContribuicoesHandler_Unauthorized(t *testing.T) {
	// nil db é seguro: sem claims no contexto, o handler retorna 401 antes de
	// chamar GetEffectiveCompanyID (que tocaria o DB).
	handler := UploadEFDContribuicoesHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/efd-contribuicoes/upload", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}
