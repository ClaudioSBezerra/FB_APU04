# Codebase Concerns

**Analysis Date:** 2026-05-08

## Tech Debt

**Stale module name (`fb_apu01`) in APU04 codebase:**
- Issue: Go module is still declared as `module fb_apu01` even though the project is APU04. Every internal import uses `fb_apu01/handlers`, `fb_apu01/services`, `fb_apu01/worker`. Renaming was avoided to dodge a refactor, but it leaves cross-module confusion and historic clues (e.g., `BackendVersion = "1.0.0"` next to `Service: "FB_APU04 Fiscal Engine"`).
- Files: `backend/go.mod` (line 1), `backend/main.go:19-20`, `backend/handlers/auth.go:15`, `backend/handlers/ai_query.go:11`, `backend/handlers/ai_reports.go:13`, `backend/handlers/rfb_apuracao.go:13`, `backend/worker/ai_integration.go:12`
- Impact: Engineers reading the code believe they are in APU01; copy-paste lineage of files between modules (APU01 → APU02 → APU04) is invisible. Was the root cause of `Dockerfile.production` shipping an APU01 binary until 2026-05-07.
- Fix approach: Rename module to `fb_apu04`, run `gofmt`/`goimports` across the tree, update all `fb_apu01/...` imports. Coordinate with any sibling repos that may reference this module path.

**Backup file `auth.go.bak` committed to handlers directory:**
- Issue: `backend/handlers/auth.go.bak` (21 KB, dated 2026-02-11) lives in the same package as live handlers.
- Files: `backend/handlers/auth.go.bak`
- Impact: Confuses code search; `.gitignore` does NOT exclude `.bak` in `handlers/`. Risk of someone editing the wrong file.
- Fix approach: Delete the file; rely on git history. Add `*.go.bak` to `.gitignore`.

**Stale `backend/backend.log` committed to repo:**
- Issue: `backend/backend.log` is checked in and contains lines from FB_APU01 ("FB_APU01 Fiscal Engine (Go) starting on port 8082" / "FB_APU01 BACKEND - 5.1.0"). Last touched 2026-02-18.
- Files: `backend/backend.log`
- Impact: Misleading provenance signal; logs should never be in version control. `.gitignore` lists `*.log` but this file was committed before the rule.
- Fix approach: `git rm` the file and ensure `.gitignore` is enforced.

**Disabled migration files left in migration directory:**
- Issue: `backend/migrations/000_reset_db.sql.disabled` and `037_delete_gilson_user.sql.disabled` sit alongside active migrations. The "disabled" extension is a convention only — easy to break by renaming back accidentally. The `000_reset_db.sql.disabled` is a destructive `DROP TABLE ... CASCADE` script.
- Files: `backend/migrations/000_reset_db.sql.disabled`, `backend/migrations/037_delete_gilson_user.sql.disabled`
- Impact: A `mv 000_reset_db.sql.disabled 000_reset_db.sql` away from wiping production schema on next boot.
- Fix approach: Move disabled migrations to `backend/migrations/_archive/` (outside the `*.sql` glob in `main.go:126`) or delete them outright.

**Cross-module migration "alignment" hacks:**
- Issue: Migrations 066–070 explicitly re-add columns the APU02 schema dropped/changed, because APU04 was historically pointed at the APU02 database. Comments say things like "Necessário quando APU04 aponta para o banco do APU02". After the 2026-05-07 split, this should not happen anymore but the comments and idempotent ALTERs remain.
- Files: `backend/migrations/066_align_with_apu02.sql`, `backend/migrations/067_add_ibs_cbs_to_nfe_tables.sql`, `backend/migrations/068_create_vw_parceiros.sql`, `backend/migrations/070_add_partilha_to_nfe_tables.sql`
- Impact: Schema-drift between APU02 and APU04 is now invisible to engineers; future migrations risk re-introducing dual-database assumptions.
- Fix approach: After 1–2 successful production cycles on the isolated `fiscal_apu04_db`, add a README in `backend/migrations/` clarifying that APU02-alignment migrations are historic only.

