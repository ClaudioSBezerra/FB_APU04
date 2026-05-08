# Testing Patterns

**Analysis Date:** 2026-05-08

## Testing Posture (Current Reality)

**Test coverage in this project is minimal.** Only two test files exist:

| File | Type | Framework |
|------|------|-----------|
| `tests/integration_test.go` | Integration smoke test | Go `testing` |
| `frontend/src/lib/utils.test.ts` | Unit test (pure helper) | Vitest |

There is **no CI step that runs tests** — `.github/workflows/deploy-production.yml` and `deploy-staging.yml` build/push Docker images only; no `go test` or `npm test` invocation.

**Implication for new code:** there is no automated regression net. New features should add at least:
- A Go test for any new pure logic (parsers, calculators)
- A Vitest test for any new `frontend/src/lib/` helper
- An integration probe in `tests/` if the new endpoint is critical

## Test Framework

### Backend (Go)

**Runner:**
- Stdlib `testing` package — no external test framework
- No `go.mod` test dependencies declared (Vitest of `testify`/`gomock` not vendored)
- Vendored deps in `backend/vendor/` — any new test framework must be `go mod vendor`'d

**Assertion Library:**
- None. Pure `t.Errorf` / `t.Logf` from stdlib.

**Run Commands:**
```bash
# From repo root (go.mod is at backend/, but tests/ is at root)
cd tests && go test -v ./...

# Or, when adding *_test.go files inside backend/:
cd backend && go test -v ./...

# With coverage:
cd backend && go test -cover ./...
```

### Frontend (TypeScript/React)

**Runner:**
- Vitest — imported in `frontend/src/lib/utils.test.ts:1` as `import { expect, test } from 'vitest'`
- **No `vitest` declared in `frontend/package.json`** — currently runs only if installed ad-hoc. This is a gap; either:
  - Add `vitest` and `@vitest/ui` as devDependencies, or
  - Convert the existing test to a different framework

**Run Commands:**
```bash
# Once vitest is in devDependencies:
cd frontend && npx vitest run        # one-shot
cd frontend && npx vitest            # watch mode
cd frontend && npx vitest --coverage # with coverage (needs @vitest/coverage-v8)
```

## Test File Organization

**Backend:**
- Go convention: `*_test.go` next to the source file in the same package
- This project places integration tests in a separate top-level `tests/` directory with `package tests` (`tests/integration_test.go:1`)
- Unit tests next to handlers/services should follow Go default (e.g., `backend/handlers/auth_test.go`)

**Frontend:**
- Co-located with source: `utils.ts` + `utils.test.ts` in same directory (`frontend/src/lib/`)
- Pattern: `<source>.test.ts` for utilities, `<Component>.test.tsx` for components (no examples yet)

## Test Structure

### Backend integration test pattern (`tests/integration_test.go`)

```go
package tests

import (
    "net/http"
    "testing"
    "time"
)

func TestHealthCheck(t *testing.T) {
    baseURL := "http://localhost:8080/api/health"
    client := &http.Client{Timeout: 5 * time.Second}

    resp, err := client.Get(baseURL)
    if err != nil {
        t.Logf("Aviso: Backend não acessível (%v). Teste ignorado se o ambiente não estiver rodando.", err)
        return // soft-skip if backend is down
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        t.Errorf("Esperado status 200, recebeu %d", resp.StatusCode)
    }
}
```

**Notes:**
- Soft-skips when the backend isn't reachable — does not fail CI/local runs
- Hits port `8080` but the backend default in `backend/main.go:279` is `8081` — **this test is currently broken** unless `PORT=8080` is set explicitly

### Frontend unit test pattern (`frontend/src/lib/utils.test.ts`)

```typescript
import { expect, test } from 'vitest'
import { cn } from './utils'

test('cn merges class names correctly', () => {
  expect(cn('c-1', 'c-2')).toBe('c-1 c-2')
})

test('cn handles conditional classes', () => {
  expect(cn('c-1', true && 'c-2', false && 'c-3')).toBe('c-1 c-2')
})

test('cn merges tailwind classes', () => {
  expect(cn('p-1', 'p-2')).toBe('p-2')
})
```

**Patterns observed:**
- Flat `test()` calls (no `describe()` block) — fine for small helpers; group with `describe()` when a file grows past ~5 cases
- One assertion concept per test
- No setup/teardown needed for pure helpers

## Mocking

**Framework:** None set up.

**Backend approach (recommended):**
- Use `*sql.DB` against a real Postgres in Docker (the integration test pattern), or
- Refactor to accept a query interface and mock with a hand-rolled stub. The current handler signature `func XxxHandler(db *sql.DB) http.HandlerFunc` makes this easy: pass a stub that satisfies the methods you call.
- For HTTP handler tests use `httptest.NewRecorder` + `http.Request` directly:
  ```go
  req := httptest.NewRequest("GET", "/api/cfop", nil)
  rec := httptest.NewRecorder()
  ListCFOPsHandler(testDB)(rec, req)
  if rec.Code != http.StatusOK { ... }
  ```

**Frontend approach (recommended):**
- Vitest's built-in `vi.fn()` and `vi.mock()` for stubbing modules
- For `fetch`-based pages, override `window.fetch` in test setup (mirrors the production interceptor at `frontend/src/contexts/AuthContext.tsx:47-60`)
- React component tests would need `@testing-library/react` + `jsdom` — neither installed

