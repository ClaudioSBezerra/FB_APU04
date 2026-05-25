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
	"V.Prod", "ICMS Atual", "V.BC ST", "V.ST", "Alíq.Inter.%", "Alíq.Interna.%", "ICMS Devido Est.",
	"Chave NF-e", "Chave CT-e", "V.IPI",
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
		fmt.Sprintf("%.2f", row.VIcms),
		fmt.Sprintf("%.2f", row.VBcST),
		fmt.Sprintf("%.2f", row.VST),
		fmt.Sprintf("%.2f", row.AliqInter),
		fmt.Sprintf("%.2f", row.AliqInterna),
		fmt.Sprintf("%.2f", row.IcmsDevidoEst),
		row.ChaveNFe,
		"",
		fmt.Sprintf("%.2f", row.VIPI),
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
				f.SetCellValue(sheetName, fmt.Sprintf("I%d", excelRow), row.VIcms)
				f.SetCellValue(sheetName, fmt.Sprintf("J%d", excelRow), row.VBcST)
				f.SetCellValue(sheetName, fmt.Sprintf("K%d", excelRow), row.VST)
				f.SetCellValue(sheetName, fmt.Sprintf("L%d", excelRow), row.AliqInter)
				f.SetCellValue(sheetName, fmt.Sprintf("M%d", excelRow), row.AliqInterna)
				f.SetCellValue(sheetName, fmt.Sprintf("N%d", excelRow), row.IcmsDevidoEst)
				f.SetCellValue(sheetName, fmt.Sprintf("O%d", excelRow), row.ChaveNFe)
				f.SetCellValue(sheetName, fmt.Sprintf("P%d", excelRow), "")
				f.SetCellValue(sheetName, fmt.Sprintf("Q%d", excelRow), row.VIPI)
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
					for _, c := range []string{"A","B","C","D","E","F","G","O","P"} {
						f.SetCellStyle(sheetName, fmt.Sprintf("%s%d", c, excelRow), fmt.Sprintf("%s%d", c, excelRow), textStyle)
					}
				}
				for _, c := range []string{"H","I","J","K","N","Q"} {
					f.SetCellStyle(sheetName, fmt.Sprintf("%s%d", c, excelRow), fmt.Sprintf("%s%d", c, excelRow), mStyle)
				}
				for _, c := range []string{"L","M"} {
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
					f.SetCellValue(sheetName, fmt.Sprintf("I%d", excelRow), cte.VIcmsCTe)
					f.SetCellValue(sheetName, fmt.Sprintf("J%d", excelRow), 0) // V.BC ST (n/a)
					f.SetCellValue(sheetName, fmt.Sprintf("K%d", excelRow), 0) // V.ST (n/a)
					f.SetCellValue(sheetName, fmt.Sprintf("L%d", excelRow), row.AliqInter)
					f.SetCellValue(sheetName, fmt.Sprintf("M%d", excelRow), row.AliqInterna)
					f.SetCellValue(sheetName, fmt.Sprintf("N%d", excelRow), cte.IcmsFronteira)
					f.SetCellValue(sheetName, fmt.Sprintf("O%d", excelRow), row.ChaveNFe)
					f.SetCellValue(sheetName, fmt.Sprintf("P%d", excelRow), cte.ChaveCTe)
					f.SetCellValue(sheetName, fmt.Sprintf("Q%d", excelRow), 0) // CT-e não tem IPI
					for _, c := range []string{"A","B","C","D","E","F","G","O","P"} {
						f.SetCellStyle(sheetName, fmt.Sprintf("%s%d", c, excelRow), fmt.Sprintf("%s%d", c, excelRow), cteStyle)
					}
					for _, c := range []string{"H","I","J","K","N","Q"} {
						f.SetCellStyle(sheetName, fmt.Sprintf("%s%d", c, excelRow), fmt.Sprintf("%s%d", c, excelRow), moneyCteStyle)
					}
					for _, c := range []string{"L","M"} {
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
			f.SetCellValue(sheetName, fmt.Sprintf("I%d", totalRow), totalVIcms)
			// Aplica formato monetário nas células TOTAL (mantém bold)
			for _, c := range []string{"H","I","J","K","N","Q"} {
				f.SetCellStyle(sheetName, fmt.Sprintf("%s%d", c, totalRow), fmt.Sprintf("%s%d", c, totalRow), moneyBoldStyle)
			}
			f.SetCellValue(sheetName, fmt.Sprintf("J%d", totalRow), totalVBcST)
			f.SetCellValue(sheetName, fmt.Sprintf("K%d", totalRow), totalVST)
			f.SetCellValue(sheetName, fmt.Sprintf("N%d", totalRow), totalIcmsDevido)
			f.SetCellValue(sheetName, fmt.Sprintf("Q%d", totalRow), totalVIPI)
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
				cHeaders := []string{"Data Emissão", "NF-e", "Fornecedor", "CNPJ", "UF", "CFOP Saída", "Regime", "V.Operação", "ICMS Est.", "Classificação", "Chave NF-e", "Chave CT-e"}
				cCols := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L"}
				for i, h := range cHeaders {
					cell := fmt.Sprintf("%s1", cCols[i])
					f.SetCellValue(cSheet, cell, h)
					f.SetCellStyle(cSheet, cell, cell, headerStyle)
				}
				slateStyle, _      := f.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"F1F5F9"}}})
				moneySlateStyle, _ := f.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"F1F5F9"}}, CustomNumFmt: &moneyFmt})
				cteRowStyle, _     := f.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"E2EFDA"}}, Font: &excelize.Font{Italic: true}})
				moneyCteRowStyle, _ := f.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"E2EFDA"}}, Font: &excelize.Font{Italic: true}, CustomNumFmt: &moneyFmt})
				var totalIcmsC float64
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
					f.SetCellValue(cSheet, fmt.Sprintf("I%d", er), row.IcmsDevidoEst)
					f.SetCellValue(cSheet, fmt.Sprintf("J%d", er), row.ClassStatus)
					f.SetCellValue(cSheet, fmt.Sprintf("K%d", er), row.ChaveNFe)
					for _, c := range []string{"A","B","C","D","E","F","G","J","K","L"} {
						f.SetCellStyle(cSheet, fmt.Sprintf("%s%d", c, er), fmt.Sprintf("%s%d", c, er), slateStyle)
					}
					for _, c := range []string{"H","I"} {
						f.SetCellStyle(cSheet, fmt.Sprintf("%s%d", c, er), fmt.Sprintf("%s%d", c, er), moneySlateStyle)
					}
					totalIcmsC += row.IcmsDevidoEst
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
						f.SetCellValue(cSheet, fmt.Sprintf("I%d", er), cte.VIcmsCTe)
						f.SetCellValue(cSheet, fmt.Sprintf("J%d", er), "")
						f.SetCellValue(cSheet, fmt.Sprintf("K%d", er), row.ChaveNFe)
						f.SetCellValue(cSheet, fmt.Sprintf("L%d", er), cte.ChaveCTe)
						for _, c := range []string{"A","B","C","D","E","F","G","J","K","L"} {
							f.SetCellStyle(cSheet, fmt.Sprintf("%s%d", c, er), fmt.Sprintf("%s%d", c, er), cteRowStyle)
						}
						for _, c := range []string{"H","I"} {
							f.SetCellStyle(cSheet, fmt.Sprintf("%s%d", c, er), fmt.Sprintf("%s%d", c, er), moneyCteRowStyle)
						}
						er++
					}
				}
				f.SetCellValue(cSheet, fmt.Sprintf("A%d", er), "TOTAL")
				f.SetCellStyle(cSheet, fmt.Sprintf("A%d", er), fmt.Sprintf("L%d", er), boldStyle)
				f.SetCellValue(cSheet, fmt.Sprintf("I%d", er), totalIcmsC)
				// Aplica moeda+bold nas colunas H e I do TOTAL
				for _, c := range []string{"H","I"} {
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
		periodo := r.URL.Query().Get("periodo")

		// Fetch company name
		var companyName string
		err = db.QueryRow(`SELECT COALESCE(nome_fantasia, razao_social, '') FROM companies WHERE id = $1::uuid`, companyID).Scan(&companyName)
		if err != nil {
			companyName = companyID
		}

		dataRows, err := fetchExportRows(db, companyID, regime, periodo, r)
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

		// Separar por bloco
		blocos := []struct{ key, label, color string }{
			{"mes_anterior", "Bloco A — NFs de Meses Anteriores no SPED", "#fff3cd"},
			{"mes_atual", "Bloco B — NFs do Mês Presentes no SPED", "#d4edda"},
		}
		headers := exportCSVHeaders[1:] // sem coluna "Bloco" no HTML (já separado em seções)

		var totalVProdGeral, totalIcmsDevidoGeral float64
		for _, bloco := range blocos {
			var blocoRows []fronteiraExportRow
			for _, row := range dataRows {
				if row.Bloco == bloco.key || (bloco.key == "mes_atual" && row.Bloco == "") {
					blocoRows = append(blocoRows, row)
				}
			}
			sb.WriteString(fmt.Sprintf(`<h3 style="margin-top:18px;padding:6px 8px;background:%s;border-radius:4px;font-size:12px;">%s <span style="font-weight:normal;color:#555;">(%d nota(s))</span></h3>`,
				bloco.color, bloco.label, len(blocoRows)))
			if bloco.key == "mes_anterior" {
				sb.WriteString(`<p style="font-size:10px;color:#856404;margin:0 0 6px;">⚠ O imposto pode já ter sido recolhido no mês de emissão. Verifique antes de incluir no cálculo.</p>`)
			}
			sb.WriteString(`<table><thead><tr>`)
			for _, h := range headers {
				sb.WriteString(fmt.Sprintf(`<th>%s</th>`, h))
			}
			sb.WriteString(`</tr></thead><tbody>`)
			var totalVProd, totalVIcms, totalVBcST, totalVST, totalIcmsDevido float64
			for _, row := range blocoRows {
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
			if len(blocoRows) > 0 {
				sb.WriteString(`<tr class="total-row"><td colspan="7"><strong>Subtotal</strong></td>`)
				sb.WriteString(fmt.Sprintf(`<td>%.2f</td><td>%.2f</td><td>%.2f</td><td>%.2f</td><td></td><td></td><td>%.2f</td>`,
					totalVProd, totalVIcms, totalVBcST, totalVST, totalIcmsDevido))
				sb.WriteString(`</tr>`)
			} else {
				sb.WriteString(`<tr><td colspan="14" style="text-align:center;color:#888;font-style:italic;">Nenhuma nota neste bloco.</td></tr>`)
			}
			sb.WriteString(`</tbody></table>`)
			if bloco.key == "mes_atual" {
				totalVProdGeral += totalVProd
				totalIcmsDevidoGeral += totalIcmsDevido
			}
		}
		sb.WriteString(fmt.Sprintf(`<p style="margin-top:12px;font-size:12px;">Bloco C (XML não lançadas no SPED) não consta neste relatório — consulte a aba Reconciliação para validar.</p>`))
		sb.WriteString(fmt.Sprintf(`<p style="font-weight:bold;font-size:13px;">Total do Mês (Bloco B): V.Prod = %.2f | ICMS Devido Est. = %.2f</p>`,
			totalVProdGeral, totalIcmsDevidoGeral))

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
