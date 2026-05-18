# Phase 5: Observabilidade e Alertas — Research

**Researched:** 2026-05-16
**Domain:** Prometheus/Grafana, Go metrics (prometheus/client_golang), Python prometheus_client, alertas SMTP
**Confidence:** HIGH (stack, arquitetura existente), MEDIUM (alerting channels)

---

## Summary

O Prometheus **NAO** está provisionado no docker-compose atual. O ROADMAP e o PROJECT.md mencionam "Prometheus já provisionado em prod", mas a inspeção dos quatro arquivos docker-compose do projeto (raiz, prod, installer/docker-compose.yml, installer/fcxlabs/docker-compose.yml) mostra apenas quatro serviços: `api`, `web`, `db`, `redis`. Não há serviços `prometheus`, `grafana` nem `alertmanager`. [VERIFIED: codebase grep]

O backend Go (go.mod) não inclui `prometheus/client_golang`. O endpoint `/metrics` não existe em `main.go` — o único endpoint de healthcheck é `/api/health` que retorna JSON estático. O bridge Python (Dockerfile) não inclui `prometheus_client`. Grafana, portanto, não tem data source configurado. [VERIFIED: codebase grep]

Isso significa que Phase 5 é fundamentalmente uma fase de **provisionamento + instrumentação**, não apenas configuração de dashboards. O esforço real é: (1) adicionar serviços Prometheus + Grafana ao docker-compose, (2) instrumentar o Go backend com `prometheus/client_golang`, (3) expor métricas do bridge Python via `prometheus_client`, (4) criar dashboards Grafana via JSON provisioning, (5) configurar alertas via Alertmanager + SMTP (SMTP já configurado no backend). O canal de alerta Slack não tem evidência de credenciais no projeto — SMTP é o caminho seguro.

**Recomendação primária:** Adicionar Prometheus + Grafana + Alertmanager como serviços no docker-compose. Instrumentar Go com `prometheus/client_golang v1.x`. Expor métricas do bridge via HTTP `prometheus_client`. Provisionar dashboards via arquivos JSON (infra-as-code, sem clique manual no Grafana). Alertas via SMTP (já configurado).

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| OBS-01 | Dashboards Grafana dedicados — runs do Bridge, latência API, taxa de erro, ocupação do banco | Prometheus + Grafana provisioning via JSON. Go: `prometheus/client_golang`. Python: `prometheus_client`. PG exporter para DB size. |
| OBS-02 | Alertas para falhas críticas — erros DPY-4011 consecutivos, falhas de upload XML, reset de banco executado | Alertmanager com rules YAML + SMTP (já configurado no stack). Métricas custom counters para cada evento crítico. |
</phase_requirements>

---

## Descobertas Críticas da Codebase (O que JÁ existe)

### O que foi encontrado

| Componente | Status Real | Onde |
|------------|-------------|------|
| Prometheus | **NAO provisionado** | Ausente nos 4 docker-compose files |
| Grafana | **NAO provisionado** | Ausente nos 4 docker-compose files |
| Alertmanager | **NAO provisionado** | Ausente |
| Go `/metrics` endpoint | **NAO existe** | main.go não registra essa rota |
| `prometheus/client_golang` | **NAO está no go.mod** | go.mod tem apenas 5 dependências |
| Python `prometheus_client` | **NAO está no Dockerfile do bridge** | pip install: pyyaml, requests, oracledb, python-dateutil |
| SMTP | **Já configurado** | SMTP_HOST/PORT/USER/PASSWORD/FROM em todos os compose files |
| `/api/health` endpoint | **Existe** | main.go:285 — retorna JSON com status DB |
| `erp_bridge_runs` table | **Existe** | migration 065 — status, total_erros, erro_msg por run |
| `admin_destructive_actions` table | **Existe** | migration 073 — audit log de resets |
| `xml_upload_batches` table | **Existe** | migration 076 — histórico de uploads XML |

