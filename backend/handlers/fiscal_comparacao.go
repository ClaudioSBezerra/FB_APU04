// fiscal_comparacao.go — superfície backend da tela "Comparação Fiscal"
// (Fase 12, TPF-06/TPF-07). Dois handlers admin-gated e company-scoped:
//
//	GET /api/fiscal/comparacao/search?q=...   → FiscalComparacaoSearchHandler
//	GET /api/fiscal/comparacao?nfe_id=...     → FiscalComparacaoReadHandler
//
// Nenhuma lógica fiscal nova: reaproveita fiscal_execution_items (migration
// 147, Fase 11) e nfe_saidas_itens já existentes. Padrão de auth/IDOR/query
// copiado verbatim de admin_nf_cancelamento.go (busca ILIKE) e
// fiscal_execution.go (guard IDOR duplo company_id+nfe_id).
package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// NfeSearchResult representa um candidato retornado pela busca de NF-e por
// número ou chave de acesso (autocomplete server-side). Inclui os totais de
// imposto do CABEÇALHO da nota (nfe_saidas — vindos do bloco <ICMSTot> do
// XML), usados pela tela para o "Resumo da Nota" (acumulado dos itens vs.
// total declarado da NF), sem precisar de uma segunda chamada ao backend.
type NfeSearchResult struct {
	ID          string  `json:"id"`
	ChaveNFe    string  `json:"chave_nfe"`
	NumeroNFe   string  `json:"numero_nfe"`
	Serie       string  `json:"serie"`
	DestNome    string  `json:"dest_nome"`
	DataEmissao string  `json:"data_emissao"`
	VIcms       float64 `json:"v_icms"`
	VSt         float64 `json:"v_st"`
	VPis        float64 `json:"v_pis"`
	VCofins     float64 `json:"v_cofins"`
	VIbs        float64 `json:"v_ibs"`
	VCbs        float64 `json:"v_cbs"`
	// Identificação/valores do cabeçalho para o strip do "Resumo da Nota"
	VProd  float64 `json:"v_prod"`  // total dos produtos (valor da venda)
	VDesc  float64 `json:"v_desc"`  // total de descontos
	VFrete float64 `json:"v_frete"` // total do frete destacado
	VNf    float64 `json:"v_nf"`    // valor total da NF
	// FCP nos 3 sabores do XML: próprio (<vFCP>), do ST (<vFCPST>) e do DIFAL
	// (<vFCPUFDest>). O pacote devolve o FCP de destino EMBUTIDO no DIFAL
	// (ValorIcmsPobreza=0, alíquota interna cheia) — a tela compara
	// DIFAL = vICMSUFDest+vFCPUFDest e FCP = vFCP+vFCPST.
	VFcpSt     float64 `json:"v_fcp_st"`
	VFcpUfDest float64 `json:"v_fcp_uf_dest"`
	// Totais fiscais extras do cabeçalho para as colunas FCP/DIFAL/ICMS
	// Reduzido do Resumo da Nota
	VFcp        float64 `json:"v_fcp"`          // <vFCP>
	VIcmsUfDest float64 `json:"v_icms_uf_dest"` // <vICMSUFDest> (DIFAL)
	VIcmsDeson  float64 `json:"v_icms_deson"`   // <vICMSDeson> (ICMS desonerado/reduzido)
	// Status agregado da execução (calculado no servidor — colunas
	// Executado/Divergência do grid em qualquer volume)
	TotalItens    int  `json:"total_itens"`
	ExecItens     int  `json:"exec_itens"`
	ItensProblema int  `json:"itens_problema"` // executados com status != ok
	Divergente    bool `json:"divergente"`     // algum item ok fora da régua
}

