package handlers

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Fila de jobs de importação de XML via ERP (padrão fila + drain).
//   UI  → ERPXMLImportTriggerHandler (JWT admin)  → cria job pending
//   UI  → ERPXMLImportJobsHandler    (JWT)        → lista jobs (histórico/logs)
//   conector → ERPXMLImportPendingHandler (X-API-Key) → reivindica próximo pendente
//   conector → ERPXMLImportStatusHandler  (X-API-Key) → reporta resultado

type erpXMLJob struct {
	ID            string  `json:"id"`
	CompanyID     string  `json:"company_id"`
	DataIni       string  `json:"data_ini"`
	DataFim       string  `json:"data_fim"`
	Tipos         string  `json:"tipos"`
	Status        string  `json:"status"`
	TotalEnviados int     `json:"total_enviados"`
	TotalErros    int     `json:"total_erros"`
	ErrorMessage  *string `json:"error_message,omitempty"`
	CreatedAt     string  `json:"created_at"`
	StartedAt     *string `json:"started_at,omitempty"`
	FinishedAt    *string `json:"finished_at,omitempty"`
	// Progresso real agregado dos lotes (xml_upload_batches.erp_job_id):
	DocsTotal        int `json:"docs_total"`        // soma de total_count dos lotes
	Importados       int `json:"importados"`        // soma de imported_count
	Rejeitados       int `json:"rejeitados"`        // soma de rejected_count
	BatchesTotal     int `json:"batches_total"`     // nº de lotes
	BatchesAndamento int `json:"batches_andamento"` // lotes pending/processing
}

// erpBridgeCompanyFromAPIKey resolve a empresa a partir do header X-API-Key
// (mesmo esquema de /import/batch e /import/xml). Retorna "" se inválida.
func erpBridgeCompanyFromAPIKey(db *sql.DB, r *http.Request) (string, bool) {
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
		return "", false
	}
	hash := sha256.Sum256([]byte(apiKey))
	hashHex := hex.EncodeToString(hash[:])
	var companyID string
	err := db.QueryRow(`SELECT company_id FROM erp_bridge_config WHERE api_key_hash = $1`, hashHex).Scan(&companyID)
	if err != nil {
		return "", false
	}
	return companyID, true
}

func validTipos(raw string) string {
	allowed := map[string]bool{"entradas": true, "ctes": true, "saidas": true}
	var out []string
	for _, t := range strings.Split(raw, ",") {
		t = strings.ToLower(strings.TrimSpace(t))
		if allowed[t] {
			out = append(out, t)
		}
	}
	return strings.Join(out, ",")
}

// POST /api/erp-bridge/xml-import/trigger  (JWT admin)
func ERPXMLImportTriggerHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		companyID, err := erpBridgeGetCompany(db, r)
		if err != nil {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		var req struct {
			DataIni string `json:"data_ini"`
			DataFim string `json:"data_fim"`
			Tipos   string `json:"tipos"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonErr(w, http.StatusBadRequest, "JSON inválido")
			return
		}
		di, err1 := time.Parse("2006-01-02", strings.TrimSpace(req.DataIni))
		df, err2 := time.Parse("2006-01-02", strings.TrimSpace(req.DataFim))
		if err1 != nil || err2 != nil {
			jsonErr(w, http.StatusBadRequest, "data_ini e data_fim devem estar em YYYY-MM-DD")
			return
		}
		if df.Before(di) {
			jsonErr(w, http.StatusBadRequest, "data_fim não pode ser anterior a data_ini")
			return
		}
		tipos := validTipos(req.Tipos)
		if tipos == "" {
			tipos = "entradas,ctes"
		}
		var createdBy interface{}
		if claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims); ok {
			if uid, ok := claims["user_id"].(string); ok && uid != "" {
				createdBy = uid
			}
		}

		var job erpXMLJob
		var started, finished, errMsg sql.NullString
		err = db.QueryRow(`
			INSERT INTO erp_xml_import_jobs (company_id, data_ini, data_fim, tipos, created_by)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id, company_id, to_char(data_ini,'YYYY-MM-DD'), to_char(data_fim,'YYYY-MM-DD'),
			          tipos, status, total_enviados, total_erros, error_message,
			          to_char(created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'), started_at, finished_at`,
			companyID, di, df, tipos, createdBy,
		).Scan(&job.ID, &job.CompanyID, &job.DataIni, &job.DataFim, &job.Tipos, &job.Status,
			&job.TotalEnviados, &job.TotalErros, &errMsg, &job.CreatedAt, &started, &finished)
		if err != nil {
			log.Printf("[ERPXMLJobs] erro ao criar job: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao criar job")
			return
		}
		json.NewEncoder(w).Encode(job)
	}
}

