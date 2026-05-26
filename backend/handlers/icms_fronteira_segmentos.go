package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
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
// IcmsFronteiraSegmentosHandler — GET+POST /api/icms-fronteira/segmentos
//
// GET: Lista todos os segmentos disponíveis para uma UF (?uf=PE obrigatório),
//      marcando quais a empresa já tem cadastrados.
// POST: Cria um novo segmento na tabela segmentos_uf (admin somente).
//       Body: { "codigo": 22, "uf": "PE", "descricao": "..." }
// ---------------------------------------------------------------------------

func IcmsFronteiraSegmentosHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		userID, _ := claims["user_id"].(string)
		role, _ := claims["role"].(string)

		// --- POST: criar novo segmento (admin) ---
		if r.Method == http.MethodPost {
			if role != "admin" {
				jsonErr(w, http.StatusForbidden, "Apenas administradores podem cadastrar segmentos")
				return
			}
			var body struct {
				Codigo    int    `json:"codigo"`
				UF        string `json:"uf"`
				Descricao string `json:"descricao"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				jsonErr(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
				return
			}
			body.UF = strings.ToUpper(strings.TrimSpace(body.UF))
			body.Descricao = strings.TrimSpace(body.Descricao)
			if body.Codigo <= 0 || body.UF == "" || body.Descricao == "" {
				jsonErr(w, http.StatusBadRequest, "codigo, uf e descricao são obrigatórios")
				return
			}
			_, err := db.Exec(`
				INSERT INTO segmentos_uf (codigo, uf, descricao)
				VALUES ($1, $2, $3)
				ON CONFLICT (codigo, uf) DO UPDATE SET descricao = EXCLUDED.descricao
			`, body.Codigo, body.UF, body.Descricao)
			if err != nil {
				log.Printf("IcmsFronteiraSegmentos create error: %v", err)
				jsonErr(w, http.StatusInternalServerError, "Erro ao criar segmento")
				return
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(SegmentoUF{Codigo: body.Codigo, UF: body.UF, Descricao: body.Descricao})
			return
		}

		if r.Method != http.MethodGet {
			jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

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

// ---------------------------------------------------------------------------
// IcmsFronteiraSegmentoItemHandler — PUT+DELETE /api/icms-fronteira/segmentos/item
//
// PUT:    atualiza descricao de um segmento. Body: {codigo, uf, descricao}
// DELETE: remove um segmento.              Query: ?codigo=11&uf=PE
// Ambos requerem role=admin.
// ---------------------------------------------------------------------------

func IcmsFronteiraSegmentoItemHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		role, _ := claims["role"].(string)
		if role != "admin" {
			jsonErr(w, http.StatusForbidden, "Apenas administradores podem alterar segmentos")
			return
		}

		switch r.Method {
		case http.MethodPut:
			var body struct {
				Codigo    int    `json:"codigo"`
				UF        string `json:"uf"`
				Descricao string `json:"descricao"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				jsonErr(w, http.StatusBadRequest, "JSON inválido")
				return
			}
			body.UF = strings.ToUpper(strings.TrimSpace(body.UF))
			body.Descricao = strings.TrimSpace(body.Descricao)
			if body.Codigo <= 0 || body.UF == "" || body.Descricao == "" {
				jsonErr(w, http.StatusBadRequest, "codigo, uf e descricao obrigatórios")
				return
			}
			res, err := db.Exec(`
				UPDATE segmentos_uf SET descricao = $1 WHERE codigo = $2 AND uf = $3
			`, body.Descricao, body.Codigo, body.UF)
			if err != nil {
				log.Printf("SegmentoItem PUT error: %v", err)
				jsonErr(w, http.StatusInternalServerError, "Erro ao atualizar segmento")
				return
			}
			if n, _ := res.RowsAffected(); n == 0 {
				jsonErr(w, http.StatusNotFound, "Segmento não encontrado")
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

		case http.MethodDelete:
			codigoStr := r.URL.Query().Get("codigo")
			uf := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("uf")))
			codigo, err := strconv.Atoi(codigoStr)
			if err != nil || codigo <= 0 || uf == "" {
				jsonErr(w, http.StatusBadRequest, "codigo e uf obrigatórios")
				return
			}
			res, err := db.Exec(`DELETE FROM segmentos_uf WHERE codigo = $1 AND uf = $2`, codigo, uf)
			if err != nil {
				log.Printf("SegmentoItem DELETE error: %v", err)
				jsonErr(w, http.StatusInternalServerError, "Erro ao excluir segmento")
				return
			}
			if n, _ := res.RowsAffected(); n == 0 {
				jsonErr(w, http.StatusNotFound, "Segmento não encontrado")
				return
			}
			w.WriteHeader(http.StatusNoContent)

		default:
			jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
		}
	}
}
