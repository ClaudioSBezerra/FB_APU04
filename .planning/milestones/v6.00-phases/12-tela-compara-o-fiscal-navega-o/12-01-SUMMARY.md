---
phase: 12-tela-compara-o-fiscal-navega-o
plan: 01
subsystem: api
tags: [go, postgresql, csv, jwt, rbac, fiscal]

# Dependency graph
requires:
  - phase: 11-motor-de-execu-o-do-pacote-fiscal-backend
    provides: "fiscal_execution_items (migration 147) + POST /api/fiscal/execute"
provides:
  - "GET /api/fiscal/comparacao/search?q=... — busca NF-e de saída por número/chave, company-scoped, ate 20 candidatos"
  - "GET /api/fiscal/comparacao?nfe_id=... — comparação item a item esperado x calculado (6 impostos), 4º estado not_executed, IBS somado"
  - "GET /api/fiscal/comparacao/csv?nfe_id=... — export CSV do mesmo escopo (sem DIFAL/FCP/full_result)"
  - "queryComparacaoRows helper compartilhada entre o read handler e o CSV handler"
affects: ["12-02-frontend-tela-comparacao"]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "LEFT JOIN nfe_saidas_itens x fiscal_execution_items com COALESCE(status, 'not_executed') para o 4º estado"
    - "Soma de colunas partidas (valor_ibs_uf + valor_ibs_mun) uma única vez em SQL, reusada por JSON e CSV via helper interna"
    - "CSV export via encoding/csv stdlib, mesmo padrão de xml_conciliacao.go (Content-Disposition attachment)"

key-files:
  created:
    - backend/handlers/fiscal_comparacao.go
    - backend/handlers/fiscal_comparacao_csv.go
  modified:
    - backend/main.go

key-decisions:
  - "queryComparacaoRows extraída como helper interna reutilizada pelo JSON read handler e pelo CSV handler — evita duplicar a soma de IBS (Pitfall 2 do RESEARCH.md)"
  - "COALESCE(fei.status, 'not_executed') resolve o 4º estado implícito do LEFT JOIN, distinto de error/sem_grupo_fiscal/pending (Open Question 1 do RESEARCH.md, já resolvida no planejamento)"
  - "CSV export espelha apenas as 6 colunas de impostos x esperado/calculado/diferença — DIFAL/FCP/grupo_fiscal_codigo/full_result ficam fora (dialog-only, Open Question 2 do RESEARCH.md)"
  - "3 rotas registradas em ordem específica-antes-de-genérica (/search, /csv, depois /comparacao) seguindo convenção do mux, mesmo não sendo estritamente necessário para paths exatos sem trailing slash"

patterns-established:
  - "Padrão de comparação esperado x calculado (LEFT JOIN + soma de colunas partidas + estado implícito de 'nunca executado') fica disponível para qualquer futura tela de validação de pacote fiscal"

requirements-completed: [TPF-06, TPF-07]

duration: 20min
completed: 2026-07-03
---

# Phase 12 Plan 01: Backend da Comparação Fiscal Summary

**Três handlers HTTP admin-gated (busca por número/chave, leitura da comparação item a item, export CSV) que expõem `fiscal_execution_items` (Fase 11) comparado a `nfe_saidas_itens`, com o 4º estado "nunca executado" e a soma de IBS resolvidos em SQL único e reutilizado.**

## Performance

- **Duration:** ~20 min (Task 1 já existia em disco, não commitado — verificado contra o plano nesta sessão; Task 2 escrito do zero)
- **Completed:** 2026-07-03
- **Tasks:** 2/2
- **Files modified:** 3 (2 criados, 1 modificado)

## Accomplishments
- Busca de NF-e de saída por número/chave (autocomplete server-side), company-scoped, ILIKE parametrizado, `LIMIT 20`, nunca retorna `null`
- Comparação item a item das 6 impostos (ICMS, ICMS-ST, PIS, COFINS, IBS, CBS) com 4º estado `not_executed` e IBS calculado já somado (`valor_ibs_uf + valor_ibs_mun`)
- Export CSV no mesmo escopo da tabela visível (sem campos dialog-only), reusando a helper de query sem duplicar aritmética
- 3 rotas registradas em `main.go`, todas exigindo role `admin` server-side (nenhuma com `""`)

