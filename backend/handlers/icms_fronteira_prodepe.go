package handlers

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/xuri/excelize/v2"
)

// ---------------------------------------------------------------------------
// PRODEPE / regime especial de central de distribuição, POR ESTABELECIMENTO.
//
// Cadastro por CNPJ da filial beneficiada: enquanto houver enquadramento ativo
// com vigência cobrindo a data do documento, o motor de fronteira ZERA a
// antecipação e a ST das aquisições daquele CNPJ (DIFAL fica de fora). Ver
// icms_fronteira.go / icms_fronteira_nao_sped.go (branch PRODEPE).
//
// A lista de NCMs (prodepe_ncms) é apenas documentação/base do crédito presumido
// das saídas — na Leitura A NÃO filtra o cálculo de fronteira.
// ---------------------------------------------------------------------------

type ProdepeEnquadramento struct {
	ID                  string  `json:"id"`
	CNPJ                string  `json:"cnpj"`
	InscricaoEstadual   string  `json:"inscricao_estadual"`
	Programa            string  `json:"programa"`           // PRODEPE | PROIND
	NumAto              string  `json:"num_ato"`
	Enquadramento       string  `json:"enquadramento"`
	CreditoPresumidoPct float64 `json:"credito_presumido_pct"`
	VigenciaInicio      string  `json:"vigencia_inicio"`
	VigenciaFim         string  `json:"vigencia_fim"`
	DispensaAntecipacao bool    `json:"dispensa_antecipacao"`
	Observacoes         string  `json:"observacoes"`
	Ativo               bool    `json:"ativo"`
	NcmCount            int     `json:"ncm_count"`
}

// onlyDigits normaliza CNPJ para os 14 dígitos (mesma regra do motor).
func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// nullDate (em erp_bridge_batch.go) converte "YYYY-MM-DD"/vazio em arg SQL (nil quando vazio).

// ---------------------------------------------------------------------------
// IcmsFronteiraProdepeHandler — GET+POST /api/icms-fronteira/prodepe
//
// GET:  lista os enquadramentos PRODEPE da empresa (com contagem de NCMs).
// POST: cria/atualiza um enquadramento (upsert por company_id+cnpj+num_ato).
// ---------------------------------------------------------------------------
func IcmsFronteiraProdepeHandler(db *sql.DB) http.HandlerFunc {
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
		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, err.Error())
			return
		}

		// --- POST: criar/atualizar enquadramento ---
		if r.Method == http.MethodPost {
			var body ProdepeEnquadramento
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				jsonErr(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
				return
			}
			cnpj := onlyDigits(body.CNPJ)
			if cnpj == "" {
				jsonErr(w, http.StatusBadRequest, "cnpj é obrigatório")
				return
			}
			programa := strings.ToUpper(strings.TrimSpace(body.Programa))
			if programa == "" {
				programa = "PRODEPE"
			}
			if programa != "PRODEPE" && programa != "PROIND" {
				jsonErr(w, http.StatusBadRequest, "programa deve ser PRODEPE ou PROIND")
				return
			}
			var id string
			err := db.QueryRow(`
				INSERT INTO prodepe_enquadramentos
					(company_id, cnpj, inscricao_estadual, programa, num_ato, enquadramento,
					 credito_presumido_pct, vigencia_inicio, vigencia_fim,
					 dispensa_antecipacao, observacoes, ativo)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
				ON CONFLICT (company_id, cnpj, num_ato) DO UPDATE SET
					inscricao_estadual    = EXCLUDED.inscricao_estadual,
					programa              = EXCLUDED.programa,
					enquadramento         = EXCLUDED.enquadramento,
					credito_presumido_pct = EXCLUDED.credito_presumido_pct,
					vigencia_inicio       = EXCLUDED.vigencia_inicio,
					vigencia_fim          = EXCLUDED.vigencia_fim,
					dispensa_antecipacao  = EXCLUDED.dispensa_antecipacao,
					observacoes           = EXCLUDED.observacoes,
					ativo                 = EXCLUDED.ativo,
					updated_at            = now()
				RETURNING id
			`, companyID, cnpj, strings.TrimSpace(body.InscricaoEstadual),
				programa, strings.TrimSpace(body.NumAto), strings.TrimSpace(body.Enquadramento),
				body.CreditoPresumidoPct, nullDate(body.VigenciaInicio), nullDate(body.VigenciaFim),
				body.DispensaAntecipacao, strings.TrimSpace(body.Observacoes), body.Ativo,
			).Scan(&id)
			if err != nil {
				log.Printf("ProdepeEnquadramento upsert error: %v", err)
				jsonErr(w, http.StatusInternalServerError, "Erro ao salvar enquadramento")
				return
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"id": id, "cnpj": cnpj})
			return
		}

		// --- GET: listar enquadramentos ---
		rows, err := db.Query(`
			SELECT e.id, e.cnpj, COALESCE(e.inscricao_estadual,''),
			       COALESCE(e.programa, 'PRODEPE'),
			       COALESCE(e.num_ato,''), COALESCE(e.enquadramento,''),
			       COALESCE(e.credito_presumido_pct, 0),
			       COALESCE(e.vigencia_inicio::text, ''),
			       COALESCE(e.vigencia_fim::text, ''),
			       e.dispensa_antecipacao, COALESCE(e.observacoes,''), e.ativo,
			       (SELECT COUNT(*) FROM prodepe_ncms n WHERE n.enquadramento_id = e.id)
			FROM prodepe_enquadramentos e
			WHERE e.company_id = $1::uuid
			ORDER BY e.programa, e.cnpj, e.num_ato
		`, companyID)
		if err != nil {
			log.Printf("ProdepeEnquadramento list error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao listar enquadramentos")
			return
		}
		defer rows.Close()

		result := []ProdepeEnquadramento{}
		for rows.Next() {
			var e ProdepeEnquadramento
			if err := rows.Scan(
				&e.ID, &e.CNPJ, &e.InscricaoEstadual, &e.Programa,
				&e.NumAto, &e.Enquadramento,
				&e.CreditoPresumidoPct, &e.VigenciaInicio, &e.VigenciaFim,
				&e.DispensaAntecipacao, &e.Observacoes, &e.Ativo, &e.NcmCount,
			); err != nil {
				continue
			}
			result = append(result, e)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"enquadramentos": result,
			"count":          len(result),
		})
	}
}

