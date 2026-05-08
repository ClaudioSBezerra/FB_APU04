# Technology Stack

**Analysis Date:** 2026-05-08

FB_APU04 (Simulador Fiscal) is a polyglot, multi-service application made of three deployable units:

1. **Backend API** — Go 1.22 service exposing the `/api/*` REST surface (`backend/main.go`).
2. **Frontend SPA** — React 18 + Vite TypeScript app served by Nginx (`frontend/`).
3. **ERP Bridge** — Standalone Python 3.12 daemon that connects to Oracle ERP databases and pushes documents to the API (`erp-bridge-aws/bridge.py`). Deployed separately on AWS EC2 (bare Python or Docker), not part of the Coolify stack.

## Languages

**Primary:**
- **Go 1.22** — Backend HTTP API and SPED background worker. Module path `fb_apu01` (legacy name retained from FB_APU01 fork). Source: `backend/`.
- **TypeScript 5.2** — Frontend SPA. Source: `frontend/src/`.
- **Python 3.12** — ERP Bridge daemon. Source: `erp-bridge-aws/bridge.py`.

**Secondary:**
- **SQL (PostgreSQL dialect)** — Schema migrations in `backend/migrations/*.sql` (numbered `001_…` through `072_…`). Materialized views (`mv_mercadorias_agregada`, `mv_simples_nacional`, `mv_compras_fornecedores`).
- **Oracle PL/SQL** — Read-only ERP extraction scripts in `scripts_oracle/` (NF-e/NFS-e/CT-e XML generation queries used by the legacy `oracle_xml` bridge mode).
- **Bash** — Operational scripts: `scripts/deploy_production.sh`, `scripts/backup_production.sh`, `scripts/tunnel-prod-db.sh`, `installer/install.sh`, `installer/update.sh`.
- **Dockerfile** — Three Dockerfiles: `backend/Dockerfile`, `frontend/Dockerfile`, `Dockerfile.production` (combined image for Coolify), plus `erp-bridge-aws/Dockerfile`.

## Runtime

**Backend:**
- Go runtime statically compiled: `golang:1.22-alpine` (build) → `alpine:latest` (runtime).
- Build flags: `CGO_ENABLED=0 GOOS=linux -ldflags="-w -s"` (`backend/Dockerfile:19`, `Dockerfile.production:28`).
- Vendored dependencies under `backend/vendor/`; built with `go build -mod=vendor`.
- Listens on port **8084** (`PORT` env, default `8081` for local dev — `backend/main.go:277`).
- Custom `http.Server` with 300 s read/write timeouts and 60 s idle timeout (`backend/main.go:644-651`) to support large EFD/SPED uploads.

**Frontend:**
- Build runtime: `node:18-alpine` (`frontend/Dockerfile:2`, `Dockerfile.production:6`).
- Serve runtime: `nginx:stable-alpine` (`frontend/Dockerfile:12`).
- Dev server: Vite on port 3000 with proxy `/api → http://localhost:8081` (`frontend/vite.config.ts:14-23`).
- Build outputs static assets to `frontend/dist/`; nginx serves from `/usr/share/nginx/html`.

**ERP Bridge:**
- `python:3.12-slim` Docker image (`erp-bridge-aws/Dockerfile:1`) OR bare-Python install on AWS EC2 (per memory note `project_bridge_aws_status.md`: APU02 uses bare Python, APU04 uses Docker).
- Default command: `python bridge.py --daemon`.
- Logging to per-config directory `logs-<config-stem>/` and SQLite watermark tracker `tracker-<config-stem>.db` (`erp-bridge-aws/bridge.py:55-65`).

**Package Managers:**
- **Go modules** — `backend/go.mod`, `backend/go.sum`. Lockfile equivalent committed (vendored).
- **npm** — `frontend/package.json`, `frontend/package-lock.json` (lockfile v3, committed). Uses `npm ci` in production build (`Dockerfile.production:10`).
- **pip** — Bridge dependencies installed inline in `erp-bridge-aws/Dockerfile:5-9`. No `requirements.txt`.

## Frameworks

**Backend (Go):**
- **Standard library `net/http`** — All HTTP routing via `http.HandleFunc` (~80 endpoints registered in `backend/main.go:282-614`). No third-party router.
- **`database/sql`** + **`github.com/lib/pq` v1.11.2** — PostgreSQL driver.
- **`github.com/golang-jwt/jwt/v5` v5.3.1** — JWT token signing/parsing (`backend/handlers/auth.go`).
- **`github.com/joho/godotenv` v1.5.1** — `.env` loading at startup (`backend/main.go:254`).
- **`golang.org/x/crypto` v0.17.0** — bcrypt password hashing.
- **`golang.org/x/text` v0.14.0** — Charset transforms (Latin-1 → UTF-8) for SPED file parsing in `backend/worker/worker.go:17-18`.
- **Built-in `net/smtp`** — Email delivery (`backend/services/email.go:8`). No third-party mailer.
- **Built-in `crypto/aes` + `crypto/cipher`** — AES-256-GCM field encryption (`backend/services/crypto.go:32-44`) for stored credentials.