[VERIFIED: codebase grep + file reads de docker-compose.yml, docker-compose.prod.yml, backend/go.mod, erp-bridge-aws/Dockerfile]

### O que o bridge JA reporta via API (sem Prometheus)

O bridge Python já faz chamadas REST para o backend Go a cada run:
- `POST /api/erp-bridge/runs` — cria run com data_ini, data_fim, origem
- `PATCH /api/erp-bridge/runs/{id}` — status (success/partial/error), total_enviados, total_ignorados, total_erros, erro_msg
- `POST /api/erp-bridge/runs/{id}/items` — stats por servidor/tipo (enviados, ignorados, erros, status, erro_msg)
- `POST /api/erp-bridge/heartbeat` — sinal de vida do daemon

Esses dados JÁ estão em `erp_bridge_runs` e `erp_bridge_run_items` no PostgreSQL. Isso permite que os alertas sejam gerados **por queries SQL** no Grafana, sem necessariamente precisar de métricas Prometheus custom para o bridge — mas metrics custom são mais rápidos para detecção em <1 min.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Coleta de métricas Go (latência, erros 5xx) | API (backend Go) | — | Instrumentação no handler HTTP com `prometheus/client_golang` |
| Coleta de métricas Bridge (runs, erros DPY-4011) | Bridge Python | API Go (dados históricos) | Bridge expõe `/metrics` via `prometheus_client`; API tem dados históricos em SQL |
| Armazenamento de séries temporais | Prometheus | — | Scrape de `/metrics` do Go e do Bridge |
| Dashboards visuais | Grafana | — | Conecta ao Prometheus; equipe fiscal vê sem SSH |
| Regras de alerta | Alertmanager | Grafana Alerting | Alertmanager para SMTP; Grafana Alerting como alternativa mais simples |
| Notificações críticas | SMTP (serviço email existente) | — | Credenciais SMTP já existem no stack |
| Tamanho do banco (pg_size_approx) | postgres_exporter | PostgreSQL | Exporter separado ou query direta via Grafana datasource SQL |
| Status público Bridge (equipe fiscal) | Grafana (dashboard público/read-only) | — | Grafana anonymous viewer ou read-only user |

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| prometheus/client_golang | v1.20.x | Instrumentação do Go backend | Biblioteca oficial Prometheus para Go |
| prometheus_client (Python) | 0.21.x | Exposição de métricas do bridge | Biblioteca oficial Prometheus para Python |
| prom/prometheus Docker image | v2.x (latest stable) | Servidor de coleta e storage | Padrão da indústria |
| grafana/grafana Docker image | 11.x (latest stable) | Dashboards e alertas | Padrão; integra nativamente com Prometheus |
| prom/alertmanager Docker image | v0.27.x | Roteamento de alertas → SMTP | Componente oficial do ecossistema Prometheus |

[ASSUMED — versões exatas precisam de verificação via `docker pull` + release notes; a lógica de versão (client_golang v1.x, prometheus v2.x, grafana 11.x) é estável mas pode ter patches mais recentes]

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| prom/postgres-exporter | 0.15.x | Métricas PostgreSQL (tamanho DB, conexões) | Necessário para OBS-01 (tamanho do banco) sem escrever SQL no Prometheus |
| grafana/loki (FUTURO) | — | Log aggregation | Não necessário agora — logs via Docker json-file são suficientes |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Alertmanager | Grafana Alerting nativo | Grafana Alerting é mais simples mas menos flexível em roteamento; Alertmanager tem melhor grouping/dedup |
| postgres_exporter | Grafana PostgreSQL datasource direto | Grafana pode queryar PG diretamente, eliminando o exporter — mais simples mas menos granular |
| prometheus_client HTTP server no bridge | Pushgateway | Pushgateway é para jobs batch sem servidor HTTP; bridge roda como daemon com loop, portanto HTTP server é melhor |

**Instalação Go:**
```bash
go get github.com/prometheus/client_golang/prometheus
go get github.com/prometheus/client_golang/prometheus/promhttp
```

