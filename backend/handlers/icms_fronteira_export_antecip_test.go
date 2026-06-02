package handlers

import (
	"testing"

	"github.com/xuri/excelize/v2"
)

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
