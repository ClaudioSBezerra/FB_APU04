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
// Demonstrativo de ICMS-ST POR ITEM (nova tela)
//
// GET /api/icms-fronteira/st-itens?periodo=MM/YYYY&uf=BA
//
// Diferente de /api/icms-fronteira/st (que agrega por NOTA), este endpoint
// retorna UMA LINHA POR ITEM de NF cujo CFOP é de ST (2403/2409/2651/2652).
// O cálculo de ST é feito por item, casando o NCM DO ITEM com a regra NCM
// (mesma LATERAL do fronteiraBaseQuery) e aplicando a trava de segmento.
//
// O frontend agrupa por nota (chave_nfe) — aqui só devolvemos os itens com a
// chave da nota para o agrupamento + os CT-es por nota (fetchCteLinksForNFs).
//
// Duas fontes (UNION ALL):
//  1. SPED        — reg_c170 × reg_0200 × reg_c100 × import_jobs (bloco
//                   "mes_atual" / "mes_anterior" conforme dt_doc (emissão) vs período).
//  2. XML não-SPED — nfe_entradas_itens × nfe_entradas, onde a chave NÃO está
//                   em nenhum SPED da empresa (bloco "nao_sped").
// ---------------------------------------------------------------------------

// STItemRow — uma linha por item de NF com CFOP de ST.
type STItemRow struct {
	// Campos da NOTA (para o frontend agrupar)
	ChaveNFe    string `json:"chave_nfe"`
	NumeroNFe   string `json:"numero_nfe"`
	DataEmissao string `json:"data_emissao"`
	FornCNPJ    string `json:"forn_cnpj"`
	FornNome    string `json:"forn_nome"`
	FornUF      string `json:"forn_uf"`
	CFOP        string `json:"cfop"`
	Bloco       string `json:"bloco"`      // "mes_atual" | "mes_anterior" | "nao_sped"
	UFFilial    string `json:"uf_filial"`  // UF do estabelecimento (BA/PE) — define a regra aplicável
	StatusXML   string `json:"status_xml"` // "Encontrado" | "Faltante"

	// Campos do ITEM
	CodProduto string  `json:"cod_produto"`
	Descricao  string  `json:"descricao"`
	NCM        string  `json:"ncm"`
	CEST       string  `json:"cest"`
	VProd      float64 `json:"v_prod"`
	VIPI       float64 `json:"v_ipi"`
	VOutro     float64 `json:"v_outro"`
	VOperacao  float64 `json:"v_operacao"` // v_prod + v_ipi + v_outro

	// Regra / segmento
	TemRegra     bool    `json:"tem_regra"`
	MVAOriginal  float64 `json:"mva_original"`
	MVAAjustado  float64 `json:"mva_ajustado"` // MVA efetivo calculado
	AliqInter    float64 `json:"aliq_inter"`   // alíquota interestadual do item
	AliqInterna  float64 `json:"aliq_interna"` // regra.aliquota_interna (default 20.5)
	SegmentoOK   bool    `json:"segmento_ok"`

	// Cálculo
	IcmsDebitado  float64 `json:"icms_debitado"`  // ICMS próprio (vl_icms / v_icms)
	BaseCalculo   float64 `json:"base_calculo"`   // v_operacao*(1+mva/100) quando aplicável
	ReducaoBC     float64 `json:"reducao_bc"`     // regra.reducao_bc_pct
	BCReduzida    float64 `json:"bc_reduzida"`    // base_calculo*(1-reducao_bc/100)
	IcmsCalculado float64 `json:"icms_calculado"` // bc_reduzida*aliq_interna/100 - icms_debitado (min 0)
	IcmsRetido    float64 `json:"icms_retido"`    // vl_icms_st / v_st
	IcmsAPagar    float64 `json:"icms_a_pagar"`   // max(0, icms_calculado - icms_retido)
}

type STItensResponse struct {
	Rows     []STItemRow          `json:"rows"`
	Count    int                  `json:"count"`
	CteLinks map[string][]CteLink `json:"cte_links"`
}

