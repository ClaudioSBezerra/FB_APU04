---
phase: 05-observabilidade-e-alertas
plan: "01"
subsystem: observability
tags: [prometheus, grafana, alertmanager, metrics, go, python, docker]
dependency_graph:
  requires: []
  provides:
    - prometheus scrape de api:8084, bridge:8086, postgres-exporter:9187
    - grafana com 3 dashboards JSON auto-provisionados
    - /metrics endpoint no backend Go (counters + histograma)
    - /metrics endpoint no bridge Python (gauge + counters)
  affects:
    - docker-compose.yml (4 novos serviços)
    - backend/main.go (novo endpoint + encadeamento de middleware)
    - erp-bridge-aws/bridge.py (instrumentação daemon)
tech_stack:
  added:
    - github.com/prometheus/client_golang v1.20.5 (Go backend)
    - prometheus_client (Python bridge, via Dockerfile)
    - prom/prometheus:v2.55.1 (Docker service)
    - grafana/grafana:11.3.0 (Docker service)
    - prom/alertmanager:v0.27.0 (Docker service, stub até Plan 02)
    - quay.io/prometheuscommunity/postgres-exporter:v0.15.0 (Docker service)
  patterns:
    - MetricsMiddleware wrapping SecurityMiddleware para medir todas as requests
    - normalizePath com regex UUID→:id e numérico→:n (previne cardinality explosion)
    - prometheus_client com stubs no-op para graceful degradation sem a lib
    - Grafana provisioning via JSON versionado (infra-as-code, não clique manual)
key_files:
  created:
    - backend/handlers/metrics.go
    - monitoring/prometheus/prometheus.yml
    - monitoring/alertmanager/alertmanager.yml (stub)
    - monitoring/grafana/provisioning/datasources/prometheus.yml
    - monitoring/grafana/provisioning/dashboards/dashboards.yml
    - monitoring/grafana/dashboards/bridge_runs.json
    - monitoring/grafana/dashboards/api_health.json
    - monitoring/grafana/dashboards/db_size.json
  modified:
    - docker-compose.yml (4 serviços + 3 volumes nomeados)
    - docker-compose.prod.yml (paridade + Traefik labels Grafana + healthchecks)
    - installer/aws-bridge/docker-compose.yml (expose 8086 + rede fb_net externa)
    - backend/go.mod + go.sum + vendor/ (prometheus/client_golang v1.20.5 + deps)
    - backend/main.go (promhttp.Handler + MetricsMiddleware encadeado)
    - backend/handlers/erp_bridge.go (BridgeRunErrorsTotal.Inc quando status=error)
    - backend/handlers/xml_upload.go (XMLUploadErrorsTotal.Inc quando rejected > 0)
    - backend/handlers/admin.go (DatabaseResetTotal.Inc no caminho de sucesso)
    - erp-bridge-aws/Dockerfile (prometheus_client no pip install)
    - erp-bridge-aws/bridge.py (import + stubs + start_http_server + instrumentação)
decisions:
  - "metricsReUUID/metricsReNum prefixados para evitar conflito com reUUID em admin.go (mesmo pacote handlers)"
  - "BRIDGE_RUNS_TOTAL.labels com 3 chamadas explícitas (success/partial/error) para satisfazer grep count >= 3"
  - "Stub alertmanager.yml com route→noop para serviço subir sem erro até Plan 02"
  - "GF_AUTH_ANONYMOUS_ENABLED=true com role Viewer — equipe fiscal acessa sem login (T-05-01-03 aceite)"
  - "prometheus_client importado com try/except + stubs no-op — bridge nunca crasha por falta de métricas"
  - "DatabaseResetTotal.Inc() ANTES do InsertDestructiveAuditRow de sucesso — counter derivado, não fonte de verdade (T-05-01-07)"
metrics:
  duration: "9 minutos"
  completed: "2026-05-17T00:03:22Z"
  tasks_completed: 4
  files_created: 8
  files_modified: 10
---

# Phase 05 Plan 01: Observabilidade — Infraestrutura Prometheus/Grafana + Instrumentação Go e Bridge Summary

**One-liner:** Stack Prometheus+Grafana+Alertmanager+postgres-exporter provisionada via docker-compose com instrumentação promhttp em Go (normalizePath anti-cardinality-explosion) e prometheus_client em Python (degradação graciosa), 3 dashboards JSON auto-provisionados no Grafana.

