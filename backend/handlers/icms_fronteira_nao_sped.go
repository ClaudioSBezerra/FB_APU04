package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
)

// ---------------------------------------------------------------------------
// Structs — NFs em XML que não estão em nenhum SPED (Block C "nao_sped")
// ---------------------------------------------------------------------------

type FronteiraXmlNaoSpedRow struct {
	ChaveNFe      string  `json:"chave_nfe"`
	DataEmissao   string  `json:"data_emissao"`
	NumeroNFe     string  `json:"numero_nfe"`
	FornCNPJ      string  `json:"forn_cnpj"`
	FornNome      string  `json:"forn_nome"`
	FornUF        string  `json:"forn_uf"`
	CfopSaida     string  `json:"cfop_saida"`
	NCM           string  `json:"ncm"`
	VProd         float64 `json:"v_prod"`
	VFrete        float64 `json:"v_frete"`
	VFreteCTe     float64 `json:"v_frete_cte"`       // soma dos CT-es onde tomador=destinatário
	VOutro        float64 `json:"v_outro"`
	VOpr          float64 `json:"v_opr"`
	VIcmsNF       float64 `json:"v_icms_nf"`        // ICMS destacado na NF (<vICMS>)
	VIcmsCTe      float64 `json:"v_icms_cte"`        // ICMS dos CT-es do destinatário
	AliqInter     float64 `json:"aliq_inter"`        // alíquota interestadual efetiva = vIcms/vProd × 100
	AliqInterna   float64 `json:"aliq_interna"`      // alíquota interna usada (regra ou fallback)
	MVA           float64 `json:"mva"`               // MVA original (só usado em ST)
	IcmsDevidoEst float64 `json:"icms_devido_est"`
	Regime        string  `json:"regime"`
	ClassStatus   string  `json:"class_status"` // "auto" | "manual"
}

type FronteiraXmlNaoSpedResponse struct {
	Rows  []FronteiraXmlNaoSpedRow `json:"rows"`
	Total float64                  `json:"total"`
	Count int                      `json:"count"`
}

// ---------------------------------------------------------------------------
// SQL
// ---------------------------------------------------------------------------

