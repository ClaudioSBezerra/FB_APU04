package handlers

// motor_fiscal.go — Motor de Cálculo Fiscal, Fase 1: Substituição Tributária BA.
//
// Pipeline (POST /api/icms-fronteira/motor-fiscal/calcular):
//   1. Filtra itens de NF (nfe_entradas_itens) com cfop='2403' e dest_uf='BA'
//      no período informado.
//   2. Para cada item, busca a MVA aplicável (longest-prefix-wins) em
//      icms_fronteira_regras_ncm filtrando uf_estado='BA'.
//   3. Calcula frete proporcional ao item:
//         frete_prop  = v_item / Σ(v_item NF) × v_frete_header
//         outras_prop = idem para v_outro
//   4. Calcula frete CT-e rateado (apenas CT-es com toma=3 do destinatário):
//         v_frete_cte_rateado = (v_item / Σv_item_NF) × Σ(v_prest CT-es válidos)
//   5. Aplica fórmula Base ST = (V.Item + IPI + Frete prop + Frete CT-e + Outras) × (1 + MVA/100)
//   6. ICMS ST estimado = Base ST × Alíq.Interna% − V.ICMS destacado no item
//   7. Persiste em fiscal_calculations (idempotente via uq_fiscal_calc_item_fase).
//
// Lista (GET /api/icms-fronteira/motor-fiscal/resultados):
//   - Retorna o cálculo persistido com paginação simples (LIMIT 1000).

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
			WHERE company_id=$1 AND fase=$2
			  AND EXTRACT(MONTH FROM data_emissao)::int = $3::int
			  AND EXTRACT(YEAR  FROM data_emissao)::int = $4::int`,
			companyID, fasaST_BA, parts[0], parts[1])

		// ─── Pipeline em SQL ──────────────────────────────────────────────────
		// Calcula e insere em fiscal_calculations num único INSERT...SELECT.
		const q = `
			WITH itens_alvo AS (
			    SELECT
			        ne.id           AS nfe_id,
			        ne.chave_nfe,
			        COALESCE(ne.numero_nfe,'') AS numero_nfe,
			        ne.data_emissao,
			        ne.forn_uf,
			        ne.dest_uf,
			        COALESCE(ne.v_frete,0)  AS nf_v_frete,
			        COALESCE(ne.v_outro,0)  AS nf_v_outro,
			        nii.id           AS item_id,
			        nii.n_item,
			        COALESCE(nii.ncm,'')      AS ncm,
			        nii.cfop,
			        COALESCE(nii.cst_icms,'') AS cst_icms,
			        COALESCE(nii.v_prod,0)    AS v_item,
			        COALESCE(nii.v_ipi,0)     AS v_ipi,
			        COALESCE(nii.v_icms,0)    AS v_icms_item
			    FROM nfe_entradas ne
			    JOIN nfe_entradas_itens nii ON nii.nfe_id = ne.id
			    WHERE ne.company_id = $1
			      AND ne.dest_uf = 'BA'
			      -- Converte CFOP de saída (do emitente) para entrada (do destinatário):
			      -- 6xxx → 2xxx (interestadual), 5xxx → 1xxx (intraestadual).
			      AND CASE WHEN LEFT(nii.cfop,1)='6' THEN '2' || SUBSTRING(nii.cfop FROM 2)
			               WHEN LEFT(nii.cfop,1)='5' THEN '1' || SUBSTRING(nii.cfop FROM 2)
			               ELSE nii.cfop END = '2403'
			      AND EXTRACT(MONTH FROM ne.data_emissao)::int = $2::int
			      AND EXTRACT(YEAR  FROM ne.data_emissao)::int = $3::int
			),
			soma_nf AS (
			    SELECT nfe_id, SUM(v_item) AS total_prod
			    FROM itens_alvo GROUP BY nfe_id
			),
			frete_cte_por_nf AS (
			    -- Soma do v_prest dos CT-es do destinatário (toma=3 ou toma=4=dest)
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
			        COALESCE(fc.v_frete_cte_total, 0)         AS v_frete_cte_total,
			        regra.id                                   AS regra_id,
			        regra.ncm_prefixo                          AS regra_prefixo,
			        regra.mva_original,
			        regra.mva_ajustado_4pct,
			        regra.mva_ajustado_7pct,
			        regra.mva_ajustado_12pct,
			        regra.aliquota_interna,
			        -- Alíquota interestadual efetiva da NF (v_icms_item / v_item × 100).
			        -- Arredondada para encontrar a MVA ajustada correspondente.
			        CASE WHEN it.v_item > 0
			             THEN ROUND((it.v_icms_item / it.v_item * 100.0)::numeric, 0)
			             ELSE 12 END AS aliq_inter_round
			    FROM itens_alvo it
			    JOIN soma_nf s ON s.nfe_id = it.nfe_id
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
			        -- Frete proporcional ao item (rateio pelo v_item / Σv_item da NF)
			        CASE WHEN total_prod > 0 THEN v_item / total_prod * nf_v_frete ELSE 0 END
			            AS frete_prop,
			        CASE WHEN total_prod > 0 THEN v_item / total_prod * nf_v_outro ELSE 0 END
			            AS outro_prop,
			        CASE WHEN total_prod > 0 THEN v_item / total_prod * v_frete_cte_total ELSE 0 END
			            AS frete_cte_rateado,
			        -- MVA aplicada: preferência ajustada 4/7/12 conforme aliq_inter; fallback original
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
			)
			INSERT INTO fiscal_calculations (
			    company_id, nfe_id, item_id, chave_nfe, numero_nfe, data_emissao,
			    n_item, cfop, ncm, cst_icms, dest_uf, forn_uf,
			    v_item, v_ipi, v_frete_proporcional, v_frete_cte_rateado, v_outras_desp,
			    v_icms_item, ncm_regra_id, ncm_prefixo_aplicado,
			    mva_aplicada, mva_tipo, aliq_inter, aliq_interna,
			    base_st, icms_st_estimado, fase
			)
			SELECT
			    $1, c.nfe_id, c.item_id, c.chave_nfe, c.numero_nfe, c.data_emissao,
			    c.n_item, c.cfop, c.ncm, c.cst_icms, c.dest_uf, c.forn_uf,
			    c.v_item, c.v_ipi, c.frete_prop, c.frete_cte_rateado, c.outro_prop,
			    c.v_icms_item, c.regra_id, c.regra_prefixo,
			    c.mva_aplicada, c.mva_tipo, c.aliq_inter_round, COALESCE(c.aliquota_interna, 18.0),
			    -- Base ST = (V.Item + IPI + Frete prop + Frete CT-e + Outras) × (1 + MVA/100)
			    GREATEST(0,
			        (c.v_item + c.v_ipi + c.frete_prop + c.frete_cte_rateado + c.outro_prop)
			        * (1.0 + COALESCE(c.mva_aplicada,0)/100.0)
			    ) AS base_st,
			    -- ICMS ST = Base ST × Alíq.Interna − V.ICMS destacado no item
			    GREATEST(0,
			        ((c.v_item + c.v_ipi + c.frete_prop + c.frete_cte_rateado + c.outro_prop)
			         * (1.0 + COALESCE(c.mva_aplicada,0)/100.0))
			        * COALESCE(c.aliquota_interna, 18.0)/100.0
			        - c.v_icms_item
			    ) AS icms_st_estimado,
			    $4
			FROM calc c
			ON CONFLICT ON CONSTRAINT uq_fiscal_calc_item_fase DO UPDATE SET
			    v_item = EXCLUDED.v_item,
			    v_ipi = EXCLUDED.v_ipi,
			    v_frete_proporcional = EXCLUDED.v_frete_proporcional,
			    v_frete_cte_rateado = EXCLUDED.v_frete_cte_rateado,
			    v_outras_desp = EXCLUDED.v_outras_desp,
			    mva_aplicada = EXCLUDED.mva_aplicada,
			    mva_tipo = EXCLUDED.mva_tipo,
			    base_st = EXCLUDED.base_st,
			    icms_st_estimado = EXCLUDED.icms_st_estimado,
			    updated_at = now()
		`
		res, err := db.Exec(q, companyID, parts[0], parts[1], fasaST_BA)
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
		parts := strings.SplitN(periodo, "/", 2)
		if len(parts) == 2 {
			q += ` AND EXTRACT(MONTH FROM data_emissao)::int = $3::int
				   AND EXTRACT(YEAR  FROM data_emissao)::int = $4::int`
			args = append(args, parts[0], parts[1])
		}
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
