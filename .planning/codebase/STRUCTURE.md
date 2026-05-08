# Codebase Structure

**Analysis Date:** 2026-05-08

## Directory Layout

```
FB_APU04/
├── backend/                      # Go API + worker (single binary)
│   ├── main.go                   # Entry point, routes, migrations runner
│   ├── go.mod / go.sum           # Module: fb_apu01 (legacy name kept)
│   ├── Dockerfile                # Dev image (used by docker-compose.yml)
│   ├── handlers/                 # HTTP handlers — one file per feature
│   ├── services/                 # AI, email, crypto, RFB, text-to-SQL
│   ├── worker/                   # SPED background worker pool
│   ├── migrations/               # 70+ numbered SQL files (000..072)
│   ├── tools/                    # Standalone debug Go programs
│   └── vendor/                   # Vendored Go modules
│
├── frontend/                     # React 18 + Vite SPA
│   ├── src/
│   │   ├── main.tsx              # ReactDOM root
│   │   ├── App.tsx               # Routes + layout shell
│   │   ├── pages/                # One .tsx per screen
│   │   ├── components/           # Shared layout components
│   │   │   └── ui/               # shadcn/Radix primitives (50+ files)
│   │   ├── contexts/             # AuthContext, FilialContext
│   │   ├── hooks/                # use-mobile (only one)
│   │   └── lib/                  # utils, navigation, exportToExcel, logger
│   ├── public/                   # Static assets copied as-is
│   ├── dist/                     # Vite build output (committed nothing)
│   ├── nginx.conf                # Reverse proxy → api:8084
│   ├── Dockerfile / Dockerfile.dev
│   ├── vite.config.ts            # Dev proxy /api → VITE_API_TARGET
│   ├── tailwind.config.js
│   └── package.json              # name: fb_apu01-frontend (legacy)
│
├── erp-bridge-aws/               # Python ERP integration daemon
│   ├── bridge.py                 # Single-file daemon (Oracle → /api/erp-bridge/*)
│   ├── Dockerfile                # python:3.12-slim
│   ├── config-apu02.yaml         # Config for sister system (FB_APU02)
│   └── config-apu04.yaml         # Config for THIS system (FB_APU04)
│
├── installer/                    # Customer-facing deployment bundles
│   ├── install.sh / update.sh
│   ├── docker-compose.yml        # Top-level installer compose
│   ├── README.md
│   ├── QUESTIONARIO_CLIENTE.md
│   ├── aws-bridge/               # Bridge container compose for AWS
│   │   ├── config.yaml.example
│   │   └── docker-compose.yml    # Pulls fb_apu04-bridge:latest + watchtower
│   └── fcxlabs/
│       └── docker-compose.yml    # Pulls fb_apu04-api/web/db/redis from GHCR
│
├── scripts/                      # Ops / deploy shell scripts
│   ├── deploy_production.sh
│   ├── backup_production.sh
│   ├── tunnel-prod-db.sh
│   ├── check_materialized_views.sh
│   ├── promote_to_prd.bat
│   ├── transport_to_qa.bat
│   ├── cfop_completo.csv
│   └── cfop_completo_semicolon.csv
│
├── scripts_oracle/               # Oracle SQL templates used by ERP Bridge
│   ├── gera_xmls_nfe.sql
│   ├── gera_xmls_nfse_saidas*.sql
│   ├── importa_cte_entrada.sql
│   └── importa_xmls_entrada.sql
│
├── tests/                        # Top-level tests directory
├── config/                       # Static config files
├── docs/                         # Project documentation
├── _bmad/ + _bmad-output/        # BMAD agent workflow tooling (vendor tooling)
├── .planning/                    # GSD planning artifacts (this directory)
│   └── codebase/                 # Codebase maps (ARCHITECTURE.md, STRUCTURE.md, …)
├── .claude/                      # Claude Code project memory
├── .github/                      # GitHub Actions / templates
├── docker-compose.yml            # Dev compose (build from source)
├── docker-compose.prod.yml       # Production compose (build via Dockerfile.production)
├── Dockerfile.production         # Multi-stage: front+back → single image
├── .env.example                  # Documented env template
├── .env.production / .env.hostinger  # Environment-specific overrides
├── coolify-env-template.txt
├── README.md                     # Project overview (legacy FB_APU01 wording)
├── VERSIONAMENTO.md / VERSIONAMENTO_AUTO.md
├── VALIDATION_REPORT_20260204.md
├── setup_wsl_env.sh
├── start_dev.bat / start_docker.bat
├── cleanup_old_branches.sh / delete_branches_with_token.sh / git_push_helper.sh
└── .vscode/
```

