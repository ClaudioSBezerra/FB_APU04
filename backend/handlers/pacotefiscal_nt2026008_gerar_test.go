package handlers

import (
	"encoding/xml"
	"math"
	"strings"
	"testing"
	"time"
)

// Testes da ADIÇÃO das tags de valor líquido (NT 2026.008) e do vNFTot recalculado no XML
// da Simulação 2027. Ver pacotefiscal_sim2027_liquido.go.

// nt008Raiz lê do nfeProc gerado só o que estes testes conferem.
type nt008Raiz struct {
	XMLName xml.Name `xml:"nfeProc"`
	Dets    []struct {
		Prod struct {
			QCom      string `xml:"qCom"`
			VProd     string `xml:"vProd"`
			VUnComLiq string `xml:"vUnComLiq"`
			VProdLiq  string `xml:"vProdLiq"`
		} `xml:"prod"`
		VItem string `xml:"vItem"`
		VIBS  string `xml:"imposto>IBSCBS>gIBSCBS>vIBS"`
		VCBS  string `xml:"imposto>IBSCBS>gIBSCBS>gCBS>vCBS"`
	} `xml:"NFe>infNFe>det"`
	Total struct {
		Filhos []struct {
			XMLName xml.Name
			Valor   string `xml:",chardata"`
		} `xml:",any"`
	} `xml:"NFe>infNFe>total"`
	// preenchidos por uma 2ª decodificação (Go não aceita campo "total" e filhos de total na mesma struct)
	VNF, VIBS, VCBS, VNFTot string `xml:"-"`
}

func nt008Gerar(t *testing.T, original []byte) (string, nt008Raiz) {
	t.Helper()
	params := pfsParametros{AliqIbsUfPct: 0.05, AliqIbsMunPct: 0.05, AliqCbsPct: 9.90}
	out, err := pfsGerarXMLSimulado(original, params, time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("pfsGerarXMLSimulado: %v", err)
	}
	var r nt008Raiz
	if err := xml.Unmarshal(pfsStripNamespaces(out), &r); err != nil {
		t.Fatalf("XML gerado inválido: %v", err)
	}
	var tot struct {
		XMLName xml.Name `xml:"nfeProc"`
		VNF     string   `xml:"NFe>infNFe>total>ICMSTot>vNF"`
		VIBS    string   `xml:"NFe>infNFe>total>IBSCBSTot>gIBS>vIBS"`
		VCBS    string   `xml:"NFe>infNFe>total>IBSCBSTot>gCBS>vCBS"`
		VNFTot  string   `xml:"NFe>infNFe>total>vNFTot"`
	}
	if err := xml.Unmarshal(pfsStripNamespaces(out), &tot); err != nil {
		t.Fatalf("XML gerado inválido (totais): %v", err)
	}
	r.VNF, r.VIBS, r.VCBS, r.VNFTot = tot.VNF, tot.VIBS, tot.VCBS, tot.VNFTot
	return string(out), r
}

func (r nt008Raiz) totalFilho(nome string) string {
	for _, f := range r.Total.Filhos {
		if f.XMLName.Local == nome {
			return strings.TrimSpace(f.Valor)
		}
	}
	return ""
}

// vNFTot esperado = vNF + vIBS + vCBS, lidos da própria saída (já recalculados).
func (r nt008Raiz) vNFTotEsperado() float64 {
	return math.Round((toDecimal(r.VNF)+toDecimal(r.VIBS)+toDecimal(r.VCBS))*100) / 100
}

func TestSim2027_AdicionaValorLiquido_FormulaPosicaoERegrasDaNT(t *testing.T) {
	_, r := nt008Gerar(t, readTestdata(t, "sample_nfe55_sim2027.xml"))

	// vProd - vICMS - vPIS - vCOFINS (originais), conta feita à mão:
	// 2590.27-103.61-41.03-188.99 | 7.45-0.30-0.12-0.54 | 613.41-24.54-9.72-44.75 | 277.55-11.10-4.40-20.25
	esperado := []string{"2256.64", "6.49", "534.40", "241.80"}
	if len(r.Dets) != 4 {
		t.Fatalf("esperava 4 itens, veio %d", len(r.Dets))
	}
	var soma float64
	for i, d := range r.Dets {
		if d.Prod.VProdLiq != esperado[i] {
			t.Errorf("item %d: vProdLiq = %q, esperava %q", i+1, d.Prod.VProdLiq, esperado[i])
		}
		liq, un, q := toDecimal(d.Prod.VProdLiq), toDecimal(d.Prod.VUnComLiq), toDecimal(d.Prod.QCom)
		// Regra I11c-20: vProdLiq difere de vUnComLiq * qCom em mais de 0,01?
		if math.Abs(liq-un*q) > 0.01 {
			t.Errorf("item %d viola I11c-20: vProdLiq=%v vUnComLiq*qCom=%v", i+1, liq, un*q)
		}
		soma += liq
	}
	// vProdLiqTot é o 1º filho de <total> (XSD) e bate com a soma (regra W01a-20).
	if len(r.Total.Filhos) == 0 || r.Total.Filhos[0].XMLName.Local != "vProdLiqTot" {
		t.Fatalf("vProdLiqTot deveria ser o 1º filho de <total>")
	}
	if got := r.totalFilho("vProdLiqTot"); got != "3039.33" || math.Abs(toDecimal(got)-soma) > 0.001 {
		t.Errorf("vProdLiqTot = %q, esperava 3039.33 (= soma %v)", got, soma)
	}
}

