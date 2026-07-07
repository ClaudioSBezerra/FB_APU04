// fiscal_execution.go — endpoint admin de execução em lote do pacote fiscal
// (TPF-05). Porte adaptado de FB_TESTESFC backend/handlers/fiscal_execution.go
// (validado contra Oracle real, 2026-06-30..07-02).
//
// Costura os artefatos das waves anteriores desta fase:
//   - openFiscalOracleConn (Plan 11-01, fiscal_oracle_conn.go)
//   - resolveCodEmpresa / lookupGrupoFiscal (Plan 11-03, fiscal_group_lookup.go)
//   - services.CallFiscalPackage (Plan 11-04, services/oracle_fiscal.go)
//   - fiscal_execution_items (Plan 11-03, migration 147)
//
// Padrão novo nesta base de código (11-RESEARCH.md Pattern 4): fan-out de
// goroutines com semáforo cap 5, timeout de 15s por item, isolamento de
// panic por item (defer recover()) e upsert por item — nunca uma transação
// única para o lote inteiro (TPF-05/D-04).
package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"fb_apu04/services"
)

// ---------------------------------------------------------------------------
// Defaults de parâmetros do pacote fiscal sem fonte de dados persistida em
// nfe_saidas/nfe_saidas_itens. Porte verbatim do FB_TESTESFC — só o caminho
// "normal" de venda foi validado contra Oracle real; Simples Nacional/
// prestação de serviço podem expor default incorreto (gap conhecido e aceito,
// ver 11-CONTEXT.md <specifics>). Divergências aparecerão na tela da Fase 12,
// não travam este endpoint.
// ---------------------------------------------------------------------------
const (
	defaultTipoCentroFiscal          = "VRJNE" // valor do script de teste do pacote fiscal original
	defaultIndicadorServico          = "N"     // comércio, não serviço
	defaultFornecedorSimplesNacional = "N"     // CRT do emitente não persistido em nfe_saidas
)

// tipoContribuinte deriva pTipoContribuinte (regra refinada em 2026-07-06
// após NF-e 55 para PJ não contribuinte não calcular DIFAL). Precedência:
//
//  1. <dest><indIEDest> do XML — fonte da verdade sobre o destinatário:
//     1 = contribuinte ICMS            → "S"
//     2 = contribuinte isento de IE    → "S"
//     9 = NÃO contribuinte (PF ou PJ)  → "N"  ← caso do DIFAL EC 87/2015
//  2. CFOP do item 6107/6108 (venda interestadual destinada a NÃO
//     contribuinte) → "N" quando o indIEDest não veio no XML
//  3. Fallback por modelo: 55 → "S", 65 → "N"
func tipoContribuinte(destIndIE string, cfop string, modelo int) string {
	switch strings.TrimSpace(destIndIE) {
	case "1", "2":
		return "S"
	case "9":
		return "N"
	}
	switch strings.TrimSpace(cfop) {
	case "6107", "6108":
		return "N"
	}
	if modelo == 55 {
		return "S"
	}
	return "N"
}

// cfopsTransferencia são os CFOPs de saída por transferência (mesma empresa,
// filial→filial). Para eles o pacote fiscal recebe pTipoOperacao=20; qualquer
// outro CFOP de saída é tratado como venda (1).
var cfopsTransferencia = map[string]bool{
	"5151": true, "5152": true, "5155": true, "5156": true,
	"5408": true, "5409": true,
	"6151": true, "6152": true, "6155": true, "6156": true,
	"6408": true, "6409": true,
}

func tipoOperacaoPorCFOP(cfop string) int {
	if cfopsTransferencia[strings.TrimSpace(cfop)] {
		return 20
	}
	return 1
}

// ---------------------------------------------------------------------------
// Simulação "IBS/CBS na base do ICMS" (fase BOA, 2026-07) — quando a execução
// roda com incluir_ibs_cbs_base=true, cada item ganha uma 2ª chamada ao
// pacote com pPrecoTotal = original + IBS + CBS (retornados pela 1ª chamada).
// A simulação interna escala os valores da 1ª chamada pelo fator do acréscimo
// (linear: mesma alíquota/redução/MVA) e compara com a 2ª chamada — se o
// pacote divergir da simulação, ou a inclusão não está linear (pauta, faixa)
// ou não está sendo aplicada como esperado.
// ---------------------------------------------------------------------------

