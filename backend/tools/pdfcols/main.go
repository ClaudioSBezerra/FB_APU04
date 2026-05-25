// pdfcols — sonda a estrutura 2D do PDF: para cada linha (Y), imprime os
// fragmentos com sua coordenada X. Serve para decidir se dá para reconstruir
// as colunas (CEST | NCM | MVA ajustada | MVA original) por faixa de X.
//
//	go run ./tools/pdfcols "<arquivo.pdf>" <pagina>
package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"

	pdflib "github.com/ledongthuc/pdf"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "uso: pdfcols <arquivo.pdf> [pagina inicial] [qtd paginas]")
		os.Exit(1)
	}
	pg := 1
	if len(os.Args) >= 3 {
		pg, _ = strconv.Atoi(os.Args[2])
	}
	npg := 1
	if len(os.Args) >= 4 {
		npg, _ = strconv.Atoi(os.Args[3])
	}

	f, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer f.Close()
	fi, _ := f.Stat()
	pr, err := pdflib.NewReader(f, fi.Size())
	if err != nil {
		panic(err)
	}

	for p := pg; p < pg+npg && p <= pr.NumPage(); p++ {
		page := pr.Page(p)
		if page.V.IsNull() {
			continue
		}
		rows, err := page.GetTextByRow()
		if err != nil {
			fmt.Println("err:", err)
			continue
		}
		fmt.Printf("\n========== PÁGINA %d (%d linhas) ==========\n", p, len(rows))
		// histograma de X iniciais (para ver onde estão as colunas)
		xstart := map[int]int{}
		for _, row := range rows {
			if len(row.Content) == 0 {
				continue
			}
			xstart[int(row.Content[0].X/20)*20]++
		}
		var keys []int
		for k := range xstart {
			keys = append(keys, k)
		}
		sort.Ints(keys)
		fmt.Print("Histograma X inicial (faixa→qtd linhas): ")
		for _, k := range keys {
			fmt.Printf("%d→%d  ", k, xstart[k])
		}
		fmt.Println()

		for i, row := range rows {
			if i >= 40 {
				fmt.Println("... (truncado)")
				break
			}
			fmt.Printf("Y=%-5d | ", row.Position)
			for _, t := range row.Content {
				fmt.Printf("[x=%.0f]%s ", t.X, t.S)
			}
			fmt.Println()
		}
	}
}
