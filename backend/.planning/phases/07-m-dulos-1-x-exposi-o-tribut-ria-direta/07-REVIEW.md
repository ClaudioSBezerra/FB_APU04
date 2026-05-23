---
phase: 07-modulos-1x-exposicao-tributaria-direta
reviewed: 2026-05-23T00:00:00Z
depth: standard
files_reviewed: 9
files_reviewed_list:
  - backend/handlers/reforma_modulo1.go
  - backend/handlers/reforma_modulo1_test.go
  - backend/main.go
  - frontend/src/App.tsx
  - frontend/src/lib/navigation.ts
  - frontend/src/pages/Reforma11CreditosBloqueados.tsx
  - frontend/src/pages/Reforma12Reprecificacao.tsx
  - frontend/src/pages/Reforma13RankingFornecedores.tsx
  - frontend/src/pages/Reforma14SplitPayment.tsx
findings:
  critical: 4
  warning: 5
  info: 4
  total: 13
status: issues_found
---

# Phase 07: Code Review Report

**Reviewed:** 2026-05-23T00:00:00Z
**Depth:** standard
**Files Reviewed:** 9
**Status:** issues_found

## Summary

This phase delivers four new backend handlers (Créditos Bloqueados, Reprecificação, Ranking Fornecedores, Split Payment) plus their CSV variants, route registration in `main.go`, and four React pages. The logic is broadly sound but the review found four critical issues: a runtime-panic-inducing unchecked type assertion on `user_id` (present in all four JSON handlers), silent row scan errors that silently corrupt aggregate totals, missing `rows.Err()` checks after iteration, and a complete absence of the `X-Company-ID` header in all four frontend pages — causing every multi-company user to always query the wrong (default) company.

---

## Critical Issues

### CR-01: Unchecked type assertion on `claims["user_id"]` panics at runtime

**File:** `backend/handlers/reforma_modulo1.go:115`, `318`, `515`, `781`

**Issue:** All four JSON handlers (CreditosBloqueadosHandler, RankingFornecedoresHandler, ReprecificacaoHandler, SplitPaymentHandler) perform `userID := claims["user_id"].(string)` as a direct, unchecked type assertion. If the JWT is well-formed but the `user_id` claim is absent, is `nil`, or is not a `string` (e.g., a numeric `sub`), this assertion panics and crashes the goroutine with no recovery, returning a 500 with an empty body to the client. The CSV variants at lines 210 and 406 use the safe two-value form `userID, _ := claims["user_id"].(string)` — the JSON handlers should do the same.

**Fix:**
```go
// Replace the direct assertion in all four JSON handlers:
userID, ok2 := claims["user_id"].(string)
if !ok2 || userID == "" {
    jsonErr(w, http.StatusUnauthorized, "Unauthorized")
    return
}
```

---

### CR-02: Silent `rows.Scan` errors silently corrupt aggregate totals

**File:** `backend/handlers/reforma_modulo1.go:169-178` (CreditosBloqueados), `369-376` (RankingFornecedores)

**Issue:** When `rows.Scan` fails, the loop `continue`s without logging. For CreditosBloqueados, `totalIcms`, `totalIBS`, and `totalCBS` are accumulated only from successfully scanned rows. If any rows fail to scan — e.g., due to a NULL `v_nf` reaching a non-nullable target — the JSON totals are silently under-reported while the partial row list is served. The user sees incorrect financial totals with no indication of data loss.

**Fix:**
```go
if err := rows.Scan(...); err != nil {
    log.Printf("[CreditosBloqueados] scan error: %v", err)
    continue
}
```
Additionally, consider returning a 500 if scan errors exceed a threshold, or at minimum surfacing a `"data_partial": true` flag in the response.

---

### CR-03: Missing `rows.Err()` check after iteration in all six query loops

**File:** `backend/handlers/reforma_modulo1.go:179`, `265`, `377`, `461`, `610`, `726`

**Issue:** None of the six `for rows.Next()` loops call `rows.Err()` after the loop exits. The Go `database/sql` contract requires checking `rows.Err()` to detect mid-iteration network errors or context cancellations. A truncated result set (e.g., a connection reset after 80 of 100 rows) will be served as if it were complete, producing silently incorrect financial totals.

**Fix:** After every `rows.Close()` (or immediately after the loop, before `rows.Close()` is deferred), add:
```go
if err := rows.Err(); err != nil {
    log.Printf("[CreditosBloqueados] rows iteration error: %v", err)
    jsonErr(w, http.StatusInternalServerError, "Erro ao ler dados")
    return
}
```

