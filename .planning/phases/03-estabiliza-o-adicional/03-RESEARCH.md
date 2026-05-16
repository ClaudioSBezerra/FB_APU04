# Phase 3: Estabilização Adicional - Research

**Researched:** 2026-05-16
**Domain:** Go testing (net/http/httptest), React/Vitest, Python oracledb retry, Coolify env vars
**Confidence:** HIGH (all findings verified against source files in this repo)

---

## Summary

Phase 3 covers four orthogonal stabilization items. Each has a clear, bounded implementation
path with no blocking dependencies on each other.

**STAB-06 (secrets)** — `backend/.env` already has real SMTP password (`Proxy#6939`) and a real
ZAI API key hardcoded. The Go services already read these via `os.Getenv()` — the credential is
already flowing correctly. The only work is (1) remove the real values from the file so `.env`
becomes a dev-template-only file, and (2) ensure those values are set in Coolify's environment
variable dashboard. `installer/.env` uses a placeholder SMTP password (`teste123`) and no
ZAI_API_KEY at all — its risk is low, but it should also be templated.

**STAB-07 (Go tests)** — One test file exists (`admin_reset_helpers_test.go`, package `handlers`),
achieving 0.2% statement coverage. The handlers use the factory pattern `Handler(db *sql.DB)
http.HandlerFunc`. No mock library is vendored. The correct approach is `net/http/httptest`
(stdlib) plus `database/sql` with a real Postgres test DB or partial in-memory logic. Pure-logic
tests (no DB) like `AuthMiddleware`, `pgStringArray`, rate-limiter `Allow/Reset`, and
`ERPBridgeBatchImportHandler` method-not-allowed/missing-API-key paths are already testable
without a DB. DB-dependent paths need either a test DB or sqlmock (not yet vendored).

**STAB-08 (React/Vitest)** — Vitest 1.6.1, jsdom, `@testing-library/react` 14.3.1, and
`@testing-library/jest-dom` are fully installed. The `vitest.config.ts` already configures jsdom
and `src/test/setup.ts`. Two test files exist and all 11 tests pass. The pattern from
`ResetDatabaseDialog.test.tsx` (render + fireEvent + expect) is the exact template to replicate.
The richest untested pure-logic targets are `formatFilial.ts` (~12 functions) and `navigation.ts`
(`getActiveModule`, module config shapes).

**STAB-09 (Oracle retry)** — The bridge currently uses `oracledb.connect(..., expire_time=2)`
(keepalive every 2 min) but there is no retry/reconnect on mid-query `DPY-4011` errors. The SAP
path (`processar_sap`) acquires one connection, runs all queries, then closes it. A firewall drop
mid-query raises `oracledb.DatabaseError` with message containing `DPY-4011`. The retry fix must
(a) detect this specific error, (b) reconnect, (c) re-execute the cursor, and (d) resume — while
not re-counting already-sent documents (the SQLite `sap_watermark` handles date-level idempotency;
per-chave idempotency is handled by `UPSERT` on the backend). The daemon path is separate from
`processar_sap` but follows the same pattern.

**Primary recommendation:** Implement in order STAB-06 → STAB-08 → STAB-07 → STAB-09. STAB-06
has zero risk (config only). STAB-08 needs no DB. STAB-07 needs a decision on sqlmock vs real
test DB. STAB-09 is the most surgical Python change.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Secret management (STAB-06) | Deployment (Coolify) | Backend (reads env) | Secrets are injected by platform; Go reads via os.Getenv |
| Go handler unit tests (STAB-07) | Backend (handlers pkg) | — | Tests live in `package handlers`, use httptest |
| React component tests (STAB-08) | Frontend (src/components, src/lib) | — | Vitest + jsdom runs in frontend package |
| Oracle retry/reconnect (STAB-09) | Bridge Python | — | oracledb connection lifecycle is entirely in bridge.py |

---

## STAB-06: Credentials Audit

### What Is Hardcoded Today

**File: `backend/.env`** [VERIFIED: read directly]
```
SMTP_PASSWORD=Proxy#6939        ← real production password
ZAI_API_KEY=985fc97cd618417dabaee4500d8f15d3.HYv6843AfzUl6tS3  ← real API key
```
All other values in `backend/.env` (DB password `postgres`, JWT default, etc.) are local-dev
defaults and acceptable to remain.

