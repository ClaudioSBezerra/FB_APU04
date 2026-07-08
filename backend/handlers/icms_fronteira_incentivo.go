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

// ---------------------------------------------------------------------------
// Relatório "Incentivo" — espelho de Antecipação/ST/DIFAL filtrado pelas notas
// dispensadas pelo motor (PRODEPE/PROIND) durante a vigência do ato.
//
// O motor de fronteira (icms_fronteira.go / icms_fronteira_nao_sped.go) zera
// silenciosamente o ICMS quando há enquadramento ativo. Esta tela inverte o
// olhar: mostra exatamente o que foi zerado, com programa, nº ato, vigência
// e — crucialmente — o valor que SERIA devido sem o incentivo (= economia).
//
// DIFAL (CFOPs 2551/2556) NÃO está na dispensa do motor, então também NÃO
// aparece aqui.
//
// Bloco A/B/C: mesma divisão de Antecipação — A = mês selecionado (SPED),
// B = SPED meses anteriores, C = NF-e em XML sem SPED (Bloco C / nao-sped).
// ---------------------------------------------------------------------------

type FronteiraIncentivoRow struct {
	ChaveNFe        string  `json:"chave_nfe"`
	DataEmissao     string  `json:"data_emissao"`
	NumeroNFe       string  `json:"numero_nfe"`
	FornCNPJ        string  `json:"forn_cnpj"`
	FornNome        string  `json:"forn_nome"`
	FornUF          string  `json:"forn_uf"`
	CFOP            string  `json:"cfop"`
	VProd           float64 `json:"v_prod"`
	VIPI            float64 `json:"v_ipi"`
	VIcms           float64 `json:"v_icms"`
	AliqInter       float64 `json:"aliq_inter"`
	AliqInterna     float64 `json:"aliq_interna"`
	Regime          string  `json:"regime"`      // ANTECIPACAO ou ST (DIFAL excluído)
	Bloco           string  `json:"bloco"`       // mes_atual | mes_anterior | nao_sped
	CnpjFilial      string  `json:"cnpj_filial"` // CNPJ recebedor (chave de junção)
	Programa        string  `json:"programa"`    // PRODEPE | PROIND
	NumAto          string  `json:"num_ato"`
	VigenciaInicio  string  `json:"vigencia_inicio"`
	VigenciaFim     string  `json:"vigencia_fim"`
	IcmsSeriaDevido float64 `json:"icms_seria_devido"` // = quanto foi dispensado
}

type FronteiraIncentivoResponse struct {
	Rows             []FronteiraIncentivoRow `json:"rows"`
	Count            int                     `json:"count"`
	TotalDispensado  float64                 `json:"total_dispensado"`
	TotalMesAtual    float64                 `json:"total_mes_atual"`
	TotalMesAnterior float64                 `json:"total_mes_anterior"`
	TotalNaoSped     float64                 `json:"total_nao_sped"`
	CountMesAtual    int                     `json:"count_mes_atual"`
	CountMesAnterior int                     `json:"count_mes_anterior"`
	CountNaoSped     int                     `json:"count_nao_sped"`
	PorPrograma      map[string]float64      `json:"por_programa"` // soma dispensada por programa
	PorFilial        map[string]float64      `json:"por_filial"`   // soma dispensada por CNPJ
}

// ---------------------------------------------------------------------------
// SQL — Bloco SPED (A + B)
//
// Reusa o fronteiraBaseQuery (CTEs classified com campos crus expostos: base_calc,
// regra_aliq_interna, regra_mva_*, base_por_dentro, cnpj_filial) e:
//   1. JOIN prodepe_enquadramentos ativo+vigente por CNPJ (digits-only) → traz
//      programa, num_ato, vigência. Sem JOIN = nota não dispensada → fora.
//   2. Recalcula icms_seria_devido aplicando a mesma lógica fiscal que o motor
//      usaria SEM o branch PRODEPE — i.e., o cálculo padrão ANT/ST.
//   3. Exclui DIFAL (2551/2556) — não está na dispensa.
//
// O cálculo do icms_seria_devido replica os branches ANT/ST do fronteiraBaseQuery
// para preservar fidelidade fiscal (MVA ajustado/Convênio 110, base por dentro,
// segmento ST). É a única coisa duplicada — CTEs compartilhadas.
// ---------------------------------------------------------------------------

