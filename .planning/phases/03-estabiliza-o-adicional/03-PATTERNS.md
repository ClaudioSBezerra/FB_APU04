# Phase 3: Estabilizacao Adicional — Pattern Map

**Mapped:** 2026-05-16
**Files analyzed:** 7 target files/areas
**Analogs found:** 7 / 7

---

## File Classification

| Target File / Area | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `backend/.env` | config | - | `installer/.env` + `coolify-env-template.txt` | exact |
| `erp-bridge-aws/config-apu04.yaml` | config | - | `erp-bridge-aws/config-apu02.yaml` | exact |
| `backend/handlers/*_test.go` (new) | test | request-response | `backend/handlers/admin_reset_helpers_test.go` | exact |
| `frontend/src/**/*.test.tsx` (new) | test | request-response | `frontend/src/components/ResetDatabaseDialog.test.tsx` | exact |
| `erp-bridge-aws/bridge.py` (modify) | service | event-driven | self — no retry/backoff block exists yet | partial |
| `backend/services/email.go` (verify) | service | request-response | self — already uses `os.Getenv` cleanly | exact |
| `backend/main.go` `withDB` wrapper | middleware | request-response | self — `withDB`/`withAuth` factory pattern | exact |

---

## Pattern Assignments

### Area 1: Secrets / Env Migration

**Problem:** `backend/.env` line 16 contains a real SMTP password (`Proxy#6939`) and line 23 contains a real Z.AI API key. `erp-bridge-aws/config-apu04.yaml` lines 19-88 contain real Oracle passwords (`fcosta2013`).

**Analog — correct placeholder style:** `installer/.env` (all placeholders)

```
# installer/.env lines 1-25 — CORRECT PATTERN (all placeholders, no real secrets)
DB_PASSWORD=fbtax_test_2026          # test value, not production
JWT_SECRET=a1b2c3d4e5f6...           # long random, still clearly a test value
SMTP_PASSWORD=teste123               # obviously a test placeholder
```

**Analog — production template:** `coolify-env-template.txt` lines 33-40

```
# coolify-env-template.txt lines 33-40 — CORRECT PATTERN
SMTP_PASSWORD=your_smtp_password_here
ZAI_API_KEY=your_zai_api_key_here
```

**What to change in `backend/.env`:**

```diff
- SMTP_PASSWORD=Proxy#6939
+ SMTP_PASSWORD=your_smtp_password_here

- ZAI_API_KEY=985fc97cd618417dabaee4500d8f15d3.HYv6843AfzUl6tS3
+ ZAI_API_KEY=your_zai_api_key_here
```

**What to change in `erp-bridge-aws/config-apu04.yaml`** (all 12 servers, lines 21/27/33/39/45/51/57/63/69/75/81/87):

```diff
-     senha:   "fcosta2013"
+     senha:   "CHANGE_ME"
```

**Note:** `backend/.env` is a local dev file. The real secrets must only exist in Coolify's environment variables UI. The `.env` file in the repo should contain only placeholders or safe test values. `config-apu04.yaml` is shipped inside the Docker image so real passwords must not be committed.

---

### Area 2: Go Test Bootstrap — `net/http/httptest` for Handler Factory Pattern

**Analog:** `backend/handlers/admin_reset_helpers_test.go` (lines 1-93)

**Existing test pattern** (package-level unit test, no HTTP, lines 1-52):

```go
package handlers

import (
    "os"
    "testing"
)

func TestPgStringArray(t *testing.T) {
    tests := []struct {
        name     string
        input    []string
        expected string
    }{
        {"empty", []string{}, "{}"},
        {"single", []string{"a"}, `{"a"}`},
    }
    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            got := pgStringArray(tc.input)
            if got != tc.expected {
                t.Errorf("pgStringArray(%v) = %q, want %q", tc.input, got, tc.expected)
            }
        })
    }
}
```

**New pattern to add — `httptest` handler factory test:**
The existing tests use pure unit tests. For handler integration tests, the project has no `httptest` usage yet. The handler factory signature is:

```go
// All handlers follow this factory signature (e.g. backend/handlers/filiais.go line 20)
func GetFiliaisHandler(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) { ... }
}
```

New test files should use this pattern:

