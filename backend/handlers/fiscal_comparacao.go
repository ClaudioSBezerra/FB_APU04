// fiscal_comparacao.go — superfície backend da tela "Comparação Fiscal"
// (Fase 12, TPF-06/TPF-07). Dois handlers admin-gated e company-scoped:
//
//   GET /api/fiscal/comparacao/search?q=...   → FiscalComparacaoSearchHandler
//   GET /api/fiscal/comparacao?nfe_id=...     → FiscalComparacaoReadHandler
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
}

// ComparacaoRow representa um item da comparação esperado (nfe_saidas_itens)
// vs. calculado (fiscal_execution_items). Ponteiros para os campos calculados
// porque o LEFT JOIN pode não ter linha correspondente (item nunca executado).
type ComparacaoRow struct {
	// Identificação (esperado)
	ID     string  `json:"id"`
	NItem  int     `json:"n_item"`
	CProd  string  `json:"c_prod"`
	XProd  string  `json:"x_prod"`
	NCM    string  `json:"ncm"`
	CFOP   string  `json:"cfop"`

	// Esperado (nfe_saidas_itens)
	VBcIcms    float64 `json:"v_bc_icms"`
	VIcms      float64 `json:"v_icms"`
	VBcSt      float64 `json:"v_bc_st"`
	VSt        float64 `json:"v_st"`
	VBcPis     float64 `json:"v_bc_pis"`
	VPis       float64 `json:"v_pis"`
	VBcCofins  float64 `json:"v_bc_cofins"`
	VCofins    float64 `json:"v_cofins"`
	VIbs       float64 `json:"v_ibs"`
	VCbs       float64 `json:"v_cbs"`

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
}

// ---------------------------------------------------------------------------
// GET /api/fiscal/comparacao/search?q=...
// Busca NF-e de saída por número ou chave de acesso, company-scoped.
// Retorna até 20 candidatos. Nunca roda a query com menos de 3 caracteres —
// responde []  imediatamente (nunca null).
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

		// Sem nenhum filtro, roda mesmo assim e lista as 50 notas mais recentes
		// da empresa (mesmo padrão de "Nota a Nota" em Painel XMLs) — não exige
		// mais 3+ caracteres em "q" para trazer resultado.
		where := "WHERE company_id = $1"
		args := []interface{}{companyID}
		idx := 2

		if q != "" {
			where += fmt.Sprintf(" AND (numero_nfe ILIKE '%%'||$%d||'%%' OR chave_nfe ILIKE '%%'||$%d||'%%')", idx, idx)
			args = append(args, q)
			idx++
		}
		if dataInicio != "" {
			where += fmt.Sprintf(" AND data_emissao >= $%d", idx)
			args = append(args, dataInicio)
			idx++
		}
		if dataFim != "" {
			where += fmt.Sprintf(" AND data_emissao <= $%d", idx)
			args = append(args, dataFim)
			idx++
		}
		if ufOrigem != "" {
			where += fmt.Sprintf(" AND emit_uf = $%d", idx)
			args = append(args, ufOrigem)
			idx++
		}
		if ufDestino != "" {
			where += fmt.Sprintf(" AND dest_uf = $%d", idx)
			args = append(args, ufDestino)
			idx++
		}
		if cliente != "" {
			where += fmt.Sprintf(" AND dest_xnome ILIKE '%%'||$%d||'%%'", idx)
			args = append(args, cliente)
			idx++
		}
		if emitente != "" {
			where += fmt.Sprintf(" AND emit_xnome ILIKE '%%'||$%d||'%%'", idx)
			args = append(args, emitente)
			idx++
		}

		query := fmt.Sprintf(`
			SELECT id, chave_nfe, COALESCE(numero_nfe,''), COALESCE(serie,''),
			       COALESCE(dest_xnome,''), TO_CHAR(data_emissao,'DD/MM/YYYY'),
			       COALESCE(v_icms,0), COALESCE(v_st,0), COALESCE(v_pis,0),
			       COALESCE(v_cofins,0), COALESCE(v_ibs,0), COALESCE(v_cbs,0),
			       COALESCE(v_prod,0), COALESCE(v_desc,0), COALESCE(v_frete,0), COALESCE(v_nf,0)
			FROM pacotefiscal_nfe_saidas
			%s
			ORDER BY data_emissao DESC
			LIMIT 50`, where)

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
				&row.VProd, &row.VDesc, &row.VFrete, &row.VNf); err != nil {
				log.Printf("[FiscalComparacaoSearch] scan error: %v", err)
				continue
			}
			result = append(result, row)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[FiscalComparacaoSearch] rows error: %v", err)
		}

		if encErr := json.NewEncoder(w).Encode(result); encErr != nil {
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
			COALESCE(nsi.v_bc_icms,0), COALESCE(nsi.v_icms,0),
			COALESCE(nsi.v_bc_st,0), COALESCE(nsi.v_st,0),
			COALESCE(nsi.v_bc_pis,0), COALESCE(nsi.v_pis,0),
			COALESCE(nsi.v_bc_cofins,0), COALESCE(nsi.v_cofins,0),
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
			(fei.full_result->>'BaseCalculoIbsCbs')::numeric AS base_calculo_ibs_cbs
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

		if err := rows.Scan(
			&row.ID, &row.NItem, &row.CProd, &row.XProd, &row.NCM, &row.CFOP,
			&row.VBcIcms, &row.VIcms,
			&row.VBcSt, &row.VSt,
			&row.VBcPis, &row.VPis,
			&row.VBcCofins, &row.VCofins,
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
		); err != nil {
			log.Printf("[FiscalComparacaoRead] scan error: %v", err)
			continue
		}

		if executedAt.Valid {
			s := executedAt.Time.Format("2006-01-02T15:04:05Z07:00")
			row.ExecutedAt = &s
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
