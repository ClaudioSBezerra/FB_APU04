package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// ---------------------------------------------------------------------------
// XMLNotasHandler — GET /api/xml/notas/{tipo}
// Serve linhas individuais (não agregadas) das tabelas de XMLs importados:
//   - GET /api/xml/notas/entradas → nfe_entradas (uma linha por nota)
//   - GET /api/xml/notas/saidas  → nfe_saidas   (uma linha por nota)
//   - GET /api/xml/notas/ctes    → cte_entradas  (uma linha por CT-e)
//
// Filtros aceitos via query string:
//   mes_ano  — MM/YYYY  (exato)
//   cnpj     — CNPJ do emitente/fornecedor (prefixo, 14 dígitos sem máscara)
//   limit    — 1-500, default 100
//   offset   — default 0
// ---------------------------------------------------------------------------

type notaRow struct {
	Chave       string  `json:"chave"`
	Numero      string  `json:"numero"`
	Serie       string  `json:"serie"`
	DataEmissao string  `json:"data_emissao"`
	MesAno      string  `json:"mes_ano"`
	NatOp       string  `json:"nat_op"`
	ParCNPJ     string  `json:"par_cnpj"`
	ParNome     string  `json:"par_nome"`
	DestCNPJ    string  `json:"dest_cnpj"`
	DestNome    string  `json:"dest_nome"`
	DestUF      string  `json:"dest_uf"`
	VTotal      float64 `json:"v_total"`
	VBCICMS     float64 `json:"v_bc_icms"`
	VICMS       float64 `json:"v_icms"`
	VPIS        float64 `json:"v_pis"`
	VCOFINS     float64 `json:"v_cofins"`
	VIPI        float64 `json:"v_ipi"`
	VIBS        float64 `json:"v_ibs"`
	VCBS        float64 `json:"v_cbs"`
	Source      string  `json:"source"`
	Status      string  `json:"status"` // ATIVO | CANCELADO (entradas apenas)
}

func XMLNotasHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodGet {
			jsonErr(w, http.StatusMethodNotAllowed, "Método não permitido")
			return
		}

		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			jsonErr(w, http.StatusUnauthorized, "Não autenticado")
			return
		}
		userID, _ := claims["user_id"].(string)

		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa: "+err.Error())
			return
		}

		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/xml/notas/"), "/")
		tipo := parts[0]

		q := r.URL.Query()
		dataInicio := strings.TrimSpace(q.Get("data_inicio"))
		dataFim := strings.TrimSpace(q.Get("data_fim"))
		cnpj := strings.TrimSpace(q.Get("cnpj"))
		destUF := strings.ToUpper(strings.TrimSpace(q.Get("dest_uf")))
		cnpjFilial := onlyDigits(strings.TrimSpace(q.Get("cnpj_filial")))
		chave := strings.TrimSpace(q.Get("chave"))
		limit := 100
		offset := 0
		if v, err := strconv.Atoi(q.Get("limit")); err == nil && v > 0 && v <= 500 {
			limit = v
		}
		if v, err := strconv.Atoi(q.Get("offset")); err == nil && v >= 0 {
			offset = v
		}

		var selectSQL, fromTable, orderBy, cnpjCol, chaveCol, numCol string

		switch tipo {
		case "entradas":
			fromTable = "nfe_entradas"
			cnpjCol = "forn_cnpj"
			chaveCol, numCol = "chave_nfe", "numero_nfe"
			selectSQL = `
				SELECT chave_nfe, COALESCE(numero_nfe,''), COALESCE(serie,''),
				       TO_CHAR(data_emissao,'DD/MM/YYYY'), mes_ano,
				       COALESCE(nat_op,''),
				       forn_cnpj, COALESCE(forn_nome,''),
				       COALESCE(dest_cnpj_cpf,''), COALESCE(dest_nome,''), COALESCE(dest_uf,''),
				       COALESCE(v_nf,0), COALESCE(v_bc,0), COALESCE(v_icms,0),
				       COALESCE(v_pis,0), COALESCE(v_cofins,0), COALESCE(v_ipi,0),
				       COALESCE(v_ibs,0), COALESCE(v_cbs,0), source,
				       COALESCE(status, 'ATIVO')
				FROM nfe_entradas`
			orderBy = "data_emissao DESC, numero_nfe DESC"

		case "saidas":
			fromTable = "nfe_saidas"
			cnpjCol = "emit_cnpj"
			chaveCol, numCol = "chave_nfe", "numero_nfe"
			selectSQL = `
				SELECT chave_nfe, COALESCE(numero_nfe,''), COALESCE(serie,''),
				       TO_CHAR(data_emissao,'DD/MM/YYYY'), mes_ano,
				       COALESCE(nat_op,''),
				       emit_cnpj, COALESCE(emit_nome,''),
				       COALESCE(dest_cnpj_cpf,''), COALESCE(dest_nome,''), COALESCE(dest_uf,''),
				       COALESCE(v_nf,0), COALESCE(v_bc,0), COALESCE(v_icms,0),
				       COALESCE(v_pis,0), COALESCE(v_cofins,0), COALESCE(v_ipi,0),
				       COALESCE(v_ibs,0), COALESCE(v_cbs,0), source, ''
				FROM nfe_saidas`
			orderBy = "data_emissao DESC, numero_nfe DESC"

		case "ctes":
			fromTable = "cte_entradas"
			cnpjCol = "emit_cnpj"
			chaveCol, numCol = "chave_cte", "numero_cte"
			selectSQL = `
				SELECT chave_cte, COALESCE(numero_cte,''), '',
				       TO_CHAR(data_emissao,'DD/MM/YYYY'), mes_ano,
				       '',
				       emit_cnpj, COALESCE(emit_nome,''),
				       '', '', '',
				       COALESCE(v_rec,0), COALESCE(v_bc_icms,0), COALESCE(v_icms,0),
				       0, 0, 0,
				       COALESCE(v_ibs,0), COALESCE(v_cbs,0), source, ''
				FROM cte_entradas`
			orderBy = "data_emissao DESC, numero_cte DESC"

		default:
			jsonErr(w, http.StatusBadRequest, "tipo inválido: use entradas, saidas ou ctes")
			return
		}

		args := []interface{}{companyID}
		where := "WHERE company_id = $1"
		idx := 2

		if dataInicio != "" {
			where += fmt.Sprintf(" AND data_emissao >= $%d", idx)
			args = append(args, dataInicio)
			idx++
		}
		if dataFim != "" {
			where += fmt.Sprintf(" AND data_emissao <= $%d", idx)
			args = append(args, dataFim)
			idx++
		}
		if cnpj != "" {
			where += fmt.Sprintf(" AND %s LIKE $%d", cnpjCol, idx)
			args = append(args, cnpj+"%")
			idx++
		}
		if destUF != "" {
			where += fmt.Sprintf(" AND dest_uf = $%d", idx)
			args = append(args, destUF)
			idx++
		}
		if cnpjFilial != "" {
			var filialCol string
			switch tipo {
			case "entradas", "ctes":
				filialCol = "dest_cnpj_cpf"
			case "saidas":
				filialCol = "emit_cnpj"
			}
			if filialCol != "" {
				where += fmt.Sprintf(" AND regexp_replace(COALESCE(%s,''),'[^0-9]','','g') LIKE $%d", filialCol, idx)
				args = append(args, cnpjFilial+"%")
				idx++
			}
		}
		if chave != "" {
			where += fmt.Sprintf(" AND (COALESCE(%s,'') ILIKE $%d OR COALESCE(%s,'') ILIKE $%d)", chaveCol, idx, numCol, idx+1)
			like := "%" + chave + "%"
			args = append(args, like, like)
			idx += 2
		}

		countArgs := make([]interface{}, len(args))
		copy(countArgs, args)

		args = append(args, limit, offset)

		query := fmt.Sprintf(`%s %s ORDER BY %s LIMIT $%d OFFSET $%d`,
			selectSQL, where, orderBy, idx, idx+1)

		rows, err := db.Query(query, args...)
		if err != nil {
			log.Printf("[XMLNotas] query error (tipo=%s): %v", tipo, err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar banco")
			return
		}
		defer rows.Close()

		var list []notaRow
		for rows.Next() {
			var row notaRow
			if err := rows.Scan(
				&row.Chave, &row.Numero, &row.Serie,
				&row.DataEmissao, &row.MesAno, &row.NatOp,
				&row.ParCNPJ, &row.ParNome,
				&row.DestCNPJ, &row.DestNome, &row.DestUF,
				&row.VTotal, &row.VBCICMS, &row.VICMS,
				&row.VPIS, &row.VCOFINS, &row.VIPI,
				&row.VIBS, &row.VCBS, &row.Source, &row.Status,
			); err != nil {
				log.Printf("[XMLNotas] scan error: %v", err)
				continue
			}
			list = append(list, row)
		}

		var total int
		db.QueryRow(
			fmt.Sprintf(`SELECT COUNT(*) FROM %s %s`, fromTable, where),
			countArgs...,
		).Scan(&total)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"total":  total,
			"limit":  limit,
			"offset": offset,
			"tipo":   tipo,
			"items":  list,
		})
	}
}
