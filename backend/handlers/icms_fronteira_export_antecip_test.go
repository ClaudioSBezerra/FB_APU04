package handlers

import (
	"math"
	"testing"

	"github.com/xuri/excelize/v2"
)

// TestCteAntecip valida a antecipação do frete (CT-e) na mesma cadeia da NF —
// mapa confirmado pelo contador (frete 1000, destacado 120, alíq interna 20,5%).
func TestCteAntecip(t *testing.T) {
	const frete, destacado, aliq = 1000.0, 120.0, 20.5

	// BA (direto): devido = 1000 × 20,5% = 205; a pagar = 205 − 120 = 85.
	dev, pagar := cteAntecip(frete, destacado, aliq, false)
	if math.Abs(dev-205.0) > 0.01 {
		t.Errorf("direto devido = %.2f, quer 205,00", dev)
	}
	if math.Abs(pagar-85.0) > 0.01 {
		t.Errorf("direto a pagar = %.2f, quer 85,00", pagar)
	}

	// PE (por dentro): devido = (1000−120)/0,795 × 20,5% = 226,92; a pagar = 106,92.
	devPD, pagarPD := cteAntecip(frete, destacado, aliq, true)
	if math.Abs(devPD-226.92) > 0.01 {
		t.Errorf("por dentro devido = %.2f, quer ~226,92", devPD)
	}
	if math.Abs(pagarPD-106.92) > 0.01 {
		t.Errorf("por dentro a pagar = %.2f, quer ~106,92", pagarPD)
	}

	// Nunca negativo: destacado maior que o devido → a pagar = 0.
	if _, p := cteAntecip(1000, 500, aliq, false); p != 0 {
		t.Errorf("a pagar deveria ser 0 quando destacado > devido, veio %.2f", p)
	}
}

// TestWriteBlocoCAntecipXLSX exercita a geração da aba C (antecipação) com a
// cadeia de cálculo do contador. Usa db=nil (sem CT-e) — o guard em
// fetchCteLinksForNFs devolve mapa vazio, então só as linhas de NF são geradas.
func TestWriteBlocoCAntecipXLSX(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	sheet := "C"
	f.NewSheet(sheet)

	rows := []FronteiraXmlNaoSpedRow{
		{
			ChaveNFe: "111", DataEmissao: "2026-05-10", NumeroNFe: "1001",
			FornCNPJ: "12345678000199", FornNome: "Forn A", FornUF: "SP",
			CfopSaida: "6101", VProd: 10000, VIPI: 1000, VFrete: 500, VOutro: 0,
			AliqInter: 12, AliqInterna: 20.5, VIcmsNF: 1200,
			ValorDevido: 2255, IcmsDevidoEst: 1055, Regime: "ANTECIPACAO", ClassStatus: "auto",
		},
		{
			ChaveNFe: "222", DataEmissao: "2026-05-12", NumeroNFe: "1002",
			FornCNPJ: "98765432000111", FornNome: "Forn B", FornUF: "MG",
			CfopSaida: "6102", VProd: 5000, VIPI: 0, VFrete: 0, VOutro: 100,
			AliqInter: 12, AliqInterna: 20.5, VIcmsNF: 600,
			ValorDevido: 1045.5, IcmsDevidoEst: 445.5, Regime: "ANTECIPACAO", ClassStatus: "auto",
		},
	}

	moneyFmt := `"R$" #,##0.00`
	writeBlocoCAntecipXLSX(f, nil, "co1", sheet, rows, 0, moneyFmt, "DCE6F1", 0, 0)

	// Cabeçalho da cadeia
	if got, _ := f.GetCellValue(sheet, "A1"); got != "Data Emissão" {
		t.Errorf("A1 = %q, quer 'Data Emissão'", got)
	}
	if got, _ := f.GetCellValue(sheet, "L1"); got != "Total Operação" {
		t.Errorf("L1 = %q, quer 'Total Operação'", got)
	}
	if got, _ := f.GetCellValue(sheet, "Q1"); got != "ICMS a Pagar" {
		t.Errorf("Q1 = %q, quer 'ICMS a Pagar'", got)
	}

	// Primeira NF na linha 2
	if got, _ := f.GetCellValue(sheet, "B2"); got != "1001" {
		t.Errorf("B2 (NF-e) = %q, quer '1001'", got)
	}
	if got, _ := f.GetCellValue(sheet, "L2"); got == "" {
		t.Error("L2 (Total Operação) não deveria estar vazio")
	}

	// Sem CT-e (db=nil) → 2 NFs nas linhas 2 e 3, TOTAL na linha 4
	if got, _ := f.GetCellValue(sheet, "A4"); got != "TOTAL" {
		t.Errorf("A4 = %q, quer 'TOTAL'", got)
	}
}
