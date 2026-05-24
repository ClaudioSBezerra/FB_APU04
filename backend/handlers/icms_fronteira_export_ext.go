package handlers

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/xuri/excelize/v2"
)

// ---------------------------------------------------------------------------
// Helpers compartilhados
// ---------------------------------------------------------------------------

func exportAuth(db *sql.DB, w http.ResponseWriter, r *http.Request) (companyID string, ok bool) {
	if r.Method != http.MethodGet {
		jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
		return "", false
	}
	claims, valid := r.Context().Value(ClaimsKey).(jwt.MapClaims)
	if !valid {
		jsonErr(w, http.StatusUnauthorized, "Unauthorized")
		return "", false
	}
	userID, _ := claims["user_id"].(string)
	id, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa: "+err.Error())
		return "", false
	}
	return id, true
}

func xlsxHeaderStyle(f *excelize.File) int {
	s, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"4472C4"}},
	})
	return s
}

func xlsxBoldStyle(f *excelize.File) int {
	s, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	return s
}

// colLetter returns the Excel column letter(s) for a 0-based index.
func colLetter(i int) string {
	letters := []string{
		"A","B","C","D","E","F","G","H","I","J","K","L","M",
		"N","O","P","Q","R","S","T","U","V","W","X","Y","Z",
	}
	if i < 26 {
		return letters[i]
	}
	return letters[(i/26)-1] + letters[i%26]
}

// ---------------------------------------------------------------------------
// B — Export planilha itens
// ---------------------------------------------------------------------------

var itensCSVHeaders = []string{
	"Data Emissão", "NF-e", "CNPJ Forn.", "Fornecedor", "UF", "CFOP", "Regime",
	"N.Item", "Cód.Prod", "Descrição", "NCM", "CEST",
	"V.Prod", "V.IPI", "V.Outro", "V.Operação", "V.ICMS",
	"Alíq.Inter%", "Alíq.Int%", "BC", "MVA%", "BC-ST", "ICMS Calc.", "ICMS Ret.",
}

func fetchItensExport(db *sql.DB, companyID, regime, periodo string) ([]FronteiraItemRow, error) {
	rows, err := db.Query(fronteiraItensQueryBody, companyID, regime, periodo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []FronteiraItemRow
	for rows.Next() {
		var row FronteiraItemRow
		var mvaOrig sql.NullFloat64
		// G14: query inclui total_count e total_full via window functions;
		// exports descartam pois o CSV/XLSX é linha-a-linha sem agregado.
		var totalCount sql.NullInt64
		var totalFull sql.NullFloat64
		if err := rows.Scan(
			&row.ChaveNFe, &row.DataEmissao, &row.NumeroNFe,
			&row.FornCNPJ, &row.FornNome, &row.FornUF,
			&row.FornSimples, &row.CFOP, &row.Regime,
			&row.NItem, &row.CProd, &row.XProd, &row.NCM, &row.CEST,
			&row.VProdItem, &row.VIpiItem, &row.VOutroRateado, &row.VOperacao, &row.VIcmsItem,
			&row.AliqInter, &row.AliqInterna, &row.BC,
			&row.IcmsCalculado, &row.IcmsRetido,
			&mvaOrig, &row.BcSt,
			&totalCount, &totalFull,
		); err != nil {
			log.Printf("fetchItensExport scan: %v", err)
			continue
		}
		if mvaOrig.Valid {
			row.MvaOriginal = &mvaOrig.Float64
		}
		result = append(result, row)
	}
	return result, nil
}

func itenRowToCSV(row FronteiraItemRow) []string {
	d := row.DataEmissao
	if len(d) > 10 {
		d = d[:10]
	}
	mvaStr := ""
	if row.MvaOriginal != nil {
		mvaStr = fmt.Sprintf("%.2f", *row.MvaOriginal)
	}
	bcStStr := ""
	if row.BcSt > 0 {
		bcStStr = fmt.Sprintf("%.2f", row.BcSt)
	}
	return []string{
		d, row.NumeroNFe, row.FornCNPJ, row.FornNome, row.FornUF,
		row.CFOP, row.Regime, fmt.Sprintf("%d", row.NItem),
		row.CProd, row.XProd, row.NCM, row.CEST,
		fmt.Sprintf("%.2f", row.VProdItem),
		fmt.Sprintf("%.2f", row.VIpiItem),
		fmt.Sprintf("%.2f", row.VOutroRateado),
		fmt.Sprintf("%.2f", row.VOperacao),
		fmt.Sprintf("%.2f", row.VIcmsItem),
		fmt.Sprintf("%.2f", row.AliqInter),
		fmt.Sprintf("%.2f", row.AliqInterna),
		fmt.Sprintf("%.2f", row.BC),
		mvaStr,
		bcStStr,
		fmt.Sprintf("%.2f", row.IcmsCalculado),
		fmt.Sprintf("%.2f", row.IcmsRetido),
	}
}

// IcmsFronteiraExportItensCSVHandler — GET /api/icms-fronteira/itens/exportar/csv
func IcmsFronteiraExportItensCSVHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		companyID, ok := exportAuth(db, w, r)
		if !ok {
			return
		}
		regime := r.URL.Query().Get("regime")
		if regime == "" {
			regime = "todos"
		}
		periodo := r.URL.Query().Get("periodo")
		rows, err := fetchItensExport(db, companyID, regime, periodo)
		if err != nil {
			log.Printf("ExportItensCSV error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar dados")
			return
		}
		filename := fmt.Sprintf("icms-fronteira-itens-%s.csv", strings.ToLower(regime))
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		w.Write([]byte{0xEF, 0xBB, 0xBF})
		cw := csv.NewWriter(w)
		cw.Write(itensCSVHeaders)
		for _, row := range rows {
			cw.Write(itenRowToCSV(row))
		}
		cw.Flush()
	}
}

