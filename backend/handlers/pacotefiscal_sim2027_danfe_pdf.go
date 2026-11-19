// pacotefiscal_sim2027_danfe_pdf.go — DANFE (modelo 55, retrato) da Simulação 2027, no leiaute
// de referência da NT 2026.010 v1.00 (DANFE Reforma Tributária, produção em 01/12/2026):
//
//   - bloco novo "TOTAL DO IBS / CBS / IS" logo depois de "TOTAL DO ICMS / IPI";
//   - no emitente: Código do Regime Tributário e a caixa RESERVADA do "Tipo de Regime de
//     Apuração do IBS e da CBS" (sem tag publicada — a NT manda não imprimir conteúdo);
//   - por item: cClassTrib, bases, alíquotas e valores de ICMS, IBS UF, IBS Município, CBS, IS e
//     IPI. Com redução de alíquota (gRed) imprime a alíquota EFETIVA (pAliqEfet), senão a vigente;
//   - campos facultativos (FCP, DIFAL, monofásicos, ISSQN, duplicatas, transporte, QR Code) só
//     aparecem se o XML os traz — a NT não autoriza criar nem inferir informação.
//
// É um documento de SIMULAÇÃO: cada página leva uma marca d'água "SIMULAÇÃO 2027 — SEM VALIDADE
// FISCAL" (o XML gerado também não é válido para transmissão). Os campos azuis/tracejados do
// modelo da NT são apenas a indicação de "facultativo" no desenho de referência e não são
// reproduzidos.
package handlers

import (
	"bytes"
	"fmt"
	"image/color"
	"math"
	"strings"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/boombuler/barcode/qr"
	"github.com/go-pdf/fpdf"
)

// dfeCompressPDF: os testes desligam para poder procurar texto no fluxo do PDF.
var dfeCompressPDF = true

const (
	dfeX0         = 7.0   // margem esquerda
	dfeW          = 196.0 // largura útil (A4 - 2 × 7 mm)
	dfeRowH       = 7.4   // altura padrão de uma linha de campos
	dfePageBottom = 290.0
)

type dfeR struct {
	pdf   *fpdf.Fpdf
	tr    func(string) string
	p     *dfeNfeProc
	inf   *dfeInfNFe
	chave string
	y     float64
	marca bool
}

type dfeKV struct{ label, value string }

// dfeRenderPDF desenha o DANFE da NF-e e devolve os bytes do PDF.
func dfeRenderPDF(proc *dfeNfeProc, marca bool) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetTitle("DANFE - Simulação 2027", true)
	pdf.SetMargins(dfeX0, 7, dfeX0)
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetCellMargin(0)
	pdf.SetCompression(dfeCompressPDF)
	pdf.AliasNbPages("")

	r := &dfeR{
		pdf:   pdf,
		tr:    pdf.UnicodeTranslatorFromDescriptor(""),
		p:     proc,
		inf:   &proc.NFe.InfNFe,
		chave: proc.chave(),
		marca: marca,
	}
	r.newPage(true)
	r.destinatario()
	r.duplicatas()
	r.totaisProdutos()
	r.totaisICMS()
	r.totaisIBSCBS()
	r.issqn()
	r.transportador()
	r.itens()
	r.dadosAdicionais()

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ---------------------------------------------------------------------------
// Primitivas
// ---------------------------------------------------------------------------

func (r *dfeR) font(style string, size float64) { r.pdf.SetFont("Arial", style, size) }

func (r *dfeR) rect(x, y, w, h float64) {
	r.pdf.SetDrawColor(0, 0, 0)
	r.pdf.SetLineWidth(0.2)
	r.pdf.Rect(x, y, w, h, "D")
}

// fit reduz a fonte (até min) e, se ainda não couber, trunca com "...".
func (r *dfeR) fit(s string, w, size, min float64, style string) (string, float64) {
	for sz := size; sz >= min; sz -= 0.5 {
		r.font(style, sz)
		if r.pdf.GetStringWidth(r.tr(s)) <= w {
			return s, sz
		}
	}
	r.font(style, min)
	rs := []rune(s)
	for len(rs) > 1 && r.pdf.GetStringWidth(r.tr(string(rs))+"...") > w {
		rs = rs[:len(rs)-1]
	}
	return string(rs) + "...", min
}

