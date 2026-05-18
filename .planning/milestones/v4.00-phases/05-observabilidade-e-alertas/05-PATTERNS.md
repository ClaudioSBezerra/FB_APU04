# Phase 5: Observabilidade e Alertas — Pattern Map

**Mapped:** 2026-05-16
**Files analyzed:** 12 novos/modificados
**Analogs found:** 8 / 12

---

## File Classification

| Arquivo Novo/Modificado | Role | Data Flow | Analog Mais Próximo | Qualidade |
|-------------------------|------|-----------|---------------------|-----------|
| `docker-compose.yml` (adicionar serviços) | config | request-response | `docker-compose.prod.yml` | exact |
| `installer/aws-bridge/docker-compose.yml` (porta 8086) | config | request-response | `installer/aws-bridge/docker-compose.yml` (si mesmo) | exact |
| `backend/go.mod` (adicionar prometheus/client_golang) | config | — | `backend/go.mod` (si mesmo) | exact |
| `backend/handlers/metrics.go` (novo arquivo) | middleware | request-response | `backend/handlers/middleware.go` | role-match |
| `backend/main.go` (registrar /metrics + wrap mux) | config | request-response | `backend/main.go` (si mesmo) | exact |
| `erp-bridge-aws/Dockerfile` (adicionar prometheus_client) | config | — | `erp-bridge-aws/Dockerfile` (si mesmo) | exact |
| `erp-bridge-aws/bridge.py` (adicionar métricas) | service | event-driven | `erp-bridge-aws/bridge.py` — `run_daemon()` | role-match |
| `monitoring/prometheus/prometheus.yml` (novo arquivo) | config | request-response | Nenhum — sem analog no projeto | none |
| `monitoring/prometheus/rules/fiscal.yml` (novo arquivo) | config | event-driven | Nenhum — sem analog no projeto | none |
| `monitoring/alertmanager/alertmanager.yml` (novo arquivo) | config | request-response | `backend/services/email.go` — SMTP config | partial |
| `monitoring/grafana/provisioning/**` (novos arquivos) | config | — | Nenhum — sem analog no projeto | none |
| `docs/runbooks/*.md` (novos arquivos) | — | — | Nenhum — documentação nova | none |

---

## Pattern Assignments

### `docker-compose.yml` — Adicionar prometheus, grafana, alertmanager, postgres-exporter

**Analog:** `docker-compose.prod.yml` e `docker-compose.yml` existentes

**Padrão de serviço com rede fb_net** (docker-compose.yml, linhas 65–99):
```yaml
# PADRÃO EXISTENTE: serviço na rede fb_net, sem exposição externa, com restart e volume
db:
  image: postgres:15-alpine
  container_name: fb_apu04-db
  restart: always
  volumes:
    - postgres_data_apu04:/var/lib/postgresql/data
  networks:
    - fb_net
  healthcheck:
    test: ["CMD-SHELL", "pg_isready -U postgres"]
    interval: 10s
    timeout: 5s
    retries: 10
    start_period: 30s
```

**Padrão de healthcheck HTTP** (docker-compose.prod.yml, linhas 37–42):
```yaml
# PADRÃO EXISTENTE: healthcheck com curl para endpoint HTTP interno
healthcheck:
  test: ["CMD", "curl", "-f", "http://localhost:8084/api/health"]
  interval: 30s
  timeout: 10s
  retries: 3
  start_period: 40s
```

**Padrão de env vars via `${VAR}`** (docker-compose.yml, linhas 13–28):
```yaml
# PADRÃO EXISTENTE: vars de ambiente injetadas via ${} do .env / Coolify
environment:
  - SMTP_HOST=${SMTP_HOST}
  - SMTP_PORT=${SMTP_PORT}
  - SMTP_USER=${SMTP_USER}
  - SMTP_PASSWORD=${SMTP_PASSWORD}
  - SMTP_FROM=${SMTP_FROM}
```

