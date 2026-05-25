package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// fronteiraFiltros monta o WHERE adicional (fornecedor, número da nota, intervalo
// de data) a partir dos query params, com placeholders posicionais a partir de
// startIdx. Retorna o fragmento SQL (começando com " AND ...") e os argumentos.
// As colunas referenciadas (forn_cnpj/forn_nome/numero_nfe/data_emissao) existem
// no CTE classified do fronteiraBaseQuery.
func fronteiraFiltros(r *http.Request, startIdx int) (string, []interface{}) {
	var sb strings.Builder
	var args []interface{}
	idx := startIdx

	if forn := strings.TrimSpace(r.URL.Query().Get("forn")); forn != "" {
		sb.WriteString(fmt.Sprintf(" AND (forn_cnpj ILIKE $%d OR forn_nome ILIKE $%d)", idx, idx))
		args = append(args, "%"+forn+"%")
		idx++
	}
	if num := strings.TrimSpace(r.URL.Query().Get("num_nota")); num != "" {
		sb.WriteString(fmt.Sprintf(" AND numero_nfe ILIKE $%d", idx))
		args = append(args, "%"+num+"%")
		idx++
	}
	if di := strings.TrimSpace(r.URL.Query().Get("data_ini")); di != "" {
		sb.WriteString(fmt.Sprintf(" AND data_emissao::date >= $%d::date", idx))
		args = append(args, di)
		idx++
	}
	if df := strings.TrimSpace(r.URL.Query().Get("data_fim")); df != "" {
		sb.WriteString(fmt.Sprintf(" AND data_emissao::date <= $%d::date", idx))
		args = append(args, df)
		idx++
	}
	return sb.String(), args
}

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

// CFOPs conforme especificação do contador (Bloco 1):
//
//   Antecipação sem liberação : 2101, 2102, 2152
//   Antecipação com liberação (ST): 2403, 2409, 2651, 2652
//   Uso/consumo/ativo imobilizado (DIFAL): 2551, 2556
//
// Outros CFOPs são excluídos do cálculo de fronteira.

// fronteiraAllCFOPs contém todos os CFOPs válidos para qualquer regime de fronteira.
var fronteiraAllCFOPs = []string{
	"2101", "2102", "2152",           // Antecipação sem liberação
	"2403", "2409", "2651", "2652",   // ST (antecipação com liberação)
	"2551", "2556",                    // DIFAL
}

// Sul/Sudeste states subject to 7% interestadual rate (ES and MT excluded per legislação).
var sulSudesteUF = map[string]bool{
	"PR": true, "RS": true, "SC": true,
	"MG": true, "RJ": true, "SP": true,
}

// aliqInterestadual retorna a alíquota interestadual aplicável.
// cstOrig: código da Tabela A do CST (origem da mercadoria); uf: UF do fornecedor.
// Origens 1,2,3,6,7,8 → mercadoria estrangeira/alto conteúdo importado → 4% (Res. Senado 13/2012).
func aliqInterestadual(cstOrig, uf string) float64 {
	switch cstOrig {
	case "1", "2", "3", "6", "7", "8":
		return 4.0
	}
	if sulSudesteUF[strings.ToUpper(strings.TrimSpace(uf))] {
		return 7.0
	}
	return 12.0
}

// ---------------------------------------------------------------------------
// Structs — Resumo
// ---------------------------------------------------------------------------

type FronteiraResumoRow struct {
	Regime         string  `json:"regime"`
	QtdNotas       int     `json:"qtd_notas"`
	VProdTotal     float64 `json:"v_prod_total"`
	VIpiTotal      float64 `json:"v_ipi_total"`
	VStRetido      float64 `json:"v_st_retido"`
	IcmsDevidoEst  float64 `json:"icms_devido_est"`
}

type FronteiraResumoResponse struct {
	Rows         []FronteiraResumoRow `json:"rows"`
	TotalDevido  float64              `json:"total_devido"`
	TotalProd    float64              `json:"total_prod"`
}

// ---------------------------------------------------------------------------
// Structs — Notas (shared across Antecipação / ST / DIFAL tabs)
// ---------------------------------------------------------------------------

