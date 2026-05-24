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
	ChaveNFe      string   `json:"chave_nfe"`
	DataEmissao   string   `json:"data_emissao"`
	NumeroNFe     string   `json:"numero_nfe"`
	FornCNPJ      string   `json:"forn_cnpj"`
	FornNome      string   `json:"forn_nome"`
	FornUF        string   `json:"forn_uf"`
	FornSimples   bool     `json:"forn_simples"`
	CFOP          string   `json:"cfop"`
	Regime        string   `json:"regime"`
	NItem         int      `json:"n_item"`
	CProd         string   `json:"c_prod"`
	XProd         string   `json:"x_prod"`
	NCM           string   `json:"ncm"`
	CEST          string   `json:"cest"`
	VProdItem     float64  `json:"v_prod_item"`
	VIpiItem      float64  `json:"v_ipi_item"`
	VOutroRateado float64  `json:"v_outro_rateado"`
	VOperacao     float64  `json:"v_operacao"`
	VIcmsItem     float64  `json:"v_icms_item"`
	AliqInter     float64  `json:"aliq_inter"`
	AliqInterna   float64  `json:"aliq_interna"`
	BC            float64  `json:"bc"`
	IcmsCalculado float64  `json:"icms_calculado"`
	IcmsRetido    float64  `json:"icms_retido"`
	MvaOriginal   *float64 `json:"mva_original"`
	BcSt          float64  `json:"bc_st"`
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
        -- G4: frete CIF rateado proporcional ao valor do produto
        CASE
            WHEN COALESCE(ne.v_prod, 0) > 0
            THEN ROUND(COALESCE(ne.v_frete, 0) * COALESCE(nii.v_prod, 0) / ne.v_prod, 2)
            ELSE 0
        END                                                             AS v_frete_rateado,
        COALESCE(nii.v_icms, 0)                                         AS v_icms_item,
        CASE
            WHEN COALESCE(nii.cst_orig, ne.cst_orig_pred) IN ('1','2','3','6','7','8') THEN 4.0
            WHEN ne.forn_uf = ANY(ARRAY['PR','RS','SC','MG','RJ','SP'])                 THEN 7.0
            ELSE 12.0
        END                                                             AS aliq_inter,
        COALESCE(regra.aliquota_interna, 20.5)                          AS aliq_interna,
        regra.mva_original                                               AS mva_original,
        -- G2: MVA ajustado pelas alíquotas pré-calculadas + fallback Convênio ICMS 110/07
        regra.mva_ajustado_4pct                                          AS mva_aj_4pct,
        regra.mva_ajustado_7pct                                          AS mva_aj_7pct,
        regra.mva_ajustado_12pct                                         AS mva_aj_12pct,
        -- G3: redução de BC (ex.: medicamentos)
        COALESCE(regra.reducao_bc_pct, 0)                                AS reducao_bc_pct,
        COALESCE(ne.v_prod, 0)                                          AS v_prod_nf_total,
        COALESCE(ne.v_st, 0)                                            AS v_st_nf_total,
        COALESCE(ne.forn_uf, '')                                        AS forn_uf_raw
    FROM nfe_entradas ne
    INNER JOIN nfe_entradas_itens nii ON nii.nfe_id = ne.id
    LEFT JOIN forn_simples fs ON fs.cnpj = ne.forn_cnpj
    LEFT JOIN LATERAL (
        -- G1: filtro por uf_estado evita aplicar regra de PE em nota destinada à BA/CE
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
      AND ne.forn_uf IS NOT NULL
      AND ne.forn_uf != ''
      AND ne.forn_uf != COALESCE(ne.dest_uf, 'PE')
      AND ne.cfop = ANY(ARRAY['2101','2102','2152','2403','2409','2651','2652','2551','2556'])
      AND ($3::text = '' OR (
          EXTRACT(MONTH FROM ne.data_emissao)::int = SPLIT_PART($3::text,'/',1)::int
          AND EXTRACT(YEAR  FROM ne.data_emissao)::int = SPLIT_PART($3::text,'/',2)::int
      ))
), computed AS (
    SELECT
        chave_nfe, data_emissao, numero_nfe, forn_cnpj, forn_nome, forn_uf,
        forn_simples, cfop, regime,
        n_item, c_prod, x_prod, ncm, cest,
        v_prod_item, v_ipi_item, v_outro_rateado, v_frete_rateado,
        -- G4: v_operacao agora inclui v_frete rateado
        (v_prod_item + v_ipi_item + v_outro_rateado + v_frete_rateado)  AS v_operacao,
        v_icms_item, aliq_inter, aliq_interna, mva_original,
        reducao_bc_pct,
        -- G2: MVA efetivo: preferência por mva_ajustado pré-calculado; fallback fórmula Convênio 110/07; último fallback mva_original
        COALESCE(
            CASE
                WHEN aliq_inter = 4.0  THEN mva_aj_4pct
                WHEN aliq_inter = 7.0  THEN mva_aj_7pct
                WHEN aliq_inter = 12.0 THEN mva_aj_12pct
            END,
            CASE WHEN mva_original IS NOT NULL AND aliq_interna < 100 THEN
                ((1.0 + mva_original/100.0) * (1.0 - aliq_inter/100.0)
                 / NULLIF(1.0 - aliq_interna/100.0, 0) - 1.0) * 100.0
            END,
            mva_original
        )                                                                AS mva_efetivo,
        -- BC: PE antecipação usa preço presumido (gross-up) APENAS quando fornecedor NÃO é Simples Nacional (G5).
        -- BA/CE, DIFAL e Simples Nacional usam v_operacao direta. G3: aplicar redução de BC sobre o resultado.
        CASE
            WHEN regime = 'ANTECIPACAO' AND forn_uf_raw NOT IN ('BA','CE') AND NOT forn_simples
                THEN GREATEST(0,
                    (v_prod_item + v_ipi_item + v_outro_rateado + v_frete_rateado - v_icms_item)
                    / NULLIF(1.0 - aliq_interna / 100.0, 0))
                    * (1.0 - reducao_bc_pct/100.0)
            ELSE (v_prod_item + v_ipi_item + v_outro_rateado + v_frete_rateado)
                 * (1.0 - reducao_bc_pct/100.0)
        END                                                             AS bc,
        -- BC-ST via MVA efetivo: base presumida de varejo para substituição tributária (G2 + G3 + G4)
        CASE
            WHEN regime = 'ST' AND COALESCE(
                CASE WHEN aliq_inter = 4.0  THEN mva_aj_4pct
                     WHEN aliq_inter = 7.0  THEN mva_aj_7pct
                     WHEN aliq_inter = 12.0 THEN mva_aj_12pct
                END,
                CASE WHEN mva_original IS NOT NULL AND aliq_interna < 100 THEN
                    ((1.0 + mva_original/100.0) * (1.0 - aliq_inter/100.0)
                     / NULLIF(1.0 - aliq_interna/100.0, 0) - 1.0) * 100.0
                END,
                mva_original
            ) IS NOT NULL
                THEN ROUND(
                    (v_prod_item + v_ipi_item + v_outro_rateado + v_frete_rateado)
                    * (1.0 + COALESCE(
                        CASE WHEN aliq_inter = 4.0  THEN mva_aj_4pct
                             WHEN aliq_inter = 7.0  THEN mva_aj_7pct
                             WHEN aliq_inter = 12.0 THEN mva_aj_12pct
                        END,
                        CASE WHEN mva_original IS NOT NULL AND aliq_interna < 100 THEN
                            ((1.0 + mva_original/100.0) * (1.0 - aliq_inter/100.0)
                             / NULLIF(1.0 - aliq_interna/100.0, 0) - 1.0) * 100.0
                        END,
                        mva_original
                    )/100.0)
                    * (1.0 - reducao_bc_pct/100.0), 2)
            ELSE 0
        END                                                             AS bc_st,
        CASE
            WHEN regime = 'ST' AND v_prod_nf_total > 0
                THEN ROUND(v_st_nf_total * v_prod_item / v_prod_nf_total, 2)
            ELSE 0
        END                                                             AS icms_retido
    FROM base
)
-- G14: window functions retornam totais do conjunto completo (sem LIMIT).
SELECT
    chave_nfe, data_emissao, numero_nfe, forn_cnpj, forn_nome, forn_uf,
    forn_simples, cfop, regime,
    n_item, c_prod, x_prod, ncm, cest,
    v_prod_item, v_ipi_item, v_outro_rateado, v_operacao, v_icms_item,
    aliq_inter, aliq_interna, bc,
    CASE
        WHEN regime = 'ST' AND mva_efetivo IS NOT NULL
            THEN GREATEST(0, ROUND(bc_st * aliq_interna/100.0 - v_operacao * aliq_inter/100.0, 2))
        WHEN regime = 'ST' THEN icms_retido
        ELSE GREATEST(0, bc * (aliq_interna - aliq_inter) / 100.0)
    END                                                                 AS icms_calculado,
    icms_retido,
    mva_original,
    bc_st,
    COUNT(*) OVER ()                                                    AS total_count,
    SUM(
        CASE
            WHEN regime = 'ST' AND mva_efetivo IS NOT NULL
                THEN GREATEST(0, ROUND(bc_st * aliq_interna/100.0 - v_operacao * aliq_inter/100.0, 2))
            WHEN regime = 'ST' THEN icms_retido
            ELSE GREATEST(0, bc * (aliq_interna - aliq_inter) / 100.0)
        END
    ) OVER ()                                                           AS total_full
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
		periodo := r.URL.Query().Get("periodo")

		rows, err := db.Query(fronteiraItensQuery, companyID, regime, periodo)
		if err != nil {
			log.Printf("IcmsFronteiraItens error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar itens ICMS Fronteira")
			return
		}
		defer rows.Close()

		result := []FronteiraItemRow{}
		var totalFull float64
		var totalCount int

		for rows.Next() {
			var row FronteiraItemRow
			var mvaOrig sql.NullFloat64
			// G14: total_count/total_full via window functions retornam o agregado
			// do conjunto completo (sem o LIMIT 2000). Backend ainda limita as
			// linhas materializadas, mas o usuário vê o total real.
			var rowTotalCount sql.NullInt64
			var rowTotalFull sql.NullFloat64
			if err := rows.Scan(
				&row.ChaveNFe, &row.DataEmissao, &row.NumeroNFe,
				&row.FornCNPJ, &row.FornNome, &row.FornUF,
				&row.FornSimples, &row.CFOP, &row.Regime,
				&row.NItem, &row.CProd, &row.XProd, &row.NCM, &row.CEST,
				&row.VProdItem, &row.VIpiItem, &row.VOutroRateado, &row.VOperacao, &row.VIcmsItem,
				&row.AliqInter, &row.AliqInterna, &row.BC,
				&row.IcmsCalculado, &row.IcmsRetido,
				&mvaOrig, &row.BcSt,
				&rowTotalCount, &rowTotalFull,
			); err != nil {
				log.Printf("IcmsFronteiraItens scan error: %v", err)
				continue
			}
			if mvaOrig.Valid {
				row.MvaOriginal = &mvaOrig.Float64
			}
			if rowTotalCount.Valid {
				totalCount = int(rowTotalCount.Int64)
			}
			if rowTotalFull.Valid {
				totalFull = rowTotalFull.Float64
			}
			result = append(result, row)
		}

		json.NewEncoder(w).Encode(FronteiraItensResponse{
			Rows:  result,
			Total: totalFull,
			Count: totalCount,
		})
	}
}
