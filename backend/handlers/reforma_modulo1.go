package handlers

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/golang-jwt/jwt/v5"
)

// ---------------------------------------------------------------------------
// Structs — Módulo 1.1: Créditos ICMS Bloqueados
// ---------------------------------------------------------------------------

// TipoBloqueio classifica o crédito pelo mecanismo que o bloqueia na transição.
// Valores: "ICMS-ST" (substituição tributária) | "Diferido" (CST 51)
type Modulo11Row struct {
	TipoBloqueio string  `json:"tipo_bloqueio"`
	TipoCFOP     string  `json:"tipo_cfop"`
	CFOP         string  `json:"cfop"`
	VlBloqueado  float64 `json:"vl_bloqueado"`
	VlOprTotal   float64 `json:"vl_opr_total"`
	IBSEquiv     float64 `json:"ibs_equiv"`
	CBSEquiv     float64 `json:"cbs_equiv"`
	QtdRegistros int     `json:"qtd_registros"`
}

type Modulo11Response struct {
	Rows           []Modulo11Row `json:"rows"`
	TotalBloqueado float64       `json:"total_bloqueado"`
	TotalIBS       float64       `json:"total_ibs"`
	TotalCBS       float64       `json:"total_cbs"`
}

// ---------------------------------------------------------------------------
// Structs — Módulo 1.3: Ranking Fornecedores Simples Nacional
// ---------------------------------------------------------------------------

type Modulo13Row struct {
	FornCNPJ      string  `json:"forn_cnpj"`
	FornNome      string  `json:"forn_nome"`
	QtdNotas      int     `json:"qtd_notas"`
	ValorTotal    float64 `json:"valor_total"`
	IBSPerdidoEst float64 `json:"ibs_perdido_est"`
	CBSPerdidoEst float64 `json:"cbs_perdido_est"`
	Simples       bool    `json:"simples"`
}

type Modulo13Response struct {
	Rows            []Modulo13Row `json:"rows"`
	FatorSimplesPct float64       `json:"fator_simples_pct"`
}

// ---------------------------------------------------------------------------
// Structs — Módulo 1.2: Reprecificação por Produto
// ---------------------------------------------------------------------------

type Modulo12Row struct {
	NCM            string  `json:"ncm"`
	XProd          string  `json:"x_prod"`
	CSTICMS        string  `json:"cst_icms"`
	CSTPath        string  `json:"cst_path"`
	PrecoAtual     float64 `json:"preco_atual"`
	IcmsAtual      float64 `json:"icms_atual"`
	IBSProjetado   float64 `json:"ibs_projetado"`
	CBSProjetado   float64 `json:"cbs_projetado"`
	PrecoSugerido  float64 `json:"preco_sugerido"`  // preco_atual + (ibs + cbs − icms_atual)
	VariacaoPct    float64 `json:"variacao_pct"`
}

type Modulo12Response struct {
	Rows            []Modulo12Row `json:"rows"`
	AliqIBSPct      float64       `json:"aliq_ibs_pct"`
	AliqCBSPct      float64       `json:"aliq_cbs_pct"`
	Ano             int           `json:"ano"`               // ano-base usado nas projeções
	AnosDisponiveis []int         `json:"anos_disponiveis"`  // anos da tabela_aliquotas
}

// ---------------------------------------------------------------------------
// Structs — Módulo 1.4: Split Payment
// ---------------------------------------------------------------------------

type SensibilidadeRow struct {
	DSO    int       `json:"dso"`
	Custos []float64 `json:"custos"`
}

type Modulo14Response struct {
	FloatTributario  float64            `json:"float_tributario"`
	CustoCDI         float64            `json:"custo_cdi"`
	TotalSaidas      float64            `json:"total_saidas"`
	AliqTotal        float64            `json:"aliq_total"`
	TaxaCDIAnualPct  float64            `json:"taxa_cdi_anual_pct"`
	PrazoMedioDias   int                `json:"prazo_medio_dias"`
	CDIColunas       []float64          `json:"cdi_colunas"`
	DSOLinhas        []int              `json:"dso_linhas"`
	Sensibilidade    []SensibilidadeRow `json:"sensibilidade"`
}

// ---------------------------------------------------------------------------
// CreditosBloqueadosHandler — GET /api/reforma/modulo1/creditos (RFMB-01)
// ---------------------------------------------------------------------------

