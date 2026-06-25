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

// fronteiraFiltros monta o WHERE adicional (UF da filial, fornecedor, número da
// nota, intervalo de data) a partir dos query params, com placeholders posicionais
// a partir de startIdx. Retorna o fragmento SQL (começando com " AND ...") e os
// argumentos. As colunas referenciadas (uf_filial/forn_cnpj/forn_nome/numero_nfe/
// data_emissao) existem no CTE classified do fronteiraBaseQuery.
// fronteiraInaplicAtivo lê o flag do simulador (?inaplic=1). HOOK da Fase 2 do
// motor de inaplicabilidade: quando true, futuramente as regras aprovadas e
// auto-aplicáveis serão aplicadas ao cálculo (excluir/zerar notas que casam).
// HOJE é apenas lido e encanado — NO-OP. O resultado é idêntico com ou sem o
// flag, garantindo risco zero em produção até a lógica da Fase 2 ser implementada.
func fronteiraInaplicAtivo(r *http.Request) bool {
	v := strings.TrimSpace(r.URL.Query().Get("inaplic"))
	return v == "1" || v == "true"
}

func fronteiraFiltros(r *http.Request, startIdx int) (string, []interface{}) {
	var sb strings.Builder
	var args []interface{}
	idx := startIdx

	// Eixo UF: quando informado, restringe às filiais (estabelecimentos) da UF.
	if uf := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("uf"))); uf != "" {
		sb.WriteString(fmt.Sprintf(" AND uf_filial = $%d", idx))
		args = append(args, uf)
		idx++
	}
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
	"2101", "2102", "2152", // Antecipação sem liberação
	"2403", "2409", "2651", "2652", // ST (antecipação com liberação)
	"2551", "2556", // DIFAL
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
	Regime        string  `json:"regime"`
	QtdNotas      int     `json:"qtd_notas"`
	VProdTotal    float64 `json:"v_prod_total"`
	VIpiTotal     float64 `json:"v_ipi_total"`
	VStRetido     float64 `json:"v_st_retido"`
	IcmsDevidoEst float64 `json:"icms_devido_est"`
}

type FronteiraResumoResponse struct {
	Rows        []FronteiraResumoRow `json:"rows"`
	TotalDevido float64              `json:"total_devido"`
	TotalProd   float64              `json:"total_prod"`
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
	VFrete        float64 `json:"v_frete"` // frete da NF rateado (cadeia antecipação)
	VOutro        float64 `json:"v_outro"` // outras despesas da NF rateadas
	AliqInter     float64 `json:"aliq_inter"`
	AliqInterna   float64 `json:"aliq_interna"`
	IcmsDevidoEst float64 `json:"icms_devido_est"` // ICMS a pagar (devido − ICMS destacado)
	ValorDevido   float64 `json:"valor_devido"`    // V. Devido bruto (antecipação)
	BasePorDentro bool    `json:"base_por_dentro"` // UF usa cálculo "por dentro" (ex.: PE)
	Regime        string  `json:"regime"`
	Bloco         string  `json:"bloco"`
}

// CteLink — CT-e vinculado a uma NF-e (apenas quando tomador = destinatário)
type CteLink struct {
	ChaveCTe    string  `json:"chave_cte"`
	NumeroCTe   string  `json:"numero_cte"`
	DataEmissao string  `json:"data_emissao"`
	EmitNome    string  `json:"emit_nome"`
	EmitCNPJ    string  `json:"emit_cnpj"`
	VPrest      float64 `json:"v_prest"`
	VIcmsCTe    float64 `json:"v_icms_cte"`
}

type FronteiraNotasResponse struct {
	Rows             []FronteiraNotaRow   `json:"rows"`
	Total            float64              `json:"total"`
	Count            int                  `json:"count"`
	TotalMesAtual    float64              `json:"total_mes_atual"`
	TotalMesAnterior float64              `json:"total_mes_anterior"`
	CountMesAtual    int                  `json:"count_mes_atual"`
	CountMesAnterior int                  `json:"count_mes_anterior"`
	CteLinks         map[string][]CteLink `json:"cte_links"`
}

