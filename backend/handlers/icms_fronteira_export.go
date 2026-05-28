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
    COALESCE(base_calc, 0) AS base_calc,
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
	VBcCalc       float64 // base_calc: BC para Antecipação/DIFAL (v_prod+IPI+frete)
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
			&row.VBcCalc,
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

// CSV — colunas Blocos A/B (19 colunas). Modelo correto com CT-e interleaved.
// V.BC ST = BC da ST (com MVA aplicado); V.BC Antecip. = base_calc (v_prod+IPI+frete).
var exportCSVHeaders = []string{
	"Bloco", "Data Emissão", "Número NF-e", "Fornecedor", "CNPJ", "UF", "CFOP", "Regime",
	"V.Prod", "V.IPI", "ICMS Atual", "V.BC ST", "V.BC Antecip.", "V.ST", "Alíq.Inter.%", "Alíq.Interna.%",
	"ICMS Devido Est.", "Chave NF-e", "Chave CT-e",
}

func blocoLabel(bloco string) string {
	if bloco == "mes_anterior" {
		return "A - Mês Anterior"
	}
	return "B - Mês Atual"
}

func rowToCSVRecord(row fronteiraExportRow) []string {
	// V.BC Antecip. = base_calc para Antecipação e DIFAL; zero para ST
	// (ST já expõe seu BC próprio em V.BC ST, que inclui MVA aplicado)
	bcAnt := row.VBcCalc
	if row.Regime == "ST" {
		bcAnt = 0
	}
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
		fmt.Sprintf("%.2f", row.VIcms),      // ICMS Atual = ICMS NF
		fmt.Sprintf("%.2f", row.VBcST),      // V.BC ST
		fmt.Sprintf("%.2f", bcAnt),           // V.BC Antecip. (base_calc)
		fmt.Sprintf("%.2f", row.VST),        // V.ST
		fmt.Sprintf("%.2f", row.AliqInter),
		fmt.Sprintf("%.2f", row.AliqInterna),
		fmt.Sprintf("%.2f", row.IcmsDevidoEst),
		row.ChaveNFe,
		"", // Chave CT-e (vazia para NF-e)
	}
}