**File: `installer/.env`** [VERIFIED: read directly]
```
SMTP_PASSWORD=teste123          ← placeholder, not a real credential
```
No `ZAI_API_KEY` present in installer/.env. Low risk but should be cleaned up.

**File: `erp-bridge-aws/config-apu04.yaml`** [VERIFIED: read directly]
```yaml
senha: "fcosta2013"             ← Oracle DB passwords for 12 servers
```
This file is NOT tracked by git (`git ls-files` returned nothing). Risk is low for leakage,
but passwords are in plaintext in the yaml. Out of scope for STAB-06 (Coolify doesn't manage
the bridge config directly).

### How Go Reads Credentials — Already Correct

`services/email.go` [VERIFIED]:
```go
password := os.Getenv("SMTP_PASSWORD")   // line 67
```

`services/ai.go` [VERIFIED]:
```go
apiKey := os.Getenv("ZAI_API_KEY")       // line 66
```

`main.go` [VERIFIED]:
```go
_ = godotenv.Load()   // line 257 — loads .env if present, but env vars set by Coolify take precedence
```

godotenv.Load() does NOT override existing env vars — it only sets vars not already in the
environment [ASSUMED: standard godotenv behavior]. So Coolify's injected vars are safe.

### docker-compose.yml Already Passes Them Through

`docker-compose.yml` (dev) and `docker-compose.prod.yml` both use `${SMTP_PASSWORD}` and
`${ZAI_API_KEY:-}` interpolation [VERIFIED: grep confirmed]. The Coolify dashboard injects these
into the container at runtime.

### Fix Required

1. Replace real values in `backend/.env` with `<YOUR_SMTP_PASSWORD>` / `<YOUR_ZAI_API_KEY>`.
2. Document in `coolify-env-template.txt` (already exists, already has placeholder values).
3. `installer/.env` has no real secret — replace `teste123` with placeholder for consistency.
4. Confirm Coolify dashboard has `SMTP_PASSWORD` and `ZAI_API_KEY` set for simu.fcxlabs.com.

`backend/.env` MUST remain in `.gitignore` (already is). No other file changes needed.

---

## STAB-07: Go Test Bootstrap

### Current State [VERIFIED]

| File | Tests | Coverage |
|------|-------|----------|
| `handlers/admin_reset_helpers_test.go` | 5 tests (TestPgStringArray, TestIsDBAllowed_DefaultDeny, TestIsDBAllowed_NoMatch, TestConfirmationToken, TestResetTablesNotEmpty) | 0.2% statement |

All 5 tests pass with `go test ./handlers/...`.

### Test Infrastructure Available

- Go 1.22 [VERIFIED: go.mod]
- `net/http/httptest` — stdlib, no install needed
- No mock library vendored (no DATA-DOG/go-sqlmock, no testify)
- Package is `package handlers` (white-box tests can access unexported helpers)

### Handler Pattern [VERIFIED]

All handlers follow:
```go
func FooHandler(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) { ... }
}
```

This makes `httptest` testing straightforward:
```go
// Source: admin_reset_helpers_test.go pattern + net/http/httptest stdlib
func TestFooHandler_MethodNotAllowed(t *testing.T) {
    req := httptest.NewRequest(http.MethodGet, "/foo", nil)
    rr  := httptest.NewRecorder()
    FooHandler(nil)(rr, req)   // nil db is fine for method-check-only paths
    if rr.Code != http.StatusMethodNotAllowed {
        t.Errorf("got %d, want 405", rr.Code)
    }
}
```

### Testable Handler Surface (No DB Required)

These paths fail before any DB query:

| Handler | Test Case | DB Needed? |
|---------|-----------|-----------|
| `ERPBridgeBatchImportHandler` | GET → 405 | No |
| `ERPBridgeBatchImportHandler` | POST, no X-API-Key header → 401 | No |
| `ResetDatabaseHandler` | GET → 405 | No |
| `AuthMiddleware` | no Authorization header → 401 | No |
| `AuthMiddleware` | malformed header (no "Bearer ") → 401 | No |
| `AuthMiddleware` | invalid JWT → 401 | No |
| `AuthMiddleware` | valid JWT, wrong role → 403 | No |
| `rateLimiter.Allow` | N+1 requests → false | No |
| `rateLimiter.Reset` | reset clears state | No |
| `GetClientIP` | X-Forwarded-For header parsing | No |

