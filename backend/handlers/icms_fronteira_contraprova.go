package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// ---------------------------------------------------------------------------
// Contraprova SPED vs. Sistema
//
// GET /api/icms-fronteira/contraprova?periodo=MM/YYYY&uf=UF
//
// Compara o SPED bruto (reg_c170 × reg_c100) com o que a MV capturou, por
// CFOP de fronteira. Se diff_qtd = 0 em todos os CFOPs, o sistema está lendo
// 100% do SPED. Diff > 0 revela notas perdidas por filtro ou MV desatualizada.
//
// Nota: Bloco C (XML sem SPED) não entra na comparação — por definição não
// está no SPED, logo não tem como contraprovar.
// ---------------------------------------------------------------------------

type ContraprovaRow struct {
	CFOP       string  `json:"cfop"`
	SPEDQtd    int     `json:"sped_qtd"`
	MVQtd      int     `json:"mv_qtd"`
	DiffQtd    int     `json:"diff_qtd"`
	SPEDValor  float64 `json:"sped_valor"`
	MVValor    float64 `json:"mv_valor"`
	DiffValor  float64 `json:"diff_valor"`
}

type ContraprovaResp struct {
	Rows    []ContraprovaRow `json:"rows"`
	Total   ContraprovaRow   `json:"total"`
	Periodo string           `json:"periodo"`
	UF      string           `json:"uf"`
}

func IcmsFronteiraContraprovaHandler(db *sql.DB) http.HandlerFunc {
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
		periodo := strings.TrimSpace(r.URL.Query().Get("periodo"))
		uf := strings.TrimSpace(r.URL.Query().Get("uf"))

		rows, err := runContraprova(db, companyID, uf, periodo)
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, err.Error())
			return
		}

		total := ContraprovaRow{CFOP: "TOTAL"}
		for _, row := range rows {
			total.SPEDQtd += row.SPEDQtd
			total.MVQtd += row.MVQtd
			total.SPEDValor += row.SPEDValor
			total.MVValor += row.MVValor
		}
		total.DiffQtd = total.SPEDQtd - total.MVQtd
		total.DiffValor = total.SPEDValor - total.MVValor

		json.NewEncoder(w).Encode(ContraprovaResp{
			Rows:    rows,
			Total:   total,
			Periodo: periodo,
			UF:      uf,
		})
	}
}

func runContraprova(db *sql.DB, companyID, uf, periodo string) ([]ContraprovaRow, error) {
	// Parse período MM/YYYY
	var mes, ano int
	if len(periodo) >= 7 {
		fmt.Sscanf(periodo[:2], "%d", &mes)
		fmt.Sscanf(periodo[3:7], "%d", &ano)
	}

	// Montar args e filtros — mesmos para as duas queries
	args := []interface{}{companyID} // $1
	n := 2

	ufFilter := ""
	if uf != "" {
		ufFilter = fmt.Sprintf(" AND j.uf = $%d", n)
		args = append(args, uf)
		n++
	}

	periodFilter := ""
	if mes > 0 && ano > 0 {
		periodFilter = fmt.Sprintf(
			" AND EXTRACT(MONTH FROM c100.dt_doc)::int = $%d AND EXTRACT(YEAR FROM c100.dt_doc)::int = $%d",
			n, n+1)
		args = append(args, mes, ano)
		n += 2
	}

	// CFOPs de fronteira (excluindo 2551/2556 DIFAL que têm regime distinto)
	cfops := []string{"2101", "2102", "2152", "2403", "2409", "2651", "2652", "2551", "2556"}
	phs := make([]string, len(cfops))
	for i, c := range cfops {
		phs[i] = fmt.Sprintf("$%d", n+i)
		args = append(args, c)
	}
	cfopIN := strings.Join(phs, ",")

	// ---- SPED ground truth --------------------------------------------------
	spedQ := fmt.Sprintf(`
		SELECT c170.cfop,
		       COUNT(DISTINCT c100.id)::int       AS qtd,
		       COALESCE(SUM(c170.vl_item), 0)     AS valor
		FROM reg_c170 c170
		JOIN reg_c100 c100 ON c100.id = c170.c100_id
		JOIN import_jobs j  ON j.id   = c100.job_id
		WHERE j.company_id = $1::uuid
		  AND c170.cfop IN (%s)
		  %s%s
		GROUP BY c170.cfop
		ORDER BY c170.cfop
	`, cfopIN, ufFilter, periodFilter)

	type kv struct{ qtd int; valor float64 }
	spedMap := map[string]kv{}
	rows1, err := db.Query(spedQ, args...)
	if err != nil {
		return nil, fmt.Errorf("contraprova sped query: %w", err)
	}
	defer rows1.Close()
	for rows1.Next() {
		var cfop string
		var v kv
		if err := rows1.Scan(&cfop, &v.qtd, &v.valor); err != nil {
			continue
		}
		spedMap[cfop] = v
	}

	// ---- Sistema (MV) -------------------------------------------------------
	mvQ := fmt.Sprintf(`
		SELECT l.cfop,
		       COUNT(DISTINCT l.c100_id)::int      AS qtd,
		       COALESCE(SUM(l.v_prod_disp), 0)     AS valor
		FROM mv_icms_fronteira_linhas l
		JOIN reg_c100 c100 ON c100.id = l.c100_id
		JOIN import_jobs j  ON j.id   = c100.job_id
		WHERE l.company_id = $1::uuid
		  AND l.cfop IN (%s)
		  %s%s
		GROUP BY l.cfop
		ORDER BY l.cfop
	`, cfopIN, ufFilter, periodFilter)

	mvMap := map[string]kv{}
	rows2, err := db.Query(mvQ, args...)
	if err != nil {
		return nil, fmt.Errorf("contraprova mv query: %w", err)
	}
	defer rows2.Close()
	for rows2.Next() {
		var cfop string
		var v kv
		if err := rows2.Scan(&cfop, &v.qtd, &v.valor); err != nil {
			continue
		}
		mvMap[cfop] = v
	}

	// ---- Merge --------------------------------------------------------------
	var result []ContraprovaRow
	for _, cfop := range cfops {
		s := spedMap[cfop]
		m := mvMap[cfop]
		if s.qtd == 0 && m.qtd == 0 {
			continue
		}
		result = append(result, ContraprovaRow{
			CFOP:      cfop,
			SPEDQtd:   s.qtd,
			MVQtd:     m.qtd,
			DiffQtd:   s.qtd - m.qtd,
			SPEDValor: s.valor,
			MVValor:   m.valor,
			DiffValor: s.valor - m.valor,
		})
	}
	return result, nil
}
