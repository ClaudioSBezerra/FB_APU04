// fiscal_execute_lote.go — execução em LOTE server-side do pacote fiscal.
//
// Motivação (2026-07-08): o lote rodava no NAVEGADOR (loop de POSTs) — logout,
// refresh ou fechar a aba matava a execução no meio, e ao voltar o usuário
// ficava às cegas. Agora o lote é um job em goroutine no servidor (mesmo
// padrão do import de XML): sobrevive à sessão do navegador, e o status é
// consultável por empresa a qualquer momento (inclusive após re-login).
//
//	POST /api/fiscal/execute-lote          {nfe_ids: [...], incluir_ibs_cbs_base}
//	GET  /api/fiscal/execute-lote/status   → job da empresa (ativo ou último)
//
// Um job por empresa por vez (409 se já houver um em andamento). Estado em
// memória: sobrevive à sessão, não a restart do processo (deploy no meio =
// job "perdido", mas as notas já executadas ficam persistidas — reexecutar o
// lote é seguro, upsert por item).
package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Concorrência de NOTAS dentro do job — cada nota roda até 5 itens em
// paralelo, e o pool Oracle da empresa tem 15 conexões: 5 notas simultâneas
// mantêm o Oracle ocupado sem enfileirar demais.
const loteNotasConcorrentes = 5

type fiscalLoteJob struct {
	mu            sync.Mutex
	Total         int
	Processed     int
	NotasOK       int
	NotasParciais int // itens com sem_grupo_fiscal/erro dentro da nota
	NotasErro     int // nota inteira falhou (Oracle fora, etc.)
	IncluirIbsCbs bool
	Done          bool
	StartedAt     time.Time
	FinishedAt    time.Time
}

type fiscalLoteStatus struct {
	Ativo         bool   `json:"ativo"`
	Total         int    `json:"total"`
	Processed     int    `json:"processed"`
	NotasOK       int    `json:"notas_ok"`
	NotasParciais int    `json:"notas_parciais"`
	NotasErro     int    `json:"notas_erro"`
	IncluirIbsCbs bool   `json:"incluir_ibs_cbs"`
	Done          bool   `json:"done"`
	IniciadoEm    string `json:"iniciado_em"`
	TerminadoEm   string `json:"terminado_em,omitempty"`
}

var fiscalLoteJobs sync.Map // companyID → *fiscalLoteJob

func (j *fiscalLoteJob) snapshot() fiscalLoteStatus {
	j.mu.Lock()
	defer j.mu.Unlock()
	s := fiscalLoteStatus{
		Ativo: !j.Done, Total: j.Total, Processed: j.Processed,
		NotasOK: j.NotasOK, NotasParciais: j.NotasParciais, NotasErro: j.NotasErro,
		IncluirIbsCbs: j.IncluirIbsCbs, Done: j.Done,
		IniciadoEm: j.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if j.Done {
		s.TerminadoEm = j.FinishedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	return s
}

// FiscalExecuteLoteHandler — POST /api/fiscal/execute-lote
func FiscalExecuteLoteHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPost {
			jsonErr(w, http.StatusMethodNotAllowed, "Método não permitido")
			return
		}

		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		userID, _ := claims["user_id"].(string)

		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa")
			return
		}

		var req struct {
			NfeIDs            []string `json:"nfe_ids"`
			IncluirIbsCbsBase bool     `json:"incluir_ibs_cbs_base"`
		}
		if decErr := json.NewDecoder(r.Body).Decode(&req); decErr != nil || len(req.NfeIDs) == 0 {
			jsonErr(w, http.StatusBadRequest, "nfe_ids é obrigatório")
			return
		}
		if len(req.NfeIDs) > 50000 {
			jsonErr(w, http.StatusBadRequest, "Máximo de 50.000 notas por lote")
			return
		}

		// Um lote por empresa por vez
		if val, found := fiscalLoteJobs.Load(companyID); found {
			job := val.(*fiscalLoteJob)
			if snap := job.snapshot(); snap.Ativo {
				w.WriteHeader(http.StatusConflict)
				json.NewEncoder(w).Encode(snap)
				return
			}
		}

		job := &fiscalLoteJob{
			Total:         len(req.NfeIDs),
			IncluirIbsCbs: req.IncluirIbsCbsBase,
			StartedAt:     time.Now(),
		}
		fiscalLoteJobs.Store(companyID, job)

		go runFiscalLote(db, companyID, req.NfeIDs, req.IncluirIbsCbsBase, job)

		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(job.snapshot())
	}
}

func runFiscalLote(db *sql.DB, companyID string, nfeIDs []string, incluirIbsCbs bool, job *fiscalLoteJob) {
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("[FiscalLote] panic recuperado (company=%s): %v", companyID, rec)
		}
		job.mu.Lock()
		job.Done = true
		job.FinishedAt = time.Now()
		job.mu.Unlock()
		log.Printf("[FiscalLote] concluído (company=%s): %d notas em %.0fs (ok=%d parciais=%d erro=%d)",
			companyID, job.Total, time.Since(job.StartedAt).Seconds(), job.NotasOK, job.NotasParciais, job.NotasErro)
	}()

	sem := make(chan struct{}, loteNotasConcorrentes)
	var wg sync.WaitGroup
	for idx, id := range nfeIDs {
		wg.Add(1)
		sem <- struct{}{}
		go func(nfeID string) {
			defer wg.Done()
			defer func() { <-sem }()
			defer func() {
				if rec := recover(); rec != nil {
					log.Printf("[FiscalLote] panic na nota %s: %v", nfeID, rec)
					job.mu.Lock()
					job.NotasErro++
					job.Processed++
					job.mu.Unlock()
				}
			}()

			summary, execErr := executarNotaPacote(db, companyID, nfeID, incluirIbsCbs)
			job.mu.Lock()
			job.Processed++
			switch {
			case execErr != nil:
				job.NotasErro++
			case summary.Error > 0 || summary.SemGrupoFiscal > 0:
				job.NotasParciais++
			default:
				job.NotasOK++
			}
			if job.Processed%500 == 0 {
				log.Printf("[FiscalLote] progresso (company=%s): %d/%d", companyID, job.Processed, job.Total)
			}
			job.mu.Unlock()
		}(id)
		_ = idx
	}
	wg.Wait()
}

// FiscalExecuteLoteStatusHandler — GET /api/fiscal/execute-lote/status
// Devolve o job da empresa (ativo ou o último concluído); 404 se nunca houve.
func FiscalExecuteLoteStatusHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodGet {
			jsonErr(w, http.StatusMethodNotAllowed, "Método não permitido")
			return
		}

		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		userID, _ := claims["user_id"].(string)

		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa")
			return
		}

		val, found := fiscalLoteJobs.Load(companyID)
		if !found {
			jsonErr(w, http.StatusNotFound, "Nenhum lote para esta empresa")
			return
		}
		json.NewEncoder(w).Encode(val.(*fiscalLoteJob).snapshot())
	}
}
