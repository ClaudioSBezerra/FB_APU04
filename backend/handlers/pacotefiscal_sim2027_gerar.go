// pacotefiscal_sim2027_gerar.go — gera o XML "simulado 2027" a partir do XML
// original (NF-e modelo 55 ou CT-e), preservando TODOS os campos exceto os
// explicitamente listados no plano aprovado (PIS/COFINS/IPI zerados, IBS/CBS
// recalculado). Implementado como transformação por stream de tokens
// (xml.Decoder → xml.Encoder) em vez de remontar o documento do zero via
// xml.Marshal — assim é impossível "esquecer" de copiar um campo que não está
// na lista de exceção: tudo que não bate numa regra passa inalterado.
//
// Namespaces (xmlns) são removidos do resultado (mesma convenção já usada
// pelos parsers deste módulo, que sempre leem documentos sem namespace) —
// o XML gerado é um artefato interno de simulação, não é reenviado à SEFAZ.
// A assinatura digital original (<Signature>) é removida: qualquer alteração
// de valor já invalida a assinatura original, mantê-la seria enganoso.
package handlers

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"time"
)

// pfsGerarXMLSimulado aplica a transformação completa sobre os bytes
// originais e devolve o novo XML (UTF-8, sem assinatura, com comentário de
// marcação de simulação logo após a tag raiz).
func pfsGerarXMLSimulado(original []byte, params pfsParametros, geradoEm time.Time) ([]byte, error) {
	data := pfsStripNamespaces(original)

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
			if t.Name.Local == "Signature" {
				if err := pfsSkipElement(dec); err != nil {
					return nil, fmt.Errorf("erro ao remover assinatura original: %w", err)
				}
				continue
			}
			t.Name.Space = ""
			for i := range t.Attr {
				t.Attr[i].Name.Space = ""
			}
			path = append(path, t.Name.Local)
			if err := enc.EncodeToken(t); err != nil {
				return nil, err
			}
			if !commentWritten {
				commentWritten = true
				msg := fmt.Sprintf(
					" SIMULACAO 2027 (NAO VALIDO PARA TRANSMISSAO/SEFAZ) - PIS, COFINS e IPI zerados;"+
						" IBS/CBS recalculado com IBS-UF=%s%%, IBS-Mun=%s%%, CBS=%s%% - gerado em %s ",
					pfsFmtPct(params.AliqIbsUfPct), pfsFmtPct(params.AliqIbsMunPct), pfsFmtPct(params.AliqCbsPct),
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
			if err := enc.EncodeToken(t); err != nil {
				return nil, err
			}

		case xml.CharData:
			tp, p, gp := top(), parent(), grand()
			newText, changed := pfsComputeReplacement(tp, p, gp, string(t), params, &curVBC, &curVIbsUF, &curVIbsMun, &totals)
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

// pfsSkipElement consome (sem reemitir) toda a subárvore de um elemento cuja
// tag de abertura já foi lida (e descartada) pelo chamador.
func pfsSkipElement(dec *xml.Decoder) error {
	depth := 1
	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		}
	}
	return nil
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