func cteLinkToCSVRecord(bloco string, link CteLink, chaveNFe string, aliqInterna float64) []string {
	icmsCTeDev := link.VPrest*aliqInterna/100.0 - link.VIcmsCTe
	if icmsCTeDev < 0 {
		icmsCTeDev = 0
	}
	return []string{
		bloco,
		link.DataEmissao,
		"CT-e " + link.NumeroCTe,
		link.EmitNome,
		link.EmitCNPJ,
		"",    // UF (vazia para CT-e)
		"CTE", // CFOP
		"",    // Regime
		fmt.Sprintf("%.2f", link.VPrest),
		"0.00",
		fmt.Sprintf("%.2f", link.VIcmsCTe), // ICMS Atual = ICMS do CT-e
		"0.00", // V.BC ST
		"0.00", // V.BC Antecip.
		"0.00",
		"",
		fmt.Sprintf("%.2f", aliqInterna),
		fmt.Sprintf("%.2f", icmsCTeDev),
		chaveNFe,
		link.ChaveCTe,
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

		// Busca CT-es vinculados para as NFs exportadas
		chaves := make([]string, len(rows))
		for i, row := range rows {
			chaves[i] = row.ChaveNFe
		}
		cteLinks := fetchCteLinksForNFs(db, companyID, chaves)

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
			for _, cte := range cteLinks[row.ChaveNFe] {
				if err := cw.Write(cteLinkToCSVRecord(blocoLabel(row.Bloco), cte, row.ChaveNFe, row.AliqInterna)); err != nil {
					log.Printf("IcmsFronteiraExportCSV cte row error: %v", err)
				}
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

		// Modelo correto: 18 colunas (A–R), CT-es interleaved após cada NF.
		// J = ICMS Atual (ICMS NF); K = V.BC ST; L = V.BC Antecip.; M = V.ST; R = Chave CT-e.
		abHeaders := []string{
			"Data Emissão", "Número NF-e", "Fornecedor", "CNPJ", "UF", "CFOP", "Regime",
			"V.Prod", "V.IPI", "ICMS Atual", "V.BC ST", "V.BC Antecip.", "V.ST", "Alíq.Inter.%", "Alíq.Interna.%",
			"ICMS Devido Est.", "Chave NF-e", "Chave CT-e",
		}
		cols := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R"}

		// Busca CT-es vinculados para as NFs exportadas
		spedChaves := make([]string, len(dataRows))
		for i, row := range dataRows {
			spedChaves[i] = row.ChaveNFe
		}
		cteLinksMap := fetchCteLinksForNFs(db, companyID, spedChaves)

		// Estilos específicos para linhas de CT-e (fundo azul-acinzentado claro)
		cteBlueFmt := "DCE6F1"
		cteTextStyle, _  := f.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{cteBlueFmt}}})
		cteMoneyStyle, _ := f.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{cteBlueFmt}}, CustomNumFmt: &moneyFmt})
		cteNumStyle, _   := f.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{cteBlueFmt}}, CustomNumFmt: &numFmt})

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

			for i, h := range abHeaders {
				cell := fmt.Sprintf("%s1", cols[i])
				f.SetCellValue(sheetName, cell, h)
				f.SetCellStyle(sheetName, cell, cell, headerStyle)
			}

			var totalVProd, totalVIcms, totalIcmsDevido, totalVIPI float64
			excelRow := 2
			for _, row := range sheetRows {
				// ── Linha da NF ──
				textStyle := 0
				mStyle := moneyStyle
				nStyle := numStyle
				if sd.warn {
					textStyle = warnStyle
					mStyle = moneyWarnStyle
					nStyle = numWarnStyle
				}
				f.SetCellValue(sheetName, fmt.Sprintf("A%d", excelRow), row.DataEmissao)
				f.SetCellValue(sheetName, fmt.Sprintf("B%d", excelRow), row.NumeroNFe)
				f.SetCellValue(sheetName, fmt.Sprintf("C%d", excelRow), row.FornNome)
				f.SetCellValue(sheetName, fmt.Sprintf("D%d", excelRow), row.FornCNPJ)
				f.SetCellValue(sheetName, fmt.Sprintf("E%d", excelRow), row.FornUF)
				f.SetCellValue(sheetName, fmt.Sprintf("F%d", excelRow), row.CFOP)
				f.SetCellValue(sheetName, fmt.Sprintf("G%d", excelRow), row.Regime)
				f.SetCellValue(sheetName, fmt.Sprintf("H%d", excelRow), row.VProd)
				f.SetCellValue(sheetName, fmt.Sprintf("I%d", excelRow), row.VIPI)
				f.SetCellValue(sheetName, fmt.Sprintf("J%d", excelRow), row.VIcms)  // ICMS Atual = ICMS NF
				f.SetCellValue(sheetName, fmt.Sprintf("K%d", excelRow), row.VBcST)  // V.BC ST
				// L: V.BC Antecip. = base_calc para Antecipação/DIFAL, zero para ST
				bcAnt := row.VBcCalc
				if row.Regime == "ST" {
					bcAnt = 0
				}
				f.SetCellValue(sheetName, fmt.Sprintf("L%d", excelRow), bcAnt)       // V.BC Antecip.
				f.SetCellValue(sheetName, fmt.Sprintf("M%d", excelRow), row.VST)     // V.ST
				f.SetCellValue(sheetName, fmt.Sprintf("N%d", excelRow), row.AliqInter)
				f.SetCellValue(sheetName, fmt.Sprintf("O%d", excelRow), row.AliqInterna)
				f.SetCellValue(sheetName, fmt.Sprintf("P%d", excelRow), row.IcmsDevidoEst)
				f.SetCellValue(sheetName, fmt.Sprintf("Q%d", excelRow), row.ChaveNFe)
				// R (Chave CT-e) fica vazio nas linhas de NF
				if textStyle > 0 {
					for _, c := range []string{"A","B","C","D","E","F","G","Q","R"} {
						f.SetCellStyle(sheetName, fmt.Sprintf("%s%d", c, excelRow), fmt.Sprintf("%s%d", c, excelRow), textStyle)
					}
				}
				for _, c := range []string{"H","I","J","K","L","M","P"} {
					f.SetCellStyle(sheetName, fmt.Sprintf("%s%d", c, excelRow), fmt.Sprintf("%s%d", c, excelRow), mStyle)
				}
				for _, c := range []string{"N","O"} {
					f.SetCellStyle(sheetName, fmt.Sprintf("%s%d", c, excelRow), fmt.Sprintf("%s%d", c, excelRow), nStyle)
				}
				totalVProd += row.VProd
				totalVIcms += row.VIcms
				totalIcmsDevido += row.IcmsDevidoEst
				totalVIPI += row.VIPI
				excelRow++

				// ── Linhas de CT-e (interleaved) ──
				for _, cte := range cteLinksMap[row.ChaveNFe] {
					icmsCTeDev := cte.VPrest*row.AliqInterna/100.0 - cte.VIcmsCTe
					if icmsCTeDev < 0 {
						icmsCTeDev = 0
					}
					f.SetCellValue(sheetName, fmt.Sprintf("A%d", excelRow), cte.DataEmissao)
					f.SetCellValue(sheetName, fmt.Sprintf("B%d", excelRow), "CT-e "+cte.NumeroCTe)
					f.SetCellValue(sheetName, fmt.Sprintf("C%d", excelRow), cte.EmitNome)
					f.SetCellValue(sheetName, fmt.Sprintf("D%d", excelRow), cte.EmitCNPJ)
					// E (UF) vazia; F = "CTE"; G (Regime) vazio
					f.SetCellValue(sheetName, fmt.Sprintf("F%d", excelRow), "CTE")
					f.SetCellValue(sheetName, fmt.Sprintf("H%d", excelRow), cte.VPrest)
					f.SetCellValue(sheetName, fmt.Sprintf("I%d", excelRow), 0.0)
					f.SetCellValue(sheetName, fmt.Sprintf("J%d", excelRow), cte.VIcmsCTe) // ICMS Atual = ICMS CT-e
					f.SetCellValue(sheetName, fmt.Sprintf("K%d", excelRow), 0.0) // V.BC ST
					f.SetCellValue(sheetName, fmt.Sprintf("L%d", excelRow), 0.0) // V.BC Antecip.
					f.SetCellValue(sheetName, fmt.Sprintf("M%d", excelRow), 0.0) // V.ST
					f.SetCellValue(sheetName, fmt.Sprintf("O%d", excelRow), row.AliqInterna)
					f.SetCellValue(sheetName, fmt.Sprintf("P%d", excelRow), icmsCTeDev) // ICMS Devido Est. do CT-e
					f.SetCellValue(sheetName, fmt.Sprintf("Q%d", excelRow), row.ChaveNFe) // Chave da NF!
					f.SetCellValue(sheetName, fmt.Sprintf("R%d", excelRow), cte.ChaveCTe)
					for _, c := range []string{"A","B","C","D","E","F","G","Q","R"} {
						f.SetCellStyle(sheetName, fmt.Sprintf("%s%d", c, excelRow), fmt.Sprintf("%s%d", c, excelRow), cteTextStyle)
					}
					for _, c := range []string{"H","I","J","K","L","M","P"} {
						f.SetCellStyle(sheetName, fmt.Sprintf("%s%d", c, excelRow), fmt.Sprintf("%s%d", c, excelRow), cteMoneyStyle)
					}
					for _, c := range []string{"N","O"} {
						f.SetCellStyle(sheetName, fmt.Sprintf("%s%d", c, excelRow), fmt.Sprintf("%s%d", c, excelRow), cteNumStyle)
					}
					totalIcmsDevido += icmsCTeDev
					excelRow++
				}
			}
			totalRow := excelRow
			f.SetCellValue(sheetName, fmt.Sprintf("A%d", totalRow), "TOTAL")
			f.SetCellStyle(sheetName, fmt.Sprintf("A%d", totalRow), fmt.Sprintf("R%d", totalRow), boldStyle)
			f.SetCellValue(sheetName, fmt.Sprintf("H%d", totalRow), totalVProd)
			f.SetCellValue(sheetName, fmt.Sprintf("I%d", totalRow), totalVIPI)
			f.SetCellValue(sheetName, fmt.Sprintf("J%d", totalRow), totalVIcms) // ICMS NF total
			f.SetCellValue(sheetName, fmt.Sprintf("P%d", totalRow), totalIcmsDevido)
			for _, c := range []string{"H","I","J","P"} {
				f.SetCellStyle(sheetName, fmt.Sprintf("%s%d", c, totalRow), fmt.Sprintf("%s%d", c, totalRow), moneyBoldStyle)
			}
		}

		// ── Sheet C — XML não lançadas no SPED (13 cols + CT-e interleaved) ──
		if periodo != "" && regime != "todos" {
			regimeUpper := strings.ToUpper(regime)
			ufExp := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("uf")))
			cRows, err := fetchNaoSpedRows(db, companyID, periodo, regimeUpper, ufExp)
			if err != nil {
				log.Printf("IcmsFronteiraExportXLSX nao-sped error: %v", err)
			} else {
				cSheet := "C - Não no SPED (XML)"
				f.NewSheet(cSheet)
				// Modelo correto Bloco C — 13 colunas:
				// A Data, B NF-e, C Forn, D CNPJ, E UF, F CFOP Saída,
				// G Regime, H V.Operação, I V.IPI, J ICMS Est., K Classificação,
				// L Chave NF-e, M Chave CT-e.
				cHeaders := []string{
					"Data Emissão", "NF-e", "Fornecedor", "CNPJ", "UF", "CFOP Saída",
					"Regime", "V.Operação", "V.IPI", "ICMS Est.", "Classificação",
					"Chave NF-e", "Chave CT-e",
				}
				cCols := cols[:13] // A–M
				for i, h := range cHeaders {
					cell := fmt.Sprintf("%s1", cCols[i])
					f.SetCellValue(cSheet, cell, h)
					f.SetCellStyle(cSheet, cell, cell, headerStyle)
				}
				slateStyle, _      := f.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"F1F5F9"}}})
				moneySlateStyle, _ := f.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"F1F5F9"}}, CustomNumFmt: &moneyFmt})
				cteCSlateStyle, _  := f.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{cteBlueFmt}}})
				cteCSlateMoney, _  := f.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{cteBlueFmt}}, CustomNumFmt: &moneyFmt})

				// Busca CT-es para as NFs do Bloco C
				cChaves := make([]string, len(cRows))
				for i, row := range cRows {
					cChaves[i] = row.ChaveNFe
				}
				cteLinksCMap := fetchCteLinksForNFs(db, companyID, cChaves)

				var totalVProdC, totalVIpiC, totalIcmsC float64
				er := 2
				for _, row := range cRows {
					f.SetCellValue(cSheet, fmt.Sprintf("A%d", er), row.DataEmissao)
					f.SetCellValue(cSheet, fmt.Sprintf("B%d", er), row.NumeroNFe)
					f.SetCellValue(cSheet, fmt.Sprintf("C%d", er), row.FornNome)
					f.SetCellValue(cSheet, fmt.Sprintf("D%d", er), row.FornCNPJ)
					f.SetCellValue(cSheet, fmt.Sprintf("E%d", er), row.FornUF)
					f.SetCellValue(cSheet, fmt.Sprintf("F%d", er), row.CfopSaida)
					f.SetCellValue(cSheet, fmt.Sprintf("G%d", er), row.Regime)
					f.SetCellValue(cSheet, fmt.Sprintf("H%d", er), row.VProd)        // V.Operação
					f.SetCellValue(cSheet, fmt.Sprintf("I%d", er), row.VIPI)
					f.SetCellValue(cSheet, fmt.Sprintf("J%d", er), row.IcmsDevidoEst) // ICMS Est.
					f.SetCellValue(cSheet, fmt.Sprintf("K%d", er), row.ClassStatus)   // Classificação
					f.SetCellValue(cSheet, fmt.Sprintf("L%d", er), row.ChaveNFe)
					// M (Chave CT-e) vazia para NF-e
					for _, c := range []string{"A","B","C","D","E","F","G","K","L","M"} {
						f.SetCellStyle(cSheet, fmt.Sprintf("%s%d", c, er), fmt.Sprintf("%s%d", c, er), slateStyle)
					}
					for _, c := range []string{"H","I","J"} {
						f.SetCellStyle(cSheet, fmt.Sprintf("%s%d", c, er), fmt.Sprintf("%s%d", c, er), moneySlateStyle)
					}
					totalVProdC += row.VProd
					totalVIpiC += row.VIPI
					totalIcmsC += row.IcmsDevidoEst
					er++

					// ── CT-e rows (ICMS Est = v_icms_cte, informativo) ──
					for _, cte := range cteLinksCMap[row.ChaveNFe] {
						f.SetCellValue(cSheet, fmt.Sprintf("A%d", er), cte.DataEmissao)
						f.SetCellValue(cSheet, fmt.Sprintf("B%d", er), "CT-e "+cte.NumeroCTe)
						f.SetCellValue(cSheet, fmt.Sprintf("C%d", er), cte.EmitNome)
						f.SetCellValue(cSheet, fmt.Sprintf("D%d", er), cte.EmitCNPJ)
						f.SetCellValue(cSheet, fmt.Sprintf("F%d", er), "CTE")
						f.SetCellValue(cSheet, fmt.Sprintf("H%d", er), cte.VPrest)
						f.SetCellValue(cSheet, fmt.Sprintf("I%d", er), 0.0)
						f.SetCellValue(cSheet, fmt.Sprintf("J%d", er), cte.VIcmsCTe) // ICMS informativo
						f.SetCellValue(cSheet, fmt.Sprintf("L%d", er), row.ChaveNFe)
						f.SetCellValue(cSheet, fmt.Sprintf("M%d", er), cte.ChaveCTe)
						for _, c := range []string{"A","B","C","D","E","F","G","K","L","M"} {
							f.SetCellStyle(cSheet, fmt.Sprintf("%s%d", c, er), fmt.Sprintf("%s%d", c, er), cteCSlateStyle)
						}
						for _, c := range []string{"H","I","J"} {
							f.SetCellStyle(cSheet, fmt.Sprintf("%s%d", c, er), fmt.Sprintf("%s%d", c, er), cteCSlateMoney)
						}
						er++
					}
				}
				f.SetCellValue(cSheet, fmt.Sprintf("A%d", er), "TOTAL")
				f.SetCellStyle(cSheet, fmt.Sprintf("A%d", er), fmt.Sprintf("M%d", er), boldStyle)
				f.SetCellValue(cSheet, fmt.Sprintf("H%d", er), totalVProdC)
				f.SetCellValue(cSheet, fmt.Sprintf("I%d", er), totalVIpiC)
				f.SetCellValue(cSheet, fmt.Sprintf("J%d", er), totalIcmsC)
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

