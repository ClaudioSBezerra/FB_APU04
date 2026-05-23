package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// ---------------------------------------------------------------------------
// Structs
// ---------------------------------------------------------------------------

type ContestacaoRow struct {
	ID              string  `json:"id"`
	ChaveNFe        string  `json:"chave_nfe"`
	NumeroNF        string  `json:"numero_nf"`
	FornCNPJ        string  `json:"forn_cnpj"`
	FornNome        string  `json:"forn_nome"`
	Periodo         string  `json:"periodo"`
	ValorContestado float64 `json:"valor_contestado"`
	Motivo          string  `json:"motivo"`
	Status          string  `json:"status"`
	RespostaSefaz   *string `json:"resposta_sefaz"`
	DataRegistro    string  `json:"data_registro"`
	DataResposta    *string `json:"data_resposta"`
}

type ContestacaoCreateRequest struct {
	ChaveNFe        string  `json:"chave_nfe"`
	NumeroNF        string  `json:"numero_nf"`
	FornCNPJ        string  `json:"forn_cnpj"`
	FornNome        string  `json:"forn_nome"`
	Periodo         string  `json:"periodo"`
	ValorContestado float64 `json:"valor_contestado"`
	Motivo          string  `json:"motivo"`
}

type ContestacaoUpdateRequest struct {
	Status        string `json:"status"`
	RespostaSefaz string `json:"resposta_sefaz"`
}

// ---------------------------------------------------------------------------
// IcmsFronteiraContestacaoListHandler — GET /api/icms-fronteira/contestacoes
// ---------------------------------------------------------------------------

func IcmsFronteiraContestacaoListHandler(db *sql.DB) http.HandlerFunc {
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

		status := strings.TrimSpace(r.URL.Query().Get("status"))
		periodo := strings.TrimSpace(r.URL.Query().Get("periodo"))

		// Build dynamic WHERE
		args := []interface{}{companyID}
		where := []string{"company_id = $1::uuid"}

		if status != "" {
			args = append(args, status)
			where = append(where, "status = $"+itoa(len(args)))
		}
		if periodo != "" {
			args = append(args, periodo)
			where = append(where, "periodo = $"+itoa(len(args)))
		}

		query := `
			SELECT
				id::text,
				COALESCE(chave_nfe, ''),
				COALESCE(numero_nf, ''),
				COALESCE(forn_cnpj, ''),
				COALESCE(forn_nome, ''),
				COALESCE(periodo, ''),
				COALESCE(valor_contestado, 0),
				COALESCE(motivo, ''),
				COALESCE(status, 'pendente'),
				resposta_sefaz,
				COALESCE(data_registro::text, ''),
				data_resposta::text
			FROM icms_fronteira_contestacoes
			WHERE ` + strings.Join(where, " AND ") + `
			ORDER BY data_registro DESC
		`

		rows, err := db.Query(query, args...)
		if err != nil {
			log.Printf("IcmsFronteiraContestacaoList error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar contestações")
			return
		}
		defer rows.Close()

		result := []ContestacaoRow{}
		for rows.Next() {
			var row ContestacaoRow
			var respostaSefaz sql.NullString
			var dataResposta sql.NullString
			if err := rows.Scan(
				&row.ID,
				&row.ChaveNFe,
				&row.NumeroNF,
				&row.FornCNPJ,
				&row.FornNome,
				&row.Periodo,
				&row.ValorContestado,
				&row.Motivo,
				&row.Status,
				&respostaSefaz,
				&row.DataRegistro,
				&dataResposta,
			); err != nil {
				log.Printf("IcmsFronteiraContestacaoList scan error: %v", err)
				continue
			}
			if respostaSefaz.Valid {
				row.RespostaSefaz = &respostaSefaz.String
			}
			if dataResposta.Valid {
				row.DataResposta = &dataResposta.String
			}
			result = append(result, row)
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"rows":  result,
			"count": len(result),
		})
	}
}

// ---------------------------------------------------------------------------
// IcmsFronteiraContestacaoCreateHandler — POST /api/icms-fronteira/contestacoes
// ---------------------------------------------------------------------------

func IcmsFronteiraContestacaoCreateHandler(db *sql.DB) http.HandlerFunc {
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

		var body ContestacaoCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonErr(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
			return
		}

		body.Motivo = strings.TrimSpace(body.Motivo)
		if body.Motivo == "" {
			jsonErr(w, http.StatusBadRequest, "Campo 'motivo' é obrigatório")
			return
		}

		var row ContestacaoRow
		var respostaSefaz sql.NullString
		var dataResposta sql.NullString

		err = db.QueryRow(`
			INSERT INTO icms_fronteira_contestacoes
				(company_id, chave_nfe, numero_nf, forn_cnpj, forn_nome, periodo,
				 valor_contestado, motivo, status, data_registro)
			VALUES
				($1::uuid, $2, $3, $4, $5, $6, $7, $8, 'pendente', now())
			RETURNING
				id::text,
				COALESCE(chave_nfe, ''),
				COALESCE(numero_nf, ''),
				COALESCE(forn_cnpj, ''),
				COALESCE(forn_nome, ''),
				COALESCE(periodo, ''),
				COALESCE(valor_contestado, 0),
				COALESCE(motivo, ''),
				COALESCE(status, 'pendente'),
				resposta_sefaz,
				COALESCE(data_registro::text, ''),
				data_resposta::text
		`, companyID, body.ChaveNFe, body.NumeroNF, body.FornCNPJ, body.FornNome,
			body.Periodo, body.ValorContestado, body.Motivo,
		).Scan(
			&row.ID,
			&row.ChaveNFe,
			&row.NumeroNF,
			&row.FornCNPJ,
			&row.FornNome,
			&row.Periodo,
			&row.ValorContestado,
			&row.Motivo,
			&row.Status,
			&respostaSefaz,
			&row.DataRegistro,
			&dataResposta,
		)
		if err != nil {
			log.Printf("IcmsFronteiraContestacaoCreate error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao criar contestação")
			return
		}
		if respostaSefaz.Valid {
			row.RespostaSefaz = &respostaSefaz.String
		}
		if dataResposta.Valid {
			row.DataResposta = &dataResposta.String
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(row)
	}
}

