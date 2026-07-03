---
phase: 11-motor-de-execu-o-do-pacote-fiscal-backend
plan: 02
subsystem: api
tags: [migration, nfe-parsing, postgres, xml]

# Dependency graph
requires: []
provides:
  - "v_desc/v_outro (NUMERIC(15,2) DEFAULT 0) em nfe_saidas_itens e nfe_entradas_itens"
  - "struct prod (nfe_saidas.go) parseia VOutro (xml:\"vOutro\") item-level, ao lado de VDesc"
  - "insertNFeItens grava e atualiza v_desc/v_outro por item (ON CONFLICT DO UPDATE)"
affects: [11-03, 11-04, 11-05]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Migration idempotente dual-table (ADD COLUMN IF NOT EXISTS em nfe_saidas_itens E nfe_entradas_itens) — segue convenção 094/095/141, motivada aqui pela obrigatoriedade técnica de insertNFeItens ser compartilhado entre as duas tabelas"

key-files:
  created:
    - backend/migrations/146_nfe_itens_desc_outro.sql
  modified:
    - backend/handlers/nfe_saidas.go

key-decisions:
  - "Migration cobre nfe_entradas_itens além de nfe_saidas_itens (requisito direto de TPF-02): insertNFeItens usa o mesmo texto SQL parametrizado para as duas tabelas (tableName é o único diferencial), então sem as colunas em nfe_entradas_itens o INSERT de entradas quebraria em runtime assim que v_desc/v_outro fossem adicionados ao SQL"

requirements-completed: [TPF-02]

# Metrics
duration: ~15min
completed: 2026-07-03
---

# Phase 11 Plan 02: Persistência de Desconto/Despesas por Item (TPF-02) Summary

**Migration 146 adiciona v_desc/v_outro (NUMERIC(15,2) DEFAULT 0) a nfe_saidas_itens e nfe_entradas_itens; struct `prod` passa a parsear `vOutro` item-level e `insertNFeItens` grava/atualiza ambos os campos via ON CONFLICT DO UPDATE — fecha o único gap de dados de entrada identificado para os 23 parâmetros IN do pacote fiscal (`pDesconto`/`pDespesas`).**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-07-03T16:41:00Z
- **Completed:** 2026-07-03T16:42:47Z
- **Tasks:** 2 (both `type="auto"`)
- **Files modified:** 2 (1 new migration, 1 handler file)

## Accomplishments
- Criada `backend/migrations/146_nfe_itens_desc_outro.sql`, idempotente (`ADD COLUMN IF NOT EXISTS`), adicionando `v_desc`/`v_outro` (`NUMERIC(15,2) DEFAULT 0`) a `nfe_saidas_itens` (requisito direto TPF-02) e `nfe_entradas_itens` (obrigatório pela simetria técnica de `insertNFeItens` compartilhado)
- Struct `prod` (`backend/handlers/nfe_saidas.go`) ganhou `VOutro string \`xml:"vOutro"\`` ao lado do já existente `VDesc`
- `insertNFeItens` estendido: `v_desc`/`v_outro` adicionados à lista de colunas, placeholders `$23`/`$24`, `ON CONFLICT (nfe_id, n_item) DO UPDATE SET v_desc = EXCLUDED.v_desc, v_outro = EXCLUDED.v_outro`, e args finais `toDecimal(d.Prod.VDesc), toDecimal(d.Prod.VOutro)`
- `cd backend && go build ./...` e `go vet ./handlers/` passam sem erros; `gofmt` realinhou o struct `prod` automaticamente

## Task Commits

Each task was committed atomically:

1. **Task 1: Migration 146 — v_desc/v_outro em nfe_saidas_itens E nfe_entradas_itens** — `7cbe9a9` (feat)
2. **Task 2: Parsear vOutro e persistir v_desc/v_outro em insertNFeItens** — `82f6d83` (feat)

**Plan metadata:** (this commit) `docs(11-02): complete plan`

## Files Created/Modified
- `backend/migrations/146_nfe_itens_desc_outro.sql` — novo, colunas v_desc/v_outro nas duas tabelas de itens
- `backend/handlers/nfe_saidas.go` — struct `prod` (campo VOutro) + `insertNFeItens` (INSERT/ON CONFLICT/args)

## Decisions Made
- Nenhuma decisão nova além das já capturadas no plano/pesquisa (D-02, resolução da Open Question #3 da pesquisa: migration dual-table adotada, mantendo convenção 094/095/141).

## Deviations from Plan

None - plan executado exatamente como escrito. `gofmt` re-alinhou o `struct prod` (whitespace apenas, sem mudança semântica) — não conta como desvio de conteúdo.

## Issues Encountered
None.

## User Setup Required
None — migration 146 roda automaticamente via o runner de migrations já existente (`onDBConnected()`/`filepath.Glob("*.sql")`) no próximo deploy/restart do backend. Nenhuma ação manual necessária.

## Next Phase Readiness
- TPF-02 atendido: `v_desc`/`v_outro` disponíveis por item em `nfe_saidas_itens`, prontos para servir como `pDesconto`/`pDespesas` na Fase 11 Plan 03+ (execução do pacote fiscal via PL/SQL).
- Novos uploads/reimports de XML (via `xml_upload.go`, que já chama `insertNFeItens`) passam a popular `v_desc`/`v_outro` automaticamente sem qualquer mudança adicional nesses fluxos.
- Notas importadas antes desta migration têm `v_desc`/`v_outro = 0` (DEFAULT) até serem reimportadas — comportamento aceitável, consistente com o padrão já usado em 141 (v_bc_st/v_st).
- No blockers.

---
*Phase: 11-motor-de-execu-o-do-pacote-fiscal-backend*
*Completed: 2026-07-03*

## Self-Check: PASSED

- FOUND: `backend/migrations/146_nfe_itens_desc_outro.sql`
- FOUND: `backend/handlers/nfe_saidas.go` contains `xml:"vOutro"` and `v_outro`
- FOUND: commit `7cbe9a9` in git log
- FOUND: commit `82f6d83` in git log
- `cd backend && go build ./...` exits 0
