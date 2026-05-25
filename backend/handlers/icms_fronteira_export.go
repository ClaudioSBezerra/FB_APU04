package handlers

import (
	"bytes"
	"database/sql"
	"encoding/base64"
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
    chave_nfe,
    data_emissao,
    numero_nfe,
    forn_nome,
    forn_cnpj,
    forn_uf,
    cfop,
    regime,
    bloco,
    v_prod,
    v_icms,
    v_bc_st,
    v_st,
    aliq_inter,
    aliq_interna,
    icms_devido_est,
    v_ipi
FROM classified
`

// buildExportQuery monta a query do export. filtroSQL/filtroArgs vêm de
// fronteiraFiltros (fornecedor/num_nota/data) — os placeholders já devem estar
// numerados a partir do índice correto pelo chamador.
func buildExportQuery(regime, periodo, filtroSQL string, filtroArgs []interface{}) (string, []interface{}) {
	if regime == "todos" || regime == "" {
		where := ""
		if filtroSQL != "" {
			where = " WHERE 1=1" + filtroSQL
		}
		q := fronteiraBaseQuery + fronteiraExportSelectCols + where + ` ORDER BY regime, bloco, data_emissao DESC LIMIT 2000`
		return q, filtroArgs
	}
	q := fronteiraBaseQuery + fronteiraExportSelectCols + `WHERE regime = $3` + filtroSQL + ` ORDER BY bloco, data_emissao DESC LIMIT 2000`
	return q, append([]interface{}{strings.ToUpper(regime)}, filtroArgs...)
}

type fronteiraExportRow struct {
	ChaveNFe      string
	DataEmissao   string
	NumeroNFe     string
	FornNome      string
	FornCNPJ      string
	FornUF        string
	CFOP          string
	Regime        string
	Bloco         string // "mes_atual" | "mes_anterior"
	VProd         float64
	VIcms         float64
	VBcST         float64
	VST           float64
	AliqInter     float64
	AliqInterna   float64
	IcmsDevidoEst float64
	VIPI          float64
}

func fetchExportRows(db *sql.DB, companyID, regime, periodo string, r *http.Request) ([]fronteiraExportRow, error) {
	// Filtros começam após company_id($1), periodo($2) e, quando há regime
	// específico, o regime($3). Para "todos" não há $3.
	startIdx := 3
	if !(regime == "todos" || regime == "") {
		startIdx = 4
	}
	filtroSQL, filtroArgs := fronteiraFiltros(r, startIdx)
	baseQuery, extraArgs := buildExportQuery(regime, periodo, filtroSQL, filtroArgs)

	var args []interface{}
	args = append(args, companyID, periodo)
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
			&row.ChaveNFe,
			&row.DataEmissao,
			&row.NumeroNFe,
			&row.FornNome,
			&row.FornCNPJ,
			&row.FornUF,
			&row.CFOP,
			&row.Regime,
			&row.Bloco,
			&row.VProd,
			&row.VIcms,
			&row.VBcST,
			&row.VST,
			&row.AliqInter,
			&row.AliqInterna,
			&row.IcmsDevidoEst,
			&row.VIPI,
		); err != nil {
			log.Printf("fronteiraExport scan error: %v", err)
			continue
		}
		result = append(result, row)
	}
	return result, nil
}

var exportCSVHeaders = []string{
	"Bloco", "Data Emissão", "Número NF-e", "Fornecedor", "CNPJ", "UF", "CFOP", "Regime",
	"V.Prod", "V.IPI", "ICMS Atual", "V.BC ST", "V.ST", "Alíq.Inter.%", "Alíq.Interna.%", "ICMS Devido Est.",
	"Chave NF-e", "Chave CT-e",
}

func blocoLabel(bloco string) string {
	if bloco == "mes_anterior" {
		return "A - Mês Anterior"
	}
	return "B - Mês Atual"
}

func rowToCSVRecord(row fronteiraExportRow) []string {
	return []string{
		blocoLabel(row.Bloco),
		row.DataEmissao,
		row.NumeroNFe,
		row.FornNome,
		row.FornCNPJ,
		row.FornUF,
		row.CFOP,
		row.Regime,
		fmt.Sprintf("%.2f", row.VProd),
		fmt.Sprintf("%.2f", row.VIPI),
		fmt.Sprintf("%.2f", row.VIcms),
		fmt.Sprintf("%.2f", row.VBcST),
		fmt.Sprintf("%.2f", row.VST),
		fmt.Sprintf("%.2f", row.AliqInter),
		fmt.Sprintf("%.2f", row.AliqInterna),
		fmt.Sprintf("%.2f", row.IcmsDevidoEst),
		row.ChaveNFe,
		"",
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
		periodo := r.URL.Query().Get("periodo")

		rows, err := fetchExportRows(db, companyID, regime, periodo, r)
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
		periodo := r.URL.Query().Get("periodo")

		dataRows, err := fetchExportRows(db, companyID, regime, periodo, r)
		if err != nil {
			log.Printf("IcmsFronteiraExportXLSX error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar dados")
			return
		}

		f := excelize.NewFile()

		headerStyle, _ := f.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
			Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"4472C4"}},
		})
		// Formato monetário BR — "R$ #,##0.00" — e número 2 casas para alíquotas
		moneyFmt := `"R$" #,##0.00`
		numFmt   := `#,##0.00`

		boldStyle, _      := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
		moneyBoldStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}, CustomNumFmt: &moneyFmt})
		moneyStyle, _     := f.NewStyle(&excelize.Style{CustomNumFmt: &moneyFmt})
		numStyle, _       := f.NewStyle(&excelize.Style{CustomNumFmt: &numFmt})

		warnStyle, _      := f.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"FFF3CD"}}})
		moneyWarnStyle, _ := f.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"FFF3CD"}}, CustomNumFmt: &moneyFmt})
		numWarnStyle, _   := f.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"FFF3CD"}}, CustomNumFmt: &numFmt})

		// CT-e rows: fundo verde-claro para distinguir da NF
		cteStyle, _      := f.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"E2EFDA"}}, Font: &excelize.Font{Italic: true}})
		moneyCteStyle, _ := f.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"E2EFDA"}}, Font: &excelize.Font{Italic: true}, CustomNumFmt: &moneyFmt})
		numCteStyle, _   := f.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"E2EFDA"}}, Font: &excelize.Font{Italic: true}, CustomNumFmt: &numFmt})

		// exportCSVHeaders[0] = "Bloco", drop it (already separated by sheet)
		sheetHeaders := exportCSVHeaders[1:]
		cols := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q"}

		// Monta mapa de parâmetros das NFs para cálculo do frete
		nfParams := make(map[string]nfFreteParams)
		for _, row := range dataRows {
			if row.ChaveNFe != "" && row.Regime != "" {
				nfParams[row.ChaveNFe] = nfFreteParams{
					Regime:      row.Regime,
					AliqInter:   row.AliqInter,
					AliqInterna: row.AliqInterna,
				}
			}
		}
		freteLinks := fetchFreteLinks(db, companyID, periodo, nfParams)

		// ── Sheets B e A (SPED) ──────────────────────────────────────────────
		type sheetDef struct{ key, name string; warn bool }
		sheets := []sheetDef{
			{"mes_atual", "B - Mês Atual", false},
			{"mes_anterior", "A - Mês Anterior", true},
		}

		firstSheet := true
		for _, sd := range sheets {
			var sheetRows []fronteiraExportRow
			for _, row := range dataRows {
				if row.Bloco == sd.key || (sd.key == "mes_atual" && row.Bloco == "") {
					sheetRows = append(sheetRows, row)
				}
			}

			var sheetName string
			if firstSheet {
				f.SetSheetName("Sheet1", sd.name)
				sheetName = sd.name
				firstSheet = false
			} else {
				f.NewSheet(sd.name)
				sheetName = sd.name
			}

			for i, h := range sheetHeaders {
				cell := fmt.Sprintf("%s1", cols[i])
				f.SetCellValue(sheetName, cell, h)
				f.SetCellStyle(sheetName, cell, cell, headerStyle)
			}

			var totalVProd, totalVIcms, totalVBcST, totalVST, totalIcmsDevido, totalVIPI float64
			excelRow := 2
			for _, row := range sheetRows {
				// ── linha da NF ──────────────────────────────────────────────
				f.SetCellValue(sheetName, fmt.Sprintf("A%d", excelRow), row.DataEmissao)
				f.SetCellValue(sheetName, fmt.Sprintf("B%d", excelRow), row.NumeroNFe)
				f.SetCellValue(sheetName, fmt.Sprintf("C%d", excelRow), row.FornNome)
				f.SetCellValue(sheetName, fmt.Sprintf("D%d", excelRow), row.FornCNPJ)
				f.SetCellValue(sheetName, fmt.Sprintf("E%d", excelRow), row.FornUF)
				f.SetCellValue(sheetName, fmt.Sprintf("F%d", excelRow), row.CFOP)
				f.SetCellValue(sheetName, fmt.Sprintf("G%d", excelRow), row.Regime)
				f.SetCellValue(sheetName, fmt.Sprintf("H%d", excelRow), row.VProd)
				f.SetCellValue(sheetName, fmt.Sprintf("I%d", excelRow), row.VIPI)
				f.SetCellValue(sheetName, fmt.Sprintf("J%d", excelRow), row.VIcms)
				f.SetCellValue(sheetName, fmt.Sprintf("K%d", excelRow), row.VBcST)
				f.SetCellValue(sheetName, fmt.Sprintf("L%d", excelRow), row.VST)
				f.SetCellValue(sheetName, fmt.Sprintf("M%d", excelRow), row.AliqInter)
				f.SetCellValue(sheetName, fmt.Sprintf("N%d", excelRow), row.AliqInterna)
				f.SetCellValue(sheetName, fmt.Sprintf("O%d", excelRow), row.IcmsDevidoEst)
				f.SetCellValue(sheetName, fmt.Sprintf("P%d", excelRow), row.ChaveNFe)
				f.SetCellValue(sheetName, fmt.Sprintf("Q%d", excelRow), "")
				// Aplica estilos por tipo de coluna
				textStyle := 0
				mStyle := moneyStyle
				nStyle := numStyle
				if sd.warn {
					textStyle = warnStyle
					mStyle = moneyWarnStyle
					nStyle = numWarnStyle
				}
				if textStyle > 0 {
					for _, c := range []string{"A","B","C","D","E","F","G","P","Q"} {
						f.SetCellStyle(sheetName, fmt.Sprintf("%s%d", c, excelRow), fmt.Sprintf("%s%d", c, excelRow), textStyle)
					}
				}
				for _, c := range []string{"H","I","J","K","L","O"} {
					f.SetCellStyle(sheetName, fmt.Sprintf("%s%d", c, excelRow), fmt.Sprintf("%s%d", c, excelRow), mStyle)
				}
				for _, c := range []string{"M","N"} {
					f.SetCellStyle(sheetName, fmt.Sprintf("%s%d", c, excelRow), fmt.Sprintf("%s%d", c, excelRow), nStyle)
				}
				totalVProd += row.VProd
				totalVIcms += row.VIcms
				totalVBcST += row.VBcST
				totalVST += row.VST
				totalIcmsDevido += row.IcmsDevidoEst
				totalVIPI += row.VIPI
				excelRow++

				// ── linhas de CT-e abaixo da NF ─────────────────────────────
				for _, cte := range freteLinks[row.ChaveNFe] {
					// Coluna A: identificação "CT-e NNNNNN" + fonte
					label := fmt.Sprintf("CT-e %s", cte.NumeroCTe)
					if cte.Fonte != "D162" && cte.Fonte != "XML-CTE" {
						label += " (" + cte.Fonte + ")"
					}
					f.SetCellValue(sheetName, fmt.Sprintf("A%d", excelRow), row.DataEmissao)
					f.SetCellValue(sheetName, fmt.Sprintf("B%d", excelRow), label)
					f.SetCellValue(sheetName, fmt.Sprintf("C%d", excelRow), cte.EmitNome)
					f.SetCellValue(sheetName, fmt.Sprintf("D%d", excelRow), cte.EmitCNPJ)
					f.SetCellValue(sheetName, fmt.Sprintf("E%d", excelRow), "") // UF transportadora não crítica
					f.SetCellValue(sheetName, fmt.Sprintf("F%d", excelRow), "CTE")
					f.SetCellValue(sheetName, fmt.Sprintf("G%d", excelRow), row.Regime)
					f.SetCellValue(sheetName, fmt.Sprintf("H%d", excelRow), cte.VPrest)
					f.SetCellValue(sheetName, fmt.Sprintf("I%d", excelRow), 0) // CT-e não tem IPI
					f.SetCellValue(sheetName, fmt.Sprintf("J%d", excelRow), cte.VIcmsCTe)
					f.SetCellValue(sheetName, fmt.Sprintf("K%d", excelRow), 0) // V.BC ST (n/a)
					f.SetCellValue(sheetName, fmt.Sprintf("L%d", excelRow), 0) // V.ST (n/a)
					f.SetCellValue(sheetName, fmt.Sprintf("M%d", excelRow), row.AliqInter)
					f.SetCellValue(sheetName, fmt.Sprintf("N%d", excelRow), row.AliqInterna)
					f.SetCellValue(sheetName, fmt.Sprintf("O%d", excelRow), cte.IcmsFronteira)
					f.SetCellValue(sheetName, fmt.Sprintf("P%d", excelRow), row.ChaveNFe)
					f.SetCellValue(sheetName, fmt.Sprintf("Q%d", excelRow), cte.ChaveCTe)
					for _, c := range []string{"A","B","C","D","E","F","G","P","Q"} {
						f.SetCellStyle(sheetName, fmt.Sprintf("%s%d", c, excelRow), fmt.Sprintf("%s%d", c, excelRow), cteStyle)
					}
					for _, c := range []string{"H","I","J","K","L","O"} {
						f.SetCellStyle(sheetName, fmt.Sprintf("%s%d", c, excelRow), fmt.Sprintf("%s%d", c, excelRow), moneyCteStyle)
					}
					for _, c := range []string{"M","N"} {
						f.SetCellStyle(sheetName, fmt.Sprintf("%s%d", c, excelRow), fmt.Sprintf("%s%d", c, excelRow), numCteStyle)
					}
					totalVProd += cte.VPrest
					totalVIcms += cte.VIcmsCTe
					totalIcmsDevido += cte.IcmsFronteira
					excelRow++
				}
			}
			totalRow := excelRow
			f.SetCellValue(sheetName, fmt.Sprintf("A%d", totalRow), "TOTAL")
			f.SetCellStyle(sheetName, fmt.Sprintf("A%d", totalRow), fmt.Sprintf("Q%d", totalRow), boldStyle)
			f.SetCellValue(sheetName, fmt.Sprintf("H%d", totalRow), totalVProd)
			f.SetCellValue(sheetName, fmt.Sprintf("I%d", totalRow), totalVIPI)
			f.SetCellValue(sheetName, fmt.Sprintf("J%d", totalRow), totalVIcms)
			// Aplica formato monetário nas células TOTAL (mantém bold)
			for _, c := range []string{"H","I","J","K","L","O"} {
				f.SetCellStyle(sheetName, fmt.Sprintf("%s%d", c, totalRow), fmt.Sprintf("%s%d", c, totalRow), moneyBoldStyle)
			}
			f.SetCellValue(sheetName, fmt.Sprintf("K%d", totalRow), totalVBcST)
			f.SetCellValue(sheetName, fmt.Sprintf("L%d", totalRow), totalVST)
			f.SetCellValue(sheetName, fmt.Sprintf("O%d", totalRow), totalIcmsDevido)
		}

		// ── Sheet C — XML não lançadas no SPED ───────────────────────────────
		if periodo != "" && regime != "todos" {
			regimeUpper := strings.ToUpper(regime)
			cRows, err := fetchNaoSpedRows(db, companyID, periodo, regimeUpper)
			if err != nil {
				log.Printf("IcmsFronteiraExportXLSX nao-sped error: %v", err)
			} else {
				// Pré-carrega CT-es (toma=destinatário) para todas as NFs do Bloco C.
				chaves := make([]string, 0, len(cRows))
				for _, r := range cRows {
					chaves = append(chaves, r.ChaveNFe)
				}
				ctesPorNFe := fetchCTesPorChaveNFe(db, companyID, chaves)

				cSheet := "C - Não no SPED (XML)"
				f.NewSheet(cSheet)
				cHeaders := []string{"Data Emissão", "NF-e", "Fornecedor", "CNPJ", "UF", "CFOP Saída", "Regime", "V.Operação", "V.IPI", "ICMS Est.", "Classificação", "Chave NF-e", "Chave CT-e"}
				cCols := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M"}
				for i, h := range cHeaders {
					cell := fmt.Sprintf("%s1", cCols[i])
					f.SetCellValue(cSheet, cell, h)
					f.SetCellStyle(cSheet, cell, cell, headerStyle)
				}
				slateStyle, _      := f.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"F1F5F9"}}})
				moneySlateStyle, _ := f.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"F1F5F9"}}, CustomNumFmt: &moneyFmt})
				cteRowStyle, _     := f.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"E2EFDA"}}, Font: &excelize.Font{Italic: true}})
				moneyCteRowStyle, _ := f.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"E2EFDA"}}, Font: &excelize.Font{Italic: true}, CustomNumFmt: &moneyFmt})
				var totalIcmsC, totalIpiC float64
				er := 2
				for _, row := range cRows {
					// ── linha da NF ─────────────────────────────────────────────
					f.SetCellValue(cSheet, fmt.Sprintf("A%d", er), row.DataEmissao)
					f.SetCellValue(cSheet, fmt.Sprintf("B%d", er), row.NumeroNFe)
					f.SetCellValue(cSheet, fmt.Sprintf("C%d", er), row.FornNome)
					f.SetCellValue(cSheet, fmt.Sprintf("D%d", er), row.FornCNPJ)
					f.SetCellValue(cSheet, fmt.Sprintf("E%d", er), row.FornUF)
					f.SetCellValue(cSheet, fmt.Sprintf("F%d", er), row.CfopSaida)
					f.SetCellValue(cSheet, fmt.Sprintf("G%d", er), row.Regime)
					f.SetCellValue(cSheet, fmt.Sprintf("H%d", er), row.VOpr)
					f.SetCellValue(cSheet, fmt.Sprintf("I%d", er), row.VIPI)
					f.SetCellValue(cSheet, fmt.Sprintf("J%d", er), row.IcmsDevidoEst)
					f.SetCellValue(cSheet, fmt.Sprintf("K%d", er), row.ClassStatus)
					f.SetCellValue(cSheet, fmt.Sprintf("L%d", er), row.ChaveNFe)
					for _, c := range []string{"A","B","C","D","E","F","G","K","L","M"} {
						f.SetCellStyle(cSheet, fmt.Sprintf("%s%d", c, er), fmt.Sprintf("%s%d", c, er), slateStyle)
					}
					for _, c := range []string{"H","I","J"} {
						f.SetCellStyle(cSheet, fmt.Sprintf("%s%d", c, er), fmt.Sprintf("%s%d", c, er), moneySlateStyle)
					}
					totalIcmsC += row.IcmsDevidoEst
					totalIpiC += row.VIPI
					er++

					// ── linhas filhas: CT-es vinculados (verde) ────────────────
					for _, cte := range ctesPorNFe[row.ChaveNFe] {
						label := fmt.Sprintf("CT-e %s", cte.NumeroCTe)
						f.SetCellValue(cSheet, fmt.Sprintf("A%d", er), row.DataEmissao)
						f.SetCellValue(cSheet, fmt.Sprintf("B%d", er), label)
						f.SetCellValue(cSheet, fmt.Sprintf("C%d", er), cte.EmitNome)
						f.SetCellValue(cSheet, fmt.Sprintf("D%d", er), cte.EmitCNPJ)
						f.SetCellValue(cSheet, fmt.Sprintf("E%d", er), "")
						f.SetCellValue(cSheet, fmt.Sprintf("F%d", er), "CTE")
						f.SetCellValue(cSheet, fmt.Sprintf("G%d", er), row.Regime)
						f.SetCellValue(cSheet, fmt.Sprintf("H%d", er), cte.VPrest)
						f.SetCellValue(cSheet, fmt.Sprintf("I%d", er), 0) // CT-e não tem IPI
						f.SetCellValue(cSheet, fmt.Sprintf("J%d", er), cte.VIcmsCTe)
						f.SetCellValue(cSheet, fmt.Sprintf("K%d", er), "")
						f.SetCellValue(cSheet, fmt.Sprintf("L%d", er), row.ChaveNFe)
						f.SetCellValue(cSheet, fmt.Sprintf("M%d", er), cte.ChaveCTe)
						for _, c := range []string{"A","B","C","D","E","F","G","K","L","M"} {
							f.SetCellStyle(cSheet, fmt.Sprintf("%s%d", c, er), fmt.Sprintf("%s%d", c, er), cteRowStyle)
						}
						for _, c := range []string{"H","I","J"} {
							f.SetCellStyle(cSheet, fmt.Sprintf("%s%d", c, er), fmt.Sprintf("%s%d", c, er), moneyCteRowStyle)
						}
						er++
					}
				}
				f.SetCellValue(cSheet, fmt.Sprintf("A%d", er), "TOTAL")
				f.SetCellStyle(cSheet, fmt.Sprintf("A%d", er), fmt.Sprintf("M%d", er), boldStyle)
				f.SetCellValue(cSheet, fmt.Sprintf("I%d", er), totalIpiC)
				f.SetCellValue(cSheet, fmt.Sprintf("J%d", er), totalIcmsC)
				// Aplica moeda+bold nas colunas H, I e J do TOTAL
				for _, c := range []string{"H","I","J"} {
					f.SetCellStyle(cSheet, fmt.Sprintf("%s%d", c, er), fmt.Sprintf("%s%d", c, er), moneyBoldStyle)
				}
			}
		}

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
// helpers para o relatório HTML
// ---------------------------------------------------------------------------