**Hardcoded fallback DB strings in Go tools:**
- Issue: Every script in `backend/tools/` hardcodes `postgres://postgres:postgres@localhost:5432/fiscal_apu04_db?sslmode=disable` (or, in `verify_data.go`, the wrong DB name `fb_apu01`). Just fixed the `fiscal_db` → `fiscal_apu04_db` naming on 2026-05-07, but the pattern remains: every new tool requires a manual edit.
- Files: `backend/tools/debug_detailed.go:17`, `backend/tools/debug_query.go:17`, `backend/tools/debug_stats.go:16`, `backend/tools/debug_gilson.go:26`, `backend/tools/verify_data.go:14` (still references `fb_apu01`)
- Impact: Easy to run a debug script against the wrong DB. `verify_data.go` will silently connect to `fb_apu01` (which may not exist on APU04 environments) and fail or — worse — query the wrong DB if both exist locally.
- Fix approach: Centralize in a `backend/tools/common/db.go` helper that reads `DATABASE_URL` and panics if unset (no silent fallback). Fix `verify_data.go` immediately.

**`debug_gilson.go` lacks `//go:build scripts` tag:**
- Issue: `backend/tools/debug_gilson.go` declares `package main` but has no build constraint. The other four tools use `//go:build scripts`.
- Files: `backend/tools/debug_gilson.go:1`
- Impact: When the production binary is built, Go sees two `package main` declarations in different directories. The `Dockerfile.production` only compiles `main.go` from `backend/`, so this *currently* doesn't break the build, but `go build ./...` from `backend/` fails because of the conflict.
- Fix approach: Add `//go:build scripts` as line 1.

**Multiple env templates with overlapping/conflicting values:**
- Issue: Five env files coexist: `.env`, `.env.example`, `.env.hostinger`, `.env.production`, `installer/.env.template`, `coolify-env-template.txt`. Some had APU01 references until 2026-05-07. Defaults vary: `DATABASE_URL` uses `sslmode=disable` in some, `sslmode=require` in others. `JWT_SECRET` placeholders look like real secrets ("super-secure-jwt-secret-for-production-change-me-2026").
- Files: `.env`, `.env.example`, `.env.hostinger`, `.env.production`, `installer/.env.template`, `coolify-env-template.txt`
- Impact: High risk of misconfig in deploys. The 2026-05-07 incident traces partly to env-file confusion (which DB to point at).
- Fix approach: Keep exactly TWO env templates: `.env.example` (dev, committed) and `.env.production.example` (prod, committed). Delete the rest. Add a CI check that fails the build if either drifts from a fixed schema of variable names.

**`Dockerfile.production` recently shipped APU01 binary:**
- Issue: Until 2026-05-07 (commit 947de42), the binary was built as `fb_apu01-api` in this repo. Now corrected to `fb_apu04-api`, but the file still has copy-paste tells: line 1 says `FB_APU04` but `LABEL maintainer="FB_APU04 Team"` and `COPY --from=backend-builder /app/backend/fb_apu04-api .` are recent edits, not a clean rewrite.
- Files: `Dockerfile.production:31`, `Dockerfile.production:45`, `Dockerfile.production:61`
- Impact: Indicates the Docker tooling was forked from APU01 without sanitization. Other forked artifacts (compose files, scripts) likely have similar drift.
- Fix approach: Audit all `installer/`, `scripts/`, and root Dockerfiles for stale APU01/APU02 strings. Add a CI grep gate: `! grep -r "fb_apu0[12]\|fiscal_db\b" Dockerfile* docker-compose* installer/ scripts/`.

## Known Bugs