**Frontend (React/TypeScript):**
- **React 18.3.1** + **react-dom 18.3.1** (`frontend/package.json:50-52`).
- **Vite 5.2** with `@vitejs/plugin-react-swc` 3.5.
- **react-router-dom 6.22** — Client-side routing (`frontend/src/App.tsx:1`).
- **@tanstack/react-query 5.90** — Server-state data fetching (`frontend/src/App.tsx:2`).
- **react-hook-form 7.71** + **@hookform/resolvers 5.2** + **zod 4.3** — Forms and schema validation.
- **Tailwind CSS 3.4** + **tailwindcss-animate** + **tailwind-merge** + **class-variance-authority** + **clsx** — Styling.
- **Radix UI primitives** — 28 `@radix-ui/react-*` packages (Dialog, Dropdown, Tabs, Tooltip, Toast, Popover, Select, etc.) for accessible components.
- **lucide-react 0.363** — Icon set.
- **recharts 3.7** — Charts on Dashboard, Mercadorias, ExecutiveSummary.
- **sonner 2.0** — Toast notifications.
- **xlsx 0.18.5** — Excel export (`frontend/src/lib/exportToExcel.ts`).
- **date-fns 4.1**, **react-day-picker 9.13** — Dates / calendars.
- **cmdk 1.1**, **embla-carousel-react 8.6**, **vaul 1.1**, **input-otp 1.4**, **next-themes 0.4** — UI utility libraries.

**Frontend (Build/Dev):**
- TypeScript 5.2 (`frontend/tsconfig.json`, `frontend/tsconfig.app.json`).
- ESLint via `npm run lint` (no committed config file detected — uses defaults).
- PostCSS 8.4 + Autoprefixer 10.4.

**ERP Bridge (Python):**
- **`oracledb`** — Oracle thin-mode client (no Oracle Instant Client required) — `erp-bridge-aws/bridge.py:34`.
- **`requests`** — HTTPS client to FBTax API.
- **`pyyaml`** — Config parsing (`config-apu04.yaml`, `config-apu02.yaml`).
- **`python-dateutil`** — Date utilities.
- Standard library: `sqlite3` (watermark tracker), `argparse`, `logging`, `zoneinfo`.

**Testing:**
- **No test frameworks detected.** A single sample test exists at `frontend/src/lib/utils.test.ts`. No `*_test.go` in backend, no `tests/` content under bridge. The top-level `tests/` directory is present but empty.

## Key Dependencies

**Critical (Backend):**
- `github.com/lib/pq` — PostgreSQL driver. All persistence flows through it.
- `github.com/golang-jwt/jwt/v5` — Authentication tokens (`HS256`, signed with `JWT_SECRET`).
- `golang.org/x/crypto/bcrypt` — User password hashing.

**Critical (Frontend):**
- `@tanstack/react-query` — Centralized API state.
- `react-router-dom` — All page navigation.
- 28 `@radix-ui/*` packages — Underpin every UI primitive in `frontend/src/components/ui/`.

**Critical (Bridge):**
- `oracledb` — Without it, `bridge.py` exits at startup (`erp-bridge-aws/bridge.py:33-38`).
- `requests` — All FBTax API communication (XML multipart upload, JSON batch import, run reporting).

**Infrastructure:**
- **PostgreSQL 15-alpine** (`docker-compose.yml:64`, `docker-compose.prod.yml:66`).
- **Redis 7-alpine** (production) / `redis:alpine` (dev) — Container present in compose, but **no Go code references Redis** (no `go-redis` import, no `REDIS_ADDR` usage in `backend/`). Currently a reserved/unused dependency.
- **Nginx stable-alpine** — Frontend container; reverse-proxies `/api/` → `http://api:8084` (`frontend/nginx.conf`).
- **Traefik** — Edge router via Coolify labels (`docker-compose.yml:48-55`); routes `simu.fbtax.cloud` and `simu.fcxlabs.com` with Let's Encrypt TLS.

## Configuration

**Environment loading:**
- Backend reads `.env` via `godotenv.Load()` at startup (`backend/main.go:254`); fall through to OS env if no file.
- Bridge reads YAML config selected by `--config <path>` (defaults to `config.yaml`). Tracker DB and log dir are derived per config stem (`erp-bridge-aws/bridge.py:55-65`).