// ComparacaoRow representa um item da comparação esperado (nfe_saidas_itens)
// vs. calculado (fiscal_execution_items). Ponteiros para os campos calculados
// porque o LEFT JOIN pode não ter linha correspondente (item nunca executado).
type ComparacaoRow struct {
	// Identificação (esperado)
	ID    string `json:"id"`
	NItem int    `json:"n_item"`
	CProd string `json:"c_prod"`
	XProd string `json:"x_prod"`
	NCM   string `json:"ncm"`
	CFOP  string `json:"cfop"`

	// Esperado (nfe_saidas_itens)
	// CST/valores comerciais do item — usados para derivar a "base reduzida
	// esperada" do XML (CST 20/70: valor bruto do item − v_bc_icms)
	CstIcms string  `json:"cst_icms"`
	VProd   float64 `json:"v_prod"`
	VFrete  float64 `json:"v_frete"`
	VDesc   float64 `json:"v_desc"`
	VOutro  float64 `json:"v_outro"`

	VBcIcms   float64 `json:"v_bc_icms"`
	VIcms     float64 `json:"v_icms"`
	VBcSt     float64 `json:"v_bc_st"`
	VSt       float64 `json:"v_st"`
	VBcPis    float64 `json:"v_bc_pis"`
	VPis      float64 `json:"v_pis"`
	PPis      float64 `json:"p_pis"` // alíquota — ajusta o esperado no modo inclusão IBS/CBS
	VBcCofins float64 `json:"v_bc_cofins"`
	VCofins   float64 `json:"v_cofins"`
	PCofins   float64 `json:"p_cofins"`
	VIbs      float64 `json:"v_ibs"`
	VCbs      float64 `json:"v_cbs"`

	// Calculado (fiscal_execution_items) — sempre presentes como coluna,
	// mas com valor NULL quando o item nunca foi executado (LEFT JOIN).
	Status                   string   `json:"status"` // COALESCE(fei.status, 'not_executed')
	ErrorMessage             *string  `json:"error_message"`
	ExecutedAt               *string  `json:"executed_at"`
	BaseCalculoIcms          *float64 `json:"base_calculo_icms"`
	ValorIcms                *float64 `json:"valor_icms"`
	BaseSubstituicao         *float64 `json:"base_substituicao"`
	ValorSubstituicao        *float64 `json:"valor_substituicao"`
	BaseCalculoPis           *float64 `json:"base_calculo_pis"`
	ValorPis                 *float64 `json:"valor_pis"`
	BaseCalculoCofins        *float64 `json:"base_calculo_cofins"`
	ValorCofins              *float64 `json:"valor_cofins"`
	ValorIbsTotal            *float64 `json:"valor_ibs_total"` // valor_ibs_uf + valor_ibs_mun somado no SQL
	ValorCbs                 *float64 `json:"valor_cbs"`
	PercentualDifal          *float64 `json:"percentual_difal"`
	ValorIcmsPartilhaDestino *float64 `json:"valor_icms_partilha_destino"`
	ValorIcmsPobreza         *float64 `json:"valor_icms_pobreza"`
	GrupoFiscalCodigo        *string  `json:"grupo_fiscal_codigo"`
	// BaseCalculoIbsCbs — base de cálculo compartilhada entre IBS e CBS. Não
	// existe coluna dedicada em fiscal_execution_items (só valor_ibs_uf/
	// valor_ibs_mun/valor_cbs) nem em nfe_saidas_itens (XML não traz essa
	// base hoje) — extraído do full_result JSONB (campo Go BaseCalculoIbsCbs
	// do pacote Oracle, oracle_fiscal.go). Só existe lado "calculado"; sem
	// "esperado" para comparar, não entra em getTaxPairs/divergência no
	// frontend — é informativo.
	BaseCalculoIbsCbs *float64 `json:"base_calculo_ibs_cbs"`
	// ValorReducao — redução de base concedida pelo pacote (usado no acumulado
	// "ICMS Reduzido" do Resumo da Nota, comparado com v_icms_deson do
	// cabeçalho). Extraído do full_result JSONB, como BaseCalculoIbsCbs.
	ValorReducao *float64 `json:"valor_reducao"`
	// FullResult — retorno completo do pacote (~88 campos), para a seção de
	// diagnóstico do dialog de detalhe (Mensagem1-4, natureza da operação,
	// CST, leis, id das regras aplicadas). Null quando nunca executado.
	FullResult json.RawMessage `json:"full_result"`
	// Simulacao — comparação "IBS/CBS na base do ICMS" (fiscalSimulacao):
	// original × simulado interno × pacote 2ª chamada. Null quando a execução
	// não rodou em modo simulação.
	Simulacao json.RawMessage `json:"simulacao"`
}

