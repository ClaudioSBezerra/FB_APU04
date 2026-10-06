// pacotefiscal_sim2027_parse.go — parsing ISOLADO para a 4ª funcionalidade do
// módulo "Teste Pacote Fiscal": "Simulação 2027". Mesma doutrina de isolamento
// de pacotefiscal_xml_import.go — não reaproveita os tipos de nfe_saidas.go
// nem de cte_entradas.go, só helpers genéricos já testados (nfeCharsetReader,
// convertWindows1252, toDecimal, round2).
//
// Este fluxo é STATELESS: não grava nada em banco. Lê o XML enviado (NF-e
// modelo 55 ou CT-e modelo 57), calcula a simulação e devolve — por isso os
// structs aqui cobrem campos que pacotefiscal_xml_import.go não precisa
// persistir (pIBSUF/pIBSMun/pCBS, variantes de PIS/COFINS/IPI, campos de
// IBSCBSTot) mas que são necessários para regerar o XML fielmente.
package handlers

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// NF-e — structs de leitura (prefixo pfs = pacote fiscal sim2027)
// ---------------------------------------------------------------------------

type pfsNfeProc struct {
	XMLName xml.Name `xml:"nfeProc"`
	NFe     pfsNfe   `xml:"NFe"`
	ProtNFe pfsProtN `xml:"protNFe"`
}

type pfsNfe struct {
	InfNFe pfsInfNFe `xml:"infNFe"`
}

type pfsProtN struct {
	InfProt struct {
		ChNFe string `xml:"chNFe"`
	} `xml:"infProt"`
}

type pfsInfNFe struct {
	ID    string    `xml:"Id,attr"`
	Ide   pfsIdeNFe `xml:"ide"`
	Det   []pfsDet  `xml:"det"`
	Total pfsTotal  `xml:"total"`
}

type pfsIdeNFe struct {
	Mod   string `xml:"mod"`
	Serie string `xml:"serie"`
	NNF   string `xml:"nNF"`
	DhEmi string `xml:"dhEmi"`
}

type pfsDet struct {
	NItem   string        `xml:"nItem,attr"`
	Prod    pfsProd       `xml:"prod"`
	Imposto pfsImpostoNFe `xml:"imposto"`
}

type pfsProd struct {
	CProd string `xml:"cProd"`
	XProd string `xml:"xProd"`
}

type pfsImpostoNFe struct {
	IPI    pfsIPI       `xml:"IPI"`
	PIS    pfsPIS       `xml:"PIS"`
	COFINS pfsCOFINS    `xml:"COFINS"`
	IBSCBS pfsDetIBSCBS `xml:"IBSCBS"`
}

// pfsIPI — cobre as variantes mais comuns (IPITrib tributado / IPINT não
// tributado); só IPITrib carrega valores numéricos a zerar.
type pfsIPI struct {
	IPITrib struct {
		CST  string `xml:"CST"`
		VBC  string `xml:"vBC"`
		PIPI string `xml:"pIPI"`
		VIPI string `xml:"vIPI"`
	} `xml:"IPITrib"`
}

// pfsPIS — cobre PISAliq (% sobre valor) e PISQtde (valor por quantidade);
// PISNT/PISOutr não têm vBC/pPIS/vPIS a zerar (isentos/outras operações).
type pfsPIS struct {
	PISAliq struct {
		CST  string `xml:"CST"`
		VBC  string `xml:"vBC"`
		PPIS string `xml:"pPIS"`
		VPIS string `xml:"vPIS"`
	} `xml:"PISAliq"`
	PISQtde struct {
		CST  string `xml:"CST"`
		VPIS string `xml:"vPIS"`
	} `xml:"PISQtde"`
}

type pfsCOFINS struct {
	COFINSAliq struct {
		CST     string `xml:"CST"`
		VBC     string `xml:"vBC"`
		PCOFINS string `xml:"pCOFINS"`
		VCOFINS string `xml:"vCOFINS"`
	} `xml:"COFINSAliq"`
	COFINSQtde struct {
		CST     string `xml:"CST"`
		VCOFINS string `xml:"vCOFINS"`
	} `xml:"COFINSQtde"`
}

