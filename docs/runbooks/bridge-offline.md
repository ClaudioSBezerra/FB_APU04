# Runbook: BridgeOffline / BridgeDaemonDown

**Severidade:** critical
**Dispara quando:**
- `BridgeOffline`: `time() - bridge_last_run_timestamp_seconds > 3900` (> 65 minutos sem run)
- `BridgeDaemonDown`: `bridge_daemon_online == 0 OR up{job="fb_apu04_bridge"} == 0`
**Detectado em:** imediato para BridgeOffline (for: 0s) | 1 minuto para BridgeDaemonDown (for: 1m)
**Métrica:** `bridge_last_run_timestamp_seconds`, `bridge_daemon_online` (bridge:8086/metrics)

---

## Sintomas

- Email de alerta `[FB_APU04][CRITICAL] BridgeOffline` ou `BridgeDaemonDown` chegou
- Dashboard Grafana "Bridge Runs" mostra "Daemon offline" ou sem novos registros
- Painel de entradas não está recebendo novas notas do ERP (NF-e/CT-e Oracle)
- Tabela `erp_bridge_runs` não tem registros novos há mais de 65 minutos
- Endpoint `/api/erp-bridge/heartbeat` não recebe chamadas do bridge

## Causa Mais Provável

1. **Container bridge parou inesperadamente** — OOM killer, crash de Python, exceção não tratada
2. **Conexão Oracle perdida sem retry efetivo** — DPY-4011 repetido além do limite de retries
3. **Kill manual** — container foi parado manualmente (`docker compose stop bridge`)
4. **Falha no servidor AWS** — máquina reiniciou ou ficou sem disco
5. **Porta 8086 não acessível** — rede Docker `fb_net` reconfigured ou bridge fora da rede
6. **Redeploy em andamento** — Watchtower está atualizando o container (falso positivo, aguardar 2min)

## Passos de Mitigação

1. **Verificar status do container:**
   ```
   docker compose ps bridge
   ```
   Se `Status: exited` → passou por crash. Ver passo 2.
   Se `Status: running` → pode ser problema de métricas ou loop travado. Ver passo 3.

2. **Verificar logs do container (últimos 100 linhas):**
   ```
   docker compose logs --tail 100 bridge
   ```
   Procurar por: `Exception`, `Error`, `ORA-`, `DPY-`, `ConnectionError`, `Killed`.

3. **Verificar se o endpoint de métricas responde:**
   ```
   curl -s http://localhost:8086/metrics | head -20
   ```
   Se não responder → bridge está fora da rede ou port não exposto.

4. **Reiniciar o container bridge:**
   ```
   docker compose restart bridge
   ```
   Aguardar 30 segundos e verificar logs novamente:
   ```
   docker compose logs --tail 50 bridge
   ```

5. **Confirmar que o heartbeat está sendo enviado:**
   ```sql
   SELECT * FROM erp_bridge_runs ORDER BY created_at DESC LIMIT 5;
   ```
   Novo run deve aparecer dentro de 60 minutos após o restart.

6. **Se bridge não sobe após restart, fazer redeploy:**
   ```
   docker compose pull bridge
   docker compose up -d bridge
   ```

## Verificação Pós-Mitigação

1. `bridge_daemon_online` deve ser 1:
   ```
   curl -s 'http://localhost:9090/api/v1/query?query=bridge_daemon_online'
   ```

2. `bridge_last_run_timestamp_seconds` deve atualizar no próximo ciclo (até 60 min):
   ```
   curl -s 'http://localhost:9090/api/v1/query?query=time()-bridge_last_run_timestamp_seconds'
   ```
   Valor em segundos deve ser < 3900 (65 minutos).

3. Alerta resolve automaticamente quando `bridge_daemon_online == 1` e `time() - last_run < 3900`.

## Escalar Para

- **claudio.bezerra@ferreiracosta.com.br:** Se container não sobe após restart
- **Suporte AWS:** Se problema for no servidor (disco cheio, OOM, instância parada)

---
*Runbook interno FB_APU04 — equipe fiscal Ferreira Costa*
