package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGlobalRefTables_EscritaExigeAdmin_UsuarioComumRecebe403 é o teste de regressão do
// achado HIGH da auditoria de segurança (06/11/2026): ImportCFOPsHandler/
// CreateFornSimplesHandler/DeleteFornSimplesHandler/ImportFornSimplesHandler escrevem em
// tabelas GLOBAIS (cfop/forn_simples, sem company_id — migrations 040/0xx) mas eram
// registradas em main.go com role exigida vazia, deixando qualquer usuário comum de
// qualquer tenant corromper dados usados por toda a plataforma. Mesmo padrão de teste já
// usado em TestSaveNFSeCertificadoHandler_AuthMiddlewareNaoAdmin_Retorna403
// (nfse_credentials_test.go) — AuthMiddleware "de verdade", não claims injetadas direto no
// contexto, já que a causa raiz de ambos os achados foi exatamente a falta dessa camada.
func TestGlobalRefTables_EscritaExigeAdmin_UsuarioComumRecebe403(t *testing.T) {
	token, err := GenerateToken("usuario-comum-de-teste", "user", nil)
	if err != nil {
		t.Fatalf("falha ao gerar token de teste: %v", err)
	}

	cases := []struct {
		nome    string
		metodo  string
		path    string
		handler http.HandlerFunc
	}{
		{"ImportCFOPsHandler", http.MethodPost, "/api/config/cfop/import", AuthMiddleware(ImportCFOPsHandler(nil), "admin")},
		{"CreateFornSimplesHandler", http.MethodPost, "/api/config/forn-simples", AuthMiddleware(CreateFornSimplesHandler(nil), "admin")},
		{"DeleteFornSimplesHandler", http.MethodDelete, "/api/config/forn-simples", AuthMiddleware(DeleteFornSimplesHandler(nil), "admin")},
		{"ImportFornSimplesHandler", http.MethodPost, "/api/config/forn-simples/import", AuthMiddleware(ImportFornSimplesHandler(nil), "admin")},
	}

	for _, c := range cases {
		t.Run(c.nome, func(t *testing.T) {
			req := httptest.NewRequest(c.metodo, c.path, bytes.NewReader([]byte("{}")))
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			c.handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("REGRESSÃO DE SEGURANÇA: %s aceitou role=user numa tabela global (esperava 403, obteve %d): %s",
					c.nome, rec.Code, rec.Body.String())
			}
		})
	}
}