// Emitente do Simples (como o XML real de 31/08/2026): sem vICMS, PISOutr/COFINSOutr com
// vPIS/vCOFINS zerados, vNFTot já presente. Nada a abater: vProdLiq = vProd.
const nt008XMLSimples = `<?xml version="1.0" encoding="UTF-8"?>
<nfeProc xmlns="http://www.portalfiscal.inf.br/nfe" versao="4.00"><NFe xmlns="http://www.portalfiscal.inf.br/nfe"><infNFe Id="NFe35260813902190000184550010000058841006275368" versao="4.00"><ide><mod>55</mod><serie>1</serie><nNF>5884</nNF><dhEmi>2026-08-31T16:50:06-03:00</dhEmi></ide><det nItem="1"><prod><cProd>1</cProd><xProd>A</xProd><qCom>30.0000</qCom><vUnCom>15.0000</vUnCom><vProd>450.00</vProd><cEANTrib>SEM GTIN</cEANTrib></prod><imposto><ICMS><ICMSSN101><orig>0</orig><CSOSN>101</CSOSN><pCredSN>3.08</pCredSN><vCredICMSSN>13.86</vCredICMSSN></ICMSSN101></ICMS><PIS><PISOutr><CST>99</CST><vBC>0.00</vBC><pPIS>0.00</pPIS><vPIS>0.00</vPIS></PISOutr></PIS><COFINS><COFINSOutr><CST>99</CST><vBC>0.00</vBC><pCOFINS>0.00</pCOFINS><vCOFINS>0.00</vCOFINS></COFINSOutr></COFINS><IBSCBS><CST>000</CST><cClassTrib>000001</cClassTrib><gIBSCBS><vBC>450.00</vBC><gIBSUF><pIBSUF>0.1000</pIBSUF><vIBSUF>0.45</vIBSUF></gIBSUF><gIBSMun><pIBSMun>0.0000</pIBSMun><vIBSMun>0.00</vIBSMun></gIBSMun><vIBS>0.45</vIBS><gCBS><pCBS>0.9000</pCBS><vCBS>4.05</vCBS></gCBS></gIBSCBS></IBSCBS></imposto></det><det nItem="2"><prod><cProd>2</cProd><xProd>B</xProd><qCom>6.0000</qCom><vUnCom>18.0000</vUnCom><vProd>108.00</vProd><cEANTrib>SEM GTIN</cEANTrib></prod><imposto><ICMS><ICMSSN101><orig>0</orig><CSOSN>101</CSOSN><pCredSN>3.08</pCredSN><vCredICMSSN>3.33</vCredICMSSN></ICMSSN101></ICMS><IBSCBS><CST>000</CST><cClassTrib>000001</cClassTrib><gIBSCBS><vBC>108.00</vBC><gIBSUF><pIBSUF>0.1000</pIBSUF><vIBSUF>0.11</vIBSUF></gIBSUF><gIBSMun><pIBSMun>0.0000</pIBSMun><vIBSMun>0.00</vIBSMun></gIBSMun><vIBS>0.11</vIBS><gCBS><pCBS>0.9000</pCBS><vCBS>0.97</vCBS></gCBS></gIBSCBS></IBSCBS></imposto></det><total><ICMSTot><vProd>558.00</vProd><vNF>558.00</vNF></ICMSTot><IBSCBSTot><vBCIBSCBS>558.00</vBCIBSCBS><gIBS><gIBSUF><vIBSUF>0.56</vIBSUF></gIBSUF><gIBSMun><vIBSMun>0.00</vIBSMun></gIBSMun><vIBS>0.56</vIBS></gIBS><gCBS><vCBS>5.02</vCBS></gCBS></IBSCBSTot><vNFTot>563.58</vNFTot></total></infNFe></NFe></nfeProc>`