**ResetDatabaseHandler destroys ALL imported data without a confirmation gate:**
- Symptoms: A single DELETE request to `/api/admin/reset-db` runs `TRUNCATE TABLE import_jobs CASCADE` plus `TRUNCATE` on `nfe_entradas`, `nfe_saidas`, `cte_entradas`, `parceiros`, `erp_bridge_run_items` and `DELETE` from `filial_apelidos` and `erp_bridge_runs`. No confirmation token, no dry-run, no soft-delete, no backup. On 2026-05-07 this destroyed 4 months of production data when APU04 was misconfigured to point at the APU02 database.
- Files: `backend/handlers/admin.go:217-308`
- Trigger: HTTP `DELETE /api/admin/reset-db` with a valid admin JWT. The frontend (`frontend/src/pages/ImportarEFD.tsx:476`) shows a `window.confirm` text-box, but server-side has zero protection beyond `role == "admin"`.
- Workaround: None. The endpoint is auth-gated only by role.
- Fix approach (priority CRITICAL):
  1. Require a confirmation token in the request body (e.g., `{ "confirmation": "RESET-<uuid>-<today>" }`) generated by a separate `POST /api/admin/reset-db/prepare` endpoint that expires in 5 minutes.
  2. Take a `pg_dump` of `import_jobs`, `nfe_entradas`, `nfe_saidas`, `cte_entradas`, `parceiros` to `/app/backups/reset-<timestamp>.sql` BEFORE truncating; refuse to truncate if backup fails.
  3. Switch from `TRUNCATE` to a soft-delete pattern (`UPDATE ... SET deleted_at = now()`), and let a separate scheduled job hard-delete after N days.
  4. Add an audit row in a `destructive_actions` table with `user_id`, `request_id`, `tables_affected`, `rows_before`.
  5. Refuse to run unless the connected DB name matches a hardcoded allow-list set at startup (`ALLOWED_DESTRUCTIVE_DBS=fiscal_apu04_db`).

**ERP Bridge Oracle SAP S/4HANA connections drop with `DPY-4011` and the run aborts:**
- Symptoms: Production logs show `DPY-4011: the database or network closed the connection` mid-import. The bridge currently has a single `oracledb.connect(...)` call wrapped in a try/except, but if the connection drops mid-`cur.execute()` or mid-fetchall, the whole run fails and is restarted manually. `expire_time=2` is set for keepalive but doesn't auto-reconnect a dropped session.
- Files: `erp-bridge-aws/bridge.py:627-640` (SAP), `erp-bridge-aws/bridge.py:813-822` (Oracle XML legacy), `erp-bridge-aws/bridge.py:1110-1125` (daemon parceiros)
- Trigger: Long-running queries on Oracle FCCORP, especially during peak hours or when a network appliance kills idle TCP sessions.
- Workaround: Manually re-trigger the run from the UI or scheduler.
- Fix approach: Wrap Oracle calls in a `@retry(retries=3, backoff=exp)` decorator that detects `DPY-4011`, `DPY-4024`, `ORA-03114`, `ORA-03135` and rebuilds the connection. For SAP batch processing in `processar_sap`, catch within the per-batch loop (`for i in range(0, len(documents), BATCH_SIZE)`) so partial progress is preserved. Persist a per-document `processed_at` watermark in `tracker.db` instead of only at run-end.

**Schema-migration self-healing logic is destructive in legacy DBs:**
- Symptoms: `backend/main.go:142-180` inspects `schema_migrations` and, if the `filename` column has the wrong type, executes `DROP TABLE schema_migrations` then recreates it — losing the entire migration history. Comment says "old integer data is not useful".
- Files: `backend/main.go:160-170`
- Trigger: Pointing a fresh build at an old DB whose `schema_migrations.filename` column is `integer`.
- Workaround: Manually fix the column type before deploying.
- Fix approach: Use `ALTER TABLE schema_migrations ALTER COLUMN filename TYPE VARCHAR(255) USING filename::VARCHAR` instead of DROP. Add a precondition check that aborts startup if any unexpected schema mismatch is found, instead of silently destroying history.