// sqlItemDivergente — expressão booleana de divergência POR ITEM (aliases
// fei = fiscal_execution_items, i = pacotefiscal_nfe_saidas_itens), com a
// MESMA régua da tela: modo simulação IBS/CBS (esperado ajustado, tolerância
// de 1 centavo) ou modo normal (XML cru, tolerância zero). Fonte única usada
// pelo LATERAL de status do grid e pelo filtro "Resultado".
const sqlItemDivergente = `(fei.status = 'ok' AND (
	CASE WHEN fei.simulacao IS NOT NULL AND fei.simulacao->>'erro' IS NULL THEN
	     abs(COALESCE((fei.simulacao->>'icms_simulado')::numeric,0) - COALESCE(fei.valor_icms,0)) > 0.011
	  OR abs(COALESCE((fei.simulacao->>'st_simulado')::numeric,0) - COALESCE(fei.valor_substituicao,0)) > 0.011
	  OR abs((COALESCE(i.v_pis,0) - (COALESCE(fei.valor_icms,0)-COALESCE(i.v_icms,0))*COALESCE(i.p_pis,0)/100.0) - COALESCE(fei.valor_pis,0)) > 0.011
	  OR abs((COALESCE(i.v_cofins,0) - (COALESCE(fei.valor_icms,0)-COALESCE(i.v_icms,0))*COALESCE(i.p_cofins,0)/100.0) - COALESCE(fei.valor_cofins,0)) > 0.011
	  OR abs(COALESCE(i.v_ibs,0) - (COALESCE(fei.valor_ibs_uf,0)+COALESCE(fei.valor_ibs_mun,0))) > 0.011
	  OR abs(COALESCE(i.v_cbs,0) - COALESCE(fei.valor_cbs,0)) > 0.011
	ELSE
	     abs(COALESCE(i.v_icms,0) - COALESCE(fei.valor_icms,0)) > 0
	  OR abs(COALESCE(i.v_st,0) - COALESCE(fei.valor_substituicao,0)) > 0
	  OR abs(COALESCE(i.v_pis,0) - COALESCE(fei.valor_pis,0)) > 0
	  OR abs(COALESCE(i.v_cofins,0) - COALESCE(fei.valor_cofins,0)) > 0
	  OR abs(COALESCE(i.v_ibs,0) - (COALESCE(fei.valor_ibs_uf,0)+COALESCE(fei.valor_ibs_mun,0))) > 0
	  OR abs(COALESCE(i.v_cbs,0) - COALESCE(fei.valor_cbs,0)) > 0
	END))`

// NfeSearchResponse é o envelope paginado da busca: total de notas que batem
// nos filtros (para os controles de página) + a página solicitada.
type NfeSearchResponse struct {
	Total    int               `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"` // 0 = todas
	Rows     []NfeSearchResult `json:"rows"`
}

