package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
)

// ReformaParametros holds the reforma tributária simulation parameters for a company.
type ReformaParametros struct {
	CompanyID       string  `json:"company_id"`
	TargetAno       int     `json:"target_ano"`
	AliqIBSPct      float64 `json:"aliq_ibs_pct"`
	AliqCBSPct      float64 `json:"aliq_cbs_pct"`
	FatorSimplesPct float64 `json:"fator_simples_pct"`
	TaxaCDIAnualPct float64 `json:"taxa_cdi_anual_pct"`
	PrazoMedioDias  int     `json:"prazo_medio_dias"`
}

// GetReformaParametrosHandler returns the reforma parameters for the authenticated user's company.
// Any authenticated user can read (D-06). Returns {"parametros": null} if not yet configured.
func GetReformaParametrosHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		userID := claims["user_id"].(string)

		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			http.Error(w, "Error getting company: "+err.Error(), http.StatusInternalServerError)
			return
		}

		var p ReformaParametros
		err = db.QueryRow(`
			SELECT company_id, target_ano, aliq_ibs_pct, aliq_cbs_pct,
			       fator_simples_pct, taxa_cdi_anual_pct, prazo_medio_dias
			FROM reforma_parametros
			WHERE company_id = $1
		`, companyID).Scan(&p.CompanyID, &p.TargetAno, &p.AliqIBSPct, &p.AliqCBSPct,
			&p.FatorSimplesPct, &p.TaxaCDIAnualPct, &p.PrazoMedioDias)

		if err == sql.ErrNoRows {
			// Company has not yet configured parameters — return null (not an error).
			json.NewEncoder(w).Encode(map[string]interface{}{"parametros": nil})
			return
		}
		if err != nil {
			http.Error(w, "Error querying parametros: "+err.Error(), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{"parametros": p})
	}
}

// PutReformaParametrosHandler upserts the reforma parameters for the authenticated user's company.
// Requires role admin (enforced by AuthMiddleware in main.go — D-07).
// company_id is always derived from JWT (IDOR protection — T-06-09).
func PutReformaParametrosHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		userID := claims["user_id"].(string)

		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			http.Error(w, "Error getting company: "+err.Error(), http.StatusInternalServerError)
			return
		}

		var req ReformaParametros
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Input validation — V5 ASVS ranges (T-06-11)
		if req.AliqIBSPct < 0 || req.AliqIBSPct > 100 {
			http.Error(w, "aliq_ibs_pct deve estar entre 0 e 100", http.StatusBadRequest)
			return
		}
		if req.AliqCBSPct < 0 || req.AliqCBSPct > 100 {
			http.Error(w, "aliq_cbs_pct deve estar entre 0 e 100", http.StatusBadRequest)
			return
		}
		if req.FatorSimplesPct < 0 || req.FatorSimplesPct > 100 {
			http.Error(w, "fator_simples_pct deve estar entre 0 e 100", http.StatusBadRequest)
			return
		}
		if req.TaxaCDIAnualPct < 0 || req.TaxaCDIAnualPct > 100 {
			http.Error(w, "taxa_cdi_anual_pct deve estar entre 0 e 100", http.StatusBadRequest)
			return
		}
		if req.PrazoMedioDias < 1 || req.PrazoMedioDias > 3650 {
			http.Error(w, "prazo_medio_dias deve estar entre 1 e 3650", http.StatusBadRequest)
			return
		}
		if req.TargetAno < 2024 || req.TargetAno > 2100 {
			http.Error(w, "target_ano deve estar entre 2024 e 2100", http.StatusBadRequest)
			return
		}

		// UPSERT — DO UPDATE SET (never DO NOTHING for mutable parameters — Pitfall 2)
		// company_id from JWT ($1) — never from req.CompanyID (IDOR protection — T-06-09)
		// All values via parametrized placeholders — no string interpolation (T-06-10)
		_, err = db.Exec(`
			INSERT INTO reforma_parametros
			  (company_id, target_ano, aliq_ibs_pct, aliq_cbs_pct,
			   fator_simples_pct, taxa_cdi_anual_pct, prazo_medio_dias)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (company_id) DO UPDATE SET
			  target_ano         = $2,
			  aliq_ibs_pct       = $3,
			  aliq_cbs_pct       = $4,
			  fator_simples_pct  = $5,
			  taxa_cdi_anual_pct = $6,
			  prazo_medio_dias   = $7,
			  updated_at         = CURRENT_TIMESTAMP
		`, companyID, req.TargetAno, req.AliqIBSPct, req.AliqCBSPct,
			req.FatorSimplesPct, req.TaxaCDIAnualPct, req.PrazoMedioDias)
		if err != nil {
			http.Error(w, "Error saving parametros: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "Parâmetros salvos com sucesso"})
	}
}
