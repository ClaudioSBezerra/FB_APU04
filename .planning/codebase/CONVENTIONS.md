# Coding Conventions

**Analysis Date:** 2026-05-08

## Naming Patterns

### Backend (Go)

**Files:**
- Lower snake_case: `auth.go`, `nfe_entradas.go`, `erp_bridge_batch.go`, `simples_dashboard.go`
- Domain-grouped per file (one feature area per file): `cfop.go`, `dashboard.go`, `cte_entradas.go`
- Backup files use `.bak` suffix and should be cleaned up: `auth.go.bak` (`backend/handlers/auth.go.bak`)

**Packages:**
- Single short lowercase name matching directory: `package handlers`, `package services`, `package worker`, `package main`
- Module path: `fb_apu01` (legacy name from APU01 fork; see `backend/go.mod`)

**Functions (Go):**
- Exported: PascalCase ending in `Handler` for HTTP handlers — `ListCFOPsHandler`, `LoginHandler`, `GetDashboardProjectionHandler`
- Unexported helpers: camelCase — `jsonErr`, `parseNFeXML`, `erpBridgeGetCompany`, `getJWTSecret`
- Handler factory pattern: `func XxxHandler(db *sql.DB) http.HandlerFunc` returning a closure (`backend/handlers/cfop.go:18`, `backend/handlers/auth.go:184`)
- Boolean checkers/getters omit `Get` for unexported (`isSecureCookie`) but use `Get` for exported (`GetClientIP`, `GetUserIDFromContext`)

**Types (Go):**
- Exported structs: PascalCase — `CFOP`, `User`, `RegisterRequest`, `AuthResponse`, `ERPBridgeConfig`, `ProjectionPoint`
- Unexported request/response/row structs: camelCase — `chatRequest`, `chatMessage`, `nfeEntradaRow`, `nfeImpostosRow`, `batchDoc`
- JSON tags use snake_case: `` `json:"company_id"` ``, `` `json:"vl_icms"` ``, `` `json:"chave_nfe"` ``

**Variables (Go):**
- Local: short camelCase — `userID`, `companyID`, `mesAno`, `claims`, `tx`, `rows`
- Exported package-level: PascalCase — `LoginRL`, `RegisterRL`, `ForgotPasswordRL`, `ClaimsKey`
- Constants: PascalCase for exported (`BackendVersion`, `FeatureSet`, `ModelFlash`); ALL_CAPS not used

### Frontend (TypeScript/React)

**Files:**
- Components/Pages: PascalCase `.tsx` — `App.tsx`, `Dashboard.tsx`, `Mercadorias.tsx`, `ConsultaNFesEntradas.tsx`, `AuthContext.tsx`
- Utilities: camelCase `.ts` — `utils.ts`, `formatFilial.ts`, `exportToExcel.ts`, `navigation.ts`, `logger.ts`
- Tests: `.test.ts` next to source — `utils.test.ts` (`frontend/src/lib/utils.test.ts`)
- Hooks: kebab-case `.tsx` — `use-mobile.tsx`

**Functions:**
- React components: PascalCase — `Login`, `Dashboard`, `ProtectedRoute`, `AppHeader`, `ModuleTabs`
- Hooks/utilities: camelCase — `useAuth`, `formatCurrency`, `formatCNPJ`, `parseFilialName`, `getActiveModule`
- Event handlers: `handleXxx` prefix — `handleLogin`, `handleFilterChange`

**Types/Interfaces:**
- Always PascalCase — `User`, `AuthContextType`, `ProjectionPoint`, `NfeImpostosRow`, `ModuleTab`
- React props inlined in function signature: `function ComingSoon({ title }: { title: string })` (`frontend/src/App.tsx:36`)

**Variables:**
- camelCase — `companyId`, `selectedMonth`, `tokenRef`, `apiVersion`
- Boolean state often uses `is`/`has` prefix — `isLoading`, `isAuthenticated`, `isAdmin`, `isFilialSelected`

## Code Style

### Backend (Go)