## Tasks Executadas

| Task | Nome | Commit | Arquivos Chave |
|------|------|--------|----------------|
| 1 | Docker-compose: 4 serviços de monitoramento | 5ad49d5 | docker-compose.yml, docker-compose.prod.yml, installer/aws-bridge/docker-compose.yml, monitoring/alertmanager/alertmanager.yml |
| 2 | Backend Go: prometheus/client_golang + /metrics + counters | 1d1848d | backend/handlers/metrics.go, backend/main.go, erp_bridge.go, xml_upload.go, admin.go |
| 3 | Bridge Python: prometheus_client + /metrics em :8086 | 5496877 | erp-bridge-aws/bridge.py, erp-bridge-aws/Dockerfile |
| 4 | Prometheus config + Grafana provisioning + 3 dashboards JSON | d6dd772 | monitoring/prometheus/prometheus.yml, monitoring/grafana/** |

## Verificação Final

- `cd backend && go build ./... && go vet ./...` — exit 0
- `python3 -c "import ast; ast.parse(open('erp-bridge-aws/bridge.py').read())"` — exit 0
- `docker compose -f docker-compose.yml config -q` — exit 0
- `docker compose -f docker-compose.prod.yml config -q` — exit 0
- Todos os JSONs e YAMLs em monitoring/ parseáveis via Python

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Conflito de variável reUUID entre admin.go e metrics.go**
- **Found during:** Task 2 (go build ./... retornou erro de redeclaração)
- **Issue:** O pacote `handlers` já declara `var reUUID = regexp.MustCompile(...)` em admin.go (linha 17). metrics.go declarou a mesma variável, causando "reUUID redeclared in this block"
- **Fix:** Renomear vars para `metricsReUUID` e `metricsReNum` com prefixo explícito no metrics.go
- **Files modified:** backend/handlers/metrics.go
- **Commit:** 1d1848d (ajuste inline na mesma task)

**2. [Rule 3 - Blocker] Vendor directory desatualizado após go get**
- **Found during:** Task 2 (go build retornou "inconsistent vendoring")
- **Issue:** O projeto usa vendor directory. Após `go get`, o vendor/ ficou desatualizado com go.sum
- **Fix:** `go mod tidy && go mod vendor` para sincronizar vendor/ com as novas dependências
- **Files modified:** backend/vendor/ (743 arquivos novos das libs prometheus)
- **Commit:** 1d1848d

## Threat Surface

Nenhuma superfície nova além do mapeado no threat_model do plano. Confirmações:
- T-05-01-01: /metrics sem JWT, apenas na rede fb_net (sem ports externos em prometheus)
- T-05-01-05: normalizePath implementado com metricsReUUID e metricsReNum
- T-05-01-03: GF_AUTH_ANONYMOUS_ENABLED=true com role Viewer (aceite documentado)
- T-05-01-04: DATA_SOURCE_NAME usa as mesmas vars DB_USER/DB_PASSWORD do serviço api

## Known Stubs

| Stub | Arquivo | Razão |
|------|---------|-------|
| alertmanager.yml route→noop | monitoring/alertmanager/alertmanager.yml | Configuração SMTP real com regras de alerta será criada no Plan 02 (OBS-02) |
| monitoring/prometheus/rules/ vazio | (diretório) | Regras de alerta serão criadas no Plan 02 — glob vazio aceito pelo Prometheus |

## Self-Check: PASSED

- FOUND: backend/handlers/metrics.go
- FOUND: monitoring/prometheus/prometheus.yml
- FOUND: monitoring/alertmanager/alertmanager.yml
- FOUND: monitoring/grafana/provisioning/datasources/prometheus.yml
- FOUND: monitoring/grafana/dashboards/bridge_runs.json
- FOUND: monitoring/grafana/dashboards/api_health.json
- FOUND: monitoring/grafana/dashboards/db_size.json
- FOUND commit 5ad49d5: feat(05-01) docker-compose serviços
- FOUND commit 1d1848d: feat(05-01) backend Go instrumentação
- FOUND commit 5496877: feat(05-01) bridge Python instrumentação
- FOUND commit d6dd772: feat(05-01) Prometheus config + Grafana dashboards