type fiscalSimulacao struct {
	Fator           float64 `json:"fator"`             // (preço+IBS+CBS)/preço
	AcrescimoIbsCbs float64 `json:"acrescimo_ibs_cbs"` // IBS+CBS da 1ª chamada
	PrecoOriginal   float64 `json:"preco_original"`
	PrecoSimulado   float64 `json:"preco_simulado"`
	// Preço Líquido = (venda − descontos + frete + outras despesas) − ICMS −
	// ICMS-ST − PIS − COFINS − ISS (valores declarados no XML; ISS = 0 em
	// NF-e de mercadoria). REGRA DEFINITIVA do negócio (Claudio, 2026-07-07):
	// o ST entra na exclusão — compõe o total da NF pago pelo cliente. É a
	// base do IBS/CBS na transição, comparável com BaseIbsCbsPacote (o que o
	// pacote usou na 1ª chamada — que também abate ST; diferença esperada ≈
	// centavos de arredondamento por item).
	PrecoLiquido     float64 `json:"preco_liquido"`
	BaseIbsCbsPacote float64 `json:"base_ibs_cbs_pacote"`

	// Alíquotas retornadas pela 1ª chamada do pacote (memória de cálculo)
	AliquotaIcms    float64 `json:"aliquota_icms"`    // %
	AliquotaFcp     float64 `json:"aliquota_fcp"`     // % (fundo pobreza)
	PercentualDifal float64 `json:"percentual_difal"` // %

	// Original (XML, sem inclusão; FCP/DIFAL da 1ª chamada — XML não destaca por item)
	BaseIcmsOriginal float64 `json:"base_icms_original"`
	IcmsOriginal     float64 `json:"icms_original"`
	BaseStOriginal   float64 `json:"base_st_original"`
	StOriginal       float64 `json:"st_original"`
	FcpOriginal      float64 `json:"fcp_original"`
	DifalOriginal    float64 `json:"difal_original"`

	// Cálculo simulado interno (original × fator)
	BaseIcmsSimulada float64 `json:"base_icms_simulada"`
	IcmsSimulado     float64 `json:"icms_simulado"`
	BaseStSimulada   float64 `json:"base_st_simulada"`
	StSimulado       float64 `json:"st_simulado"`
	FcpSimulado      float64 `json:"fcp_simulado"`
	DifalSimulado    float64 `json:"difal_simulado"`

	// Cálculo do pacote fiscal (2ª chamada, preço acrescido)
	BaseIcmsPacote float64 `json:"base_icms_pacote"`
	IcmsPacote     float64 `json:"icms_pacote"`
	BaseStPacote   float64 `json:"base_st_pacote"`
	StPacote       float64 `json:"st_pacote"`
	FcpPacote      float64 `json:"fcp_pacote"`
	DifalPacote    float64 `json:"difal_pacote"`

	Erro string `json:"erro,omitempty"` // 2ª chamada falhou / IBS+CBS zerados
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// runSimulacaoIbsCbs executa a 2ª chamada e monta a comparação. Nunca aborta
// o item — em falha, devolve a simulação preenchida só com Erro.
func runSimulacaoIbsCbs(ctx context.Context, oracleDB *sql.DB, in services.FiscalInput, it fiscalItemInput, r1 *services.FiscalResult, trace *fiscalDebugTrace, produtoLabel string) fiscalSimulacao {
	// Original = o que a nota DECLAROU no XML (base da simulação, coerente com
	// o "Esperado" do Resumo). FCP e DIFAL não são destacados por item no XML
	// — nesses dois o original vem da 1ª chamada do pacote.
	sim := fiscalSimulacao{
		PrecoOriginal:    it.VProd,
		PrecoLiquido:     round2((it.VProd - it.VDesc + it.VFrete + it.VOutro) - it.VIcmsXML - it.VStXML - it.VPisXML - it.VCofinsXML),
		BaseIbsCbsPacote: r1.BaseCalculoIbsCbs,
		AliquotaIcms:     r1.AliquotaImposto,
		AliquotaFcp:      r1.AliquotaFundoPobreza,
		PercentualDifal:  r1.PercentualDifal,
		BaseIcmsOriginal: it.VBcIcmsXML,
		IcmsOriginal:     it.VIcmsXML,
		BaseStOriginal:   it.VBcStXML,
		StOriginal:       it.VStXML,
		FcpOriginal:      r1.ValorIcmsPobreza,
		DifalOriginal:    r1.ValorIcmsPartilhaDestino,
	}

	ibsCbs := r1.ValorIbsUF + r1.ValorIbsMUN + r1.ValorCbs
	if it.VProd <= 0 || ibsCbs <= 0 {
		sim.Erro = fmt.Sprintf("Sem base para simular (preço %.2f, IBS+CBS %.2f retornados pela 1ª chamada).", it.VProd, ibsCbs)
		trace.add(it.ID, produtoLabel, "simulacao", sim.Erro)
		return sim
	}

	sim.AcrescimoIbsCbs = round2(ibsCbs)
	sim.PrecoSimulado = round2(it.VProd + ibsCbs)
	fator := sim.PrecoSimulado / it.VProd
	sim.Fator = math.Round(fator*10000) / 10000

	sim.BaseIcmsSimulada = round2(sim.BaseIcmsOriginal * fator)
	sim.IcmsSimulado = round2(sim.IcmsOriginal * fator)
	sim.BaseStSimulada = round2(sim.BaseStOriginal * fator)
	sim.StSimulado = round2(sim.StOriginal * fator)
	sim.FcpSimulado = round2(sim.FcpOriginal * fator)
	sim.DifalSimulado = round2(sim.DifalOriginal * fator)

	in2 := in
	in2.PPrecoTotal = sim.PrecoSimulado
	trace.add(it.ID, produtoLabel, "simulacao_chamada", fmt.Sprintf("2ª chamada (IBS/CBS na base): pPrecoTotal %.2f → %.2f (acréscimo IBS+CBS %.2f, fator %.4f)", it.VProd, sim.PrecoSimulado, sim.AcrescimoIbsCbs, sim.Fator))

	r2, err := services.CallFiscalPackage(ctx, oracleDB, in2)
	if err != nil {
		sim.Erro = "Falha na 2ª chamada do pacote (preço acrescido): " + sanitizeOracleErrForDebug(err)
		trace.add(it.ID, produtoLabel, "simulacao", sim.Erro)
		return sim
	}

	sim.BaseIcmsPacote = r2.BaseCalculo
	sim.IcmsPacote = r2.ValorImposto
	sim.BaseStPacote = r2.BaseSubstituicao
	sim.StPacote = r2.ValorSubstituicao
	sim.FcpPacote = r2.ValorIcmsPobreza
	sim.DifalPacote = r2.ValorIcmsPartilhaDestino

	trace.add(it.ID, produtoLabel, "simulacao_concluida", fmt.Sprintf(
		"ICMS sim %.2f × pacote %.2f | ST sim %.2f × %.2f | FCP sim %.2f × %.2f | DIFAL sim %.2f × %.2f",
		sim.IcmsSimulado, sim.IcmsPacote, sim.StSimulado, sim.StPacote,
		sim.FcpSimulado, sim.FcpPacote, sim.DifalSimulado, sim.DifalPacote))
	return sim
}

// fiscalNotaContext agrega os dados de cabeçalho da nota necessários para
// montar o FiscalInput de cada item + o cod_empresa resolvido uma única vez
// por nota (guard IDOR T-11-14: nfe_id sempre escopado por company_id, nunca
// aceito "solto" do cliente).
type fiscalNotaContext struct {
	EmitCNPJ      string
	EmitUF        string
	DestUF        string
	DestCMun      string
	DestIndIE     string // <indIEDest>: 1/2 = contribuinte, 9 = não contribuinte
	Modelo        int    // 55 = NF-e, 65 = NFC-e — fallback do pTipoContribuinte
	DataEmissao   time.Time
	CodEmpresa    int
	CodEmpresaErr error
}

type fiscalItemInput struct {
	ID     string
	CProd  string
	XProd  string
	CFOP   string
	VProd  float64
	VDesc  float64
	VOutro float64
	VFrete float64
	VIPI   float64
	// Valores DECLARADOS no XML — linha "Original" da simulação IBS/CBS
	// (a base da simulação é o que a nota destacou, não a 1ª chamada)
	VBcIcmsXML float64
	VIcmsXML   float64
	VBcStXML   float64
	VStXML     float64
	VPisXML    float64
	VCofinsXML float64
}

type fiscalExecutionSummary struct {
	Total          int                `json:"total"`
	OK             int                `json:"ok"`
	SemGrupoFiscal int                `json:"sem_grupo_fiscal"`
	Error          int                `json:"error"`
	Debug          []fiscalDebugEntry `json:"debug"`
}

// ---------------------------------------------------------------------------
// Rastro de depuração (2026-07) — a execução é uma chamada HTTP síncrona
// (não streaming), então este trace é devolvido no corpo da resposta ao
// final da execução ("o que aconteceu", não "o que está acontecendo agora"
// em tempo real — isso exigiria SSE/WebSocket). Thread-safe: escrito por até
// 5 goroutines concorrentes (semáforo de processFiscalBatch).
// ---------------------------------------------------------------------------

type fiscalDebugEntry struct {
	Timestamp string `json:"timestamp"`
	ItemID    string `json:"item_id,omitempty"`
	Produto   string `json:"produto,omitempty"`
	Etapa     string `json:"etapa"`
	Mensagem  string `json:"mensagem"`
}

type fiscalDebugTrace struct {
	mu      sync.Mutex
	entries []fiscalDebugEntry
}

// sanitizeOracleErrForDebug prepara um erro Oracle para o debug trace
// admin-only (nunca para a resposta de erro normal da API — T-11-18): troca
// quebras de linha por espaço e limita o tamanho, para nunca despejar um
// stacktrace inteiro ou (em tese) uma connection string na tela.
func sanitizeOracleErrForDebug(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ReplaceAll(err.Error(), "\n", " ")
	const maxLen = 300
	if len(msg) > maxLen {
		msg = msg[:maxLen] + "..."
	}
	return msg
}

func (t *fiscalDebugTrace) add(itemID, produto, etapa, mensagem string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.entries = append(t.entries, fiscalDebugEntry{
		Timestamp: time.Now().Format("15:04:05.000"),
		ItemID:    itemID,
		Produto:   produto,
		Etapa:     etapa,
		Mensagem:  mensagem,
	})
}

// ---------------------------------------------------------------------------
// POST /api/fiscal/execute
// Body: {"nfe_id": "<uuid>"}
// ---------------------------------------------------------------------------

// FiscalExecutionRunHandler orquestra a execução em lote do pacote fiscal
// para todos os itens de uma nfe_saidas: carrega cabeçalho+itens (escopados
// por company_id — T-11-14), abre a conexão Oracle dedicada (Plan 11-01),
// resolve o grupo fiscal por item (Plan 11-03), chama o pacote fiscal
// (Plan 11-04) e persiste cada resultado em fiscal_execution_items com
// concorrência limitada, timeout por item e isolamento de erro (TPF-05).
func FiscalExecutionRunHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPost {
			jsonErr(w, http.StatusMethodNotAllowed, "Método não permitido")
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
			log.Printf("FiscalExecutionRunHandler: GetEffectiveCompanyID falhou para user_id=%s: %v", userID, err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa")
			return
		}

		var req struct {
			NfeID string `json:"nfe_id"`
			// Simulação IBS/CBS na base do ICMS: 2ª chamada por item com
			// pPrecoTotal = original + IBS + CBS (fase BOA, 2026-07)
			IncluirIbsCbsBase bool `json:"incluir_ibs_cbs_base"`
		}
		if decErr := json.NewDecoder(r.Body).Decode(&req); decErr != nil || strings.TrimSpace(req.NfeID) == "" {
			jsonErr(w, http.StatusBadRequest, "nfe_id é obrigatório")
			return
		}

		// Guard IDOR (T-11-14): a nota só é carregada se pertencer à company_id
		// resolvida via JWT — nunca confiar em company_id vindo do corpo/cliente.
		var emitCNPJ, emitUF, emitCMun, destUF, destCMun, destIndIE string
		var modelo int
		var dataEmissao time.Time
		err = db.QueryRow(`
			SELECT COALESCE(emit_cnpj,''), COALESCE(emit_uf,''), COALESCE(emit_c_mun,''), COALESCE(dest_uf,''), COALESCE(dest_c_mun,''), COALESCE(dest_ind_ie,''), modelo, data_emissao
			FROM pacotefiscal_nfe_saidas
			WHERE id = $1 AND company_id = $2`, req.NfeID, companyID,
		).Scan(&emitCNPJ, &emitUF, &emitCMun, &destUF, &destCMun, &destIndIE, &modelo, &dataEmissao)
		if err == sql.ErrNoRows {
			jsonErr(w, http.StatusNotFound, "Nota não encontrada")
			return
		}
		if err != nil {
			log.Printf("FiscalExecutionRunHandler: erro ao carregar pacotefiscal_nfe_saidas (nfe_id=%s): %v", req.NfeID, err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao carregar nota")
			return
		}

		// Consumidor não identificado (NFC-e sem <dest> no XML): o pacote exige
		// UF de destino (ORA-20000 "E necessario informar a UF de Destino") —
		// venda presencial assume UF e município do EMITENTE (regra do negócio
		// 2026-07-06).
		if strings.TrimSpace(destUF) == "" {
			destUF = emitUF
			if strings.TrimSpace(destCMun) == "" {
				destCMun = emitCMun
			}
		}

		nfeCtx := fiscalNotaContext{
			EmitCNPJ:    emitCNPJ,
			EmitUF:      emitUF,
			DestUF:      destUF,
			DestCMun:    destCMun,
			DestIndIE:   destIndIE,
			Modelo:      modelo,
			DataEmissao: dataEmissao,
		}
		nfeCtx.CodEmpresa, nfeCtx.CodEmpresaErr = resolveCodEmpresa(emitCNPJ, emitUF)

		itemRows, err := db.Query(`
			SELECT id, COALESCE(c_prod,''), x_prod, COALESCE(cfop,''), COALESCE(v_prod,0), COALESCE(v_desc,0), COALESCE(v_outro,0), COALESCE(v_frete,0), COALESCE(v_ipi,0),
			       COALESCE(v_bc_icms,0), COALESCE(v_icms,0), COALESCE(v_bc_st,0), COALESCE(v_st,0), COALESCE(v_pis,0), COALESCE(v_cofins,0)
			FROM pacotefiscal_nfe_saidas_itens
			WHERE nfe_id = $1 AND company_id = $2
			ORDER BY n_item ASC`, req.NfeID, companyID)
		if err != nil {
			log.Printf("FiscalExecutionRunHandler: erro ao carregar pacotefiscal_nfe_saidas_itens (nfe_id=%s): %v", req.NfeID, err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao carregar itens da nota")
			return
		}
		var itens []fiscalItemInput
		for itemRows.Next() {
			var it fiscalItemInput
			if scanErr := itemRows.Scan(&it.ID, &it.CProd, &it.XProd, &it.CFOP, &it.VProd, &it.VDesc, &it.VOutro, &it.VFrete, &it.VIPI,
				&it.VBcIcmsXML, &it.VIcmsXML, &it.VBcStXML, &it.VStXML, &it.VPisXML, &it.VCofinsXML); scanErr != nil {
				log.Printf("FiscalExecutionRunHandler: erro ao escanear item (nfe_id=%s): %v", req.NfeID, scanErr)
				continue
			}
			itens = append(itens, it)
		}
		if scanErr := itemRows.Err(); scanErr != nil {
			log.Printf("FiscalExecutionRunHandler: erro ao iterar itens (nfe_id=%s): %v", req.NfeID, scanErr)
			itemRows.Close()
			jsonErr(w, http.StatusInternalServerError, "Erro ao carregar itens da nota")
			return
		}
		itemRows.Close()

		if len(itens) == 0 {
			json.NewEncoder(w).Encode(fiscalExecutionSummary{})
			return
		}

		trace := &fiscalDebugTrace{}
		trace.add("", "", "conexao", fmt.Sprintf("Conectando ao Oracle (company_id=%s, filial cod_empresa=%d)...", companyID, nfeCtx.CodEmpresa))

		// Conexão Oracle dedicada a este lote (Plan 11-01) — SetMaxOpenConns(5)
		// já casado com o cap do semáforo usado em processFiscalBatch.
		oracleConn, err := openFiscalOracleConn(db, companyID)
		if err != nil {
			log.Printf("FiscalExecutionRunHandler: openFiscalOracleConn falhou (company_id=%s): %v", companyID, err)
			trace.add("", "", "conexao", "Falha ao conectar ao Oracle — verifique as credenciais ERP configuradas.")
			jsonErr(w, http.StatusBadGateway, "Falha ao conectar ao Oracle. Verifique as credenciais ERP configuradas.")
			return
		}
		defer oracleConn.Close()
		trace.add("", "", "conexao", "Conexão Oracle estabelecida (FCCORP/PRODB).")

		// Backstop apenas — o timeout real é por item (15s), não do lote inteiro.
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
		defer cancel()

		if req.IncluirIbsCbsBase {
			trace.add("", "", "simulacao", "Modo simulação ATIVO: IBS/CBS na base do ICMS (2ª chamada por item com preço acrescido).")
		}
		summary := processFiscalBatch(ctx, oracleConn, db, companyID, nfeCtx, itens, trace, req.IncluirIbsCbsBase)
		summary.Debug = trace.entries
		json.NewEncoder(w).Encode(summary)
	}
}

// ---------------------------------------------------------------------------
// Isolamento de erro por item — nunca aborta o lote (TPF-05).
// Semáforo de concorrência limitado a 5 + recover por item + upsert por item.
// ---------------------------------------------------------------------------

func processFiscalBatch(ctx context.Context, oracleDB *sql.DB, pgDB *sql.DB, companyID string, nfe fiscalNotaContext, itens []fiscalItemInput, trace *fiscalDebugTrace, simularIbsCbs bool) fiscalExecutionSummary {
	summary := fiscalExecutionSummary{Total: len(itens)}
	var mu sync.Mutex
	sem := make(chan struct{}, 5)
	var wg sync.WaitGroup

	for _, item := range itens {
		wg.Add(1)
		sem <- struct{}{}
		go func(it fiscalItemInput) {
			defer wg.Done()
			defer func() { <-sem }()
			defer func() {
				if rec := recover(); rec != nil {
					log.Printf("FiscalExecutionRunHandler: item=%s panic recuperado: %v", it.ID, rec)
					trace.add(it.ID, it.XProd, "erro", fmt.Sprintf("Panic recuperado: %v", rec))
					if perr := persistFiscalItemResult(pgDB, companyID, it.ID, "error",
						"Falha inesperada ao processar o item.", "", nil, nil); perr != nil {
						log.Printf("FiscalExecutionRunHandler: item=%s persist error after panic: %v", it.ID, perr)
					}
					mu.Lock()
					summary.Error++
					mu.Unlock()
				}
			}()

			// Timeout POR ITEM (15s; 30s com simulação — são 2 chamadas Oracle),
			// não para o lote inteiro (T-11-16).
			itemTimeout := 15 * time.Second
			if simularIbsCbs {
				itemTimeout = 30 * time.Second
			}
			itemCtx, cancel := context.WithTimeout(ctx, itemTimeout)
			defer cancel()

			status := processSingleFiscalItem(itemCtx, oracleDB, pgDB, companyID, nfe, it, trace, simularIbsCbs)
			mu.Lock()
			switch status {
			case "ok":
				summary.OK++
			case "sem_grupo_fiscal":
				summary.SemGrupoFiscal++
			default:
				summary.Error++
			}
			mu.Unlock()
		}(item)
	}
	wg.Wait()
	return summary
}

// processSingleFiscalItem executa o pipeline (lookup grupo fiscal → pacote
// fiscal → persistência) para um único item, isolando qualquer falha nesse
// item — nunca aborta os demais itens do lote (T-11-17).
func processSingleFiscalItem(ctx context.Context, oracleDB *sql.DB, pgDB *sql.DB, companyID string, nfe fiscalNotaContext, it fiscalItemInput, trace *fiscalDebugTrace, simularIbsCbs bool) string {
	produtoLabel := fmt.Sprintf("%s — %s", it.CProd, it.XProd)
	trace.add(it.ID, produtoLabel, "inicio", fmt.Sprintf("Processando item (CFOP %s, v_prod %.2f)", it.CFOP, it.VProd))

	if nfe.CodEmpresaErr != nil {
		log.Printf("FiscalExecutionRunHandler: item=%s err=%v", it.ID, nfe.CodEmpresaErr)
		trace.add(it.ID, produtoLabel, "erro", "Não foi possível determinar a filial (cod_empresa) do emitente.")
		if perr := persistFiscalItemResult(pgDB, companyID, it.ID, "error",
			"Não foi possível determinar a filial (cod_empresa) do emitente para o lookup fiscal.", "", nil, nil); perr != nil {
			log.Printf("FiscalExecutionRunHandler: item=%s persist error: %v", it.ID, perr)
		}
		return "error"
	}

	trace.add(it.ID, produtoLabel, "lookup_grupo_fiscal", fmt.Sprintf("Buscando grupo fiscal em PROD/PRODB (cod_empresa=%d, código XML=%s, código buscado=%s)...", nfe.CodEmpresa, it.CProd, stripCheckDigit(it.CProd)))
	grupoFiscal, _, _, err := lookupGrupoFiscal(ctx, oracleDB, it.CProd, nfe.CodEmpresa)
	if err != nil {
		if errors.Is(err, errSemGrupoFiscal) {
			trace.add(it.ID, produtoLabel, "sem_grupo_fiscal", "Produto não encontrado em PROD/PRODB.")
			if perr := persistFiscalItemResult(pgDB, companyID, it.ID, "sem_grupo_fiscal",
				"Produto não encontrado em PROD/PRODB — grupo fiscal não pôde ser determinado.", "", nil, nil); perr != nil {
				log.Printf("FiscalExecutionRunHandler: item=%s persist error: %v", it.ID, perr)
			}
			return "sem_grupo_fiscal"
		}
		// Nunca propagar err.Error() cru do Oracle na resposta normal (T-11-18) —
		// mas o debug trace é admin-only e efêmero (não persistido em
		// fiscal_execution_items), então inclui o detalhe sanitizado (truncado,
		// sem newline) para diagnóstico direto na tela sem precisar de SSH/logs.
		log.Printf("FiscalExecutionRunHandler: item=%s err=%v", it.ID, err)
		trace.add(it.ID, produtoLabel, "erro", "Falha ao consultar o grupo fiscal no Oracle (prod/PRODB): "+sanitizeOracleErrForDebug(err))
		if perr := persistFiscalItemResult(pgDB, companyID, it.ID, "error",
			"Falha ao consultar o grupo fiscal no Oracle (prod/PRODB).", "", nil, nil); perr != nil {
			log.Printf("FiscalExecutionRunHandler: item=%s persist error: %v", it.ID, perr)
		}
		return "error"
	}
	trace.add(it.ID, produtoLabel, "grupo_fiscal_resolvido", fmt.Sprintf("Grupo fiscal resolvido: %s", grupoFiscal))

	// Mapeamento verificado contra FB_TESTESFC fiscal_execution.go:285-309 —
	// pUFOrigem<-emit_uf, pUFDestino<-dest_uf (NÃO emit_uf), pCodigoIbge<-
	// dest_c_mun (código IBGE do DESTINO — NUNCA emit_municipio, que é o NOME
	// do município) (T-11-19). pDespesas<-v_outro por item: o original
	// FB_TESTESFC hardcodava 0 porque seu schema não tinha despesas
	// acessórias por item; o Plan 11-02 (migration 146 + TPF-02) adicionou
	// v_outro a nfe_saidas_itens especificamente para alimentar este campo —
	// hardcodar 0 aqui descartaria silenciosamente essa despesa da base de
	// cálculo.
	in := services.FiscalInput{
		PCnpjEmpresa:      nfe.EmitCNPJ,
		PUFOrigem:         nfe.EmitUF,
		PUFDestino:        nfe.DestUF,
		PTipoContribuinte: tipoContribuinte(nfe.DestIndIE, it.CFOP, nfe.Modelo),
		PTipoCentroFiscal: defaultTipoCentroFiscal,
		PTipoOperacao:     tipoOperacaoPorCFOP(it.CFOP),
		PEntradaSaida:     "S", // módulo cobre apenas NF-e de saída
		// Sem o dígito verificador, como em PROD/PRODB — o pacote valida o
		// produto nas mesmas tabelas do lookup; com o código cheio do XML o
		// Oracle rejeita com ORA-20000 "Código produto informado não existe
		// no SFC" (confirmado em execução real, 2026-07-06).
		PProduto:           stripCheckDigit(it.CProd),
		PCodigoGrupoFiscal: grupoFiscal,
		PCnpjExcecao:       "",
		PIndicadorServico:  defaultIndicadorServico,
		PPrecoTotal:        it.VProd,
		// Despesas acessórias = frete destacado no item (<prod><vFrete>) +
		// outras despesas (<prod><vOutro>) — ambos compõem a base de cálculo
		// (regra do negócio 2026-07-06: frete do XML entra em pDespesas).
		PDespesas:                    it.VOutro + it.VFrete,
		PDesconto:                    it.VDesc,
		PIPI:                         it.VIPI,
		PAliquotaSimplesNacional:     0,
		FornecedorSimplesNacional:    defaultFornecedorSimplesNacional,
		PTipoIsencaoPedidoBonificado: "",
		PCFOPOperacao:                it.CFOP,
		PTipoContribuinteSecundario:  "",
		PSimulacaoCalculo:            "N",
		PDataReferenciaFiscal:        &nfe.DataEmissao,
		PCodigoIbge:                  nfe.DestCMun,
	}

	inputJSON, marshalErr := json.Marshal(in)
	if marshalErr != nil {
		inputJSON = []byte("{}")
	}

	trace.add(it.ID, produtoLabel, "chamando_pacote", fmt.Sprintf("Executando PKG_FISCAL_FCTAX.calcula_imposto_produto com: %s [pDespesas = frete %.2f + outras %.2f]", in.FormatParams(), it.VFrete, it.VOutro))
	result, callErr := services.CallFiscalPackage(ctx, oracleDB, in)
	if callErr != nil {
		// Nunca propagar callErr.Error() cru do Oracle na resposta normal
		// (T-11-18) — debug trace é admin-only/efêmero, inclui detalhe sanitizado.
		log.Printf("FiscalExecutionRunHandler: item=%s err=%v", it.ID, callErr)
		trace.add(it.ID, produtoLabel, "erro", "Falha ao executar o pacote fiscal no Oracle: "+sanitizeOracleErrForDebug(callErr))
		if perr := persistFiscalItemResult(pgDB, companyID, it.ID, "error",
			"Falha ao executar o pacote fiscal no Oracle (FCCORP_BKP).", grupoFiscal, inputJSON, nil); perr != nil {
			log.Printf("FiscalExecutionRunHandler: item=%s persist error: %v", it.ID, perr)
		}
		return "error"
	}

	if perr := persistFiscalItemResult(pgDB, companyID, it.ID, "ok", "", grupoFiscal, inputJSON, result); perr != nil {
		log.Printf("FiscalExecutionRunHandler: item=%s persist error: %v", it.ID, perr)
		trace.add(it.ID, produtoLabel, "erro", "Falha ao persistir o resultado.")
		return "error"
	}

	// Simulação IBS/CBS na base (2ª chamada) — nunca muda o status do item:
	// o resultado normal já está persistido; a simulação é anexada à parte.
	if simularIbsCbs {
		sim := runSimulacaoIbsCbs(ctx, oracleDB, in, it, result, trace, produtoLabel)
		if simJSON, mErr := json.Marshal(sim); mErr == nil {
			if _, uErr := pgDB.Exec(`UPDATE fiscal_execution_items SET simulacao = $1 WHERE nfe_item_id = $2`, simJSON, it.ID); uErr != nil {
				log.Printf("FiscalExecutionRunHandler: item=%s persist simulacao error: %v", it.ID, uErr)
			}
		}
	}

	trace.add(it.ID, produtoLabel, "concluido", "Item calculado com sucesso (status ok).")
	return "ok"
}

// persistFiscalItemResult grava (ou atualiza, se já existir) o resultado do
// item em fiscal_execution_items. Cada item é sua própria unidade de
// trabalho — INSERT ... ON CONFLICT (nfe_item_id) DO UPDATE — nunca uma
// transação única para o lote inteiro (TPF-05/D-04).
func persistFiscalItemResult(pgDB *sql.DB, companyID, nfeItemID, status, errMsg, grupoFiscalCodigo string, inputParams []byte, result *services.FiscalResult) error {
	fullResultJSON := []byte(`{}`)
	var baseICMS, valorICMS, baseST, valorST *float64
	var basePIS, valorPIS, baseCOFINS, valorCOFINS *float64
	var percDifal, valorPartilhaDest, valorPobreza *float64
	var valorIbsUF, valorIbsMun, valorCbs *float64

	if result != nil {
		if b, mErr := json.Marshal(result); mErr == nil {
			fullResultJSON = b
		}
		baseICMS = &result.BaseCalculo
		valorICMS = &result.ValorImposto
		baseST = &result.BaseSubstituicao
		valorST = &result.ValorSubstituicao
		// PIS/COFINS SEM ICMS na base (tese da exclusão do ICMS — regra do
		// negócio 2026-07-06): é o que a NF-e declara, então é o comparável.
		// As variantes com ICMS continuam disponíveis no full_result JSON.
		basePIS = &result.BaseCalculoPISSemIcms
		valorPIS = &result.ValorPISSemIcms
		baseCOFINS = &result.BaseCalculoCOFINSSemIcms
		valorCOFINS = &result.ValorCOFINSSemIcms
		percDifal = &result.PercentualDifal
		valorPartilhaDest = &result.ValorIcmsPartilhaDestino
		valorPobreza = &result.ValorIcmsPobreza
		valorIbsUF = &result.ValorIbsUF
		valorIbsMun = &result.ValorIbsMUN
		valorCbs = &result.ValorCbs
	}

	var errMsgSQL, grupoFiscalSQL interface{}
	if errMsg != "" {
		errMsgSQL = errMsg
	}
	if grupoFiscalCodigo != "" {
		grupoFiscalSQL = grupoFiscalCodigo
	}
	var inputParamsSQL interface{}
	if len(inputParams) > 0 {
		inputParamsSQL = inputParams
	}

	_, err := pgDB.Exec(`
		INSERT INTO fiscal_execution_items (
			company_id, nfe_item_id, status, error_message, executed_at,
			grupo_fiscal_codigo, input_params,
			base_calculo_icms, valor_icms, base_substituicao, valor_substituicao,
			base_calculo_pis, valor_pis, base_calculo_cofins, valor_cofins,
			percentual_difal, valor_icms_partilha_destino, valor_icms_pobreza,
			valor_ibs_uf, valor_ibs_mun, valor_cbs,
			full_result
		) VALUES (
			$1, $2, $3, $4, NOW(),
			$5, $6,
			$7, $8, $9, $10,
			$11, $12, $13, $14,
			$15, $16, $17,
			$18, $19, $20,
			$21
		)
		ON CONFLICT (nfe_item_id) DO UPDATE SET
			status                      = EXCLUDED.status,
			error_message               = EXCLUDED.error_message,
			executed_at                 = EXCLUDED.executed_at,
			grupo_fiscal_codigo         = EXCLUDED.grupo_fiscal_codigo,
			input_params                = EXCLUDED.input_params,
			base_calculo_icms           = EXCLUDED.base_calculo_icms,
			valor_icms                  = EXCLUDED.valor_icms,
			base_substituicao           = EXCLUDED.base_substituicao,
			valor_substituicao          = EXCLUDED.valor_substituicao,
			base_calculo_pis            = EXCLUDED.base_calculo_pis,
			valor_pis                   = EXCLUDED.valor_pis,
			base_calculo_cofins         = EXCLUDED.base_calculo_cofins,
			valor_cofins                = EXCLUDED.valor_cofins,
			percentual_difal            = EXCLUDED.percentual_difal,
			valor_icms_partilha_destino = EXCLUDED.valor_icms_partilha_destino,
			valor_icms_pobreza          = EXCLUDED.valor_icms_pobreza,
			valor_ibs_uf                = EXCLUDED.valor_ibs_uf,
			valor_ibs_mun               = EXCLUDED.valor_ibs_mun,
			valor_cbs                   = EXCLUDED.valor_cbs,
			full_result                 = EXCLUDED.full_result,
			simulacao                   = NULL
	`,
		companyID, nfeItemID, status, errMsgSQL,
		grupoFiscalSQL, inputParamsSQL,
		baseICMS, valorICMS, baseST, valorST,
		basePIS, valorPIS, baseCOFINS, valorCOFINS,
		percDifal, valorPartilhaDest, valorPobreza,
		valorIbsUF, valorIbsMun, valorCbs,
		fullResultJSON,
	)
	return err
}