type pfsDetIBSCBS struct {
	CST        string        `xml:"CST"`
	CClassTrib string        `xml:"cClassTrib"`
	GIBSCBS    pfsDetGIBSCBS `xml:"gIBSCBS"`
}

type pfsDetGIBSCBS struct {
	VBC    string `xml:"vBC"`
	GIBSUF struct {
		PIBSUF string `xml:"pIBSUF"`
		VIBSUF string `xml:"vIBSUF"`
	} `xml:"gIBSUF"`
	GIBSMun struct {
		PIBSMun string `xml:"pIBSMun"`
		VIBSMun string `xml:"vIBSMun"`
	} `xml:"gIBSMun"`
	VIBS string `xml:"vIBS"`
	GCBS struct {
		PCBS string `xml:"pCBS"`
		VCBS string `xml:"vCBS"`
	} `xml:"gCBS"`
}

type pfsTotal struct {
	IBSCBSTot pfsIBSCBSTot `xml:"IBSCBSTot"`
}

type pfsIBSCBSTot struct {
	VBCIBSCBS string `xml:"vBCIBSCBS"`
}

// ---------------------------------------------------------------------------
// CT-e — structs novos e próprios deste fluxo (não reaproveita cte_entradas.go)
// ---------------------------------------------------------------------------

type pfsCteProc struct {
	XMLName xml.Name  `xml:"cteProc"`
	CTe     pfsCteDoc `xml:"CTe"`
	ProtCTe pfsProtC  `xml:"protCTe"`
}

type pfsCteDoc struct {
	InfCte pfsInfCte `xml:"infCte"`
}

type pfsProtC struct {
	InfProt struct {
		ChCTe string `xml:"chCTe"`
	} `xml:"infProt"`
}

type pfsInfCte struct {
	ID  string    `xml:"Id,attr"`
	Ide pfsIdeCTe `xml:"ide"`
	Imp pfsImpCTe `xml:"imp"`
}

type pfsIdeCTe struct {
	Mod   string `xml:"mod"`
	Serie string `xml:"serie"`
	NCT   string `xml:"nCT"`
	DhEmi string `xml:"dhEmi"`
}

// pfsImpCTe — CT-e não tem <det>/PIS/COFINS/IPI no schema padrão; o bloco
// IBSCBS aparece uma única vez, diretamente sob <imp> (documento inteiro é
// uma só "linha" de serviço).
type pfsImpCTe struct {
	IBSCBS pfsDetIBSCBS `xml:"IBSCBS"`
}

// ---------------------------------------------------------------------------
// Detecção de tipo de documento + parsing
// ---------------------------------------------------------------------------

type pfsTipoDoc string

const (
	pfsTipoNFe       pfsTipoDoc = "nfe"
	pfsTipoCTe       pfsTipoDoc = "cte"
	pfsTipoDesconhec pfsTipoDoc = ""
)

// pfsStripNamespaces remove declarações xmlns (nfe/cte) para simplificar o
// parsing — mesma técnica de stripNFeNamespaces, generalizada para os dois
// namespaces usados por este módulo.
func pfsStripNamespaces(data []byte) []byte {
	s := string(data)
	replacements := []string{
		` xmlns="http://www.portalfiscal.inf.br/nfe"`, "",
		` xmlns='http://www.portalfiscal.inf.br/nfe'`, "",
		` xmlns="http://www.portalfiscal.inf.br/cte"`, "",
		` xmlns='http://www.portalfiscal.inf.br/cte'`, "",
		` xmlns="http://www.w3.org/2000/09/xmldsig#"`, "",
	}
	for i := 0; i < len(replacements); i += 2 {
		s = strings.ReplaceAll(s, replacements[i], replacements[i+1])
	}
	return []byte(s)
}

