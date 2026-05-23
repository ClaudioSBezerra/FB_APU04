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

type ExtratoSefazRow struct {
	ID            string  `json:"id"`
	Periodo       string  `json:"periodo"`
	RegistroNota  string  `json:"registro_nota"`
	CNPJEmitente  string  `json:"cnpj_emitente"`
	NomeEmitente  string  `json:"nome_emitente"`
	UFEmitente    string  `json:"uf_emitente"`
	NumeroNF      string  `json:"numero_nf"`
	ChaveNFe      string  `json:"chave_nfe"`
	ICMSDevido    float64 `json:"icms_devido"`
}

type ExtratoSefazResponse struct {
	Rows  []ExtratoSefazRow `json:"rows"`
	Total float64           `json:"total"`
	Count int               `json:"count"`
}

// ---------------------------------------------------------------------------
// IcmsFronteiraExtratoImportarHandler — POST /api/icms-fronteira/extrato/importar
// ---------------------------------------------------------------------------

func IcmsFronteiraExtratoImportarHandler(db *sql.DB) http.HandlerFunc {
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
			jsonErr(w, http.StatusBadRequest, "Formulário inválido: "+err.Error())
			return
		}

		periodo := strings.TrimSpace(r.FormValue("periodo"))
		if periodo == "" {
			jsonErr(w, http.StatusBadRequest, "Campo 'periodo' é obrigatório (MM/YYYY)")
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			jsonErr(w, http.StatusBadRequest, "Campo 'file' não encontrado: "+err.Error())
			return
		}
		defer file.Close()

		filename := strings.ToLower(header.Filename)

		var records [][]string

		if strings.HasSuffix(filename, ".xlsx") {
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
				jsonErr(w, http.StatusBadRequest, "Arquivo XLSX sem planilhas")
				return
			}
			rows2, err := f.GetRows(sheets[0])
			if err != nil {
				jsonErr(w, http.StatusBadRequest, "Erro ao ler planilha: "+err.Error())
				return
			}
			records = rows2
		} else {
			// CSV
			data, err := io.ReadAll(file)
			if err != nil {
				jsonErr(w, http.StatusInternalServerError, "Erro ao ler CSV")
				return
			}
			content := string(data)
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
				jsonErr(w, http.StatusBadRequest, "CSV inválido: "+err.Error())
				return
			}
		}

		// Parse records into typed rows (skip header at index 0)
		type parsedRow struct {
			RegistroNota string
			CNPJEmitente string
			NomeEmitente string
			UFEmitente   string
			NumeroNF     string
			ChaveNFe     string
			ICMSDevido   float64
		}

		var parsed []parsedRow
		for i, rec := range records {
			if i == 0 {
				continue // skip header
			}
			if len(rec) == 0 {
				continue
			}

			var pr parsedRow
			if len(rec) >= 7 {
				// Full 7-column format
				pr.RegistroNota = strings.TrimSpace(rec[0])
				pr.CNPJEmitente = strings.TrimSpace(rec[1])
				pr.NomeEmitente = strings.TrimSpace(rec[2])
				pr.UFEmitente = strings.TrimSpace(rec[3])
				pr.NumeroNF = strings.TrimSpace(rec[4])
				pr.ChaveNFe = strings.TrimSpace(rec[5])
				if v, err2 := strconv.ParseFloat(strings.Replace(strings.TrimSpace(rec[6]), ",", ".", 1), 64); err2 == nil {
					pr.ICMSDevido = v
				}
			} else if len(rec) >= 3 {
				// Simplified SEFAZ-PE format: cnpj_emitente, numero_nf, icms_devido
				pr.CNPJEmitente = strings.TrimSpace(rec[0])
				pr.NumeroNF = strings.TrimSpace(rec[1])
				if v, err2 := strconv.ParseFloat(strings.Replace(strings.TrimSpace(rec[2]), ",", ".", 1), 64); err2 == nil {
					pr.ICMSDevido = v
				}
			} else {
				continue
			}

			if pr.CNPJEmitente == "" && pr.NumeroNF == "" {
				continue
			}
			parsed = append(parsed, pr)
		}

		// Replace all existing rows for this company+periodo in a transaction
		tx, err := db.Begin()
		if err != nil {
			log.Printf("IcmsFronteiraExtratoImportar tx begin error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao iniciar transação")
			return
		}
		defer func() {
			if err != nil {
				tx.Rollback()
			}
		}()

		if _, err = tx.Exec(`
			DELETE FROM icms_fronteira_extrato_sefaz
			WHERE company_id = $1::uuid AND periodo = $2
		`, companyID, periodo); err != nil {
			log.Printf("IcmsFronteiraExtratoImportar delete error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao limpar extrato anterior")
			return
		}

		imported := 0
		for _, pr := range parsed {
			_, err2 := tx.Exec(`
				INSERT INTO icms_fronteira_extrato_sefaz
					(company_id, periodo, registro_nota, cnpj_emitente, nome_emitente,
					 uf_emitente, numero_nf, chave_nfe, icms_devido)
				VALUES
					($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9)
			`, companyID, periodo, pr.RegistroNota, pr.CNPJEmitente, pr.NomeEmitente,
				pr.UFEmitente, pr.NumeroNF, pr.ChaveNFe, pr.ICMSDevido)
			if err2 != nil {
				log.Printf("IcmsFronteiraExtratoImportar insert error: %v", err2)
				continue
			}
			imported++
		}

		if err = tx.Commit(); err != nil {
			log.Printf("IcmsFronteiraExtratoImportar commit error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao confirmar importação")
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"imported": imported,
			"periodo":  periodo,
		})
	}
}

