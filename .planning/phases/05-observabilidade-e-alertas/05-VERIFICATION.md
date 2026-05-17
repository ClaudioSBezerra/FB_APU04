---
phase: 05-observabilidade-e-alertas
verified: 2026-05-17T01:06:37Z
status: human_needed
score: 3/4 success criteria verified
overrides_applied: 0
human_verification:
  - test: "Disparar BridgeDaemonDown manualmente e medir tempo até email"
    expected: "Email chega em menos de 90s após bridge_daemon_online=0 (nota: for:1m é deliberado para evitar falso positivo em redeploy)"
    why_human: "O critério diz <1min; BridgeDaemonDown usa for:1m resultando em ~90s. Precisa de decisão humana sobre aceitar a exceção documentada."
  - test: "Verificar se Grafana anônimo é acessível via URL pública sem login"
    expected: "Acesso a http://grafana.fcxlabs.com/ sem autenticação mostra dashboards Bridge Runs, API Health, DB Size"
    why_human: "GF_AUTH_ANONYMOUS_ENABLED=true está configurado no compose, mas acesso real ao Grafana publicado requer verificação no ambiente de produção"
  - test: "Confirmar que SMTP_PASSWORD sem caracteres & ou \\ funciona com alertmanager"
    expected: "Alertmanager inicia e envia email de teste para claudio.bezerra@ferreiracosta.com.br quando alerta dispara"
    why_human: "O fix de CR-01 (escaping de & e \\ no awk) está correto no código, mas a entrega real do email requer ambiente com credenciais SMTP reais"
---

# Verification: Phase 05 — Observabilidade e Alertas

**Date:** 2026-05-17
**Goal:** Provisionar do zero a stack Prometheus + Grafana + Alertmanager, instrumentar backend Go e bridge Python, entregar dashboards visíveis para a equipe fiscal sem SSH e alertas críticos em <1min via SMTP.
**Outcome:** PASS com ressalva — 3 de 4 critérios de sucesso verificados no código; 1 critério (alertas <1min) tem exceção deliberada documentada para BridgeDaemonDown que requer decisão humana.

---

## Success Criteria

| # | Criterion | Status | Evidence |
|---|-----------|--------|----------|
| 1 | Dashboards Grafana funcionais (runs Bridge, latência API, erro 5xx, tamanho DB) | PASS | `bridge_runs.json` (245 linhas, painéis com `bridge_runs_total`, `bridge_daemon_online`, `bridge_last_run_timestamp_seconds`, `bridge_dpy4011_total`); `api_health.json` (230 linhas, painéis com `http_request_duration_seconds`, `http_requests_total{status=~"5.."}`, RPS); `db_size.json` (200 linhas, painéis com `pg_database_size_bytes`, `pg_stat_activity_count`); provisioning datasource e dashboards provider configurados e montados via volume |
| 2 | Cada alerta crítico dispara em <1min após o evento | WARNING | 4 de 5 alertas críticos dentro de 60s: BridgeDPY4011Consecutivos=60s exato (15+15+30), BridgeOffline=30s (15+15+0), XMLUploadFalha=30s, ResetBancoExecutado=30s. EXCEÇÃO: BridgeDaemonDown usa `for: 1m` resultando em ~90s — deliberado para evitar falso positivo em redeploy (comentado na regra). BridgeOffline cobre o cenário principal de bridge parado em <30s |
| 3 | Runbook escrito para cada tipo de alerta com passos de mitigação | PASS | 6 arquivos presentes: `dpy4011-consecutivos.md`, `bridge-offline.md`, `xml-upload-falha.md`, `reset-banco-executado.md`, `db-tamanho.md`, `README.md`. Todos contêm seções Sintomas, Causa Mais Provável, Passos de Mitigação, Verificação Pós-Mitigação, Escalar Para |
| 4 | Equipe fiscal vê status do Bridge sem acesso SSH (Grafana anônimo) | PASS (code) / NEEDS HUMAN (runtime) | `GF_AUTH_ANONYMOUS_ENABLED=true` e `GF_AUTH_ANONYMOUS_ORG_ROLE=Viewer` presentes em `docker-compose.yml:124-125` e `docker-compose.prod.yml:144-145`; dashboards montados em `/var/lib/grafana/dashboards`; Traefik labels com `grafana.fcxlabs.com` em prod. Acesso real requer verificação em produção |