**Instalação Python bridge:**
```bash
pip install prometheus_client
```

---

## Architecture Patterns

### System Architecture Diagram

```
[Bridge Python daemon]
    |-- POST /api/erp-bridge/heartbeat → [Go API]
    |-- POST /api/erp-bridge/runs     → [Go API]
    |-- /metrics (porta 8086)         → [Prometheus scrape]

[Go API :8084]
    |-- GET /metrics                  → [Prometheus scrape]
    |-- (handlers instrumentados com histogramas/counters)

[Prometheus :9090]
    |-- scrape api:8084/metrics
    |-- scrape bridge:8086/metrics
    |-- scrape postgres-exporter:9187/metrics
    |-- evaluate alert rules
    |-- route alerts → [Alertmanager]

[Alertmanager :9093]
    |-- receive alerts from Prometheus
    |-- route → SMTP (relay existente)

[Grafana :3000]
    |-- datasource: Prometheus
    |-- dashboards provisioned via JSON
    |-- equipe fiscal acessa via browser (sem SSH)
    |-- anonymous viewer OR read-only user
```

### Recommended Project Structure

```
monitoring/
├── prometheus/
│   ├── prometheus.yml          # scrape configs + alertmanager url
│   └── rules/
│       └── fiscal.yml          # alert rules (DPY-4011, upload errors, reset)
├── alertmanager/
│   └── alertmanager.yml        # smtp route
├── grafana/
│   ├── provisioning/
│   │   ├── datasources/
│   │   │   └── prometheus.yml  # auto-provision datasource
│   │   └── dashboards/
│   │       └── dashboards.yml  # auto-provision dashboard loader
│   └── dashboards/
│       ├── bridge_runs.json    # OBS-01: runs do bridge
│       ├── api_health.json     # OBS-01: latência API, erro 5xx
│       └── db_size.json        # OBS-01: tamanho DB
docs/
└── runbooks/
    ├── dpy4011-consecutivos.md
    ├── xml-upload-falha.md
    └── reset-banco-executado.md
```

---

## Pattern 1: Instrumentação Go — Middleware HTTP com Prometheus

O padrão correto para o backend Go existente é criar um middleware que envolve o `http.DefaultServeMux` e registra histograma de latência + counter de status codes.

```go
// Source: [ASSUMED — padrão da biblioteca prometheus/client_golang]
// backend/handlers/metrics.go

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
    httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
        Name:    "http_request_duration_seconds",
        Help:    "Latência das requisições HTTP",
        Buckets: prometheus.DefBuckets,
    }, []string{"method", "path", "status"})

    httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "http_requests_total",
        Help: "Total de requisições HTTP",
    }, []string{"method", "path", "status"})

    // Counters de eventos críticos (OBS-02)
    BridgeRunErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
        Name: "bridge_run_errors_total",
        Help: "Total de runs do Bridge com erro",
    })
    XMLUploadErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
        Name: "xml_upload_errors_total",
        Help: "Total de uploads XML com falha",
    })
    DatabaseResetTotal = promauto.NewCounter(prometheus.CounterOpts{
        Name: "database_reset_total",
        Help: "Total de resets de banco executados",
    })
)

// Em main.go: http.Handle("/metrics", promhttp.Handler())
```

---

## Pattern 2: Bridge Python — HTTP Exporter com prometheus_client

O bridge roda como daemon com loop de 60 segundos. O padrão é iniciar um servidor HTTP na porta 8086 em uma goroutine separada (thread Python).

```python
# Source: [ASSUMED — padrão da biblioteca prometheus_client]
# erp-bridge-aws/bridge.py (adições)

from prometheus_client import start_http_server, Counter, Gauge

BRIDGE_RUNS_TOTAL = Counter('bridge_runs_total', 'Total de runs iniciados', ['status'])
BRIDGE_DPY4011_TOTAL = Counter('bridge_dpy4011_total', 'Total de erros DPY-4011')
BRIDGE_DAEMON_ONLINE = Gauge('bridge_daemon_online', 'Bridge daemon está online (1=sim)')
BRIDGE_LAST_RUN_TIMESTAMP = Gauge('bridge_last_run_timestamp_seconds', 'Timestamp do último run')

# Em main() antes do loop:
start_http_server(8086)  # Expõe /metrics na porta 8086
```

