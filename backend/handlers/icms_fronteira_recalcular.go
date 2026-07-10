// icms_fronteira_recalcular.go — POST /api/icms-fronteira/recalcular
//
// Refresca a materialized view mv_icms_fronteira_linhas (base dos Blocos A/B do
// Resumo). Necessário porque o import de SPED NÃO dispara refresh e o botão
// "Recalcular" antes só limpava o cache do navegador — a MV ficava congelada
// (diagnóstico 2026-07-10: 05/2026 com 30 jobs importados mas só 47 linhas).
//
// Detalhes aprendidos em produção (AWS, container cliente-db):
//   - CONCURRENTLY NÃO funciona aqui ("cannot refresh concurrently / create a
//     unique index") — então usamos REFRESH normal (lock exclusivo ~2min).
//   - O container tem /dev/shm de 64 MB (default do Docker); com workers
//     paralelos o refresh estoura "No space left on device" ao alocar o
//     segmento de memória compartilhada. Por isso desligamos o paralelismo
//     (max_parallel_workers_per_gather / max_parallel_maintenance_workers = 0)
//     NA MESMA sessão do refresh (por isso db.Conn, não db.Exec avulso).
//   - O refresh leva ~1min48 → maior que o timeout do proxy/browser. Então
//     rodamos em GOROUTINE e devolvemos 202 na hora; a tela avisa p/ recarregar.
package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Um refresh por vez no processo — evita concorrência de REFRESH (que
// serializaria no lock exclusivo de qualquer forma) e dá 409 amigável.
var (
	fronteiraRefreshMu  sync.Mutex
	fronteiraRefreshing bool
)

func IcmsFronteiraRecalcularHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPost {
			jsonErr(w, http.StatusMethodNotAllowed, "Método não permitido")
			return
		}
		if _, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims); !ok {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		fronteiraRefreshMu.Lock()
		if fronteiraRefreshing {
			fronteiraRefreshMu.Unlock()
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": false, "status": "em_andamento",
				"mensagem": "Já existe um recálculo em andamento — aguarde e recarregue a tela.",
			})
			return
		}
		fronteiraRefreshing = true
		fronteiraRefreshMu.Unlock()

		// Roda em background: o refresh (~2min) excede o timeout do proxy/browser.
		go func() {
			defer func() {
				fronteiraRefreshMu.Lock()
				fronteiraRefreshing = false
				fronteiraRefreshMu.Unlock()
				if rec := recover(); rec != nil {
					log.Printf("[FronteiraRecalcular] panic: %v", rec)
				}
			}()

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
			defer cancel()

			// Mesma SESSÃO para os SET + REFRESH (SET de sessão não vale entre
			// conexões diferentes do pool).
			conn, err := db.Conn(ctx)
			if err != nil {
				log.Printf("[FronteiraRecalcular] db.Conn: %v", err)
				return
			}
			defer conn.Close()

			for _, s := range []string{
				"SET max_parallel_workers_per_gather = 0",
				"SET max_parallel_maintenance_workers = 0",
			} {
				if _, err := conn.ExecContext(ctx, s); err != nil {
					log.Printf("[FronteiraRecalcular] %q: %v", s, err)
				}
			}

			inicio := time.Now()
			if _, err := conn.ExecContext(ctx, "REFRESH MATERIALIZED VIEW mv_icms_fronteira_linhas"); err != nil {
				log.Printf("[FronteiraRecalcular] refresh falhou: %v", err)
				return
			}
			log.Printf("[FronteiraRecalcular] MV refrescada em %.0fs", time.Since(inicio).Seconds())
		}()

		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": true, "status": "iniciado",
			"mensagem": "Recálculo iniciado. Leva ~1-2 minutos; recarregue a tela em seguida.",
		})
	}
}