// ---------------------------------------------------------------------------
// GET /api/fiscal/comparacao/search?q=...&page=1&page_size=50
// Busca NF-e de saída company-scoped, com filtros fiscais opcionais
// (com_icms/com_st/com_difal/com_fcp/com_base_reduzida=1) e paginação (page_size=0
// traz todas). Resposta paginada com total.
// ---------------------------------------------------------------------------
func FiscalComparacaoSearchHandler(db *sql.DB) http.HandlerFunc {
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

		q := strings.TrimSpace(r.URL.Query().Get("q"))
		dataInicio := strings.TrimSpace(r.URL.Query().Get("data_inicio"))
		dataFim := strings.TrimSpace(r.URL.Query().Get("data_fim"))
		ufOrigem := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("uf_origem")))
		ufDestino := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("uf_destino")))
		cliente := strings.TrimSpace(r.URL.Query().Get("cliente"))
		emitente := strings.TrimSpace(r.URL.Query().Get("emitente"))

		// Sem nenhum filtro, roda mesmo assim e lista as notas mais recentes
		// da empresa (mesmo padrão de "Nota a Nota" em Painel XMLs) — não exige
		// mais 3+ caracteres em "q" para trazer resultado.
		where := "WHERE n.company_id = $1"
		args := []interface{}{companyID}
		idx := 2

		if q != "" {
			where += fmt.Sprintf(" AND (n.numero_nfe ILIKE '%%'||$%d||'%%' OR n.chave_nfe ILIKE '%%'||$%d||'%%')", idx, idx)
			args = append(args, q)
			idx++
		}
		if dataInicio != "" {
			where += fmt.Sprintf(" AND n.data_emissao >= $%d", idx)
			args = append(args, dataInicio)
			idx++
		}
		if dataFim != "" {
			where += fmt.Sprintf(" AND n.data_emissao <= $%d", idx)
			args = append(args, dataFim)
			idx++
		}
		if ufOrigem != "" {
			where += fmt.Sprintf(" AND n.emit_uf = $%d", idx)
			args = append(args, ufOrigem)
			idx++
		}
		if ufDestino != "" {
			where += fmt.Sprintf(" AND n.dest_uf = $%d", idx)
			args = append(args, ufDestino)
			idx++
		}
		if cliente != "" {
			where += fmt.Sprintf(" AND n.dest_xnome ILIKE '%%'||$%d||'%%'", idx)
			args = append(args, cliente)
			idx++
		}
		if emitente != "" {
			where += fmt.Sprintf(" AND n.emit_xnome ILIKE '%%'||$%d||'%%'", idx)
			args = append(args, emitente)
			idx++
		}

		// Filtros fiscais (checkboxes da tela) — todos sobre totais do
		// cabeçalho, exceto base reduzida, que só existe no item (CST 20/70).
		if r.URL.Query().Get("com_icms") == "1" {
			where += " AND COALESCE(n.v_icms,0) > 0"
		}
		if r.URL.Query().Get("com_st") == "1" {
			where += " AND COALESCE(n.v_st,0) > 0"
		}
		if r.URL.Query().Get("com_difal") == "1" {
			where += " AND COALESCE(n.v_icms_uf_dest,0) > 0"
		}
		if r.URL.Query().Get("com_fcp") == "1" {
			where += " AND (COALESCE(n.v_fcp,0) + COALESCE(n.v_fcp_st,0) + COALESCE(n.v_fcp_uf_dest,0)) > 0"
		}
		if r.URL.Query().Get("com_base_reduzida") == "1" {
			where += ` AND EXISTS (
				SELECT 1 FROM pacotefiscal_nfe_saidas_itens i
				WHERE i.nfe_id = n.id AND i.cst_icms IN ('20','70'))`
		}
		// SOMENTE VENDAS (padrão da tela, decisão 2026-07-08): o pacote fiscal
		// domina operações de VENDA — remessas, devoluções, bonificações,
		// consertos, transferências etc. geram "falso erro". A nota só entra
		// quando TODOS os itens têm CFOP de venda (nota mista fica de fora):
		// grupos 5.1xx/6.1xx exceto 5.15x/6.15x (transferências) + vendas ST
		// (5401/5402/5403/5405 e 6401-6404).
		if r.URL.Query().Get("somente_vendas") == "1" {
			where += ` AND NOT EXISTS (
				SELECT 1 FROM pacotefiscal_nfe_saidas_itens i
				WHERE i.nfe_id = n.id AND NOT (
					(i.cfop LIKE '51%' AND i.cfop NOT LIKE '515%')
					OR (i.cfop LIKE '61%' AND i.cfop NOT LIKE '615%')
					OR i.cfop IN ('5401','5402','5403','5405','6401','6402','6403','6404')
				))`
		}

		// RESULTADO da execução (filtro pós-processamento, 2026-07-08): isola
		// as notas por veredito com a MESMA régua das colunas do grid.
		switch r.URL.Query().Get("resultado") {
		case "divergentes":
			where += ` AND EXISTS (
				SELECT 1 FROM pacotefiscal_nfe_saidas_itens i
				JOIN fiscal_execution_items fei ON fei.nfe_item_id = i.id
				WHERE i.nfe_id = n.id AND ` + sqlItemDivergente + `)`
		case "com_erro":
			where += ` AND EXISTS (
				SELECT 1 FROM pacotefiscal_nfe_saidas_itens i
				JOIN fiscal_execution_items fei ON fei.nfe_item_id = i.id
				WHERE i.nfe_id = n.id AND fei.status <> 'ok')`
		case "ok":
			// Totalmente executada, sem item problemático e sem divergência
			where += ` AND NOT EXISTS (
				SELECT 1 FROM pacotefiscal_nfe_saidas_itens i
				LEFT JOIN fiscal_execution_items fei ON fei.nfe_item_id = i.id
				WHERE i.nfe_id = n.id AND (fei.id IS NULL OR fei.status <> 'ok' OR ` + sqlItemDivergente + `))
				AND EXISTS (
				SELECT 1 FROM pacotefiscal_nfe_saidas_itens i
				JOIN fiscal_execution_items fei ON fei.nfe_item_id = i.id
				WHERE i.nfe_id = n.id)`
		case "nao_executadas":
			where += ` AND NOT EXISTS (
				SELECT 1 FROM pacotefiscal_nfe_saidas_itens i
				JOIN fiscal_execution_items fei ON fei.nfe_item_id = i.id
				WHERE i.nfe_id = n.id)`
		}

		// Paginação: page 1-based; page_size 0 = todas (sem LIMIT).
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
		pageSize := 50
		if ps := strings.TrimSpace(r.URL.Query().Get("page_size")); ps != "" {
			if v, convErr := strconv.Atoi(ps); convErr == nil && v >= 0 {
				pageSize = v
			}
		}

		var total int
		if err := db.QueryRow("SELECT COUNT(*) FROM pacotefiscal_nfe_saidas n "+where, args...).Scan(&total); err != nil {
			log.Printf("[FiscalComparacaoSearch] count error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao buscar NF-e")
			return
		}

		limitClause := ""
		if pageSize > 0 {
			limitClause = fmt.Sprintf(" LIMIT %d OFFSET %d", pageSize, (page-1)*pageSize)
		}

		// st (LATERAL): status agregado da execução por nota, calculado no
		// servidor — o grid mostra Executado/Divergência para QUALQUER volume
		// (2026-07-08: com 8.000 notas o usuário ficava às cegas; a avaliação
		// no cliente só cobria páginas ≤100). Divergência item a item com a
		// régua da tela (modo simulação: esperado ajustado + tolerância 1
		// centavo; sem simulação: XML cru). Divergências nota-level
		// (DIFAL/FCP/reduzido) continuam só no detalhe.
		query := fmt.Sprintf(`
			SELECT n.id, n.chave_nfe, COALESCE(n.numero_nfe,''), COALESCE(n.serie,''),
			       COALESCE(n.dest_xnome,''), TO_CHAR(n.data_emissao,'DD/MM/YYYY'),
			       COALESCE(n.v_icms,0), COALESCE(n.v_st,0), COALESCE(n.v_pis,0),
			       COALESCE(n.v_cofins,0), COALESCE(n.v_ibs,0), COALESCE(n.v_cbs,0),
			       COALESCE(n.v_prod,0), COALESCE(n.v_desc,0), COALESCE(n.v_frete,0), COALESCE(n.v_nf,0),
			       COALESCE(n.v_fcp,0), COALESCE(n.v_icms_uf_dest,0), COALESCE(n.v_icms_deson,0),
			       COALESCE(n.v_fcp_st,0), COALESCE(n.v_fcp_uf_dest,0),
			       COALESCE(st.total_itens,0), COALESCE(st.exec_itens,0),
			       COALESCE(st.itens_problema,0), COALESCE(st.divergente,false)
			FROM pacotefiscal_nfe_saidas n
			LEFT JOIN LATERAL (
				SELECT COUNT(*) AS total_itens,
				       COUNT(fei.id) AS exec_itens,
				       COUNT(*) FILTER (WHERE fei.id IS NOT NULL AND fei.status <> 'ok') AS itens_problema,
				       BOOL_OR(`+sqlItemDivergente+`) AS divergente
				FROM pacotefiscal_nfe_saidas_itens i
				LEFT JOIN fiscal_execution_items fei ON fei.nfe_item_id = i.id
				WHERE i.nfe_id = n.id
			) st ON true
			%s
			ORDER BY n.data_emissao DESC, n.numero_nfe DESC
			%s`, where, limitClause)

		rows, err := db.Query(query, args...)
		if err != nil {
			log.Printf("[FiscalComparacaoSearch] query error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao buscar NF-e")
			return
		}
		defer rows.Close()

		result := []NfeSearchResult{}
		for rows.Next() {
			var row NfeSearchResult
			if err := rows.Scan(&row.ID, &row.ChaveNFe, &row.NumeroNFe, &row.Serie,
				&row.DestNome, &row.DataEmissao,
				&row.VIcms, &row.VSt, &row.VPis, &row.VCofins, &row.VIbs, &row.VCbs,
				&row.VProd, &row.VDesc, &row.VFrete, &row.VNf,
				&row.VFcp, &row.VIcmsUfDest, &row.VIcmsDeson,
				&row.VFcpSt, &row.VFcpUfDest,
				&row.TotalItens, &row.ExecItens, &row.ItensProblema, &row.Divergente); err != nil {
				log.Printf("[FiscalComparacaoSearch] scan error: %v", err)
				continue
			}
			result = append(result, row)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[FiscalComparacaoSearch] rows error: %v", err)
		}

		if encErr := json.NewEncoder(w).Encode(NfeSearchResponse{
			Total: total, Page: page, PageSize: pageSize, Rows: result,
		}); encErr != nil {
			log.Printf("[FiscalComparacaoSearch] encode error: %v", encErr)
		}
	}
}

