package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
)

// ---------------------------------------------------------------------------
// Reconciliação SPED × XML — "Notas sobrando e faltando" (requisito do contador)
//
// O ICMS antecipado é devido pela DATA DE EMISSÃO da NF, mas o SPED organiza por
// DATA DE ENTRADA (recebimento). Isso gera 3 blocos para o mês de análise M:
//
//   1. normal               → SPED, emitida em M (dt_doc em M)
//   2. emitida_mes_anterior → SPED deste mês (dt_e_s em M) mas emitida antes
//                             (dt_doc < M). ALERTA: imposto provavelmente já
//                             recolhido no mês de emissão — verificar.
//   3. nao_localizada_sped  → XML, emitida em M, ausente do SPED. Classificada
//                             pelo mapeamento determinístico CFOP saída→entrada
//                             (6xxx→2xxx). Soma ao cálculo do mês.
// ---------------------------------------------------------------------------

type ReconNota struct {
	ChaveNFe      string  `json:"chave_nfe"`
	DataEmissao   string  `json:"data_emissao"`
	DataEntrada   string  `json:"data_entrada"`
	NumeroNFe     string  `json:"numero_nfe"`
	FornCNPJ      string  `json:"forn_cnpj"`
	FornNome      string  `json:"forn_nome"`
	FornUF        string  `json:"forn_uf"`
	CFOP          string  `json:"cfop"`
	CFOPEntrada   string  `json:"cfop_entrada"`   // bloco 3: CFOP entrada derivado
	Regime        string  `json:"regime"`
	ClassStatus   string  `json:"class_status,omitempty"` // auto | manual | excluded (só bloco 3)
	VOpr          float64 `json:"v_opr"`
	IcmsDevidoEst float64 `json:"icms_devido_est"`
	Origem        string  `json:"origem"` // "sped" | "xml"
	Alerta        string  `json:"alerta,omitempty"`
	TemXML        bool    `json:"tem_xml"` // XML importado para esta nota?
}

type ReconBlock struct {
	Rows  []ReconNota `json:"rows"`
	Total float64     `json:"total"`
	Count int         `json:"count"`
}

type ReconciliacaoResponse struct {
	Periodo            string     `json:"periodo"`
	Normal             ReconBlock `json:"normal"`
	EmitidaMesAnterior ReconBlock `json:"emitida_mes_anterior"`
	SpedSemXML         ReconBlock `json:"sped_sem_xml"`        // SPED sem XML importado
	NaoLocalizadaSped  ReconBlock `json:"nao_localizada_sped"` // XML sem SPED
}