## Task Commits

Each task was committed atomically:

1. **Task 1: Handlers de busca e leitura da comparação (fiscal_comparacao.go)** - `1f66324` (feat) — arquivo já existia em disco, uncommitted; verificado linha a linha contra o plano (COALESCE not_executed, soma IBS, guard IDOR duplo, helper `queryComparacaoRows`) e commitado sem alterações
2. **Task 2: Handler CSV + registro das 3 rotas admin-gated** - `91e7ed9` (feat)

**Plan metadata:** (this commit) `docs(12-01): complete Backend da Comparação Fiscal plan`

## Files Created/Modified
- `backend/handlers/fiscal_comparacao.go` - `FiscalComparacaoSearchHandler` (busca) + `FiscalComparacaoReadHandler` (leitura) + `queryComparacaoRows` (helper interna compartilhada)
- `backend/handlers/fiscal_comparacao_csv.go` - `FiscalComparacaoCSVHandler` (export CSV, reusa a helper acima)
- `backend/main.go` - 3 rotas registradas: `/api/fiscal/comparacao/search`, `/api/fiscal/comparacao/csv`, `/api/fiscal/comparacao`, todas `withAuth(..., "admin")`

## Decisions Made
- **queryComparacaoRows como helper interna reutilizável** — evita reimplementar a soma de IBS no handler CSV (Pitfall 2 do RESEARCH.md)
- **COALESCE(fei.status, 'not_executed')** — 4º estado explícito, não colapsado em "Não calculado" (resolução já travada no planejamento)
- **CSV sem DIFAL/FCP/grupo_fiscal_codigo/full_result** — escopo idêntico à tabela visível da tela (Plan 12-02), campos extras ficam no dialog de detalhe

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Comentário do arquivo CSV continha os nomes literais das colunas excluídas, quebrando o próprio gate de verificação do plano**
- **Found during:** Task 2 (verificação automatizada pós-implementação)
- **Issue:** O header comment de `fiscal_comparacao_csv.go` documentava a exclusão de DIFAL/FCP citando os identificadores literais `percentual_difal` e `valor_icms_pobreza` — o grep de verificação do plano (`! grep -qi "valor_icms_pobreza\|percentual_difal" fiscal_comparacao_csv.go`) não distingue comentário de código, então o próprio arquivo falhava seu gate de conformidade mesmo sem essas colunas aparecerem na query/CSV real
- **Fix:** Reescrita do comentário para descrever a exclusão sem citar os identificadores de coluna litealmente (referência genérica a "DIFAL, FCP, código do grupo fiscal e resultado completo")
- **Files modified:** backend/handlers/fiscal_comparacao_csv.go
- **Verification:** `grep -qi "valor_icms_pobreza\|percentual_difal" handlers/fiscal_comparacao_csv.go` retorna vazio; `go build ./...` e `go vet ./handlers/` seguem limpos
- **Committed in:** 91e7ed9 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Correção cosmética em comentário, sem impacto funcional. Nenhum scope creep.

## Issues Encountered
None além do deviation acima.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Plan 12-02 (frontend React) pode consumir os 3 endpoints diretamente: `GET /api/fiscal/comparacao/search`, `GET /api/fiscal/comparacao`, `GET /api/fiscal/comparacao/csv` — todos já retornam o shape documentado em `NfeSearchResult`/`ComparacaoRow` (ver `fiscal_comparacao.go`)
- Nenhum bloqueio conhecido. Validação end-to-end com dados Oracle reais (execução do pacote fiscal) ainda depende da credencial Oracle real, já sinalizada como pendente desde a Fase 11 (11-06-SUMMARY.md)

---
*Phase: 12-tela-compara-o-fiscal-navega-o*
*Completed: 2026-07-03*

## Self-Check: PASSED

All created files exist on disk (backend/handlers/fiscal_comparacao.go, backend/handlers/fiscal_comparacao_csv.go, backend/main.go, this SUMMARY.md) and both task commits (1f66324, 91e7ed9) are present in git log.
