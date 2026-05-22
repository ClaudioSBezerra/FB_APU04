---
phase: 06-infraestrutura-reforma-tribut-ria
plan: "03"
subsystem: backend
tags: [go, handler, reforma-tributaria, api, upsert, rbac]
dependency_graph:
  requires: ["06-01"]
  provides: ["06-04"]
  affects: ["backend/handlers/reforma_config.go", "backend/main.go"]
tech_stack:
  added: []
  patterns:
    - "GET+PUT multi-method route com switch r.Method (padrão /api/rfb/credentials)"
    - "UPSERT ON CONFLICT (company_id) DO UPDATE SET ... updated_at=CURRENT_TIMESTAMP"
    - "company_id do JWT via GetEffectiveCompanyID (IDOR protection)"
    - "Role check no AuthMiddleware, não no handler"
key_files:
  created:
    - backend/handlers/reforma_config.go
  modified:
    - backend/main.go
decisions:
  - "Rota /api/reforma/parametros posicionada FORA do bloco if appModule != simulador — Reforma é produto simulador"
  - "GET sem restrição de role (D-06); PUT com role admin (D-07) — controle no AuthMiddleware"
  - "company_id sempre do JWT ($1 no UPSERT), req.CompanyID do body é ignorado (T-06-09)"
metrics:
  duration: "~15min"
  completed: "2026-05-22"
  tasks_completed: 2
  tasks_total: 2
  files_created: 1
  files_modified: 1
---

# Phase 06 Plan 03: Handler GET+PUT /api/reforma/parametros Summary

**One-liner:** Handlers Go para leitura e upsert de parâmetros da Reforma Tributária por empresa, com proteção IDOR e role admin no PUT.

## What Was Built

Criados `GetReformaParametrosHandler` e `PutReformaParametrosHandler` no novo arquivo `backend/handlers/reforma_config.go`, e registrada a rota multi-método `/api/reforma/parametros` em `backend/main.go`.

### Task 1: reforma_config.go — handlers GET e PUT com UPSERT

Commit `6a1af4a`

Arquivo criado com:

- `struct ReformaParametros` com 7 campos (company_id, target_ano, aliq_ibs_pct, aliq_cbs_pct, fator_simples_pct, taxa_cdi_anual_pct, prazo_medio_dias) e tags JSON snake_case exatas
- `GetReformaParametrosHandler`: extrai claims do contexto JWT, usa `GetEffectiveCompanyID`, faz SELECT em `reforma_parametros`; retorna `{"parametros": null}` se `sql.ErrNoRows` (empresa não configurada), 401 se claims inválidas
- `PutReformaParametrosHandler`: mesma extração de claims/company_id; valida ranges V5 ASVS (aliq_*_pct ∈ [0,100], prazo_medio_dias ∈ [1,3650], target_ano ∈ [2024,2100]); executa UPSERT com `ON CONFLICT (company_id) DO UPDATE SET ... updated_at=CURRENT_TIMESTAMP`; `companyID` do JWT como $1 (nunca `req.CompanyID`)

### Task 2: main.go — registrar rota /api/reforma/parametros

Commit `6a58dda`

Adicionado bloco multi-method fora do `if appModule != "simulador"`:
- GET → `AuthMiddleware(GetReformaParametrosHandler, "")` — qualquer autenticado lê (D-06)
- PUT → `AuthMiddleware(PutReformaParametrosHandler, "admin")` — somente admin (D-07, 403 para não-admin)
- default → 405 Method Not Allowed

## Verification

```
go build ./handlers/  # OK
go build .            # OK
grep "/api/reforma/parametros" main.go  # match
grep 'handlers.GetReformaParametrosHandler(database), ""' main.go  # match
grep 'handlers.PutReformaParametrosHandler(database), "admin"' main.go  # match
grep "ON CONFLICT (company_id) DO UPDATE" handlers/reforma_config.go  # match
```

## Deviations from Plan

None — plano executado exatamente como escrito.

## Threat Model Coverage

| Threat | Mitigation | Status |
|--------|-----------|--------|
| T-06-08: Usuário não-admin tenta PUT alíquotas | `AuthMiddleware(..., "admin")` em main.go rejeita com 403 | Mitigado |
| T-06-09: IDOR via company_id no body | `companyID` do JWT como $1; `req.CompanyID` ignorado | Mitigado |
| T-06-10: Injeção SQL | Todos os valores via $1..$7 parametrizados | Mitigado |
| T-06-11: Alíquotas absurdas | Validação de range retorna 400 | Mitigado |

## Known Stubs

Nenhum — os handlers são funcionais e prontos para uso pelo plano 06-04 (frontend).

## Self-Check: PASSED

- `backend/handlers/reforma_config.go` — FOUND
- `backend/main.go` — MODIFIED (grep confirmado)
- commit `6a1af4a` — FOUND
- commit `6a58dda` — FOUND