## Directory Purposes

**`backend/`:**
- Purpose: Go 1.22 API server and SPED worker — single binary.
- Contains: `main.go`, `handlers/`, `services/`, `worker/`, `migrations/`, `tools/`, `vendor/`.
- Key files: `backend/main.go` (route registry + migration runner), `backend/go.mod` (`module fb_apu01` — legacy name kept).

**`backend/handlers/`:**
- Purpose: HTTP request handlers. One Go file per feature.
- Contains: 30 `.go` files. Notable: `auth.go` (JWT + AuthMiddleware), `middleware.go` (CORS + rate limiting), `upload.go` (chunked SPED upload), `erp_bridge.go` + `erp_bridge_batch.go` + `erp_bridge_parceiros.go`, `dashboard.go`, `report.go`, `nfe_entradas.go`, `nfe_saidas.go`, `cte_entradas.go`, `rfb_*.go`, `ai_*.go`, `hierarchy.go`, `managers.go`, `admin.go`.
- Key files: `backend/handlers/auth.go`, `backend/handlers/middleware.go`, `backend/handlers/erp_bridge.go`.

**`backend/services/`:**
- Purpose: Cross-cutting integrations shared by handlers and worker.
- Contains: `ai.go` (Z.AI GLM client), `email.go` (SMTP), `crypto.go` (AES-GCM), `rfb.go` + `rfb_processor.go` (Receita Federal integration — disabled when `APP_MODULE=simulador`), `text_to_sql.go` (NL → SQL).
- Key files: `backend/services/ai.go`, `backend/services/crypto.go`.

**`backend/worker/`:**
- Purpose: Background SPED EFD ICMS/IPI processing.
- Contains: `worker.go` (pool startup, job loop, parser, MV refresh), `ai_integration.go` (post-completion AI hooks).
- Key files: `backend/worker/worker.go`.

**`backend/migrations/`:**
- Purpose: Versioned schema migrations applied at startup by `onDBConnected`.
- Contains: 70+ SQL files numbered `000_..072_`. Files ending `.disabled` are skipped (kept for history).
- Naming: `NNN_description.sql` strictly numeric prefix; gaps allowed; alphabetical ordering by `filepath.Glob` is what's used.
- Key files: `backend/migrations/065_erp_bridge.sql` (ERP Bridge schema), `backend/migrations/021_create_mv_mercadorias.sql` + `027_update_mv_mercadorias_v2.sql` (materialized views).

**`backend/tools/`:**
- Purpose: One-off diagnostic Go programs (NOT linked into `main.go`).
- Contains: `debug_detailed.go`, `debug_gilson.go`, `debug_query.go`, `debug_stats.go`, `verify_data.go`.
- Key files: Each compile-able with `go run backend/tools/<file>.go`. Not gated by build tags.

**`frontend/src/pages/`:**
- Purpose: Top-level route components. One per screen.
- Contains: 33 `.tsx` files. Auth screens (`Login`, `Register`, `ForgotPassword`, `ResetPassword`); Simulador screens (`Mercadorias`, `OperacoesSimplesNacional`, `Dashboard`, `ImportarEFD`, `ExecutiveSummary`, `ConsultaInteligente`); Notas Importadas screens (`ConsultaNFesEntradas`, `ConsultaNFeSaidas`, `ConsultaCTesEntradas`, `ImportarXMLs*`, `ERPBridge*`); Config screens (`TabelaAliquotas`, `TabelaCFOP`, `TabelaFornSimples`, `ApelidosFiliais`, `GestaoAmbiente`, `Managers`, `AdminUsers`, `RFB*`).
- Key files: `frontend/src/pages/Mercadorias.tsx`, `frontend/src/pages/Dashboard.tsx`, `frontend/src/pages/ERPBridgeConfig.tsx`.

**`frontend/src/components/`:**
- Purpose: Shared layout-level components (sidebar, rail, switchers, footer, upload UI).
- Contains: 9 root components + `ui/` (shadcn primitives).
- Key files: `frontend/src/components/AppRail.tsx` (left navigation rail), `frontend/src/components/AppSidebar.tsx`, `frontend/src/components/CompanySwitcher.tsx`, `frontend/src/components/FilialSelector.tsx`.