// stItensQuery — UNION ALL das duas fontes. Cada item casa a regra via LATERAL
// (lógica idêntica ao fronteiraBaseQuery, casando o NCM DO ITEM) e expõe os
// campos crus + o MVA efetivo já calculado. Os campos derivados (base, ICMS
// calculado, a pagar) são montados em Go a partir das colunas cruas para manter
// a query legível e o cálculo fiscal auditável em um único lugar.
//
// Placeholders: $1 company_id (uuid), $2 periodo "MM/YYYY", $3 uf.
const stItensQuery = `
WITH
latest_jobs AS (
    SELECT DISTINCT ON (company_id, COALESCE(cnpj, uf, ''), COALESCE(mes_ano, ''))
        id
    FROM import_jobs
    WHERE status = 'completed'
    ORDER BY company_id,
             COALESCE(cnpj, uf, ''),
             COALESCE(mes_ano, ''),
             created_at DESC
),
sped_itens AS (
    SELECT
        c100.chv_nfe                                        AS chave_nfe,
        COALESCE(c100.num_doc, '')                          AS numero_nfe,
        c100.dt_doc::text                                   AS data_emissao,
        COALESCE(part.cnpj, ne.forn_cnpj, '')               AS forn_cnpj,
        COALESCE(part.nome, ne.forn_nome, '')               AS forn_nome,
        COALESCE(NULLIF(ne.forn_uf, ''), NULLIF(m_part.uf, ''), '') AS forn_uf,
        ci.cfop                                             AS cfop,
        ci.num_item                                         AS num_item,
        -- Bloco pela data de EMISSÃO (dt_doc) — modelo confirmado pelo contador
        -- Gilson (2026-06-25): tanto ST quanto antecipação calculam pela emissão.
        -- Nota emitida em abril que entra no SPED de maio = Bloco A (já recolhida).
        CASE
            WHEN $2::text = ''
              OR (EXTRACT(MONTH FROM c100.dt_doc)::int = SPLIT_PART($2::text,'/',1)::int
                  AND EXTRACT(YEAR  FROM c100.dt_doc)::int = SPLIT_PART($2::text,'/',2)::int)
            THEN 'mes_atual'
            ELSE 'mes_anterior'
        END                                                 AS bloco,
        COALESCE(ci.cod_item, '')                           AS cod_produto,
        COALESCE(NULLIF(ci.descr_compl,''), p.descr_item, '') AS descricao,
        LEFT(regexp_replace(COALESCE(p.cod_ncm,''),'[^0-9]','','g'),8) AS ncm,
        COALESCE(p.cest, '')                                AS cest,
        COALESCE(ci.vl_item, 0)                             AS v_prod,
        COALESCE(ci.vl_ipi, 0)                              AS v_ipi,
        0::numeric                                          AS v_outro,
        -- Alíq. interestadual: o C170 do SPED frequentemente não traz aliq_icms
        -- por item → cai pro XML (v_icms/v_prod), senão 12.
        COALESCE(NULLIF(ci.aliq_icms,0),
                 CASE WHEN COALESCE(xi.v_prod,0) > 0 AND COALESCE(xi.v_icms,0) > 0
                      THEN ROUND((xi.v_icms / xi.v_prod * 100.0)::numeric, 2) END,
                 12.0)                                      AS aliq_inter,
        -- ICMS próprio destacado (a abater do ST): o reg_c170 do SPED em geral NÃO
        -- traz vl_icms por item (fica consolidado no C190) → prioriza o XML por item
        -- (xi.v_icms), cai pro SPED, senão 0. Sem este fallback o ICMS debitado
        -- zerava e a ST a pagar saía A MAIOR (relatado por Gilson — Rolimec BA).
        COALESCE(NULLIF(xi.v_icms,0), NULLIF(ci.vl_icms,0), 0) AS icms_debitado,
        -- ST retido: prioriza o XML por item (v_st), cai pro SPED, senão 0.
        COALESCE(NULLIF(xi.v_st,0), ci.vl_icms_st, 0)       AS icms_retido,
        COALESCE(j.uf, 'PE')                                AS uf_filial,
        -- tem_xml: a NOTA do SPED possui XML importado nesta empresa? Usado para
        -- o Bloco D (SPED sem XML / "Faltante"). Nível NOTA (ne.id), não item.
        (ne.id IS NOT NULL)                                 AS tem_xml
    FROM reg_c170 ci
    JOIN reg_c100 c100 ON c100.id = ci.c100_id
    JOIN import_jobs j  ON j.id = c100.job_id
    LEFT JOIN reg_0200 p ON p.job_id = c100.job_id AND p.cod_item = ci.cod_item
    LEFT JOIN participants part
        ON part.job_id = c100.job_id AND part.cod_part = c100.cod_part
    LEFT JOIN municipios_ibge m_part ON m_part.codigo_ibge = part.cod_mun
    LEFT JOIN nfe_entradas ne ON ne.company_id = j.company_id AND ne.chave_nfe = c100.chv_nfe
    LEFT JOIN nfe_entradas_itens xi ON xi.nfe_id = ne.id AND xi.n_item = ci.num_item
    WHERE j.company_id = $1
      AND j.id IN (SELECT id FROM latest_jobs)
      AND ci.cfop IN ('2403','2409','2651','2652')
      AND c100.cod_sit NOT IN ('02','03','04','05')
      AND ($3::text = '' OR COALESCE(j.uf,'PE') = $3)
      AND ($2::text = '' OR j.mes_ano = $2
          OR (j.mes_ano IS NULL AND (
              EXTRACT(MONTH FROM j.dt_ini)::int = SPLIT_PART($2::text,'/',1)::int
              AND EXTRACT(YEAR  FROM j.dt_ini)::int = SPLIT_PART($2::text,'/',2)::int
          ))
      )
),
xml_itens AS (
    SELECT
        ne.chave_nfe                                        AS chave_nfe,
        COALESCE(ne.numero_nfe,'')                          AS numero_nfe,
        ne.data_emissao::text                               AS data_emissao,
        COALESCE(ne.forn_cnpj,'')                           AS forn_cnpj,
        COALESCE(ne.forn_nome,'')                           AS forn_nome,
        COALESCE(ne.forn_uf,'')                             AS forn_uf,
        -- Os itens do XML trazem o CFOP do EMITENTE (saída: 6xxx/5xxx).
        -- Reclassifica para o CFOP de ENTRADA (2xxx/1xxx) para casar com o SPED
        -- e permitir o filtro de ST. (igual icms_fronteira_incentivo.go)
        CASE WHEN LEFT(nii.cfop,1)='6' THEN '2'||SUBSTRING(nii.cfop FROM 2)
             WHEN LEFT(nii.cfop,1)='5' THEN '1'||SUBSTRING(nii.cfop FROM 2)
             ELSE nii.cfop END                              AS cfop,
        nii.n_item                                          AS num_item,
        'nao_sped'                                          AS bloco,
        COALESCE(nii.c_prod,'')                             AS cod_produto,
        COALESCE(nii.x_prod,'')                             AS descricao,
        LEFT(regexp_replace(COALESCE(nii.ncm,''),'[^0-9]','','g'),8) AS ncm,
        COALESCE(nii.cest,'')                               AS cest,
        COALESCE(nii.v_prod, 0)                             AS v_prod,
        COALESCE(nii.v_ipi, 0)                              AS v_ipi,
        0::numeric                                          AS v_outro,
        -- Alíquota interestadual derivada do item: v_icms/v_prod×100, default 12.
        CASE WHEN COALESCE(nii.v_prod,0) > 0 AND COALESCE(nii.v_icms,0) > 0
             THEN ROUND((nii.v_icms / nii.v_prod * 100.0)::numeric, 2)
             ELSE 12.0 END                                  AS aliq_inter,
        COALESCE(nii.v_icms, 0)                             AS icms_debitado,
        COALESCE(nii.v_st, 0)                               AS icms_retido,
        COALESCE(ne.dest_uf,'PE')                           AS uf_filial,
        true                                                AS tem_xml
    FROM nfe_entradas_itens nii
    JOIN nfe_entradas ne ON ne.id = nii.nfe_id
    WHERE ne.company_id = $1
      AND (CASE WHEN LEFT(nii.cfop,1)='6' THEN '2'||SUBSTRING(nii.cfop FROM 2)
                WHEN LEFT(nii.cfop,1)='5' THEN '1'||SUBSTRING(nii.cfop FROM 2)
                ELSE nii.cfop END) IN ('2403','2409','2651','2652')
      AND EXTRACT(MONTH FROM ne.data_emissao)::int = SPLIT_PART($2::text,'/',1)::int
      AND EXTRACT(YEAR  FROM ne.data_emissao)::int = SPLIT_PART($2::text,'/',2)::int
      AND ($3::text = '' OR COALESCE(ne.dest_uf,'PE') = $3)
      AND NOT EXISTS (
          SELECT 1 FROM reg_c100 c100 JOIN import_jobs j ON j.id = c100.job_id
          WHERE j.company_id = $1 AND c100.chv_nfe = ne.chave_nfe
      )
),
itens AS (
    SELECT * FROM sped_itens
    UNION ALL
    SELECT * FROM xml_itens
)
SELECT
    i.chave_nfe, i.numero_nfe, i.data_emissao,
    i.forn_cnpj, i.forn_nome, i.forn_uf,
    i.cfop, i.bloco,
    i.cod_produto, i.descricao, COALESCE(i.ncm,''), i.cest,
    i.v_prod, i.v_ipi, i.v_outro,
    i.aliq_inter,
    i.icms_debitado, i.icms_retido,
    (regra.aliquota_interna IS NOT NULL) AS tem_regra,
    COALESCE(regra.aliquota_interna, 20.5) AS aliq_interna,
    COALESCE(regra.mva_original, 0)        AS mva_original,
    COALESCE(regra.reducao_bc_pct, 0)      AS reducao_bc,
    -- segmento_ok: regra com segmento_codigo E empresa tem o segmento na UF.
    (regra.segmento_codigo IS NOT NULL
     AND EXISTS (
         SELECT 1 FROM company_segmentos cs
         WHERE cs.company_id = $1::uuid
           AND cs.segmento_codigo = regra.segmento_codigo
           AND cs.uf = i.uf_filial
     )) AS segmento_ok,
    -- MVA efetivo: ajustado pré-calc por alíq interestadual real, fallback
    -- Convênio 110/07 a partir do MVA original, fallback MVA original.
    COALESCE(
        CASE i.aliq_inter
            WHEN 4.0  THEN regra.mva_ajustado_4pct
            WHEN 7.0  THEN regra.mva_ajustado_7pct
            WHEN 12.0 THEN regra.mva_ajustado_12pct
        END,
        CASE WHEN regra.mva_original IS NOT NULL AND COALESCE(regra.aliquota_interna,20.5) < 100 THEN
            ((1.0 + regra.mva_original/100.0) * (1.0 - i.aliq_inter/100.0)
             / NULLIF(1.0 - COALESCE(regra.aliquota_interna,20.5)/100.0, 0) - 1.0) * 100.0
        END,
        regra.mva_original,
        0
    ) AS mva_ajustado,
    i.tem_xml,
    i.uf_filial
FROM itens i
LEFT JOIN LATERAL (
    SELECT r.aliquota_interna, r.mva_original,
           r.mva_ajustado_4pct, r.mva_ajustado_7pct, r.mva_ajustado_12pct,
           r.reducao_bc_pct, r.segmento_codigo
    FROM icms_fronteira_regras_ncm r
    WHERE (r.company_id = $1 OR r.company_id IS NULL)
      AND r.uf_estado = i.uf_filial
      AND NULLIF(i.ncm,'') IS NOT NULL
      AND LEFT(i.ncm, LENGTH(r.ncm_prefixo)) = r.ncm_prefixo
      AND LENGTH(r.ncm_prefixo) >= 4
    ORDER BY r.company_id NULLS LAST, LENGTH(r.ncm_prefixo) DESC
    LIMIT 1
) regra ON true
ORDER BY i.bloco, i.data_emissao, i.chave_nfe, i.num_item
`