---

## Requirements Coverage

| Requirement | Status | Evidence |
|-------------|--------|----------|
| OBS-01: Dashboards Grafana dedicados (runs Bridge, latência API, taxa de erro, ocupação banco) | PASS | Três dashboards JSON substantivos (675 linhas total). Métricas: `bridge_runs_total`, `bridge_daemon_online`, `bridge_last_run_timestamp_seconds`, `bridge_dpy4011_total` (bridge_runs.json); `http_request_duration_seconds` p95, `http_requests_total{status=~"5.."}` (api_health.json); `pg_database_size_bytes` (db_size.json). Prometheus scrape targets: `api:8084`, `bridge:8086`, `postgres-exporter:9187`, `localhost:9090` |
| OBS-02: Alertas para falhas críticas (DPY-4011, upload XML, reset banco) | PASS | 6 regras em `fiscal.yml`: BridgeDPY4011Consecutivos (`increase(bridge_dpy4011_total[5m]) >= 3`, for:30s), XMLUploadFalha (`increase(xml_upload_errors_total[5m]) > 0`, for:0s), ResetBancoExecutado (`increase(database_reset_total[5m]) > 0`, for:0s). Alertmanager com 3 receivers + 1 inhibit_rule wired via `alertmanager:9093` |

---

## Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `backend/handlers/metrics.go` | MetricsMiddleware, normalizePath, BridgeRunErrorsTotal, XMLUploadErrorsTotal, DatabaseResetTotal | VERIFIED | 138 linhas; todos os 3 counters definidos com promauto; normalizePath implementado com regex UUID+numérico; MetricsMiddleware captura status via statusRecorder |
| `backend/main.go` | /metrics route + MetricsMiddleware no server | VERIFIED | linha 287: `http.Handle("/metrics", promhttp.Handler())`; linha 673: `Handler: handlers.MetricsMiddleware(handlers.SecurityMiddleware(...))` |
| `backend/handlers/erp_bridge.go` | BridgeRunErrorsTotal.Inc() no caminho de erro | VERIFIED | linha 408-410: `if req.Status == "error" { BridgeRunErrorsTotal.Inc() }` — condicionado ao PATCH /api/erp-bridge/runs/{id} com status=error |
| `backend/handlers/xml_upload.go` | XMLUploadErrorsTotal.Inc() em processXMLBatch | VERIFIED | linha 97-99: `ProcessXMLBatch` (exportado) delega a `processXMLBatch`; linha 142-144: `if rejected > 0 { XMLUploadErrorsTotal.Inc() }` — cobre tanto inline (<= 50 XMLs via linha 480) quanto async (> 50 XMLs via xml_worker.go:116) |
| `backend/handlers/admin.go` | DatabaseResetTotal.Inc() após commit bem-sucedido | VERIFIED | linha 398-400: `DatabaseResetTotal.Inc()` após `tx.Commit()` no caminho de sucesso, ANTES do audit log de sucesso |
| `erp-bridge-aws/bridge.py` | prometheus_client com stubs, start_http_server(8086), 4 gauges/counters | VERIFIED | linha 44: import com try/except; linhas 51-76: BRIDGE_RUNS_TOTAL, BRIDGE_DPY4011_TOTAL, BRIDGE_DAEMON_ONLINE, BRIDGE_LAST_RUN_TIMESTAMP definidos; linha 1194: `start_http_server(8086)` no modo daemon; linha 1196: `BRIDGE_DAEMON_ONLINE.set(1)` na inicialização; linha 672: `BRIDGE_LAST_RUN_TIMESTAMP.set(_time.time())` após run; linha 965: `BRIDGE_DPY4011_TOTAL.inc()` em DPY-4011; linha 1348: `BRIDGE_DAEMON_ONLINE.set(0)` em exceção |
| `docker-compose.yml` | 4 serviços de monitoring | VERIFIED | prometheus:v2.55.1, grafana:11.3.0, alertmanager:v0.27.0, postgres-exporter:v0.15.0; GF_AUTH_ANONYMOUS_ENABLED=true; awk com escaping de & e \ (CR-01 corrigido) |
| `docker-compose.prod.yml` | Mesmo stack com Traefik labels | VERIFIED | Mesmos 4 serviços; Traefik labels para `grafana.fcxlabs.com`; GF_AUTH_ANONYMOUS_ENABLED=true; awk com escaping correto |
| `monitoring/prometheus/prometheus.yml` | 4 scrape targets | VERIFIED | `fb_apu04_api` (api:8084), `fb_apu04_bridge` (bridge:8086), `fb_apu04_postgres` (postgres-exporter:9187), `prometheus_self` (localhost:9090); `rule_files: ["rules/*.yml"]` |
| `monitoring/prometheus/rules/fiscal.yml` | 6 regras de alerta | VERIFIED | 6 alertas em 3 grupos: bridge_critico (BridgeDPY4011Consecutivos, BridgeOffline, BridgeDaemonDown), ops_critico (XMLUploadFalha, ResetBancoExecutado), db_critico (DBTamanhoAlto); todas com runbook_url annotation |
| `monitoring/grafana/dashboards/bridge_runs.json` | Painéis de bridge | VERIFIED | 245 linhas; painéis: "Taxa de Runs por Status (5m)", "Daemon Online", "Minutos desde Último Run", "Erros DPY-4011 por Hora" com queries Prometheus reais |
| `monitoring/grafana/dashboards/api_health.json` | Painéis de HTTP | VERIFIED | 230 linhas; painéis: "Latência p95 por Path", "Taxa de Erros 5xx por Path", "RPS Atual" com queries histogram_quantile e http_requests_total |
| `monitoring/grafana/dashboards/db_size.json` | Painéis de DB | VERIFIED | 200 linhas; painéis: "Tamanho do Banco por Database (MB)", "Tamanho Atual — fb_apu04", "Conexões Ativas PostgreSQL", "Top Tabelas por Linhas" |
| `monitoring/grafana/provisioning/datasources/prometheus.yml` | Auto-provisioning datasource | VERIFIED | Datasource Prometheus com url `http://prometheus:9090`, isDefault:true, timeInterval:15s |
| `monitoring/grafana/provisioning/dashboards/dashboards.yml` | Auto-provisioning provider | VERIFIED | Provider `fiscal-dashboards` com path `/var/lib/grafana/dashboards` e updateIntervalSeconds:30 |
| `monitoring/alertmanager/alertmanager.yml.tpl` | SMTP com 3 receivers + inhibit_rule | VERIFIED | smtp_smarthost com variáveis ${SMTP_*}; 3 receivers: fiscal-team-default, fiscal-team-critical, fiscal-team-warning; 1 inhibit_rule (BridgeDaemonDown suprime BridgeDPY4011Consecutivos); rotas por severity |
| `docs/runbooks/dpy4011-consecutivos.md` | Runbook com Sintomas/Causa/Passos/Verificação | VERIFIED | 104 linhas; 6 causas possíveis; 7 passos operacionais com comandos docker/SQL; Verificação Pós-Mitigação com curl Prometheus; Escalar Para |
| `docs/runbooks/bridge-offline.md` | Runbook BridgeOffline + BridgeDaemonDown | VERIFIED | 93 linhas; cobre ambos os alertas; 6 causas; 6 passos com docker compose commands; Verificação Pós-Mitigação com curl |
| `docs/runbooks/xml-upload-falha.md` | Runbook XMLUploadFalha | VERIFIED | 107 linhas; 7 causas; 7 passos com SQL e comandos docker; seção de histórico de incidentes |
| `docs/runbooks/reset-banco-executado.md` | Runbook ResetBancoExecutado | VERIFIED | 129 linhas; contexto explícito do incidente 2026-05-07; 6 passos de resposta a incidente; procedimento de restauração de backup; SQL de verificação de integridade |
| `docs/runbooks/db-tamanho.md` | Runbook DBTamanhoAlto | VERIFIED | 96 linhas; 5 causas; 6 passos com SQL de diagnóstico (pg_size_pretty, pg_stat_user_tables, VACUUM ANALYZE/FULL) |
| `docs/runbooks/README.md` | Índice mapeando alertas para runbooks | VERIFIED | Tabela com todos os 6 alertas, severidade, métrica e link para runbook; links de monitoramento Prometheus/Alertmanager/Grafana; instruções para adicionar novo runbook |