// text escreve uma linha de texto (já ajustada à largura) em (x,y), altura de linha lh.
func (r *dfeR) text(x, y, w, lh float64, s string, size float64, style, align string, gray bool) {
	if s == "" {
		return
	}
	if gray {
		r.pdf.SetTextColor(95, 95, 95)
	} else {
		r.pdf.SetTextColor(0, 0, 0)
	}
	v, sz := r.fit(s, w, size, 4, style)
	r.font(style, sz)
	r.pdf.SetXY(x, y)
	r.pdf.CellFormat(w, lh, r.tr(v), "", 0, align, false, 0, "")
}

// field desenha uma caixa com rótulo miúdo no topo e o valor embaixo (leiaute do DANFE).
func (r *dfeR) field(x, y, w, h float64, label, value, align string) {
	r.rect(x, y, w, h)
	r.text(x+0.7, y+0.4, w-1.4, 2, label, 4.8, "", "L", true)
	r.text(x+0.8, y+2.3, w-1.6, h-2.5, value, 8, "", align, false)
}

// row desenha campos lado a lado (larguras em mm) e avança y.
func (r *dfeR) row(h float64, cells ...dfeCell) {
	x := dfeX0
	for _, c := range cells {
		r.field(x, r.y, c.w, h, c.label, c.value, c.align)
		x += c.w
	}
	r.y += h
}

type dfeCell struct {
	w                   float64
	label, value, align string
}

func cell(w float64, label, value string) dfeCell { return dfeCell{w, label, value, "L"} }
func num(w float64, label, value string) dfeCell  { return dfeCell{w, label, value, "R"} }

// rowKV distribui os pares (rótulo, valor) em linhas com 'cols' colunas iguais.
func (r *dfeR) rowKV(cols int, items []dfeKV, align string) {
	if len(items) == 0 {
		return
	}
	w := dfeW / float64(cols)
	for i := 0; i < len(items); i += cols {
		end := i + cols
		if end > len(items) {
			end = len(items)
		}
		x := dfeX0
		for _, it := range items[i:end] {
			r.field(x, r.y, w, dfeRowH, it.label, it.value, align)
			x += w
		}
		r.y += dfeRowH
	}
}

func (r *dfeR) title(s string) {
	r.text(dfeX0, r.y, dfeW, 3.6, s, 6.5, "B", "L", false)
	r.y += 3.8
}

func (r *dfeR) ensure(h float64, emItens bool) {
	if r.y+h <= dfePageBottom {
		return
	}
	r.newPage(false)
	if emItens {
		r.itensCabecalho()
	}
}

func dfeIsBlack(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return r+g+b < 3*0x8000
}

func (r *dfeR) barcode128(x, y, w, h float64, content string) {
	bc, err := code128.Encode(content)
	if err != nil {
		return
	}
	r.drawBars(bc, x, y, w, h)
}

func (r *dfeR) drawBars(bc barcode.Barcode, x, y, w, h float64) {
	n := bc.Bounds().Dx()
	if n == 0 {
		return
	}
	mw := w / float64(n)
	r.pdf.SetFillColor(0, 0, 0)
	for i := 0; i < n; {
		if !dfeIsBlack(bc.At(i, 0)) {
			i++
			continue
		}
		j := i
		for j < n && dfeIsBlack(bc.At(j, 0)) {
			j++
		}
		r.pdf.Rect(x+float64(i)*mw, y, float64(j-i)*mw, h, "F")
		i = j
	}
}

func (r *dfeR) qrcode(x, y, size float64, content string) {
	bc, err := qr.Encode(content, qr.M, qr.Auto)
	if err != nil {
		return
	}
	n := bc.Bounds().Dx()
	if n == 0 {
		return
	}
	mw := size / float64(n)
	r.pdf.SetFillColor(0, 0, 0)
	for yy := 0; yy < n; yy++ {
		for xx := 0; xx < n; xx++ {
			if dfeIsBlack(bc.At(xx, yy)) {
				r.pdf.Rect(x+float64(xx)*mw, y+float64(yy)*mw, mw+0.02, mw+0.02, "F")
			}
		}
	}
}

func (r *dfeR) wrap(s string, w, size float64, style string) []string {
	r.font(style, size)
	var out []string
	for _, l := range r.pdf.SplitLines([]byte(r.tr(s)), w) {
		out = append(out, string(l))
	}
	return out
}

// ---------------------------------------------------------------------------
// Páginas e blocos
// ---------------------------------------------------------------------------

