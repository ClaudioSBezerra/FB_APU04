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
	ID               string   `json:"id"`
	NCMPrefixo       string   `json:"ncm_prefixo"`
	Descricao        string   `json:"descricao"`
	Regime           string   `json:"regime"`
	AliquotaInterna  float64  `json:"aliquota_interna"`
	MVAOriginal      *float64 `json:"mva_original"`
	MVAAjustado4pct  *float64 `json:"mva_ajustado_4pct"`
	MVAAjustado7pct  *float64 `json:"mva_ajustado_7pct"`
	MVAAjustado12pct *float64 `json:"mva_ajustado_12pct"`
	ReducaoBCPct     float64  `json:"reducao_bc_pct"`
	UFEstado         string   `json:"uf_estado"`
	IsGlobal         bool     `json:"is_global"`
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

		ufEstado := r.URL.Query().Get("uf_estado")
		if ufEstado == "" {
			ufEstado = "PE"
		}
		validUFs := map[string]bool{"PE": true, "BA": true, "CE": true}
		if !validUFs[ufEstado] {
			jsonErr(w, http.StatusBadRequest, "uf_estado inválido: deve ser PE, BA ou CE")
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
				mva_ajustado_4pct,
				mva_ajustado_7pct,
				mva_ajustado_12pct,
				COALESCE(reducao_bc_pct, 0),
				uf_estado,
				(company_id IS NULL) AS is_global
			FROM icms_fronteira_regras_ncm
			WHERE (company_id = $1 OR company_id IS NULL)
			  AND uf_estado = $2
			ORDER BY ncm_prefixo
		`, companyID, ufEstado)
		if err != nil {
			log.Printf("IcmsFronteiraRegrasList error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar regras NCM")
			return
		}
		defer rows.Close()

		result := []FronteiraRegraRow{}
		for rows.Next() {
			var row FronteiraRegraRow
			var mva, mva4, mva7, mva12 sql.NullFloat64
			if err := rows.Scan(
				&row.ID,
				&row.NCMPrefixo,
				&row.Descricao,
				&row.Regime,
				&row.AliquotaInterna,
				&mva,
				&mva4,
				&mva7,
				&mva12,
				&row.ReducaoBCPct,
				&row.UFEstado,
				&row.IsGlobal,
			); err != nil {
				log.Printf("IcmsFronteiraRegrasList scan error: %v", err)
				continue
			}
			if mva.Valid {
				row.MVAOriginal = &mva.Float64
			}
			if mva4.Valid {
				row.MVAAjustado4pct = &mva4.Float64
			}
			if mva7.Valid {
				row.MVAAjustado7pct = &mva7.Float64
			}
			if mva12.Valid {
				row.MVAAjustado12pct = &mva12.Float64
			}
			result = append(result, row)
		}
		if err := rows.Err(); err != nil {
			log.Printf("IcmsFronteiraRegrasList rows error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar regras NCM")
			return
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
			UFEstado        string   `json:"uf_estado"`
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
		if len([]rune(body.NCMPrefixo)) > 8 {
			jsonErr(w, http.StatusBadRequest, "ncm_prefixo não pode ter mais de 8 caracteres")
			return
		}

		if body.AliquotaInterna == 0 {
			body.AliquotaInterna = 20.5
		}

		if body.UFEstado == "" {
			body.UFEstado = "PE"
		}
		validUFs := map[string]bool{"PE": true, "BA": true, "CE": true}
		if !validUFs[body.UFEstado] {
			jsonErr(w, http.StatusBadRequest, "uf_estado inválido: deve ser PE, BA ou CE")
			return
		}

		var mvaArg interface{}
		if body.MVAOriginal != nil && *body.MVAOriginal != 0 {
			mvaArg = *body.MVAOriginal
		}

		var row FronteiraRegraRow
		var mva sql.NullFloat64
		err = db.QueryRow(`
			INSERT INTO icms_fronteira_regras_ncm
				(company_id, ncm_prefixo, descricao, regime, aliquota_interna, mva_original, reducao_bc_pct, uf_estado)
			VALUES
				($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (company_id, ncm_prefixo, uf_estado) DO UPDATE
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
				uf_estado,
				(company_id IS NULL)
		`, companyID, body.NCMPrefixo, body.Descricao, body.Regime, body.AliquotaInterna, mvaArg, body.ReducaoBCPct, body.UFEstado,
		).Scan(
			&row.ID, &row.NCMPrefixo, &row.Descricao, &row.Regime,
			&row.AliquotaInterna, &mva, &row.ReducaoBCPct, &row.UFEstado, &row.IsGlobal,
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
// IcmsFronteiraRegraUpdateHandler — PUT/PATCH /api/icms-fronteira/regras/{id}
// ---------------------------------------------------------------------------

func IcmsFronteiraRegraUpdateHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPut && r.Method != http.MethodPatch {
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

		var body struct {
			Descricao        string   `json:"descricao"`
			Regime           string   `json:"regime"`
			AliquotaInterna  float64  `json:"aliquota_interna"`
			MVAOriginal      *float64 `json:"mva_original"`
			MVAAjustado4pct  *float64 `json:"mva_ajustado_4pct"`
			MVAAjustado7pct  *float64 `json:"mva_ajustado_7pct"`
			MVAAjustado12pct *float64 `json:"mva_ajustado_12pct"`
			ReducaoBCPct     float64  `json:"reducao_bc_pct"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonErr(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
			return
		}

		validRegimes := map[string]bool{
			"ST": true, "ANTECIPACAO": true, "DIFAL": true, "ISENTO": true, "NORMAL": true,
		}
		if body.Regime != "" && !validRegimes[body.Regime] {
			jsonErr(w, http.StatusBadRequest, "regime inválido")
			return
		}

		res, err := db.Exec(`
			UPDATE icms_fronteira_regras_ncm SET
				descricao        = $1,
				regime           = $2,
				aliquota_interna = $3,
				mva_original     = $4,
				mva_ajustado_4pct  = $5,
				mva_ajustado_7pct  = $6,
				mva_ajustado_12pct = $7,
				reducao_bc_pct   = $8
			WHERE id = $9::uuid AND company_id = $10::uuid
		`, body.Descricao, body.Regime, body.AliquotaInterna,
			body.MVAOriginal, body.MVAAjustado4pct, body.MVAAjustado7pct, body.MVAAjustado12pct,
			body.ReducaoBCPct, id, companyID)
		if err != nil {
			log.Printf("IcmsFronteiraRegraUpdate error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao atualizar regra NCM")
			return
		}

		n, _ := res.RowsAffected()
		if n == 0 {
			jsonErr(w, http.StatusNotFound, "Regra não encontrada ou sem permissão")
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{"updated": true, "id": id})
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

		ufEstado := r.FormValue("uf_estado")
		if ufEstado == "" {
			ufEstado = "PE"
		}
		validUFs := map[string]bool{"PE": true, "BA": true, "CE": true}
		if !validUFs[ufEstado] {
			jsonErr(w, http.StatusBadRequest, "uf_estado inválido: deve ser PE, BA ou CE")
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
			if len([]rune(ncmPrefixo)) > 8 {
				if len(res.Errors) < 100 {
					res.Errors = append(res.Errors, "Linha "+strconv.Itoa(i+1)+": ncm_prefixo não pode ter mais de 8 caracteres (valor: "+ncmPrefixo+")")
				}
				res.Skipped++
				continue
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
					(company_id, ncm_prefixo, descricao, regime, aliquota_interna, mva_original, reducao_bc_pct, uf_estado)
				VALUES
					($1, $2, $3, $4, $5, $6, $7, $8)
				ON CONFLICT (company_id, ncm_prefixo, uf_estado) DO UPDATE
					SET descricao = EXCLUDED.descricao,
					    regime = EXCLUDED.regime,
					    aliquota_interna = EXCLUDED.aliquota_interna,
					    mva_original = EXCLUDED.mva_original,
					    reducao_bc_pct = EXCLUDED.reducao_bc_pct
			`, companyID, ncmPrefixo, descricao, regime, aliquotaInterna, mvaArg, reducaoBCPct, ufEstado)
			if err2 != nil {
				log.Printf("IcmsFronteiraRegrasImportar upsert error row %d: %v", i+1, err2)
				if len(res.Errors) < 100 {
					res.Errors = append(res.Errors, "Linha "+strconv.Itoa(i+1)+": "+err2.Error())
				}
				res.Skipped++
				continue
			}
			res.Imported++
		}

		json.NewEncoder(w).Encode(res)
	}
}

