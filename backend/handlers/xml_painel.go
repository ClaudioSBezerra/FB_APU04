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
// XMLPainelHandler — GET /api/xml/painel/{tipo}
// Serve 3 rotas do painel de XMLs importados:
//   - GET /api/xml/painel/entradas → vw_xml_entradas_resumo
//   - GET /api/xml/painel/saidas  → vw_xml_saidas_resumo
//   - GET /api/xml/painel/ctes    → vw_xml_ctes_resumo
// ---------------------------------------------------------------------------

func XMLPainelHandler(db *sql.DB) http.HandlerFunc {
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

		// Extrair tipo do path: /api/xml/painel/{tipo}
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/xml/painel/"), "/")
		tipo := parts[0] // "entradas", "saidas" ou "ctes"

		viewMap := map[string]string{
			"entradas": "vw_xml_entradas_resumo",
			"saidas":   "vw_xml_saidas_resumo",
			"ctes":     "vw_xml_ctes_resumo",
		}
		view, ok := viewMap[tipo]
		if !ok {
			jsonErr(w, http.StatusBadRequest, "tipo inválido: use entradas, saidas ou ctes")
			return
		}

		// Parâmetros de filtro
		q := r.URL.Query()
		mesAnoFiltro := strings.TrimSpace(q.Get("mes_ano"))
		limit := 100
		offset := 0
		if v, err := strconv.Atoi(q.Get("limit")); err == nil && v > 0 && v <= 500 {
			limit = v
		}
		if v, err := strconv.Atoi(q.Get("offset")); err == nil && v >= 0 {
			offset = v
		}

		args := []interface{}{companyID}
		where := "WHERE company_id = $1"
		idx := 2

		if mesAnoFiltro != "" {
			where += fmt.Sprintf(" AND mes_ano = $%d", idx)
			args = append(args, mesAnoFiltro)
			idx++
		}

		args = append(args, limit, offset)

		query := fmt.Sprintf(`
			SELECT forn_cnpj, forn_nome, mes_ano, source,
			       qtd_notas, v_total, v_bc_icms, v_icms,
			       v_pis, v_cofins, v_ipi, v_ibs, v_cbs
			FROM %s
			%s
			ORDER BY mes_ano DESC, forn_nome
			LIMIT $%d OFFSET $%d`,
			view, where, idx, idx+1,
		)

		rows, err := db.Query(query, args...)
		if err != nil {
			log.Printf("[XMLPainel] query error (tipo=%s): %v", tipo, err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar banco")
			return
		}
		defer rows.Close()

		type painelRow struct {
			FornCNPJ string  `json:"forn_cnpj"`
			FornNome string  `json:"forn_nome"`
			MesAno   string  `json:"mes_ano"`
			Source   string  `json:"source"`
			QtdNotas int     `json:"qtd_notas"`
			VTotal   float64 `json:"v_total"`
			VBCICMS  float64 `json:"v_bc_icms"`
			VICMS    float64 `json:"v_icms"`
			VPIS     float64 `json:"v_pis"`
			VCOFINS  float64 `json:"v_cofins"`
			VIPI     float64 `json:"v_ipi"`
			VIBS     float64 `json:"v_ibs"`
			VCBS     float64 `json:"v_cbs"`
		}

		var list []painelRow
		for rows.Next() {
			var row painelRow
			if err := rows.Scan(
				&row.FornCNPJ, &row.FornNome, &row.MesAno, &row.Source,
				&row.QtdNotas, &row.VTotal, &row.VBCICMS, &row.VICMS,
				&row.VPIS, &row.VCOFINS, &row.VIPI, &row.VIBS, &row.VCBS,
			); err != nil {
				log.Printf("[XMLPainel] scan error: %v", err)
				continue
			}
			list = append(list, row)
		}

		// Contagem total para paginação
		var total int
		countArgs := args[:len(args)-2] // remove limit e offset
		db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s %s`, view, where), countArgs...).Scan(&total)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"total":  total,
			"limit":  limit,
			"offset": offset,
			"tipo":   tipo,
			"items":  list,
		})
	}
}