// fetchCteLinksForNFs retorna os CT-es vinculados a cada NF-e, filtrando apenas
// quando o tomador é o destinatário (mesmo critério fiscal do motor de cálculo).
func fetchCteLinksForNFs(db *sql.DB, companyID string, chaves []string) map[string][]CteLink {
	result := make(map[string][]CteLink)
	if len(chaves) == 0 {
		return result
	}
	ph := make([]string, len(chaves))
	args := make([]interface{}, len(chaves)+1)
	args[0] = companyID
	for i, c := range chaves {
		ph[i] = fmt.Sprintf("$%d", i+2)
		args[i+1] = c
	}
	q := fmt.Sprintf(`
		SELECT ref.chave_nfe,
		       ce.chave_cte,
		       COALESCE(ce.numero_cte,'')        AS numero_cte,
		       COALESCE(ce.data_emissao::text,'') AS data_emissao,
		       COALESCE(ce.emit_nome,'')          AS emit_nome,
		       COALESCE(ce.emit_cnpj,'')          AS emit_cnpj,
		       COALESCE(ce.v_prest, 0)            AS v_prest,
		       COALESCE(ce.v_icms, 0)             AS v_icms_cte
		FROM cte_entradas_nfe_refs ref
		JOIN cte_entradas ce ON ce.id = ref.cte_id
		WHERE ref.company_id = $1::uuid
		  AND ref.chave_nfe IN (%s)
		  AND (
		      ce.toma = '3'
		      OR (ce.toma = '4' AND ce.toma4_cnpj = ce.dest_cnpj_cpf)
		  )
		ORDER BY ref.chave_nfe, ce.data_emissao
	`, strings.Join(ph, ","))
	rows, err := db.Query(q, args...)
	if err != nil {
		log.Printf("fetchCteLinksForNFs error: %v", err)
		return result
	}
	defer rows.Close()
	for rows.Next() {
		var chaveNFe string
		var link CteLink
		if err := rows.Scan(&chaveNFe, &link.ChaveCTe, &link.NumeroCTe, &link.DataEmissao,
			&link.EmitNome, &link.EmitCNPJ, &link.VPrest, &link.VIcmsCTe); err == nil {
			result[chaveNFe] = append(result[chaveNFe], link)
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

		// Linhas normalizadas: NF rows + CT-e rows interleaved (todos os blocos).
		type pdfRow struct {
			Data, NF, Forn, CNPJ, UF, CFOP, Chave string
			VProd, VIPI, VIcms, AliqInter, AliqInterna, IcmsFront float64
			IsCTe    bool   // true = linha de CT-e
			ChaveCTe string // chave do CT-e (quando IsCTe=true)
		}
		// Fetch CT-es para Blocos A/B
		spedChavesHTML := make([]string, len(dataRows))
		for i, row := range dataRows {
			spedChavesHTML[i] = row.ChaveNFe
		}
		cteLinksHTML := fetchCteLinksForNFs(db, companyID, spedChavesHTML)

		var blocoA, blocoB, blocoC []pdfRow
		for _, row := range dataRows {
			pr := pdfRow{Data: row.DataEmissao, NF: row.NumeroNFe, Forn: row.FornNome, CNPJ: row.FornCNPJ,
				UF: row.FornUF, CFOP: row.CFOP, Chave: row.ChaveNFe,
				VProd: row.VProd, VIPI: row.VIPI, VIcms: row.VIcms,
				AliqInter: row.AliqInter, AliqInterna: row.AliqInterna, IcmsFront: row.IcmsDevidoEst}
			var target *[]pdfRow
			if row.Bloco == "mes_anterior" {
				target = &blocoA
			} else {
				target = &blocoB
			}
			*target = append(*target, pr)
			for _, cte := range cteLinksHTML[row.ChaveNFe] {
				icmsCTeDev := cte.VPrest*row.AliqInterna/100.0 - cte.VIcmsCTe
				if icmsCTeDev < 0 {
					icmsCTeDev = 0
				}
				*target = append(*target, pdfRow{
					Data: cte.DataEmissao, NF: "CT-e " + cte.NumeroCTe,
					Forn: cte.EmitNome, CNPJ: cte.EmitCNPJ, UF: "", CFOP: "CTE",
					Chave: row.ChaveNFe, ChaveCTe: cte.ChaveCTe,
					VProd: cte.VPrest, VIPI: 0, VIcms: cte.VIcmsCTe,
					AliqInter: 0, AliqInterna: row.AliqInterna, IcmsFront: icmsCTeDev,
					IsCTe: true,
				})
			}
		}
		if regime != "todos" && periodo != "" {
			ufExp := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("uf")))
			if cRows, errC := fetchNaoSpedRows(db, companyID, periodo, strings.ToUpper(regime), ufExp); errC == nil {
				cChavesHTML := make([]string, len(cRows))
				for i, row := range cRows {
					cChavesHTML[i] = row.ChaveNFe
				}
				cteLinksC := fetchCteLinksForNFs(db, companyID, cChavesHTML)
				for _, row := range cRows {
					blocoC = append(blocoC, pdfRow{Data: row.DataEmissao, NF: row.NumeroNFe, Forn: row.FornNome, CNPJ: row.FornCNPJ,
						UF: row.FornUF, CFOP: row.CfopSaida, Chave: row.ChaveNFe,
						VProd: row.VProd, VIPI: row.VIPI, VIcms: row.VIcmsNF,
						AliqInter: row.AliqInter, AliqInterna: row.AliqInterna, IcmsFront: row.IcmsDevidoEst})
					for _, cte := range cteLinksC[row.ChaveNFe] {
						blocoC = append(blocoC, pdfRow{
							Data: cte.DataEmissao, NF: "CT-e " + cte.NumeroCTe,
							Forn: cte.EmitNome, CNPJ: cte.EmitCNPJ, UF: "", CFOP: "CTE",
							Chave: row.ChaveNFe, ChaveCTe: cte.ChaveCTe,
							VProd: cte.VPrest, VIPI: 0, VIcms: cte.VIcmsCTe,
							IcmsFront: cte.VIcmsCTe, // ICMS informativo no Bloco C
							IsCTe: true,
						})
					}
				}
			}
		}
		totalNotas := len(blocoA) + len(blocoB) + len(blocoC)

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

		if totalNotas == 0 {
			sb.WriteString(`<div class="empty">Nenhuma nota encontrada para este período e regime.</div>`)
		}

		showAliq := regime != "st"
		numTd := func(v float64) string { return fmt.Sprintf(`<td style="text-align:right">%s</td>`, brl(v)) }

		var grandVProd, grandVIPI, grandVIcms, grandDevido float64

		// CT-e rows interleaved em todos os blocos. Linha de CT-e recebe fundo azul.
		renderBloco := func(titulo string, rows []pdfRow) {
			if len(rows) == 0 {
				return
			}
			nfCount := 0
			for _, r := range rows {
				if !r.IsCTe {
					nfCount++
				}
			}
			var tVProd, tVIPI, tVIcms, tDevido float64
			sb.WriteString(fmt.Sprintf(
				`<div class="rpt-section">%s <span class="rpt-section-cnt">(%d nota(s))</span></div>`,
				htmlEscape(titulo), nfCount))
			sb.WriteString(`<table class="nf-tbl"><thead><tr>`)
			heads := []string{"Data", "NF-e / CT-e", "Fornecedor", "UF", "CFOP", "V. Prod.", "V. IPI", "V. Operação", "ICMS NF"}
			if showAliq {
				heads = append(heads, "Alíq. Inter.", "Alíq. Int.")
			}
			heads = append(heads, "ICMS fronteira", "Chave NF-e")
			for _, h := range heads {
				sb.WriteString(fmt.Sprintf(`<th>%s</th>`, h))
			}
			sb.WriteString(`</tr></thead><tbody>`)
			for _, r := range rows {
				data := r.Data
				if len(data) >= 10 {
					data = data[:10]
				}
				rowStyle := ""
				if r.IsCTe {
					rowStyle = ` style="background:#dce6f1"`
				}
				sb.WriteString(fmt.Sprintf(`<tr%s>`, rowStyle))
				sb.WriteString(fmt.Sprintf(`<td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td>`,
					htmlEscape(data), htmlEscape(r.NF), htmlEscape(r.Forn), htmlEscape(r.UF), htmlEscape(r.CFOP)))
				sb.WriteString(numTd(r.VProd) + numTd(r.VIPI) + numTd(r.VProd+r.VIPI))
				sb.WriteString(numTd(r.VIcms))
				if showAliq {
					if r.IsCTe {
						sb.WriteString(`<td></td>`)
						sb.WriteString(fmt.Sprintf(`<td style="text-align:right">%.2f%%</td>`, r.AliqInterna))
					} else {
						sb.WriteString(fmt.Sprintf(`<td style="text-align:right">%.2f%%</td><td style="text-align:right">%.2f%%</td>`, r.AliqInter, r.AliqInterna))
					}
				}
				sb.WriteString(numTd(r.IcmsFront))
				chaveDisplay := r.Chave
				if r.IsCTe && r.ChaveCTe != "" {
					chaveDisplay = r.ChaveCTe
				}
				sb.WriteString(fmt.Sprintf(`<td style="font-size:7px;word-break:break-all">%s</td>`, htmlEscape(chaveDisplay)))
				sb.WriteString(`</tr>`)
				if !r.IsCTe {
					tVProd += r.VProd
					tVIPI += r.VIPI
					tVIcms += r.VIcms
				}
				tDevido += r.IcmsFront
			}
			sb.WriteString(`<tr class="tot-row"><td colspan="5">TOTAL</td>`)
			sb.WriteString(numTd(tVProd) + numTd(tVIPI) + numTd(tVProd+tVIPI))
			sb.WriteString(numTd(tVIcms))
			if showAliq {
				sb.WriteString(`<td></td><td></td>`)
			}
			sb.WriteString(numTd(tDevido) + `<td></td></tr>`)
			sb.WriteString(`</tbody></table>`)
			grandVProd += tVProd
			grandVIPI += tVIPI
			grandVIcms += tVIcms
			grandDevido += tDevido
		}

		renderBloco("Bloco A — NFs de meses anteriores no SPED", blocoA)
		renderBloco("Bloco B — NFs do mês presentes no SPED", blocoB)
		renderBloco("Bloco C — NFs do mês não localizadas no SPED (apenas XML)", blocoC)

		// Página de Totalização — um valor por linha, fontes grandes
		if totalNotas > 0 {
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
			totRow("Quantidade de Notas", fmt.Sprintf("%d", totalNotas))
			totRow("Valor dos Produtos", brl(grandVProd))
			totRow("IPI", brl(grandVIPI))
			totRow("Valor da Operação", brl(grandVProd+grandVIPI))
			totRow("ICMS Destacado (NF)", brl(grandVIcms))
			sb.WriteString(fmt.Sprintf(
				`<div class="totrow totrow-grand"><span class="totlabel">Total ICMS Fronteira</span><span class="totval">%s</span></div>`,
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