---

## Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `main.go` | `metrics.go` | MetricsMiddleware no server.Handler | WIRED | `handlers.MetricsMiddleware(handlers.SecurityMiddleware(...))` na linha 673 |
| `main.go` | `promhttp.Handler()` | `/metrics` route | WIRED | `http.Handle("/metrics", promhttp.Handler())` na linha 287 |
| `erp_bridge.go` | `metrics.go:BridgeRunErrorsTotal` | `.Inc()` em PATCH status=error | WIRED | linha 408-410 condicionado a `req.Status == "error"` |
| `xml_upload.go:processXMLBatch` | `metrics.go:XMLUploadErrorsTotal` | `.Inc()` quando rejected>0 | WIRED | linha 142-144; coberto tanto pelo caminho inline (via linha 480) quanto async (via `ProcessXMLBatch` exportado) |
| `admin.go:ResetDatabaseHandler` | `metrics.go:DatabaseResetTotal` | `.Inc()` após tx.Commit() | WIRED | linha 400; somente no caminho de sucesso |
| `xml_worker.go` | `xml_upload.go:ProcessXMLBatch` | `handlers.ProcessXMLBatch(...)` | WIRED | linha 116 do worker chama o wrapper exportado |
| `prometheus.yml` | `rules/fiscal.yml` | `rule_files: ["rules/*.yml"]` | WIRED | glob inclui o arquivo de regras |
| `prometheus.yml` | alertmanager:9093 | `alerting.alertmanagers` | WIRED | `targets: ["alertmanager:9093"]` |
| `docker-compose.yml:grafana` | dashboards JSON | volume mount | WIRED | `./monitoring/grafana/dashboards:/var/lib/grafana/dashboards:ro` |
| `docker-compose.yml:grafana` | provisioning | volume mount | WIRED | `./monitoring/grafana/provisioning:/etc/grafana/provisioning:ro` |
| `alertmanager.yml.tpl` | `docker-compose.yml` entrypoint awk | expansão de ${SMTP_*} | WIRED | awk com `ENVIRON[]` + escaping de & e \ (fix CR-01 aplicado) |

