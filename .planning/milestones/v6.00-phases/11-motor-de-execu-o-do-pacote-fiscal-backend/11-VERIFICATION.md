---
phase: 11-motor-de-execu-o-do-pacote-fiscal-backend
verified: 2026-07-03T17:26:23Z
status: human_needed
score: 14/16 must-haves verified
overrides_applied: 0
must_haves:
  truths:
    - "Grupo fiscal resolvido via Oracle prod/PRODB por item de nfe_saidas_itens"
    - "Filial não mapeada (fora de Recife/PE) retorna erro explícito, nunca cod_empresa adivinhado"
    - "Produto ausente em prod/PRODB retorna status sem_grupo_fiscal, não fatal"
    - "v_desc/v_outro persistidos por item, disponíveis como pDesconto/pDespesas do pacote fiscal"
    - "Reimport de XML atualiza v_desc/v_outro via ON CONFLICT"
    - "CallFiscalPackage executa via bloco PL/SQL 100% estático (zero concatenação de input), 23 IN / ~88 OUT via bind seguro"
    - "Campos OUT string usam go_ora.Out com Size explícito; IdRegraCalculo* tipados como string"
    - "fiscal_execution_items existe com status por item, colunas típicas indexáveis e full_result JSONB (~88 campos)"
    - "Um item que falha não impede o processamento dos demais itens do lote"
    - "Execução em lote respeita cap de concorrência 5 e timeout de 15s por item"
    - "Cada item é seu próprio INSERT ... ON CONFLICT, nunca uma transação única do lote"
    - "Backend Go abre conexão síncrona ao Oracle via erp_bridge_config, nunca o bridge Python"
    - "Admin dispara smoke test de conectividade Oracle e vê sucesso/falha explícito"
    - "Credenciais Oracle e erros de conexão nunca vazam no corpo da resposta HTTP"
    - "Uma execução real de lote contra Oracle produz linhas fiscal_execution_items com status='ok' e valores numéricos calculados"
    - "As duas pegadinhas do go-ora (buffer OUT string; IdRegraCalculo* VARCHAR2) NÃO se manifestam com dados reais não-vazios"
  artifacts:
    - path: "backend/handlers/fiscal_oracle_conn.go"
      provides: "openFiscalOracleConn + FiscalOraclePingHandler"
    - path: "backend/handlers/fiscal_group_lookup.go"
      provides: "resolveCodEmpresa + lookupGrupoFiscal"
    - path: "backend/services/oracle_fiscal.go"
      provides: "FiscalInput/FiscalResult + BuildCalculaImpostoBlock + CallFiscalPackage"
    - path: "backend/handlers/fiscal_execution.go"
      provides: "FiscalExecutionRunHandler + processFiscalBatch + persistFiscalItemResult"
    - path: "backend/handlers/fiscal_execution_guards_test.go"
      provides: "guard tests 405/401"
    - path: "backend/migrations/146_nfe_itens_desc_outro.sql"
      provides: "v_desc/v_outro em nfe_saidas_itens e nfe_entradas_itens"
    - path: "backend/migrations/147_fiscal_execution_items.sql"
      provides: "tabela fiscal_execution_items"
  key_links:
    - from: "fiscal_oracle_conn.go"
      to: "erp_bridge_config"
      via: "SELECT + DecryptFieldWithFallback"
    - from: "fiscal_execution.go processSingleFiscalItem"
      to: "services.CallFiscalPackage"
      via: "in.PDespesas <- it.VOutro (CR-01 fix)"
    - from: "processFiscalBatch"
      to: "fiscal_execution_items"
      via: "semáforo cap 5 + timeout 15s/item + upsert por item"
    - from: "main.go"
      to: "FiscalExecutionRunHandler / FiscalOraclePingHandler"
      via: "withAuth(..., \"admin\")"
