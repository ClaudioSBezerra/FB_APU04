# External Integrations

**Analysis Date:** 2026-05-08

## APIs & External Services

**AI / LLM:**
- **Z.AI GLM** — Chat-completions API used for natural-language report generation, executive summaries, daily insights, and Text-to-SQL.
  - Base URL: `https://api.z.ai/api/paas/v4/chat/completions` (`backend/services/ai.go:75`).
  - Models in use:
    - `glm-4.7-flash` (primary, free tier) — `ModelFlash` constant.
    - `glm-4.5-flash` (fallback on HTTP 429 rate limit) — `ModelFlashFallback`.
  - Auth: `Authorization: Bearer ${ZAI_API_KEY}` header. Constructor returns `nil` and AI endpoints respond `503 IA não configurada` if `ZAI_API_KEY` is absent (`backend/services/ai.go:65-69`, `backend/handlers/ai_query.go:99`).
  - Retry policy: up to 3 attempts with exponential back-off in `Generate(...)` (`backend/services/ai.go:160-200`); single attempt with model fallback in `GenerateFast(...)` for synchronous handlers; 3 s back-off retry on transient TLS/dial errors in `GenerateFastRaw(...)`.
  - Consumers: `backend/services/ai.go`, `backend/services/text_to_sql.go`, `backend/handlers/ai_query.go`, `backend/handlers/ai_reports.go`, `backend/worker/ai_integration.go`.
  - Special handling: Flash models return chain-of-thought in `reasoning_content`; helper `extractMarkdownReport` in `backend/services/ai.go:350+` unwraps the final Markdown report.

**Receita Federal do Brasil (RFB) — CBS API:**
- **OAuth2 client-credentials** flow against the official CBS apuração endpoints (`backend/services/rfb.go:75-113`).
  - Default base URL: `https://api.receitafederal.gov.br` (override via `RFB_API_URL`).
  - Token URL: `<base>/token` (override via `RFB_TOKEN_URL`).
  - Path prefix is environment-dependent: `rtc` (production) or `prr-rtc` (`producao_restrita` / beta — `backend/services/rfb.go:66-72`).
- Endpoints used:
  - `POST <base>/<prefix>/apuracao-cbs/v1/<cnpjBase>` — Solicita apuração CBS (`backend/services/rfb.go:127`).
  - `GET <base>/<prefix>/.../tiquete/<id>` — Download de arquivo (`backend/services/rfb.go:174`+).
- Webhook callback: `https://fbtax.cloud/api/rfb/webhook` (override via `RFB_WEBHOOK_URL`) — handled by `handlers.RFBWebhookHandler` (`backend/main.go:549`).
- Conditional registration: RFB routes only register when `APP_MODULE` includes `apuracao` (or `all`). FB_APU04 runs as `simulador`, so RFB endpoints are disabled in this deployment but the code remains shared with FB_APU02.

**ERP Bridge (Oracle ERP → API):**
- Inbound webhook-style endpoints exposed by the Go API for the AWS bridge daemon (`backend/main.go:568-595`):
  - `POST /api/erp-bridge/import/batch` — JSON batch import for SAP S/4HANA mode (auth `X-API-Key`).
  - `POST /api/erp-bridge/parceiros/sync` — Bulk parceiro upsert (auth `X-API-Key`).
  - `POST /api/erp-bridge/heartbeat` — Daemon liveness ping (auth `X-API-Key`).
  - `GET /api/erp-bridge/credentials` — Pulls AES-encrypted Oracle credentials by API key.
  - `GET/PATCH /api/erp-bridge/config` — Daemon config sync.
  - `GET /api/erp-bridge/pending` — Pull pending run requests.
  - `POST/GET /api/erp-bridge/runs` and `/api/erp-bridge/runs/<id>` and `/api/erp-bridge/runs/<id>/items` — Report run progress.
  - `POST /api/erp-bridge/servidores/registrar` — Upsert list of Oracle servers.
- Bridge-side client: `FBTaxClient` in `erp-bridge-aws/bridge.py:326-605`. Two import modes:
  - `oracle_xml` (legacy Totvs/Protheus) — `multipart/form-data` XML upload to `/api/nfe-saidas/upload`, `/api/nfe-entradas/upload`, `/api/cte-entradas/upload` with cookie/JWT session login (`POST /api/auth/login`).
  - `sap_s4hana` — JSON batch via `/api/erp-bridge/import/batch` with `X-API-Key` header. Reads from Oracle views `s4i_nfe` + `s4i_nfe_impostos`.

## Data Storage

