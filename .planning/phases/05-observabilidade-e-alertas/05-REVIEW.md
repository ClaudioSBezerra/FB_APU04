---
phase: 05-observabilidade-e-alertas
reviewed: 2026-05-16T00:00:00Z
depth: standard
files_reviewed: 10
files_reviewed_list:
  - backend/handlers/metrics.go
  - backend/main.go
  - backend/handlers/erp_bridge.go
  - backend/handlers/xml_upload.go
  - backend/handlers/admin.go
  - erp-bridge-aws/bridge.py
  - monitoring/prometheus/prometheus.yml
  - monitoring/prometheus/rules/fiscal.yml
  - monitoring/alertmanager/alertmanager.yml.tpl
  - docker-compose.yml
  - docker-compose.prod.yml
  - installer/aws-bridge/docker-compose.yml
findings:
  critical: 2
  warning: 4
  info: 3
  total: 9
status: issues_found
---

# Code Review: Phase 05 — Observabilidade e Alertas

**Reviewed:** 2026-05-16
**Depth:** Standard
**Files Reviewed:** 12 (10 source + 2 compose variants)
**Status:** Issues Found

## Summary

The observability stack is well-structured and covers the critical use cases. The Go instrumentation is correctly placed, the normalizePath cardinality guard works, and the middleware chain ordering (MetricsMiddleware outside SecurityMiddleware) is correct. Two blockers require immediate attention: (1) the awk ENVIRON[] substitution used to expand SMTP credentials silently corrupts any password containing `&` or `\`, which will cause alertmanager to fail authentication; (2) the `XMLUploadErrorsTotal` counter is never incremented for asynchronous batches (>50 XMLs), producing a systematic blind spot in the `XMLUploadFalha` alert. Four warnings cover a startup false-positive on `BridgeOffline`, an unsecured sslmode in the prod postgres-exporter, a weak Grafana default password, and a semantic mismatch in the batch counter increment granularity.

---

## Critical Issues

### CR-01: awk `gsub` corrupts SMTP credentials containing `&` or `\`

**File:** `docker-compose.yml:146` (identical in `docker-compose.prod.yml:172`)

**Issue:** In awk's `gsub(regex, replacement)`, the character `&` in the replacement string means "the matched text", and `\` introduces escape sequences. The entrypoint awk one-liner substitutes SMTP variables directly from `ENVIRON[]` into the replacement argument of `gsub`. If `SMTP_PASSWORD` (or any other SMTP variable) contains `&`, awk expands it to the literal pattern being replaced (`${SMTP_PASSWORD}`), corrupting the output. A password like `p@ss&word` produces `p@ss${SMTP_PASSWORD}word` in the generated YAML, causing alertmanager SMTP authentication to fail silently. This was confirmed with a live `awk` test.

**Impact:** If the SMTP password contains `&` or `\`, alertmanager starts but cannot authenticate with the mail server. All alert notifications will silently fail. No error is surfaced in container logs unless alertmanager attempts to send — there is no startup-time SMTP validation.

**Fix:** Use awk string concatenation to build the replacement (never use user-provided strings directly as the replacement argument of `gsub`). The safe idiom is:

```sh
awk -v h="$SMTP_HOST" -v p="$SMTP_PORT" -v u="$SMTP_USER" \
    -v pw="$SMTP_PASSWORD" -v f="$SMTP_FROM" \
    '{
       gsub(/\$\{SMTP_HOST\}/, h)
       gsub(/\$\{SMTP_PORT\}/, p)
       gsub(/\$\{SMTP_USER\}/, u)
       gsub(/\$\{SMTP_PASSWORD\}/, pw)
       gsub(/\$\{SMTP_FROM\}/, f)
       print
    }' /etc/alertmanager/alertmanager.yml.tpl > /tmp/alertmanager.yml
