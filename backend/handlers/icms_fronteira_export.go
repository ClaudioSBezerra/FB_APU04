package handlers

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/xuri/excelize/v2"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// fronteiraExportSelectCols is the column list appended after fronteiraBaseQuery CTE.
const fronteiraExportSelectCols = `
SELECT
    data_emissao,
    numero_nfe,
    forn_nome,
    forn_cnpj,
    forn_uf,
    cfop,
    regime,
    v_prod,
    v_icms,
    v_bc_st,
    v_st,
    aliq_inter,
    aliq_interna,
    icms_devido_est
FROM classified
`

func buildExportQuery(regime string) (string, []interface{}) {
	if regime == "todos" || regime == "" {
		q := fronteiraBaseQuery + fronteiraExportSelectCols + `ORDER BY regime, data_emissao DESC LIMIT 2000`
		return q, nil
	}
	q := fronteiraBaseQuery + fronteiraExportSelectCols + `WHERE regime = $2 ORDER BY data_emissao DESC LIMIT 2000`
	return q, []interface{}{strings.ToUpper(regime)}
}

type fronteiraExportRow struct {
	DataEmissao   string
	NumeroNFe     string
	FornNome      string
	FornCNPJ      string
	FornUF        string
	CFOP          string
	Regime        string
	VProd         float64
	VIcms         float64
	VBcST         float64
	VST           float64
	AliqInter     float64
	AliqInterna   float64
	IcmsDevidoEst float64
}

func fetchExportRows(db *sql.DB, companyID, regime string) ([]fronteiraExportRow, error) {
	baseQuery, extraArgs := buildExportQuery(regime)

	var args []interface{}
	args = append(args, companyID)
	args = append(args, extraArgs...)

	rows, err := db.Query(baseQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []fronteiraExportRow
	for rows.Next() {
		var row fronteiraExportRow
		if err := rows.Scan(
			&row.DataEmissao,
			&row.NumeroNFe,
			&row.FornNome,
			&row.FornCNPJ,
			&row.FornUF,
			&row.CFOP,
			&row.Regime,
			&row.VProd,
			&row.VIcms,
			&row.VBcST,
			&row.VST,
			&row.AliqInter,
			&row.AliqInterna,
			&row.IcmsDevidoEst,
		); err != nil {
			log.Printf("fronteiraExport scan error: %v", err)
			continue
		}
		result = append(result, row)
	}
	return result, nil
}

var exportCSVHeaders = []string{
	"Data Emissão", "Número NF-e", "Fornecedor", "CNPJ", "UF", "CFOP", "Regime",
	"V.Prod", "ICMS Atual", "V.BC ST", "V.ST", "Alíq.Inter.%", "Alíq.Interna.%", "ICMS Devido Est.",
}

func rowToCSVRecord(row fronteiraExportRow) []string {
	return []string{
		row.DataEmissao,
		row.NumeroNFe,
		row.FornNome,
		row.FornCNPJ,
		row.FornUF,
		row.CFOP,
		row.Regime,
		fmt.Sprintf("%.2f", row.VProd),
		fmt.Sprintf("%.2f", row.VIcms),
		fmt.Sprintf("%.2f", row.VBcST),
		fmt.Sprintf("%.2f", row.VST),
		fmt.Sprintf("%.2f", row.AliqInter),
		fmt.Sprintf("%.2f", row.AliqInterna),
		fmt.Sprintf("%.2f", row.IcmsDevidoEst),
	}
}

// ---------------------------------------------------------------------------
// IcmsFronteiraExportCSVHandler — GET /api/icms-fronteira/exportar/csv
// ---------------------------------------------------------------------------

func IcmsFronteiraExportCSVHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		regime := strings.ToLower(r.URL.Query().Get("regime"))
		if regime == "" {
			regime = "todos"
		}

		rows, err := fetchExportRows(db, companyID, regime)
		if err != nil {
			log.Printf("IcmsFronteiraExportCSV error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar dados")
			return
		}

		filename := fmt.Sprintf("icms-fronteira-%s.csv", regime)
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

		// BOM for Excel UTF-8 compatibility
		w.Write([]byte{0xEF, 0xBB, 0xBF})

		cw := csv.NewWriter(w)
		if err := cw.Write(exportCSVHeaders); err != nil {
			log.Printf("IcmsFronteiraExportCSV header write error: %v", err)
			return
		}
		for _, row := range rows {
			if err := cw.Write(rowToCSVRecord(row)); err != nil {
				log.Printf("IcmsFronteiraExportCSV row write error: %v", err)
				return
			}
		}
		cw.Flush()
		if err := cw.Error(); err != nil {
			log.Printf("IcmsFronteiraExportCSV flush error: %v", err)
		}
	}
}

