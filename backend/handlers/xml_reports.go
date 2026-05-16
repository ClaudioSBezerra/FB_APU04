package handlers

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// ---------------------------------------------------------------------------
// xml_reports.go — Relatórios de Saneamento CCLASSTRIB
//
// GET /api/xml/reports/saneamento/csv          → XMLSaneamentoCSVHandler
// GET /api/xml/reports/saneamento              → XMLSaneamentoCCLASSTRIBHandler
// GET /api/xml/reports/fornecedores-cclasstrib → XMLFornecedoresCCLASSTRIBHandler
//
// Todos os handlers:
//   - Usam GetEffectiveCompanyID (company_id do usuário autenticado)
//   - Usam parâmetros $N nas queries (nunca concatenação de strings com input do usuário)
//   - Não setam CORS headers (tratados pelo SecurityMiddleware em main.go)
// ---------------------------------------------------------------------------

// saneamentoRow representa um NCM com divergência de classificação tributária.
type saneamentoRow struct {
	NCM                 string   `json:"ncm"`
	VariantesCstPis     int      `json:"variantes_cst_pis"`
	VariantesCstCofins  int      `json:"variantes_cst_cofins"`
	VariantesCclasstrib int      `json:"variantes_cclasstrib"`
	TemCclasstribNulo   bool     `json:"tem_cclasstrib_nulo"`
	QtdItens            int      `json:"qtd_itens"`
	VPisTotal           float64  `json:"v_pis_total"`
	VCofinsTotal        float64  `json:"v_cofins_total"`
	CstsPis             []string `json:"csts_pis"`
	CstsCofins          []string `json:"csts_cofins"`
}

// parsePgArray converte o formato PostgreSQL array literal "{a,b,c}" em []string.
func parsePgArray(src interface{}) []string {
	if src == nil {
		return []string{}
	}
	var raw string
	switch v := src.(type) {
	case string:
		raw = v
	case []byte:
		raw = string(v)
	default:
		return []string{}
	}
	raw = strings.TrimPrefix(raw, "{")
	raw = strings.TrimSuffix(raw, "}")
	if raw == "" {
		return []string{}
	}
	return strings.Split(raw, ",")
}

// executeSaneamentoQuery executa a query central de saneamento e retorna os dados.
func executeSaneamentoQuery(db *sql.DB, companyID, mesAno string) ([]saneamentoRow, error) {
	args := []interface{}{companyID}
	whereClause := "WHERE ei.company_id = $1"
	paramIdx := 2

	if mesAno != "" {
		whereClause += fmt.Sprintf(
			" AND ei.nfe_id IN (SELECT id FROM nfe_entradas WHERE company_id = $1 AND mes_ano = $%d)",
			paramIdx,
		)
		args = append(args, mesAno)
	}

	query := fmt.Sprintf(`
SELECT
    ei.ncm,
    COUNT(DISTINCT ei.cst_pis)    AS variantes_cst_pis,
    COUNT(DISTINCT ei.cst_cofins) AS variantes_cst_cofins,
    COUNT(DISTINCT ei.cclasstrib) FILTER (WHERE ei.cclasstrib IS NOT NULL) AS variantes_cclasstrib,
    BOOL_OR(ei.cclasstrib IS NULL) AS tem_cclasstrib_nulo,
    COUNT(*) AS qtd_itens,
    COALESCE(SUM(ei.v_pis),    0) AS v_pis_total,
    COALESCE(SUM(ei.v_cofins), 0) AS v_cofins_total,
    COALESCE(array_agg(DISTINCT ei.cst_pis)    FILTER (WHERE ei.cst_pis IS NOT NULL),    ARRAY[]::text[]) AS csts_pis,
    COALESCE(array_agg(DISTINCT ei.cst_cofins) FILTER (WHERE ei.cst_cofins IS NOT NULL), ARRAY[]::text[]) AS csts_cofins
FROM nfe_entradas_itens ei
%s
GROUP BY ei.ncm
HAVING COUNT(DISTINCT ei.cst_pis) > 1
    OR COUNT(DISTINCT ei.cst_cofins) > 1
    OR BOOL_OR(ei.cclasstrib IS NULL)
ORDER BY qtd_itens DESC
LIMIT 500`, whereClause)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []saneamentoRow
	for rows.Next() {
		var row saneamentoRow
		var rawCstsPis interface{}
		var rawCstsCofins interface{}
		if err := rows.Scan(
			&row.NCM,
			&row.VariantesCstPis,
			&row.VariantesCstCofins,
			&row.VariantesCclasstrib,
			&row.TemCclasstribNulo,
			&row.QtdItens,
			&row.VPisTotal,
			&row.VCofinsTotal,
			&rawCstsPis,
			&rawCstsCofins,
		); err != nil {
			log.Printf("[XMLReports] scan error: %v", err)
			continue
		}
		row.CstsPis = parsePgArray(rawCstsPis)
		row.CstsCofins = parsePgArray(rawCstsCofins)
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// XMLSaneamentoCCLASSTRIBHandler — GET /api/xml/reports/saneamento
// Retorna JSON com NCMs que têm divergência de CST entre fornecedores.
// ---------------------------------------------------------------------------
func XMLSaneamentoCCLASSTRIBHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodGet {
			jsonErr(w, http.StatusMethodNotAllowed, "Método não permitido")
			return
		}

		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			jsonErr(w, http.StatusUnauthorized, "Não autenticado")
			return
		}
		userID, _ := claims["user_id"].(string)

		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa: "+err.Error())
			return
		}

		mesAno := strings.TrimSpace(r.URL.Query().Get("mes_ano"))

		data, err := executeSaneamentoQuery(db, companyID, mesAno)
		if err != nil {
			log.Printf("[XMLReports/Saneamento] query error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar banco")
			return
		}

		// Retornar array vazio em vez de null quando não há divergências
		if data == nil {
			data = []saneamentoRow{}
		}

		if encErr := json.NewEncoder(w).Encode(data); encErr != nil {
			log.Printf("[XMLReports/Saneamento] encode error: %v", encErr)
		}
	}
}

