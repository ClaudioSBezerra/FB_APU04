---
phase: quick-260519-ixg
plan: 01
subsystem: frontend-backend
tags: [simulador, xml, sped, crt, regime-tributario, painel]
dependency_graph:
  requires: []
  provides:
    - /api/xml/painel/entradas-informativos
    - /api/xml/reports/mercadorias
    - vw_xml_entradas_informativos
    - vw_xml_operacoes_resumo
    - /mercadorias/xml
    - updateCompanyRegimeFromCRT
  affects:
    - frontend/src/pages/Mercadorias.tsx
    - frontend/src/components/AppSidebar.tsx
    - frontend/src/lib/navigation.ts
tech_stack:
  added:
    - vw_xml_entradas_informativos (migration 080)
    - vw_xml_operacoes_resumo (migration 081)
  patterns:
    - XML-only view filtering (source='xml_upload')
    - CRT-to-regime mapping via buildRegimeUpdate pure function
    - Informative rows (render at R$ 0,00 when no data)
key_files:
  created:
    - backend/migrations/080_create_vw_xml_entradas_informativos.sql
    - backend/migrations/081_create_vw_xml_operacoes_resumo.sql
    - frontend/src/pages/MercadoriasXML.tsx
  modified:
    - backend/handlers/xml_painel.go
    - backend/handlers/xml_reports.go
    - backend/handlers/xml_upload.go
    - backend/handlers/nfe_saidas.go
    - backend/handlers/xml_upload_test.go
    - backend/main.go
    - frontend/src/components/AppSidebar.tsx
    - frontend/src/lib/navigation.ts
    - frontend/src/components/AppRail.tsx
    - frontend/src/pages/Login.tsx
    - frontend/src/pages/Mercadorias.tsx
    - frontend/src/App.tsx
decisions:
  - Label rename "- SPED" suffix only in display text — module ID `simulador` unchanged
  - MercadoriasXML drops nfe-entradas/impostos call — XML page reads only XML-sourced endpoints
  - CRT=3 defaults to lucro_real (comment in code documents LP manual override)
  - buildRegimeUpdate pure function enables unit testing without DB dependency
  - updateCompanyRegimeFromCRT called after tx.Commit() — import never blocked by regime update failure
metrics:
  duration_seconds: 514
  completed_date: "2026-05-19"
  tasks_completed: 3
  tasks_total: 3
  files_created: 3
  files_modified: 12
---

# Phase quick-260519-ixg Plan 01: Painel SPED vs XML, CRT Detection Summary

**One-liner:** Renomeação de labels para "- SPED", novos rows informativos XML em /mercadorias, clone /mercadorias/xml sem SPED, e detecção automática de CRT do XML de saída para atualizar regime_tributario.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Rename label SPED, add XML informative rows in /mercadorias | c47f86a | AppSidebar, navigation.ts, AppRail, Login.tsx, App.tsx, Mercadorias.tsx, migration 080, xml_painel.go, main.go |
| 2 | Duplicate Painel into /mercadorias/xml | 4203d34 | MercadoriasXML.tsx, App.tsx, AppSidebar.tsx, navigation.ts, xml_reports.go, migration 081, main.go |
| 3 | Auto-detect CRT from XML saída, update companies.regime_tributario | 22782ac | xml_upload.go, nfe_saidas.go, xml_upload_test.go |

## What Was Built

### Task 1 — Renomeação e Rows Informativos

**Label rename:**
- `AppSidebar.tsx`: section title e header subtitle → "Simulador da Reforma Tributária - SPED"
- `navigation.ts`: module label → "Simulador da Reforma Tributária - SPED"
- `AppRail.tsx`: tooltip label → "Simulador RT - SPED"
- `Login.tsx`: badge text → "Simulador da Reforma Tributária - SPED"
- `App.tsx`: console.log e comentário de rota atualizados

**Migration 080** (`vw_xml_entradas_informativos`): agrega IPI e PIS/COFINS de fornecedores SN de `nfe_entradas WHERE source='xml_upload'` apenas.

