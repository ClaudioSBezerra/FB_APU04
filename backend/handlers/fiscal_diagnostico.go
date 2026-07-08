// fiscal_diagnostico.go — GET /api/fiscal/diagnostico
//
// Relatório-sumário dos testes do pacote fiscal (pedido do Claudio,
// 2026-07-08): agrega tudo que já foi executado (fiscal_execution_items ×
// itens × notas) para a empresa ativa — período, notas/itens, status, CFOPs,
// CSTs de ICMS/PIS, parâmetros usados (centro fiscal, tipo contribuinte) e
// divergências por tributo, com a MESMA régua da tela: no modo inclusão
// IBS/CBS (simulacao presente) compara contra o esperado ajustado com
// tolerância de 1 centavo; sem simulação, XML cru com tolerância zero.
package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type diagCfopRow struct {
	CFOP      string  `json:"cfop"`
	Notas     int     `json:"notas"`
	Itens     int     `json:"itens"`
	VProd     float64 `json:"v_prod"`
	OK        int     `json:"ok"`
	SemGrupo  int     `json:"sem_grupo_fiscal"`
	Erro      int     `json:"error"`
	DivIcms   int     `json:"div_icms"`
	DivSt     int     `json:"div_st"`
	DivPis    int     `json:"div_pis"`
	DivCofins int     `json:"div_cofins"`
	DivIbs    int     `json:"div_ibs"`
	DivCbs    int     `json:"div_cbs"`
}

type diagDistRow struct {
	Chave string  `json:"chave"`
	Itens int     `json:"itens"`
	VProd float64 `json:"v_prod"`
}

type diagErroRow struct {
	Mensagem string `json:"mensagem"`
	Itens    int    `json:"itens"`
}

type fiscalDiagnostico struct {
	PeriodoInicio   string        `json:"periodo_inicio"`
	PeriodoFim      string        `json:"periodo_fim"`
	NotasExecutadas int           `json:"notas_executadas"`
	ItensExecutados int           `json:"itens_executados"`
	ItensOK         int           `json:"itens_ok"`
	ItensSemGrupo   int           `json:"itens_sem_grupo"`
	ItensErro       int           `json:"itens_erro"`
	ComSimulacao    int           `json:"com_simulacao"`
	VProdTotal      float64       `json:"v_prod_total"`
	DivIcms         int           `json:"div_icms"`
	DivSt           int           `json:"div_st"`
	DivPis          int           `json:"div_pis"`
	DivCofins       int           `json:"div_cofins"`
	DivIbs          int           `json:"div_ibs"`
	DivCbs          int           `json:"div_cbs"`
	PorCfop         []diagCfopRow `json:"por_cfop"`
	PorCstIcms      []diagDistRow `json:"por_cst_icms"`
	PorCstPis       []diagDistRow `json:"por_cst_pis"`
	PorCentroFiscal []diagDistRow `json:"por_centro_fiscal"`
	PorContribuinte []diagDistRow `json:"por_contribuinte"`
	Erros           []diagErroRow `json:"erros"`
}

