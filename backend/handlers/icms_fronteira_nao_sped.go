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
	CfopOriginal  string  `json:"cfop_original"` // CFOP original do XML (antes do override)
	CfopOverride  string  `json:"cfop_override"` // CFOP sobrescrito pelo usuário (vazio se automático)
	NfStatus      string  `json:"nf_status"`     // ATIVO | CANCELADO (deleção lógica)
	NCM           string  `json:"ncm"`
	VProd         float64 `json:"v_prod"`
	VIPI          float64 `json:"v_ipi"` // IPI do XML (<vIPI> do header)
	VFrete        float64 `json:"v_frete"`
	VFreteCTe     float64 `json:"v_frete_cte"` // soma dos CT-es onde tomador=destinatário
	VOutro        float64 `json:"v_outro"`
	VOpr          float64 `json:"v_opr"`
	VIcmsNF       float64 `json:"v_icms_nf"`       // ICMS destacado na NF (<vICMS>)
	VIcmsCTe      float64 `json:"v_icms_cte"`      // ICMS dos CT-es do destinatário
	AliqInter     float64 `json:"aliq_inter"`      // alíquota interestadual efetiva = vIcms/vProd × 100
	AliqInterna   float64 `json:"aliq_interna"`    // alíquota interna usada (regra ou fallback)
	MVA           float64 `json:"mva"`             // MVA original (só usado em ST)
	IcmsDevidoEst float64 `json:"icms_devido_est"` // ICMS a pagar (devido − ICMS destacado NF)
	ValorDevido   float64 `json:"valor_devido"`    // V. Devido bruto (antecipação): operação × alíq, antes de abater
	BasePorDentro bool    `json:"base_por_dentro"` // UF usa cálculo "por dentro" (ex.: PE)
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
WITH emp_uf AS (
    -- UF efetiva da empresa: fonte confiável para fallback quando dest_uf do XML
    -- está nulo. Usa o import_jobs mais recente (evita hardcode 'PE' para CE/outros).
    SELECT COALESCE(MAX(uf) FILTER (WHERE uf IS NOT NULL AND uf <> ''), 'PE') AS uf
    FROM import_jobs WHERE company_id = $1
), xml_falt AS (
    SELECT
        ne.id, ne.chave_nfe, ne.data_emissao, ne.forn_cnpj, ne.forn_nome,
        ne.forn_uf, ne.dest_uf, ne.dest_cnpj_cpf, COALESCE(ne.numero_nfe,'') AS numero_nfe,
        COALESCE(ne.v_prod,0) AS v_prod, COALESCE(ne.v_frete,0) AS v_frete,
        COALESCE(ne.v_outro,0) AS v_outro,
        COALESCE(ne.v_ipi,0) AS v_ipi,    -- IPI total do XML (<vIPI> do header)
        COALESCE(ne.v_icms,0) AS v_icms,   -- ICMS interestadual pago pelo fornecedor (<vICMS>)
        COALESCE(ne.status, 'ATIVO') AS nf_status
    FROM nfe_entradas ne
    WHERE ne.company_id = $1
      AND EXTRACT(MONTH FROM ne.data_emissao)::int = SPLIT_PART($2::text,'/',1)::int
      AND EXTRACT(YEAR  FROM ne.data_emissao)::int = SPLIT_PART($2::text,'/',2)::int
      AND NOT EXISTS (
          SELECT 1 FROM reg_c100 c100 JOIN import_jobs j ON j.id = c100.job_id
          WHERE j.company_id = $1 AND c100.chv_nfe = ne.chave_nfe
      )
), items_grouped AS (
    -- Agrupa por (NF, CFOP, NCM): cada combinação distinta vira linha própria,
    -- permitindo classificação correta quando uma NF tem itens ST e não-ST
    -- no mesmo CFOP (ex.: NCM 7318=ST e NCM 38249941=Antecipação).
    SELECT nii.nfe_id,
           COALESCE(nii.cfop,'') AS cfop_saida,
           COALESCE(nii.ncm,'')  AS ncm,
           SUM(COALESCE(nii.v_prod, 0)) AS item_sum,
           SUM(COALESCE(nii.v_ipi,  0)) AS item_ipi
    FROM nfe_entradas_itens nii
    JOIN xml_falt xf ON xf.id = nii.nfe_id
    GROUP BY nii.nfe_id, nii.cfop, nii.ncm
), nf_total AS (
    SELECT nfe_id, SUM(item_sum) AS total_sum
    FROM items_grouped
    GROUP BY nfe_id
), top AS (
    SELECT
        xf.id, xf.chave_nfe, xf.data_emissao, xf.forn_cnpj, xf.forn_nome,
        xf.forn_uf, xf.dest_uf, xf.dest_cnpj_cpf, xf.numero_nfe,
        -- CFOP efetivo: usa override do usuário quando presente, senão XML original
        COALESCE(ov.cfop_saida_override, ig.cfop_saida) AS cfop_saida,
        ig.cfop_saida                                    AS cfop_xml,
        COALESCE(ov.cfop_saida_override, '')             AS cfop_override_val,
        ig.ncm,
        -- v_prod = soma dos itens deste grupo (NCM+CFOP)
        ig.item_sum                                                               AS v_prod,
        -- Frete/outro/ICMS do cabeçalho da NF rateados pela participação deste grupo
        CASE WHEN nt.total_sum > 0 THEN xf.v_frete * ig.item_sum / nt.total_sum ELSE 0 END AS v_frete,
        CASE WHEN nt.total_sum > 0 THEN xf.v_outro * ig.item_sum / nt.total_sum ELSE 0 END AS v_outro,
        -- IPI: soma real dos itens deste grupo no XML (mesmo critério dos Blocos A/B via SPED)
        ig.item_ipi                                                                          AS v_ipi,
        CASE WHEN nt.total_sum > 0 THEN xf.v_icms  * ig.item_sum / nt.total_sum ELSE 0 END AS v_icms,
        -- Fração deste grupo no total da NF (para ratear v_frete_cte / v_icms_cte do CT-e)
        CASE WHEN nt.total_sum > 0 THEN ig.item_sum / nt.total_sum             ELSE 1 END AS item_ratio,
        xf.nf_status
    FROM xml_falt xf
    JOIN items_grouped ig ON ig.nfe_id = xf.id
    JOIN nf_total      nt ON nt.nfe_id  = xf.id
    LEFT JOIN nao_sped_cfop_override ov
           ON ov.company_id = $1::uuid
          AND ov.chave_nfe  = xf.chave_nfe
          AND COALESCE(ov.ncm, '') = COALESCE(ig.ncm, '')
), mapped AS (
    SELECT *,
        CASE
            WHEN LEFT(cfop_saida,1) = '6' THEN '2' || SUBSTRING(cfop_saida FROM 2)
            WHEN LEFT(cfop_saida,1) = '5' THEN '1' || SUBSTRING(cfop_saida FROM 2)
            ELSE cfop_saida
        END AS cfop_entrada,
        -- eff_uf: resolução em 3 camadas para empresas multi-filial (ex.: ROLIMEC PE+BA+CE):
        --   1) dest_uf do XML (campo <UF> do destinatário na NF-e) — fonte primária.
        --   2) UF do estabelecimento pelo CNPJ destino: cruza dest_cnpj_cpf com
        --      import_jobs.cnpj do mesmo company_id. Correto para filiais — o CNPJ
        --      do destinatário identifica exatamente qual filial recebeu a mercadoria
        --      e, portanto, qual UF rege a antecipação/ST.
        --   3) emp_uf (MAX uf dos import_jobs) — último recurso; retorna a UF
        --      dominante (ex.: PE quando há PE+BA+CE), mas só alcançado quando o
        --      XML não traz dest_uf E o CNPJ destino não bate com nenhuma filial.
        COALESCE(
            NULLIF(dest_uf, ''),
            (SELECT j.uf
             FROM import_jobs j
             WHERE j.company_id = $1
               AND j.status = 'completed'
               AND j.uf IS NOT NULL AND j.uf <> ''
               AND regexp_replace(COALESCE(j.cnpj,''), '[^0-9]', '', 'g')
                   = regexp_replace(COALESCE(dest_cnpj_cpf,''), '[^0-9]', '', 'g')
             LIMIT 1),
            (SELECT uf FROM emp_uf)
        ) AS eff_uf
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
            -- ST por NCM (orientação Gilson 2026-06-27): NCM cadastrado com segmento ST
            -- → classifica como ST independentemente do CFOP do fornecedor. Necessário
            -- quando o remetente não tem protocolo CONFAZ e usa CFOP 6101/6102.
            WHEN regra.segmento_codigo IS NOT NULL
              AND EXISTS (
                  SELECT 1 FROM company_segmentos cs
                  WHERE cs.company_id = $1::uuid
                    AND cs.segmento_codigo = regra.segmento_codigo
                    AND cs.uf = m.eff_uf
              )
            THEN 'ST'
            WHEN m.cfop_entrada IN ('2403','2409','2651','2652','2101','2102','2152') THEN 'ANTECIPACAO'
            ELSE 'NAO_FRONTEIRA'
        END
    ) AS regime,
    COALESCE(cm.status,
        -- 'ncm': CFOP normal (6101/6102) mas NCM tem regra ST → reclassificado pelo NCM
        CASE
            WHEN m.cfop_entrada IN ('2101','2102','2152')
              AND regra.segmento_codigo IS NOT NULL
              AND EXISTS (
                  SELECT 1 FROM company_segmentos cs
                  WHERE cs.company_id = $1::uuid
                    AND cs.segmento_codigo = regra.segmento_codigo
                    AND cs.uf = m.eff_uf
              )
            THEN 'ncm'
            ELSE 'auto'
        END
    ) AS class_status,
    m.v_prod,
    m.v_ipi,
    m.v_frete,
    COALESCE(cte.v_frete_cte, 0) * m.item_ratio AS v_frete_cte,
    m.v_outro,
    -- V.Operação EXIBIDO = produto + frete do CT-e (tomador=destinatário).
    -- Consistente com o que o contador usa como base de referência.
    m.v_prod + COALESCE(cte.v_frete_cte, 0) * m.item_ratio AS v_opr,
    m.v_icms AS v_icms_nf,
    COALESCE(cte.v_icms_cte, 0) * m.item_ratio AS v_icms_cte,
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
                    ((m.v_prod + m.v_ipi + m.v_frete + COALESCE(cte.v_frete_cte,0) * m.item_ratio + m.v_outro
                      - GREATEST(m.v_icms, m.v_prod * 4.0/100.0) - COALESCE(cte.v_icms_cte,0) * m.item_ratio)
                     / NULLIF(1.0 - COALESCE(regra.aliquota_interna,20.5)/100.0, 0))
                    * (COALESCE(regra.aliquota_interna,20.5)
                       - CASE WHEN m.v_prod > 0 THEN GREATEST(m.v_icms, m.v_prod * 4.0/100.0) / m.v_prod * 100.0 ELSE 4.0 END) / 100.0)
                ELSE GREATEST(0,
                    (m.v_prod + m.v_ipi + m.v_frete + COALESCE(cte.v_frete_cte,0) * m.item_ratio + m.v_outro)
                    * COALESCE(regra.aliquota_interna,20.5)/100.0
                    - GREATEST(m.v_icms, m.v_prod * 4.0/100.0) - COALESCE(cte.v_icms_cte,0) * m.item_ratio)
            END
        -- ST por NCM: calcula ST com MVA independentemente do CFOP (inclui CT-e na base).
        WHEN regra.segmento_codigo IS NOT NULL
          AND EXISTS (
              SELECT 1 FROM company_segmentos cs
              WHERE cs.company_id = $1::uuid
                AND cs.segmento_codigo = regra.segmento_codigo
                AND cs.uf = m.eff_uf
          )
        THEN CASE WHEN COALESCE(regra.mva_original, regra.mva_ajustado_12pct) IS NOT NULL
            THEN GREATEST(0,
                 (m.v_prod + m.v_ipi + m.v_frete + COALESCE(cte.v_frete_cte,0) * m.item_ratio + m.v_outro)
                 * (1.0 + COALESCE(regra.mva_original, regra.mva_ajustado_12pct)/100.0)
                 * COALESCE(regra.aliquota_interna,20.5)/100.0
                 - GREATEST(m.v_icms, m.v_prod * 4.0/100.0) - COALESCE(cte.v_icms_cte,0) * m.item_ratio)
            ELSE 0 END
        -- ANTECIPACAO: todos os CFOPs em escopo sem regra de ST no NCM (inclui 6101/6102
        -- sem protocolo CONFAZ e 6401/6403 sem segmento cadastrado).
        WHEN m.cfop_entrada IN ('2403','2409','2651','2652','2101','2102','2152') THEN
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
    END AS valor_devido,
    COALESCE(ufb.base_por_dentro, false) AS base_por_dentro,
    m.cfop_xml          AS cfop_original,
    m.cfop_override_val AS cfop_override,
    m.nf_status