```go
package handlers

import (
    "database/sql"
    "net/http"
    "net/http/httptest"
    "testing"
)

// TestHealthHandler_NoDBRequired — tests a handler that does NOT need a DB
func TestHealthEndpointStructure(t *testing.T) {
    // For handlers that need *sql.DB, pass nil and check for 503
    // (mirrors the withDB wrapper in main.go lines 348-357)
    req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
    w := httptest.NewRecorder()

    // Call handler directly with nil db → tests nil-safety
    handler := GetFiliaisHandler(nil)
    handler(w, req)

    // A nil db should return 500 or be caught gracefully
    resp := w.Result()
    if resp.StatusCode == 200 {
        t.Error("expected non-200 when db is nil")
    }
}
```

**Key constraint:** No sqlmock is vendored (`backend/go.mod` has no test-double libraries). Tests must:
1. Use `package handlers` (same package, so unexported symbols are accessible)
2. Either test pure logic (no DB needed) or use a real test DB via `DATABASE_URL` env var
3. Mirror the `t.Run` table-driven style from `admin_reset_helpers_test.go`

**Run command (from `backend/`):**

```bash
go test ./handlers/... -v -run TestXxx
```

---

### Area 3: React / Vitest Test Bootstrap

**Existing vitest config:** `frontend/vitest.config.ts` (lines 1-17)

```typescript
// frontend/vitest.config.ts — FULL FILE (17 lines)
import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react-swc';
import path from 'path';

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    globals: true,
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
});
```

**Setup file:** `frontend/src/test/setup.ts` (line 1 only):

```typescript
import '@testing-library/jest-dom';
```

**Existing component test pattern:** `frontend/src/components/ResetDatabaseDialog.test.tsx` (lines 1-64)

```typescript
// Import pattern (lines 1-3)
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ResetDatabaseDialog } from './ResetDatabaseDialog';

// Test structure pattern (lines 5-63)
describe('ComponentName', () => {
  it('not rendered when closed', () => {
    render(<Component open={false} onOpenChange={() => {}} />);
    expect(screen.queryByRole('alertdialog')).toBeNull();
  });

  it('renders correctly when open', () => {
    render(<Component open={true} onOpenChange={() => {}} />);
    expect(screen.getByText(/ExpectedText/i)).toBeInTheDocument();
  });

  it('callback fires correctly', () => {
    const handler = vi.fn();
    render(<Component open={true} onOpenChange={() => {}} onConfirm={handler} />);
    fireEvent.click(screen.getByRole('button', { name: /ButtonLabel/i }));
    expect(handler).toHaveBeenCalledTimes(1);
    expect(handler).toHaveBeenCalledWith({ key: 'value' });
  });
});
```

**Existing utility test pattern:** `frontend/src/lib/utils.test.ts` (lines 1-13)

```typescript
import { expect, test } from 'vitest'
import { cn } from './utils'

test('cn merges class names correctly', () => {
  expect(cn('c-1', 'c-2')).toBe('c-1 c-2')
})
```

**Available testing libraries** (`frontend/package.json` devDependencies):
- `vitest@^1.6.1`
- `@testing-library/react@^14.3.1`
- `@testing-library/jest-dom@^6.9.1`
- `@testing-library/user-event@^14.6.1`
- `jsdom@^24.1.3`

**Run command (from `frontend/`):**

```bash
npm test
# or: npx vitest run
```

New test files go in the same directory as the component: `frontend/src/components/MyComponent.test.tsx`.

---

### Area 4: Python Bridge — Oracle DPY-4011 Retry / Reconnect

**Current state:** `erp-bridge-aws/bridge.py` has NO retry logic on Oracle connection failures. The two connection sites are:

**Site 1 — `processar_servidor` (line 812-822):**

```python
try:
    conn_ora = oracledb.connect(
        user=srv["usuario"],
        password=srv["senha"],
        dsn=srv["dsn"],
        expire_time=2,
    )
    log.info("Conectado ao Oracle (thin mode)")
except Exception as exc:
    log.error("Falha ao conectar em %s: %s", nome, exc)
    return stats   # <-- single attempt, returns immediately on failure
```

**Site 2 — `processar_sap` (line 627-639):**

```python
try:
    conn_ora = oracledb.connect(
        user=oracle_cfg["usuario"],
        password=oracle_cfg["senha"],
        dsn=oracle_cfg["dsn"],
        expire_time=2,
    )
    log.info("Conectado ao Oracle SAP FCCORP (thin mode)")
except Exception as exc:
    log.error("Falha ao conectar ao FCCORP: %s", exc)
    stats["sap_batch"]["erros"] = 1
    stats["sap_batch"]["erro_msg"] = str(exc)
    stats["sap_batch"]["erro_conexao"] = True
    return stats   # <-- single attempt, returns immediately on failure
```