**What to mock:**
- External APIs: Z.AI (`backend/services/ai.go`), email SMTP (`backend/services/email.go`), RFB endpoints (`backend/services/rfb.go`)
- `os.Getenv` for env-driven branching (`JWT_SECRET`, `ZAI_API_KEY`, `ALLOWED_ORIGINS`)
- `time.Now()` for token-expiry/rate-limiter tests

**What NOT to mock:**
- Pure helpers (`formatCNPJ`, `formatCurrency`, `cn`) — test them directly
- SQL itself when running against a disposable Postgres in CI/Docker — prefer real DB for integration tests

## Fixtures and Factories

**Test data:** None checked in.

**Suggested locations when added:**
- Backend: `backend/handlers/testdata/` (Go convention — auto-excluded from build) for sample SPED, NF-e XML, CT-e XML
- Frontend: `frontend/src/__fixtures__/` or inline in the test file

**SPED/NFe sample files:** large XML samples are not committed (see `.gitignore`). When tests need them, use minimal hand-crafted XML strings inline.

## Coverage

**Requirements:** None enforced.

**View Coverage:**
```bash
# Backend
cd backend && go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Frontend (after adding @vitest/coverage-v8)
cd frontend && npx vitest run --coverage
```

## Test Types

**Unit Tests:**
- Scope: pure functions in `frontend/src/lib/` and pure Go helpers (parsers, calculators)
- Approach: input → expected output, no I/O
- Examples in repo: only `cn` (`frontend/src/lib/utils.test.ts`)
- High-leverage candidates not yet covered:
  - `frontend/src/lib/formatFilial.ts` (CNPJ/CPF formatting + masking — 14 exported functions, regex-heavy)
  - `frontend/src/lib/navigation.ts` (`getActiveModule` routing logic)
  - Backend XML parsers in `backend/handlers/nfe_saidas.go`, `nfe_entradas.go`, `cte_entradas.go`
  - IBS/CBS projection math in `backend/handlers/dashboard.go:148-180`

**Integration Tests:**
- Scope: HTTP endpoint smoke checks against a running stack
- Approach: real `http.Client` to `localhost:PORT` — assumes Docker/dev server is up
- Example: `TestHealthCheck` in `tests/integration_test.go`
- **Known issue:** uses `:8080` but backend defaults to `:8081`; either fix the test or set `PORT=8080` in the integration environment

**E2E Tests:**
- Framework: Not used
- Manual QA via `simu.fcxlabs.com` (production) and Docker compose (`docker-compose.yml`) for local dev

## Common Patterns

### Async Testing (when added)

**Frontend (Vitest):**
```typescript
test('fetches projection data', async () => {
  const data = await fetchProjection({ mesAno: '03/2026' })
  expect(data).toHaveLength(7) // 2027-2033
})
```

**Backend (Go):**
```go
func TestLoginHandler(t *testing.T) {
    req := httptest.NewRequest("POST", "/api/auth/login",
        strings.NewReader(`{"email":"x@y.com","password":"secret123"}`))
    rec := httptest.NewRecorder()

    LoginHandler(testDB)(rec, req)

    if rec.Code != http.StatusOK {
        t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
    }
}
```

### Error Testing

**Backend:**
```go
// Test that the handler returns 401 with no auth header
req := httptest.NewRequest("GET", "/api/dashboard/projection", nil)
rec := httptest.NewRecorder()
AuthMiddleware(GetDashboardProjectionHandler(testDB), "")(rec, req)
if rec.Code != http.StatusUnauthorized { ... }
```

**Frontend:**
```typescript
test('formatCNPJ returns input when not 14 digits', () => {
  expect(formatCNPJ('123')).toBe('123')
  expect(formatCNPJ(null)).toBe('')
  expect(formatCNPJ(undefined)).toBe('')
})
```

### Rate Limiter Testing

The rate limiters in `backend/handlers/middleware.go:122-197` are testable in isolation:
```go
rl := newRateLimiter(2, time.Second)
if !rl.Allow("ip1") { t.Error("first attempt should pass") }
if !rl.Allow("ip1") { t.Error("second attempt should pass") }
if rl.Allow("ip1")  { t.Error("third attempt should be blocked") }
```

## Gaps and Recommendations

1. **No CI test step** — add `go test ./...` and `npm run test` to `.github/workflows/deploy-*.yml` before the build job
2. **Vitest not in `package.json`** — `frontend/src/lib/utils.test.ts` cannot run without manual install; fix dependencies first
3. **Integration test port mismatch** — `tests/integration_test.go:13` hits `:8080`, backend defaults to `:8081`
4. **Auth flow has zero tests** — `RegisterHandler`, `LoginHandler`, `RefreshHandler`, `AuthMiddleware` are critical and uncovered (`backend/handlers/auth.go`)
5. **Tax calculation logic untested** — IBS/CBS/ICMS projections in `backend/handlers/dashboard.go` and `backend/handlers/erp_bridge_batch.go` are business-critical and have no unit tests
6. **CNPJ/CPF formatting is regex-driven** — `frontend/src/lib/formatFilial.ts` is a high-leverage testing target

---

*Testing analysis: 2026-05-08*
