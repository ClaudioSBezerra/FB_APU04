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
// Structs
// ---------------------------------------------------------------------------

type SegmentoUF struct {
	Codigo   int    `json:"codigo"`
	UF       string `json:"uf"`
	Descricao string `json:"descricao"`
	Ativo    bool   `json:"ativo"` // true se a empresa tem esse segmento cadastrado
}

// ---------------------------------------------------------------------------
// IcmsFronteiraSegmentosHandler — GET /api/icms-fronteira/segmentos
//
// Lista todos os segmentos disponíveis para uma UF, marcando quais a empresa
// já tem cadastrados. O campo ?uf=PE é obrigatório.
// ---------------------------------------------------------------------------

func IcmsFronteiraSegmentosHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodGet {
			jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
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
			jsonErr(w, http.StatusInternalServerError, err.Error())
			return
		}

		uf := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("uf")))
		if uf == "" {
			jsonErr(w, http.StatusBadRequest, "Parâmetro 'uf' é obrigatório")
			return
		}

		rows, err := db.Query(`
			SELECT
				s.codigo,
				s.uf,
				s.descricao,
				(cs.company_id IS NOT NULL) AS ativo
			FROM segmentos_uf s
			LEFT JOIN company_segmentos cs
				ON cs.company_id = $1::uuid
				AND cs.segmento_codigo = s.codigo
				AND cs.uf = s.uf
			WHERE s.uf = $2
			ORDER BY s.codigo
		`, companyID, uf)
		if err != nil {
			log.Printf("IcmsFronteiraSegmentos error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar segmentos")
			return
		}
		defer rows.Close()

		result := []SegmentoUF{}
		for rows.Next() {
			var seg SegmentoUF
			if err := rows.Scan(&seg.Codigo, &seg.UF, &seg.Descricao, &seg.Ativo); err != nil {
				continue
			}
			result = append(result, seg)
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"segmentos": result,
			"count":     len(result),
		})
	}
}

// ---------------------------------------------------------------------------
// IcmsFronteiraCompanySegmentosHandler — PUT /api/icms-fronteira/company-segmentos
//
// Substitui os segmentos da empresa para uma UF específica.
// Body: { "uf": "PE", "codigos": [8, 11, 14] }
// ---------------------------------------------------------------------------

func IcmsFronteiraCompanySegmentosHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPut {
			jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
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
			jsonErr(w, http.StatusInternalServerError, err.Error())
			return
		}

		var body struct {
			UF     string `json:"uf"`
			Codigos []int  `json:"codigos"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonErr(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
			return
		}

		body.UF = strings.ToUpper(strings.TrimSpace(body.UF))
		if body.UF == "" {
			jsonErr(w, http.StatusBadRequest, "Campo 'uf' é obrigatório")
			return
		}
		if body.Codigos == nil {
			body.Codigos = []int{}
		}

		tx, err := db.Begin()
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao iniciar transação")
			return
		}
		defer tx.Rollback() //nolint:errcheck

		// Remove segmentos existentes para essa empresa/UF
		if _, err := tx.Exec(`
			DELETE FROM company_segmentos
			WHERE company_id = $1::uuid AND uf = $2
		`, companyID, body.UF); err != nil {
			log.Printf("IcmsFronteiraCompanySegmentos delete error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao atualizar segmentos")
			return
		}

		// Insere os novos segmentos
		for _, codigo := range body.Codigos {
			if _, err := tx.Exec(`
				INSERT INTO company_segmentos (company_id, segmento_codigo, uf)
				VALUES ($1::uuid, $2, $3)
				ON CONFLICT DO NOTHING
			`, companyID, codigo, body.UF); err != nil {
				log.Printf("IcmsFronteiraCompanySegmentos insert error (codigo=%d): %v", codigo, err)
				jsonErr(w, http.StatusInternalServerError, "Erro ao inserir segmento")
				return
			}
		}

		if err := tx.Commit(); err != nil {
			log.Printf("IcmsFronteiraCompanySegmentos commit error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao confirmar alterações")
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"saved": len(body.Codigos),
			"uf":    body.UF,
		})
	}
}
