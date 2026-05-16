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
// xml_conciliacao.go — Conciliação Bridge vs XML
//
// GET /api/xml/conciliacao?mes_ano=MM/YYYY&tipo=entradas|saidas → ConciliacaoHandler
// GET /api/xml/cobertura?mes_ano=MM/YYYY&tipo=entradas|saidas   → CoberturaHandler
// GET /api/xml/conciliacao/csv                                   → ConciliacaoCSVHandler
//
// Todos os handlers:
//   - Usam GetEffectiveCompanyID (company_id do usuário autenticado)
//   - Usam parâmetros $N nas queries (nunca concatenação de strings com input do usuário)
//   - Não setam CORS headers (tratados pelo SecurityMiddleware em main.go)
// ---------------------------------------------------------------------------

// conciliacaoRow representa uma NF-e com divergência entre valores Bridge e XML.
type conciliacaoRow struct {
	ChaveNfe    string  `json:"chave_nfe"`
	FornCNPJ    string  `json:"forn_cnpj"`
	FornNome    string  `json:"forn_nome"`
	MesAno      string  `json:"mes_ano"`
	DataEmissao string  `json:"data_emissao"`
	CFOP        string  `json:"cfop"`
	// Valores XML (SEFAZ autêntico)
	XmlPis    float64 `json:"xml_pis"`
	XmlCofins float64 `json:"xml_cofins"`
	XmlIcms   float64 `json:"xml_icms"`
	XmlIpi    float64 `json:"xml_ipi"`
	XmlVNf    float64 `json:"xml_v_nf"`
	// Valores Bridge (legado do ERP)
	BridgePis    float64 `json:"bridge_pis"`
	BridgeCofins float64 `json:"bridge_cofins"`
	BridgeIcms   float64 `json:"bridge_icms"`
	BridgeIpi    float64 `json:"bridge_ipi"`
	// Deltas absolutos
	DeltaPis    float64 `json:"delta_pis"`
	DeltaCofins float64 `json:"delta_cofins"`
	DeltaIcms   float64 `json:"delta_icms"`
	DeltaIpi    float64 `json:"delta_ipi"`
	DeltaTotal  float64 `json:"delta_total"`
}

// coberturaRow representa agregação de cobertura XML por mês.
type coberturaRow struct {
	MesAno    string  `json:"mes_ano"`
	TotalNfes int     `json:"total_nfes"`
	ComXml    int     `json:"com_xml"`
	SoBridge  int     `json:"so_bridge"`
	PctXml    float64 `json:"pct_xml"`
}