**DPY-4011 context:** This error means "connection closed" / "not connected". It fires mid-query when a firewall drops a long-idle TCP connection. The `expire_time=2` keepalive helps but is not sufficient in all environments.

**Retry pattern to add** (modeled on the DB retry in `backend/main.go` lines 74-108):

```python
# Helper to add above processar_servidor and processar_sap
import time as _time  # already imported

MAX_CONN_RETRIES = 3
CONN_RETRY_DELAY = 5  # seconds

def _connect_oracle(usuario: str, senha: str, dsn: str, nome: str) -> "oracledb.Connection":
    """Connect to Oracle with retry on DPY-4011 / transient errors.

    Raises the last exception if all retries are exhausted.
    """
    last_exc = None
    for attempt in range(1, MAX_CONN_RETRIES + 1):
        try:
            conn = oracledb.connect(
                user=usuario,
                password=senha,
                dsn=dsn,
                expire_time=2,
            )
            if attempt > 1:
                log.info("[%s] Conectado ao Oracle na tentativa %d", nome, attempt)
            return conn
        except Exception as exc:
            last_exc = exc
            is_dpy4011 = "DPY-4011" in str(exc)
            if attempt < MAX_CONN_RETRIES:
                delay = CONN_RETRY_DELAY * attempt  # linear backoff: 5s, 10s
                log.warning(
                    "[%s] Falha ao conectar (tentativa %d/%d%s): %s — tentando em %ds",
                    nome, attempt, MAX_CONN_RETRIES,
                    " DPY-4011" if is_dpy4011 else "",
                    exc, delay,
                )
                _time.sleep(delay)
            else:
                log.error("[%s] Falha ao conectar após %d tentativas: %s", nome, MAX_CONN_RETRIES, exc)
    raise last_exc
```

**State tracker pattern** (existing, `bridge.py` lines 87-116):

```python
# The tracker SQLite DB (TRACKER_DB) already records per-document state.
# DPY-4011 mid-query reconnect should use the same tracker to avoid re-sending.
# Pattern: catch oracledb errors inside the processing loop, close conn, reconnect.

# Mid-query reconnect pattern (to add inside for-loop of processar_servidor):
except oracledb.Error as ora_exc:
    err_str = str(ora_exc)
    if "DPY-4011" in err_str and _reconnect_attempt < 1:
        log.warning("  DPY-4011 detectado — reconectando ao Oracle...")
        try:
            conn_ora.close()
        except Exception:
            pass
        conn_ora = _connect_oracle(srv["usuario"], srv["senha"], srv["dsn"], nome)
        _reconnect_attempt += 1
        continue  # retry this document
    raise  # re-raise if not DPY-4011 or already retried
```

**Existing heartbeat pattern** for daemon reconnect check (bridge.py lines 481-494):

```python
# FBTaxClient.heartbeat() — already called once per daemon loop (line 1063).
# If heartbeat fails → network issue → do not attempt Oracle connection that cycle.
fbtax.heartbeat()
```

---

### Area 5: Go Env Var Consumption — No Config Struct Exists

**Current state:** All handlers read env vars via inline `os.Getenv` at call time. There is NO central config struct. This is consistent and intentional.

**Pattern for reading env vars** (from `backend/services/email.go` lines 52-80):

```go
// GetEmailConfig — lazy-read pattern: reads env at call time, not at init.
// This ensures godotenv.Load() in main() has already run.
func GetEmailConfig() *EmailConfig {
    portStr := os.Getenv("SMTP_PORT")
    port := 465  // safe default
    if portStr != "" {
        if p, err := strconv.Atoi(portStr); err == nil {
            port = p
        }
    }

    host := os.Getenv("SMTP_HOST")
    if host == "" {
        host = "smtp.hostinger.com"   // <-- fallback only for host, never for secrets
    }

    username := os.Getenv("SMTP_USER")
    password := os.Getenv("SMTP_PASSWORD")   // <-- empty string if not set; guarded downstream

    return &EmailConfig{
        Host: host, Port: port, Username: username, Password: password,
    }
}
```

**Guard pattern for missing secret** (email.go lines 134-137):

