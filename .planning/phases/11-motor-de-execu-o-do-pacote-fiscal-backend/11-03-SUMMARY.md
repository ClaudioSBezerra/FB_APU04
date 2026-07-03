---
phase: 11-motor-de-execu-o-do-pacote-fiscal-backend
plan: 03
subsystem: database
tags: [go, oracle, sql-named, postgresql, jsonb, migration]

# Dependency graph
requires:
  - phase: 11-01
    provides: conexão Oracle síncrona dedicada (fiscal_oracle_conn.go) que o
      futuro Plan 11-05 usará para abrir oracleDB e chamar lookupGrupoFiscal
  - phase: 11-02
    provides: v_desc/v_outro persistidos em nfe_saidas_itens/nfe_entradas_itens
      (parâmetros pDespesas/pDesconto do pacote fiscal)
provides:
  - resolveCodEmpresa (mapa CNPJ raiz → cod_empresa, erro explícito p/ filial
    não mapeada)
  - lookupGrupoFiscal (query Oracle prod/PRODB via sql.Named, sql.ErrNoRows →
    errSemGrupoFiscal não-fatal)
  - tabela fiscal_execution_items (schema híbrido: 11 colunas típicas + 3
    colunas IBS/CBS + full_result JSONB, UNIQUE(nfe_item_id) para upsert)
affects: [11-04, 11-05, 12]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "sql.Named para binds Oracle (zero concatenação de string em query)"
    - "erro explícito em vez de valor adivinhado (resolveCodEmpresa)"
    - "modelo híbrido de persistência: colunas típicas indexáveis + JSONB
      completo para auditoria (fiscal_execution_items)"

key-files:
  created:
    - backend/handlers/fiscal_group_lookup.go
    - backend/migrations/147_fiscal_execution_items.sql
  modified: []

key-decisions:
  - "Porte verbatim do fiscal_group_lookup.go do FB_TESTESFC, apenas removendo
    a redefinição de onlyDigits (já existe em icms_fronteira_prodepe.go)"
  - "Modelo híbrido (11 colunas típicas + full_result JSONB) em vez de 88
    colunas literais — replica o schema validado contra Oracle real no
    FB_TESTESFC, com 3 colunas adicionais valor_ibs_uf/valor_ibs_mun/valor_cbs
    para a Fase 12 (TPF-06)"

patterns-established:
  - "Lookup Oracle sempre via sql.Named, nunca fmt.Sprintf/concatenação"
  - "cod_empresa nunca adivinhado — erro explícito citando raiz+UF quando
    filial não mapeada"
  - "fiscal_execution_items.nfe_item_id UNIQUE habilita upsert por item
    (INSERT ... ON CONFLICT DO UPDATE), nunca uma transação única de lote"

requirements-completed: [TPF-01, TPF-04]

# Metrics
duration: 12min
completed: 2026-07-03
---

# Phase 11 Plan 03: Lookup de Grupo Fiscal + Tabela fiscal_execution_items Summary

**Lookup de grupo fiscal via Oracle prod/PRODB com erro explícito para filial não mapeada, e tabela fiscal_execution_items no modelo híbrido (colunas típicas + JSONB) com colunas IBS/CBS para a Reforma Tributária**

## Performance

- **Duration:** 12 min
- **Started:** 2026-07-03T16:45:17Z
- **Completed:** 2026-07-03T16:47:50Z
- **Tasks:** 2
- **Files modified:** 2 (both new)

## Accomplishments
- `resolveCodEmpresa` + `lookupGrupoFiscal` portados verbatim do validador FB_TESTESFC (já validado contra Oracle real), reusando `onlyDigits` existente em vez de redefini-lo
- Migration 147 cria `fiscal_execution_items` no modelo híbrido validado (11 colunas típicas indexáveis + `full_result` JSONB) acrescido de `valor_ibs_uf`/`valor_ibs_mun`/`valor_cbs` para a Fase 12
- FK `nfe_item_id → nfe_saidas_itens(id) ON DELETE CASCADE` e `company_id → companies(id) ON DELETE CASCADE`, com `UNIQUE(nfe_item_id)` habilitando upsert por item (TPF-05)

## Task Commits

Each task was committed atomically:

1. **Task 1: Portar fiscal_group_lookup.go (resolveCodEmpresa + lookupGrupoFiscal)** - `44cb23c` (feat)
2. **Task 2: Migration 147 — tabela fiscal_execution_items (schema híbrido + IBS/CBS)** - `b27915c` (feat)

**Plan metadata:** (this commit) - docs: complete plan

## Files Created/Modified
- `backend/handlers/fiscal_group_lookup.go` - `resolveCodEmpresa`, `lookupGrupoFiscal`, `errSemGrupoFiscal`, `codEmpresaPorCNPJRaiz`
- `backend/migrations/147_fiscal_execution_items.sql` - `CREATE TABLE IF NOT EXISTS fiscal_execution_items` + 2 índices idempotentes

## Decisions Made
- Nenhum desvio de decisão em relação ao plano — ambas as tarefas foram execução direta das instruções do PLAN.md (porte verbatim + migration híbrida especificada campo a campo).

## Deviations from Plan

None - plan executado exatamente como escrito. Único ajuste foi cosmético (espaçamento de colunas na migration para alinhar exatamente com os greps do `<verify>` do plano — sem mudança de conteúdo/semântica).

## Issues Encountered
None.

## User Setup Required

None - no external service configuration required. A tabela `fiscal_execution_items` será criada automaticamente pelo runner de migrations (`filepath.Glob("*.sql")` em `main.go`) na próxima subida do backend.

## Next Phase Readiness
- TPF-01 e TPF-04 atendidos; `lookupGrupoFiscal`/`resolveCodEmpresa` e `fiscal_execution_items` prontos para consumo pelo endpoint de execução em lote (Plan 11-05, que também depende do serviço `PKG_FISCAL_FCTAX` do Plan 11-04).
- Nenhum bloqueio novo. Gap conhecido e aceito: apenas Recife/PE (`10230480`) está mapeado em `codEmpresaPorCNPJRaiz` — notas de outras filiais retornarão erro explícito por item até confirmação da raiz correspondente contra o Oracle real (comportamento intencional, não um bug).

---
*Phase: 11-motor-de-execu-o-do-pacote-fiscal-backend*
*Completed: 2026-07-03*