---

## Pattern 3: Prometheus prometheus.yml com scrape configs

```yaml
# Source: [ASSUMED — padrão de configuração Prometheus]
# monitoring/prometheus/prometheus.yml

global:
  scrape_interval: 15s
  evaluation_interval: 15s

alerting:
  alertmanagers:
    - static_configs:
        - targets: ['alertmanager:9093']

rule_files:
  - 'rules/*.yml'

scrape_configs:
  - job_name: 'fb_apu04_api'
    static_configs:
      - targets: ['api:8084']
    metrics_path: '/metrics'

  - job_name: 'fb_apu04_bridge'
    static_configs:
      - targets: ['bridge:8086']   # bridge roda na rede docker da AWS
    metrics_path: '/metrics'

  - job_name: 'postgres'
    static_configs:
      - targets: ['postgres-exporter:9187']
```

---

## Pattern 4: Alert Rules para OBS-02

```yaml
# Source: [ASSUMED — padrão Prometheus alerting rules]
# monitoring/prometheus/rules/fiscal.yml

groups:
  - name: bridge_critico
    rules:
      - alert: BridgeDPY4011Consecutivos
        expr: increase(bridge_dpy4011_total[5m]) >= 3
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Bridge com erros DPY-4011 consecutivos"
          description: "{{ $value }} erros DPY-4011 em 5 minutos"
          runbook_url: "http://simu.fcxlabs.com/docs/runbooks/dpy4011"

      - alert: XMLUploadFalha
        expr: increase(xml_upload_errors_total[5m]) > 0
        for: 0m  # dispara imediatamente
        labels:
          severity: warning
        annotations:
          summary: "Upload XML com falha"

      - alert: ResetBancoExecutado
        expr: increase(database_reset_total[5m]) > 0
        for: 0m
        labels:
          severity: critical
        annotations:
          summary: "Reset de banco executado"
```

---

## Pattern 5: Grafana Provisioning (dashboards como código)

```yaml
# Source: [ASSUMED — padrão Grafana provisioning]
# monitoring/grafana/provisioning/dashboards/dashboards.yml

apiVersion: 1
providers:
  - name: 'fiscal-dashboards'
    orgId: 1
    type: file
    disableDeletion: false
    updateIntervalSeconds: 30
    options:
      path: /var/lib/grafana/dashboards
```

---

## Pattern 6: Alertmanager SMTP (usando credenciais existentes)

```yaml
# Source: [ASSUMED — padrão Alertmanager]
# monitoring/alertmanager/alertmanager.yml

global:
  smtp_smarthost: '${SMTP_HOST}:${SMTP_PORT}'
  smtp_from: '${SMTP_FROM}'
  smtp_auth_username: '${SMTP_USER}'
  smtp_auth_password: '${SMTP_PASSWORD}'

route:
  group_by: ['alertname']
  group_wait: 30s
  group_interval: 5m
  repeat_interval: 4h
  receiver: 'fiscal-team'

receivers:
  - name: 'fiscal-team'
    email_configs:
      - to: 'claudio.bezerra@ferreiracosta.com.br'
        subject: '[ALERTA FB_APU04] {{ .GroupLabels.alertname }}'
        body: '{{ range .Alerts }}{{ .Annotations.description }}{{ end }}'
```

---

## Anti-Patterns to Avoid

