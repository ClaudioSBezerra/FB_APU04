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

type DivergenciaRow struct {
	ChaveNFe      string  `json:"chave_nfe"`
	Periodo       string  `json:"periodo"`
	NumeroNF      string  `json:"numero_nf"`
	FornCNPJ      string  `json:"forn_cnpj"`
	FornNome      string  `json:"forn_nome"`
	FornUF        string  `json:"forn_uf"`
	DataEmissao   string  `json:"data_emissao"`
	Regime        string  `json:"regime"`
	IcmsSefaz     float64 `json:"icms_sefaz"`
	IcmsCalculado float64 `json:"icms_calculado"`
	Diferenca     float64 `json:"diferenca"` // sefaz - calculado; positivo = cobrado a mais
	Status        string  `json:"status"`    // OK | COBRADO_A_MAIS | COBRADO_A_MENOS | SEM_NOTA | NAO_COBRADO
}

type DivergenciasResponse struct {
	Rows           []DivergenciaRow `json:"rows"`
	TotalSefaz     float64          `json:"total_sefaz"`
	TotalCalculado float64          `json:"total_calculado"`
	TotalDiferenca float64          `json:"total_diferenca"`
	Count          int              `json:"count"`
}

// ---------------------------------------------------------------------------
// SQL
// ---------------------------------------------------------------------------

