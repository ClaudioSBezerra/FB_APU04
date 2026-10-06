package handlers

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"
)

func TestPfsGerarXMLSimulado_NFe(t *testing.T) {
	original := readTestdata(t, "sample_nfe55_sim2027.xml")
	params := pfsParametros{AliqIbsUfPct: 0.05, AliqIbsMunPct: 0.05, AliqCbsPct: 9.90}

	out, err := pfsGerarXMLSimulado(original, params, time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("erro ao gerar XML simulado: %v", err)
	}
	outStr := string(out)

	// Deve continuar bem-formado
	var generic struct {
		XMLName xml.Name
	}
	if err := xml.Unmarshal(out, &generic); err != nil {
		t.Fatalf("XML gerado não é bem-formado: %v", err)
	}

	// Assinatura original MANTIDA (removê-la quebra o schema — rejeição real
	// 225 na homologação SEFAZ 2026-10-06) e com o namespace xmldsig restaurado
	if !strings.Contains(outStr, `<Signature xmlns="http://www.w3.org/2000/09/xmldsig#">`) {
		t.Error("o XML gerado deveria manter <Signature> com o namespace xmldsig")
	}
	if !strings.Contains(outStr, "<DigestValue>C8hXkDh2THS33NWTE7EwVQh6+cs=</DigestValue>") {
		t.Error("o conteúdo original da assinatura deveria ser preservado (mesmo desatualizado)")
	}
	// xmlns da NF-e restaurado na raiz
	if !strings.Contains(outStr, `xmlns="http://www.portalfiscal.inf.br/nfe"`) {
		t.Error("esperava xmlns da NF-e na raiz do XML gerado")
	}
	// Marcador de simulação presente
	if !strings.Contains(outStr, "SIMULACAO 2027") {
		t.Error("esperava comentário de marcação SIMULACAO 2027")
	}

	// ICMSTot intocado (todos os valores originais do bloco devem continuar
	// presentes exatamente como no XML de entrada)
	for _, orig := range []string{"<vBC>3488.68</vBC>", "<vICMS>139.55</vICMS>", "<vIPI>181.01</vIPI>", "<vPIS>55.27</vPIS>", "<vCOFINS>254.53</vCOFINS>", "<vNF>3669.69</vNF>"} {
		if !strings.Contains(outStr, orig) {
			t.Errorf("ICMSTot não deveria mudar — esperava encontrar %q no resultado", orig)
		}
	}
	// Duplicata intocada
	if !strings.Contains(outStr, "<vDup>3669.69</vDup>") {
		t.Error("cobr/dup não deveria mudar")
	}

	// Reparse do resultado para validar os valores recalculados
	proc, err := pfsParseNFe(out)
	if err != nil {
		t.Fatalf("erro ao reparsear XML gerado: %v", err)
	}
	inf := proc.NFe.InfNFe
	if len(inf.Det) != 4 {
		t.Fatalf("esperava 4 itens no resultado, veio %d", len(inf.Det))
	}

	item1 := inf.Det[0].Imposto
	if toDecimal(item1.IPI.IPITrib.VIPI) != 0 || toDecimal(item1.IPI.IPITrib.VBC) != 0 || toDecimal(item1.IPI.IPITrib.PIPI) != 0 {
		t.Errorf("IPI do item 1 deveria estar zerado: %+v", item1.IPI.IPITrib)
	}
	if toDecimal(item1.PIS.PISAliq.VPIS) != 0 || toDecimal(item1.PIS.PISAliq.VBC) != 0 {
		t.Errorf("PIS do item 1 deveria estar zerado: %+v", item1.PIS.PISAliq)
	}
	if toDecimal(item1.COFINS.COFINSAliq.VCOFINS) != 0 {
		t.Errorf("COFINS do item 1 deveria estar zerado: %+v", item1.COFINS.COFINSAliq)
	}
	// Base do IBS/CBS mantida (2724.95), valores recalculados
	if toDecimal(item1.IBSCBS.GIBSCBS.VBC) != 2724.95 {
		t.Errorf("vBC do IBSCBS não deveria mudar, veio %v", item1.IBSCBS.GIBSCBS.VBC)
	}
	if toDecimal(item1.IBSCBS.GIBSCBS.GIBSUF.VIBSUF) != 1.36 {
		t.Errorf("vIBSUF recalculado = %v, want 1.36", item1.IBSCBS.GIBSCBS.GIBSUF.VIBSUF)
	}
	if toDecimal(item1.IBSCBS.GIBSCBS.GCBS.VCBS) != 269.77 {
		t.Errorf("vCBS recalculado = %v, want 269.77", item1.IBSCBS.GIBSCBS.GCBS.VCBS)
	}

	// IBSCBSTot recalculado como soma dos itens
	if !strings.Contains(outStr, "<IBSCBSTot>") {
		t.Fatal("IBSCBSTot ausente no resultado")
	}
}

func TestPfsGerarXMLSimulado_CTe(t *testing.T) {
	original := readTestdata(t, "sample_cte_sim2027.xml")
	params := pfsParametros{AliqIbsUfPct: 0.05, AliqIbsMunPct: 0.05, AliqCbsPct: 9.90}

	out, err := pfsGerarXMLSimulado(original, params, time.Now())
	if err != nil {
		t.Fatalf("erro ao gerar XML simulado de CT-e: %v", err)
	}
	outStr := string(out)

	if !strings.Contains(outStr, `<Signature xmlns="http://www.w3.org/2000/09/xmldsig#">`) {
		t.Error("o XML gerado deveria manter <Signature> com o namespace xmldsig")
	}
	if !strings.Contains(outStr, `xmlns="http://www.portalfiscal.inf.br/cte"`) {
		t.Error("esperava xmlns do CT-e na raiz do XML gerado")
	}
	if !strings.Contains(outStr, "<vTPrest>2747.18</vTPrest>") || !strings.Contains(outStr, "<vDup>2747.18</vDup>") {
		t.Error("vTPrest/duplicata do CT-e não deveriam mudar")
	}

	proc, err := pfsParseCTe(out)
	if err != nil {
		t.Fatalf("erro ao reparsear CT-e gerado: %v", err)
	}
	ib := proc.CTe.InfCte.Imp.IBSCBS.GIBSCBS
	if toDecimal(ib.VBC) != 2747.18 {
		t.Errorf("vBC não deveria mudar, veio %v", ib.VBC)
	}
	if toDecimal(ib.GCBS.VCBS) != 271.97 {
		t.Errorf("vCBS recalculado = %v, want 271.97", ib.GCBS.VCBS)
	}
}

func TestPfsGerarXMLSimulado_TipoInvalido(t *testing.T) {
	_, err := pfsGerarXMLSimulado([]byte("<outraCoisa></outraCoisa>"), pfsParametros{}, time.Now())
	// Documento sem IBSCBS/IPI/PIS/COFINS simplesmente não sofre nenhuma
	// substituição — não deve dar erro, só não altera nada relevante.
	if err != nil {
		t.Fatalf("não deveria falhar com XML genérico bem-formado: %v", err)
	}
}
