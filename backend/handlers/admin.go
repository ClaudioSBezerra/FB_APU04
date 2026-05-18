package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var reUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func isValidUUID(s string) bool {
	return reUUID.MatchString(s)
}

// ResetCompanyDataRequest é o body de DELETE /api/company/reset-data.
type ResetCompanyDataRequest struct {
	CompanyID    string   `json:"company_id"`
	Tables       []string `json:"tables"`
	Confirmation string   `json:"confirmation"`
}

// ResetCompanyDataHandler apaga dados de uma empresa nas tabelas selecionadas.
// Usa DELETE WHERE company_id (não TRUNCATE) — tabelas são multi-tenant.
// Requer token de confirmação e grava audit log.
func ResetCompanyDataHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodDelete {
			jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		userID, _ := claims["user_id"].(string)
		role, _ := claims["role"].(string)
		clientIP := GetClientIP(r)
		userEmail := ResolveUserEmail(db, userID)

		var req ResetCompanyDataRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
			jsonErr(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		if req.CompanyID == "" {
			jsonErr(w, http.StatusBadRequest, "company_id obrigatório")
			return
		}

		// Token de confirmação (mesmo token do reset global — STAB-01)
		if req.Confirmation != ConfirmationToken {
			InsertDestructiveAuditRow(db, DestructiveAuditRow{
				UserID: userID, UserEmail: userEmail, Action: "reset_company",
				Scope: req.CompanyID, Status: "rejected_token", ClientIP: clientIP,
				ErrorMessage: "token inválido",
			})
			jsonErr(w, http.StatusBadRequest,
				fmt.Sprintf("Token de confirmação deve ser exatamente %q", ConfirmationToken))
			return
		}

		// Validar tabelas contra allowlist
		if len(req.Tables) == 0 {
			jsonErr(w, http.StatusBadRequest, "Selecione ao menos uma tabela")
			return
		}
		for _, t := range req.Tables {
			if !IsCompanyDeletableTable(t) {
				jsonErr(w, http.StatusBadRequest, fmt.Sprintf("Tabela não permitida: %q", t))
				return
			}
		}

		// Autorização: admin global pode qualquer empresa; outros só a própria
		if role != "admin" {
			var exists bool
			err := db.QueryRow(`
				SELECT EXISTS(
					SELECT 1 FROM companies c
					JOIN enterprise_groups eg ON c.group_id = eg.id
					JOIN user_environments ue ON ue.environment_id = eg.environment_id
					WHERE ue.user_id = $1 AND c.id = $2 AND ue.role = 'admin'
				)`, userID, req.CompanyID).Scan(&exists)
			if err != nil || !exists {
				InsertDestructiveAuditRow(db, DestructiveAuditRow{
					UserID: userID, UserEmail: userEmail, Action: "reset_company",
					Scope: req.CompanyID, Status: "rejected_role", ClientIP: clientIP,
					ErrorMessage: "sem permissão para esta empresa",
				})
				jsonErr(w, http.StatusForbidden, "Sem permissão para limpar dados desta empresa")
				return
			}
		}

		// Snapshot antes para audit
		rowsBefore, _ := RowsBefore(r.Context(), db, req.Tables)

		// Deletar arquivos físicos de import_jobs se a tabela foi selecionada
		for _, t := range req.Tables {
			if t == "import_jobs" {
				rows, err := db.Query(
					"SELECT filename FROM import_jobs WHERE company_id = $1 AND filename != ''",
					req.CompanyID)
				if err == nil {
					for rows.Next() {
						var fname string
						if rows.Scan(&fname) == nil && fname != "" {
							fpath := filepath.Join("uploads", fname)
							if removeErr := os.Remove(fpath); removeErr != nil && !os.IsNotExist(removeErr) {
								log.Printf("ResetCompanyData: warning ao deletar arquivo %s: %v", fpath, removeErr)
							}
						}
					}
					rows.Close()
				}
				break
			}
		}

		// DELETE por tabela — tudo em uma transação
		tx, err := db.Begin()
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao iniciar transação")
			return
		}
		defer tx.Rollback()

		rowsDeleted := make(map[string]int64, len(req.Tables))
		for _, t := range req.Tables {
			// table name vem de allowlist hardcoded — seguro para concat
			res, err := tx.Exec("DELETE FROM "+t+" WHERE company_id = $1", req.CompanyID)
			if err != nil {
				InsertDestructiveAuditRow(db, DestructiveAuditRow{
					UserID: userID, UserEmail: userEmail, Action: "reset_company",
					Scope: req.CompanyID, TablesAffected: req.Tables, RowsBefore: rowsBefore,
					Status: "failed_delete", ClientIP: clientIP,
					ErrorMessage: t + ": " + err.Error(),
				})
				jsonErr(w, http.StatusInternalServerError, "Erro ao deletar "+t)
				return
			}
			n, _ := res.RowsAffected()
			rowsDeleted[t] = n
		}

		if err := tx.Commit(); err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao commitar deleção")
			return
		}

		// Audit: sucesso
		InsertDestructiveAuditRow(db, DestructiveAuditRow{
			UserID: userID, UserEmail: userEmail, Action: "reset_company",
			Scope: req.CompanyID, TablesAffected: req.Tables, RowsBefore: rowsBefore,
			Status: "success", ClientIP: clientIP,
		})

		log.Printf("ResetCompanyData: user=%s company=%s tables=%v rows=%v",
			userID, req.CompanyID, req.Tables, rowsDeleted)

		// Refresh das views em background
		go refreshMVsAfterReset(db)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":      "Dados da empresa removidos com sucesso",
			"rows_deleted": rowsDeleted,
		})
	}
}

