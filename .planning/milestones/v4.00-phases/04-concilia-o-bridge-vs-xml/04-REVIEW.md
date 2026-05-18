---
phase: 04-conciliacao-bridge-vs-xml
reviewed: 2026-05-16T21:00:00Z
depth: standard
files_reviewed: 6
files_reviewed_list:
  - backend/handlers/xml_conciliacao.go
  - backend/main.go
  - frontend/src/pages/ConciliacaoBridgeXML.tsx
  - frontend/src/lib/navigation.ts
  - frontend/src/App.tsx
  - frontend/src/index.css
findings:
  critical: 3
  warning: 4
  info: 2
  total: 9
status: issues_found
---

# Phase 04: Code Review Report — Conciliação Bridge vs XML

**Reviewed:** 2026-05-16T21:00:00Z
**Depth:** standard
**Files Reviewed:** 6
**Status:** issues_found

## Summary

This phase delivers the Conciliação Bridge vs XML feature: three new backend handlers (`ConciliacaoHandler`, `CoberturaHandler`, `ConciliacaoCSVHandler`) in `xml_conciliacao.go`, route registration in `main.go`, and a React page `ConciliacaoBridgeXML.tsx` with navigation wiring in `navigation.ts` and `App.tsx`.

The backend SQL parameterization is correct — table name is whitelisted (`nfe_entradas`/`nfe_saidas`) and all dynamic values use `$N` placeholders. The frontend correctly relies on the global fetch interceptor in `AuthContext.tsx` for `Authorization`/`X-Company-ID` injection.

Three blockers were found: a logic error in the SQL `delta_total` computation (IPI omitted), a migration runner that permanently marks failed migrations as executed, and an unquoted column name interpolated into DDL SQL from a DB-sourced value. Four warnings cover a hardcoded UI label inconsistency, a silent data truncation at 500 rows, a WHERE filter that misses IPI-only divergences, and `console.log` in production App init.

---

## Critical Issues

### CR-01: `delta_total` Omits IPI — Understates Divergence and Wrong Sort Order

**File:** `backend/handlers/xml_conciliacao.go:102-106`

**Issue:** The `delta_total` SQL expression sums PIS + COFINS + ICMS deltas but excludes IPI. The `delta_ipi` column is correctly computed (line 101), but then not included in `delta_total`. Since rows are sorted `ORDER BY delta_total DESC` (line 116), any NF-e with a large IPI divergence but small PIS/COFINS/ICMS deltas will be sorted below less-significant rows. Additionally, the summary card in the frontend sums each row's `delta_total` (line 181-183 of `ConciliacaoBridgeXML.tsx`) — so the displayed "Delta tributário total" also silently excludes IPI.

```sql
-- Current (wrong):
ROUND(
    ABS(COALESCE(ne.v_pis,0) - COALESCE(ne.pis,0)) +
    ABS(COALESCE(ne.v_cofins,0) - COALESCE(ne.cofins,0)) +
    ABS(COALESCE(ne.v_icms,0) - COALESCE(ne.icms,0)),
2) AS delta_total

-- Fixed:
ROUND(
    ABS(COALESCE(ne.v_pis,0)    - COALESCE(ne.pis,0))    +
    ABS(COALESCE(ne.v_cofins,0) - COALESCE(ne.cofins,0)) +
    ABS(COALESCE(ne.v_icms,0)   - COALESCE(ne.icms,0))   +
    ABS(COALESCE(ne.v_ipi,0)    - COALESCE(ne.ipi,0)),
2) AS delta_total
```

---

### CR-02: Migration Runner Records Failed Migrations as Executed — Broken Schema Silently Skipped Forever

**File:** `backend/main.go:214-219`

**Issue:** After executing each migration, line 215 unconditionally inserts the filename into `schema_migrations` regardless of whether the migration succeeded or failed with a non-idempotent error. Only "already exists"/"duplicate" errors record inside the `if err != nil` block (lines 209-213) — but the unconditional insert at line 215 runs in ALL cases, including genuine failures (syntax error, constraint violation, missing table, etc.). This means a migration that fails due to a real bug is permanently recorded and will never be re-attempted on restart.

```go
// Current (wrong): unconditional insert after the error branch
_, insertErr := database.Exec("INSERT INTO schema_migrations (filename) VALUES ($1) ON CONFLICT DO NOTHING", baseName)

// Fixed: only record on success
} else {
    fmt.Printf("Migration %s executed successfully.\n", file)
    _, insertErr := database.Exec("INSERT INTO schema_migrations (filename) VALUES ($1) ON CONFLICT DO NOTHING", baseName)
    if insertErr != nil {
        log.Printf("Warning: Could not record migration %s: %v", baseName, insertErr)
    }
}
// Remove the unconditional insert below
```

---

### CR-03: Unquoted Column Name Interpolated into DDL SQL — Second-Order SQL Injection Risk

**File:** `backend/main.go:150-155`

**Issue:** The migration schema repair code fetches a column name from `information_schema.columns` and interpolates it directly into `ALTER TABLE ... RENAME COLUMN %s TO filename` using `fmt.Sprintf`. The column name is not quoted with `"` (double-quote identifier escaping). While `information_schema.columns.column_name` is normally safe, if the column name contains a space, special character, or a reserved SQL keyword, the DDL silently changes meaning or produces a parse error. More importantly, if the database were ever compromised at the metadata level, this provides a DDL injection vector.

```go
// Current (wrong):
_, renameErr := database.Exec(fmt.Sprintf(`ALTER TABLE schema_migrations RENAME COLUMN %s TO filename`, oldCol))

// Fixed: quote the identifier
_, renameErr := database.Exec(fmt.Sprintf(`ALTER TABLE schema_migrations RENAME COLUMN "%s" TO filename`, oldCol))
```