type FronteiraNotaRow struct {
	ChaveNFe      string  `json:"chave_nfe"`
	DataEmissao   string  `json:"data_emissao"`
	NumeroNFe     string  `json:"numero_nfe"`
	FornCNPJ      string  `json:"forn_cnpj"`
	FornNome      string  `json:"forn_nome"`
	FornUF        string  `json:"forn_uf"`
	CFOP          string  `json:"cfop"`
	VProd         float64 `json:"v_prod"`
	VIPI          float64 `json:"v_ipi"`
	VIcms         float64 `json:"v_icms"`
	VBcST         float64 `json:"v_bc_st"`
	VST           float64 `json:"v_st"`
	AliqInter     float64 `json:"aliq_inter"`
	AliqInterna   float64 `json:"aliq_interna"`
	IcmsDevidoEst float64 `json:"icms_devido_est"`
	Regime        string  `json:"regime"`
	Bloco         string  `json:"bloco"`
}

type FronteiraNotasResponse struct {
	Rows             []FronteiraNotaRow `json:"rows"`
	Total            float64            `json:"total"`
	Count            int                `json:"count"`
	TotalMesAtual    float64            `json:"total_mes_atual"`
	TotalMesAnterior float64            `json:"total_mes_anterior"`
	CountMesAtual    int                `json:"count_mes_atual"`
	CountMesAnterior int                `json:"count_mes_anterior"`
}

// ---------------------------------------------------------------------------
// SQL helpers
// ---------------------------------------------------------------------------