func (r *dfeR) watermark() {
	r.pdf.SetAlpha(0.13, "Normal")
	r.pdf.SetTextColor(120, 120, 120)
	r.font("B", 26)
	msg := r.tr("SIMULAÇÃO 2027 — SEM VALIDADE FISCAL")
	wTxt := r.pdf.GetStringWidth(msg)
	r.pdf.TransformBegin()
	r.pdf.TransformRotate(50, 105, 148.5) // em torno do centro da página
	r.pdf.Text(105-wTxt/2, 148.5, msg)
	r.pdf.TransformEnd()
	r.pdf.SetAlpha(1, "Normal")
	r.pdf.SetTextColor(0, 0, 0)
}

func (r *dfeR) newPage(first bool) {
	r.pdf.AddPage()
	if r.marca {
		r.watermark()
	}
	r.y = 7
	if first {
		r.canhoto()
	}
	r.cabecalho(first)
}

func (r *dfeR) canhoto() {
	y0 := r.y
	x0 := dfeX0
	r.rect(x0, y0, 164, 8)
	r.text(x0+0.8, y0+0.5, 162.4, 7, "RECEBEMOS DE "+strings.TrimSpace(r.inf.Emit.XNome)+
		" OS PRODUTOS/SERVIÇOS CONSTANTES DA NOTA FISCAL ELETRÔNICA INDICADA AO LADO", 6, "", "L", false)
	r.field(x0, y0+8, 40, 8, "DATA DE RECEBIMENTO", "", "L")
	r.field(x0+40, y0+8, 124, 8, "IDENTIFICAÇÃO E ASSINATURA DO RECEBEDOR", "", "L")
	r.rect(x0+164, y0, 32, 16)
	r.text(x0+164, y0+2, 32, 4, "NF-e", 10, "B", "C", false)
	r.text(x0+164, y0+7.5, 32, 3, "Nº "+dfeNumDoc(r.inf.Ide.NNF), 7, "", "C", false)
	r.text(x0+164, y0+10.8, 32, 3, "SÉRIE "+dfeSerie(r.inf.Ide.Serie), 7, "", "C", false)
	r.y = y0 + 16
	r.pdf.SetDashPattern([]float64{1, 1}, 0)
	r.pdf.SetDrawColor(120, 120, 120)
	r.pdf.Line(x0, r.y+1.6, x0+dfeW, r.y+1.6)
	r.pdf.SetDashPattern([]float64{}, 0)
	r.y += 3.4
}

