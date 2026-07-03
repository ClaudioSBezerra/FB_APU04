---
phase: 12-tela-compara-o-fiscal-navega-o
plan: 02
subsystem: ui
tags: [react, typescript, tanstack-react-query, cmdk, xlsx, fiscal]

# Dependency graph
requires:
  - phase: 12-tela-compara-o-fiscal-navega-o (Plan 01)
    provides: "GET /api/fiscal/comparacao/search, GET /api/fiscal/comparacao, GET /api/fiscal/comparacao/csv (admin-gated)"
provides:
  - "NfeSearchCombobox.tsx — combobox de busca server-side debounced de NF-e (reutilizável)"
  - "ComparacaoFiscal.tsx — tela completa: busca→executa→recarrega→compara 6 impostos, filtro, resumo, export"
affects: ["12-03-navegacao-adminonly"]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Combobox server-driven: Command shouldFilter={false} + useQuery debounced 300ms, enabled >= 3 chars"
    - "Trigger-then-reload: useMutation.onSuccess -> queryClient.invalidateQueries(['fiscal-comparacao', nfeId])"
    - "Divergência tolerância zero (!== 0) avaliada só quando status === 'ok' — 4 estados de badge (ok/divergente/nao_calculado/nunca_executado)"

key-files:
  created:
    - frontend/src/components/NfeSearchCombobox.tsx
    - frontend/src/pages/ComparacaoFiscal.tsx
  modified: []

key-decisions:
  - "Comentários que citavam literalmente 'não usar > 0.01' foram reescritos sem o literal — o próprio grep de verificação do plano não distingue comentário de código (mesmo pitfall documentado em 12-01-SUMMARY.md)"
  - "Divergência avaliada em base E valor por par de imposto (não só valor) para ICMS/ICMS-ST/PIS/COFINS, seguindo a regra binding do UI-SPEC ('todo par base/valor'); tabela exibe só a subcoluna de valor (3 colunas), base completa fica no dialog de detalhe"
  - "Chave NF-e exibida truncada em toda linha da tabela (repetida, já que é 1 nota por vez) para cumprir literalmente o layout de 8 blocos do UI-SPEC (identificação inclui NF-e/chave por linha)"
  - "Ícone 'Ver detalhes' usa Search (lucide-react) em vez de Eye — mantém a lista de ícones já fechada nas <interfaces> do plano (Search, HelpCircle, AlertTriangle, CheckCircle, Download, FileSpreadsheet, Loader2, Send)"

patterns-established:
  - "Primeiro combobox de busca server-driven do codebase — candidato a reuso em telas futuras que precisem de autocomplete backend-scoped"

requirements-completed: [TPF-06, TPF-07]

duration: 30min
completed: 2026-07-03
---

# Phase 12 Plan 02: Frontend da Comparação Fiscal Summary

**Tela React "Comparação Fiscal" completa — busca NF-e por número/chave (combobox debounced server-side), dispara `POST /api/fiscal/execute` e recarrega automaticamente 6 impostos (ICMS/ICMS-ST/PIS/COFINS/IBS/CBS) com tolerância zero, 4 estados de badge, filtro "só divergentes", resumo agregado (4 cards + 6 chips) e exportação Excel/CSV.**

## Performance

- **Duration:** ~30 min
- **Completed:** 2026-07-03
- **Tasks:** 2/2
- **Files modified:** 2 (ambos criados)

## Accomplishments
- `NfeSearchCombobox.tsx`: Command/Popover com `shouldFilter={false}`, debounce 300ms, `useQuery` gated em `debounced.length >= 3`, consumindo `GET /api/fiscal/comparacao/search` (Plan 12-01)
- `ComparacaoFiscal.tsx`: fluxo completo busca→executar→recarrega na mesma tela via `useMutation.onSuccess` + `invalidateQueries(['fiscal-comparacao', nfeId])` (D-01)
- Regra de divergência tolerância zero (`!== 0`, base e valor) avaliada só quando `status === 'ok'`; 4 estados de badge distintos (OK / Divergente / Não calculado / Nunca executado), com tooltip contextual nos 2 últimos
- Resumo agregado: 4 cards (Total/Sem Divergência/Divergentes/Não Calculados) + 6 chips por imposto com contagem/percentual
- Filtro "Só divergentes" (Switch, off por default), tabela densa (`text-[11px]`, `py-1 px-2`) com 18 colunas numéricas + identificação + status + "Ver detalhes"
- Dialog de detalhe com 3 seções (Identificação / Comparação full precision / Só calculado — DIFAL/FCP/grupo fiscal com nota explicativa)
- Tooltip mandatório nos headers IBS/CBS avisando sobre o gap de parser (D-05) — impostos continuam contando no filtro/resumo
- Exportar Excel (client-side, `exportToExcel`) e Exportar CSV (blob download de `/api/fiscal/comparacao/csv`)
- `npm run build` e `tsc --noEmit` limpos para os dois arquivos