// baseQuery returns the common SELECT that classifies each nota and computes
// the estimated ICMS due. Caller appends a WHERE clause for the regime filter
// and the $1 company_id placeholder.
// Fonte: SPED Fiscal (reg_c190 → reg_c100 → import_jobs). O CFOP de entrada
// (2xxx) que classifica o regime de fronteira só existe no SPED — o XML traz o
// CFOP de saída do fornecedor (6xxx). Detalhe de NCM (p/ regra MVA) vem do XML
// via join por chave. aliq_icms e vl_icms do SPED são os valores reais da
// operação interestadual (não estimados por CST de origem).
const fronteiraBaseQuery = `
WITH classified AS (
    SELECT
        c100.chv_nfe                                        AS chave_nfe,
        c100.dt_doc::text                                   AS data_emissao,
        COALESCE(c100.num_doc, '')                          AS numero_nfe,
        COALESCE(part.cnpj, ne.forn_cnpj, '')               AS forn_cnpj,
        COALESCE(part.nome, ne.forn_nome, '')               AS forn_nome,
        COALESCE(ne.forn_uf, '')                            AS forn_uf,
        c190.cfop                                           AS cfop,
        COALESCE(c190.vl_opr, 0)                            AS v_prod,
        COALESCE(ipi_calc.v, 0)                             AS v_ipi,
        COALESCE(c190.vl_icms, 0)                           AS v_icms,
        COALESCE(c190.vl_bc_icms_st, 0)                     AS v_bc_st,
        COALESCE(c190.vl_icms_st, 0)                        AS v_st,
        -- Alíquota interestadual: real do SPED (aliq_icms); fallback 12% se ausente.
        COALESCE(NULLIF(c190.aliq_icms, 0), 12.0)           AS aliq_inter,
        COALESCE(regra.aliquota_interna, 20.5)              AS aliq_interna,
        CASE
            WHEN c190.cfop IN ('2551','2556')
                THEN 'DIFAL'
            WHEN c190.cfop IN ('2403','2409','2651','2652')
                THEN 'ST'
            WHEN c190.cfop IN ('2101','2102','2152')
                THEN 'ANTECIPACAO'
        END                                                 AS regime,
        CASE
            WHEN $2::text = ''
              OR (EXTRACT(MONTH FROM c100.dt_doc)::int = SPLIT_PART($2::text,'/',1)::int
                  AND EXTRACT(YEAR  FROM c100.dt_doc)::int = SPLIT_PART($2::text,'/',2)::int)
            THEN 'mes_atual'
            ELSE 'mes_anterior'
        END                                                 AS bloco,
        -- ICMS devido estimado por regime (nota-nível; detalhe item no Bloco 2)
        CASE
            WHEN c190.cfop IN ('2551','2556')
                THEN GREATEST(0,
                    (COALESCE(c190.vl_opr, 0) + COALESCE(ipi_calc.v, 0)) * (
                        COALESCE(regra.aliquota_interna, 20.5)
                        - COALESCE(NULLIF(c190.aliq_icms, 0), 12.0)
                    ) / 100.0)
            WHEN c190.cfop IN ('2403','2409','2651','2652')
                THEN CASE
                    -- MVA efetivo: ajustado pré-calc por alíquota interestadual real,
                    -- fallback Convênio 110/07 a partir do MVA original, fallback MVA original.
                    WHEN COALESCE(
                        CASE COALESCE(NULLIF(c190.aliq_icms,0),12.0)
                            WHEN 4.0  THEN regra.mva_ajustado_4pct
                            WHEN 7.0  THEN regra.mva_ajustado_7pct
                            WHEN 12.0 THEN regra.mva_ajustado_12pct
                        END,
                        CASE WHEN regra.mva_original IS NOT NULL AND COALESCE(regra.aliquota_interna,20.5) < 100 THEN
                            ((1.0 + regra.mva_original/100.0) * (1.0 - COALESCE(NULLIF(c190.aliq_icms,0),12.0)/100.0)
                             / NULLIF(1.0 - COALESCE(regra.aliquota_interna,20.5)/100.0, 0) - 1.0) * 100.0
                        END,
                        regra.mva_original
                    ) IS NOT NULL
                        THEN GREATEST(0,
                            (COALESCE(c190.vl_opr, 0) + COALESCE(ipi_calc.v, 0))
                            * (1.0 + COALESCE(
                                CASE COALESCE(NULLIF(c190.aliq_icms,0),12.0)
                                    WHEN 4.0  THEN regra.mva_ajustado_4pct
                                    WHEN 7.0  THEN regra.mva_ajustado_7pct
                                    WHEN 12.0 THEN regra.mva_ajustado_12pct
                                END,
                                CASE WHEN regra.mva_original IS NOT NULL AND COALESCE(regra.aliquota_interna,20.5) < 100 THEN
                                    ((1.0 + regra.mva_original/100.0) * (1.0 - COALESCE(NULLIF(c190.aliq_icms,0),12.0)/100.0)
                                     / NULLIF(1.0 - COALESCE(regra.aliquota_interna,20.5)/100.0, 0) - 1.0) * 100.0
                                END,
                                regra.mva_original
                            )/100.0)
                            * COALESCE(regra.aliquota_interna, 20.5)/100.0
                            - COALESCE(c190.vl_icms, 0))
                    ELSE COALESCE(c190.vl_icms_st, 0)
                END
            WHEN c190.cfop IN ('2101','2102','2152')
                THEN GREATEST(0,
                    (COALESCE(c190.vl_opr, 0) + COALESCE(ipi_calc.v, 0)) * COALESCE(regra.aliquota_interna, 20.5)/100.0
                    - COALESCE(c190.vl_icms, 0))
            ELSE 0
        END                                                 AS icms_devido_est
    FROM reg_c190 c190
    JOIN reg_c100 c100 ON c100.id = c190.id_pai_c100
    JOIN import_jobs j ON j.id = c100.job_id
    LEFT JOIN participants part
        ON part.job_id = c100.job_id AND part.cod_part = c100.cod_part
    LEFT JOIN nfe_entradas ne ON ne.chave_nfe = c100.chv_nfe
    -- IPI por linha c190: SÓ considera IPI quando há XML. O XML é por item e o
    -- c190 é agregado por CFOP, então o IPI total da nota (somado do XML) é
    -- prorateado pela participação desta linha no valor de operação da nota
    -- (correto mesmo em notas multi-CFOP).
    -- SEM XML (somente SPED): IPI = 0. No SPED o vl_opr do total da nota já
    -- embute o IPI; somá-lo de novo causaria dupla contagem. Por isso NÃO se
    -- usa c190.vl_ipi como fallback aqui.
    LEFT JOIN LATERAL (
        SELECT CASE
            WHEN x.nota_ipi_xml > 0 AND o.nota_opr > 0
                THEN x.nota_ipi_xml * COALESCE(c190.vl_opr, 0) / o.nota_opr
            ELSE 0
        END AS v
        FROM (
            SELECT COALESCE(SUM(nii.v_ipi), 0) AS nota_ipi_xml
            FROM nfe_entradas_itens nii WHERE nii.nfe_id = ne.id
        ) x
        CROSS JOIN (
            SELECT COALESCE(SUM(c190b.vl_opr), 0) AS nota_opr
            FROM reg_c190 c190b WHERE c190b.id_pai_c100 = c100.id
        ) o
    ) ipi_calc ON true
    LEFT JOIN LATERAL (
        SELECT nii.ncm AS ncm
        FROM nfe_entradas_itens nii
        WHERE nii.nfe_id = ne.id
        ORDER BY nii.v_prod DESC NULLS LAST
        LIMIT 1
    ) top_item ON true
    LEFT JOIN LATERAL (
        SELECT r.aliquota_interna, r.mva_original,
               r.mva_ajustado_4pct, r.mva_ajustado_7pct, r.mva_ajustado_12pct
        FROM icms_fronteira_regras_ncm r
        WHERE (r.company_id = $1 OR r.company_id IS NULL)
          AND r.uf_estado = COALESCE(ne.dest_uf, 'PE')
          AND top_item.ncm IS NOT NULL
          AND LEFT(top_item.ncm, LENGTH(r.ncm_prefixo)) = r.ncm_prefixo
        ORDER BY r.company_id NULLS LAST, LENGTH(r.ncm_prefixo) DESC
        LIMIT 1
    ) regra ON true
    WHERE j.company_id = $1
      AND c100.cod_sit NOT IN ('02','03','04','05')
      AND c190.cfop = ANY(ARRAY['2101','2102','2152','2403','2409','2651','2652','2551','2556'])
      AND ($2::text = '' OR j.mes_ano = $2
          OR (j.mes_ano IS NULL AND (
              EXTRACT(MONTH FROM j.dt_ini)::int = SPLIT_PART($2::text,'/',1)::int
              AND EXTRACT(YEAR  FROM j.dt_ini)::int = SPLIT_PART($2::text,'/',2)::int
          ))
      )
)
`