**`frontend/src/components/ui/`:**
- Purpose: shadcn/ui-style Radix primitives (button, dialog, table, form, …).
- Contains: 40+ `.tsx` files, one per primitive. Imported via `@/components/ui/<name>`.
- Key files: `frontend/src/components/ui/button.tsx`, `frontend/src/components/ui/table.tsx`, `frontend/src/components/ui/dialog.tsx`.

**`frontend/src/contexts/`:**
- Purpose: Global React state.
- Contains: `AuthContext.tsx` (user, JWT, company, fetch interceptor), `FilialContext.tsx` (selected branches/CNPJs).

**`frontend/src/lib/`:**
- Purpose: Pure helpers, not React.
- Contains: `utils.ts` (`cn`, `formatCurrency`), `utils.test.ts`, `navigation.ts` (module map), `exportToExcel.ts` (XLSX writer), `formatFilial.ts`, `logger.ts`.

**`frontend/src/hooks/`:**
- Purpose: Reusable React hooks.
- Contains: Only `use-mobile.tsx` today.

**`erp-bridge-aws/`:**
- Purpose: Python ERP daemon source + per-environment configs + Dockerfile.
- Contains: `bridge.py` (single-file daemon), `Dockerfile`, `config-apu02.yaml`, `config-apu04.yaml`.
- Generated: SQLite tracker DB (`tracker-<config>.db`) and `logs-<config>/` are created at runtime, NOT committed.

**`installer/`:**
- Purpose: Self-contained deployment bundles for customers.
- Contains: `install.sh`, `update.sh`, top-level `docker-compose.yml`, `README.md`, `QUESTIONARIO_CLIENTE.md`, plus subfolders `aws-bridge/` and `fcxlabs/`.
- Generated: No.
- Committed: Yes.

**`scripts/`:**
- Purpose: Operational shell scripts and reference CSVs.
- Contains: Deploy/backup scripts, MV health check, CFOP seed CSV.

**`scripts_oracle/`:**
- Purpose: Oracle SQL templates referenced by the ERP Bridge for reading source data.
- Contains: 6 `.sql` files (NF-e generation, NFS-e per municipality, CT-e/NF-e import staging).

**`docs/`:**
- Purpose: Long-form project documentation referenced by `README.md`.
- Contains: Subfolder `docs/` (one level deep).

**`_bmad/` and `_bmad-output/`:**
- Purpose: BMAD agent framework / workflow tooling. Vendor tooling, not runtime code.
- Generated: `_bmad-output/` is generated by BMAD agents.
- Committed: Yes (intentionally tracked).

**`.planning/codebase/`:**
- Purpose: GSD codebase maps (this analysis lives here).
- Contains: `ARCHITECTURE.md`, `STRUCTURE.md` (and other focus docs when produced).

## Key File Locations

**Entry Points:**
- `backend/main.go`: Go API entry — routes, middleware wiring, migration runner, graceful shutdown.
- `frontend/src/main.tsx`: React DOM root mount.
- `frontend/src/App.tsx`: Top-level component, route table, providers, layout shell.
- `erp-bridge-aws/bridge.py`: Python daemon entry (`--daemon` mode for Docker).

**Configuration:**
- `.env.example`: Documented env template (DB, JWT, SMTP, Z.AI, APP_MODULE, VITE_APP_MODULE).
- `.env.production`, `.env.hostinger`: Environment-specific overrides.
- `coolify-env-template.txt`: Template for Coolify deployment platform.
- `docker-compose.yml`: Dev compose (builds from source).
- `docker-compose.prod.yml`: Production compose (uses `Dockerfile.production`).
- `Dockerfile.production`: Multi-stage build — Vite frontend + Go backend → single Alpine image.
- `frontend/nginx.conf`: Reverse proxy config baked into web container; `/api/*` → `api:8084`.
- `frontend/vite.config.ts`: Dev server proxy `/api` → `VITE_API_TARGET` (default `http://localhost:8081`).
- `frontend/tailwind.config.js`, `frontend/postcss.config.js`: Styling pipeline.
- `frontend/tsconfig.json` + `tsconfig.app.json` + `tsconfig.node.json`: TypeScript projects.
- `erp-bridge-aws/config-apu04.yaml`: Bridge runtime config for THIS system.