const incentivoSpedSelectCols = `
SELECT
    c.chave_nfe, c.data_emissao, c.numero_nfe,
    c.forn_cnpj, c.forn_nome, c.forn_uf,
    c.cfop, c.v_prod, c.v_ipi, c.v_icms,
    c.aliq_inter, c.aliq_interna, c.regime, c.bloco,
    c.cnpj_filial, pe.programa,
    COALESCE(pe.num_ato, '')               AS num_ato,
    COALESCE(pe.vigencia_inicio::text, '') AS vigencia_inicio,
    COALESCE(pe.vigencia_fim::text, '')    AS vigencia_fim,
    -- icms_seria_devido = cálculo padrão SEM o branch PRODEPE
    CASE
        WHEN c.cfop IN ('2403','2409','2651','2652')
         AND c.regra_seg_codigo IS NOT NULL
         AND EXISTS (
             SELECT 1 FROM company_segmentos cs
             WHERE cs.company_id = $1::uuid
               AND cs.segmento_codigo = c.regra_seg_codigo
               AND cs.uf = c.uf_filial
         )
            THEN CASE
                WHEN COALESCE(
                    CASE COALESCE(NULLIF(c.aliq_inter,0), 12.0)
                        WHEN 4.0  THEN c.regra_mva_4
                        WHEN 7.0  THEN c.regra_mva_7
                        WHEN 12.0 THEN c.regra_mva_12
                    END,
                    CASE WHEN c.regra_mva_original IS NOT NULL AND COALESCE(c.regra_aliq_interna,20.5) < 100 THEN
                        ((1.0 + c.regra_mva_original/100.0) * (1.0 - COALESCE(NULLIF(c.aliq_inter,0),12.0)/100.0)
                         / NULLIF(1.0 - COALESCE(c.regra_aliq_interna,20.5)/100.0, 0) - 1.0) * 100.0
                    END,
                    c.regra_mva_original
                ) IS NOT NULL
                THEN GREATEST(0,
                    c.base_calc
                    * (1.0 + COALESCE(
                        CASE COALESCE(NULLIF(c.aliq_inter,0), 12.0)
                            WHEN 4.0  THEN c.regra_mva_4
                            WHEN 7.0  THEN c.regra_mva_7
                            WHEN 12.0 THEN c.regra_mva_12
                        END,
                        CASE WHEN c.regra_mva_original IS NOT NULL AND COALESCE(c.regra_aliq_interna,20.5) < 100 THEN
                            ((1.0 + c.regra_mva_original/100.0) * (1.0 - COALESCE(NULLIF(c.aliq_inter,0),12.0)/100.0)
                             / NULLIF(1.0 - COALESCE(c.regra_aliq_interna,20.5)/100.0, 0) - 1.0) * 100.0
                        END,
                        c.regra_mva_original
                    )/100.0)
                    * COALESCE(c.regra_aliq_interna, 20.5)/100.0
                    - COALESCE(c.v_icms, 0))
                ELSE 0
            END
        -- Antecipação (CFOPs 2101/2102/2152, ou ST sem segmento da empresa).
        -- Regra Gilson: IPI integra a base (base_calc + v_ipi), igual ao Bloco C.
        ELSE CASE WHEN c.base_por_dentro
            THEN GREATEST(0,
                ((c.base_calc + c.v_ipi - COALESCE(c.v_icms, 0))
                 / NULLIF(1.0 - COALESCE(c.regra_aliq_interna, 20.5)/100.0, 0))
                * COALESCE(c.regra_aliq_interna, 20.5)/100.0
                - COALESCE(c.v_icms, 0))
            ELSE GREATEST(0,
                (c.base_calc + c.v_ipi) * COALESCE(c.regra_aliq_interna, 20.5)/100.0
                - COALESCE(c.v_icms, 0))
        END
    END AS icms_seria_devido
FROM classified c
JOIN prodepe_enquadramentos pe
  ON pe.company_id = $1::uuid
 AND pe.ativo = true
 AND pe.dispensa_antecipacao = true
 AND regexp_replace(pe.cnpj, '[^0-9]', '', 'g')
     = regexp_replace(COALESCE(c.cnpj_filial, ''), '[^0-9]', '', 'g')
 AND (pe.vigencia_inicio IS NULL OR c.data_emissao::date >= pe.vigencia_inicio)
 AND (pe.vigencia_fim    IS NULL OR c.data_emissao::date <= pe.vigencia_fim)
WHERE c.cfop NOT IN ('2551','2556')
  AND c.regime IS NOT NULL
`

// ---------------------------------------------------------------------------
// SQL — Bloco C (NF-e em XML sem SPED)
//
// Estrutura derivada do naoSpedQuery (CTEs idênticas). O SELECT final filtra
// por JOIN prodepe e calcula icms_seria_devido SEM o branch PRODEPE.
// CTE cte_por_nfe é replicada (igual ao Bloco C original).
// ---------------------------------------------------------------------------