// ---------------------------------------------------------------------------
// IcmsFronteiraExportXLSXHandler — GET /api/icms-fronteira/exportar/xlsx
// ---------------------------------------------------------------------------

func IcmsFronteiraExportXLSXHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		regime := strings.ToLower(r.URL.Query().Get("regime"))
		if regime == "" {
			regime = "todos"
		}

		dataRows, err := fetchExportRows(db, companyID, regime)
		if err != nil {
			log.Printf("IcmsFronteiraExportXLSX error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar dados")
			return
		}

		f := excelize.NewFile()
		sheetName := "ICMS Fronteira"
		f.SetSheetName("Sheet1", sheetName)

		// Header style: bold, blue bg, white font
		headerStyle, _ := f.NewStyle(&excelize.Style{
			Font: &excelize.Font{
				Bold:  true,
				Color: "FFFFFF",
			},
			Fill: excelize.Fill{
				Type:    "pattern",
				Pattern: 1,
				Color:   []string{"4472C4"},
			},
		})

		// Write header row
		cols := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N"}
		for i, h := range exportCSVHeaders {
			cell := fmt.Sprintf("%s1", cols[i])
			f.SetCellValue(sheetName, cell, h)
			f.SetCellStyle(sheetName, cell, cell, headerStyle)
		}

		// Write data rows
		var totalVProd, totalVIcms, totalVBcST, totalVST, totalIcmsDevido float64
		for rowIdx, row := range dataRows {
			excelRow := rowIdx + 2
			f.SetCellValue(sheetName, fmt.Sprintf("A%d", excelRow), row.DataEmissao)
			f.SetCellValue(sheetName, fmt.Sprintf("B%d", excelRow), row.NumeroNFe)
			f.SetCellValue(sheetName, fmt.Sprintf("C%d", excelRow), row.FornNome)
			f.SetCellValue(sheetName, fmt.Sprintf("D%d", excelRow), row.FornCNPJ)
			f.SetCellValue(sheetName, fmt.Sprintf("E%d", excelRow), row.FornUF)
			f.SetCellValue(sheetName, fmt.Sprintf("F%d", excelRow), row.CFOP)
			f.SetCellValue(sheetName, fmt.Sprintf("G%d", excelRow), row.Regime)
			f.SetCellValue(sheetName, fmt.Sprintf("H%d", excelRow), row.VProd)
			f.SetCellValue(sheetName, fmt.Sprintf("I%d", excelRow), row.VIcms)
			f.SetCellValue(sheetName, fmt.Sprintf("J%d", excelRow), row.VBcST)
			f.SetCellValue(sheetName, fmt.Sprintf("K%d", excelRow), row.VST)
			f.SetCellValue(sheetName, fmt.Sprintf("L%d", excelRow), row.AliqInter)
			f.SetCellValue(sheetName, fmt.Sprintf("M%d", excelRow), row.AliqInterna)
			f.SetCellValue(sheetName, fmt.Sprintf("N%d", excelRow), row.IcmsDevidoEst)

			totalVProd += row.VProd
			totalVIcms += row.VIcms
			totalVBcST += row.VBcST
			totalVST += row.VST
			totalIcmsDevido += row.IcmsDevidoEst
		}

		// Totals row (bold)
		boldStyle, _ := f.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true},
		})
		totalRow := len(dataRows) + 2
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", totalRow), "TOTAL")
		f.SetCellStyle(sheetName, fmt.Sprintf("A%d", totalRow), fmt.Sprintf("A%d", totalRow), boldStyle)
		f.SetCellValue(sheetName, fmt.Sprintf("H%d", totalRow), totalVProd)
		f.SetCellStyle(sheetName, fmt.Sprintf("H%d", totalRow), fmt.Sprintf("H%d", totalRow), boldStyle)
		f.SetCellValue(sheetName, fmt.Sprintf("I%d", totalRow), totalVIcms)
		f.SetCellStyle(sheetName, fmt.Sprintf("I%d", totalRow), fmt.Sprintf("I%d", totalRow), boldStyle)
		f.SetCellValue(sheetName, fmt.Sprintf("J%d", totalRow), totalVBcST)
		f.SetCellStyle(sheetName, fmt.Sprintf("J%d", totalRow), fmt.Sprintf("J%d", totalRow), boldStyle)
		f.SetCellValue(sheetName, fmt.Sprintf("K%d", totalRow), totalVST)
		f.SetCellStyle(sheetName, fmt.Sprintf("K%d", totalRow), fmt.Sprintf("K%d", totalRow), boldStyle)
		f.SetCellValue(sheetName, fmt.Sprintf("N%d", totalRow), totalIcmsDevido)
		f.SetCellStyle(sheetName, fmt.Sprintf("N%d", totalRow), fmt.Sprintf("N%d", totalRow), boldStyle)

		var buf bytes.Buffer
		if err := f.Write(&buf); err != nil {
			log.Printf("IcmsFronteiraExportXLSX write error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao gerar arquivo XLSX")
			return
		}

		filename := fmt.Sprintf("icms-fronteira-%s.xlsx", regime)
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		w.Write(buf.Bytes())
	}
}