human_verification:
  - test: "Rodar POST /api/fiscal/execute com credenciais Oracle REAIS (não placeholder) contra uma nfe_id real de Recife/PE (CNPJ raiz 10230480) com vários itens, em ambiente com acesso à rede Oracle prod/PRODB + FCCORP_BKP"
    expected: "Resposta JSON com ok > 0; SELECT status, count(*) FROM fiscal_execution_items GROUP BY status mostra ao menos uma linha status='ok' com base_calculo_icms/valor_icms preenchidos; full_result mostra campos OUT string preenchidos (sem ORA-06502 buffer too small) e IdRegraCalculo* como texto tipo \"IVA_...\" (sem ORA-06502 character-to-number)"
    why_human: "Nenhuma sessão automatizada (incluindo esta verificação e a execução original do Plan 11-06) teve acesso a uma senha Oracle real — apenas ORA-01017 (rejeição de autenticação) foi observado. As duas pegadinhas do driver go-ora (Pitfall 1: buffer OUT string; Pitfall 2: IdRegraCalculo* VARCHAR2) só se manifestam quando o pacote Oracle retorna dados não-vazios; até hoje isso nunca aconteceu em nenhuma execução real. Requer credencial Oracle de produção que só um humano com acesso à infraestrutura pode fornecer."
  - test: "Rodar POST /api/fiscal/execute para uma nota de filial fora de Recife/PE (fora do mapa codEmpresaPorCNPJRaiz) e confirmar que o item recebe status explícito ('error', mensagem 'cod_empresa não mapeado...') sem abortar os demais itens do lote"
    expected: "summary agregado correto (total=N, error>=1 para os itens da filial não mapeada) e nenhum crash/abort do processamento"
    why_human: "Estrutura de código comprova isolamento por item (goroutine + recover + status por item), mas não há evidência de execução real com uma nota de filial não-Recife/PE — apenas inferência estrutural do código."
---

# Phase 11: Motor de Execução do Pacote Fiscal (Backend) Verification Report

**Phase Goal:** Dado um item de `nfe_saidas_itens`, o sistema resolve seu grupo fiscal no Oracle (prod/PRODB), executa `PKG_FISCAL_FCTAX.calcula_imposto_produto` com bind seguro e persiste os ~88 campos de saída em `fiscal_execution_items`, em lote, com isolamento de erro e limites de concorrência/timeout — sem nenhuma tela ainda, apenas a fundação de dados que a Phase 12 vai exibir.
**Verified:** 2026-07-03T17:26:23Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Pre-Verification Context: CR-01 Fix Applied

Before this verification ran, code review (`11-REVIEW.md`) found a Critical issue (CR-01): `fiscal_execution.go` hardcoded `PDespesas: 0` instead of reading the newly-added `v_outro` column (added by Plan 11-02/migration 146 specifically to feed `pDespesas`). This produced silently-wrong tax bases for any item with non-zero accessory expenses, persisted as `status='ok'` (no signal of the problem). The orchestrator applied a direct fix in commit `e1e4102` before this verification.

**Fix independently confirmed in the current codebase** (not just trusted from the commit message):

- `backend/handlers/fiscal_execution.go:151` — item query now selects `COALESCE(v_outro,0)` (and also fixes WR-01: `COALESCE(v_prod,0)`, `COALESCE(v_ipi,0)`)
- `backend/handlers/fiscal_execution.go:69` — `fiscalItemInput.VOutro float64` field added
- `backend/handlers/fiscal_execution.go:163` — `Scan(&it.ID, &it.CProd, &it.CFOP, &it.VProd, &it.VDesc, &it.VOutro, &it.VIPI)` reads the new column
- `backend/handlers/fiscal_execution.go:169-174` — `itemRows.Err()` check added after the loop (WR-02 fix)
- `backend/handlers/fiscal_execution.go:305` — `PDespesas: it.VOutro` (no longer hardcoded to 0)
- `backend/handlers/fiscal_oracle_conn.go:51-53` — validates `usuarioPlain`/`senhaPlain` non-empty before building the no-scheme DSN (WR-03 fix)
- `cd backend && go build ./...` and `go vet ./handlers/ ./services/` both exit 0 with the fix in place

