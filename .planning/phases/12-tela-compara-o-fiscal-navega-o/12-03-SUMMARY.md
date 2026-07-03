---
phase: 12-tela-compara-o-fiscal-navega-o
plan: 03
subsystem: ui
tags: [react, typescript, navigation, rbac]

# Dependency graph
requires:
  - phase: 12-tela-compara-o-fiscal-navega-o (Plan 02)
    provides: "ComparacaoFiscal.tsx (default export) pronta para ser roteada"
provides:
  - "Item de navegação 'Teste Pacote Fiscal → Comparação Fiscal' com gate adminOnly (código completo, Task 1)"
  - "Rota /pacote-fiscal/comparacao registrada em App.tsx, envolta em AdminRoute"
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Reuso verbatim do padrão de NavSection adminOnly já existente (seção 'malha') — sem novo componente ou lógica de gate"

key-files:
  created: []
  modified:
    - frontend/src/lib/navigation.ts
    - frontend/src/components/AppSidebar.tsx
    - frontend/src/App.tsx

key-decisions:
  - "Nenhuma — plano executado exatamente como escrito (código já vinha redigido verbatim no UI-SPEC/PATTERNS.md)"

patterns-established: []

requirements-completed: []  # TPF-08 code-side está implementado (Task 1), mas o plano só considera o requisito fechado após o checkpoint humano (Task 2) validar o fluxo end-to-end. Não marcar como completo em REQUIREMENTS.md até aprovação.

# Metrics
duration: "~10min (Task 1 apenas; Task 2 pendente)"
completed: null  # plano NÃO concluído — aguardando checkpoint humano
---

# Phase 12 Plan 03: Navegação adminOnly (Task 1) — PENDENTE checkpoint humano

**Task 1 completa: wiring de navegação em 3 arquivos (navigation.ts + AppSidebar.tsx + App.tsx) para o item "Teste Pacote Fiscal → Comparação Fiscal", gate `adminOnly: true` reusando o padrão idêntico da seção "malha". Task 2 (checkpoint humano de verificação end-to-end) está PENDENTE — este plano NÃO está concluído.**

## Status

**Task 1: CONCLUÍDA e commitada.**
**Task 2 (`checkpoint:human-verify`, gate `blocking`): PENDENTE.** Aguardando o usuário rodar a verificação manual descrita no PLAN.md e responder "approved" ou reportar problemas. Este executor NÃO pode simular/aprovar essa verificação — ela requer navegação visual em browser real (login, clicar, observar UI), que está fora do alcance desta sessão não-interativa.

## Performance

- **Duration:** ~10 min (Task 1)
- **Completed:** Task 1 em 2026-07-03; Task 2 ainda não iniciada
- **Tasks:** 1/2
- **Files modified:** 3

## Accomplishments (Task 1)
- `frontend/src/lib/navigation.ts`: módulo `pacotefiscal` (label "Teste Pacote Fiscal", tab "Comparação Fiscal" → `/pacote-fiscal/comparacao`) + branch em `getActiveModule()` para `/pacote-fiscal`
- `frontend/src/components/AppSidebar.tsx`: import de `FlaskConical`/`GitCompare` (lucide-react); nova `NavSection` `"pacotefiscal"` com `adminOnly: true`, item único "Comparação Fiscal"
- `frontend/src/App.tsx`: import de `ComparacaoFiscal`; rota `/pacote-fiscal/comparacao` envolta em `<AdminRoute>` (defesa em profundidade além do gate de nav)
- `npx tsc --noEmit` limpo; `npm run build` conclui sem erro
- Todos os greps de verificação do plano passam (`pacotefiscal` em navigation.ts, branch `startsWith('/pacote-fiscal')`, `adminOnly: true` em AppSidebar.tsx, `AdminRoute><ComparacaoFiscal` em App.tsx)

## Task Commits

1. **Task 1: Wiring de navegação nos 3 arquivos (adminOnly gate)** - `ed837a5` (feat)

Task 2 (checkpoint humano) não gera commit de código — é verificação manual pura.

**Plan metadata:** Este commit de SUMMARY (pendente) — plano ainda não fechado, sem commit final `docs(12-03): complete...` até aprovação humana.

