package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// ---------------------------------------------------------------------------
// Classificação manual de notas do bloco "Faltando" (Reconciliação Etapa 2b).
//
// POST /api/icms-fronteira/reconciliacao/classificacao
//   body: { "chave_nfe": "...", "regime": "ANTECIPACAO|ST|DIFAL|NAO_FRONTEIRA",
//           "status": "manual|excluded" (default "manual"),
//           "notes": "...opcional" }
//   Upsert por (company_id, chave_nfe). validated_by = userID do JWT.
//
// DELETE /api/icms-fronteira/reconciliacao/classificacao?chave=NNN...
//   Remove a classificação manual (volta a respeitar a sugestão automática).
// ---------------------------------------------------------------------------

type ClassificacaoManualReq struct {
	ChaveNFe string `json:"chave_nfe"`
	Regime   string `json:"regime"`
	Status   string `json:"status"`
	Notes    string `json:"notes"`
}

var validClassRegimes = map[string]bool{
	"ANTECIPACAO": true, "ST": true, "DIFAL": true, "NAO_FRONTEIRA": true,
}

// IcmsFronteiraClassificacaoHandler — POST/DELETE classificação manual.
func IcmsFronteiraClassificacaoHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPost && r.Method != http.MethodDelete {
			jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		userID, _ := claims["user_id"].(string)
		if userID == "" {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa: "+err.Error())
			return
		}

		if r.Method == http.MethodDelete {
			chave := strings.TrimSpace(r.URL.Query().Get("chave"))
			if chave == "" {
				jsonErr(w, http.StatusBadRequest, "Parâmetro 'chave' obrigatório")
				return
			}
			res, err := db.Exec(
				"DELETE FROM icms_fronteira_classificacao_manual WHERE company_id = $1 AND chave_nfe = $2",
				companyID, chave)
			if err != nil {
				log.Printf("ClassificacaoManual delete error: %v", err)
				jsonErr(w, http.StatusInternalServerError, "Erro ao remover classificação")
				return
			}
			n, _ := res.RowsAffected()
			json.NewEncoder(w).Encode(map[string]interface{}{"removed": n})
			return
		}

		// POST (upsert)
		var req ClassificacaoManualReq
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
			jsonErr(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
			return
		}
		req.ChaveNFe = strings.TrimSpace(req.ChaveNFe)
		req.Regime = strings.ToUpper(strings.TrimSpace(req.Regime))
		req.Status = strings.ToLower(strings.TrimSpace(req.Status))
		if req.Status == "" {
			req.Status = "manual"
		}
		if req.ChaveNFe == "" || len(req.ChaveNFe) > 44 {
			jsonErr(w, http.StatusBadRequest, "chave_nfe obrigatória (44 chars)")
			return
		}
		if !validClassRegimes[req.Regime] {
			jsonErr(w, http.StatusBadRequest, "regime inválido: use ANTECIPACAO|ST|DIFAL|NAO_FRONTEIRA")
			return
		}
		if req.Status != "manual" && req.Status != "excluded" {
			jsonErr(w, http.StatusBadRequest, "status inválido: use manual|excluded")
			return
		}

		var notesArg interface{}
		if strings.TrimSpace(req.Notes) != "" {
			notesArg = req.Notes
		}
		_, err = db.Exec(`
			INSERT INTO icms_fronteira_classificacao_manual
				(company_id, chave_nfe, regime, status, notes, validated_by)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (company_id, chave_nfe) DO UPDATE
				SET regime = EXCLUDED.regime,
				    status = EXCLUDED.status,
				    notes  = EXCLUDED.notes,
				    validated_by = EXCLUDED.validated_by,
				    validated_at = now(),
				    updated_at   = now()
		`, companyID, req.ChaveNFe, req.Regime, req.Status, notesArg, userID)
		if err != nil {
			log.Printf("ClassificacaoManual upsert error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao gravar classificação")
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"chave_nfe": req.ChaveNFe,
			"regime":    req.Regime,
			"status":    req.Status,
		})
	}
}
