// pacotefiscal_sim2027_parametros.go — parâmetros GLOBAIS (não por empresa)
// da Simulação 2027: alíquotas de teste de IBS-UF, IBS-Mun e CBS. Segue o
// padrão de reforma_config.go, mas sem company_id (registro singleton — ver
// migration 165_pacotefiscal_sim2027_parametros.sql).
package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
)

type pfsParametrosResponse struct {
	AliqIbsUfPct  float64 `json:"aliq_ibs_uf_pct"`
	AliqIbsMunPct float64 `json:"aliq_ibs_mun_pct"`
	AliqCbsPct    float64 `json:"aliq_cbs_pct"`
}

// pfsCarregarParametros lê os parâmetros globais (singleton id=1), criando o
// registro default na primeira leitura se a migration ainda não tiver rodado
// o INSERT inicial (defensivo — não deveria acontecer em produção).
func pfsCarregarParametros(db *sql.DB) (pfsParametros, error) {
	var p pfsParametros
	err := db.QueryRow(`
		SELECT aliq_ibs_uf_pct, aliq_ibs_mun_pct, aliq_cbs_pct
		FROM pacotefiscal_sim2027_parametros WHERE id = 1
	`).Scan(&p.AliqIbsUfPct, &p.AliqIbsMunPct, &p.AliqCbsPct)
	if err == sql.ErrNoRows {
		p = pfsParametros{AliqIbsUfPct: 0.05, AliqIbsMunPct: 0.05, AliqCbsPct: 9.90}
		_, insErr := db.Exec(`
			INSERT INTO pacotefiscal_sim2027_parametros (id, aliq_ibs_uf_pct, aliq_ibs_mun_pct, aliq_cbs_pct)
			VALUES (1, $1, $2, $3) ON CONFLICT (id) DO NOTHING`,
			p.AliqIbsUfPct, p.AliqIbsMunPct, p.AliqCbsPct)
		if insErr != nil {
			return p, insErr
		}
		return p, nil
	}
	return p, err
}

// GetPacoteFiscalSim2027ParametrosHandler — GET, qualquer usuário autenticado lê.
func GetPacoteFiscalSim2027ParametrosHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodGet {
			jsonErr(w, http.StatusMethodNotAllowed, "Método não permitido")
			return
		}
		if _, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims); !ok {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		p, err := pfsCarregarParametros(db)
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao carregar parâmetros: "+err.Error())
			return
		}
		json.NewEncoder(w).Encode(pfsParametrosResponse{
			AliqIbsUfPct: p.AliqIbsUfPct, AliqIbsMunPct: p.AliqIbsMunPct, AliqCbsPct: p.AliqCbsPct,
		})
	}
}

// PutPacoteFiscalSim2027ParametrosHandler — PUT, admin-only (mesma régua da
// importação de XML deste módulo).
func PutPacoteFiscalSim2027ParametrosHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPut && r.Method != http.MethodPost {
			jsonErr(w, http.StatusMethodNotAllowed, "Método não permitido")
			return
		}
		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		userID, _ := claims["user_id"].(string)

		var req pfsParametrosResponse
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonErr(w, http.StatusBadRequest, "Corpo da requisição inválido")
			return
		}

		for _, v := range []float64{req.AliqIbsUfPct, req.AliqIbsMunPct, req.AliqCbsPct} {
			if v < 0 || v > 100 {
				jsonErr(w, http.StatusBadRequest, "Alíquotas devem estar entre 0 e 100")
				return
			}
		}

		_, err := db.Exec(`
			INSERT INTO pacotefiscal_sim2027_parametros (id, aliq_ibs_uf_pct, aliq_ibs_mun_pct, aliq_cbs_pct, updated_at, updated_by)
			VALUES (1, $1, $2, $3, now(), $4)
			ON CONFLICT (id) DO UPDATE SET
				aliq_ibs_uf_pct = $1, aliq_ibs_mun_pct = $2, aliq_cbs_pct = $3,
				updated_at = now(), updated_by = $4
		`, req.AliqIbsUfPct, req.AliqIbsMunPct, req.AliqCbsPct, nullIfEmpty(userID))
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao salvar parâmetros: "+err.Error())
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"message": "Parâmetros salvos com sucesso"})
	}
}
