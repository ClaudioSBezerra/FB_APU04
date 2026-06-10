package handlers

import (
	"bytes"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/xuri/excelize/v2"
)

// ---------------------------------------------------------------------------
// Export do demonstrativo de ICMS-ST POR ITEM (Excel + PDF/HTML).
//
// Espelha exatamente a tela STItensTab (IcmsFronteira.tsx): agrupa por nota e
// renderiza linhas de produto → "Subtotal Produtos NF {n}" → linhas de CT-e com
// frete RATEADO por item → "Subtotal CT-es Vinculados" → "TOTAL GERAL NF {n}",
// e um rodapé com o total geral de ICMS a pagar.
//
//   GET /api/icms-fronteira/st-itens/exportar/xlsx?periodo=&uf=
//   GET /api/icms-fronteira/st-itens/exportar/pdf?periodo=&uf=
// ---------------------------------------------------------------------------

// stItensColHeaders — as 24 colunas, mesmos cabeçalhos da tela.
var stItensColHeaders = []string{
	"CFOP", "NF-e / CT-e", "Chave", "Fornecedor", "Cód. Produto", "Descrição",
	"NCM", "CEST", "Status", "V. Produto", "V. IPI", "Demais Acrésc.",
	"V. Operação", "MVA Orig.", "MVA Aj.", "Alíq. Inter", "Alíq. Int.",
	"ICMS Debitado", "Base Cálc.", "Red. BC", "BC Reduz.", "ICMS Calc.",
	"ICMS Retido", "ICMS a Pagar",
}

// stItemGrupo agrupa as linhas por chave_nfe mantendo a ordem de chegada — igual
// ao agrupamento do frontend.
type stItemGrupo struct {
	Chave string
	Itens []STItemRow
}

func groupSTItens(rows []STItemRow) []stItemGrupo {
	var grupos []stItemGrupo
	idx := map[string]int{}
	for _, r := range rows {
		gi, ok := idx[r.ChaveNFe]
		if !ok {
			gi = len(grupos)
			idx[r.ChaveNFe] = gi
			grupos = append(grupos, stItemGrupo{Chave: r.ChaveNFe})
		}
		grupos[gi].Itens = append(grupos[gi].Itens, r)
	}
	return grupos
}

// stBlocoGrupo agrupa as notas (stItemGrupo) de um mesmo bloco A/B/C.
type stBlocoGrupo struct {
	Bloco  string // "mes_anterior" | "mes_atual" | "nao_sped"
	Label  string // rótulo de seção
	Grupos []stItemGrupo
}

// groupSTItensByBloco particiona as linhas por bloco na ordem A/B/C, reusando
// groupSTItens dentro de cada partição. Só inclui blocos com itens.
func groupSTItensByBloco(rows []STItemRow) []stBlocoGrupo {
	defs := []struct{ key, label string }{
		{"mes_anterior", "Bloco A — Mês anterior (SPED)"},
		{"mes_atual", "Bloco B — Mês atual (SPED)"},
		{"nao_sped", "Bloco C — Notas fora do SPED (XML)"},
	}
	var out []stBlocoGrupo
	for _, d := range defs {
		var sub []STItemRow
		for _, r := range rows {
			if r.Bloco == d.key {
				sub = append(sub, r)
			}
		}
		if len(sub) == 0 {
			continue
		}
		out = append(out, stBlocoGrupo{Bloco: d.key, Label: d.label, Grupos: groupSTItens(sub)})
	}
	return out
}