// brl formata um float64 como moeda brasileira: R$ 1.234,56
func brl(v float64) string {
	prefix := "R$ "
	if v < 0 {
		v = -v
		prefix = "-R$ "
	}
	cents := int64(v*100 + 0.5)
	whole := cents / 100
	dec := cents % 100
	s := fmt.Sprintf("%d", whole)
	if len(s) > 3 {
		r := len(s) % 3
		var buf strings.Builder
		if r > 0 {
			buf.WriteString(s[:r])
		}
		for i := r; i < len(s); i += 3 {
			if buf.Len() > 0 {
				buf.WriteByte('.')
			}
			buf.WriteString(s[i : i+3])
		}
		return prefix + buf.String() + fmt.Sprintf(",%02d", dec)
	}
	return prefix + s + fmt.Sprintf(",%02d", dec)
}

// fetchTopNcmByChave busca o NCM principal de cada chave_nfe em lote.
func fetchTopNcmByChave(db *sql.DB, companyID string, chaves []string) map[string]string {
	result := make(map[string]string)
	if len(chaves) == 0 {
		return result
	}
	ph := make([]string, len(chaves))
	args := make([]interface{}, len(chaves))
	for i, c := range chaves {
		ph[i] = fmt.Sprintf("$%d", i+1)
		args[i] = c
	}
	args = append(args, companyID)
	q := fmt.Sprintf(`
		SELECT ne.chave_nfe, sub.ncm
		FROM nfe_entradas ne
		JOIN (
			SELECT DISTINCT ON (nfe_id) nfe_id, ncm
			FROM nfe_entradas_itens
			WHERE ncm IS NOT NULL AND ncm != ''
			ORDER BY nfe_id, ncm
		) sub ON sub.nfe_id = ne.id
		WHERE ne.chave_nfe IN (%s)
		  AND ne.company_id = $%d::uuid
	`, strings.Join(ph, ","), len(chaves)+1)
	rows, err := db.Query(q, args...)
	if err != nil {
		log.Printf("fetchTopNcmByChave error: %v", err)
		return result
	}
	defer rows.Close()
	for rows.Next() {
		var chave, ncm string
		if err := rows.Scan(&chave, &ncm); err == nil {
			if _, exists := result[chave]; !exists {
				result[chave] = ncm
			}
		}
	}
	return result
}

