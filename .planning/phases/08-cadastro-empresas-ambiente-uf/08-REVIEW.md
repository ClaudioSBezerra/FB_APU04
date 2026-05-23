---
phase: 08-cadastro-empresas-ambiente-uf
reviewed: 2026-05-23T12:00:00Z
depth: standard
files_reviewed: 10
files_reviewed_list:
  - backend/handlers/environment.go
  - backend/handlers/icms_fronteira_regras.go
  - backend/handlers/icms_fronteira_regras_update_test.go
  - backend/main.go
  - backend/migrations/096_add_fields_to_companies.sql
  - backend/migrations/097_add_uf_estado_to_fronteira_regras.sql
  - backend/migrations/098_seed_ba_ce_fronteira.sql
  - frontend/src/pages/GestaoAmbiente.tsx
  - frontend/src/lib/navigation.ts
  - frontend/src/pages/IcmsFronteira.tsx
findings:
  critical: 4
  warning: 8
  info: 3
  total: 15
status: issues_found
---

# Phase 08: Code Review Report

**Reviewed:** 2026-05-23T12:00:00Z
**Depth:** standard
**Files Reviewed:** 10
**Status:** issues_found

## Summary

This phase adds company master-data fields (CNPJ, CNAE, inscricao estadual, etc.), a UF selector for ICMS Fronteira rules (PE/BA/CE), and seeds initial BA/CE rules. The implementation is structurally correct but carries four blockers: two security-class issues (panic risk from unchecked JWT type assertions, and users able to overwrite global NCM rules), one missing idempotency guard in the seed migration, and a logic ordering bug in the React useEffect that briefly shows stale data. Eight warnings cover missing `rows.Err()` checks, missing `Content-Type` headers, CNPJ field size mismatch, and several quality issues.

---

## Critical Issues

### CR-01: Panic on missing JWT claim keys in `GetEnvironmentsHandler`

**File:** `backend/handlers/environment.go:55-56`
**Issue:** After a successful `.(jwt.MapClaims)` assertion, the code performs bare type-assertion casts on two map keys:
```go
userID := claims["user_id"].(string)
role := claims["role"].(string)
```
If either key is absent or not a string (malformed token, older token format, test token), the process **panics** with a nil pointer dereference. The rest of the codebase (e.g., `icms_fronteira_regras.go:60`) correctly uses the comma-ok form `userID, _ := claims["user_id"].(string)`. This inconsistency is a crash vector.

**Fix:**
```go
userID, _ := claims["user_id"].(string)
role, _ := claims["role"].(string)
if userID == "" {
    http.Error(w, "Unauthorized", http.StatusUnauthorized)
    return
}
```

---

### CR-02: Users can overwrite global NCM rules via `IcmsFronteiraRegraUpdateHandler`

**File:** `backend/handlers/icms_fronteira_regras.go:364-388`
**Issue:** The UPDATE query's WHERE clause is:
```sql
WHERE id = $9::uuid AND (company_id = $10::uuid OR company_id IS NULL)
```
The `company_id IS NULL` arm means any authenticated user can locate a global rule by its UUID and overwrite its `descricao`, `regime`, `aliquota_interna`, and all MVA fields. Global rules are supposed to be shared read-only records; the DELETE handler correctly blocks removal (company-scoped only), but the UPDATE handler does not. This allows a single user to corrupt tax parameters for every company on the platform.

**Fix:** Remove the `OR company_id IS NULL` arm from the UPDATE statement so updates are restricted to company-owned rows only:
```sql
WHERE id = $9::uuid AND company_id = $10::uuid
```
If editing global rules is a legitimate admin operation, add a separate admin-only endpoint with explicit authorization.

---

### CR-03: `ON CONFLICT DO NOTHING` in migration 098 is silently wrong — global seed rows will never insert