---

### CR-04: All four frontend pages omit `X-Company-ID` header — multi-company users always query wrong company

**File:** `frontend/src/pages/Reforma11CreditosBloqueados.tsx:70`, `frontend/src/pages/Reforma12Reprecificacao.tsx:93`, `frontend/src/pages/Reforma13RankingFornecedores.tsx:69`, `frontend/src/pages/Reforma14SplitPayment.tsx:46`

**Issue:** Every other page in the codebase that calls a company-scoped endpoint passes `X-Company-ID` in request headers (confirmed in `Mercadorias.tsx`, `MercadoriasXML.tsx`, `ConsultaNFesEntradas.tsx`, `Managers.tsx`, etc.). All four new Reforma pages send bare `fetch('/api/...`)` calls with no headers. On the backend, `GetEffectiveCompanyID` falls back to the user's primary company when the header is absent — so users operating in a secondary company context will silently receive data for their primary company. The CSV export handlers have the same omission. This is a data integrity bug for any multi-company installation.

**Fix (example for Reforma11CreditosBloqueados.tsx):**
```tsx
// Add useFilial (or equivalent context) to get companyId
const { companyId } = useFilial()

// Pass header in queryFn
const res = await fetch('/api/reforma/modulo1/creditos', {
  headers: {
    'X-Company-ID': companyId || '',
  },
})

// Also pass in handleExportCSV
const res = await fetch('/api/reforma/modulo1/creditos/csv', {
  headers: { 'X-Company-ID': companyId || '' },
})
```
Apply the same pattern to Reforma12, Reforma13, and Reforma14.

---

## Warnings

### WR-01: CSV handlers set `Content-Type` and `Content-Disposition` headers after potentially writing rows — header already sent

**File:** `backend/handlers/reforma_modulo1.go:267-268`, `463-464`, `729-730`

**Issue:** The CSV handlers query the database and iterate rows _before_ setting `Content-Type` and `Content-Disposition` headers. In the current code this is safe only because no bytes are written to `w` before the headers. However, if any future change writes to `w` during the row loop (e.g., a partial flush on error), headers would already be sent with the default `Content-Type: text/plain`. The idiomatic Go pattern is to set headers before any write to `w`.

**Fix:** Move `w.Header().Set(...)` calls to immediately after the auth/param checks, before the DB query, so they are always written first.

---

### WR-02: `cstFilter` value `"00"` in Reforma12 filter dropdown does not match `cst_path` values — filter never matches

**File:** `frontend/src/pages/Reforma12Reprecificacao.tsx:172`, `119-130`

**Issue:** The `<SelectItem value="00">Normal (00)</SelectItem>` item sets `cstFilter` to the string `"00"`. The client-side filter on line 128 checks `row.cst_path === cstFilter`, and `cst_path` is never `"00"` — it is `"normal"`, `"st"`, `"base_reduzida"`, or `"outro"` (set by the backend switch statement at `reforma_modulo1.go:581-588`). Selecting "Normal (00)" from the dropdown will always yield zero rows.

**Fix:**
```tsx
// Change SelectItem value to match cst_path value:
<SelectItem value="normal">Normal (00)</SelectItem>
```

---

### WR-03: `fmtCNPJ` defined but never called in `Reforma11CreditosBloqueados.tsx`

**File:** `frontend/src/pages/Reforma11CreditosBloqueados.tsx:56-59`

**Issue:** `fmtCNPJ` is defined at line 56 and is not used anywhere in the file (Módulo 1.1 shows CFOP data, not CNPJs). TypeScript does not flag unused functions as errors, but the dead code indicates copy-paste from Reforma13 and adds confusion. The identical function is correctly used in `Reforma13RankingFornecedores.tsx`.

**Fix:** Remove the unused `fmtCNPJ` function from `Reforma11CreditosBloqueados.tsx`.

---

### WR-04: SQL column-name injection risk via unchecked `oldCol` in `schema_migrations` repair path

**File:** `backend/main.go:153`

**Issue:** In the migration self-repair block, `oldCol` is read from `information_schema.columns` and then interpolated directly into a `fmt.Sprintf` DDL statement:
```go
_, renameErr := database.Exec(fmt.Sprintf(`ALTER TABLE schema_migrations RENAME COLUMN %s TO filename`, oldCol))
```
`oldCol` originates from the database itself (not user input), so exploitation requires the database to already be compromised — but it is still a bad pattern. If a future code change reads `oldCol` from an external source, or if the DB schema is manipulated, this becomes a SQL injection vector.

**Fix:**
```go
// Whitelist acceptable column names before interpolating:
if oldCol != "id" && oldCol != "migration" && oldCol != "name" {
    log.Printf("Unexpected column name %q in schema_migrations, skipping rename", oldCol)
} else {
    // safe to interpolate — column names cannot be parameterized in DDL
    _, renameErr := database.Exec(fmt.Sprintf(`ALTER TABLE schema_migrations RENAME COLUMN %s TO filename`, oldCol))
    ...
}
```

---

### WR-05: Sensitivity matrix cell highlight uses float equality comparison — fragile

**File:** `frontend/src/pages/Reforma14SplitPayment.tsx:165-167`

**Issue:** The "highlight current scenario" logic compares `cdi === data?.taxa_cdi_anual_pct` where both are `number` (float64 from JSON). The backend hardcodes CDI columns as `[]float64{8, 10, 12, 14}` and `taxa_cdi_anual_pct` defaults to `10.5`. Since `10.5` is never in the CDI columns array, no cell will ever be highlighted when using the default. If a user sets `taxa_cdi_anual_pct = 10`, the comparison `10 === 10` works because both originate from JSON integer serialization, but this is fragile — any float arithmetic in the backend (e.g., rounding) could break the equality. An epsilon comparison or index-based lookup should be used.

**Fix:**
```tsx
const isCurrentCell =
  senRow.dso === data?.prazo_medio_dias &&
  Math.abs((cdi ?? 0) - (data?.taxa_cdi_anual_pct ?? 0)) < 0.001