// Bloco 1 (normal) e Bloco 2 (emitida mês anterior) saem do SPED.
// O discriminador é a comparação dt_doc (emissão) × período de análise.
//   bloco = 'normal'               quando mês/ano de dt_doc == período
//   bloco = 'emitida_mes_anterior' quando dt_e_s no período mas dt_doc anterior
const reconSpedQuery = `
WITH sped AS (
    SELECT
        c100.chv_nfe                                  AS chave_nfe,
        c100.dt_doc                                   AS dt_doc,
        c100.dt_e_s                                   AS dt_e_s,
        COALESCE(c100.num_doc, '')                    AS numero_nfe,
        c100.cod_part                                 AS cod_part,
        c100.job_id                                   AS job_id,
        c190.cfop                                     AS cfop,
        COALESCE(c190.vl_opr, 0)                      AS vl_opr,
        COALESCE(c190.vl_icms, 0)                     AS vl_icms,
        COALESCE(c190.vl_icms_st, 0)                  AS vl_icms_st,
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
)
SELECT
    s.chave_nfe,
    s.dt_doc::text  AS data_emissao,
    s.dt_e_s::text  AS data_entrada,
    s.numero_nfe,
    COALESCE(part.cnpj, ne.forn_cnpj, '')  AS forn_cnpj,
    COALESCE(part.nome, ne.forn_nome, '')  AS forn_nome,
    COALESCE(ne.forn_uf, '')               AS forn_uf,
    s.cfop,
    s.regime,
    s.vl_opr,
    CASE
        WHEN s.regime = 'DIFAL' THEN
            GREATEST(0, s.vl_opr * (COALESCE(regra.aliquota_interna,20.5) - s.aliq_inter)/100.0)
        WHEN s.regime = 'ST' THEN
            CASE WHEN COALESCE(regra.mva_original, regra.mva_ajustado_12pct) IS NOT NULL
                THEN GREATEST(0, s.vl_opr * (1.0 + COALESCE(regra.mva_original, regra.mva_ajustado_12pct)/100.0)
                     * COALESCE(regra.aliquota_interna,20.5)/100.0 - s.vl_icms)
                ELSE s.vl_icms_st END
        WHEN s.regime = 'ANTECIPACAO' THEN
            GREATEST(0, s.vl_opr * COALESCE(regra.aliquota_interna,20.5)/100.0 - s.vl_icms)
        ELSE 0
    END AS icms_devido_est,
    CASE
        WHEN EXTRACT(MONTH FROM s.dt_doc)::int = SPLIT_PART($2::text,'/',1)::int
         AND EXTRACT(YEAR  FROM s.dt_doc)::int = SPLIT_PART($2::text,'/',2)::int
        THEN 'normal'
        ELSE 'emitida_mes_anterior'
    END AS bloco,
    (ne.id IS NOT NULL) AS tem_xml
FROM sped s
LEFT JOIN participants part ON part.job_id = s.job_id AND part.cod_part = s.cod_part
LEFT JOIN nfe_entradas ne ON ne.chave_nfe = s.chave_nfe
LEFT JOIN LATERAL (
    SELECT nii.ncm FROM nfe_entradas_itens nii
    WHERE nii.nfe_id = ne.id ORDER BY nii.v_prod DESC NULLS LAST LIMIT 1
) top_item ON true
LEFT JOIN LATERAL (
    SELECT r.aliquota_interna, r.mva_original, r.mva_ajustado_12pct
    FROM icms_fronteira_regras_ncm r
    WHERE (r.company_id = $1 OR r.company_id IS NULL)
      AND r.uf_estado = COALESCE(ne.dest_uf, 'PE')
      AND top_item.ncm IS NOT NULL
      AND LEFT(top_item.ncm, LENGTH(r.ncm_prefixo)) = r.ncm_prefixo
    ORDER BY r.company_id NULLS LAST, LENGTH(r.ncm_prefixo) DESC LIMIT 1
) regra ON true
WHERE
    -- Bloco normal: emitida no período. Bloco "mês anterior": entrada no período
    -- mas emitida antes.
    (EXTRACT(MONTH FROM s.dt_doc)::int = SPLIT_PART($2::text,'/',1)::int
     AND EXTRACT(YEAR FROM s.dt_doc)::int = SPLIT_PART($2::text,'/',2)::int)
    OR
    (EXTRACT(MONTH FROM s.dt_e_s)::int = SPLIT_PART($2::text,'/',1)::int
     AND EXTRACT(YEAR FROM s.dt_e_s)::int = SPLIT_PART($2::text,'/',2)::int
     AND s.dt_doc < date_trunc('month', s.dt_e_s))
ORDER BY bloco, s.dt_doc, s.chave_nfe
`