// blocoLetraST extrai a letra (A/B/C) do rótulo "Bloco X — ...".
func blocoLetraST(label string) string {
	s := strings.TrimSpace(strings.TrimPrefix(label, "Bloco"))
	if i := strings.IndexAny(s, " —-"); i > 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

// cteSTItem — frete (CT-e) na Substituição Tributária (regra Gilson 2026-06-10):
// aplica a MVA do produto à fração do frete e calcula a ST, abatendo o ICMS do
// CT-e rateado. Espelha cteST do frontend. Só aplica quando o item tem
// regra+segmento (mesma trava do produto).
func cteSTItem(fracaoFrete, mvaAjustado, aliqInterna, icmsCteRateado float64, aplica bool) (base, calc, aPagar float64) {
	if !aplica {
		return 0, 0, 0
	}
	base = fracaoFrete * (1 + mvaAjustado/100)
	calc = base * aliqInterna / 100
	aPagar = calc - icmsCteRateado
	if aPagar < 0 {
		aPagar = 0
	}
	return
}

// fmtDateBRGo converte data ISO (YYYY-MM-DD / timestamp) para DD/MM/AAAA.
func fmtDateBRGo(v string) string {
	if len(v) < 10 {
		return v
	}
	s := v[:10]
	if len(s) == 10 && s[4] == '-' && s[7] == '-' {
		return s[8:10] + "/" + s[5:7] + "/" + s[0:4]
	}
	return s
}

// stBlocoDFaltantes retorna as notas do SPED (Bloco A/B) sem XML importado
// (StatusXML="Faltante"), agrupadas por nota. É o Bloco D — seção informativa de
// pendências (repete notas de A/B); NÃO entra no total geral, para não duplicar.
func stBlocoDFaltantes(rows []STItemRow) []stItemGrupo {
	var sub []STItemRow
	for _, r := range rows {
		if (r.Bloco == "mes_atual" || r.Bloco == "mes_anterior") && r.StatusXML == "Faltante" {
			sub = append(sub, r)
		}
	}
	if len(sub) == 0 {
		return nil
	}
	return groupSTItens(sub)
}

// ---------------------------------------------------------------------------
// XLSX
// ---------------------------------------------------------------------------

// IcmsFronteiraSTItensFaltantesXLSXHandler — GET /api/icms-fronteira/st-itens/faltantes/xlsx
// Planilha enxuta com as notas de ST do SPED SEM XML importado (todas as UFs),
// uma linha por nota, com a Chave de Acesso — para o contador baixar os XML na
// SEFAZ. Ignora o filtro de UF de propósito (lista completa BA+PE).
func IcmsFronteiraSTItensFaltantesXLSXHandler(db *sql.DB) http.HandlerFunc {
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
		periodo := strings.TrimSpace(r.URL.Query().Get("periodo"))

		rows, err := fetchSTItens(db, companyID, periodo, "") // uf="" → todas as filiais
		if err != nil {
			log.Printf("IcmsFronteiraSTItensFaltantesXLSX error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao gerar Excel de notas faltantes")
			return
		}
		data, err := buildSTFaltantesXLSX(rows)
		if err != nil {
			log.Printf("IcmsFronteiraSTItensFaltantesXLSX write error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao gerar arquivo XLSX")
			return
		}
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", `attachment; filename="st-notas-sem-xml.xlsx"`)
		w.Write(data)
	}
}

// buildSTFaltantesXLSX gera a planilha das notas de ST sem XML (Bloco D), uma
// linha por nota, com a Chave de Acesso para download na SEFAZ. Função PURA.
func buildSTFaltantesXLSX(rows []STItemRow) ([]byte, error) {
	grupos := stBlocoDFaltantes(rows)
	f := excelize.NewFile()
	sheet := "Notas sem XML"
	f.SetSheetName("Sheet1", sheet)

	headers := []string{"Número NF", "Chave de Acesso", "Fornecedor", "CNPJ Forn.", "UF Forn.", "Qtd. Itens", "V. Produtos", "ICMS-ST Retido (SPED)"}
	headStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"CBD5E1"}},
	})
	moneyFmt := `"R$" #,##0.00`
	moneyStyle, _ := f.NewStyle(&excelize.Style{CustomNumFmt: &moneyFmt})
	for i, h := range headers {
		c, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, c, h)
	}
	hl, _ := excelize.CoordinatesToCellName(len(headers), 1)
	f.SetCellStyle(sheet, "A1", hl, headStyle)

	er := 2
	for _, g := range grupos {
		it0 := g.Itens[0]
		var vProd, icmsRet float64
		for _, it := range g.Itens {
			vProd += it.VProd
			icmsRet += it.IcmsRetido
		}
		set := func(col int, v interface{}) {
			c, _ := excelize.CoordinatesToCellName(col, er)
			f.SetCellValue(sheet, c, v)
		}
		set(1, it0.NumeroNFe)
		set(2, g.Chave)
		set(3, it0.FornNome)
		set(4, it0.FornCNPJ)
		set(5, it0.FornUF)
		set(6, len(g.Itens))
		set(7, vProd)
		set(8, icmsRet)
		vc, _ := excelize.CoordinatesToCellName(7, er)
		rc, _ := excelize.CoordinatesToCellName(8, er)
		f.SetCellStyle(sheet, vc, rc, moneyStyle)
		er++
	}

	_ = f.SetColWidth(sheet, "A", "A", 14)
	_ = f.SetColWidth(sheet, "B", "B", 46)
	_ = f.SetColWidth(sheet, "C", "C", 36)
	_ = f.SetColWidth(sheet, "D", "E", 16)

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// IcmsFronteiraSTItensXLSXHandler — GET /api/icms-fronteira/st-itens/exportar/xlsx
func IcmsFronteiraSTItensXLSXHandler(db *sql.DB) http.HandlerFunc {
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
		periodo := strings.TrimSpace(r.URL.Query().Get("periodo"))
		uf := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("uf")))

		rows, err := fetchSTItens(db, companyID, periodo, uf)
		if err != nil {
			log.Printf("IcmsFronteiraSTItensXLSX error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao gerar Excel de ST por item")
			return
		}
		chaves := make([]string, len(rows))
		for i, rw := range rows {
			chaves[i] = rw.ChaveNFe
		}
		cteLinks := fetchCteLinksForNFs(db, companyID, chaves)

		data, err := buildSTItensXLSX(rows, cteLinks)
		if err != nil {
			log.Printf("IcmsFronteiraSTItensXLSX write error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao gerar arquivo XLSX")
			return
		}
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", `attachment; filename="st-por-item.xlsx"`)
		w.Write(data)
	}
}

