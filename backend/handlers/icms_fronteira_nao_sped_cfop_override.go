package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
)

// NaoSpedCfopOverrideHandler — POST / DELETE /api/icms-fronteira/nao-sped/cfop-override
//
// Permite corrigir o CFOP de uma linha do Bloco C (chave_nfe + NCM) sem
// alterar o XML original. O naoSpedQuery aplica o override via LEFT JOIN
// e recalcula regime e ICMS com o CFOP corrigido.
//
// POST  { chave_nfe, ncm, cfop_saida_override } — cria ou atualiza o override
// DELETE ?chave=...&ncm=...                     — remove o override (volta ao automático)
func NaoSpedCfopOverrideHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		userID, _ := claims["user_id"].(string)

		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa: "+err.Error())
			return
		}

		switch r.Method {
		case http.MethodPost:
			var body struct {
				ChaveNFe     string `json:"chave_nfe"`
				NCM          string `json:"ncm"`
				CfopOverride string `json:"cfop_saida_override"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ChaveNFe == "" || body.CfopOverride == "" {
				jsonErr(w, http.StatusBadRequest, "chave_nfe e cfop_saida_override são obrigatórios")
				return
			}
			_, err = db.Exec(`
				INSERT INTO nao_sped_cfop_override (company_id, chave_nfe, ncm, cfop_saida_override)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (company_id, chave_nfe, ncm)
				DO UPDATE SET cfop_saida_override = EXCLUDED.cfop_saida_override, created_at = NOW()
			`, companyID, body.ChaveNFe, body.NCM, body.CfopOverride)
			if err != nil {
				jsonErr(w, http.StatusInternalServerError, "Erro ao salvar override: "+err.Error())
				return
			}
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})

		case http.MethodDelete:
			chave := r.URL.Query().Get("chave")
			ncm := r.URL.Query().Get("ncm")
			if chave == "" {
				jsonErr(w, http.StatusBadRequest, "Parâmetro 'chave' obrigatório")
				return
			}
			_, err = db.Exec(`
				DELETE FROM nao_sped_cfop_override
				WHERE company_id = $1 AND chave_nfe = $2 AND COALESCE(ncm,'') = COALESCE($3,'')
			`, companyID, chave, ncm)
			if err != nil {
				jsonErr(w, http.StatusInternalServerError, "Erro ao remover override: "+err.Error())
				return
			}
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})

		default:
			jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
		}
	}
}