**Core Logic:**
- `backend/main.go`: Route registry — single source of truth for backend endpoints.
- `backend/worker/worker.go`: SPED ingestion + materialized view refresh trigger.
- `backend/handlers/auth.go`: JWT issuance, AuthMiddleware, role gating, `GetEffectiveCompanyID`.
- `backend/handlers/erp_bridge.go` + `erp_bridge_batch.go` + `erp_bridge_parceiros.go`: ERP integration endpoints.
- `backend/handlers/middleware.go`: SecurityMiddleware (CORS + security headers + rate limiter).
- `backend/services/crypto.go`: AES-GCM encryption for ERP credentials at rest.
- `frontend/src/contexts/AuthContext.tsx`: JWT lifecycle + global fetch interceptor.
- `frontend/src/lib/navigation.ts`: Module/tab navigation map.

**Testing:**
- `frontend/src/lib/utils.test.ts`: Only test file currently in the repo.
- `tests/`: Top-level tests directory — currently sparse.

## Naming Conventions

**Backend Go files:**
- `snake_case.go` for source files: `erp_bridge_batch.go`, `nfe_entradas.go`, `creditos_perdidos.go`.
- One Go file per feature/handler group.
- Disabled migration files use `.sql.disabled` suffix.

**Backend Go identifiers:**
- Exported handlers: `PascalCase` ending in `Handler`, e.g. `UploadHandler`, `ERPBridgeBatchImportHandler`.
- Handler factories: `func XHandler(db *sql.DB) http.HandlerFunc`.
- Internal helpers: `camelCase`, e.g. `erpBridgeGetCompany`, `getJWTSecret`.

**Frontend files:**
- Pages and components: `PascalCase.tsx`, e.g. `Mercadorias.tsx`, `AppRail.tsx`.
- shadcn primitives in `components/ui/`: `kebab-case.tsx`, e.g. `alert-dialog.tsx`, `dropdown-menu.tsx`.
- Hooks: `use-name.tsx` (kebab-case with `use-` prefix), e.g. `use-mobile.tsx`.
- Helpers in `lib/`: `camelCase.ts`, e.g. `exportToExcel.ts`, `formatFilial.ts`.
- Contexts: `XContext.tsx`, e.g. `AuthContext.tsx`, `FilialContext.tsx`.

**Frontend identifiers:**
- Components: `PascalCase`.
- Hooks: `useCamelCase`.
- Variables / props: `camelCase`.
- Types / interfaces: `PascalCase`.

**Migrations:**
- `NNN_snake_case_description.sql` — three-digit zero-padded numeric prefix is mandatory.
- Disabled: append `.disabled`.

**Routes (URLs):**
- All API routes prefixed `/api/`.
- Resource segments use `kebab-case` or `snake_case` mixed: `/api/erp-bridge/runs`, `/api/nfe-entradas`, `/api/auth/forgot-password`, `/api/config/forn-simples`.
- Frontend routes use Portuguese kebab/word: `/importar-efd`, `/operacoes/simples`, `/config/aliquotas`.

**Database:**
- Tables: `snake_case` plural (`users`, `companies`, `import_jobs`, `erp_bridge_runs`, `nfe_entradas`).
- Materialized views: `mv_<purpose>` (`mv_mercadorias_agregada`, `mv_operacoes_simples`, `mv_compras_fornecedores`).
- Views: `vw_<purpose>` (`vw_parceiros`, `vw_nfe_entradas_impostos`).
- Columns: `snake_case` (`company_id`, `created_at`, `mes_ano`).

## Where to Add New Code

**New backend HTTP endpoint:**
1. Create handler file under `backend/handlers/<feature>.go` exposing `func XHandler(db *sql.DB) http.HandlerFunc`.
2. Internally `switch r.Method` if multiple verbs serve the same URL (do NOT register multiple URL handlers in `main.go`).
3. Register the route in `backend/main.go` using `withAuth(...)` (JWT-protected) or `withDB(...)` (no auth) — see `backend/main.go:345`.
4. If admin-only, pass `"admin"` as the second arg to `withAuth`.
5. If gated by module, place the registration inside the `if appModule != "simulador"` block (`backend/main.go:520`).