// buildSTItensXLSX gera o arquivo XLSX do demonstrativo de ST por item a partir
// dos grupos (já agrupados por nota) e dos links de CT-e por chave. Função PURA
// — não depende de banco de dados nem de HTTP.
func buildSTItensXLSX(rows []STItemRow, cteLinks map[string][]CteLink) ([]byte, error) {
	blocos := groupSTItensByBloco(rows)
	f := excelize.NewFile()
	sheet := "ST por item"
	f.SetSheetName("Sheet1", sheet)

	moneyFmt := `"R$" #,##0.00`
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"4472C4"}},
	})
	boldStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	subStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"E2E8F0"}},
	})
	totalStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"CBD5E1"}},
	})
	blocoStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"CBD5E1"}},
	})
	moneyStyle, _ := f.NewStyle(&excelize.Style{CustomNumFmt: &moneyFmt})
	cteStyle, _ := f.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"DCE6F1"}}})

	// Header
	letters := make([]string, len(stItensColHeaders))
	for i := range stItensColHeaders {
		c, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, c, stItensColHeaders[i])
		f.SetCellStyle(sheet, c, c, headerStyle)
		col, _, _ := excelize.SplitCellName(c)
		letters[i] = col
	}
	// money columns: V.Produto(10) V.IPI(11) Demais(12) V.Operação(13)
	// ICMS Debitado(18) Base Cálc(19) BC Reduz(21) ICMS Calc(22) ICMS Retido(23) ICMS a Pagar(24)
	moneyCols := []int{10, 11, 12, 13, 18, 19, 21, 22, 23, 24}
	pct := func(v float64) string { return fmt.Sprintf("%.2f%%", v) }

	er := 2
	var totalGeralAPagar float64
	var totalGrupos int
	for _, bl := range blocos {
		// Cabeçalho de seção do bloco.
		hc, _ := excelize.CoordinatesToCellName(1, er)
		f.SetCellValue(sheet, hc, bl.Label)
		hf, _ := excelize.CoordinatesToCellName(1, er)
		hl, _ := excelize.CoordinatesToCellName(24, er)
		f.SetCellStyle(sheet, hf, hl, blocoStyle)
		er++

		var blVProd, blVIpi, blVOper, blIcmsDeb, blBase, blIcmsCalc, blIcmsRet, blAPagar float64

	for _, g := range bl.Grupos {
		totalGrupos++
		itens := g.Itens
		numero := "—"
		if len(itens) > 0 {
			if n := itens[0].NumeroNFe; n != "" {
				numero = n
			}
		}
		var somaOper float64
		for _, it := range itens {
			somaOper += it.VOperacao
		}

		// 1. Linhas de produto.
		for _, it := range itens {
			set := func(col int, v interface{}) {
				c, _ := excelize.CoordinatesToCellName(col, er)
				f.SetCellValue(sheet, c, v)
			}
			set(1, it.CFOP)
			set(2, it.NumeroNFe+" · "+fmtDateBRGo(it.DataEmissao))
			set(3, it.ChaveNFe)
			set(4, it.FornNome)
			set(5, it.CodProduto)
			set(6, it.Descricao)
			set(7, it.NCM)
			set(8, it.CEST)
			set(9, it.StatusXML)
			set(10, it.VProd)
			set(11, it.VIPI)
			set(12, it.VOutro)
			set(13, it.VOperacao)
			if it.TemRegra {
				set(14, pct(it.MVAOriginal))
				set(15, pct(it.MVAAjustado))
			}
			set(16, pct(it.AliqInter))
			set(17, pct(it.AliqInterna))
			set(18, it.IcmsDebitado)
			set(19, it.BaseCalculo)
			set(20, pct(it.ReducaoBC))
			set(21, it.BCReduzida)
			set(22, it.IcmsCalculado)
			set(23, it.IcmsRetido)
			set(24, it.IcmsAPagar)
			for _, mc := range moneyCols {
				c, _ := excelize.CoordinatesToCellName(mc, er)
				f.SetCellStyle(sheet, c, c, moneyStyle)
			}
			er++
		}

		// 2. Subtotal Produtos.
		var subVProd, subVIpi, subVOper, subIcmsDeb, subBase, subIcmsCalc, subIcmsRet, subAPagar float64
		for _, it := range itens {
			subVProd += it.VProd
			subVIpi += it.VIPI
			subVOper += it.VOperacao
			subIcmsDeb += it.IcmsDebitado
			subBase += it.BaseCalculo
			subIcmsCalc += it.IcmsCalculado
			subIcmsRet += it.IcmsRetido
			subAPagar += it.IcmsAPagar
		}
		setSub := func(col int, v interface{}) {
			c, _ := excelize.CoordinatesToCellName(col, er)
			f.SetCellValue(sheet, c, v)
		}
		setSub(1, fmt.Sprintf("Subtotal Produtos NF %s:", numero))
		setSub(10, subVProd)
		setSub(11, subVIpi)
		setSub(13, subVOper)
		setSub(18, subIcmsDeb)
		setSub(19, subBase)
		setSub(22, subIcmsCalc)
		setSub(23, subIcmsRet)
		setSub(24, subAPagar)
		firstC, _ := excelize.CoordinatesToCellName(1, er)
		lastC, _ := excelize.CoordinatesToCellName(24, er)
		f.SetCellStyle(sheet, firstC, lastC, subStyle)
		er++

		// 3. Linhas de CT-e (frete rateado por item).
		ctes := cteLinks[g.Chave]
		var cteFreteTotal, cteIcmsTotal float64
		for _, cte := range ctes {
			for _, it := range itens {
				fracao, icmsCteR := 0.0, 0.0
				if somaOper > 0 {
					fracao = cte.VPrest * it.VOperacao / somaOper
					icmsCteR = cte.VIcmsCTe * it.VOperacao / somaOper
				}
				aplica := it.TemRegra && it.SegmentoOK
				base, calc, aPagar := cteSTItem(fracao, it.MVAAjustado, it.AliqInterna, icmsCteR, aplica)
				cteFreteTotal += fracao
				cteIcmsTotal += aPagar
				set := func(col int, v interface{}) {
					c, _ := excelize.CoordinatesToCellName(col, er)
					f.SetCellValue(sheet, c, v)
				}
				set(1, "CTE")
				set(2, "CT-e "+cte.NumeroCTe+" · "+fmtDateBRGo(cte.DataEmissao))
				set(3, cte.ChaveCTe)
				set(4, cte.EmitNome)
				set(5, it.CodProduto)
				set(6, fmt.Sprintf("Rateio CT-e %s s/ Prod. %s", cte.NumeroCTe, it.CodProduto))
				set(7, it.NCM)
				set(8, it.CEST)
				set(13, fracao)
				if aplica {
					set(15, pct(it.MVAAjustado))
					set(19, base)
					set(22, calc)
				}
				set(17, pct(it.AliqInterna))
				set(18, icmsCteR)
				set(24, aPagar)
				firstC, _ := excelize.CoordinatesToCellName(1, er)
				lastC, _ := excelize.CoordinatesToCellName(24, er)
				f.SetCellStyle(sheet, firstC, lastC, cteStyle)
				er++
			}
		}

		// 4. Subtotal CT-es Vinculados (só quando há CT-e).
		if len(ctes) > 0 {
			setSC := func(col int, v interface{}) {
				c, _ := excelize.CoordinatesToCellName(col, er)
				f.SetCellValue(sheet, c, v)
			}
			setSC(1, "Subtotal CT-es Vinculados:")
			setSC(13, cteFreteTotal)
			setSC(24, cteIcmsTotal)
			firstC, _ := excelize.CoordinatesToCellName(1, er)
			lastC, _ := excelize.CoordinatesToCellName(24, er)
			f.SetCellStyle(sheet, firstC, lastC, subStyle)
			er++
		}

		// 5. TOTAL GERAL NF (produtos + CT-es).
		setT := func(col int, v interface{}) {
			c, _ := excelize.CoordinatesToCellName(col, er)
			f.SetCellValue(sheet, c, v)
		}
		setT(1, fmt.Sprintf("TOTAL GERAL NF: %s", numero))
		setT(10, subVProd)
		setT(11, subVIpi)
		setT(13, subVOper+cteFreteTotal)
		setT(18, subIcmsDeb)
		setT(19, subBase)
		setT(22, subIcmsCalc)
		setT(23, subIcmsRet)
		setT(24, subAPagar+cteIcmsTotal)
		firstC, _ = excelize.CoordinatesToCellName(1, er)
		lastC, _ = excelize.CoordinatesToCellName(24, er)
		f.SetCellStyle(sheet, firstC, lastC, totalStyle)
		er++

		totalGeralAPagar += subAPagar + cteIcmsTotal

		blVProd += subVProd
		blVIpi += subVIpi
		blVOper += subVOper + cteFreteTotal
		blIcmsDeb += subIcmsDeb
		blBase += subBase
		blIcmsCalc += subIcmsCalc
		blIcmsRet += subIcmsRet
		blAPagar += subAPagar + cteIcmsTotal
	}

		// Subtotal do bloco (todos os itens + CT-es do bloco).
		setBl := func(col int, v interface{}) {
			c, _ := excelize.CoordinatesToCellName(col, er)
			f.SetCellValue(sheet, c, v)
		}
		setBl(1, fmt.Sprintf("Subtotal Bloco %s", blocoLetraST(bl.Label)))
		setBl(10, blVProd)
		setBl(11, blVIpi)
		setBl(13, blVOper)
		setBl(18, blIcmsDeb)
		setBl(19, blBase)
		setBl(22, blIcmsCalc)
		setBl(23, blIcmsRet)
		setBl(24, blAPagar)
		blF, _ := excelize.CoordinatesToCellName(1, er)
		blL, _ := excelize.CoordinatesToCellName(24, er)
		f.SetCellStyle(sheet, blF, blL, blocoStyle)
		er++
	}

	// Rodapé — total geral de ICMS a pagar.
	setF := func(col int, v interface{}) {
		c, _ := excelize.CoordinatesToCellName(col, er)
		f.SetCellValue(sheet, c, v)
	}
	notaLbl := fmt.Sprintf("Total Geral ICMS a Pagar (%d nota", totalGrupos)
	if totalGrupos != 1 {
		notaLbl += "s"
	}
	notaLbl += ")"
	setF(1, notaLbl)
	setF(24, totalGeralAPagar)
	firstC, _ := excelize.CoordinatesToCellName(1, er)
	lastC, _ := excelize.CoordinatesToCellName(24, er)
	f.SetCellStyle(sheet, firstC, lastC, boldStyle)

	// Bloco D — SPED sem XML (pendências). Informativo: repete notas de A/B sem
	// XML; NÃO soma no total geral. Lista o que falta importar.
	if dnotas := stBlocoDFaltantes(rows); len(dnotas) > 0 {
		er += 2 // linha em branco + cabeçalho de seção
		dPlural := ""
		if len(dnotas) != 1 {
			dPlural = "s"
		}
		hc, _ := excelize.CoordinatesToCellName(1, er)
		f.SetCellValue(sheet, hc, fmt.Sprintf("Bloco D — SPED sem XML · %d nota%s (importe o XML para capturar o ICMS-ST retido · não somado no total geral)", len(dnotas), dPlural))
		hf, _ := excelize.CoordinatesToCellName(1, er)
		hl, _ := excelize.CoordinatesToCellName(24, er)
		f.SetCellStyle(sheet, hf, hl, blocoStyle)
		er++
		for _, g := range dnotas {
			for _, it := range g.Itens {
				setD := func(col int, v interface{}) {
					c, _ := excelize.CoordinatesToCellName(col, er)
					f.SetCellValue(sheet, c, v)
				}
				setD(1, it.CFOP)
				setD(2, it.NumeroNFe)
				setD(3, g.Chave)
				setD(4, it.FornNome)
				setD(5, it.CodProduto)
				setD(6, it.Descricao)
				setD(7, it.NCM)
				setD(8, it.CEST)
				setD(9, "Faltante")
				setD(10, it.VProd)
				setD(11, it.VIPI)
				setD(13, it.VOperacao)
				setD(18, it.IcmsDebitado)
				setD(23, it.IcmsRetido)
				er++
			}
		}
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ---------------------------------------------------------------------------
// PDF / HTML
// ---------------------------------------------------------------------------

// IcmsFronteiraSTItensHTMLHandler — GET /api/icms-fronteira/st-itens/exportar/pdf
// Gera HTML imprimível com a mesma estrutura agrupada da tela. O front abre via
// window.open e o usuário imprime/salva como PDF.
func IcmsFronteiraSTItensHTMLHandler(db *sql.DB) http.HandlerFunc {
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
		periodo := strings.TrimSpace(r.URL.Query().Get("periodo"))
		uf := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("uf")))

		rows, err := fetchSTItens(db, companyID, periodo, uf)
		if err != nil {
			log.Printf("IcmsFronteiraSTItensHTML error: %v", err)
			http.Error(w, "Erro ao consultar dados", http.StatusInternalServerError)
			return
		}
		chaves := make([]string, len(rows))
		for i, rw := range rows {
			chaves[i] = rw.ChaveNFe
		}
		cteLinks := fetchCteLinksForNFs(db, companyID, chaves)

		var companyName, groupName string
		_ = db.QueryRow(`SELECT COALESCE(NULLIF(c.trade_name,''), c.name, ''), COALESCE(eg.name,'')
			FROM companies c LEFT JOIN enterprise_groups eg ON c.group_id = eg.id
			WHERE c.id = $1::uuid`, companyID).Scan(&companyName, &groupName)
		today := time.Now().Format("02/01/2006")

		var sb strings.Builder
		sb.WriteString(`<!DOCTYPE html><html lang="pt-BR"><head><meta charset="UTF-8">`)
		sb.WriteString(fmt.Sprintf(`<title>ICMS-ST por Item — %s</title>`, htmlEscape(periodo)))
		sb.WriteString(antecipacaoReportCSS)
		sb.WriteString(`</head><body>`)

		sb.WriteString(`<div class="rpt-header"><div class="rpt-head-txt">`)
		if groupName != "" {
			sb.WriteString(fmt.Sprintf(`<div class="rpt-grupo">%s</div>`, htmlEscape(groupName)))
		}
		sb.WriteString(`<div class="rpt-title">Demonstrativo de ICMS-ST por Item</div>`)
		sb.WriteString(fmt.Sprintf(`<div class="rpt-meta">Período: %s &nbsp;|&nbsp; UF: %s &nbsp;|&nbsp; Empresa: %s &nbsp;|&nbsp; Emissão: %s</div>`,
			htmlEscape(periodo), htmlEscape(uf), htmlEscape(companyName), today))
		sb.WriteString(`</div></div>`)

		sb.WriteString(buildSTItensHTML(rows, cteLinks))

		sb.WriteString(`<script>window.onload=function(){window.print()}</script>`)
		sb.WriteString(`</body></html>`)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(sb.String()))
	}
}