FROM mapped m
LEFT JOIN LATERAL (
    SELECT r.aliquota_interna, r.mva_original, r.mva_ajustado_12pct,
           r.segmento_codigo
    FROM icms_fronteira_regras_ncm r
    WHERE (r.company_id = $1 OR r.company_id IS NULL)
      AND r.uf_estado = m.eff_uf
      AND m.ncm IS NOT NULL
      AND LEFT(m.ncm, LENGTH(r.ncm_prefixo)) = r.ncm_prefixo
      AND LENGTH(r.ncm_prefixo) >= 4
    ORDER BY r.company_id NULLS LAST, LENGTH(r.ncm_prefixo) DESC LIMIT 1
) regra ON true
LEFT JOIN cte_por_nfe cte ON cte.chave_nfe = m.chave_nfe
LEFT JOIN uf_beneficios_fiscais ufb
    ON ufb.company_id = $1 AND ufb.uf = m.eff_uf
LEFT JOIN icms_fronteira_classificacao_manual cm
    ON cm.company_id = $1 AND cm.chave_nfe = m.chave_nfe
WHERE ($3 = '' OR COALESCE(cm.regime,
    CASE
        WHEN m.cfop_entrada IN ('2551','2556') THEN 'DIFAL'
        WHEN regra.segmento_codigo IS NOT NULL
          AND EXISTS (
              SELECT 1 FROM company_segmentos cs
              WHERE cs.company_id = $1::uuid
                AND cs.segmento_codigo = regra.segmento_codigo
                AND cs.uf = m.eff_uf
          )
        THEN 'ST'
        WHEN m.cfop_entrada IN ('2403','2409','2651','2652','2101','2102','2152') THEN 'ANTECIPACAO'
        ELSE 'NAO_FRONTEIRA'
    END) = $3)
  AND COALESCE(cm.status, 'auto') <> 'excluded'
  -- Eixo de UF do módulo: restringe às NFs cujo destinatário (filial) é da UF
  -- selecionada. Consistente com o filtro uf_filial dos Blocos A/B. Vazio = todas.
  AND ($4::text = '' OR m.eff_uf = $4)
  -- Filtros opcionais (fornecedor / número da nota / intervalo de data), iguais
  -- aos dos Blocos A/B. Vazio = sem filtro.
  AND ($5::text = '' OR m.forn_cnpj ILIKE '%'||$5||'%' OR m.forn_nome ILIKE '%'||$5||'%')
  AND ($6::text = '' OR m.numero_nfe ILIKE '%'||$6||'%')
  AND ($7::text = '' OR m.data_emissao::date >= $7::date)
  AND ($8::text = '' OR m.data_emissao::date <= $8::date)
