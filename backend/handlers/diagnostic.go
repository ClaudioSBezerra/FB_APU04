package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
)

// DiagnosticResponse expõe o estado dos dados do usuário logado:
// qual company_id está sendo usado, quantos registros existem nas
// principais tabelas para esse company_id, e quantos existem em outras
// company_ids (para detectar dados órfãos).
type DiagnosticResponse struct {
	UserID                string         `json:"user_id"`
	EffectiveCompanyID    string         `json:"effective_company_id"`
	EffectiveCompanyError string         `json:"effective_company_error,omitempty"`
	XCompanyHeader        string         `json:"x_company_header"`
	CountsForCompany      map[string]int `json:"counts_for_company"`
	OtherCompaniesData    []CompanyData  `json:"other_companies_data"`
}

type CompanyData struct {
	CompanyID     string `json:"company_id"`
	CompanyName   string `json:"company_name"`
	NfeEntradas   int    `json:"nfe_entradas"`
	NfeSaidas     int    `json:"nfe_saidas"`
	RegC100       int    `json:"reg_c100"`
	CteEntradas   int    `json:"cte_entradas"`
}

// DiagnosticDataHandler — GET /api/admin/diagnostic
// Retorna o estado dos dados para diagnóstico de "relatórios vazios".
func DiagnosticDataHandler(db *sql.DB) http.HandlerFunc {
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

		resp := DiagnosticResponse{
			UserID:           userID,
			XCompanyHeader:   r.Header.Get("X-Company-ID"),
			CountsForCompany: map[string]int{},
		}

		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			resp.EffectiveCompanyError = err.Error()
		} else {
			resp.EffectiveCompanyID = companyID

			tables := []struct {
				name  string
				query string
			}{
				{"nfe_entradas", "SELECT COUNT(*) FROM nfe_entradas WHERE company_id = $1"},
				{"nfe_saidas", "SELECT COUNT(*) FROM nfe_saidas WHERE company_id = $1"},
				{"cte_entradas", "SELECT COUNT(*) FROM cte_entradas WHERE company_id = $1"},
				{"nfe_entradas_itens", "SELECT COUNT(*) FROM nfe_entradas_itens WHERE company_id = $1"},
				{"import_jobs", "SELECT COUNT(*) FROM import_jobs WHERE company_id = $1"},
				{"reg_c100", "SELECT COUNT(*) FROM reg_c100 c100 JOIN import_jobs j ON j.id = c100.job_id WHERE j.company_id = $1"},
				{"reg_c190", "SELECT COUNT(*) FROM reg_c190 c190 JOIN reg_c100 c100 ON c100.id = c190.id_pai_c100 JOIN import_jobs j ON j.id = c100.job_id WHERE j.company_id = $1"},
				{"reforma_parametros", "SELECT COUNT(*) FROM reforma_parametros WHERE company_id = $1"},
			}
			for _, t := range tables {
				var n int
				if err := db.QueryRow(t.query, companyID).Scan(&n); err != nil {
					log.Printf("diagnostic %s scan error: %v", t.name, err)
					resp.CountsForCompany[t.name] = -1
				} else {
					resp.CountsForCompany[t.name] = n
				}
			}
		}

		// Lista todas as outras companies que têm dados (útil para detectar
		// dados importados em company_id diferente do que o usuário está usando).
		rows, err := db.Query(`
			SELECT c.id, c.name,
			       COALESCE((SELECT COUNT(*) FROM nfe_entradas WHERE company_id = c.id), 0),
			       COALESCE((SELECT COUNT(*) FROM nfe_saidas WHERE company_id = c.id), 0),
			       COALESCE((SELECT COUNT(*) FROM reg_c100 c100 JOIN import_jobs j ON j.id = c100.job_id WHERE j.company_id = c.id), 0),
			       COALESCE((SELECT COUNT(*) FROM cte_entradas WHERE company_id = c.id), 0)
			FROM companies c
			ORDER BY c.created_at DESC
		`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var cd CompanyData
				if err := rows.Scan(&cd.CompanyID, &cd.CompanyName, &cd.NfeEntradas, &cd.NfeSaidas, &cd.RegC100, &cd.CteEntradas); err == nil {
					if cd.NfeEntradas > 0 || cd.NfeSaidas > 0 || cd.RegC100 > 0 || cd.CteEntradas > 0 {
						resp.OtherCompaniesData = append(resp.OtherCompaniesData, cd)
					}
				}
			}
		}

		json.NewEncoder(w).Encode(resp)
	}
}