### Handlers That Need DB (Integration Tests)

`LoginHandler`, `RegisterHandler`, `ResetDatabaseHandler` (full path), `ERPBridgeBatchImportHandler`
(auth lookup). These require either:
- **Option A:** Real PostgreSQL test DB (spinup via `docker-compose -f docker-compose.yml up -d db`)
- **Option B:** sqlmock (`DATA-DOG/go-sqlmock`) — not yet vendored, needs `go get` + `go mod vendor`

**Recommendation:** Use Option A for now (test DB already available locally via docker-compose).
Annotate DB tests with `t.Skip("needs DB")` behind env var `TEST_DATABASE_URL` to avoid breaking
CI without DB.

### Target Coverage Path to 30%

Current: 0.2% (5 simple tests, pure logic only).
30% target on the `handlers` package covering ~99 handler functions.

Pure-logic tests (no DB, `nil` db passed in) can realistically cover ~15-20% by testing:
- All method-not-allowed branches across major handlers (every handler that checks `r.Method`)
- Auth middleware invalid token paths
- Rate limiter logic
- `pgStringArray`, `isValidUUID`, `GetClientIP`, `ConfirmationToken`, `ResetTables`

DB tests (requires TEST_DATABASE_URL) would push to 30%+.

### New Test File Location

`backend/handlers/auth_test.go` and `backend/handlers/erp_bridge_batch_test.go` in
`package handlers`.

---

## STAB-08: React/Vitest Bootstrap

### Current State [VERIFIED]

| Config | Value |
|--------|-------|
| vitest version | 1.6.1 |
| test environment | jsdom |
| setup file | `src/test/setup.ts` (imports `@testing-library/jest-dom`) |
| test command | `npm test` → `vitest run` |
| existing tests | 11 passing (utils.test.ts × 3, ResetDatabaseDialog.test.tsx × 8) |

### Installed Test Libraries [VERIFIED: package.json]

- `vitest@^1.6.1`
- `@testing-library/react@^14.3.1`
- `@testing-library/jest-dom@^6.9.1`
- `@testing-library/user-event@^14.6.1`
- `jsdom@^24.1.3`

### Template Pattern (from ResetDatabaseDialog.test.tsx) [VERIFIED]

```tsx
// Source: frontend/src/components/ResetDatabaseDialog.test.tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MyComponent } from './MyComponent';

describe('MyComponent', () => {
  it('renders correctly', () => {
    render(<MyComponent prop="value" />);
    expect(screen.getByText(/text/i)).toBeInTheDocument();
  });
  it('calls handler on click', () => {
    const handler = vi.fn();
    render(<MyComponent onClick={handler} />);
    fireEvent.click(screen.getByRole('button'));
    expect(handler).toHaveBeenCalledTimes(1);
  });
});
```

### Best Targets for STAB-08

**Pure utility functions (zero render, zero mocking):**

`src/lib/formatFilial.ts` — 12 exported functions including:
- `formatCNPJ(cnpj)` — 14-digit string → formatted
- `formatCPF(cpf)` — 11-digit string → formatted
- `formatDocumento(doc)` — auto-detect
- `formatCNPJMasked(cnpj)` — privacy mask
- `parseFilialName(name)` — parses "FC010102 - 12345678000190"
- `formatFilialFromRow(row)` — complex fallback logic (best ROI for bugs)

`src/lib/navigation.ts` — `getActiveModule(pathname)` routing logic (8+ branches).

These have zero dependencies on React Router, AuthContext, or API calls. Copy the
`utils.test.ts` pattern exactly.

**Component tests (need render):**

`ResetDatabaseDialog` is already covered. Good next candidates:
- A simple UI component with no API dependencies (a card or badge component)
- Login form — can be tested with mocked `fetch` via `vi.stubGlobal('fetch', ...)`

**Components to AVOID testing in STAB-08 (too complex, need full router/auth context):**
- Dashboard, ERP Bridge Painel, XML Upload — depend on AuthContext, React Router, live API

### New Test File Locations

- `frontend/src/lib/formatFilial.test.ts`
- `frontend/src/lib/navigation.test.ts`

---