**File:** `backend/migrations/098_seed_ba_ce_fronteira.sql:17-27, 33-41`
**Issue:** The INSERT statement omits `company_id` from both the column list and values, which means the column will be `NULL` (the global-rule convention). The unique constraint added by migration 097 is `UNIQUE NULLS NOT DISTINCT (company_id, ncm_prefixo, uf_estado)`. `ON CONFLICT DO NOTHING` silently suppresses errors — including the case where a duplicate NCM prefix already exists for the same UF with `company_id IS NULL`.

More critically, `UNIQUE NULLS NOT DISTINCT` is a **PostgreSQL 15+** feature. If the database is running PostgreSQL 14 or earlier (which is common in containerised environments), the constraint syntax in migration 097 will fail, and the ON CONFLICT clause in 098 may behave unexpectedly (treating NULL as non-equal, allowing duplicates). There is no version guard or comment documenting this requirement.

**Fix:**
1. Add a PostgreSQL version check comment at the top of migration 097: `-- Requires PostgreSQL >= 15 for UNIQUE NULLS NOT DISTINCT`.
2. Verify the ON CONFLICT target matches the actual constraint name for explicitness: `ON CONFLICT ON CONSTRAINT uq_icms_fronteira_regras_uf DO NOTHING`.

---

### CR-04: Race condition in `GestaoAmbiente` — stale groups shown after environment switch

**File:** `frontend/src/pages/GestaoAmbiente.tsx:131-134`
**Issue:** When `selectedEnv` changes, the effect calls `fetchGroups(selectedEnv.id)` **before** calling `setGroups([])`. If the fetch for the new environment completes before React re-renders with the cleared list, the old groups from the previous environment will briefly flash, then be replaced. But if the user switches environments rapidly, the fetch for environment A may resolve *after* the fetch for environment B, resulting in environment B's UI permanently displaying environment A's groups (a classic stale closure / out-of-order response bug).

```javascript
// current order — wrong
fetchGroups(selectedEnv.id);   // fires async request
setGroups([]);                  // clears state after firing
```

**Fix:** Clear state first, then fetch — and use an AbortController or a stale-result guard:
```typescript
useEffect(() => {
  setGroups([]);
  setSelectedGroup(null);
  setCompanies([]);
  if (selectedEnv) {
    let cancelled = false;
    fetchGroups(selectedEnv.id).then((data) => {
      if (!cancelled) setGroups(data);
    });
    return () => { cancelled = true; };
  }
}, [selectedEnv]);
```

---

## Warnings

### WR-01: Missing `rows.Err()` check after scan loops in environment handlers

**File:** `backend/handlers/environment.go:84-95, 184-198, 272-294`
**Issue:** All three list handlers (`GetEnvironmentsHandler`, `GetGroupsHandler`, `GetCompaniesHandler`) iterate `rows.Next()` and scan but never call `rows.Err()` after the loop. A network interruption or server-side cursor error mid-scan will silently return a partial result with HTTP 200, indistinguishable from a full result. The same pattern is absent in `icms_fronteira_regras.go:105-138`.

**Fix:** After each `for rows.Next()` block:
```go
if err := rows.Err(); err != nil {
    http.Error(w, err.Error(), http.StatusInternalServerError)
    return
}
```

---

### WR-02: Missing `Content-Type: application/json` header in all `environment.go` handlers

**File:** `backend/handlers/environment.go:96, 118, 142, 159, 197, 219, 236, 297, 389`
**Issue:** Every handler in `environment.go` writes JSON via `json.NewEncoder(w).Encode(...)` but never sets `w.Header().Set("Content-Type", "application/json")`. Browsers and API clients may interpret the response as `text/plain` or `text/html`, breaking JSON parsing in edge cases. The `icms_fronteira_regras.go` handlers correctly set this header; the environment handlers do not.

**Fix:** Add at the top of each handler function (before any write):
```go
w.Header().Set("Content-Type", "application/json")
```

---

### WR-03: CNPJ database column size mismatch — VARCHAR(18) in DB but validation enforces 14 digits only