**Formatting:**
- Tabs for indentation (Go default — `gofmt`)
- No formatter config committed; rely on `go fmt` / `goimports`
- Banner comments as section dividers using `─` box-drawing: `// ─── CORS-fixing ResponseWriter ─` (`backend/handlers/middleware.go:36`)
- Block comments using `// ---------------------------------------------------------------------------` separate handler groups (`backend/handlers/nfe_entradas.go:16`)

**Linting:**
- No `golangci.yml` or other lint config committed
- Vendored dependencies in `backend/vendor/` (`go.sum`/`go.mod` strict)

### Frontend (TypeScript/React)

**Formatting:**
- 2-space indentation
- Single quotes for strings (`'react'`, `'@/lib/utils'`)
- Trailing commas in multi-line object/array literals
- No `.prettierrc` or `eslint.config.*` checked into repo — relies on default `eslint .` script (`frontend/package.json:9`)

**Linting:**
- Run via `npm run lint` (`frontend/package.json`)
- TypeScript strict mode enabled (`frontend/tsconfig.app.json:21`)
  - `noUnusedLocals: true`
  - `noUnusedParameters: true`
  - `noFallthroughCasesInSwitch: true`

## Import Organization

### Backend (Go) — `backend/handlers/auth.go:3-18`

**Order (separated by blank line):**
1. Standard library — `"context"`, `"database/sql"`, `"encoding/json"`, `"net/http"`
2. Internal modules — `"fb_apu01/services"`
3. Third-party — `"github.com/golang-jwt/jwt/v5"`, `"golang.org/x/crypto/bcrypt"`

Driver imports use blank identifier: `_ "github.com/lib/pq"` (`backend/main.go:23`)

### Frontend (TypeScript) — `frontend/src/pages/Mercadorias.tsx:1-29`

**Order (no enforced grouping, but conventional):**
1. React + react-router — `react`, `react-router-dom`
2. UI library — `@/components/ui/*` (Radix-based shadcn components)
3. Third-party — `recharts`, `lucide-react`
4. Internal aliased — `@/lib/utils`, `@/lib/formatFilial`, `@/contexts/AuthContext`
5. Relative — rare; pages typically use `@` alias

**Path Aliases:**
- `@/*` → `./src/*` (defined in `frontend/tsconfig.app.json:27` and `frontend/vite.config.ts:30`)

## Error Handling

### Backend — Two Coexisting Patterns

**Legacy pattern (plain text via `http.Error`)** — used in older handlers (`backend/handlers/cfop.go`, `backend/handlers/dashboard.go`, `backend/handlers/auth.go`):
```go
if err != nil {
    http.Error(w, err.Error(), http.StatusInternalServerError)
    return
}
```
- Sends `text/plain` body with raw error string. Frontend often parses with `typeof data === 'string'` fallback (`frontend/src/pages/Login.tsx:53`).

**Modern pattern (JSON via `jsonErr`)** — used in newer handlers (`backend/handlers/nfe_entradas.go`, `backend/handlers/nfe_saidas.go`, `backend/handlers/ai_query.go`):
```go
jsonErr(w, http.StatusUnauthorized, "Unauthorized")
```
- `jsonErr` defined once at `backend/handlers/ai_query.go:55` — sets `Content-Type: application/json`, encodes `{"error": "..."}`, accepts optional `extra` map for additional fields.
- **Prefer this pattern in new code.** Do not duplicate the helper — import from package.

**Defer rollback on transactions:**
```go
tx, err := db.Begin()
if err != nil { ... }
defer tx.Rollback()  // safe even after Commit
```
(`backend/handlers/auth.go:436`, `backend/handlers/cfop.go:106`)

**Sentinel/wrapped errors:** uses `fmt.Errorf` for context, `sql.ErrNoRows` for missing-row branching (`backend/handlers/auth.go:313`).

### Frontend — try/catch + toast

```typescript
try {
  const res = await fetch("/api/auth/login", { ... });
  const data = await res.json();
  if (!res.ok) throw new Error(typeof data === 'string' ? data : "Credenciais inválidas");
  // ...
} catch (error: any) {
  const msg = error.message || "Erro desconhecido";
  setErrorMsg(msg);
  toast.error(msg);
} finally {
  setIsLoading(false);
}
```
(`frontend/src/pages/Login.tsx:42-66`)