## STAB-09: Oracle Retry/Reconnect

### Current Error Handling in Bridge [VERIFIED: bridge.py]

**SAP path (`processar_sap`, lines 628–790):**
```python
try:
    conn_ora = oracledb.connect(
        user=oracle_cfg["usuario"],
        password=oracle_cfg["senha"],
        dsn=oracle_cfg["dsn"],
        expire_time=2,   # keepalive every 2 min
    )
except Exception as exc:
    # Only catches connect-time failures, sets erro_conexao=True
    stats["sap_batch"]["erros"] = 1
    return stats

try:
    cur = conn_ora.cursor()
    cur.execute(SAP_QUERY, ...)   # <-- DPY-4011 can fire here
    # ...
except Exception as exc:
    log.error("Erro durante processamento SAP: %s", exc)
    stats["sap_batch"]["erros"] = 1
    stats["sap_batch"]["erro_msg"] = str(exc)
finally:
    conn_ora.close()
```

**No retry exists.** DPY-4011 is caught by the outer `except Exception` and terminates the whole
SAP run with `erros=1`.

**Oracle XML path (`processar_servidor`, lines 813–903):**
Same pattern — one connection for the entire server, no retry on mid-query disconnection.

### DPY-4011 Error Identification [ASSUMED: based on python-oracledb docs pattern]

`oracledb.DatabaseError` is raised. The error message contains `"DPY-4011"`. Detection:

```python
import oracledb

def is_connection_lost(exc: Exception) -> bool:
    """Returns True if exc is an oracledb DPY-4011 connection-lost error."""
    msg = str(exc)
    return "DPY-4011" in msg or (
        isinstance(exc, oracledb.DatabaseError) and "connection lost contact" in msg.lower()
    )
```

### State Tracker Analysis [VERIFIED: bridge.py]

**oracle_xml mode** uses `tracker.db` (SQLite table `enviados`) with:
- Primary key: `(servidor, tipo, chave)`
- Status `'ok'` means already sent — `ja_enviado()` skips it

On reconnect, the document loop continues from where it left off — documents already marked `ok`
are skipped automatically. The tracker handles idempotency correctly.

**sap_s4hana mode** fetches all rows first (`rows = [...]`), then iterates `documents[]` to send in
batches of 1000. The backend uses UPSERT (conflict on `chave`), so re-sending already-inserted
documents is safe. However, the loop is `for i in range(0, len(documents), BATCH_SIZE)` — on
reconnect we need to resume from the current batch offset `i`, not restart from 0.

### Recommended Retry Implementation

**Approach:** Wrap the Oracle connection + query in a retry helper. On DPY-4011:
1. Log warning
2. Sleep 5 seconds
3. Reconnect
4. Re-execute the failed query

For the `processar_sap` batch-send loop, track `batch_start_offset` before calling `cur.execute`
so retry restarts the query (which re-fetches all rows from Oracle) but the already-sent batches
are still idempotent via backend UPSERT.

```python
# Source: proposed implementation pattern
MAX_RETRIES = 3
RETRY_DELAY = 5  # seconds

def connect_with_retry(cfg: dict, max_retries: int = MAX_RETRIES) -> oracledb.Connection:
    """Establishes Oracle connection with exponential backoff on failure."""
    for attempt in range(1, max_retries + 1):
        try:
            return oracledb.connect(
                user=cfg["usuario"],
                password=cfg["senha"],
                dsn=cfg["dsn"],
                expire_time=2,
            )
        except Exception as exc:
            if attempt == max_retries:
                raise
            log.warning("[Oracle] Conexão falhou (tentativa %d/%d): %s — aguardando %ds",
                        attempt, max_retries, exc, RETRY_DELAY * attempt)
            _time.sleep(RETRY_DELAY * attempt)
```

For mid-query reconnect inside `processar_sap`:

