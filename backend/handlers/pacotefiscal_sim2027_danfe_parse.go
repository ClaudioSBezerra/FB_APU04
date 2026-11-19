// pacotefiscal_sim2027_danfe_parse.go — leitura do XML da NF-e (modelo 55) para o DANFE da
// Simulação 2027, e helpers de formatação (padrão brasileiro). Isolado dos outros módulos,
// mesma doutrina de pacotefiscal_sim2027_parse.go: structs próprios com prefixo "dfe".
//
// Os campos opcionais do DANFE (FCP, DIFAL, monofásicos, ISSQN, IS...) são strings: vazio
// significa "não existe no XML" — a NT 2026.010 proíbe imprimir/inferir informação que o XML
// não traz, então o renderizador só desenha o que veio preenchido.
package handlers

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

type dfeEnder struct {
	XLgr    string `xml:"xLgr"`
	Nro     string `xml:"nro"`
	XCpl    string `xml:"xCpl"`
	XBairro string `xml:"xBairro"`
	CMun    string `xml:"cMun"`
	XMun    string `xml:"xMun"`
	UF      string `xml:"UF"`
	CEP     string `xml:"CEP"`
	Fone    string `xml:"fone"`
}

type dfeNfeProc struct {
	XMLName xml.Name `xml:"nfeProc"`
	NFe     dfeNFe   `xml:"NFe"`
	Prot    struct {
		InfProt struct {
			ChNFe    string `xml:"chNFe"`
			NProt    string `xml:"nProt"`
			DhRecbto string `xml:"dhRecbto"`
			CStat    string `xml:"cStat"`
		} `xml:"infProt"`
	} `xml:"protNFe"`
}

type dfeNFe struct {
	InfNFe dfeInfNFe `xml:"infNFe"`
	Supl   struct {
		QrCode string `xml:"qrCode"`
	} `xml:"infNFeSupl"`
}

type dfeInfNFe struct {
	ID      string     `xml:"Id,attr"`
	Ide     dfeIde     `xml:"ide"`
	Emit    dfeEmit    `xml:"emit"`
	Dest    dfeDest    `xml:"dest"`
	Det     []dfeDet   `xml:"det"`
	Total   dfeTotal   `xml:"total"`
	Transp  dfeTransp  `xml:"transp"`
	Cobr    dfeCobr    `xml:"cobr"`
	InfAdic dfeInfAdic `xml:"infAdic"`
}

type dfeIde struct {
	Mod     string `xml:"mod"`
	Serie   string `xml:"serie"`
	NNF     string `xml:"nNF"`
	DhEmi   string `xml:"dhEmi"`
	DhSaiEn string `xml:"dhSaiEnt"`
	TpNF    string `xml:"tpNF"`
	NatOp   string `xml:"natOp"`
}

type dfeEmit struct {
	CNPJ      string   `xml:"CNPJ"`
	CPF       string   `xml:"CPF"`
	XNome     string   `xml:"xNome"`
	XFant     string   `xml:"xFant"`
	IE        string   `xml:"IE"`
	IEST      string   `xml:"IEST"`
	IM        string   `xml:"IM"`
	CRT       string   `xml:"CRT"`
	EnderEmit dfeEnder `xml:"enderEmit"`
}

type dfeDest struct {
	CNPJ          string   `xml:"CNPJ"`
	CPF           string   `xml:"CPF"`
	IDEstrangeiro string   `xml:"idEstrangeiro"`
	XNome         string   `xml:"xNome"`
	IE            string   `xml:"IE"`
	EnderDest     dfeEnder `xml:"enderDest"`
}

type dfeICMSGrupo struct {
	Orig  string `xml:"orig"`
	CST   string `xml:"CST"`
	CSOSN string `xml:"CSOSN"`
	VBC   string `xml:"vBC"`
	PICMS string `xml:"pICMS"`
	VICMS string `xml:"vICMS"`
}

type dfeGRed struct {
	PRedAliq  string `xml:"pRedAliq"`
	PAliqEfet string `xml:"pAliqEfet"`
}