func (r *dfeR) cabecalho(first bool) {
	y0, x0, h := r.y, dfeX0, 32.0
	e := r.inf.Emit

	// Emitente
	r.rect(x0, y0, 78, h)
	r.text(x0+1, y0+2.5, 76, 4, strings.TrimSpace(e.XNome), 9, "B", "C", false)
	linhas := []string{
		dfeEnderLinha(e.EnderEmit),
		strings.TrimSpace(e.EnderEmit.XBairro),
		strings.TrimSpace(e.EnderEmit.XMun) + "/" + e.EnderEmit.UF + " - CEP " + dfeCEP(e.EnderEmit.CEP),
	}
	if e.EnderEmit.Fone != "" {
		linhas = append(linhas, "Fone: "+dfeFone(e.EnderEmit.Fone))
	}
	ly := y0 + 8.5
	for _, l := range linhas {
		r.text(x0+1, ly, 76, 3, l, 6.8, "", "C", false)
		ly += 3.3
	}

	// DANFE
	xd := x0 + 78
	r.rect(xd, y0, 34, h)
	r.text(xd, y0+1.8, 34, 5, "DANFE", 13, "B", "C", false)
	r.text(xd, y0+7.6, 34, 2.4, "DOCUMENTO AUXILIAR DA", 4.8, "", "C", true)
	r.text(xd, y0+9.8, 34, 2.4, "NOTA FISCAL ELETRÔNICA", 4.8, "", "C", true)
	r.text(xd+2, y0+13.2, 20, 2.6, "0 - ENTRADA", 5.6, "", "L", false)
	r.text(xd+2, y0+16.0, 20, 2.6, "1 - SAÍDA", 5.6, "", "L", false)
	r.rect(xd+23, y0+13, 7.5, 7.5)
	r.text(xd+23, y0+13, 7.5, 7.5, strings.TrimSpace(r.inf.Ide.TpNF), 11, "B", "C", false)
	r.text(xd, y0+22.2, 34, 3, "Nº "+dfeNumDoc(r.inf.Ide.NNF), 7.2, "B", "C", false)
	r.text(xd, y0+25.2, 34, 3, "SÉRIE "+dfeSerie(r.inf.Ide.Serie), 6.4, "", "C", false)
	r.text(xd, y0+28.0, 34, 3, fmt.Sprintf("FOLHA %d/{nb}", r.pdf.PageNo()), 6.4, "", "C", false)

	// Chave de acesso
	xc := xd + 34
	wc := dfeW - 78 - 34
	r.rect(xc, y0, wc, h)
	if len(r.chave) == 44 {
		r.barcode128(xc+3, y0+2, wc-6, 11, r.chave)
	}
	r.text(xc+0.8, y0+14.2, wc-1.6, 2, "CHAVE DE ACESSO", 4.8, "", "L", true)
	r.text(xc+0.8, y0+16.6, wc-1.6, 4, dfeChaveFmt(r.chave), 8.2, "B", "C", false)
	r.text(xc+0.8, y0+22.4, wc-1.6, 2.6, "Consulte a autenticidade no portal nacional da NF-e", 5.4, "", "C", true)
	r.text(xc+0.8, y0+25.0, wc-1.6, 2.6, "www.nfe.fazenda.gov.br/portal ou no site da SEFAZ autorizadora", 5.4, "", "C", true)
	r.y = y0 + h

	if !first {
		return
	}
	prot := ""
	if p := strings.TrimSpace(r.p.Prot.InfProt.NProt); p != "" {
		prot = p
		if d := dfeData(r.p.Prot.InfProt.DhRecbto); d != "" {
			prot += " – " + d
			if hh := dfeHora(r.p.Prot.InfProt.DhRecbto); hh != "" {
				prot += " " + hh
			}
		}
	}
	r.row(dfeRowH, cell(116, "NATUREZA DA OPERAÇÃO", strings.TrimSpace(r.inf.Ide.NatOp)),
		dfeCell{80, "PROTOCOLO DE AUTORIZAÇÃO DE USO", prot, "R"})
	r.row(dfeRowH, cell(62, "INSCRIÇÃO ESTADUAL", e.IE), cell(62, "INSCRIÇÃO ESTADUAL DO SUBSTITUTO TRIBUTÁRIO", e.IEST),
		cell(72, "CNPJ / CPF", dfeCNPJCPF(e.CNPJ, e.CPF)))
	// Código do Regime Tributário + caixa reservada do Tipo de Regime de Apuração do IBS e da CBS
	// (sem tag publicada: a NT manda NÃO imprimir conteúdo).
	r.row(dfeRowH, cell(98, "CÓDIGO DO REGIME TRIBUTÁRIO", dfeCRT(e.CRT)),
		cell(98, "TIPO DE REGIME DE APURAÇÃO DO IBS E DA CBS", ""))
}

func (r *dfeR) destinatario() {
	d := r.inf.Dest
	id := dfeCNPJCPF(d.CNPJ, d.CPF)
	if id == "" {
		id = d.IDEstrangeiro
	}
	e := d.EnderDest
	r.ensure(4+3*dfeRowH, false)
	r.y += 1.2
	r.title("DESTINATÁRIO / REMETENTE")
	r.row(dfeRowH, cell(124, "NOME / RAZÃO SOCIAL", strings.TrimSpace(d.XNome)), cell(40, "CNPJ / CPF", id),
		cell(32, "DATA DE EMISSÃO", dfeData(r.inf.Ide.DhEmi)))
	r.row(dfeRowH, cell(90, "ENDEREÇO", dfeEnderLinha(e)), cell(44, "BAIRRO / DISTRITO", strings.TrimSpace(e.XBairro)),
		cell(30, "CEP", dfeCEP(e.CEP)), cell(32, "DATA DE ENTRADA / SAÍDA", dfeData(r.inf.Ide.DhSaiEn)))
	r.row(dfeRowH, cell(60, "MUNICÍPIO", strings.TrimSpace(e.XMun)), cell(34, "FONE / FAX", dfeFone(e.Fone)),
		cell(12, "UF", e.UF), cell(50, "INSCRIÇÃO ESTADUAL", d.IE), cell(40, "HORA DE SAÍDA", dfeHora(r.inf.Ide.DhSaiEn)))
}