// divergenciasQueryBody is the full divergências query without a row cap.
// Append a LIMIT clause for interactive use; use as-is for exports.
//
// $1 = company_id
// $2 = periodo MM/YYYY ('' = sem filtro; quando vazio mostra tudo de ambos os lados)
//
// Statuses:
//   COBRADO_A_MAIS  — SEFAZ cobrou mais do que calculamos
//   COBRADO_A_MENOS — SEFAZ cobrou menos do que calculamos
//   SEM_NOTA        — extrato sem NF correspondente no sistema
//   NAO_COBRADO     — NF no sistema sem lançamento no extrato
//   OK              — diferença < R$ 0,05 (tolerância de arredondamento)
const divergenciasQueryBody = `
WITH item_icms AS (
    SELECT
        ne.chave_nfe,
        ne.data_emissao,
        COALESCE(ne.numero_nfe, '')    AS numero_nfe,
        COALESCE(ne.forn_cnpj, '')     AS forn_cnpj,
        COALESCE(ne.forn_nome, '')     AS forn_nome,
        COALESCE(ne.forn_uf, '')       AS forn_uf,
        COALESCE(ne.cfop, '')          AS cfop,
        CASE
            WHEN ne.cfop IN ('2551','2556')               THEN 'DIFAL'
            WHEN ne.cfop IN ('2403','2409','2651','2652') THEN 'ST'
            WHEN ne.cfop IN ('2101','2102','2152')        THEN 'ANTECIPACAO'
        END                            AS regime,
        -- icms_calculado por item
        CASE
            WHEN ne.cfop IN ('2403','2409','2651','2652') THEN
                CASE WHEN regra.mva_original IS NOT NULL THEN
                    -- GAP 4: BC-ST = v_operacao * (1 + MVA/100); ICMS-ST = BC-ST * aliq_int - ICMS próprio
                    GREATEST(0, ROUND(
                        ( COALESCE(nii.v_prod,0) + COALESCE(nii.v_ipi,0)
                          + CASE WHEN COALESCE(ne.v_prod,0) > 0
                                 THEN COALESCE(ne.v_outro,0) * COALESCE(nii.v_prod,0) / ne.v_prod
                                 ELSE 0 END )
                        * (1.0 + regra.mva_original/100.0)
                        * COALESCE(regra.aliquota_interna, 20.5)/100.0
                        - ( COALESCE(nii.v_prod,0) + COALESCE(nii.v_ipi,0)
                            + CASE WHEN COALESCE(ne.v_prod,0) > 0
                                   THEN COALESCE(ne.v_outro,0) * COALESCE(nii.v_prod,0) / ne.v_prod
                                   ELSE 0 END )
                        * CASE WHEN COALESCE(nii.cst_orig, ne.cst_orig_pred) IN ('1','2','3','6','7','8') THEN 4.0
                               WHEN ne.forn_uf = ANY(ARRAY['PR','RS','SC','MG','RJ','SP']) THEN 7.0
                               ELSE 12.0 END
                        / 100.0, 2))
                WHEN COALESCE(ne.v_prod, 0) > 0
                    THEN ne.v_st * COALESCE(nii.v_prod, 0) / ne.v_prod
                ELSE 0 END
            WHEN ne.cfop IN ('2101','2102','2152') AND COALESCE(ne.forn_uf,'') NOT IN ('BA','CE') THEN
                GREATEST(0,
                    GREATEST(0,
                        ( COALESCE(nii.v_prod,0) + COALESCE(nii.v_ipi,0)
                          + CASE WHEN COALESCE(ne.v_prod,0) > 0
                                 THEN COALESCE(ne.v_outro,0) * COALESCE(nii.v_prod,0) / ne.v_prod
                                 ELSE 0 END
                          - COALESCE(nii.v_icms,0) )
                        / NULLIF(1.0 - COALESCE(regra.aliquota_interna, 20.5) / 100.0, 0))
                    * ( COALESCE(regra.aliquota_interna, 20.5)
                        - CASE WHEN COALESCE(nii.cst_orig, ne.cst_orig_pred) IN ('1','2','3','6','7','8') THEN 4.0
                               WHEN ne.forn_uf = ANY(ARRAY['PR','RS','SC','MG','RJ','SP']) THEN 7.0
                               ELSE 12.0 END )
                    / 100.0)
            ELSE
                GREATEST(0,
                    ( COALESCE(nii.v_prod,0) + COALESCE(nii.v_ipi,0)
                      + CASE WHEN COALESCE(ne.v_prod,0) > 0
                             THEN COALESCE(ne.v_outro,0) * COALESCE(nii.v_prod,0) / ne.v_prod
                             ELSE 0 END )
                    * ( COALESCE(regra.aliquota_interna, 20.5)
                        - CASE WHEN COALESCE(nii.cst_orig, ne.cst_orig_pred) IN ('1','2','3','6','7','8') THEN 4.0
                               WHEN ne.forn_uf = ANY(ARRAY['PR','RS','SC','MG','RJ','SP']) THEN 7.0
                               ELSE 12.0 END )
                    / 100.0)
        END AS icms_item
    FROM nfe_entradas ne
    INNER JOIN nfe_entradas_itens nii ON nii.nfe_id = ne.id
    LEFT JOIN LATERAL (
        SELECT r.aliquota_interna, r.mva_original
        FROM icms_fronteira_regras_ncm r
        WHERE (r.company_id = $1 OR r.company_id IS NULL)
          AND nii.ncm IS NOT NULL
          AND LEFT(nii.ncm, LENGTH(r.ncm_prefixo)) = r.ncm_prefixo
        ORDER BY r.company_id NULLS LAST, LENGTH(r.ncm_prefixo) DESC
        LIMIT 1
    ) regra ON true
    WHERE ne.company_id = $1
      AND ne.forn_uf IS NOT NULL AND ne.forn_uf != ''
      AND ne.forn_uf != COALESCE(ne.dest_uf, 'PE')
      AND ne.cfop = ANY(ARRAY['2101','2102','2152','2403','2409','2651','2652','2551','2556'])
      AND ($2::text = '' OR (
          EXTRACT(MONTH FROM ne.data_emissao)::int = SPLIT_PART($2::text,'/',1)::int
          AND EXTRACT(YEAR  FROM ne.data_emissao)::int = SPLIT_PART($2::text,'/',2)::int
      ))
),
nf_calc AS (
    SELECT
        chave_nfe,
        MAX(data_emissao)::text  AS data_emissao,
        MAX(numero_nfe)          AS numero_nfe,
        MAX(forn_cnpj)           AS forn_cnpj,
        MAX(forn_nome)           AS forn_nome,
        MAX(forn_uf)             AS forn_uf,
        MAX(cfop)                AS cfop,
        MAX(regime)              AS regime,
        ROUND(SUM(icms_item), 2) AS icms_calculado
    FROM item_icms
    GROUP BY chave_nfe
),
ext_data AS (
    SELECT *
    FROM icms_fronteira_extrato_sefaz
    WHERE company_id = $1
      AND ($2::text = '' OR periodo = $2::text)
),
joined AS (
    SELECT
        COALESCE(ext.chave_nfe,  calc.chave_nfe)          AS chave_nfe,
        COALESCE(ext.periodo,    $2::text, '')             AS periodo,
        COALESCE(ext.numero_nf,  calc.numero_nfe,  '')     AS numero_nf,
        COALESCE(ext.cnpj_emitente, calc.forn_cnpj, '')    AS forn_cnpj,
        COALESCE(ext.nome_emitente, calc.forn_nome, '')    AS forn_nome,
        COALESCE(ext.uf_emitente,   calc.forn_uf,  '')     AS forn_uf,
        COALESCE(calc.data_emissao, '')                    AS data_emissao,
        COALESCE(calc.regime, '')                          AS regime,
        COALESCE(ext.icms_devido, 0)                       AS icms_sefaz,
        COALESCE(calc.icms_calculado, 0)                   AS icms_calculado,
        ROUND(COALESCE(ext.icms_devido,0) - COALESCE(calc.icms_calculado,0), 2) AS diferenca,
        CASE
            WHEN calc.chave_nfe IS NULL                                                         THEN 'SEM_NOTA'
            WHEN ext.chave_nfe  IS NULL                                                         THEN 'NAO_COBRADO'
            WHEN ABS(COALESCE(ext.icms_devido,0) - COALESCE(calc.icms_calculado,0)) < 0.05     THEN 'OK'
            WHEN COALESCE(ext.icms_devido,0) > COALESCE(calc.icms_calculado,0)                 THEN 'COBRADO_A_MAIS'
            ELSE 'COBRADO_A_MENOS'
        END AS status
    FROM ext_data ext
    FULL OUTER JOIN nf_calc calc ON calc.chave_nfe = ext.chave_nfe
)
SELECT
    chave_nfe, periodo, numero_nf, forn_cnpj, forn_nome, forn_uf,
    data_emissao, regime,
    icms_sefaz, icms_calculado, diferenca, status
FROM joined
ORDER BY
    CASE status
        WHEN 'COBRADO_A_MAIS'  THEN 1
        WHEN 'SEM_NOTA'        THEN 2
        WHEN 'COBRADO_A_MENOS' THEN 3
        WHEN 'NAO_COBRADO'     THEN 4
        ELSE 5
    END,
    ABS(diferenca) DESC
`