// ---------------------------------------------------------------------------
// IcmsFronteiraExportHTMLHandler — GET /api/icms-fronteira/exportar/pdf
// Returns printable HTML (browser triggers window.print())
// ---------------------------------------------------------------------------

func IcmsFronteiraExportHTMLHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		regime := strings.ToLower(r.URL.Query().Get("regime"))
		if regime == "" {
			regime = "todos"
		}

		// Fetch company name
		var companyName string
		err = db.QueryRow(`SELECT COALESCE(nome_fantasia, razao_social, '') FROM companies WHERE id = $1::uuid`, companyID).Scan(&companyName)
		if err != nil {
			companyName = companyID
		}

		dataRows, err := fetchExportRows(db, companyID, regime)
		if err != nil {
			log.Printf("IcmsFronteiraExportHTML error: %v", err)
			http.Error(w, "Erro ao consultar dados", http.StatusInternalServerError)
			return
		}

		regimeLabel := strings.ToUpper(regime)
		if regimeLabel == "TODOS" {
			regimeLabel = "Todos os Regimes"
		}
		today := time.Now().Format("02/01/2006")
		title := fmt.Sprintf("ICMS Fronteira — %s — %s", regimeLabel, today)

		var sb strings.Builder
		sb.WriteString(`<!DOCTYPE html><html lang="pt-BR"><head><meta charset="UTF-8">`)
		sb.WriteString(fmt.Sprintf(`<title>%s</title>`, title))
		sb.WriteString(`<style>
body { font-family: Arial, sans-serif; font-size: 11px; margin: 20px; }
h1 { font-size: 14px; margin-bottom: 4px; }
h2 { font-size: 12px; color: #555; margin-bottom: 12px; }
table { border-collapse: collapse; width: 100%; }
th { background: #4472C4; color: #fff; padding: 4px 6px; text-align: left; font-size: 10px; }
td { border: 1px solid #ccc; padding: 3px 6px; }
tr:nth-child(even) td { background: #f0f4ff; }
.total-row td { font-weight: bold; background: #d9e1f2; }
@media print {
  @page { size: landscape; margin: 10mm; }
  body { margin: 0; }
}
</style></head><body>`)

		sb.WriteString(fmt.Sprintf(`<h1>%s</h1>`, title))
		sb.WriteString(fmt.Sprintf(`<h2>Empresa: %s</h2>`, companyName))

		sb.WriteString(`<table><thead><tr>`)
		for _, h := range exportCSVHeaders {
			sb.WriteString(fmt.Sprintf(`<th>%s</th>`, h))
		}
		sb.WriteString(`</tr></thead><tbody>`)

		var totalVProd, totalVIcms, totalVBcST, totalVST, totalIcmsDevido float64
		for _, row := range dataRows {
			sb.WriteString(`<tr>`)
			sb.WriteString(fmt.Sprintf(`<td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td>`,
				row.DataEmissao, row.NumeroNFe, htmlEscape(row.FornNome), row.FornCNPJ, row.FornUF, row.CFOP, row.Regime))
			sb.WriteString(fmt.Sprintf(`<td>%.2f</td><td>%.2f</td><td>%.2f</td><td>%.2f</td><td>%.2f</td><td>%.2f</td><td>%.2f</td>`,
				row.VProd, row.VIcms, row.VBcST, row.VST, row.AliqInter, row.AliqInterna, row.IcmsDevidoEst))
			sb.WriteString(`</tr>`)
			totalVProd += row.VProd
			totalVIcms += row.VIcms
			totalVBcST += row.VBcST
			totalVST += row.VST
			totalIcmsDevido += row.IcmsDevidoEst
		}

		// Totals row
		sb.WriteString(`<tr class="total-row"><td colspan="7"><strong>TOTAL</strong></td>`)
		sb.WriteString(fmt.Sprintf(`<td>%.2f</td><td>%.2f</td><td>%.2f</td><td>%.2f</td><td></td><td></td><td>%.2f</td>`,
			totalVProd, totalVIcms, totalVBcST, totalVST, totalIcmsDevido))
		sb.WriteString(`</tr>`)

		sb.WriteString(`</tbody></table>`)
		sb.WriteString(`<script>window.onload=function(){window.print()}</script>`)
		sb.WriteString(`</body></html>`)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(sb.String()))
	}
}

// htmlEscape escapes basic HTML entities in strings.
func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&#34;")
	return s
}