// executeConciliacaoQuery executa a query de divergências e retorna os dados.
// tabela deve ser "nfe_entradas" ou "nfe_saidas" — validado pelo chamador via whitelist.
// mesAno é passado como parâmetro $2 (nunca interpolado na string SQL).
func executeConciliacaoQuery(db *sql.DB, companyID, mesAno, tabela string) ([]conciliacaoRow, error) {
	args := []interface{}{companyID}
	whereExtra := ""
	paramIdx := 2

	if mesAno != "" {
		whereExtra = fmt.Sprintf(" AND ne.mes_ano = $%d", paramIdx)
		args = append(args, mesAno)
		paramIdx++
	}
	// paramIdx referenced to avoid unused variable warning in future expansions
	_ = paramIdx

	// tabela já validada pelo chamador como "nfe_entradas" ou "nfe_saidas"
	query := fmt.Sprintf(`
SELECT
    ne.chave_nfe,
    COALESCE(ne.forn_cnpj, '')                      AS forn_cnpj,
    COALESCE(NULLIF(ne.forn_nome, ''), '')           AS forn_nome,
    ne.mes_ano,
    TO_CHAR(ne.data_emissao, 'DD/MM/YYYY')           AS data_emissao,
    COALESCE(ne.cfop, '')                            AS cfop,
    COALESCE(ne.v_pis, 0)    AS xml_pis,
    COALESCE(ne.v_cofins, 0) AS xml_cofins,
    COALESCE(ne.v_icms, 0)   AS xml_icms,
    COALESCE(ne.v_ipi, 0)    AS xml_ipi,
    COALESCE(ne.v_nf, 0)     AS xml_v_nf,
    COALESCE(ne.pis, 0)      AS bridge_pis,
    COALESCE(ne.cofins, 0)   AS bridge_cofins,
    COALESCE(ne.icms, 0)     AS bridge_icms,
    COALESCE(ne.ipi, 0)      AS bridge_ipi,
    ROUND(ABS(COALESCE(ne.v_pis,0)    - COALESCE(ne.pis,0)),    2) AS delta_pis,
    ROUND(ABS(COALESCE(ne.v_cofins,0) - COALESCE(ne.cofins,0)), 2) AS delta_cofins,
    ROUND(ABS(COALESCE(ne.v_icms,0)   - COALESCE(ne.icms,0)),   2) AS delta_icms,
    ROUND(ABS(COALESCE(ne.v_ipi,0)    - COALESCE(ne.ipi,0)),    2) AS delta_ipi,
    ROUND(
        ABS(COALESCE(ne.v_pis,0) - COALESCE(ne.pis,0)) +
        ABS(COALESCE(ne.v_cofins,0) - COALESCE(ne.cofins,0)) +
        ABS(COALESCE(ne.v_icms,0) - COALESCE(ne.icms,0)),
    2) AS delta_total
FROM %s ne
WHERE ne.company_id = $1
  AND ne.source = 'xml_upload'
  AND ne.cancelado != 'S'
  AND (COALESCE(ne.pis,0) + COALESCE(ne.cofins,0) + COALESCE(ne.icms,0)) > 0
  AND (ABS(COALESCE(ne.v_pis,0)    - COALESCE(ne.pis,0))    > 0.01
    OR ABS(COALESCE(ne.v_cofins,0) - COALESCE(ne.cofins,0)) > 0.01
    OR ABS(COALESCE(ne.v_icms,0)   - COALESCE(ne.icms,0))   > 0.01)
  %s
ORDER BY delta_total DESC
LIMIT 500`, tabela, whereExtra)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []conciliacaoRow
	for rows.Next() {
		var row conciliacaoRow
		if err := rows.Scan(
			&row.ChaveNfe, &row.FornCNPJ, &row.FornNome, &row.MesAno, &row.DataEmissao, &row.CFOP,
			&row.XmlPis, &row.XmlCofins, &row.XmlIcms, &row.XmlIpi, &row.XmlVNf,
			&row.BridgePis, &row.BridgeCofins, &row.BridgeIcms, &row.BridgeIpi,
			&row.DeltaPis, &row.DeltaCofins, &row.DeltaIcms, &row.DeltaIpi, &row.DeltaTotal,
		); err != nil {
			log.Printf("[Conciliacao] scan error: %v", err)
			continue
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if result == nil {
		result = []conciliacaoRow{}
	}
	return result, nil
}

// executeCoberturaQuery executa a query de cobertura XML por mês e retorna os dados.
// tabela deve ser "nfe_entradas" ou "nfe_saidas" — validado pelo chamador via whitelist.
func executeCoberturaQuery(db *sql.DB, companyID, tabela string) ([]coberturaRow, error) {
	query := fmt.Sprintf(`
SELECT
    ne.mes_ano,
    COUNT(*)                                               AS total_nfes,
    COUNT(*) FILTER (WHERE ne.source = 'xml_upload')      AS com_xml,
    COUNT(*) FILTER (WHERE ne.source = 'oracle_bridge')   AS so_bridge,
    ROUND(
        COUNT(*) FILTER (WHERE ne.source = 'xml_upload')::numeric
        / NULLIF(COUNT(*), 0) * 100,
    1) AS pct_xml
FROM %s ne
WHERE ne.company_id = $1
  AND ne.cancelado != 'S'
GROUP BY ne.mes_ano
ORDER BY ne.mes_ano DESC
LIMIT 24`, tabela)

	rows, err := db.Query(query, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []coberturaRow
	for rows.Next() {
		var row coberturaRow
		if err := rows.Scan(
			&row.MesAno, &row.TotalNfes, &row.ComXml, &row.SoBridge, &row.PctXml,
		); err != nil {
			log.Printf("[Cobertura] scan error: %v", err)
			continue
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if result == nil {
		result = []coberturaRow{}
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// ConciliacaoHandler — GET /api/xml/conciliacao
// Retorna JSON com NF-es que têm divergência entre valores Bridge e XML.
// Parâmetros: mes_ano (opcional), tipo (entradas|saidas, padrão entradas).
// ---------------------------------------------------------------------------
func ConciliacaoHandler(db *sql.DB) http.HandlerFunc {
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
		tipo := strings.TrimSpace(r.URL.Query().Get("tipo"))
		if tipo == "" {
			tipo = "entradas"
		}
		// whitelist de tipo para evitar injeção SQL em nome de tabela
		tabela := "nfe_entradas"
		if tipo == "saidas" {
			tabela = "nfe_saidas"
		}

		data, err := executeConciliacaoQuery(db, companyID, mesAno, tabela)
		if err != nil {
			log.Printf("[Conciliacao] query error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar banco")
			return
		}

		if data == nil {
			data = []conciliacaoRow{}
		}

		if encErr := json.NewEncoder(w).Encode(data); encErr != nil {
			log.Printf("[Conciliacao] encode error: %v", encErr)
		}
	}
}

// ---------------------------------------------------------------------------
// CoberturaHandler — GET /api/xml/cobertura
// Retorna JSON com agregação de cobertura XML por mês (total, com XML, só Bridge, %).
// Parâmetros: tipo (entradas|saidas, padrão entradas).
// ---------------------------------------------------------------------------
func CoberturaHandler(db *sql.DB) http.HandlerFunc {
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

		tipo := strings.TrimSpace(r.URL.Query().Get("tipo"))
		if tipo == "" {
			tipo = "entradas"
		}
		// whitelist de tipo para evitar injeção SQL em nome de tabela
		tabela := "nfe_entradas"
		if tipo == "saidas" {
			tabela = "nfe_saidas"
		}

		data, err := executeCoberturaQuery(db, companyID, tabela)
		if err != nil {
			log.Printf("[Cobertura] query error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar banco")
			return
		}

		if data == nil {
			data = []coberturaRow{}
		}

		if encErr := json.NewEncoder(w).Encode(data); encErr != nil {
			log.Printf("[Cobertura] encode error: %v", encErr)
		}
	}
}

// ---------------------------------------------------------------------------
// ConciliacaoCSVHandler — GET /api/xml/conciliacao/csv
// Exporta as divergências de conciliação como CSV para download pelo auditor.
// Cabeçalho em PT-BR. Parâmetros: mes_ano (opcional), tipo (entradas|saidas).
// ---------------------------------------------------------------------------
func ConciliacaoCSVHandler(db *sql.DB) http.HandlerFunc {
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
		tipo := strings.TrimSpace(r.URL.Query().Get("tipo"))
		if tipo == "" {
			tipo = "entradas"
		}
		// whitelist de tipo para evitar injeção SQL em nome de tabela
		tabela := "nfe_entradas"
		if tipo == "saidas" {
			tabela = "nfe_saidas"
		}

		data, err := executeConciliacaoQuery(db, companyID, mesAno, tabela)
		if err != nil {
			log.Printf("[Conciliacao/CSV] query error: %v", err)
			http.Error(w, "Erro ao consultar banco", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="conciliacao-bridge-xml.csv"`)

		cw := csv.NewWriter(w)

		header := []string{
			"Chave NF-e", "CNPJ Fornecedor", "Fornecedor", "Mês/Ano", "Data Emissão", "CFOP",
			"PIS XML", "PIS Bridge", "Delta PIS",
			"COFINS XML", "COFINS Bridge", "Delta COFINS",
			"ICMS XML", "ICMS Bridge", "Delta ICMS",
			"IPI XML", "IPI Bridge", "Delta IPI",
			"Delta Total",
		}
		if err := cw.Write(header); err != nil {
			log.Printf("[Conciliacao/CSV] write header error: %v", err)
			return
		}

		for _, row := range data {
			record := []string{
				row.ChaveNfe, row.FornCNPJ, row.FornNome, row.MesAno, row.DataEmissao, row.CFOP,
				fmt.Sprintf("%.2f", row.XmlPis), fmt.Sprintf("%.2f", row.BridgePis), fmt.Sprintf("%.2f", row.DeltaPis),
				fmt.Sprintf("%.2f", row.XmlCofins), fmt.Sprintf("%.2f", row.BridgeCofins), fmt.Sprintf("%.2f", row.DeltaCofins),
				fmt.Sprintf("%.2f", row.XmlIcms), fmt.Sprintf("%.2f", row.BridgeIcms), fmt.Sprintf("%.2f", row.DeltaIcms),
				fmt.Sprintf("%.2f", row.XmlIpi), fmt.Sprintf("%.2f", row.BridgeIpi), fmt.Sprintf("%.2f", row.DeltaIpi),
				fmt.Sprintf("%.2f", row.DeltaTotal),
			}
			if err := cw.Write(record); err != nil {
				log.Printf("[Conciliacao/CSV] write row error: %v", err)
				return
			}
		}

		cw.Flush()
		if err := cw.Error(); err != nil {
			log.Printf("[Conciliacao/CSV] flush error: %v", err)
		}
	}
}