// ---------------------------------------------------------------------------
// IcmsFronteiraProdepeItemHandler — DELETE /api/icms-fronteira/prodepe/item?id=
//
// Remove um enquadramento da empresa (NCMs saem por FK ON DELETE CASCADE).
// ---------------------------------------------------------------------------
func IcmsFronteiraProdepeItemHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodDelete {
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

		id := strings.TrimSpace(r.URL.Query().Get("id"))
		if id == "" || !isValidUUID(id) {
			jsonErr(w, http.StatusBadRequest, "id (uuid) é obrigatório")
			return
		}
		res, err := db.Exec(`
			DELETE FROM prodepe_enquadramentos WHERE id = $1::uuid AND company_id = $2::uuid
		`, id, companyID)
		if err != nil {
			log.Printf("ProdepeEnquadramento delete error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao excluir enquadramento")
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			jsonErr(w, http.StatusNotFound, "Enquadramento não encontrado")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ---------------------------------------------------------------------------
// IcmsFronteiraProdepeFiliaisHandler — GET /api/icms-fronteira/prodepe/filiais
//
// Lista os CNPJs candidatos (estabelecimentos da empresa) para o cadastro,
// derivados do SPED (import_jobs.cnpj) e do XML (nfe_entradas.dest_cnpj_cpf).
// ---------------------------------------------------------------------------
func IcmsFronteiraProdepeFiliaisHandler(db *sql.DB) http.HandlerFunc {
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

		rows, err := db.Query(`
			SELECT cnpj, MAX(nome) AS nome
			FROM (
				SELECT regexp_replace(cnpj, '[^0-9]', '', 'g') AS cnpj,
				       MAX(COALESCE(company_name, '')) AS nome
				FROM import_jobs
				WHERE company_id = $1::uuid AND cnpj IS NOT NULL AND cnpj <> ''
				GROUP BY 1
				UNION ALL
				SELECT regexp_replace(dest_cnpj_cpf, '[^0-9]', '', 'g') AS cnpj,
				       MAX(COALESCE(dest_nome, '')) AS nome
				FROM nfe_entradas
				WHERE company_id = $1::uuid AND dest_cnpj_cpf IS NOT NULL AND dest_cnpj_cpf <> ''
				GROUP BY 1
			) t
			WHERE cnpj <> ''
			GROUP BY cnpj
			ORDER BY cnpj
		`, companyID)
		if err != nil {
			log.Printf("ProdepeFiliais error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao listar filiais")
			return
		}
		defer rows.Close()

		type filial struct {
			CNPJ string `json:"cnpj"`
			Nome string `json:"nome"`
		}
		result := []filial{}
		for rows.Next() {
			var f filial
			if err := rows.Scan(&f.CNPJ, &f.Nome); err == nil {
				result = append(result, f)
			}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"filiais": result, "count": len(result)})
	}
}

