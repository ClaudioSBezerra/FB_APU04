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
	VIcms         float64 `json:"v_icms"`
	VBcST         float64 `json:"v_bc_st"`
	VST           float64 `json:"v_st"`
	AliqInter     float64 `json:"aliq_inter"`
	AliqInterna   float64 `json:"aliq_interna"`
	IcmsDevidoEst float64 `json:"icms_devido_est"`
	Regime        string  `json:"regime"`
}

type FronteiraNotasResponse struct {
	Rows  []FronteiraNotaRow `json:"rows"`
	Total float64            `json:"total"`
	Count int                `json:"count"`
}

// ---------------------------------------------------------------------------
// SQL helpers
// ---------------------------------------------------------------------------

// baseQuery returns the common SELECT that classifies each nota and computes
// the estimated ICMS due. Caller appends a WHERE clause for the regime filter
// and the $1 company_id placeholder.
const fronteiraBaseQuery = `
WITH classified AS (
    SELECT
        ne.chave_nfe,
        ne.data_emissao::text                               AS data_emissao,
        COALESCE(ne.numero_nfe, '')                         AS numero_nfe,
        COALESCE(ne.forn_cnpj, '')                          AS forn_cnpj,
        COALESCE(ne.forn_nome, '')                          AS forn_nome,
        COALESCE(ne.forn_uf, '')                            AS forn_uf,
        COALESCE(ne.cfop, '')                               AS cfop,
        COALESCE(ne.v_prod, 0)                              AS v_prod,
        COALESCE(ne.v_icms, 0)                              AS v_icms,
        COALESCE(ne.v_bc_st, 0)                             AS v_bc_st,
        COALESCE(ne.v_st, 0)                                AS v_st,
        -- Alíquota interestadual: 4% para mercadoria importada (CST orig 1,2,3,6,7,8),
        -- 7% para Sul/Sudeste (exceto ES e MT), 12% para demais.
        CASE
            WHEN ne.cst_orig_pred IN ('1','2','3','6','7','8') THEN 4.0
            WHEN ne.forn_uf = ANY(ARRAY['PR','RS','SC','MG','RJ','SP']) THEN 7.0
            ELSE 12.0
        END                                                 AS aliq_inter,
        COALESCE(regra.aliquota_interna, 20.5)              AS aliq_interna,
        -- Classificação por CFOP conforme especificação do contador
        CASE
            WHEN ne.cfop IN ('2551','2556')
                THEN 'DIFAL'
            WHEN ne.cfop IN ('2403','2409','2651','2652')
                THEN 'ST'
            WHEN ne.cfop IN ('2101','2102','2152')
                THEN 'ANTECIPACAO'
        END                                                 AS regime,
        -- ICMS devido estimado por regime (cálculo provisório; BC completa no Bloco 2)
        CASE
            WHEN ne.cfop IN ('2551','2556')
                THEN GREATEST(0,
                    COALESCE(ne.v_prod, 0) * (
                        COALESCE(regra.aliquota_interna, 20.5) -
                        CASE
                            WHEN ne.cst_orig_pred IN ('1','2','3','6','7','8') THEN 4.0
                            WHEN ne.forn_uf = ANY(ARRAY['PR','RS','SC','MG','RJ','SP']) THEN 7.0
                            ELSE 12.0
                        END
                    ) / 100.0)
            WHEN ne.cfop IN ('2403','2409','2651','2652')
                THEN COALESCE(ne.v_st, 0)
            WHEN ne.cfop IN ('2101','2102','2152')
                THEN GREATEST(0,
                    COALESCE(ne.v_prod, 0) * (
                        COALESCE(regra.aliquota_interna, 20.5) -
                        CASE
                            WHEN ne.cst_orig_pred IN ('1','2','3','6','7','8') THEN 4.0
                            WHEN ne.forn_uf = ANY(ARRAY['PR','RS','SC','MG','RJ','SP']) THEN 7.0
                            ELSE 12.0
                        END
                    ) / 100.0)
            ELSE 0
        END                                                 AS icms_devido_est
    FROM nfe_entradas ne
    LEFT JOIN LATERAL (
        SELECT nii.ncm AS ncm
        FROM nfe_entradas_itens nii
        WHERE nii.nfe_id = ne.id
        ORDER BY nii.v_prod DESC NULLS LAST
        LIMIT 1
    ) top_item ON true
    LEFT JOIN LATERAL (
        SELECT r.aliquota_interna
        FROM icms_fronteira_regras_ncm r
        WHERE (r.company_id = $1 OR r.company_id IS NULL)
          AND top_item.ncm IS NOT NULL
          AND LEFT(top_item.ncm, LENGTH(r.ncm_prefixo)) = r.ncm_prefixo
        ORDER BY r.company_id NULLS LAST, LENGTH(r.ncm_prefixo) DESC
        LIMIT 1
    ) regra ON true
    WHERE ne.company_id = $1
      AND ne.forn_uf IS NOT NULL
      AND ne.forn_uf != ''
      AND ne.forn_uf != COALESCE(ne.dest_uf, 'PE')
      AND ne.cfop = ANY(ARRAY['2101','2102','2152','2403','2409','2651','2652','2551','2556'])
      AND ($2::text = '' OR (
          EXTRACT(MONTH FROM ne.data_emissao)::int = SPLIT_PART($2::text,'/',1)::int
          AND EXTRACT(YEAR  FROM ne.data_emissao)::int = SPLIT_PART($2::text,'/',2)::int
      ))
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

		query := fronteiraBaseQuery + `
SELECT
    regime,
    COUNT(*)            AS qtd_notas,
    SUM(v_prod)         AS v_prod_total,
    SUM(v_st)           AS v_st_retido,
    SUM(icms_devido_est) AS icms_devido_est
FROM classified
GROUP BY regime
ORDER BY regime
`
		rows, err := db.Query(query, companyID, periodo)
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
				&row.Regime, &row.QtdNotas, &row.VProdTotal, &row.VStRetido, &row.IcmsDevidoEst,
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

	query := fronteiraBaseQuery + `
SELECT
    chave_nfe, data_emissao, numero_nfe, forn_cnpj, forn_nome, forn_uf,
    cfop, v_prod, v_icms, v_bc_st, v_st,
    aliq_inter, aliq_interna, icms_devido_est, regime
FROM classified
WHERE regime = $3
ORDER BY data_emissao DESC, chave_nfe
LIMIT 500
`
	rows, err := db.Query(query, companyID, periodo, regime)
	if err != nil {
		log.Printf("IcmsFronteiraNotas[%s] error: %v", regime, err)
		jsonErr(w, http.StatusInternalServerError, "Erro ao consultar notas ICMS Fronteira")
		return
	}
	defer rows.Close()

	result := []FronteiraNotaRow{}
	var total float64

	for rows.Next() {
		var row FronteiraNotaRow
		if err := rows.Scan(
			&row.ChaveNFe, &row.DataEmissao, &row.NumeroNFe,
			&row.FornCNPJ, &row.FornNome, &row.FornUF,
			&row.CFOP, &row.VProd, &row.VIcms, &row.VBcST, &row.VST,
			&row.AliqInter, &row.AliqInterna, &row.IcmsDevidoEst, &row.Regime,
		); err != nil {
			log.Printf("IcmsFronteiraNotas[%s] scan error: %v", regime, err)
			continue
		}
		total += row.IcmsDevidoEst
		result = append(result, row)
	}

	json.NewEncoder(w).Encode(FronteiraNotasResponse{
		Rows:  result,
		Total: total,
		Count: len(result),
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