type dfeDet struct {
	NItem string `xml:"nItem,attr"`
	Prod  struct {
		CProd string `xml:"cProd"`
		XProd string `xml:"xProd"`
		NCM   string `xml:"NCM"`
		CFOP  string `xml:"CFOP"`
		UCom  string `xml:"uCom"`
		QCom  string `xml:"qCom"`
		VUn   string `xml:"vUnCom"`
		VProd string `xml:"vProd"`
	} `xml:"prod"`
	Imposto struct {
		ICMS struct {
			Grupos []dfeICMSGrupo `xml:",any"`
		} `xml:"ICMS"`
		IPI struct {
			IPITrib struct {
				VBC  string `xml:"vBC"`
				PIPI string `xml:"pIPI"`
				VIPI string `xml:"vIPI"`
			} `xml:"IPITrib"`
		} `xml:"IPI"`
		IS struct {
			VBCIS string `xml:"vBCIS"`
			PIS   string `xml:"pIS"`
			VIS   string `xml:"vIS"`
		} `xml:"IS"`
		IBSCBS struct {
			CST        string `xml:"CST"`
			CClassTrib string `xml:"cClassTrib"`
			G          struct {
				VBC string `xml:"vBC"`
				UF  struct {
					P    string  `xml:"pIBSUF"`
					GRed dfeGRed `xml:"gRed"`
					V    string  `xml:"vIBSUF"`
				} `xml:"gIBSUF"`
				Mun struct {
					P    string  `xml:"pIBSMun"`
					GRed dfeGRed `xml:"gRed"`
					V    string  `xml:"vIBSMun"`
				} `xml:"gIBSMun"`
				CBS struct {
					P    string  `xml:"pCBS"`
					GRed dfeGRed `xml:"gRed"`
					V    string  `xml:"vCBS"`
				} `xml:"gCBS"`
			} `xml:"gIBSCBS"`
		} `xml:"IBSCBS"`
	} `xml:"imposto"`
	InfAdProd string `xml:"infAdProd"`
}

type dfeTotal struct {
	ICMSTot struct {
		VBC          string `xml:"vBC"`
		VICMS        string `xml:"vICMS"`
		VBCST        string `xml:"vBCST"`
		VST          string `xml:"vST"`
		VFCP         string `xml:"vFCP"`
		VFCPST       string `xml:"vFCPST"`
		VICMSUFDest  string `xml:"vICMSUFDest"`
		VFCPUFDest   string `xml:"vFCPUFDest"`
		QBCMono      string `xml:"qBCMono"`
		VICMSMono    string `xml:"vICMSMono"`
		QBCMonoReten string `xml:"qBCMonoReten"`
		VICMSMonoRet string `xml:"vICMSMonoReten"`
		VProd        string `xml:"vProd"`
		VFrete       string `xml:"vFrete"`
		VSeg         string `xml:"vSeg"`
		VDesc        string `xml:"vDesc"`
		VIPI         string `xml:"vIPI"`
		VOutro       string `xml:"vOutro"`
		VNF          string `xml:"vNF"`
	} `xml:"ICMSTot"`
	ISSQNtot *struct {
		VServ string `xml:"vServ"`
		VBC   string `xml:"vBC"`
		VISS  string `xml:"vISS"`
	} `xml:"ISSQNtot"`
	ISTot *struct {
		VIS string `xml:"vIS"`
	} `xml:"ISTot"`
	IBSCBSTot *struct {
		GIBS struct {
			UF struct {
				V string `xml:"vIBSUF"`
			} `xml:"gIBSUF"`
			Mun struct {
				V string `xml:"vIBSMun"`
			} `xml:"gIBSMun"`
		} `xml:"gIBS"`
		GCBS struct {
			V string `xml:"vCBS"`
		} `xml:"gCBS"`
		GMono *struct {
			VIBSMono    string `xml:"vIBSMono"`
			VCBSMono    string `xml:"vCBSMono"`
			VIBSMonoRet string `xml:"vIBSMonoReten"`
			VCBSMonoRet string `xml:"vCBSMonoReten"`
		} `xml:"gMono"`
	} `xml:"IBSCBSTot"`
}

