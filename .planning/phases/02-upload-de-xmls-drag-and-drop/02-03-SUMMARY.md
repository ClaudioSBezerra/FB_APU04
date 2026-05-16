---
phase: 02-upload-de-xmls-drag-and-drop
plan: "03"
subsystem: frontend-xml-upload
tags: [react, typescript, react-dropzone, tanstack-query, radix-ui, tabs, progress, upload, xml, drag-and-drop]

dependency_graph:
  requires:
    - phase: 02-upload-de-xmls-drag-and-drop
      plan: "02"
      provides:
        - api/xml/upload (POST)
        - api/xml/upload-batches (GET)
        - api/xml/upload-batches/{id}/status (GET)
        - api/xml/painel/entradas (GET)
        - api/xml/painel/saidas (GET)
        - api/xml/painel/ctes (GET)
  provides:
    - page/importacoes/xml/entradas (AdminRoute drag-and-drop NFe Entradas)
    - page/importacoes/xml/saidas (AdminRoute drag-and-drop NFe Saídas)
    - page/importacoes/xml/ctes (AdminRoute drag-and-drop CT-es)
    - page/painel/xmls (Painel com 3 abas Entradas/Saídas/CT-es)
    - campo regime_tributario no formulário de empresa (GestaoAmbiente)
    - erp_type oracle_xml + checkboxes por tipo (ERPBridgeConfig)
  affects:
    - phase-03 (consulta de dados usará vw_xml_* e painel)

tech-stack:
  added:
    - react-dropzone ^15.0.0
  patterns:
    - "useDropzone com accept xml/zip + maxSize 100MB + onDropRejected toast"
    - "useQuery polling refetchInterval:2000 enquanto uploadState===polling"
    - "uploadState machine: idle→uploading→polling/done/error"
    - "select callback do useQuery para side effects (setProgress, setUploadState)"
    - "fetch() global interceptado por AuthContext.window.fetch — sem authHeaders manuais"

key-files:
  created:
    - frontend/src/pages/PainelXMLs.tsx
  modified:
    - frontend/src/pages/ImportarXMLsEntrada.tsx
    - frontend/src/pages/ImportarXMLsSaida.tsx
    - frontend/src/pages/ImportarXMLsCTe.tsx
    - frontend/src/pages/GestaoAmbiente.tsx
    - frontend/src/pages/ERPBridgeConfig.tsx
    - frontend/src/lib/navigation.ts
    - frontend/src/App.tsx
    - frontend/package.json

key-decisions:
  - "fetch() global interceptado — não há fetchAuth exportado; AuthContext injeta headers via window.fetch"
  - "select callback do useQuery usado para side effects (setProgress, setUploadState, toast) — evita useEffect extra"
  - "PainelXMLs usa os campos forn_cnpj/forn_nome do handler Go (que mapeia emit_cnpj para CT-es internamente)"
  - "regime_tributario apenas no payload de criação — não no GET (backend pode não retornar ainda)"
  - "ERPBridgeConfig: checkboxes oracle_xml são apenas UI local — backend de endpoint /api/erp-bridge/config ainda não persiste esses campos"

requirements-completed: [XML-01, XML-02, XML-05, XML-07, XML-08]

duration: ~35min
completed: "2026-05-16"
---

# Phase 02 Plan 03: Frontend React Upload Drag-and-Drop e Painel XMLs — Summary

**Zona drag-and-drop (react-dropzone) com barra de progresso por polling, histórico de uploads e painel de 3 abas consultando vw_xml_* — substituindo o seletor de pasta manual nas 3 páginas de importação XML**

## Performance

- **Duration:** ~35 min
- **Started:** 2026-05-16T18:00:00Z
- **Completed:** 2026-05-16T18:35:00Z
- **Tasks:** 2
- **Files modified:** 8 (1 criado)

## Accomplishments

- Refatorou as 3 páginas de upload (Entrada, Saída, CT-e) com zona drag-and-drop visual usando react-dropzone, com polling automático a cada 2s para batches assíncronos (>50 XMLs)
- Criou PainelXMLs.tsx com 3 abas Radix Tabs consultando /api/xml/painel/{tipo}, filtro mes_ano e exportação CSV
- Adicionou campo Select de regime_tributario no modal de empresa (GestaoAmbiente) com nota informativa para Lucro Real/Presumido
- Adicionou seleção de erp_type oracle_xml com checkboxes de tipos por servidor no ERPBridgeConfig
- Registrou 4 novas rotas em App.tsx e 4 novas tabs no módulo "notas" de navigation.ts

## Rotas Frontend Adicionadas

| Rota | Componente | Proteção |
|------|-----------|---------|
| `/importacoes/xml/entradas` | ImportarXMLsEntrada | AdminRoute |
| `/importacoes/xml/saidas` | ImportarXMLsSaida | AdminRoute |
| `/importacoes/xml/ctes` | ImportarXMLsCTe | AdminRoute |
| `/painel/xmls` | PainelXMLs | ProtectedRoute (sem AdminRoute) |

## Tabs Adicionadas em navigation.ts