// IcmsFronteiraExportItensXLSXHandler — GET /api/icms-fronteira/itens/exportar/xlsx
func IcmsFronteiraExportItensXLSXHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		companyID, ok := exportAuth(db, w, r)
		if !ok {
			return
		}
		regime := r.URL.Query().Get("regime")
		if regime == "" {
			regime = "todos"
		}
		periodo := r.URL.Query().Get("periodo")
		dataRows, err := fetchItensExport(db, companyID, regime, periodo)
		if err != nil {
			log.Printf("ExportItensXLSX error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar dados")
			return
		}

		f := excelize.NewFile()
		sheet := "Planilha Itens"
		f.SetSheetName("Sheet1", sheet)
		hs := xlsxHeaderStyle(f)
		bs := xlsxBoldStyle(f)

		for i, h := range itensCSVHeaders {
			cell := fmt.Sprintf("%s1", colLetter(i))
			f.SetCellValue(sheet, cell, h)
			f.SetCellStyle(sheet, cell, cell, hs)
		}

		var totalCalc, totalRet float64
		for ri, row := range dataRows {
			exRow := ri + 2
			d := row.DataEmissao
			if len(d) > 10 {
				d = d[:10]
			}
			mvaVal := interface{}(nil)
			if row.MvaOriginal != nil {
				mvaVal = *row.MvaOriginal
			}
			bcStVal := interface{}(nil)
			if row.BcSt > 0 {
				bcStVal = row.BcSt
			}
			vals := []interface{}{
				d, row.NumeroNFe, row.FornCNPJ, row.FornNome, row.FornUF,
				row.CFOP, row.Regime, row.NItem,
				row.CProd, row.XProd, row.NCM, row.CEST,
				row.VProdItem, row.VIpiItem, row.VOutroRateado, row.VOperacao, row.VIcmsItem,
				row.AliqInter, row.AliqInterna, row.BC,
				mvaVal, bcStVal,
				row.IcmsCalculado, row.IcmsRetido,
			}
			for ci, v := range vals {
				if v != nil {
					f.SetCellValue(sheet, fmt.Sprintf("%s%d", colLetter(ci), exRow), v)
				}
			}
			totalCalc += row.IcmsCalculado
			totalRet += row.IcmsRetido
		}

		tr := len(dataRows) + 2
		f.SetCellValue(sheet, fmt.Sprintf("A%d", tr), "TOTAL")
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", tr), fmt.Sprintf("A%d", tr), bs)
		f.SetCellValue(sheet, fmt.Sprintf("%s%d", colLetter(22), tr), totalCalc)
		f.SetCellStyle(sheet, fmt.Sprintf("%s%d", colLetter(22), tr), fmt.Sprintf("%s%d", colLetter(22), tr), bs)
		f.SetCellValue(sheet, fmt.Sprintf("%s%d", colLetter(23), tr), totalRet)
		f.SetCellStyle(sheet, fmt.Sprintf("%s%d", colLetter(23), tr), fmt.Sprintf("%s%d", colLetter(23), tr), bs)

		var buf bytes.Buffer
		if err := f.Write(&buf); err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao gerar XLSX")
			return
		}
		filename := fmt.Sprintf("icms-fronteira-itens-%s.xlsx", strings.ToLower(regime))
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		w.Write(buf.Bytes())
	}
}

// ---------------------------------------------------------------------------
// C — Export divergências
// ---------------------------------------------------------------------------

var divCSVHeaders = []string{
	"Período", "NF", "CNPJ Forn.", "Fornecedor", "UF",
	"Data Emissão", "Regime", "ICMS SEFAZ", "ICMS Calculado", "Diferença", "Status",
}

