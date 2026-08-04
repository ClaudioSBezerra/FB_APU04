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
// Fonte: SPED (reg_c190) classifica o regime pelo CFOP de entrada; o detalhe de
// item (NCM/CEST/produto p/ regra MVA) vem do XML por chave NF-e. Notas no SPED
// sem XML correspondente não aparecem na planilha de itens (sem detalhe), mas
// entram nos totais nota-nível das outras abas. aliq_inter = alíquota real do SPED.
const fronteiraItensQueryBody = `
WITH sped_class AS (
    SELECT DISTINCT ON (c100.chv_nfe)
        c100.chv_nfe                                  AS chave_nfe,
        c100.dt_doc::text                             AS data_emissao,
        COALESCE(c100.num_doc, '')                    AS numero_nfe,
        c100.cod_part                                 AS cod_part,
        c100.job_id                                   AS job_id,
        c190.cfop                                     AS cfop,
        COALESCE(NULLIF(c190.aliq_icms, 0), 12.0)     AS aliq_inter,
        CASE
            WHEN c190.cfop IN ('2551','2556')               THEN 'DIFAL'
            WHEN c190.cfop IN ('2403','2409','2651','2652') THEN 'ST'
            WHEN c190.cfop IN ('2101','2102','2152')        THEN 'ANTECIPACAO'
        END                                           AS regime
    FROM reg_c190 c190
    JOIN reg_c100 c100 ON c100.id = c190.id_pai_c100
    JOIN import_jobs j ON j.id = c100.job_id
    WHERE j.company_id = $1
      AND c100.cod_sit NOT IN ('02','03','04','05')
      AND c190.cfop = ANY(ARRAY['2101','2102','2152','2403','2409','2651','2652','2551','2556'])
      AND ($3::text = '' OR (
          EXTRACT(MONTH FROM c100.dt_doc)::int = SPLIT_PART($3::text,'/',1)::int
          AND EXTRACT(YEAR  FROM c100.dt_doc)::int = SPLIT_PART($3::text,'/',2)::int
      ))
    ORDER BY c100.chv_nfe, c190.vl_opr DESC NULLS LAST
), base AS (
    SELECT
        sc.chave_nfe,
        sc.data_emissao,
        sc.numero_nfe,
        COALESCE(part.cnpj, ne.forn_cnpj, '')                          AS forn_cnpj,
        COALESCE(part.nome, ne.forn_nome, '')                          AS forn_nome,
        COALESCE(ne.forn_uf, '')                                        AS forn_uf,
        COALESCE(ne.dest_uf, '')                                        AS dest_uf,
        (fs.cnpj IS NOT NULL)                                           AS forn_simples,
        sc.cfop                                                          AS cfop,
        sc.regime                                                       AS regime,
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
        sc.aliq_inter                                                   AS aliq_inter,
        COALESCE(regra.aliquota_interna, 20.5)                          AS aliq_interna,
        regra.mva_original                                               AS mva_original,
        -- G2: MVA ajustado pré-calculado + fallback Convênio ICMS 110/07
        regra.mva_ajustado_4pct                                          AS mva_aj_4pct,
        regra.mva_ajustado_7pct                                          AS mva_aj_7pct,
        regra.mva_ajustado_12pct                                         AS mva_aj_12pct,
        -- G3: redução de BC (ex.: medicamentos)
        COALESCE(regra.reducao_bc_pct, 0)                                AS reducao_bc_pct,
        COALESCE(ne.v_prod, 0)                                          AS v_prod_nf_total,
        COALESCE(ne.v_st, 0)                                            AS v_st_nf_total,
        COALESCE(ne.forn_uf, '')                                        AS forn_uf_raw,
        COALESCE(ub.base_por_dentro, false)                             AS base_por_dentro
    FROM sped_class sc
    JOIN nfe_entradas ne ON ne.chave_nfe = sc.chave_nfe
    INNER JOIN nfe_entradas_itens nii ON nii.nfe_id = ne.id
    LEFT JOIN participants part ON part.job_id = sc.job_id AND part.cod_part = sc.cod_part
    LEFT JOIN forn_simples fs ON fs.cnpj = ne.forn_cnpj
    -- Parâmetro "base por dentro" (gross-up) é configurável por UF de destino
    -- (aba UFs → Benefícios), não mais fixo por UF do fornecedor.
    LEFT JOIN uf_beneficios_fiscais ub ON ub.company_id = $1 AND ub.uf = ne.dest_uf
    LEFT JOIN LATERAL (
        -- G1: filtra pela UF de destino real da nota — cada UF só casa com suas
        -- próprias regras (sem fallback silencioso para outra UF quando nula).
        SELECT r.aliquota_interna, r.mva_original,
               r.mva_ajustado_4pct, r.mva_ajustado_7pct, r.mva_ajustado_12pct,
               r.reducao_bc_pct
        FROM icms_fronteira_regras_ncm r
        WHERE (r.company_id = $1 OR r.company_id IS NULL)
          AND r.uf_estado = ne.dest_uf
          AND nii.ncm IS NOT NULL
          AND LEFT(nii.ncm, LENGTH(r.ncm_prefixo)) = r.ncm_prefixo
        ORDER BY r.company_id NULLS LAST, LENGTH(r.ncm_prefixo) DESC
        LIMIT 1
    ) regra ON true
), computed AS (
    SELECT
        chave_nfe, data_emissao, numero_nfe, forn_cnpj, forn_nome, forn_uf,
        dest_uf,
        forn_simples, cfop, regime,
        n_item, c_prod, x_prod, ncm, cest,
        v_prod_item, v_ipi_item, v_outro_rateado, v_frete_rateado,
        -- G4: v_operacao agora inclui v_frete rateado
        (v_prod_item + v_ipi_item + v_outro_rateado + v_frete_rateado)  AS v_operacao,
        v_icms_item, aliq_inter, aliq_interna, mva_original,
        reducao_bc_pct, base_por_dentro,
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
        -- BC: antecipação usa preço presumido (gross-up) quando a UF de DESTINO tem
        -- base_por_dentro=true (uf_beneficios_fiscais) e o fornecedor NÃO é Simples
        -- Nacional (G5). Demais casos (UF sem gross-up, DIFAL, Simples) usam
        -- v_operacao direta. G3: aplicar redução de BC sobre o resultado.
        CASE
            WHEN regime = 'ANTECIPACAO' AND base_por_dentro AND NOT forn_simples
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
  AND ($4::text = '' OR dest_uf = $4::text)
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
		uf := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("uf")))

		rows, err := db.Query(fronteiraItensQuery, companyID, regime, periodo, uf)
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
