package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// ---------------------------------------------------------------------------
// Relatório de fretes finais PENDENTES (ICMS Fronteira)
//
// Detecta NFs cujo transporte foi entregue a um RECEBEDOR (redespacho, tag
// <receb> do CT-e) que ainda NÃO emitiu o CT-e do trecho final referenciando
// aquela NF. Caso típico: TECADI (toma=0) leva SC→SP com receb=TRANSWINTER; a
// TRANSWINTER faria SP→destino (toma=3, frete da empresa) mas o CT-e dela ainda
// não chegou. Pela regra Gilson (só entra CT-e com tomador=empresa) essas NFs
// ficam com frete ZERO — este relatório sinaliza que há frete a chegar.
//
// Critério: existe CT-e vinculado à NF com receb de raiz CNPJ diferente da
// empresa (transportadora externa) E não existe CT-e emitido por esse receb
// referenciando a mesma NF.
// ---------------------------------------------------------------------------

type FretePendenteRow struct {
	NumeroNFe   string `json:"numero_nfe"`
	ChaveNFe    string `json:"chave_nfe"`
	DataEmissao string `json:"data_emissao"`
	TranspCNPJ  string `json:"transp_cnpj"`
	TranspNome  string `json:"transp_nome"`
}

type FretesPendentesResponse struct {
	Rows  []FretePendenteRow `json:"rows"`
	Count int                `json:"count"`
}

const fretesPendentesQuery = `
WITH liga AS (
    SELECT ne.company_id, ne.chave_nfe, ne.numero_nfe, ne.data_emissao,
           ne.dest_cnpj_cpf, ce.receb_cnpj_cpf, ce.receb_nome
    FROM nfe_entradas ne
    JOIN cte_entradas_nfe_refs ref
         ON ref.company_id = ne.company_id AND ref.chave_nfe = ne.chave_nfe
    JOIN cte_entradas ce ON ce.id = ref.cte_id
    WHERE ne.company_id = $1
      AND ce.receb_cnpj_cpf IS NOT NULL
      -- recebedor externo: raiz de CNPJ diferente da empresa (exclui redespacho
      -- entre filiais do próprio grupo)
      AND LEFT(ce.receb_cnpj_cpf, 8) <> LEFT(COALESCE(ne.dest_cnpj_cpf,''), 8)
      AND ($2::text = '' OR ne.mes_ano = $2)
      AND ($3::text = '' OR COALESCE(ne.dest_uf,'PE') = $3)
), pend AS (
    SELECT l.chave_nfe, l.numero_nfe, l.data_emissao, l.receb_cnpj_cpf,
           MAX(l.receb_nome) AS receb_nome   -- nome mais completo p/ o CNPJ
    FROM liga l
    WHERE NOT EXISTS (
        SELECT 1 FROM cte_entradas_nfe_refs r2
        JOIN cte_entradas c2 ON c2.id = r2.cte_id
        WHERE r2.company_id = l.company_id
          AND r2.chave_nfe = l.chave_nfe
          AND c2.emit_cnpj = l.receb_cnpj_cpf
    )
    GROUP BY l.chave_nfe, l.numero_nfe, l.data_emissao, l.receb_cnpj_cpf
)
SELECT numero_nfe, chave_nfe, COALESCE(data_emissao::text,''), receb_cnpj_cpf, COALESCE(receb_nome,'')
FROM pend
ORDER BY data_emissao, numero_nfe
`

// IcmsFronteiraFretesPendentesHandler — GET /api/icms-fronteira/fretes-pendentes
//
//	Parâmetros opcionais: periodo (MM/YYYY → mes_ano), uf.
func IcmsFronteiraFretesPendentesHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodGet {
			jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
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

		periodo := strings.TrimSpace(r.URL.Query().Get("periodo"))
		uf := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("uf")))

		rows, err := db.Query(fretesPendentesQuery, companyID, periodo, uf)
		if err != nil {
			log.Printf("IcmsFronteiraFretesPendentes error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar fretes pendentes")
			return
		}
		defer rows.Close()

		result := []FretePendenteRow{}
		for rows.Next() {
			var row FretePendenteRow
			if err := rows.Scan(&row.NumeroNFe, &row.ChaveNFe, &row.DataEmissao,
				&row.TranspCNPJ, &row.TranspNome); err != nil {
				log.Printf("IcmsFronteiraFretesPendentes scan error: %v", err)
				continue
			}
			result = append(result, row)
		}

		json.NewEncoder(w).Encode(FretesPendentesResponse{Rows: result, Count: len(result)})
	}
}
