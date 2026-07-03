---
phase: 11-motor-de-execu-o-do-pacote-fiscal-backend
plan: 05
subsystem: api
tags: [go, oracle, concurrency, semaphore, batch, upsert]

# Dependency graph
requires:
  - phase: 11-01
    provides: "openFiscalOracleConn(db, companyID) — conexão Oracle síncrona dedicada (SetMaxOpenConns(5))"
  - phase: 11-03
    provides: "resolveCodEmpresa/lookupGrupoFiscal (errSemGrupoFiscal não-fatal) + tabela fiscal_execution_items (UNIQUE nfe_item_id)"
  - phase: 11-04
    provides: "services.FiscalInput/FiscalResult + services.CallFiscalPackage(ctx, oracleDB, in) (*FiscalResult, error)"
provides:
  - "POST /api/fiscal/execute — endpoint admin de execução em lote end-to-end (TPF-05)"
  - "processFiscalBatch — fan-out com semáforo cap 5, timeout 15s/item, defer recover() por item"
  - "persistFiscalItemResult — upsert por item em fiscal_execution_items (INSERT ... ON CONFLICT DO UPDATE)"
affects: [12]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Fan-out de goroutines com semáforo chan struct{} cap 5 + sync.WaitGroup + sync.Mutex para agregação de summary — primeiro uso deste padrão no FB_APU04"
    - "context.WithTimeout aplicado por item (15s), não para o lote inteiro — ctx externo do handler é só backstop (10min)"
    - "defer recover() por goroutine de item — panic isolado nunca aborta o lote, vira status='error' persistido"
    - "Upsert por item (nunca transação única do lote) — cada persistFiscalItemResult é sua própria unidade de trabalho"

key-files:
  created:
    - backend/handlers/fiscal_execution.go
    - backend/handlers/fiscal_execution_guards_test.go
  modified:
    - backend/main.go

key-decisions:
  - "Guard IDOR aplicado tanto na query de cabeçalho (nfe_saidas WHERE id=$1 AND company_id=$2) quanto na query de itens (nfe_saidas_itens WHERE nfe_id=$1 AND company_id=$2) — o plano só exigia explicitamente no cabeçalho, mas nfe_saidas_itens também carrega company_id desnormalizado (migration 075), então aplicar o mesmo guard ali é defesa em profundidade sem custo adicional"
  - "openFiscalOracleConn reaproveitado tal como já existe em fiscal_oracle_conn.go (Plan 11-01) — não redefinido em fiscal_execution.go, evitando duplicação/conflito de símbolo no package handlers"
  - "persistFiscalItemResult grava também valor_ibs_uf/valor_ibs_mun/valor_cbs (colunas adicionadas em migration 147 além do modelo original FB_TESTESFC) — extensão natural do porte para cobrir as 3 colunas que a Fase 12 (TPF-06) vai precisar"

patterns-established:
  - "FiscalInput mapeado explicitamente campo a campo com comentário citando a verificação contra FB_TESTESFC fiscal_execution.go:285-309 — documentação inline da armadilha IBGE/UF para futuros mantenedores"

requirements-completed: [TPF-05]

# Metrics
duration: ~25min
completed: 2026-07-03
---

# Phase 11 Plan 05: Endpoint de Execução em Lote do Pacote Fiscal Summary

**`POST /api/fiscal/execute` costura conexão Oracle dedicada + lookup de grupo fiscal + chamada do pacote `PKG_FISCAL_FCTAX` + persistência em `fiscal_execution_items`, com fan-out de goroutines (semáforo cap 5), timeout de 15s por item, isolamento de panic e upsert por item — nunca uma transação única do lote.**

## Performance

- **Duration:** ~25 min
- **Tasks:** 2
- **Files modified:** 3 (2 created, 1 modified)

## Accomplishments