**New backend background processing:**
- Add to `backend/worker/` and call from `worker.StartWorker` in `backend/worker/worker.go:21`. Worker is started from `onDBConnected` (`backend/main.go:223`) — add startup wiring there if a brand-new pool is needed.

**New cross-cutting integration:**
- Add to `backend/services/<integration>.go` (mirror `services/ai.go` shape). Inject into handlers as a function/struct argument, NOT via globals.

**New schema change:**
- Create `backend/migrations/NNN_description.sql` using the next sequential number.
- Use `IF NOT EXISTS` / `ON CONFLICT DO NOTHING` for idempotency — migrations re-run on every restart and tolerate "already exists" errors silently (`backend/main.go:208`).
- For materialized views, ensure indexes for `REFRESH ... CONCURRENTLY` (a UNIQUE index is required by PostgreSQL).

**New aggregation report:**
- Prefer adding/extending a materialized view (`mv_*`) over computing on the fly. Refresh logic lives in `backend/worker/worker.go:218`.
- Wire a new `/api/reports/<name>` handler that simply queries the MV.

**New frontend page:**
1. Create `frontend/src/pages/<Name>.tsx` (PascalCase).
2. Add a `<Route>` inside `<AppLayout>` in `frontend/src/App.tsx` (`frontend/src/App.tsx:138`).
3. Wrap in `<AdminRoute>` if admin-only.
4. Add a tab entry in the appropriate module of `frontend/src/lib/navigation.ts` (`simulador` / `notas` / `config`).
5. If the URL prefix doesn't match an existing module, extend `getActiveModule(pathname)` in the same file.

**New frontend component:**
- Layout/shared (uses app context): `frontend/src/components/<Name>.tsx`.
- Reusable primitive (no business logic): `frontend/src/components/ui/<kebab-name>.tsx`.

**New shared frontend helper:**
- Pure functions: `frontend/src/lib/<camelCase>.ts`.
- Reusable hooks: `frontend/src/hooks/use-<kebab>.tsx`.

**New ERP Bridge integration:**
- Bridge logic goes in `erp-bridge-aws/bridge.py` (single-file by design).
- New per-environment settings: add fields to `erp-bridge-aws/config-apu04.yaml` (and/or `config-apu02.yaml`).
- Backend-side ingestion: extend `backend/handlers/erp_bridge_batch.go` if new document types or fields are pushed.

**New environment variable:**
- Document in `.env.example` first.
- Add to `docker-compose.yml` AND `docker-compose.prod.yml` (and `installer/fcxlabs/docker-compose.yml` if customer-facing).
- Read in Go via `os.Getenv("VAR")` — there is no central config struct.

**New diagnostic/admin script:**
- Go: `backend/tools/<name>.go` (run with `go run`, NOT linked into the main binary).
- Shell: `scripts/<name>.sh`.
- Oracle SQL: `scripts_oracle/<name>.sql`.

## Special Directories

**`backend/vendor/`:**
- Purpose: Vendored Go dependencies for offline/reproducible builds.
- Generated: Yes (`go mod vendor`).
- Committed: Yes.

**`frontend/node_modules/`:**
- Purpose: Vite/React deps.
- Generated: Yes (`npm install`).
- Committed: No (in `.gitignore`).

**`frontend/dist/`:**
- Purpose: Vite build output, copied into the production image's `./static` (see `Dockerfile.production:46`).
- Generated: Yes (`npm run build`).
- Committed: No.

**`uploads/` (runtime, container path `/root/uploads` → `/app/uploads` in prod):**
- Purpose: Temporary SPED file storage between upload and worker pickup.
- Generated: At runtime by the API.
- Committed: No.
- Cleanup: Worker deletes files after success/failure (`backend/worker/worker.go:170`); orphan files are GC'd at boot (`backend/worker/worker.go:43`).

**`_bmad/` and `_bmad-output/`:**
- Purpose: BMAD agent workflow tooling and its generated artifacts.
- Generated: `_bmad-output/` is generated by BMAD; `_bmad/` is checked-in tooling.
- Committed: Yes (both).

**`.planning/`:**
- Purpose: GSD planning workspace.
- Generated: Yes (by GSD commands like `/gsd-map-codebase`).
- Committed: Optional — depends on team policy.

**`.claude/`:**
- Purpose: Claude Code project state.
- Generated: Yes.
- Committed: Selectively.

---

*Structure analysis: 2026-05-08*