func (r *dfeR) duplicatas() {
	dups := r.inf.Cobr.Dup
	if len(dups) == 0 {
		return
	}
	linhas := (len(dups) + 2) / 3
	r.ensure(4+float64(linhas)*dfeRowH, false)
	r.y += 1.2
	r.title("FATURA / DUPLICATAS")
	for i := 0; i < len(dups); i += 3 {
		x := dfeX0
		for j := i; j < i+3 && j < len(dups); j++ {
			d := dups[j]
			r.field(x, r.y, 22, dfeRowH, "FATURA/DUPLICATA", d.NDup, "L")
			r.field(x+22, r.y, 20, dfeRowH, "VENCIMENTO", dfeData(d.DVenc), "L")
			r.field(x+42, r.y, dfeW/3-42, dfeRowH, "VALOR", dfeMoney(d.VDup), "R")
			x += dfeW / 3
		}
		r.y += dfeRowH
	}
}

func (r *dfeR) totaisProdutos() {
	t := r.inf.Total.ICMSTot
	r.ensure(4+2*dfeRowH, false)
	r.y += 1.2
	r.title("TOTAL DOS PRODUTOS E TOTAL DA NOTA")
	w := dfeW / 5
	r.row(dfeRowH, num(w, "VALOR TOTAL DOS PRODUTOS", dfeMoney(t.VProd)), num(w, "VALOR DO FRETE", dfeMoney(t.VFrete)),
		num(w, "VALOR DO SEGURO", dfeMoney(t.VSeg)), num(w, "DESCONTO", dfeMoney(t.VDesc)),
		num(w, "OUTRAS DESPESAS", dfeMoney(t.VOutro)))
	r.row(dfeRowH, num(w, "VALOR TOTAL DA NOTA", dfeMoney(t.VNF)))
}

func (r *dfeR) totaisICMS() {
	t := r.inf.Total.ICMSTot
	r.ensure(4+dfeRowH, false)
	r.y += 1.2
	r.title("TOTAL DO ICMS / IPI")
	w := dfeW / 5
	r.row(dfeRowH, num(w, "BASE DE CÁLCULO DO ICMS", dfeMoney(t.VBC)), num(w, "VALOR DO ICMS", dfeMoney(t.VICMS)),
		num(w, "BASE DE CÁLCULO DO ICMS ST", dfeMoney(t.VBCST)), num(w, "VALOR DO ICMS ST", dfeMoney(t.VST)),
		num(w, "VALOR DO IPI", dfeMoney(t.VIPI)))
	// facultativos: só os que o XML traz
	var opt []dfeKV
	add := func(label, v string) {
		if strings.TrimSpace(v) != "" {
			opt = append(opt, dfeKV{label, dfeMoney(v)})
		}
	}
	add("VALOR DO FCP", t.VFCP)
	add("VALOR DO FCP RETIDO POR ST", t.VFCPST)
	add("VALOR DO DIFAL NA UF DE DESTINO", t.VICMSUFDest)
	add("VALOR DO FCP NA UF DE DESTINO", t.VFCPUFDest)
	add("BC DO ICMS MONOFÁSICO", t.QBCMono)
	add("VALOR DO ICMS MONOFÁSICO", t.VICMSMono)
	add("BC DO ICMS MONOFÁSICO POR RETENÇÃO", t.QBCMonoReten)
	add("VALOR DO ICMS MONOFÁSICO POR RETENÇÃO", t.VICMSMonoRet)
	r.ensure(float64((len(opt)+3)/4)*dfeRowH, false)
	r.rowKV(4, opt, "R")
}

// totaisIBSCBS — bloco NOVO da NT 2026.010 (4.1): "Total do IBS/CBS/IS".
func (r *dfeR) totaisIBSCBS() {
	t := r.inf.Total
	if t.IBSCBSTot == nil && t.ISTot == nil {
		return
	}
	r.ensure(4+dfeRowH, false)
	r.y += 1.2
	r.title("TOTAL DO IBS / CBS / IS")
	var cbs, ibsUF, ibsMun, is string
	var mono []dfeKV
	if t.IBSCBSTot != nil {
		cbs = dfeMoney(t.IBSCBSTot.GCBS.V)
		ibsUF = dfeMoney(t.IBSCBSTot.GIBS.UF.V)
		ibsMun = dfeMoney(t.IBSCBSTot.GIBS.Mun.V)
		if g := t.IBSCBSTot.GMono; g != nil {
			for _, kv := range []dfeKV{
				{"VALOR DO IBS MONOFÁSICO", g.VIBSMono}, {"VALOR DA CBS MONOFÁSICA", g.VCBSMono},
				{"VALOR DO IBS MONOFÁSICO POR RETENÇÃO", g.VIBSMonoRet}, {"VALOR DA CBS MONOFÁSICA POR RETENÇÃO", g.VCBSMonoRet},
			} {
				if strings.TrimSpace(kv.value) != "" {
					mono = append(mono, dfeKV{kv.label, dfeMoney(kv.value)})
				}
			}
		}
	}
	if t.ISTot != nil {
		is = dfeMoney(t.ISTot.VIS)
	}
	w := dfeW / 4
	r.row(dfeRowH, num(w, "VALOR DA CBS", cbs), num(w, "VALOR DO IBS UF", ibsUF), num(w, "VALOR DO IBS MUNICÍPIO", ibsMun),
		num(w, "VALOR DO IMPOSTO SELETIVO", is))
	r.ensure(float64((len(mono)+3)/4)*dfeRowH, false)
	r.rowKV(4, mono, "R")
}