## Files Created/Modified
- `frontend/src/lib/navigation.ts` - módulo `pacotefiscal` + branch `getActiveModule`
- `frontend/src/components/AppSidebar.tsx` - NavSection `adminOnly` "Teste Pacote Fiscal" + ícones importados
- `frontend/src/App.tsx` - import + rota `/pacote-fiscal/comparacao` sob `AdminRoute`

## Decisions Made
Nenhuma — plano executado exatamente como escrito (o UI-SPEC e o PATTERNS.md já traziam o código exato a inserir nos 3 arquivos).

## Deviations from Plan
None - plano executado exatamente como escrito.

## Issues Encountered
None. Build e tsc limpos na primeira tentativa.

## User Setup Required

Nenhuma configuração de serviço externo. O que falta é **verificação humana** (Task 2), não configuração.

## Next Phase Readiness

**Este plano NÃO está pronto para ser considerado concluído.** Falta:
1. Rodar `cd frontend && npm run dev` (ou equivalente) e o backend, logar como admin
2. Seguir os 10 passos de `<how-to-verify>` do Task 2 do PLAN.md (ver abaixo, seção "Checkpoint Pendente")
3. Responder "approved" (ou descrever problemas) para que a execução seja retomada e o plano seja fechado (SUMMARY final + STATE.md + ROADMAP.md + REQUIREMENTS.md atualizados)

Até essa aprovação, a Fase 12 permanece com Plan 03 em andamento (não concluída), e o milestone v6.00 não deve ser dado como fechado.

## Checkpoint Pendente (Task 2)

**Tipo:** human-verify (gate: blocking)
**O que foi construído:** Fluxo completo do módulo Teste Pacote Fiscal: item de navegação admin-gated (TPF-08, Task 1 desta sessão), tela Comparação Fiscal (TPF-06/07, Plan 12-02) que busca uma NF-e, dispara a execução do pacote fiscal (endpoint da Fase 11) e recarrega a comparação item a item das 6 impostos com divergências destacadas, filtro "só divergentes", resumo agregado e exportação.

**Passos de verificação (executar manualmente, em browser real):**
1. Rodar o frontend (`npm run dev`) e o backend, logar como usuário admin.
2. Confirmar que a seção "Teste Pacote Fiscal" aparece na sidebar; abrir "Comparação Fiscal" (`/pacote-fiscal/comparacao`).
3. Buscar uma NF-e de saída existente (digitar >=3 chars do número ou chave) e selecionar um resultado do autocomplete.
4. Clicar "Executar" — o botão mostra spinner, e ao concluir a tabela recarrega automaticamente na mesma tela (sem navegação extra).
5. Verificar: 4 cards de resumo, 6 chips por imposto, tabela com Esperado/Calculado/Diferença para ICMS/ICMS-ST/PIS/COFINS/IBS/CBS, células divergentes em vermelho (tolerância zero).
6. Confirmar os badges de estado: itens com status != "ok" = "Não calculado"; itens de uma nota nunca executada (buscar sem clicar Executar) = "Nunca executado" (badge distinto).
7. Ativar "Só divergentes" e confirmar que itens sem divergência somem.
8. Passar o mouse nos headers IBS/CBS e confirmar o tooltip de aviso de dado zerado.
9. Clicar "Exportar Excel" e "Exportar CSV" e confirmar os downloads.
10. (Opcional/segurança) logar como usuário NÃO-admin e confirmar que a seção não aparece e que `/pacote-fiscal/comparacao` redireciona.

**Sinal de retorno:** Digite "approved" ou descreva os problemas encontrados.

---
*Phase: 12-tela-compara-o-fiscal-navega-o*
*Status: Task 1/2 concluída — Task 2 (checkpoint humano) pendente*

## Self-Check: PASSED

Arquivos modificados existem em disco (frontend/src/lib/navigation.ts, frontend/src/components/AppSidebar.tsx, frontend/src/App.tsx) e o commit da Task 1 (`ed837a5`) está presente no git log. Build de produção e `tsc --noEmit` confirmados limpos nesta sessão.
