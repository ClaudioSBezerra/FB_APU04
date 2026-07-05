package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/lib/pq"
)

// Personas: pacotes nomeados de módulos (migration 149). Usuários não-admin
// só acessam os módulos da união de suas personas; admin ignora tudo isso.

// Persona representa um perfil funcional (Contador, Controller, ...) e os
// módulos do frontend que ele libera.
type Persona struct {
	ID      string   `json:"id"`
	Label   string   `json:"label"`
	Modules []string `json:"modules"`
}

// GetUserModules retorna a união (ordenada, sem duplicatas) dos módulos das
// personas do usuário. Para admin retorna nil — significa "sem restrição".
func GetUserModules(db *sql.DB, userID, role string) ([]string, error) {
	if role == "admin" {
		return nil, nil
	}
	rows, err := db.Query(`
		SELECT DISTINCT m
		FROM user_personas up
		JOIN personas p ON p.id = up.persona_id
		CROSS JOIN LATERAL unnest(p.modules) AS m
		WHERE up.user_id = $1
		ORDER BY m
	`, userID)
	if err != nil {
		return []string{}, err
	}
	defer rows.Close()

	modules := []string{}
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err != nil {
			return modules, err
		}
		modules = append(modules, m)
	}
	return modules, rows.Err()
}

// GrantAllPersonas vincula todas as personas ao usuário (padrão para usuários
// recém-criados — o admin remove depois o que não se aplica; mesmo espírito do
// backfill da migration 149).
func GrantAllPersonas(db *sql.DB, userID string) error {
	_, err := db.Exec(`
		INSERT INTO user_personas (user_id, persona_id)
		SELECT $1, id FROM personas
		ON CONFLICT DO NOTHING
	`, userID)
	return err
}

// SetUserPersonas substitui as personas do usuário pelo conjunto informado.
func SetUserPersonas(db *sql.DB, userID string, personaIDs []string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM user_personas WHERE user_id = $1", userID); err != nil {
		return err
	}
	if len(personaIDs) > 0 {
		_, err = tx.Exec(`
			INSERT INTO user_personas (user_id, persona_id)
			SELECT $1, unnest($2::text[])
			ON CONFLICT DO NOTHING
		`, userID, pq.Array(personaIDs))
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListPersonasHandler retorna o catálogo de personas (para os checkboxes da
// tela de usuários). Admin only — registrado com requiredRole "admin".
func ListPersonasHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query("SELECT id, label, modules FROM personas ORDER BY label")
		if err != nil {
			log.Printf("ListPersonas error: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		personas := []Persona{}
		for rows.Next() {
			var p Persona
			if err := rows.Scan(&p.ID, &p.Label, pq.Array(&p.Modules)); err != nil {
				log.Printf("ListPersonas scan error: %v", err)
				continue
			}
			personas = append(personas, p)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(personas)
	}
}

// validPersonaModules são os módulos do frontend que podem ser vinculados a
// personas (mesmos ids de mainItems no AppRail). Config fica de fora: é
// aberto a todos e as abas sensíveis lá dentro já são adminOnly.
var validPersonaModules = map[string]bool{
	"simulador":    true,
	"notas":        true,
	"painel":       true,
	"reforma":      true,
	"fronteira":    true,
	"auditoria":    true,
	"pacotefiscal": true,
}

// UpdatePersonaRequest é o body de POST /api/admin/personas/update?id=X.
type UpdatePersonaRequest struct {
	Label   string   `json:"label"`   // opcional — vazio não altera
	Modules []string `json:"modules"` // lista completa (substitui a atual)
}

// UpdatePersonaHandler altera o label e/ou os módulos de uma persona.
// Admin only — registrado com requiredRole "admin". A mudança vale para os
// usuários no próximo refresh do token (≤30 min), sem novo login.
func UpdatePersonaHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		personaID := r.URL.Query().Get("id")
		if personaID == "" {
			http.Error(w, "Persona ID required", http.StatusBadRequest)
			return
		}

		var req UpdatePersonaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		if req.Modules == nil {
			http.Error(w, "modules é obrigatório (lista completa de módulos da persona)", http.StatusBadRequest)
			return
		}
		for _, m := range req.Modules {
			if !validPersonaModules[m] {
				http.Error(w, "Módulo inválido: "+m, http.StatusBadRequest)
				return
			}
		}

		var result sql.Result
		var err error
		if label := strings.TrimSpace(req.Label); label != "" {
			result, err = db.Exec("UPDATE personas SET label = $1, modules = $2 WHERE id = $3",
				label, pq.Array(req.Modules), personaID)
		} else {
			result, err = db.Exec("UPDATE personas SET modules = $1 WHERE id = $2",
				pq.Array(req.Modules), personaID)
		}
		if err != nil {
			log.Printf("UpdatePersona error: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		if n, _ := result.RowsAffected(); n == 0 {
			http.Error(w, "Persona não encontrada", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "Persona atualizada com sucesso"})
	}
}

// moduleAPIPrefixes mapeia prefixos de API → módulo do frontend. Só entram
// prefixos inequívocos; caminhos fora da lista (auth, config, erp-bridge,
// uploads transversais) não são bloqueados por módulo. Ordem importa:
// prefixos mais específicos primeiro (/api/xml/painel/ antes de /api/xml/).
var moduleAPIPrefixes = []struct {
	prefix string
	module string
}{
	{"/api/icms-fronteira/", "fronteira"},
	{"/api/reforma/", "reforma"},
	{"/api/auditoria-efd", "auditoria"},
	{"/api/fiscal/", "pacotefiscal"},
	{"/api/pacotefiscal/", "pacotefiscal"},
	{"/api/xml/painel/", "painel"},
	{"/api/xml/", "notas"},
	{"/api/nfe-entradas", "notas"},
	{"/api/nfe-saidas", "notas"},
	{"/api/cte-entradas", "notas"},
	{"/api/mercadorias", "simulador"},
	{"/api/dashboard/", "simulador"},
	{"/api/reports/", "simulador"},
}

// ModuleForAPIPath retorna o módulo dono do caminho de API, ou "" quando o
// caminho é transversal (não sujeito a controle por persona).
func ModuleForAPIPath(path string) string {
	for _, m := range moduleAPIPrefixes {
		if strings.HasPrefix(path, m.prefix) {
			return m.module
		}
	}
	return ""
}