- **Criar dashboards via clique manual no Grafana:** Dashboards clicados não sobrevivem a um `docker-compose down -v`. Sempre usar provisioning via JSON.
- **Usar Pushgateway para o bridge:** Pushgateway é para jobs efêmeros (cron). O bridge é um daemon com loop — HTTP server direto com `prometheus_client` é o padrão correto.
- **Expor Grafana/Prometheus sem autenticação na internet:** Grafana deve ficar na rede interna ou atrás de Traefik com autenticação básica. Não expor porta Prometheus ao público.
- **Alertas com `for: 0m` para todos os alertas:** Reservar `for: 0m` apenas para eventos que NUNCA são falso-positivo (ex: reset de banco). Para DPY-4011, usar `for: 1m` para evitar spam de alerta transitório.
- **Instrumentar cada rota individualmente:** Usar middleware wrapper no `http.DefaultServeMux` — uma vez, em `main.go`. Não repetir código de instrumentação em cada handler.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Tamanho do banco PostgreSQL | Query SQL periódica num goroutine | `prom/postgres_exporter` | Exporter já coleta pg_database_size, conexões, bloqueios com labels corretos |
| Agregação e retenção de métricas | Qualquer coisa customizada | Prometheus | 15 dias de retenção padrão é suficiente; não usar InfluxDB/TimescaleDB sem razão |
| Envio de emails de alerta | net/smtp direto do Go | Alertmanager | Deduplicação, agrupamento, silence, inibição são features críticas do Alertmanager |
| Dashboard HTML customizado | React page com métricas | Grafana | O requisito "equipe fiscal vê sem SSH" é exatamente para o que Grafana serve |
| Detecção de alertas por polling SQL | Goroutine que faz SELECT periodicamente | Prometheus alert rules | Rules são declarativas, versionadas, e testáveis com `promtool` |

**Key insight:** O ecossistema Prometheus é projetado para esse stack exato (Go + Python + Postgres). Usar os componentes canônicos economiza semanas de desenvolvimento e garante integração com Grafana out-of-the-box.

---

## Common Pitfalls

### Pitfall 1: Bridge na rede isolada (aws-bridge/docker-compose.yml)
**What goes wrong:** O bridge na AWS roda em `installer/aws-bridge/docker-compose.yml` numa rede sem nome compartilhado com o Prometheus. Prometheus não consegue fazer scrape de `bridge:8086`.
**Why it happens:** O Prometheus está no compose principal (simu.fcxlabs.com) e o bridge está num compose separado na mesma máquina (ou em máquina diferente).
**How to avoid:** Duas opções: (a) adicionar `bridge` ao compose principal e à rede `fb_net`; (b) usar `host.docker.internal` ou IP fixo do host para scrape externo. Descobrir a topologia exata de deploy antes de planejar o scrape config.
**Warning signs:** `prometheus: connection refused` nos targets de scrape.

### Pitfall 2: Grafana não persiste dashboards sem volume
**What goes wrong:** `docker-compose down -v` apaga o SQLite do Grafana com todos os dashboards criados manualmente.
**Why it happens:** Sem volume `grafana_data`, os dados ficam na camada de container.
**How to avoid:** Sempre provisionar dashboards via `/etc/grafana/provisioning/dashboards/` (mount de volume com JSON). Volume `grafana_data` persiste apenas datasources e users — dashboards devem ser código.

### Pitfall 3: Label cardinality explosion em histogramas HTTP
**What goes wrong:** Label `path` com valor dinâmico como `/api/erp-bridge/runs/abc123-uuid` cria milhares de séries no Prometheus.
**Why it happens:** UUIDs na URL viram labels distintos.
**How to avoid:** Normalizar paths antes de usar como label: `/api/erp-bridge/runs/` (trailing slash) ou regex substitution no middleware. Nunca usar o path raw como label de histograma.

### Pitfall 4: Alertmanager SMTP em produção precisa de TLS/STARTTLS
**What goes wrong:** `smtp_require_tls: true` por padrão no Alertmanager. Se o SMTP_HOST do projeto usa porta 587 com STARTTLS, funciona. Mas se usar 465 (SSL), o Alertmanager pode falhar silenciosamente.
**Why it happens:** Configuração padrão assume STARTTLS na 587.
**How to avoid:** Verificar SMTP_HOST e SMTP_PORT do ambiente de produção. Testar com `promtool check` e enviar alerta de teste antes do go-live.