CR-01, WR-01, WR-02, WR-03 are all resolved in the current code, not just claimed.

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Grupo fiscal resolvido via Oracle prod/PRODB por item | ✓ VERIFIED | `fiscal_group_lookup.go:56` `lookupGrupoFiscal` uses exact validated query with `sql.Named(:codigoProduto, :codEmpresa)` |
| 2 | Filial não mapeada retorna erro explícito, nunca cod_empresa adivinhado | ✓ VERIFIED | `resolveCodEmpresa` (line 33-43) returns `fmt.Errorf(...)` on map miss, never a guessed value |
| 3 | Produto ausente em prod/PRODB → status `sem_grupo_fiscal`, não fatal | ✓ VERIFIED | `errSemGrupoFiscal` + `processSingleFiscalItem` maps `sql.ErrNoRows` to non-fatal status, confirmed live in Plan 11-06 session against synthetic data |
| 4 | v_desc/v_outro persistidos por item, disponíveis como pDesconto/pDespesas | ✓ VERIFIED | Migration 146 confirmed (both tables, idempotent); `insertNFeItens` writes both; `fiscal_execution.go` now reads `v_outro` and maps `PDespesas: it.VOutro` (post CR-01 fix) |
| 5 | Reimport atualiza v_desc/v_outro via ON CONFLICT | ✓ VERIFIED | `nfe_saidas.go:446-447` `ON CONFLICT ... DO UPDATE SET v_desc = EXCLUDED.v_desc, v_outro = EXCLUDED.v_outro` |
| 6 | CallFiscalPackage: bloco PL/SQL 100% estático, 23 IN / ~88 OUT via bind | ✓ VERIFIED (code) | `BuildCalculaImpostoBlock` builds only from `fiscalInParams`(23)/`fiscalOutFields`(88) fixed tables; no `fmt.Sprintf` of request values found; counted programmatically: 23 IN entries, 88 OUT entries, 88 `FiscalResult` fields — all match |
| 7 | OUT string usa go_ora.Out{Size}; IdRegraCalculo* como string | ✓ VERIFIED | `buildBindArgs` branches on `reflect.Kind() == reflect.String` → `go_ora.Out{Dest,Size:4000}`; all 5 `IdRegraCalculo*` fields typed `string` in `FiscalResult` |
| 8 | fiscal_execution_items existe com status/colunas típicas/full_result JSONB | ✓ VERIFIED | Migration 147: `CREATE TABLE IF NOT EXISTS`, `UNIQUE(nfe_item_id)`, `full_result JSONB NOT NULL`, IBS/CBS columns, 2 indexes |
| 9 | Um item que falha não impede o processamento dos demais | ✓ VERIFIED | `processFiscalBatch` per-item goroutine with `defer recover()`; live-tested in Plan 11-06 session (synthetic data): item failed with Oracle auth error, `summary.Total=1` still correctly reported |
| 10 | Cap de concorrência 5 + timeout 15s por item | ✓ VERIFIED | `make(chan struct{}, 5)` + `context.WithTimeout(ctx, 15*time.Second)` inside the per-item goroutine |
| 11 | Cada item é seu próprio UPSERT, nunca transação única do lote | ✓ VERIFIED | `persistFiscalItemResult` issues one `pgDB.Exec(...)` per item with `ON CONFLICT (nfe_item_id) DO UPDATE`; no `Begin()`/`Tx` wrapping the batch loop |
| 12 | Conexão síncrona Oracle via erp_bridge_config, nunca o bridge Python | ✓ VERIFIED | `openFiscalOracleConn` reads `erp_bridge_config`, decrypts via `DecryptFieldWithFallback`, opens directly via `sql.Open("oracle", ...)` — independent of the Python bridge |
| 13 | Admin smoke test de conectividade Oracle com sucesso/falha explícito | ✓ VERIFIED | `POST /api/fiscal/oracle-ping` registered with `withAuth(..., "admin")`; reachability independently proven twice (Plan 11-01 and Plan 11-06 sessions) via `ORA-01017` (auth rejection, not network failure) |
| 14 | Credenciais/erros nunca vazam no corpo HTTP | ✓ VERIFIED | `grep err.Error()` in HTTP-response-writing code paths of the 3 files returns no matches; all failure branches use generic literals + `log.Printf` |
| 15 | Execução real de lote contra Oracle produz status='ok' com valores calculados | ? UNCERTAIN | Never observed. Every execution attempt (Plan 11-01, Plan 11-06, this verification) lacked a valid Oracle password; only `ORA-01017` (auth rejection) was reached. `CallFiscalPackage` has never actually been invoked with real credentials. |
| 16 | Pitfalls 1/2 do go-ora (buffer OUT string; IdRegraCalculo* VARCHAR2) NÃO se manifestam com dados reais não-vazios | ? UNCERTAIN | Confirmed only by static code inspection (`go_ora.Out{Size:4000}`, string typing) — the runtime code path that would trigger these bugs (`CallFiscalPackage` returning non-empty OUT values) has never executed |