## Security Considerations

**Live secrets committed in `.env` (gitignored, but locally present):**
- Risk: The local `.env` (line 25, 32, 35) contains real-looking values for `SMTP_PASSWORD`, `ZAI_API_KEY`, and `JWT_SECRET`. `.gitignore` excludes `.env`, so it should not be in the repo, but `git log -- .env` should be audited to confirm it was never committed in the past.
- Files: `.env` (NOT committed; existence noted only)
- Current mitigation: `.gitignore` lists `.env`, `backend/.env`. `.dockerignore` lists `.env`.
- Recommendations:
  1. Run `git log --all -- .env` and `git log --all --diff-filter=A` to confirm secrets never landed in history. If they did, rotate `JWT_SECRET`, `SMTP_PASSWORD`, `ZAI_API_KEY` immediately.
  2. Move all production secrets to AWS Secrets Manager or HashiCorp Vault; have the API container fetch them at startup.
  3. Add `pre-commit` hook with `gitleaks` to block future leaks.

**Bridge `config.yaml` stores plaintext Oracle credentials (server-side):**
- Risk: On the AWS host, `/opt/apps/fbtax/erp-bridge/config.yaml` (APU02) and `/opt/apps/fbtax/bridge-apu04/config.yaml` (APU04) hold Oracle passwords (user `fcosta` + FCCORP creds — values redacted) plus FBTax `password` and `api_key` in plaintext YAML. The `bridge.py` code at lines 1218–1252 *can* fetch credentials from the FBTax API via `api_key`, which is the secure path, but the file still contains static fallbacks.
- Files: `erp-bridge-aws/config-apu04.yaml` (gitignored example), `installer/aws-bridge/config.yaml.example:13,25`, AWS hosts: `/opt/apps/fbtax/erp-bridge/config.yaml`, `/opt/apps/fbtax/bridge-apu04/config.yaml`
- Current mitigation: `.gitignore` excludes `erp-bridge-aws/config*.yaml` so they don't reach Git. File mode on AWS not verified.
- Recommendations:
  1. Force the bridge to refuse running if `config.yaml` contains Oracle/FBTax password fields when `api_key` is set — i.e., make API-fetched credentials the only path on production.
  2. Set `chmod 0600` on `config.yaml` and a dedicated UNIX user.
  3. Long-term: store creds in AWS Secrets Manager and inject via env vars at container start (rebuild the `Dockerfile` in `erp-bridge-aws/` to read env, not file).

**Reset endpoints lack CSRF protection:**
- Risk: `/api/admin/reset-db`, `/api/company/reset-data`, `/api/admin/refresh-views` rely on bearer JWT only. If a user has a valid JWT and visits a malicious page, a `<form method=DELETE>` (or fetch from same-origin if XSS exists) could trigger destructive operations. CSP allows `'unsafe-inline'` for scripts (`backend/handlers/middleware.go:73`).
- Files: `backend/handlers/middleware.go:73` (CSP), `backend/handlers/admin.go:217`, `backend/main.go:435-437`
- Current mitigation: Cookie-based auth uses `COOKIE_SECURE=true`; CORS allowlist is enforced. No double-submit token, no `SameSite` declaration visible in middleware.
- Recommendations:
  1. Require a custom header `X-Confirm-Reset: <token>` for all destructive endpoints.
  2. Tighten CSP — drop `'unsafe-inline'` for `script-src` and migrate frontend to nonce-based scripts.
  3. Verify cookies are issued with `SameSite=Strict` for admin sessions.

