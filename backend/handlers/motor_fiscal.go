package handlers

// motor_fiscal.go — Motor de Cálculo Fiscal, Fase 1: Substituição Tributária BA.
//
// Fonte: SPED Fiscal (reg_c170 → reg_c100 → import_jobs).
//   reg_c170: itens da NF (cod_item, CFOP, valores, ICMS destacado)
//   reg_0200: cadastro de produtos do SPED (NCM por cod_item)
//   reg_c100: cabeçalho (data, cod_part fornecedor)
//   import_jobs.uf: UF do destinatário declarado no SPED (reg 0000)
//
// Pipeline (POST /api/icms-fronteira/motor-fiscal/calcular):
//   1. Filtra itens reg_c170 com cfop='2403' onde import_jobs.uf='BA'
//      e job.mes_ano = periodo informado.
//   2. Cruza NCM via reg_0200 por (job_id, cod_item).
//   3. Busca a MVA aplicável (longest-prefix-wins) em
//      icms_fronteira_regras_ncm filtrando uf_estado='BA'.
//   4. Rateia frete e outras despesas da NF (reg_c100.vl_doc) proporcional
//      ao v_item / Σv_item do C100.  (Nota: SPED C100 não detalha v_frete
//      separado — usamos o XML.nfe_entradas.v_frete quando há chave casada.)
//   5. Soma frete CT-e rateado (apenas CT-es do destinatário, toma='3').
//   6. Aplica fórmula Base ST = (V.Item + IPI + Frete prop + Frete CT-e + Outras) × (1 + MVA/100)
//   7. ICMS ST = Base ST × Alíq.Interna% − V.ICMS destacado no item
//   8. Persiste em fiscal_calculations (idempotente via uq_fiscal_calc_item_fase).
//
// Lista (GET /api/icms-fronteira/motor-fiscal/resultados):
//   - Retorna o cálculo persistido com paginação simples (LIMIT 2000).

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

const fasaST_BA = "F1_ST_BA"

// FiscalCalcRow — linha de retorno (calcular + listar usam o mesmo formato)
type FiscalCalcRow struct {
	ID                  string  `json:"id"`
	ChaveNFe            string  `json:"chave_nfe"`
	NumeroNFe           string  `json:"numero_nfe"`
	DataEmissao         string  `json:"data_emissao"`
	NItem               int     `json:"n_item"`
	CFOP                string  `json:"cfop"`
	NCM                 string  `json:"ncm"`
	CSTICMS             string  `json:"cst_icms"`
	DestUF              string  `json:"dest_uf"`
	FornUF              string  `json:"forn_uf"`
	VItem               float64 `json:"v_item"`
	VIPI                float64 `json:"v_ipi"`
	VFreteProporcional  float64 `json:"v_frete_proporcional"`
	VFreteCTeRateado    float64 `json:"v_frete_cte_rateado"`
	VOutrasDesp         float64 `json:"v_outras_desp"`
	VIcmsItem           float64 `json:"v_icms_item"`
	NCMPrefixoAplicado  string  `json:"ncm_prefixo_aplicado"`
	MVAAplicada         float64 `json:"mva_aplicada"`
	MVATipo             string  `json:"mva_tipo"`
	AliqInter           float64 `json:"aliq_inter"`
	AliqInterna         float64 `json:"aliq_interna"`
	BaseST              float64 `json:"base_st"`
	IcmsSTEstimado      float64 `json:"icms_st_estimado"`
}

type FiscalCalcResponse struct {
	Rows           []FiscalCalcRow `json:"rows"`
	Count          int             `json:"count"`
	TotalBaseST    float64         `json:"total_base_st"`
	TotalIcmsST    float64         `json:"total_icms_st"`
	Periodo        string          `json:"periodo"`
	Fase           string          `json:"fase"`
	Mensagem       string          `json:"mensagem,omitempty"`
}

// ---------------------------------------------------------------------------
// MotorFiscalCalcularHandler — POST /api/icms-fronteira/motor-fiscal/calcular
// ---------------------------------------------------------------------------

func MotorFiscalCalcularHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPost {
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
			jsonErr(w, http.StatusBadRequest, "Parâmetro 'periodo' obrigatório (MM/YYYY)")
			return
		}
		parts := strings.SplitN(periodo, "/", 2)
		if len(parts) != 2 {
			jsonErr(w, http.StatusBadRequest, "Período inválido — use MM/YYYY")
			return
		}

		// Limpa cálculos anteriores deste período/fase (idempotente)
		_, _ = db.Exec(`
			DELETE FROM fiscal_calculations
			WHERE company_id=$1 AND fase=$2 AND periodo=$3`,
			companyID, fasaST_BA, periodo)

		// ─── Pipeline em SQL ──────────────────────────────────────────────────
		// Fonte: SPED Fiscal (reg_c170 + reg_c100 + reg_0200 + import_jobs).
		// Frete da NF: vem do XML (nfe_entradas) por casamento de chave_nfe — o
		// SPED não detalha v_frete separado no C100.
		const q = `
			WITH itens_alvo AS (
			    SELECT
			        c170.id          AS item_id,
			        c170.num_item    AS n_item,
			        c170.cod_item,
			        c170.cfop,
			        c170.cst_icms,
			        COALESCE(c170.vl_item, 0)    AS v_item,
			        COALESCE(c170.vl_ipi, 0)     AS v_ipi,
			        COALESCE(c170.vl_icms, 0)    AS v_icms_item,
			        c100.id          AS nfe_c100_id,
			        c100.chv_nfe     AS chave_nfe,
			        COALESCE(c100.num_doc, '')   AS numero_nfe,
			        c100.dt_doc      AS data_emissao,
			        j.uf             AS dest_uf,
			        COALESCE(p0200.cod_ncm, '')  AS ncm
			    FROM reg_c170 c170
			    JOIN reg_c100 c100 ON c100.id = c170.c100_id
			    JOIN import_jobs j ON j.id = c100.job_id
			    LEFT JOIN reg_0200 p0200
			           ON p0200.job_id = c170.job_id
			          AND p0200.cod_item = c170.cod_item
			    WHERE j.company_id = $1
			      AND j.uf = 'BA'
			      AND c170.cfop = '2403'
			      AND c100.ind_oper = '0'   -- 0 = entrada
			      AND j.mes_ano = $2 || '/' || $3
			),
			itens_com_xml AS (
			    -- Casa com nfe_entradas para obter v_frete (header) e UF fornecedor.
			    -- O frete não vem no C170 — vem só no XML do header da NF.
			    -- Fallback de NCM: quando reg_0200 está vazio, usa NCM do XML
			    -- (nfe_entradas_itens) por chave + n_item.
			    -- IPI: o XML do item é a fonte preferencial (valor real do
			    -- fornecedor); o SPED C170 de entradas costuma vir SEM IPI (o
			    -- comprador não credita). Usa XML e, se ausente, cai no C170.
			    -- A base ST inclui o IPI do produto.
			    SELECT it.item_id, it.n_item, it.cod_item, it.cfop, it.cst_icms,
			        it.v_item,
			        COALESCE(NULLIF(nii.v_ipi,0), it.v_ipi, 0) AS v_ipi,
			        it.v_icms_item,
			        it.nfe_c100_id, it.chave_nfe, it.numero_nfe, it.data_emissao, it.dest_uf,
			        COALESCE(NULLIF(it.ncm,''), COALESCE(nii.ncm,'')) AS ncm,
			        COALESCE(ne.v_frete, 0) AS nf_v_frete,
			        COALESCE(ne.v_outro, 0) AS nf_v_outro,
			        ne.forn_uf              AS forn_uf,
			        ne.id                   AS nfe_id_xml
			    FROM itens_alvo it
			    LEFT JOIN nfe_entradas ne
			           ON ne.company_id = $1 AND ne.chave_nfe = it.chave_nfe
			    LEFT JOIN nfe_entradas_itens nii
			           ON nii.nfe_id = ne.id AND nii.n_item = it.n_item
			),
			soma_c100 AS (
			    -- Soma vl_item por C100 para ratear frete e outras despesas
			    SELECT nfe_c100_id, SUM(v_item) AS total_prod
			    FROM itens_alvo GROUP BY nfe_c100_id
			),
			frete_cte_por_nf AS (
			    SELECT ref.chave_nfe, SUM(COALESCE(ce.v_prest,0)) AS v_frete_cte_total
			    FROM cte_entradas_nfe_refs ref
			    JOIN cte_entradas ce ON ce.id = ref.cte_id
			    WHERE ref.company_id = $1
			      AND (ce.toma='3' OR (ce.toma='4' AND ce.toma4_cnpj = ce.dest_cnpj_cpf))
			    GROUP BY ref.chave_nfe
			),
			com_regra AS (
			    SELECT it.*,
			        s.total_prod,
			        COALESCE(fc.v_frete_cte_total, 0) AS v_frete_cte_total,
			        regra.id           AS regra_id,
			        regra.ncm_prefixo  AS regra_prefixo,
			        regra.mva_original,
			        regra.mva_ajustado_4pct,
			        regra.mva_ajustado_7pct,
			        regra.mva_ajustado_12pct,
			        regra.aliquota_interna,
			        CASE WHEN it.v_item > 0
			             THEN ROUND((it.v_icms_item / it.v_item * 100.0)::numeric, 0)
			             ELSE 12 END AS aliq_inter_round
			    FROM itens_com_xml it
			    JOIN soma_c100 s ON s.nfe_c100_id = it.nfe_c100_id
			    LEFT JOIN frete_cte_por_nf fc ON fc.chave_nfe = it.chave_nfe
			    LEFT JOIN LATERAL (
			        SELECT id, ncm_prefixo,
			               mva_original, mva_ajustado_4pct, mva_ajustado_7pct,
			               mva_ajustado_12pct, aliquota_interna
			        FROM icms_fronteira_regras_ncm r
			        WHERE (r.company_id = $1 OR r.company_id IS NULL)
			          AND r.uf_estado = 'BA'
			          AND it.ncm <> ''
			          AND it.ncm LIKE r.ncm_prefixo || '%'
			        ORDER BY r.company_id NULLS LAST, length(r.ncm_prefixo) DESC
			        LIMIT 1
			    ) regra ON true
			),
			calc AS (
			    SELECT cr.*,
			        CASE WHEN total_prod > 0 THEN v_item / total_prod * nf_v_frete ELSE 0 END
			            AS frete_prop,
			        CASE WHEN total_prod > 0 THEN v_item / total_prod * nf_v_outro ELSE 0 END
			            AS outro_prop,
			        CASE WHEN total_prod > 0 THEN v_item / total_prod * v_frete_cte_total ELSE 0 END
			            AS frete_cte_rateado,
			        COALESCE(
			            CASE aliq_inter_round
			                 WHEN 4 THEN mva_ajustado_4pct
			                 WHEN 7 THEN mva_ajustado_7pct
			                 WHEN 12 THEN mva_ajustado_12pct
			            END,
			            mva_original
			        ) AS mva_aplicada,
			        CASE
			            WHEN COALESCE(
			                    CASE aliq_inter_round
			                         WHEN 4 THEN mva_ajustado_4pct
			                         WHEN 7 THEN mva_ajustado_7pct
			                         WHEN 12 THEN mva_ajustado_12pct
			                    END, NULL) IS NOT NULL THEN 'ajustada'
			            WHEN mva_original IS NOT NULL THEN 'original'
			            ELSE 'indisponivel'
			        END AS mva_tipo
			    FROM com_regra cr
			    -- Aceita itens SPED-only (sem XML). Frete da NF fica 0, mas
			    -- IPI/V.Item/ICMS do C170 alimentam a base ST.
			)
			INSERT INTO fiscal_calculations (
			    company_id, nfe_id, item_id, sped_c170_id, sped_c100_id,
			    chave_nfe, numero_nfe, data_emissao,
			    n_item, cfop, ncm, cst_icms, dest_uf, forn_uf,
			    v_item, v_ipi, v_frete_proporcional, v_frete_cte_rateado, v_outras_desp,
			    v_icms_item, ncm_regra_id, ncm_prefixo_aplicado,
			    mva_aplicada, mva_tipo, aliq_inter, aliq_interna,
			    base_st, icms_st_estimado, fase, periodo
			)
			SELECT
			    $1,
			    c.nfe_id_xml,
			    (SELECT nii.id FROM nfe_entradas_itens nii
			      WHERE nii.nfe_id = c.nfe_id_xml AND nii.n_item = c.n_item LIMIT 1),
			    c.item_id,
			    c.nfe_c100_id,
			    c.chave_nfe, c.numero_nfe, c.data_emissao,
			    c.n_item, c.cfop, c.ncm, c.cst_icms, c.dest_uf, c.forn_uf,
			    c.v_item, c.v_ipi, c.frete_prop, c.frete_cte_rateado, c.outro_prop,
			    c.v_icms_item, c.regra_id, c.regra_prefixo,
			    c.mva_aplicada, c.mva_tipo, c.aliq_inter_round, COALESCE(c.aliquota_interna, 18.0),
			    GREATEST(0,
			        (c.v_item + c.v_ipi + c.frete_prop + c.frete_cte_rateado + c.outro_prop)
			        * (1.0 + COALESCE(c.mva_aplicada,0)/100.0)
			    ),
			    GREATEST(0,
			        ((c.v_item + c.v_ipi + c.frete_prop + c.frete_cte_rateado + c.outro_prop)
			         * (1.0 + COALESCE(c.mva_aplicada,0)/100.0))
			        * COALESCE(c.aliquota_interna, 18.0)/100.0
			        - c.v_icms_item
			    ),
			    $4, $5
			FROM calc c
		`
		res, err := db.Exec(q, companyID, parts[0], parts[1], fasaST_BA, periodo)
		if err != nil {
			log.Printf("MotorFiscalCalcular[%s] exec error: %v", periodo, err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao executar cálculo: "+err.Error())
			return
		}
		affected, _ := res.RowsAffected()

		// Resposta — recarrega resultados persistidos
		listResp, err := loadFiscalCalcRows(db, companyID, periodo)
		if err != nil {
			log.Printf("MotorFiscalCalcular[%s] reload error: %v", periodo, err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao recarregar resultados")
			return
		}
		listResp.Mensagem = "Cálculo concluído — " + itoaMF(int(affected)) + " itens processados"
		listResp.Fase = fasaST_BA
		listResp.Periodo = periodo
		json.NewEncoder(w).Encode(listResp)
	}
}

