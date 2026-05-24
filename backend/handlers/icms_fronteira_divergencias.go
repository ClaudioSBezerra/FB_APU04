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
WITH item_base AS (
    -- Pré-calcula campos comuns (rateios, alíquotas, MVA efetivo, redução BC) para evitar
    -- duplicação de CASEs gigantes no SELECT final. G1+G2+G3+G4+G5 ficam centralizados aqui.
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
        COALESCE(nii.v_prod, 0)        AS v_prod_item,
        COALESCE(nii.v_ipi, 0)         AS v_ipi_item,
        COALESCE(nii.v_icms, 0)        AS v_icms_item,
        -- G4: rateio de v_outro e v_frete por valor do produto
        CASE WHEN COALESCE(ne.v_prod, 0) > 0
            THEN ROUND(COALESCE(ne.v_outro, 0) * COALESCE(nii.v_prod, 0) / ne.v_prod, 2)
            ELSE 0
        END                            AS v_outro_rateado,
        CASE WHEN COALESCE(ne.v_prod, 0) > 0
            THEN ROUND(COALESCE(ne.v_frete, 0) * COALESCE(nii.v_prod, 0) / ne.v_prod, 2)
            ELSE 0
        END                            AS v_frete_rateado,
        CASE
            WHEN COALESCE(nii.cst_orig, ne.cst_orig_pred) IN ('1','2','3','6','7','8') THEN 4.0
            WHEN ne.forn_uf = ANY(ARRAY['PR','RS','SC','MG','RJ','SP'])                THEN 7.0
            ELSE 12.0
        END                            AS aliq_inter,
        COALESCE(regra.aliquota_interna, 20.5)  AS aliq_interna,
        regra.mva_original,
        regra.mva_ajustado_4pct, regra.mva_ajustado_7pct, regra.mva_ajustado_12pct,
        COALESCE(regra.reducao_bc_pct, 0)       AS reducao_bc_pct,
        (fs.cnpj IS NOT NULL)                   AS forn_simples,
        COALESCE(ne.v_prod, 0)                  AS v_prod_nf_total,
        COALESCE(ne.v_st, 0)                    AS v_st_nf_total
    FROM nfe_entradas ne
    INNER JOIN nfe_entradas_itens nii ON nii.nfe_id = ne.id
    LEFT JOIN forn_simples fs ON fs.cnpj = ne.forn_cnpj
    LEFT JOIN LATERAL (
        -- G1: filtrar pela UF de destino da nota — evita aplicar regra de PE em BA/CE
        SELECT r.aliquota_interna, r.mva_original,
               r.mva_ajustado_4pct, r.mva_ajustado_7pct, r.mva_ajustado_12pct,
               r.reducao_bc_pct
        FROM icms_fronteira_regras_ncm r
        WHERE (r.company_id = $1 OR r.company_id IS NULL)
          AND r.uf_estado = COALESCE(ne.dest_uf, 'PE')
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
), item_icms AS (
    SELECT
        chave_nfe, data_emissao, numero_nfe, forn_cnpj, forn_nome, forn_uf, cfop, regime,
        -- G2: MVA efetivo (pré-calculado ou Convênio 110/07 ou original)
        COALESCE(
            CASE WHEN aliq_inter = 4.0  THEN mva_ajustado_4pct
                 WHEN aliq_inter = 7.0  THEN mva_ajustado_7pct
                 WHEN aliq_inter = 12.0 THEN mva_ajustado_12pct
            END,
            CASE WHEN mva_original IS NOT NULL AND aliq_interna < 100 THEN
                ((1.0 + mva_original/100.0) * (1.0 - aliq_inter/100.0)
                 / NULLIF(1.0 - aliq_interna/100.0, 0) - 1.0) * 100.0
            END,
            mva_original
        )                                       AS mva_efetivo,
        -- v_operacao com v_frete (G4)
        (v_prod_item + v_ipi_item + v_outro_rateado + v_frete_rateado) AS v_operacao,
        v_icms_item, aliq_inter, aliq_interna, reducao_bc_pct,
        forn_simples, v_prod_nf_total, v_st_nf_total
    FROM item_base
), item_calc AS (
    SELECT
        chave_nfe, data_emissao, numero_nfe, forn_cnpj, forn_nome, forn_uf, cfop, regime,
        -- ICMS por item conforme regime, aplicando G1/G2/G3/G4/G5
        CASE
            -- ST: usa MVA efetivo se disponível, senão rateia v_st da nota
            WHEN regime = 'ST' AND mva_efetivo IS NOT NULL THEN
                GREATEST(0, ROUND(
                    v_operacao * (1.0 + mva_efetivo/100.0) * (1.0 - reducao_bc_pct/100.0)
                    * aliq_interna/100.0
                    - v_operacao * aliq_inter/100.0
                , 2))
            WHEN regime = 'ST' AND v_prod_nf_total > 0 THEN
                ROUND(v_st_nf_total * v_prod_item / v_prod_nf_total, 2)
            -- ANTECIPACAO PE (não Simples): gross-up sobre BC com redução
            WHEN regime = 'ANTECIPACAO' AND forn_uf NOT IN ('BA','CE') AND NOT forn_simples THEN
                GREATEST(0, ROUND(
                    ((v_operacao - v_icms_item) / NULLIF(1.0 - aliq_interna/100.0, 0))
                    * (1.0 - reducao_bc_pct/100.0)
                    * (aliq_interna - aliq_inter) / 100.0
                , 2))
            -- DIFAL / Antecipação BA/CE / Antecipação de Simples: BC direta com redução
            ELSE
                GREATEST(0, ROUND(
                    v_operacao * (1.0 - reducao_bc_pct/100.0)
                    * (aliq_interna - aliq_inter) / 100.0
                , 2))
        END AS icms_item,
        v_prod_item, v_prod_nf_total
    FROM item_icms
),
item_icms_final AS (
    SELECT chave_nfe, data_emissao, numero_nfe, forn_cnpj, forn_nome, forn_uf, cfop, regime, icms_item
    FROM item_calc
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
    FROM item_icms_final
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
-- G14: window functions retornam totais do conjunto completo, evitando que
-- o LIMIT 1000 quebre os agregados exibidos no rodapé da divergências.
SELECT
    chave_nfe, periodo, numero_nf, forn_cnpj, forn_nome, forn_uf,
    data_emissao, regime,
    icms_sefaz, icms_calculado, diferenca, status,
    COUNT(*)         OVER () AS total_count,
    SUM(icms_sefaz)  OVER () AS total_sefaz_full,
    SUM(icms_calculado) OVER () AS total_calc_full,
    SUM(diferenca)   OVER () AS total_dif_full
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
		var totalCount int

		for rows.Next() {
			var row DivergenciaRow
			// G14: window functions devolvem o agregado do conjunto completo,
			// independente do LIMIT 1000 que controla apenas as linhas materializadas.
			var rowTotalCount sql.NullInt64
			var rowTotalSefaz, rowTotalCalc, rowTotalDif sql.NullFloat64
			if err := rows.Scan(
				&row.ChaveNFe, &row.Periodo, &row.NumeroNF,
				&row.FornCNPJ, &row.FornNome, &row.FornUF,
				&row.DataEmissao, &row.Regime,
				&row.IcmsSefaz, &row.IcmsCalculado, &row.Diferenca, &row.Status,
				&rowTotalCount, &rowTotalSefaz, &rowTotalCalc, &rowTotalDif,
			); err != nil {
				log.Printf("IcmsFronteiraDivergencias scan error: %v", err)
				continue
			}
			if rowTotalCount.Valid {
				totalCount = int(rowTotalCount.Int64)
			}
			if rowTotalSefaz.Valid {
				totalSefaz = rowTotalSefaz.Float64
			}
			if rowTotalCalc.Valid {
				totalCalculado = rowTotalCalc.Float64
			}
			if rowTotalDif.Valid {
				totalDiferenca = rowTotalDif.Float64
			}
			result = append(result, row)
		}

		json.NewEncoder(w).Encode(DivergenciasResponse{
			Rows:           result,
			TotalSefaz:     totalSefaz,
			TotalCalculado: totalCalculado,
			TotalDiferenca: totalDiferenca,
			Count:          totalCount,
		})
	}
}
