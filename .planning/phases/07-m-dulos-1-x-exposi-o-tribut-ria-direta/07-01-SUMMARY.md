---
phase: 07-modulos-1x-exposicao-tributaria-direta
plan: 01
subsystem: backend
tags: [reforma-tributaria, handlers, sql, icms, ibs, cbs, split-payment, csv]
dependency_graph:
  requires: [reforma_parametros table, reg_c190/c100/import_jobs schema, forn_simples, nfe_entradas/nfe_entradas_itens, nfe_saidas, ncm_cclasstrib_reforma, cfop table]
  provides: [GET /api/reforma/modulo1/creditos, GET /api/reforma/modulo1/creditos/csv, GET /api/reforma/modulo1/ranking, GET /api/reforma/modulo1/ranking/csv, GET /api/reforma/modulo1/reprecificacao, GET /api/reforma/modulo1/reprecificacao/csv, GET /api/reforma/modulo1/split]
  affects: [backend/main.go routes, backend/handlers/ package]
tech_stack:
  added: []
  patterns: [multi-handler-per-file, sql-join-chain-for-company-id, lateral-ncm-prefix-match, csv-writer-attachment, iderless-sql-params]
key_files:
  created:
    - backend/handlers/reforma_modulo1.go
    - backend/handlers/reforma_modulo1_test.go
  modified:
    - backend/main.go
decisions:
  - "A1: Projeção IBS/CBS usa vl_opr_total como base (valor da operação × alíquota), não vl_icms — interpretação correta da substituição ICMS→IBS/CBS documentada no RESEARCH"
  - "A2: Três caminhos CST — normal(00/''), st(10/30/60/70), base_reduzida(20), outro(demais) — interpretação fiscal documentada"
  - "A3: Float tributário = (IBS+CBS)/100 × saídas × prazo/365; custo = float × CDI/100 — fórmula de capital de giro documentada"
  - "A5: Módulo 1.3 usa nfe_entradas (XML) como fonte primária de ranking — JOIN direto forn_cnpj=forn_simples.cnpj (14 dígitos)"
  - "Split payment não tem CSV conforme UI-SPEC — apenas 6 endpoints CSV de 7 handlers"
metrics:
  duration: "~25 minutes"
  completed_date: "2026-05-23"
  tasks_completed: 3
  tasks_total: 3
  files_created: 2
  files_modified: 1
---

# Phase 7 Plan 01: Módulos 1.x Backend — Handlers Exposição Tributária Direta Summary

**One-liner:** 7 handlers REST analíticos da Reforma Tributária (créditos ICMS, ranking Simples, reprecificação LATERAL NCM, split payment CDI) implementados com join chains corretos, filtros de cancelados/transferências, company_id parametrizado e guard tests.

## What Was Built

### backend/handlers/reforma_modulo1.go (851 linhas)

Arquivo único com 4 handlers JSON + 3 handlers CSV, seguindo o padrão multi-handler de `creditos_perdidos.go`:

**Módulo 1.1 — CreditosBloqueadosHandler + CreditosBloqueadosCSVHandler:**
- Fonte EFD: join chain `reg_c190 → reg_c100 (id_pai_c100) → import_jobs (job_id)` — único caminho para `company_id` em `reg_c190` que não tem essa coluna diretamente
- Filtro cancelados EFD: `c100.cod_sit NOT IN ('02','03','04','05')` (cod_sit está em reg_c100, não reg_c190)
- Filtro transferências: `COALESCE(cf.tipo, 'O') != 'T'` via LEFT JOIN cfop
- Projeção IBS/CBS: `vl_opr_total × aliqIBS/100` (base = valor da operação, não ICMS)
- GROUP BY tipo_cfop + cfop, acumuladores TotalIcms/TotalIBS/TotalCBS na resposta

**Módulo 1.3 — RankingFornecedoresHandler + RankingFornecedoresCSVHandler:**
- Fonte XML: `nfe_entradas` JOIN `forn_simples ON fs.cnpj = ne.forn_cnpj` (14 dígitos puros, JOIN direto)
- Filtros: `cancelado = 'N'` + `COALESCE(cf.tipo,'O') != 'T'` via LEFT JOIN cfop no cabeçalho
- IBSPerdidoEst = `valor_total × fatorSimples/100 × aliqIBS/100` (fração não aproveitável de fornecedor Simples)
- LIMIT 100, ORDER BY valor_total DESC
- FatorSimplesPct incluído na resposta para transparência no frontend