**`fiscal_apu04_db` admin user reuses default `postgres` superuser:**
- Risk: All env templates default to `DB_USER=postgres` with `DB_PASSWORD=postgres` (in `.env`) or placeholder `CHANGE_ME_*` (in `.env.production`). The application connects with the superuser; there is no separate `app_user` with limited grants. If the API binary is compromised, attacker has full DDL access.
- Files: `.env:8`, `.env.example:2`, `.env.production:20`, `coolify-env-template.txt`, `docker-compose.prod.yml:70-71`
- Current mitigation: `sslmode=require` is set in `.env.production` line 23. DB is on a private docker network.
- Recommendations:
  1. Create `fiscal_app` PG role with only `SELECT, INSERT, UPDATE, DELETE` on the application tables. Run migrations as `postgres`, app as `fiscal_app`.
  2. Drop `postgres` superuser permissions from the runtime path.

**No automated backup before destructive operations:**
- Risk: `ResetDatabaseHandler` and `ResetCompanyDataHandler` do `TRUNCATE`/`DELETE` without first calling `pg_dump`. If admin clicks reset by mistake, no rollback path other than manual restore from a stale daily backup (if it exists). 2026-05-07 incident is the canonical example.
- Files: `backend/handlers/admin.go:217-308`, `backend/handlers/admin.go:29-144`, `scripts/backup_production.sh` (separate, scheduled)
- Current mitigation: `scripts/backup_production.sh` exists for scheduled backups, but is not invoked from the destructive handlers.
- Recommendations:
  1. From `ResetDatabaseHandler`, shell out to `pg_dump` (writing to `/app/backups/reset-<ts>.sql`) and require success before truncating.
  2. Add a startup health-check that fails if `/app/backups/` is not a writable, dedicated volume.
  3. Document a 5-minute restore SOP and exercise it quarterly.

**`auth.go.bak` may contain old hashed credentials or token logic:**
- Risk: Old auth code preserved as `.bak` may contain weaker JWT secrets or password-hashing parameters that diverge from the current implementation. Disclosure (e.g., a `cat *` in a static file route) could leak design details.
- Files: `backend/handlers/auth.go.bak`
- Current mitigation: Backend doesn't serve handler source; risk is theoretical unless an attacker reads the container filesystem.
- Recommendations: Delete the file.

## Performance Bottlenecks

**Materialized views refreshed inline in admin handler (no concurrency control):**
- Problem: `ResetDatabaseHandler` (`backend/handlers/admin.go:279-300`) refreshes three MVs synchronously (NOT concurrently) right after the truncate transaction commits. `ResetCompanyDataHandler` does the same in a goroutine (line 114) using `REFRESH MATERIALIZED VIEW CONCURRENTLY` with a non-concurrent fallback. `RefreshViewsHandler` (line 147) and the worker (`backend/worker/worker.go:219+`) also refresh the same MVs.
- Files: `backend/handlers/admin.go:279-300`, `backend/handlers/admin.go:114-136`, `backend/handlers/admin.go:165-201`, `backend/worker/worker.go:219+`, `backend/main.go:229`
- Cause: No deduplication or queue. Three places can call `REFRESH MATERIALIZED VIEW` simultaneously. Non-concurrent refresh takes an `ACCESS EXCLUSIVE` lock, blocking all readers; if it overlaps an import, the import stalls. The reset path uses *non-concurrent* refresh on the assumption "the view is empty", but if a parallel import already started, that's wrong.
- Improvement path:
  1. Replace direct `db.Exec("REFRESH MATERIALIZED VIEW ...")` calls with a publisher to a Redis-backed queue (`mv_refresh_jobs`). A single worker consumes the queue and refreshes each MV at most once per N seconds.
  2. Always use `CONCURRENTLY`; never fall back to non-concurrent unless an explicit operator command says so.
  3. Add a lock check (`pg_try_advisory_lock`) so two worker instances cannot refresh the same MV concurrently.

