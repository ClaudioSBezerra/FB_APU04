package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"

	"github.com/xuri/excelize/v2"
)

type DiffRow struct {
	Status       string  `json:"status"` // "only_1" | "only_2" | "diff"
	ChaveNFe     string  `json:"chave_nfe"`
	NumeroNFe    string  `json:"numero_nfe"`
	Fornecedor   string  `json:"fornecedor"`
	CFOP         string  `json:"cfop"`
	VProdP1      float64 `json:"v_prod_p1"`
	ICMSDevidoP1 float64 `json:"icms_devido_p1"`
	VProdP2      float64 `json:"v_prod_p2"`
	ICMSDevidoP2 float64 `json:"icms_devido_p2"`
	DiffICMS     float64 `json:"diff_icms"`
	Causa        string  `json:"causa"` // causa provável da divergência (heurística)
}

type ComparativoResponse struct {
	BlocoA []DiffRow `json:"bloco_a"`
	BlocoB []DiffRow `json:"bloco_b"`
	BlocoC []DiffRow `json:"bloco_c"`
}

// icmsTolerancia: diferenças de ICMS até este valor (em R$) são tratadas como
// arredondamento e NÃO marcadas como divergência. Notas ausentes em uma das
// planilhas (only_1/only_2) são sempre sinalizadas, independente do valor.
const icmsTolerancia = 0.05