## Task Commits

Each task was committed atomically:

1. **Task 1: NfeSearchCombobox — busca server-side debounced** - `a732be9` (feat)
2. **Task 2: ComparacaoFiscal.tsx — tela completa** - `388fb83` (feat)

**Plan metadata:** (this commit) `docs(12-02): complete Frontend da Comparação Fiscal plan`

## Files Created/Modified
- `frontend/src/components/NfeSearchCombobox.tsx` - Combobox reutilizável de busca de NF-e, server-side debounced, exporta `NfeSearchResult`
- `frontend/src/pages/ComparacaoFiscal.tsx` - Tela completa: busca+execução+comparação+filtro+resumo+export+dialog de detalhe

## Decisions Made
- **Divergência avaliada em base + valor** (não só valor) para ICMS/ICMS-ST/PIS/COFINS — cumpre literalmente a regra binding do UI-SPEC ("todo par base/valor"), com a tabela mostrando só a subcoluna de valor por espaço, e base completa disponível no dialog
- **Comentários reescritos sem o literal "> 0.01"** — mesmo pitfall de verificação já documentado em 12-01-SUMMARY.md (grep não distingue comentário de código); nenhuma mudança funcional
- **Ícone "Ver detalhes" = Search** — respeita a lista fechada de ícones das `<interfaces>` do plano (não introduz `Eye` fora do catálogo aprovado)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Comentários de código continham o literal `> 0.01`, quebrando o próprio gate de verificação do plano**
- **Found during:** Task 2 (verificação automatizada pós-implementação, `! grep -q "> 0.01"`)
- **Issue:** Dois comentários explicativos citavam literalmente "NÃO reusar o `> 0.01`" para documentar a decisão de tolerância zero — o grep de verificação do plano não distingue comentário de código-executável, então o próprio arquivo falhava seu gate de conformidade mesmo sem o threshold de 1 centavo existir em nenhuma lógica real
- **Fix:** Reescritos os 2 comentários para descrever a regra sem citar o literal `> 0.01` ("threshold de um centavo de ConciliacaoBridgeXML.tsx")
- **Files modified:** frontend/src/pages/ComparacaoFiscal.tsx
- **Verification:** `grep -q "> 0.01" src/pages/ComparacaoFiscal.tsx` retorna vazio; `npx tsc --noEmit` e `npm run build` seguem limpos
- **Committed in:** 388fb83 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug, cosmético em comentário)
**Impact on plan:** Nenhum impacto funcional. Mesmo padrão de deviation já visto no Plan 12-01 (comentário citando literal bloqueado pelo próprio grep de verificação).

## Issues Encountered
None além do deviation acima.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Plan 12-03 (navegação `adminOnly` — TPF-08) pode importar `ComparacaoFiscal` diretamente em `App.tsx`/`navigation.ts`/`AppSidebar.tsx`; a página já está pronta para ser roteada em `/pacote-fiscal/comparacao` com `AdminRoute`
- Validação end-to-end com execução real do pacote fiscal (Oracle) ainda depende de credencial Oracle real, já sinalizada como pendente desde a Fase 11

---
*Phase: 12-tela-compara-o-fiscal-navega-o*
*Completed: 2026-07-03*

## Self-Check: PASSED

All created files exist on disk (frontend/src/components/NfeSearchCombobox.tsx, frontend/src/pages/ComparacaoFiscal.tsx, this SUMMARY.md) and both task commits (a732be9, 388fb83) are present in git log.