**Goroutine in `ResetCompanyDataHandler` runs without context/cancellation:**
- Problem: `backend/handlers/admin.go:114` spawns `go func() { ... db.Exec(...) }()` without a `context.Context`. If the server shuts down mid-refresh, the goroutine dies with the connection but the client received `200 OK` already.
- Files: `backend/handlers/admin.go:114-136`
- Cause: Fire-and-forget pattern.
- Improvement path: Use a worker pool with `context.Background()` derived from the server's main context, and persist the refresh request to a DB queue table so it survives restarts.

**Schema migrations run on every backend startup:**
- Problem: `backend/main.go:185-220` reads every `*.sql` file in `migrations/`, queries `schema_migrations` once per file, and re-executes any not present. With 70+ migrations this is many round-trips at boot.
- Files: `backend/main.go:117-220`
- Cause: Naive per-file `EXISTS` check.
- Improvement path: Single query `SELECT filename FROM schema_migrations` into a Go set; iterate files in memory. Negligible perf gain in practice but cleaner code.

**Bridge SAP query has no pagination:**
- Problem: `processar_sap` (`erp-bridge-aws/bridge.py:642-790`) does `cur.execute(SAP_QUERY); for raw in cur: rows.append(...)` then converts the entire result set to a Python list before sending. For large historical periods, this can OOM the bridge process and tie up the Oracle session for many minutes.
- Files: `erp-bridge-aws/bridge.py:646-654`
- Cause: Whole-result fetch.
- Improvement path: Stream from cursor in chunks of 1000; send each chunk as a batch and update watermark per-chunk rather than per-run.

## Fragile Areas

**Multi-environment ERP Bridge config selection by string-stem:**
- Files: `erp-bridge-aws/bridge.py:44-65`
- Why fragile: `--config config-apu04.yaml` becomes `tracker-config-apu04.db` and `logs-config-apu04/`. If two tenants use slightly different filenames (e.g., `apu04.yaml` vs `config-apu04.yaml`), tracker isolation silently breaks — the bridge will read/write a tracker matching the stem, which may be the wrong one. There is no validation that the config-derived stem is unique per tenant.
- Safe modification: Always pass `--config` with the canonical name. Never rename a config file without first migrating its tracker DB.
- Test coverage: No tests for the config-stem split. `tests/integration_test.go` exists but doesn't cover Python.

**`AuthMiddleware` allows admin to pass any role check, hard to audit:**
- Files: `backend/handlers/auth.go:253` — `if requiredRole != "" && userRole != requiredRole && userRole != "admin"`
- Why fragile: Implicit "admin overrides everything" behavior is not declared in route registration. Every `withAuth(handler, "")` is open to all authenticated users; `withAuth(handler, "admin")` is admin-only; nothing in between can be expressed (e.g., "admin OR env_admin"). `ResetCompanyDataHandler` therefore has to re-check env-admin role manually inside the handler (lines 56-81).
- Safe modification: Don't change the override semantics without a global audit. Adding a new role tier requires touching every `withAuth` call.
- Test coverage: No unit tests for `AuthMiddleware`.

**Async DB initialization with infinite retry hides config errors:**
- Files: `backend/main.go:62-109`
- Why fragile: If `DATABASE_URL` is wrong, the API process keeps retrying every 5s forever, returning HTTP 503 to all clients. Nothing in the logs distinguishes "DB not ready yet" from "credentials are wrong, will never work". Health endpoint at `/api/health` reports `Database: "unavailable"`.
- Safe modification: Cap retries at 60 (5 minutes) and exit with error so container orchestrator (Coolify) marks the deploy failed, rather than running a permanently broken instance.
- Test coverage: None.

**`ResetCompanyDataHandler` deletes uploaded files from disk based on DB content:**
- Files: `backend/handlers/admin.go:86-100`
- Why fragile: `os.Remove(filepath.Join("uploads", fname))` runs on whatever filename comes from the DB row. There is no path-traversal validation. If `import_jobs.filename` ever contained `../config.yaml`, an admin reset could delete arbitrary files. Currently fine because uploads come from controlled handlers, but the trust boundary is implicit.
- Safe modification: `filepath.Clean` and verify the result is still inside `uploads/` before deleting.
- Test coverage: None.

