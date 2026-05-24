package handlers

// cte_parser_test.go — cobertura para o parser de CT-e (parseCTeXML)
// focando nos campos adicionados pela migration 105:
//   - <ide><toma3><toma>X</toma></toma3>   (X ∈ 0,1,2,3)
//   - <ide><toma4><toma>4</toma><CNPJ>…    (caso "Outros")
//   - <infCTeNorm><infDoc><infNFe><chave>  (referências de NF-e)

import (
	"strings"
	"testing"
)

// cteXMLToma3 produz um CT-e mínimo com toma3.toma = X.
func cteXMLToma3(toma string) []byte {
	tpl := `<?xml version="1.0" encoding="UTF-8"?>
<cteProc xmlns="http://www.portalfiscal.inf.br/cte" versao="4.00">
  <CTe><infCte Id="CTe35260112345678901234567890123456789012345678">
    <ide>
      <mod>57</mod><serie>1</serie><nCT>123</nCT>
      <dhEmi>2026-04-15T10:00:00-03:00</dhEmi>
      <natOp>PRESTACAO DE SERVICO DE TRANSPORTE</natOp>
      <CFOP>5353</CFOP><modal>01</modal>
      <toma3><toma>__TOMA__</toma></toma3>
    </ide>
    <emit><CNPJ>11111111000111</CNPJ><xNome>TRANSP X</xNome>
      <enderEmit><UF>SP</UF></enderEmit></emit>
    <rem><CNPJ>22222222000122</CNPJ><xNome>REM</xNome>
      <enderReme><UF>SC</UF></enderReme></rem>
    <dest><CNPJ>33333333000133</CNPJ><xNome>DEST</xNome>
      <enderDest><UF>PE</UF></enderDest></dest>
    <vPrest><vTPrest>100.00</vTPrest><vRec>100.00</vRec></vPrest>
    <imp><ICMS><ICMS00><vBC>100.00</vBC><vICMS>12.00</vICMS></ICMS00></ICMS></imp>
    <infCTeNorm>
      <infCarga><vCarga>1000.00</vCarga></infCarga>
      <infDoc>
        <infNFe><chave>44444444000144555010000000010000000010000000</chave></infNFe>
      </infDoc>
    </infCTeNorm>
  </infCte></CTe>
  <protCTe><infProt><chCTe>35260112345678901234567890123456789012345678</chCTe></infProt></protCTe>
</cteProc>`
	return []byte(strings.Replace(tpl, "__TOMA__", toma, 1))
}

// cteXMLToma4 produz um CT-e com toma4 (Outros) + CNPJ explícito.
func cteXMLToma4(cnpj string) []byte {
	tpl := `<?xml version="1.0" encoding="UTF-8"?>
<cteProc xmlns="http://www.portalfiscal.inf.br/cte" versao="4.00">
  <CTe><infCte Id="CTe35260112345678901234567890123456789012345678">
    <ide>
      <mod>57</mod><serie>1</serie><nCT>123</nCT>
      <dhEmi>2026-04-15T10:00:00-03:00</dhEmi>
      <natOp>PRESTACAO DE SERVICO DE TRANSPORTE</natOp>
      <CFOP>5353</CFOP><modal>01</modal>
      <toma4><toma>4</toma><CNPJ>__CNPJ__</CNPJ></toma4>
    </ide>
    <emit><CNPJ>11111111000111</CNPJ><xNome>TRANSP X</xNome>
      <enderEmit><UF>SP</UF></enderEmit></emit>
    <rem><CNPJ>22222222000122</CNPJ><xNome>REM</xNome></rem>
    <dest><CNPJ>33333333000133</CNPJ><xNome>DEST</xNome></dest>
    <vPrest><vTPrest>50.00</vTPrest><vRec>50.00</vRec></vPrest>
    <imp><ICMS><ICMS00><vBC>50.00</vBC><vICMS>6.00</vICMS></ICMS00></ICMS></imp>
    <infCTeNorm>
      <infCarga><vCarga>500.00</vCarga></infCarga>
      <infDoc></infDoc>
    </infCTeNorm>
  </infCte></CTe>
</cteProc>`
	return []byte(strings.Replace(tpl, "__CNPJ__", cnpj, 1))
}

