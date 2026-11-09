package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// storeRefreshTokenTeste insere uma entrada em refreshTokenStore simulando um login
// anterior (o que RefreshHandler consultaria), e devolve o valor do cookie + um cleanup.
func storeRefreshTokenTeste(t *testing.T, userID, role string) (cookieValue string, cleanup func()) {
	t.Helper()
	cookieValue = generateRefreshTokenString()
	refreshTokenStore.Store(cookieValue, refreshTokenData{
		UserID:    userID,
		Role:      role,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	})
	return cookieValue, func() { refreshTokenStore.Delete(cookieValue) }
}

func refreshRequestTeste(cookieValue string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: cookieValue})
	return req
}

func decodeRoleFromToken(t *testing.T, token string) string {
	t.Helper()
	parsed, _, err := jwt.NewParser().ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		t.Fatalf("falha ao decodificar token: %v", err)
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatalf("claims inesperadas no token")
	}
	role, _ := claims["role"].(string)
	return role
}

// TestRefreshHandler_RoleDesatualizadaNoRefreshToken_UsaRoleAtualDoBanco é o teste de
// regressão do achado HIGH da auditoria de segurança (06/11/2026): antes, RefreshHandler
// reemitia o access token com data.Role (snapshot congelado do momento do login original),
// então um rebaixamento de role no banco não tinha efeito prático até o refresh token
// expirar sozinho (até 7 dias via renovação deslizante). Agora revalida direto no banco a
// cada uso.
func TestRefreshHandler_RoleDesatualizadaNoRefreshToken_UsaRoleAtualDoBanco(t *testing.T) {
	db := openAuthTestDB(t)
	defer db.Close()

	userID, cleanupUser := insertOrphanUserTeste(t, db, "refresh-role-"+time.Now().Format("20060102150405.000")+"@teste.com", "Usuário Teste", "SenhaForte123!")
	defer cleanupUser()
	// Simula sessão emitida quando o usuário ainda era admin — depois foi rebaixado para
	// 'user' no banco, mas o refresh token em memória ainda carrega "admin" (role da
	// insertOrphanUserTeste é sempre 'user', já nasce com o papel atual; o refreshTokenStore
	// é que fica desatualizado de propósito aqui, simulando uma sessão antiga).
	cookieValue, cleanupToken := storeRefreshTokenTeste(t, userID, "admin")
	defer cleanupToken()

	rec := httptest.NewRecorder()
	RefreshHandler(db)(rec, refreshRequestTeste(cookieValue))

	if rec.Code != http.StatusOK {
		t.Fatalf("esperava 200, obteve %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("erro ao decodificar resposta: %v", err)
	}
	role := decodeRoleFromToken(t, resp.Token)
	if role != "user" {
		t.Fatalf("REGRESSÃO DE SEGURANÇA: token reemitido manteve role=%q (desatualizada do refresh token em memória) em vez de reler 'user' do banco", role)
	}
}

// TestRefreshHandler_UsuarioBloqueado_RecebeForbidden cobre o outro lado do mesmo achado:
// bloquear uma conta deve impedir a renovação do access token, não só o login.
func TestRefreshHandler_UsuarioBloqueado_RecebeForbidden(t *testing.T) {
	db := openAuthTestDB(t)
	defer db.Close()

	userID, cleanupUser := insertOrphanUserTeste(t, db, "refresh-blocked-"+time.Now().Format("20060102150405.000")+"@teste.com", "Usuário Bloqueado", "SenhaForte123!")
	defer cleanupUser()
	if _, err := db.Exec("UPDATE users SET is_blocked = true WHERE id = $1", userID); err != nil {
		t.Fatalf("falha ao bloquear usuário de teste: %v", err)
	}
	cookieValue, cleanupToken := storeRefreshTokenTeste(t, userID, "user")
	defer cleanupToken()

	rec := httptest.NewRecorder()
	RefreshHandler(db)(rec, refreshRequestTeste(cookieValue))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("REGRESSÃO DE SEGURANÇA: esperava 403 pra usuário bloqueado, obteve %d: %s", rec.Code, rec.Body.String())
	}
	if _, stillThere := refreshTokenStore.Load(cookieValue); stillThere {
		t.Fatalf("refresh token de usuário bloqueado deveria ter sido removido do store")
	}
}

// TestChangePasswordHandler_RevogaRefreshTokensDoUsuario é o teste de regressão da parte
// "revoga sessões" do mesmo achado HIGH: trocar a senha deve invalidar qualquer refresh
// token já emitido (sem isso, uma sessão com a senha antiga continuava renovando por até
// 7 dias).
func TestChangePasswordHandler_RevogaRefreshTokensDoUsuario(t *testing.T) {
	db := openAuthTestDB(t)
	defer db.Close()

	const senhaAtual = "SenhaAtual123!"
	userID, cleanupUser := insertOrphanUserTeste(t, db, "changepw-"+time.Now().Format("20060102150405.000")+"@teste.com", "Usuário Teste", senhaAtual)
	defer cleanupUser()
	cookieValue, _ := storeRefreshTokenTeste(t, userID, "user")
	defer refreshTokenStore.Delete(cookieValue) // só por segurança, deve já ter sido removido

	token, err := GenerateToken(userID, "user", nil)
	if err != nil {
		t.Fatalf("falha ao gerar token de teste: %v", err)
	}

	body := []byte(`{"current_password":"` + senhaAtual + `","new_password":"SenhaNova456!"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/change-password", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	AuthMiddleware(ChangePasswordHandler(db), "")(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperava 200, obteve %d: %s", rec.Code, rec.Body.String())
	}
	if _, stillThere := refreshTokenStore.Load(cookieValue); stillThere {
		t.Fatalf("REGRESSÃO DE SEGURANÇA: refresh token continuou válido após troca de senha")
	}
}

// TestBlockUserHandler_RevogaRefreshTokensDoUsuario cobre o mesmo achado no caminho de
// bloqueio administrativo.
func TestBlockUserHandler_RevogaRefreshTokensDoUsuario(t *testing.T) {
	db := openAuthTestDB(t)
	defer db.Close()

	userID, cleanupUser := insertOrphanUserTeste(t, db, "blockuser-"+time.Now().Format("20060102150405.000")+"@teste.com", "Usuário Teste", "SenhaForte123!")
	defer cleanupUser()
	cookieValue, _ := storeRefreshTokenTeste(t, userID, "user")
	defer refreshTokenStore.Delete(cookieValue)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/block?id="+userID, bytes.NewReader([]byte(`{"blocked":true}`)))
	rec := httptest.NewRecorder()
	BlockUserHandler(db)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperava 200, obteve %d: %s", rec.Code, rec.Body.String())
	}
	if _, stillThere := refreshTokenStore.Load(cookieValue); stillThere {
		t.Fatalf("REGRESSÃO DE SEGURANÇA: refresh token continuou válido após bloqueio de usuário")
	}
}
