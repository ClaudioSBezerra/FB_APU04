package handlers

// cnpj_publico.go — enriquecimento de fornecedores/clientes com dados
// públicos de CNPJ (BrasilAPI/Receita Federal): situação cadastral, CNAE,
// Simples Nacional/MEI. Consultas ficam num cache global (cnpj_cadastro_
// publico, migration 163) — não é por empresa, porque o dado da Receita é o
// mesmo pra quem perguntar.
//
// NÃO confundir com rfb_apuracao.go/rfb_credentials.go: aquilo é a API paga
// da Receita Federal para a Reforma Tributária (IBS/CBS). Isto aqui é
// consulta pública de cadastro de CNPJ, sem credencial.
//
// Fluxo:
//  1. POST /api/fornecedores-clientes/enriquecer — lê CNPJs distintos de
//     fornecedores (nfe_entradas.forn_cnpj) e clientes (nfe_saidas.
//     dest_cnpj_cpf, só os de 14 dígitos — CPF de consumidor final fica de
//     fora) da empresa ativa, filtra os que faltam ou estão desatualizados
//     no cache (> 30 dias), e dispara um job em background que consulta a
//     BrasilAPI com rate limit conservador (1 req/s — API pública gratuita
//     compartilhada, não é nossa para saturar).
//  2. GET /api/fornecedores-clientes/jobs/{id} — progresso do job (poll).
//  3. GET /api/fornecedores-clientes/relatorio — fornecedores/clientes com
//     valor de compra/venda acumulado por ano, cruzado com o cache de CNPJ.

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"fb_apu04/services"
)

// ---------------------------------------------------------------------------
// POST /api/fornecedores-clientes/enriquecer
// ---------------------------------------------------------------------------

func CNPJPublicoEnriquecerHandler(db *sql.DB) http.HandlerFunc {
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

		// CNPJs distintos de fornecedores (entradas) e clientes (saídas, só
		// quando o destinatário é PJ — length 14; CPF de consumidor final fica
		// de fora, não faz sentido consultar situação cadastral de um CPF aqui).
		rows, err := db.Query(`
			SELECT DISTINCT cnpj FROM (
				SELECT forn_cnpj AS cnpj FROM nfe_entradas
				WHERE company_id = $1::uuid AND cancelado = 'N' AND length(forn_cnpj) = 14
				UNION
				SELECT dest_cnpj_cpf AS cnpj FROM nfe_saidas
				WHERE company_id = $1::uuid AND cancelado = 'N' AND length(dest_cnpj_cpf) = 14
			) t
			WHERE cnpj IS NOT NULL AND cnpj <> ''
		`, companyID)
		if err != nil {
			log.Printf("CNPJPublicoEnriquecer: erro ao listar CNPJs: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao listar CNPJs da base")
			return
		}
		var cnpjs []string
		for rows.Next() {
			var c string
			if err := rows.Scan(&c); err == nil {
				cnpjs = append(cnpjs, c)
			}
		}
		rows.Close()

		if len(cnpjs) == 0 {
			jsonErr(w, http.StatusBadRequest, "Nenhum CNPJ de fornecedor/cliente encontrado para esta empresa")
			return
		}

		// Só consulta o que falta ou está desatualizado (> 30 dias) — cache
		// compartilhado entre empresas, então um CNPJ já consultado por outra
		// empresa não precisa ser consultado de novo.
		pendentes := make([]string, 0, len(cnpjs))
		for _, c := range cnpjs {
			var existe bool
			_ = db.QueryRow(`
				SELECT EXISTS(
					SELECT 1 FROM cnpj_cadastro_publico
					WHERE cnpj = $1 AND consultado_em > now() - interval '30 days'
				)
			`, c).Scan(&existe)
			if !existe {
				pendentes = append(pendentes, c)
			}
		}

		var jobID string
		err = db.QueryRow(`
			INSERT INTO cnpj_consulta_jobs (company_id, status, total, mensagem)
			VALUES ($1::uuid, 'pending', $2, $3)
			RETURNING id::text
		`, companyID, len(pendentes), "Aguardando início — "+strconv.Itoa(len(cnpjs)-len(pendentes))+" já em cache").Scan(&jobID)
		if err != nil {
			log.Printf("CNPJPublicoEnriquecer: erro ao criar job: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao criar job de enriquecimento")
			return
		}

		if len(pendentes) == 0 {
			db.Exec(`UPDATE cnpj_consulta_jobs SET status='completed', mensagem='Todos os CNPJs já estavam em cache', updated_at=now() WHERE id=$1::uuid`, jobID)
		} else {
			go processarEnriquecimentoCNPJ(db, jobID, pendentes)
		}

		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"job_id":      jobID,
			"total_cnpjs": len(cnpjs),
			"a_consultar": len(pendentes),
			"ja_em_cache": len(cnpjs) - len(pendentes),
		})
	}
}