- User-facing errors via `sonner`'s `toast.error(...)` / `toast.success(...)`
- Inline `<Alert variant="destructive">` for form-level errors
- Network failures often swallowed with `.catch(() => {})` for non-critical fetches (e.g. apelidos lookup, `frontend/src/pages/Mercadorias.tsx:116`)

## Logging

### Backend

**Framework:** stdlib `log` and `fmt`. No structured logger.

**Patterns:**
- `log.Printf("[Register] Error creating user: %v", err)` — bracketed prefix tag for area (`backend/handlers/auth.go:449`)
- `fmt.Printf("Worker Startup: ...\n", ...)` — for worker/startup output (`backend/worker/worker.go:68`)
- `log.Println("WARNING: ...")` for security-relevant warnings (`backend/handlers/auth.go:78`)
- `log.Fatal(...)` only at startup for fatal misconfig (`backend/handlers/auth.go:76`: missing JWT_SECRET in prod)

**What to log:**
- All DB/transaction errors before responding to the client
- Auth events (failed lookups, fallback to default company)
- Migration progress in `onDBConnected` (`backend/main.go:198`)

**What NOT to log:**
- Passwords, JWT contents, refresh tokens
- Request bodies for `/api/auth/login`, `/api/auth/register`

### Frontend

**Framework:** `frontend/src/lib/logger.ts` defines a `Logger` static class with levels `info|warn|error|debug`, but it is **not yet used widely**. Most code uses `console.log` / `console.error` directly.

**Pattern (when used):**
```typescript
import { logger } from '@/lib/logger';
logger.error('Failed to fetch X', err, 'Mercadorias');
```
- Production: serializes JSON; console.error/warn/log by level
- Development: collapsed groups with colored prefix

**Current dominant pattern:**
```typescript
.catch((err) => console.error("Failed to fetch tax rates", err))
```
(`frontend/src/pages/Mercadorias.tsx:99`) — prefer migrating to `logger` over time.

## Comments

**When to Comment:**
- Section dividers (Go): `// ─── Section Name ────` and `// ---` blocks separate handler groups
- Bilingual: comments mix Portuguese (domain context) and English (technical) — both accepted: `// Restaura preferência de empresa salva` (`frontend/src/contexts/AuthContext.tsx:131`)
- Business rules: explain SPED layouts, CFOP types, IBS/CBS calculation rules (`backend/handlers/erp_bridge_batch.go:14-23`, `backend/handlers/dashboard.go:148`)
- Security rationale: `// Use rightmost IP (added by our trusted reverse proxy, not spoofable)` (`backend/handlers/middleware.go:204`)

**JSDoc/TSDoc:**
- Used in `frontend/src/lib/formatFilial.ts` for every exported helper:
  ```typescript
  /**
   * Formata CNPJ completo com pontuação
   * Entrada: "12345678000190" → Saída: "12.345.678/0001-90"
   */
  export function formatCNPJ(cnpj: string | null | undefined): string { ... }
  ```
- Inconsistent across the codebase — most components/handlers have only inline `//` comments.

**Remove before commit:**
- TODO/FIXME comments are rare; do not introduce them without a tracking issue
- "Force rebuild" comments at top of `backend/main.go:3` and `backend/worker/worker.go:4` are deployment artifacts — do not propagate

## Function Design

**Backend size:**
- Handler factory functions are large (200-800 lines) and contain the full request lifecycle — **this is the established pattern** for `LoginHandler`, `RegisterHandler`, `NfeEntradasUploadHandler`, `ERPBridgeBatchImportHandler`. Splitting is fine but not required.
- Helpers (`GetEffectiveCompanyID`, `GetClientIP`, `parseNFeXML`) are short and reusable.

**Backend parameters:**
- All handlers receive `*sql.DB` via the factory pattern, never as a global
- The `*sql.DB` is read at request time via `withDB`/`withAuth` wrappers in `backend/main.go:345-366` (so DB-not-ready returns `503`)

**Frontend size:**
- Pages are large (300-900 lines); business logic and JSX colocated. Examples: `Mercadorias.tsx` (35KB), `ERPBridgeConfig.tsx` (31KB), `Login.tsx` (~210 lines).
- Extracting hooks/sub-components is acceptable but not enforced.

