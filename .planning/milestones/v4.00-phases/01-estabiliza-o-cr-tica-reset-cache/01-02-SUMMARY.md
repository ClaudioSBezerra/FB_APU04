---
phase: 01-estabiliza-o-cr-tica-reset-cache
plan: "02"
subsystem: frontend/ui
tags: [react, typescript, testing, alert-dialog, modal, vitest, tdd, admin, reset, destructive]

requires:
  - phase: 01-01
    provides: "ResetDatabaseHandler com gate token DELETE-FB_APU04 + contratos de status HTTP 400/403/429/503/500"

provides:
  - "ResetDatabaseDialog: componente modal de confirmação destrutiva reutilizável exportado de @/components/ResetDatabaseDialog"
  - "ImportarEFD.tsx com botão Zerar Tudo abrindo dialog em vez de fetch direto"
  - "Infraestrutura de testes React: vitest 1.6.x + @testing-library/react 14.x + jsdom 24.x"

affects:
  - "Qualquer plan que queira endurecer ResetCompanyDataHandler (mesmo padrão de dialog)"
  - "STAB-08: bootstrap completo de testes React — vitest config já criado aqui"

tech-stack:
  added:
    - "vitest 1.6.x (Node 18 compat)"
    - "@testing-library/react 14.x"
    - "@testing-library/jest-dom 6.x"
    - "jsdom 24.x"
    - "@testing-library/user-event 14.x"
  patterns:
    - "TDD RED/GREEN: testes criados antes da implementação, falha verificada, depois componente passa todos"
    - "Modal controlado via open/onOpenChange sem AlertDialogTrigger (abertura programática)"
    - "Confirmação destrutiva por token exato case-sensitive — string comparison === sem trim"
    - "loading prop desabilita CTA durante fetch para evitar double-submit"

key-files:
  created:
    - "frontend/src/components/ResetDatabaseDialog.tsx"
    - "frontend/src/components/ResetDatabaseDialog.test.tsx"
    - "frontend/vitest.config.ts"
    - "frontend/src/test/setup.ts"
  modified:
    - "frontend/src/pages/ImportarEFD.tsx"
    - "frontend/package.json (script test + devDeps vitest/testing-library/jsdom)"

key-decisions:
  - "Vitest 1.6.x (não 4.x) por compatibilidade com Node 18.19 — 4.x requer styleText de node:util indisponível no Node 18"
  - "Modal usa AlertDialog Radix (já disponível) sem AlertDialogTrigger — abertura via setState do componente pai"
  - "handleResetCompanyData não foi alterado — escopo deste plan é APENAS reset global (ResetDatabaseHandler)"
  - "Lista de tabelas estática espelhando ResetTables do backend — sem endpoint extra de contagem"

patterns-established:
  - "Confirmação destrutiva via token: <ResetDatabaseDialog open onOpenChange onConfirm loading> — reutilizável para outros handlers destrutivos"
  - "TDD: criar test.tsx falhando → commit test → criar componente → commit feat — ciclo Red/Green rastreável no git"

requirements-completed:
  - STAB-01

duration: 18min
completed: "2026-05-08"
---

# Phase 01 Plan 02: Modal de Confirmação Destrutiva para Reset Global

**Modal ResetDatabaseDialog com token obrigatório DELETE-FB_APU04, integrado ao botão admin em ImportarEFD.tsx via TDD RED/GREEN e infraestrutura vitest instalada.**

## Performance

- **Duration:** 18 min
- **Started:** 2026-05-08T14:45:00Z
- **Completed:** 2026-05-08T15:03:00Z
- **Tasks:** 3 de 3 (Task 3 checkpoint:human-verify APROVADO em modo YOLO auto-advance 2026-05-08)
- **Files modified:** 5

## Accomplishments

- Componente `ResetDatabaseDialog` criado com ciclo TDD completo (8 testes, todos passando)
- `ImportarEFD.tsx` desacoplado do `window.confirm` e do `fetch` direto: botão "Zerar Tudo" agora abre dialog modal
- Handler `handleResetDatabaseConfirm` envia body `{"confirmation":"DELETE-FB_APU04"}` e trata 400/403/429/503/5xx com toasts específicos
- Infraestrutura de testes React instalada (vitest + testing-library + jsdom) com vitest.config.ts e src/test/setup.ts

## Task Commits

1. **Test (01-02): add failing tests for ResetDatabaseDialog (RED)** — `07622e1` (test)
2. **feat(01-02): implementar ResetDatabaseDialog (GREEN)** — `66d6372` (feat)
3. **feat(01-02): acoplar ResetDatabaseDialog em ImportarEFD.tsx** — `a66d296` (feat)

## Files Created/Modified

