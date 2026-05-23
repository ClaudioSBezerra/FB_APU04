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

type FronteiraRegraRow struct {
	ID             string   `json:"id"`
	NCMPrefixo     string   `json:"ncm_prefixo"`
	Descricao      string   `json:"descricao"`
	Regime         string   `json:"regime"`
	AliquotaInterna float64 `json:"aliquota_interna"`
	MVAOriginal    *float64 `json:"mva_original"`
	ReducaoBCPct   float64  `json:"reducao_bc_pct"`
	IsGlobal       bool     `json:"is_global"`
}

type FronteiraRegrasResponse struct {
	Rows  []FronteiraRegraRow `json:"rows"`
	Count int                 `json:"count"`
}

// ---------------------------------------------------------------------------
// IcmsFronteiraRegrasListHandler — GET /api/icms-fronteira/regras
// ---------------------------------------------------------------------------

func IcmsFronteiraRegrasListHandler(db *sql.DB) http.HandlerFunc {
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
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa: "+err.Error())
			return
		}

		rows, err := db.Query(`
			SELECT
				id::text,
				ncm_prefixo,
				COALESCE(descricao, ''),
				COALESCE(regime, 'ST'),
				COALESCE(aliquota_interna, 20.5),
				mva_original,
				COALESCE(reducao_bc_pct, 0),
				(company_id IS NULL) AS is_global
			FROM icms_fronteira_regras_ncm
			WHERE company_id = $1 OR company_id IS NULL
			ORDER BY ncm_prefixo
		`, companyID)
		if err != nil {
			log.Printf("IcmsFronteiraRegrasList error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar regras NCM")
			return
		}
		defer rows.Close()

		result := []FronteiraRegraRow{}
		for rows.Next() {
			var row FronteiraRegraRow
			var mva sql.NullFloat64
			if err := rows.Scan(
				&row.ID,
				&row.NCMPrefixo,
				&row.Descricao,
				&row.Regime,
				&row.AliquotaInterna,
				&mva,
				&row.ReducaoBCPct,
				&row.IsGlobal,
			); err != nil {
				log.Printf("IcmsFronteiraRegrasList scan error: %v", err)
				continue
			}
			if mva.Valid {
				row.MVAOriginal = &mva.Float64
			}
			result = append(result, row)
		}

		json.NewEncoder(w).Encode(FronteiraRegrasResponse{
			Rows:  result,
			Count: len(result),
		})
	}
}

// ---------------------------------------------------------------------------
// IcmsFronteiraRegraCreateHandler — POST /api/icms-fronteira/regras
// ---------------------------------------------------------------------------