**`erp_bridge.go` cleans `erp_bridge_runs` older than X days inline:**
- Files: `backend/handlers/erp_bridge.go:291` — `DELETE FROM erp_bridge_runs ...`
- Why fragile: A standard query path triggers a destructive cleanup. If the date filter is wrong, history evaporates.
- Safe modification: Read the surrounding code carefully before touching dates. Move retention cleanup to a scheduled worker.
- Test coverage: None.

## Scaling Limits

**API connection pool is fixed at 25 / 10 / 30 min:**
- Current capacity: `MaxOpenConns=25`, `MaxIdleConns=10`, `ConnMaxLifetime=30min` (`backend/main.go:84-86`).
- Limit: At ~25 concurrent slow handlers (e.g., AI-query, MV refresh, large XML upload), the API will queue and timeout new requests.
- Scaling path: Make pool size driven by env vars (`DB_MAX_OPEN_CONNS`, etc.). Add Prometheus metrics for `db.Stats()` to observe saturation. Move long-running operations (MV refresh, AI report generation) out of HTTP handlers entirely.

**Single Postgres instance, no read replica:**
- Current capacity: One `db` container (`docker-compose.prod.yml:65`), no replication.
- Limit: AI/Reports queries that scan large materialized views compete with import writes. A single 5-minute report query can block other reads.
- Scaling path: Add a read replica and route AI/dashboard queries there. Until then, ensure AI queries always hit MVs (already enforced in `text_to_sql.go`).

**Bridge SQLite tracker grows unbounded:**
- Current capacity: One row per (servidor, tipo, chave) in `enviados` table forever.
- Limit: After ~1M rows, query latency for `ja_enviado` lookups grows. SQLite handles this but each daemon iteration re-opens the file (line 88).
- Scaling path: Add `WHERE enviado_em > now() - interval '180 days'` retention. Index `(servidor, tipo, chave)` is already the PK so lookups are O(log n).

**Frontend bundles all routes into a single chunk:**
- Current capacity: 825 lines `Mercadorias.tsx`, 790 lines `ImportarEFD.tsx`, 686 lines `ERPBridgeConfig.tsx` are direct imports.
- Limit: First-load TTI degrades as features are added.
- Scaling path: Lazy-load route components with `React.lazy(() => import(...))` and code-split per feature.

## Dependencies at Risk

**`oracledb` thin-mode quirks:**
- Risk: `python-oracledb` thin mode (used in `bridge.py:34`) does not support all Oracle network features; the `expire_time=2` parameter is observed but reconnection on `DPY-4011` is not automatic.
- Impact: Production runs fail when SAP Oracle network drops the connection.
- Migration plan: Either switch to thick mode (requires Oracle Instant Client install in the bridge container) for better network resilience, or implement explicit retry logic.

**`go-jwt/v5` raw `claims["role"].(string)` casts:**
- Risk: `backend/handlers/auth.go:191`, `backend/handlers/admin.go:53-54` use unchecked type assertions on JWT claims. A malformed (but signed) token could cause a runtime panic.
- Impact: Could be triggered by an attacker with the JWT secret to crash a request handler.
- Migration plan: Replace `claims["role"].(string)` with `s, ok := claims["role"].(string); if !ok { ... }` everywhere.

**Watchtower auto-pulling `:latest` for the bridge:**
- Risk: `installer/aws-bridge/docker-compose.yml:18-28` runs Watchtower with `WATCHTOWER_POLL_INTERVAL=300` against `ghcr.io/claudiosbezerra/fb_apu04-bridge:latest`. Any push to `:latest` deploys to all customer machines within 5 minutes, with no canary.
- Impact: A bad bridge release reaches every tenant simultaneously.
- Migration plan: Tag releases (`:1.0.0`) and update the customer compose file deliberately. Or pin Watchtower to a versioned tag and use a canary group first.

