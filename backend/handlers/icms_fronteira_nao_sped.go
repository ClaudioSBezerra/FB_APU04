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
	VIPI          float64 `json:"v_ipi"`             // IPI do XML (<vIPI> do header)
	VFrete        float64 `json:"v_frete"`
	VFreteCTe     float64 `json:"v_frete_cte"`       // soma dos CT-es onde tomador=destinatário
	VOutro        float64 `json:"v_outro"`
	VOpr          float64 `json:"v_opr"`
	VIcmsNF       float64 `json:"v_icms_nf"`        // ICMS destacado na NF (<vICMS>)
	VIcmsCTe      float64 `json:"v_icms_cte"`        // ICMS dos CT-es do destinatário
	AliqInter     float64 `json:"aliq_inter"`        // alíquota interestadual efetiva = vIcms/vProd × 100
	AliqInterna   float64 `json:"aliq_interna"`      // alíquota interna usada (regra ou fallback)
	MVA           float64 `json:"mva"`               // MVA original (só usado em ST)
	IcmsDevidoEst float64 `json:"icms_devido_est"`   // ICMS a pagar (devido − ICMS destacado NF)
	ValorDevido   float64 `json:"valor_devido"`      // V. Devido bruto (antecipação): operação × alíq, antes de abater
	Regime        string  `json:"regime"`
	ClassStatus   string  `json:"class_status"` // "auto" | "manual"
}

type FronteiraXmlNaoSpedResponse struct {
	Rows     []FronteiraXmlNaoSpedRow `json:"rows"`
	Total    float64                  `json:"total"`
	Count    int                      `json:"count"`
	CteLinks map[string][]CteLink     `json:"cte_links"`
}

// ---------------------------------------------------------------------------
// SQL
// ---------------------------------------------------------------------------