const naoSpedQuery = `
WITH xml_falt AS (
    SELECT
        ne.id, ne.chave_nfe, ne.data_emissao, ne.forn_cnpj, ne.forn_nome,
        ne.forn_uf, ne.dest_uf, COALESCE(ne.numero_nfe,'') AS numero_nfe,
        COALESCE(ne.v_prod,0) AS v_prod, COALESCE(ne.v_frete,0) AS v_frete,
        COALESCE(ne.v_outro,0) AS v_outro,
        COALESCE(ne.v_icms,0) AS v_icms   -- ICMS interestadual pago pelo fornecedor (<vICMS>)
    FROM nfe_entradas ne
    WHERE ne.company_id = $1
      AND EXTRACT(MONTH FROM ne.data_emissao)::int = SPLIT_PART($2::text,'/',1)::int
      AND EXTRACT(YEAR  FROM ne.data_emissao)::int = SPLIT_PART($2::text,'/',2)::int
      AND NOT EXISTS (
          SELECT 1 FROM reg_c100 c100 JOIN import_jobs j ON j.id = c100.job_id
          WHERE j.company_id = $1 AND c100.chv_nfe = ne.chave_nfe
      )
), top AS (
    SELECT DISTINCT ON (xf.id)
        xf.id, xf.chave_nfe, xf.data_emissao, xf.forn_cnpj, xf.forn_nome,
        xf.forn_uf, xf.dest_uf, xf.numero_nfe,
        xf.v_prod, xf.v_frete, xf.v_outro, xf.v_icms,
        COALESCE(nii.cfop,'') AS cfop_saida, COALESCE(nii.ncm,'') AS ncm
    FROM xml_falt xf
    JOIN nfe_entradas_itens nii ON nii.nfe_id = xf.id
    ORDER BY xf.id, nii.v_prod DESC NULLS LAST
), mapped AS (
    SELECT *,
        CASE
            WHEN LEFT(cfop_saida,1) = '6' THEN '2' || SUBSTRING(cfop_saida FROM 2)
            WHEN LEFT(cfop_saida,1) = '5' THEN '1' || SUBSTRING(cfop_saida FROM 2)
            ELSE cfop_saida
        END AS cfop_entrada
    FROM top
), cte_por_nfe AS (
    -- Frete CT-e por NF-e, considerando APENAS quando tomador = destinatário
    -- (mesma regra fiscal aplicada na aba Fretes / Layer 2 do fetchFreteLinks).
    SELECT
        ref.chave_nfe,
        SUM(COALESCE(ce.v_prest, 0)) AS v_frete_cte,
        SUM(COALESCE(ce.v_icms, 0))  AS v_icms_cte
    FROM cte_entradas_nfe_refs ref
    JOIN cte_entradas ce ON ce.id = ref.cte_id
    WHERE ref.company_id = $1
      AND (
          ce.toma = '3'                                                   -- Destinatário
          OR (ce.toma = '4' AND ce.toma4_cnpj = ce.dest_cnpj_cpf)          -- Outros = destinatário
      )
    GROUP BY ref.chave_nfe
)
SELECT
    m.chave_nfe,
    m.data_emissao::text,
    m.numero_nfe,
    m.forn_cnpj, m.forn_nome, COALESCE(m.forn_uf,'') AS forn_uf,
    m.cfop_saida,
    COALESCE(m.ncm,'') AS ncm,
    COALESCE(cm.regime,
        CASE
            WHEN m.cfop_entrada IN ('2551','2556') THEN 'DIFAL'
            WHEN m.cfop_entrada IN ('2403','2409','2651','2652') THEN 'ST'
            WHEN m.cfop_entrada IN ('2101','2102','2152') THEN 'ANTECIPACAO'
            ELSE 'NAO_FRONTEIRA'
        END
    ) AS regime,
    COALESCE(cm.status, 'auto') AS class_status,
    m.v_prod,
    m.v_frete,
    COALESCE(cte.v_frete_cte, 0) AS v_frete_cte,
    m.v_outro,
    (m.v_prod + m.v_frete + COALESCE(cte.v_frete_cte, 0) + m.v_outro) AS v_opr,
    m.v_icms AS v_icms_nf,
    COALESCE(cte.v_icms_cte, 0) AS v_icms_cte,
    CASE WHEN m.v_prod > 0 THEN ROUND((m.v_icms / m.v_prod * 100.0)::numeric, 2) ELSE 0 END AS aliq_inter,
    COALESCE(regra.aliquota_interna, 20.5) AS aliq_interna,
    COALESCE(regra.mva_original, regra.mva_ajustado_12pct, 0) AS mva,
    -- Mesma lógica do Bloco B, agora somando o frete do CT-e (tomador=destinatário)
    -- e deduzindo o ICMS recolhido pela transportadora.
    CASE
        WHEN m.cfop_entrada IN ('2551','2556') THEN
            GREATEST(0,
                (m.v_prod + m.v_frete + COALESCE(cte.v_frete_cte,0) + m.v_outro)
                * COALESCE(regra.aliquota_interna,20.5)/100.0
                - m.v_icms - COALESCE(cte.v_icms_cte,0))
        WHEN m.cfop_entrada IN ('2101','2102','2152') THEN
            GREATEST(0,
                (m.v_prod + m.v_frete + COALESCE(cte.v_frete_cte,0) + m.v_outro)
                * COALESCE(regra.aliquota_interna,20.5)/100.0
                - m.v_icms - COALESCE(cte.v_icms_cte,0))
        WHEN m.cfop_entrada IN ('2403','2409','2651','2652') THEN
            CASE WHEN COALESCE(regra.mva_original, regra.mva_ajustado_12pct) IS NOT NULL
                THEN GREATEST(0,
                     (m.v_prod + m.v_frete + COALESCE(cte.v_frete_cte,0) + m.v_outro)
                     * (1.0 + COALESCE(regra.mva_original, regra.mva_ajustado_12pct)/100.0)
                     * COALESCE(regra.aliquota_interna,20.5)/100.0
                     - m.v_icms - COALESCE(cte.v_icms_cte,0))
                ELSE 0 END
        ELSE 0
    END AS icms_devido_est
FROM mapped m
LEFT JOIN LATERAL (
    SELECT r.aliquota_interna, r.mva_original, r.mva_ajustado_12pct
    FROM icms_fronteira_regras_ncm r
    WHERE (r.company_id = $1 OR r.company_id IS NULL)
      AND r.uf_estado = COALESCE(m.dest_uf, 'PE')
      AND m.ncm IS NOT NULL
      AND LEFT(m.ncm, LENGTH(r.ncm_prefixo)) = r.ncm_prefixo
    ORDER BY r.company_id NULLS LAST, LENGTH(r.ncm_prefixo) DESC LIMIT 1
) regra ON true
LEFT JOIN cte_por_nfe cte ON cte.chave_nfe = m.chave_nfe
LEFT JOIN icms_fronteira_classificacao_manual cm
    ON cm.company_id = $1 AND cm.chave_nfe = m.chave_nfe
WHERE COALESCE(cm.regime,
    CASE
        WHEN m.cfop_entrada IN ('2551','2556') THEN 'DIFAL'
        WHEN m.cfop_entrada IN ('2403','2409','2651','2652') THEN 'ST'
        WHEN m.cfop_entrada IN ('2101','2102','2152') THEN 'ANTECIPACAO'
        ELSE 'NAO_FRONTEIRA'
    END) = $3
  AND COALESCE(cm.status, 'auto') <> 'excluded'
ORDER BY m.data_emissao, m.chave_nfe
`