```go
// Before using secret: check empty, log + return meaningful error
if config.Password == "" {
    log.Printf("[Email Service] SMTP not configured. Skipping email send to %s", email)
    return fmt.Errorf("servico de e-mail nao configurado - configure SMTP_PASSWORD")
}
```

**JWT secret pattern** (`backend/handlers/auth.go` lines 64-79):

```go
// getJWTSecret — lazy read, fallback only for local dev
func getJWTSecret() []byte {
    secret := os.Getenv("JWT_SECRET")
    if secret == "" {
        return []byte("super-secret-key-change-me-in-prod")
    }
    return []byte(secret)
}

// ValidateJWTSecret — called in main() after godotenv.Load(), fatals in prod
func ValidateJWTSecret() {
    if os.Getenv("JWT_SECRET") == "" {
        if os.Getenv("DATABASE_URL") != "" {   // DATABASE_URL set = prod
            log.Fatal("FATAL: JWT_SECRET not set")
        }
        log.Println("WARNING: JWT_SECRET not set — insecure default (dev only).")
    }
}
```

**Rule:** New env vars should follow this same pattern:
1. Read via `os.Getenv` at call time (lazy, not at `init()`)
2. Provide safe non-secret defaults only (e.g. hostnames, ports)
3. For secrets: no default, guard with empty check, return descriptive error

---

### Area 6: SMTP Password in `backend/main.go` — Confirmed NOT Hardcoded

`backend/main.go` does NOT initialize SMTP. SMTP is initialized lazily in `backend/services/email.go:GetEmailConfig()` which reads `os.Getenv("SMTP_PASSWORD")` with no hardcoded fallback (lines 66-67). The only hardcoded credential risk is in `backend/.env` itself (see Area 1).

---

## Shared Patterns

### Handler Factory Pattern (applies to all new Go handler tests)

**Source:** `backend/main.go` lines 348-369

```go
// withDB wraps a factory: gets current db at request time, returns 503 if nil
withDB := func(handlerFactory func(*sql.DB) http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        database := getDB()
        if database == nil {
            jsonServiceUnavailable(w)
            return
        }
        handlerFactory(database)(w, r)
    }
}
```

In tests, call the factory directly: `handler := handlers.GetFiliaisHandler(db)` then use `httptest`.

### Error Response Pattern (applies to all new Go handlers)

**Source:** `backend/handlers/filiais.go` lines 48-55

```go
if err != nil {
    http.Error(w, "Error querying filiais: "+err.Error(), http.StatusInternalServerError)
    return
}
```

### Vitest `vi.fn()` Mock Pattern (applies to all new React component tests)

**Source:** `frontend/src/components/ResetDatabaseDialog.test.tsx` lines 45-54

```typescript
const onConfirm = vi.fn();
render(<Component open={true} onOpenChange={() => {}} onConfirm={onConfirm} />);
fireEvent.click(screen.getByRole('button', { name: /ButtonLabel/i }));
expect(onConfirm).toHaveBeenCalledTimes(1);
expect(onConfirm).toHaveBeenCalledWith({ key: 'value' });
```

---

## No Analog Found

None — all areas have existing analogs in the codebase.

---

## Critical Findings

1. **`backend/.env` line 16:** Real SMTP password `Proxy#6939` committed to repo.
   **Action:** Replace with placeholder `your_smtp_password_here`.

2. **`backend/.env` line 23:** Real Z.AI API key committed to repo.
   **Action:** Replace with placeholder `your_zai_api_key_here`.

3. **`erp-bridge-aws/config-apu04.yaml` lines 21-87:** Real Oracle password `fcosta2013` for 12 servers committed to repo.
   **Action:** Replace all `senha: "fcosta2013"` with `senha: "CHANGE_ME"`.

4. **`erp-bridge-aws/bridge.py`:** No retry on Oracle DPY-4011. Single-attempt connect only.
   **Action:** Add `_connect_oracle()` helper with 3-attempt linear backoff.

5. **Go tests:** Only one test file exists (`admin_reset_helpers_test.go`) and it contains no `httptest` usage. New tests must add `net/http/httptest` pattern.

6. **No sqlmock vendored:** `backend/go.mod` has no test-double library. Handler tests that need DB must either test pure logic or use a real DB via `DATABASE_URL`.

---

## Metadata

**Analog search scope:** `backend/`, `frontend/src/`, `erp-bridge-aws/`, `installer/`
**Files scanned:** 12 source files read fully
**Pattern extraction date:** 2026-05-16