### Pitfall 5: Prometheus sem autenticação vaza dados de negócio
**What goes wrong:** Porta 9090 exposta sem autenticação revela nomes de tabelas, volumes de transação, status de runs — dados sensíveis fiscalmente.
**Why it happens:** Prometheus por padrão não tem autenticação.
**How to avoid:** Não expor porta 9090 externamente (apenas na rede `fb_net`). Grafana é o único ponto de acesso público (com autenticação). Alternativamente, usar `basic_auth` do Prometheus (recurso a partir do v2.24).

### Pitfall 6: "Prometheus já provisionado" — expectativa vs. realidade
**What goes wrong:** O plano pressupõe que apenas dashboards precisam ser criados, mas nada está provisionado.
**Why it happens:** A documentação do projeto (ROADMAP, PROJECT.md) diz "Prometheus já provisionado em prod", mas a inspeção da codebase não confirma isso nos arquivos versionados.
**How to avoid:** Fase 5 deve começar do zero: adicionar serviços ao docker-compose, instrumentar código, só então criar dashboards. O plano de 2 plans no ROADMAP é correto, mas o scope do Plan 01 é maior do que "só dashboards".

---

## Métricas Críticas para OBS-01 e OBS-02

### Métricas do Go Backend (instrumentar)

| Métrica | Tipo | Label | OBS | Alerta |
|---------|------|-------|-----|--------|
| `http_request_duration_seconds` | Histogram | method, path_pattern, status | OBS-01 latência | — |
| `http_requests_total` | Counter | method, path_pattern, status | OBS-01 erro 5xx | `rate > threshold` |
| `database_reset_total` | Counter | — | OBS-01, OBS-02 | dispara imediatamente |
| `xml_upload_errors_total` | Counter | — | OBS-01, OBS-02 | `increase > 0` |
| `bridge_run_errors_total` | Counter | — | OBS-01, OBS-02 | `increase > N` |
| `go_goroutines` | Gauge | — | OBS-01 (built-in) | — |
| `process_resident_memory_bytes` | Gauge | — | OBS-01 (built-in) | — |

Métricas com sufixo `(built-in)` são expostas automaticamente pelo `prometheus/client_golang` sem código adicional.

### Métricas do Bridge Python (instrumentar)

| Métrica | Tipo | Label | OBS | Alerta |
|---------|------|-------|-----|--------|
| `bridge_runs_total` | Counter | status (success/partial/error) | OBS-01 | — |
| `bridge_dpy4011_total` | Counter | servidor | OBS-01, OBS-02 | `increase >= 3 em 5min` |
| `bridge_daemon_online` | Gauge | — | OBS-01 | `== 0 por > 5min` |
| `bridge_docs_sent_total` | Counter | tipo (nfe_saidas/etc) | OBS-01 | — |
| `bridge_last_run_timestamp_seconds` | Gauge | — | OBS-01 | `time() - value > 86400` |

### Métricas do PostgreSQL (via postgres_exporter)

| Métrica | Purpose | Dashboard |
|---------|---------|---------|
| `pg_database_size_bytes` | Tamanho do banco (OBS-01) | db_size panel |
| `pg_stat_activity_count` | Conexões ativas | api health panel |
| `pg_stat_user_tables_n_live_tup` | Linhas por tabela | não crítico |

---

## Runbooks (estrutura recomendada)

Os runbooks devem ser Markdown no repositório (`docs/runbooks/`), não em wiki externo, porque:
- Ficam versionados junto com o código
- Podem ser linkados de alertas Alertmanager via `runbook_url`
- A equipe fiscal pode acessar via GitHub/Gitea ou via URL pública se o repo for privado

