package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// insertOrphanUserTeste cria um usuário sem nenhuma empresa/ambiente vinculado (o cenário
// que dispara o auto-provisioning em finishLogin) com senha conhecida, e devolve uma função
// de cleanup.
func insertOrphanUserTeste(t *testing.T, db *sql.DB, email, fullName, senha string) (userID string, cleanup func()) {
	t.Helper()
	hash, err := HashPassword(senha)
	if err != nil {
		t.Fatalf("falha ao gerar hash de senha: %v", err)
	}
	err = db.QueryRow(`
		INSERT INTO users (email, password_hash, full_name, trial_ends_at, is_verified, role)
		VALUES ($1, $2, $3, $4, true, 'user')
		RETURNING id
	`, email, hash, fullName, time.Now().Add(365*24*time.Hour)).Scan(&userID)
	if err != nil {
		t.Fatalf("falha ao inserir usuário órfão de teste: %v", err)
	}
	return userID, func() {
		// Apaga a empresa/ambiente/grupo criados pelo auto-provisioning (se houver) antes do
		// usuário, já que companies.owner_id/environments não têm ON DELETE CASCADE garantido
		// a partir de users.
		var companyID sql.NullString
		db.QueryRow(`SELECT id FROM companies WHERE owner_id = $1`, userID).Scan(&companyID)
		if companyID.Valid {
			var groupID sql.NullString
			db.QueryRow(`SELECT group_id FROM companies WHERE id = $1`, companyID.String).Scan(&groupID)
			db.Exec(`DELETE FROM companies WHERE id = $1`, companyID.String)
			if groupID.Valid {
				var envID sql.NullString
				db.QueryRow(`SELECT environment_id FROM enterprise_groups WHERE id = $1`, groupID.String).Scan(&envID)
				db.Exec(`DELETE FROM enterprise_groups WHERE id = $1`, groupID.String)
				if envID.Valid {
					db.Exec(`DELETE FROM user_environments WHERE environment_id = $1`, envID.String)
					db.Exec(`DELETE FROM environments WHERE id = $1`, envID.String)
				}
			}
		}
		db.Exec(`DELETE FROM users WHERE id = $1`, userID)
	}
}

func loginTeste(t *testing.T, db *sql.DB, email, senha string) AuthResponse {
	t.Helper()
	body, _ := json.Marshal(LoginRequest{Email: email, Password: senha})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:0"
	rr := httptest.NewRecorder()
	LoginHandler(db)(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("esperava login OK (200), obteve %d: %s", rr.Code, rr.Body.String())
	}
	var resp AuthResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("erro ao decodificar resposta de login: %v", err)
	}
	return resp
}

// TestFinishLogin_AutoProvisioning_UsuariosOrfaosNaoCompartilhamAmbiente é o teste de
// regressão do achado CRITICAL da auditoria de segurança de 06/11: dois usuários "órfãos"
// (sem empresa/ambiente vinculado) não-relacionados entre si, ao logar, NÃO podem mais cair
// no mesmo ambiente/grupo compartilhado (antes: lookup fixo por nome "Ambiente de Testes" /
// "Grupo de Empresas Testes", pré-semeado pela migration 016 — qualquer usuário trial caía
// ali e virava admin mútuo da empresa de qualquer outro usuário trial). Agora cada login de
// usuário órfão deve criar um ambiente/grupo/empresa NOVOS e isolados, como o RegisterHandler
// já fazia.
func TestFinishLogin_AutoProvisioning_UsuariosOrfaosNaoCompartilhamAmbiente(t *testing.T) {
	db := openAuthTestDB(t)
	defer db.Close()

	const senha = "SenhaForte123!"
	emailA := "orfao-a-" + time.Now().Format("20060102150405.000") + "@teste.com"
	emailB := "orfao-b-" + time.Now().Format("20060102150405.000") + "@teste.com"

	_, cleanupA := insertOrphanUserTeste(t, db, emailA, "Usuário Órfão A", senha)
	defer cleanupA()
	_, cleanupB := insertOrphanUserTeste(t, db, emailB, "Usuário Órfão B", senha)
	defer cleanupB()

	respA := loginTeste(t, db, emailA, senha)
	respB := loginTeste(t, db, emailB, senha)

	if respA.CompanyID == "" || respB.CompanyID == "" {
		t.Fatalf("esperava auto-provisioning bem-sucedido para ambos — CompanyID A=%q B=%q", respA.CompanyID, respB.CompanyID)
	}
	if respA.CompanyID == respB.CompanyID {
		t.Fatalf("REGRESSÃO DE SEGURANÇA: usuários órfãos não-relacionados caíram na MESMA empresa (%s) — admin mútuo entre tenants", respA.CompanyID)
	}
	if respA.Environment == respB.Environment {
		t.Fatalf("REGRESSÃO DE SEGURANÇA: usuários órfãos não-relacionados caíram no MESMO ambiente (%q) — admin mútuo entre tenants", respA.Environment)
	}
	if respA.Group == respB.Group {
		t.Fatalf("REGRESSÃO DE SEGURANÇA: usuários órfãos não-relacionados caíram no MESMO grupo (%q) — admin mútuo entre tenants", respA.Group)
	}
	if respA.Environment == "Ambiente de Testes" || respB.Environment == "Ambiente de Testes" {
		t.Fatalf("auto-provisioning ainda está usando o ambiente compartilhado legado 'Ambiente de Testes'")
	}
}

// openAuthTestDB abre o Postgres de teste e pula (não falha) se não houver banco com a
// tabela users disponível — mesmo padrão do openTestDB do FB_APU02.
func openAuthTestDB(t *testing.T) *sql.DB {
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
	var exists bool
	if err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'users')`).Scan(&exists); err != nil || !exists {
		t.Skip("tabela users não existe — rode as migrations antes deste teste")
	}
	return db
}