func CreditosBloqueadosHandler(db *sql.DB) http.HandlerFunc {
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

		var aliqIBS, aliqCBS float64
		err = db.QueryRow(`
			SELECT COALESCE(ta.perc_ibs_uf + ta.perc_ibs_mun, 17.7), COALESCE(ta.perc_cbs, 8.8)
			FROM reforma_parametros rp
			LEFT JOIN tabela_aliquotas ta ON ta.ano = rp.target_ano
			WHERE rp.company_id = $1
		`, companyID).Scan(&aliqIBS, &aliqCBS)
		if err == sql.ErrNoRows {
			aliqIBS, aliqCBS = 17.7, 8.8
		} else if err != nil {
			log.Printf("CreditosBloqueados parametros error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao ler parâmetros")
			return
		}

		// Critérios de bloqueio na transição (EC 132/2023):
		//   ICMS-ST: pago antecipadamente nas entradas, sem mecanismo de devolução no IBS/CBS
		//   Diferido: CST 51 — créditos escriturais suspensos/diferidos que não serão compensados
		//   CIAP: não implementado (requer importação do Bloco G — futuro)
		rows, err := db.Query(`
			SELECT tipo_bloqueio, tipo_cfop, cfop, vl_bloqueado, vl_opr_total, qtd_registros
			FROM (
				SELECT
					'ICMS-ST'                       AS tipo_bloqueio,
					COALESCE(cf.tipo, 'O')          AS tipo_cfop,
					c190.cfop,
					SUM(c190.vl_icms_st)            AS vl_bloqueado,
					SUM(c190.vl_opr)                AS vl_opr_total,
					COUNT(DISTINCT c100.id)         AS qtd_registros
				FROM reg_c190 c190
				JOIN reg_c100 c100 ON c100.id = c190.id_pai_c100
				JOIN import_jobs j ON j.id = c100.job_id
				LEFT JOIN cfop cf ON cf.cfop = c190.cfop
				WHERE j.company_id = $1
				  AND c100.ind_oper = '0'
				  AND c100.cod_sit NOT IN ('02','03','04','05')
				  AND COALESCE(cf.tipo, 'O') != 'T'
				  AND c190.vl_icms_st > 0
				GROUP BY cf.tipo, c190.cfop

				UNION ALL

				SELECT
					'Diferido'                      AS tipo_bloqueio,
					COALESCE(cf.tipo, 'O')          AS tipo_cfop,
					c190.cfop,
					SUM(c190.vl_icms)               AS vl_bloqueado,
					SUM(c190.vl_opr)                AS vl_opr_total,
					COUNT(DISTINCT c100.id)         AS qtd_registros
				FROM reg_c190 c190
				JOIN reg_c100 c100 ON c100.id = c190.id_pai_c100
				JOIN import_jobs j ON j.id = c100.job_id
				LEFT JOIN cfop cf ON cf.cfop = c190.cfop
				WHERE j.company_id = $1
				  AND c100.ind_oper = '0'
				  AND c100.cod_sit NOT IN ('02','03','04','05')
				  AND COALESCE(cf.tipo, 'O') != 'T'
				  AND c190.cst_icms = '51'
				GROUP BY cf.tipo, c190.cfop
			) t
			ORDER BY tipo_bloqueio, vl_bloqueado DESC
		`, companyID)
		if err != nil {
			log.Printf("CreditosBloqueados query error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar dados")
			return
		}
		defer rows.Close()

		var list []Modulo11Row
		var totalBloqueado, totalIBS, totalCBS float64

		for rows.Next() {
			var row Modulo11Row
			if err := rows.Scan(&row.TipoBloqueio, &row.TipoCFOP, &row.CFOP, &row.VlBloqueado, &row.VlOprTotal, &row.QtdRegistros); err != nil {
				log.Printf("[CreditosBloqueados] scan error: %v", err)
				continue
			}
			row.IBSEquiv = row.VlOprTotal * aliqIBS / 100.0
			row.CBSEquiv = row.VlOprTotal * aliqCBS / 100.0
			totalBloqueado += row.VlBloqueado
			totalIBS += row.IBSEquiv
			totalCBS += row.CBSEquiv
			list = append(list, row)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[CreditosBloqueados] rows iteration error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao ler dados")
			return
		}

		if list == nil {
			list = []Modulo11Row{}
		}

		json.NewEncoder(w).Encode(Modulo11Response{
			Rows:           list,
			TotalBloqueado: totalBloqueado,
			TotalIBS:       totalIBS,
			TotalCBS:       totalCBS,
		})
	}
}