func TestParseCTe_Toma3_Destinatario(t *testing.T) {
	proc, err := parseCTeXML(cteXMLToma3("3"))
	if err != nil {
		t.Fatalf("parseCTeXML toma3=3: %v", err)
	}
	if got := proc.CTe.InfCte.Ide.Toma3.Toma; got != "3" {
		t.Errorf("Toma3.Toma: got %q, want %q", got, "3")
	}
	// toma4 vazio
	if got := proc.CTe.InfCte.Ide.Toma4.Toma; got != "" {
		t.Errorf("Toma4.Toma: got %q, want empty", got)
	}
}

func TestParseCTe_Toma3_Remetente(t *testing.T) {
	proc, err := parseCTeXML(cteXMLToma3("0"))
	if err != nil {
		t.Fatalf("parseCTeXML toma3=0: %v", err)
	}
	if got := proc.CTe.InfCte.Ide.Toma3.Toma; got != "0" {
		t.Errorf("Toma3.Toma: got %q, want %q", got, "0")
	}
}

func TestParseCTe_Toma4_CNPJ(t *testing.T) {
	proc, err := parseCTeXML(cteXMLToma4("99999999000199"))
	if err != nil {
		t.Fatalf("parseCTeXML toma4: %v", err)
	}
	if got := proc.CTe.InfCte.Ide.Toma4.Toma; got != "4" {
		t.Errorf("Toma4.Toma: got %q, want %q", got, "4")
	}
	if got := proc.CTe.InfCte.Ide.Toma4.CNPJ; got != "99999999000199" {
		t.Errorf("Toma4.CNPJ: got %q, want %q", got, "99999999000199")
	}
}

func TestParseCTe_InfNFe_ChaveExtraida(t *testing.T) {
	proc, err := parseCTeXML(cteXMLToma3("3"))
	if err != nil {
		t.Fatalf("parseCTeXML: %v", err)
	}
	refs := proc.CTe.InfCte.InfCTeNorm.InfDoc.InfNFe
	if len(refs) != 1 {
		t.Fatalf("InfNFe: got %d refs, want 1", len(refs))
	}
	want := "44444444000144555010000000010000000010000000"
	if got := refs[0].ChNFe; got != want {
		t.Errorf("InfNFe[0].ChNFe (tag <chave>): got %q, want %q", got, want)
	}
}

func TestParseCTe_DadosBasicos(t *testing.T) {
	proc, err := parseCTeXML(cteXMLToma3("3"))
	if err != nil {
		t.Fatalf("parseCTeXML: %v", err)
	}
	inf := proc.CTe.InfCte
	if inf.Ide.Mod != "57" {
		t.Errorf("Mod: got %q, want 57", inf.Ide.Mod)
	}
	if inf.Ide.NCT != "123" {
		t.Errorf("NCT: got %q, want 123", inf.Ide.NCT)
	}
	if inf.Emit.CNPJ != "11111111000111" {
		t.Errorf("Emit.CNPJ: got %q", inf.Emit.CNPJ)
	}
	if inf.Rem.EnderReme.UF != "SC" {
		t.Errorf("Rem.UF: got %q, want SC", inf.Rem.EnderReme.UF)
	}
	if inf.Dest.EnderDest.UF != "PE" {
		t.Errorf("Dest.UF: got %q, want PE", inf.Dest.EnderDest.UF)
	}
}

func TestParseCTe_InvalidoNaoPanica(t *testing.T) {
	_, err := parseCTeXML([]byte("<not-xml>"))
	if err == nil {
		t.Error("parseCTeXML: esperava erro com XML inválido")
	}
}
