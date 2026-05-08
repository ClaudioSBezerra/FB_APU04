<!-- refreshed: 2026-05-08 -->
# Architecture

**Analysis Date:** 2026-05-08

## System Overview

```text
┌──────────────────────────────────────────────────────────────────┐
│                       FB_APU04 — Simulador Fiscal                 │
│                  (3-tier + external Bridge daemon)                │
└──────────────────────────────────────────────────────────────────┘

                        Browser (HTTPS)
                              │
                              ▼
┌──────────────────────────────────────────────────────────────────┐
│  Tier 1 — Frontend (React 18 + Vite SPA)                          │
│  Container: fb_apu04-web (nginx:stable-alpine, port 80)           │
│  Module: VITE_APP_MODULE=simu                                     │
│  Code: `frontend/src/`                                            │
│  Static built into `frontend/dist/` and served by nginx           │
│  /api/* proxied via `frontend/nginx.conf` → upstream `api:8084`   │
└──────────────────────┬────────────────────────────────────────────┘
                       │ JSON / multipart over HTTPS
                       │ Headers: Authorization: Bearer <JWT>
                       │          X-Company-ID: <uuid>
                       ▼
┌──────────────────────────────────────────────────────────────────┐
│  Tier 2 — Backend API (Go 1.22, net/http)                         │
│  Container: fb_apu04-api (port 8084)                              │
│  Module: APP_MODULE=simulador                                     │
│  Entry: `backend/main.go`                                         │
│  Layers:                                                          │
│    handlers/  — HTTP route handlers (per-feature files)           │
│    services/  — Cross-cutting integrations (AI, email, RFB, crypto)│
│    worker/    — Background SPED processor (3-worker pool)         │
│    migrations/— SQL files run at startup (file-based migrator)    │
└──────────┬───────────────────────────┬────────────────────┬───────┘
           │                           │                    │
           ▼                           ▼                    ▼
┌────────────────────┐   ┌─────────────────────────┐  ┌──────────────┐
│ PostgreSQL 15      │   │ Redis 7-alpine          │  │ Filesystem   │
│ Container: db      │   │ Container: redis        │  │ /root/uploads│
│ Volume:            │   │ maxmem 256mb,           │  │ (api_uploads │
│ postgres_data_apu04│   │ allkeys-lru, AOF on     │  │  named vol)  │
│ Schema-managed by  │   │ REDIS_ADDR=redis:6379   │  │              │
│ `migrations/*.sql` │   │ (token / cache use)     │  │              │
│ + Materialized     │   │                         │  │              │
│ Views:             │   │                         │  │              │
│  mv_mercadorias_   │   │                         │  │              │
│   agregada         │   │                         │  │              │
│  mv_operacoes_     │   │                         │  │              │
│   simples          │   │                         │  │              │
│  mv_compras_       │   │                         │  │              │
│   fornecedores     │   │                         │  │              │
└────────────────────┘   └─────────────────────────┘  └──────────────┘
           ▲
           │ POST /api/erp-bridge/import/batch  (X-API-Key auth)
           │ POST /api/erp-bridge/parceiros/sync
           │ GET  /api/erp-bridge/credentials  (returns AES-decoded creds)
           │ POST /api/erp-bridge/heartbeat
           │ GET  /api/erp-bridge/pending      (run queue polling)
           │
┌──────────────────────────────────────────────────────────────────┐
│  External — ERP Bridge (Python 3.12 daemon, AWS-hosted)           │
│  Code: `erp-bridge-aws/bridge.py` (single file, ~oracle_xml +     │
│  sap_s4hana modes)                                                │
│  Image: ghcr.io/claudiosbezerra/fb_apu04-bridge:latest            │
│  Compose: `installer/aws-bridge/docker-compose.yml`               │
│  Two parallel deployments (independent processes):                │
│    • APU02 — bare Python on AWS (config-apu02.yaml,               │
│      target https://fctax.fcxlabs.com)                            │
│    • APU04 — Docker container (config-apu04.yaml,                 │
│      target https://simu.fcxlabs.com)                             │
│  Local state: SQLite `tracker-<config>.db`, logs `logs-<config>/` │
│  Reads Oracle ERP (NF-e, CT-e, SAP S4/HANA tables) and posts      │
│  batches to Backend API.                                          │
└──────────────────────────────────────────────────────────────────┘
```

## Component Responsibilities

| Component | Responsibility | File |
|-----------|----------------|------|
| Go API entrypoint | Boot, env load, DB connect, route registration, graceful shutdown | `backend/main.go` |
| HTTP handlers | Per-feature request handlers (auth, upload, reports, ERP bridge, RFB, etc.) | `backend/handlers/*.go` |
| Background worker pool | Picks pending SPED jobs (`SKIP LOCKED`), parses, writes DB, refreshes views | `backend/worker/worker.go` |
| AI worker integration | Triggers Z.AI report generation after job completion | `backend/worker/ai_integration.go` |
| Services layer | AI client, email (SMTP), crypto (AES), RFB scraper/processor, text-to-SQL | `backend/services/*.go` |
| Migrations | Versioned SQL files applied on startup; tracked in `schema_migrations` | `backend/migrations/*.sql` |
| Frontend SPA root | React Router, QueryClient, AuthProvider, FilialProvider, layout shell | `frontend/src/App.tsx` |
| Auth context | Holds JWT + company; injects Authorization + X-Company-ID via fetch interceptor | `frontend/src/contexts/AuthContext.tsx` |
| Module navigation | Maps URL → module (simulador / notas / config) and tab list | `frontend/src/lib/navigation.ts` |
| Pages | One `.tsx` per screen (Mercadorias, Dashboard, ImportarEFD, …) | `frontend/src/pages/*.tsx` |
| ERP Bridge daemon | Reads Oracle, dedupes via SQLite tracker, POSTs JSON batches | `erp-bridge-aws/bridge.py` |
| Reverse proxy | TLS termination + path routing; `/api/*` → `api:8084` | `frontend/nginx.conf` (Traefik in prod) |

## Pattern Overview

**Overall:** Monolithic 3-tier (SPA + Go API + PostgreSQL) with one external Python daemon for ERP integration. The Go backend is a single process containing both the HTTP API and the SPED background worker pool — no separate worker service.

**Key Characteristics:**
- Single Go binary serves API and embeds the SPED worker pool (started inside `onDBConnected`).
- Module flag (`APP_MODULE=simulador`) gates which route groups are registered, allowing the same binary to run as Simulador (this repo) or Apuração (sister repo APU02).
- File-based, idempotent SQL migrator runs on every startup (`onDBConnected` in `backend/main.go`).
- Materialized views are the canonical aggregation surface; refreshed by the worker after the queue drains.
- ERP Bridge is a polling daemon: backend exposes a queue (`/api/erp-bridge/pending`), bridge picks runs and posts results.
- Two authentication schemes coexist on the API: **JWT Bearer** (browser/UI) and **X-API-Key** (bridge daemon) on a small subset of `/api/erp-bridge/*` routes.

## Layers

**Frontend SPA (`frontend/src/`):**
- Purpose: User-facing UI for Simulador da Reforma Tributária.
- Location: `frontend/src/`
- Contains: Pages (`pages/`), shared layout/components (`components/`, `components/ui/` shadcn), contexts (`contexts/`), helpers (`lib/`).
- Depends on: Backend API via `fetch` (auto-injected `Authorization` + `X-Company-ID`).
- Used by: Browser. Mounted by `frontend/src/main.tsx` into `#root`.

**HTTP Handlers (`backend/handlers/`):**
- Purpose: Translate HTTP requests into DB operations and JSON responses.
- Location: `backend/handlers/`
- Contains: One file per feature (`auth.go`, `upload.go`, `report.go`, `dashboard.go`, `nfe_entradas.go`, `cte_entradas.go`, `erp_bridge.go`, `erp_bridge_batch.go`, `rfb_apuracao.go`, `ai_query.go`, etc.).
- Depends on: `database/sql` (PostgreSQL), `services/` for crypto/email/AI, JWT.
- Used by: HTTP mux registered in `backend/main.go`.

**Services (`backend/services/`):**
- Purpose: External-integration / cross-cutting logic shared by handlers and worker.
- Location: `backend/services/`
- Contains: `ai.go` (Z.AI GLM client), `email.go` (SMTP), `crypto.go` (AES-GCM for ERP creds), `rfb.go` + `rfb_processor.go` (Receita Federal scraper), `text_to_sql.go` (NL → SQL for AI query).
- Depends on: stdlib + outbound HTTP / SMTP.
- Used by: Handlers and worker.

**Worker (`backend/worker/`):**
- Purpose: Asynchronous SPED EFD ICMS/IPI parsing and ingestion.
- Location: `backend/worker/`
- Contains: `worker.go` (pool, job picker, parser, view refresh), `ai_integration.go` (post-success AI triggers).
- Depends on: PostgreSQL, filesystem `uploads/`.
- Used by: Started by `worker.StartWorker(db)` from `backend/main.go:223` after DB connect.

**Persistence (`backend/migrations/`):**
- Purpose: Schema definition and seed data.
- Location: `backend/migrations/`
- Contains: 70+ numbered `.sql` files (000–072).
- Applied: At startup by `onDBConnected` in `backend/main.go:111`. Tracked in `schema_migrations(filename, executed_at)`.

**External Bridge (`erp-bridge-aws/`):**
- Purpose: Pull Oracle ERP data and push to backend.
- Location: `erp-bridge-aws/bridge.py`
- Contains: Single-file Python daemon, two configs (`config-apu02.yaml`, `config-apu04.yaml`).
- Per-config isolation: `tracker-<stem>.db` (SQLite dedupe) and `logs-<stem>/` chosen via `--config` (parsed in `_early_config_path` at module load — see `bridge.py:44`).

## Data Flow

### Primary Request Path — Browser → Report

1. User loads `/mercadorias` (`frontend/src/pages/Mercadorias.tsx:72`).
2. `AuthContext` fetch interceptor (`frontend/src/contexts/AuthContext.tsx:47`) attaches `Authorization: Bearer <jwt>` and `X-Company-ID`.
3. Request hits nginx (`frontend/nginx.conf:30`), proxied to `api:8084`.
4. `SecurityMiddleware` applies CORS + security headers (`backend/handlers/middleware.go:94`).
5. Mux dispatches to `/api/reports/mercadorias` → `handlers.GetMercadoriasReportHandler` (`backend/main.go:373`).
6. `withAuth` wrapper validates DB readiness, then `AuthMiddleware` validates JWT and sets `ClaimsKey` in context (`backend/handlers/auth.go:209`).
7. Handler calls `GetEffectiveCompanyID(db, userID, X-Company-ID)` (`backend/handlers/auth.go:276`) and queries `mv_mercadorias_agregada`.
8. JSON response returned; React Query caches it on the client.

### SPED Upload + Async Processing

1. UI POSTs `multipart/form-data` to `/api/upload` — supports chunked mode (`is_chunked=true`, `upload_id`, `chunk_index`) — see `backend/handlers/upload.go:80`.
2. Last chunk triggers job creation: row in `import_jobs` (status=`pending`).
3. Worker pool (3 goroutines, `backend/worker/worker.go:90`) selects a pending job using `FOR UPDATE SKIP LOCKED` inside a transaction (`backend/worker/worker.go:118`).
4. Worker parses SPED file, inserts rows, deletes file on completion (`backend/worker/worker.go:170`).
5. When queue empties (no `pending`/`processing` rows), worker `REFRESH MATERIALIZED VIEW CONCURRENTLY` for `mv_mercadorias_agregada`, `mv_operacoes_simples`, `mv_compras_fornecedores` (`backend/worker/worker.go:218`).
6. Worker triggers AI report generation (`TriggerAIReportGeneration` in `backend/worker/ai_integration.go`).

### ERP Bridge Import Flow

1. Admin sets bridge config (Oracle DSN + creds) via UI → `/api/erp-bridge/config` (encrypted with `services/crypto.go`).
2. Admin generates API key → `/api/erp-bridge/config/generate-api-key` (sha256 hash stored in `erp_bridge_config.api_key_hash`).
3. Daemon (`erp-bridge-aws/bridge.py`) GET `/api/erp-bridge/credentials` with `X-API-Key` → backend decrypts and returns Oracle creds (`backend/handlers/erp_bridge.go:572`).
4. Daemon polls `/api/erp-bridge/pending` for queued runs (`backend/handlers/erp_bridge.go:694`); UI enqueues runs via `/api/erp-bridge/trigger`.
5. Daemon connects to Oracle (`oracledb`), dedupes against `tracker-<config>.db`, batches results.
6. Daemon POSTs to `/api/erp-bridge/import/batch` (`backend/handlers/erp_bridge_batch.go:77`) — backend routes each `batchDoc` by `direct` + `modelo`:
   - `direct=2` → `nfe_saidas`
   - `direct=1` + modelo ∈ {55,62,65} → `nfe_entradas`
   - `direct=1` + modelo ∈ {57,66,67} → `cte_entradas`
7. Daemon also calls `/api/erp-bridge/parceiros/sync` to upsert CNPJ→nome lookup table (`parceiros`).
8. Daemon sends periodic `/api/erp-bridge/heartbeat` to update `daemon_last_seen`.

**State Management:**
- Server: PostgreSQL is the single source of truth; in-memory `sync.Map` for refresh-token store and JWT blacklist (`backend/handlers/auth.go:90`); in-memory rate limiters (`backend/handlers/middleware.go:140`).
- Client: React Query (`@tanstack/react-query`) for server cache; Auth + Filial in React Context; selected company persisted (via `AuthContext.switchCompany`).
- Bridge: SQLite tracker per config (`tracker-config-apu02.db`, `tracker-config-apu04.db`) — never shared between deployments.

## Key Abstractions

**Module flag (`APP_MODULE` / `VITE_APP_MODULE`):**
- Purpose: Single codebase serves Simulador (FB_APU04) and Apuração (FB_APU02) by gating route groups and frontend tabs.
- Examples: `backend/main.go:268`, `backend/main.go:520` (RFB routes skipped when `APP_MODULE=simulador`), `frontend/src/lib/navigation.ts`.
- Pattern: Env-flag conditional registration (no plugin system).

**Hierarchical company context:**
- Purpose: Multi-tenant scoping. User belongs to Environment → Group → Company → Branches (filiais).
- Examples: `backend/handlers/hierarchy.go:21`, `backend/handlers/auth.go:276` (`GetEffectiveCompanyID`).
- Pattern: Every privileged handler reads `claims.user_id` from context, then resolves company via header `X-Company-ID` with DB validation.

**Handler factory:**
- Purpose: Inject DB into handlers without globals.
- Examples: `func XHandler(db *sql.DB) http.HandlerFunc` everywhere in `backend/handlers/`.
- Pattern: Wrapped at registration with `withDB` / `withAuth` (`backend/main.go:345`) so each request retrieves the live `*sql.DB`, surviving early startup when DB is still connecting.

**Materialized view aggregation:**
- Purpose: Pre-compute heavy aggregations across multi-million-row SPED data.
- Examples: `mv_mercadorias_agregada`, `mv_operacoes_simples`, `mv_compras_fornecedores` (defined across migrations 027–070).
- Pattern: Refreshed `CONCURRENTLY` from worker only when import queue is empty (`backend/worker/worker.go:218`).

## Entry Points

**Backend HTTP API:**
- Location: `backend/main.go:252` (`func main`)
- Triggers: Process start (Docker `CMD ["./fb_apu04-api"]` in `Dockerfile.production`).
- Responsibilities: Load `.env`, validate `JWT_SECRET`, kick off `initDBAsync`, register all routes, start `http.Server` on `PORT=8084` with 5-minute Read/Write timeouts, graceful shutdown on SIGTERM/SIGINT.

**Background worker:**
- Location: `backend/worker/worker.go:21` (`StartWorker`)
- Triggers: Called from `onDBConnected` (`backend/main.go:223`) after first successful DB connection.
- Responsibilities: Spawn 3 goroutines (`workerLoop`), each polling `import_jobs` every 2s, processing SPED files, refreshing MVs, deleting upload files.

**Frontend SPA:**
- Location: `frontend/src/main.tsx:6` mounts `frontend/src/App.tsx:176`.
- Triggers: Browser load of `index.html` served by nginx.
- Responsibilities: Wrap app in `QueryClientProvider` + `BrowserRouter` + `AuthProvider`; route to login or `AppLayout`; gate admin pages with `<AdminRoute>`.

**ERP Bridge daemon:**
- Location: `erp-bridge-aws/bridge.py` (Docker `CMD ["python", "bridge.py", "--daemon"]`).
- Triggers: Container start (`installer/aws-bridge/docker-compose.yml`); auto-updated by `watchtower` on a 300s poll.
- Responsibilities: Read `--config <file>`, connect Oracle, poll backend pending queue, push batches.

## Architectural Constraints

- **Threading:** Backend uses Go's net/http per-request goroutines + a fixed worker pool of 3 SPED workers (`backend/worker/worker.go:23`). Worker count is tuned for ~2 vCPU since DB I/O dominates.
- **Global state:** Module-level globals in backend — `db *sql.DB` + `dbMutex sync.RWMutex` (`backend/main.go:50`), `refreshTokenStore` and `tokenBlacklist` `sync.Map` (`backend/handlers/auth.go:90`), in-memory rate limiters `LoginRL` / `RegisterRL` / `ForgotPasswordRL` (`backend/handlers/middleware.go:140`). All survive only as long as the process; restart wipes all sessions/limits.
- **DB readiness:** `db` may be `nil` during startup (async connect with infinite retry — `backend/main.go:62`). All handler registrations wrap with `withDB`/`withAuth` which return 503 until ready.
- **Migrations are non-transactional and partially-tolerant:** errors containing "already exists" are silently recorded as applied (`backend/main.go:208`). Adding migrations that fail other ways will spam logs but not crash.
- **Module flag is binary:** Setting `APP_MODULE=simulador` removes RFB / Apuração routes entirely (`backend/main.go:520`). FB_APU04 runs only in this mode in production.
- **Two-instance bridge isolation:** The Python bridge enforces per-config isolation via `--config` filename (`bridge.py:44`). Running APU02 and APU04 from the same binary requires distinct config files; tracker DBs and log dirs derive from `Path(--config).stem`.
- **Port allocation:** Backend listens on `8084` in production (override via `PORT` env). Default fallback inside `main.go:279` is `8081` (legacy).
- **Upload size:** Nginx `client_max_body_size 2048M` (`frontend/nginx.conf:7`); backend `proxy_request_buffering off` for streaming. Backend `ParseMultipartForm(64<<20)` keeps 64MB in memory and spills to disk.

## Anti-Patterns

### Per-handler manual CORS

**What happens:** Some handlers still set `Access-Control-Allow-Origin: *` directly (e.g. `backend/handlers/upload.go:33`, `backend/handlers/dashboard.go:26`).
**Why it's wrong:** It contradicts the centralized origin allowlist in `SecurityMiddleware` (`backend/handlers/middleware.go:53`), which then has to override these headers via a wrapping `ResponseWriter`.
**Do this instead:** New handlers must NOT set CORS headers. Rely on `SecurityMiddleware` registered in `backend/main.go:647`. The middleware's `secureResponseWriter` already strips wildcard headers, but cleaner to never write them.

### Mux-method dispatch inline in `main.go`

**What happens:** Several routes (`/api/config/forn-simples`, `/api/config/environments`, `/api/config/groups`, `/api/config/companies`, `/api/rfb/credentials`, `/api/managers/`) implement `switch r.Method` blocks directly inside `main.go` (e.g. `backend/main.go:449`, `backend/main.go:472`).
**Why it's wrong:** Bloats `main.go` (~680 lines) and duplicates DB-readiness checks.
**Do this instead:** New endpoints should expose a single `XHandler(db) http.HandlerFunc` that internally switches on `r.Method` (pattern used in `erp_bridge.go`), and `main.go` should only register the route.

### Direct SQL in handlers (no repository layer)

**What happens:** Handlers run inline `db.Query` / `db.Exec` SQL (e.g. `backend/handlers/hierarchy.go:30`, `backend/handlers/erp_bridge.go:560`). There is no repository / model abstraction.
**Why it's wrong:** Schema changes require touching every handler that references a table; SQL is duplicated.
**Do this instead:** Acceptable for the current size, but new aggregations should live in `backend/services/` or in materialized views, not be re-implemented per handler.

### Tools directory shipped with binary

**What happens:** `backend/tools/debug_*.go` are compile-able diagnostic scripts living next to `main.go` (`backend/tools/debug_query.go`, etc.).
**Why it's wrong:** They are not isolated by build tags and could accidentally be linked or run in prod.
**Do this instead:** Either move under a `cmd/<tool>/main.go` per-binary layout or guard with `//go:build tools`.

## Error Handling

**Strategy:** Plain HTTP status + `http.Error` text for handlers; structured logging for worker; panic recovery with auto-restart for worker goroutines (`backend/worker/worker.go:99`).

**Patterns:**
- Handlers return `http.StatusUnauthorized` / `Forbidden` / `BadRequest` with plain-text body via `http.Error`.
- DB-not-ready returns 503 with JSON `{"error":"service_unavailable", ...}` (`backend/main.go:339`).
- Worker wraps each loop with `defer recover()` → logs panic and respawns the goroutine after 5s.
- Migration errors containing "already exists" are downgraded to "applied" to allow idempotent restarts (`backend/main.go:208`).
- Crash recovery: stuck `processing` jobs are reset to `pending` at boot (`backend/worker/worker.go:80`); orphan upload files with no active job are deleted (`backend/worker/worker.go:43`).

## Cross-Cutting Concerns

**Logging:**
- Backend: `log` (stdlib) + `fmt.Printf`. No structured logger. Output goes to stdout, captured by Docker's `json-file` driver.
- Bridge: stdlib `logging` with file handler in `LOG_DIR` and stdout handler (`erp-bridge-aws/bridge.py:67`).
- Frontend: `frontend/src/lib/logger.ts` thin wrapper over `console`.

**Validation:**
- Server: ad-hoc per handler (no validator library). Some `zod` schemas exist on the frontend (`zod` dep in `frontend/package.json`).
- File integrity: SPED upload validates `|0000|` header and `|9999|` trailer (`backend/worker/worker.go:272`, `backend/handlers/upload.go:167`).

**Authentication:**
- JWT (HS256, 30-min access tokens) for browser/UI — `backend/handlers/auth.go:136`.
- Refresh tokens stored in `sync.Map` and an HttpOnly cookie scoped to `/api/auth/`.
- API-Key (sha256-hashed, stored in `erp_bridge_config.api_key_hash`) for the bridge daemon — `backend/handlers/erp_bridge.go:572`.
- Role gate: `AuthMiddleware(handler, "admin")` — admin always passes; non-admins fail when role mismatches (`backend/handlers/auth.go:253`).

**Encryption:**
- AES-GCM for ERP credentials (Oracle DSN/user/password, FBTax email/password) via `backend/services/crypto.go` and `backend/handlers/crypto.go`. Falls back to JWT_SECRET if `ENCRYPTION_KEY` is unset (`backend/main.go:260`).

---

*Architecture analysis: 2026-05-08*