// processarEnriquecimentoCNPJ roda em background: consulta a BrasilAPI CNPJ
// por CNPJ com rate limit fixo (1 req/s — API pública gratuita, sem chave,
// compartilhada por todo mundo que usa o serviço; não é nossa para saturar),
// grava cada resultado no cache assim que chega, e atualiza o progresso do
// job pra tela poder fazer polling.
func processarEnriquecimentoCNPJ(db *sql.DB, jobID string, cnpjs []string) {
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("processarEnriquecimentoCNPJ[%s] panic: %v", jobID, rec)
			db.Exec(`UPDATE cnpj_consulta_jobs SET status='error', mensagem=$1, updated_at=now() WHERE id=$2::uuid`,
				"Erro interno durante o processamento", jobID)
		}
	}()

	db.Exec(`UPDATE cnpj_consulta_jobs SET status='processing', updated_at=now() WHERE id=$1::uuid`, jobID)

	client := services.NewCNPJPublicoClient()
	ticker := time.NewTicker(1 * time.Second) // 1 req/s — conservador de propósito
	defer ticker.Stop()

	var processados, encontrados, erros int
	for _, cnpj := range cnpjs {
		<-ticker.C

		if jobCancelado(db, jobID) {
			db.Exec(`
				UPDATE cnpj_consulta_jobs
				SET mensagem=$1, updated_at=now()
				WHERE id=$2::uuid AND status='cancelled'
			`, "Cancelado pelo usuário — "+strconv.Itoa(processados)+"/"+strconv.Itoa(len(cnpjs))+" processados antes de parar", jobID)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		result, err := client.Consultar(ctx, cnpj)
		cancel()

		processados++
		if err != nil {
			erros++
			// Persiste o erro também — evita reconsultar em loop um CNPJ que
			// não existe na Receita a cada rodada (fica só 30 dias em cache
			// como qualquer outro registro, depois pode tentar de novo).
			db.Exec(`
				INSERT INTO cnpj_cadastro_publico (cnpj, erro, consultado_em)
				VALUES ($1, $2, now())
				ON CONFLICT (cnpj) DO UPDATE SET erro = EXCLUDED.erro, consultado_em = now()
			`, cnpj, err.Error())
		} else {
			encontrados++
			_, dbErr := db.Exec(`
				INSERT INTO cnpj_cadastro_publico (
					cnpj, razao_social, nome_fantasia, situacao_cadastral, data_situacao_cadastral,
					natureza_juridica, porte, cnae_codigo, cnae_descricao, uf, municipio,
					data_inicio_atividade, simples_nacional, data_opcao_simples, data_exclusao_simples,
					mei, data_opcao_mei, data_exclusao_mei, erro, consultado_em
				) VALUES (
					$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, NULL, now()
				)
				ON CONFLICT (cnpj) DO UPDATE SET
					razao_social = EXCLUDED.razao_social,
					nome_fantasia = EXCLUDED.nome_fantasia,
					situacao_cadastral = EXCLUDED.situacao_cadastral,
					data_situacao_cadastral = EXCLUDED.data_situacao_cadastral,
					natureza_juridica = EXCLUDED.natureza_juridica,
					porte = EXCLUDED.porte,
					cnae_codigo = EXCLUDED.cnae_codigo,
					cnae_descricao = EXCLUDED.cnae_descricao,
					uf = EXCLUDED.uf,
					municipio = EXCLUDED.municipio,
					data_inicio_atividade = EXCLUDED.data_inicio_atividade,
					simples_nacional = EXCLUDED.simples_nacional,
					data_opcao_simples = EXCLUDED.data_opcao_simples,
					data_exclusao_simples = EXCLUDED.data_exclusao_simples,
					mei = EXCLUDED.mei,
					data_opcao_mei = EXCLUDED.data_opcao_mei,
					data_exclusao_mei = EXCLUDED.data_exclusao_mei,
					erro = NULL,
					consultado_em = now()
			`, result.CNPJ, result.RazaoSocial, result.NomeFantasia, result.SituacaoCadastral,
				result.DataSituacaoCadastral, result.NaturezaJuridica, result.Porte,
				result.CNAECodigo, result.CNAEDescricao, result.UF, result.Municipio,
				result.DataInicioAtividade, result.SimplesNacional, result.DataOpcaoSimples,
				result.DataExclusaoSimples, result.MEI, result.DataOpcaoMEI, result.DataExclusaoMEI)
			if dbErr != nil {
				log.Printf("processarEnriquecimentoCNPJ[%s] erro ao gravar %s: %v", jobID, cnpj, dbErr)
			}
		}

		db.Exec(`
			UPDATE cnpj_consulta_jobs
			SET processados=$1, encontrados=$2, erros=$3,
			    mensagem=$4, updated_at=now()
			WHERE id=$5::uuid
		`, processados, encontrados, erros,
			"Processando "+strconv.Itoa(processados)+"/"+strconv.Itoa(len(cnpjs)), jobID)
	}

	db.Exec(`
		UPDATE cnpj_consulta_jobs
		SET status='completed', mensagem=$1, updated_at=now()
		WHERE id=$2::uuid AND status <> 'cancelled'
	`, "Concluído: "+strconv.Itoa(encontrados)+" encontrados, "+strconv.Itoa(erros)+" com erro", jobID)
}