// ---------------------------------------------------------------------------
// XMLSaneamentoCSVHandler — GET /api/xml/reports/saneamento/csv
// Exporta os dados de saneamento como CSV para download e correção do cadastro.
// Cabeçalho em PT-BR. Coluna "Sugestão CCLASSTRIB" vazia — preenchida pelo contador (D-16b).
// ---------------------------------------------------------------------------
func XMLSaneamentoCSVHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			http.Error(w, "Não autenticado", http.StatusUnauthorized)
			return
		}
		userID, _ := claims["user_id"].(string)

		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			http.Error(w, "Erro ao obter empresa: "+err.Error(), http.StatusInternalServerError)
			return
		}

		mesAno := strings.TrimSpace(r.URL.Query().Get("mes_ano"))

		data, err := executeSaneamentoQuery(db, companyID, mesAno)
		if err != nil {
			log.Printf("[XMLReports/SaneamentoCSV] query error: %v", err)
			http.Error(w, "Erro ao consultar banco", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="saneamento-cclasstrib.csv"`)

		cw := csv.NewWriter(w)

		// Cabeçalho em PT-BR conforme D-16b
		header := []string{
			"NCM",
			"Variantes CST PIS",
			"Variantes CST COFINS",
			"Variantes CCLASSTRIB",
			"CCLASSTRIB Ausente",
			"Qtd Itens",
			"V. PIS Total",
			"V. COFINS Total",
			"CSTs PIS Encontrados",
			"CSTs COFINS Encontrados",
			"Sugestão CCLASSTRIB",
		}
		if err := cw.Write(header); err != nil {
			log.Printf("[XMLReports/SaneamentoCSV] write header error: %v", err)
			return
		}

		for _, row := range data {
			ausente := "Não"
			if row.TemCclasstribNulo {
				ausente = "Sim"
			}
			record := []string{
				row.NCM,
				fmt.Sprintf("%d", row.VariantesCstPis),
				fmt.Sprintf("%d", row.VariantesCstCofins),
				fmt.Sprintf("%d", row.VariantesCclasstrib),
				ausente,
				fmt.Sprintf("%d", row.QtdItens),
				fmt.Sprintf("%.2f", row.VPisTotal),
				fmt.Sprintf("%.2f", row.VCofinsTotal),
				strings.Join(row.CstsPis, "; "),
				strings.Join(row.CstsCofins, "; "),
				"", // Sugestão CCLASSTRIB — preenchido manualmente pelo contador
			}
			if err := cw.Write(record); err != nil {
				log.Printf("[XMLReports/SaneamentoCSV] write row error: %v", err)
				return
			}
		}

		cw.Flush()
		if err := cw.Error(); err != nil {
			log.Printf("[XMLReports/SaneamentoCSV] flush error: %v", err)
		}
	}
}

// ---------------------------------------------------------------------------
// XMLFornecedoresCCLASSTRIBHandler — GET /api/xml/reports/fornecedores-cclasstrib
// Retorna fornecedores cujas notas têm itens com cclasstrib NULL ou múltiplas
// classificações para o mesmo NCM. Ordenado por V. PIS+COFINS desc (mais crítico primeiro).
// ---------------------------------------------------------------------------
func XMLFornecedoresCCLASSTRIBHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodGet {
			jsonErr(w, http.StatusMethodNotAllowed, "Método não permitido")
			return
		}

		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			jsonErr(w, http.StatusUnauthorized, "Não autenticado")
			return
		}
		userID, _ := claims["user_id"].(string)

		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa: "+err.Error())
			return
		}

		args := []interface{}{companyID}
		whereClause := "WHERE ei.company_id = $1"
		paramIdx := 2

		mesAno := strings.TrimSpace(r.URL.Query().Get("mes_ano"))
		if mesAno != "" {
			whereClause += fmt.Sprintf(" AND ne.mes_ano = $%d", paramIdx)
			args = append(args, mesAno)
		}

		query := fmt.Sprintf(`
SELECT
    ne.forn_cnpj,
    ne.forn_nome,
    ei.ncm,
    COUNT(*) FILTER (WHERE ei.cclasstrib IS NULL) AS itens_sem_cclasstrib,
    COUNT(DISTINCT ei.cclasstrib) FILTER (WHERE ei.cclasstrib IS NOT NULL) AS variantes_cclasstrib,
    COALESCE(SUM(ei.v_pis + ei.v_cofins), 0) AS v_pis_cofins_total
FROM nfe_entradas_itens ei
JOIN nfe_entradas ne ON ne.id = ei.nfe_id
%s
GROUP BY ne.forn_cnpj, ne.forn_nome, ei.ncm
HAVING COUNT(*) FILTER (WHERE ei.cclasstrib IS NULL) > 0
    OR COUNT(DISTINCT ei.cclasstrib) FILTER (WHERE ei.cclasstrib IS NOT NULL) > 1
ORDER BY v_pis_cofins_total DESC
LIMIT 200`, whereClause)

		rows, err := db.Query(query, args...)
		if err != nil {
			log.Printf("[XMLReports/FornecedoresCCLASSTRIB] query error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar banco")
			return
		}
		defer rows.Close()

		type fornRow struct {
			FornCNPJ            string  `json:"forn_cnpj"`
			FornNome            string  `json:"forn_nome"`
			NCM                 string  `json:"ncm"`
			ItensSemCclasstrib  int     `json:"itens_sem_cclasstrib"`
			VariantesCclasstrib int     `json:"variantes_cclasstrib"`
			VPisCofinsTotal     float64 `json:"v_pis_cofins_total"`
		}

		var list []fornRow
		for rows.Next() {
			var row fornRow
			if err := rows.Scan(
				&row.FornCNPJ,
				&row.FornNome,
				&row.NCM,
				&row.ItensSemCclasstrib,
				&row.VariantesCclasstrib,
				&row.VPisCofinsTotal,
			); err != nil {
				log.Printf("[XMLReports/FornecedoresCCLASSTRIB] scan error: %v", err)
				continue
			}
			list = append(list, row)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[XMLReports/FornecedoresCCLASSTRIB] rows error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao processar resultado")
			return
		}

		if list == nil {
			list = []fornRow{}
		}

		if encErr := json.NewEncoder(w).Encode(list); encErr != nil {
			log.Printf("[XMLReports/FornecedoresCCLASSTRIB] encode error: %v", encErr)
		}
	}
}
