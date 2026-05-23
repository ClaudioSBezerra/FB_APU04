package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
)

// ---------------------------------------------------------------------------
// Structs
// ---------------------------------------------------------------------------

type FronteiraItemRow struct {
	ChaveNFe      string  `json:"chave_nfe"`
	DataEmissao   string  `json:"data_emissao"`
	NumeroNFe     string  `json:"numero_nfe"`
	FornCNPJ      string  `json:"forn_cnpj"`
	FornNome      string  `json:"forn_nome"`
	FornUF        string  `json:"forn_uf"`
	FornSimples   bool    `json:"forn_simples"`
	CFOP          string  `json:"cfop"`
	Regime        string  `json:"regime"`
	NItem         int     `json:"n_item"`
	CProd         string  `json:"c_prod"`
	XProd         string  `json:"x_prod"`
	NCM           string  `json:"ncm"`
	CEST          string  `json:"cest"`
	VProdItem     float64 `json:"v_prod_item"`
	VIpiItem      float64 `json:"v_ipi_item"`
	VOutroRateado float64 `json:"v_outro_rateado"`
	VOperacao     float64 `json:"v_operacao"`
	VIcmsItem     float64 `json:"v_icms_item"`
	AliqInter     float64 `json:"aliq_inter"`
	AliqInterna   float64 `json:"aliq_interna"`
	BC            float64 `json:"bc"`
	IcmsCalculado float64 `json:"icms_calculado"`
	IcmsRetido    float64 `json:"icms_retido"`
}

type FronteiraItensResponse struct {
	Rows  []FronteiraItemRow `json:"rows"`
	Total float64            `json:"total"`
	Count int                `json:"count"`
}

// ---------------------------------------------------------------------------
// IcmsFronteiraItensHandler — GET /api/icms-fronteira/itens?regime=todos|ANTECIPACAO|ST|DIFAL
// ---------------------------------------------------------------------------