// jobCancelado verifica se o usuário pediu cancelamento (CNPJPublicoCancelarJobHandler)
// desde a última iteração. Consultado a cada CNPJ — como já há um rate limit
// de 1 req/s, esse SELECT extra é irrelevante no tempo total.
func jobCancelado(db *sql.DB, jobID string) bool {
	var status string
	if err := db.QueryRow(`SELECT status FROM cnpj_consulta_jobs WHERE id=$1::uuid`, jobID).Scan(&status); err != nil {
		return false
	}
	return status == "cancelled"
}

// ---------------------------------------------------------------------------
// GET  /api/fornecedores-clientes/jobs/{id}            — status (poll)
// POST /api/fornecedores-clientes/jobs/{id}/cancelar   — cancela um job em andamento
// ---------------------------------------------------------------------------

type CNPJConsultaJobStatus struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	Total       int    `json:"total"`
	Processados int    `json:"processados"`
	Encontrados int    `json:"encontrados"`
	Erros       int    `json:"erros"`
	Mensagem    string `json:"mensagem"`
}

// CNPJPublicoJobStatusHandler atende tanto o polling de status (GET) quanto
// o cancelamento (POST .../cancelar) — registrados sob o mesmo prefixo em
// main.go, já que o ID do job é um segmento dinâmico no meio do path.
func CNPJPublicoJobStatusHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims); !ok {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/api/fornecedores-clientes/jobs/")

		if strings.HasSuffix(path, "/cancelar") {
			if r.Method != http.MethodPost {
				jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
				return
			}
			jobID := strings.TrimSpace(strings.TrimSuffix(path, "/cancelar"))
			if jobID == "" {
				jsonErr(w, http.StatusBadRequest, "ID do job não informado")
				return
			}
			res, err := db.Exec(`
				UPDATE cnpj_consulta_jobs SET status='cancelled', updated_at=now()
				WHERE id=$1::uuid AND status IN ('pending','processing')
			`, jobID)
			if err != nil {
				log.Printf("CNPJPublicoCancelarJob error: %v", err)
				jsonErr(w, http.StatusInternalServerError, "Erro ao cancelar job")
				return
			}
			if n, _ := res.RowsAffected(); n == 0 {
				jsonErr(w, http.StatusConflict, "Job não está em andamento (já concluído, cancelado ou inexistente)")
				return
			}
			// A rotina em background lê o status a cada CNPJ (até 1s de atraso
			// pelo rate limit) e para sozinha ao ver 'cancelled'.
			json.NewEncoder(w).Encode(map[string]string{"status": "cancelled"})
			return
		}

		if r.Method != http.MethodGet {
			jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		jobID := strings.TrimSpace(path)
		if jobID == "" {
			jsonErr(w, http.StatusBadRequest, "ID do job não informado")
			return
		}

		var s CNPJConsultaJobStatus
		var mensagem sql.NullString
		err := db.QueryRow(`
			SELECT id::text, status, total, processados, encontrados, erros, mensagem
			FROM cnpj_consulta_jobs WHERE id = $1::uuid
		`, jobID).Scan(&s.ID, &s.Status, &s.Total, &s.Processados, &s.Encontrados, &s.Erros, &mensagem)
		if err == sql.ErrNoRows {
			jsonErr(w, http.StatusNotFound, "Job não encontrado")
			return
		}
		if err != nil {
			log.Printf("CNPJPublicoJobStatus error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar job")
			return
		}
		s.Mensagem = mensagem.String
		json.NewEncoder(w).Encode(s)
	}
}

// ---------------------------------------------------------------------------
// GET /api/fornecedores-clientes/relatorio
// ---------------------------------------------------------------------------