- `frontend/src/components/ResetDatabaseDialog.tsx` — Modal de confirmação destrutiva; token case-sensitive; lista 8 tabelas afetadas; prop loading
- `frontend/src/components/ResetDatabaseDialog.test.tsx` — 8 testes: render fechado/aberto, token exato habilita, case-mismatch desabilita, trailing space desabilita, onConfirm chamado com body correto, Cancelar chama onOpenChange(false)
- `frontend/vitest.config.ts` — Config vitest com environment jsdom + alias @/
- `frontend/src/test/setup.ts` — Setup @testing-library/jest-dom
- `frontend/src/pages/ImportarEFD.tsx` — Import ResetDatabaseDialog; estados resetOpen/resetLoading; handleResetDatabaseConfirm substitui handleResetDatabase; botão admin abre dialog; componente montado no JSX

## Decisions Made

- **Vitest 1.6.x:** versão 4.x instalada pelo npm satisfazia semver mas requer Node 22+ (usa `node:util#styleText`); Node 18.19.1 no servidor. Rebaixado para 1.6.x — compatível e suficiente para nossos testes.
- **Sem AlertDialogTrigger:** abertura programática via `open={resetOpen}` do componente pai, conforme convenção do plan. O trigger padrão do Radix atrapalharia o controle de estado.
- **Escopo preservado:** `handleResetCompanyData` e seu botão "Limpar {company}" não foram tocados — conforme instrução explícita do plan ("NÃO mexer em handleResetCompanyData").
- **Lista estática de tabelas:** endpoint de contagem seria feature extra; lista espelhando `ResetTables` do backend é suficiente para o usuário entender o que será destruído.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Rebaixamento de vitest 4.x para 1.6.x por incompatibilidade com Node 18**

- **Found during:** Task 1 — tentativa de executar testes RED
- **Issue:** `npm install vitest` instalou 4.1.5 que falha ao iniciar com `SyntaxError: The requested module 'node:util' does not provide an export named 'styleText'` — API disponível apenas no Node 22+
- **Fix:** Reinstalou `vitest@^1.6.0` + `@testing-library/react@^14.0.0` + `jsdom@^24.0.0` (versões compatíveis com Node 18)
- **Files modified:** `frontend/package.json`, `frontend/package-lock.json`
- **Verification:** `npx vitest run src/lib/utils.test.ts` passa com v1.6.1
- **Committed in:** 07622e1 (commit do RED)

---

**Total deviations:** 1 auto-fixed (Rule 3 — dependência bloqueante com versão incompatível)
**Impact on plan:** Fix necessário para execução dos testes. Sem impacto no comportamento do componente ou nos critérios de sucesso.

## TDD Notes

O ciclo RED/GREEN foi executado conforme planejado:

| Gate | Commit | Status |
|------|--------|--------|
| RED (test falha) | `07622e1` | Confirmado: `Error: Failed to resolve import "./ResetDatabaseDialog"` |
| GREEN (componente passa) | `66d6372` | Confirmado: 8/8 testes passando |
| REFACTOR | N/A | Código limpo desde o GREEN; sem refactor necessário |

## Por que ResetCompanyDataHandler NÃO foi alterado

O plan 01-02 tem escopo estrito: proteger o `DELETE /api/admin/reset-db` (reset GLOBAL). O handler `ResetCompanyDataHandler` (`/api/company/reset-data`) é operação de escopo menor (uma empresa, não todas) e requer análise de ameaça separada. Seu endurecimento foi deliberadamente adiado para outro plan — a prioridade 1 do post-incidente era o reset global.

## Issues Encountered

Nenhum além da incompatibilidade de vitest (tratada como desvio acima).

## Known Stubs

Nenhum stub identificado. Todos os caminhos do componente e do handler têm comportamento definido.

## Threat Flags

Nenhuma superfície nova além do planejado no `<threat_model>` do plan 01-02.

## Next Phase Readiness

- Task 3 (checkpoint:human-verify) APROVADO em modo YOLO auto-advance (2026-05-08)
- Phase 01 COMPLETA — todos os 3 plans entregues (01-01, 01-02, 01-03)
- Infraestrutura de testes React (vitest.config.ts + setup.ts) disponível para STAB-08 (phase 3)
- Próxima fase: Phase 02 — Upload de XMLs (Drag-and-Drop)

## Self-Check: PASSED

- [x] `frontend/src/components/ResetDatabaseDialog.tsx` existe
- [x] `frontend/src/components/ResetDatabaseDialog.test.tsx` existe
- [x] `frontend/vitest.config.ts` existe
- [x] `frontend/src/test/setup.ts` existe
- [x] Commits 07622e1, 66d6372, a66d296 existem no log
- [x] 8/8 testes passam
- [x] `tsc --noEmit` passa sem erros
- [x] `npm run build` passa
- [x] `window.confirm.*APAGAR TODOS` removido de ImportarEFD.tsx
- [x] `handleResetDatabaseConfirm` aparece 2x (definição + uso)
- [x] `<ResetDatabaseDialog` aparece 1x no JSX

---
*Phase: 01-estabiliza-o-cr-tica-reset-cache*
*Completed: 2026-05-08*