**Return Values:**
- Go: idiomatic `(value, error)` pairs; handlers return nothing (write to `ResponseWriter`)
- TypeScript: explicit return types optional; complex helpers in `formatFilial.ts` use explicit annotations

## Module Design

### Backend

**Exports:**
- Single package per directory: `handlers`, `services`, `worker`
- Cross-package access is by capitalization (`handlers.AuthMiddleware`, `services.NewAIClient`)
- `services/` is restricted directory (mode `0700` — `drwx------`); contains email and crypto-sensitive code

**Package layout:**
```
backend/
├── main.go            # Routes, DB init, middleware wiring
├── handlers/          # HTTP handlers (one file per domain area)
├── services/          # External integrations (AI, email, RFB, crypto)
├── worker/            # Background SPED processor
├── tools/             # CLI debugging utilities (debug_*.go, verify_data.go)
├── migrations/        # SQL files NNN_description.sql
└── vendor/            # Vendored deps (committed)
```

### Frontend

**Exports:**
- Default export for pages: `export default Login` (`frontend/src/pages/Login.tsx:208`)
- Named exports for utilities and components: `export function cn(...)`, `export const AuthProvider = ...`, `export const useAuth = ...`

**Barrel files:** Not used — every import paths to the source file (`@/components/ui/card`, `@/lib/utils`).

**Layout:**
```
frontend/src/
├── App.tsx            # Router + layout + module tabs
├── main.tsx           # ReactDOM entry
├── pages/             # Route components (PascalCase.tsx)
├── components/        # Shared components + components/ui/ (shadcn)
├── contexts/          # React Contexts (AuthContext, FilialContext)
├── hooks/             # Custom hooks (use-mobile)
├── lib/               # Pure utilities (utils, logger, formatFilial, exportToExcel, navigation)
└── index.css          # Tailwind base + globals
```

## API Conventions

**HTTP method handling:**
- Each handler explicitly checks `r.Method` and returns `405 Method Not Allowed` for non-allowed verbs
- `OPTIONS` → `200/204` for CORS preflight (handled both globally in `SecurityMiddleware` and locally in some handlers — defensive duplication)

**Auth contract:**
- `Authorization: Bearer <jwt>` required on protected routes (validated in `AuthMiddleware`, `backend/handlers/auth.go:209`)
- `X-Company-ID` header selects the active company (passed to `GetEffectiveCompanyID`)
- Refresh token via httpOnly cookie at `/api/auth/refresh`
- Frontend interceptor at `frontend/src/contexts/AuthContext.tsx:47-60` injects both headers automatically into `window.fetch`

**JSON tag style:** snake_case for all request/response fields — even when Go field is PascalCase

**Pagination/filtering:**
- Query params: `mes_ano`, `filiais` (comma-separated CNPJs), `company_id`
- Filiais filter pattern: split CSV → build `IN ($n,...)` placeholders dynamically (`backend/handlers/dashboard.go:75-90`)

## Database Conventions

**Migrations:** `backend/migrations/NNN_description.sql`
- Three-digit prefix, snake_case description
- All statements idempotent: `CREATE TABLE IF NOT EXISTS`, `ADD COLUMN IF NOT EXISTS`, `ON CONFLICT DO NOTHING`
- Tracked in `schema_migrations(filename, executed_at)` — auto-created/upgraded by `onDBConnected` (`backend/main.go:111-220`)
- Disabled migrations use `.sql.disabled` suffix (e.g., `000_reset_db.sql.disabled`, `037_delete_gilson_user.sql.disabled`)
- **Do not edit existing migrations**; add a new one with the next number

**SQL style in handlers:**
- Multi-line raw strings with backtick delimiters
- Always parameterized (`$1`, `$2`) — never string concatenation
- `COALESCE(..., default)` to handle nullable columns instead of `sql.NullX` in struct
- `SELECT EXISTS(...)` pattern for boolean checks (`backend/main.go:132`)

**Comments on schema:** Use `COMMENT ON COLUMN` for non-obvious enum values (`backend/migrations/065_erp_bridge.sql:24`).

---

*Convention analysis: 2026-05-08*