// ---------------------------------------------------------------------------
// SQL helpers
// ---------------------------------------------------------------------------

// fronteiraBaseQuery — SELECT que classifica cada nota e calcula o ICMS estimado.
// Os CTEs pesados (reg_c170 × reg_c190 × reg_c100 × nfe_entradas × nfe_entradas_itens)
// são pré-computados em mv_icms_fronteira_linhas (migration 126) e refrescados pelo
// worker SPED após cada import. A query aqui aplica apenas a lógica de negócio
// (PRODEPE, ST vs ANTECIPAÇÃO, regras NCM, DIFAL) sobre a MV já materializada.
const fronteiraBaseQuery = `
WITH
classified AS (
    SELECT
        c100.chv_nfe                                        AS chave_nfe,
        c100.dt_doc::text                                   AS data_emissao,
        COALESCE(c100.num_doc, '')                          AS numero_nfe,
        COALESCE(part.cnpj, ne.forn_cnpj, '')               AS forn_cnpj,
        COALESCE(part.nome, ne.forn_nome, '')               AS forn_nome,
        -- forn_uf: 1º o XML (mais preciso), senão a UF resolvida via município
        -- do participante (reg 0150 do SPED → cod_mun → municipios_ibge.uf).
        -- Sem o fallback, o Bloco A (NFs de meses anteriores no SPED) ficava
        -- com UF vazia quando o XML do fornecedor não estava importado.
        COALESCE(NULLIF(ne.forn_uf, ''), NULLIF(m_part.uf, ''), '') AS forn_uf,
        l.cfop                                              AS cfop,
        l.v_prod_disp                                       AS v_prod,
        COALESCE(l.ipi_eff, 0)                              AS v_ipi,
        -- Crédito interestadual: prioriza vl_icms_inter (SUM de bc×aliq por linha
        -- do C190, cap 12%) que preserva mix de alíquotas dentro do mesmo CFOP.
        -- Fallback: v_prod × aliq quando vl_bc_icms não foi preenchido no SPED.
        COALESCE(
            NULLIF(l.vl_icms_inter, 0),
            l.v_prod_disp * COALESCE(NULLIF(l.aliq_icms, 0), 12.0) / 100.0
        ) AS v_icms,
        COALESCE(l.vl_bc_st, 0)                             AS v_bc_st,
        COALESCE(l.vl_icms_st, 0)                           AS v_st,
        -- Frete/Outras da NF rateados por item (cadeia Gilson — Blocos A/B iguais ao C).
        COALESCE(l.frete_rat, 0)                            AS v_frete,
        COALESCE(l.outro_rat, 0)                            AS v_outro,
        COALESCE(NULLIF(l.aliq_icms, 0), 12.0)              AS aliq_inter,
        COALESCE(regra.aliquota_interna, 20.5)              AS aliq_interna,
        -- ST só se aplica quando a regra NCM tem segmento_codigo cadastrado E a
        -- empresa tem esse segmento registrado (company_segmentos). Sem match →
        -- reclassifica como ANTECIPAÇÃO (decisão do contador, 2026-05).
        CASE
            WHEN l.cfop IN ('2551','2556')
                THEN 'DIFAL'
            WHEN l.cfop IN ('2403','2409','2651','2652')
                THEN CASE
                    WHEN regra.segmento_codigo IS NOT NULL
                      AND EXISTS (
                          SELECT 1 FROM company_segmentos cs
                          WHERE cs.company_id = $1::uuid
                            AND cs.segmento_codigo = regra.segmento_codigo
                            AND cs.uf = COALESCE(j.uf, 'PE')
                      )
                    THEN 'ST'
                    ELSE 'ANTECIPACAO'
                END
            WHEN l.cfop IN ('2101','2102','2152')
                THEN 'ANTECIPACAO'
        END                                                 AS regime,
        -- Bloco pela data de EMISSÃO (dt_doc) — modelo confirmado pelo contador
        -- Gilson (2026-06-25): tanto antecipação quanto ST calculam pela emissão.
        -- Nota emitida em abril que aparece no SPED de maio = Bloco A (já recolhida
        -- na emissão). Bloco B = emitida no mês corrente e no SPED.
        CASE
            WHEN $2::text = ''
              OR (EXTRACT(MONTH FROM c100.dt_doc)::int = SPLIT_PART($2::text,'/',1)::int
                  AND EXTRACT(YEAR  FROM c100.dt_doc)::int = SPLIT_PART($2::text,'/',2)::int)
            THEN 'mes_atual'
            ELSE 'mes_anterior'
        END                                                 AS bloco,
        -- ICMS devido estimado por regime. Base = l.base_calc (já inclui IPI/
        -- frete quando há XML, ou vl_opr do SPED quando não há).
        CASE
            -- PRODEPE / regime especial de central de distribuição (art. 11-A do
            -- Dec. 21.959/1999): a filial beneficiada é DISPENSADA de antecipação
            -- E de ST nas aquisições → ICMS fronteira = 0. Identificação por CNPJ
            -- da filial recebedora (import_jobs.cnpj) com vigência cobrindo a data
            -- do documento. DIFAL (2551/2556) fica FORA da dispensa. EXISTS evita
            -- multiplicar linhas quando há mais de um enquadramento p/ o mesmo CNPJ.
            -- O regime classificado é preservado — só o valor é zerado.
            WHEN l.cfop NOT IN ('2551','2556')
             AND EXISTS (
                 SELECT 1 FROM prodepe_enquadramentos pe
                 WHERE pe.company_id = $1
                   AND pe.ativo = true
                   AND pe.dispensa_antecipacao = true
                   AND regexp_replace(pe.cnpj, '[^0-9]', '', 'g')
                       = regexp_replace(COALESCE(j.cnpj, ''), '[^0-9]', '', 'g')
                   AND (pe.vigencia_inicio IS NULL OR c100.dt_doc >= pe.vigencia_inicio)
                   AND (pe.vigencia_fim    IS NULL OR c100.dt_doc <= pe.vigencia_fim)
             )
                THEN 0
            WHEN l.cfop IN ('2551','2556')
                THEN CASE WHEN COALESCE(ufb.base_por_dentro, false)
                    -- DIFAL por dentro (PE): base = (operação − crédito inter.) /
                    -- (1 − alíq_interna), aplicada à diferença de alíquotas, sem dedução.
                    THEN GREATEST(0,
                        ((l.base_calc - COALESCE(NULLIF(l.vl_icms_inter,0), l.v_prod_disp * COALESCE(NULLIF(l.aliq_icms,0),12.0) / 100.0))
                         / NULLIF(1.0 - COALESCE(regra.aliquota_interna,20.5)/100.0, 0))
                        * (COALESCE(regra.aliquota_interna,20.5) - COALESCE(NULLIF(l.aliq_icms,0),12.0)) / 100.0)
                    ELSE GREATEST(0,
                        l.base_calc * (
                            COALESCE(regra.aliquota_interna, 20.5)
                            - COALESCE(NULLIF(l.aliq_icms, 0), 12.0)
                        ) / 100.0)
                END
            WHEN l.cfop IN ('2403','2409','2651','2652')
                THEN CASE
                    -- ST: segmento da empresa coincide com o da regra NCM
                    WHEN regra.segmento_codigo IS NOT NULL
                      AND EXISTS (
                          SELECT 1 FROM company_segmentos cs
                          WHERE cs.company_id = $1::uuid
                            AND cs.segmento_codigo = regra.segmento_codigo
                            AND cs.uf = COALESCE(j.uf, 'PE')
                      )
                    THEN CASE
                        -- MVA efetivo: ajustado pré-calc por alíquota interestadual real,
                        -- fallback Convênio 110/07 a partir do MVA original, fallback MVA original.
                        WHEN COALESCE(
                            CASE COALESCE(NULLIF(l.aliq_icms,0),12.0)
                                WHEN 4.0  THEN regra.mva_ajustado_4pct
                                WHEN 7.0  THEN regra.mva_ajustado_7pct
                                WHEN 12.0 THEN regra.mva_ajustado_12pct
                            END,
                            CASE WHEN regra.mva_original IS NOT NULL AND COALESCE(regra.aliquota_interna,20.5) < 100 THEN
                                ((1.0 + regra.mva_original/100.0) * (1.0 - COALESCE(NULLIF(l.aliq_icms,0),12.0)/100.0)
                                 / NULLIF(1.0 - COALESCE(regra.aliquota_interna,20.5)/100.0, 0) - 1.0) * 100.0
                            END,
                            regra.mva_original
                        ) IS NOT NULL
                            THEN GREATEST(0,
                                l.base_calc
                                * (1.0 + COALESCE(
                                    CASE COALESCE(NULLIF(l.aliq_icms,0),12.0)
                                        WHEN 4.0  THEN regra.mva_ajustado_4pct
                                        WHEN 7.0  THEN regra.mva_ajustado_7pct
                                        WHEN 12.0 THEN regra.mva_ajustado_12pct
                                    END,
                                    CASE WHEN regra.mva_original IS NOT NULL AND COALESCE(regra.aliquota_interna,20.5) < 100 THEN
                                        ((1.0 + regra.mva_original/100.0) * (1.0 - COALESCE(NULLIF(l.aliq_icms,0),12.0)/100.0)
                                         / NULLIF(1.0 - COALESCE(regra.aliquota_interna,20.5)/100.0, 0) - 1.0) * 100.0
                                    END,
                                    regra.mva_original
                                )/100.0)
                                * COALESCE(regra.aliquota_interna, 20.5)/100.0
                                - COALESCE(NULLIF(l.vl_icms_inter,0), l.v_prod_disp * COALESCE(NULLIF(l.aliq_icms, 0), 12.0) / 100.0))
                        ELSE COALESCE(l.vl_icms_st, 0)
                    END
                    -- Sem segmento cadastrado → reclassificado como ANTECIPAÇÃO
                    -- (regra Gilson: IPI integra a base — base + ipi_eff)
                    ELSE CASE WHEN COALESCE(ufb.base_por_dentro, false)
                        THEN GREATEST(0,
                            ((l.base_calc + l.ipi_eff - COALESCE(NULLIF(l.vl_icms_inter,0), l.v_prod_disp * COALESCE(NULLIF(l.aliq_icms,0),12.0) / 100.0))
                             / NULLIF(1.0 - COALESCE(regra.aliquota_interna,20.5)/100.0, 0))
                            * COALESCE(regra.aliquota_interna,20.5)/100.0
                            - COALESCE(NULLIF(l.vl_icms_inter,0), l.v_prod_disp * COALESCE(NULLIF(l.aliq_icms,0),12.0) / 100.0))
                        ELSE GREATEST(0,
                            (l.base_calc + l.ipi_eff) * COALESCE(regra.aliquota_interna, 20.5)/100.0
                            - COALESCE(NULLIF(l.vl_icms_inter,0), l.v_prod_disp * COALESCE(NULLIF(l.aliq_icms, 0), 12.0) / 100.0))
                    END
                END
            WHEN l.cfop IN ('2101','2102','2152')
                -- Antecipação (regra Gilson: IPI integra a base — base + ipi_eff).
                -- Por dentro (PE): base = (operação − crédito inter.) /
                -- (1 − alíq_interna), depois × alíq_interna − crédito inter.
                THEN CASE WHEN COALESCE(ufb.base_por_dentro, false)
                    THEN GREATEST(0,
                        ((l.base_calc + l.ipi_eff - COALESCE(NULLIF(l.vl_icms_inter,0), l.v_prod_disp * COALESCE(NULLIF(l.aliq_icms,0),12.0) / 100.0))
                         / NULLIF(1.0 - COALESCE(regra.aliquota_interna,20.5)/100.0, 0))
                        * COALESCE(regra.aliquota_interna,20.5)/100.0
                        - COALESCE(NULLIF(l.vl_icms_inter,0), l.v_prod_disp * COALESCE(NULLIF(l.aliq_icms,0),12.0) / 100.0))
                    ELSE GREATEST(0,
                        (l.base_calc + l.ipi_eff) * COALESCE(regra.aliquota_interna, 20.5)/100.0
                        - COALESCE(NULLIF(l.vl_icms_inter,0), l.v_prod_disp * COALESCE(NULLIF(l.aliq_icms, 0), 12.0) / 100.0))
                END
            ELSE 0
        END                                                 AS icms_devido_est,
        -- V. Devido BRUTO (cadeia Gilson, antes de abater o ICMS destacado).
        -- Só antecipação (PE por dentro / demais direto), com IPI na base; 0 no resto.
        CASE
            WHEN l.cfop IN ('2101','2102','2152','2403','2409','2651','2652') THEN
                CASE WHEN COALESCE(ufb.base_por_dentro, false)
                    THEN GREATEST(0,
                        ((l.base_calc + l.ipi_eff - COALESCE(NULLIF(l.vl_icms_inter,0), l.v_prod_disp * COALESCE(NULLIF(l.aliq_icms,0),12.0) / 100.0))
                         / NULLIF(1.0 - COALESCE(regra.aliquota_interna,20.5)/100.0, 0))
                        * COALESCE(regra.aliquota_interna,20.5)/100.0)
                    ELSE GREATEST(0,
                        (l.base_calc + l.ipi_eff) * COALESCE(regra.aliquota_interna,20.5)/100.0)
                END
            ELSE 0
        END                                                 AS valor_devido,
        COALESCE(j.uf, 'PE')                                AS uf_filial,
        -- Campos crus expostos para o relatório "Incentivo" recalcular o
        -- icms_que_seria_devido (sem o branch PRODEPE) e fazer JOIN por CNPJ.
        -- Nenhum SELECT atual referencia estas colunas — adição inócua.
        COALESCE(j.cnpj, '')                                AS cnpj_filial,
        l.base_calc                                         AS base_calc,
        regra.aliquota_interna                              AS regra_aliq_interna,
        regra.mva_original                                  AS regra_mva_original,
        regra.mva_ajustado_4pct                             AS regra_mva_4,
        regra.mva_ajustado_7pct                             AS regra_mva_7,
        regra.mva_ajustado_12pct                            AS regra_mva_12,
        regra.segmento_codigo                               AS regra_seg_codigo,
        COALESCE(ufb.base_por_dentro, false)                AS base_por_dentro
    FROM mv_icms_fronteira_linhas l
    JOIN reg_c100 c100 ON c100.id = l.c100_id
    JOIN import_jobs j ON j.id = c100.job_id
    LEFT JOIN participants part
        ON part.job_id = c100.job_id AND part.cod_part = c100.cod_part
    LEFT JOIN municipios_ibge m_part ON m_part.codigo_ibge = part.cod_mun
    LEFT JOIN nfe_entradas ne ON ne.company_id = j.company_id AND ne.chave_nfe = c100.chv_nfe
    -- NCM efetivo: 1º o ncm_8 do MV (reg_0200 do SPED), 2º fallback XML quando
    -- o SPED não tem NCM para o produto (ncm_8 = ''). Cada row do MV já representa
    -- um NCM distinto, então o lookup de regra é sempre correto por NCM.
    LEFT JOIN LATERAL (
        SELECT COALESCE(
            NULLIF(l.ncm_8, ''),
            (SELECT nii.ncm
             FROM nfe_entradas_itens nii
             WHERE nii.nfe_id = ne.id AND NULLIF(nii.ncm, '') IS NOT NULL
             ORDER BY nii.v_prod DESC NULLS LAST
             LIMIT 1)
        ) AS ncm
    ) ncm_eff ON true
    LEFT JOIN LATERAL (
        SELECT r.aliquota_interna, r.mva_original,
               r.mva_ajustado_4pct, r.mva_ajustado_7pct, r.mva_ajustado_12pct,
               r.segmento_codigo
        FROM icms_fronteira_regras_ncm r
        WHERE (r.company_id = $1 OR r.company_id IS NULL)
          AND r.uf_estado = COALESCE(j.uf, 'PE')
          AND ncm_eff.ncm IS NOT NULL
          AND LEFT(ncm_eff.ncm, LENGTH(r.ncm_prefixo)) = r.ncm_prefixo
          AND LENGTH(r.ncm_prefixo) >= 4
        ORDER BY r.company_id NULLS LAST, LENGTH(r.ncm_prefixo) DESC
        LIMIT 1
    ) regra ON true
    LEFT JOIN uf_beneficios_fiscais ufb
        ON ufb.company_id = $1 AND ufb.uf = COALESCE(j.uf, 'PE')
    WHERE l.company_id = $1
      AND c100.cod_sit NOT IN ('02','03','04','05')
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

// IcmsFronteiraUFsHandler — GET /api/icms-fronteira/ufs
// Lista as UFs das filiais (estabelecimentos) da empresa, derivadas do reg 0000
// do SPED (import_jobs.uf). Alimenta o seletor de UF do módulo Fronteira.
func IcmsFronteiraUFsHandler(db *sql.DB) http.HandlerFunc {
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
			jsonErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		rows, err := db.Query(`
			SELECT DISTINCT uf
			FROM import_jobs
			WHERE company_id = $1::uuid AND uf IS NOT NULL AND uf <> ''
			ORDER BY uf`, companyID)
		if err != nil {
			log.Printf("IcmsFronteiraUFs error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao listar UFs")
			return
		}
		defer rows.Close()
		ufs := []string{}
		for rows.Next() {
			var uf string
			if err := rows.Scan(&uf); err == nil {
				ufs = append(ufs, uf)
			}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"ufs": ufs})
	}
}

// IcmsFronteiraPeriodosHandler — GET /api/icms-fronteira/periodos
// Lista os períodos (mes_ano "MM/YYYY") com SPED importado para a empresa, do mais
// recente para o mais antigo. Alimenta o seletor de período do módulo Fronteira e
// permite que o frontend escolha um período default — evitando carregar TODOS os
// meses de uma vez no mount (que dispara a varredura completa do classified CTE).
func IcmsFronteiraPeriodosHandler(db *sql.DB) http.HandlerFunc {
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
			jsonErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		// Período efetivo = mes_ano ("MM/YYYY") quando preenchido, senão derivado de
		// dt_ini — mesmo fallback do classified CTE, para jobs antigos sem mes_ano.
		// Ordena por ano/mês reais (não alfabético) para que "01/2026" não venha
		// antes de "12/2025".
		rows, err := db.Query(`
			SELECT periodo FROM (
			    SELECT COALESCE(NULLIF(mes_ano, ''), to_char(dt_ini, 'MM/YYYY')) AS periodo
			    FROM import_jobs
			    WHERE company_id = $1::uuid AND (mes_ano IS NOT NULL OR dt_ini IS NOT NULL)
			) p
			WHERE periodo IS NOT NULL AND periodo <> ''
			GROUP BY periodo
			ORDER BY SPLIT_PART(periodo, '/', 2) DESC, SPLIT_PART(periodo, '/', 1) DESC`, companyID)
		if err != nil {
			log.Printf("IcmsFronteiraPeriodos error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao listar períodos")
			return
		}
		defer rows.Close()
		periodos := []string{}
		for rows.Next() {
			var p string
			if err := rows.Scan(&p); err == nil {
				periodos = append(periodos, p)
			}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"periodos": periodos})
	}
}

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

		// Fase 2 (fatia segura): aplica regras aprovadas+auto de inaplicabilidade
		// quando o flag do simulador está ON. OFF ou sem regras → SQL idêntica.
		var inaplicSQL string
		if fronteiraInaplicAtivo(r) {
			ufParam := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("uf")))
			cstVals, aplicaVlSt := loadInaplicSafe(db, ufParam)
			inaplicSQL = inaplicCond(cstVals, aplicaVlSt)
		}
		icmsExpr := icmsDevidoExpr(inaplicSQL)

		query := fronteiraBaseQuery + `