// IcmsFronteiraSTItensHandler — GET /api/icms-fronteira/st-itens
//
//	Parâmetros: periodo (MM/YYYY), uf (opcional).
func IcmsFronteiraSTItensHandler(db *sql.DB) http.HandlerFunc {
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

		periodo := strings.TrimSpace(r.URL.Query().Get("periodo"))
		uf := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("uf")))

		result, err := fetchSTItens(db, companyID, periodo, uf)
		if err != nil {
			log.Printf("IcmsFronteiraSTItens error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar itens de ST")
			return
		}

		chaves := make([]string, len(result))
		for i, rw := range result {
			chaves[i] = rw.ChaveNFe
		}
		cteLinks := fetchCteLinksForNFs(db, companyID, chaves)
		// Rateio do frete: pré-escala cada CT-e pela fração de ST da nota, para a
		// TELA de ST não contar o frete cheio (que já é contado na antecipação).
		// O rateio interno por item (somaOper) no frontend continua igual.
		cteLinks = scaleCteMapForRegime(cteLinks, fetchCteRateioFactors(db, companyID, periodo, chaves), "ST")

		json.NewEncoder(w).Encode(STItensResponse{
			Rows:     result,
			Count:    len(result),
			CteLinks: cteLinks,
		})
	}
}