const incentivoNaoSpedQuery = `
WITH xml_falt AS (
    SELECT
        ne.id, ne.chave_nfe, ne.data_emissao, ne.forn_cnpj, ne.forn_nome,
        ne.forn_uf, ne.dest_uf, ne.dest_cnpj_cpf, COALESCE(ne.numero_nfe,'') AS numero_nfe,
        COALESCE(ne.v_prod,0) AS v_prod, COALESCE(ne.v_frete,0) AS v_frete,
        COALESCE(ne.v_outro,0) AS v_outro,
        COALESCE(ne.v_ipi,0) AS v_ipi,
        COALESCE(ne.v_icms,0) AS v_icms
    FROM nfe_entradas ne
    WHERE ne.company_id = $1
      AND ($2::text = '' OR (
          EXTRACT(MONTH FROM ne.data_emissao)::int = SPLIT_PART($2::text,'/',1)::int
          AND EXTRACT(YEAR FROM ne.data_emissao)::int = SPLIT_PART($2::text,'/',2)::int
      ))
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
    SELECT
        ref.chave_nfe,
        SUM(COALESCE(ce.v_prest, 0)) AS v_frete_cte,
        SUM(COALESCE(ce.v_icms, 0))  AS v_icms_cte
    FROM cte_entradas_nfe_refs ref
    JOIN cte_entradas ce ON ce.id = ref.cte_id
    WHERE ref.company_id = $1
      AND (
          ce.toma = '3'
          OR (ce.toma = '4' AND ce.toma4_cnpj = ce.dest_cnpj_cpf)
      )
    GROUP BY ref.chave_nfe
)
SELECT
    m.chave_nfe,
    m.data_emissao::text,
    m.numero_nfe,
    m.forn_cnpj, m.forn_nome, COALESCE(m.forn_uf,'') AS forn_uf,
    m.cfop_entrada AS cfop,
    m.v_prod, m.v_ipi, m.v_icms AS v_icms_nf,
    CASE WHEN m.v_prod > 0 THEN ROUND((m.v_icms / m.v_prod * 100.0)::numeric, 2) ELSE 0 END AS aliq_inter,
    COALESCE(regra.aliquota_interna, 20.5) AS aliq_interna,
    -- regime: ST se segmento bate, senão antecipação. DIFAL filtrado fora abaixo.
    CASE
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
    END AS regime,
    COALESCE(m.dest_cnpj_cpf, '') AS cnpj_filial,
    pe.programa,
    COALESCE(pe.num_ato, '')               AS num_ato,
    COALESCE(pe.vigencia_inicio::text, '') AS vigencia_inicio,
    COALESCE(pe.vigencia_fim::text, '')    AS vigencia_fim,
    -- icms_seria_devido SEM PRODEPE: cálculo padrão do Bloco C (base por dentro
    -- para antecipação; MVA para ST com segmento). Replica naoSpedQuery sem o
    -- branch PRODEPE.
    CASE
        WHEN m.cfop_entrada IN ('2403','2409','2651','2652')
         AND regra.segmento_codigo IS NOT NULL
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
                 - m.v_icms - COALESCE(cte.v_icms_cte,0))
            ELSE 0 END
        -- Antecipação (regra Gilson 2026-06-02): base = produto + IPI + frete da NF
        -- + outras despesas da NF (SEM frete do CT-e — calculado em separado); abate
        -- só o ICMS destacado na NF (sem ICMS do CT-e). Mantém coerência com o cálculo
        -- real da antecipação no naoSpedQuery. PE por dentro; demais UFs, direto.
        WHEN m.cfop_entrada IN ('2101','2102','2152','2403','2409','2651','2652') THEN
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
    END AS icms_seria_devido
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
JOIN prodepe_enquadramentos pe
  ON pe.company_id = $1::uuid
 AND pe.ativo = true
 AND pe.dispensa_antecipacao = true
 AND regexp_replace(pe.cnpj, '[^0-9]', '', 'g')
     = regexp_replace(COALESCE(m.dest_cnpj_cpf, ''), '[^0-9]', '', 'g')
 AND (pe.vigencia_inicio IS NULL OR m.data_emissao >= pe.vigencia_inicio)
 AND (pe.vigencia_fim    IS NULL OR m.data_emissao <= pe.vigencia_fim)
WHERE m.cfop_entrada NOT IN ('2551','2556')
  AND m.cfop_entrada IN ('2101','2102','2152','2403','2409','2651','2652')
  AND ($3::text = '' OR COALESCE(m.dest_uf, 'PE') = $3)
ORDER BY m.data_emissao DESC, m.chave_nfe
`

