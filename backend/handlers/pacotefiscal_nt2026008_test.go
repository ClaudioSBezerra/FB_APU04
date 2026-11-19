package handlers

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"
)

// NT 2026.008 v1.01 (Reforma Tributária do Consumo — Valor Líquido) cria, no leiaute da
// NF-e/NFC-e, as tags: vUnComLiq (I11b) e vProdLiq (I11c) no item, vProdLiqTot (W01a) no
// total e o grupo ICMSPrevistoPagtoAntecip/vICMSPrevisto (NB01/NB02) dentro de <ICMS>,
// só para pagamento antecipado (tpNFDebito=06 / tpOperGov=4). Produção a partir de
// 16/11/2026. Estes testes garantem que o módulo Teste Pacote Fiscal continua importando
// e simulando XMLs que já trazem essas tags. Posições conferidas nos schemas oficiais
// (docs/Schemas/PL_010g e PL_010h): em <prod>, o bloco opcional [vUnComLiq, vProdLiq] vem
// entre vProd e cEANTrib; vProdLiqTot é o 1º filho de <total>; ICMSPrevistoPagtoAntecip é
// uma das alternativas do choice de <ICMS> (substitui o ICMSxx).

const (
	nt008ItemAntes   = `</vProd><cEANTrib>`
	nt008ItemDepois  = `</vProd><vUnComLiq>1.0000000000</vUnComLiq><vProdLiq>9.99</vProdLiq><cEANTrib>`
	nt008ICMS00Item1 = `<ICMS><ICMS00><orig>2</orig><CST>00</CST><modBC>3</modBC><vBC>2590.27</vBC><pICMS>4.0000</pICMS><vICMS>103.61</vICMS></ICMS00></ICMS>`
	nt008ICMSPrev    = `<ICMS><ICMSPrevistoPagtoAntecip><vICMSPrevisto>0.00</vICMSPrevisto></ICMSPrevistoPagtoAntecip></ICMS>`
)

// xmlComTagsNT008 parte do XML de exemplo (layout anterior à NT) e injeta as tags novas.
func xmlComTagsNT008(t *testing.T, pagtoAntecipado bool) []byte {
	t.Helper()
	s := string(readTestdata(t, "sample_nfe55_sim2027.xml"))
	if strings.Count(s, nt008ItemAntes) != 4 {
		t.Fatalf("fixture mudou: esperava 4 itens com %q", nt008ItemAntes)
	}
	s = strings.ReplaceAll(s, nt008ItemAntes, nt008ItemDepois)
	// Posição conforme o XSD oficial (PL_010h): vProdLiqTot é o 1º filho de <total>, antes de ICMSTot.
	s = strings.Replace(s, `<total>`, `<total><vProdLiqTot>39.96</vProdLiqTot>`, 1)
	if pagtoAntecipado {
		if !strings.Contains(s, nt008ICMS00Item1) {
			t.Fatalf("fixture mudou: ICMS00 do item 1 não encontrado")
		}
		s = strings.Replace(s, nt008ICMS00Item1, nt008ICMSPrev, 1)
	}
	return []byte(s)
}

