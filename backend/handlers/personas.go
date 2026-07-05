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
