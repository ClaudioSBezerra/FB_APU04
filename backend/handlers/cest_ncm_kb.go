package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// ---------------------------------------------------------------------------
// Base de Conhecimento CEST→NCM (auto-construída dos dados importados)
//
// O par CEST→NCM chega ao sistema via SPED (reg_0200) e XML
// (nfe_entradas_itens). refreshCestNcmKB consolida esses pares em
// cest_ncm_ref (UPSERT idempotente) — serve tanto para o backfill inicial
// quanto para o auto-feed após cada importação.
//
// GET /api/cest-ncm/kb?segmento=01   → NCMs do segmento (prefixo CEST)
// GET /api/cest-ncm/kb?cest=0104900  → NCMs daquele CEST
// POST /api/cest-ncm/kb/refresh      → reprocessa a KB da empresa
// ---------------------------------------------------------------------------

// normalizaCest remove pontuação do CEST ("01.049.00" → "0104900").
func normalizaCest(s string) string {
	return strings.NewReplacer(".", "", " ", "", "-", "").Replace(strings.TrimSpace(s))
}

// refreshCestNcmKB consolida os pares CEST→NCM da empresa a partir do SPED
// (reg_0200) e do XML (nfe_entradas_itens) em cest_ncm_ref. Idempotente:
// pode ser chamada após cada importação. Retorna quantos pares foram
// inseridos/atualizados (linhas afetadas pelos dois UPSERTs).
//
// O CEST é normalizado (sem pontuação). Pares com CEST ou NCM vazios são
// ignorados. ocorrencias acumula a cada execução que revê o par (confiança).
func RefreshCestNcmKB(db *sql.DB, companyID string) (int64, error) {
	var total int64

	// 1) Do SPED: reg_0200 (cadastro de produtos) via job → company.
	resSped, err := db.Exec(`
		INSERT INTO cest_ncm_ref (company_id, cest, ncm, descricao, fonte, ocorrencias, last_seen)
		SELECT
			$1,
			regexp_replace(p.cest, '[^0-9]', '', 'g')        AS cest,
			LEFT(regexp_replace(p.cod_ncm, '[^0-9]', '', 'g'), 8) AS ncm,
			MAX(p.descr_item)                                AS descricao,
			'sped',
			COUNT(*)                                         AS ocorrencias,
			now()
		FROM reg_0200 p
		JOIN import_jobs j ON j.id = p.job_id
		WHERE j.company_id = $1
		  AND p.cest IS NOT NULL    AND regexp_replace(p.cest, '[^0-9]', '', 'g') <> ''
		  AND p.cod_ncm IS NOT NULL AND regexp_replace(p.cod_ncm, '[^0-9]', '', 'g') <> ''
		GROUP BY 2, 3
		ON CONFLICT (company_id, cest, ncm) DO UPDATE SET
			ocorrencias = cest_ncm_ref.ocorrencias + EXCLUDED.ocorrencias,
			descricao   = COALESCE(NULLIF(EXCLUDED.descricao,''), cest_ncm_ref.descricao),
			last_seen   = now()
	`, companyID)
	if err != nil {
		return total, err
	}
	n, _ := resSped.RowsAffected()
	total += n

	// 2) Do XML: nfe_entradas_itens (já tem company_id direto).
	resXML, err := db.Exec(`
		INSERT INTO cest_ncm_ref (company_id, cest, ncm, descricao, fonte, ocorrencias, last_seen)
		SELECT
			$1,
			regexp_replace(i.cest, '[^0-9]', '', 'g')        AS cest,
			LEFT(regexp_replace(i.ncm, '[^0-9]', '', 'g'), 8) AS ncm,
			MAX(i.x_prod)                                    AS descricao,
			'xml',
			COUNT(*)                                         AS ocorrencias,
			now()
		FROM nfe_entradas_itens i
		WHERE i.company_id = $1
		  AND i.cest IS NOT NULL AND regexp_replace(i.cest, '[^0-9]', '', 'g') <> ''
		  AND i.ncm  IS NOT NULL AND regexp_replace(i.ncm, '[^0-9]', '', 'g')  <> ''
		GROUP BY 2, 3
		ON CONFLICT (company_id, cest, ncm) DO UPDATE SET
			ocorrencias = cest_ncm_ref.ocorrencias + EXCLUDED.ocorrencias,
			descricao   = COALESCE(NULLIF(EXCLUDED.descricao,''), cest_ncm_ref.descricao),
			last_seen   = now()
	`, companyID)
	if err != nil {
		return total, err
	}
	n, _ = resXML.RowsAffected()
	total += n

	return total, nil
}

type cestNcmKBRow struct {
	Cest        string `json:"cest"`
	NCM         string `json:"ncm"`
	Descricao   string `json:"descricao"`
	Fonte       string `json:"fonte"`
	Ocorrencias int    `json:"ocorrencias"`
}

// CestNcmKBHandler — GET consulta a KB; POST /refresh reprocessa.
func CestNcmKBHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
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

		// POST /refresh — reprocessa a KB (backfill manual / sob demanda).
		if r.Method == http.MethodPost {
			n, err := RefreshCestNcmKB(db, companyID)
			if err != nil {
				log.Printf("CestNcmKB refresh error: %v", err)
				jsonErr(w, http.StatusInternalServerError, "Erro ao atualizar base CEST→NCM")
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"atualizados": n})
			return
		}

		if r.Method != http.MethodGet {
			jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		// GET — filtro por segmento (prefixo CEST) ou CEST exato.
		segmento := normalizaCest(r.URL.Query().Get("segmento"))
		cest := normalizaCest(r.URL.Query().Get("cest"))

		query := `
			SELECT cest, ncm, COALESCE(descricao,''), fonte, ocorrencias
			FROM cest_ncm_ref
			WHERE company_id = $1`
		args := []interface{}{companyID}
		switch {
		case cest != "":
			query += ` AND cest = $2`
			args = append(args, cest)
		case segmento != "":
			query += ` AND cest LIKE $2`
			args = append(args, segmento+"%")
		}
		query += ` ORDER BY cest, ocorrencias DESC, ncm LIMIT 5000`

		rows, err := db.Query(query, args...)
		if err != nil {
			log.Printf("CestNcmKB query error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar base CEST→NCM")
			return
		}
		defer rows.Close()

		result := []cestNcmKBRow{}
		for rows.Next() {
			var row cestNcmKBRow
			if err := rows.Scan(&row.Cest, &row.NCM, &row.Descricao, &row.Fonte, &row.Ocorrencias); err != nil {
				continue
			}
			result = append(result, row)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"rows":  result,
			"count": len(result),
		})
	}
}