func (r *dfeR) issqn() {
	s := r.inf.Total.ISSQNtot
	if s == nil {
		return
	}
	r.ensure(4+dfeRowH, false)
	r.y += 1.2
	r.title("CÁLCULO DO ISSQN")
	w := dfeW / 4
	r.row(dfeRowH, cell(w, "INSCRIÇÃO MUNICIPAL", r.inf.Emit.IM), num(w, "VALOR TOTAL DOS SERVIÇOS", dfeMoney(s.VServ)),
		num(w, "BASE DE CÁLCULO DO ISSQN", dfeMoney(s.VBC)), num(w, "VALOR DO ISSQN", dfeMoney(s.VISS)))
}

func (r *dfeR) transportador() {
	t := r.inf.Transp
	if strings.TrimSpace(t.ModFrete) == "" {
		return
	}
	tr := t.Transporta
	r.ensure(4+3*dfeRowH, false)
	r.y += 1.2
	r.title("TRANSPORTADOR / VOLUMES TRANSPORTADOS")
	r.row(dfeRowH, cell(64, "RAZÃO SOCIAL", strings.TrimSpace(tr.XNome)), cell(32, "FRETE POR CONTA", dfeModFrete(t.ModFrete)),
		cell(24, "CÓDIGO ANTT", t.Veic.RNTC), cell(24, "PLACA DO VEÍCULO", t.Veic.Placa), cell(10, "UF", t.Veic.UF),
		cell(42, "CNPJ / CPF", dfeCNPJCPF(tr.CNPJ, tr.CPF)))
	r.row(dfeRowH, cell(84, "ENDEREÇO", strings.TrimSpace(tr.XEnder)), cell(60, "MUNICÍPIO", strings.TrimSpace(tr.XMun)),
		cell(10, "UF", tr.UF), cell(42, "INSCRIÇÃO ESTADUAL", tr.IE))
	var qvol, pb, pl float64
	var esp, marca, nvol string
	for i, v := range t.Vol {
		qvol += toDecimal(v.QVol)
		pb += toDecimal(v.PesoB)
		pl += toDecimal(v.PesoL)
		if i == 0 {
			esp, marca, nvol = v.Esp, v.Marca, v.NVol
		}
	}
	q, sb, sl := "", "", ""
	if len(t.Vol) > 0 {
		q = dfeNumMin(fmt.Sprintf("%v", qvol), 0, 0)
		sb = dfeNumMin(fmt.Sprintf("%v", pb), 3, 3)
		sl = dfeNumMin(fmt.Sprintf("%v", pl), 3, 3)
	}
	r.row(dfeRowH, cell(24, "QUANTIDADE", q), cell(40, "ESPÉCIE", esp), cell(40, "MARCA", marca), cell(30, "NUMERAÇÃO", nvol),
		num(31, "PESO BRUTO", sb), num(31, "PESO LÍQUIDO", sl))
}

// colunas da tabela de itens: soma = 196
var dfeCols = []struct {
	w     float64
	title string
}{
	{42, "DESCRIÇÃO DO PRODUTO / SERVIÇO"}, {12, "CST / CFOP"}, {17, "QTD / UN"}, {16, "VLR UNIT"}, {17, "VLR TOTAL"},
	{30, "BASES DE CÁLCULO"}, {30, "ALÍQUOTAS"}, {32, "VALOR DOS TRIBUTOS"},
}

func (r *dfeR) itensCabecalho() {
	r.title("DADOS DOS PRODUTOS / SERVIÇOS")
	x := dfeX0
	for _, c := range dfeCols {
		r.rect(x, r.y, c.w, 6)
		r.text(x, r.y+0.4, c.w, 5.2, c.title, 5.2, "B", "C", false)
		x += c.w
	}
	r.y += 6
}

