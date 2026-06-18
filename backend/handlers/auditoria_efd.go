package handlers

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/ledongthuc/pdf"
	"golang.org/x/text/encoding/charmap"
)

// ---------------------------------------------------------------------------
// Auditoria Fiscal EFD ICMS/IPI × Guias (DARE) — cliente GO (JC Distribuição).
//
// Cruza a apuração declarada no Bloco E do SPED (TXT) com as guias de
// recolhimento (DARE em PDF) e emite um relatório executivo de 1 página.
// MVP: processa os arquivos diretamente (não persiste o Bloco E no banco —
// isso é o épico de importação, ver docs/PLANO-BLOCO-E.md).
//
// Regras (prompt do contador, 2026-06-10):
//   - ICMS Normal (108): E110 c13 (VL_ICMS_RECOLHER) = Σ E116(108) = Σ guias 108
//   - PROTEGE (4014):     Σ E115(GO000076+GO000082) = guia 4014
//   - Adicional 2% (4146): E116(045) = guia 4146
//   - Cadastro: referência da guia inicia "300" e competência = 0000
// ---------------------------------------------------------------------------

const auditTol = 0.01 // tolerância de comparação (centavos)

// E116Linha — uma obrigação do registro E116.
type E116Linha struct {
	CodReceita   string  // campo 2 (COD_OR)
	Valor        float64 // campo 3 (VL_OR)
	Vencimento   string  // campo 4 (DT_VCTO, DDMMAAAA)
	CodObrigacao string  // campo 5 (COD_REC) — ex.: 108, 045
	Descricao    string  // campo 9 (TXT_COMPL)
}

// DareGuia — dados extraídos de uma guia DARE (PDF).
type DareGuia struct {
	Arquivo        string
	CodReceita     string  // ex.: 108, 4014, 4146
	Descricao      string
	ValorOriginal  float64
	Referencia     string // texto bruto ("300-Mensal - 05/2026")
	RefCodigo      string // "300"
	RefCompetencia string // "05/2026"
	Vencimento     string // "DD/MM/AAAA"
}

// AuditoriaEFD — dados consolidados da apuração (lado EFD) + guias.
type AuditoriaEFD struct {
	RazaoSocial  string
	CNPJ         string
	Competencia  string // MM/AAAA (do registro 0000)
	E110Recolher float64
	E115Protege  float64
	E116         []E116Linha
	Guias        []DareGuia
}

var (
	reRefDare  = regexp.MustCompile(`(\d{3})\s*-\s*\w+\s*-\s*(\d{2}/\d{4})`) // "300-Mensal - 05/2026"
	reData     = regexp.MustCompile(`\b(\d{2}/\d{2}/\d{4})\b`)
	reReceita  = regexp.MustCompile(`(?m)^\s*(\d{2,4})\s*-\s*(.+?)\s*$`)
	reMilhar   = regexp.MustCompile(`^\d{1,3}(\.\d{3})*,\d{2}$`)
)

// parseValorBR converte "5.805.606,05" (ou "1755,6") em float.
func parseValorBR(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, ",", ".")
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// decodeLatin1 converte bytes ISO-8859-1 (encoding do SPED GO) para UTF-8.
func decodeLatin1(b []byte) string {
	out, err := charmap.ISO8859_1.NewDecoder().Bytes(b)
	if err != nil {
		return string(b)
	}
	return string(out)
}

// ddmmaaaaToBR converte "20062026" em "20/06/2026".
func ddmmaaaaToBR(s string) string {
	s = strings.TrimSpace(s)
	if len(s) != 8 {
		return s
	}
	return s[0:2] + "/" + s[2:4] + "/" + s[4:8]
}