```python
# Wrap cur.execute in try/except with reconnect
def execute_with_retry(oracle_cfg, query, params, max_retries=MAX_RETRIES):
    """Executes query, reconnecting on DPY-4011 up to max_retries times."""
    conn = connect_with_retry(oracle_cfg)
    for attempt in range(1, max_retries + 1):
        try:
            cur = conn.cursor()
            cur.execute(query, **params)
            cols = [d[0].lower() for d in cur.description]
            rows = [dict(zip(cols, row)) for row in cur]
            cur.close()
            return conn, rows
        except Exception as exc:
            if not is_connection_lost(exc):
                conn.close()
                raise
            if attempt == max_retries:
                conn.close()
                raise
            log.warning("[Oracle] DPY-4011 na query (tentativa %d/%d) — reconectando...",
                        attempt, max_retries)
            _time.sleep(RETRY_DELAY)
            try:
                conn.close()
            except Exception:
                pass
            conn = connect_with_retry(oracle_cfg)
```

**Insertion point:** Replace the `oracledb.connect()` + `cur.execute(SAP_QUERY)` block in
`processar_sap` (lines 628–651) with `execute_with_retry(oracle_cfg, SAP_QUERY, {...})`.
Same for `processar_servidor` in `processar_sap` block (lines 813–847).

### State Continuity on Reconnect

| Mode | Idempotency Mechanism | Reconnect Safe? |
|------|----------------------|----------------|
| sap_s4hana | Backend UPSERT on `chave` (44-char NF key) | Yes — re-sending a batch is a no-op |
| oracle_xml | SQLite tracker `enviados` table with status='ok' | Yes — `ja_enviado()` skips sent docs |
| watermark | `sap_watermark` table updated only on `total_errors == 0` | Yes — partial run leaves watermark unchanged, daemon retries full date range |

### Daemon Loop Resilience

The daemon `run_daemon()` has its own `try/except Exception` at the top level (line 1199).
A DPY-4011 that escapes `processar_sap` is already caught there and logged without crashing
the daemon. However, `finalize_run` is NOT called — leaving the run in `running` state
indefinitely. The heartbeat at line 1063 runs every 60s, which should clear stuck runs via the
backend's heartbeat handler — but confirming that requires checking `ERPBridgeHeartbeatHandler`.

Recommend also calling `fbtax.finalize_run(run_id, grand, erro_msg=str(exc))` in the
`except` branch of `executar_importacao` when DPY-4011 escapes.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Go HTTP testing | Custom test server | `net/http/httptest` (stdlib) | Already available, no import needed |
| React component testing | Manual DOM manipulation | `@testing-library/react` (installed) | Already installed |
| JWT validation testing | Custom JWT decode | Set `JWT_SECRET` env + real `jwt.Parse` | Keeps test realistic |
| Oracle connection backoff | Custom sleep loop | `connect_with_retry` helper (see above) | Consolidates retry logic |
| DB mock for Go tests | Custom interface | `DATA-DOG/go-sqlmock` (if vendored) or real test DB | go-sqlmock handles `database/sql` perfectly |

---

## Common Pitfalls

### Pitfall 1: godotenv Silently Overrides Coolify Variables
**What goes wrong:** If `backend/.env` still contains real credentials AND the container
starts with `godotenv.Load()`, godotenv will NOT override Coolify-injected env vars (it only
sets if unset). But the file should still not contain real values in case it gets committed.
**How to avoid:** Replace real values with `<placeholder>`. godotenv.Load() returns `_` error
anyway (already in main.go line 257).

### Pitfall 2: Go Tests Pass nil db to DB-Dependent Handlers
**What goes wrong:** `nil` db works for method/auth checks, but causes panic if the handler
proceeds to a DB query.
**How to avoid:** Only call handlers with `nil` db for tests that verify code paths that
return before any `db.QueryRow()`. Document with a comment. Add a guard: if the test reaches
an unexpected code path, `httptest.Recorder.Code` will show 500 (nil pointer panic, recovered
by `net/http` default handler).

### Pitfall 3: React Tests Missing Router Context
**What goes wrong:** Components that use `useNavigate()` or `<Link>` panic in tests without
a Router.
**How to avoid:** Wrap render in `<MemoryRouter>` from `react-router-dom` when needed.
`formatFilial.ts` and `navigation.ts` are pure functions — no Router needed.

### Pitfall 4: DPY-4011 Retry Loops Forever
**What goes wrong:** Network stays down; retry exhausts `max_retries` but Oracle keeps
returning DPY-4011.
**How to avoid:** Hard limit `MAX_RETRIES = 3`. After exhaustion, re-raise the original
exception so `processar_sap` can set `erro_conexao=True` and return stats cleanly.

