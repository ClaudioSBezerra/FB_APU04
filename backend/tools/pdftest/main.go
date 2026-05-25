// pdftest — utilitário descartável para comparar 3 estratégias de extração
// de texto do PDF do decreto, e gerar uma amostra do que a IA receberia.
//
//   go run ./tools/pdftest "/tmp/Decreto....pdf" > /tmp/pdftest.out
//
// Para diagnosticar o motivo de "texto reduzido de 156117 para 0 chars".
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	pdflib "github.com/ledongthuc/pdf"
)

func abs64(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// (a) estratégia atual: walks content.Text, espaço após cada Text element
func extractAtual(buf []byte) (string, error) {
	r := bytes.NewReader(buf)
	pr, err := pdflib.NewReader(r, int64(len(buf)))
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for i := 1; i <= pr.NumPage(); i++ {
		page := pr.Page(i)
		if page.V.IsNull() {
			continue
		}
		content := page.Content()
		prevY := -1.0
		for _, t := range content.Text {
			if prevY >= 0 && abs64(t.Y-prevY) > 2 {
				sb.WriteByte('\n')
			}
			sb.WriteString(t.S)
			sb.WriteByte(' ')
			prevY = t.Y
		}
		sb.WriteByte('\n')
	}
	return sb.String(), nil
}

// (b) GetPlainText — usa os operadores PDF reais (Tj/TJ/T*/BT), sem fabricar espaços
func extractPlainText(buf []byte) (string, error) {
	r := bytes.NewReader(buf)
	pr, err := pdflib.NewReader(r, int64(len(buf)))
	if err != nil {
		return "", err
	}
	reader, err := pr.GetPlainText()
	if err != nil {
		return "", err
	}
	var sb bytes.Buffer
	_, err = io.Copy(&sb, reader)
	return sb.String(), err
}

// (c) GetTextByRow — agrupa por Y, ordena por X dentro da linha
func extractByRow(buf []byte) (string, error) {
	r := bytes.NewReader(buf)
	pr, err := pdflib.NewReader(r, int64(len(buf)))
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for i := 1; i <= pr.NumPage(); i++ {
		page := pr.Page(i)
		if page.V.IsNull() {
			continue
		}
		rows, err := page.GetTextByRow()
		if err != nil {
			continue
		}
		for _, row := range rows {
			var line strings.Builder
			prevX := -9999.0
			for _, t := range row.Content {
				// se há "espaço" geométrico considerável entre fragments, insere espaço
				if prevX > -9000 && (t.X-prevX) > 1.5 {
					line.WriteByte(' ')
				}
				line.WriteString(t.S)
				prevX = t.X + float64(len(t.S))
			}
			s := strings.TrimRight(line.String(), " ")
			if s != "" {
				sb.WriteString(s)
				sb.WriteByte('\n')
			}
		}
		sb.WriteByte('\n')
	}
	return sb.String(), nil
}

// regex do filtro atual
var reNCMOrPct = regexp.MustCompile(`\d{4}|\d+[,\.]\d+\s*%|\bMVA\b|\bNCM\b|\bCEST\b`)

func filterLines(texto string) (kept, total int, sampleKept []string) {
	for _, line := range strings.Split(texto, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		total++
		if reNCMOrPct.MatchString(t) {
			kept++
			if len(sampleKept) < 5 {
				sampleKept = append(sampleKept, t)
			}
		}
	}
	return
}

func preview(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...[truncado]"
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "uso: pdftest <arquivo.pdf>")
		os.Exit(1)
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "ler arquivo:", err)
		os.Exit(1)
	}
	fmt.Println("=== Arquivo:", os.Args[1])
	fmt.Println("=== Tamanho:", len(data), "bytes")

	type strat struct {
		nome string
		fn   func([]byte) (string, error)
	}
	strats := []strat{
		{"(a) ATUAL (content.Text + espaço por elem.)", extractAtual},
		{"(b) GetPlainText (operadores PDF)", extractPlainText},
		{"(c) GetTextByRow (linha por Y)", extractByRow},
	}

	for _, s := range strats {
		fmt.Println("\n========================================")
		fmt.Println(s.nome)
		fmt.Println("========================================")
		txt, err := s.fn(data)
		if err != nil {
			fmt.Println("ERRO:", err)
			continue
		}
		fmt.Println("Total chars:", len(txt))
		fmt.Println("Total linhas (split \\n):", len(strings.Split(txt, "\n")))
		kept, total, samples := filterLines(txt)
		fmt.Println("Linhas não-vazias:", total)
		fmt.Println("Linhas que o filtro mantém (NCM/MVA/%/...):", kept)
		if kept > 0 {
			fmt.Println("Amostra das mantidas:")
			for i, l := range samples {
				fmt.Printf("  [%d] %s\n", i, preview(l, 200))
			}
		}
		fmt.Println("\nPrimeiros 600 chars:")
		fmt.Println(preview(strings.ReplaceAll(txt[:min(len(txt), 600)], "\n", "\\n"), 600))
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