// diagDivExprs — flags de divergência por tributo, com a régua da tela:
// item ok + (modo simulação: |esperado ajustado − calculado| > 0,011;
// modo normal: diferença > 0). PIS/COFINS ajustados pelo ΔICMS × alíquota.
const diagDivExprs = `
	(fei.simulacao IS NOT NULL AND fei.simulacao->>'erro' IS NULL)            AS sim_ok,
	CASE WHEN fei.status = 'ok' THEN
		CASE WHEN fei.simulacao IS NOT NULL AND fei.simulacao->>'erro' IS NULL
			THEN abs(COALESCE((fei.simulacao->>'icms_simulado')::numeric,0) - COALESCE(fei.valor_icms,0)) > 0.011
			ELSE abs(COALESCE(i.v_icms,0) - COALESCE(fei.valor_icms,0)) > 0 END
	ELSE false END                                                            AS div_icms,
	CASE WHEN fei.status = 'ok' THEN
		CASE WHEN fei.simulacao IS NOT NULL AND fei.simulacao->>'erro' IS NULL
			THEN abs(COALESCE((fei.simulacao->>'st_simulado')::numeric,0) - COALESCE(fei.valor_substituicao,0)) > 0.011
			ELSE abs(COALESCE(i.v_st,0) - COALESCE(fei.valor_substituicao,0)) > 0 END
	ELSE false END                                                            AS div_st,
	CASE WHEN fei.status = 'ok' THEN
		CASE WHEN fei.simulacao IS NOT NULL AND fei.simulacao->>'erro' IS NULL
			THEN abs((COALESCE(i.v_pis,0) - (COALESCE(fei.valor_icms,0)-COALESCE(i.v_icms,0)) * COALESCE(i.p_pis,0)/100.0) - COALESCE(fei.valor_pis,0)) > 0.011
			ELSE abs(COALESCE(i.v_pis,0) - COALESCE(fei.valor_pis,0)) > 0 END
	ELSE false END                                                            AS div_pis,
	CASE WHEN fei.status = 'ok' THEN
		CASE WHEN fei.simulacao IS NOT NULL AND fei.simulacao->>'erro' IS NULL
			THEN abs((COALESCE(i.v_cofins,0) - (COALESCE(fei.valor_icms,0)-COALESCE(i.v_icms,0)) * COALESCE(i.p_cofins,0)/100.0) - COALESCE(fei.valor_cofins,0)) > 0.011
			ELSE abs(COALESCE(i.v_cofins,0) - COALESCE(fei.valor_cofins,0)) > 0 END
	ELSE false END                                                            AS div_cofins,
	CASE WHEN fei.status = 'ok' THEN
		abs(COALESCE(i.v_ibs,0) - (COALESCE(fei.valor_ibs_uf,0)+COALESCE(fei.valor_ibs_mun,0)))
			> CASE WHEN fei.simulacao IS NOT NULL THEN 0.011 ELSE 0 END
	ELSE false END                                                            AS div_ibs,
	CASE WHEN fei.status = 'ok' THEN
		abs(COALESCE(i.v_cbs,0) - COALESCE(fei.valor_cbs,0))
			> CASE WHEN fei.simulacao IS NOT NULL THEN 0.011 ELSE 0 END
	ELSE false END                                                            AS div_cbs`

