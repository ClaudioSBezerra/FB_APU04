package handlers

// validacao_cadastro.go — validação de nomes e autorização de ambiente/grupo/empresa.
//
// Contexto (nov/2026): um teste de intrusão criou 14 ambientes/grupos/empresas de lixo,
// com HTML/JS no nome ("<script>alert(1)</script>"). Não era SQL injection (todos os
// INSERT usam parâmetros), e sim: cadastro público aberto, nomes sem validação (XSS
// armazenado), rotas de ambiente/grupo sem checagem de dono e sem limite de criação.

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/golang-jwt/jwt/v5"
)

const maxNome = 120

// RegistroPublicoAtivo diz se POST /api/auth/register aceita cadastros. Por padrão o
// cadastro público fica DESLIGADO (o sistema é de uso interno); ligue com
// REGISTRO_PUBLICO=true só se houver necessidade de negócio (e então com convite/captcha).
func RegistroPublicoAtivo() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("REGISTRO_PUBLICO")), "true")
}

func caractereProibido(r rune) bool {
	// < e > (HTML), controles (incl. \n \t \x00) e formatação invisível/bidi (U+202E etc.)
	return r == '<' || r == '>' || unicode.IsControl(r) || unicode.Is(unicode.Cf, r)
}

// NomeSeguro valida um nome informado pelo usuário (empresa, ambiente, grupo, pessoa):
// 1..maxNome caracteres, sem < > nem caracteres de controle/invisíveis. Devolve o valor
// aparado e com espaços internos normalizados.
func NomeSeguro(campo, v string, max int) (string, error) {
	if max <= 0 {
		max = maxNome
	}
	v = strings.Join(strings.Fields(v), " ")
	if v == "" {
		return "", fmt.Errorf("%s é obrigatório", campo)
	}
	if !utf8.ValidString(v) {
		return "", fmt.Errorf("%s contém caracteres inválidos", campo)
	}
	if utf8.RuneCountInString(v) > max {
		return "", fmt.Errorf("%s deve ter no máximo %d caracteres", campo, max)
	}
	for _, r := range v {
		if caractereProibido(r) {
			return "", fmt.Errorf("%s contém caracteres não permitidos", campo)
		}
	}
	return v, nil
}

// SanitizarNome é a versão "sem erro" para valores que vêm de fora e não podem ser
// recusados (ex.: nome vindo do Keycloak no auto-provisionamento): remove o que for
// proibido e trunca.
func SanitizarNome(v string, max int) string {
	if max <= 0 {
		max = maxNome
	}
	var b strings.Builder
	for _, r := range v {
		if r == utf8.RuneError || caractereProibido(r) {
			continue
		}
		b.WriteRune(r)
	}
	s := strings.Join(strings.Fields(b.String()), " ")
	if utf8.RuneCountInString(s) > max {
		s = string([]rune(s)[:max])
	}
	return s
}

// EmailValido: formato simples (RFC 5322 via net/mail), sem nome de exibição, até 254.
func EmailValido(e string) bool {
	if e == "" || len(e) > 254 {
		return false
	}
	a, err := mail.ParseAddress(e)
	return err == nil && a.Address == e && !strings.ContainsAny(e, "<> ")
}

func claimsDe(r *http.Request) (userID, role string, ok bool) {
	claims, okc := r.Context().Value(ClaimsKey).(jwt.MapClaims)
	if !okc {
		return "", "", false
	}
	userID, _ = claims["user_id"].(string)
	role, _ = claims["role"].(string)
	return userID, role, userID != ""
}

// exigirAdmin responde 401/403 e devolve false quando o usuário não é administrador.
func exigirAdmin(w http.ResponseWriter, r *http.Request) bool {
	_, role, ok := claimsDe(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}
	if role != "admin" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return false
	}
	return true
}

// podeAcessarAmbiente: administrador, ou usuário vinculado ao ambiente.
func podeAcessarAmbiente(db *sql.DB, userID, role, envID string) bool {
	if role == "admin" {
		return true
	}
	if envID == "" {
		return false
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM user_environments WHERE user_id = $1 AND environment_id = $2`, userID, envID).Scan(&n); err != nil {
		return false
	}
	return n > 0
}

// podeAcessarGrupo: idem, a partir do grupo (resolve o ambiente dele).
func podeAcessarGrupo(db *sql.DB, userID, role, groupID string) bool {
	if role == "admin" {
		return true
	}
	if groupID == "" {
		return false
	}
	var n int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM enterprise_groups eg
		JOIN user_environments ue ON ue.environment_id = eg.environment_id
		WHERE eg.id = $1 AND ue.user_id = $2`, groupID, userID).Scan(&n); err != nil {
		return false
	}
	return n > 0
}

// erroInterno registra o detalhe no log e devolve mensagem genérica (não vaza erro de banco).
func erroInterno(w http.ResponseWriter, contexto string, err error) {
	if err == nil {
		err = errors.New("erro desconhecido")
	}
	log.Printf("[erro] %s: %v", contexto, err)
	http.Error(w, "Erro interno", http.StatusInternalServerError)
}