**Instrução de cópia para novos serviços:**
- Copiar padrão `image + container_name + restart: always + networks: [fb_net]` de qualquer serviço existente
- NÃO expor porta Prometheus (9090) externamente — apenas `expose:` dentro de fb_net
- Grafana pode ter `labels: traefik.*` copiado do serviço `web` se for acessado via domínio
- Volumes nomeados seguem padrão do bloco `volumes:` na raiz do compose (ex: `prometheus_data:`, `grafana_data:`)

---

### `installer/aws-bridge/docker-compose.yml` — Expor porta 8086 do bridge

**Analog:** `installer/aws-bridge/docker-compose.yml` (si mesmo, linhas 1–33)

**Padrão existente do serviço bridge:**
```yaml
services:
  bridge:
    image: ghcr.io/claudiosbezerra/fb_apu04-bridge:latest
    container_name: fbtax-bridge-apu04
    restart: always
    volumes:
      - ./config.yaml:/app/config.yaml:ro
      - bridge_data:/app/data
    environment:
      - TZ=America/Recife
    logging:
      driver: json-file
      options:
        max-size: "10m"
        max-file: "5"
```

**Instrução:** Adicionar `expose: ["8086"]` ao serviço `bridge` existente. Se Prometheus está na mesma máquina mas em compose separado, adicionar também ao `ports: ["8086:8086"]` para acesso via IP do host. Ver nota sobre topologia de rede na seção "Questões Abertas" do RESEARCH.md.

---

### `backend/go.mod` — Adicionar dependência prometheus/client_golang

**Analog:** `backend/go.mod` (si mesmo, linhas 1–11)

**Padrão existente do go.mod:**
```go
module fb_apu01

go 1.22.0

require (
    github.com/golang-jwt/jwt/v5 v5.3.1
    github.com/lib/pq v1.11.2
    golang.org/x/crypto v0.17.0
    golang.org/x/text v0.14.0
)

require github.com/joho/godotenv v1.5.1
```

**Instrução:** Adicionar ao bloco `require` principal:
```
github.com/prometheus/client_golang v1.20.x
```
Executar `go mod tidy` após adição para popular o `go.sum` e adicionar dependências transitivas (`prometheus/client_model`, `prometheus/common`, `prometheus/procfs`, etc.).

---

### `backend/handlers/metrics.go` — Novo arquivo: middleware Prometheus + counters de eventos críticos

**Analog:** `backend/handlers/middleware.go`

**Padrão de imports no pacote handlers** (middleware.go, linhas 1–9):
```go
package handlers

import (
    "net/http"
    "os"
    "strings"
    "sync"
    "time"
)
```

**Padrão de middleware wrapper** (middleware.go, linhas 94–120):
```go
// PADRÃO: middleware como função que retorna http.Handler, envolve o mux inteiro
func SecurityMiddleware(next http.Handler, allowedOrigins map[string]bool) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // ... lógica de interceptação ...
        next.ServeHTTP(srw, r)
    })
}
```

**Padrão de vars de pacote globais** (middleware.go, linhas 141–146):
```go
// PADRÃO: vars exportadas no nível do pacote, inicializadas na declaração
var (
    LoginRL          = newRateLimiter(5, 15*time.Minute)
    RegisterRL       = newRateLimiter(10, time.Hour)
    ForgotPasswordRL = newRateLimiter(3, time.Hour)
    ResetDBRateLimiter = newRateLimiter(1, time.Hour)
)
```

**Instrução para metrics.go:** Seguir o mesmo padrão de vars exportadas no nível de pacote, substituindo `newRateLimiter(...)` por `promauto.NewCounter(...)` e `promauto.NewHistogramVec(...)`. O middleware de instrumentação HTTP deve ser uma função `MetricsMiddleware(next http.Handler) http.Handler` com a mesma assinatura do `SecurityMiddleware`.

**Padrão de normalização de path (crítico — evitar Pitfall 3):**
```go
// PADRÃO A IMPLEMENTAR: normalizar paths com UUIDs antes de usar como label
// Extraído do padrão de route registration em main.go (linhas 587-595)
// "/api/erp-bridge/runs/abc123-uuid" → "/api/erp-bridge/runs/:id"
func normalizePath(path string) string {
    // Substituir UUIDs por ":id"
    // Substituir segmentos numéricos por ":n"
    // Baseado nos padrões de rota em main.go
}
```