```

Note: `-v pw="$SMTP_PASSWORD"` also has the `&` issue in replacement. The truly safe fix when the replacement value may contain `&` or `\` is to escape them before passing to gsub:

```sh
awk 'BEGIN{
  pw=ENVIRON["SMTP_PASSWORD"]
  gsub(/&/, "\\&", pw)      # escape & so it becomes literal
  gsub(/\\/, "\\\\", pw)    # escape \ so it becomes literal
  # ... same for other vars
}{gsub(/\$\{SMTP_PASSWORD\}/, pw); print}'
```

Apply the same escaping to all five ENVIRON variables before use in `gsub`.

---

### CR-02: `XMLUploadErrorsTotal` is never incremented for async batches (>50 XMLs)

**File:** `backend/handlers/xml_upload.go:477` / `backend/worker/xml_worker.go:116`

**Issue:** `XMLUploadErrorsTotal.Inc()` is only called inside the inline path (`len(xmlFiles) <= BatchAsyncThreshold`, i.e., ≤50 XMLs). When the XML upload is large (>50 files), the request returns HTTP 202 and the actual processing is delegated to `xml_worker.go` via `handlers.ProcessXMLBatch(...)`. The worker calls `processXMLBatch` which correctly records `rejected_count` in the database but never calls `XMLUploadErrorsTotal.Inc()`. As a result, the `XMLUploadFalha` alert (`increase(xml_upload_errors_total[5m]) > 0`) will never fire for large upload failures — the most likely scenario in a production fiscal batch.

**Impact:** The alert that is supposed to detect upload failures is blind to the majority of production traffic. Large XML batches (the primary use case for this system) fail silently from the alert's perspective.

**Fix:** Call `XMLUploadErrorsTotal.Inc()` in `processXMLBatch` in `xml_upload.go` when `rejected > 0`, so it applies regardless of inline/async path:

```go
// In processXMLBatch, after the final db.Exec UPDATE:
if rejected > 0 {
    XMLUploadErrorsTotal.Inc()
}
log.Printf("[XMLUpload] batch=%s concluído: imported=%d rejected=%d", batchID, imported, rejected)
```

This places the increment in the shared code path that both inline and async processing use.

---

## Warnings

### WR-01: `BridgeOffline` alert fires immediately on first daemon startup (startup false positive)

**File:** `monitoring/prometheus/rules/fiscal.yml:37`

**Issue:** The alert expression is `time() - bridge_last_run_timestamp_seconds > 3900` with `for: 0s`. When the bridge daemon starts for the first time (or after a restart), `BRIDGE_LAST_RUN_TIMESTAMP` is a Prometheus Gauge initialized to `0` by the `prometheus_client` library. The expression evaluates to `time() - 0`, which equals the current Unix epoch (approximately 1.7 billion seconds), which is vastly larger than 3900. The alert fires instantly on every daemon restart, before the bridge has had a chance to complete its first scheduled run. Every deploy will generate an alert.

**Impact:** Alert fatigue. On-call team receives a `BridgeOffline` critical alert on every routine restart, deploy, or container update via Watchtower. Teams learn to ignore it.

**Fix:** Add `for: 70m` (slightly more than one run cycle) to allow the bridge to complete its first run without triggering the alert:

```yaml
- alert: BridgeOffline
  expr: >
    bridge_last_run_timestamp_seconds > 0
    AND time() - bridge_last_run_timestamp_seconds > 3900
  for: 0s
```

The `> 0` guard means the alert only fires after the gauge has been set at least once (i.e., after the first run completes). Alternatively, use `for: 70m` instead of `for: 0s` to absorb startup transients.

---

### WR-02: `postgres-exporter` connects with `sslmode=disable` in production

**File:** `docker-compose.prod.yml:194`

**Issue:** The `api` service uses `sslmode=require` in the production database URL (line 14), but `postgres-exporter` uses `sslmode=disable` on line 194:
```yaml
DATA_SOURCE_NAME=postgresql://${DB_USER}:${DB_PASSWORD}@db:5432/${DB_NAME}?sslmode=disable
```

The DB password is passed in plaintext over the Docker network without TLS. While both containers are on `fb_net` (internal Docker network, not exposed to the internet), this is an inconsistency with the explicit SSL requirement used for the API service.

**Impact:** If the Docker network is compromised or traffic is inspected, postgres credentials are exposed in plaintext. The dev compose uses `sslmode=disable` for the API too, so this is a prod-only gap.

**Fix:** Change the postgres-exporter `DATA_SOURCE_NAME` in `docker-compose.prod.yml` to use `sslmode=require`:

```yaml
- DATA_SOURCE_NAME=postgresql://${DB_USER}:${DB_PASSWORD}@db:5432/${DB_NAME}?sslmode=require
```

---

### WR-03: Grafana admin password defaults to `admin` in both compose files

**File:** `docker-compose.yml:122`, `docker-compose.prod.yml:142`

**Issue:** Both compose files define:
```yaml
- GF_SECURITY_ADMIN_PASSWORD=${GRAFANA_ADMIN_PASSWORD:-admin}
```

The fallback `admin` is the well-known Grafana default. If `GRAFANA_ADMIN_PASSWORD` is not set in the deployment environment, Grafana starts with `admin/admin`. In `docker-compose.prod.yml`, Grafana is exposed via Traefik on a public hostname (`GRAFANA_HOST`), making the admin account publicly accessible with the default password. `GF_AUTH_ANONYMOUS_ENABLED=true` is accepted per the review context, but a weak admin password on a public endpoint is a distinct risk.

**Impact:** Any person who discovers the Grafana URL can authenticate as admin, access all dashboards, modify alert rules, and create new data sources.

**Fix:** Remove the `:-admin` fallback and require `GRAFANA_ADMIN_PASSWORD` to be explicitly set. Fail at startup if unset via an entrypoint check, or document this as a mandatory secret in the deployment guide:

```yaml
- GF_SECURITY_ADMIN_PASSWORD=${GRAFANA_ADMIN_PASSWORD}
```

If `GRAFANA_ADMIN_PASSWORD` is not in Coolify secrets, Docker Compose will error on deployment rather than silently using the insecure default.

---

### WR-04: `_NoOpCounter.inc` is defined twice — the first definition is silently discarded

**File:** `erp-bridge-aws/bridge.py:83-88`

**Issue:** The `_NoOpCounter` stub class defines the `inc` method twice:

```python
class _NoOpCounter:
    def inc(self, amount=1):  # line 83 — first definition
        pass
    def labels(self, **kwargs):
        return self
    def inc(self, amount=1):  # line 87 — second definition (silently overwrites)
        pass