```

---

## Info

### IN-01: Test suite covers only handler construction and one method-not-allowed case

**File:** `backend/handlers/reforma_modulo1_test.go:1-70`

**Issue:** The seven creation tests simply verify that `HandlerFunc(nil) != nil` — this tests Go closure mechanics, not handler behavior. There is one method-not-allowed test. There are no tests for auth checking, parameter fallback logic, or the calculation formulas (IBS/CBS projection, variacao_pct, float tributario). Given these handlers implement financial calculations that inform business decisions, the lack of unit tests for the computation logic is a quality gap.

---

### IN-02: `console.log` in production App component

**File:** `frontend/src/App.tsx:207`

**Issue:** `console.log('App Version: 1.0.0 — ...')` runs on every render of the App component. This leaks version/feature information to the browser console and adds noise to developer tools.

**Fix:** Remove the `console.log` call. Version information is already served by `/api/health`.

---

### IN-03: `getActiveModule` path-prefix ordering: `/reforma` catches `/reforma/parametros` before config module

**File:** `frontend/src/lib/navigation.ts:89`

**Issue:** `navigation.ts` line 89 has `if (pathname.startsWith('/reforma')) return 'reforma'`. This is correct for the new Reforma pages. However, `/config/reforma-parametros` (line 63 in navigation.ts tabs, line 180 in App.tsx routes) maps to the `config` module via the `/config/` prefix check on line 90 — that works fine. No routing bug exists, but it is worth noting the `reforma` check at line 89 lacks a trailing slash (`/reforma` vs `/reforma/`), meaning a hypothetical future path `/reformado` would incorrectly match the `reforma` module. This is a latent fragility.

**Fix:**
```ts
if (pathname.startsWith('/reforma/') || pathname === '/reforma') return 'reforma'
```

---

### IN-04: Magic numbers for IBS/CBS default aliquots hardcoded in four separate handler locations

**File:** `backend/handlers/reforma_modulo1.go:131`, `225`, `333`, `422`, `529`, `655`, `797`

**Issue:** The default aliquot fallback values `aliqIBS = 26.5`, `aliqCBS = 9.9`, and `fatorSimples = 20.0` are repeated verbatim in six locations across four handlers. If the regulatory default changes, all six sites must be updated consistently.

**Fix:** Define package-level constants:
```go
const (
    defaultAliqIBSPct    = 26.5
    defaultAliqCBSPct    = 9.9
    defaultFatorSimples  = 20.0
    defaultTaxaCDI       = 10.5
    defaultPrazoMedioDias = 30
)
```

---

_Reviewed: 2026-05-23T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
