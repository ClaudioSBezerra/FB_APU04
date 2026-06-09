package handlers

import (
	"bytes"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// mkSTItem cria uma STItemRow já com os derivados calculados via computeST.
func mkSTItem(chave, numero, cod, ncm string, vProd, vIpi, mvaAj, aliqInter, aliqInt, redBC, icmsDeb, icmsRet float64, temRegra, segOK bool) STItemRow {
	return mkSTItemBloco("mes_atual", chave, numero, cod, ncm, vProd, vIpi, mvaAj, aliqInter, aliqInt, redBC, icmsDeb, icmsRet, temRegra, segOK)
}

// mkSTItemBloco é como mkSTItem mas permite definir o bloco (A/B/C).
func mkSTItemBloco(bloco, chave, numero, cod, ncm string, vProd, vIpi, mvaAj, aliqInter, aliqInt, redBC, icmsDeb, icmsRet float64, temRegra, segOK bool) STItemRow {
	row := STItemRow{
		ChaveNFe:     chave,
		NumeroNFe:    numero,
		FornNome:     "Fornecedor " + numero,
		CFOP:         "2403",
		Bloco:        bloco,
		CodProduto:   cod,
		Descricao:    "Produto " + cod,
		NCM:          ncm,
		CEST:         "0100100",
		VProd:        vProd,
		VIPI:         vIpi,
		VOutro:       0,
		TemRegra:     temRegra,
		SegmentoOK:   segOK,
		MVAOriginal:  mvaAj,
		MVAAjustado:  mvaAj,
		AliqInter:    aliqInter,
		AliqInterna:  aliqInt,
		ReducaoBC:    redBC,
		IcmsDebitado: icmsDeb,
		IcmsRetido:   icmsRet,
	}
	row.StatusXML = "Encontrado"
	(&row).computeST()
	return row
}

// fabricaRows monta notas em blocos distintos para cobrir o particionamento A/B/C:
//   - NF 1001 (Bloco B / mes_atual): 2 itens com regra+segmento OK, reducao_bc 0 e 33,33
//   - NF 1002 (Bloco A / mes_anterior): 1 item SEM regra (TemRegra=false)
//   - NF 1003 (Bloco C / nao_sped): 1 item com regra mas segmento NÃO casou (SegmentoOK=false)
func fabricaRows() []STItemRow {
	return []STItemRow{
		mkSTItemBloco("mes_atual", "CHAVE1001", "1001", "P1", "30049099", 1000, 100, 40, 12, 20.5, 0, 120, 0, true, true),
		mkSTItemBloco("mes_atual", "CHAVE1001", "1001", "P2", "30049099", 500, 0, 40, 12, 20.5, 33.33, 60, 10, true, true),
		mkSTItemBloco("mes_anterior", "CHAVE1002", "1002", "P3", "21069090", 800, 0, 0, 12, 20.5, 0, 96, 0, false, false),
		mkSTItemBloco("nao_sped", "CHAVE1003", "1003", "P4", "22021000", 700, 50, 50, 7, 18, 0, 84, 0, true, false),
	}
}

func fabricaCteLinks() map[string][]CteLink {
	return map[string][]CteLink{
		"CHAVE1001": {
			{
				ChaveCTe:  "CTECHAVE1",
				NumeroCTe: "9001",
				EmitNome:  "Transportadora X",
				EmitCNPJ:  "00000000000191",
				VPrest:    300,
				VIcmsCTe:  36,
			},
		},
	}
}

func TestBuildSTItensXLSX(t *testing.T) {
	rows := fabricaRows()
	cteLinks := fabricaCteLinks()

	data, err := buildSTItensXLSX(rows, cteLinks)
	if err != nil {
		t.Fatalf("buildSTItensXLSX retornou erro: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("buildSTItensXLSX retornou bytes vazios")
	}

	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("falha ao reabrir XLSX gerado: %v", err)
	}
	defer f.Close()

	sheet := "ST por item"
	idx, err := f.GetSheetIndex(sheet)
	if err != nil || idx < 0 {
		t.Fatalf("aba %q não encontrada (idx=%d err=%v)", sheet, idx, err)
	}

	xlRows, err := f.GetRows(sheet)
	if err != nil {
		t.Fatalf("GetRows: %v", err)
	}
	// Header + 4 itens + subtotais/totais por nota (3) + CT-e (2 linhas)
	// + subtotal CT-e + rodapé. Deve ser bem mais que 10 linhas.
	if len(xlRows) < 10 {
		t.Fatalf("esperava várias linhas, obtive %d", len(xlRows))
	}

	// Cabeçalho na primeira célula.
	if v, _ := f.GetCellValue(sheet, "A1"); v != "CFOP" {
		t.Fatalf("A1 esperado CFOP, obtido %q", v)
	}

	// Junta todo o conteúdo para checar marcadores.
	var all strings.Builder
	for _, r := range xlRows {
		all.WriteString(strings.Join(r, "|"))
		all.WriteString("\n")
	}
	body := all.String()
	for _, want := range []string{"Subtotal Produtos NF 1001:", "TOTAL GERAL NF: 1001", "Subtotal CT-es Vinculados:", "CTECHAVE1", "30049099"} {
		if !strings.Contains(body, want) {
			t.Errorf("XLSX não contém marcador esperado %q", want)
		}
	}
}

func TestBuildSTItensXLSXSemCte(t *testing.T) {
	// Caminho sem nenhum CT-e em nenhuma nota.
	rows := fabricaRows()
	data, err := buildSTItensXLSX(rows, map[string][]CteLink{})
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("bytes vazios")
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("reabrir: %v", err)
	}
	defer f.Close()
	xlRows, _ := f.GetRows("ST por item")
	if len(xlRows) < 8 {
		t.Fatalf("esperava várias linhas, obtive %d", len(xlRows))
	}
}

func TestBuildSTItensXLSXVazio(t *testing.T) {
	data, err := buildSTItensXLSX(nil, nil)
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("bytes vazios para entrada vazia")
	}
}

func TestBuildSTItensHTML(t *testing.T) {
	rows := fabricaRows()
	cteLinks := fabricaCteLinks()

	html := buildSTItensHTML(rows, cteLinks)
	if html == "" {
		t.Fatal("buildSTItensHTML retornou string vazia")
	}

	for _, want := range []string{
		"<table",
		"TOTAL GERAL NF: 1001",
		"Subtotal Produtos NF 1001:",
		"Subtotal CT-es Vinculados:",
		"Total Geral ICMS a Pagar",
		"30049099",
		"CTECHAVE1",
		"Rateio CT-e 9001",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML não contém marcador esperado %q", want)
		}
	}
}

func TestBuildSTItensHTMLSemCte(t *testing.T) {
	rows := fabricaRows()
	html := buildSTItensHTML(rows, map[string][]CteLink{})
	if !strings.Contains(html, "<table") {
		t.Error("HTML sem tabela")
	}
	if strings.Contains(html, "Subtotal CT-es Vinculados:") {
		t.Error("não deveria haver subtotal de CT-e sem links")
	}
	if !strings.Contains(html, "TOTAL GERAL NF: 1002") {
		t.Error("faltou TOTAL GERAL da NF 1002")
	}
}

func TestBuildSTItensHTMLVazio(t *testing.T) {
	html := buildSTItensHTML(nil, nil)
	if !strings.Contains(html, "Nenhum item de ST encontrado") {
		t.Error("esperava mensagem de vazio")
	}
	if !strings.Contains(html, "<table") {
		t.Error("esperava tabela mesmo vazia")
	}
}