Módulo `notas` — adicionadas após "Logs de Importação":
- `Importar XMLs Entradas` → `/importacoes/xml/entradas` (adminOnly)
- `Importar XMLs Saídas` → `/importacoes/xml/saidas` (adminOnly)
- `Importar XMLs CT-es` → `/importacoes/xml/ctes` (adminOnly)
- `Painel XMLs` → `/painel/xmls`

`getActiveModule` atualizado: `/painel/xmls` → `'notas'`

## Endpoints Backend Consumidos

| Endpoint | Parâmetros | Usado por |
|----------|-----------|----------|
| POST /api/xml/upload | FormData: tipo + file | ImportarXML*.tsx |
| GET /api/xml/upload-batches?tipo=&limit= | tipo, limit | Histórico nas 3 páginas |
| GET /api/xml/upload-batches/{id}/status | batchId | Polling de progresso |
| GET /api/xml/painel/entradas?mes_ano=&limit= | mes_ano, limit | PainelXMLs aba Entradas |
| GET /api/xml/painel/saidas?mes_ano=&limit= | mes_ano, limit | PainelXMLs aba Saídas |
| GET /api/xml/painel/ctes?mes_ano=&limit= | mes_ano, limit | PainelXMLs aba CT-es |

## Análise do GestaoAmbiente.tsx

O formulário de empresa usa estado local (não react-hook-form). Modal de diálogo com campos Input padrão. `CardDescription` estava importado mas não usado — removido para evitar erro `noUnusedLocals`. Campo `regime_tributario` adicionado como Select Radix com 4 opções; incluído no payload do POST `/api/config/companies`. O backend pode não persistir ainda (campo novo na migration 078), mas o frontend envia o valor corretamente.

## Status do Build TypeScript

`npm run build` — **PASSOU** sem erros de TypeScript.

Aviso de chunk size (1.4MB) é pré-existente e não relacionado a este plan.

## Task Commits

1. **Task 1: react-dropzone + refatorar 3 páginas de upload** — `82415fe` (feat)
2. **Task 2: PainelXMLs + regime tributário + navegação + rotas** — `974a09e` (feat)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Removida importação não usada CardDescription**
- **Found during:** Task 2 (edição de GestaoAmbiente.tsx)
- **Issue:** `CardDescription` estava importada mas não usada no JSX. Com `noUnusedLocals: true`, isso causaria erro de TypeScript.
- **Fix:** Removida da importação do `@/components/ui/card`.
- **Files modified:** frontend/src/pages/GestaoAmbiente.tsx
- **Committed in:** 974a09e

---

**Total deviations:** 1 auto-fixed (1 Rule 2 - missing critical / correctness)
**Impact on plan:** Correção necessária para compilação sem erros. Sem scope creep.

## Known Stubs

**ERPBridgeConfig — checkboxes oracle_xml (UI only)**
- **Arquivo:** `frontend/src/pages/ERPBridgeConfig.tsx`
- **Estado:** Os checkboxes `oracleEntradas`, `oracleSaidas`, `oracleCtes` são estado local apenas — não são persistidos via `/api/erp-bridge/config` (o endpoint PATCH ainda não aceita esses campos no backend).
- **Razão:** O plano especifica "apenas expor os campos na UI e documentar que o backend precisa de endpoint". O backend será atualizado em fase futura.
- **Qual plano resolve:** Phase futura de configuração avançada do ERP Bridge.

## Threat Flags

Nenhuma nova superfície de segurança introduzida além das especificadas no threat model do plano.

Mitigações T-02-03-01 a T-02-03-04 satisfeitas:
- T-02-03-01: `maxSize: 100 * 1024 * 1024` no useDropzone + verificação 413 no handleUpload
- T-02-03-02: error_details exibe apenas `{filename, motivo}` — sem stack trace
- T-02-03-03: AdminRoute wrapper em App.tsx para as 3 rotas de importação
- T-02-03-04: GestaoAmbiente acessível apenas por admin (ProtectedRoute + verificação `user.role === 'admin'` interna)

## Self-Check: PASSED

Arquivos verificados:
- frontend/src/pages/PainelXMLs.tsx — FOUND (commit 974a09e)
- frontend/src/pages/ImportarXMLsEntrada.tsx — FOUND (commit 82415fe)
- frontend/src/pages/ImportarXMLsSaida.tsx — FOUND (commit 82415fe)
- frontend/src/pages/ImportarXMLsCTe.tsx — FOUND (commit 82415fe)
- frontend/src/pages/GestaoAmbiente.tsx — FOUND (commit 974a09e)
- frontend/src/pages/ERPBridgeConfig.tsx — FOUND (commit 974a09e)
- frontend/src/lib/navigation.ts — FOUND (commit 974a09e)
- frontend/src/App.tsx — FOUND (commit 974a09e)

Build TypeScript: PASSOU sem erros
npm run build: PASSOU (vite build em 7.62s)

## Next Phase Readiness

- Frontend de upload XML completo e funcional
- PainelXMLs pronto para consultar dados assim que XMLs forem importados
- Phase 02 concluída (todos os 3 plans executados)
- Próximo: Phase 03 — apuração e relatórios fiscais baseados nos dados XML importados

---
*Phase: 02-upload-de-xmls-drag-and-drop*
*Completed: 2026-05-16*
