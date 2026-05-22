package handlers

// ---------------------------------------------------------------------------
// xml_comparativo.go — Comparativo EFD ICMS vs XMLs Importados
//
// GET /api/xml/comparativo/resumo?tipo=saidas|entradas  → ResumoComparativoHandler
// GET /api/xml/comparativo/lacunas?tipo=saidas|entradas[&mes_ano=MM/YYYY] → LacunasHandler
// GET /api/xml/comparativo/modelos                       → ModelosEFDHandler
//
// Todos os handlers usam GetEffectiveCompanyID e parâmetros $N (sem interpolação).
// ---------------------------------------------------------------------------

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// ── Structs de resposta ───────────────────────────────────────────────────────

type comparativoResumoRow struct {
	MesAno      string  `json:"mes_ano"`
	QtdEFD      int     `json:"qtd_efd"`
	QtdXML      int     `json:"qtd_xml"`
	TotalEFD    float64 `json:"total_efd"`
	TotalXML    float64 `json:"total_xml"`
	Diferenca   float64 `json:"diferenca"`
	PctCobertura float64 `json:"pct_cobertura"`
}

type lacunaRow struct {
	MesAno string  `json:"mes_ano"`
	ChvNfe string  `json:"chv_nfe"`
	NumDoc string  `json:"num_doc"`
	DtDoc  string  `json:"dt_doc"`
	CodMod string  `json:"cod_mod"`
	CodSit string  `json:"cod_sit"`
	VlDoc  float64 `json:"vl_doc"`
}

type lacunaMensalRow struct {
	MesAno    string  `json:"mes_ano"`
	QtdFalta  int     `json:"qtd_falta"`
	ValorFalta float64 `json:"valor_falta"`
}

type modeloEFDRow struct {
	CodMod    string  `json:"cod_mod"`
	Descricao string  `json:"descricao"`
	IndOper   string  `json:"ind_oper"`
	Qtd       int     `json:"qtd"`
	Total     float64 `json:"total"`
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func descricaoModelo(cod string) string {
	m := map[string]string{
		"55": "NF-e (XML importável)",
		"65": "NFC-e (XML importável)",
		"01": "NF Modelo 1 (papel)",
		"02": "NF Modelo 2 (papel)",
		"04": "NF Produtor Rural",
		"06": "NF Energia Elétrica",
		"07": "SEFAZ Comunicações",
		"08": "Conhecimento Rodoviário",
		"09": "Conhecimento Aquaviário",
		"10": "Conhecimento Aéreo",
		"11": "Conhecimento Ferroviário",
		"21": "NF-e Combustível",
		"22": "NF-e Simplificada",
		"57": "CT-e (XML importável)",
		"58": "CT-e OS (XML importável)",
		"59": "CF-e SAT",
		"60": "CF-e MFe",
	}
	if d, ok := m[cod]; ok {
		return d
	}
	return "Modelo " + cod
}

func validaTipoComparativo(tipo string) (indOper string, tabelaXML string, ok bool) {
	switch tipo {
	case "saidas":
		return "1", "nfe_saidas", true
	case "entradas":
		return "0", "nfe_entradas", true
	}
	return "", "", false
}

// ── ResumoComparativoHandler ──────────────────────────────────────────────────
// Retorna comparativo mensal: qtd e valor total no EFD (C100 mod 55/65) vs XMLs.

func ResumoComparativoHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "método não permitido", http.StatusMethodNotAllowed)
			return
		}

		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			http.Error(w, "não autorizado", http.StatusUnauthorized)
			return
		}
		userID, _ := claims["user_id"].(string)
		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			http.Error(w, "empresa não encontrada", http.StatusBadRequest)
			return
		}

		tipo := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("tipo")))
		indOper, tabelaXML, valid := validaTipoComparativo(tipo)
		if !valid {
			http.Error(w, "tipo inválido: use saidas ou entradas", http.StatusBadRequest)
			return
		}

		// tabelaXML é validada pela whitelist acima — seguro usar em fmt.Sprintf
		query := fmt.Sprintf(`
WITH efd AS (
  SELECT
    j.mes_ano,
    COUNT(*) FILTER (WHERE c.cod_mod IN ('55','65'))  AS qtd_efd,
    SUM(c.vl_doc) FILTER (WHERE c.cod_mod IN ('55','65')) AS total_efd
  FROM reg_c100 c
  JOIN import_jobs j ON j.id = c.job_id
  WHERE j.company_id = $1
    AND c.ind_oper = $2
    AND j.mes_ano IS NOT NULL
    AND c.cod_sit NOT IN ('02','03','04','05')
  GROUP BY 1
),
xml AS (
  SELECT
    mes_ano,
    COUNT(*)  AS qtd_xml,
    SUM(v_nf) AS total_xml
  FROM %s
  WHERE company_id = $1
    AND mes_ano IS NOT NULL
  GROUP BY 1
)
SELECT
  COALESCE(e.mes_ano, x.mes_ano)       AS mes_ano,
  COALESCE(e.qtd_efd,   0)             AS qtd_efd,
  COALESCE(x.qtd_xml,   0)             AS qtd_xml,
  COALESCE(e.total_efd, 0)             AS total_efd,
  COALESCE(x.total_xml, 0)             AS total_xml,
  COALESCE(e.total_efd, 0) - COALESCE(x.total_xml, 0) AS diferenca,
  CASE
    WHEN COALESCE(e.qtd_efd, 0) = 0 THEN 0
    ELSE ROUND((COALESCE(x.qtd_xml, 0)::numeric / COALESCE(e.qtd_efd, 0)) * 100, 1)
  END AS pct_cobertura
FROM efd e
FULL OUTER JOIN xml x USING (mes_ano)
ORDER BY
  SUBSTRING(COALESCE(e.mes_ano, x.mes_ano), 4, 4),
  SUBSTRING(COALESCE(e.mes_ano, x.mes_ano), 1, 2)
`, tabelaXML)

		rows, err := db.Query(query, companyID, indOper)
		if err != nil {
			log.Printf("ResumoComparativoHandler: %v", err)
			http.Error(w, "erro ao consultar dados", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		result := []comparativoResumoRow{}
		for rows.Next() {
			var row comparativoResumoRow
			if err := rows.Scan(&row.MesAno, &row.QtdEFD, &row.QtdXML,
				&row.TotalEFD, &row.TotalXML, &row.Diferenca, &row.PctCobertura); err != nil {
				log.Printf("ResumoComparativoHandler scan: %v", err)
				continue
			}
			result = append(result, row)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": result,
			"total": len(result),
		})
	}
}

