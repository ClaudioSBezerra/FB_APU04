package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// AdminNFCancelamentoHandler — cancela/reativa NFs por chave_nfe (deleção lógica).
//
// POST /api/admin/nf/cancelamento
//
//	Body: { "chave_nfe": "...", "status": "CANCELADO" | "ATIVO" }
//	Atualiza nfe_entradas.status e reg_c100.status para a chave informada.
//	A NF continua visível nos relatórios/tela mas NÃO é somada nos totais.
//
// GET /api/admin/nf/cancelamentos?forn=...&num_nota=...&periodo=MM/YYYY
//
//	Busca NFs do company_id com filtros opcionais. Retorna até 200 registros.
//	Fonte = XML (nfe_entradas) — filtra por company_id, parâmetros opcionais.
func AdminNFCancelamentoHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		userID, _ := claims["user_id"].(string)

		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa: "+err.Error())
			return
		}

		switch r.Method {

		case http.MethodGet:
			forn := strings.TrimSpace(r.URL.Query().Get("forn"))
			numNota := strings.TrimSpace(r.URL.Query().Get("num_nota"))
			periodo := strings.TrimSpace(r.URL.Query().Get("periodo"))

			type NFRow struct {
				ChaveNFe    string `json:"chave_nfe"`
				NumeroNFe   string `json:"numero_nfe"`
				DataEmissao string `json:"data_emissao"`
				FornNome    string `json:"forn_nome"`
				FornCNPJ    string `json:"forn_cnpj"`
				FornUF      string `json:"forn_uf"`
				Status      string `json:"status"`
			}

			rows, err := db.Query(`
				SELECT
					ne.chave_nfe,
					COALESCE(ne.numero_nfe, '') AS numero_nfe,
					ne.data_emissao::text        AS data_emissao,
					COALESCE(ne.forn_nome, '')   AS forn_nome,
					COALESCE(ne.forn_cnpj, '')   AS forn_cnpj,
					COALESCE(ne.forn_uf, '')     AS forn_uf,
					COALESCE(ne.status, 'ATIVO') AS status
				FROM nfe_entradas ne
				WHERE ne.company_id = $1
				  AND ($2::text = '' OR COALESCE(ne.forn_cnpj,'') ILIKE '%'||$2||'%'
				                     OR COALESCE(ne.forn_nome,'') ILIKE '%'||$2||'%')
				  AND ($3::text = '' OR COALESCE(ne.numero_nfe,'') ILIKE '%'||$3||'%')
				  AND ($4::text = '' OR (
				          EXTRACT(MONTH FROM ne.data_emissao)::int = SPLIT_PART($4,'/',1)::int
				      AND EXTRACT(YEAR  FROM ne.data_emissao)::int = SPLIT_PART($4,'/',2)::int
				  ))
				ORDER BY ne.data_emissao DESC, ne.numero_nfe
				LIMIT 200
			`, companyID, forn, numNota, periodo)
			if err != nil {
				jsonErr(w, http.StatusInternalServerError, "Erro ao buscar NFs: "+err.Error())
				return
			}
			defer rows.Close()

			result := []NFRow{}
			for rows.Next() {
				var row NFRow
				if err := rows.Scan(&row.ChaveNFe, &row.NumeroNFe, &row.DataEmissao,
					&row.FornNome, &row.FornCNPJ, &row.FornUF, &row.Status); err != nil {
					continue
				}
				result = append(result, row)
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"rows": result, "count": len(result)})

		case http.MethodPost:
			var body struct {
				ChaveNFe string `json:"chave_nfe"`
				Status   string `json:"status"` // "CANCELADO" | "ATIVO"
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ChaveNFe == "" {
				jsonErr(w, http.StatusBadRequest, "chave_nfe é obrigatório")
				return
			}
			if body.Status != "CANCELADO" && body.Status != "ATIVO" {
				jsonErr(w, http.StatusBadRequest, "status deve ser CANCELADO ou ATIVO")
				return
			}

			// Atualiza nfe_entradas
			_, err = db.Exec(`
				UPDATE nfe_entradas
				SET status = $1
				WHERE company_id = $2 AND chave_nfe = $3
			`, body.Status, companyID, body.ChaveNFe)
			if err != nil {
				jsonErr(w, http.StatusInternalServerError, "Erro ao atualizar nfe_entradas: "+err.Error())
				return
			}

			// Atualiza reg_c100 (via import_jobs para garantir isolamento por empresa)
			_, err = db.Exec(`
				UPDATE reg_c100 rc
				SET status = $1
				FROM import_jobs j
				WHERE rc.job_id = j.id
				  AND j.company_id = $2
				  AND rc.chv_nfe = $3
			`, body.Status, companyID, body.ChaveNFe)
			if err != nil {
				jsonErr(w, http.StatusInternalServerError, "Erro ao atualizar reg_c100: "+err.Error())
				return
			}

			json.NewEncoder(w).Encode(map[string]string{"ok": "true", "status": body.Status})

		default:
			jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
		}
	}
}