// pfsDetectTipoDoc inspeciona a tag raiz do XML (ignorando o prólogo) para
// decidir entre NF-e e CT-e sem precisar decodificar o documento inteiro.
func pfsDetectTipoDoc(data []byte) pfsTipoDoc {
	trimmed := bytes.TrimSpace(data)
	// Pula o prólogo <?xml ...?> se presente
	if idx := bytes.Index(trimmed, []byte("?>")); idx >= 0 && bytes.HasPrefix(trimmed, []byte("<?xml")) {
		trimmed = bytes.TrimSpace(trimmed[idx+2:])
	}
	switch {
	case bytes.HasPrefix(trimmed, []byte("<nfeProc")), bytes.HasPrefix(trimmed, []byte("<NFe")):
		return pfsTipoNFe
	case bytes.HasPrefix(trimmed, []byte("<cteProc")), bytes.HasPrefix(trimmed, []byte("<CTe")):
		return pfsTipoCTe
	default:
		return pfsTipoDesconhec
	}
}

func pfsParseNFe(data []byte) (*pfsNfeProc, error) {
	data = pfsStripNamespaces(data)
	trimmed := strings.TrimSpace(string(data))
	if strings.HasPrefix(trimmed, "<NFe") {
		data = []byte("<nfeProc>" + trimmed + "</nfeProc>")
	}

	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = nfeCharsetReader
	var proc pfsNfeProc
	if err := dec.Decode(&proc); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("arquivo truncado ou incompleto")
		}
		converted, convErr := convertWindows1252(data)
		if convErr != nil {
			return nil, fmt.Errorf("XML inválido: %w", err)
		}
		dec2 := xml.NewDecoder(bytes.NewReader(converted))
		dec2.CharsetReader = nfeCharsetReader
		var proc2 pfsNfeProc
		if err2 := dec2.Decode(&proc2); err2 != nil {
			return nil, fmt.Errorf("XML inválido (encoding não reconhecido): %w", err2)
		}
		return &proc2, nil
	}
	return &proc, nil
}

func pfsParseCTe(data []byte) (*pfsCteProc, error) {
	data = pfsStripNamespaces(data)
	trimmed := strings.TrimSpace(string(data))
	if strings.HasPrefix(trimmed, "<CTe") {
		data = []byte("<cteProc>" + trimmed + "</cteProc>")
	}

	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = nfeCharsetReader
	var proc pfsCteProc
	if err := dec.Decode(&proc); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("arquivo truncado ou incompleto")
		}
		converted, convErr := convertWindows1252(data)
		if convErr != nil {
			return nil, fmt.Errorf("XML inválido: %w", err)
		}
		dec2 := xml.NewDecoder(bytes.NewReader(converted))
		dec2.CharsetReader = nfeCharsetReader
		var proc2 pfsCteProc
		if err2 := dec2.Decode(&proc2); err2 != nil {
			return nil, fmt.Errorf("XML inválido (encoding não reconhecido): %w", err2)
		}
		return &proc2, nil
	}
	return &proc, nil
}

func pfsExtractChaveNFe(proc *pfsNfeProc) string {
	ch := strings.TrimSpace(proc.ProtNFe.InfProt.ChNFe)
	if len(ch) == 44 {
		return ch
	}
	id := strings.TrimSpace(proc.NFe.InfNFe.ID)
	if strings.HasPrefix(id, "NFe") && len(id) == 47 {
		return id[3:]
	}
	return ""
}

func pfsExtractChaveCTe(proc *pfsCteProc) string {
	ch := strings.TrimSpace(proc.ProtCTe.InfProt.ChCTe)
	if len(ch) == 44 {
		return ch
	}
	id := strings.TrimSpace(proc.CTe.InfCte.ID)
	if strings.HasPrefix(id, "CTe") && len(id) == 47 {
		return id[3:]
	}
	return ""
}

func pfsAtoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}