**Endpoint** `GET /api/xml/painel/entradas-informativos`: retorna `{ total, items[{ mes_ano, total_ipi, total_pis_simples, total_cofins_simples, qtd_notas }] }` por company.

**Mercadorias.tsx**: novo `useEffect` busca informativos XML; dois novos rows abaixo de "Total Créditos:" — "IPI (Informativo):" e "PIS/COFINS Fornecedores Simples Nacional (Informativo):" — rendem R$ 0,00 quando não há dados XML.

### Task 2 — Página /mercadorias/xml

**Migration 081** (`vw_xml_operacoes_resumo`): UNION ALL de `nfe_entradas` (tipo='ENTRADA', tipo_operacao='Entrada_XML') e `nfe_saidas` (tipo='SAIDA', tipo_operacao='Saida_XML'), ambos com `source='xml_upload'`. Expõe mesmas colunas que `AggregatedData` do frontend. `vl_ibs_projetado` e `vl_cbs_projetado` usam valores reais do XML quando disponíveis.

**Endpoint** `GET /api/xml/reports/mercadorias`: retorna array JSON no mesmo shape de `/api/reports/mercadorias`.

**MercadoriasXML.tsx**: clone de Mercadorias.tsx com:
- h1: "Simulador da Reforma Tributária - XMLs"
- Fetch: `/api/xml/reports/mercadorias`
- Removido: "Reconstruir Painel" button, `/api/nfe-entradas/impostos` fetch, `isRefreshing` state, `handleRefreshViews`
- Mantidos: informativos IPI/PIS-COFINS-SN (XML-sourced), todos os filtros, gráfico, exportação

**Rota** `/mercadorias/xml` registrada em App.tsx (antes de `/mercadorias` para especificidade).

**Sidebar** e **navigation.ts**: nova entrada "Operações Comerciais (XMLs)" → `/mercadorias/xml`.

### Task 3 — Detecção CRT

**`buildRegimeUpdate(emitCNPJ, crt string) (sql, regime string)`**: função pura que mapeia CRT → regime_tributario. CRT=1,2 → `simples_nacional`; CRT=3 → `lucro_real`; outros → retorno vazio (skip). Não acessa banco — testável sem dependência.

**`updateCompanyRegimeFromCRT(db, companyID, emitCNPJ, crt)`**: executa `UPDATE companies SET regime_tributario=... WHERE id IN (SELECT company_id FROM filial_apelidos WHERE cnpj=...)`. 0 rows quando CNPJ não cadastrado — skip silencioso. Log de erro em falha de DB (não propaga).

**Hook em `processSingleXML`** (xml_upload.go): chamado após `tx.Commit()` no `case "saidas"`. Nunca chamado em entradas.

**Hook em `NfeSaidasUploadHandler`** (nfe_saidas.go): chamado após `tx.Commit()` bem-sucedido no loop de saídas.

**Testes** (`xml_upload_test.go`): `TestUpdateCompanyRegimeFromCRT` — 7 casos table-driven, todos PASS.

## Verification

- `go build ./...` — PASS
- `npm run build` — PASS (2399 modules transformed)
- `go test ./handlers/ -run TestUpdateCompanyRegimeFromCRT` — 7/7 PASS
- Label "Simulador da Reforma Tributária - SPED" presente em AppSidebar, navigation.ts, Login.tsx
- Migration 080 e 081 criadas como `CREATE OR REPLACE VIEW` (idempotentes)
- `/api/xml/painel/entradas-informativos` registrado antes do prefixo `/api/xml/painel/`
- `/api/xml/reports/mercadorias` registrado antes dos demais `/api/xml/reports/...`
- `updateCompanyRegimeFromCRT` ausente em `nfe_entradas.go`

## Deviations from Plan

None - plan executed exactly as written.

## Known Stubs

None. Todos os rows informativos rendem R$ 0,00 quando não há dados XML — comportamento correto especificado no plano, não um stub.

## Threat Flags

None. Sem novos endpoints de rede, paths de auth, ou mudanças de schema em trust boundaries não documentados no plano.

## Self-Check: PASSED

All 12 key files verified present on disk. All 3 task commits found in git log.
