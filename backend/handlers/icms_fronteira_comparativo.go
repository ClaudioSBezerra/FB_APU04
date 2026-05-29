package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

type DiffRow struct {
	Status        string  `json:"status"`        // "only_1" | "only_2" | "diff"
	ChaveNFe      string  `json:"chave_nfe"`
	NumeroNFe     string  `json:"numero_nfe"`
	Fornecedor    string  `json:"fornecedor"`
	CFOP          string  `json:"cfop"`
	VProdP1       float64 `json:"v_prod_p1"`
	ICMSDevidoP1  float64 `json:"icms_devido_p1"`
	VProdP2       float64 `json:"v_prod_p2"`
	ICMSDevidoP2  float64 `json:"icms_devido_p2"`
	DiffICMS      float64 `json:"diff_icms"`
}

type ComparativoResponse struct {
	BlocoA []DiffRow `json:"bloco_a"`
	BlocoB []DiffRow `json:"bloco_b"`
	BlocoC []DiffRow `json:"bloco_c"`
}

func IcmsFronteiraComparativoHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
			return
		}

		file1, _, err := r.FormFile("file1")
		if err != nil {
			http.Error(w, "file1 is required", http.StatusBadRequest)
			return
		}
		defer file1.Close()

		file2, _, err := r.FormFile("file2")
		if err != nil {
			http.Error(w, "file2 is required", http.StatusBadRequest)
			return
		}
		defer file2.Close()

		data1, err := io.ReadAll(file1)
		if err != nil {
			http.Error(w, "Failed to read file1: "+err.Error(), http.StatusInternalServerError)
			return
		}

		data2, err := io.ReadAll(file2)
		if err != nil {
			http.Error(w, "Failed to read file2: "+err.Error(), http.StatusInternalServerError)
			return
		}

		f1, err := excelize.OpenReader(bytes.NewReader(data1))
		if err != nil {
			http.Error(w, "Failed to parse file1: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer f1.Close()

		f2, err := excelize.OpenReader(bytes.NewReader(data2))
		if err != nil {
			http.Error(w, "Failed to parse file2: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer f2.Close()

		sheetsMap1 := findSheetsByKeyword(f1)
		sheetsMap2 := findSheetsByKeyword(f2)

		respBlocoA := compareBlocos(f1, f2, sheetsMap1["anterior"], sheetsMap2["anterior"], "A")
		respBlocoB := compareBlocos(f1, f2, sheetsMap1["atual"], sheetsMap2["atual"], "B")
		respBlocoC := compareBlocos(f1, f2, sheetsMap1["nao_sped"], sheetsMap2["nao_sped"], "C")

		resp := ComparativoResponse{
			BlocoA: respBlocoA,
			BlocoB: respBlocoB,
			BlocoC: respBlocoC,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func findSheetsByKeyword(f *excelize.File) map[string]string {
	result := map[string]string{}
	sheets := f.GetSheetList()

	for _, sheet := range sheets {
		lower := strings.ToLower(sheet)
		if strings.Contains(lower, "anterior") {
			result["anterior"] = sheet
		} else if strings.Contains(lower, "atual") {
			result["atual"] = sheet
		} else if strings.Contains(lower, "não") || strings.Contains(lower, "nao") {
			result["nao_sped"] = sheet
		}
	}

	return result
}

func compareBlocos(f1, f2 *excelize.File, sheet1, sheet2, bloco string) []DiffRow {
	var diffs []DiffRow

	if sheet1 == "" || sheet2 == "" {
		return diffs
	}

	rows1 := getSheetRows(f1, sheet1, bloco)
	rows2 := getSheetRows(f2, sheet2, bloco)

	for chaveNFe, row1 := range rows1 {
		row2, exists := rows2[chaveNFe]

		if !exists {
			diffs = append(diffs, DiffRow{
				Status:       "only_1",
				ChaveNFe:     chaveNFe,
				NumeroNFe:    row1.NumeroNFe,
				Fornecedor:   row1.Fornecedor,
				CFOP:         row1.CFOP,
				VProdP1:      row1.VProdP1,
				ICMSDevidoP1: row1.ICMSDevidoP1,
				VProdP2:      0,
				ICMSDevidoP2: 0,
				DiffICMS:     row1.ICMSDevidoP1,
			})
		} else {
			diffICMS := roundFloat(row1.ICMSDevidoP1, 2) - roundFloat(row2.ICMSDevidoP1, 2)
			if diffICMS != 0 {
				diffs = append(diffs, DiffRow{
					Status:       "diff",
					ChaveNFe:     chaveNFe,
					NumeroNFe:    row1.NumeroNFe,
					Fornecedor:   row1.Fornecedor,
					CFOP:         row1.CFOP,
					VProdP1:      row1.VProdP1,
					ICMSDevidoP1: row1.ICMSDevidoP1,
					VProdP2:      row2.VProdP1,
					ICMSDevidoP2: row2.ICMSDevidoP1,
					DiffICMS:     diffICMS,
				})
			}
		}

		delete(rows2, chaveNFe)
	}

	for chaveNFe, row2 := range rows2 {
		diffs = append(diffs, DiffRow{
			Status:       "only_2",
			ChaveNFe:     chaveNFe,
			NumeroNFe:    row2.NumeroNFe,
			Fornecedor:   row2.Fornecedor,
			CFOP:         row2.CFOP,
			VProdP1:      0,
			ICMSDevidoP1: 0,
			VProdP2:      row2.VProdP1,
			ICMSDevidoP2: row2.ICMSDevidoP1,
			DiffICMS:     -row2.ICMSDevidoP1,
		})
	}

	return diffs
}

func getSheetRows(f *excelize.File, sheet, bloco string) map[string]DiffRow {
	result := make(map[string]DiffRow)

	rows, err := f.GetRows(sheet)
	if err != nil {
		return result
	}

	if len(rows) == 0 {
		return result
	}

	for i, row := range rows {
		if i == 0 {
			continue
		}

		if len(row) == 0 {
			continue
		}

		var chaveNFe, numeroNFe, fornecedor, cfop string
		var vProd, icmsDue float64

		if bloco == "A" || bloco == "B" {
			if len(row) < 17 {
				continue
			}

			chaveNFe = strings.TrimSpace(row[16])
			if chaveNFe == "" {
				continue
			}

			numeroNFe = strings.TrimSpace(row[1])
			fornecedor = strings.TrimSpace(row[2])
			cfop = strings.TrimSpace(row[5])
			vProd = toFloat(row[7])
			icmsDue = toFloat(row[15])
		} else if bloco == "C" {
			if len(row) < 12 {
				continue
			}

			chaveNFe = strings.TrimSpace(row[11])
			if chaveNFe == "" {
				continue
			}

			numeroNFe = strings.TrimSpace(row[1])
			fornecedor = strings.TrimSpace(row[2])
			cfop = strings.TrimSpace(row[5])
			vProd = toFloat(row[7])
			icmsDue = toFloat(row[9])
		} else {
			continue
		}

		result[chaveNFe] = DiffRow{
			ChaveNFe:     chaveNFe,
			NumeroNFe:    numeroNFe,
			Fornecedor:   fornecedor,
			CFOP:         cfop,
			VProdP1:      vProd,
			ICMSDevidoP1: icmsDue,
		}
	}

	return result
}

func toFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, ",", ".")

	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func roundFloat(f float64, precision int) float64 {
	shift := math.Pow(10, float64(precision))
	return math.Round(f*shift) / shift
}