// fronteiraItensQueryBody is the full item-level query without a row cap.
// Append a LIMIT clause for interactive use; use as-is for exports.
const fronteiraItensQueryBody = `
WITH base AS (
    SELECT
        ne.chave_nfe,
        ne.data_emissao::text                                           AS data_emissao,
        COALESCE(ne.numero_nfe, '')                                     AS numero_nfe,
        COALESCE(ne.forn_cnpj, '')                                      AS forn_cnpj,
        COALESCE(ne.forn_nome, '')                                      AS forn_nome,
        COALESCE(ne.forn_uf, '')                                        AS forn_uf,
        (fs.cnpj IS NOT NULL)                                           AS forn_simples,
        COALESCE(ne.cfop, '')                                           AS cfop,
        CASE
            WHEN ne.cfop IN ('2551','2556')               THEN 'DIFAL'
            WHEN ne.cfop IN ('2403','2409','2651','2652') THEN 'ST'
            WHEN ne.cfop IN ('2101','2102','2152')        THEN 'ANTECIPACAO'
        END                                                             AS regime,
        nii.n_item::int                                                 AS n_item,
        COALESCE(nii.c_prod, '')                                        AS c_prod,
        COALESCE(nii.x_prod, '')                                        AS x_prod,
        COALESCE(nii.ncm, '')                                           AS ncm,
        COALESCE(nii.cest, '')                                          AS cest,
        COALESCE(nii.v_prod, 0)                                         AS v_prod_item,
        COALESCE(nii.v_ipi, 0)                                          AS v_ipi_item,
        CASE
            WHEN COALESCE(ne.v_prod, 0) > 0
            THEN ROUND(COALESCE(ne.v_outro, 0) * COALESCE(nii.v_prod, 0) / ne.v_prod, 2)
            ELSE 0
        END                                                             AS v_outro_rateado,
        COALESCE(nii.v_icms, 0)                                         AS v_icms_item,
        CASE
            WHEN COALESCE(nii.cst_orig, ne.cst_orig_pred) IN ('1','2','3','6','7','8') THEN 4.0
            WHEN ne.forn_uf = ANY(ARRAY['PR','RS','SC','MG','RJ','SP'])                 THEN 7.0
            ELSE 12.0
        END                                                             AS aliq_inter,
        COALESCE(regra.aliquota_interna, 20.5)                          AS aliq_interna,
        COALESCE(ne.v_prod, 0)                                          AS v_prod_nf_total,
        COALESCE(ne.v_st, 0)                                            AS v_st_nf_total,
        COALESCE(ne.forn_uf, '')                                        AS forn_uf_raw
    FROM nfe_entradas ne
    INNER JOIN nfe_entradas_itens nii ON nii.nfe_id = ne.id
    LEFT JOIN forn_simples fs ON fs.cnpj = ne.forn_cnpj
    LEFT JOIN LATERAL (
        SELECT r.aliquota_interna
        FROM icms_fronteira_regras_ncm r
        WHERE (r.company_id = $1 OR r.company_id IS NULL)
          AND nii.ncm IS NOT NULL
          AND LEFT(nii.ncm, LENGTH(r.ncm_prefixo)) = r.ncm_prefixo
        ORDER BY r.company_id NULLS LAST, LENGTH(r.ncm_prefixo) DESC
        LIMIT 1
    ) regra ON true
    WHERE ne.company_id = $1
      AND ne.forn_uf IS NOT NULL
      AND ne.forn_uf != ''
      AND ne.forn_uf != COALESCE(ne.dest_uf, 'PE')
      AND ne.cfop = ANY(ARRAY['2101','2102','2152','2403','2409','2651','2652','2551','2556'])
), computed AS (
    SELECT
        chave_nfe, data_emissao, numero_nfe, forn_cnpj, forn_nome, forn_uf,
        forn_simples, cfop, regime,
        n_item, c_prod, x_prod, ncm, cest,
        v_prod_item, v_ipi_item, v_outro_rateado,
        (v_prod_item + v_ipi_item + v_outro_rateado)                    AS v_operacao,
        v_icms_item, aliq_inter, aliq_interna,
        -- BC: PE antecipação usa preço presumido; BA/CE e DIFAL usam v_operacao direta
        CASE
            WHEN regime = 'ANTECIPACAO' AND forn_uf_raw NOT IN ('BA','CE')
                THEN GREATEST(0,
                    (v_prod_item + v_ipi_item + v_outro_rateado - v_icms_item)
                    / NULLIF(1.0 - aliq_interna / 100.0, 0))
            ELSE (v_prod_item + v_ipi_item + v_outro_rateado)
        END                                                             AS bc,
        CASE
            WHEN regime = 'ST' AND v_prod_nf_total > 0
                THEN ROUND(v_st_nf_total * v_prod_item / v_prod_nf_total, 2)
            ELSE 0
        END                                                             AS icms_retido
    FROM base
)
SELECT
    chave_nfe, data_emissao, numero_nfe, forn_cnpj, forn_nome, forn_uf,
    forn_simples, cfop, regime,
    n_item, c_prod, x_prod, ncm, cest,
    v_prod_item, v_ipi_item, v_outro_rateado, v_operacao, v_icms_item,
    aliq_inter, aliq_interna, bc,
    CASE
        WHEN regime = 'ST' THEN icms_retido
        ELSE GREATEST(0, bc * (aliq_interna - aliq_inter) / 100.0)
    END                                                                 AS icms_calculado,
    icms_retido
FROM computed
WHERE ($2::text = 'todos' OR regime = $2::text)
ORDER BY data_emissao DESC, chave_nfe, n_item
`

const fronteiraItensQuery = fronteiraItensQueryBody + "LIMIT 2000\n"

func IcmsFronteiraItensHandler(db *sql.DB) http.HandlerFunc {
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

		regime := r.URL.Query().Get("regime")
		if regime == "" {
			regime = "todos"
		}

		rows, err := db.Query(fronteiraItensQuery, companyID, regime)
		if err != nil {
			log.Printf("IcmsFronteiraItens error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar itens ICMS Fronteira")
			return
		}
		defer rows.Close()

		result := []FronteiraItemRow{}
		var total float64

		for rows.Next() {
			var row FronteiraItemRow
			if err := rows.Scan(
				&row.ChaveNFe, &row.DataEmissao, &row.NumeroNFe,
				&row.FornCNPJ, &row.FornNome, &row.FornUF,
				&row.FornSimples, &row.CFOP, &row.Regime,
				&row.NItem, &row.CProd, &row.XProd, &row.NCM, &row.CEST,
				&row.VProdItem, &row.VIpiItem, &row.VOutroRateado, &row.VOperacao, &row.VIcmsItem,
				&row.AliqInter, &row.AliqInterna, &row.BC,
				&row.IcmsCalculado, &row.IcmsRetido,
			); err != nil {
				log.Printf("IcmsFronteiraItens scan error: %v", err)
				continue
			}
			total += row.IcmsCalculado
			result = append(result, row)
		}

		json.NewEncoder(w).Encode(FronteiraItensResponse{
			Rows:  result,
			Total: total,
			Count: len(result),
		})
	}
}