func fetchDivExport(db *sql.DB, companyID, periodo string) ([]DivergenciaRow, error) {
	rows, err := db.Query(divergenciasQueryBody, companyID, periodo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []DivergenciaRow
	for rows.Next() {
		var row DivergenciaRow
		// G14: query inclui 4 window functions; exports descartam.
		var totalCount sql.NullInt64
		var totalSefazFull, totalCalcFull, totalDifFull sql.NullFloat64
		if err := rows.Scan(
			&row.ChaveNFe, &row.Periodo, &row.NumeroNF,
			&row.FornCNPJ, &row.FornNome, &row.FornUF,
			&row.DataEmissao, &row.Regime,
			&row.IcmsSefaz, &row.IcmsCalculado, &row.Diferenca, &row.Status,
			&totalCount, &totalSefazFull, &totalCalcFull, &totalDifFull,
		); err != nil {
			log.Printf("fetchDivExport scan: %v", err)
			continue
		}
		result = append(result, row)
	}
	return result, nil
}

func divRowToCSV(row DivergenciaRow) []string {
	d := row.DataEmissao
	if len(d) > 10 {
		d = d[:10]
	}
	return []string{
		row.Periodo, row.NumeroNF, row.FornCNPJ, row.FornNome, row.FornUF,
		d, row.Regime,
		fmt.Sprintf("%.2f", row.IcmsSefaz),
		fmt.Sprintf("%.2f", row.IcmsCalculado),
		fmt.Sprintf("%.2f", row.Diferenca),
		row.Status,
	}
}

// IcmsFronteiraExportDivCSVHandler — GET /api/icms-fronteira/divergencias/exportar/csv
func IcmsFronteiraExportDivCSVHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		companyID, ok := exportAuth(db, w, r)
		if !ok {
			return
		}
		periodo := r.URL.Query().Get("periodo")
		rows, err := fetchDivExport(db, companyID, periodo)
		if err != nil {
			log.Printf("ExportDivCSV error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar dados")
			return
		}
		filename := "icms-fronteira-divergencias.csv"
		if periodo != "" {
			filename = fmt.Sprintf("icms-fronteira-divergencias-%s.csv", strings.ReplaceAll(periodo, "/", "-"))
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		w.Write([]byte{0xEF, 0xBB, 0xBF})
		cw := csv.NewWriter(w)
		cw.Write(divCSVHeaders)
		for _, row := range rows {
			cw.Write(divRowToCSV(row))
		}
		cw.Flush()
	}
}

// IcmsFronteiraExportDivXLSXHandler — GET /api/icms-fronteira/divergencias/exportar/xlsx
func IcmsFronteiraExportDivXLSXHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		companyID, ok := exportAuth(db, w, r)
		if !ok {
			return
		}
		periodo := r.URL.Query().Get("periodo")
		dataRows, err := fetchDivExport(db, companyID, periodo)
		if err != nil {
			log.Printf("ExportDivXLSX error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar dados")
			return
		}

		f := excelize.NewFile()
		sheet := "Divergências"
		f.SetSheetName("Sheet1", sheet)
		hs := xlsxHeaderStyle(f)
		bs := xlsxBoldStyle(f)

		for i, h := range divCSVHeaders {
			cell := fmt.Sprintf("%s1", colLetter(i))
			f.SetCellValue(sheet, cell, h)
			f.SetCellStyle(sheet, cell, cell, hs)
		}

		var totalSefaz, totalCalc, totalDif float64
		for ri, row := range dataRows {
			exRow := ri + 2
			d := row.DataEmissao
			if len(d) > 10 {
				d = d[:10]
			}
			vals := []interface{}{
				row.Periodo, row.NumeroNF, row.FornCNPJ, row.FornNome, row.FornUF,
				d, row.Regime,
				row.IcmsSefaz, row.IcmsCalculado, row.Diferenca, row.Status,
			}
			for ci, v := range vals {
				f.SetCellValue(sheet, fmt.Sprintf("%s%d", colLetter(ci), exRow), v)
			}
			totalSefaz += row.IcmsSefaz
			totalCalc += row.IcmsCalculado
			totalDif += row.Diferenca
		}

		tr := len(dataRows) + 2
		f.SetCellValue(sheet, fmt.Sprintf("A%d", tr), "TOTAL")
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", tr), fmt.Sprintf("A%d", tr), bs)
		for ci, v := range []float64{totalSefaz, totalCalc, totalDif} {
			col := colLetter(7 + ci)
			f.SetCellValue(sheet, fmt.Sprintf("%s%d", col, tr), v)
			f.SetCellStyle(sheet, fmt.Sprintf("%s%d", col, tr), fmt.Sprintf("%s%d", col, tr), bs)
		}

		var buf bytes.Buffer
		if err := f.Write(&buf); err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao gerar XLSX")
			return
		}
		filename := "icms-fronteira-divergencias.xlsx"
		if periodo != "" {
			filename = fmt.Sprintf("icms-fronteira-divergencias-%s.xlsx", strings.ReplaceAll(periodo, "/", "-"))
		}
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		w.Write(buf.Bytes())
	}
}