- `FiscalExecutionRunHandler`: exige POST (405), resolve `company_id` via JWT/`GetEffectiveCompanyID` (401 sem claims), carrega cabeçalho `nfe_saidas` e itens `nfe_saidas_itens` com guard IDOR duplo (`WHERE id/nfe_id = $1 AND company_id = $2`), abre conexão Oracle dedicada via `openFiscalOracleConn` (Plan 11-01, reaproveitada — não redefinida), dispara `processFiscalBatch` e responde com o summary agregado `{total, ok, sem_grupo_fiscal, error}`.
- `processFiscalBatch`: semáforo `chan struct{}` cap 5, `sync.WaitGroup`, `sync.Mutex` para o summary; por item — `defer wg.Done()` / `defer <-sem` / `defer recover()` (panic isolado vira `status="error"`, nunca aborta o lote) e `context.WithTimeout(ctx, 15*time.Second)` aplicado POR ITEM (não para o lote inteiro).
- `processSingleFiscalItem`: encadeia `resolveCodEmpresa` → `lookupGrupoFiscal` (`errSemGrupoFiscal` → status `sem_grupo_fiscal`, não fatal) → monta `services.FiscalInput` → `services.CallFiscalPackage` → `persistFiscalItemResult`.
- Mapeamento `FiscalInput` verificado campo a campo contra o original `FB_TESTESFC fiscal_execution.go:285-309`: `pUFOrigem<-emit_uf`, `pUFDestino<-dest_uf` (NÃO `emit_uf`), `pCodigoIbge<-dest_c_mun` (código IBGE do DESTINO — NUNCA `emit_municipio`, que é o NOME do município), `pDespesas=0` hardcoded (NF-e não carrega despesas acessórias por item).
- `persistFiscalItemResult`: `INSERT INTO fiscal_execution_items (...) VALUES (...) ON CONFLICT (nfe_item_id) DO UPDATE SET ...` — cada item é seu próprio statement, grava status/error_message/executed_at/grupo_fiscal_codigo/input_params (JSON), colunas típicas (ICMS/ST/PIS/COFINS/DIFAL/FCP) e as 3 colunas IBS/CBS adicionais (`valor_ibs_uf`, `valor_ibs_mun`, `valor_cbs`) + `full_result` JSONB completo.
- Rota `POST /api/fiscal/execute` registrada em `main.go` com `withAuth(handlers.FiscalExecutionRunHandler, "admin")`, no mesmo bloco da rota `oracle-ping` (Plan 11-01).
- Guard tests (`TestFiscalExecution_Guards`) cobrindo 405 (método errado) e 401 (sem auth), passando `nil *sql.DB` — seguindo a convenção `icms_fronteira_st_itens_guards_test.go`, sem testify, sem tocar Oracle/Postgres.

## Task Commits

Each task was committed atomically:

1. **Task 1: Handler de lote + processFiscalBatch + persistFiscalItemResult** - `0986254` (feat)
2. **Task 2: Guard tests (405/401) do endpoint de lote** - `b1ed5fc` (test)

**Plan metadata:** (this commit) `docs(11-05): complete plan`

## Files Created/Modified

- `backend/handlers/fiscal_execution.go` — `FiscalExecutionRunHandler`, `processFiscalBatch`, `processSingleFiscalItem`, `persistFiscalItemResult`, defaults dos parâmetros (novo, 412 linhas)
- `backend/handlers/fiscal_execution_guards_test.go` — `TestFiscalExecution_Guards` (novo)
- `backend/main.go` — registra `POST /api/fiscal/execute` com `withAuth(..., "admin")`

## Decisions Made