Estrutura de cada runbook:
```markdown
# Runbook: [Nome do Alerta]
**Severidade:** critical/warning
**Dispara quando:** [condição]
**Detectado em:** <1 min após evento

## Sintomas
## Causa Mais Provável
## Passos de Mitigação
1. [ação concreta com comando ou URL]
## Verificação Pós-Mitigação
## Escalar Para
```

---

## Topologia de Deploy: Questão Aberta Crítica

A **maior incerteza** desta fase é a topologia de rede entre os componentes em produção:

- O `docker-compose.yml` principal roda em `simu.fcxlabs.com` (AWS)
- O `installer/aws-bridge/docker-compose.yml` roda o bridge — pode ser na **mesma máquina** (AWS) ou em outra máquina (on-premises na Ferreira Costa)
- Se o bridge está em outra máquina, o Prometheus não consegue fazer scrape via nome de serviço Docker — precisa de IP/hostname e abertura de firewall

Esta questão deve ser confirmada com o usuário **antes** de planejar o scrape config do bridge.

Opções:
- **Mesma máquina:** bridge join na rede `fb_net` ou usar `host.docker.internal`
- **Máquinas distintas:** Prometheus scrape via IP externo (requer firewall rule) ou bridge pusha para Pushgateway (mas Pushgateway é antipadrão para daemon)
- **Solução pragmática sem mudança de rede:** Bridge Python pode enviar dados via `POST` para o backend Go, que os expõe como métricas — elimina necessidade de acesso direto do Prometheus ao bridge

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Docker | docker-compose services | Inferred sim (prod usa Docker) | Unknown | — |
| docker-compose v2 | `docker compose up` | Inferred sim | Unknown | — |
| SMTP relay | Alertmanager notificações | Sim (env vars existem) | — | — |
| Rede `fb_net` compartilhada | Prometheus scrape da API | Sim | — | — |
| Prometheus → Bridge network | Prometheus scrape do bridge | **DESCONHECIDO** | — | Push metrics via Go API |
| Traefik (Coolify) | Grafana via HTTPS | Inferred sim | — | Porta 3000 diretamente |

**Missing dependencies with no fallback:**
- Topologia de rede do bridge (máquina separada vs. mesma máquina) — requer confirmação antes do Plan 01

**Missing dependencies with fallback:**
- Grafana via Traefik: se não configurado, usar porta 3000 com acesso restrito por IP ou VPN

---

## Open Questions (RESOLVED)

1. **Topologia de rede do bridge**
   - What we know: O bridge tem seu próprio docker-compose (`installer/aws-bridge/`)
   - What's unclear: Está na mesma máquina AWS que o `docker-compose.yml` principal? Ou em outra máquina?
   - RESOLVED: Plan 01 adota estratégia de juntar o bridge à rede Docker `fb_net` (mesma máquina). O `installer/aws-bridge/docker-compose.yml` declara `networks: [fb_net]` com `external: true`. Scrape direto em `bridge:8086`. Alternativa on-prem documentada como comentário inline no `prometheus.yml`.

2. **Credenciais SMTP de produção**
   - What we know: SMTP_HOST/PORT/USER/PASSWORD existem como env vars no docker-compose — mas os valores reais estão em Coolify env vars (não no repo)
   - What's unclear: TLS ou STARTTLS? Porta 587 ou 465?
   - RESOLVED: Plan 02 usa `require_tls: false` + `starttls: true` com porta 587 (STARTTLS). Alertmanager v0.27 não expande env vars diretamente; solução: arquivo `.tpl` + `envsubst` no entrypoint do container. Task 4 inclui validação `amtool check-config`.

3. **Grafana: acesso anônimo ou usuário fiscal dedicado?**
   - What we know: Requisito é "equipe fiscal vê sem SSH"
   - What's unclear: Sem senha (anônimo) ou com conta específica (viewer)?
   - RESOLVED: Plan 01 Task 1 cria conta `viewer` dedicada via env vars `GF_AUTH_ANONYMOUS_ENABLED=false` + `GF_SECURITY_ADMIN_PASSWORD=${GRAFANA_ADMIN_PASSWORD}` + role `viewer` com senha compartilhada. Mais seguro que anônimo para dados fiscais sensíveis.