// fetchSTItens executa stItensQuery, faz o scan e aplica o cálculo de ST por
// item (base, BC reduzida, ICMS calculado, ICMS a pagar). Fonte única do cálculo
// fiscal — reusada pelo handler JSON e pelos exports (XLSX/PDF).
// computeST preenche os campos derivados (VOperacao, BaseCalculo, BCReduzida,
// IcmsCalculado, IcmsAPagar) a partir dos valores crus + regra. A base de ST só
// existe quando há regra E o segmento da empresa casou; senão a ST não se aplica
// (base e derivados ficam zerados). ICMS calculado e a pagar têm piso 0.
func (row *STItemRow) computeST() {
	row.VOperacao = row.VProd + row.VIPI + row.VOutro
	if row.TemRegra && row.SegmentoOK {
		row.BaseCalculo = row.VOperacao * (1.0 + row.MVAAjustado/100.0)
	} else {
		row.BaseCalculo = 0
	}
	row.BCReduzida = row.BaseCalculo * (1.0 - row.ReducaoBC/100.0)
	row.IcmsCalculado = row.BCReduzida*row.AliqInterna/100.0 - row.IcmsDebitado
	if row.IcmsCalculado < 0 {
		row.IcmsCalculado = 0
	}
	row.IcmsAPagar = row.IcmsCalculado - row.IcmsRetido
	if row.IcmsAPagar < 0 {
		row.IcmsAPagar = 0
	}
}

func fetchSTItens(db *sql.DB, companyID, periodo, uf string) ([]STItemRow, error) {
	rows, err := db.Query(stItensQuery, companyID, periodo, uf)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []STItemRow{}
	for rows.Next() {
		var row STItemRow
		var temXML bool
		if err := rows.Scan(
			&row.ChaveNFe, &row.NumeroNFe, &row.DataEmissao,
			&row.FornCNPJ, &row.FornNome, &row.FornUF,
			&row.CFOP, &row.Bloco,
			&row.CodProduto, &row.Descricao, &row.NCM, &row.CEST,
			&row.VProd, &row.VIPI, &row.VOutro,
			&row.AliqInter,
			&row.IcmsDebitado, &row.IcmsRetido,
			&row.TemRegra, &row.AliqInterna, &row.MVAOriginal, &row.ReducaoBC,
			&row.SegmentoOK, &row.MVAAjustado,
			&temXML, &row.UFFilial,
		); err != nil {
			log.Printf("fetchSTItens scan error: %v", err)
			continue
		}

		if temXML {
			row.StatusXML = "Encontrado"
		} else {
			row.StatusXML = "Faltante"
		}
		row.computeST()
		result = append(result, row)
	}
	return result, nil
}