// Bloco 3 (nao_localizada_sped): XML emitido no período, chave ausente do SPED.
// CFOP de entrada derivado do CFOP de saída do fornecedor (6xxx→2xxx, 5xxx→1xxx);
// só 2xxx (interestadual) interessa à fronteira.
const reconXmlQuery = `
WITH xml_falt AS (
    SELECT
        ne.id, ne.chave_nfe, ne.data_emissao, ne.forn_cnpj, ne.forn_nome,
        ne.forn_uf, ne.dest_uf, COALESCE(ne.numero_nfe,'') AS numero_nfe,
        COALESCE(ne.v_prod,0) AS v_prod, COALESCE(ne.v_frete,0) AS v_frete,
        COALESCE(ne.v_outro,0) AS v_outro
    FROM nfe_entradas ne
    WHERE ne.company_id = $1
      AND EXTRACT(MONTH FROM ne.data_emissao)::int = SPLIT_PART($2::text,'/',1)::int
      AND EXTRACT(YEAR  FROM ne.data_emissao)::int = SPLIT_PART($2::text,'/',2)::int
      AND NOT EXISTS (
          SELECT 1 FROM reg_c100 c100 JOIN import_jobs j ON j.id = c100.job_id
          WHERE j.company_id = $1 AND c100.chv_nfe = ne.chave_nfe
      )
), top AS (
    -- CFOP dominante do item (por valor) + NCM
    SELECT DISTINCT ON (xf.id)
        xf.id, xf.chave_nfe, xf.data_emissao, xf.forn_cnpj, xf.forn_nome,
        xf.forn_uf, xf.dest_uf, xf.numero_nfe, xf.v_prod, xf.v_frete, xf.v_outro,
        COALESCE(nii.cfop,'') AS cfop_saida, COALESCE(nii.ncm,'') AS ncm
    FROM xml_falt xf
    JOIN nfe_entradas_itens nii ON nii.nfe_id = xf.id
    ORDER BY xf.id, nii.v_prod DESC NULLS LAST
), mapped AS (
    SELECT *,
        -- mapeamento determinístico saída→entrada
        CASE
            WHEN LEFT(cfop_saida,1) = '6' THEN '2' || SUBSTRING(cfop_saida FROM 2)
            WHEN LEFT(cfop_saida,1) = '5' THEN '1' || SUBSTRING(cfop_saida FROM 2)
            ELSE cfop_saida
        END AS cfop_entrada
    FROM top
)
SELECT
    m.chave_nfe,
    m.data_emissao::text AS data_emissao,
    m.numero_nfe,
    m.forn_cnpj, m.forn_nome, COALESCE(m.forn_uf,'') AS forn_uf,
    m.cfop_saida, m.cfop_entrada,
    -- Regime final: manual sobrescreve a sugestão automática (cfop_entrada).
    COALESCE(cm.regime,
        CASE
            WHEN m.cfop_entrada IN ('2551','2556') THEN 'DIFAL'
            WHEN m.cfop_entrada IN ('2403','2409','2651','2652') THEN 'ST'
            WHEN m.cfop_entrada IN ('2101','2102','2152') THEN 'ANTECIPACAO'
            ELSE 'NAO_FRONTEIRA'
        END
    ) AS regime,
    -- class_status: 'auto' (sugestão), 'manual' (validada/editada), 'excluded' (fora do cálculo)
    COALESCE(cm.status, 'auto') AS class_status,
    (m.v_prod + m.v_frete + m.v_outro) AS v_opr,
    -- Estimativa (XML-only não traz ICMS interestadual do SPED): usa alíquota
    -- interestadual presumida pela UF do fornecedor (7% Sul/Sudeste, senão 12%).
    -- Valor é ESTIMADO e exige validação antes de entrar no cálculo oficial.
    CASE
        WHEN m.cfop_entrada IN ('2551','2556') THEN
            GREATEST(0, (m.v_prod+m.v_frete+m.v_outro)
                * (COALESCE(regra.aliquota_interna,20.5)
                   - CASE WHEN m.forn_uf = ANY(ARRAY['PR','RS','SC','MG','RJ','SP']) THEN 7.0 ELSE 12.0 END)/100.0)
        WHEN m.cfop_entrada IN ('2101','2102','2152') THEN
            GREATEST(0, (m.v_prod+m.v_frete+m.v_outro)
                * (COALESCE(regra.aliquota_interna,20.5)
                   - CASE WHEN m.forn_uf = ANY(ARRAY['PR','RS','SC','MG','RJ','SP']) THEN 7.0 ELSE 12.0 END)/100.0)
        WHEN m.cfop_entrada IN ('2403','2409','2651','2652') THEN
            CASE WHEN COALESCE(regra.mva_original, regra.mva_ajustado_12pct) IS NOT NULL
                THEN GREATEST(0, (m.v_prod+m.v_frete+m.v_outro)
                     * (1.0 + COALESCE(regra.mva_original, regra.mva_ajustado_12pct)/100.0)
                     * COALESCE(regra.aliquota_interna,20.5)/100.0
                     - (m.v_prod+m.v_frete+m.v_outro)
                       * CASE WHEN m.forn_uf = ANY(ARRAY['PR','RS','SC','MG','RJ','SP']) THEN 7.0 ELSE 12.0 END/100.0)
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
LEFT JOIN icms_fronteira_classificacao_manual cm
    ON cm.company_id = $1 AND cm.chave_nfe = m.chave_nfe
WHERE m.cfop_entrada IN ('2101','2102','2152','2403','2409','2651','2652','2551','2556')
  -- Notas marcadas como 'excluded' não aparecem no bloco
  AND COALESCE(cm.status, 'auto') <> 'excluded'
ORDER BY m.data_emissao, m.chave_nfe
`