// buildSTItensHTML gera o corpo HTML (mensagem de vazio + tabela completa) do
// demonstrativo de ST por item a partir dos grupos e dos links de CT-e por
// chave. Função PURA — não depende de banco de dados nem de HTTP.
func buildSTItensHTML(rows []STItemRow, cteLinks map[string][]CteLink) string {
	blocos := groupSTItensByBloco(rows)
	pct := func(v float64) string { return fmt.Sprintf("%.2f%%", v) }
	td := func(v string) string { return "<td>" + htmlEscape(v) + "</td>" }
	tdR := func(v float64) string { return fmt.Sprintf(`<td style="text-align:right">%s</td>`, brl(v)) }
	tdRs := func(s string) string { return fmt.Sprintf(`<td style="text-align:right">%s</td>`, htmlEscape(s)) }
	empty := func(n int) string { return strings.Repeat("<td></td>", n) }

	var sb strings.Builder

	if len(blocos) == 0 {
		sb.WriteString(`<div class="empty">Nenhum item de ST encontrado para o período.</div>`)
	}

	sb.WriteString(`<table class="nf-tbl"><thead><tr>`)
	for _, h := range stItensColHeaders {
		sb.WriteString(fmt.Sprintf(`<th>%s</th>`, h))
	}
	sb.WriteString(`</tr></thead><tbody>`)

	var totalGeralAPagar float64
	var totalGrupos int
	for _, bl := range blocos {
		// Cabeçalho de seção do bloco.
		sb.WriteString(`<tr class="tot-row" style="background:#cbd5e1!important">`)
		sb.WriteString(fmt.Sprintf(`<td colspan="24"><strong>%s</strong></td>`, htmlEscape(bl.Label)))
		sb.WriteString(`</tr>`)

		var blVProd, blVIpi, blVOper, blIcmsDeb, blBase, blIcmsCalc, blIcmsRet, blAPagar float64

	for _, g := range bl.Grupos {
		totalGrupos++
		itens := g.Itens
		numero := "—"
		if len(itens) > 0 && itens[0].NumeroNFe != "" {
			numero = itens[0].NumeroNFe
		}
		var somaOper float64
		for _, it := range itens {
			somaOper += it.VOperacao
		}

		// 1. Produtos.
		for _, it := range itens {
			sb.WriteString(`<tr>`)
			sb.WriteString(td(it.CFOP) + td(it.NumeroNFe+" · "+fmtDateBRGo(it.DataEmissao)) + td(it.ChaveNFe) + td(it.FornNome))
			sb.WriteString(td(it.CodProduto) + td(it.Descricao) + td(it.NCM) + td(it.CEST) + td(it.StatusXML))
			sb.WriteString(tdR(it.VProd) + tdR(it.VIPI) + tdR(it.VOutro) + tdR(it.VOperacao))
			if it.TemRegra {
				sb.WriteString(tdRs(pct(it.MVAOriginal)) + tdRs(pct(it.MVAAjustado)))
			} else {
				sb.WriteString(tdRs("—") + tdRs("—"))
			}
			sb.WriteString(tdRs(pct(it.AliqInter)) + tdRs(pct(it.AliqInterna)))
			sb.WriteString(tdR(it.IcmsDebitado) + tdR(it.BaseCalculo) + tdRs(pct(it.ReducaoBC)))
			sb.WriteString(tdR(it.BCReduzida) + tdR(it.IcmsCalculado) + tdR(it.IcmsRetido) + tdR(it.IcmsAPagar))
			sb.WriteString(`</tr>`)
		}

		// 2. Subtotal Produtos.
		var subVProd, subVIpi, subVOper, subIcmsDeb, subBase, subIcmsCalc, subIcmsRet, subAPagar float64
		for _, it := range itens {
			subVProd += it.VProd
			subVIpi += it.VIPI
			subVOper += it.VOperacao
			subIcmsDeb += it.IcmsDebitado
			subBase += it.BaseCalculo
			subIcmsCalc += it.IcmsCalculado
			subIcmsRet += it.IcmsRetido
			subAPagar += it.IcmsAPagar
		}
		sb.WriteString(`<tr class="tot-row">`)
		sb.WriteString(fmt.Sprintf(`<td colspan="9">Subtotal Produtos NF %s:</td>`, htmlEscape(numero)))
		sb.WriteString(tdR(subVProd) + tdR(subVIpi) + `<td></td>` + tdR(subVOper))
		sb.WriteString(empty(4))
		sb.WriteString(tdR(subIcmsDeb) + tdR(subBase) + `<td></td><td></td>`)
		sb.WriteString(tdR(subIcmsCalc) + tdR(subIcmsRet) + tdR(subAPagar))
		sb.WriteString(`</tr>`)

		// 3. CT-es rateados.
		ctes := cteLinks[g.Chave]
		var cteFreteTotal, cteIcmsTotal float64
		for _, cte := range ctes {
			for _, it := range itens {
				fracao, icmsCteR := 0.0, 0.0
				if somaOper > 0 {
					fracao = cte.VPrest * it.VOperacao / somaOper
					icmsCteR = cte.VIcmsCTe * it.VOperacao / somaOper
				}
				aplica := it.TemRegra && it.SegmentoOK
				base, calc, aPagar := cteSTItem(fracao, it.MVAAjustado, it.AliqInterna, icmsCteR, aplica)
				cteFreteTotal += fracao
				cteIcmsTotal += aPagar
				mvaCell, baseCell, calcCell := tdRs("—"), tdRs("—"), tdRs("—")
				if aplica {
					mvaCell = tdRs(pct(it.MVAAjustado))
					baseCell = tdR(base)
					calcCell = tdR(calc)
				}
				sb.WriteString(`<tr style="background:#dce6f1">`)
				sb.WriteString(td("CTE") + td("CT-e "+cte.NumeroCTe+" · "+fmtDateBRGo(cte.DataEmissao)) + td(cte.ChaveCTe) + td(cte.EmitNome))
				sb.WriteString(td(it.CodProduto) + td(fmt.Sprintf("Rateio CT-e %s s/ Prod. %s", cte.NumeroCTe, it.CodProduto)))
				sb.WriteString(td(it.NCM) + td(it.CEST) + td("—"))
				sb.WriteString(`<td style="text-align:right">—</td><td style="text-align:right">—</td><td style="text-align:right">—</td>`)
				sb.WriteString(tdR(fracao))                  // 13 V.Operação (frete)
				sb.WriteString(tdRs("—") + mvaCell + tdRs("—")) // 14 MVA orig · 15 MVA aj · 16 alíq inter
				sb.WriteString(tdRs(pct(it.AliqInterna)))    // 17 alíq interna
				sb.WriteString(tdR(icmsCteR) + baseCell)     // 18 ICMS deb (CT-e) · 19 base
				sb.WriteString(`<td style="text-align:right">—</td><td style="text-align:right">—</td>`) // 20 red BC · 21 BC reduz
				sb.WriteString(calcCell)                     // 22 ICMS calc
				sb.WriteString(`<td style="text-align:right">—</td>`) // 23 ICMS retido
				sb.WriteString(tdR(aPagar))                  // 24 ICMS a pagar
				sb.WriteString(`</tr>`)
			}
		}

		// 4. Subtotal CT-es.
		if len(ctes) > 0 {
			sb.WriteString(`<tr class="tot-row" style="background:#dce6f1!important">`)
			sb.WriteString(`<td colspan="12">Subtotal CT-es Vinculados:</td>`)
			sb.WriteString(tdR(cteFreteTotal))
			sb.WriteString(empty(10))
			sb.WriteString(tdR(cteIcmsTotal))
			sb.WriteString(`</tr>`)
		}

		// 5. TOTAL GERAL NF.
		sb.WriteString(`<tr class="tot-row">`)
		sb.WriteString(fmt.Sprintf(`<td colspan="9">TOTAL GERAL NF: %s</td>`, htmlEscape(numero)))
		sb.WriteString(tdR(subVProd) + tdR(subVIpi) + `<td></td>` + tdR(subVOper+cteFreteTotal))
		sb.WriteString(empty(4))
		sb.WriteString(tdR(subIcmsDeb) + tdR(subBase) + `<td></td><td></td>`)
		sb.WriteString(tdR(subIcmsCalc) + tdR(subIcmsRet) + tdR(subAPagar+cteIcmsTotal))
		sb.WriteString(`</tr>`)

		totalGeralAPagar += subAPagar + cteIcmsTotal

		blVProd += subVProd
		blVIpi += subVIpi
		blVOper += subVOper + cteFreteTotal
		blIcmsDeb += subIcmsDeb
		blBase += subBase
		blIcmsCalc += subIcmsCalc
		blIcmsRet += subIcmsRet
		blAPagar += subAPagar + cteIcmsTotal
	}

		// Subtotal do bloco.
		sb.WriteString(`<tr class="tot-row" style="background:#cbd5e1!important">`)
		sb.WriteString(fmt.Sprintf(`<td colspan="9"><strong>Subtotal Bloco %s</strong></td>`, htmlEscape(blocoLetraST(bl.Label))))
		sb.WriteString(tdR(blVProd) + tdR(blVIpi) + `<td></td>` + tdR(blVOper))
		sb.WriteString(empty(4))
		sb.WriteString(tdR(blIcmsDeb) + tdR(blBase) + `<td></td><td></td>`)
		sb.WriteString(tdR(blIcmsCalc) + tdR(blIcmsRet) + tdR(blAPagar))
		sb.WriteString(`</tr>`)
	}

	// Bloco D — SPED sem XML (pendências). Informativo: repete notas de A/B sem
	// XML; NÃO soma no total geral.
	if dnotas := stBlocoDFaltantes(rows); len(dnotas) > 0 {
		dPlural := "s"
		if len(dnotas) == 1 {
			dPlural = ""
		}
		sb.WriteString(`<tr class="tot-row" style="background:#fde68a!important">`)
		sb.WriteString(fmt.Sprintf(`<td colspan="24"><strong>Bloco D — SPED sem XML · %d nota%s</strong> (importe o XML para capturar o ICMS-ST retido · não somado no total geral)</td>`, len(dnotas), dPlural))
		sb.WriteString(`</tr>`)
		for _, g := range dnotas {
			for _, it := range g.Itens {
				sb.WriteString(`<tr style="background:#fffbeb">`)
				sb.WriteString(td(it.CFOP) + td(it.NumeroNFe) + td(g.Chave) + td(it.FornNome))
				sb.WriteString(td(it.CodProduto) + td(it.Descricao) + td(it.NCM) + td(it.CEST))
				sb.WriteString(td("Faltante"))
				sb.WriteString(tdR(it.VProd) + tdR(it.VIPI) + `<td></td>` + tdR(it.VOperacao))
				sb.WriteString(empty(4))
				sb.WriteString(tdR(it.IcmsDebitado))
				sb.WriteString(`<td></td><td></td><td></td><td></td>`)
				sb.WriteString(tdR(it.IcmsRetido))
				sb.WriteString(`<td></td>`)
				sb.WriteString(`</tr>`)
			}
		}
	}
	sb.WriteString(`</tbody>`)

	if totalGrupos > 0 {
		notaS := "s"
		if totalGrupos == 1 {
			notaS = ""
		}
		sb.WriteString(`<tfoot><tr class="tot-row">`)
		sb.WriteString(fmt.Sprintf(`<td colspan="23" style="text-align:right">Total Geral ICMS a Pagar (%d nota%s)</td>`, totalGrupos, notaS))
		sb.WriteString(tdR(totalGeralAPagar))
		sb.WriteString(`</tr></tfoot>`)
	}
	sb.WriteString(`</table>`)

	return sb.String()
}