func (r *dfeR) itens() {
	r.ensure(4+6+11, false)
	r.y += 1.2
	r.itensCabecalho()
	for _, d := range r.inf.Det {
		r.item(d)
	}
}

func joinSlash(a, b string) string {
	switch {
	case a != "" && b != "":
		return a + " / " + b
	case a != "":
		return a
	}
	return b
}

func tag(label, v string) string {
	if v == "" {
		return ""
	}
	return label + " " + v
}

func (r *dfeR) item(d dfeDet) {
	g := d.Imposto.IBSCBS.G
	var ic dfeICMSGrupo
	if len(d.Imposto.ICMS.Grupos) > 0 {
		ic = d.Imposto.ICMS.Grupos[0]
	}
	cst := ic.CSOSN
	if cst == "" {
		cst = ic.CST
	}
	cst = strings.TrimSpace(ic.Orig) + strings.TrimSpace(cst)

	// descrição (XML) + informações adicionais do item + referência NCM/cClassTrib
	const lh = 2.45
	desc := r.wrap(strings.TrimSpace(d.Prod.XProd), dfeCols[0].w-1.6, 5.8, "")
	var extra []string
	if s := strings.TrimSpace(d.InfAdProd); s != "" {
		extra = r.wrap(s, dfeCols[0].w-1.6, 5, "")
		if len(extra) > 5 {
			extra = append(extra[:5], "...")
		}
	}
	ref := ""
	if d.Prod.NCM != "" {
		ref = "[NCM " + strings.TrimSpace(d.Prod.NCM) + "]"
	}
	if cc := strings.TrimSpace(d.Imposto.IBSCBS.CClassTrib); cc != "" {
		ref = strings.TrimSpace(ref + " [cClassTrib " + cc + "]")
	}
	n := len(desc) + len(extra)
	if ref != "" {
		n++
	}
	h := math.Max(10.4, float64(n)*lh+1.6)
	r.ensure(h, true)

	y := r.y
	xs := make([]float64, len(dfeCols)+1)
	xs[0] = dfeX0
	for i, c := range dfeCols {
		xs[i+1] = xs[i] + c.w
		r.rect(xs[i], y, c.w, h)
	}
	col := func(i int) (float64, float64) { return xs[i] + 0.8, dfeCols[i].w - 1.6 }

	// 0 descrição
	x, w := col(0)
	ty := y + 0.8
	r.pdf.SetTextColor(0, 0, 0)
	r.font("", 5.8)
	for _, l := range desc {
		r.pdf.SetXY(x, ty)
		r.pdf.CellFormat(w, lh, l, "", 0, "L", false, 0, "")
		ty += lh
	}
	r.pdf.SetTextColor(95, 95, 95)
	r.font("", 5)
	for _, l := range extra {
		r.pdf.SetXY(x, ty)
		r.pdf.CellFormat(w, lh, l, "", 0, "L", false, 0, "")
		ty += lh
	}
	if ref != "" {
		r.text(x, y+h-lh-0.6, w, lh, ref, 5, "", "L", true)
	}

	// 1 CST / CFOP
	x, w = col(1)
	r.text(x, y+1, w, 2.4, "CST", 4.6, "", "L", true)
	r.text(x, y+3, w, 2.6, cst, 6, "", "R", false)
	r.text(x, y+5.8, w, 2.4, "CFOP", 4.6, "", "L", true)
	r.text(x, y+7.8, w, 2.6, strings.TrimSpace(d.Prod.CFOP), 6, "", "R", false)

	// 2 QTD / UN, 3 VLR UNIT, 4 VLR TOTAL
	x, w = col(2)
	r.text(x, y+1.2, w, 2.6, dfeNumMin(d.Prod.QCom, 4, 4), 6, "", "R", false)
	if u := strings.TrimSpace(d.Prod.UCom); u != "" {
		r.text(x, y+4, w, 2.4, "["+u+"]", 5.2, "", "R", true)
	}
	x, w = col(3)
	r.text(x, y+1.2, w, 2.6, dfeNumMin(d.Prod.VUn, 2, 4), 6, "", "R", false)
	x, w = col(4)
	r.text(x, y+1.2, w, 2.6, dfeMoney(d.Prod.VProd), 6, "", "R", false)

	// 5 bases de cálculo (rótulo à esquerda, valor à direita)
	x, w = col(5)
	bases := []dfeKV{
		{"ICMS", dfeMoney(ic.VBC)}, {"IBS / CBS", dfeMoney(g.VBC)},
		{"IS", dfeMoney(d.Imposto.IS.VBCIS)}, {"IPI", dfeMoney(d.Imposto.IPI.IPITrib.VBC)},
	}
	for i, b := range bases {
		yy := y + 0.8 + float64(i)*2.4
		r.text(x, yy, w, 2.4, b.label, 4.8, "", "L", true)
		r.text(x, yy, w, 2.4, b.value, 5.6, "", "R", false)
	}

	// 6 alíquotas — com gRed informado imprime a EFETIVA (NT 2026.010, 4.3)
	ali := func(vigente string, gr dfeGRed) string {
		if strings.TrimSpace(gr.PAliqEfet) != "" {
			return gr.PAliqEfet
		}
		return vigente
	}
	x, w = col(6)
	aliq := []string{
		joinSlash(tag("ICMS", dfePct(ic.PICMS)), tag("CBS", dfePct(ali(g.CBS.P, g.CBS.GRed)))),
		joinSlash(tag("IBS UF", dfePct(ali(g.UF.P, g.UF.GRed))), tag("IPI", dfePct(d.Imposto.IPI.IPITrib.PIPI))),
		joinSlash(tag("IBS MUN", dfePct(ali(g.Mun.P, g.Mun.GRed))), tag("IS", dfePct(d.Imposto.IS.PIS))),
	}
	for i, l := range aliq {
		r.text(x, y+1.2+float64(i)*3, w, 2.6, l, 5.4, "", "L", false)
	}

	// 7 valores dos tributos
	x, w = col(7)
	vals := []string{
		joinSlash(tag("ICMS", dfeMoney(ic.VICMS)), tag("CBS", dfeMoney(g.CBS.V))),
		joinSlash(tag("IBS UF", dfeMoney(g.UF.V)), tag("IPI", dfeMoney(d.Imposto.IPI.IPITrib.VIPI))),
		joinSlash(tag("IBS MUN", dfeMoney(g.Mun.V)), tag("IS", dfeMoney(d.Imposto.IS.VIS))),
	}
	for i, l := range vals {
		r.text(x, y+1.2+float64(i)*3, w, 2.6, l, 5.4, "", "L", false)
	}

	r.y += h
}