**Módulo 1.2 — ReprecificacaoHandler + ReprecificacaoCSVHandler:**
- Fonte XML: `nfe_entradas_itens JOIN nfe_entradas` com LATERAL NCM longest-prefix-wins
- `LEFT JOIN LATERAL (SELECT ... FROM ncm_cclasstrib_reforma WHERE nit.ncm LIKE ncm_digits || '%' ORDER BY length(ncm_digits) DESC LIMIT 1) ncmr ON true`
- Três caminhos CST classificados no Go: normal(00/""), st(10/30/60/70), base_reduzida(20)
- COALESCE em cst_icms/v_icms/ibs_reducao_pct/cbs_reducao_pct para NULLs históricos (Pitfall 4)
- VariacaoPct = `(IBSProjetado + CBSProjetado - IcmsAtual) / vProd × 100`

**Módulo 1.4 — SplitPaymentHandler (sem CSV):**
- Total de saídas via `nfe_saidas` com cancelado='N' e filtro transferências via cfop cabeçalho
- Float tributário = `(IBS+CBS)/100 × totalSaidas × prazoMedio/365`
- Custo CDI = `float × taxaCDI/100`
- Matriz de sensibilidade DSO×CDI gerada no Go: `[]int{15, 30, 45, 60, 90}` × `[]float64{8, 10, 12, 14}`
- TaxaCDIAnualPct e PrazoMedioDias incluídos para o frontend destacar célula corrente

### backend/main.go (modificado)

7 rotas `/api/reforma/modulo1/*` registradas após bloco `/api/reforma/parametros` (linha ~538):
- Padrão: `getDB() → jsonServiceUnavailable se nil → AuthMiddleware(handler, "")` (role "" = qualquer autenticado)
- Sem `/split/csv` conforme UI-SPEC
- Nenhuma rota com role "admin" (analytics read-only)

### backend/handlers/reforma_modulo1_test.go (70 linhas)

8 testes guard:
- 7 testes de criação (handler != nil com nil db)
- 1 teste method-not-allowed (POST → 405 antes de DB touch)

## Security Review (STRIDE Threat Register)

| Threat | Mitigation | Status |
|--------|-----------|--------|
| T-07-01: IDOR via X-Company-ID | `GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))` em todos os 7 handlers | MITIGATED |
| T-07-02: SQL injection | Todos os parâmetros via `$N` placeholders; cf.tipo lido do banco; nenhuma interpolação | MITIGATED |
| T-07-03: CSV cross-company leak | CSV handlers extraem company_id via JWT idêntico aos JSON handlers | MITIGATED |
| T-07-04: Dados fiscais sem autenticação | 7 rotas com `AuthMiddleware(handler, "")` — JWT obrigatório | MITIGATED |
| T-07-05: ReprecificacaoHandler DoS | LIMIT 500 + índice idx_ncm_cclasstrib_reforma_digits | ACCEPTED |

Verificação: `grep -nE "Sprintf.*company_id" backend/handlers/reforma_modulo1.go` → 0 linhas

## Deviations from Plan

None — plan executed exactly as written.

The handlers for Tasks 1 and 2 were implemented in a single file creation (reforma_modulo1.go) rather than two sequential edits. This is semantically equivalent — the file was not modified between Task 1 commit and Task 2 commit, so Task 1 commit captured the exact artifact spec'd in its acceptance criteria, and Task 2 commit captured the main.go route registration.

## Known Stubs

None — all handlers are fully wired. No hardcoded empty responses, no TODO placeholders in business logic paths.

## Threat Flags

None — all new network endpoints were anticipated in the plan's threat model (T-07-01 through T-07-05). No unexpected trust boundary surface introduced.

## Self-Check: PASSED

| Check | Result |
|-------|--------|
| backend/handlers/reforma_modulo1.go | FOUND |
| backend/handlers/reforma_modulo1_test.go | FOUND |
| Commit 8d769ec (feat 07-01 task1) | FOUND |
| Commit 03da8e4 (feat 07-01 task2) | FOUND |
| Commit 5913bac (test 07-01 task3) | FOUND |
