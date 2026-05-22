---
phase: 06-infraestrutura-reforma-tribut-ria
plan: "04"
subsystem: frontend
tags: [reforma-tributaria, react-query, navigation, maps]
dependency_graph:
  requires: ["06-03"]
  provides: ["hook-reforma-parametros", "page-reforma-parametros", "nav-reforma", "brazil-states-json"]
  affects: ["frontend/src/hooks", "frontend/src/pages", "frontend/src/lib", "frontend/src/App.tsx"]
tech_stack:
  added: ["react-simple-maps@3.0.0"]
  patterns: ["useQuery/useMutation via AuthContext interceptor", "admin-only UI gate", "tooltip CG-IBS disclaimer"]
key_files:
  created:
    - frontend/src/hooks/useReformaParametros.ts
    - frontend/src/pages/ReformaParametros.tsx
    - frontend/public/brazil-states.json
  modified:
    - frontend/src/lib/navigation.ts
    - frontend/src/App.tsx
    - frontend/package.json
    - frontend/package-lock.json
decisions:
  - "fetch() global interceptado por AuthContext — hook não passa headers de auth manualmente, apenas Content-Type no PUT"
  - "getActiveModule adiciona '/reforma' antes de '/config/' para evitar sobreposição de prefixo"
  - "react-simple-maps@3.0.0 fixado sem ^ pin no código — package.json usa ^3.0.0 (npm padrão)"
  - "brazil-states.json obtido de codeforamerica/click_that_hood (27 features, MIT, fonte pública consolidada)"
metrics:
  duration: "~10 min"
  completed: "2026-05-22"
  tasks_completed: 3
  files_changed: 7
requirements_satisfied: [RFMA-06, RFMA-07, RFMA-08]
---

# Phase 06 Plan 04: Frontend Infraestrutura Reforma Tributária Summary

## One-Liner

Hook react-query global `useReformaParametros` + página de config admin-only com tooltip CG-IBS + módulo `reforma` na sidebar com 8 placeholders + setup `react-simple-maps@3.0.0` e `brazil-states.json` para Phase 8.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Hook useReformaParametros.ts (RFMA-06) | d43044e | frontend/src/hooks/useReformaParametros.ts |
| 2 | Página ReformaParametros + nav + rotas (RFMA-07) | 0f4797e | ReformaParametros.tsx, navigation.ts, App.tsx |
| 3 | Instalar react-simple-maps + brazil-states.json (RFMA-08) | 3766d1f | package.json, package-lock.json, public/brazil-states.json |

## What Was Built

### Task 1 — useReformaParametros.ts (RFMA-06)

Criado `frontend/src/hooks/useReformaParametros.ts` com:
- `interface ReformaParametros` — 7 campos snake_case (company_id: string, target_ano/aliq_ibs_pct/aliq_cbs_pct/fator_simples_pct/taxa_cdi_anual_pct/prazo_medio_dias: number)
- `useReformaParametros()` — useQuery com queryKey `['reforma-parametros']`, fetch GET `/api/reforma/parametros` (auth via interceptor global)
- `useUpdateReformaParametros()` — useMutation com PUT `/api/reforma/parametros`, `onSuccess` invalida `['reforma-parametros']`

### Task 2 — Página + Navegação + Rotas (RFMA-07)

**ReformaParametros.tsx:** Card com 6 campos editáveis (target_ano, aliq_ibs_pct, aliq_cbs_pct, fator_simples_pct, taxa_cdi_anual_pct, prazo_medio_dias). Admin vê campos habilitados + botão Salvar com spinner. Não-admin vê campos disabled sem botão. Tooltip ⓘ no campo fator_simples_pct com texto exato: "Valor estimado. Alíquota definitiva ainda não publicada pelo CG-IBS."

**navigation.ts:** Adicionado módulo `reforma` (após painel, antes config) com label 'Análise Reforma Tributária', tab ativa 'Parâmetros' (/reforma/parametros) e 8 tabs disabled (Créditos IBS/CBS, Reprecificação, Ranking Fornecedores, Split Payment, Análise CFOP, Análise NCM, UF Destino, B2B vs B2C). Tab 'Parâmetros Reforma' adicionada ao módulo config. `getActiveModule` retorna 'reforma' para `/reforma*` (antes da regra `/config/`).

**App.tsx:** Import de ReformaParametros adicionado. Rotas `/reforma/parametros` e `/config/reforma-parametros` registradas sem AdminRoute.

### Task 3 — react-simple-maps + brazil-states.json (RFMA-08)

`react-simple-maps@3.0.0` instalado em dependencies (pacote verificado: MIT, ~8 anos, sem postinstall). `frontend/public/brazil-states.json` commitado como GeoJSON FeatureCollection com 27 features dos estados brasileiros. Nenhum componente de mapa criado — setup puro de pré-requisito para Phase 8.

## Deviations from Plan

None — plano executado exatamente como escrito.

## Threat Surface Scan

| Flag | File | Description |
|------|------|-------------|
| T-06-12 (mitigated) | ReformaParametros.tsx | Botão Salvar renderizado somente `{isAdmin && ...}` — não-admin não pode submeter |
| T-06-13 (mitigated) | ReformaParametros.tsx | Tooltip com texto exato "Alíquota definitiva ainda não publicada pelo CG-IBS." presente |
| T-06-14 (mitigated) | package.json | react-simple-maps@3.0.0 VERIFIED npm registry, MIT, sem postinstall |

Nenhuma nova superfície de ameaça além do já catalogado no threat_model do plano.

## Known Stubs

Nenhum stub que bloqueie o objetivo do plano. Os 8 placeholders de tabs na sidebar são intencionais (disabled: true) — serão implementados nas Phases 7/8 conforme ROADMAP.

## Self-Check: PASSED

- [x] `frontend/src/hooks/useReformaParametros.ts` — existe
- [x] `frontend/src/pages/ReformaParametros.tsx` — existe
- [x] `frontend/public/brazil-states.json` — existe (27 features)
- [x] `frontend/package.json` contém react-simple-maps
- [x] `frontend/src/lib/navigation.ts` contém `reforma:` e `startsWith('/reforma')`
- [x] `frontend/src/App.tsx` contém 2 routes para ReformaParametros
- [x] Commits d43044e, 0f4797e, 3766d1f existem no branch