// ---------------------------------------------------------------------------
// IcmsFronteiraIncentivoHandler — GET /api/icms-fronteira/incentivo
//
// Query params:
//   - periodo: MM/YYYY (default: vazio = todos os meses)
//   - uf:      filtra por UF da filial (default: vazio = todas)
//
// Resposta: lista unificada A+B+C com totais separados e quebra por programa
// (PRODEPE / PROIND) e por CNPJ recebedor.
// ---------------------------------------------------------------------------

func IcmsFronteiraIncentivoHandler(db *sql.DB) http.HandlerFunc {
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
		uf := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("uf")))

		// --- Bloco SPED (A + B) ---
		spedQuery := fronteiraBaseQuery + incentivoSpedSelectCols
		if uf != "" {
			spedQuery += " AND c.uf_filial = $3"
		}
		spedQuery += " ORDER BY c.bloco, c.data_emissao DESC, c.chave_nfe"

		spedArgs := []interface{}{companyID, periodo}
		if uf != "" {
			spedArgs = append(spedArgs, uf)
		}

		resp := FronteiraIncentivoResponse{
			Rows:        []FronteiraIncentivoRow{},
			PorPrograma: map[string]float64{},
			PorFilial:   map[string]float64{},
		}

		spedRows, err := db.Query(spedQuery, spedArgs...)
		if err != nil {
			log.Printf("IcmsFronteiraIncentivo[SPED] error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar incentivo (SPED): "+err.Error())
			return
		}
		for spedRows.Next() {
			var row FronteiraIncentivoRow
			// Ignorados: v_bc_st, v_st (não relevantes ao relatório de dispensa)
			var vBcST, vST sql.NullFloat64
			_ = vBcST
			_ = vST
			if err := spedRows.Scan(
				&row.ChaveNFe, &row.DataEmissao, &row.NumeroNFe,
				&row.FornCNPJ, &row.FornNome, &row.FornUF,
				&row.CFOP, &row.VProd, &row.VIPI, &row.VIcms,
				&row.AliqInter, &row.AliqInterna, &row.Regime, &row.Bloco,
				&row.CnpjFilial, &row.Programa, &row.NumAto, &row.VigenciaInicio, &row.VigenciaFim,
				&row.IcmsSeriaDevido,
			); err != nil {
				log.Printf("IcmsFronteiraIncentivo[SPED] scan: %v", err)
				continue
			}
			resp.Rows = append(resp.Rows, row)
		}
		spedRows.Close()

		// --- Bloco C (NF-e em XML sem SPED) ---
		naoSpedRows, err := db.Query(incentivoNaoSpedQuery, companyID, periodo, uf)
		if err != nil {
			log.Printf("IcmsFronteiraIncentivo[BlocoC] error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar incentivo (Bloco C): "+err.Error())
			return
		}
		for naoSpedRows.Next() {
			var row FronteiraIncentivoRow
			row.Bloco = "nao_sped"
			if err := naoSpedRows.Scan(
				&row.ChaveNFe, &row.DataEmissao, &row.NumeroNFe,
				&row.FornCNPJ, &row.FornNome, &row.FornUF,
				&row.CFOP, &row.VProd, &row.VIPI, &row.VIcms,
				&row.AliqInter, &row.AliqInterna, &row.Regime,
				&row.CnpjFilial, &row.Programa, &row.NumAto, &row.VigenciaInicio, &row.VigenciaFim,
				&row.IcmsSeriaDevido,
			); err != nil {
				log.Printf("IcmsFronteiraIncentivo[BlocoC] scan: %v", err)
				continue
			}
			resp.Rows = append(resp.Rows, row)
		}
		naoSpedRows.Close()

		// --- Agregações ---
		for _, row := range resp.Rows {
			resp.TotalDispensado += row.IcmsSeriaDevido
			switch row.Bloco {
			case "mes_atual":
				resp.TotalMesAtual += row.IcmsSeriaDevido
				resp.CountMesAtual++
			case "mes_anterior":
				resp.TotalMesAnterior += row.IcmsSeriaDevido
				resp.CountMesAnterior++
			case "nao_sped":
				resp.TotalNaoSped += row.IcmsSeriaDevido
				resp.CountNaoSped++
			}
			resp.PorPrograma[row.Programa] += row.IcmsSeriaDevido
			if row.CnpjFilial != "" {
				resp.PorFilial[row.CnpjFilial] += row.IcmsSeriaDevido
			}
		}
		resp.Count = len(resp.Rows)

		log.Printf("IcmsFronteiraIncentivo: periodo=%s uf=%s count=%d total=%s",
			periodo, uf, resp.Count, fmt.Sprintf("R$ %.2f", resp.TotalDispensado))
		json.NewEncoder(w).Encode(resp)
	}
}
