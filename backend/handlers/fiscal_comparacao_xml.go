// fiscal_comparacao_xml.go — GET /api/fiscal/comparacao/xml?nfe_id=...
//
// Devolve o XML BRUTO da NF-e (coluna pacotefiscal_nfe_saidas.xml_content,
// migration 154), para conferir divergências contra a fonte na tela de
// Comparação Fiscal (pedido do Claudio, 2026-07-08). Company-scoped com o
// mesmo guard IDOR das demais rotas (o nfe_id só resolve dentro da empresa
// efetiva). Aceita token via ?token= (AuthMiddleware) e company via
// ?company_id= para permitir abrir em nova aba (window.open não manda headers).
//
// Notas importadas ANTES da migration 154 têm xml_content NULL → 404 com
// mensagem orientando a reimportar.
package handlers

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// FiscalComparacaoXMLHandler — GET /api/fiscal/comparacao/xml
func FiscalComparacaoXMLHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonErr(w, http.StatusMethodNotAllowed, "Método não permitido")
			return
		}

		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		userID, _ := claims["user_id"].(string)

		// company via header, com fallback ?company_id= (abrir em nova aba)
		reqCompany := r.Header.Get("X-Company-ID")
		if q := strings.TrimSpace(r.URL.Query().Get("company_id")); q != "" {
			reqCompany = q
		}
		companyID, err := GetEffectiveCompanyID(db, userID, reqCompany)
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa")
			return
		}

		nfeID := strings.TrimSpace(r.URL.Query().Get("nfe_id"))
		if nfeID == "" {
			jsonErr(w, http.StatusBadRequest, "nfe_id é obrigatório")
			return
		}

		// Guard IDOR: a nota só resolve dentro da empresa efetiva.
		var xmlContent sql.NullString
		var chave string
		qerr := db.QueryRow(`
			SELECT xml_content, chave_nfe
			FROM pacotefiscal_nfe_saidas
			WHERE id = $1 AND company_id = $2`, nfeID, companyID).Scan(&xmlContent, &chave)
		if qerr == sql.ErrNoRows {
			jsonErr(w, http.StatusNotFound, "Nota não encontrada")
			return
		}
		if qerr != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar a nota")
			return
		}
		if !xmlContent.Valid || strings.TrimSpace(xmlContent.String) == "" {
			jsonErr(w, http.StatusNotFound,
				"XML não disponível para esta nota — importada antes da atualização. Reimporte os XMLs para habilitar a visualização.")
			return
		}

		// Content-Disposition inline com nome pela chave para o "abrir em nova aba"
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.Header().Set("Content-Disposition", "inline; filename=\""+chave+".xml\"")
		w.Write([]byte(xmlContent.String))
	}
}
