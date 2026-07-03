// fiscal_comparacao_csv.go — exportação CSV da tela "Comparação Fiscal"
// (Fase 12, TPF-06/TPF-07).
//
//	GET /api/fiscal/comparacao/csv?nfe_id=...   → FiscalComparacaoCSVHandler
//
// Reusa queryComparacaoRows (fiscal_comparacao.go, Task 1) — não reimplementa
// a soma de IBS. Escopo de colunas resolvido no planejamento (12-01-PLAN.md,
// Task 2 / 12-RESEARCH.md Open Question 2): espelha SOMENTE as 6 impostos ×
// esperado/calculado/diferença da tabela visível + identificação — os campos
// só-calculado (DIFAL, FCP, código do grupo fiscal e resultado completo) são
// dialog-only e ficam fora deste export.
package handlers

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// ---------------------------------------------------------------------------
// GET /api/fiscal/comparacao/csv?nfe_id=...
// Exporta a comparação item a item de uma nota em CSV (6 impostos x
// esperado/calculado/diferença), mesmo preâmbulo de auth/companyID/nfe_id do
// FiscalComparacaoReadHandler.
// ---------------------------------------------------------------------------
func FiscalComparacaoCSVHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonErr(w, http.StatusMethodNotAllowed, "Método não permitido")
			return
		}

		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		userID, _ := claims["user_id"].(string)

		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa")
			return
		}

		nfeID := strings.TrimSpace(r.URL.Query().Get("nfe_id"))
		if nfeID == "" {
			jsonErr(w, http.StatusBadRequest, "nfe_id é obrigatório")
			return
		}

		data, err := queryComparacaoRows(db, nfeID, companyID)
		if err != nil {
			log.Printf("[FiscalComparacaoCSV] query error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar comparação fiscal")
			return
		}

		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="comparacao-fiscal.csv"`)

		cw := csv.NewWriter(w)

		header := []string{
			"Item", "Produto", "NCM", "CFOP", "Status",
			"ICMS Esperado", "ICMS Calculado", "ICMS Diferença",
			"ICMS-ST Esperado", "ICMS-ST Calculado", "ICMS-ST Diferença",
			"PIS Esperado", "PIS Calculado", "PIS Diferença",
			"COFINS Esperado", "COFINS Calculado", "COFINS Diferença",
			"IBS Esperado", "IBS Calculado", "IBS Diferença",
			"CBS Esperado", "CBS Calculado", "CBS Diferença",
			"Base IBS/CBS Calculado",
		}
		if err := cw.Write(header); err != nil {
			log.Printf("[FiscalComparacaoCSV] write header error: %v", err)
			return
		}

		for _, row := range data {
			icmsCalc := ptrFloat(row.ValorIcms)
			stCalc := ptrFloat(row.ValorSubstituicao)
			pisCalc := ptrFloat(row.ValorPis)
			cofinsCalc := ptrFloat(row.ValorCofins)
			ibsCalc := ptrFloat(row.ValorIbsTotal)
			cbsCalc := ptrFloat(row.ValorCbs)
			baseIbsCbsCalc := ptrFloat(row.BaseCalculoIbsCbs)

			record := []string{
				fmt.Sprintf("%d", row.NItem), row.XProd, row.NCM, row.CFOP, row.Status,
				fmt.Sprintf("%.2f", row.VIcms), fmt.Sprintf("%.2f", icmsCalc), fmt.Sprintf("%.2f", row.VIcms-icmsCalc),
				fmt.Sprintf("%.2f", row.VSt), fmt.Sprintf("%.2f", stCalc), fmt.Sprintf("%.2f", row.VSt-stCalc),
				fmt.Sprintf("%.2f", row.VPis), fmt.Sprintf("%.2f", pisCalc), fmt.Sprintf("%.2f", row.VPis-pisCalc),
				fmt.Sprintf("%.2f", row.VCofins), fmt.Sprintf("%.2f", cofinsCalc), fmt.Sprintf("%.2f", row.VCofins-cofinsCalc),
				fmt.Sprintf("%.2f", row.VIbs), fmt.Sprintf("%.2f", ibsCalc), fmt.Sprintf("%.2f", row.VIbs-ibsCalc),
				fmt.Sprintf("%.2f", row.VCbs), fmt.Sprintf("%.2f", cbsCalc), fmt.Sprintf("%.2f", row.VCbs-cbsCalc),
				fmt.Sprintf("%.2f", baseIbsCbsCalc),
			}
			if err := cw.Write(record); err != nil {
				log.Printf("[FiscalComparacaoCSV] write row error: %v", err)
				return
			}
		}

		cw.Flush()
		if err := cw.Error(); err != nil {
			log.Printf("[FiscalComparacaoCSV] flush error: %v", err)
		}
	}
}

// ptrFloat retorna 0 quando o ponteiro é nil (item nunca executado ou coluna
// NULL no LEFT JOIN) — mesma semântica de COALESCE(...,0) usada no SQL.
func ptrFloat(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}