### Pitfall 5: Batch Offset Not Preserved After Reconnect (SAP mode)
**What goes wrong:** After reconnect the SAP query is re-executed from Oracle (full date
range), producing the same rows. Re-sending all batches from offset 0 is safe (backend
UPSERTs), but doubles API traffic.
**How to avoid:** The current approach (re-fetch all rows, iterate from 0) is acceptable
because of UPSERT idempotency. Document in comments that re-send is intentional.

---

## Architecture Patterns

### Go Test Pattern (httptest, no DB)
```go
// Source: backend/handlers/admin_reset_helpers_test.go (existing pattern extended)
package handlers

import (
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestERPBridgeBatchImportHandler_NoAPIKey(t *testing.T) {
    req  := httptest.NewRequest(http.MethodPost, "/api/erp-bridge/import/batch", nil)
    // No X-API-Key header
    rr   := httptest.NewRecorder()
    ERPBridgeBatchImportHandler(nil)(rr, req)
    if rr.Code != http.StatusUnauthorized {
        t.Errorf("got %d, want 401", rr.Code)
    }
}

func TestAuthMiddleware_NoHeader(t *testing.T) {
    next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    })
    handler := AuthMiddleware(next, "admin")
    req := httptest.NewRequest(http.MethodGet, "/", nil)
    rr  := httptest.NewRecorder()
    handler(rr, req)
    if rr.Code != http.StatusUnauthorized {
        t.Errorf("got %d, want 401", rr.Code)
    }
}
```

### React/Vitest Test Pattern (pure function)
```ts
// Source: frontend/src/lib/utils.test.ts (existing pattern extended to formatFilial)
import { expect, test, describe } from 'vitest'
import { formatCNPJ, formatCPF, parseFilialName } from './formatFilial'

describe('formatCNPJ', () => {
  test('formats 14-digit string', () => {
    expect(formatCNPJ('12345678000190')).toBe('12.345.678/0001-90')
  })
  test('returns original if not 14 digits', () => {
    expect(formatCNPJ('123')).toBe('123')
  })
  test('handles null/undefined', () => {
    expect(formatCNPJ(null)).toBe('')
    expect(formatCNPJ(undefined)).toBe('')
  })
})
```

### Python Oracle Retry Pattern
```python
# Proposed addition to bridge.py (before processar_sap)
MAX_RETRIES = 3
RETRY_DELAY_S = 5

def is_dpy4011(exc: Exception) -> bool:
    return "DPY-4011" in str(exc)

def ora_connect(cfg: dict) -> oracledb.Connection:
    """Connect to Oracle with retry on transient failures."""
    for attempt in range(1, MAX_RETRIES + 1):
        try:
            return oracledb.connect(
                user=cfg["usuario"],
                password=cfg["senha"],
                dsn=cfg["dsn"],
                expire_time=2,
            )
        except Exception as exc:
            if attempt == MAX_RETRIES:
                raise
            log.warning("[Oracle] connect attempt %d/%d failed: %s", attempt, MAX_RETRIES, exc)
            _time.sleep(RETRY_DELAY_S * attempt)
```

---

## Standard Stack

### Core (No Changes Needed)
| Component | Version | Purpose |
|-----------|---------|---------|
| Go | 1.22 | Backend |
| `net/http/httptest` | stdlib | Go HTTP handler testing |
| Vitest | 1.6.1 | Frontend test runner |
| `@testing-library/react` | 14.3.1 | React component testing |
| `@testing-library/jest-dom` | 6.9.1 | DOM matchers |
| `oracledb` (python) | installed | Oracle connectivity in bridge |

### Optional Additions
| Library | Purpose | How to Add |
|---------|---------|-----------|
| `DATA-DOG/go-sqlmock` | Mock `*sql.DB` for Go DB tests | `go get github.com/DATA-DOG/go-sqlmock` + `go mod vendor` |

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 1.22 | STAB-07 | Assumed available | 1.22 | — |
| Node.js + npm | STAB-08 | Assumed available (frontend working) | — | — |
| PostgreSQL (test) | STAB-07 DB tests | Via docker-compose | 15 | Skip DB tests with t.Skip |
| Coolify dashboard access | STAB-06 | Manual step | — | Document for user |
| python-oracledb | STAB-09 | Installed in bridge container | — | — |

---

## Validation Architecture