// GET /api/erp-bridge/xml-import/jobs  (JWT) — lista os últimos jobs da empresa.
func ERPXMLImportJobsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		companyID, err := erpBridgeGetCompany(db, r)
		if err != nil {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		rows, err := db.Query(`
			SELECT j.id, j.company_id, to_char(j.data_ini,'YYYY-MM-DD'), to_char(j.data_fim,'YYYY-MM-DD'),
			       j.tipos, j.status, j.total_enviados, j.total_erros, j.error_message,
			       to_char(j.created_at  AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
			       to_char(j.started_at  AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
			       to_char(j.finished_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
			       COALESCE(b.docs_total,0), COALESCE(b.importados,0), COALESCE(b.rejeitados,0),
			       COALESCE(b.batches_total,0), COALESCE(b.batches_andamento,0)
			FROM erp_xml_import_jobs j
			LEFT JOIN (
				SELECT erp_job_id,
				       SUM(total_count)    AS docs_total,
				       SUM(imported_count) AS importados,
				       SUM(rejected_count) AS rejeitados,
				       COUNT(*)            AS batches_total,
				       COUNT(*) FILTER (WHERE status IN ('pending','processing')) AS batches_andamento
				FROM xml_upload_batches
				WHERE erp_job_id IS NOT NULL
				GROUP BY erp_job_id
			) b ON b.erp_job_id = j.id
			WHERE j.company_id = $1
			ORDER BY j.created_at DESC
			LIMIT 100`, companyID)
		if err != nil {
			log.Printf("[ERPXMLJobs] erro ao listar: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao listar jobs")
			return
		}
		defer rows.Close()
		jobs := []erpXMLJob{}
		for rows.Next() {
			var j erpXMLJob
			var errMsg, started, finished sql.NullString
			if err := rows.Scan(&j.ID, &j.CompanyID, &j.DataIni, &j.DataFim, &j.Tipos, &j.Status,
				&j.TotalEnviados, &j.TotalErros, &errMsg, &j.CreatedAt, &started, &finished,
				&j.DocsTotal, &j.Importados, &j.Rejeitados, &j.BatchesTotal, &j.BatchesAndamento); err != nil {
				continue
			}
			if errMsg.Valid {
				j.ErrorMessage = &errMsg.String
			}
			if started.Valid {
				j.StartedAt = &started.String
			}
			if finished.Valid {
				j.FinishedAt = &finished.String
			}
			jobs = append(jobs, j)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"jobs": jobs})
	}
}

// GET /api/erp-bridge/xml-import/pending  (X-API-Key) — reivindica o próximo job
// pendente da empresa (marca 'running') e o devolve. {} se não houver.
func ERPXMLImportPendingHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		companyID, ok := erpBridgeCompanyFromAPIKey(db, r)
		if !ok {
			jsonErr(w, http.StatusUnauthorized, "API key inválida")
			return
		}
		var job erpXMLJob
		err := db.QueryRow(`
			UPDATE erp_xml_import_jobs
			SET status='running', started_at=now(), updated_at=now()
			WHERE id = (
				SELECT id FROM erp_xml_import_jobs
				WHERE company_id = $1 AND status = 'pending'
				ORDER BY created_at
				LIMIT 1
				FOR UPDATE SKIP LOCKED
			)
			RETURNING id, company_id, to_char(data_ini,'YYYY-MM-DD'), to_char(data_fim,'YYYY-MM-DD'), tipos, status`,
			companyID,
		).Scan(&job.ID, &job.CompanyID, &job.DataIni, &job.DataFim, &job.Tipos, &job.Status)
		if err == sql.ErrNoRows {
			json.NewEncoder(w).Encode(map[string]interface{}{})
			return
		}
		if err != nil {
			log.Printf("[ERPXMLJobs] erro ao reivindicar pendente: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao buscar pendente")
			return
		}
		json.NewEncoder(w).Encode(job)
	}
}

// POST /api/erp-bridge/xml-import/status  (X-API-Key) — conector reporta resultado.
func ERPXMLImportStatusHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		companyID, ok := erpBridgeCompanyFromAPIKey(db, r)
		if !ok {
			jsonErr(w, http.StatusUnauthorized, "API key inválida")
			return
		}
		var req struct {
			JobID         string `json:"job_id"`
			Status        string `json:"status"`
			TotalEnviados int    `json:"total_enviados"`
			TotalErros    int    `json:"total_erros"`
			ErrorMessage  string `json:"error_message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.JobID == "" {
			jsonErr(w, http.StatusBadRequest, "job_id obrigatório")
			return
		}
		st := strings.TrimSpace(req.Status)
		if st != "done" && st != "error" && st != "running" {
			st = "done"
		}
		var errMsg interface{}
		if strings.TrimSpace(req.ErrorMessage) != "" {
			errMsg = req.ErrorMessage
		}
		res, err := db.Exec(`
			UPDATE erp_xml_import_jobs
			SET status=$1, total_enviados=$2, total_erros=$3, error_message=$4,
			    updated_at=now(),
			    finished_at = CASE WHEN $1 IN ('done','error') THEN now() ELSE finished_at END
			WHERE id=$5 AND company_id=$6`,
			st, req.TotalEnviados, req.TotalErros, errMsg, req.JobID, companyID)
		if err != nil {
			log.Printf("[ERPXMLJobs] erro ao atualizar status: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao atualizar status")
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			jsonErr(w, http.StatusNotFound, "Job não encontrado")
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": st})
	}
}
