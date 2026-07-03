---
phase: 11-motor-de-execu-o-do-pacote-fiscal-backend
reviewed: 2026-07-03T17:20:13Z
depth: standard
files_reviewed: 9
files_reviewed_list:
  - backend/handlers/fiscal_oracle_conn.go
  - backend/main.go
  - backend/migrations/146_nfe_itens_desc_outro.sql
  - backend/handlers/nfe_saidas.go
  - backend/handlers/fiscal_group_lookup.go
  - backend/migrations/147_fiscal_execution_items.sql
  - backend/services/oracle_fiscal.go
  - backend/handlers/fiscal_execution.go
  - backend/handlers/fiscal_execution_guards_test.go
findings:
  critical: 1
  warning: 4
  info: 2
  total: 7
status: issues_found
---

# Phase 11: Code Review Report

**Reviewed:** 2026-07-03T17:20:13Z
**Depth:** standard
**Files Reviewed:** 9
**Status:** issues_found

## Summary

Reviewed the batch fiscal-execution engine (Oracle connection helper, grupo-fiscal
lookup, PL/SQL block builder, batch handler, and the two supporting migrations).

The area the review was asked to scrutinize hardest — SQL-injection risk in the
dynamically-built PL/SQL block — is clean: `BuildCalculaImpostoBlock()` in
`services/oracle_fiscal.go` is built exclusively from the fixed `fiscalInParams`/
`fiscalOutFields` metadata tables, and every request-derived value travels through
`sql.Named`/`go_ora.Out` bind variables in `buildBindArgs`. No string concatenation
of request data into the PL/SQL text was found. The IDOR guard on
`/api/fiscal/execute` is also sound: both the header (`nfe_saidas`) and item
(`nfe_saidas_itens`) queries filter by `company_id` derived server-side via
`GetEffectiveCompanyID`, never by a client-supplied `company_id`. Error messages
returned to HTTP clients are consistently genericized; raw Oracle driver/`go-ora`
errors are only ever written to `log.Printf`, never to the JSON response body or to
`fiscal_execution_items.error_message`.

However, the "despesas" (`pDespesas`) leg of the `FiscalInput` mapping — the exact
thing flagged for extra scrutiny — is broken: migration 146 adds `v_outro` to
`nfe_saidas_itens` specifically so the fiscal package has a source for `pDespesas`,
`insertNFeItens` now persists it correctly, but `fiscal_execution.go` never selects
that column and hardcodes `PDespesas: 0`. Any item with a non-zero `vOutro` in the
XML will get a silently wrong tax base from the Oracle call, with the item still
recorded as `status = "ok"`. This is flagged as Critical because it is a silent,
undetectable-by-the-caller wrong-answer in the middle of a tax-calculation engine
(a "loud" error would be preferable and is the standard this fiscal engine holds
itself to elsewhere per the `T-11-18` comments in the same file).

A few secondary robustness gaps were also found around NULL-handling in the batch
item loader and around missing `rows.Err()` checks.

## Critical Issues

### CR-01: `pDespesas` is hardcoded to 0; the `v_outro` column added for exactly this purpose is never read

**File:** `backend/handlers/fiscal_execution.go:150` (item query) and `:294` (`PDespesas: 0`)

**Issue:**
Migration `146_nfe_itens_desc_outro.sql` states explicitly:

> "Necessário como pDesconto/pDespesas — dois dos 23 parâmetros IN do pacote
> fiscal Oracle (PKG_FISCAL_FCTAX.calcula_imposto_produto, Fase 11). Sem estas
> colunas, o motor de execução da Fase 11 não teria de onde ler esses dois
> inputs a partir de nfe_saidas_itens."

`insertNFeItens` (`backend/handlers/nfe_saidas.go:413,424,458`) was correctly
extended in this same phase to persist `v_outro` (mapped from `<det><prod><vOutro>`,
the item-level accessory-expense field) into `nfe_saidas_itens.v_outro`. So the
column is written correctly.

But `FiscalExecutionRunHandler`'s item query never selects it:

```go
itemRows, err := db.Query(`
    SELECT id, COALESCE(c_prod,''), COALESCE(cfop,''), v_prod, COALESCE(v_desc,0), v_ipi
    FROM nfe_saidas_itens
    WHERE nfe_id = $1 AND company_id = $2
    ORDER BY n_item ASC`, req.NfeID, companyID)
```

and `fiscalItemInput` has no `VOutro` field. In `processSingleFiscalItem`:

```go
PDespesas: 0, // NF-e não carrega despesas acessórias por item — nunca v_outro
```