ORDER BY m.data_emissao, m.chave_nfe
`

// naoSpedFiltros extrai os filtros opcionais (forn / num_nota / data_ini /
// data_fim) da requisição, com os mesmos nomes de query param dos Blocos A/B.
// Vazio = sem filtro.
func naoSpedFiltros(r *http.Request) (forn, numNota, dataIni, dataFim string) {
	forn = strings.TrimSpace(r.URL.Query().Get("forn"))
	numNota = strings.TrimSpace(r.URL.Query().Get("num_nota"))
	dataIni = strings.TrimSpace(r.URL.Query().Get("data_ini"))
	dataFim = strings.TrimSpace(r.URL.Query().Get("data_fim"))
	return
}

// fetchNaoSpedRows é usado pelo export handler para montar o Bloco C.
// forn/numNota/dataIni/dataFim são filtros opcionais (vazio = sem filtro).
func fetchNaoSpedRows(db *sql.DB, companyID, periodo, regime, uf, forn, numNota, dataIni, dataFim string) ([]FronteiraXmlNaoSpedRow, error) {
	rows, err := db.Query(naoSpedQuery, companyID, periodo, regime, uf, forn, numNota, dataIni, dataFim)
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
			&row.IcmsDevidoEst, &row.ValorDevido, &row.BasePorDentro,
			&row.CfopOriginal, &row.CfopOverride, &row.NfStatus,
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

		forn, numNota, dataIni, dataFim := naoSpedFiltros(r)
		rows, err := db.Query(naoSpedQuery, companyID, periodo, regime, uf, forn, numNota, dataIni, dataFim)
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
				&row.IcmsDevidoEst, &row.ValorDevido, &row.BasePorDentro,
				&row.CfopOriginal, &row.CfopOverride, &row.NfStatus,
			); err != nil {
				log.Printf("IcmsFronteiraXmlNaoSped[%s] scan error: %v", regime, err)
				continue
			}
			if row.NfStatus != "CANCELADO" {
				total += row.IcmsDevidoEst
			}
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