type dfeTransp struct {
	ModFrete   string `xml:"modFrete"`
	Transporta struct {
		CNPJ   string `xml:"CNPJ"`
		CPF    string `xml:"CPF"`
		XNome  string `xml:"xNome"`
		IE     string `xml:"IE"`
		XEnder string `xml:"xEnder"`
		XMun   string `xml:"xMun"`
		UF     string `xml:"UF"`
	} `xml:"transporta"`
	Veic struct {
		Placa string `xml:"placa"`
		UF    string `xml:"UF"`
		RNTC  string `xml:"RNTC"`
	} `xml:"veicTransp"`
	Vol []struct {
		QVol  string `xml:"qVol"`
		Esp   string `xml:"esp"`
		Marca string `xml:"marca"`
		NVol  string `xml:"nVol"`
		PesoL string `xml:"pesoL"`
		PesoB string `xml:"pesoB"`
	} `xml:"vol"`
}

type dfeCobr struct {
	Dup []struct {
		NDup  string `xml:"nDup"`
		DVenc string `xml:"dVenc"`
		VDup  string `xml:"vDup"`
	} `xml:"dup"`
}

type dfeInfAdic struct {
	InfAdFisco string `xml:"infAdFisco"`
	InfCpl     string `xml:"infCpl"`
}

// dfeParseNFe lê uma NF-e (nfeProc, ou <NFe> avulsa) — mesma tolerância de encoding de pfsParseNFe.
func dfeParseNFe(data []byte) (*dfeNfeProc, error) {
	data = pfsStripNamespaces(data)
	tri := bytes.TrimSpace(data)
	prolog := []byte{}
	body := tri
	if bytes.HasPrefix(tri, []byte("<?xml")) {
		if i := bytes.Index(tri, []byte("?>")); i >= 0 {
			prolog = append(append([]byte{}, tri[:i+2]...), '\n')
			body = bytes.TrimSpace(tri[i+2:])
		}
	}
	if bytes.HasPrefix(body, []byte("<NFe")) {
		data = append(append(append([]byte{}, prolog...), []byte("<nfeProc>")...), append(append([]byte{}, body...), []byte("</nfeProc>")...)...)
	}

	decode := func(b []byte) (*dfeNfeProc, error) {
		dec := xml.NewDecoder(bytes.NewReader(b))
		dec.CharsetReader = nfeCharsetReader
		var p dfeNfeProc
		if err := dec.Decode(&p); err != nil {
			return nil, err
		}
		return &p, nil
	}
	p, err := decode(data)
	if err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("arquivo truncado ou incompleto")
		}
		converted, convErr := convertWindows1252(data)
		if convErr != nil {
			return nil, fmt.Errorf("XML inválido: %w", err)
		}
		p2, err2 := decode(converted)
		if err2 != nil {
			return nil, fmt.Errorf("XML inválido (encoding não reconhecido): %w", err2)
		}
		return p2, nil
	}
	return p, nil
}

func (p *dfeNfeProc) chave() string {
	if ch := strings.TrimSpace(p.Prot.InfProt.ChNFe); len(ch) == 44 {
		return ch
	}
	id := strings.TrimSpace(p.NFe.InfNFe.ID)
	if strings.HasPrefix(id, "NFe") && len(id) == 47 {
		return id[3:]
	}
	return ""
}

// ---------------------------------------------------------------------------
// Formatação (padrão brasileiro)
// ---------------------------------------------------------------------------

func dfeGroupThousands(intPart string) string {
	n := len(intPart)
	if n <= 3 {
		return intPart
	}
	var b strings.Builder
	pre := n % 3
	if pre > 0 {
		b.WriteString(intPart[:pre])
	}
	for i := pre; i < n; i += 3 {
		if b.Len() > 0 {
			b.WriteByte('.')
		}
		b.WriteString(intPart[i : i+3])
	}
	return b.String()
}

