// fiscal_diagnostico_pdf.go — GET /api/fiscal/diagnostico/pdf
//
// Versão imprimível do Relatório Diagnóstico (mesmo padrão dos PDFs do
// Fronteira: HTML com CSS de impressão + window.print() no load — o
// navegador salva como PDF). Aberto via window.open com ?token= (o
// AuthMiddleware aceita token por query) e ?company_id= (validado pelo
// GetEffectiveCompanyID — admin acessa qualquer empresa, user só as suas).
package handlers

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func fmtBRLPdf(v float64) string {
	s := fmt.Sprintf("%.2f", v)
	s = strings.ReplaceAll(s, ".", ",")
	// separador de milhar simples
	parts := strings.SplitN(s, ",", 2)
	intPart := parts[0]
	for i := len(intPart) - 3; i > 0; i -= 3 {
		intPart = intPart[:i] + "." + intPart[i:]
	}
	return "R$ " + intPart + "," + parts[1]
}

// FiscalDiagnosticoPDFHandler — GET /api/fiscal/diagnostico/pdf
func FiscalDiagnosticoPDFHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		// company via query (window.open não envia headers) com fallback header
		reqCompany := r.Header.Get("X-Company-ID")
		if q := strings.TrimSpace(r.URL.Query().Get("company_id")); q != "" {
			reqCompany = q
		}
		companyID, err := GetEffectiveCompanyID(db, userID, reqCompany)
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa")
			return
		}

		dataInicio := strings.TrimSpace(r.URL.Query().Get("data_inicio"))
		dataFim := strings.TrimSpace(r.URL.Query().Get("data_fim"))
		filial := strings.TrimSpace(r.URL.Query().Get("filial"))
		ufOrigem := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("uf_origem")))

		diag, err := montarFiscalDiagnostico(db, companyID, dataInicio, dataFim, filial, ufOrigem)
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao montar diagnóstico")
			return
		}

		var companyName string
		_ = db.QueryRow(`SELECT COALESCE(NULLIF(trade_name,''), name, '') FROM companies WHERE id = $1`, companyID).Scan(&companyName)

		// Logo da empresa (mesmo helper dos demais relatórios): embutida como
		// data URI base64 — o CSP dos PDFs não permite host externo.
		logoTag := ""
		if logoData, logoMime := loadEmpresaLogo(db, companyID, ""); len(logoData) > 0 {
			if logoMime == "" {
				logoMime = "image/png"
			}
			logoTag = fmt.Sprintf(`<img class="hdr-logo" src="data:%s;base64,%s" alt="logo">`,
				logoMime, base64.StdEncoding.EncodeToString(logoData))
		}

		filialLabel := "Todas"
		if filial != "" {
			filialLabel = filial
			for _, f := range diag.Filiais {
				if f.CNPJ == filial && f.Nome != "" {
					filialLabel = fmt.Sprintf("%s (%s)", f.Nome, filial)
					break
				}
			}
		}
		ufLabel := ufOrigem
		if ufLabel == "" {
			ufLabel = "Todas"
		}
		periodo := "Todo o histórico executado"
		if dataInicio != "" || dataFim != "" {
			periodo = fmt.Sprintf("%s a %s", orDash(dataInicio), orDash(dataFim))
		}

		esc := html.EscapeString
		var b strings.Builder
		b.WriteString(`<!DOCTYPE html><html lang="pt-BR"><head><meta charset="UTF-8">`)
		b.WriteString(`<title>Relatório Diagnóstico — Teste Pacote Fiscal</title><style>
* { box-sizing: border-box; font-family: -apple-system, 'Segoe UI', Roboto, Arial, sans-serif; }
body { margin: 24px; color: #1e293b; font-size: 12px; }
h1 { font-size: 18px; margin: 0 0 2px; } h2 { font-size: 13px; margin: 18px 0 6px; border-bottom: 2px solid #cbd5e1; padding-bottom: 3px; }
.hdr { display: flex; align-items: center; gap: 16px; border-bottom: 2px solid #cbd5e1; padding-bottom: 10px; margin-bottom: 12px; }
.hdr-logo { max-height: 56px; max-width: 220px; object-fit: contain; }
.hdr-txt { min-width: 0; }
.meta { color: #64748b; font-size: 11px; margin-bottom: 14px; }
.cards { display: flex; gap: 8px; flex-wrap: wrap; margin: 10px 0; }
.card { border: 1px solid #e2e8f0; border-radius: 6px; padding: 8px 12px; min-width: 130px; }
.card .t { font-size: 10px; color: #64748b; text-transform: uppercase; } .card .v { font-size: 15px; font-weight: 700; }
table { border-collapse: collapse; width: 100%; margin: 6px 0; }
th, td { border: 1px solid #e2e8f0; padding: 4px 6px; font-size: 11px; text-align: right; }
th { background: #f1f5f9; } td:first-child, th:first-child { text-align: left; }
.chips span { display: inline-block; border: 1px solid; border-radius: 10px; padding: 2px 8px; margin: 2px 4px 2px 0; font-size: 11px; }
.ok { color: #047857; border-color: #a7f3d0; background: #ecfdf5; } .bad { color: #b91c1c; border-color: #fecaca; background: #fef2f2; }
.red { color: #b91c1c; font-weight: 700; }
.rodape { margin-top: 18px; color: #94a3b8; font-size: 10px; border-top: 1px solid #e2e8f0; padding-top: 6px; }
@media print { body { margin: 8mm; } .no-print { display: none; } }
</style></head><body>`)

		b.WriteString(`<div class="hdr">` + logoTag + `<div class="hdr-txt">`)
		b.WriteString(`<h1>Relatório Diagnóstico — Teste Pacote Fiscal (PKG_FISCAL_FCTAX)</h1>`)
		b.WriteString(fmt.Sprintf(`<div class="meta">Empresa: <b>%s</b> &nbsp;|&nbsp; Período (emissão): <b>%s</b> &nbsp;|&nbsp; Filial: <b>%s</b> &nbsp;|&nbsp; UF Origem: <b>%s</b> &nbsp;|&nbsp; Gerado em %s</div>`,
			esc(companyName), esc(periodo), esc(filialLabel), esc(ufLabel), time.Now().Format("02/01/2006 15:04")))
		b.WriteString(`</div></div>`)

		// Cards
		pctOK := 0.0
		if diag.ItensExecutados > 0 {
			pctOK = float64(diag.ItensOK) / float64(diag.ItensExecutados) * 100
		}
		b.WriteString(`<div class="cards">`)
		card := func(t, v string) {
			b.WriteString(fmt.Sprintf(`<div class="card"><div class="t">%s</div><div class="v">%s</div></div>`, esc(t), esc(v)))
		}
		card("Período executado", fmt.Sprintf("%s a %s", orDash(diag.PeriodoInicio), orDash(diag.PeriodoFim)))
		card("Notas executadas", fmt.Sprintf("%d", diag.NotasExecutadas))
		card("Itens executados", fmt.Sprintf("%d", diag.ItensExecutados))
		card("Itens OK", fmt.Sprintf("%d (%.1f%%)", diag.ItensOK, pctOK))
		card("Sem grupo / Erro", fmt.Sprintf("%d / %d", diag.ItensSemGrupo, diag.ItensErro))
		card("Com simulação IBS/CBS", fmt.Sprintf("%d", diag.ComSimulacao))
		card("Valor produtos", fmtBRLPdf(diag.VProdTotal))
		b.WriteString(`</div>`)

		// Clientes por categoria
		b.WriteString(`<h2>Clientes (destinatários distintos por CNPJ/CPF)</h2><div class="cards">`)
		card("Identificados", fmt.Sprintf("%d", diag.Clientes.Identificados))
		card("Contribuintes", fmt.Sprintf("%d", diag.Clientes.Contribuintes))
		card("Não contribuintes", fmt.Sprintf("%d", diag.Clientes.NaoContribuintes))
		card("Com DIFAL", fmt.Sprintf("%d", diag.Clientes.ComDifal))
		card("Com FCP", fmt.Sprintf("%d", diag.Clientes.ComFcp))
		card("Com ST", fmt.Sprintf("%d", diag.Clientes.ComSt))
		card("Com base reduzida", fmt.Sprintf("%d", diag.Clientes.ComReducao))
		card("Notas sem destinatário", fmt.Sprintf("%d", diag.Clientes.SemIdentificacao))
		b.WriteString(`</div>`)
		b.WriteString(`<div class="meta">Contribuinte/não contribuinte = pTipoContribuinte efetivamente passado ao pacote (indIEDest &gt; CFOP 6107/6108 &gt; modelo); DIFAL/FCP/ST = totais do cabeçalho do XML; base reduzida = itens CST 20/70. Um cliente pode contar em mais de uma categoria.</div>`)

		// Divergências por tributo
		b.WriteString(`<h2>Itens divergentes por tributo</h2><div class="chips">`)
		divs := []struct {
			label string
			n     int
		}{{"ICMS", diag.DivIcms}, {"ICMS-ST", diag.DivSt}, {"PIS", diag.DivPis}, {"COFINS", diag.DivCofins}, {"IBS", diag.DivIbs}, {"CBS", diag.DivCbs}}
		for _, d := range divs {
			cls := "ok"
			if d.n > 0 {
				cls = "bad"
			}
			pct := 0.0
			if diag.ItensOK > 0 {
				pct = float64(d.n) / float64(diag.ItensOK) * 100
			}
			b.WriteString(fmt.Sprintf(`<span class="%s">%s: %d (%.1f%%)</span>`, cls, d.label, d.n, pct))
		}
		b.WriteString(`</div>`)

		// Por CFOP
		b.WriteString(`<h2>Por CFOP</h2><table><tr><th>CFOP</th><th>Notas</th><th>Itens</th><th>Valor Produtos</th><th>OK</th><th>S/ Grupo</th><th>Erro</th><th>Div ICMS</th><th>Div ST</th><th>Div PIS</th><th>Div COFINS</th><th>Div IBS</th><th>Div CBS</th></tr>`)
		for _, c := range diag.PorCfop {
			red := func(n int) string {
				if n > 0 {
					return fmt.Sprintf(`<td class="red">%d</td>`, n)
				}
				return fmt.Sprintf(`<td>%d</td>`, n)
			}
			b.WriteString(fmt.Sprintf(`<tr><td>%s</td><td>%d</td><td>%d</td><td>%s</td><td>%d</td><td>%d</td><td>%d</td>%s%s%s%s%s%s</tr>`,
				esc(orDash(c.CFOP)), c.Notas, c.Itens, fmtBRLPdf(c.VProd), c.OK, c.SemGrupo, c.Erro,
				red(c.DivIcms), red(c.DivSt), red(c.DivPis), red(c.DivCofins), red(c.DivIbs), red(c.DivCbs)))
		}
		b.WriteString(`</table>`)

		// Distribuições
		distTable := func(titulo, chave string, rows []diagDistRow, comValor bool) {
			b.WriteString(fmt.Sprintf(`<h2>%s</h2><table><tr><th>%s</th><th>Itens</th>`, esc(titulo), esc(chave)))
			if comValor {
				b.WriteString(`<th>Valor Produtos</th>`)
			}
			b.WriteString(`</tr>`)
			for _, d := range rows {
				b.WriteString(fmt.Sprintf(`<tr><td>%s</td><td>%d</td>`, esc(orDash(d.Chave)), d.Itens))
				if comValor {
					b.WriteString(fmt.Sprintf(`<td>%s</td>`, fmtBRLPdf(d.VProd)))
				}
				b.WriteString(`</tr>`)
			}
			b.WriteString(`</table>`)
		}
		distTable("Situações de ICMS (CST do XML)", "CST ICMS", diag.PorCstIcms, true)
		distTable("Situações de PIS/COFINS (CST do XML)", "CST PIS", diag.PorCstPis, true)
		distTable("Centro fiscal usado na chamada", "pTipoCentroFiscal", diag.PorCentroFiscal, false)
		distTable("Tipo de contribuinte na chamada", "pTipoContribuinte", diag.PorContribuinte, false)

		// Erros
		if len(diag.Erros) > 0 {
			b.WriteString(`<h2>Erros mais frequentes</h2><table><tr><th>Mensagem</th><th>Itens</th></tr>`)
			for _, e := range diag.Erros {
				b.WriteString(fmt.Sprintf(`<tr><td>%s</td><td>%d</td></tr>`, esc(orDash(e.Mensagem)), e.Itens))
			}
			b.WriteString(`</table>`)
		}

		b.WriteString(`<div class="rodape">Divergência com a mesma régua da Comparação Fiscal: itens executados com simulação IBS/CBS comparam contra o esperado ajustado (tolerância de 1 centavo); sem simulação, contra o XML cru (tolerância zero). Percentuais de divergência sobre os itens OK. FBTax Cloud — módulo Teste Pacote Fiscal.</div>`)
		b.WriteString(`<script>window.onload=function(){window.print()}</script>`)
		b.WriteString(`</body></html>`)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(b.String()))
	}
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}
