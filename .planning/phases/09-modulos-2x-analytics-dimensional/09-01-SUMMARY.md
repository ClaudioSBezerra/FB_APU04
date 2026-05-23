---
phase: 09-modulos-2x-analytics-dimensional
plan: "01"
subsystem: backend
tags: [reforma-tributaria, analytics, cfop, ncm, uf-destino, b2b-b2c, go, handlers]
dependency_graph:
  requires: []
  provides:
    - GET /api/reforma/modulo2/cfop (RFMC-01)
    - GET /api/reforma/modulo2/cfop/csv (RFMC-01)
    - GET /api/reforma/modulo2/ncm (RFMC-02)
    - GET /api/reforma/modulo2/ncm/csv (RFMC-02)
    - GET /api/reforma/modulo2/uf-destino (RFMC-03)
    - GET /api/reforma/modulo2/b2b-b2c (RFMC-04)
  affects:
    - backend/main.go
tech_stack:
  added: []
  patterns:
    - LATERAL join com longest-prefix-wins para NCM
    - readModulo2Params via tabela_aliquotas (migration 090)
    - company_id parametrizado via $1 (IDOR protection)
    - CSV com Content-Disposition attachment + csv.NewWriter + Flush/Error
key_files:
  created:
    - backend/handlers/reforma_modulo2.go
    - backend/handlers/reforma_modulo2_test.go
  modified:
    - backend/main.go
decisions:
  - "readModulo2Params usa tabela_aliquotas via target_ano (não colunas removidas de reforma_parametros)"
  - "IBS/CBS de Transferências = 0.0 (regime distinto na transição EC 132)"
  - "Defaults 26.5% IBS e 9.9% CBS (fase plena 2033, EC 132/2023)"
  - "Method-not-allowed tests adicionados para todos os 6 handlers (além do mínimo do plano)"
metrics:
  duration: "~5 min"
  completed_date: "2026-05-23"
  tasks_completed: 3
  tasks_total: 3
  files_created: 2
  files_modified: 1
requirements_satisfied: [RFMC-01, RFMC-02, RFMC-03, RFMC-04]
---

# Phase 9 Plan 01: Backend Módulos 2.x Analytics Dimensional — Summary

**One-liner:** 4 handlers analíticos JSON + 2 CSV (CFOP, NCM, UF-Destino, B2B/B2C) com LATERAL join NCM, segmentação ind_final+CPF e 6 rotas autenticadas registradas em main.go.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Handlers CFOP e NCM (JSON + CSV) | e6229ad | backend/handlers/reforma_modulo2.go (criado, 724 linhas) |
| 2 | Handlers UF Destino e B2B/B2C + rotas main.go | d5fccd7 | backend/main.go (+50 linhas) |
| 3 | Guard tests para os 6 handlers | fb99577 | backend/handlers/reforma_modulo2_test.go (criado, 113 linhas) |

## What Was Built

### reforma_modulo2.go (724 linhas)

**Módulo 2.2 — CfopAnalysisHandler (RFMC-01):**
- Agrupamento por natureza CFOP via CASE SQL (Transferência / Revenda / Uso e Consumo / Ativo Permanente / Exportação / Outras Operações)
- IBS/CBS projetado = 0 para Transferências; calculado para demais
- TotalIBS e TotalCBS acumulados excluindo transferências
- CSV handler com `attachment; filename="analise-cfop.csv"`

**Módulo 2.1 — NcmAnalysisHandler (RFMC-02):**
- LATERAL join com longest-prefix-wins em ncm_cclasstrib_reforma
- AliqICMSEfet = vl_icms/vl_prod*100 (zerado se vl_prod = 0)
- IBSProjetado e CBSProjetado com fator de redução ibs_reducao_pct / cbs_reducao_pct
- IsFlag = true quando cclasstrib IS NOT NULL
- CSV handler com `attachment; filename="analise-ncm.csv"`
- LIMIT 100 para performance

**Módulo 2.3 — UfDestinoHandler (RFMC-03):**
- Fonte: nfe_saidas, agrupando por dest_uf com COALESCE('N/A')
- ICMS real (v_icms) + IBS/CBS projetado por UF

