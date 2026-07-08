package handlers

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/xuri/excelize/v2"
)

// ---------------------------------------------------------------------------
// Structs
// ---------------------------------------------------------------------------

type SegmentoUF struct {
	Codigo    int    `json:"codigo"`
	UF        string `json:"uf"`
	Descricao string `json:"descricao"`
	Ativo     bool   `json:"ativo"` // true se a empresa tem esse segmento cadastrado
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

		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		userID, _ := claims["user_id"].(string)

		// --- POST: criar novo segmento ---
		if r.Method == http.MethodPost {
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
			UF      string `json:"uf"`
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

		if r.Method != http.MethodPut && r.Method != http.MethodDelete {
			jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		if _, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims); !ok {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
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

// ---------------------------------------------------------------------------
// IcmsFronteiraSegmentosImportarHandler — POST /api/icms-fronteira/segmentos/importar
//
// Recebe um arquivo CSV/XLSX (campo multipart "file") + a UF (campo "uf") e faz
// upsert em massa no catálogo segmentos_uf. Parsing no servidor para ser robusto
// a delimitadores (vírgula, ponto-e-vírgula, TAB), BOM e cabeçalho.
//
// Colunas: 1ª = código (int), 2ª = descrição. Linha de cabeçalho ("codigo",
// "cod", "code") é ignorada automaticamente. Mesmo padrão do import de regras.
// ---------------------------------------------------------------------------
func IcmsFronteiraSegmentosImportarHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPost {
			jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		if _, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims); !ok {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 2<<20) // 2 MB
		if err := r.ParseMultipartForm(2 << 20); err != nil {
			jsonErr(w, http.StatusBadRequest, "Arquivo muito grande ou formulário inválido: "+err.Error())
			return
		}

		uf := strings.ToUpper(strings.TrimSpace(r.FormValue("uf")))
		if uf == "" {
			jsonErr(w, http.StatusBadRequest, "Campo 'uf' é obrigatório")
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			jsonErr(w, http.StatusBadRequest, "Campo 'file' não encontrado: "+err.Error())
			return
		}
		defer file.Close()

		var records [][]string
		if strings.HasSuffix(strings.ToLower(header.Filename), ".xlsx") {
			data, err := io.ReadAll(file)
			if err != nil {
				jsonErr(w, http.StatusInternalServerError, "Erro ao ler XLSX")
				return
			}
			f, err := excelize.OpenReader(bytes.NewReader(data))
			if err != nil {
				jsonErr(w, http.StatusBadRequest, "XLSX inválido: "+err.Error())
				return
			}
			sheets := f.GetSheetList()
			if len(sheets) == 0 {
				jsonErr(w, http.StatusBadRequest, "XLSX sem planilhas")
				return
			}
			records, _ = f.GetRows(sheets[0])
		} else {
			data, err := io.ReadAll(file)
			if err != nil {
				jsonErr(w, http.StatusInternalServerError, "Erro ao ler CSV")
				return
			}
			content := strings.TrimPrefix(string(data), "\ufeff") // remove BOM
			// Detecta delimitador: ponto-e-vírgula, TAB ou vírgula.
			delim := ','
			if strings.Contains(content, ";") {
				delim = ';'
			} else if strings.Contains(content, "\t") {
				delim = '\t'
			}
			cr := csv.NewReader(strings.NewReader(content))
			cr.Comma = delim
			cr.LazyQuotes = true
			cr.TrimLeadingSpace = true
			cr.FieldsPerRecord = -1 // tolera nº de colunas variável
			records, err = cr.ReadAll()
			if err != nil {
				jsonErr(w, http.StatusBadRequest, "CSV inválido: "+err.Error())
				return
			}
		}

		type importResult struct {
			Imported int      `json:"imported"`
			Skipped  int      `json:"skipped"`
			Errors   []string `json:"errors"`
		}
		res := importResult{Errors: []string{}}

		for i, rec := range records {
			if len(rec) == 0 {
				res.Skipped++
				continue
			}
			codeStr := strings.TrimSpace(strings.Trim(rec[0], `"'`))
			desc := ""
			if len(rec) > 1 {
				desc = strings.TrimSpace(strings.Trim(strings.Join(rec[1:], " "), `"'`))
			}
			// Ignora cabeçalho.
			low := strings.ToLower(codeStr)
			if low == "codigo" || low == "código" || low == "cod" || low == "code" {
				continue
			}
			codigo, errc := strconv.Atoi(codeStr)
			if errc != nil || codigo <= 0 {
				if len(res.Errors) < 50 {
					res.Errors = append(res.Errors, "Linha "+strconv.Itoa(i+1)+": código inválido ("+codeStr+")")
				}
				res.Skipped++
				continue
			}
			if desc == "" {
				if len(res.Errors) < 50 {
					res.Errors = append(res.Errors, "Linha "+strconv.Itoa(i+1)+": descrição vazia")
				}
				res.Skipped++
				continue
			}
			if _, err := db.Exec(`
				INSERT INTO segmentos_uf (codigo, uf, descricao)
				VALUES ($1, $2, $3)
				ON CONFLICT (codigo, uf) DO UPDATE SET descricao = EXCLUDED.descricao
			`, codigo, uf, desc); err != nil {
				log.Printf("SegmentosImportar upsert error linha %d: %v", i+1, err)
				if len(res.Errors) < 50 {
					res.Errors = append(res.Errors, "Linha "+strconv.Itoa(i+1)+": "+err.Error())
				}
				res.Skipped++
				continue
			}
			res.Imported++
		}

		log.Printf("SegmentosImportar: uf=%s imported=%d skipped=%d", uf, res.Imported, res.Skipped)
		json.NewEncoder(w).Encode(res)
	}
}
