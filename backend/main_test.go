package main

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

// openTestDBMain réplica o padrão já usado em backend/handlers/*_test.go — helpers de teste
// não são compartilhados entre pacotes Go, então cada pacote tem sua própria cópia.
func openTestDBMain(t *testing.T) *sql.DB {
	t.Helper()
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		connStr = "postgres://postgres:postgres@localhost:5432/fiscal_db?sslmode=disable"
	}
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Skipf("DB indisponível para teste de integração: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Skipf("DB indisponível para teste de integração: %v", err)
	}
	return db
}

// TestBootstrapInitialAdmin_SemEnvVars_NaoCriaNada cobre o caso default (nenhum deployment
// deve ganhar uma conta admin sem pedir explicitamente via env var).
func TestBootstrapInitialAdmin_SemEnvVars_NaoCriaNada(t *testing.T) {
	db := openTestDBMain(t)
	defer db.Close()

	os.Unsetenv("INITIAL_ADMIN_EMAIL")
	os.Unsetenv("INITIAL_ADMIN_PASSWORD")

	const email = "bootstrap-test-sem-env@teste.com"
	db.Exec("DELETE FROM users WHERE email = $1", email) // garante estado limpo
	defer db.Exec("DELETE FROM users WHERE email = $1", email)

	bootstrapInitialAdmin(db)

	var exists bool
	if err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", email).Scan(&exists); err != nil {
		t.Fatalf("erro ao verificar: %v", err)
	}
	if exists {
		t.Fatalf("bootstrapInitialAdmin criou um usuário sem as env vars definidas")
	}
}

// TestBootstrapInitialAdmin_ComEnvVars_CriaUmaVezENuncaSobrescreve cobre o caso principal: cria
// a conta quando as 2 env vars estão setadas, e é idempotente em chamadas subsequentes (não
// duplica, não sobrescreve a senha de uma conta já existente).
func TestBootstrapInitialAdmin_ComEnvVars_CriaUmaVezENuncaSobrescreve(t *testing.T) {
	db := openTestDBMain(t)
	defer db.Close()

	const email = "bootstrap-test-com-env@teste.com"
	const senha1 = "SenhaInicial123!"
	const senha2 = "SenhaDiferente456!"

	db.Exec("DELETE FROM users WHERE email = $1", email) // garante estado limpo
	defer db.Exec("DELETE FROM users WHERE email = $1", email)

	os.Setenv("INITIAL_ADMIN_EMAIL", email)
	os.Setenv("INITIAL_ADMIN_PASSWORD", senha1)
	defer os.Unsetenv("INITIAL_ADMIN_EMAIL")
	defer os.Unsetenv("INITIAL_ADMIN_PASSWORD")

	bootstrapInitialAdmin(db)

	var hash1, role string
	var count int
	if err := db.QueryRow("SELECT password_hash, role FROM users WHERE email = $1", email).Scan(&hash1, &role); err != nil {
		t.Fatalf("esperava admin criado, erro: %v", err)
	}
	if role != "admin" {
		t.Errorf("esperava role=admin, obteve %q", role)
	}

	// Segunda chamada com senha diferente — não deve sobrescrever nem duplicar.
	os.Setenv("INITIAL_ADMIN_PASSWORD", senha2)
	bootstrapInitialAdmin(db)

	if err := db.QueryRow("SELECT COUNT(*) FROM users WHERE email = $1", email).Scan(&count); err != nil {
		t.Fatalf("erro ao contar: %v", err)
	}
	if count != 1 {
		t.Fatalf("esperava exatamente 1 usuário após 2 chamadas, obteve %d (duplicação)", count)
	}

	var hash2 string
	if err := db.QueryRow("SELECT password_hash FROM users WHERE email = $1", email).Scan(&hash2); err != nil {
		t.Fatalf("erro ao reler hash: %v", err)
	}
	if hash1 != hash2 {
		t.Fatalf("segunda chamada sobrescreveu a senha da conta já existente — idempotência quebrada")
	}
}

// TestBootstrapInitialAdmin_SenhaCurta_Ignora cobre a guarda mínima de política de senha.
func TestBootstrapInitialAdmin_SenhaCurta_Ignora(t *testing.T) {
	db := openTestDBMain(t)
	defer db.Close()

	const email = "bootstrap-test-senha-curta@teste.com"
	db.Exec("DELETE FROM users WHERE email = $1", email)
	defer db.Exec("DELETE FROM users WHERE email = $1", email)

	os.Setenv("INITIAL_ADMIN_EMAIL", email)
	os.Setenv("INITIAL_ADMIN_PASSWORD", "curta1")
	defer os.Unsetenv("INITIAL_ADMIN_EMAIL")
	defer os.Unsetenv("INITIAL_ADMIN_PASSWORD")

	bootstrapInitialAdmin(db)

	var exists bool
	if err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", email).Scan(&exists); err != nil {
		t.Fatalf("erro ao verificar: %v", err)
	}
	if exists {
		t.Fatalf("bootstrapInitialAdmin criou conta com senha abaixo do mínimo de 8 caracteres")
	}
}