// ---------------------------------------------------------------------------
// CreditosBloqueadosCSVHandler — GET /api/reforma/modulo1/creditos/csv
// ---------------------------------------------------------------------------

func CreditosBloqueadosCSVHandler(db *sql.DB) http.HandlerFunc {
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

		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			http.Error(w, "Erro ao obter empresa: "+err.Error(), http.StatusInternalServerError)
			return
		}

		var aliqIBS, aliqCBS float64
		err = db.QueryRow(`
			SELECT COALESCE(ta.perc_ibs_uf + ta.perc_ibs_mun, 17.7), COALESCE(ta.perc_cbs, 8.8)
			FROM reforma_parametros rp
			LEFT JOIN tabela_aliquotas ta ON ta.ano = rp.target_ano
			WHERE rp.company_id = $1
		`, companyID).Scan(&aliqIBS, &aliqCBS)
		if err == sql.ErrNoRows {
			aliqIBS, aliqCBS = 17.7, 8.8
		} else if err != nil {
			log.Printf("CreditosBloqueadosCSV parametros error: %v", err)
			http.Error(w, "Erro ao ler parâmetros", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="creditos-icms-bloqueados.csv"`)

		rows, err := db.Query(`
			SELECT tipo_bloqueio, tipo_cfop, cfop, vl_bloqueado, vl_opr_total, qtd_registros
			FROM (
				SELECT
					'ICMS-ST'                       AS tipo_bloqueio,
					COALESCE(cf.tipo, 'O')          AS tipo_cfop,
					c190.cfop,
					SUM(c190.vl_icms_st)            AS vl_bloqueado,
					SUM(c190.vl_opr)                AS vl_opr_total,
					COUNT(DISTINCT c100.id)         AS qtd_registros
				FROM reg_c190 c190
				JOIN reg_c100 c100 ON c100.id = c190.id_pai_c100
				JOIN import_jobs j ON j.id = c100.job_id
				LEFT JOIN cfop cf ON cf.cfop = c190.cfop
				WHERE j.company_id = $1
				  AND c100.ind_oper = '0'
				  AND c100.cod_sit NOT IN ('02','03','04','05')
				  AND COALESCE(cf.tipo, 'O') != 'T'
				  AND c190.vl_icms_st > 0
				GROUP BY cf.tipo, c190.cfop

				UNION ALL

				SELECT
					'Diferido'                      AS tipo_bloqueio,
					COALESCE(cf.tipo, 'O')          AS tipo_cfop,
					c190.cfop,
					SUM(c190.vl_icms)               AS vl_bloqueado,
					SUM(c190.vl_opr)                AS vl_opr_total,
					COUNT(DISTINCT c100.id)         AS qtd_registros
				FROM reg_c190 c190
				JOIN reg_c100 c100 ON c100.id = c190.id_pai_c100
				JOIN import_jobs j ON j.id = c100.job_id
				LEFT JOIN cfop cf ON cf.cfop = c190.cfop
				WHERE j.company_id = $1
				  AND c100.ind_oper = '0'
				  AND c100.cod_sit NOT IN ('02','03','04','05')
				  AND COALESCE(cf.tipo, 'O') != 'T'
				  AND c190.cst_icms = '51'
				GROUP BY cf.tipo, c190.cfop
			) t
			ORDER BY tipo_bloqueio, vl_bloqueado DESC
		`, companyID)
		if err != nil {
			log.Printf("CreditosBloqueadosCSV query error: %v", err)
			http.Error(w, "Erro ao consultar dados", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var list []Modulo11Row
		for rows.Next() {
			var row Modulo11Row
			if err := rows.Scan(&row.TipoBloqueio, &row.TipoCFOP, &row.CFOP, &row.VlBloqueado, &row.VlOprTotal, &row.QtdRegistros); err != nil {
				continue
			}
			row.IBSEquiv = row.VlOprTotal * aliqIBS / 100.0
			row.CBSEquiv = row.VlOprTotal * aliqCBS / 100.0
			list = append(list, row)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[CreditosBloqueadosCSV] rows iteration error: %v", err)
			http.Error(w, "Erro ao ler dados", http.StatusInternalServerError)
			return
		}

		cw := csv.NewWriter(w)
		header := []string{"Tipo de Crédito", "Tipo CFOP", "CFOP", "Valor Bloqueado (R$)", "VL Operações (R$)", "IBS Equiv. (R$)", "CBS Equiv. (R$)", "Qtd Registros"}
		if err := cw.Write(header); err != nil {
			log.Printf("[CreditosCSV] write header error: %v", err)
			return
		}

		for _, row := range list {
			record := []string{
				row.TipoBloqueio,
				row.TipoCFOP,
				row.CFOP,
				fmt.Sprintf("%.2f", row.VlBloqueado),
				fmt.Sprintf("%.2f", row.VlOprTotal),
				fmt.Sprintf("%.2f", row.IBSEquiv),
				fmt.Sprintf("%.2f", row.CBSEquiv),
				fmt.Sprintf("%d", row.QtdRegistros),
			}
			if err := cw.Write(record); err != nil {
				log.Printf("[CreditosCSV] write row error: %v", err)
				return
			}
		}

		cw.Flush()
		if err := cw.Error(); err != nil {
			log.Printf("[CreditosCSV] flush error: %v", err)
		}
	}
}

// ---------------------------------------------------------------------------
// RankingFornecedoresHandler — GET /api/reforma/modulo1/ranking (RFMB-02)
// ---------------------------------------------------------------------------

func RankingFornecedoresHandler(db *sql.DB) http.HandlerFunc {
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

		var fatorSimples, aliqIBS, aliqCBS float64
		err = db.QueryRow(`
			SELECT rp.fator_simples_pct,
			       COALESCE(ta.perc_ibs_uf + ta.perc_ibs_mun, 17.7),
			       COALESCE(ta.perc_cbs, 8.8)
			FROM reforma_parametros rp
			LEFT JOIN tabela_aliquotas ta ON ta.ano = rp.target_ano
			WHERE rp.company_id = $1
		`, companyID).Scan(&fatorSimples, &aliqIBS, &aliqCBS)
		if err == sql.ErrNoRows {
			fatorSimples, aliqIBS, aliqCBS = 20.0, 17.7, 8.8
		} else if err != nil {
			log.Printf("RankingFornecedores parametros error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao ler parâmetros")
			return
		}

		// Fonte: nfe_entradas (XML) — JOIN direto com forn_simples via forn_cnpj (14 dígitos puros)
		// filtro: cancelado = 'N', CFOP cabeçalho != transferência
		rows, err := db.Query(`
			SELECT
				ne.forn_cnpj,
				COALESCE(ne.forn_nome, '')   AS forn_nome,
				COUNT(*)                     AS qtd_notas,
				SUM(ne.v_nf)                 AS valor_total
			FROM nfe_entradas ne
			JOIN forn_simples fs ON fs.cnpj = ne.forn_cnpj
			LEFT JOIN cfop cf ON cf.cfop = ne.cfop
			WHERE ne.company_id = $1
			  AND ne.cancelado = 'N'
			  AND COALESCE(cf.tipo, 'O') != 'T'
			GROUP BY ne.forn_cnpj, ne.forn_nome
			ORDER BY valor_total DESC
			LIMIT 100
		`, companyID)
		if err != nil {
			log.Printf("RankingFornecedores query error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar dados")
			return
		}
		defer rows.Close()

		var list []Modulo13Row

		for rows.Next() {
			var row Modulo13Row
			if err := rows.Scan(&row.FornCNPJ, &row.FornNome, &row.QtdNotas, &row.ValorTotal); err != nil {
				log.Printf("[RankingFornecedores] scan error: %v", err)
				continue
			}
			row.Simples = true
			// fator_simples representa a fração de crédito não aproveitável de fornecedor Simples
			row.IBSPerdidoEst = row.ValorTotal * fatorSimples / 100.0 * aliqIBS / 100.0
			row.CBSPerdidoEst = row.ValorTotal * fatorSimples / 100.0 * aliqCBS / 100.0
			list = append(list, row)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[RankingFornecedores] rows iteration error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao ler dados")
			return
		}

		if list == nil {
			list = []Modulo13Row{}
		}

		json.NewEncoder(w).Encode(Modulo13Response{
			Rows:            list,
			FatorSimplesPct: fatorSimples,
		})
	}
}

// ---------------------------------------------------------------------------
// RankingFornecedoresCSVHandler — GET /api/reforma/modulo1/ranking/csv
// ---------------------------------------------------------------------------

func RankingFornecedoresCSVHandler(db *sql.DB) http.HandlerFunc {
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

		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			http.Error(w, "Erro ao obter empresa: "+err.Error(), http.StatusInternalServerError)
			return
		}

		var fatorSimples, aliqIBS, aliqCBS float64
		err = db.QueryRow(`
			SELECT rp.fator_simples_pct,
			       COALESCE(ta.perc_ibs_uf + ta.perc_ibs_mun, 17.7),
			       COALESCE(ta.perc_cbs, 8.8)
			FROM reforma_parametros rp
			LEFT JOIN tabela_aliquotas ta ON ta.ano = rp.target_ano
			WHERE rp.company_id = $1
		`, companyID).Scan(&fatorSimples, &aliqIBS, &aliqCBS)
		if err == sql.ErrNoRows {
			fatorSimples, aliqIBS, aliqCBS = 20.0, 17.7, 8.8
		} else if err != nil {
			log.Printf("RankingFornecedoresCSV parametros error: %v", err)
			http.Error(w, "Erro ao ler parâmetros", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="ranking-fornecedores-simples.csv"`)

		rows, err := db.Query(`
			SELECT
				ne.forn_cnpj,
				COALESCE(ne.forn_nome, '')   AS forn_nome,
				COUNT(*)                     AS qtd_notas,
				SUM(ne.v_nf)                 AS valor_total
			FROM nfe_entradas ne
			JOIN forn_simples fs ON fs.cnpj = ne.forn_cnpj
			LEFT JOIN cfop cf ON cf.cfop = ne.cfop
			WHERE ne.company_id = $1
			  AND ne.cancelado = 'N'
			  AND COALESCE(cf.tipo, 'O') != 'T'
			GROUP BY ne.forn_cnpj, ne.forn_nome
			ORDER BY valor_total DESC
			LIMIT 100
		`, companyID)
		if err != nil {
			log.Printf("RankingFornecedoresCSV query error: %v", err)
			http.Error(w, "Erro ao consultar dados", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var list []Modulo13Row
		for rows.Next() {
			var row Modulo13Row
			if err := rows.Scan(&row.FornCNPJ, &row.FornNome, &row.QtdNotas, &row.ValorTotal); err != nil {
				continue
			}
			row.Simples = true
			row.IBSPerdidoEst = row.ValorTotal * fatorSimples / 100.0 * aliqIBS / 100.0
			row.CBSPerdidoEst = row.ValorTotal * fatorSimples / 100.0 * aliqCBS / 100.0
			list = append(list, row)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[RankingFornecedoresCSV] rows iteration error: %v", err)
			http.Error(w, "Erro ao ler dados", http.StatusInternalServerError)
			return
		}

		cw := csv.NewWriter(w)
		header := []string{"CNPJ", "Fornecedor", "Qtd Notas", "Valor Total (R$)", "IBS Estimado (R$)", "CBS Estimado (R$)", "Simples Nacional"}
		if err := cw.Write(header); err != nil {
			log.Printf("[RankingCSV] write header error: %v", err)
			return
		}

		for _, row := range list {
			simples := "Sim"
			record := []string{
				row.FornCNPJ,
				row.FornNome,
				fmt.Sprintf("%d", row.QtdNotas),
				fmt.Sprintf("%.2f", row.ValorTotal),
				fmt.Sprintf("%.2f", row.IBSPerdidoEst),
				fmt.Sprintf("%.2f", row.CBSPerdidoEst),
				simples,
			}
			if err := cw.Write(record); err != nil {
				log.Printf("[RankingCSV] write row error: %v", err)
				return
			}
		}

		cw.Flush()
		if err := cw.Error(); err != nil {
			log.Printf("[RankingCSV] flush error: %v", err)
		}
	}
}

// ---------------------------------------------------------------------------
// ReprecificacaoHandler — GET /api/reforma/modulo1/reprecificacao (RFMB-03)
// ---------------------------------------------------------------------------

func ReprecificacaoHandler(db *sql.DB) http.HandlerFunc {
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

		// Lista de anos disponíveis na tabela_aliquotas (curva de transição).
		anosDisponiveis := []int{}
		if anoRows, err := db.Query(`SELECT ano FROM tabela_aliquotas ORDER BY ano`); err == nil {
			for anoRows.Next() {
				var a int
				if anoRows.Scan(&a) == nil {
					anosDisponiveis = append(anosDisponiveis, a)
				}
			}
			anoRows.Close()
		}

		// Resolução do ano-base:
		//   1) query string ?ano=NNNN (se válido na tabela_aliquotas)
		//   2) target_ano configurado em reforma_parametros
		//   3) último ano da tabela_aliquotas (carga máxima)
		anoSolicitado, _ := strconv.Atoi(r.URL.Query().Get("ano"))
		ano := 0
		if anoSolicitado > 0 {
			for _, a := range anosDisponiveis {
				if a == anoSolicitado {
					ano = a
					break
				}
			}
		}
		if ano == 0 {
			_ = db.QueryRow(`SELECT target_ano FROM reforma_parametros WHERE company_id=$1`, companyID).Scan(&ano)
		}
		if ano == 0 && len(anosDisponiveis) > 0 {
			ano = anosDisponiveis[len(anosDisponiveis)-1]
		}

		var aliqIBS, aliqCBS float64
		err = db.QueryRow(`
			SELECT COALESCE(perc_ibs_uf + perc_ibs_mun, 17.7), COALESCE(perc_cbs, 8.8)
			FROM tabela_aliquotas WHERE ano = $1
		`, ano).Scan(&aliqIBS, &aliqCBS)
		if err == sql.ErrNoRows {
			aliqIBS, aliqCBS = 17.7, 8.8
		} else if err != nil {
			log.Printf("Reprecificacao parametros error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao ler parâmetros")
			return
		}

		// LATERAL join NCM longest-prefix-wins
		rows, err := db.Query(`
			SELECT
				COALESCE(nit.ncm, '')                        AS ncm,
				COALESCE(nit.x_prod, '')                     AS x_prod,
				COALESCE(nit.cst_icms, '')                   AS cst_icms,
				nit.v_prod,
				COALESCE(nit.v_icms, 0)                      AS v_icms,
				COALESCE(ncmr.ibs_reducao_pct, 0)            AS ibs_reducao_pct,
				COALESCE(ncmr.cbs_reducao_pct, 0)            AS cbs_reducao_pct
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
			WHERE nit.company_id = $1
			  AND ne.cancelado = 'N'
			  AND COALESCE(cf.tipo, 'O') != 'T'
			ORDER BY nit.v_prod DESC
			LIMIT 500
		`, companyID)
		if err != nil {
			log.Printf("Reprecificacao query error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar dados")
			return
		}
		defer rows.Close()

		var list []Modulo12Row

		for rows.Next() {
			var ncm, xprod, cstIcms string
			var vProd, vIcms, ibsReducao, cbsReducao float64
			if err := rows.Scan(&ncm, &xprod, &cstIcms, &vProd, &vIcms, &ibsReducao, &cbsReducao); err != nil {
				continue
			}

			// Classificar CST em três caminhos (decisão A2 — interpretação documentada)
			cstPath := "outro"
			switch cstIcms {
			case "00", "":
				cstPath = "normal"
			case "10", "30", "60", "70":
				cstPath = "st"
			case "20":
				cstPath = "base_reduzida"
			}

			// Cálculo de reprecificação (interpretação A2 documentada)
			ibsProjetado := vProd * aliqIBS / 100.0 * (1 - ibsReducao/100.0)
			cbsProjetado := vProd * aliqCBS / 100.0 * (1 - cbsReducao/100.0)

			// Preço sugerido = preço atual + delta de carga tributária.
			// Mantém receita líquida do vendedor (interpretação A2): se IBS+CBS
			// projetado > ICMS atual, o preço sobe na mesma proporção; se for
			// menor, o preço desce.
			precoSugerido := vProd + (ibsProjetado + cbsProjetado - vIcms)
			if precoSugerido < 0 {
				precoSugerido = 0
			}

			variacaoPct := 0.0
			if vProd > 0 {
				variacaoPct = (ibsProjetado + cbsProjetado - vIcms) / vProd * 100.0
			}

			list = append(list, Modulo12Row{
				NCM:           ncm,
				XProd:         xprod,
				CSTICMS:       cstIcms,
				CSTPath:       cstPath,
				PrecoAtual:    vProd,
				IcmsAtual:     vIcms,
				IBSProjetado:  ibsProjetado,
				CBSProjetado:  cbsProjetado,
				PrecoSugerido: precoSugerido,
				VariacaoPct:   variacaoPct,
			})
		}
		if err := rows.Err(); err != nil {
			log.Printf("[Reprecificacao] rows iteration error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao ler dados")
			return
		}

		if list == nil {
			list = []Modulo12Row{}
		}

		json.NewEncoder(w).Encode(Modulo12Response{
			Rows:            list,
			AliqIBSPct:      aliqIBS,
			AliqCBSPct:      aliqCBS,
			Ano:             ano,
			AnosDisponiveis: anosDisponiveis,
		})
	}
}