// IcmsFronteiraReconciliacaoHandler — GET /api/icms-fronteira/reconciliacao?periodo=MM/YYYY
func IcmsFronteiraReconciliacaoHandler(db *sql.DB) http.HandlerFunc {
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
		if periodo == "" {
			jsonErr(w, http.StatusBadRequest, "Parâmetro 'periodo' (MM/YYYY) é obrigatório para reconciliação")
			return
		}

		resp := ReconciliacaoResponse{
			Periodo:            periodo,
			Normal:             ReconBlock{Rows: []ReconNota{}},
			EmitidaMesAnterior: ReconBlock{Rows: []ReconNota{}},
			SpedSemXML:         ReconBlock{Rows: []ReconNota{}},
			NaoLocalizadaSped:  ReconBlock{Rows: []ReconNota{}},
		}

		// Blocos 1 e 2 (SPED)
		rows, err := db.Query(reconSpedQuery, companyID, periodo)
		if err != nil {
			log.Printf("Reconciliacao SPED error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar SPED")
			return
		}
		for rows.Next() {
			var n ReconNota
			var bloco string
			if err := rows.Scan(&n.ChaveNFe, &n.DataEmissao, &n.DataEntrada, &n.NumeroNFe,
				&n.FornCNPJ, &n.FornNome, &n.FornUF, &n.CFOP, &n.Regime, &n.VOpr,
				&n.IcmsDevidoEst, &bloco, &n.TemXML); err != nil {
				log.Printf("Reconciliacao SPED scan: %v", err)
				continue
			}
			n.Origem = "sped"
			if bloco == "emitida_mes_anterior" {
				n.Alerta = "Emitida em mês anterior — imposto provavelmente já recolhido no mês de emissão. Verificar."
				resp.EmitidaMesAnterior.Rows = append(resp.EmitidaMesAnterior.Rows, n)
				resp.EmitidaMesAnterior.Total += n.IcmsDevidoEst
			} else {
				resp.Normal.Rows = append(resp.Normal.Rows, n)
				resp.Normal.Total += n.IcmsDevidoEst
			}
			// SpedSemXML: só notas do mês de análise (bloco "normal") sem XML.
			// Notas de mês anterior sem XML já aparecem em EmitidaMesAnterior — não duplicar.
			if !n.TemXML && bloco == "normal" {
				resp.SpedSemXML.Rows = append(resp.SpedSemXML.Rows, n)
				resp.SpedSemXML.Total += n.IcmsDevidoEst
			}
		}
		rows.Close()

		// Bloco 3 (XML faltante)
		rows2, err := db.Query(reconXmlQuery, companyID, periodo)
		if err != nil {
			log.Printf("Reconciliacao XML error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar XML")
			return
		}
		for rows2.Next() {
			var n ReconNota
			if err := rows2.Scan(&n.ChaveNFe, &n.DataEmissao, &n.NumeroNFe,
				&n.FornCNPJ, &n.FornNome, &n.FornUF, &n.CFOP, &n.CFOPEntrada,
				&n.Regime, &n.ClassStatus, &n.VOpr, &n.IcmsDevidoEst); err != nil {
				log.Printf("Reconciliacao XML scan: %v", err)
				continue
			}
			n.Origem = "xml"
			// Mensagem do alerta varia conforme status da classificação
			switch n.ClassStatus {
			case "manual":
				n.Alerta = "Classificação validada manualmente."
			default:
				n.Alerta = "Não localizada no SPED — classificação por CFOP do fornecedor; validar antes de incluir no cálculo."
			}
			resp.NaoLocalizadaSped.Rows = append(resp.NaoLocalizadaSped.Rows, n)
			resp.NaoLocalizadaSped.Total += n.IcmsDevidoEst
		}
		rows2.Close()

		resp.Normal.Count = len(resp.Normal.Rows)
		resp.EmitidaMesAnterior.Count = len(resp.EmitidaMesAnterior.Rows)
		resp.SpedSemXML.Count = len(resp.SpedSemXML.Rows)
		resp.NaoLocalizadaSped.Count = len(resp.NaoLocalizadaSped.Rows)

		json.NewEncoder(w).Encode(resp)
	}
}