---

### `backend/main.go` — Registrar `/metrics` e encadear MetricsMiddleware

**Analog:** `backend/main.go` (si mesmo)

**Padrão de registro de rota pública** (main.go, linhas 285–321):
```go
// PADRÃO EXISTENTE: rota pública sem auth, com header Content-Type
http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    // ... lógica ...
    json.NewEncoder(w).Encode(response)
})
```

**Padrão de uso do SecurityMiddleware** (main.go, linhas 665–667):
```go
// PADRÃO EXISTENTE: handler encadeado na criação do http.Server
server := &http.Server{
    Addr:    ":" + port,
    Handler: handlers.SecurityMiddleware(http.DefaultServeMux, handlers.GetAllowedOrigins()),
    // ...
}
```

**Instrução:** Adicionar antes do bloco de routes existentes:
```go
// /metrics exposto via promhttp.Handler() — SEM auth JWT (Prometheus faz scrape interno)
http.Handle("/metrics", promhttp.Handler())
```
E encadear o MetricsMiddleware no handler do servidor:
```go
Handler: handlers.MetricsMiddleware(
    handlers.SecurityMiddleware(http.DefaultServeMux, handlers.GetAllowedOrigins()),
),
```

---

### `erp-bridge-aws/Dockerfile` — Adicionar prometheus_client ao pip install

**Analog:** `erp-bridge-aws/Dockerfile` (si mesmo, linhas 1–16)

**Padrão existente:**
```dockerfile
FROM python:3.12-slim

WORKDIR /app

RUN pip install --no-cache-dir \
    pyyaml \
    requests \
    oracledb \
    python-dateutil

COPY bridge.py .
VOLUME ["/app/data"]
CMD ["python", "bridge.py", "--daemon"]
```

**Instrução:** Adicionar `prometheus_client` à lista do `pip install --no-cache-dir`, mantendo o formato multiline com `\`.

---

### `erp-bridge-aws/bridge.py` — Adicionar métricas Prometheus ao daemon

**Analog:** `erp-bridge-aws/bridge.py` — função `run_daemon()` e bloco de imports

**Padrão de imports no topo do bridge** (bridge.py, linhas 18–38):
```python
import argparse
import io
import json as _json
import logging
import re
# ...
import requests
import yaml

try:
    import oracledb
except ImportError:
    print("ERRO: python-oracledb nao instalado.")
    sys.exit(1)
```

**Padrão de import com try/except** (bridge.py, linhas 33–38):
```python
# PADRÃO EXISTENTE: import opcional com fallback gracioso
try:
    import oracledb
except ImportError:
    print("ERRO: python-oracledb nao instalado.")
    sys.exit(1)
```

**Padrão do loop do daemon** (bridge.py, linhas 1110–1127):
```python
def run_daemon(cfg: dict, fbtax: FBTaxClient) -> int:
    # ...
    while True:
        try:
            now = datetime.now(tz=BRASILIA)
            # ── 0. Heartbeat ──────────────────────────────────────────────────
            fbtax.heartbeat()
            # ── 1. Reset tracker ──────────────────────────────────────────────
            # ...