// ---------------------------------------------------------------------------
// queryComparacaoRows — helper interna compartilhada entre o handler JSON
// (FiscalComparacaoReadHandler) e o handler CSV (Task 2, fiscal_comparacao_csv.go).
// Guard IDOR duplo: WHERE nsi.nfe_id = $1 AND nsi.company_id = $2 — nunca
// confia em nfe_id/company_id vindos soltos do cliente (T-12-01).
//
// COALESCE(fei.status, 'not_executed') distingue item nunca executado (sem
// linha em fiscal_execution_items) de erro real (Pitfall 1 do 12-RESEARCH.md).
// valor_ibs_uf + valor_ibs_mun somados uma única vez aqui — reusado pelo CSV
// (Pitfall 2 do 12-RESEARCH.md, não existe coluna de total em
// fiscal_execution_items).
// ---------------------------------------------------------------------------
func queryComparacaoRows(db *sql.DB, nfeID, companyID string) ([]ComparacaoRow, error) {
	rows, err := db.Query(`
		SELECT
			nsi.id, nsi.n_item, COALESCE(nsi.c_prod,''), nsi.x_prod, COALESCE(nsi.ncm,''), COALESCE(nsi.cfop,''),
			COALESCE(nsi.cst_icms,''), COALESCE(nsi.v_prod,0), COALESCE(nsi.v_frete,0), COALESCE(nsi.v_desc,0), COALESCE(nsi.v_outro,0),
			COALESCE(nsi.v_bc_icms,0), COALESCE(nsi.v_icms,0),
			COALESCE(nsi.v_bc_st,0), COALESCE(nsi.v_st,0),
			COALESCE(nsi.v_bc_pis,0), COALESCE(nsi.v_pis,0), COALESCE(nsi.p_pis,0),
			COALESCE(nsi.v_bc_cofins,0), COALESCE(nsi.v_cofins,0), COALESCE(nsi.p_cofins,0),
			COALESCE(nsi.v_ibs,0), COALESCE(nsi.v_cbs,0),
			COALESCE(fei.status, 'not_executed'), fei.error_message, fei.executed_at,
			fei.base_calculo_icms, fei.valor_icms,
			fei.base_substituicao, fei.valor_substituicao,
			fei.base_calculo_pis, fei.valor_pis,
			fei.base_calculo_cofins, fei.valor_cofins,
			(COALESCE(fei.valor_ibs_uf,0) + COALESCE(fei.valor_ibs_mun,0)) AS valor_ibs_total,
			fei.valor_cbs,
			fei.percentual_difal, fei.valor_icms_partilha_destino, fei.valor_icms_pobreza,
			fei.grupo_fiscal_codigo,
			(fei.full_result->>'BaseCalculoIbsCbs')::numeric AS base_calculo_ibs_cbs,
			(fei.full_result->>'ValorReducao')::numeric AS valor_reducao,
			fei.full_result,
			fei.simulacao
		FROM pacotefiscal_nfe_saidas_itens nsi
		LEFT JOIN fiscal_execution_items fei ON fei.nfe_item_id = nsi.id
		WHERE nsi.nfe_id = $1 AND nsi.company_id = $2
		ORDER BY nsi.n_item ASC`, nfeID, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []ComparacaoRow{}
	for rows.Next() {
		var row ComparacaoRow
		var executedAt sql.NullTime
		var hasIbsTotal sql.NullFloat64
		var fullResult, simulacao sql.NullString

		if err := rows.Scan(
			&row.ID, &row.NItem, &row.CProd, &row.XProd, &row.NCM, &row.CFOP,
			&row.CstIcms, &row.VProd, &row.VFrete, &row.VDesc, &row.VOutro,
			&row.VBcIcms, &row.VIcms,
			&row.VBcSt, &row.VSt,
			&row.VBcPis, &row.VPis, &row.PPis,
			&row.VBcCofins, &row.VCofins, &row.PCofins,
			&row.VIbs, &row.VCbs,
			&row.Status, &row.ErrorMessage, &executedAt,
			&row.BaseCalculoIcms, &row.ValorIcms,
			&row.BaseSubstituicao, &row.ValorSubstituicao,
			&row.BaseCalculoPis, &row.ValorPis,
			&row.BaseCalculoCofins, &row.ValorCofins,
			&hasIbsTotal,
			&row.ValorCbs,
			&row.PercentualDifal, &row.ValorIcmsPartilhaDestino, &row.ValorIcmsPobreza,
			&row.GrupoFiscalCodigo,
			&row.BaseCalculoIbsCbs,
			&row.ValorReducao,
			&fullResult,
			&simulacao,
		); err != nil {
			log.Printf("[FiscalComparacaoRead] scan error: %v", err)
			continue
		}

		if executedAt.Valid {
			s := executedAt.Time.Format("2006-01-02T15:04:05Z07:00")
			row.ExecutedAt = &s
		}
		if fullResult.Valid && fullResult.String != "" {
			row.FullResult = json.RawMessage(fullResult.String)
		}
		if simulacao.Valid && simulacao.String != "" {
			row.Simulacao = json.RawMessage(simulacao.String)
		}
		if hasIbsTotal.Valid {
			v := hasIbsTotal.Float64
			row.ValorIbsTotal = &v
		}

		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// GET /api/fiscal/comparacao?nfe_id=...
// Leitura da comparação item a item: esperado (nfe_saidas_itens) vs.
// calculado (fiscal_execution_items) para ICMS, ICMS-ST, PIS, COFINS, IBS, CBS.
// ---------------------------------------------------------------------------
func FiscalComparacaoReadHandler(db *sql.DB) http.HandlerFunc {
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

		nfeID := strings.TrimSpace(r.URL.Query().Get("nfe_id"))
		if nfeID == "" {
			jsonErr(w, http.StatusBadRequest, "nfe_id é obrigatório")
			return
		}

		data, err := queryComparacaoRows(db, nfeID, companyID)
		if err != nil {
			log.Printf("[FiscalComparacaoRead] query error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar comparação fiscal")
			return
		}

		if encErr := json.NewEncoder(w).Encode(data); encErr != nil {
			log.Printf("[FiscalComparacaoRead] encode error: %v", encErr)
		}
	}
}