**Score:** 14/16 truths verified (2 require human action with real production Oracle credentials)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `backend/handlers/fiscal_oracle_conn.go` | `openFiscalOracleConn` + Ping route | ✓ VERIFIED | Present, substantive, wired into `main.go` |
| `backend/handlers/fiscal_group_lookup.go` | `resolveCodEmpresa` + `lookupGrupoFiscal` | ✓ VERIFIED | Present, uses exact validated Oracle query |
| `backend/services/oracle_fiscal.go` | `FiscalInput`/`FiscalResult`/`CallFiscalPackage` | ✓ VERIFIED | 23 IN params, 88 OUT fields, both counted programmatically |
| `backend/handlers/fiscal_execution.go` | Batch handler + processFiscalBatch + persist | ✓ VERIFIED | Present, wired to `main.go`, includes CR-01 fix |
| `backend/handlers/fiscal_execution_guards_test.go` | 405/401 guard tests | ✓ VERIFIED | `go test ./handlers/ -run TestFiscalExecution_Guards -v` passes (2/2 subtests) |
| `backend/migrations/146_nfe_itens_desc_outro.sql` | v_desc/v_outro on both item tables | ✓ VERIFIED | Idempotent, both tables covered |
| `backend/migrations/147_fiscal_execution_items.sql` | fiscal_execution_items table | ✓ VERIFIED | UNIQUE(nfe_item_id), FKs CASCADE, full_result JSONB, IBS/CBS columns, 2 indexes |
| `backend/main.go` routes | `/api/fiscal/oracle-ping`, `/api/fiscal/execute` | ✓ VERIFIED | Both registered `withAuth(..., "admin")` (lines 532, 535) |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `fiscal_oracle_conn.go` | `erp_bridge_config` | SELECT + DecryptFieldWithFallback | ✓ WIRED | Confirmed in code |
| `fiscal_execution.go processSingleFiscalItem` | `services.CallFiscalPackage` | `in.PDespesas <- it.VOutro` | ✓ WIRED (post-fix) | CR-01 fix confirmed present and correct |
| `processFiscalBatch` | `fiscal_execution_items` | semáforo cap 5 + timeout 15s + upsert por item | ✓ WIRED | All three mechanisms present in code |
| `main.go` | `FiscalExecutionRunHandler`/`FiscalOraclePingHandler` | `withAuth(..., "admin")` | ✓ WIRED | Both routes confirmed registered |
| `services.CallFiscalPackage` | Oracle `PKG_FISCAL_FCTAX` | `ExecContext` with PL/SQL block + bind args | ⚠️ WIRED but never exercised successfully | Code path complete and structurally sound; never run to completion against real Oracle data |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Backend compiles | `cd backend && go build ./...` | exit 0 | ✓ PASS |
| Static analysis clean | `cd backend && go vet ./handlers/ ./services/` | exit 0, no output | ✓ PASS |
| Guard tests pass | `cd backend && go test ./handlers/ -run TestFiscalExecution -v` | 2/2 subtests PASS | ✓ PASS |
| fiscalInParams count | programmatic count of `fiscalInParams` entries | 23 | ✓ PASS |
| fiscalOutFields / FiscalResult field count | programmatic count | 88 / 88 (match) | ✓ PASS |
| Live Oracle round-trip with real data | N/A — requires production credentials | not run | ? SKIP (routed to human verification) |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| TPF-01 | 11-03 | Lookup de grupo fiscal via Oracle por item | ✓ SATISFIED | `fiscal_group_lookup.go` |
| TPF-02 | 11-02 | Extensão de nfe_saidas_itens/insertNFeItens para desconto/despesas | ✓ SATISFIED | Migration 146 + struct/INSERT + CR-01 fix wires it through to the fiscal call |
| TPF-03 | 11-01, 11-04 | Serviço de execução do PKG_FISCAL_FCTAX via PL/SQL estático com bind seguro | ✓ SATISFIED (code) — runtime proof pending | `oracle_fiscal.go`; live proof is the human-verification item |
| TPF-04 | 11-03 | Tabela fiscal_execution_items com ~88 campos de saída | ✓ SATISFIED | Migration 147 |
| TPF-05 | 11-01, 11-05, 11-06 | Endpoint de execução em lote com concorrência/timeout/isolamento de erro | ✓ SATISFIED (mechanism); live proof of a successful Oracle round-trip pending | `fiscal_execution.go` |