// ---------------------------------------------------------------------------
// IcmsFronteiraResumoHandler — GET /api/icms-fronteira/resumo
// ---------------------------------------------------------------------------

func IcmsFronteiraResumoHandler(db *sql.DB) http.HandlerFunc {
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

		periodo := r.URL.Query().Get("periodo")
		filtroSQL, filtroArgs := fronteiraFiltros(r, 3)

		query := fronteiraBaseQuery + `
SELECT
    regime,
    COUNT(DISTINCT chave_nfe) AS qtd_notas,
    SUM(v_prod)         AS v_prod_total,
    SUM(v_ipi)          AS v_ipi_total,
    SUM(v_st)           AS v_st_retido,
    SUM(icms_devido_est) AS icms_devido_est
FROM classified
WHERE regime IS NOT NULL` + filtroSQL + `
GROUP BY regime
ORDER BY regime
`
		args := append([]interface{}{companyID, periodo}, filtroArgs...)
		rows, err := db.Query(query, args...)
		if err != nil {
			log.Printf("IcmsFronteiraResumo error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar resumo ICMS Fronteira")
			return
		}
		defer rows.Close()

		result := []FronteiraResumoRow{}
		var totalDevido, totalProd float64

		for rows.Next() {
			var row FronteiraResumoRow
			if err := rows.Scan(
				&row.Regime, &row.QtdNotas, &row.VProdTotal, &row.VIpiTotal, &row.VStRetido, &row.IcmsDevidoEst,
			); err != nil {
				log.Printf("IcmsFronteiraResumo scan error: %v", err)
				continue
			}
			totalDevido += row.IcmsDevidoEst
			totalProd += row.VProdTotal
			result = append(result, row)
		}

		json.NewEncoder(w).Encode(FronteiraResumoResponse{
			Rows:        result,
			TotalDevido: totalDevido,
			TotalProd:   totalProd,
		})
	}
}

