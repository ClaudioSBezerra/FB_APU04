// icms_fronteira_recalcular.go — POST /api/icms-fronteira/recalcular
//
// Refresca a materialized view mv_icms_fronteira_linhas (base dos Blocos A/B do
// Resumo). Necessário porque o import de SPED NÃO dispara refresh e o botão
// "Recalcular" antes só limpava o cache do navegador — a MV ficava congelada,
// então o Resumo não refletia os SPEDs recém-importados (diagnóstico 2026-07-10:
// 05/2026 com 30 jobs importados mas só 47 linhas na MV).
//
// CONCURRENTLY (a MV tem índice único idx_mv_fronteira_linhas_key) para não
// travar leituras durante o refresh; fallback para refresh normal se o
// CONCURRENTLY falhar (ex.: MV nunca populada).
package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
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

		// Refresh pode ser pesado (minutos em bases grandes) — timeout generoso e
		// context próprio para não morrer se o cliente desconectar.
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()

		inicio := time.Now()
		_, err := db.ExecContext(ctx, "REFRESH MATERIALIZED VIEW CONCURRENTLY mv_icms_fronteira_linhas")
		if err != nil {
			log.Printf("[FronteiraRecalcular] CONCURRENTLY falhou, tentando normal: %v", err)
			if _, err2 := db.ExecContext(ctx, "REFRESH MATERIALIZED VIEW mv_icms_fronteira_linhas"); err2 != nil {
				log.Printf("[FronteiraRecalcular] refresh falhou: %v", err2)
				jsonErr(w, http.StatusInternalServerError, "Falha ao recalcular (refresh da base do Fronteira)")
				return
			}
		}
		dur := time.Since(inicio).Seconds()
		log.Printf("[FronteiraRecalcular] MV refrescada em %.1fs", dur)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "segundos": dur})
	}
}