No orphaned requirements — all 5 (TPF-01 through TPF-05) mapped to Phase 11 in `REQUIREMENTS.md` are claimed by at least one plan and have supporting code evidence.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `backend/handlers/fiscal_execution_guards_test.go` | n/a | No automated regression test for the IDOR/company-scoping guard (`WHERE id=$1 AND company_id=$2`) | ⚠️ Warning | A future refactor of the WHERE clause could silently regress cross-tenant access with no test failing (WR-04 from `11-REVIEW.md`, not addressed by the CR-01 fix commit) |
| `backend/handlers/fiscal_execution.go:258` | 258 | `lookupGrupoFiscal` returns `origem`/`ncm` but caller discards both with `_, _` | ℹ️ Info | Not a defect — likely reserved for Phase 12, but undocumented (IN-01 from review, not addressed) |
| `backend/handlers/fiscal_oracle_conn.go:94-124` | 94-124 | Oracle-ping/batch-connect failures return HTTP 200 with `{"ok":false}` (no `WriteHeader`) | ℹ️ Info | Reasonable for a UI-consumed smoke test; would mislead infra-level health checks that only look at status code (IN-02 from review, not addressed) |

No blocker-level anti-patterns (no unresolved `TBD`/`FIXME`/`XXX`, no stub returns, no `err.Error()` leaked to HTTP clients) were found in the 9 files this phase touched.

### Human Verification Required

### 1. Real Oracle execution proving successful calculation + absence of go-ora Pitfalls 1/2

**Test:** Run `POST /api/fiscal/execute` with real (non-placeholder) Oracle credentials configured in `erp_bridge_config` for the Recife/PE company (CNPJ raiz `10230480`), targeting a real `nfe_id` with several items, from an environment with actual network access to Oracle prod/PRODB + FCCORP_BKP.
**Expected:** JSON response with `ok > 0`; `SELECT status, count(*) FROM fiscal_execution_items GROUP BY status` shows at least one `status='ok'` row with `base_calculo_icms`/`valor_icms` etc. populated with real numeric values; inspecting `full_result` of an `'ok'` row shows OUT string fields populated (no `ORA-06502: character string buffer too small`) and `IdRegraCalculo*` fields as text like `"IVA_..."` (no `ORA-06502: character to number conversion error`).
**Why human:** No automated session across this phase (Plan 11-01, Plan 11-06, or this verification) has had access to a valid Oracle password — every attempt reached only `ORA-01017` (authentication rejected), which proves network/protocol reachability but never actually invokes `PKG_FISCAL_FCTAX.calcula_imposto_produto` with real data. The two go-ora driver pitfalls this phase explicitly worried about (`11-RESEARCH.md` "Common Pitfalls 1 and 2") can only manifest when the Oracle call actually returns non-empty OUT values — this has literally never happened yet. This is exactly the checkpoint Plan `11-06` Task 1 (`gate="blocking"`) was designed to gate on, and its own SUMMARY.md documents this as unresolved ("Validação Pendente" / "User Setup Required").

### 2. Filial não mapeada — comportamento de erro explícito sem abortar o lote

**Test:** Run `POST /api/fiscal/execute` for a nota emitted by a filial outside the confirmed CNPJ root map (`codEmpresaPorCNPJRaiz = {"10230480": 2}`).
**Expected:** The item(s) for that nota get `status='error'` with an explicit message (`cod_empresa não mapeado...`), and the rest of the batch (if mixed) is unaffected; summary counts are accurate.
**Why human:** The isolation mechanism is structurally sound in code (per-item goroutine + `resolveCodEmpresa` returning an explicit error), but no live execution against a real unmapped-filial nota has been observed — only inferred from code and unit-level guard tests that don't touch this path.

### Gaps Summary

No code-level gaps block the phase. All artifacts exist, are substantive, and are wired correctly, including the CR-01 critical fix (which was independently re-verified in this session, not just trusted from the commit message) and the accompanying WR-01/WR-02/WR-03 hardening. `go build`, `go vet`, and the guard test suite all pass cleanly with zero warnings.

The phase's own risk register (`11-RESEARCH.md`) and Plan `11-06`'s explicit `gate="blocking"` human-verify task both identify the same open item: the two go-ora driver pitfalls (OUT string buffer sizing; `IdRegraCalculo*` VARCHAR2 typing) have only been confirmed absent by **static inspection**, never by a real, successful Oracle round-trip returning non-empty data. Every attempt to exercise this path in every session to date (including this verification) failed at the authentication step (`ORA-01017`) for lack of a real Oracle password — an external-service dependency that only a human with production/Coolify access can supply. This is not a code defect; it is the single genuinely unresolved technical risk of the phase, and it requires human action to close, not further coding.

---

_Verified: 2026-07-03T17:26:23Z_
_Verifier: Claude (gsd-verifier)_