func TestSim2027_Simples_ValorLiquidoIgualAoVProdEVNFTot(t *testing.T) {
	saida, r := nt008Gerar(t, []byte(nt008XMLSimples))
	if len(r.Dets) != 2 || r.Dets[0].Prod.VProdLiq != "450.00" || r.Dets[1].Prod.VProdLiq != "108.00" {
		t.Fatalf("Simples sem ICMS/PIS/COFINS a abater: vProdLiq deveria ser = vProd; veio %+v", r.Dets)
	}
	if got := r.totalFilho("vProdLiqTot"); got != "558.00" {
		t.Errorf("vProdLiqTot = %q, esperava 558.00", got)
	}
	if r.VNFTot == "563.58" || r.VNFTot == "" {
		t.Fatalf("vNFTot deveria ser recalculado (era o valor original 563.58), veio %q", r.VNFTot)
	}
	// Simples sem IPI/frete: Σ vItem = Σ vProd + IBS + CBS (= vNF + IBS + CBS, que aqui coincide)
	if want := r.vNFTotEsperado(); math.Abs(toDecimal(r.VNFTot)-want) > 0.001 {
		t.Errorf("vNFTot = %s, esperava %.2f", r.VNFTot, want)
	}
	if !strings.Contains(saida, "valor liquido (NT 2026.008) adicionado") {
		t.Error("o comentário de simulação deveria avisar que o valor líquido foi adicionado")
	}
}

func TestPfsItemLiquido_NuncaFicaNegativo(t *testing.T) {
	it := pfsItemLiquido{vProd: 1000, vICMS: 900, vPIS: 100, vCOFINS: 50} // tributos > preço
	if got := it.liquidoCents(); got != 0 {
		t.Errorf("liquidoCents = %d, esperava 0 (nunca negativo)", got)
	}
}

func TestSim2027_CTeNaoRecebeTagsDaNT(t *testing.T) {
	out, err := pfsGerarXMLSimulado(readTestdata(t, "sample_cte_sim2027.xml"),
		pfsParametros{AliqIbsUfPct: 0.05, AliqIbsMunPct: 0.05, AliqCbsPct: 9.90}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "vProdLiq") || strings.Contains(string(out), "vUnComLiq") {
		t.Error("CT-e não faz parte da NT 2026.008: não deve receber vProdLiq/vUnComLiq")
	}
}

// ---------------------------------------------------------------------------
// vItem por item e vNFTot = Σ vItem (NT 2026.008, decisão do usuário 2026-11-19)
// ---------------------------------------------------------------------------

func TestSim2027_VItemPorItem_VNFTotEhSomaDosItens_SemOIPIOriginal(t *testing.T) {
	_, r := nt008Gerar(t, readTestdata(t, "sample_nfe55_sim2027.xml"))
	if len(r.Dets) != 4 {
		t.Fatalf("esperava 4 itens, veio %d", len(r.Dets))
	}
	var somaVItem, somaVProd, somaIBS, somaCBS float64
	for i, d := range r.Dets {
		vp, ibs, cbs := toDecimal(d.Prod.VProd), toDecimal(d.VIBS), toDecimal(d.VCBS)
		// a simulação zera o IPI do item; sem frete/seguro/desconto/ST: vItem = vProd + IBS + CBS
		if want := math.Round((vp+ibs+cbs)*100) / 100; math.Abs(toDecimal(d.VItem)-want) > 0.001 {
			t.Errorf("item %d: vItem = %q, esperava %.2f (= %v + %v + %v)", i+1, d.VItem, want, vp, ibs, cbs)
		}
		somaVItem += toDecimal(d.VItem)
		somaVProd += vp
		somaIBS += ibs
		somaCBS += cbs
	}
	if math.Abs(toDecimal(r.VNFTot)-somaVItem) > 0.001 {
		t.Errorf("vNFTot = %s, esperava a soma dos vItem = %.2f", r.VNFTot, somaVItem)
	}
	// O IPI original (181,01) continua no ICMSTot/vNF, mas NÃO entra no vNFTot: o item foi
	// simulado sem IPI. O valor "vNF + IBS + CBS" (fórmula antiga) ficaria 181,01 acima.
	antiga := toDecimal(r.VNF) + toDecimal(r.VIBS) + toDecimal(r.VCBS)
	if got := antiga - toDecimal(r.VNFTot); math.Abs(got-181.01) > 0.011 {
		t.Errorf("a diferença para a fórmula antiga deveria ser o IPI original 181,01, veio %.2f", got)
	}
	// totais do IBS/CBS do documento batem com a soma dos itens
	if math.Abs(somaIBS-toDecimal(r.VIBS)) > 0.011 || math.Abs(somaCBS-toDecimal(r.VCBS)) > 0.011 {
		t.Errorf("Σ IBS/CBS dos itens (%.2f/%.2f) ≠ totais do documento (%s/%s)", somaIBS, somaCBS, r.VIBS, r.VCBS)
	}
}

