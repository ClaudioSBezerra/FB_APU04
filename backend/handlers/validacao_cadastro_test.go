package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func TestNomeSeguro(t *testing.T) {
	ok := []string{"Ferreira Costa & Cia Ltda", "  Empresa   Teste ", "João D'Ávila-Silva", "日本語 Ltda"}
	for _, v := range ok {
		if _, err := NomeSeguro("name", v, 120); err != nil {
			t.Errorf("%q deveria passar: %v", v, err)
		}
	}
	ruins := []string{
		"", "   ", "<script>alert(1)</script>", "<img src=x onerror=fetch('https://x')>", "a>b",
		"nul\x00byte", "esc\x1b[31m", "bidi‮oculto", "zero​width",
		strings.Repeat("a", 121), "\xff\xfe",
	}
	for _, v := range ruins {
		if _, err := NomeSeguro("name", v, 120); err == nil {
			t.Errorf("%q deveria ser recusado", v)
		}
	}
	got, _ := NomeSeguro("name", "  A   B  ", 120)
	if got != "A B" {
		t.Errorf("normalização: %q", got)
	}
	// quebra de linha/tab viram espaço (normalização), não passam como estão
	if got, err := NomeSeguro("name", "linha1\nlinha2\tx", 120); err != nil || got != "linha1 linha2 x" {
		t.Errorf("quebras de linha: %q %v", got, err)
	}
}

func TestSanitizarNome(t *testing.T) {
	cases := map[string]string{
		"<script>alert(1)</script>": "scriptalert(1)/script",
		"Maria\n da  Silva":         "Maria da Silva",
		"ok":                        "ok",
		"x‮y":                       "xy",
	}
	for in, want := range cases {
		if got := SanitizarNome(in, 120); got != want {
			t.Errorf("SanitizarNome(%q) = %q, quer %q", in, got, want)
		}
	}
	if got := SanitizarNome(strings.Repeat("é", 300), 120); len([]rune(got)) != 120 {
		t.Errorf("deveria truncar em 120 caracteres, veio %d", len([]rune(got)))
	}
}

func TestEmailValido(t *testing.T) {
	for _, e := range []string{"a@b.com", "nome.sobrenome+tag@empresa.com.br"} {
		if !EmailValido(e) {
			t.Errorf("%q deveria ser válido", e)
		}
	}
	for _, e := range []string{"", "sem-arroba", "Nome <a@b.com>", "a@b.com ", "<a@b.com>", strings.Repeat("a", 250) + "@b.com"} {
		if EmailValido(e) {
			t.Errorf("%q deveria ser inválido", e)
		}
	}
}

func TestRegistroPublicoDesligadoPorPadrao(t *testing.T) {
	t.Setenv("REGISTRO_PUBLICO", "")
	if RegistroPublicoAtivo() {
		t.Fatal("padrão deve ser desligado")
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(`{"full_name":"x","email":"a@b.com","password":"12345678","company_name":"y"}`))
	RegisterHandler(nil)(rec, req) // não pode chegar ao banco
	if rec.Code != http.StatusForbidden {
		t.Fatalf("esperava 403 com cadastro desligado, veio %d", rec.Code)
	}
	t.Setenv("REGISTRO_PUBLICO", "true")
	if !RegistroPublicoAtivo() {
		t.Fatal("REGISTRO_PUBLICO=true deveria ligar")
	}
}

func TestRegistroRecusaNomeComHTMLMesmoLigado(t *testing.T) {
	t.Setenv("REGISTRO_PUBLICO", "true")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(`{"full_name":"<script>alert(1)</script>","email":"a@b.com","password":"12345678","company_name":"Empresa"}`))
	req.RemoteAddr = "203.0.113.50:1234"
	RegisterHandler(nil)(rec, req) // recusa antes de tocar o banco
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperava 400 para nome com HTML, veio %d", rec.Code)
	}
}

func TestGetClientIPSaltosConfiaveis(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Forwarded-For", "198.51.100.7, 172.16.1.1, 172.18.0.5")
	t.Setenv("TRUSTED_PROXY_HOPS", "")
	if got := GetClientIP(r); got != "172.18.0.5" { // padrão: comportamento antigo
		t.Errorf("padrão: %q", got)
	}
	t.Setenv("TRUSTED_PROXY_HOPS", "3")
	if got := GetClientIP(r); got != "198.51.100.7" {
		t.Errorf("3 saltos: %q", got)
	}
	t.Setenv("TRUSTED_PROXY_HOPS", "9") // maior que a cadeia: não estoura
	if got := GetClientIP(r); got != "198.51.100.7" {
		t.Errorf("salto excessivo: %q", got)
	}
	// cliente tentando forjar: o valor forjado fica à esquerda e é ignorado com os saltos certos
	r.Header.Set("X-Forwarded-For", "6.6.6.6, 198.51.100.7, 172.16.1.1, 172.18.0.5")
	t.Setenv("TRUSTED_PROXY_HOPS", "3")
	if got := GetClientIP(r); got != "198.51.100.7" {
		t.Errorf("forjado: %q", got)
	}
}

func comClaims(r *http.Request, userID, role string) *http.Request {
	ctx := context.WithValue(r.Context(), ClaimsKey, jwt.MapClaims{"user_id": userID, "role": role})
	return r.WithContext(ctx)
}

func TestAmbienteSoAdministrador(t *testing.T) {
	corpo := `{"name":"Ambiente X"}`
	handlers := map[string]http.HandlerFunc{
		"criar":  CreateEnvironmentHandler(nil),
		"editar": UpdateEnvironmentHandler(nil),
		"apagar": DeleteEnvironmentHandler(nil),
	}
	for nome, h := range handlers {
		// sem sessão → 401
		rec := httptest.NewRecorder()
		h(rec, httptest.NewRequest(http.MethodPost, "/api/config/environments?id=abc", strings.NewReader(corpo)))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s sem sessão: esperava 401, veio %d", nome, rec.Code)
		}
		// usuário comum → 403, sem tocar o banco (nil)
		rec = httptest.NewRecorder()
		h(rec, comClaims(httptest.NewRequest(http.MethodPost, "/api/config/environments?id=abc", strings.NewReader(corpo)), "u1", "user"))
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s como usuário comum: esperava 403, veio %d", nome, rec.Code)
		}
	}
}

func TestCriarAmbienteRecusaNomeComHTML(t *testing.T) {
	for _, nome := range []string{"<script>alert(1)</script>", "<img src=x onerror=fetch('https://x')>", ""} {
		rec := httptest.NewRecorder()
		req := comClaims(httptest.NewRequest(http.MethodPost, "/api/config/environments", strings.NewReader(`{"name":`+jsonStr(nome)+`}`)), "u1", "admin")
		CreateEnvironmentHandler(nil)(rec, req) // recusa antes do banco
		if rec.Code != http.StatusBadRequest {
			t.Errorf("nome %q: esperava 400, veio %d", nome, rec.Code)
		}
	}
}

func TestCriarGrupoERecusaNomeComHTMLAntesDoBanco(t *testing.T) {
	rec := httptest.NewRecorder()
	req := comClaims(httptest.NewRequest(http.MethodPost, "/api/config/groups", strings.NewReader(`{"environment_id":"e1","name":"<b>x</b>"}`)), "u1", "user")
	CreateGroupHandler(nil)(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("esperava 400, veio %d", rec.Code)
	}
}

func jsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