**Módulo 2.4 — B2bB2cHandler (RFMC-04):**
- Segmentação em 3 vias: b2b_credit / b2c / sem_classificacao
- ind_final='1' → b2c; ind_final='0' → b2b_credit
- Fallback: NULL ind_final + LENGTH(dest_cnpj_cpf)=11 → b2c; =14 → b2b_credit
- QtdSemIndFinal acumulado para aviso de UI sobre notas históricas

**Helper readModulo2Params:**
- Derivado de tabela_aliquotas via target_ano (migration 090 removeu colunas de reforma_parametros)
- Defaults 26.5% IBS e 9.9% CBS para sql.ErrNoRows

### main.go (+50 linhas)

6 rotas registradas com `AuthMiddleware(handler, "")` (role "" = read-only analytics):
- /api/reforma/modulo2/cfop
- /api/reforma/modulo2/cfop/csv
- /api/reforma/modulo2/ncm
- /api/reforma/modulo2/ncm/csv
- /api/reforma/modulo2/uf-destino
- /api/reforma/modulo2/b2b-b2c

### reforma_modulo2_test.go (113 linhas)

- 6 testes de criação (non-nil) para todos os handlers
- 6 testes method-not-allowed (POST → 405) para todos os handlers
- Todos PASS sem acesso a DB

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] readModulo2Params: colunas aliq_ibs_pct/aliq_cbs_pct removidas de reforma_parametros**
- **Found during:** Task 1 — verificação do schema de migrations antes de compilar
- **Issue:** O plano especificava query direta `SELECT COALESCE(aliq_ibs_pct, 26.5), COALESCE(aliq_cbs_pct, 9.9) FROM reforma_parametros WHERE company_id = $1`, mas a migration 090 removeu essas colunas (`DROP COLUMN IF EXISTS aliq_ibs_pct, aliq_cbs_pct`) para centralizar alíquotas em `tabela_aliquotas`.
- **Fix:** Helper `readModulo2Params` usa `JOIN tabela_aliquotas ta ON ta.ano = rp.target_ano` com `COALESCE(ta.perc_ibs_uf + ta.perc_ibs_mun, 26.5)` e `COALESCE(ta.perc_cbs, 9.9)` — mesmo padrão de reforma_modulo1.go.
- **Files modified:** backend/handlers/reforma_modulo2.go
- **Commit:** e6229ad (incluído no commit da Task 1)

**2. [Rule 2 - Critical Functionality] Method-not-allowed para todos os 6 handlers (não apenas CfopAnalysisHandler)**
- **Found during:** Task 3 — implementação dos guard tests
- **Issue:** O plano especificava apenas 1 teste method-not-allowed (CfopAnalysisHandler). Os outros 5 handlers também têm method guard e devem ser testados para cobertura completa.
- **Fix:** Adicionados 6 testes method-not-allowed (um por handler) em vez de apenas 1.
- **Files modified:** backend/handlers/reforma_modulo2_test.go
- **Commit:** fb99577

## Verification

```
cd backend && go build ./...           # EXIT 0
cd backend && go vet ./handlers/       # EXIT 0 (sem warnings)
cd backend && go test ./handlers/ -run "TestCfopAnalysisHandler|TestNcmAnalysisHandler|TestUfDestinoHandler|TestB2bB2cHandler"  # PASS (8 testes)
grep -c "api/reforma/modulo2/" backend/main.go  # 6
grep -nE "Sprintf.*company_id" backend/handlers/reforma_modulo2.go  # 0 linhas
```

## Known Stubs

Nenhum — todos os handlers retornam dados reais do banco ou array vazio `[]` (empty-slice guard). Não há valores hardcoded fluindo para UI.

## Threat Flags

Nenhum — todos os 4 endpoints analíticos estão no threat model do plano (T-09-01 a T-09-04) e as mitigações foram aplicadas:
- company_id via GetEffectiveCompanyID (IDOR protection — T-09-01)
- Todos os filtros via placeholders $N (SQL injection — T-09-02)
- CSV handlers com mesma proteção JWT (T-09-03)
- Todos os endpoints com AuthMiddleware(handler, "") — exigem JWT válido (T-09-04)

## Self-Check: PASSED

- `backend/handlers/reforma_modulo2.go`: FOUND (724 linhas)
- `backend/handlers/reforma_modulo2_test.go`: FOUND (113 linhas)
- `backend/main.go`: MODIFIED (6 rotas confirmadas)
- Commit e6229ad: FOUND
- Commit d5fccd7: FOUND
- Commit fb99577: FOUND