This comment directly contradicts migration 146's stated rationale for adding the
column. Compare with `PDesconto: it.VDesc` a few lines above, which *is* wired
correctly through `v_desc` — so the two new columns added by the same migration
received inconsistent treatment: one is used, one is a write-only dead column.

Net effect: any NF-e item that has a non-zero `vOutro` (accessory expenses) will
be sent to `PKG_FISCAL_FCTAX.calcula_imposto_produto` with `pDespesas = 0` instead
of the real value, silently understating the taxable base for ICMS/ST/PIS/COFINS
for that item. The item is still persisted with `status = "ok"` — there is no
signal to the caller or to `fiscal_execution_items` that the input was incomplete.

This matches a documented "gap known and accepted" note in
`.planning/phases/11-motor-de-execu-o-do-pacote-fiscal-backend/11-05-PLAN.md:125`
("o original hardcoda 0, NAO usa v_outro") — i.e. the team consciously ported the
FB_TESTESFC behavior verbatim. That context downgrades this from "accidental bug"
to "known limitation," but it does not change the fact that (a) the code now
silently produces incorrect fiscal output for a class of real-world NF-e items,
and (b) migration 146's own comment overstates what the column is actually used
for, which will mislead the next engineer who reads it. For a fiscal-calculation
engine, a silently wrong number is strictly worse than a loud, explicit error —
if the gap is to remain deferred, the item should at minimum be flagged (e.g. a
distinct status, or a note in `input_params`) whenever `v_outro <> 0` was ignored,
so the wrong result is discoverable in the Phase 12 comparison UI instead of
looking identical to a fully-correct calculation.

**Fix:**
Either wire `v_outro` through:

```go
itemRows, err := db.Query(`
    SELECT id, COALESCE(c_prod,''), COALESCE(cfop,''), v_prod,
           COALESCE(v_desc,0), COALESCE(v_outro,0), v_ipi
    FROM nfe_saidas_itens
    WHERE nfe_id = $1 AND company_id = $2
    ORDER BY n_item ASC`, req.NfeID, companyID)
...
PDespesas: it.VOutro,
```

or, if `pDespesas=0` is genuinely intentional for this phase (matching the
original), correct migration 146's comment to stop claiming the column feeds
`pDespesas`, and add an explicit signal (log line + a note persisted alongside
the item, e.g. in `input_params` or a dedicated flag) whenever a skipped item has
`v_outro <> 0`, so silently-wrong results are distinguishable from fully-correct
ones downstream.

## Warnings

### WR-01: Missing NULL-safety on `v_prod`/`v_ipi` causes items to silently vanish from the batch, breaking the "never abort other items" guarantee

**File:** `backend/handlers/fiscal_execution.go:149-168`

**Issue:** The item query wraps `c_prod`, `cfop`, and `v_desc` in `COALESCE(...)`
but not `v_prod` or `v_ipi`:

```go
SELECT id, COALESCE(c_prod,''), COALESCE(cfop,''), v_prod, COALESCE(v_desc,0), v_ipi
```