**File:** `backend/migrations/096_add_fields_to_companies.sql:11` and `backend/handlers/environment.go:328-334`
**Issue:** The migration defines `cnpj VARCHAR(18)` with a comment noting it can hold the formatted 18-character form (`XX.XXX.XXX/XXXX-XX`). The handler validates `^\d{14}$` — 14 raw digits — and the comment in the migration says "14 dígitos numéricos ou formatado (18 chars)". The validation only allows the unformatted form, yet the database column is sized for the formatted form. If any other code path or migration inserts a formatted CNPJ, the regex check in `UpdateCompanyHandler` (line 431) will reject it with 400. The UI also only accepts "apenas números" (only numbers). This ambiguity about which format the system stores will cause confusion and potential data inconsistency.

**Fix:** Document and enforce a single canonical format. If storing only digits, change the column to `VARCHAR(14)` and remove the "ou formatado" note in the migration comment. If storing formatted CNPJs, update the regex to accept both forms.

---

### WR-04: `UpdateCompanyHandler` silently succeeds when `id` does not exist

**File:** `backend/handlers/environment.go:439-468`
**Issue:** After `db.Exec(UPDATE ... WHERE id = $9)`, the handler does not check `RowsAffected()`. If `id` refers to a non-existent company, the UPDATE executes zero rows, returns no error from the driver, and the handler responds with HTTP 200. The caller (frontend) has no way to know the update had no effect. The `IcmsFronteiraRegraUpdateHandler` correctly checks `RowsAffected()` (line 384) — the company handler should do the same.

**Fix:**
```go
n, err := res.RowsAffected()
if err == nil && n == 0 {
    http.Error(w, "Company not found", http.StatusNotFound)
    return
}
```

---

### WR-05: `DeleteEnvironmentHandler`, `DeleteGroupHandler`, and `DeleteCompanyHandler` have no authorization check

**File:** `backend/handlers/environment.go:146-161, 223-238, 471-486`
**Issue:** These handlers accept any authenticated user (the `withAuth` wrapper in main.go uses `role: ""`). Any non-admin user who knows an `id` UUID can delete any environment, group, or company on the platform regardless of whether they belong to it. The environment list handler correctly scopes to the user's assigned environments, but the delete handler has no such scoping. A user could enumerate UUIDs from their own environment response and then delete environments belonging to other users.

**Fix:** For delete operations, verify the caller has access to the resource before deleting. At minimum, for non-admin users, verify the environment/group/company belongs to a user-accessible hierarchy before executing the DELETE.

---

### WR-06: CSV/XLSX import in `IcmsFronteiraRegrasImportarHandler` skips errors without a size limit on the error list

**File:** `backend/handlers/icms_fronteira_regras.go:583-588`
**Issue:** Each failed row appends to `res.Errors []string`. If a malicious or malformed file has thousands of rows all failing, the error slice grows unbounded in memory before being serialised and returned to the client. A 5 MB file with 50,000 rows of invalid data could produce an enormous JSON response. Additionally, each row that errors increments `res.Skipped` but the function correctly imports successfully-parsed rows — there is no partial-rollback on failure, so a file that partially imports is left in an inconsistent state.

**Fix:** Cap the error list at a reasonable size (e.g., 100 errors), then report "and N more errors":
```go
if len(res.Errors) < 100 {
    res.Errors = append(res.Errors, "Linha "+strconv.Itoa(i+1)+": "+err2.Error())
}
```

---

### WR-07: NCM prefix silently truncated to 8 characters without user feedback

**File:** `backend/handlers/icms_fronteira_regras.go:193-195, 521-523`
**Issue:** Both the create handler and the import handler truncate NCM prefixes longer than 8 characters silently:
```go
if len(body.NCMPrefixo) > 8 {
    body.NCMPrefixo = body.NCMPrefixo[:8]
}
```
This is a byte-level slice on a string, which may corrupt multi-byte UTF-8 characters if the user enters non-ASCII content in the NCM field. Additionally, the silent truncation means a user who types "12345678901" expects an error but gets a quietly-modified insert, which can create wrong records without any diagnostic.