// ---------------------------------------------------------------------------
// MotorFiscalResultadosHandler — GET /api/icms-fronteira/motor-fiscal/resultados
// ---------------------------------------------------------------------------

func MotorFiscalResultadosHandler(db *sql.DB) http.HandlerFunc {
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
		resp, err := loadFiscalCalcRows(db, companyID, periodo)
		if err != nil {
			log.Printf("MotorFiscalResultados error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao carregar resultados")
			return
		}
		resp.Fase = fasaST_BA
		resp.Periodo = periodo
		json.NewEncoder(w).Encode(resp)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func loadFiscalCalcRows(db *sql.DB, companyID, periodo string) (FiscalCalcResponse, error) {
	var resp FiscalCalcResponse
	resp.Rows = []FiscalCalcRow{}

	q := `
		SELECT id::text, chave_nfe, COALESCE(numero_nfe,''), data_emissao::text,
		       n_item, cfop, COALESCE(ncm,''), COALESCE(cst_icms,''), dest_uf, COALESCE(forn_uf,''),
		       v_item, v_ipi, v_frete_proporcional, v_frete_cte_rateado, v_outras_desp, v_icms_item,
		       COALESCE(ncm_prefixo_aplicado,''), COALESCE(mva_aplicada,0), COALESCE(mva_tipo,''),
		       aliq_inter, aliq_interna,
		       base_st, icms_st_estimado
		FROM fiscal_calculations
		WHERE company_id=$1 AND fase=$2`
	args := []interface{}{companyID, fasaST_BA}
	if periodo != "" {
		q += ` AND periodo = $3`
		args = append(args, periodo)
	}
	q += ` ORDER BY data_emissao DESC, chave_nfe, n_item LIMIT 2000`

	rows, err := db.Query(q, args...)
	if err != nil {
		return resp, err
	}
	defer rows.Close()
	for rows.Next() {
		var r FiscalCalcRow
		if err := rows.Scan(
			&r.ID, &r.ChaveNFe, &r.NumeroNFe, &r.DataEmissao,
			&r.NItem, &r.CFOP, &r.NCM, &r.CSTICMS, &r.DestUF, &r.FornUF,
			&r.VItem, &r.VIPI, &r.VFreteProporcional, &r.VFreteCTeRateado, &r.VOutrasDesp, &r.VIcmsItem,
			&r.NCMPrefixoAplicado, &r.MVAAplicada, &r.MVATipo,
			&r.AliqInter, &r.AliqInterna,
			&r.BaseST, &r.IcmsSTEstimado,
		); err != nil {
			continue
		}
		resp.Rows = append(resp.Rows, r)
		resp.TotalBaseST += r.BaseST
		resp.TotalIcmsST += r.IcmsSTEstimado
	}
	resp.Count = len(resp.Rows)
	return resp, rows.Err()
}

// itoaMF — local pequeno (já existe outro 'itoa' em contestações)
func itoaMF(n int) string {
	if n == 0 {
		return "0"
	}
	negative := false
	if n < 0 {
		negative = true
		n = -n
	}
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