**Primary Database:**
- **PostgreSQL 15** — Database name `fiscal_apu04_db` (isolated from FB_APU02's `fiscal_db` since 2026-05-07; see commit `90d1b93`).
  - Connection: `DATABASE_URL=postgres://${DB_USER}:${DB_PASSWORD}@db:5432/fiscal_apu04_db?sslmode=disable|require`.
  - Local dev fallback DSN baked in code: `postgres://postgres:postgres@localhost:5432/fiscal_apu04_db?sslmode=disable` (`backend/main.go:69`).
  - Driver: `github.com/lib/pq` v1.11.2.
  - Pool config: `MaxOpenConns=25`, `MaxIdleConns=10`, `ConnMaxLifetime=30 min` (`backend/main.go:84-86`).
  - Connection retry: infinite loop with 5 s back-off until first successful `Ping` (`backend/main.go:75-107`).
- **Migrations:** auto-applied at startup from `backend/migrations/*.sql` in lexical order. Tracked in `schema_migrations` table; logic auto-creates/repairs the table on legacy schemas (`backend/main.go:130-220`). Latest migration: `072_create_vw_nfe_entradas_impostos.sql`.
- **Materialized views:** `mv_mercadorias_agregada` (refreshed at startup — `backend/main.go:226-235`), `mv_simples_nacional`, `mv_compras_fornecedores`. Manual refresh via `POST /api/admin/refresh-views`.
- **Volumes:** `postgres_data_apu04` (dev compose), `postgres_data_prod` (prod compose).

**ERP Source Databases (read-only):**
- **Oracle Database** (12 production DSNs in `erp-bridge-aws/config-apu04.yaml`) — Filiais Ferreira Costa across NE Brazil:
  - FC - Aracaju (`10.135.1.111:1521/fcaju`), Salvador, Imbiribeira, Tamarineira, Alhandra, João Pessoa, Barris, Cabedelo, Caruaru, Garanhuns, Lauro, Natal.
  - All use the same shared user `fcosta` (credentials in YAML — see CONCERNS.md).
- **Oracle FCCORP** (single DSN) — SAP S/4HANA source for the consolidated `sap_s4hana` mode (`installer/aws-bridge/config.yaml.example:21-25`).
- Connection client: Python `oracledb` thin mode (no Oracle Instant Client needed).
- Tracker: SQLite `tracker-config-apu04.db` keeps watermark per `(servidor, tipo, chave)` to avoid re-sending (`erp-bridge-aws/bridge.py:87-100`).

**Cache:**
- **Redis 7-alpine** (prod) / `redis:alpine` (dev) — Container is provisioned and exposed at `redis:6379` (`docker-compose.yml:84-90`, `docker-compose.prod.yml:85-99`), with `maxmemory 256mb` LRU + AOF in production. **However, the Go backend does NOT import any Redis client today** (no `go-redis`, no `REDIS_ADDR` reads). The container is reserved for future use. See CONCERNS.md.

**File Storage:**
- **Local filesystem** — Uploaded SPED/EFD files are stored in the API container at `/root/uploads` (dev compose, `docker-compose.yml:33`) or `/app/uploads` (production multi-stage image, `Dockerfile.production:49`). Backed by Docker named volume `api_uploads`.
- Worker startup performs orphan-file cleanup of unreferenced uploads (`backend/worker/worker.go:42-60`).
- No object storage (S3, MinIO) is integrated.

## Authentication & Identity

**User Authentication:**
- Custom JWT (HS256) issued by `backend/handlers/auth.go`. Tokens are signed with `JWT_SECRET` and parsed via `github.com/golang-jwt/jwt/v5` (`backend/handlers/auth.go:137-143`).
- Endpoints: `/api/auth/register`, `/api/auth/login`, `/api/auth/logout`, `/api/auth/me`, `/api/auth/refresh`, `/api/auth/forgot-password`, `/api/auth/reset-password`, `/api/auth/change-password` (`backend/main.go:423-430`).
- Authorization header: `Authorization: Bearer <jwt>` parsed by `AuthMiddleware` (`backend/handlers/auth.go:211-242`).
- Tenant scoping via `X-Company-ID` header read by `GetEffectiveCompanyID(...)` in handlers.
- Roles: `admin` enforced by `withAuth(handler, "admin")` wrapper (`backend/main.go:435-442`). Frontend mirror: `AdminRoute` in `frontend/src/App.tsx:53-60`.
- Password hashing: `golang.org/x/crypto/bcrypt` (per `go.mod`).
- Frontend session context: `frontend/src/contexts/AuthContext.tsx`. Tokens stored client-side and attached to all API calls.

**Service-to-Service Auth (ERP Bridge → API):**
- **API Key** — `X-API-Key` header. Generated server-side by `POST /api/erp-bridge/config/generate-api-key` (admin only) and persisted on `erp_bridge_config.api_key` (`backend/main.go:568`, `backend/handlers/erp_bridge.go`).
- Bridge stores it in `config-apu04.yaml` under `fbtax.api_key` and presents it in every request (`erp-bridge-aws/bridge.py:366`).
- The legacy `oracle_xml` mode falls back to email/password login (`POST /api/auth/login`) when `api_key` is empty (`erp-bridge-aws/bridge.py:337-346`).

**Credential Encryption at Rest:**
- AES-256-GCM with a key derived via SHA-256 from `ENCRYPTION_KEY` (or `JWT_SECRET` fallback) — `backend/services/crypto.go:11-21`.
- Used for: RFB credentials (`/api/rfb/credentials`), Oracle credentials served back to the bridge (`/api/erp-bridge/credentials`), bridge `fbtax_password` and `oracle_senha` columns of `erp_bridge_config`.
- Decryption helper falls back to plaintext when ciphertext can't be decoded (handles legacy migration rows) — `backend/services/crypto.go:26-49`.

## Monitoring & Observability

**Health Endpoints:**
- `GET /api/health` (no auth) — Returns service status, DB pool stats, version, and feature set (`backend/main.go:282-318`).
- `Dockerfile.production` healthcheck calls this every 30 s.

**Logging:**
- Backend: Standard library `log` and `fmt.Println` to stdout. Tagged prefixes: `[RFB]`, `[AI]`, `[AI Fast]`, `[AI Raw]`, `[Email Service]`, `Worker:`, `Background:`. Captured by Docker / Coolify.
- Bridge: Python `logging` to per-config dir `logs-config-apu04/bridge_<YYYYMMDD_HHMMSS>.log` plus stdout (`erp-bridge-aws/bridge.py:67-83`). Default level: `DEBUG` to file, `INFO` to screen.
- Frontend: Custom `frontend/src/lib/logger.ts` wrapper.

**Error Tracking:**
- None integrated. No Sentry, Rollbar, Datadog, or similar SDK detected.

**Metrics:**
- None — `coolify-env-template.txt` references `METRICS_ENABLED=true` but no metrics endpoint exists in code.

## CI/CD & Deployment

**Hosting:**
- **AWS EC2** under **Coolify** orchestrator. Two production hostnames routed by Traefik:
  - `simu.fbtax.cloud` — Primary domain.
  - `simu.fcxlabs.com` — Secondary / customer-facing alias used by ERP Bridge `config-apu04.yaml`.
- The ERP Bridge runs on a separate AWS EC2 (NOT in the Coolify stack), as bare Python or Docker, to retain Oracle reachability over the private network. Per memory: APU02 = bare Python at `fctax.fcxlabs.com`, APU04 = Docker at `simu.fcxlabs.com` (status as of 2026-05-07).

**CI Pipeline:**
- **GitHub Actions:**
  - `.github/workflows/deploy-production.yml` — Build & push to GHCR (`ghcr.io/${{ github.repository }}`) on push to `main` and on `v*` tags. Triggers Coolify redeploy.
  - `.github/workflows/deploy-staging.yml` — Staging variant.
- **Container Registry:** GitHub Container Registry (GHCR), authenticated with `GITHUB_TOKEN`.
- **Build:** `docker/buildx` with `docker/setup-buildx-action@v3`, `docker/login-action@v3`, `docker/metadata-action@v5`.

**Deployment Tooling:**
- `scripts/deploy_production.sh` — Manual deploy helper.
- `scripts/backup_production.sh` — DB backup script.
- `scripts/tunnel-prod-db.sh` — SSH tunnel for production DB access.
- `installer/install.sh`, `installer/update.sh` — On-prem installer for self-hosted clients.
- `delete_branches_with_token.sh`, `cleanup_old_branches.sh`, `git_push_helper.sh` — Repo housekeeping.

## Environment Configuration

**Required env vars (production deployment):**

| Variable | Purpose | Source |
|----------|---------|--------|
| `PORT` | API listen port (8084) | Compose |
| `DATABASE_URL` | Postgres DSN | Compose (built from `DB_USER`/`DB_PASSWORD`/`DB_NAME`) |
| `DB_USER`, `DB_PASSWORD`, `DB_NAME` | Postgres init | Compose |
| `JWT_SECRET` | JWT signing (FATAL if unset in prod) | Coolify secret |
| `ENCRYPTION_KEY` | AES-256-GCM key for stored credentials | Coolify secret (recommended; falls back to `JWT_SECRET`) |
| `APP_MODULE` | `simulador` for FB_APU04 | Compose |
| `COOKIE_SECURE` | `true` in prod | Compose |
| `APP_URL` | Public URL in email links | Compose (default `https://simu.fbtax.cloud`) |
| `SMTP_HOST` | `smtp.hostinger.com` | Compose |
| `SMTP_PORT` | `465` (implicit TLS) | Compose |
| `SMTP_USER` | `contato@fortesbezerra.com.br` | Compose |
| `SMTP_PASSWORD` | Hostinger SMTP password | Coolify secret |
| `SMTP_FROM` | Display name + address | Compose |
| `ZAI_API_KEY` | Z.AI GLM key (AI features 503 if missing) | Coolify secret |
| `RFB_API_URL`, `RFB_TOKEN_URL`, `RFB_WEBHOOK_URL` | RFB CBS endpoints (defaults work for prod) | Optional |
| `REDIS_ADDR` | `redis:6379` | Compose (currently unused by Go) |
| `CORS_ORIGINS` | Comma-separated origin list | Compose |
| `VITE_APP_MODULE` | `simu` — frontend build arg | Dockerfile build arg |

**Required env vars (ERP Bridge):**
- Bridge does NOT use process env for runtime config — everything comes from the YAML file passed via `--config`. Secrets (Oracle passwords, FBTax api_key) live inline in `config-apu04.yaml` (gitignored).

**Secrets locations:**
- **Production:** Coolify environment variables UI (per `coolify-env-template.txt`).
- **Local dev:** `backend/.env` (committed in repo today — see CONCERNS.md), root `.env` (gitignored).
- **Bridge:** `erp-bridge-aws/config-apu04.yaml` and `config-apu02.yaml` are gitignored (`*.gitignore` excludes both `config.yaml` and `config-apu0?.yaml`).

**Files present (NOT inspected for secret content):**
- `/home/claudiobezerra/projetos/FB_APU04/.env` — Root local env file.
- `/home/claudiobezerra/projetos/FB_APU04/.env.hostinger` — Hostinger-specific overrides.
- `/home/claudiobezerra/projetos/FB_APU04/backend/.env` — Backend local env file (committed).

## Webhooks & Callbacks

**Incoming (handled by this API):**
- `POST /api/rfb/webhook` (`backend/main.go:549`) — Receita Federal callback for asynchronous CBS apuração results. Handler: `handlers.RFBWebhookHandler`. Active only when `APP_MODULE` includes `apuracao`; **disabled in FB_APU04** (`simulador` module).
- `POST /api/erp-bridge/import/batch` — Bridge daemon webhook (auth `X-API-Key`).
- `POST /api/erp-bridge/parceiros/sync` — Bridge daemon parceiros webhook (auth `X-API-Key`).
- `POST /api/erp-bridge/heartbeat` — Bridge liveness ping (auth `X-API-Key`).

**Outgoing (this API calls out):**
- **Z.AI** — `POST https://api.z.ai/api/paas/v4/chat/completions` (per AI handler invocation).
- **RFB** — `POST <RFB_API_URL>/<prefix>/apuracao-cbs/v1/<cnpjBase>` and `GET <RFB_API_URL>/<prefix>/.../tiquete/<id>` (`apuracao` module only).
- **SMTP/Hostinger** — `smtp.hostinger.com:465` implicit TLS (`crypto/tls`) for password reset, AI report delivery, executive summary email (`backend/services/email.go:97-201`).
- **No outgoing webhook to third parties** other than the above.

## Email / SMTP

- **Provider:** Hostinger SMTP (`smtp.hostinger.com:465`, implicit TLS).
- **From address:** `FBTax Cloud <contato@fortesbezerra.com.br>` (default in `backend/services/email.go:71`).
- **Auth:** PLAIN over implicit TLS (`smtp.PlainAuth` with `crypto/tls.Dial` — `backend/services/email.go:103`).
- **Triggers:**
  - Password reset (`/api/auth/forgot-password`).
  - Account verification.
  - AI report delivery (executive summary, fiscal comparison) — `backend/services/email.go:200-340`.
  - Daily insight emails.
- **Failure mode:** When `SMTP_PASSWORD` is empty, send is skipped and the handler returns `"serviço de e-mail não configurado"` rather than crashing (`backend/services/email.go:135-137`).
- Implementation uses Go standard `net/smtp` only — no third-party mailer library.

---

*Integration audit: 2026-05-08*