---

## Alert Timing Analysis (Critério 2: <1min)

| Alerta | Severidade | scrape | eval | for | Total | Status |
|--------|-----------|--------|------|-----|-------|--------|
| BridgeDPY4011Consecutivos | critical | 15s | 15s | 30s | 60s | PASS (exato 1min) |
| BridgeOffline | critical | 15s | 15s | 0s | 30s | PASS |
| BridgeDaemonDown | critical | 15s | 15s | 60s | 90s | WARNING — excede 1min por design |
| XMLUploadFalha | warning | 15s | 15s | 0s | 30s | N/A (warning) |
| ResetBancoExecutado | critical | 15s | 15s | 0s | 30s | PASS |
| DBTamanhoAlto | warning | 15s | 15s | 5m | ~5m30s | N/A (warning) |

**Nota sobre BridgeDaemonDown:** O `for: 1m` é uma decisão de design documentada no comentário da regra: "for: 1m para evitar falso positivo em redeploy curto." O alerta BridgeOffline (for:0s, 30s total) cobre o cenário mais crítico de bridge que parou de executar runs. BridgeDaemonDown é complementar — detecta quando o endpoint `/metrics` em :8086 está inacessível. A exceção é deliberada e documentada, mas tecnicamente viola o critério "<1min" para todos os alertas críticos.