func TestNT2026008_SimulacaoImportaEPreservaTagsNovas(t *testing.T) {
	original := xmlComTagsNT008(t, false)

	proc, err := pfsParseNFe(original)
	if err != nil {
		t.Fatalf("pfsParseNFe falhou com as tags da NT 2026.008: %v", err)
	}
	if len(proc.NFe.InfNFe.Det) != 4 {
		t.Fatalf("esperava 4 itens, veio %d", len(proc.NFe.InfNFe.Det))
	}

	params := pfsParametros{AliqIbsUfPct: 0.05, AliqIbsMunPct: 0.05, AliqCbsPct: 9.90}
	out, err := pfsGerarXMLSimulado(original, params, time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("pfsGerarXMLSimulado falhou: %v", err)
	}
	var generic struct{ XMLName xml.Name }
	if err := xml.Unmarshal(out, &generic); err != nil {
		t.Fatalf("XML gerado não é bem-formado: %v", err)
	}
	saida := string(out)

	// As tags novas passam inalteradas — a simulação zera PIS/COFINS/IPI e recalcula IBS/CBS,
	// mas o valor líquido (sem tributos) não depende de nenhum deles.
	if n := strings.Count(saida, "<vProdLiq>9.99</vProdLiq>"); n != 4 {
		t.Errorf("esperava 4 <vProdLiq> preservados, veio %d", n)
	}
	if n := strings.Count(saida, "<vUnComLiq>1.0000000000</vUnComLiq>"); n != 4 {
		t.Errorf("esperava 4 <vUnComLiq> preservados, veio %d", n)
	}
	if !strings.Contains(saida, "<vProdLiqTot>39.96</vProdLiqTot>") {
		t.Error("<vProdLiqTot> deveria ser preservado")
	}
	// E o cálculo de IBS/CBS segue funcionando (CBS do item 1: 2724.95 * 9,90% = 269.77)
	if !strings.Contains(saida, "<vCBS>269.77</vCBS>") {
		t.Error("IBS/CBS deveriam continuar sendo recalculados normalmente")
	}
}

func TestNT2026008_ImportPersistidoToleraTagsNovas(t *testing.T) {
	for _, pagto := range []bool{false, true} {
		nome := "nota normal"
		if pagto {
			nome = "pagamento antecipado (ICMSPrevistoPagtoAntecip no lugar do ICMSxx)"
		}
		t.Run(nome, func(t *testing.T) {
			proc, err := pfParseNFeXML(xmlComTagsNT008(t, pagto))
			if err != nil {
				t.Fatalf("pfParseNFeXML falhou com as tags da NT 2026.008: %v", err)
			}
			dets := proc.NFe.InfNFe.Det
			if len(dets) != 4 {
				t.Fatalf("esperava 4 itens, veio %d", len(dets))
			}
			// vProd (bruto) segue sendo lido do item — vProdLiq não o substitui.
			if got := toDecimal(dets[0].Prod.VProd); got != 2590.27 {
				t.Errorf("vProd do item 1 = %v, esperava 2590.27", got)
			}
			g := firstICMSGrupo(dets[0].Imposto.ICMS.Grupos)
			if pagto {
				// Hoje o grupo novo cai no ",any" como um "ICMS" vazio: não quebra nem
				// inventa valores, só não há CST/base de ICMS para esse item (o que é o
				// esperado numa nota de pagamento antecipado).
				if g == nil || g.CST != "" || g.VICMS != "" {
					t.Errorf("grupo ICMSPrevistoPagtoAntecip deveria virar um grupo ICMS vazio, veio %+v", g)
				}
			} else if g == nil || g.CST != "00" || toDecimal(g.VICMS) != 103.61 {
				t.Errorf("ICMS00 do item 1 deveria continuar sendo lido: %+v", g)
			}
			// Os demais itens não são afetados.
			if g2 := firstICMSGrupo(dets[1].Imposto.ICMS.Grupos); g2 == nil || g2.CST != "00" {
				t.Errorf("ICMS do item 2 deveria continuar sendo lido: %+v", g2)
			}
		})
	}
}

func TestNT2026008_SimulacaoPagamentoAntecipado(t *testing.T) {
	original := xmlComTagsNT008(t, true)
	params := pfsParametros{AliqIbsUfPct: 0.05, AliqIbsMunPct: 0.05, AliqCbsPct: 9.90}
	out, err := pfsGerarXMLSimulado(original, params, time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("pfsGerarXMLSimulado falhou: %v", err)
	}
	if !strings.Contains(string(out), nt008ICMSPrev) {
		t.Error("o grupo ICMSPrevistoPagtoAntecip/vICMSPrevisto deveria passar inalterado")
	}
	var generic struct{ XMLName xml.Name }
	if err := xml.Unmarshal(out, &generic); err != nil {
		t.Fatalf("XML gerado não é bem-formado: %v", err)
	}
}