---

## Warnings

### WR-01: WHERE Clause Excludes IPI-Only Divergences — Silent Coverage Gap

**File:** `backend/handlers/xml_conciliacao.go:112-114`

**Issue:** The divergence filter condition only checks PIS, COFINS, and ICMS deltas:
```sql
AND (ABS(COALESCE(ne.v_pis,0)    - COALESCE(ne.pis,0))    > 0.01
  OR ABS(COALESCE(ne.v_cofins,0) - COALESCE(ne.cofins,0)) > 0.01
  OR ABS(COALESCE(ne.v_icms,0)   - COALESCE(ne.icms,0))   > 0.01)
```
A NF-e with a significant IPI divergence (and matching PIS/COFINS/ICMS) will not appear in the results at all. This is inconsistent with the fact that the query fetches, computes, and returns `delta_ipi`. An auditor would assume the report covers all four taxes.

**Fix:** Add IPI to the WHERE filter:
```sql
AND (ABS(COALESCE(ne.v_pis,0)    - COALESCE(ne.pis,0))    > 0.01
  OR ABS(COALESCE(ne.v_cofins,0) - COALESCE(ne.cofins,0)) > 0.01
  OR ABS(COALESCE(ne.v_icms,0)   - COALESCE(ne.icms,0))   > 0.01
  OR ABS(COALESCE(ne.v_ipi,0)    - COALESCE(ne.ipi,0))    > 0.01)
```

---

### WR-02: Summary Card Label "Cobertura XML (entradas)" Is Hardcoded — Wrong When `tipo=saidas`

**File:** `frontend/src/pages/ConciliacaoBridgeXML.tsx:222`

**Issue:** The third summary card title is hardcoded as `"Cobertura XML (entradas)"` but the `cobertura` query uses the `tipo` state variable (line 116: `buildUrl('/api/xml/cobertura', { tipo })`). When the user switches to `tipo=saidas`, the coverage percentage displayed is for saídas but the card label still reads "entradas". This is a user-visible incorrect label.

**Fix:**
```tsx
// Replace hardcoded label:
Cobertura XML (entradas)

// With dynamic label:
Cobertura XML ({tipo === 'entradas' ? 'entradas' : 'saídas'})
```

---

### WR-03: Excel Export Missing `Delta IPI` Column — Inconsistent with CSV Export

**File:** `frontend/src/pages/ConciliacaoBridgeXML.tsx:131-155`

**Issue:** The `handleExportExcel` function exports `'IPI XML'` and `'IPI Bridge'` columns but omits `'Delta IPI'`. The CSV export from the backend (line 352-358 of `xml_conciliacao.go`) correctly includes `"Delta IPI"` between `"IPI Bridge"` and `"Delta Total"`. This means an auditor comparing the two exports will find inconsistent column sets.

**Fix:** Add `'Delta IPI'` after `'IPI Bridge'` in the Excel export mapping:
```tsx
'IPI XML':         r.xml_ipi    ?? 0,
'IPI Bridge':      r.bridge_ipi ?? 0,
'Delta IPI':       r.delta_ipi  ?? 0,   // add this line
'Delta Total':     r.delta_total ?? 0,
```

---

### WR-04: Hard Limit of 500 Rows Has No User Notification — Silent Data Truncation

**File:** `backend/handlers/xml_conciliacao.go:117` and `frontend/src/pages/ConciliacaoBridgeXML.tsx`

**Issue:** The query applies `LIMIT 500` (line 117) but neither the API response nor the UI communicate this truncation to the user. A user with >500 divergences sees a table sorted by `delta_total DESC` with no indication that the bottom rows of the full dataset are missing. For an audit tool this is misleading — an auditor may believe they are looking at all divergences.

**Fix:** Return a count or `truncated: true` flag from the API, or display a banner when `divergencias.length === 500`:
```tsx
{divergencias && divergencias.length === 500 && (
  <p className="text-xs text-amber-600 mt-1 flex items-center gap-1">
    <AlertTriangle className="w-3 h-3" />
    Exibindo os 500 maiores desvios. Use a exportação CSV para obter todos os registros.
  </p>
)}
```

---

## Info

### IN-01: `console.log` in Production App Component

**File:** `frontend/src/App.tsx:189`

**Issue:** `console.log('App Version: 1.0.0 — FB_APU04 Simulador da Reforma Tributária')` executes on every render of the `App` component. While not harmful, it leaks version information to any user with DevTools open and is noise in production console output.

**Fix:** Remove the line or move it to a build-time comment. If version logging is desired for debugging, guard it:
```tsx
if (import.meta.env.DEV) {
  console.log('App Version: 1.0.0 — FB_APU04 Simulador da Reforma Tributária')
}
```

---

### IN-02: Hardcoded Local Development Credentials in Production Binary

**File:** `backend/main.go:69`

**Issue:** The fallback database connection string `"postgres://postgres:postgres@localhost:5432/fiscal_apu04_db?sslmode=disable"` is compiled into the production binary. Although it only applies when `DATABASE_URL` is unset, the default username/password `postgres:postgres` is printed to stdout when used (line 70: `fmt.Println(...)`). This is a developer convenience that should not be in a production binary.

**Fix:** Remove the fallback string and fail with a clear error message if `DATABASE_URL` is not set:
```go
connStr := os.Getenv("DATABASE_URL")
if connStr == "" {
    log.Fatal("DATABASE_URL environment variable is required")
}
```

---

_Reviewed: 2026-05-16T21:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
