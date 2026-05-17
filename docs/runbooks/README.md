# Runbooks — FB_APU04 Simulador Fiscal

Índice de runbooks operacionais para alertas Prometheus do sistema FB_APU04.
Cada runbook descreve o procedimento de triagem e mitigação para um alerta específico.

## Índice de Alertas

| Alerta Prometheus | Severidade | Métrica | Runbook |
|-------------------|------------|---------|---------|
| BridgeDPY4011Consecutivos | critical | `bridge_dpy4011_total` | [dpy4011-consecutivos.md](dpy4011-consecutivos.md) |
| BridgeOffline | critical | `bridge_last_run_timestamp_seconds` | [bridge-offline.md](bridge-offline.md) |
| BridgeDaemonDown | critical | `bridge_daemon_online` | [bridge-offline.md](bridge-offline.md) |
| XMLUploadFalha | warning | `xml_upload_errors_total` | [xml-upload-falha.md](xml-upload-falha.md) |
| ResetBancoExecutado | critical | `database_reset_total` | [reset-banco-executado.md](reset-banco-executado.md) |
| DBTamanhoAlto | warning | `pg_database_size_bytes` | [db-tamanho.md](db-tamanho.md) |

## Links de Monitoramento

- **Prometheus (alertas ativos):** http://localhost:9090/alerts
- **Prometheus (regras):** http://localhost:9090/rules
- **Alertmanager:** http://localhost:9093
- **Grafana:** http://localhost:3000
  - Dashboard: Bridge Runs
  - Dashboard: API Health
  - Dashboard: DB Size

## Regras de Alerta

As regras estão definidas em `monitoring/prometheus/rules/fiscal.yml`.

Grupos:
- `bridge_critico` (interval 15s): BridgeDPY4011Consecutivos, BridgeOffline, BridgeDaemonDown
- `ops_critico` (interval 15s): XMLUploadFalha, ResetBancoExecutado
- `db_critico` (interval 1m): DBTamanhoAlto

## Como Adicionar um Novo Runbook

1. Criar arquivo `docs/runbooks/<nome-do-alerta>.md` seguindo a estrutura:
   - Título: `# Runbook: <AlertName>`
   - Metadados: Severidade, Dispara quando, Detectado em, Alerta, Métrica
   - Seções: Sintomas, Causa Mais Provável, Passos de Mitigação, Verificação Pós-Mitigação, Escalar Para, Histórico
2. Adicionar linha na tabela de alertas acima
3. Adicionar regra em `monitoring/prometheus/rules/fiscal.yml` com `runbook_url: "/docs/runbooks/<nome-do-alerta>.md"`
4. Validar com: `docker run --rm -v $(pwd)/monitoring/prometheus:/etc/prometheus --entrypoint promtool prom/prometheus:v2.55.1 check rules /etc/prometheus/rules/fiscal.yml`
5. Commit no repositório

## Responsável Técnico

**claudio.bezerra@ferreiracosta.com.br** — Equipe Fiscal Ferreira Costa

---
*Runbooks internos FB_APU04 — equipe fiscal Ferreira Costa*
