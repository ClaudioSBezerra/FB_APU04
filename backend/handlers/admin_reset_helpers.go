package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ConfirmationToken é o literal exigido no body para confirmar reset (STAB-01).
const ConfirmationToken = "DELETE-FB_APU04"

// ResetTables são as tabelas afetadas pelo reset global, na ordem em que serão truncadas
// e na ordem em que serão dumpadas pelo backup (STAB-02).
var ResetTables = []string{
	"import_jobs",
	"filial_apelidos",
	"nfe_entradas",
	"nfe_saidas",
	"cte_entradas",
	"xml_upload_batches",
	"parceiros",
	"erp_bridge_run_items",
	"erp_bridge_runs",
}

// CompanyDeleteOp é uma operação DELETE dentro de um grupo de limpeza per-company.
// Table e WhereExtra são hardcoded — nunca recebem input do usuário.
type CompanyDeleteOp struct {
	Table      string // nome da tabela (hardcoded, seguro para concat)
	WhereExtra string // cláusula WHERE extra hardcoded, ex: "AND source = 'xml_upload'"
	ResultKey  string // chave no map rows_deleted da resposta
	// GlobalDelete: a tabela NÃO tem company_id (catálogo global, ex: segmentos_uf).
	// Quando true, o DELETE/COUNT não filtra por company_id — remove todas as linhas
	// (opcionalmente restritas por WhereExtra). Usado para dar "clean slate".
	GlobalDelete bool
}

// CompanyGroups mapeia cada grupo de limpeza para suas operações DELETE.
// Grupos disponíveis: sped, xml, erp_bridge, config.
var CompanyGroups = map[string][]CompanyDeleteOp{
	"sped": {
		{Table: "import_jobs", ResultKey: "import_jobs"},
	},
	"xml": {
		{Table: "nfe_entradas",       WhereExtra: "AND source = 'xml_upload'", ResultKey: "nfe_entradas[xml]"},
		{Table: "nfe_saidas",         WhereExtra: "AND source = 'xml_upload'", ResultKey: "nfe_saidas[xml]"},
		{Table: "cte_entradas",       WhereExtra: "AND source = 'xml_upload'", ResultKey: "cte_entradas[xml]"},
		{Table: "xml_upload_batches", ResultKey: "xml_upload_batches"},
	},
	"erp_bridge": {
		{Table: "nfe_entradas",    WhereExtra: "AND source = 'oracle_bridge'", ResultKey: "nfe_entradas[erp_bridge]"},
		{Table: "nfe_saidas",      WhereExtra: "AND source = 'oracle_bridge'", ResultKey: "nfe_saidas[erp_bridge]"},
		{Table: "cte_entradas",    WhereExtra: "AND source = 'oracle_bridge'", ResultKey: "cte_entradas[erp_bridge]"},
		{Table: "erp_bridge_runs", ResultKey: "erp_bridge_runs"},
		{Table: "parceiros",       ResultKey: "parceiros"},
	},
	"config": {
		{Table: "filial_apelidos", ResultKey: "filial_apelidos"},
	},
	// Módulo ICMS Fronteira: limpeza "clean slate". Remove tanto os dados da
	// empresa quanto os GLOBAIS compartilhados (regras NCM seed company_id NULL
	// e o catálogo segmentos_uf), pois não há mais seed automático — regras e
	// segmentos passam a ser cadastrados só manualmente ou via CSV.
	//   • regras NCM: apaga as da empresa E as globais (GlobalDelete remove tudo)
	//   • company_segmentos: por empresa
	//   • segmentos_uf: catálogo global, sem company_id (GlobalDelete)
	"fronteira": {
		{Table: "icms_fronteira_regras_ncm",          ResultKey: "icms_fronteira_regras_ncm", GlobalDelete: true},
		{Table: "segmentos_uf",                        ResultKey: "segmentos_uf", GlobalDelete: true},
		{Table: "company_segmentos",                   ResultKey: "company_segmentos"},
		{Table: "icms_fronteira_extrato_sefaz",        ResultKey: "icms_fronteira_extrato_sefaz"},
		{Table: "icms_fronteira_contestacoes",         ResultKey: "icms_fronteira_contestacoes"},
		{Table: "icms_fronteira_classificacao_manual", ResultKey: "icms_fronteira_classificacao_manual"},
		{Table: "legislacao_fronteira",                ResultKey: "legislacao_fronteira"},
		// PRODEPE/regime especial por CNPJ; prodepe_ncms sai por FK ON DELETE CASCADE.
		{Table: "prodepe_enquadramentos",              ResultKey: "prodepe_enquadramentos"},
	},
}

// ValidCompanyGroups é a lista de grupos aceitos no endpoint per-company.
var ValidCompanyGroups = []string{"sped", "xml", "erp_bridge", "config", "fronteira"}

// IsValidCompanyGroup verifica se o grupo está no allowlist.
func IsValidCompanyGroup(g string) bool {
	for _, v := range ValidCompanyGroups {
		if v == g {
			return true
		}
	}
	return false
}

// BackupDir é onde pg_dump grava /backups/reset-<TS>.sql.
// Em prod é volume named (api_backups). Em dev fallback ./backups/.
func BackupDir() string {
	if _, err := os.Stat("/backups"); err == nil {
		return "/backups"
	}
	return "./backups"
}

// ConnectedDBName retorna o nome do DB conectado (current_database()).
func ConnectedDBName(ctx context.Context, db *sql.DB) (string, error) {
	var name string
	err := db.QueryRowContext(ctx, "SELECT current_database()").Scan(&name)
	return name, err
}