**Env files in repo:**
- `.env` — Local secrets (gitignored at root, but `backend/.env` is committed and contains real credentials — see CONCERNS.md).
- `.env.example` — Template for development.
- `.env.production` — Production reference (placeholders, committed).
- `.env.hostinger` — Hostinger-specific overrides (committed).
- `coolify-env-template.txt` — Variables to paste into Coolify UI.

**Required environment variables (backend):**
- `PORT` (default 8084 in prod, 8081 fallback in code)
- `DATABASE_URL` — Postgres DSN (e.g. `postgres://user:pass@db:5432/fiscal_apu04_db?sslmode=disable|require`)
- `JWT_SECRET` — Must be set in production; `ValidateJWTSecret()` fatals if missing when `APP_MODULE != ""` (`backend/handlers/auth.go:73-79`).
- `ENCRYPTION_KEY` — Optional; falls back to `JWT_SECRET` for AES-256-GCM credential encryption (`backend/services/crypto.go:11-21`). Warning logged if not set separately (`backend/main.go:260-262`).
- `APP_MODULE` — `simulador` (default for FB_APU04), `apuracao`, or `all` — gates which route groups register (`backend/main.go:264-269`).
- `COOKIE_SECURE` — `true` in prod.
- `APP_URL` — Public URL used in email links (default `https://simu.fbtax.cloud`).
- `SMTP_HOST` / `SMTP_PORT` / `SMTP_USER` / `SMTP_PASSWORD` / `SMTP_FROM` — SMTP delivery (`backend/services/email.go:51-80`).
- `ZAI_API_KEY` — Z.AI GLM credential; AI client returns nil and AI endpoints respond `503` when missing (`backend/services/ai.go:65-69`, `backend/handlers/ai_query.go:99`).
- `RFB_API_URL` / `RFB_TOKEN_URL` / `RFB_WEBHOOK_URL` — Receita Federal CBS endpoints (defaults: `https://api.receitafederal.gov.br`, `<base>/token`, `https://fbtax.cloud/api/rfb/webhook`).
- `REDIS_ADDR` — Defined in compose (`redis:6379`) but unused by Go code today.
- `CORS_ORIGINS` — Comma-separated origin list (`https://simu.fbtax.cloud,https://simu.fcxlabs.com`); consumed by `handlers.GetAllowedOrigins()` in `backend/handlers/middleware.go`.
- `VITE_APP_MODULE` — Frontend build arg; baked at `npm run build` (`frontend/Dockerfile:8`).
- `VITE_API_TARGET` — Vite dev-server proxy target (`frontend/vite.config.ts:9`).

**Build Configuration:**
- Backend: `backend/Dockerfile` (vendored build for the dev `docker-compose.yml`) and `Dockerfile.production` (multi-stage Coolify build that bakes the React `dist/` into the Go image's `./static`).
- Frontend: `frontend/Dockerfile` (standalone nginx serving) — used by `docker-compose.yml`. The Coolify production stack bakes both into one image instead.
- Tailwind: `frontend/tailwind.config.js` (HSL CSS-variable theme; design tokens for `pis-cofins`, `ibs-cbs`, `positive`, `negative`).
- TypeScript: `frontend/tsconfig.app.json` — strict mode, bundler module resolution, path alias `@/* → ./src/*`.
- Vite: `frontend/vite.config.ts` — SWC React, port 3000, alias `@`.

## Platform Requirements

**Development:**
- Linux/WSL or macOS host.
- Docker + docker-compose plugin (per `installer/install.sh:34-41`).
- Go 1.22 toolchain (vendor mode — no internet needed at build time).
- Node.js 18+ for frontend (`Dockerfile.production:6` pins `node:18-alpine`).
- Python 3.12 + `oracledb`/`requests`/`pyyaml`/`python-dateutil` for running the bridge locally.
- PostgreSQL 15 (containerized in compose).
- Reachable Oracle ERP servers (TCP 1521) for bridge testing — see `erp-bridge-aws/config-apu04.yaml` for 12 production DSNs (`10.131.x.x`–`10.139.x.x`).

**Production:**
- **Coolify** orchestrator on AWS EC2 (host: `simu.fcxlabs.com` via memory note `project_bridge_aws_status.md`; also `simu.fbtax.cloud` via Traefik label).
- **Traefik** (external network `coolify`) handles TLS termination via Let's Encrypt for both hostnames.
- **GitHub Actions** push image to GHCR on every commit to `main` (`.github/workflows/deploy-production.yml`); Coolify pulls and redeploys.
- **Oracle network reachability** — Bridge runs on a separate AWS EC2 instance (NOT in Coolify) so it can reach internal Oracle DSNs over private IPs.
- **TZ** — Containers set to `America/Sao_Paulo` (`Dockerfile.production:41`).
- **Healthcheck** — `curl -f http://localhost:8084/api/health` every 30 s (`Dockerfile.production:54-55`).

---

*Stack analysis: 2026-05-08*