const antecipacaoReportCSS = `<style>
* { box-sizing: border-box; }
html { -webkit-print-color-adjust: exact; print-color-adjust: exact; }
body { font-family: Arial, Helvetica, sans-serif; font-size: 11px; line-height: 1.4; margin: 24px; color: #1f2937; }

/* Paleta discreta: tons slate (azul-acinzentado), pouca saturação */
/* ── Cabeçalho do relatório ─────────────────────────────────────────── */
.rpt-header { border-bottom: 2px solid #cbd5e1; padding-bottom: 12px; margin-bottom: 18px; display: flex; align-items: center; gap: 18px; }
.rpt-head-txt { flex: 1; }
.rpt-logo   { max-height: 60px; max-width: 220px; object-fit: contain; }
.rpt-grupo  { font-size: 17px; font-weight: 700; color: #334155; letter-spacing: .2px; }
.rpt-title  { font-size: 14px; font-weight: 600; color: #475569; margin-top: 1px; }
.rpt-sub    { font-size: 12px; font-weight: 600; color: #64748b; margin-top: 2px; }
.rpt-meta   { font-size: 10px; color: #94a3b8; margin-top: 5px; }

/* ── Quebra de seção (modelo do regime) ─────────────────────────────── */
.rpt-section { margin: 6px 0 14px; padding: 8px 14px; background: #f1f5f9; border-left: 3px solid #94a3b8; border-radius: 3px; font-size: 13px; font-weight: 700; color: #334155; }
.rpt-section-cnt { font-weight: 400; color: #64748b; font-size: 11px; }

/* ── Card por nota ──────────────────────────────────────────────────── */
.nf-card    { margin-bottom: 16px; border: 1px solid #e2e8f0; border-radius: 5px; overflow: hidden; page-break-inside: avoid; }
.nf-hdr     { background: #475569; color: #fff; padding: 7px 12px; display: flex; justify-content: space-between; align-items: center; }
.nf-num     { font-weight: 700; font-size: 12px; }
.nf-total   { font-weight: 700; font-size: 12px; }
.nf-forn    { background: #f1f5f9; padding: 5px 12px; font-size: 10px; color: #475569; border-bottom: 1px solid #e2e8f0; }
.nf-chave   { background: #f8fafc; padding: 4px 12px; font-size: 9px; color: #94a3b8; border-bottom: 1px solid #e2e8f0; font-family: "Courier New", monospace; letter-spacing: .5px; }
.nf-tbl     { width: 100%; border-collapse: collapse; }
.nf-tbl th  { background: #64748b; color: #fff; padding: 5px 7px; font-size: 9px; text-transform: uppercase; letter-spacing: .3px; text-align: right; white-space: nowrap; }
.nf-tbl th:nth-child(-n+2) { text-align: left; }
.nf-tbl td  { border-top: 1px solid #eef2f6; padding: 4px 7px; font-size: 10px; text-align: right; font-variant-numeric: tabular-nums; }
.nf-tbl td:nth-child(-n+2) { text-align: left; }
.nf-tbl tr:nth-child(even) td { background: #f8fafc; }
.tot-row td { font-weight: 700; background: #e2e8f0 !important; border-top: 2px solid #cbd5e1; color: #334155; }

/* ── Página de Totalização (uma linha por valor, fontes grandes) ────── */
.totpage       { page-break-before: always; padding-top: 10px; }
.totpage-title { font-size: 24px; font-weight: 700; color: #334155; border-bottom: 2px solid #cbd5e1; padding-bottom: 8px; }
.totpage-sub   { font-size: 13px; color: #94a3b8; margin: 6px 0 22px; }
.totlist       { max-width: 640px; }
.totrow        { display: flex; justify-content: space-between; align-items: baseline; padding: 14px 18px; font-size: 18px; border-bottom: 1px solid #e2e8f0; }
.totrow:nth-child(even) { background: #f8fafc; }
.totlabel      { color: #475569; }
.totval        { font-weight: 700; color: #334155; font-variant-numeric: tabular-nums; }
.totrow.totrow-grand { margin-top: 16px; padding: 20px 24px; background: #475569; border-radius: 6px; border-bottom: none; font-size: 24px; }
.totrow-grand .totlabel { color: #fff; font-weight: 600; }
.totrow-grand .totval   { color: #fff; font-size: 28px; }

.empty { text-align: center; color: #9ca3af; font-style: italic; padding: 40px; }

@media print {
  @page { size: landscape; margin: 10mm; }
  body  { margin: 0; }
  .nf-card { page-break-inside: avoid; }
  .totpage { page-break-before: always; }
}
</style>`

