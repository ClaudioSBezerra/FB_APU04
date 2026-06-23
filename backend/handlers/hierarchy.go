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

		// 1. Empresa ativa: respeita X-Company-ID (company switcher) via GetEffectiveCompanyID.
		// Antes usava ORDER BY owner+created_at → sempre retornava a empresa mais antiga (SCALA),
		// ignorando a seleção do usuário.
		activeCompanyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			http.Error(w, "Empresa não encontrada: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// 2. Company details from the active company ID
		var company Company
		_ = db.QueryRow(`
			SELECT id, group_id, name, COALESCE(trade_name, ''), created_at
			FROM companies WHERE id = $1
		`, activeCompanyID).Scan(&company.ID, &company.GroupID, &company.Name, &company.TradeName, &company.CreatedAt)

		// 3. Group details
		var group EnterpriseGroup
		if company.GroupID != "" {
			_ = db.QueryRow("SELECT id, environment_id, name, COALESCE(description, ''), created_at FROM enterprise_groups WHERE id = $1", company.GroupID).Scan(
				&group.ID, &group.EnvironmentID, &group.Name, &group.Description, &group.CreatedAt)
		}

		// 4. Environment details
		var env Environment
		if group.EnvironmentID != "" {
			_ = db.QueryRow("SELECT id, name, COALESCE(description, ''), created_at FROM environments WHERE id = $1", group.EnvironmentID).Scan(
				&env.ID, &env.Name, &env.Description, &env.CreatedAt)
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
