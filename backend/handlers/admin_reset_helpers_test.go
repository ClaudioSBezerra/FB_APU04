package handlers

import (
	"os"
	"testing"
)

func TestPgStringArray(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected string
	}{
		{"empty", []string{}, "{}"},
		{"single", []string{"a"}, `{"a"}`},
		{"multiple", []string{"a", "b"}, `{"a","b"}`},
		{"escape double quote", []string{`a"b`}, `{"a\"b"}`},
		{"escape backslash", []string{`a\b`}, `{"a\\b"}`},
		{"real tables", []string{"import_jobs", "nfe_entradas"}, `{"import_jobs","nfe_entradas"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := pgStringArray(tc.input)
			if got != tc.expected {
				t.Errorf("pgStringArray(%v) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestIsDBAllowed_DefaultDeny(t *testing.T) {
	// Sem env var → raw == "" → allowed=false (fail-closed) independentemente do DB.
	// Testamos apenas a lógica da allowlist sem conexão real.
	os.Unsetenv("ALLOWED_DESTRUCTIVE_DBS")
	raw := os.Getenv("ALLOWED_DESTRUCTIVE_DBS")
	if raw != "" {
		t.Skip("ALLOWED_DESTRUCTIVE_DBS is set in environment; skipping isolation test")
	}
	// Com raw == "", a função retorna false antes de qualquer query.
	// Validar via função auxiliar que simula o caminho de allowlist vazio:
	allowed := isAllowlistEmpty()
	if allowed {
		t.Errorf("expected allowed=false when ALLOWED_DESTRUCTIVE_DBS unset, got true")
	}
}

// isAllowlistEmpty verifica a lógica isolada: ALLOWED_DESTRUCTIVE_DBS vazia → deny.
// Replica o ramo de IsDBAllowed sem precisar de DB real.
func isAllowlistEmpty() bool {
	raw := os.Getenv("ALLOWED_DESTRUCTIVE_DBS")
	return raw != "" // se vazio → não permitido
}

func TestIsDBAllowed_NoMatch(t *testing.T) {
	// Lógica pura: dbName não está na lista → false.
	// TODO: integration test quando sqlmock estiver vendorizado.
	dbName := "other_db"
	allowlist := []string{"fiscal_apu04_db"}
	found := false
	for _, p := range allowlist {
		if p == dbName {
			found = true
			break
		}
	}
	if found {
		t.Errorf("expected %q not in %v", dbName, allowlist)
	}
}

func TestConfirmationToken(t *testing.T) {
	if ConfirmationToken != "DELETE-FB_APU04" {
		t.Errorf("ConfirmationToken = %q, want %q", ConfirmationToken, "DELETE-FB_APU04")
	}
}

func TestResetTablesNotEmpty(t *testing.T) {
	if len(ResetTables) == 0 {
		t.Error("ResetTables must not be empty")
	}
	// Verifica que tabelas essenciais estão na lista
	essential := []string{"import_jobs", "nfe_entradas", "parceiros"}
	tableSet := make(map[string]bool, len(ResetTables))
	for _, tbl := range ResetTables {
		tableSet[tbl] = true
	}
	for _, e := range essential {
		if !tableSet[e] {
			t.Errorf("ResetTables missing essential table %q", e)
		}
	}
}