// ---------------------------------------------------------------------------
// IcmsFronteiraExportHTMLHandler — GET /api/icms-fronteira/exportar/pdf
// Returns printable HTML with per-NF cards (browser triggers window.print())
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
			regime = "antecipacao"
		}
		periodo := r.URL.Query().Get("periodo")

		var companyName, groupName string
		var logoData []byte
		var logoMime string
		if err := db.QueryRow(`SELECT COALESCE(NULLIF(c.trade_name,''), c.name, ''), COALESCE(eg.name,''),
			c.logo_data, COALESCE(c.logo_mime,'image/png')
			FROM companies c
			LEFT JOIN enterprise_groups eg ON c.group_id = eg.id
			WHERE c.id = $1::uuid`, companyID).Scan(&companyName, &groupName, &logoData, &logoMime); err != nil {
			log.Printf("IcmsFronteiraExportHTML: company lookup failed for %s: %v", companyID, err)
		}
		logoTag := ""
		if len(logoData) > 0 {
			logoTag = fmt.Sprintf(`<img class="rpt-logo" src="data:%s;base64,%s" alt="logo">`,
				logoMime, base64.StdEncoding.EncodeToString(logoData))
		}

		dataRows, err := fetchExportRows(db, companyID, regime, periodo, r)
		if err != nil {
			log.Printf("IcmsFronteiraExportHTML error: %v", err)
			http.Error(w, "Erro ao consultar dados", http.StatusInternalServerError)
			return
		}

		// Agrupar linhas por chave_nfe mantendo ordem de aparição
		type nfGroup struct {
			rows []fronteiraExportRow
		}
		var nfOrder []string
		nfMap := make(map[string]*nfGroup)
		for _, row := range dataRows {
			if _, exists := nfMap[row.ChaveNFe]; !exists {
				nfOrder = append(nfOrder, row.ChaveNFe)
				nfMap[row.ChaveNFe] = &nfGroup{}
			}
			nfMap[row.ChaveNFe].rows = append(nfMap[row.ChaveNFe].rows, row)
		}

		ncmByChave := fetchTopNcmByChave(db, companyID, nfOrder)

		regimeLabel := strings.ToUpper(regime)
		if regimeLabel == "TODOS" {
			regimeLabel = "Todos os Regimes"
		}
		regimeNome := map[string]string{
			"antecipacao": "Antecipação",
			"st":          "Substituição Tributária (ST)",
			"difal":       "DIFAL",
			"todos":       "Todos os Regimes",
		}[regime]
		if regimeNome == "" {
			regimeNome = regimeLabel
		}
		today := time.Now().Format("02/01/2006")

		var sb strings.Builder
		sb.WriteString(`<!DOCTYPE html><html lang="pt-BR"><head><meta charset="UTF-8">`)
		sb.WriteString(fmt.Sprintf(`<title>Relatório ICMS %s — %s</title>`, regimeLabel, htmlEscape(periodo)))
		sb.WriteString(antecipacaoReportCSS)
		sb.WriteString(`</head><body>`)

		// Cabeçalho do relatório — logo à esquerda, nome do grupo ao lado dela
		sb.WriteString(`<div class="rpt-header">`)
		sb.WriteString(logoTag)
		sb.WriteString(`<div class="rpt-head-txt">`)
		if groupName != "" {
			sb.WriteString(fmt.Sprintf(`<div class="rpt-grupo">%s</div>`, htmlEscape(groupName)))
		}
		sb.WriteString(`<div class="rpt-title">Relatório de Cálculo - ICMS Fronteira</div>`)
		sb.WriteString(fmt.Sprintf(`<div class="rpt-sub">Relatório de Cálculo - %s</div>`, regimeNome))
		sb.WriteString(fmt.Sprintf(`<div class="rpt-meta">Período: %s &nbsp;|&nbsp; Empresa: %s &nbsp;|&nbsp; Emissão: %s</div>`,
			htmlEscape(periodo), htmlEscape(companyName), today))
		sb.WriteString(`</div>`)
		sb.WriteString(`</div>`)

		if len(nfOrder) == 0 {
			sb.WriteString(`<div class="empty">Nenhuma nota encontrada para este período e regime.</div>`)
		} else {
			// Quebra única pelo modelo do regime — não repetido em cada nota
			sb.WriteString(fmt.Sprintf(
				`<div class="rpt-section">Modelo: %s <span class="rpt-section-cnt">(%d nota(s))</span></div>`,
				htmlEscape(regimeNome), len(nfOrder)))
		}

		var grandVOpr, grandBase, grandIcmsDest, grandST, grandDevido float64

		for _, chave := range nfOrder {
			grp := nfMap[chave]
			first := grp.rows[0]
			ncm := ncmByChave[chave]
			if ncm == "" {
				ncm = "-"
			}

			var nfVOpr, nfBase, nfIcmsDest, nfST, nfDevido float64
			for _, row := range grp.rows {
				nfVOpr += row.VProd
				nfBase += row.VProd
				nfIcmsDest += row.VIcms
				nfST += row.VST
				nfDevido += row.IcmsDevidoEst
			}

			sb.WriteString(`<div class="nf-card">`)

			// Cabeçalho da NF
			sb.WriteString(fmt.Sprintf(
				`<div class="nf-hdr"><span class="nf-num">NF: %s</span><span class="nf-total">Total Devido: %s</span></div>`,
				htmlEscape(first.NumeroNFe), brl(nfDevido)))
			sb.WriteString(fmt.Sprintf(
				`<div class="nf-forn">Fornecedor: %s</div>`,
				htmlEscape(first.FornNome)))
			sb.WriteString(fmt.Sprintf(`<div class="nf-chave">Chave: %s</div>`, htmlEscape(chave)))

			// Tabela de itens
			sb.WriteString(`<table class="nf-tbl"><thead><tr>`)
			for _, h := range []string{"Cód.", "NCM", "V. Operação", "MVA", "Alíq. I/I", "Base Cálc.", "ICMS Dest.", "ICMS-ST Ret", "V. Devido"} {
				sb.WriteString(fmt.Sprintf(`<th>%s</th>`, h))
			}
			sb.WriteString(`</tr></thead><tbody>`)

			for i, row := range grp.rows {
				vstDisp := "-"
				if row.VST > 0.001 {
					vstDisp = brl(row.VST)
				}
				aliqII := fmt.Sprintf("%.1f%% / %.1f%%", row.AliqInter, row.AliqInterna)
				sb.WriteString(fmt.Sprintf(
					`<tr><td>%d</td><td>%s</td><td>%s</td><td>-</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>`,
					i+1, ncm,
					brl(row.VProd), aliqII, brl(row.VProd),
					brl(row.VIcms), vstDisp, brl(row.IcmsDevidoEst)))
			}

			// Linha TOTAIS da NF
			stTotDisp := "-"
			if nfST > 0.001 {
				stTotDisp = brl(nfST)
			}
			sb.WriteString(fmt.Sprintf(
				`<tr class="tot-row"><td colspan="2">TOTAIS</td><td>%s</td><td>-</td><td>-</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>`,
				brl(nfVOpr), brl(nfBase), brl(nfIcmsDest), stTotDisp, brl(nfDevido)))

			sb.WriteString(`</tbody></table></div>`)

			grandVOpr += nfVOpr
			grandBase += nfBase
			grandIcmsDest += nfIcmsDest
			grandST += nfST
			grandDevido += nfDevido
		}

		// Página de Totalização — um valor por linha, fontes grandes
		if len(nfOrder) > 0 {
			sb.WriteString(`<div class="totpage">`)
			sb.WriteString(`<div class="totpage-title">Totalização</div>`)
			sb.WriteString(fmt.Sprintf(
				`<div class="totpage-sub">Modelo: %s &nbsp;&bull;&nbsp; Período: %s &nbsp;&bull;&nbsp; Empresa: %s</div>`,
				htmlEscape(regimeNome), htmlEscape(periodo), htmlEscape(companyName)))
			sb.WriteString(`<div class="totlist">`)
			totRow := func(label, val string) {
				sb.WriteString(fmt.Sprintf(
					`<div class="totrow"><span class="totlabel">%s</span><span class="totval">%s</span></div>`, label, val))
			}
			totRow("Quantidade de Notas", fmt.Sprintf("%d", len(nfOrder)))
			totRow("Valor da Operação", brl(grandVOpr))
			totRow("Base de Cálculo", brl(grandBase))
			totRow("ICMS Destacado", brl(grandIcmsDest))
			totRow("ICMS-ST Retido", brl(grandST))
			sb.WriteString(fmt.Sprintf(
				`<div class="totrow totrow-grand"><span class="totlabel">Valor Total Devido</span><span class="totval">%s</span></div>`,
				brl(grandDevido)))
			sb.WriteString(`</div></div>`)
		}

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
