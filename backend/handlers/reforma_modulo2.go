package handlers

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
)

// ---------------------------------------------------------------------------
// Structs — Módulo 2.2: Análise por Natureza CFOP (RFMC-01)
// ---------------------------------------------------------------------------

type Modulo22Row struct {
	NaturezaCFOP string  `json:"natureza_cfop"`
	QtdNotas     int     `json:"qtd_notas"`
	ValorTotal   float64 `json:"valor_total"`
	IBSProjetado float64 `json:"ibs_projetado"`
	CBSProjetado float64 `json:"cbs_projetado"`
}

type Modulo22Response struct {
	Rows       []Modulo22Row `json:"rows"`
	TotalIBS   float64       `json:"total_ibs"`
	TotalCBS   float64       `json:"total_cbs"`
	AliqIBSPct float64       `json:"aliq_ibs_pct"`
	AliqCBSPct float64       `json:"aliq_cbs_pct"`
}

// ---------------------------------------------------------------------------
// Structs — Módulo 2.1: Análise por NCM (RFMC-02)
// ---------------------------------------------------------------------------

type Modulo21Row struct {
	NCM          string  `json:"ncm"`
	XProd        string  `json:"x_prod"`
	VlProd       float64 `json:"vl_prod"`
	VlICMS       float64 `json:"vl_icms"`
	AliqICMSEfet float64 `json:"aliq_icms_efet"`
	IBSProjetado float64 `json:"ibs_projetado"`
	CBSProjetado float64 `json:"cbs_projetado"`
	IsFlag       bool    `json:"is_flag"`
}

type Modulo21Response struct {
	Rows       []Modulo21Row `json:"rows"`
	AliqIBSPct float64       `json:"aliq_ibs_pct"`
	AliqCBSPct float64       `json:"aliq_cbs_pct"`
}

// ---------------------------------------------------------------------------
// Structs — Módulo 2.3: Análise por UF Destino (RFMC-03)
// ---------------------------------------------------------------------------

type Modulo23Row struct {
	DestUF       string  `json:"dest_uf"`
	QtdNotas     int     `json:"qtd_notas"`
	ValorTotal   float64 `json:"valor_total"`
	VlICMS       float64 `json:"vl_icms"`
	IBSProjetado float64 `json:"ibs_projetado"`
	CBSProjetado float64 `json:"cbs_projetado"`
}

type Modulo23Response struct {
	Rows       []Modulo23Row `json:"rows"`
	AliqIBSPct float64       `json:"aliq_ibs_pct"`
	AliqCBSPct float64       `json:"aliq_cbs_pct"`
}

// ---------------------------------------------------------------------------
// Structs — Módulo 2.4: Segmentação B2B vs B2C (RFMC-04)
// ---------------------------------------------------------------------------

type Modulo24Row struct {
	Segmento     string  `json:"segmento"`
	QtdNotas     int     `json:"qtd_notas"`
	ValorTotal   float64 `json:"valor_total"`
	IBSProjetado float64 `json:"ibs_projetado"`
	CBSProjetado float64 `json:"cbs_projetado"`
}

type Modulo24Response struct {
	Rows           []Modulo24Row `json:"rows"`
	QtdSemIndFinal int           `json:"qtd_sem_ind_final"`
	AliqIBSPct     float64       `json:"aliq_ibs_pct"`
	AliqCBSPct     float64       `json:"aliq_cbs_pct"`
}

// ---------------------------------------------------------------------------
// helper: lê aliq_ibs_pct, aliq_cbs_pct via tabela_aliquotas (target_ano).
// aliq_ibs_pct e aliq_cbs_pct foram removidas de reforma_parametros na
// migration 090 — derivam de tabela_aliquotas (EC 132/2023).
// defaults 26.5 e 9.9 (fase plena 2033, conformes EC 132).
// ---------------------------------------------------------------------------