func (r *dfeR) dadosAdicionais() {
	info := strings.TrimSpace(strings.Join(nonEmpty(r.inf.InfAdic.InfAdFisco, r.inf.InfAdic.InfCpl), "\n"))
	const wInfo, wFisco, wQR = 100.0, 56.0, 40.0
	const lh = 2.4
	lines := r.wrap(info, wInfo-2, 5.8, "")
	h := math.Max(24, float64(len(lines))*lh+6)
	if h > dfePageBottom-20 {
		h = dfePageBottom - 20
	}
	r.ensure(4+h, false)
	r.y += 1.2
	r.title("DADOS ADICIONAIS")
	y := r.y
	r.rect(dfeX0, y, wInfo, h)
	r.rect(dfeX0+wInfo, y, wFisco, h)
	r.rect(dfeX0+wInfo+wFisco, y, wQR, h)
	r.text(dfeX0+0.8, y+0.5, wInfo-1.6, 2, "INFORMAÇÕES COMPLEMENTARES", 4.8, "", "L", true)
	r.text(dfeX0+wInfo+0.8, y+0.5, wFisco-1.6, 2, "RESERVADO AO FISCO", 4.8, "", "L", true)
	r.text(dfeX0+wInfo+wFisco, y+0.8, wQR, 3, "QR CODE", 6.5, "B", "C", false)

	maxLines := int((h - 4) / lh)
	if len(lines) > maxLines {
		lines = append(lines[:maxLines-1], "...")
	}
	r.pdf.SetTextColor(0, 0, 0)
	r.font("", 5.8)
	ty := y + 3.4
	for _, l := range lines {
		r.pdf.SetXY(dfeX0+0.8, ty)
		r.pdf.CellFormat(wInfo-1.6, lh, l, "", 0, "L", false, 0, "")
		ty += lh
	}
	// QR Code só se o XML trouxer o conteúdo (infNFeSupl/qrCode) — a NT não autoriza inferir.
	if qrc := strings.TrimSpace(r.p.NFe.Supl.QrCode); qrc != "" {
		size := math.Min(wQR-6, h-9)
		r.qrcode(dfeX0+wInfo+wFisco+(wQR-size)/2, y+5.5, size, qrc)
	}
	r.y = y + h
}

func nonEmpty(ss ...string) []string {
	var out []string
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}