func TestSim2027_VItemSubstituiValorExistente_ENaoDuplica(t *testing.T) {
	orig := strings.Replace(string(readTestdata(t, "sample_nfe55_sim2027.xml")),
		`</imposto></det><det nItem="2">`, `</imposto><vItem>1.00</vItem></det><det nItem="2">`, 1)
	saida, r := nt008Gerar(t, []byte(orig))
	if n := strings.Count(saida, "<vItem>"); n != 4 {
		t.Fatalf("esperava exatamente 4 <vItem> (1 substituído + 3 adicionados), veio %d", n)
	}
	if r.Dets[0].VItem == "1.00" || r.Dets[0].VItem == "" {
		t.Errorf("o vItem desatualizado do item 1 deveria ser recalculado, veio %q", r.Dets[0].VItem)
	}
}

func TestSim2027_VItemSomaOsComponentesDaRegraVB0110(t *testing.T) {
	// item 1 do XML do Simples (vProd 450,00) com frete 10,00 e desconto 4,00 no <prod>
	orig := strings.Replace(nt008XMLSimples, `<vProd>450.00</vProd>`, `<vProd>450.00</vProd><vFrete>10.00</vFrete><vDesc>4.00</vDesc>`, 1)
	_, r := nt008Gerar(t, []byte(orig))
	vp := 450.00 + 10.00 - 4.00
	want := math.Round((vp+toDecimal(r.Dets[0].VIBS)+toDecimal(r.Dets[0].VCBS))*100) / 100
	if math.Abs(toDecimal(r.Dets[0].VItem)-want) > 0.001 {
		t.Errorf("vItem = %s, esperava %.2f (vProd + frete - desconto + IBS + CBS)", r.Dets[0].VItem, want)
	}
}

func TestSim2027_VItemFicaDepoisDoImpostoEAntesDeDFeReferenciado(t *testing.T) {
	orig := strings.Replace(string(readTestdata(t, "sample_nfe55_sim2027.xml")),
		`</imposto></det><det nItem="2">`,
		`</imposto><DFeReferenciado><chaveAcesso>32260612449448000176550010001815821291031700</chaveAcesso></DFeReferenciado></det><det nItem="2">`, 1)
	saida, _ := nt008Gerar(t, []byte(orig))
	iImposto := strings.Index(saida, "</imposto>")
	iVItem := strings.Index(saida, "<vItem>")
	iDFe := strings.Index(saida, "<DFeReferenciado>")
	if !(iImposto >= 0 && iImposto < iVItem && iVItem < iDFe) {
		t.Errorf("ordem esperada </imposto> < <vItem> < <DFeReferenciado>, veio %d, %d, %d", iImposto, iVItem, iDFe)
	}
}

func TestSim2027_AliquotaEfetivaComGRedSegueAAliquotaDaTelaDeTeste(t *testing.T) {
	orig := nt008XMLSimples
	// item 1 com redução: pAliqEfet "antigo" nos três grupos
	orig = strings.Replace(orig, `<pIBSUF>0.1000</pIBSUF><vIBSUF>0.45</vIBSUF>`,
		`<pIBSUF>0.1000</pIBSUF><gRed><pRedAliq>60.0000</pRedAliq><pAliqEfet>0.0400</pAliqEfet></gRed><vIBSUF>0.45</vIBSUF>`, 1)
	orig = strings.Replace(orig, `<pIBSMun>0.0000</pIBSMun><vIBSMun>0.00</vIBSMun>`,
		`<pIBSMun>0.0000</pIBSMun><gRed><pRedAliq>60.0000</pRedAliq><pAliqEfet>0.0000</pAliqEfet></gRed><vIBSMun>0.00</vIBSMun>`, 1)
	orig = strings.Replace(orig, `<pCBS>0.9000</pCBS><vCBS>4.05</vCBS>`,
		`<pCBS>0.9000</pCBS><gRed><pRedAliq>60.0000</pRedAliq><pAliqEfet>0.3600</pAliqEfet></gRed><vCBS>4.05</vCBS>`, 1)
	saida, _ := nt008Gerar(t, []byte(orig)) // parâmetros do teste: UF 0,05 / Mun 0,05 / CBS 9,90
	for _, want := range []string{
		`<gIBSUF><pIBSUF>0.0500</pIBSUF><gRed><pRedAliq>60.0000</pRedAliq><pAliqEfet>0.0500</pAliqEfet></gRed>`,
		`<gIBSMun><pIBSMun>0.0500</pIBSMun><gRed><pRedAliq>60.0000</pRedAliq><pAliqEfet>0.0500</pAliqEfet></gRed>`,
		`<gCBS><pCBS>9.9000</pCBS><gRed><pRedAliq>60.0000</pRedAliq><pAliqEfet>9.9000</pAliqEfet></gRed>`,
	} {
		if !strings.Contains(saida, want) {
			t.Errorf("não achei %q no XML simulado", want)
		}
	}
}