// ── LacunasMensalHandler ──────────────────────────────────────────────────────
// Retorna resumo agregado por mês das lacunas (qtd e valor). Query leve, sem paginação.

func LacunasMensalHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "método não permitido", http.StatusMethodNotAllowed)
			return
		}
		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			http.Error(w, "não autorizado", http.StatusUnauthorized)
			return
		}
		userID, _ := claims["user_id"].(string)
		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			http.Error(w, "empresa não encontrada", http.StatusBadRequest)
			return
		}
		tipo := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("tipo")))
		indOper, tabelaXML, valid := validaTipoComparativo(tipo)
		if !valid {
			http.Error(w, "tipo inválido: use saidas ou entradas", http.StatusBadRequest)
			return
		}

		// CTE isola o alias mes_ano para evitar ambiguidade com coluna homônima do JOIN
		query := fmt.Sprintf(`
WITH lacunas AS (
  SELECT
    j.mes_ano,
    COUNT(*)       AS qtd_falta,
    SUM(c.vl_doc)  AS valor_falta
  FROM reg_c100 c
  JOIN import_jobs j ON j.id = c.job_id
  LEFT JOIN %s x ON x.company_id = $1 AND x.chave_nfe = TRIM(c.chv_nfe)
  WHERE j.company_id = $1
    AND c.ind_oper = $2
    AND c.cod_mod IN ('55', '65')
    AND c.chv_nfe IS NOT NULL AND c.chv_nfe <> ''
    AND j.mes_ano IS NOT NULL
    AND c.cod_sit NOT IN ('02','03','04','05')
    AND x.chave_nfe IS NULL
  GROUP BY 1
)
SELECT mes_ano, qtd_falta, valor_falta
FROM lacunas
WHERE mes_ano IS NOT NULL
ORDER BY SUBSTRING(mes_ano,4,4), SUBSTRING(mes_ano,1,2)
`, tabelaXML)

		rows, err := db.Query(query, companyID, indOper)
		if err != nil {
			log.Printf("LacunasMensalHandler: %v", err)
			http.Error(w, "erro ao consultar dados", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		result := []lacunaMensalRow{}
		for rows.Next() {
			var row lacunaMensalRow
			if err := rows.Scan(&row.MesAno, &row.QtdFalta, &row.ValorFalta); err != nil {
				log.Printf("LacunasMensalHandler scan: %v", err)
				continue
			}
			result = append(result, row)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"items": result, "total": len(result)})
	}
}

// ── LacunasHandler ─────────────────────────────────────────────────────────
// Retorna NF-e/NFC-e de um mês específico presentes no EFD mas ausentes nos XMLs.
// Requer mes_ano para evitar consulta pesada sem filtro.

func LacunasHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "método não permitido", http.StatusMethodNotAllowed)
			return
		}
		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			http.Error(w, "não autorizado", http.StatusUnauthorized)
			return
		}
		userID, _ := claims["user_id"].(string)
		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			http.Error(w, "empresa não encontrada", http.StatusBadRequest)
			return
		}
		tipo := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("tipo")))
		indOper, tabelaXML, valid := validaTipoComparativo(tipo)
		if !valid {
			http.Error(w, "tipo inválido: use saidas ou entradas", http.StatusBadRequest)
			return
		}
		mesAno := strings.TrimSpace(r.URL.Query().Get("mes_ano"))
		if mesAno == "" {
			http.Error(w, "mes_ano obrigatório para detalhe de lacunas", http.StatusBadRequest)
			return
		}

		// LEFT JOIN anti-join (mais eficiente que NOT EXISTS)
		query := fmt.Sprintf(`
SELECT
  j.mes_ano                        AS mes_ano,
  COALESCE(c.chv_nfe, '')         AS chv_nfe,
  COALESCE(c.num_doc, '')         AS num_doc,
  TO_CHAR(c.dt_doc, 'DD/MM/YYYY') AS dt_doc,
  COALESCE(c.cod_mod, '')         AS cod_mod,
  COALESCE(c.cod_sit, '')         AS cod_sit,
  COALESCE(c.vl_doc,  0)          AS vl_doc
FROM reg_c100 c
JOIN import_jobs j ON j.id = c.job_id
LEFT JOIN %s x ON x.company_id = $1 AND x.chave_nfe = TRIM(c.chv_nfe)
WHERE j.company_id = $1
  AND c.ind_oper = $2
  AND c.cod_mod IN ('55', '65')
  AND c.chv_nfe IS NOT NULL AND c.chv_nfe <> ''
  AND j.mes_ano IS NOT NULL
  AND c.cod_sit NOT IN ('02','03','04','05')
  AND j.mes_ano = $3
  AND x.chave_nfe IS NULL
ORDER BY c.vl_doc DESC
LIMIT 500
`, tabelaXML)

		rows, err := db.Query(query, companyID, indOper, mesAno)
		if err != nil {
			log.Printf("LacunasHandler: %v", err)
			http.Error(w, "erro ao consultar dados", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		result := []lacunaRow{}
		for rows.Next() {
			var row lacunaRow
			if err := rows.Scan(&row.MesAno, &row.ChvNfe, &row.NumDoc,
				&row.DtDoc, &row.CodMod, &row.CodSit, &row.VlDoc); err != nil {
				log.Printf("LacunasHandler scan: %v", err)
				continue
			}
			result = append(result, row)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"items": result, "total": len(result)})
	}
}

// ── LacunasExportHandler ──────────────────────────────────────────────────────
// GET /api/xml/comparativo/lacunas/export?tipo=saidas|entradas[&mes_ano=MM/YYYY]
// Retorna TODAS as lacunas (sem limite de 500) com campos extras para planilha Excel.
// O frontend converte o JSON para XLSX usando a lib xlsx.

type lacunaExportRow struct {
	MesAno     string  `json:"mes_ano"`
	FilialCNPJ string  `json:"filial_cnpj"`
	CodPart    string  `json:"cod_part"`
	Ser        string  `json:"ser"`
	NumDoc     string  `json:"num_doc"`
	ChvNfe     string  `json:"chv_nfe"`
	DtDoc      string  `json:"dt_doc"`
	DtES       string  `json:"dt_e_s"`
	CodMod     string  `json:"cod_mod"`
	CodSit     string  `json:"cod_sit"`
	CFOPs      string  `json:"cfops"`
	VlDoc      float64 `json:"vl_doc"`
	VlICMS     float64 `json:"vl_icms"`
	VlBCICMS   float64 `json:"vl_bc_icms"`
	VlPIS      float64 `json:"vl_pis"`
	VlCOFINS   float64 `json:"vl_cofins"`
}

func LacunasExportHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "método não permitido", http.StatusMethodNotAllowed)
			return
		}
		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			http.Error(w, "não autorizado", http.StatusUnauthorized)
			return
		}
		userID, _ := claims["user_id"].(string)
		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			http.Error(w, "empresa não encontrada", http.StatusBadRequest)
			return
		}

		tipo := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("tipo")))
		indOper, tabelaXML, valid := validaTipoComparativo(tipo)
		if !valid {
			http.Error(w, "tipo inválido: use saidas ou entradas", http.StatusBadRequest)
			return
		}

		mesAno := strings.TrimSpace(r.URL.Query().Get("mes_ano"))
		args := []interface{}{companyID, indOper}
		mesAnoFilter := ""
		if mesAno != "" {
			args = append(args, mesAno)
			mesAnoFilter = fmt.Sprintf("AND j.mes_ano = $%d", len(args))
		}

		query := fmt.Sprintf(`
SELECT
  j.mes_ano                                                        AS mes_ano,
  COALESCE(c.filial_cnpj, '')                                     AS filial_cnpj,
  COALESCE(c.cod_part, '')                                         AS cod_part,
  COALESCE(c.ser, '')                                              AS ser,
  COALESCE(c.num_doc, '')                                          AS num_doc,
  COALESCE(c.chv_nfe, '')                                          AS chv_nfe,
  TO_CHAR(c.dt_doc, 'DD/MM/YYYY')                                  AS dt_doc,
  COALESCE(TO_CHAR(c.dt_e_s, 'DD/MM/YYYY'), '')                   AS dt_e_s,
  COALESCE(c.cod_mod, '')                                           AS cod_mod,
  COALESCE(c.cod_sit, '')                                           AS cod_sit,
  COALESCE(STRING_AGG(DISTINCT ci.cfop, '/'), '')                  AS cfops,
  COALESCE(c.vl_doc,    0)                                         AS vl_doc,
  COALESCE(c.vl_icms,   0)                                         AS vl_icms,
  COALESCE(SUM(ci.vl_bc_icms), 0)                                  AS vl_bc_icms,
  COALESCE(c.vl_pis,    0)                                         AS vl_pis,
  COALESCE(c.vl_cofins, 0)                                         AS vl_cofins
FROM reg_c100 c
JOIN import_jobs j ON j.id = c.job_id
LEFT JOIN %s x ON x.company_id = $1 AND x.chave_nfe = TRIM(c.chv_nfe)
LEFT JOIN reg_c190 ci ON ci.id_pai_c100 = c.id
WHERE j.company_id = $1
  AND c.ind_oper = $2
  AND c.cod_mod IN ('55','65')
  AND c.chv_nfe IS NOT NULL AND c.chv_nfe <> ''
  AND j.mes_ano IS NOT NULL
  AND c.cod_sit NOT IN ('02','03','04','05')
  AND x.chave_nfe IS NULL
  %s
GROUP BY
  j.mes_ano, c.id, c.filial_cnpj, c.cod_part, c.ser, c.num_doc, c.chv_nfe,
  c.dt_doc, c.dt_e_s, c.cod_mod, c.cod_sit, c.vl_doc, c.vl_icms,
  c.vl_pis, c.vl_cofins
ORDER BY j.mes_ano, c.dt_doc, c.vl_doc DESC
`, tabelaXML, mesAnoFilter)

		rows, err := db.Query(query, args...)
		if err != nil {
			log.Printf("LacunasExportHandler: %v", err)
			http.Error(w, "erro ao consultar dados", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		result := []lacunaExportRow{}
		for rows.Next() {
			var row lacunaExportRow
			if err := rows.Scan(
				&row.MesAno, &row.FilialCNPJ, &row.CodPart, &row.Ser, &row.NumDoc,
				&row.ChvNfe, &row.DtDoc, &row.DtES, &row.CodMod, &row.CodSit,
				&row.CFOPs, &row.VlDoc, &row.VlICMS, &row.VlBCICMS, &row.VlPIS, &row.VlCOFINS,
			); err != nil {
				log.Printf("LacunasExportHandler scan: %v", err)
				continue
			}
			result = append(result, row)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": result,
			"total": len(result),
		})
	}
}

// ── ModelosEFDHandler ─────────────────────────────────────────────────────────
// Retorna breakdown dos modelos de documento no EFD (para ambas direções).

func ModelosEFDHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "método não permitido", http.StatusMethodNotAllowed)
			return
		}

		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			http.Error(w, "não autorizado", http.StatusUnauthorized)
			return
		}
		userID, _ := claims["user_id"].(string)
		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			http.Error(w, "empresa não encontrada", http.StatusBadRequest)
			return
		}

		query := `
SELECT
  COALESCE(c.cod_mod, '??') AS cod_mod,
  c.ind_oper,
  COUNT(*)       AS qtd,
  SUM(c.vl_doc)  AS total
FROM reg_c100 c
JOIN import_jobs j ON j.id = c.job_id
WHERE j.company_id = $1
GROUP BY c.cod_mod, c.ind_oper
ORDER BY c.ind_oper, SUM(c.vl_doc) DESC
`

		rows, err := db.Query(query, companyID)
		if err != nil {
			log.Printf("ModelosEFDHandler: %v", err)
			http.Error(w, "erro ao consultar dados", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		result := []modeloEFDRow{}
		for rows.Next() {
			var row modeloEFDRow
			if err := rows.Scan(&row.CodMod, &row.IndOper, &row.Qtd, &row.Total); err != nil {
				log.Printf("ModelosEFDHandler scan: %v", err)
				continue
			}
			row.Descricao = descricaoModelo(row.CodMod)
			result = append(result, row)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": result,
			"total": len(result),
		})
	}
}