// ---------------------------------------------------------------------------
// IcmsFronteiraContestacaoUpdateHandler — PUT /api/icms-fronteira/contestacoes/{id}
// ---------------------------------------------------------------------------

func IcmsFronteiraContestacaoUpdateHandler(db *sql.DB) http.HandlerFunc {
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
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa: "+err.Error())
			return
		}

		id := strings.TrimPrefix(r.URL.Path, "/api/icms-fronteira/contestacoes/")
		id = strings.TrimSpace(id)
		if id == "" {
			jsonErr(w, http.StatusBadRequest, "ID não informado")
			return
		}

		var body ContestacaoUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonErr(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
			return
		}

		// Determine whether to set data_resposta
		setDataResposta := body.Status == "deferida" || body.Status == "indeferida"

		var respostaSefazArg interface{}
		if body.RespostaSefaz != "" {
			respostaSefazArg = body.RespostaSefaz
		}

		var row ContestacaoRow
		var respostaSefaz sql.NullString
		var dataResposta sql.NullString

		var scanErr error
		if setDataResposta {
			scanErr = db.QueryRow(`
				UPDATE icms_fronteira_contestacoes
				SET status = $1,
				    resposta_sefaz = $2,
				    data_resposta = now()
				WHERE id = $3::uuid AND company_id = $4::uuid
				RETURNING
					id::text,
					COALESCE(chave_nfe, ''),
					COALESCE(numero_nf, ''),
					COALESCE(forn_cnpj, ''),
					COALESCE(forn_nome, ''),
					COALESCE(periodo, ''),
					COALESCE(valor_contestado, 0),
					COALESCE(motivo, ''),
					COALESCE(status, 'pendente'),
					resposta_sefaz,
					COALESCE(data_registro::text, ''),
					data_resposta::text
			`, body.Status, respostaSefazArg, id, companyID,
			).Scan(
				&row.ID, &row.ChaveNFe, &row.NumeroNF, &row.FornCNPJ, &row.FornNome,
				&row.Periodo, &row.ValorContestado, &row.Motivo, &row.Status,
				&respostaSefaz, &row.DataRegistro, &dataResposta,
			)
		} else {
			scanErr = db.QueryRow(`
				UPDATE icms_fronteira_contestacoes
				SET status = $1,
				    resposta_sefaz = $2
				WHERE id = $3::uuid AND company_id = $4::uuid
				RETURNING
					id::text,
					COALESCE(chave_nfe, ''),
					COALESCE(numero_nf, ''),
					COALESCE(forn_cnpj, ''),
					COALESCE(forn_nome, ''),
					COALESCE(periodo, ''),
					COALESCE(valor_contestado, 0),
					COALESCE(motivo, ''),
					COALESCE(status, 'pendente'),
					resposta_sefaz,
					COALESCE(data_registro::text, ''),
					data_resposta::text
			`, body.Status, respostaSefazArg, id, companyID,
			).Scan(
				&row.ID, &row.ChaveNFe, &row.NumeroNF, &row.FornCNPJ, &row.FornNome,
				&row.Periodo, &row.ValorContestado, &row.Motivo, &row.Status,
				&respostaSefaz, &row.DataRegistro, &dataResposta,
			)
		}

		if scanErr == sql.ErrNoRows {
			jsonErr(w, http.StatusNotFound, "Contestação não encontrada")
			return
		}
		if scanErr != nil {
			log.Printf("IcmsFronteiraContestacaoUpdate error: %v", scanErr)
			jsonErr(w, http.StatusInternalServerError, "Erro ao atualizar contestação")
			return
		}

		if respostaSefaz.Valid {
			row.RespostaSefaz = &respostaSefaz.String
		}
		if dataResposta.Valid {
			row.DataResposta = &dataResposta.String
		}

		json.NewEncoder(w).Encode(row)
	}
}

// ---------------------------------------------------------------------------
// IcmsFronteiraContestacaoDeleteHandler — DELETE /api/icms-fronteira/contestacoes/{id}
// ---------------------------------------------------------------------------

func IcmsFronteiraContestacaoDeleteHandler(db *sql.DB) http.HandlerFunc {
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

		id := strings.TrimPrefix(r.URL.Path, "/api/icms-fronteira/contestacoes/")
		id = strings.TrimSpace(id)
		if id == "" {
			jsonErr(w, http.StatusBadRequest, "ID não informado")
			return
		}

		res, err := db.Exec(`
			DELETE FROM icms_fronteira_contestacoes
			WHERE id = $1::uuid AND company_id = $2::uuid AND status = 'pendente'
		`, id, companyID)
		if err != nil {
			log.Printf("IcmsFronteiraContestacaoDelete error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao excluir contestação")
			return
		}

		n, _ := res.RowsAffected()
		if n == 0 {
			jsonErr(w, http.StatusNotFound, "Contestação não encontrada ou não está com status 'pendente'")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// itoa converts int to string for SQL placeholder building ($1, $2, ...).
func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
