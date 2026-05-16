---
phase: 03-estabiliza-o-adicional
plan: "03"
subsystem: testing
tags: [vitest, typescript, pure-functions, formatFilial, navigation, frontend]

# Dependency graph
requires:
  - phase: 03-estabiliza-o-adicional
    provides: Infraestrutura Vitest já instalada (vitest@1.6.1, jsdom, testing-library)
provides:
  - 45 testes Vitest cobrindo as 11 funções puras de formatFilial.ts
  - 25 testes Vitest cobrindo todos os branches de getActiveModule() em navigation.ts
  - Padrão de teste table-driven para funções puras estabelecido
affects: [03-estabiliza-o-adicional, planos futuros que adicionam formatadores ou rotas]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Table-driven tests com Array<{input, expected}> iterado com for-of para cobertura de branches"
    - "Import explícito de funções individuais do mesmo diretório (from './formatFilial')"
    - "describe por função + test por caso de borda — sem mocking"

key-files:
  created:
    - frontend/src/lib/formatFilial.test.ts
    - frontend/src/lib/navigation.test.ts
  modified: []

key-decisions:
  - "Corrigir expectativa de formatCNPJMasked: template literal gera **.***.***/filial-dv (3+3 estrelas) — não 4 grupos"
  - "navigation.test.ts usa table-driven com for-of loop para cobrir 25 casos em bloco único legível"
  - "Manter imports explícitos do vitest (describe/test/expect) mesmo com globals:true no config"

patterns-established:
  - "Funções puras testadas sem mocking, sem testing-library, sem Router"
  - "Caso de precedência crítica testado explicitamente: /relatorios/saneamento → notas antes de /relatorios/ → simulador"

requirements-completed:
  - STAB-08

# Metrics
duration: 15min
completed: 2026-05-16
---

# Phase 03 Plan 03: Bootstrap Testes React Vitest Summary

**81 testes Vitest passando (3 utils + 8 ResetDatabaseDialog + 45 formatFilial + 25 navigation) — zero mocking, funções puras cobertas de ponta a ponta**

## Performance

- **Duration:** 15 min
- **Started:** 2026-05-16T15:49:00Z
- **Completed:** 2026-05-16T15:51:00Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- formatFilial.test.ts criado com 45 testes cobrindo as 11 funções exportadas incluindo edge cases: null/undefined, mascaramento, fallbacks de formatFilialFromRow
- navigation.test.ts criado com 25 testes table-driven cobrindo todos os branches de getActiveModule() incluindo o caso crítico de precedência `/relatorios/saneamento` → 'notas'
- Suite completa passou de 11 para 81 testes sem regredir nenhum existente

## Task Commits

1. **Task 1 + Task 2: formatFilial.test.ts e navigation.test.ts** - `913bdab` (feat)

**Plan metadata:** (docs commit a seguir)

## Files Created/Modified

- `frontend/src/lib/formatFilial.test.ts` - 45 testes das 11 funções puras de formatação CNPJ/CPF/filial
- `frontend/src/lib/navigation.test.ts` - 25 testes table-driven de getActiveModule() cobrindo todos os módulos e default

## Decisions Made

- Descoberto que `formatCNPJMasked` gera `**.***.***/${filial}-${dv}` (9 caracteres `*` com 2 pontos = `**.***.***/`), não `**.***.****/` — corrigido nas expectativas dos testes (Rule 1 auto-fix ao executar RED)
- navigation.test.ts usa loop `for...of` sobre array de casos para evitar repetição e garantir legibilidade do relatório de falha

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Corrigido formato esperado de formatCNPJMasked**
- **Found during:** Task 1 (primeira execução RED dos testes formatFilial)
- **Issue:** O template literal `` `**.***.***/${filial}-${dv}` `` gera `**.***.***/0001-90` (com 3 grupos de * separados por `.`), mas os testes inicialmente esperavam `**.***.****/0001-90` (4 asteriscos antes da barra)
- **Fix:** Corrigidas as 5 expectativas afetadas (formatCNPJMasked, formatDocumentoMasked, formatCnpjComApelido x3) para o valor real `**.***.***/`
- **Files modified:** frontend/src/lib/formatFilial.test.ts
- **Verification:** npx vitest run src/lib/formatFilial.test.ts — 45/45 passed
- **Committed in:** 913bdab

---

**Total deviations:** 1 auto-fixed (1 bug — expectativa incorreta baseada em leitura imprecisa do template literal)
**Impact on plan:** Nenhum impacto de escopo. Fix necessário para testes precisos de mascaramento (requisito T-03-03-01 do threat model).

## Issues Encountered

Nenhum além do desvio documentado acima.

## Known Stubs

Nenhum — arquivos de teste não expõem stubs nem dados hardcoded que fluam para UI.

## Threat Flags

Nenhuma nova superfície de segurança introduzida — apenas arquivos de teste.

## Next Phase Readiness

- Padrão de teste para funções puras estabelecido; próximos planos podem seguir o mesmo padrão
- Suite completa (81 testes) serve como baseline de regressão para refatorações futuras de formatFilial e navigation
- Nenhum bloqueador identificado

---
*Phase: 03-estabiliza-o-adicional*
*Completed: 2026-05-16*