// ---------------------------------------------------------------------------
// notasHandler is the shared implementation for the three detail tabs.
// regime: "ANTECIPACAO" | "ST" | "DIFAL"
// ---------------------------------------------------------------------------

func fronteiraNotasHandler(db *sql.DB, w http.ResponseWriter, r *http.Request, regime string) {
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

	periodo := r.URL.Query().Get("periodo")
	filtroSQL, filtroArgs := fronteiraFiltros(r, 4)

	// G14: window functions retornam totais do conjunto completo (sem LIMIT),
	// resolvendo o bug onde totais exibidos só refletiam as primeiras 500 notas.
	// bloco classifica cada nota em "mes_atual" ou "mes_anterior" conforme dt_doc.
	query := fronteiraBaseQuery + `
SELECT
    chave_nfe, data_emissao, numero_nfe, forn_cnpj, forn_nome, forn_uf,
    cfop, v_prod, v_ipi, v_icms, v_bc_st, v_st,
    aliq_inter, aliq_interna, icms_devido_est, regime, bloco,
    COUNT(*)            OVER () AS total_count,
    SUM(icms_devido_est) OVER () AS total_full
FROM classified
WHERE regime = $3` + filtroSQL + `
ORDER BY bloco, data_emissao DESC, chave_nfe
LIMIT 500
`
	args := append([]interface{}{companyID, periodo, regime}, filtroArgs...)
	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("IcmsFronteiraNotas[%s] error: %v", regime, err)
		jsonErr(w, http.StatusInternalServerError, "Erro ao consultar notas ICMS Fronteira")
		return
	}
	defer rows.Close()

	result := []FronteiraNotaRow{}
	var totalFull float64
	var totalCount int
	var totalMesAtual, totalMesAnterior float64
	var countMesAtual, countMesAnterior int

	for rows.Next() {
		var row FronteiraNotaRow
		var rowTotalCount int
		var rowTotalFull sql.NullFloat64
		if err := rows.Scan(
			&row.ChaveNFe, &row.DataEmissao, &row.NumeroNFe,
			&row.FornCNPJ, &row.FornNome, &row.FornUF,
			&row.CFOP, &row.VProd, &row.VIPI, &row.VIcms, &row.VBcST, &row.VST,
			&row.AliqInter, &row.AliqInterna, &row.IcmsDevidoEst, &row.Regime,
			&row.Bloco,
			&rowTotalCount, &rowTotalFull,
		); err != nil {
			log.Printf("IcmsFronteiraNotas[%s] scan error: %v", regime, err)
			continue
		}
		totalCount = rowTotalCount
		if rowTotalFull.Valid {
			totalFull = rowTotalFull.Float64
		}
		if row.Bloco == "mes_atual" {
			totalMesAtual += row.IcmsDevidoEst
			countMesAtual++
		} else {
			totalMesAnterior += row.IcmsDevidoEst
			countMesAnterior++
		}
		result = append(result, row)
	}

	json.NewEncoder(w).Encode(FronteiraNotasResponse{
		Rows:             result,
		Total:            totalFull,
		Count:            totalCount,
		TotalMesAtual:    totalMesAtual,
		TotalMesAnterior: totalMesAnterior,
		CountMesAtual:    countMesAtual,
		CountMesAnterior: countMesAnterior,
	})
}

// ---------------------------------------------------------------------------
// IcmsFronteiraAntecipacaoHandler — GET /api/icms-fronteira/antecipacao
// ---------------------------------------------------------------------------

func IcmsFronteiraAntecipacaoHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		fronteiraNotasHandler(db, w, r, "ANTECIPACAO")
	}
}

// ---------------------------------------------------------------------------
// IcmsFronteiraSTHandler — GET /api/icms-fronteira/st
// ---------------------------------------------------------------------------

func IcmsFronteiraSTHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		fronteiraNotasHandler(db, w, r, "ST")
	}
}

// ---------------------------------------------------------------------------
// IcmsFronteiraDIFALHandler — GET /api/icms-fronteira/difal
// ---------------------------------------------------------------------------

func IcmsFronteiraDIFALHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		fronteiraNotasHandler(db, w, r, "DIFAL")
	}
}