// dfeNumMin formata "1234.5" como "1.234,50": entre minDec e maxDec casas (zeros à direita
// além de minDec são cortados). Vazio vira vazio (campo ausente no XML).
func dfeNumMin(s string, minDec, maxDec int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	v := toDecimal(s)
	neg := v < 0
	if neg {
		v = -v
	}
	str := strconv.FormatFloat(v, 'f', maxDec, 64)
	ip, dp := str, ""
	if i := strings.IndexByte(str, '.'); i >= 0 {
		ip, dp = str[:i], str[i+1:]
	}
	for len(dp) > minDec && strings.HasSuffix(dp, "0") {
		dp = dp[:len(dp)-1]
	}
	out := dfeGroupThousands(ip)
	if dp != "" {
		out += "," + dp
	}
	if neg {
		out = "-" + out
	}
	return out
}

func dfeMoney(s string) string { return dfeNumMin(s, 2, 2) }

func dfePct(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return dfeNumMin(s, 2, 4) + "%"
}

func dfeDigits(s string) string {
	var b strings.Builder
	for _, c := range s {
		if c >= '0' && c <= '9' {
			b.WriteRune(c)
		}
	}
	return b.String()
}

func dfeCNPJCPF(cnpj, cpf string) string {
	if d := dfeDigits(cnpj); len(d) == 14 {
		return d[0:2] + "." + d[2:5] + "." + d[5:8] + "/" + d[8:12] + "-" + d[12:14]
	}
	if d := dfeDigits(cpf); len(d) == 11 {
		return d[0:3] + "." + d[3:6] + "." + d[6:9] + "-" + d[9:11]
	}
	if cnpj != "" {
		return cnpj
	}
	return cpf
}

func dfeChaveFmt(ch string) string {
	var parts []string
	for i := 0; i < len(ch); i += 4 {
		e := i + 4
		if e > len(ch) {
			e = len(ch)
		}
		parts = append(parts, ch[i:e])
	}
	return strings.Join(parts, " ")
}

func dfeNumDoc(nNF string) string {
	n, err := strconv.Atoi(strings.TrimSpace(nNF))
	if err != nil {
		return nNF
	}
	s := fmt.Sprintf("%09d", n)
	return s[0:3] + "." + s[3:6] + "." + s[6:9]
}

func dfeSerie(s string) string {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return s
	}
	return fmt.Sprintf("%03d", n)
}

func dfeParseDh(dh string) (time.Time, bool) {
	dh = strings.TrimSpace(dh)
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, dh); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func dfeData(dh string) string {
	if t, ok := dfeParseDh(dh); ok {
		return t.Format("02/01/2006")
	}
	return dh
}

func dfeHora(dh string) string {
	if t, ok := dfeParseDh(dh); ok && len(strings.TrimSpace(dh)) > 10 {
		return t.Format("15:04:05")
	}
	return ""
}

func dfeCEP(s string) string {
	if d := dfeDigits(s); len(d) == 8 {
		return d[:5] + "-" + d[5:]
	}
	return s
}

func dfeFone(s string) string {
	d := dfeDigits(s)
	switch len(d) {
	case 10:
		return "(" + d[:2] + ") " + d[2:6] + "-" + d[6:]
	case 11:
		return "(" + d[:2] + ") " + d[2:7] + "-" + d[7:]
	}
	return s
}

func dfeCRT(s string) string {
	switch strings.TrimSpace(s) {
	case "1":
		return "1 - SIMPLES NACIONAL"
	case "2":
		return "2 - SIMPLES NACIONAL - EXCESSO DE SUBLIMITE"
	case "3":
		return "3 - REGIME NORMAL"
	case "4":
		return "4 - SIMPLES NACIONAL - MEI"
	}
	return s
}

func dfeModFrete(s string) string {
	switch strings.TrimSpace(s) {
	case "0":
		return "0 - Remetente"
	case "1":
		return "1 - Destinatário"
	case "2":
		return "2 - Terceiros"
	case "3":
		return "3 - Próprio (rem.)"
	case "4":
		return "4 - Próprio (dest.)"
	case "9":
		return "9 - Sem frete"
	}
	return s
}

// dfeEnderLinha monta "Logradouro, nro - compl" a partir do endereço.
func dfeEnderLinha(e dfeEnder) string {
	s := strings.TrimSpace(e.XLgr)
	if e.Nro != "" {
		s += ", " + strings.TrimSpace(e.Nro)
	}
	if e.XCpl != "" {
		s += " - " + strings.TrimSpace(e.XCpl)
	}
	return s
}
