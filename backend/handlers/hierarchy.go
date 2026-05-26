package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

type UserHierarchyResponse struct {
	Environment Environment     `json:"environment"`
	Group       EnterpriseGroup `json:"group"`
	Company     Company         `json:"company"`
	Branches    []Branch        `json:"branches"`
}

type Branch struct {
	CNPJ              string `json:"cnpj"`
	CompanyName       string `json:"company_name"`
	UF                string `json:"uf"`                 // UF do estabelecimento (reg 0000 do SPED)
	InscricaoEstadual string `json:"inscricao_estadual"` // IE (reg 0000)
	CodMunicipio      string `json:"cod_municipio"`      // código IBGE do município (reg 0000)
	MunicipioNome     string `json:"municipio_nome"`     // nome do município (ref municipios_ibge)
	UFNome            string `json:"uf_nome"`            // nome da UF (ref municipios_ibge)
}

func GetUserHierarchyHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := GetUserIDFromContext(r)
		if userID == "" {
			http.Error(w, "User ID not found in context", http.StatusUnauthorized)
			return
		}

		// 1. Get Environment ID for the user
		var envID string
		err := db.QueryRow("SELECT environment_id FROM user_environments WHERE user_id = $1 LIMIT 1", userID).Scan(&envID)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "User not assigned to any environment", http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// 2. Get Environment Details
		var env Environment
		err = db.QueryRow("SELECT id, name, COALESCE(description, ''), created_at FROM environments WHERE id = $1", envID).Scan(&env.ID, &env.Name, &env.Description, &env.CreatedAt)
		if err != nil {
			http.Error(w, "Environment not found", http.StatusNotFound)
			return
		}

		// 3. Get Group Details (First group in the environment)
		var group EnterpriseGroup
		err = db.QueryRow("SELECT id, environment_id, name, COALESCE(description, ''), created_at FROM enterprise_groups WHERE environment_id = $1 LIMIT 1", envID).Scan(&group.ID, &group.EnvironmentID, &group.Name, &group.Description, &group.CreatedAt)
		if err != nil && err != sql.ErrNoRows {
			// Log error but continue?
		}

		// 4. Get Company Details (Prioritize company owned by user)
		var company Company
		// var companyCNPJ string // CNPJ removed from companies table
		if group.ID != "" {
			// Removed 'cnpj' from SELECT list as it no longer exists in companies table
			// Prioritize the company owned by the user, then fallback to any company in the group
			err = db.QueryRow(`
				SELECT id, group_id, name, COALESCE(trade_name, ''), created_at 
				FROM companies 
				WHERE group_id = $1 
				ORDER BY (owner_id = $2) DESC, created_at ASC 
				LIMIT 1
			`, group.ID, userID).Scan(&company.ID, &company.GroupID, &company.Name, &company.TradeName, &company.CreatedAt)

			if err != nil && err != sql.ErrNoRows {
				// Log error if needed
			}
		}

		// 5. Get Branches (Filiais) from import_jobs using Company ID
		var branches []Branch
		if company.ID != "" {
			// Filiais (CNPJ) da empresa com UF, IE e município do estabelecimento
			// (reg 0000 do SPED). Resolve o código IBGE → nome do município/UF
			// via municipios_ibge (LEFT JOIN — código pode estar vazio em imports
			// antigos, então o nome volta vazio).
			rows, err := db.Query(`
                SELECT j.cnpj,
                       j.company_name,
                       j.uf,
                       j.inscricao_estadual,
                       j.cod_municipio,
                       COALESCE(m.nome, '')    AS municipio_nome,
                       COALESCE(m.uf_nome, '') AS uf_nome
                FROM (
                    SELECT cnpj,
                           MAX(company_name)                    AS company_name,
                           COALESCE(MAX(uf), '')                AS uf,
                           COALESCE(MAX(inscricao_estadual),'') AS inscricao_estadual,
                           COALESCE(MAX(cod_municipio), '')     AS cod_municipio
                    FROM import_jobs
                    WHERE company_id = $1 AND status = 'completed'
                      AND cnpj IS NOT NULL AND cnpj <> ''
                    GROUP BY cnpj
                ) j
                LEFT JOIN municipios_ibge m ON m.codigo_ibge = j.cod_municipio
                ORDER BY j.cnpj
            `, company.ID)

			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var b Branch
					var cName, cCNPJ, cUF, cIE, cMun, cMunNome, cUFNome sql.NullString
					if err := rows.Scan(&cCNPJ, &cName, &cUF, &cIE, &cMun, &cMunNome, &cUFNome); err == nil {
						if cCNPJ.Valid {
							b.CNPJ = cCNPJ.String
							b.CompanyName = cName.String
							b.UF = cUF.String
							b.InscricaoEstadual = cIE.String
							b.CodMunicipio = cMun.String
							b.MunicipioNome = cMunNome.String
							b.UFNome = cUFNome.String
							branches = append(branches, b)
						}
					}
				}
			}
		}

		resp := UserHierarchyResponse{
			Environment: env,
			Group:       group,
			Company:     company,
			Branches:    branches,
		}
		if resp.Branches == nil {
			resp.Branches = []Branch{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