// ---------------------------------------------------------------------------
// IcmsFronteiraProdepeNcmsImportarHandler — POST /api/icms-fronteira/prodepe/ncms/importar
//
// Recebe CSV/XLSX (campo multipart "file") + "enquadramento_id" e faz upsert dos
// NCMs do decreto. Parsing no servidor (robusto a delimitador, BOM, cabeçalho),
// mesmo padrão de IcmsFronteiraSegmentosImportarHandler.
// Colunas: 1ª = NCM (8 dígitos), 2ª = descrição (opcional).
// ---------------------------------------------------------------------------
func IcmsFronteiraProdepeNcmsImportarHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPost {
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

		r.Body = http.MaxBytesReader(w, r.Body, 2<<20) // 2 MB
		if err := r.ParseMultipartForm(2 << 20); err != nil {
			jsonErr(w, http.StatusBadRequest, "Arquivo muito grande ou formulário inválido: "+err.Error())
			return
		}

		enqID := strings.TrimSpace(r.FormValue("enquadramento_id"))
		if enqID == "" || !isValidUUID(enqID) {
			jsonErr(w, http.StatusBadRequest, "Campo 'enquadramento_id' (uuid) é obrigatório")
			return
		}
		// Garante que o enquadramento pertence à empresa do usuário (defesa IDOR).
		var owned bool
		if err := db.QueryRow(`
			SELECT EXISTS (SELECT 1 FROM prodepe_enquadramentos
			               WHERE id = $1::uuid AND company_id = $2::uuid)
		`, enqID, companyID).Scan(&owned); err != nil || !owned {
			jsonErr(w, http.StatusNotFound, "Enquadramento não encontrado")
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
			cr.FieldsPerRecord = -1
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
			ncm := onlyDigits(strings.Trim(rec[0], `"'`))
			desc := ""
			if len(rec) > 1 {
				desc = strings.TrimSpace(strings.Trim(strings.Join(rec[1:], " "), `"'`))
			}
			// Ignora cabeçalho.
			low := strings.ToLower(strings.TrimSpace(rec[0]))
			if low == "ncm" || low == "código" || low == "codigo" || low == "code" {
				continue
			}
			if len(ncm) < 4 || len(ncm) > 8 {
				if len(res.Errors) < 50 {
					res.Errors = append(res.Errors, "Linha "+itoa(i+1)+": NCM inválido ("+rec[0]+")")
				}
				res.Skipped++
				continue
			}
			if _, err := db.Exec(`
				INSERT INTO prodepe_ncms (enquadramento_id, ncm, descricao)
				VALUES ($1::uuid, $2, NULLIF($3,''))
				ON CONFLICT (enquadramento_id, ncm) DO UPDATE SET descricao = EXCLUDED.descricao
			`, enqID, ncm, desc); err != nil {
				log.Printf("ProdepeNcms upsert error linha %d: %v", i+1, err)
				if len(res.Errors) < 50 {
					res.Errors = append(res.Errors, "Linha "+itoa(i+1)+": "+err.Error())
				}
				res.Skipped++
				continue
			}
			res.Imported++
		}

		log.Printf("ProdepeNcmsImportar: enq=%s imported=%d skipped=%d", enqID, res.Imported, res.Skipped)
		json.NewEncoder(w).Encode(res)
	}
}