// parsedRow é o dado relevante de uma linha de NF, já normalizado.
type parsedRow struct {
	chaveNFe    string
	numeroNFe   string
	fornecedor  string
	cfop        string
	vProd       float64
	icmsDevido  float64
	aliqInter   float64 // alíquota interestadual %
	aliqInterna float64 // alíquota interna %
	vIPI        float64 // V.IPI (só existe no export novo)
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

		f1, err := openUploadedXLSX(r, "file1")
		if err != nil {
			http.Error(w, "file1: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer f1.Close()

		f2, err := openUploadedXLSX(r, "file2")
		if err != nil {
			http.Error(w, "file2: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer f2.Close()

		sheets1 := findSheetsByKeyword(f1)
		sheets2 := findSheetsByKeyword(f2)

		resp := ComparativoResponse{
			BlocoA: compareBlocos(f1, f2, sheets1["anterior"], sheets2["anterior"]),
			BlocoB: compareBlocos(f1, f2, sheets1["atual"], sheets2["atual"]),
			BlocoC: compareBlocos(f1, f2, sheets1["nao_sped"], sheets2["nao_sped"]),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func openUploadedXLSX(r *http.Request, field string) (*excelize.File, error) {
	file, _, err := r.FormFile(field)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	return excelize.OpenReader(bytes.NewReader(data))
}

func findSheetsByKeyword(f *excelize.File) map[string]string {
	result := map[string]string{}
	for _, sheet := range f.GetSheetList() {
		lower := strings.ToLower(sheet)
		switch {
		case strings.Contains(lower, "anterior"):
			result["anterior"] = sheet
		case strings.Contains(lower, "atual"):
			result["atual"] = sheet
		case strings.Contains(lower, "não") || strings.Contains(lower, "nao") || strings.Contains(lower, "sped"):
			result["nao_sped"] = sheet
		}
	}
	return result
}

func compareBlocos(f1, f2 *excelize.File, sheet1, sheet2 string) []DiffRow {
	var diffs []DiffRow
	if sheet1 == "" || sheet2 == "" {
		return diffs
	}

	rows1 := getSheetRows(f1, sheet1)
	rows2 := getSheetRows(f2, sheet2)

	// Notas na planilha 1: faltando na 2, ou com ICMS divergente.
	for chave, r1 := range rows1 {
		r2, ok := rows2[chave]
		if !ok {
			diffs = append(diffs, DiffRow{
				Status: "only_1", ChaveNFe: chave,
				NumeroNFe: r1.numeroNFe, Fornecedor: r1.fornecedor, CFOP: r1.cfop,
				VProdP1: r1.vProd, ICMSDevidoP1: r1.icmsDevido,
				DiffICMS: r1.icmsDevido,
				Causa:    diagnoseCausa("only_1", r1, parsedRow{}),
			})
			continue
		}
		d := roundFloat(r1.icmsDevido, 2) - roundFloat(r2.icmsDevido, 2)
		if math.Abs(d) > icmsTolerancia {
			diffs = append(diffs, DiffRow{
				Status: "diff", ChaveNFe: chave,
				NumeroNFe: r1.numeroNFe, Fornecedor: r1.fornecedor, CFOP: r1.cfop,
				VProdP1: r1.vProd, ICMSDevidoP1: r1.icmsDevido,
				VProdP2: r2.vProd, ICMSDevidoP2: r2.icmsDevido,
				DiffICMS: d,
				Causa:    diagnoseCausa("diff", r1, r2),
			})
		}
		delete(rows2, chave) // marca como já comparada
	}

	// Sobrou na planilha 2 → não existe na 1.
	for chave, r2 := range rows2 {
		diffs = append(diffs, DiffRow{
			Status: "only_2", ChaveNFe: chave,
			NumeroNFe: r2.numeroNFe, Fornecedor: r2.fornecedor, CFOP: r2.cfop,
			VProdP2: r2.vProd, ICMSDevidoP2: r2.icmsDevido,
			DiffICMS: -r2.icmsDevido,
			Causa:    diagnoseCausa("only_2", parsedRow{}, r2),
		})
	}

	return diffs
}

// diagnoseCausa infere a causa PROVÁVEL de uma divergência a partir dos números
// das duas planilhas. Heurística baseada no modelo de cálculo:
// ICMS ≈ base × (alíq.interna − alíq.inter)/100, com base = V.Prod (+ IPI/frete).
func diagnoseCausa(status string, r1, r2 parsedRow) string {
	switch status {
	case "only_1":
		return "Nota ausente na planilha de conferência (P2)"
	case "only_2":
		return "Nota ausente na planilha correta (P1)"
	}

	// --- divergência de valor (presente nas duas) ---

	// 1) Base de cálculo diferente (V.Prod diverge).
	if math.Abs(r1.vProd-r2.vProd) > 0.05 {
		return fmt.Sprintf("Base de cálculo difere — V.Prod P1 %s vs P2 %s", brl(r1.vProd), brl(r2.vProd))
	}

	// 2) Alíquota interestadual diferente (mix 4%/12% ou mínimo SN 4%).
	if r1.aliqInter > 0 && r2.aliqInter > 0 && math.Abs(r1.aliqInter-r2.aliqInter) > 0.01 {
		return fmt.Sprintf("Alíquota interestadual difere — P1 %.2f%% vs P2 %.2f%% (mix 4%%/12%% ou mínimo SN)",
			r1.aliqInter, r2.aliqInter)
	}

	// 3) IPI na base — confirma se a diferença de ICMS bate com o efeito do IPI.
	// Duas formas possíveis: IPI × alíq.interna (sem crédito interestadual sobre
	// o IPI) ou IPI × (interna−inter). Se qualquer uma casar, confirma a causa.
	vIPI := math.Max(r1.vIPI, r2.vIPI)
	if vIPI > 0.005 {
		ai, an := r2.aliqInter, r2.aliqInterna // planilha que tem IPI (export novo)
		if r2.vIPI < r1.vIPI {
			ai, an = r1.aliqInter, r1.aliqInterna
		}
		diff := math.Abs(roundFloat(r1.icmsDevido, 2) - roundFloat(r2.icmsDevido, 2))
		tol := math.Max(0.05, 0.03*diff)

		efeitoCheio := vIPI * an / 100.0      // IPI × interna
		efeitoLiq := vIPI * (an - ai) / 100.0 // IPI × (interna − inter)
		if an > 0 && math.Abs(efeitoCheio-diff) <= tol {
			return fmt.Sprintf("IPI na base — V.IPI %s × %.1f%% (interna) ≈ %s = a diferença",
				brl(vIPI), an, brl(efeitoCheio))
		}
		if an > 0 && ai > 0 && math.Abs(efeitoLiq-diff) <= tol {
			return fmt.Sprintf("IPI na base — V.IPI %s × (%.1f%%−%.1f%%) ≈ %s = a diferença",
				brl(vIPI), an, ai, brl(efeitoLiq))
		}
		// Sem colunas de alíquota (Bloco C): confirma pela taxa implícita Δ/V.IPI,
		// se cair numa faixa plausível de alíquota interna (15%–25%).
		if diff > 0 {
			taxa := diff / vIPI * 100.0
			if taxa >= 15.0 && taxa <= 25.0 {
				return fmt.Sprintf("IPI na base — V.IPI %s × ~%.1f%% ≈ %s = a diferença",
					brl(vIPI), taxa, brl(diff))
			}
		}
		return fmt.Sprintf("Possível IPI na base — V.IPI %s presente (efeito não confere exato)", brl(vIPI))
	}

	// 4) Alíquota interna diferente.
	if r1.aliqInterna > 0 && r2.aliqInterna > 0 && math.Abs(r1.aliqInterna-r2.aliqInterna) > 0.01 {
		return fmt.Sprintf("Alíquota interna difere — P1 %.2f%% vs P2 %.2f%%", r1.aliqInterna, r2.aliqInterna)
	}

	// 5) Mesma base e alíquotas → diferença de crédito/arredondamento de cálculo.
	return "Diferença de crédito/cálculo (mesma base e alíquotas)"
}

// colIndex localiza colunas pelo NOME do cabeçalho (robusto a layouts diferentes:
// a planilha do contador é um export antigo sem as colunas V.IPI/V.BC Antecip.,
// então os índices fixos não servem). Retorna um mapa campo→índice.
type colIndex struct {
	chaveNFe, chaveCTe, numeroNFe, fornecedor, cfop, vProd, icmsDevido int
	aliqInter, aliqInterna, vIPI                                       int
}

func detectColumns(header []string) colIndex {
	ci := colIndex{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	for i, raw := range header {
		h := strings.ToLower(strings.TrimSpace(raw))
		switch {
		case strings.Contains(h, "chave") && strings.Contains(h, "ct"):
			if ci.chaveCTe < 0 {
				ci.chaveCTe = i
			}
		case strings.Contains(h, "chave") && strings.Contains(h, "nf"):
			if ci.chaveNFe < 0 {
				ci.chaveNFe = i
			}
		case strings.Contains(h, "icms") && strings.Contains(h, "est"):
			// "ICMS Devido Est." (A/B) ou "ICMS Est." (C) — exclui "ICMS Atual".
			if ci.icmsDevido < 0 {
				ci.icmsDevido = i
			}
		case strings.Contains(h, "interna"):
			// "Alíq.Interna.%"
			if ci.aliqInterna < 0 {
				ci.aliqInterna = i
			}
		case strings.Contains(h, "inter"):
			// "Alíq.Inter.%" (interestadual) — depois de "interna" no switch.
			if ci.aliqInter < 0 {
				ci.aliqInter = i
			}
		case strings.Contains(h, "ipi"):
			// "V.IPI" (só no export novo).
			if ci.vIPI < 0 {
				ci.vIPI = i
			}
		case strings.Contains(h, "nf-e") && !strings.Contains(h, "chave"):
			// "Número NF-e" (A/B) ou "NF-e" (C); evita colidir com "Chave NF-e".
			if ci.numeroNFe < 0 {
				ci.numeroNFe = i
			}
		case strings.Contains(h, "fornecedor"):
			if ci.fornecedor < 0 {
				ci.fornecedor = i
			}
		case strings.Contains(h, "cfop"):
			if ci.cfop < 0 {
				ci.cfop = i
			}
		case strings.Contains(h, "v.prod") || strings.Contains(h, "opera"):
			// "V.Prod" (A/B) ou "V.Operação" (C).
			if ci.vProd < 0 {
				ci.vProd = i
			}
		}
	}
	return ci
}

func getSheetRows(f *excelize.File, sheet string) map[string]parsedRow {
	result := make(map[string]parsedRow)
	rows, err := f.GetRows(sheet)
	if err != nil || len(rows) == 0 {
		return result
	}

	ci := detectColumns(rows[0])
	if ci.chaveNFe < 0 {
		return result // sem coluna-chave não há como casar
	}

	at := func(row []string, idx int) string {
		if idx < 0 || idx >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[idx])
	}

	for i := 1; i < len(rows); i++ {
		row := rows[i]
		if len(row) == 0 {
			continue
		}
		// Pula linhas de CT-e (têm Chave CT-e preenchida) — evita dupla contagem.
		if ci.chaveCTe >= 0 && at(row, ci.chaveCTe) != "" {
			continue
		}
		chave := at(row, ci.chaveNFe)
		if chave == "" {
			continue
		}
		result[chave] = parsedRow{
			chaveNFe:    chave,
			numeroNFe:   at(row, ci.numeroNFe),
			fornecedor:  at(row, ci.fornecedor),
			cfop:        at(row, ci.cfop),
			vProd:       parseMoney(at(row, ci.vProd)),
			icmsDevido:  parseMoney(at(row, ci.icmsDevido)),
			aliqInter:   parseMoney(at(row, ci.aliqInter)),
			aliqInterna: parseMoney(at(row, ci.aliqInterna)),
			vIPI:        parseMoney(at(row, ci.vIPI)),
		}
	}
	return result
}

// parseMoney converte valores monetários de planilha para float64.
// Suporta os dois formatos que aparecem nos exports:
//   - US c/ prefixo:   "R$ 1,774.48"  (vírgula=milhar, ponto=decimal)
//   - BR:              "1.774,48"      (ponto=milhar, vírgula=decimal)
//
// Regra universal: o separador MAIS À DIREITA é o decimal; o outro é milhar.
func parseMoney(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	// Mantém apenas dígitos, separadores e sinal.
	var b strings.Builder
	for _, r := range s {
		if (r >= '0' && r <= '9') || r == ',' || r == '.' || r == '-' {
			b.WriteRune(r)
		}
	}
	cleaned := b.String()
	if cleaned == "" || cleaned == "-" {
		return 0
	}

	lastComma := strings.LastIndex(cleaned, ",")
	lastDot := strings.LastIndex(cleaned, ".")

	var decimalSep, thousandSep string
	switch {
	case lastComma >= 0 && lastDot >= 0:
		if lastComma > lastDot {
			decimalSep, thousandSep = ",", "."
		} else {
			decimalSep, thousandSep = ".", ","
		}
	case lastComma >= 0:
		decimalSep = ","
	case lastDot >= 0:
		decimalSep = "."
	}

	if thousandSep != "" {
		cleaned = strings.ReplaceAll(cleaned, thousandSep, "")
	}
	if decimalSep != "" && decimalSep != "." {
		cleaned = strings.ReplaceAll(cleaned, decimalSep, ".")
	}

	return parseFloatSafe(cleaned)
}

func parseFloatSafe(s string) float64 {
	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = s[1:]
	}
	parts := strings.SplitN(s, ".", 2)
	whole, frac := parts[0], ""
	if len(parts) == 2 {
		frac = parts[1]
	}

	var val float64
	for _, c := range whole {
		if c < '0' || c > '9' {
			return 0
		}
		val = val*10 + float64(c-'0')
	}
	div := 1.0
	for _, c := range frac {
		if c < '0' || c > '9' {
			break
		}
		val = val*10 + float64(c-'0')
		div *= 10
	}
	val /= div
	if neg {
		val = -val
	}
	return val
}

func roundFloat(f float64, precision int) float64 {
	shift := math.Pow(10, float64(precision))
	return math.Round(f*shift) / shift
}