## Missing Critical Features

**No audit log for destructive admin actions:**
- Problem: `ResetDatabaseHandler`, `ResetCompanyDataHandler`, `DeleteUserHandler`, `ReassignUserHandler` log via `log.Printf` to stdout only. There is no append-only audit table recording who deleted what, when, from which IP.
- Blocks: Compliance, post-incident forensics. The 2026-05-07 incident has no DB-side trail.

**No request-id / correlation-id:**
- Problem: Backend handlers do not generate/log a per-request UUID. Cross-service tracing (frontend → API → bridge → Oracle) is impossible.
- Blocks: Performance debugging, error attribution.

**No structured logging:**
- Problem: `log.Printf("Error refreshing %v", err)` produces unstructured strings. No JSON output, no log levels.
- Blocks: Aggregation in CloudWatch / Loki / ELK.

**No API rate limiting beyond auth:**
- Problem: Only `LoginRL`, `RegisterRL`, `ForgotPasswordRL` exist (`backend/handlers/middleware.go:141-143`). A logged-in user can hammer `/api/ai/query` (which calls Z.AI) without limit.
- Blocks: Cost control on Z.AI; basic abuse prevention.

**No automated tests:**
- Problem: `tests/integration_test.go` exists but is the only Go test file. No unit tests for handlers, no Python tests for the bridge, only one frontend test (`frontend/src/lib/utils.test.ts`).
- Blocks: Confidence to refactor critical paths (auth, reset, MV refresh).

## Test Coverage Gaps

**Destructive admin handlers are completely untested:**
- What's not tested: `ResetDatabaseHandler`, `ResetCompanyDataHandler`, `RefreshViewsHandler`, `DeleteUserHandler`, `ReassignUserHandler`, `PromoteUserHandler`, `CreateUserHandler`.
- Files: `backend/handlers/admin.go`
- Risk: A regression in auth checks (e.g., role typo) silently grants access to non-admins. The 2026-05-07 incident would have been caught by a single test asserting "reset only runs against `fiscal_apu04_db`".
- Priority: HIGH

**ERP Bridge has no Python tests:**
- What's not tested: `processar_sap`, `processar_servidor`, `normalizar_xml`, `executar_importacao`, `run_daemon`, watermark logic, retry logic (which doesn't exist).
- Files: `erp-bridge-aws/bridge.py`
- Risk: Schema changes in `s4i_nfe`, `s4i_nfe_impostos`, `FORN`, `CLIE` (Oracle side) are detected only in production. Any code change is shipped to AWS via `:latest` Docker tag with no automation.
- Priority: HIGH

**Frontend reset flows are untested:**
- What's not tested: `ImportarEFD.tsx:476-530` (reset DB / reset company), `GestaoAmbiente.tsx:289` (delete environment cascade).
- Files: `frontend/src/pages/ImportarEFD.tsx`, `frontend/src/pages/GestaoAmbiente.tsx`, `frontend/src/pages/ApelidosFiliais.tsx`
- Risk: A `window.confirm` dropped by a refactor could silently allow one-click destruction.
- Priority: MEDIUM

**Migration runner has no tests:**
- What's not tested: `onDBConnected` schema-migrations bootstrap, including the destructive `DROP TABLE schema_migrations` branch (`backend/main.go:165`).
- Files: `backend/main.go:111-220`
- Risk: A future migration with unusual SQL could trip the `DROP TABLE` path against a production DB.
- Priority: HIGH

**No integration test for the API ↔ DB ↔ Bridge ↔ Oracle pipeline:**
- What's not tested: End-to-end import flow from Oracle → bridge → API → Postgres → MV.
- Files: `tests/integration_test.go` exists but is minimal.
- Risk: Schema drift, breaking changes in the JSON contract between bridge and API are caught only in production.
- Priority: MEDIUM

---

*Concerns audit: 2026-05-08*