const naoSpedQuery = `
WITH xml_falt AS (
    SELECT
        ne.id, ne.chave_nfe, ne.data_emissao, ne.forn_cnpj, ne.forn_nome,
        ne.forn_uf, ne.dest_uf, ne.dest_cnpj_cpf, COALESCE(ne.numero_nfe,'') AS numero_nfe,
        COALESCE(ne.v_prod,0) AS v_prod, COALESCE(ne.v_frete,0) AS v_frete,
        COALESCE(ne.v_outro,0) AS v_outro,
        COALESCE(ne.v_ipi,0) AS v_ipi,    -- IPI total do XML (<vIPI> do header)
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
        xf.forn_uf, xf.dest_uf, xf.dest_cnpj_cpf, xf.numero_nfe,
        xf.v_prod, xf.v_frete, xf.v_outro, xf.v_ipi, xf.v_icms,
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
    -- ST apenas se segmento da regra coincide com segmento cadastrado na empresa.
    COALESCE(cm.regime,
        CASE
            WHEN m.cfop_entrada IN ('2551','2556') THEN 'DIFAL'
            WHEN m.cfop_entrada IN ('2403','2409','2651','2652') THEN
                CASE
                    WHEN regra.segmento_codigo IS NOT NULL
                      AND EXISTS (
                          SELECT 1 FROM company_segmentos cs
                          WHERE cs.company_id = $1::uuid
                            AND cs.segmento_codigo = regra.segmento_codigo
                            AND cs.uf = COALESCE(m.dest_uf, 'PE')
                      )
                    THEN 'ST'
                    ELSE 'ANTECIPACAO'
                END
            WHEN m.cfop_entrada IN ('2101','2102','2152') THEN 'ANTECIPACAO'
            ELSE 'NAO_FRONTEIRA'
        END
    ) AS regime,
    COALESCE(cm.status, 'auto') AS class_status,
    m.v_prod,
    m.v_ipi,
    m.v_frete,
    COALESCE(cte.v_frete_cte, 0) AS v_frete_cte,
    m.v_outro,
    -- V.Operação EXIBIDO = produto + frete do CT-e (tomador=destinatário).
    -- Consistente com o que o contador usa como base de referência.
    m.v_prod + COALESCE(cte.v_frete_cte, 0) AS v_opr,
    m.v_icms AS v_icms_nf,
    COALESCE(cte.v_icms_cte, 0) AS v_icms_cte,
    -- Alíquota interestadual efetiva: mínimo 4% (crédito presumido para SN).
    CASE WHEN m.v_prod > 0
         THEN ROUND((GREATEST(m.v_icms, m.v_prod * 4.0/100.0) / m.v_prod * 100.0)::numeric, 2)
         ELSE 4.0 END AS aliq_inter,
    COALESCE(regra.aliquota_interna, 20.5) AS aliq_interna,
    COALESCE(regra.mva_original, regra.mva_ajustado_12pct, 0) AS mva,
    -- Base = produto + IPI + frete da NF + frete do CT-e (tomador=destinatário)
    -- + outras despesas, deduzindo o ICMS da própria NF E o ICMS do CT-e
    -- recolhido pela transportadora. O frete do CT-e volta a integrar o ICMS
    -- fronteira (restaurado 2026-05-26 a pedido do cliente).
    -- ST só se aplica quando regra.segmento_codigo está em company_segmentos.
    -- Base por dentro (ufb.base_por_dentro, ex.: PE): a base de antecipação/DIFAL
    -- é (operação − ICMS destacado total) / (1 − alíq_interna). ST não usa gross-up.
    CASE
        -- PRODEPE / regime especial de CD por CNPJ (art. 11-A Dec. 21.959/1999):
        -- filial recebedora beneficiada → dispensa de antecipação E ST (DIFAL fora).
        -- Identificação por nfe_entradas.dest_cnpj_cpf + vigência na data de emissão.
        -- EXISTS evita multiplicar linhas com mais de um enquadramento por CNPJ.
        WHEN m.cfop_entrada NOT IN ('2551','2556')
         AND EXISTS (
             SELECT 1 FROM prodepe_enquadramentos pe
             WHERE pe.company_id = $1
               AND pe.ativo = true
               AND pe.dispensa_antecipacao = true
               AND regexp_replace(pe.cnpj, '[^0-9]', '', 'g')
                   = regexp_replace(COALESCE(m.dest_cnpj_cpf, ''), '[^0-9]', '', 'g')
               AND (pe.vigencia_inicio IS NULL OR m.data_emissao::date >= pe.vigencia_inicio)
               AND (pe.vigencia_fim    IS NULL OR m.data_emissao::date <= pe.vigencia_fim)
         )
            THEN 0
        WHEN m.cfop_entrada IN ('2551','2556') THEN
            CASE WHEN COALESCE(ufb.base_por_dentro, false)
                THEN GREATEST(0,
                    ((m.v_prod + m.v_ipi + m.v_frete + COALESCE(cte.v_frete_cte,0) + m.v_outro
                      - GREATEST(m.v_icms, m.v_prod * 4.0/100.0) - COALESCE(cte.v_icms_cte,0))
                     / NULLIF(1.0 - COALESCE(regra.aliquota_interna,20.5)/100.0, 0))
                    * (COALESCE(regra.aliquota_interna,20.5)
                       - CASE WHEN m.v_prod > 0 THEN GREATEST(m.v_icms, m.v_prod * 4.0/100.0) / m.v_prod * 100.0 ELSE 4.0 END) / 100.0)
                ELSE GREATEST(0,
                    (m.v_prod + m.v_ipi + m.v_frete + COALESCE(cte.v_frete_cte,0) + m.v_outro)
                    * COALESCE(regra.aliquota_interna,20.5)/100.0
                    - GREATEST(m.v_icms, m.v_prod * 4.0/100.0) - COALESCE(cte.v_icms_cte,0))
            END
        WHEN m.cfop_entrada IN ('2101','2102','2152') THEN
            -- Antecipação (regra Gilson 2026-06-02): base = produto + IPI + frete da NF
            -- + outras despesas da NF (SEM frete do CT-e — esse vai no bloco próprio do
            -- CT-e). Abate apenas o ICMS destacado na NF (sem piso de 4% e sem ICMS do
            -- CT-e). PE mantém o cálculo "por dentro"; demais UFs, direto.
            CASE WHEN COALESCE(ufb.base_por_dentro, false)
                THEN GREATEST(0,
                    ((m.v_prod + m.v_ipi + m.v_frete + m.v_outro - m.v_icms)
                     / NULLIF(1.0 - COALESCE(regra.aliquota_interna,20.5)/100.0, 0))
                    * COALESCE(regra.aliquota_interna,20.5)/100.0
                    - m.v_icms)
                ELSE GREATEST(0,
                    (m.v_prod + m.v_ipi + m.v_frete + m.v_outro)
                    * COALESCE(regra.aliquota_interna,20.5)/100.0
                    - m.v_icms)
            END
        WHEN m.cfop_entrada IN ('2403','2409','2651','2652') THEN
            CASE
                WHEN regra.segmento_codigo IS NOT NULL
                  AND EXISTS (
                      SELECT 1 FROM company_segmentos cs
                      WHERE cs.company_id = $1::uuid
                        AND cs.segmento_codigo = regra.segmento_codigo
                        AND cs.uf = COALESCE(m.dest_uf, 'PE')
                  )
                THEN CASE WHEN COALESCE(regra.mva_original, regra.mva_ajustado_12pct) IS NOT NULL
                    THEN GREATEST(0,
                         (m.v_prod + m.v_ipi + m.v_frete + COALESCE(cte.v_frete_cte,0) + m.v_outro)
                         * (1.0 + COALESCE(regra.mva_original, regra.mva_ajustado_12pct)/100.0)
                         * COALESCE(regra.aliquota_interna,20.5)/100.0
                         - GREATEST(m.v_icms, m.v_prod * 4.0/100.0) - COALESCE(cte.v_icms_cte,0))
                    ELSE 0 END
                -- Sem segmento → reclassificada como ANTECIPAÇÃO (regra Gilson: base sem
                -- frete do CT-e, abate só o ICMS destacado na NF; respeita base por dentro)
                ELSE CASE WHEN COALESCE(ufb.base_por_dentro, false)
                    THEN GREATEST(0,
                        ((m.v_prod + m.v_ipi + m.v_frete + m.v_outro - m.v_icms)
                         / NULLIF(1.0 - COALESCE(regra.aliquota_interna,20.5)/100.0, 0))
                        * COALESCE(regra.aliquota_interna,20.5)/100.0
                        - m.v_icms)
                    ELSE GREATEST(0,
                        (m.v_prod + m.v_ipi + m.v_frete + m.v_outro)
                        * COALESCE(regra.aliquota_interna,20.5)/100.0
                        - m.v_icms)
                END
            END
        ELSE 0
    END AS icms_devido_est,
    -- Valor devido BRUTO (coluna "V. Devido" da memória do Gilson, antes de abater o
    -- ICMS destacado). Só faz sentido na antecipação (PE por dentro / demais direto);
    -- 0 nos demais CFOPs. ICMS a pagar = GREATEST(0, valor_devido − ICMS destacado NF).
    CASE
        WHEN m.cfop_entrada IN ('2101','2102','2152','2403','2409','2651','2652') THEN
            CASE WHEN COALESCE(ufb.base_por_dentro, false)
                THEN GREATEST(0,
                    ((m.v_prod + m.v_ipi + m.v_frete + m.v_outro - m.v_icms)
                     / NULLIF(1.0 - COALESCE(regra.aliquota_interna,20.5)/100.0, 0))
                    * COALESCE(regra.aliquota_interna,20.5)/100.0)
                ELSE GREATEST(0,
                    (m.v_prod + m.v_ipi + m.v_frete + m.v_outro)
                    * COALESCE(regra.aliquota_interna,20.5)/100.0)
            END
        ELSE 0
    END AS valor_devido
FROM mapped m
LEFT JOIN LATERAL (
    SELECT r.aliquota_interna, r.mva_original, r.mva_ajustado_12pct,
           r.segmento_codigo
    FROM icms_fronteira_regras_ncm r
    WHERE (r.company_id = $1 OR r.company_id IS NULL)
      AND r.uf_estado = COALESCE(m.dest_uf, 'PE')
      AND m.ncm IS NOT NULL
      AND LEFT(m.ncm, LENGTH(r.ncm_prefixo)) = r.ncm_prefixo
      AND LENGTH(r.ncm_prefixo) >= 4
    ORDER BY r.company_id NULLS LAST, LENGTH(r.ncm_prefixo) DESC LIMIT 1
) regra ON true
LEFT JOIN cte_por_nfe cte ON cte.chave_nfe = m.chave_nfe
LEFT JOIN uf_beneficios_fiscais ufb
    ON ufb.company_id = $1 AND ufb.uf = COALESCE(m.dest_uf, 'PE')
LEFT JOIN icms_fronteira_classificacao_manual cm
    ON cm.company_id = $1 AND cm.chave_nfe = m.chave_nfe
WHERE COALESCE(cm.regime,
    CASE
        WHEN m.cfop_entrada IN ('2551','2556') THEN 'DIFAL'
        WHEN m.cfop_entrada IN ('2403','2409','2651','2652') THEN
            CASE
                WHEN regra.segmento_codigo IS NOT NULL
                  AND EXISTS (
                      SELECT 1 FROM company_segmentos cs
                      WHERE cs.company_id = $1::uuid
                        AND cs.segmento_codigo = regra.segmento_codigo
                        AND cs.uf = COALESCE(m.dest_uf, 'PE')
                  )
                THEN 'ST'
                ELSE 'ANTECIPACAO'
            END
        WHEN m.cfop_entrada IN ('2101','2102','2152') THEN 'ANTECIPACAO'
        ELSE 'NAO_FRONTEIRA'
    END) = $3
  AND COALESCE(cm.status, 'auto') <> 'excluded'
  -- Eixo de UF do módulo: restringe às NFs cujo destinatário (filial) é da UF
  -- selecionada. Consistente com o filtro uf_filial dos Blocos A/B. Vazio = todas.
  AND ($4::text = '' OR COALESCE(m.dest_uf, 'PE') = $4)
ORDER BY m.data_emissao, m.chave_nfe
`