```

**Instrução para bridge.py:** Adicionar import de `prometheus_client` seguindo o padrão `try/except` existente. Declarar counters/gauges como variáveis globais após os imports. Chamar `start_http_server(8086)` no início de `run_daemon()`, antes do `while True`. Incrementar counters nos pontos corretos dentro do loop (heartbeat → `BRIDGE_DAEMON_ONLINE.set(1)`, run com erro → `BRIDGE_RUNS_TOTAL.labels(status='error').inc()`).

---

### `monitoring/prometheus/prometheus.yml` — Novo arquivo de scrape config

**Analog:** Nenhum no projeto. Usar Pattern 3 do RESEARCH.md como base canônica.

**Nota de topologia:** O scrape do bridge depende de confirmação da topologia de rede (ver Open Question 1 no RESEARCH.md). Planejar dois cenários: (a) bridge na mesma rede docker → `targets: ['bridge:8086']`; (b) bridge em máquina separada → `targets: ['<IP_HOST_BRIDGE>:8086']`.

---

### `monitoring/prometheus/rules/fiscal.yml` — Novo arquivo de alert rules

**Analog:** Nenhum no projeto. Usar Pattern 4 do RESEARCH.md como base canônica.

---

### `monitoring/alertmanager/alertmanager.yml` — Novo arquivo de routing SMTP

**Analog parcial:** `backend/services/email.go` — configuração SMTP existente

**Padrão de leitura de env vars SMTP** (email.go, implícito via `os.Getenv`):
```go
// PADRÃO EXISTENTE: SMTP configurado via env vars idênticas às usadas no compose
EmailConfig{
    Host:     os.Getenv("SMTP_HOST"),
    Port:     /* SMTP_PORT */,
    Username: os.Getenv("SMTP_USER"),
    Password: os.Getenv("SMTP_PASSWORD"),
    From:     os.Getenv("SMTP_FROM"),
}
```

**Instrução:** O `alertmanager.yml` deve usar as mesmas variáveis de ambiente `${SMTP_HOST}`, `${SMTP_PORT}`, `${SMTP_USER}`, `${SMTP_PASSWORD}`, `${SMTP_FROM}` já definidas no `docker-compose.yml`. Isso garante que não há duplicação de credenciais. Usar Pattern 6 do RESEARCH.md como template.

**Recipient email:** `claudio.bezerra@ferreiracosta.com.br` (email do usuário do projeto).

---

### `monitoring/grafana/provisioning/**` — Novos arquivos de provisioning

**Analog:** Nenhum no projeto. Usar Pattern 5 do RESEARCH.md como base canônica.

**Estrutura de diretórios a criar:**
```
monitoring/grafana/provisioning/datasources/prometheus.yml
monitoring/grafana/provisioning/dashboards/dashboards.yml
monitoring/grafana/dashboards/bridge_runs.json
monitoring/grafana/dashboards/api_health.json
monitoring/grafana/dashboards/db_size.json
```

**Nota crítica:** Dashboards como JSON no repositório (não clicados na UI). Ver Anti-Pattern 1 no RESEARCH.md.

---

### `docs/runbooks/*.md` — Novos arquivos de runbook

**Analog:** Nenhum no projeto (primeira documentação operacional).

**Estrutura recomendada** (extraída de RESEARCH.md, seção "Runbooks"):
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

**Arquivos a criar:**
- `docs/runbooks/dpy4011-consecutivos.md`
- `docs/runbooks/xml-upload-falha.md`
- `docs/runbooks/reset-banco-executado.md`

---

## Shared Patterns

### Padrão: Env vars via `${VAR}` no docker-compose
**Fonte:** `docker-compose.yml` (linhas 13–28) e `docker-compose.prod.yml` (linhas 15–26)
**Aplicar a:** `docker-compose.yml` (novos serviços), `monitoring/alertmanager/alertmanager.yml`
```yaml
# Reutilizar exatamente as mesmas vars já declaradas no stack
environment:
  - SMTP_HOST=${SMTP_HOST}
  - SMTP_PORT=${SMTP_PORT}
  - SMTP_USER=${SMTP_USER}
  - SMTP_PASSWORD=${SMTP_PASSWORD}
  - SMTP_FROM=${SMTP_FROM}
```

### Padrão: Rede fb_net sem exposição externa
**Fonte:** `docker-compose.yml` (linhas 65–99) e `docker-compose.prod.yml` (linhas 67–84)
**Aplicar a:** Todos os novos serviços Docker (prometheus, grafana, alertmanager, postgres-exporter)
```yaml
networks:
  - fb_net
# NÃO adicionar ports: para prometheus e alertmanager (apenas expose: interno)
# Grafana pode ter traefik labels se acesso público for necessário
```

### Padrão: Middleware wrapper em Go
**Fonte:** `backend/handlers/middleware.go` (linhas 94–120)
**Aplicar a:** `backend/handlers/metrics.go` — MetricsMiddleware deve ter a mesma assinatura
```go
func MetricsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // instrumentar antes
        next.ServeHTTP(rw, r)
        // observar depois (status code, duration)
    })
}
```

### Padrão: Vars globais exportadas no pacote handlers
**Fonte:** `backend/handlers/middleware.go` (linhas 141–146)
**Aplicar a:** `backend/handlers/metrics.go` — counters e histogramas exportados para uso em outros handlers
```go
var (
    // Counters de eventos críticos — incrementados pelos handlers específicos
    BridgeRunErrorsTotal  prometheus.Counter   // incrementado em erp_bridge.go
    XMLUploadErrorsTotal  prometheus.Counter   // incrementado em xml_upload.go
    DatabaseResetTotal    prometheus.Counter   // incrementado em admin.go
)
```

### Padrão: Rota pública sem JWT
**Fonte:** `backend/main.go` (linhas 285–321 — rota `/api/health`)
**Aplicar a:** `backend/main.go` — rota `/metrics` (Prometheus não envia JWT)
```go
// Registrar ANTES dos withAuth handlers; sem wrapper de autenticação
http.Handle("/metrics", promhttp.Handler())
```

### Padrão: Import opcional com try/except em Python
**Fonte:** `erp-bridge-aws/bridge.py` (linhas 33–38)
**Aplicar a:** `erp-bridge-aws/bridge.py` — import de `prometheus_client`
```python
try:
    from prometheus_client import start_http_server, Counter, Gauge
    METRICS_ENABLED = True
except ImportError:
    METRICS_ENABLED = False
    log.warning("prometheus_client não instalado — métricas desabilitadas")
```

---

## No Analog Found

| Arquivo | Role | Data Flow | Motivo |
|---------|------|-----------|--------|
| `monitoring/prometheus/prometheus.yml` | config | request-response | Primeiro arquivo de configuração Prometheus no projeto |
| `monitoring/prometheus/rules/fiscal.yml` | config | event-driven | Primeiro arquivo de alerting rules no projeto |
| `monitoring/grafana/provisioning/datasources/prometheus.yml` | config | — | Primeiro arquivo de provisioning Grafana no projeto |
| `monitoring/grafana/provisioning/dashboards/dashboards.yml` | config | — | Primeiro arquivo de provisioning Grafana no projeto |
| `monitoring/grafana/dashboards/*.json` | config | — | Primeiro dashboard JSON do projeto |
| `docs/runbooks/*.md` | documentation | — | Primeira documentação de runbook do projeto |

Para esses arquivos, o planner deve usar os Patterns 3–6 do RESEARCH.md como referência primária (todos marcados como `[ASSUMED — padrão canônico da biblioteca]`).

---

## Metadata

**Escopo de busca de analogs:**
- `docker-compose.yml`, `docker-compose.prod.yml`, `installer/aws-bridge/docker-compose.yml`
- `backend/main.go`, `backend/go.mod`
- `backend/handlers/middleware.go`, `backend/handlers/erp_bridge.go`, `backend/handlers/xml_upload.go`, `backend/handlers/xml_conciliacao.go`, `backend/handlers/admin.go`
- `backend/services/email.go`
- `erp-bridge-aws/bridge.py`, `erp-bridge-aws/Dockerfile`

**Arquivos escaneados:** 14
**Data de mapeamento:** 2026-05-16

**Questões abertas que afetam o planner:**
1. **Topologia de rede do bridge** (Open Question 1 do RESEARCH.md): Se o bridge roda em máquina AWS separada do compose principal, o scrape config do prometheus.yml precisa usar IP externo em vez de nome de serviço Docker. O Plan 01 deve confirmar isso com o usuário antes de escrever o scrape config.
2. **Bug CR-01 em xml_conciliacao.go** (Open Question 4 do RESEARCH.md): `delta_total` omite IPI — recomendado como Task 0 do Plan 01 (correção SQL simples, 1 linha).
3. **Acesso Grafana** (Open Question 3 do RESEARCH.md): Role `viewer` com senha compartilhada vs. acesso anônimo. Decisão afeta a configuração do serviço Grafana no docker-compose.