```

Python silently discards the first method definition. This is flagged by the `# noqa: F811` comment on line 87, confirming the author was warned by the linter. While the functional result is the same (both are no-ops), this indicates a copy-paste error during stub construction. The `labels()` method also takes `**kwargs` but the real `Counter.labels()` takes positional or keyword label arguments by name — when called as `.labels(status="success")`, the stub returns `self`, which then has `inc()` called. This chain works correctly.

**Impact:** No runtime failure, but the dead method definition is dead code that creates confusion. A future maintainer who needs to change stub behavior may only update one copy.

**Fix:** Remove the duplicate `inc` definition:

```python
class _NoOpCounter:
    def inc(self, amount=1):
        pass
    def labels(self, **kwargs):
        return self
```

---

## Info

### IN-01: `XMLUploadErrorsTotal` increments once per batch, not per rejected file

**File:** `backend/handlers/xml_upload.go:476-478`

**Issue:** The counter documentation says "Total de uploads XML rejeitados ou com erro de parse" (implying individual files), but the increment fires once per batch whenever `rejected > 0`. A batch with 10 rejected XMLs increments the counter by 1, not 10.

**Impact:** The Prometheus expression `increase(xml_upload_errors_total[5m]) > 0` still fires correctly (any rejection triggers it). However, the counter value in Grafana dashboards cannot be used to estimate the number of rejected files — it only counts affected batches. Dashboard queries that display `xml_upload_errors_total` as a cumulative rejection count will underreport.

**Fix:** Either change the increment to `XMLUploadErrorsTotal.Add(float64(rejected))` to count individual rejections, or update the metric name/description to reflect batch semantics (`xml_upload_batches_with_errors_total`).

---

### IN-02: `statusRecorder` does not proxy `http.Flusher` or `http.Hijacker`

**File:** `backend/handlers/metrics.go:99-107`

**Issue:** `statusRecorder` wraps `http.ResponseWriter` but only overrides `WriteHeader`. If any downstream handler uses `http.Flusher` (for SSE/streaming) or `http.Hijacker` (for WebSocket upgrades), the type assertion `w.(http.Flusher)` will succeed on the underlying `ResponseWriter` but fail if attempted on the `statusRecorder` wrapper, because `statusRecorder` does not implement these interfaces.

**Impact:** Currently, no route in the application appears to use SSE or WebSockets, so this is not a production issue today. If such a handler is added in the future, it will fail at runtime.

**Fix:** Add interface forwarding for `http.Flusher`:

```go
func (sr *statusRecorder) Flush() {
    if f, ok := sr.ResponseWriter.(http.Flusher); ok {
        f.Flush()
    }
}
```

---

### IN-03: `metricsReNum` regex can double-replace numeric segments that end with `/`

**File:** `backend/handlers/metrics.go:27`

**Issue:** The regex `/\d+(/|$)` matches `/123/` (a segment followed by `/`). The `ReplaceAllStringFunc` replaces it with `/:n/`. If the path has two adjacent numeric segments like `/api/123/456`, the first match consumes `/123/`, replacing it with `/:n/`, leaving `/api/:n/456`. The `456` segment is then matched in the next replacement. This is correct for sequential segments.

However, for a path like `/api/123/456/`, the regex matches `/123/` then `/456/`. The result is `/api/:n/:n/` which then has the trailing slash stripped to `/api/:n/:n`. This is the expected behavior per the normalization design and poses no cardinality risk.

The actual edge case: a path ending with a numeric segment immediately followed by nothing (e.g., `/api/123`) matches `/123` via the `$` anchor in the alternation. The function returns `/:n` (no trailing slash). Then `TrimRight` only fires if `HasSuffix(path, "/")` which is false — so the result is `/api/:n`. Correct.

No actual bug; noted for completeness that the regex alternation `/|$)` means the digit-group match itself does not include the captured `/` in the replacement when `$` is the match — the captured group is zero-width. The `strings.HasSuffix(m, "/")` check correctly distinguishes the two cases.

**Fix:** No action required. The logic is correct. This note is informational.

---

## Conclusion

**Conditional Pass** — two blockers must be fixed before the observability stack is production-ready:

1. **CR-01** (awk `&` corruption): A password containing `&` will silently break alertmanager SMTP authentication. This is a likely scenario for auto-generated secure passwords. Fix the awk gsub escaping before going live.

2. **CR-02** (async XML blind spot): The primary production path for bulk XML uploads (>50 files) never increments `XMLUploadErrorsTotal`. The `XMLUploadFalha` alert is effectively disabled for the most common batch size. Add the counter call to `processXMLBatch`.

WR-01 (startup false positive) and WR-03 (Grafana default password) are also strongly recommended before exposing the stack publicly. WR-02 is a defense-in-depth issue at the network layer. IN-01 through IN-03 are minor improvements that do not affect operational correctness.

---

_Reviewed: 2026-05-16_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: Standard_