func IcmsFronteiraRegraCreateHandler(db *sql.DB) http.HandlerFunc {
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
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa: "+err.Error())
			return
		}

		var body struct {
			NCMPrefixo      string   `json:"ncm_prefixo"`
			Descricao       string   `json:"descricao"`
			Regime          string   `json:"regime"`
			AliquotaInterna float64  `json:"aliquota_interna"`
			MVAOriginal     *float64 `json:"mva_original"`
			ReducaoBCPct    float64  `json:"reducao_bc_pct"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonErr(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
			return
		}

		body.NCMPrefixo = strings.TrimSpace(body.NCMPrefixo)
		if body.NCMPrefixo == "" {
			jsonErr(w, http.StatusBadRequest, "ncm_prefixo é obrigatório")
			return
		}
		if len(body.NCMPrefixo) > 8 {
			body.NCMPrefixo = body.NCMPrefixo[:8]
		}

		if body.AliquotaInterna == 0 {
			body.AliquotaInterna = 20.5
		}

		var mvaArg interface{}
		if body.MVAOriginal != nil && *body.MVAOriginal != 0 {
			mvaArg = *body.MVAOriginal
		}

		var row FronteiraRegraRow
		var mva sql.NullFloat64
		err = db.QueryRow(`
			INSERT INTO icms_fronteira_regras_ncm
				(company_id, ncm_prefixo, descricao, regime, aliquota_interna, mva_original, reducao_bc_pct)
			VALUES
				($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (company_id, ncm_prefixo) DO UPDATE
				SET descricao = EXCLUDED.descricao,
				    regime = EXCLUDED.regime,
				    aliquota_interna = EXCLUDED.aliquota_interna,
				    mva_original = EXCLUDED.mva_original,
				    reducao_bc_pct = EXCLUDED.reducao_bc_pct
			RETURNING
				id::text,
				ncm_prefixo,
				COALESCE(descricao, ''),
				COALESCE(regime, 'ST'),
				COALESCE(aliquota_interna, 20.5),
				mva_original,
				COALESCE(reducao_bc_pct, 0),
				(company_id IS NULL)
		`, companyID, body.NCMPrefixo, body.Descricao, body.Regime, body.AliquotaInterna, mvaArg, body.ReducaoBCPct,
		).Scan(
			&row.ID, &row.NCMPrefixo, &row.Descricao, &row.Regime,
			&row.AliquotaInterna, &mva, &row.ReducaoBCPct, &row.IsGlobal,
		)
		if err != nil {
			log.Printf("IcmsFronteiraRegraCreate error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao salvar regra NCM")
			return
		}
		if mva.Valid {
			row.MVAOriginal = &mva.Float64
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(row)
	}
}

// ---------------------------------------------------------------------------
// IcmsFronteiraRegraDeleteHandler — DELETE /api/icms-fronteira/regras/{id}
// ---------------------------------------------------------------------------

func IcmsFronteiraRegraDeleteHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa: "+err.Error())
			return
		}

		id := strings.TrimPrefix(r.URL.Path, "/api/icms-fronteira/regras/")
		id = strings.TrimSpace(id)
		if id == "" {
			jsonErr(w, http.StatusBadRequest, "ID não informado")
			return
		}

		res, err := db.Exec(`
			DELETE FROM icms_fronteira_regras_ncm
			WHERE id = $1::uuid AND company_id = $2::uuid
		`, id, companyID)
		if err != nil {
			log.Printf("IcmsFronteiraRegraDelete error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao excluir regra NCM")
			return
		}

		n, _ := res.RowsAffected()
		if n == 0 {
			jsonErr(w, http.StatusNotFound, "Regra não encontrada ou é global (não pode ser excluída)")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// ---------------------------------------------------------------------------
// IcmsFronteiraRegrasImportarHandler — POST /api/icms-fronteira/regras/importar
// ---------------------------------------------------------------------------

func IcmsFronteiraRegrasImportarHandler(db *sql.DB) http.HandlerFunc {
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
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa: "+err.Error())
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 5<<20) // 5 MB

		if err := r.ParseMultipartForm(5 << 20); err != nil {
			jsonErr(w, http.StatusBadRequest, "Arquivo muito grande ou formulário inválido: "+err.Error())
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			jsonErr(w, http.StatusBadRequest, "Campo 'file' não encontrado: "+err.Error())
			return
		}
		defer file.Close()

		filename := strings.ToLower(header.Filename)

		type importResult struct {
			Imported int      `json:"imported"`
			Skipped  int      `json:"skipped"`
			Errors   []string `json:"errors"`
		}

		var records [][]string

		if strings.HasSuffix(filename, ".xlsx") {
			// --- XLSX parsing ---
			data, err := io.ReadAll(file)
			if err != nil {
				jsonErr(w, http.StatusInternalServerError, "Erro ao ler arquivo XLSX")
				return
			}
			f, err := excelize.OpenReader(bytes.NewReader(data))
			if err != nil {
				jsonErr(w, http.StatusBadRequest, "Arquivo XLSX inválido: "+err.Error())
				return
			}
			sheets := f.GetSheetList()
			if len(sheets) == 0 {
				jsonErr(w, http.StatusBadRequest, "Arquivo XLSX sem planilhas")
				return
			}
			rows2, err := f.GetRows(sheets[0])
			if err != nil {
				jsonErr(w, http.StatusBadRequest, "Erro ao ler planilha XLSX: "+err.Error())
				return
			}
			records = rows2
		} else {
			// --- CSV parsing ---
			data, err := io.ReadAll(file)
			if err != nil {
				jsonErr(w, http.StatusInternalServerError, "Erro ao ler arquivo CSV")
				return
			}
			content := string(data)
			// Detect delimiter: try semicolon first
			delim := ','
			if strings.Contains(content, ";") {
				delim = ';'
			}
			cr := csv.NewReader(strings.NewReader(content))
			cr.Comma = rune(delim)
			cr.LazyQuotes = true
			cr.TrimLeadingSpace = true
			records, err = cr.ReadAll()
			if err != nil {
				jsonErr(w, http.StatusBadRequest, "Arquivo CSV inválido: "+err.Error())
				return
			}
		}

		validRegimes := map[string]bool{
			"ST": true, "ANTECIPACAO": true, "DIFAL": true, "ISENTO": true, "NORMAL": true,
		}

		res := importResult{Errors: []string{}}
		// skip header row (index 0)
		for i, rec := range records {
			if i == 0 {
				continue
			}
			if len(rec) < 1 {
				res.Skipped++
				continue
			}

			ncmPrefixo := strings.TrimSpace(rec[0])
			if ncmPrefixo == "" {
				res.Skipped++
				continue
			}
			if len(ncmPrefixo) > 8 {
				ncmPrefixo = ncmPrefixo[:8]
			}

			descricao := ""
			if len(rec) > 1 {
				descricao = strings.TrimSpace(rec[1])
			}

			regime := "ST"
			if len(rec) > 2 {
				r2 := strings.TrimSpace(strings.ToUpper(rec[2]))
				if validRegimes[r2] {
					regime = r2
				} else if r2 != "" {
					regime = "ST"
				}
			}

			aliquotaInterna := 20.5
			if len(rec) > 3 {
				s := strings.TrimSpace(rec[3])
				if s != "" {
					if v, err2 := strconv.ParseFloat(strings.Replace(s, ",", ".", 1), 64); err2 == nil {
						aliquotaInterna = v
					}
				}
			}

			var mvaArg interface{}
			if len(rec) > 4 {
				s := strings.TrimSpace(rec[4])
				if s != "" {
					if v, err2 := strconv.ParseFloat(strings.Replace(s, ",", ".", 1), 64); err2 == nil && v != 0 {
						mvaArg = v
					}
				}
			}

			reducaoBCPct := 0.0
			if len(rec) > 5 {
				s := strings.TrimSpace(rec[5])
				if s != "" {
					if v, err2 := strconv.ParseFloat(strings.Replace(s, ",", ".", 1), 64); err2 == nil {
						reducaoBCPct = v
					}
				}
			}

			_, err2 := db.Exec(`
				INSERT INTO icms_fronteira_regras_ncm
					(company_id, ncm_prefixo, descricao, regime, aliquota_interna, mva_original, reducao_bc_pct)
				VALUES
					($1, $2, $3, $4, $5, $6, $7)
				ON CONFLICT (company_id, ncm_prefixo) DO UPDATE
					SET descricao = EXCLUDED.descricao,
					    regime = EXCLUDED.regime,
					    aliquota_interna = EXCLUDED.aliquota_interna,
					    mva_original = EXCLUDED.mva_original,
					    reducao_bc_pct = EXCLUDED.reducao_bc_pct
			`, companyID, ncmPrefixo, descricao, regime, aliquotaInterna, mvaArg, reducaoBCPct)
			if err2 != nil {
				log.Printf("IcmsFronteiraRegrasImportar upsert error row %d: %v", i+1, err2)
				res.Errors = append(res.Errors, "Linha "+strconv.Itoa(i+1)+": "+err2.Error())
				res.Skipped++
				continue
			}
			res.Imported++
		}

		json.NewEncoder(w).Encode(res)
	}
}