- **Guard IDOR duplicado (cabeçalho + itens):** o plano exigia explicitamente `WHERE id = $1 AND company_id = $2` no carregamento do cabeçalho `nfe_saidas`. Como `nfe_saidas_itens` também tem `company_id` desnormalizado (migration 075, "desnormalizado para queries sem JOIN"), o mesmo guard foi aplicado na query de itens (`WHERE nfe_id = $1 AND company_id = $2`) por defesa em profundidade — zero custo adicional e reduz a superfície de um IDOR caso `nfe_id` de outra company acidentalmente colidisse (praticamente impossível com UUID, mas consistente com o T-11-14 do threat model).
- **`openFiscalOracleConn` reaproveitado, não reimplementado:** o Plan 11-01 já criou essa função em `fiscal_oracle_conn.go` no mesmo package `handlers`. O código original do FB_TESTESFC redefine `openFiscalOracleConn` dentro do próprio `fiscal_execution.go` — aqui isso geraria erro de compilação por símbolo duplicado, então a função foi simplesmente chamada, não copiada.
- **Colunas IBS/CBS persistidas:** `persistFiscalItemResult` grava `valor_ibs_uf`/`valor_ibs_mun`/`valor_cbs` além das 11 colunas típicas originais do FB_TESTESFC — essas 3 colunas foram acrescentadas à migration 147 (Plan 11-03) especificamente para a Fase 12 (TPF-06), então persisti-las aqui fecha o ciclo completo já nesta wave.

## Deviations from Plan

None - plan executado exatamente como escrito. As duas decisões acima (guard IDOR duplicado, reaproveitar `openFiscalOracleConn` existente) são aplicações diretas de Rule 2 (funcionalidade crítica de segurança já coberta pelo schema) e Rule 3 (evitar erro de compilação por duplicação), respectivamente — não desvios de escopo.

## Issues Encountered

None. `FB_TESTESFC` ainda estava presente em disco (`/home/claudiobezerra/projetos/FB_TESTESFC/backend/handlers/fiscal_execution.go`), então o arquivo original de 415 linhas foi lido diretamente (não reconstruído a partir de resumos), eliminando risco de deriva na adaptação do mapeamento `FiscalInput`/query IDOR/padrão de concorrência.

## User Setup Required

None — nenhuma configuração externa nova. Testar o endpoint fim a fim contra Oracle real (`erp_bridge_config` populado com credenciais válidas de uma company) é um passo manual futuro (fora do escopo desta fase, que é só a fundação de dados — TPF-06/07/08 na Fase 12 vão consumir `fiscal_execution_items` na tela "Comparação Fiscal").

## Next Phase Readiness

- TPF-01 a TPF-05 completos — a Fase 11 (motor de execução do pacote fiscal, backend) está fechada.
- `fiscal_execution_items` está pronta para ser lida pela Fase 12 (tela "Comparação Fiscal" + filtro divergentes + navegação `adminOnly`), que depende inteiramente desta fase estar executada.
- Gaps conhecidos e aceitos, herdados sem mudança desta wave (documentados em `11-CONTEXT.md`/`11-RESEARCH.md`): (1) `codEmpresaPorCNPJRaiz` só mapeia Recife/PE — notas de outras filiais retornam erro explícito por item, não travam o lote; (2) defaults de parâmetros (`defaultTipoContribuinte` etc.) só validados contra Oracle real no caminho normal de venda — Simples Nacional/serviço podem expor default incorreto, visível como divergência na Fase 12.
- Nenhum bloqueio novo.

---
*Phase: 11-motor-de-execu-o-do-pacote-fiscal-backend*
*Completed: 2026-07-03*

## Self-Check: PASSED

- FOUND: `backend/handlers/fiscal_execution.go`
- FOUND: `backend/handlers/fiscal_execution_guards_test.go`
- FOUND: commit `0986254` in git log
- FOUND: commit `b1ed5fc` in git log
- `cd backend && go build ./...` exits 0
- `cd backend && go vet ./handlers/` exits 0
- `cd backend && go test ./handlers/ -run TestFiscalExecution_Guards -v` PASS
- FiscalInput mapping verified: `PUFOrigem<-EmitUF`, `PUFDestino<-DestUF`, `PCodigoIbge<-DestCMun`, `PDespesas=0` — identical to FB_TESTESFC fiscal_execution.go:285-309