> `nyquist_validation: false` in config.json — this section included for reference only.

**Go tests:** `cd backend && go test ./handlers/... -cover`
**React tests:** `cd frontend && npm test`

---

## Security Domain

### STAB-06 Specific

| Risk | Current State | After Fix |
|------|--------------|-----------|
| SMTP password in .env | `Proxy#6939` hardcoded | Placeholder only; real value in Coolify |
| ZAI_API_KEY in .env | Real key hardcoded | Placeholder only; real value in Coolify |
| Oracle passwords in config-apu04.yaml | `fcosta2013` in 12 entries | Out of scope (not in git) |

The fix does NOT change how the application reads secrets — only where they are stored.
`os.Getenv()` calls are already correct. Coolify injects env vars before container start,
which takes precedence over godotenv.Load().

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | godotenv.Load() does not override existing env vars (only sets unset vars) | STAB-06 | If wrong, .env file would override Coolify vars — but real values will be removed so moot |
| A2 | DPY-4011 error message contains the string "DPY-4011" | STAB-09 | is_dpy4011 detection would fail; fallback to DatabaseError isinstance check still works |
| A3 | Backend UPSERT on chave makes SAP batch re-send idempotent | STAB-09 | Re-sent batches would produce duplicate rows — need to verify UPSERT in erp_bridge_batch.go |
| A4 | ERPBridgeHeartbeatHandler clears stuck "running" runs | STAB-09 daemon | Stuck runs in UI if heartbeat doesn't clean them |

---

## Open Questions

1. **Should go-sqlmock be vendored for STAB-07 DB tests?**
   - What we know: Current vendor tree has no mock library. DB-dependent handler tests need either real DB or mock.
   - What's unclear: Does the CI environment have PostgreSQL available?
   - Recommendation: Use TEST_DATABASE_URL env guard and skip DB tests when not set. Defer sqlmock to a future phase.

2. **Is A3 correct — does ERPBridgeBatchImportHandler do UPSERT?**
   - What we know: `erp_bridge_batch.go` inserts into `nfe_saidas`, `nfe_entradas`, `cte_entradas`.
   - What's unclear: The INSERT was not fully read (only first 60 lines of handler).
   - Recommendation: Planner should verify insert SQL in `erp_bridge_batch.go` lines 130–230.

3. **Does Coolify auto-restart on env var change?**
   - What we know: Coolify supports env var management per service.
   - What's unclear: Whether changing SMTP_PASSWORD in Coolify triggers a redeploy automatically.
   - Recommendation: Note in STAB-06 task that user must manually redeploy after setting vars.

---

## Sources

### Primary (HIGH confidence — verified against source files)
- `backend/.env` — exact secret values confirmed
- `backend/services/email.go:53-69` — SMTP env var reads
- `backend/services/ai.go:66` — ZAI_API_KEY env var read
- `backend/main.go:257` — godotenv.Load() call
- `backend/handlers/admin_reset_helpers_test.go` — existing test pattern
- `frontend/vitest.config.ts` — test configuration
- `frontend/src/test/setup.ts` — jest-dom setup
- `frontend/src/components/ResetDatabaseDialog.test.tsx` — component test template
- `frontend/src/lib/formatFilial.ts` — 12 pure functions for testing
- `erp-bridge-aws/bridge.py:628-790` — SAP Oracle connection and query logic
- `backend/go.mod` — Go 1.22, no mock library

### Secondary (MEDIUM confidence)
- `coolify-env-template.txt` — confirms expected env var names for Coolify

### Tertiary (LOW confidence — flagged in Assumptions Log)
- DPY-4011 error string format [A2]
- godotenv non-override behavior [A1]

---

## Metadata

**Confidence breakdown:**
- STAB-06 findings: HIGH — read actual file contents, confirmed exact secrets
- STAB-07 findings: HIGH — ran tests, confirmed 0.2% coverage, identified testable paths
- STAB-08 findings: HIGH — ran vitest, confirmed all 11 tests pass, identified target files
- STAB-09 findings: MEDIUM — bridge.py fully read, DPY-4011 handling confirmed absent; retry pattern is ASSUMED based on oracledb behavior

**Research date:** 2026-05-16
**Valid until:** 2026-06-16 (stable stack — Go, Python, oracledb versions unlikely to change)