// FiscalDiagnosticoHandler — GET /api/fiscal/diagnostico?data_inicio=&data_fim=
func FiscalDiagnosticoHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodGet {
			jsonErr(w, http.StatusMethodNotAllowed, "Método não permitido")
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
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa")
			return
		}

		dataInicio := strings.TrimSpace(r.URL.Query().Get("data_inicio"))
		dataFim := strings.TrimSpace(r.URL.Query().Get("data_fim"))

		// Base comum: itens já executados (existem em fiscal_execution_items),
		// escopados por empresa e período opcional.
		baseFrom := `
			FROM fiscal_execution_items fei
			JOIN pacotefiscal_nfe_saidas_itens i ON i.id = fei.nfe_item_id
			JOIN pacotefiscal_nfe_saidas n ON n.id = i.nfe_id
			WHERE n.company_id = $1
			  AND ($2::text = '' OR n.data_emissao >= $2::date)
			  AND ($3::text = '' OR n.data_emissao <= $3::date)`

		diag := fiscalDiagnostico{
			PorCfop: []diagCfopRow{}, PorCstIcms: []diagDistRow{}, PorCstPis: []diagDistRow{},
			PorCentroFiscal: []diagDistRow{}, PorContribuinte: []diagDistRow{}, Erros: []diagErroRow{},
		}

		// 1. Cabeçalho + totais + divergências
		err = db.QueryRow(`
			WITH base AS (SELECT n.id AS nfe_id, n.data_emissao, i.v_prod, fei.status, fei.simulacao, `+diagDivExprs+` `+baseFrom+`)
			SELECT COALESCE(MIN(data_emissao)::text,''), COALESCE(MAX(data_emissao)::text,''),
			       COUNT(DISTINCT nfe_id), COUNT(*),
			       COUNT(*) FILTER (WHERE status='ok'),
			       COUNT(*) FILTER (WHERE status='sem_grupo_fiscal'),
			       COUNT(*) FILTER (WHERE status NOT IN ('ok','sem_grupo_fiscal')),
			       COUNT(*) FILTER (WHERE sim_ok),
			       COALESCE(SUM(v_prod),0),
			       COUNT(*) FILTER (WHERE div_icms), COUNT(*) FILTER (WHERE div_st),
			       COUNT(*) FILTER (WHERE div_pis), COUNT(*) FILTER (WHERE div_cofins),
			       COUNT(*) FILTER (WHERE div_ibs), COUNT(*) FILTER (WHERE div_cbs)
			FROM base`, companyID, dataInicio, dataFim,
		).Scan(&diag.PeriodoInicio, &diag.PeriodoFim, &diag.NotasExecutadas, &diag.ItensExecutados,
			&diag.ItensOK, &diag.ItensSemGrupo, &diag.ItensErro, &diag.ComSimulacao, &diag.VProdTotal,
			&diag.DivIcms, &diag.DivSt, &diag.DivPis, &diag.DivCofins, &diag.DivIbs, &diag.DivCbs)
		if err != nil {
			log.Printf("[FiscalDiagnostico] header query: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao montar diagnóstico")
			return
		}

		// 2. Por CFOP
		rows, err := db.Query(`
			WITH base AS (SELECT n.id AS nfe_id, COALESCE(i.cfop,'') AS cfop, i.v_prod, fei.status, `+diagDivExprs+` `+baseFrom+`)
			SELECT cfop, COUNT(DISTINCT nfe_id), COUNT(*), COALESCE(SUM(v_prod),0),
			       COUNT(*) FILTER (WHERE status='ok'),
			       COUNT(*) FILTER (WHERE status='sem_grupo_fiscal'),
			       COUNT(*) FILTER (WHERE status NOT IN ('ok','sem_grupo_fiscal')),
			       COUNT(*) FILTER (WHERE div_icms), COUNT(*) FILTER (WHERE div_st),
			       COUNT(*) FILTER (WHERE div_pis), COUNT(*) FILTER (WHERE div_cofins),
			       COUNT(*) FILTER (WHERE div_ibs), COUNT(*) FILTER (WHERE div_cbs)
			FROM base GROUP BY cfop ORDER BY COUNT(*) DESC`, companyID, dataInicio, dataFim)
		if err == nil {
			for rows.Next() {
				var c diagCfopRow
				if rows.Scan(&c.CFOP, &c.Notas, &c.Itens, &c.VProd, &c.OK, &c.SemGrupo, &c.Erro,
					&c.DivIcms, &c.DivSt, &c.DivPis, &c.DivCofins, &c.DivIbs, &c.DivCbs) == nil {
					diag.PorCfop = append(diag.PorCfop, c)
				}
			}
			rows.Close()
		} else {
			log.Printf("[FiscalDiagnostico] cfop query: %v", err)
		}

		// 3. Distribuições simples (CST ICMS, CST PIS, centro fiscal, tipo contribuinte)
		dist := func(expr string) []diagDistRow {
			out := []diagDistRow{}
			q := `SELECT COALESCE(` + expr + `,''), COUNT(*), COALESCE(SUM(i.v_prod),0) ` + baseFrom +
				` GROUP BY 1 ORDER BY COUNT(*) DESC LIMIT 20`
			rs, qerr := db.Query(q, companyID, dataInicio, dataFim)
			if qerr != nil {
				log.Printf("[FiscalDiagnostico] dist(%s): %v", expr, qerr)
				return out
			}
			defer rs.Close()
			for rs.Next() {
				var d diagDistRow
				if rs.Scan(&d.Chave, &d.Itens, &d.VProd) == nil {
					out = append(out, d)
				}
			}
			return out
		}
		diag.PorCstIcms = dist(`NULLIF(i.cst_icms,'')`)
		diag.PorCstPis = dist(`NULLIF(i.cst_pis,'')`)
		diag.PorCentroFiscal = dist(`fei.input_params->>'PTipoCentroFiscal'`)
		diag.PorContribuinte = dist(`fei.input_params->>'PTipoContribuinte'`)

		// 4. Erros mais frequentes
		erows, err := db.Query(`
			SELECT COALESCE(fei.error_message,''), COUNT(*) `+baseFrom+`
			  AND fei.status NOT IN ('ok')
			GROUP BY 1 ORDER BY COUNT(*) DESC LIMIT 10`, companyID, dataInicio, dataFim)
		if err == nil {
			for erows.Next() {
				var e diagErroRow
				if erows.Scan(&e.Mensagem, &e.Itens) == nil {
					diag.Erros = append(diag.Erros, e)
				}
			}
			erows.Close()
		} else {
			log.Printf("[FiscalDiagnostico] erros query: %v", err)
		}

		if encErr := json.NewEncoder(w).Encode(diag); encErr != nil {
			log.Printf("[FiscalDiagnostico] encode: %v", encErr)
		}
	}
}
