// pacotefiscal_sim2027_gerar.go — gera o XML "simulado 2027" a partir do XML
// original (NF-e modelo 55 ou CT-e), preservando TODOS os campos exceto os
// explicitamente listados no plano aprovado (PIS/COFINS/IPI zerados, IBS/CBS
// recalculado). Implementado como transformação por stream de tokens
// (xml.Decoder → xml.Encoder) em vez de remontar o documento do zero via
// xml.Marshal — assim é impossível "esquecer" de copiar um campo que não está
// na lista de exceção: tudo que não bate numa regra passa inalterado.
//
// Namespaces são removidos de todo o processamento interno (Name.Space fica
// sempre vazio — evita o problema conhecido de encoding/xml reencodar tokens
// com Name.Space preenchido gerando prefixos sintéticos/errados) e
// reinjetados manualmente como atributo literal "xmlns" só no elemento raiz
// (NFe/CTe herda por escopo — não precisa repetir em cada filho).
//
// O <Signature> original é MANTIDO (não removido): o XSD da NF-e/CT-e exige
// estruturalmente que Signature seja o último filho — removê-lo quebra a
// validação de schema antes mesmo de chegar à checagem da assinatura em si
// (achado em teste real na homologação SEFAZ 2026-10-06: rejeição 225 "NFe
// has incomplete content... expected infNFeSupl/Signature"). A assinatura
// mantida fica desatualizada/criptograficamente inválida em relação ao novo
// conteúdo — isso é esperado: este XML é uma simulação, não um documento
// pronto para transmissão. Para efetivamente transmitir/validar como
// documento real seria necessário reassiná-lo com um certificado ICP-Brasil
// válido — fora do escopo desta ferramenta (não lidamos com material
// criptográfico do cliente).
package handlers

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	pfsNamespaceNFe = "http://www.portalfiscal.inf.br/nfe"
	pfsNamespaceCTe = "http://www.portalfiscal.inf.br/cte"
)