// parseSpedAuditoria lê o SPED (TXT) e extrai 0000, E110, E115, E116.
func parseSpedAuditoria(r io.Reader) (*AuditoriaEFD, error) {
	a := &AuditoriaEFD{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 256*1024), 4*1024*1024)
	for sc.Scan() {
		raw := bytes.TrimRight(sc.Bytes(), "\r\n")
		if len(raw) < 7 || raw[0] != '|' {
			continue
		}
		reg := string(raw[1:5])
		switch reg {
		case "0000", "E110", "E115", "E116":
		default:
			continue
		}
		line := decodeLatin1(raw)
		p := strings.Split(line, "|")
		get := func(i int) string {
			if i < len(p) {
				return strings.TrimSpace(p[i])
			}
			return ""
		}
		switch reg {
		case "0000":
			a.RazaoSocial = get(6)
			a.CNPJ = get(7)
			dtIni := get(4) // DDMMAAAA
			if len(dtIni) == 8 {
				a.Competencia = dtIni[2:4] + "/" + dtIni[4:8]
			}
		case "E110":
			a.E110Recolher = parseValorBR(get(13)) // VL_ICMS_RECOLHER
		case "E115":
			cod := get(2)
			if cod == "GO000076" || cod == "GO000082" {
				a.E115Protege += parseValorBR(get(3))
			}
		case "E116":
			a.E116 = append(a.E116, E116Linha{
				CodReceita:   get(2),
				Valor:        parseValorBR(get(3)),
				Vencimento:   ddmmaaaaToBR(get(4)),
				CodObrigacao: get(5),
				Descricao:    get(9),
			})
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return a, nil
}

// somaE116 soma o VL_OR das linhas E116 com o código de obrigação dado.
func (a *AuditoriaEFD) somaE116(codObrigacao string) (total float64, venc string) {
	for _, l := range a.E116 {
		if l.CodObrigacao == codObrigacao {
			total += l.Valor
			if venc == "" {
				venc = l.Vencimento
			}
		}
	}
	return
}

// parseDareTexto extrai os campos de uma guia DARE a partir do texto já lido.
func parseDareTexto(arquivo, texto string) DareGuia {
	g := DareGuia{Arquivo: arquivo}
	linhas := strings.Split(texto, "\n")

	// Referência "300-Mensal - 05/2026"
	if m := reRefDare.FindStringSubmatch(texto); m != nil {
		g.Referencia = strings.TrimSpace(m[0])
		g.RefCodigo = m[1]
		g.RefCompetencia = m[2]
	}

	// Código da receita: linha "NNN - DESCRIÇÃO". Preferimos códigos conhecidos
	// (108, 4014, 4146); senão o primeiro código de 3-4 dígitos.
	conhecidos := map[string]bool{"108": true, "4014": true, "4146": true}
	for _, mm := range reReceita.FindAllStringSubmatch(texto, -1) {
		cod, desc := mm[1], strings.TrimSpace(mm[2])
		if conhecidos[cod] {
			g.CodReceita = cod
			g.Descricao = desc
			break
		}
		if g.CodReceita == "" && len(cod) >= 3 {
			g.CodReceita = cod
			g.Descricao = desc
		}
	}

	// Valor Original: a linha numérica imediatamente anterior a "Valor Original".
	for i, l := range linhas {
		if strings.Contains(l, "Valor Original") {
			for j := i - 1; j >= 0 && j >= i-3; j-- {
				cand := strings.TrimSpace(linhas[j])
				if reMilhar.MatchString(cand) {
					g.ValorOriginal = parseValorBR(cand)
					break
				}
			}
			break
		}
	}
	// Fallback: maior valor monetário do documento.
	if g.ValorOriginal == 0 {
		for _, l := range linhas {
			c := strings.TrimSpace(l)
			if reMilhar.MatchString(c) {
				if v := parseValorBR(c); v > g.ValorOriginal {
					g.ValorOriginal = v
				}
			}
		}
	}

	// Vencimento: primeira data após "Data de Vencimento"/"Validade do";
	// senão a maior data do documento (vencimento > emissão).
	venc := ""
	for i, l := range linhas {
		if strings.Contains(l, "Data de Vencimento") || strings.Contains(l, "Validade do") {
			for j := i; j < len(linhas) && j < i+4; j++ {
				if m := reData.FindString(linhas[j]); m != "" {
					venc = m
					break
				}
			}
		}
		if venc != "" {
			break
		}
	}
	if venc == "" {
		var datas []string
		for _, m := range reData.FindAllString(texto, -1) {
			datas = append(datas, m)
		}
		if len(datas) > 0 {
			venc = datas[len(datas)-1]
		}
	}
	g.Vencimento = venc
	return g
}

// parseDarePDF abre um PDF de DARE e extrai os campos.
func parseDarePDF(path, nomeArquivo string) (DareGuia, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return DareGuia{Arquivo: nomeArquivo}, err
	}
	defer f.Close()
	b, err := r.GetPlainText()
	if err != nil {
		return DareGuia{Arquivo: nomeArquivo}, err
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(b); err != nil {
		return DareGuia{Arquivo: nomeArquivo}, err
	}
	return parseDareTexto(nomeArquivo, buf.String()), nil
}

// ---------------------------------------------------------------------------
// Conciliação
// ---------------------------------------------------------------------------

type concLinha struct {
	Tributo    string
	OrigemEFD  string
	ValorEFD   float64
	OrigemGuia string
	ValorGuia  float64
	Diferenca  float64
	OK         bool
}

type validacaoLinha struct {
	Item       string
	Esperado   string
	Encontrado string
	OK         bool
}

type auditoriaSaida struct {
	A            *AuditoriaEFD
	Cadastro     []validacaoLinha
	Conciliacao  []concLinha
	Divergencias []string
}

func bateValor(a, b float64) bool { return math.Abs(a-b) <= auditTol }

// somaGuias soma o Valor Original das guias com o código de receita dado.
func (a *AuditoriaEFD) somaGuias(cod string) float64 {
	var t float64
	for _, g := range a.Guias {
		if g.CodReceita == cod {
			t += g.ValorOriginal
		}
	}
	return t
}

func auditar(a *AuditoriaEFD) auditoriaSaida {
	out := auditoriaSaida{A: a}

	// --- Cadastro: referência "300" + competência ---
	refOK, compOK := len(a.Guias) > 0, len(a.Guias) > 0
	var refsTxt, compsTxt []string
	for _, g := range a.Guias {
		if g.RefCodigo != "300" {
			refOK = false
		}
		if g.RefCompetencia != a.Competencia {
			compOK = false
		}
		if g.RefCodigo != "" {
			refsTxt = append(refsTxt, g.RefCodigo)
		}
		if g.RefCompetencia != "" {
			compsTxt = append(compsTxt, g.RefCompetencia)
		}
	}
	out.Cadastro = append(out.Cadastro,
		validacaoLinha{"Código de Referência Inicial", `Conter "300" (Mensal) em todas as guias`, uniqJoin(refsTxt), refOK},
		validacaoLinha{"Mês/Ano de Competência", "Igual ao período do Registro 0000 (" + a.Competencia + ")", uniqJoin(compsTxt), compOK},
	)

	// --- Conciliação ICMS Normal (108): E110 c13 = Σ guias 108 ---
	gNormal := a.somaGuias("108")
	out.Conciliacao = append(out.Conciliacao, concLinha{
		Tributo: "ICMS Normal (108)", OrigemEFD: "E110 c13 (ICMS a recolher)", ValorEFD: a.E110Recolher,
		OrigemGuia: "Σ Guia(s) 108", ValorGuia: gNormal, Diferenca: a.E110Recolher - gNormal, OK: bateValor(a.E110Recolher, gNormal),
	})

	// --- Conciliação PROTEGE (4014): Σ E115(GO000076+082) = guia 4014 ---
	gProtege := a.somaGuias("4014")
	out.Conciliacao = append(out.Conciliacao, concLinha{
		Tributo: "Fundo PROTEGE (4014)", OrigemEFD: "E115 (GO000076+GO000082)", ValorEFD: a.E115Protege,
		OrigemGuia: "Guia 4014 (Valor Original)", ValorGuia: gProtege, Diferenca: a.E115Protege - gProtege, OK: bateValor(a.E115Protege, gProtege),
	})

	// --- Conciliação Adicional 2% (4146 / obrigação 045): E116(045) = guia 4146 ---
	e116Adic, vencAdic := a.somaE116("045")
	gAdic := a.somaGuias("4146")
	out.Conciliacao = append(out.Conciliacao, concLinha{
		Tributo: "Adicional ICMS 2% (4146)", OrigemEFD: "E116 Cód. obrig. 045", ValorEFD: e116Adic,
		OrigemGuia: "Guia 4146 (Valor Original)", ValorGuia: gAdic, Diferenca: e116Adic - gAdic, OK: bateValor(e116Adic, gAdic),
	})

	// --- Divergências e amarrações internas E116 ---
	for _, c := range out.Conciliacao {
		if !c.OK {
			out.Divergencias = append(out.Divergencias, fmt.Sprintf(
				"%s: EFD R$ %s × Guia R$ %s (diferença R$ %s).",
				c.Tributo, brl2(c.ValorEFD), brl2(c.ValorGuia), brl2(math.Abs(c.Diferenca))))
		}
	}
	if !refOK {
		out.Divergencias = append(out.Divergencias, "Divergência da referência de apuração: nem toda guia inicia com \"300\".")
	}
	if !compOK {
		out.Divergencias = append(out.Divergencias, "Divergência no período/competência entre TXT e PDF.")
	}
	// Amarração interna E116(108): receita "000", vencimento começa com "20".
	somaE116Normal, _ := a.somaE116("108")
	if !bateValor(somaE116Normal, a.E110Recolher) {
		out.Divergencias = append(out.Divergencias, fmt.Sprintf(
			"Divergência E110 × E116: Σ E116(108) R$ %s ≠ E110 c13 R$ %s.", brl2(somaE116Normal), brl2(a.E110Recolher)))
	}
	for _, l := range a.E116 {
		if l.CodObrigacao == "045" && vencAdic != "" {
			// vencimento E116(045) vs guia 4146
			for _, g := range a.Guias {
				if g.CodReceita == "4146" && g.Vencimento != "" && g.Vencimento != vencAdic {
					out.Divergencias = append(out.Divergencias, fmt.Sprintf(
						"Divergência de vencimento (Adicional 2%%): E116 %s × Guia %s.", vencAdic, g.Vencimento))
				}
			}
		}
	}
	return out
}

func uniqJoin(xs []string) string {
	seen := map[string]bool{}
	var out []string
	for _, x := range xs {
		if x != "" && !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	if len(out) == 0 {
		return "—"
	}
	return strings.Join(out, ", ")
}

// brl2 formata número no padrão BR (sem símbolo): 1234567.89 -> "1.234.567,89".
func brl2(v float64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	s := strconv.FormatFloat(v, 'f', 2, 64)
	parteInt, parteDec, _ := strings.Cut(s, ".")
	var b strings.Builder
	n := len(parteInt)
	for i, d := range parteInt {
		if i > 0 && (n-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(d)
	}
	res := b.String() + "," + parteDec
	if neg {
		return "-" + res
	}
	return res
}