---

## Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `docker-compose.yml` | 122 | `GF_SECURITY_ADMIN_PASSWORD=${GRAFANA_ADMIN_PASSWORD:-admin}` | WARNING | Fallback inseguro expõe Grafana com senha padrão se variável não for definida (WR-03 do review — não corrigido) |
| `docker-compose.prod.yml` | 142 | `GF_SECURITY_ADMIN_PASSWORD=${GRAFANA_ADMIN_PASSWORD:-admin}` | WARNING | Mesmo issue em prod com Grafana exposto publicamente via Traefik |
| `erp-bridge-aws/bridge.py` | 83-87 | Método `inc` definido duas vezes no `_NoOpCounter` | INFO | Segundo `inc` silenciosamente substitui o primeiro (WR-04 do review — sem impacto funcional) |

**Nota:** CR-01 (awk corrupção de & e \\) e CR-02 (XMLUploadErrorsTotal ausente no caminho async) foram identificados no code review e CORRIGIDOS antes da submissão final. Ambos os fixes estão presentes no código verificado.

---

## Human Verification Required

### 1. Aceitar exceção de timing para BridgeDaemonDown

**Test:** Parar o container bridge (`docker compose stop bridge`) e aguardar o email de alerta chegar.
**Expected:** Email `[FB_APU04][CRITICAL] BridgeDaemonDown` chega em até 90 segundos (não 60s como exige o critério). O alerta BridgeOffline complementar chegará em 30s se o bridge também parou de executar runs.
**Why human:** O critério formal diz "<1min para cada alerta crítico", mas BridgeDaemonDown usa `for:1m` deliberadamente. Precisa de decisão do responsável técnico sobre se o critério é satisfeito pela combinação BridgeOffline (30s) + BridgeDaemonDown (90s, cobrindo o endpoint inacessível).

### 2. Verificar acesso anônimo ao Grafana em produção

**Test:** Acessar `http://grafana.fcxlabs.com/` em browser sem autenticação.
**Expected:** Grafana abre diretamente nos dashboards (Bridge Runs, API Health, DB Size) sem solicitar login. Equipe fiscal consegue ver status do Bridge sem SSH.
**Why human:** `GF_AUTH_ANONYMOUS_ENABLED=true` está configurado corretamente no código, mas o acesso real depende do deploy em produção, DNS, Traefik TLS e se o Coolify secrets definiu `GRAFANA_ADMIN_PASSWORD` (ou se está usando o fallback inseguro `admin`).

### 3. Verificar entrega de email via SMTP em produção

**Test:** Injetar um evento que dispara alerta (ex: executar reset-db via painel admin em staging, ou incrementar counter manualmente) e aguardar email.
**Expected:** Email chega para `claudio.bezerra@ferreiracosta.com.br` em até 60s (ou 90s para BridgeDaemonDown) com subject `[FB_APU04][CRITICAL] ...`, body HTML com summary, description, severity e link do runbook.
**Why human:** O awk com escaping de `&` e `\` está correto no código, mas a entrega real requer credenciais SMTP reais (`SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`) configuradas no Coolify. Não é verificável estaticamente.

---

## Gaps Summary

Não há gaps bloqueadores. O único item não-PASS é o timing de BridgeDaemonDown (90s vs. critério de 60s), que é uma exceção deliberada documentada. Os dois blockers identificados no code review (CR-01 e CR-02) foram corrigidos antes da submissão. Os warnings do review (WR-01 startup false positive no BridgeOffline — já mitigado pelo guard `> 0` na expr; WR-02 sslmode postgres-exporter; WR-03 Grafana admin password fallback) permanecem como issues de hardening, não como bloqueadores para a entrega do objetivo da fase.

---

_Verified: 2026-05-17T01:06:37Z_
_Verifier: Claude (gsd-verifier)_