// ---------------------------------------------------------------------------
// ReprecificacaoCSVHandler — GET /api/reforma/modulo1/reprecificacao/csv
// ---------------------------------------------------------------------------

func ReprecificacaoCSVHandler(db *sql.DB) http.HandlerFunc {
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

		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			http.Error(w, "Erro ao obter empresa: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Ano-base — query string ?ano=NNNN, ou target_ano, ou último da tabela
		anoSolicitado, _ := strconv.Atoi(r.URL.Query().Get("ano"))
		ano := 0
		if anoSolicitado > 0 {
			var exists bool
			_ = db.QueryRow(`SELECT EXISTS(SELECT 1 FROM tabela_aliquotas WHERE ano=$1)`, anoSolicitado).Scan(&exists)
			if exists {
				ano = anoSolicitado
			}
		}
		if ano == 0 {
			_ = db.QueryRow(`SELECT target_ano FROM reforma_parametros WHERE company_id=$1`, companyID).Scan(&ano)
		}
		if ano == 0 {
			_ = db.QueryRow(`SELECT MAX(ano) FROM tabela_aliquotas`).Scan(&ano)
		}

		var aliqIBS, aliqCBS float64
		err = db.QueryRow(`
			SELECT COALESCE(perc_ibs_uf + perc_ibs_mun, 17.7), COALESCE(perc_cbs, 8.8)
			FROM tabela_aliquotas WHERE ano = $1
		`, ano).Scan(&aliqIBS, &aliqCBS)
		if err == sql.ErrNoRows {
			aliqIBS, aliqCBS = 17.7, 8.8
		} else if err != nil {
			log.Printf("ReprecificacaoCSV parametros error: %v", err)
			http.Error(w, "Erro ao ler parâmetros", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="reprecificacao-produtos.csv"`)

		rows, err := db.Query(`
			SELECT
				COALESCE(nit.ncm, '')                        AS ncm,
				COALESCE(nit.x_prod, '')                     AS x_prod,
				COALESCE(nit.cst_icms, '')                   AS cst_icms,
				nit.v_prod,
				COALESCE(nit.v_icms, 0)                      AS v_icms,
				COALESCE(ncmr.ibs_reducao_pct, 0)            AS ibs_reducao_pct,
				COALESCE(ncmr.cbs_reducao_pct, 0)            AS cbs_reducao_pct
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
			WHERE nit.company_id = $1
			  AND ne.cancelado = 'N'
			  AND COALESCE(cf.tipo, 'O') != 'T'
			ORDER BY nit.v_prod DESC
			LIMIT 500
		`, companyID)
		if err != nil {
			log.Printf("ReprecificacaoCSV query error: %v", err)
			http.Error(w, "Erro ao consultar dados", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var list []Modulo12Row
		for rows.Next() {
			var ncm, xprod, cstIcms string
			var vProd, vIcms, ibsReducao, cbsReducao float64
			if err := rows.Scan(&ncm, &xprod, &cstIcms, &vProd, &vIcms, &ibsReducao, &cbsReducao); err != nil {
				continue
			}
			cstPath := "outro"
			switch cstIcms {
			case "00", "":
				cstPath = "normal"
			case "10", "30", "60", "70":
				cstPath = "st"
			case "20":
				cstPath = "base_reduzida"
			}
			ibsProjetado := vProd * aliqIBS / 100.0 * (1 - ibsReducao/100.0)
			cbsProjetado := vProd * aliqCBS / 100.0 * (1 - cbsReducao/100.0)
			precoSugerido := vProd + (ibsProjetado + cbsProjetado - vIcms)
			if precoSugerido < 0 {
				precoSugerido = 0
			}
			variacaoPct := 0.0
			if vProd > 0 {
				variacaoPct = (ibsProjetado + cbsProjetado - vIcms) / vProd * 100.0
			}
			list = append(list, Modulo12Row{
				NCM:           ncm,
				XProd:         xprod,
				CSTICMS:       cstIcms,
				CSTPath:       cstPath,
				PrecoAtual:    vProd,
				IcmsAtual:     vIcms,
				IBSProjetado:  ibsProjetado,
				CBSProjetado:  cbsProjetado,
				PrecoSugerido: precoSugerido,
				VariacaoPct:   variacaoPct,
			})
		}
		if err := rows.Err(); err != nil {
			log.Printf("[ReprecificacaoCSV] rows iteration error: %v", err)
			http.Error(w, "Erro ao ler dados", http.StatusInternalServerError)
			return
		}

		cw := csv.NewWriter(w)
		header := []string{"NCM", "Descrição Produto", "CST ICMS", "Preço Atual (R$)", "ICMS Atual (R$)", "IBS Projetado (R$)", "CBS Projetado (R$)", "Preço Sugerido (R$)", "Variação (%)"}
		if err := cw.Write(header); err != nil {
			log.Printf("[ReprecificacaoCSV] write header error: %v", err)
			return
		}

		for _, row := range list {
			record := []string{
				row.NCM,
				row.XProd,
				row.CSTICMS,
				fmt.Sprintf("%.2f", row.PrecoAtual),
				fmt.Sprintf("%.2f", row.IcmsAtual),
				fmt.Sprintf("%.2f", row.IBSProjetado),
				fmt.Sprintf("%.2f", row.CBSProjetado),
				fmt.Sprintf("%.2f", row.PrecoSugerido),
				fmt.Sprintf("%.2f", row.VariacaoPct),
			}
			if err := cw.Write(record); err != nil {
				log.Printf("[ReprecificacaoCSV] write row error: %v", err)
				return
			}
		}

		cw.Flush()
		if err := cw.Error(); err != nil {
			log.Printf("[ReprecificacaoCSV] flush error: %v", err)
		}
	}
}

// ---------------------------------------------------------------------------
// SplitPaymentHandler — GET /api/reforma/modulo1/split (RFMB-04, sem CSV)
// ---------------------------------------------------------------------------

func SplitPaymentHandler(db *sql.DB) http.HandlerFunc {
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

		var aliqIBS, aliqCBS, taxaCDI float64
		var prazoMedio int
		err = db.QueryRow(`
			SELECT COALESCE(ta.perc_ibs_uf + ta.perc_ibs_mun, 17.7), COALESCE(ta.perc_cbs, 8.8),
			       rp.taxa_cdi_anual_pct, rp.prazo_medio_dias
			FROM reforma_parametros rp
			LEFT JOIN tabela_aliquotas ta ON ta.ano = rp.target_ano
			WHERE rp.company_id = $1
		`, companyID).Scan(&aliqIBS, &aliqCBS, &taxaCDI, &prazoMedio)
		if err == sql.ErrNoRows {
			aliqIBS, aliqCBS, taxaCDI, prazoMedio = 17.7, 8.8, 10.5, 30
		} else if err != nil {
			log.Printf("SplitPayment parametros error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao ler parâmetros")
			return
		}

		// Total de saídas (fonte XML nfe_saidas, filtro cancelados e transferências via cfop cabeçalho)
		var totalSaidas float64
		err = db.QueryRow(`
			SELECT COALESCE(SUM(ns.v_nf), 0)
			FROM nfe_saidas ns
			LEFT JOIN cfop cf ON cf.cfop = ns.cfop
			WHERE ns.company_id = $1
			  AND ns.cancelado = 'N'
			  AND COALESCE(cf.tipo, 'O') != 'T'
		`, companyID).Scan(&totalSaidas)
		if err != nil {
			log.Printf("SplitPayment saidas query error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar saídas")
			return
		}

		// Cálculo (Pattern 7, decisão A3 — interpretação documentada)
		aliqTotal := aliqIBS + aliqCBS
		floatTributario := aliqTotal / 100.0 * totalSaidas * float64(prazoMedio) / 365.0
		custoCDI := floatTributario * taxaCDI / 100.0

		// Matriz de sensibilidade DSO × CDI gerada no Go
		dsoLinhas := []int{15, 30, 45, 60, 90}
		cdiColunas := []float64{8, 10, 12, 14}

		var sensibilidade []SensibilidadeRow
		for _, dso := range dsoLinhas {
			sr := SensibilidadeRow{DSO: dso}
			for _, cdi := range cdiColunas {
				custo := aliqTotal / 100.0 * totalSaidas * float64(dso) / 365.0 * cdi / 100.0
				sr.Custos = append(sr.Custos, custo)
			}
			sensibilidade = append(sensibilidade, sr)
		}

		json.NewEncoder(w).Encode(Modulo14Response{
			FloatTributario: floatTributario,
			CustoCDI:        custoCDI,
			TotalSaidas:     totalSaidas,
			AliqTotal:       aliqTotal,
			TaxaCDIAnualPct: taxaCDI,
			PrazoMedioDias:  prazoMedio,
			CDIColunas:      cdiColunas,
			DSOLinhas:       dsoLinhas,
			Sensibilidade:   sensibilidade,
		})
	}
}