4. **Bug pendente CR-01 em xml_conciliacao.go**
   - What we know: STATE.md menciona bug onde delta_total omite IPI — "será corrigido antes ou durante Phase 5"
   - What's unclear: Deve entrar no Plan 01 desta fase ou é tratado como hotfix separado?
   - RESOLVED: CR-01 foi corrigido como hotfix em commit `9dec6a0` antes do início da Phase 5 (2026-05-16). Não entra nos planos desta fase.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Versões das imagens Docker (prometheus v2.x, grafana 11.x, alertmanager v0.27.x) | Standard Stack | Pode haver incompatibilidade de configuração — verificar release notes antes |
| A2 | Bridge Python roda na mesma rede Docker que o Prometheus | Architecture Diagram | Se em máquina separada, scrape direto não funciona — usar alternativa push |
| A3 | SMTP funciona via STARTTLS na porta configurada | Pattern 6 / Alertmanager | Se porta 465 (SSL implícito), configuração do Alertmanager muda |
| A4 | Grafana 11.x suporta provisioning de dashboards via JSON da mesma forma que 10.x | Pattern 5 | API de provisioning é estável, mas verificar antes de escrever JSONs |
| A5 | `prometheus/client_golang v1.20.x` compatível com Go 1.22 | Standard Stack | go.mod do projeto usa go 1.22; verificar compatibilidade |
| A6 | O requisito "<1min após o evento" é alcançável com scrape_interval 15s + alerting evaluation 15s | Alert Rules | Latência total: até 30s (scrape) + regra `for: 0m` = <1min. Para `for: 1m` seria 1m30s — confirmar se aceitável |

---

## Sources

### Primary (HIGH confidence)
- Codebase direto: `docker-compose.yml`, `docker-compose.prod.yml`, `installer/aws-bridge/docker-compose.yml`, `installer/fcxlabs/docker-compose.yml` — confirmado que Prometheus/Grafana não estão presentes
- `backend/go.mod` — confirmado que `prometheus/client_golang` não está como dependência
- `erp-bridge-aws/Dockerfile` — confirmado que `prometheus_client` não está instalado
- `backend/main.go` — confirmado que `/metrics` endpoint não existe, apenas `/api/health`
- `backend/migrations/065_erp_bridge.sql` — schema de `erp_bridge_runs` e `erp_bridge_run_items`
- `backend/services/email.go` — SMTP service existente confirma viabilidade de alertas por email
- `.planning/PROJECT.md` — "Prometheus/Grafana já provisionados em prod" é uma afirmação histórica/aspiracional, não refletida nos arquivos versionados

### Secondary (MEDIUM confidence)
- `erp-bridge-aws/bridge.py` — lógica de daemon, heartbeat, run reporting — confirma que bridge já faz comunicação REST com o backend
- `.planning/codebase/CONCERNS.md` — menciona explicitamente "Add Prometheus metrics for `db.Stats()`" como improvement path

### Tertiary (ASSUMED — verificar antes de implementar)
- Versões específicas das imagens Docker (prometheus, grafana, alertmanager, postgres_exporter)
- Topologia de rede do bridge em produção
- Configuração TLS do SMTP em produção

---

## Metadata

**Confidence breakdown:**
- "Prometheus não está provisionado": HIGH — verificado em todos os docker-compose files
- Standard Stack (quais bibliotecas usar): HIGH — prometheus/client_golang e prometheus_client são as únicas escolhas canônicas
- Architecture Patterns: MEDIUM — baseado em práticas da indústria; topologia exata de rede é ASSUMED
- Pitfalls: HIGH — todos baseados em problemas reais conhecidos do ecossistema
- Versões específicas: LOW-MEDIUM — precisam de verificação contra registros Docker Hub

**Research date:** 2026-05-16
**Valid until:** 2026-06-16 (ecossistema Prometheus/Grafana é estável; versões podem ter patches menores)