func readModulo2Params(db *sql.DB, companyID string) (float64, float64, error) {
	var aliqIBS, aliqCBS float64
	err := db.QueryRow(`
		SELECT COALESCE(ta.perc_ibs_uf + ta.perc_ibs_mun, 26.5), COALESCE(ta.perc_cbs, 9.9)
		FROM reforma_parametros rp
		LEFT JOIN tabela_aliquotas ta ON ta.ano = rp.target_ano
		WHERE rp.company_id = $1
	`, companyID).Scan(&aliqIBS, &aliqCBS)
	if err == sql.ErrNoRows {
		return 26.5, 9.9, nil
	}
	if err != nil {
		return 0, 0, err
	}
	return aliqIBS, aliqCBS, nil
}

// ---------------------------------------------------------------------------
// CfopAnalysisHandler — GET /api/reforma/modulo2/cfop (RFMC-01)
// ---------------------------------------------------------------------------

func CfopAnalysisHandler(db *sql.DB) http.HandlerFunc {
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
		userID, ok2 := claims["user_id"].(string)
		if !ok2 || userID == "" {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa: "+err.Error())
			return
		}

		aliqIBS, aliqCBS, err := readModulo2Params(db, companyID)
		if err != nil {
			log.Printf("CfopAnalysis parametros error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao ler parâmetros")
			return
		}

		rows, err := db.Query(`
			SELECT
			  CASE
			    WHEN COALESCE(cf.tipo,'O') = 'T' THEN 'Transferência'
			    WHEN LEFT(ne.cfop,2) IN ('12','22','52','62') THEN 'Revenda'
			    WHEN LEFT(ne.cfop,2) IN ('13','23') THEN 'Uso e Consumo'
			    WHEN LEFT(ne.cfop,2) IN ('14','24') THEN 'Ativo Permanente'
			    WHEN LEFT(ne.cfop,1) = '7' THEN 'Exportação'
			    ELSE 'Outras Operações'
			  END AS natureza_cfop,
			  COUNT(*) AS qtd_notas,
			  SUM(ne.v_nf) AS valor_total
			FROM nfe_entradas ne
			LEFT JOIN cfop cf ON cf.cfop = ne.cfop
			WHERE ne.company_id = $1 AND ne.cancelado = 'N'
			GROUP BY natureza_cfop
			ORDER BY valor_total DESC
		`, companyID)
		if err != nil {
			log.Printf("CfopAnalysis query error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar dados")
			return
		}
		defer rows.Close()

		var list []Modulo22Row
		var totalIBS, totalCBS float64

		for rows.Next() {
			var row Modulo22Row
			if err := rows.Scan(&row.NaturezaCFOP, &row.QtdNotas, &row.ValorTotal); err != nil {
				log.Printf("[CfopAnalysis] scan error: %v", err)
				continue
			}
			// Transferências não geram IBS/CBS (regime distinto na transição)
			if row.NaturezaCFOP != "Transferência" {
				row.IBSProjetado = row.ValorTotal * aliqIBS / 100.0
				row.CBSProjetado = row.ValorTotal * aliqCBS / 100.0
				totalIBS += row.IBSProjetado
				totalCBS += row.CBSProjetado
			}
			list = append(list, row)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[CfopAnalysis] rows iteration error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao ler dados")
			return
		}

		if list == nil {
			list = []Modulo22Row{}
		}

		json.NewEncoder(w).Encode(Modulo22Response{
			Rows:       list,
			TotalIBS:   totalIBS,
			TotalCBS:   totalCBS,
			AliqIBSPct: aliqIBS,
			AliqCBSPct: aliqCBS,
		})
	}
}

// ---------------------------------------------------------------------------
// CfopAnalysisCSVHandler — GET /api/reforma/modulo2/cfop/csv
// ---------------------------------------------------------------------------

func CfopAnalysisCSVHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			http.Error(w, "Não autenticado", http.StatusUnauthorized)
			return
		}
		userID, _ := claims["user_id"].(string)
		if userID == "" {
			http.Error(w, "Não autenticado", http.StatusUnauthorized)
			return
		}

		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			http.Error(w, "Erro ao obter empresa: "+err.Error(), http.StatusInternalServerError)
			return
		}

		aliqIBS, aliqCBS, err := readModulo2Params(db, companyID)
		if err != nil {
			log.Printf("CfopAnalysisCSV parametros error: %v", err)
			http.Error(w, "Erro ao ler parâmetros", http.StatusInternalServerError)
			return
		}

		rows, err := db.Query(`
			SELECT
			  CASE
			    WHEN COALESCE(cf.tipo,'O') = 'T' THEN 'Transferência'
			    WHEN LEFT(ne.cfop,2) IN ('12','22','52','62') THEN 'Revenda'
			    WHEN LEFT(ne.cfop,2) IN ('13','23') THEN 'Uso e Consumo'
			    WHEN LEFT(ne.cfop,2) IN ('14','24') THEN 'Ativo Permanente'
			    WHEN LEFT(ne.cfop,1) = '7' THEN 'Exportação'
			    ELSE 'Outras Operações'
			  END AS natureza_cfop,
			  COUNT(*) AS qtd_notas,
			  SUM(ne.v_nf) AS valor_total
			FROM nfe_entradas ne
			LEFT JOIN cfop cf ON cf.cfop = ne.cfop
			WHERE ne.company_id = $1 AND ne.cancelado = 'N'
			GROUP BY natureza_cfop
			ORDER BY valor_total DESC
		`, companyID)
		if err != nil {
			log.Printf("CfopAnalysisCSV query error: %v", err)
			http.Error(w, "Erro ao consultar dados", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var list []Modulo22Row
		for rows.Next() {
			var row Modulo22Row
			if err := rows.Scan(&row.NaturezaCFOP, &row.QtdNotas, &row.ValorTotal); err != nil {
				continue
			}
			if row.NaturezaCFOP != "Transferência" {
				row.IBSProjetado = row.ValorTotal * aliqIBS / 100.0
				row.CBSProjetado = row.ValorTotal * aliqCBS / 100.0
			}
			list = append(list, row)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[CfopAnalysisCSV] rows iteration error: %v", err)
			http.Error(w, "Erro ao ler dados", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="analise-cfop.csv"`)
		cw := csv.NewWriter(w)
		header := []string{"Natureza CFOP", "Qtd Notas", "Valor Total (R$)", "IBS Projetado (R$)", "CBS Projetado (R$)"}
		if err := cw.Write(header); err != nil {
			log.Printf("[CfopAnalysisCSV] write header error: %v", err)
			return
		}

		for _, row := range list {
			record := []string{
				row.NaturezaCFOP,
				fmt.Sprintf("%d", row.QtdNotas),
				fmt.Sprintf("%.2f", row.ValorTotal),
				fmt.Sprintf("%.2f", row.IBSProjetado),
				fmt.Sprintf("%.2f", row.CBSProjetado),
			}
			if err := cw.Write(record); err != nil {
				log.Printf("[CfopAnalysisCSV] write row error: %v", err)
				return
			}
		}

		cw.Flush()
		if err := cw.Error(); err != nil {
			log.Printf("[CfopAnalysisCSV] flush error: %v", err)
		}
	}
}

// ---------------------------------------------------------------------------
// NcmAnalysisHandler — GET /api/reforma/modulo2/ncm (RFMC-02)
// ---------------------------------------------------------------------------

func NcmAnalysisHandler(db *sql.DB) http.HandlerFunc {
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
		userID, ok2 := claims["user_id"].(string)
		if !ok2 || userID == "" {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa: "+err.Error())
			return
		}

		aliqIBS, aliqCBS, err := readModulo2Params(db, companyID)
		if err != nil {
			log.Printf("NcmAnalysis parametros error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao ler parâmetros")
			return
		}

		rows, err := db.Query(`
			SELECT
			  nit.ncm,
			  MAX(COALESCE(nit.x_prod,'')) AS x_prod,
			  SUM(nit.v_prod) AS vl_prod,
			  SUM(nit.v_icms) AS vl_icms,
			  COALESCE(ncmr.ibs_reducao_pct, 0) AS ibs_reducao_pct,
			  COALESCE(ncmr.cbs_reducao_pct, 0) AS cbs_reducao_pct,
			  CASE WHEN ncmr.cclasstrib IS NOT NULL THEN true ELSE false END AS is_flag
			FROM nfe_entradas_itens nit
			JOIN nfe_entradas ne ON ne.id = nit.nfe_id
			LEFT JOIN cfop cf ON cf.cfop = nit.cfop
			LEFT JOIN LATERAL (
			  SELECT ibs_reducao_pct, cbs_reducao_pct, cclasstrib
			  FROM ncm_cclasstrib_reforma
			  WHERE nit.ncm LIKE ncm_digits || '%'
			  ORDER BY length(ncm_digits) DESC
			  LIMIT 1
			) ncmr ON true
			WHERE nit.company_id = $1 AND ne.cancelado = 'N' AND COALESCE(cf.tipo,'O') != 'T'
			GROUP BY nit.ncm, ncmr.ibs_reducao_pct, ncmr.cbs_reducao_pct, ncmr.cclasstrib
			ORDER BY vl_prod DESC
			LIMIT 100
		`, companyID)
		if err != nil {
			log.Printf("NcmAnalysis query error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar dados")
			return
		}
		defer rows.Close()

		var list []Modulo21Row

		for rows.Next() {
			var row Modulo21Row
			var ibsReducao, cbsReducao float64
			if err := rows.Scan(&row.NCM, &row.XProd, &row.VlProd, &row.VlICMS, &ibsReducao, &cbsReducao, &row.IsFlag); err != nil {
				log.Printf("[NcmAnalysis] scan error: %v", err)
				continue
			}
			if row.VlProd > 0 {
				row.AliqICMSEfet = row.VlICMS / row.VlProd * 100.0
			}
			row.IBSProjetado = row.VlProd * aliqIBS / 100.0 * (1 - ibsReducao/100.0)
			row.CBSProjetado = row.VlProd * aliqCBS / 100.0 * (1 - cbsReducao/100.0)
			list = append(list, row)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[NcmAnalysis] rows iteration error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao ler dados")
			return
		}

		if list == nil {
			list = []Modulo21Row{}
		}

		json.NewEncoder(w).Encode(Modulo21Response{
			Rows:       list,
			AliqIBSPct: aliqIBS,
			AliqCBSPct: aliqCBS,
		})
	}
}

// ---------------------------------------------------------------------------
// NcmAnalysisCSVHandler — GET /api/reforma/modulo2/ncm/csv
// ---------------------------------------------------------------------------

func NcmAnalysisCSVHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			http.Error(w, "Não autenticado", http.StatusUnauthorized)
			return
		}
		userID, _ := claims["user_id"].(string)
		if userID == "" {
			http.Error(w, "Não autenticado", http.StatusUnauthorized)
			return
		}

		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			http.Error(w, "Erro ao obter empresa: "+err.Error(), http.StatusInternalServerError)
			return
		}

		aliqIBS, aliqCBS, err := readModulo2Params(db, companyID)
		if err != nil {
			log.Printf("NcmAnalysisCSV parametros error: %v", err)
			http.Error(w, "Erro ao ler parâmetros", http.StatusInternalServerError)
			return
		}

		rows, err := db.Query(`
			SELECT
			  nit.ncm,
			  MAX(COALESCE(nit.x_prod,'')) AS x_prod,
			  SUM(nit.v_prod) AS vl_prod,
			  SUM(nit.v_icms) AS vl_icms,
			  COALESCE(ncmr.ibs_reducao_pct, 0) AS ibs_reducao_pct,
			  COALESCE(ncmr.cbs_reducao_pct, 0) AS cbs_reducao_pct,
			  CASE WHEN ncmr.cclasstrib IS NOT NULL THEN true ELSE false END AS is_flag
			FROM nfe_entradas_itens nit
			JOIN nfe_entradas ne ON ne.id = nit.nfe_id
			LEFT JOIN cfop cf ON cf.cfop = nit.cfop
			LEFT JOIN LATERAL (
			  SELECT ibs_reducao_pct, cbs_reducao_pct, cclasstrib
			  FROM ncm_cclasstrib_reforma
			  WHERE nit.ncm LIKE ncm_digits || '%'
			  ORDER BY length(ncm_digits) DESC
			  LIMIT 1
			) ncmr ON true
			WHERE nit.company_id = $1 AND ne.cancelado = 'N' AND COALESCE(cf.tipo,'O') != 'T'
			GROUP BY nit.ncm, ncmr.ibs_reducao_pct, ncmr.cbs_reducao_pct, ncmr.cclasstrib
			ORDER BY vl_prod DESC
			LIMIT 100
		`, companyID)
		if err != nil {
			log.Printf("NcmAnalysisCSV query error: %v", err)
			http.Error(w, "Erro ao consultar dados", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var list []Modulo21Row
		for rows.Next() {
			var row Modulo21Row
			var ibsReducao, cbsReducao float64
			if err := rows.Scan(&row.NCM, &row.XProd, &row.VlProd, &row.VlICMS, &ibsReducao, &cbsReducao, &row.IsFlag); err != nil {
				continue
			}
			if row.VlProd > 0 {
				row.AliqICMSEfet = row.VlICMS / row.VlProd * 100.0
			}
			row.IBSProjetado = row.VlProd * aliqIBS / 100.0 * (1 - ibsReducao/100.0)
			row.CBSProjetado = row.VlProd * aliqCBS / 100.0 * (1 - cbsReducao/100.0)
			list = append(list, row)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[NcmAnalysisCSV] rows iteration error: %v", err)
			http.Error(w, "Erro ao ler dados", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="analise-ncm.csv"`)
		cw := csv.NewWriter(w)
		header := []string{"NCM", "Descrição", "VL Prod (R$)", "VL ICMS (R$)", "Alíq ICMS Efet (%)", "IBS Proj (R$)", "CBS Proj (R$)", "IS Flag"}
		if err := cw.Write(header); err != nil {
			log.Printf("[NcmAnalysisCSV] write header error: %v", err)
			return
		}

		for _, row := range list {
			isFlag := "Não"
			if row.IsFlag {
				isFlag = "Sim"
			}
			record := []string{
				row.NCM,
				row.XProd,
				fmt.Sprintf("%.2f", row.VlProd),
				fmt.Sprintf("%.2f", row.VlICMS),
				fmt.Sprintf("%.2f", row.AliqICMSEfet),
				fmt.Sprintf("%.2f", row.IBSProjetado),
				fmt.Sprintf("%.2f", row.CBSProjetado),
				isFlag,
			}
			if err := cw.Write(record); err != nil {
				log.Printf("[NcmAnalysisCSV] write row error: %v", err)
				return
			}
		}

		cw.Flush()
		if err := cw.Error(); err != nil {
			log.Printf("[NcmAnalysisCSV] flush error: %v", err)
		}
	}
}

// ---------------------------------------------------------------------------
// UfDestinoHandler — GET /api/reforma/modulo2/uf-destino (RFMC-03)
// ---------------------------------------------------------------------------

func UfDestinoHandler(db *sql.DB) http.HandlerFunc {
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
		userID, ok2 := claims["user_id"].(string)
		if !ok2 || userID == "" {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa: "+err.Error())
			return
		}

		aliqIBS, aliqCBS, err := readModulo2Params(db, companyID)
		if err != nil {
			log.Printf("UfDestino parametros error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao ler parâmetros")
			return
		}

		rows, err := db.Query(`
			SELECT
			  COALESCE(NULLIF(ns.dest_uf,''), 'N/A') AS dest_uf,
			  COUNT(*) AS qtd_notas,
			  SUM(ns.v_nf) AS valor_total,
			  SUM(ns.v_icms) AS vl_icms
			FROM nfe_saidas ns
			LEFT JOIN cfop cf ON cf.cfop = ns.cfop
			WHERE ns.company_id = $1 AND ns.cancelado = 'N' AND COALESCE(cf.tipo,'O') != 'T'
			GROUP BY dest_uf
			ORDER BY valor_total DESC
		`, companyID)
		if err != nil {
			log.Printf("UfDestino query error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar dados")
			return
		}
		defer rows.Close()

		var list []Modulo23Row

		for rows.Next() {
			var row Modulo23Row
			if err := rows.Scan(&row.DestUF, &row.QtdNotas, &row.ValorTotal, &row.VlICMS); err != nil {
				log.Printf("[UfDestino] scan error: %v", err)
				continue
			}
			row.IBSProjetado = row.ValorTotal * aliqIBS / 100.0
			row.CBSProjetado = row.ValorTotal * aliqCBS / 100.0
			list = append(list, row)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[UfDestino] rows iteration error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao ler dados")
			return
		}

		if list == nil {
			list = []Modulo23Row{}
		}

		json.NewEncoder(w).Encode(Modulo23Response{
			Rows:       list,
			AliqIBSPct: aliqIBS,
			AliqCBSPct: aliqCBS,
		})
	}
}