// fetchNaoSpedRows é usado pelo export handler para montar o Bloco C.
func fetchNaoSpedRows(db *sql.DB, companyID, periodo, regime, uf string) ([]FronteiraXmlNaoSpedRow, error) {
	rows, err := db.Query(naoSpedQuery, companyID, periodo, regime, uf)
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
			&row.VProd, &row.VIPI, &row.VFrete, &row.VFreteCTe, &row.VOutro, &row.VOpr,
			&row.VIcmsNF, &row.VIcmsCTe, &row.AliqInter, &row.AliqInterna, &row.MVA,
			&row.IcmsDevidoEst, &row.ValorDevido,
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
		uf := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("uf")))

		if regime == "" {
			jsonErr(w, http.StatusBadRequest, "Parâmetro 'regime' obrigatório (ANTECIPACAO|ST|DIFAL)")
			return
		}

		rows, err := db.Query(naoSpedQuery, companyID, periodo, regime, uf)
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
				&row.VProd, &row.VIPI, &row.VFrete, &row.VFreteCTe, &row.VOutro, &row.VOpr,
				&row.VIcmsNF, &row.VIcmsCTe, &row.AliqInter, &row.AliqInterna, &row.MVA,
				&row.IcmsDevidoEst, &row.ValorDevido,
			); err != nil {
				log.Printf("IcmsFronteiraXmlNaoSped[%s] scan error: %v", regime, err)
				continue
			}
			total += row.IcmsDevidoEst
			result = append(result, row)
		}

		chaves := make([]string, len(result))
		for i, r := range result {
			chaves[i] = r.ChaveNFe
		}
		cteLinks := fetchCteLinksForNFs(db, companyID, chaves)

		json.NewEncoder(w).Encode(FronteiraXmlNaoSpedResponse{
			Rows:     result,
			Total:    total,
			Count:    len(result),
			CteLinks: cteLinks,
		})
	}
}
