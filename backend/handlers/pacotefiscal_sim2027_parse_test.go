package handlers

import (
	"os"
	"testing"
)

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("erro ao ler fixture %s: %v", name, err)
	}
	return data
}

func TestPfsDetectTipoDoc(t *testing.T) {
	nfe := readTestdata(t, "sample_nfe55_sim2027.xml")
	if got := pfsDetectTipoDoc(nfe); got != pfsTipoNFe {
		t.Errorf("tipo da NF-e = %q, want %q", got, pfsTipoNFe)
	}

	cte := readTestdata(t, "sample_cte_sim2027.xml")
	if got := pfsDetectTipoDoc(cte); got != pfsTipoCTe {
		t.Errorf("tipo do CT-e = %q, want %q", got, pfsTipoCTe)
	}

	if got := pfsDetectTipoDoc([]byte("<outraCoisa></outraCoisa>")); got != pfsTipoDesconhec {
		t.Errorf("tipo desconhecido deveria voltar vazio, veio %q", got)
	}
}

func TestPfsParseNFe(t *testing.T) {
	data := readTestdata(t, "sample_nfe55_sim2027.xml")
	proc, err := pfsParseNFe(data)
	if err != nil {
		t.Fatalf("erro ao parsear NF-e: %v", err)
	}

	inf := proc.NFe.InfNFe
	if inf.Ide.Mod != "55" {
		t.Errorf("mod = %q, want 55", inf.Ide.Mod)
	}
	if inf.Ide.NNF != "181582" {
		t.Errorf("nNF = %q, want 181582", inf.Ide.NNF)
	}
	if chave := pfsExtractChaveNFe(proc); chave != "32260612449448000176550010001815821291031700" {
		t.Errorf("chave = %q", chave)
	}
	if len(inf.Det) != 4 {
		t.Fatalf("esperava 4 itens, veio %d", len(inf.Det))
	}

	item1 := inf.Det[0]
	if toDecimal(item1.Imposto.IBSCBS.GIBSCBS.VBC) != 2724.95 {
		t.Errorf("item1 vBC IBSCBS = %v, want 2724.95", item1.Imposto.IBSCBS.GIBSCBS.VBC)
	}
	if toDecimal(item1.Imposto.IBSCBS.GIBSCBS.GIBSUF.PIBSUF) != 0.1 {
		t.Errorf("item1 pIBSUF = %v, want 0.1", item1.Imposto.IBSCBS.GIBSCBS.GIBSUF.PIBSUF)
	}
	if toDecimal(item1.Imposto.IPI.IPITrib.VIPI) != 134.68 {
		t.Errorf("item1 vIPI = %v, want 134.68", item1.Imposto.IPI.IPITrib.VIPI)
	}
	if toDecimal(item1.Imposto.PIS.PISAliq.VPIS) != 41.03 {
		t.Errorf("item1 vPIS = %v, want 41.03", item1.Imposto.PIS.PISAliq.VPIS)
	}
	if toDecimal(item1.Imposto.COFINS.COFINSAliq.VCOFINS) != 188.99 {
		t.Errorf("item1 vCOFINS = %v, want 188.99", item1.Imposto.COFINS.COFINSAliq.VCOFINS)
	}

	if toDecimal(proc.NFe.InfNFe.Total.IBSCBSTot.VBCIBSCBS) != 3669.69 {
		t.Errorf("vBCIBSCBS total = %v, want 3669.69", proc.NFe.InfNFe.Total.IBSCBSTot.VBCIBSCBS)
	}
}

func TestPfsParseCTe(t *testing.T) {
	data := readTestdata(t, "sample_cte_sim2027.xml")
	proc, err := pfsParseCTe(data)
	if err != nil {
		t.Fatalf("erro ao parsear CT-e: %v", err)
	}

	inf := proc.CTe.InfCte
	if inf.Ide.Mod != "57" {
		t.Errorf("mod = %q, want 57", inf.Ide.Mod)
	}
	if inf.Ide.NCT != "20135" {
		t.Errorf("nCT = %q, want 20135", inf.Ide.NCT)
	}
	if chave := pfsExtractChaveCTe(proc); chave != "26260745451307000147570010000201351577551272" {
		t.Errorf("chave = %q", chave)
	}
	if toDecimal(inf.Imp.IBSCBS.GIBSCBS.VBC) != 2747.18 {
		t.Errorf("vBC IBSCBS = %v, want 2747.18", inf.Imp.IBSCBS.GIBSCBS.VBC)
	}
	if toDecimal(inf.Imp.IBSCBS.GIBSCBS.GCBS.VCBS) != 24.72 {
		t.Errorf("vCBS = %v, want 24.72", inf.Imp.IBSCBS.GIBSCBS.GCBS.VCBS)
	}
}
