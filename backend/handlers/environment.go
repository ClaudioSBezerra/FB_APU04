package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"regexp"

	"github.com/golang-jwt/jwt/v5"
	"github.com/lib/pq"
)

// Structures
type Environment struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
}

type EnterpriseGroup struct {
	ID            string `json:"id"`
	EnvironmentID string `json:"environment_id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	CreatedAt     string `json:"created_at"`
}

type Company struct {
	ID                string           `json:"id"`
	GroupID           string           `json:"group_id"`
	Name              string           `json:"name"`
	TradeName         string           `json:"trade_name"`
	RegimeTributario  string           `json:"regime_tributario"`
	CNPJ              string           `json:"cnpj,omitempty"`
	InscricaoEstadual string           `json:"inscricao_estadual,omitempty"`
	CNAEPrincipal     string           `json:"cnae_principal,omitempty"`
	CNAESecundario    []string         `json:"cnae_secundario,omitempty"`
	Municipio         string           `json:"municipio,omitempty"`
	SegmentoEconomico string           `json:"segmento_economico,omitempty"`
	IncentivosFiscais *json.RawMessage `json:"incentivos_fiscais,omitempty"`
	CreatedAt         string           `json:"created_at"`
}

// --- Environment Handlers ---

func GetEnvironmentsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		userID := claims["user_id"].(string)
		role := claims["role"].(string)

		log.Printf("[GetEnvironments] User: %s, Role: %s", userID, role)

		var rows *sql.Rows
		var err error

		if role == "admin" {
			// Platform Admin sees all environments
			rows, err = db.Query("SELECT id, name, COALESCE(description, ''), created_at FROM environments ORDER BY name")
		} else {
			// Regular users see only assigned environments
			rows, err = db.Query(`
				SELECT e.id, e.name, COALESCE(e.description, ''), e.created_at 
				FROM environments e
				JOIN user_environments ue ON e.id = ue.environment_id
				WHERE ue.user_id = $1
				ORDER BY e.name
			`, userID)
		}

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var envs []Environment
		for rows.Next() {
			var e Environment
			if err := rows.Scan(&e.ID, &e.Name, &e.Description, &e.CreatedAt); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			envs = append(envs, e)
		}

		if envs == nil {
			envs = []Environment{}
		}
		json.NewEncoder(w).Encode(envs)
	}
}

func CreateEnvironmentHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var e Environment
		if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		err := db.QueryRow(
			"INSERT INTO environments (name, description) VALUES ($1, $2) RETURNING id, created_at",
			e.Name, e.Description,
		).Scan(&e.ID, &e.CreatedAt)

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(e)
	}
}

func UpdateEnvironmentHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Expects ID in URL or Body. For simplicity, we take from body now or just update based on ID
		var e Environment
		if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		_, err := db.Exec(
			"UPDATE environments SET name = $1, description = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $3",
			e.Name, e.Description, e.ID,
		)

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(e)
	}
}

func DeleteEnvironmentHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "Missing id parameter", http.StatusBadRequest)
			return
		}

		_, err := db.Exec("DELETE FROM environments WHERE id = $1", id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

// --- Group Handlers ---

func GetGroupsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		envID := r.URL.Query().Get("environment_id")
		query := "SELECT id, environment_id, name, COALESCE(description, ''), created_at FROM enterprise_groups"
		args := []interface{}{}

		if envID != "" {
			query += " WHERE environment_id = $1"
			args = append(args, envID)
		}
		query += " ORDER BY name"

		rows, err := db.Query(query, args...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var groups []EnterpriseGroup
		for rows.Next() {
			var g EnterpriseGroup
			if err := rows.Scan(&g.ID, &g.EnvironmentID, &g.Name, &g.Description, &g.CreatedAt); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			groups = append(groups, g)
		}

		if groups == nil {
			groups = []EnterpriseGroup{}
		}
		json.NewEncoder(w).Encode(groups)
	}
}

func CreateGroupHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var g EnterpriseGroup
		if err := json.NewDecoder(r.Body).Decode(&g); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		err := db.QueryRow(
			"INSERT INTO enterprise_groups (environment_id, name, description) VALUES ($1, $2, $3) RETURNING id, created_at",
			g.EnvironmentID, g.Name, g.Description,
		).Scan(&g.ID, &g.CreatedAt)

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(g)
	}
}

func DeleteGroupHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "Missing id parameter", http.StatusBadRequest)
			return
		}

		_, err := db.Exec("DELETE FROM enterprise_groups WHERE id = $1", id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

// --- Company Handlers ---

func GetCompaniesHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID := r.URL.Query().Get("group_id")
		query := `SELECT id, group_id, name,
			COALESCE(trade_name, ''),
			COALESCE(regime_tributario, 'nao_informado'),
			COALESCE(cnpj, ''),
			COALESCE(inscricao_estadual, ''),
			COALESCE(cnae_principal, ''),
			COALESCE(cnae_secundario, '{}'::text[]),
			COALESCE(municipio, ''),
			COALESCE(segmento_economico, ''),
			incentivos_fiscais,
			created_at
		FROM companies`
		args := []interface{}{}

		if groupID != "" {
			query += " WHERE group_id = $1"
			args = append(args, groupID)
		}
		query += " ORDER BY name"

		rows, err := db.Query(query, args...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var companies []Company
		for rows.Next() {
			var c Company
			var inc sql.NullString
			if err := rows.Scan(
				&c.ID, &c.GroupID, &c.Name, &c.TradeName, &c.RegimeTributario,
				&c.CNPJ, &c.InscricaoEstadual, &c.CNAEPrincipal,
				pq.Array(&c.CNAESecundario),
				&c.Municipio, &c.SegmentoEconomico,
				&inc,
				&c.CreatedAt,
			); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if inc.Valid && inc.String != "" {
				raw := json.RawMessage(inc.String)
				c.IncentivosFiscais = &raw
			}
			companies = append(companies, c)
		}

		if companies == nil {
			companies = []Company{}
		}
		json.NewEncoder(w).Encode(companies)
	}
}

func CreateCompanyHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			GroupID           string           `json:"group_id"`
			Name              string           `json:"name"`
			TradeName         string           `json:"trade_name"`
			RegimeTributario  string           `json:"regime_tributario"`
			CNPJ              string           `json:"cnpj"`
			InscricaoEstadual string           `json:"inscricao_estadual"`
			CNAEPrincipal     string           `json:"cnae_principal"`
			CNAESecundario    []string         `json:"cnae_secundario"`
			Municipio         string           `json:"municipio"`
			SegmentoEconomico string           `json:"segmento_economico"`
			IncentivosFiscais *json.RawMessage `json:"incentivos_fiscais"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Basic validation
		if payload.Name == "" || payload.GroupID == "" {
			http.Error(w, "Missing required fields (name, group_id)", http.StatusBadRequest)
			return
		}

		// Validação CNPJ: 14 dígitos numéricos quando fornecido
		if payload.CNPJ != "" {
			re := regexp.MustCompile(`^\d{14}$`)
			if !re.MatchString(payload.CNPJ) {
				http.Error(w, "CNPJ deve ter 14 dígitos numéricos", http.StatusBadRequest)
				return
			}
		}

		// Resolve owner: use group's environment owner (first user linked to the environment)
		var ownerID *string
		err := db.QueryRow(`
			SELECT ue.user_id
			FROM enterprise_groups eg
			JOIN user_environments ue ON ue.environment_id = eg.environment_id
			WHERE eg.id = $1
			ORDER BY ue.created_at ASC
			LIMIT 1
		`, payload.GroupID).Scan(&ownerID)
		if err != nil {
			ownerID = nil // no owner found, leave NULL (still visible via group query)
		}

		regime := payload.RegimeTributario
		if regime == "" {
			regime = "lucro_real"
		}

		var c Company
		err = db.QueryRow(`
			INSERT INTO companies
				(group_id, name, trade_name, owner_id, regime_tributario,
				 cnpj, inscricao_estadual, cnae_principal, cnae_secundario,
				 municipio, segmento_economico, incentivos_fiscais)
			VALUES
				($1, $2, $3, $4, $5,
				 NULLIF($6,''), NULLIF($7,''), NULLIF($8,''), $9,
				 NULLIF($10,''), NULLIF($11,''), $12)
			RETURNING id, created_at`,
			payload.GroupID, payload.Name, payload.TradeName, ownerID, regime,
			payload.CNPJ, payload.InscricaoEstadual, payload.CNAEPrincipal,
			pq.Array(payload.CNAESecundario),
			payload.Municipio, payload.SegmentoEconomico, payload.IncentivosFiscais,
		).Scan(&c.ID, &c.CreatedAt)

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		c.GroupID = payload.GroupID
		c.Name = payload.Name
		c.TradeName = payload.TradeName
		c.RegimeTributario = regime
		c.CNPJ = payload.CNPJ
		c.InscricaoEstadual = payload.InscricaoEstadual
		c.CNAEPrincipal = payload.CNAEPrincipal
		c.CNAESecundario = payload.CNAESecundario
		c.Municipio = payload.Municipio
		c.SegmentoEconomico = payload.SegmentoEconomico
		c.IncentivosFiscais = payload.IncentivosFiscais

		json.NewEncoder(w).Encode(c)
	}
}

func UpdateCompanyHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut && r.Method != http.MethodPatch {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "Missing id parameter", http.StatusBadRequest)
			return
		}

		var payload struct {
			RegimeTributario  string           `json:"regime_tributario"`
			CNPJ              string           `json:"cnpj"`
			InscricaoEstadual string           `json:"inscricao_estadual"`
			CNAEPrincipal     string           `json:"cnae_principal"`
			CNAESecundario    []string         `json:"cnae_secundario"`
			Municipio         string           `json:"municipio"`
			SegmentoEconomico string           `json:"segmento_economico"`
			IncentivosFiscais *json.RawMessage `json:"incentivos_fiscais"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		allowed := map[string]bool{
			"lucro_real": true, "lucro_presumido": true,
			"simples_nacional": true, "nao_informado": true,
		}
		if !allowed[payload.RegimeTributario] {
			http.Error(w, "regime_tributario inválido", http.StatusBadRequest)
			return
		}

		// Validação CNPJ: 14 dígitos numéricos quando fornecido
		if payload.CNPJ != "" {
			re := regexp.MustCompile(`^\d{14}$`)
			if !re.MatchString(payload.CNPJ) {
				http.Error(w, "CNPJ deve ter 14 dígitos numéricos", http.StatusBadRequest)
				return
			}
		}

		_, err := db.Exec(`
			UPDATE companies SET
				regime_tributario  = $1,
				cnpj               = NULLIF($2, ''),
				inscricao_estadual = NULLIF($3, ''),
				cnae_principal     = NULLIF($4, ''),
				cnae_secundario    = $5,
				municipio          = NULLIF($6, ''),
				segmento_economico = NULLIF($7, ''),
				incentivos_fiscais = $8,
				updated_at         = NOW()
			WHERE id = $9`,
			payload.RegimeTributario,
			payload.CNPJ,
			payload.InscricaoEstadual,
			payload.CNAEPrincipal,
			pq.Array(payload.CNAESecundario),
			payload.Municipio,
			payload.SegmentoEconomico,
			payload.IncentivosFiscais,
			id,
		)
		if err != nil {
			log.Printf("UpdateCompany error: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

func DeleteCompanyHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "Missing id parameter", http.StatusBadRequest)
			return
		}

		_, err := db.Exec("DELETE FROM companies WHERE id = $1", id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}