const divergenciasQuery = divergenciasQueryBody + "LIMIT 1000\n"

// ---------------------------------------------------------------------------
// IcmsFronteiraDivergenciasHandler — GET /api/icms-fronteira/divergencias
// ---------------------------------------------------------------------------

func IcmsFronteiraDivergenciasHandler(db *sql.DB) http.HandlerFunc {
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

		periodo := r.URL.Query().Get("periodo") // MM/YYYY ou ""

		rows, err := db.Query(divergenciasQuery, companyID, periodo)
		if err != nil {
			log.Printf("IcmsFronteiraDivergencias error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar divergências ICMS Fronteira")
			return
		}
		defer rows.Close()

		result := []DivergenciaRow{}
		var totalSefaz, totalCalculado, totalDiferenca float64

		for rows.Next() {
			var row DivergenciaRow
			if err := rows.Scan(
				&row.ChaveNFe, &row.Periodo, &row.NumeroNF,
				&row.FornCNPJ, &row.FornNome, &row.FornUF,
				&row.DataEmissao, &row.Regime,
				&row.IcmsSefaz, &row.IcmsCalculado, &row.Diferenca, &row.Status,
			); err != nil {
				log.Printf("IcmsFronteiraDivergencias scan error: %v", err)
				continue
			}
			totalSefaz += row.IcmsSefaz
			totalCalculado += row.IcmsCalculado
			totalDiferenca += row.Diferenca
			result = append(result, row)
		}

		json.NewEncoder(w).Encode(DivergenciasResponse{
			Rows:           result,
			TotalSefaz:     totalSefaz,
			TotalCalculado: totalCalculado,
			TotalDiferenca: totalDiferenca,
			Count:          len(result),
		})
	}
}
