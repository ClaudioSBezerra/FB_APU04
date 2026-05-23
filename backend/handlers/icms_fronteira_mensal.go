package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
)

// ---------------------------------------------------------------------------
// Structs
// ---------------------------------------------------------------------------

type FronteiraMensalRow struct {
	Periodo       string  `json:"periodo"`
	Regime        string  `json:"regime"`
	QtdNotas      int     `json:"qtd_notas"`
	VProdTotal    float64 `json:"v_prod_total"`
	IcmsDevido    float64 `json:"icms_devido"`
}

type FronteiraMensalResponse struct {
	Rows        []FronteiraMensalRow `json:"rows"`
	TotalDevido float64              `json:"total_devido"`
	TotalProd   float64              `json:"total_prod"`
}

// ---------------------------------------------------------------------------
// IcmsFronteiraMensalHandler — GET /api/icms-fronteira/mensal
// ---------------------------------------------------------------------------
// Returns monthly ICMS totals per regime, ordered by period ascending.
// $1 = company_id, $2 = periodo filter MM/YYYY ('' = all periods)
//
// The query reuses the fronteiraBaseQuery CTE directly to keep the
// classification logic in one place.

func IcmsFronteiraMensalHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodGet {
			jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
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
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa: "+err.Error())
			return
		}

		// Bloco A: periodo '' = mostra todos os meses
		periodo := r.URL.Query().Get("periodo")

		query := fronteiraBaseQuery + `
SELECT
    TO_CHAR(DATE_TRUNC('month', data_emissao::date), 'MM/YYYY') AS periodo,
    regime,
    COUNT(DISTINCT chave_nfe)  AS qtd_notas,
    SUM(v_prod)                AS v_prod_total,
    SUM(icms_devido_est)       AS icms_devido
FROM classified
WHERE regime IS NOT NULL
GROUP BY 1, 2
ORDER BY MIN(data_emissao::date), regime
`
		rows, err := db.Query(query, companyID, periodo)
		if err != nil {
			log.Printf("IcmsFronteiraMensal error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar apuração mensal")
			return
		}
		defer rows.Close()

		result := []FronteiraMensalRow{}
		var totalDevido, totalProd float64

		for rows.Next() {
			var row FronteiraMensalRow
			if err := rows.Scan(&row.Periodo, &row.Regime, &row.QtdNotas, &row.VProdTotal, &row.IcmsDevido); err != nil {
				log.Printf("IcmsFronteiraMensal scan error: %v", err)
				continue
			}
			totalDevido += row.IcmsDevido
			totalProd += row.VProdTotal
			result = append(result, row)
		}

		json.NewEncoder(w).Encode(FronteiraMensalResponse{
			Rows:        result,
			TotalDevido: totalDevido,
			TotalProd:   totalProd,
		})
	}
}