type FornecedorClienteRow struct {
	CNPJ              string `json:"cnpj"`
	Tipo              string `json:"tipo"` // "fornecedor" | "cliente"
	NomeNota          string `json:"nome_nota"`
	RazaoSocialRFB    string `json:"razao_social_rfb"`
	NomeFantasia      string `json:"nome_fantasia"`
	SituacaoCadastral string `json:"situacao_cadastral"`
	// DataSituacaoCadastral é a data do evento da situação cadastral atual —
	// ex.: data da baixa quando situacao_cadastral = "BAIXADA", data de
	// abertura quando "ATIVA". Nula quando o CNPJ ainda não foi consultado.
	DataSituacaoCadastral *string `json:"data_situacao_cadastral"`
	NaturezaJuridica      string  `json:"natureza_juridica"`
	Porte                 string  `json:"porte"`
	CNAECodigo            string  `json:"cnae_codigo"`
	CNAEDescricao         string  `json:"cnae_descricao"`
	UF                    string  `json:"uf"`
	Municipio             string  `json:"municipio"`
	SimplesNacional       *bool   `json:"simples_nacional"`
	MEI                   *bool   `json:"mei"`
	Ano                   int     `json:"ano"`
	ValorAcumulado        float64 `json:"valor_acumulado"`
	QtdNotas              int     `json:"qtd_notas"`
	ConsultadoRFB         bool    `json:"consultado_rfb"`
}

type FornecedorClienteRelatorioResponse struct {
	Rows  []FornecedorClienteRow `json:"rows"`
	Count int                    `json:"count"`
}

func CNPJPublicoRelatorioHandler(db *sql.DB) http.HandlerFunc {
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

		tipoFiltro := r.URL.Query().Get("tipo") // "" | "fornecedor" | "cliente"

		const query = `
			WITH fornecedores AS (
				SELECT
					forn_cnpj                              AS cnpj,
					'fornecedor'                            AS tipo,
					MAX(forn_nome)                          AS nome_nota,
					EXTRACT(YEAR FROM data_emissao)::int    AS ano,
					SUM(v_nf)                               AS valor_acumulado,
					COUNT(*)                                AS qtd_notas
				FROM nfe_entradas
				WHERE company_id = $1::uuid AND cancelado = 'N' AND length(forn_cnpj) = 14
				GROUP BY forn_cnpj, EXTRACT(YEAR FROM data_emissao)
			), clientes AS (
				SELECT
					dest_cnpj_cpf                           AS cnpj,
					'cliente'                                AS tipo,
					MAX(dest_nome)                           AS nome_nota,
					EXTRACT(YEAR FROM data_emissao)::int     AS ano,
					SUM(v_nf)                                AS valor_acumulado,
					COUNT(*)                                 AS qtd_notas
				FROM nfe_saidas
				WHERE company_id = $1::uuid AND cancelado = 'N' AND length(dest_cnpj_cpf) = 14
				GROUP BY dest_cnpj_cpf, EXTRACT(YEAR FROM data_emissao)
			), uniao AS (
				SELECT * FROM fornecedores
				UNION ALL
				SELECT * FROM clientes
			)
			SELECT
				u.cnpj, u.tipo, u.nome_nota, u.ano, u.valor_acumulado, u.qtd_notas,
				COALESCE(c.razao_social, ''), COALESCE(c.nome_fantasia, ''),
				COALESCE(c.situacao_cadastral, ''), c.data_situacao_cadastral::text, COALESCE(c.natureza_juridica, ''),
				COALESCE(c.porte, ''), COALESCE(c.cnae_codigo, ''), COALESCE(c.cnae_descricao, ''),
				COALESCE(c.uf, ''), COALESCE(c.municipio, ''),
				c.simples_nacional, c.mei, (c.cnpj IS NOT NULL AND c.erro IS NULL)
			FROM uniao u
			LEFT JOIN cnpj_cadastro_publico c ON c.cnpj = u.cnpj
			WHERE ($2::text = '' OR u.tipo = $2::text)
			ORDER BY u.tipo, u.valor_acumulado DESC, u.ano DESC
		`
		rows, err := db.Query(query, companyID, tipoFiltro)
		if err != nil {
			log.Printf("CNPJPublicoRelatorio error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar relatório")
			return
		}
		defer rows.Close()

		result := []FornecedorClienteRow{}
		for rows.Next() {
			var row FornecedorClienteRow
			var dataSituacao sql.NullString
			if err := rows.Scan(
				&row.CNPJ, &row.Tipo, &row.NomeNota, &row.Ano, &row.ValorAcumulado, &row.QtdNotas,
				&row.RazaoSocialRFB, &row.NomeFantasia,
				&row.SituacaoCadastral, &dataSituacao, &row.NaturezaJuridica,
				&row.Porte, &row.CNAECodigo, &row.CNAEDescricao,
				&row.UF, &row.Municipio,
				&row.SimplesNacional, &row.MEI, &row.ConsultadoRFB,
			); err != nil {
				log.Printf("CNPJPublicoRelatorio scan error: %v", err)
				continue
			}
			if dataSituacao.Valid {
				row.DataSituacaoCadastral = &dataSituacao.String
			}
			result = append(result, row)
		}

		json.NewEncoder(w).Encode(FornecedorClienteRelatorioResponse{
			Rows:  result,
			Count: len(result),
		})
	}
}