// ---------------------------------------------------------------------------
// IcmsFronteiraExtratoListHandler — GET /api/icms-fronteira/extrato
// ---------------------------------------------------------------------------

func IcmsFronteiraExtratoListHandler(db *sql.DB) http.HandlerFunc {
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

		periodo := strings.TrimSpace(r.URL.Query().Get("periodo"))

		var rows *sql.Rows
		if periodo != "" {
			rows, err = db.Query(`
				SELECT
					id::text,
					periodo,
					COALESCE(registro_nota, ''),
					COALESCE(cnpj_emitente, ''),
					COALESCE(nome_emitente, ''),
					COALESCE(uf_emitente, ''),
					COALESCE(numero_nf, ''),
					COALESCE(chave_nfe, ''),
					COALESCE(icms_devido, 0)
				FROM icms_fronteira_extrato_sefaz
				WHERE company_id = $1::uuid AND periodo = $2
				ORDER BY nome_emitente, numero_nf
			`, companyID, periodo)
		} else {
			rows, err = db.Query(`
				SELECT
					id::text,
					periodo,
					COALESCE(registro_nota, ''),
					COALESCE(cnpj_emitente, ''),
					COALESCE(nome_emitente, ''),
					COALESCE(uf_emitente, ''),
					COALESCE(numero_nf, ''),
					COALESCE(chave_nfe, ''),
					COALESCE(icms_devido, 0)
				FROM icms_fronteira_extrato_sefaz
				WHERE company_id = $1::uuid
				ORDER BY periodo DESC, nome_emitente, numero_nf
			`, companyID)
		}

		if err != nil {
			log.Printf("IcmsFronteiraExtratoList error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar extrato SEFAZ")
			return
		}
		defer rows.Close()

		result := []ExtratoSefazRow{}
		var total float64

		for rows.Next() {
			var row ExtratoSefazRow
			if err := rows.Scan(
				&row.ID, &row.Periodo, &row.RegistroNota, &row.CNPJEmitente,
				&row.NomeEmitente, &row.UFEmitente, &row.NumeroNF, &row.ChaveNFe,
				&row.ICMSDevido,
			); err != nil {
				log.Printf("IcmsFronteiraExtratoList scan error: %v", err)
				continue
			}
			total += row.ICMSDevido
			result = append(result, row)
		}

		json.NewEncoder(w).Encode(ExtratoSefazResponse{
			Rows:  result,
			Total: total,
			Count: len(result),
		})
	}
}

// ---------------------------------------------------------------------------
// IcmsFronteiraExtratoDeleteHandler — DELETE /api/icms-fronteira/extrato?periodo=MM/YYYY
// ---------------------------------------------------------------------------

func IcmsFronteiraExtratoDeleteHandler(db *sql.DB) http.HandlerFunc {
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

		periodo := strings.TrimSpace(r.URL.Query().Get("periodo"))
		if periodo == "" {
			jsonErr(w, http.StatusBadRequest, "Parâmetro 'periodo' obrigatório (MM/YYYY)")
			return
		}

		_, err = db.Exec(`
			DELETE FROM icms_fronteira_extrato_sefaz
			WHERE company_id = $1::uuid AND periodo = $2
		`, companyID, periodo)
		if err != nil {
			log.Printf("IcmsFronteiraExtratoDelete error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao excluir extrato")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
