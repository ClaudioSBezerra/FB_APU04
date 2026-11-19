// pacotefiscal_sim2027_liquido.go — tags de valor líquido da NT 2026.008 v1.01 (Reforma
// Tributária do Consumo — Valor Líquido) para o XML da Simulação 2027.
//
// A NT criou vUnComLiq/vProdLiq no item e vProdLiqTot no total. As NF-e emitidas antes de
// 16/11/2026 não trazem essas tags, então o gerador as ADICIONA ao XML simulado (se o XML
// original já as traz, nada é alterado — passa como está).
//
// Posições conforme o XSD oficial (PL_010g/PL_010h): em <prod>, o bloco [vUnComLiq,
// vProdLiq] vem logo depois de vProd; vProdLiqTot é o 1º filho de <total>.
//
// Valor líquido do item (decisão do usuário, 2026-11-19): vProd menos os tributos
// "por dentro" que estavam no preço da nota ORIGINAL — ICMS, PIS e COFINS do item
// (vICMS, vPIS, vCOFINS, lidos do XML antes de a simulação zerar PIS/COFINS). Nunca fica
// negativo. Garante as regras da NT: I11c-20 (vProdLiq = vUnComLiq * qCom, tol. 0,01) e
// W01a-20 (vProdLiqTot = soma dos vProdLiq).
package handlers

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// pfsItemLiquido reúne, por <det>, o que é preciso para derivar o valor líquido.
// Valores monetários em centavos (soma exata dos itens no vProdLiqTot).
type pfsItemLiquido struct {
	qCom                 float64
	vProd                int64
	vICMS, vPIS, vCOFINS int64
	temLiq               bool // o XML original já traz vProdLiq/vUnComLiq neste item
}

type pfsPrescanLiquido struct {
	itens          []pfsItemLiquido
	temVProdLiqTot bool
}

// deveAdicionar: só adiciona as tags se há itens e o XML original não traz nenhuma delas
// (documento já no leiaute novo, ou misturado/inválido, passa inalterado).
func (p pfsPrescanLiquido) deveAdicionar() bool {
	if len(p.itens) == 0 || p.temVProdLiqTot {
		return false
	}
	for _, it := range p.itens {
		if it.temLiq {
			return false
		}
	}
	return true
}

func pfsCents(s string) int64 { return int64(math.Round(toDecimal(s) * 100)) }

func (it pfsItemLiquido) liquidoCents() int64 {
	v := it.vProd - it.vICMS - it.vPIS - it.vCOFINS
	if v < 0 {
		return 0
	}
	return v
}

func pfsFmtCents(c int64) string { return fmt.Sprintf("%d.%02d", c/100, c%100) }

// vUnComLiq = valor líquido / quantidade comercial, com 10 casas (11v0-10), de modo que
// vUnComLiq * qCom reproduz vProdLiq dentro da tolerância da regra I11c-20.
func (it pfsItemLiquido) vUnComLiqStr() string {
	liq := float64(it.liquidoCents()) / 100
	if it.qCom > 0 {
		liq /= it.qCom
	}
	return strconv.FormatFloat(liq, 'f', 10, 64)
}

func (p pfsPrescanLiquido) totalLiquidoCents() int64 {
	var t int64
	for _, it := range p.itens {
		t += it.liquidoCents()
	}
	return t
}

// pfsPrescanItensLiquido percorre o XML (já sem namespaces) e coleta, por item, vProd,
// qCom e os valores de ICMS/PIS/COFINS. Precisa ser um passe à parte porque o ICMS vem
// DEPOIS do <prod> no documento, e o vProdLiq é escrito logo após o vProd.
func pfsPrescanItensLiquido(data []byte) (pfsPrescanLiquido, error) {
	var res pfsPrescanLiquido
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = nfeCharsetReader

	var path []string
	at := func(back int) string {
		if len(path) <= back {
			return ""
		}
		return path[len(path)-1-back]
	}

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return res, nil
		}
		if err != nil {
			return res, fmt.Errorf("erro ao ler itens do XML: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			path = append(path, t.Name.Local)
			switch {
			case t.Name.Local == "det" && at(1) == "infNFe":
				res.itens = append(res.itens, pfsItemLiquido{})
			case t.Name.Local == "vProdLiqTot" && at(1) == "total":
				res.temVProdLiqTot = true
			case (t.Name.Local == "vProdLiq" || t.Name.Local == "vUnComLiq") && at(1) == "prod" && len(res.itens) > 0:
				res.itens[len(res.itens)-1].temLiq = true
			}
		case xml.EndElement:
			if len(path) > 0 {
				path = path[:len(path)-1]
			}
		case xml.CharData:
			if len(res.itens) == 0 {
				continue
			}
			txt := strings.TrimSpace(string(t))
			if txt == "" {
				continue
			}
			it := &res.itens[len(res.itens)-1]
			switch top := at(0); {
			case top == "qCom" && at(1) == "prod":
				it.qCom = toDecimal(txt)
			case top == "vProd" && at(1) == "prod":
				it.vProd = pfsCents(txt)
			case top == "vICMS" && at(2) == "ICMS": // imposto/ICMS/ICMSxx/vICMS
				it.vICMS += pfsCents(txt)
			case top == "vPIS" && at(2) == "PIS": // imposto/PIS/PISxxx/vPIS (PISST fica de fora)
				it.vPIS += pfsCents(txt)
			case top == "vCOFINS" && at(2) == "COFINS":
				it.vCOFINS += pfsCents(txt)
			}
		}
	}
}

// pfsEmitLeaf escreve <nome>texto</nome> no encoder.
func pfsEmitLeaf(enc *xml.Encoder, nome, texto string) error {
	start := xml.StartElement{Name: xml.Name{Local: nome}}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	if err := enc.EncodeToken(xml.CharData([]byte(texto))); err != nil {
		return err
	}
	return enc.EncodeToken(start.End())
}