**Fix:** Return a 400 error when NCM prefix exceeds 8 characters, or add a note in the API response. For the byte slicing: `body.NCMPrefixo[:8]` is safe only for pure ASCII. Use `[]rune(body.NCMPrefixo)[:8]` or validate that the field is digits-only before slicing.

---

### WR-08: Divergencias export buttons in `IcmsFronteira.tsx` bypass authentication

**File:** `frontend/src/pages/IcmsFronteira.tsx:1661-1673`
**Issue:** The CSV and XLSX divergencias export buttons set `a.href` directly to the API URL and trigger a click:
```typescript
a.href = `/api/icms-fronteira/divergencias/exportar/csv...`
document.body.appendChild(a); a.click(); document.body.removeChild(a)
```
This navigation does not include the `Authorization: Bearer` header. If the backend export handlers require JWT authentication (which they do — they use `withAuth`), this will fail with 401 in production. The same pattern exists for `PlanilhaTab` itens exports (lines 1900, 1912). The `ExportButtons` component correctly uses `fetch` with the auth header (line 291), but the Divergencias and Planilha tabs use direct `<a>` navigation instead.

**Fix:** Use the same `fetch`-and-blob pattern as `ExportButtons.downloadFile()`, or temporarily generate a signed download URL. Alternatively, move auth to cookies (but that is a larger change).

---

## Info

### IN-01: `IcmsFronteira.tsx` page header hardcodes "PE" — does not reflect selected UF

**File:** `frontend/src/pages/IcmsFronteira.tsx:2254`
**Issue:** The page title reads `ICMS Fronteira — PE` (hardcoded), but this phase adds BA and CE support. The RegrasTab already has a UF selector. The header tooltip (line 2268) also hardcodes PE-specific aliquota explanations. As the UF filter state lives inside `RegrasTab` and is not lifted to the parent, this is a minor UX inconsistency.

**Fix:** Either lift `selectedUF` to the `IcmsFronteira` parent component and display it in the header, or remove the hardcoded "— PE" suffix.

---

### IN-02: `navigation.ts` fronteira module tabs do not include "Planilha" or "Apuração" routes

**File:** `frontend/src/lib/navigation.ts:62-70`
**Issue:** The `fronteira` module config lists 7 tabs but the actual `IcmsFronteira.tsx` page has 10 tabs including `planilha`, `divergencias`, and `apuracao` (Apuração Mensal). These three paths are not in the navigation tabs list, meaning navigation breadcrumb/tab bar driven by this config will not show them. They are only reachable by direct URL or internal tab click.

**Fix:** Add the missing tabs to the fronteira module in `navigation.ts`:
```typescript
{ label: 'Planilha de Itens',    path: '/icms-fronteira/planilha' },
{ label: 'Divergências',         path: '/icms-fronteira/divergencias' },
{ label: 'Apuração Mensal',      path: '/icms-fronteira/apuracao' },
```

---

### IN-03: Test file provides trivial coverage — only tests handler construction and method rejection

**File:** `backend/handlers/icms_fronteira_regras_update_test.go`
**Issue:** The two tests verify (a) the handler factory returns non-nil and (b) a GET is rejected with 405. No test exercises the actual update logic, the SQL execution path, the regex validation for `uf_estado`, or the `RowsAffected` check. Given CR-02 (global rule overwrite) is not caught by any test, the test suite provides false confidence.

**Fix:** Add table-driven tests covering: valid PATCH with mocked DB, attempt to update a global row (should return 404), invalid regime value (should return 400), and missing ID (should return 400). Use `database/sql/driver` with `DATA-DOG/go-sqlmock` or similar.

---

_Reviewed: 2026-05-23T12:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