// IsDBAllowed verifica se o DB conectado está em ALLOWED_DESTRUCTIVE_DBS (STAB — DB allowlist).
// Retorna (allowed, dbName, configuredAllowlist).
func IsDBAllowed(ctx context.Context, db *sql.DB) (bool, string, []string) {
	dbName, err := ConnectedDBName(ctx, db)
	if err != nil {
		return false, "", nil
	}
	raw := os.Getenv("ALLOWED_DESTRUCTIVE_DBS")
	if raw == "" {
		return false, dbName, nil
	}
	parts := strings.Split(raw, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	for _, p := range parts {
		if p == dbName {
			return true, dbName, parts
		}
	}
	return false, dbName, parts
}

// RowsBefore conta linhas em cada tabela de ResetTables. Retorna mapa para gravar em audit.
func RowsBefore(ctx context.Context, db *sql.DB, tables []string) (map[string]int64, error) {
	result := make(map[string]int64, len(tables))
	for _, t := range tables {
		// table names vêm de constante hardcoded — safe para concat
		var n int64
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+t).Scan(&n); err != nil {
			// Tabela pode não existir (migration ainda não aplicada localmente). Logar e seguir.
			log.Printf("RowsBefore: count failed for %s: %v", t, err)
			continue
		}
		result[t] = n
	}
	return result, nil
}

// RunPgDumpBackup executa pg_dump --data-only para as tabelas dadas. Retorna path do arquivo gerado.
// Em caso de erro retorna ("", err) — caller DEVE recusar truncar.
func RunPgDumpBackup(ctx context.Context, tables []string) (string, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return "", fmt.Errorf("DATABASE_URL not set")
	}
	u, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("DATABASE_URL parse: %w", err)
	}
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "5432"
	}
	user := u.User.Username()
	pass, _ := u.User.Password()
	dbName := strings.TrimPrefix(u.Path, "/")
	if dbName == "" {
		return "", fmt.Errorf("DATABASE_URL has empty dbname")
	}

	dir := BackupDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", dir, err)
	}
	ts := time.Now().UTC().Format("20060102T150405Z")
	outPath := filepath.Join(dir, fmt.Sprintf("reset-%s.sql", ts))

	args := []string{
		"--host=" + host,
		"--port=" + port,
		"--username=" + user,
		"--dbname=" + dbName,
		"--no-owner", "--no-acl", "--data-only",
		"--file=" + outPath,
	}
	for _, t := range tables {
		args = append(args, "--table="+t)
	}

	cmd := exec.CommandContext(ctx, "pg_dump", args...)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+pass)
	// Capture stderr para audit
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// Limpar arquivo parcial se existir
		_ = os.Remove(outPath)
		return "", fmt.Errorf("pg_dump failed: %w (stderr: %s)", err, stderr.String())
	}
	// Sanity: arquivo existe e tem >0 bytes
	fi, err := os.Stat(outPath)
	if err != nil {
		return "", fmt.Errorf("pg_dump produced no file: %w", err)
	}
	if fi.Size() == 0 {
		_ = os.Remove(outPath)
		return "", fmt.Errorf("pg_dump produced empty file")
	}
	return outPath, nil
}

// DestructiveAuditRow é o payload que InsertDestructiveAuditRow grava.
type DestructiveAuditRow struct {
	UserID         string
	UserEmail      string
	Action         string
	Scope          string
	TablesAffected []string
	RowsBefore     map[string]int64
	Status         string // success | rejected_token | rejected_rate | rejected_db | rejected_role | failed_backup | failed_truncate
	ErrorMessage   string
	ClientIP       string
	BackupPath     string
}

// InsertDestructiveAuditRow grava 1 linha em admin_destructive_actions. Nunca falha o request:
// se o INSERT falhar, apenas loga.
func InsertDestructiveAuditRow(db *sql.DB, row DestructiveAuditRow) {
	rowsJSON, _ := json.Marshal(row.RowsBefore)
	var userID interface{} = row.UserID
	if row.UserID == "" || !isValidUUID(row.UserID) {
		userID = nil
	}
	_, err := db.Exec(`
		INSERT INTO admin_destructive_actions
			(user_id, user_email, action, scope, tables_affected, rows_before, status, error_message, client_ip, backup_path)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, userID, row.UserEmail, row.Action, row.Scope, pgStringArray(row.TablesAffected), rowsJSON,
		row.Status, row.ErrorMessage, row.ClientIP, row.BackupPath)
	if err != nil {
		log.Printf("InsertDestructiveAuditRow: insert failed: %v", err)
	}
}

// pgStringArray formata []string como literal Postgres array (`{a,b,c}`).
func pgStringArray(ss []string) string {
	if len(ss) == 0 {
		return "{}"
	}
	quoted := make([]string, len(ss))
	for i, s := range ss {
		// tabelas vêm de constante (sem aspas/vírgula), mas defensivo:
		s = strings.ReplaceAll(s, `\`, `\\`)
		s = strings.ReplaceAll(s, `"`, `\"`)
		quoted[i] = `"` + s + `"`
	}
	return "{" + strings.Join(quoted, ",") + "}"
}

// ResolveUserEmail busca email do usuário por id (best-effort para audit).
func ResolveUserEmail(db *sql.DB, userID string) string {
	if userID == "" || !isValidUUID(userID) {
		return ""
	}
	var email string
	_ = db.QueryRow("SELECT email FROM users WHERE id = $1", userID).Scan(&email)
	return email
}