// pfsGerarXMLSimulado aplica a transformação completa sobre os bytes
// originais e devolve o novo XML (UTF-8, com xmlns e Signature originais
// preservados, e comentário de marcação de simulação logo após a tag raiz).
func pfsGerarXMLSimulado(original []byte, params pfsParametros, geradoEm time.Time) ([]byte, error) {
	rootNamespace := pfsNamespaceNFe
	if pfsDetectTipoDoc(original) == pfsTipoCTe {
		rootNamespace = pfsNamespaceCTe
	}

	data := pfsStripNamespaces(original)

	// XML em Latin-1/Windows-1252 sem declarar o encoding (comum em XML exportado por ERP): o
	// decoder supõe UTF-8 e para no 1º acento ("invalid UTF-8"). A análise (pfsParseNFe) já
	// tolera esse caso; aqui convertemos antes dos dois passes. A saída sai sempre em UTF-8.
	if !utf8.Valid(data) && !pfsDeclaraEncodingNaoUTF8(data) {
		conv, err := convertWindows1252(data)
		if err != nil {
			return nil, fmt.Errorf("erro ao converter o encoding do XML: %w", err)
		}
		data = conv
	}

	// NT 2026.008: se for NF-e e o XML ainda não traz as tags de valor líquido, o gerador
	// as adiciona (ver pacotefiscal_sim2027_liquido.go). Passe prévio porque o ICMS vem
	// depois do <prod>.
	var pre pfsPrescanLiquido
	inserirLiquido := false // [vUnComLiq, vProdLiq] por item + vProdLiqTot
	calcItens := false      // vItem por item + vNFTot = Σ vItem (NT 2026.008, só NF-e)
	if pfsDetectTipoDoc(original) == pfsTipoNFe {
		if p, err := pfsPrescanItensLiquido(data); err == nil && len(p.itens) > 0 {
			pre, calcItens = p, true
			inserirLiquido = p.deveAdicionar()
		}
	}
	detIdx := -1
	vItemFeito := false  // <vItem> do det corrente já escrito (ou já presente no original)
	vNFTotFeito := false // <vNFTot> do total já escrito

	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = nfeCharsetReader

	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	enc := xml.NewEncoder(&buf)

	var path []string
	var curVBC, curVIbsUF, curVIbsMun float64
	var totals pfsTotaisIBSCBS
	commentWritten := false

	top := func() string {
		if len(path) == 0 {
			return ""
		}
		return path[len(path)-1]
	}
	parent := func() string {
		if len(path) < 2 {
			return ""
		}
		return path[len(path)-2]
	}
	grand := func() string {
		if len(path) < 3 {
			return ""
		}
		return path[len(path)-3]
	}

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("erro ao processar XML: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			t.Name.Space = ""
			for i := range t.Attr {
				t.Attr[i].Name.Space = ""
			}
			isRoot := !commentWritten
			if t.Name.Local == "Signature" {
				// Namespace próprio do xmldsig, preservado mesmo com o
				// restante do documento sem prefixo/namespace.
				t.Attr = append(t.Attr, xml.Attr{Name: xml.Name{Local: "xmlns"}, Value: "http://www.w3.org/2000/09/xmldsig#"})
			}
			if isRoot {
				t.Attr = append([]xml.Attr{{Name: xml.Name{Local: "xmlns"}, Value: rootNamespace}}, t.Attr...)
			}
			path = append(path, t.Name.Local)
			// vItem vem antes de DFeReferenciado (XSD PL_010h: ... obsItem, vItem, DFeReferenciado)
			if calcItens && t.Name.Local == "DFeReferenciado" && parent() == "det" && !vItemFeito &&
				detIdx >= 0 && detIdx < len(pre.itens) {
				path = path[:len(path)-1]
				if err := pfsEmitLeaf(enc, "vItem", pfsFmtCents(pre.itens[detIdx].vItemCents(params))); err != nil {
					return nil, err
				}
				vItemFeito = true
				path = append(path, t.Name.Local)
			}
			if err := enc.EncodeToken(t); err != nil {
				return nil, err
			}
			if t.Name.Local == "det" && parent() == "infNFe" {
				detIdx++
				vItemFeito = false
			}
			if t.Name.Local == "vItem" && parent() == "det" {
				vItemFeito = true // já existe no original: será substituído no CharData
			}
			if t.Name.Local == "vNFTot" && parent() == "total" {
				vNFTotFeito = true
			}
			// vProdLiqTot é o 1º filho de <total> (XSD PL_010h), antes de ICMSTot.
			if inserirLiquido && t.Name.Local == "total" && parent() == "infNFe" {
				if err := pfsEmitLeaf(enc, "vProdLiqTot", pfsFmtCents(pre.totalLiquidoCents())); err != nil {
					return nil, err
				}
			}
			if isRoot {
				commentWritten = true
				extra := ""
				if inserirLiquido {
					extra += " valor liquido (NT 2026.008) adicionado;"
				}
				if calcItens {
					extra += " vItem/vNFTot (soma dos itens) recalculados;"
				}
				msg := fmt.Sprintf(
					" SIMULACAO 2027 (NAO VALIDO PARA TRANSMISSAO/SEFAZ) - PIS, COFINS e IPI zerados;"+
						" IBS/CBS recalculado com IBS-UF=%s%%, IBS-Mun=%s%%, CBS=%s%%;%s - gerado em %s ",
					pfsFmtPct(params.AliqIbsUfPct), pfsFmtPct(params.AliqIbsMunPct), pfsFmtPct(params.AliqCbsPct),
					extra,
					geradoEm.Format("2006-01-02 15:04:05 -0700"),
				)
				if err := enc.EncodeToken(xml.Comment(msg)); err != nil {
					return nil, err
				}
			}

		case xml.EndElement:
			if len(path) > 0 {
				path = path[:len(path)-1]
			}
			t.Name.Space = ""
			// vItem: último filho do det quando não há DFeReferenciado; vNFTot: último filho do total.
			if calcItens && t.Name.Local == "det" && top() == "infNFe" && !vItemFeito && detIdx >= 0 && detIdx < len(pre.itens) {
				if err := pfsEmitLeaf(enc, "vItem", pfsFmtCents(pre.itens[detIdx].vItemCents(params))); err != nil {
					return nil, err
				}
				vItemFeito = true
			}
			if calcItens && t.Name.Local == "total" && top() == "infNFe" && !vNFTotFeito {
				if err := pfsEmitLeaf(enc, "vNFTot", pfsFmtCents(pre.totalVItemCents(params))); err != nil {
					return nil, err
				}
				vNFTotFeito = true
			}
			if err := enc.EncodeToken(t); err != nil {
				return nil, err
			}
			// [vUnComLiq, vProdLiq] logo depois do vProd do item (XSD PL_010h).
			if inserirLiquido && t.Name.Local == "vProd" && top() == "prod" && detIdx >= 0 && detIdx < len(pre.itens) {
				it := pre.itens[detIdx]
				if err := pfsEmitLeaf(enc, "vUnComLiq", it.vUnComLiqStr()); err != nil {
					return nil, err
				}
				if err := pfsEmitLeaf(enc, "vProdLiq", pfsFmtCents(it.liquidoCents())); err != nil {
					return nil, err
				}
			}

		case xml.CharData:
			tp, p, gp := top(), parent(), grand()
			newText, changed := "", false
			switch {
			case calcItens && tp == "vItem" && p == "det" && detIdx >= 0 && detIdx < len(pre.itens):
				newText, changed = pfsFmtCents(pre.itens[detIdx].vItemCents(params)), true
			case calcItens && tp == "vNFTot" && p == "total":
				newText, changed = pfsFmtCents(pre.totalVItemCents(params)), true
			default:
				newText, changed = pfsComputeReplacement(tp, p, gp, string(t), params, &curVBC, &curVIbsUF, &curVIbsMun, &totals)
			}
			if changed {
				if err := enc.EncodeToken(xml.CharData([]byte(newText))); err != nil {
					return nil, err
				}
			} else {
				if err := enc.EncodeToken(t); err != nil {
					return nil, err
				}
			}

		default:
			if err := enc.EncodeToken(tok); err != nil {
				return nil, err
			}
		}
	}

	if err := enc.Flush(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// pfsComputeReplacement decide, para um nó-folha identificado por
// (tag, pai, avô), se o valor deve ser substituído — e por qual. Ver a tabela
// de escopo no plano aprovado: qualquer caminho não coberto aqui é devolvido
// inalterado (ICMSTot, cobr/dup, produto, endereços, ICMS, transporte etc.).
func pfsComputeReplacement(tp, p, gp, orig string, params pfsParametros, curVBC, curVIbsUF, curVIbsMun *float64, totals *pfsTotaisIBSCBS) (string, bool) {
	// IPI — qualquer variante tributada (IPITrib). IPINT (não tributado) não
	// tem valores numéricos de imposto a zerar.
	if p == "IPITrib" {
		switch tp {
		case "vBC", "vIPI":
			return pfsFmtMoney(0), true
		case "pIPI":
			return pfsFmtPct(0), true
		}
		return orig, false
	}

	// PIS
	if p == "PISAliq" {
		switch tp {
		case "vBC", "vPIS":
			return pfsFmtMoney(0), true
		case "pPIS":
			return pfsFmtPct(0), true
		}
		return orig, false
	}
	if p == "PISQtde" && tp == "vPIS" {
		return pfsFmtMoney(0), true
	}

	// COFINS
	if p == "COFINSAliq" {
		switch tp {
		case "vBC", "vCOFINS":
			return pfsFmtMoney(0), true
		case "pCOFINS":
			return pfsFmtPct(0), true
		}
		return orig, false
	}
	if p == "COFINSQtde" && tp == "vCOFINS" {
		return pfsFmtMoney(0), true
	}

	// Redução de alíquota (NT 2026.010 / UB28, UB47, UB66): com gRed, o DANFE imprime a alíquota
	// EFETIVA. A simulação aplica a alíquota da tela de teste sobre a base (vIBSUF = vBC * alíquota),
	// então a efetiva é essa mesma alíquota — senão o pAliqEfet original ficaria desatualizado.
	// pRedAliq não é alterado.
	if p == "gRed" && tp == "pAliqEfet" {
		switch gp {
		case "gIBSUF":
			return pfsFmtPct(params.AliqIbsUfPct), true
		case "gIBSMun":
			return pfsFmtPct(params.AliqIbsMunPct), true
		case "gCBS":
			return pfsFmtPct(params.AliqCbsPct), true
		}
		return orig, false
	}

	// IBS/CBS por item (NF-e) ou bloco único (CT-e): .../IBSCBS/gIBSCBS/...
	if p == "gIBSCBS" {
		if tp == "vBC" {
			*curVBC = toDecimal(orig)
			return orig, false // base mantida — é o que o usuário pediu para usar
		}
		if tp == "vIBS" {
			v := round2(*curVIbsUF + *curVIbsMun)
			totals.VIBS += v
			return pfsFmtMoney(v), true
		}
		return orig, false
	}
	if p == "gIBSUF" && gp == "gIBSCBS" {
		if tp == "pIBSUF" {
			return pfsFmtPct(params.AliqIbsUfPct), true
		}
		if tp == "vIBSUF" {
			v := round2(*curVBC * params.AliqIbsUfPct / 100)
			*curVIbsUF = v
			totals.VIbsUF += v
			return pfsFmtMoney(v), true
		}
		return orig, false
	}
	if p == "gIBSMun" && gp == "gIBSCBS" {
		if tp == "pIBSMun" {
			return pfsFmtPct(params.AliqIbsMunPct), true
		}
		if tp == "vIBSMun" {
			v := round2(*curVBC * params.AliqIbsMunPct / 100)
			*curVIbsMun = v
			totals.VIbsMun += v
			return pfsFmtMoney(v), true
		}
		return orig, false
	}
	if p == "gCBS" && gp == "gIBSCBS" {
		if tp == "pCBS" {
			return pfsFmtPct(params.AliqCbsPct), true
		}
		if tp == "vCBS" {
			v := round2(*curVBC * params.AliqCbsPct / 100)
			totals.VCBS += v
			return pfsFmtMoney(v), true
		}
		return orig, false
	}

	// Totais do documento (NF-e apenas): <total><IBSCBSTot><gIBS>... — soma
	// dos itens processados acima (vem sempre depois no documento). vDif,
	// vDevTrib, vCredPres*, vBCIBSCBS não são tocados (não batem em nenhuma
	// regra acima e caem no "sem alteração" no final da função).
	if p == "gIBSUF" && gp == "gIBS" && tp == "vIBSUF" {
		return pfsFmtMoney(totals.VIbsUF), true
	}
	if p == "gIBSMun" && gp == "gIBS" && tp == "vIBSMun" {
		return pfsFmtMoney(totals.VIbsMun), true
	}
	if p == "gIBS" && tp == "vIBS" {
		return pfsFmtMoney(totals.VIBS), true
	}
	if p == "gCBS" && gp == "IBSCBSTot" && tp == "vCBS" {
		return pfsFmtMoney(totals.VCBS), true
	}

	return orig, false
}

func pfsFmtMoney(v float64) string {
	return strconv.FormatFloat(round2(v), 'f', 2, 64)
}

func pfsFmtPct(v float64) string {
	return strconv.FormatFloat(v, 'f', 4, 64)
}

// pfsDeclaraEncodingNaoUTF8 diz se o prólogo declara um encoding diferente de UTF-8 (ex.:
// ISO-8859-1). Nesse caso quem converte é o CharsetReader do decoder, e converter antes
// trocaria os acentos duas vezes.
func pfsDeclaraEncodingNaoUTF8(data []byte) bool {
	t := bytes.TrimSpace(data)
	if !bytes.HasPrefix(t, []byte("<?xml")) {
		return false
	}
	fim := bytes.Index(t, []byte("?>"))
	if fim < 0 {
		return false
	}
	prologo := strings.ToLower(string(t[:fim]))
	if !strings.Contains(prologo, "encoding") {
		return false
	}
	return !strings.Contains(prologo, "utf-8") && !strings.Contains(prologo, "utf8")
}
