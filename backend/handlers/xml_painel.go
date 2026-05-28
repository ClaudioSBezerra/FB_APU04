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
// XMLEntradasInformativosHandler — GET /api/xml/painel/entradas-informativos
// ---------------------------------------------------------------------------

func XMLEntradasInformativosHandler(db *sql.DB) http.HandlerFunc {
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

		rows, err := db.Query(`
			SELECT mes_ano, total_ipi, total_pis_simples, total_cofins_simples, qtd_notas
			FROM vw_xml_entradas_informativos
			WHERE company_id = $1
			ORDER BY mes_ano`,
			companyID,
		)
		if err != nil {
			log.Printf("[XMLEntradasInformativos] query error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar banco")
			return
		}
		defer rows.Close()

		type informativoRow struct {
			MesAno             string  `json:"mes_ano"`
			TotalIPI           float64 `json:"total_ipi"`
			TotalPisSimples    float64 `json:"total_pis_simples"`
			TotalCofinsSimples float64 `json:"total_cofins_simples"`
			QtdNotas           int     `json:"qtd_notas"`
		}

		var list []informativoRow
		for rows.Next() {
			var row informativoRow
			if err := rows.Scan(
				&row.MesAno, &row.TotalIPI, &row.TotalPisSimples, &row.TotalCofinsSimples, &row.QtdNotas,
			); err != nil {
				log.Printf("[XMLEntradasInformativos] scan error: %v", err)
				continue
			}
			list = append(list, row)
		}

		if list == nil {
			list = []informativoRow{}
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"total": len(list),
			"items": list,
		})
	}
}

// ---------------------------------------------------------------------------
// XMLPainelHandler — GET /api/xml/painel/{tipo}
//
// Serve 3 rotas do painel de XMLs importados:
//   - GET /api/xml/painel/entradas → nfe_entradas  (agrupado por fornecedor/mês)
//   - GET /api/xml/painel/saidas  → nfe_saidas     (agrupado por emitente/mês)
//   - GET /api/xml/painel/ctes    → cte_entradas   (agrupado por transportadora/mês)
//
// Filtros aceitos:
//   mes_ano      — MM/YYYY
//   dest_uf      — UF do destinatário (ex: PE, BA)
//   cnpj_filial  — CNPJ parcial da filial (dest_cnpj_cpf em entradas/ctes; emit_cnpj em saidas)
//   limit/offset — paginação
// ---------------------------------------------------------------------------

type painelTipoConfig struct {
	selectSQL string // SELECT … FROM <tabela>
	groupBy   string
	destUFCol string // coluna de UF para filtro
	filialCol string // coluna de CNPJ filial para filtro
}

var painelConfigs = map[string]painelTipoConfig{
	"entradas": {
		selectSQL: `SELECT forn_cnpj, COALESCE(forn_nome,''), mes_ano, COALESCE(source,''),
		    COUNT(*)::int,
		    SUM(COALESCE(v_nf,0)), SUM(COALESCE(v_bc,0)), SUM(COALESCE(v_icms,0)),
		    SUM(COALESCE(v_pis,0)), SUM(COALESCE(v_cofins,0)), SUM(COALESCE(v_ipi,0)),
		    SUM(COALESCE(v_ibs,0)), SUM(COALESCE(v_cbs,0))
		FROM nfe_entradas`,
		groupBy:   "GROUP BY forn_cnpj, forn_nome, mes_ano, source",
		destUFCol: "dest_uf",
		filialCol: "dest_cnpj_cpf",
	},
	"saidas": {
		selectSQL: `SELECT emit_cnpj, COALESCE(emit_nome,''), mes_ano, COALESCE(source,''),
		    COUNT(*)::int,
		    SUM(COALESCE(v_nf,0)), SUM(COALESCE(v_bc,0)), SUM(COALESCE(v_icms,0)),
		    SUM(COALESCE(v_pis,0)), SUM(COALESCE(v_cofins,0)), SUM(COALESCE(v_ipi,0)),
		    SUM(COALESCE(v_ibs,0)), SUM(COALESCE(v_cbs,0))
		FROM nfe_saidas`,
		groupBy:   "GROUP BY emit_cnpj, emit_nome, mes_ano, source",
		destUFCol: "dest_uf",
		filialCol: "emit_cnpj",
	},
	"ctes": {
		selectSQL: `SELECT emit_cnpj, COALESCE(emit_nome,''), mes_ano, COALESCE(source,''),
		    COUNT(*)::int,
		    SUM(COALESCE(v_rec,0)), SUM(COALESCE(v_bc_icms,0)), SUM(COALESCE(v_icms,0)),
		    0::numeric, 0::numeric, 0::numeric,
		    SUM(COALESCE(v_ibs,0)), SUM(COALESCE(v_cbs,0))
		FROM cte_entradas`,
		groupBy:   "GROUP BY emit_cnpj, emit_nome, mes_ano, source",
		destUFCol: "dest_uf",
		filialCol: "dest_cnpj_cpf",
	},
}

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

		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/xml/painel/"), "/")
		tipo := parts[0]

		cfg, ok := painelConfigs[tipo]
		if !ok {
			jsonErr(w, http.StatusBadRequest, "tipo inválido: use entradas, saidas ou ctes")
			return
		}

		q := r.URL.Query()
		mesAnoFiltro := strings.TrimSpace(q.Get("mes_ano"))
		destUF := strings.ToUpper(strings.TrimSpace(q.Get("dest_uf")))
		cnpjFilial := onlyDigits(strings.TrimSpace(q.Get("cnpj_filial")))

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
		if destUF != "" {
			where += fmt.Sprintf(" AND %s = $%d", cfg.destUFCol, idx)
			args = append(args, destUF)
			idx++
		}
		if cnpjFilial != "" {
			where += fmt.Sprintf(" AND regexp_replace(COALESCE(%s,''),'[^0-9]','','g') LIKE $%d", cfg.filialCol, idx)
			args = append(args, cnpjFilial+"%")
			idx++
		}

		countArgs := make([]interface{}, len(args))
		copy(countArgs, args)
		args = append(args, limit, offset)

		query := fmt.Sprintf(`%s %s %s ORDER BY mes_ano DESC, 2 LIMIT $%d OFFSET $%d`,
			cfg.selectSQL, where, cfg.groupBy, idx, idx+1)

		countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM (%s %s %s) _cnt`,
			cfg.selectSQL, where, cfg.groupBy)

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

		var total int
		db.QueryRow(countQuery, countArgs...).Scan(&total)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"total":  total,
			"limit":  limit,
			"offset": offset,
			"tipo":   tipo,
			"items":  list,
		})
	}
}