// RefreshViewsHandler triggers a refresh of all materialized views
func RefreshViewsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Get User Context
		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		userID := claims["user_id"].(string)

		log.Printf("RefreshViews: User %s requested view refresh", userID)

		// Refresh Mercadorias View
		start := time.Now()

		// Refresh mv_mercadorias_agregada
		_, err := db.Exec("REFRESH MATERIALIZED VIEW CONCURRENTLY mv_mercadorias_agregada")
		if err != nil {
			log.Printf("Concurrent refresh failed for mv_mercadorias_agregada, trying standard: %v", err)
			_, err = db.Exec("REFRESH MATERIALIZED VIEW mv_mercadorias_agregada")
			if err != nil {
				log.Printf("Error refreshing mv_mercadorias_agregada: %v", err)
				http.Error(w, "Failed to refresh mv_mercadorias_agregada: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}

		// Refresh mv_operacoes_simples (Simples Nacional)
		_, err = db.Exec("REFRESH MATERIALIZED VIEW CONCURRENTLY mv_operacoes_simples")
		if err != nil {
			log.Printf("Concurrent refresh failed for mv_operacoes_simples, trying standard: %v", err)
			_, err = db.Exec("REFRESH MATERIALIZED VIEW mv_operacoes_simples")
			if err != nil {
				log.Printf("Error refreshing mv_operacoes_simples: %v", err)
				http.Error(w, "Failed to refresh mv_operacoes_simples: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}

		// Refresh mv_compras_fornecedores (todos os fornecedores)
		_, err = db.Exec("REFRESH MATERIALIZED VIEW CONCURRENTLY mv_compras_fornecedores")
		if err != nil {
			log.Printf("Concurrent refresh failed for mv_compras_fornecedores, trying standard: %v", err)
			_, err = db.Exec("REFRESH MATERIALIZED VIEW mv_compras_fornecedores")
			if err != nil {
				log.Printf("Error refreshing mv_compras_fornecedores: %v", err)
				http.Error(w, "Failed to refresh mv_compras_fornecedores: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}

		duration := time.Since(start)
		log.Printf("RefreshViews: Completed in %v", duration)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":     "Views refreshed successfully",
			"duration_ms": duration.Milliseconds(),
		})
	}
}

// ResetDatabaseHandler — protegido por 5 gates (STAB-01..05).
// Ordem: verificações mais baratas primeiro para falhar rápido e evitar linhas de audit ruidosas.
func ResetDatabaseHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if r.Method != http.MethodDelete {
			jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		// Extrai claims (já validados pelo AuthMiddleware na cadeia withAuth).
		claims, ok := ctx.Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		userID, _ := claims["user_id"].(string)
		role, _ := claims["role"].(string)
		clientIP := GetClientIP(r)
		userEmail := ResolveUserEmail(db, userID)

		// Helper closure: registra audit + responde com erro estruturado.
		auditAndReject := func(status string, httpCode int, msg string) {
			InsertDestructiveAuditRow(db, DestructiveAuditRow{
				UserID:       userID,
				UserEmail:    userEmail,
				Action:       "reset_db",
				Scope:        "global",
				Status:       status,
				ErrorMessage: msg,
				ClientIP:     clientIP,
			})
			jsonErr(w, httpCode, msg)
		}

		// Gate 1 (STAB-04): Apenas role admin global.
		// withAuth("admin") já barra non-admins, mas reforçamos aqui para registrar
		// TENTATIVA bloqueada no audit caso o middleware seja bypassado.
		if role != "admin" {
			auditAndReject("rejected_role", http.StatusForbidden,
				"Forbidden: only global admin can reset the database")
			return
		}

		// Gate 2 (STAB-01): Token de confirmação obrigatório no body JSON.
		var body struct {
			Confirmation string `json:"confirmation"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
			auditAndReject("rejected_token", http.StatusBadRequest,
				"Invalid request body: confirmation token required")
			return
		}
		if body.Confirmation != ConfirmationToken {
			auditAndReject("rejected_token", http.StatusBadRequest,
				fmt.Sprintf("Confirmation token must be exactly %q", ConfirmationToken))
			return
		}

		// Gate 3 (STAB-05): Rate limit 1 reset/hora/usuário.
		if !ResetDBRateLimiter.Allow(userID) {
			InsertDestructiveAuditRow(db, DestructiveAuditRow{
				UserID:       userID,
				UserEmail:    userEmail,
				Action:       "reset_db",
				Scope:        "global",
				Status:       "rejected_rate",
				ClientIP:     clientIP,
				ErrorMessage: "rate limit exceeded (1 reset per hour per user)",
			})
			jsonErr(w, http.StatusTooManyRequests,
				"Rate limit exceeded: only 1 database reset per hour per user")
			return
		}

		// Gate 4: DB allowlist (mitigação direta do incidente 2026-05-07).
		// Recusa executar se o banco conectado não estiver em ALLOWED_DESTRUCTIVE_DBS.
		allowed, dbName, allowlist := IsDBAllowed(ctx, db)
		if !allowed {
			// NÃO contar essa tentativa contra o rate limit — é guard estrutural, não user error.
			ResetDBRateLimiter.Reset(userID)
			auditAndReject("rejected_db", http.StatusServiceUnavailable,
				fmt.Sprintf("Refusing to reset: connected DB %q not in ALLOWED_DESTRUCTIVE_DBS=%v",
					dbName, allowlist))
			return
		}

		// Snapshot de rows antes do reset (para audit log).
		rowsBefore, _ := RowsBefore(ctx, db, ResetTables)

		// Gate 5 (STAB-02): Backup ANTES de qualquer TRUNCATE.
		// Se o backup falhar, recusa truncar.
		backupPath, err := RunPgDumpBackup(ctx, ResetTables)
		if err != nil {
			InsertDestructiveAuditRow(db, DestructiveAuditRow{
				UserID:         userID,
				UserEmail:      userEmail,
				Action:         "reset_db",
				Scope:          "global",
				TablesAffected: ResetTables,
				RowsBefore:     rowsBefore,
				Status:         "failed_backup",
				ErrorMessage:   err.Error(),
				ClientIP:       clientIP,
			})
			jsonErr(w, http.StatusInternalServerError,
				"Backup failed; refusing to truncate. See server logs.")
			return
		}

		// === TODOS os gates passaram. Executar truncate em transação. ===
		log.Printf("ResetDatabase: user=%s db=%s backup=%s", userID, dbName, backupPath)

		tx, err := db.Begin()
		if err != nil {
			InsertDestructiveAuditRow(db, DestructiveAuditRow{
				UserID:         userID,
				UserEmail:      userEmail,
				Action:         "reset_db",
				Scope:          "global",
				TablesAffected: ResetTables,
				RowsBefore:     rowsBefore,
				Status:         "failed_truncate",
				ErrorMessage:   "begin tx: " + err.Error(),
				ClientIP:       clientIP,
				BackupPath:     backupPath,
			})
			jsonErr(w, http.StatusInternalServerError, "Failed to start transaction")
			return
		}
		defer tx.Rollback()

		// Truncate na mesma ordem de ResetTables (compatibilidade com versão anterior).
		truncSQL := []string{
			"TRUNCATE TABLE import_jobs CASCADE",
			"DELETE FROM filial_apelidos",
			"TRUNCATE TABLE nfe_entradas CASCADE",
			"TRUNCATE TABLE nfe_saidas CASCADE",
			"TRUNCATE TABLE cte_entradas CASCADE",
			"TRUNCATE TABLE parceiros CASCADE",
			"TRUNCATE TABLE erp_bridge_run_items CASCADE",
			"DELETE FROM erp_bridge_runs",
		}
		for _, stmt := range truncSQL {
			if _, e := tx.Exec(stmt); e != nil {
				InsertDestructiveAuditRow(db, DestructiveAuditRow{
					UserID:         userID,
					UserEmail:      userEmail,
					Action:         "reset_db",
					Scope:          "global",
					TablesAffected: ResetTables,
					RowsBefore:     rowsBefore,
					Status:         "failed_truncate",
					ErrorMessage:   stmt + ": " + e.Error(),
					ClientIP:       clientIP,
					BackupPath:     backupPath,
				})
				jsonErr(w, http.StatusInternalServerError, "Failed: "+stmt)
				return
			}
		}

		if err := tx.Commit(); err != nil {
			InsertDestructiveAuditRow(db, DestructiveAuditRow{
				UserID:         userID,
				UserEmail:      userEmail,
				Action:         "reset_db",
				Scope:          "global",
				TablesAffected: ResetTables,
				RowsBefore:     rowsBefore,
				Status:         "failed_truncate",
				ErrorMessage:   "commit: " + err.Error(),
				ClientIP:       clientIP,
				BackupPath:     backupPath,
			})
			jsonErr(w, http.StatusInternalServerError, "Failed to commit transaction")
			return
		}

		// Refresh das materialized views em goroutine não-bloqueante.
		go refreshMVsAfterReset(db)

		// OBS-01: incrementar counter de resets bem-sucedidos (apenas no caminho de sucesso)
		// O counter é derivado do audit log — T-05-01-07: admin_destructive_actions é a fonte de verdade.
		DatabaseResetTotal.Inc()

		// Audit: sucesso.
		InsertDestructiveAuditRow(db, DestructiveAuditRow{
			UserID:         userID,
			UserEmail:      userEmail,
			Action:         "reset_db",
			Scope:          "global",
			TablesAffected: ResetTables,
			RowsBefore:     rowsBefore,
			Status:         "success",
			ClientIP:       clientIP,
			BackupPath:     backupPath,
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":     "Database reset successfully",
			"backup_path": backupPath,
			"rows_before": rowsBefore,
		})
	}
}

// refreshMVsAfterReset atualiza todas as materialized views após reset (goroutine).
func refreshMVsAfterReset(db *sql.DB) {
	for _, mv := range []string{"mv_mercadorias_agregada", "mv_operacoes_simples", "mv_compras_fornecedores"} {
		if _, err := db.Exec("REFRESH MATERIALIZED VIEW " + mv); err != nil {
			log.Printf("Admin: error refreshing %s after reset: %v", mv, err)
		} else {
			log.Printf("Admin: %s refreshed (empty)", mv)
		}
	}
}


// CreateUserRequest struct
type CreateUserRequest struct {
	FullName      string `json:"full_name"`
	Email         string `json:"email"`
	Password      string `json:"password"`
	Role          string `json:"role"`
	EnvironmentID string `json:"environment_id"` // Optional: link to existing environment
	GroupID       string `json:"group_id"`        // Optional: link to existing group
	CompanyID     string `json:"company_id"`      // Optional: link to existing company
}

// CreateUserHandler creates a new user directly (Admin only)
func CreateUserHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		if req.Email == "" || req.Password == "" || req.FullName == "" {
			http.Error(w, "Missing required fields", http.StatusBadRequest)
			return
		}

		// Hash Password
		hash, err := HashPassword(req.Password)
		if err != nil {
			http.Error(w, "Error hashing password", http.StatusInternalServerError)
			return
		}

		// Default Role
		if req.Role == "" {
			req.Role = "user"
		}

		// Insert User
		trialEnds := time.Now().Add(time.Hour * 24 * 14) // 14 days
		var userID string
		err = db.QueryRow(`
			INSERT INTO users (email, password_hash, full_name, trial_ends_at, is_verified, role)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id
		`, req.Email, hash, req.FullName, trialEnds, true, req.Role).Scan(&userID)

		if err != nil {
			log.Printf("Error creating user: %v", err)
			http.Error(w, "Error creating user (email might be taken)", http.StatusConflict)
			return
		}

		if req.EnvironmentID != "" {
			// Link to existing hierarchy
			_, err = db.Exec("INSERT INTO user_environments (user_id, environment_id, role) VALUES ($1, $2, 'user')", userID, req.EnvironmentID)
			if err != nil {
				log.Printf("Error linking user to environment: %v", err)
			}

			// If company_id provided, always set owner_id (admin explicitly chose the company)
			if req.CompanyID != "" {
				_, err = db.Exec("UPDATE companies SET owner_id = $1 WHERE id = $2", userID, req.CompanyID)
				if err != nil {
					log.Printf("Error setting company owner: %v", err)
				}
			}
		} else {
			// Auto-provision new hierarchy (original behavior)
			var envID string
			err = db.QueryRow("INSERT INTO environments (name, description) VALUES ($1, 'Ambiente Padrão') RETURNING id", "Ambiente de "+req.FullName).Scan(&envID)
			if err == nil {
				var groupID string
				db.QueryRow("INSERT INTO enterprise_groups (environment_id, name, description) VALUES ($1, 'Grupo Padrão', 'Grupo Inicial') RETURNING id", envID).Scan(&groupID)
				db.Exec("INSERT INTO user_environments (user_id, environment_id, role) VALUES ($1, $2, 'admin')", userID, envID)
				if groupID != "" {
					db.Exec("INSERT INTO companies (group_id, name, trade_name, owner_id) VALUES ($1, $2, $2, $3)", groupID, "Empresa de "+req.FullName, userID)
				}
			}
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"message": "User created successfully", "id": userID})
	}
}

// AdminUser extends User with hierarchy info for admin listing
type AdminUser struct {
	ID              string    `json:"id"`
	Email           string    `json:"email"`
	FullName        string    `json:"full_name"`
	IsVerified      bool      `json:"is_verified"`
	TrialEndsAt     time.Time `json:"trial_ends_at"`
	Role            string    `json:"role"`
	CreatedAt       string    `json:"created_at"`
	EnvironmentID   *string   `json:"environment_id"`
	EnvironmentName *string   `json:"environment_name"`
	GroupID         *string   `json:"group_id"`
	GroupName       *string   `json:"group_name"`
	CompanyID       *string   `json:"company_id"`
	CompanyName     *string   `json:"company_name"`
}

// ListUsersHandler returns all users with hierarchy info (Admin only)
func ListUsersHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(`
			SELECT DISTINCT ON (u.id)
			       u.id, u.email, u.full_name, u.is_verified, u.trial_ends_at, u.role, u.created_at,
			       e.id, e.name,
			       eg.id, eg.name,
			       c.id, c.name
			FROM users u
			LEFT JOIN user_environments ue ON u.id = ue.user_id
			LEFT JOIN environments e ON ue.environment_id = e.id
			LEFT JOIN enterprise_groups eg ON eg.environment_id = e.id
			LEFT JOIN companies c ON c.group_id = eg.id
			ORDER BY u.id, (c.owner_id = u.id) DESC NULLS LAST, c.created_at ASC NULLS LAST
		`)
		if err != nil {
			log.Printf("ListUsers error: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var users []AdminUser
		for rows.Next() {
			var u AdminUser
			if err := rows.Scan(&u.ID, &u.Email, &u.FullName, &u.IsVerified, &u.TrialEndsAt, &u.Role, &u.CreatedAt,
				&u.EnvironmentID, &u.EnvironmentName,
				&u.GroupID, &u.GroupName,
				&u.CompanyID, &u.CompanyName); err != nil {
				log.Printf("ListUsers scan error: %v", err)
				continue
			}
			users = append(users, u)
		}

		if users == nil {
			users = []AdminUser{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(users)
	}
}

// PromoteUserRequest struct
type PromoteUserRequest struct {
	Role       string `json:"role"`        // 'admin' or 'user'
	ExtendDays int    `json:"extend_days"` // Days to add to trial
	IsOfficial bool   `json:"is_official"` // If true, sets trial to 2099
}

// PromoteUserHandler updates user role or trial (Admin only)
func PromoteUserHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("id")
		if userID == "" || !isValidUUID(userID) {
			http.Error(w, "Valid User ID required", http.StatusBadRequest)
			return
		}

		var req PromoteUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// Update logic
		if req.Role != "" {
			_, err := db.Exec("UPDATE users SET role = $1 WHERE id = $2", req.Role, userID)
			if err != nil {
				http.Error(w, "Failed to update role", http.StatusInternalServerError)
				return
			}
		}

		if req.IsOfficial {
			// Set to far future (Official Client)
			newEnd := time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC)
			_, err := db.Exec("UPDATE users SET trial_ends_at = $1 WHERE id = $2", newEnd, userID)
			if err != nil {
				http.Error(w, "Failed to update trial status", http.StatusInternalServerError)
				return
			}
		} else if req.ExtendDays > 0 {
			// Get current trial end
			var currentEnd time.Time
			err := db.QueryRow("SELECT trial_ends_at FROM users WHERE id = $1", userID).Scan(&currentEnd)
			if err != nil {
				http.Error(w, "User not found", http.StatusNotFound)
				return
			}

			// If expired, start from now. If not, add to existing.
			if currentEnd.Before(time.Now()) {
				currentEnd = time.Now()
			}
			newEnd := currentEnd.Add(time.Duration(req.ExtendDays) * 24 * time.Hour)

			_, err = db.Exec("UPDATE users SET trial_ends_at = $1 WHERE id = $2", newEnd, userID)
			if err != nil {
				http.Error(w, "Failed to update trial", http.StatusInternalServerError)
				return
			}
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "User updated successfully"})
	}
}

// ReassignUserRequest struct
type ReassignUserRequest struct {
	UserID        string `json:"user_id"`
	EnvironmentID string `json:"environment_id"`
	GroupID       string `json:"group_id"`  // Optional
	CompanyID     string `json:"company_id"` // Optional
}

// ReassignUserHandler re-links a user to a different hierarchy (Admin only)
func ReassignUserHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ReassignUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		if req.UserID == "" || req.EnvironmentID == "" {
			http.Error(w, "user_id and environment_id are required", http.StatusBadRequest)
			return
		}

		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		// Remove old company ownership (clear owner_id where this user is owner)
		_, err = tx.Exec(`
			UPDATE companies SET owner_id = NULL
			WHERE owner_id = $1
		`, req.UserID)
		if err != nil {
			log.Printf("ReassignUser: Error clearing old company owner: %v", err)
		}

		// Remove old environment link
		_, err = tx.Exec("DELETE FROM user_environments WHERE user_id = $1", req.UserID)
		if err != nil {
			log.Printf("ReassignUser: Error removing old env link: %v", err)
			http.Error(w, "Failed to remove old environment link", http.StatusInternalServerError)
			return
		}

		// Insert new environment link com preferred_company_id se fornecido
		if req.CompanyID != "" {
			_, err = tx.Exec(`
				INSERT INTO user_environments (user_id, environment_id, role, preferred_company_id)
				VALUES ($1, $2, 'user', $3)
			`, req.UserID, req.EnvironmentID, req.CompanyID)
		} else {
			_, err = tx.Exec("INSERT INTO user_environments (user_id, environment_id, role) VALUES ($1, $2, 'user')", req.UserID, req.EnvironmentID)
		}
		if err != nil {
			log.Printf("ReassignUser: Error inserting new env link: %v", err)
			http.Error(w, "Failed to link to new environment", http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit changes", http.StatusInternalServerError)
			return
		}

		log.Printf("ReassignUser: User %s reassigned to environment %s, company %s", req.UserID, req.EnvironmentID, req.CompanyID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "User reassigned successfully"})
	}
}

// DeleteUserHandler deletes a user and all their data (Admin only)
func DeleteUserHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("id")
		if userID == "" || !isValidUUID(userID) {
			http.Error(w, "Valid User ID required", http.StatusBadRequest)
			return
		}

		_, err := db.Exec("DELETE FROM users WHERE id = $1", userID)
		if err != nil {
			http.Error(w, "Failed to delete user", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "User deleted successfully"})
	}
}