Both columns are nullable in `nfe_entradas_itens`/`nfe_saidas_itens`
(`migrations/075_create_nfe_itens_tables.sql` — `NUMERIC(15,2) DEFAULT 0` with no
`NOT NULL`). If either is `NULL` for a row (e.g. legacy data, a future import path
that doesn't populate them), `itemRows.Scan` fails:

```go
if scanErr := itemRows.Scan(&it.ID, &it.CProd, &it.CFOP, &it.VProd, &it.VDesc, &it.VIPI); scanErr != nil {
    log.Printf(...)
    continue
}
```

The item is silently skipped — it is never appended to `itens`, so it is not
counted in `summary.Total`, gets no row in `fiscal_execution_items`, and produces
no user-visible error. This directly undermines the per-item isolation guarantee
this file repeatedly documents (TPF-05/T-11-17: "isolamento de erro por item —
nunca aborta o lote"): here the item isn't errored, it's silently dropped from
existence.

**Fix:**
```go
SELECT id, COALESCE(c_prod,''), COALESCE(cfop,''),
       COALESCE(v_prod,0), COALESCE(v_desc,0), COALESCE(v_ipi,0)
FROM nfe_saidas_itens ...
```

### WR-02: `rows.Err()` not checked after item iteration

**File:** `backend/handlers/fiscal_execution.go:160-168`

**Issue:**
```go
for itemRows.Next() {
    var it fiscalItemInput
    if scanErr := itemRows.Scan(...); scanErr != nil {
        log.Printf(...)
        continue
    }
    itens = append(itens, it)
}
itemRows.Close()
```
No `if err := itemRows.Err(); err != nil { ... }` check after the loop. A
mid-iteration failure (e.g. connection reset) silently truncates `itens` with no
log line and no error surfaced to the client — the summary will simply report
fewer items than actually exist on the note, indistinguishable from "the note
legitimately has fewer items."

**Fix:**
```go
itemRows.Close()
if err := itemRows.Err(); err != nil {
    log.Printf("FiscalExecutionRunHandler: erro ao iterar nfe_saidas_itens (nfe_id=%s): %v", req.NfeID, err)
    jsonErr(w, http.StatusInternalServerError, "Erro ao carregar itens da nota")
    return
}
```

### WR-03: `openFiscalOracleConn` builds a malformed DSN when credentials are NULL/empty and the stored DSN has no scheme

**File:** `backend/handlers/fiscal_oracle_conn.go:39-52`

**Issue:** Only `oracleDsn` is validated for presence:
```go
if !oracleDsn.Valid || strings.TrimSpace(oracleDsn.String) == "" {
    return nil, fmt.Errorf("DSN Oracle não configurado para a empresa")
}
...
connStr = fmt.Sprintf("oracle://%s:%s@%s", usuarioPlain, senhaPlain, dsnPlain)
```
If `oracle_usuario`/`oracle_senha` are `NULL` in `erp_bridge_config` (both are
plain `sql.NullString`, unchecked for `.Valid`) and the stored DSN doesn't already
include a scheme, this silently builds `oracle://:@host:port/service` — go-ora
will fail with an unhelpful auth/parse error, and the caller only sees the generic
"Falha ao conectar ao Oracle" message, making the actual misconfiguration (missing
credentials vs. wrong credentials vs. unreachable host) indistinguishable from the
logs alone.

**Fix:** Validate `oracleUsuario.Valid`/`oracleSenha.Valid` alongside `oracleDsn`
(when the DSN doesn't already carry a scheme) and log a more specific message,
e.g. `"credenciais Oracle incompletas (usuario/senha ausentes) para company_id=%s"`.

### WR-04: No automated test coverage for the IDOR/company-scoping guard on `/api/fiscal/execute`

**File:** `backend/handlers/fiscal_execution_guards_test.go`

**Issue:** The test file (by its own doc comment) intentionally covers only the
method (405) and auth (401) guards, since those don't require touching the DB.
The actual IDOR protection this phase's design docs call out as `T-11-14`
(`nfe_id` scoped by `company_id` server-side, never trusting the request body) has
no regression test anywhere in this file set — a future refactor of the
`WHERE id = $1 AND company_id = $2` clause in `FiscalExecutionRunHandler` could
silently regress cross-tenant access with no test failing.

**Fix:** Not blocking, but worth adding an integration-style test (using a test DB,
consistent with other handler test suites in this repo) that asserts a request for
an `nfe_id` belonging to a different `company_id` returns 404, not the note's data.

## Info

### IN-01: `origem`/`ncm` returned by `lookupGrupoFiscal` are discarded

**File:** `backend/handlers/fiscal_execution.go:258`

**Issue:** `grupoFiscal, _, _, err := lookupGrupoFiscal(ctx, oracleDB, it.CProd, nfe.CodEmpresa)` throws
away the `origem`/`ncm` values that `lookupGrupoFiscal` fetches from Oracle on
every call. They aren't part of the 23 `FiscalInput` params, so this may be
intentional (reserved for Phase 12), but as written it's an unexplained silent
discard with no comment clarifying why two of the three query results are unused.

**Fix:** Add a one-line comment noting these are intentionally unused in Phase 11
(reserved for a future comparison feature), or drop them from the return signature
until needed.

### IN-02: Oracle-ping and batch-connect failures return HTTP 200 with `"ok": false`

**File:** `backend/handlers/fiscal_oracle_conn.go:94-124`

**Issue:** All failure branches of `FiscalOraclePingHandler` (`openFiscalOracleConn`
failure, `PingContext` failure, `SELECT 1 FROM dual` failure) write
`{"ok": false, "error": "..."}` without ever calling `w.WriteHeader(...)`, so the
response is `200 OK`. This is a reasonable choice for a UI-consumed "smoke test"
endpoint, but any infra-level monitoring that checks HTTP status codes rather than
parsing the JSON body will treat a broken Oracle connection as a healthy response.

**Fix:** Consider `w.WriteHeader(http.StatusBadGateway)` before encoding the
`ok:false` body, if this endpoint is ever wired into automated health checks.

---

_Reviewed: 2026-07-03T17:20:13Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