// ---------------------------------------------------------------------------
// B2bB2cHandler — GET /api/reforma/modulo2/b2b-b2c (RFMC-04)
// ---------------------------------------------------------------------------

func B2bB2cHandler(db *sql.DB) http.HandlerFunc {
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
		userID, ok2 := claims["user_id"].(string)
		if !ok2 || userID == "" {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa: "+err.Error())
			return
		}

		aliqIBS, aliqCBS, err := readModulo2Params(db, companyID)
		if err != nil {
			log.Printf("B2bB2c parametros error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao ler parâmetros")
			return
		}

		rows, err := db.Query(`
			SELECT
			  CASE
			    WHEN ns.ind_final = '1' THEN 'b2c'
			    WHEN ns.ind_final = '0' THEN 'b2b_credit'
			    WHEN ns.ind_final IS NULL AND LENGTH(COALESCE(ns.dest_cnpj_cpf,'')) = 11 THEN 'b2c'
			    WHEN ns.ind_final IS NULL AND LENGTH(COALESCE(ns.dest_cnpj_cpf,'')) = 14 THEN 'b2b_credit'
			    ELSE 'sem_classificacao'
			  END AS segmento,
			  COUNT(*) AS qtd_notas,
			  SUM(ns.v_nf) AS valor_total,
			  SUM(CASE WHEN ns.ind_final IS NULL THEN 1 ELSE 0 END) AS qtd_sem_ind_final
			FROM nfe_saidas ns
			LEFT JOIN cfop cf ON cf.cfop = ns.cfop
			WHERE ns.company_id = $1 AND ns.cancelado = 'N' AND COALESCE(cf.tipo,'O') != 'T'
			GROUP BY segmento
			ORDER BY valor_total DESC
		`, companyID)
		if err != nil {
			log.Printf("B2bB2c query error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar dados")
			return
		}
		defer rows.Close()

		var list []Modulo24Row
		var qtdSemIndFinal int

		for rows.Next() {
			var row Modulo24Row
			var rowQtdSemIndFinal int
			if err := rows.Scan(&row.Segmento, &row.QtdNotas, &row.ValorTotal, &rowQtdSemIndFinal); err != nil {
				log.Printf("[B2bB2c] scan error: %v", err)
				continue
			}
			row.IBSProjetado = row.ValorTotal * aliqIBS / 100.0
			row.CBSProjetado = row.ValorTotal * aliqCBS / 100.0
			qtdSemIndFinal += rowQtdSemIndFinal
			list = append(list, row)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[B2bB2c] rows iteration error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao ler dados")
			return
		}

		if list == nil {
			list = []Modulo24Row{}
		}

		json.NewEncoder(w).Encode(Modulo24Response{
			Rows:           list,
			QtdSemIndFinal: qtdSemIndFinal,
			AliqIBSPct:     aliqIBS,
			AliqCBSPct:     aliqCBS,
		})
	}
}