// fetchNaoSpedRows é usado pelo export handler para montar o Bloco C.
func fetchNaoSpedRows(db *sql.DB, companyID, periodo, regime string) ([]FronteiraXmlNaoSpedRow, error) {
	rows, err := db.Query(naoSpedQuery, companyID, periodo, regime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []FronteiraXmlNaoSpedRow
	for rows.Next() {
		var row FronteiraXmlNaoSpedRow
		if err := rows.Scan(
			&row.ChaveNFe, &row.DataEmissao, &row.NumeroNFe,
			&row.FornCNPJ, &row.FornNome, &row.FornUF,
			&row.CfopSaida, &row.NCM,
			&row.Regime, &row.ClassStatus,
			&row.VProd, &row.VFrete, &row.VFreteCTe, &row.VOutro, &row.VOpr,
			&row.VIcmsNF, &row.VIcmsCTe, &row.AliqInter, &row.AliqInterna, &row.MVA,
			&row.IcmsDevidoEst,
		); err != nil {
			continue
		}
		result = append(result, row)
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// IcmsFronteiraXmlNaoSpedHandler — GET /api/icms-fronteira/nao-sped
//
// Parâmetros:
//   - periodo: MM/YYYY
//   - regime:  ANTECIPACAO | ST | DIFAL
//
// Retorna NFs presentes no XML (nfe_entradas) com emissão no mês de análise
// que NÃO constam em nenhum SPED importado para a empresa (Block C).
// ---------------------------------------------------------------------------

func IcmsFronteiraXmlNaoSpedHandler(db *sql.DB) http.HandlerFunc {
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
		regime := r.URL.Query().Get("regime")

		if regime == "" {
			jsonErr(w, http.StatusBadRequest, "Parâmetro 'regime' obrigatório (ANTECIPACAO|ST|DIFAL)")
			return
		}

		rows, err := db.Query(naoSpedQuery, companyID, periodo, regime)
		if err != nil {
			log.Printf("IcmsFronteiraXmlNaoSped[%s] error: %v", regime, err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar NFs não encontradas no SPED")
			return
		}
		defer rows.Close()

		result := []FronteiraXmlNaoSpedRow{}
		var total float64

		for rows.Next() {
			var row FronteiraXmlNaoSpedRow
			if err := rows.Scan(
				&row.ChaveNFe, &row.DataEmissao, &row.NumeroNFe,
				&row.FornCNPJ, &row.FornNome, &row.FornUF,
				&row.CfopSaida, &row.NCM,
				&row.Regime, &row.ClassStatus,
				&row.VProd, &row.VFrete, &row.VFreteCTe, &row.VOutro, &row.VOpr,
				&row.VIcmsNF, &row.VIcmsCTe, &row.AliqInter, &row.AliqInterna, &row.MVA,
				&row.IcmsDevidoEst,
			); err != nil {
				log.Printf("IcmsFronteiraXmlNaoSped[%s] scan error: %v", regime, err)
				continue
			}
			total += row.IcmsDevidoEst
			result = append(result, row)
		}

		json.NewEncoder(w).Encode(FronteiraXmlNaoSpedResponse{
			Rows:  result,
			Total: total,
			Count: len(result),
		})
	}
}