SELECT
    regime,
    COUNT(DISTINCT chave_nfe) AS qtd_notas,
    SUM(v_prod)         AS v_prod_total,
    SUM(v_ipi)          AS v_ipi_total,
    SUM(v_st)           AS v_st_retido,
    SUM(` + icmsExpr + `) AS icms_devido_est
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

	// Fase 2 (fatia segura): aplica inaplicabilidade aprovada quando o flag está ON.
	var inaplicSQL string
	if fronteiraInaplicAtivo(r) {
		ufParam := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("uf")))
		cstVals, aplicaVlSt := loadInaplicSafe(db, ufParam)
		inaplicSQL = inaplicCond(cstVals, aplicaVlSt)
	}
	icmsExpr := icmsDevidoExpr(inaplicSQL)

	// G14: window functions retornam totais do conjunto completo (sem LIMIT),
	// resolvendo o bug onde totais exibidos só refletiam as primeiras 500 notas.
	// bloco classifica cada nota em "mes_atual" ou "mes_anterior" conforme a data
	// de EMISSÃO (dt_doc) — paga-se a antecipação no mês de emissão da nota.
	query := fronteiraBaseQuery + `
SELECT
    chave_nfe, data_emissao, numero_nfe, forn_cnpj, forn_nome, forn_uf,
    cfop, v_prod, v_ipi, v_icms, v_bc_st, v_st, v_frete, v_outro,
    aliq_inter, aliq_interna, ` + icmsExpr + ` AS icms_devido_est, valor_devido, base_por_dentro, regime, bloco,
    COUNT(*)            OVER () AS total_count,
    SUM(` + icmsExpr + `) OVER () AS total_full
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
			&row.VFrete, &row.VOutro,
			&row.AliqInter, &row.AliqInterna, &row.IcmsDevidoEst, &row.ValorDevido, &row.BasePorDentro, &row.Regime,
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

	chaves := make([]string, len(result))
	for i, r := range result {
		chaves[i] = r.ChaveNFe
	}
	cteLinks := fetchCteLinksForNFs(db, companyID, chaves)
	// Rateio do frete: como este handler é mono-regime ($3), pré-escala cada CT-e
	// pela fração do regime na nota — sem isto o frete do CT-e era contado CHEIO
	// em antecipação E em ST da mesma nota (duplicidade; vide CteRateio).
	cteLinks = scaleCteMapForRegime(cteLinks, fetchCteRateioFactors(db, companyID, periodo, chaves), regime)

	json.NewEncoder(w).Encode(FronteiraNotasResponse{
		Rows:             result,
		Total:            totalFull,
		Count:            totalCount,
		TotalMesAtual:    totalMesAtual,
		TotalMesAnterior: totalMesAnterior,
		CountMesAtual:    countMesAtual,
		CountMesAnterior: countMesAnterior,
		CteLinks:         cteLinks,
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
