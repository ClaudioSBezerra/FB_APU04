# Runbook: BridgeDPY4011Consecutivos

**Severidade:** critical
**Dispara quando:** `increase(bridge_dpy4011_total[5m]) >= 3`
**Detectado em:** até 60 segundos após o 3º erro (scrape 15s + eval 15s + for 30s)
**Alerta Prometheus:** BridgeDPY4011Consecutivos
**Métrica:** `bridge_dpy4011_total` (Counter, bridge:8086/metrics)

---

## Sintomas

- Email de alerta `[FB_APU04][CRITICAL] BridgeDPY4011Consecutivos` chegou na caixa
- Logs do bridge mostram linhas como `DPY-4011` ou `DPY-4011: the database or network closed the connection`
- Runs recentes no painel `/api/erp-bridge/runs` com `status=error` e `erro_msg` contendo "DPY-4011"
- Dashboard Grafana "Bridge Runs" mostra spike de erros
- Bridge pode estar tentando reconectar mas sem sucesso

## Causa Mais Provável

1. **Rede Oracle instável** — o servidor Oracle (on-prem Ferreira Costa) derrubou a conexão por timeout ou instabilidade de rede
2. **Listener Oracle reiniciado** — listener TNS foi reiniciado sem avisar o bridge
3. **Firewall reset** — sessão TCP entre bridge e Oracle foi derrubada pelo firewall por inatividade
4. **Credenciais expiradas** — senha do usuário Oracle expirou (política de senha periódica)
5. **Oracle em manutenção** — DBA aplicou patch ou reiniciou instância sem comunicar
6. **Bridge sem retry efetivo** — o mecanismo de reconexão tentou mas falhou (verificar logs)

## Passos de Mitigação

1. **Verificar logs do bridge para DPY-4011:**
   ```
   docker compose logs --tail 200 bridge | grep -i "DPY-4011\|dpy4011\|error"
   ```

2. **Verificar quantos erros foram registrados nos últimos runs:**
   ```
   SELECT id, status, total_erros, erro_msg, created_at
   FROM erp_bridge_runs
   WHERE created_at > NOW() - INTERVAL '30 minutes'
   ORDER BY created_at DESC LIMIT 10;
   ```

3. **Testar conectividade de rede com o servidor Oracle:**
   ```
   docker compose exec bridge ping -c 4 <ORACLE_HOST>
   ```

4. **Verificar se o listener Oracle responde (dentro do container bridge):**
   ```
   docker compose exec bridge nc -zv <ORACLE_HOST> <ORACLE_PORT>
   ```
   Substitua `<ORACLE_HOST>` e `<ORACLE_PORT>` pelos valores em `bridge.conf` ou `.env`.

5. **Se rede OK, testar conexão Oracle diretamente:**
   Entre em contato com o DBA da Ferreira Costa para verificar:
   - Status do listener: `lsnrctl status`
   - Status da instância: `SELECT STATUS FROM V$INSTANCE;`
   - Logs de alerta do Oracle: `alert_<sid>.log`

6. **Reiniciar o container bridge para forçar reconexão:**
   ```
   docker compose restart bridge
   docker compose logs --tail 50 bridge
   ```

7. **Confirmar que o mecanismo de retry (reconexão) está ativo:**
   Verificar em `bridge.py` se a função `_is_dpy4011` está sendo chamada e se o retry
   está incrementando `_query_retry`. Se o retry não funcionar, o bridge será reiniciado
   pelo Docker `restart: always` automaticamente.

## Verificação Pós-Mitigação

1. O counter `bridge_dpy4011_total` deve parar de crescer:
   ```
   curl -s http://localhost:9090/api/v1/query?query=increase(bridge_dpy4011_total[5m])
   ```
   Esperar resultado próximo de 0.

2. O próximo run do bridge deve retornar `status=success` ou `status=partial`:
   ```
   SELECT status, total_enviados, total_erros, created_at
   FROM erp_bridge_runs
   ORDER BY created_at DESC LIMIT 3;
   ```

3. Dashboard Grafana "Bridge Runs" deve mostrar run bem-sucedido dentro de 60 minutos.

4. O alerta Prometheus deve resolver automaticamente quando `increase(bridge_dpy4011_total[5m]) < 3`.

## Escalar Para

- **DBA Oracle:** Se o problema for no servidor Oracle (listener, instância, rede on-prem)
- **Equipe de TI Ferreira Costa:** Para questões de firewall ou rede entre AWS e on-prem
- **claudio.bezerra@ferreiracosta.com.br:** Responsável pela operação do Bridge

## Histórico de Incidentes

| Data | Descrição | Resolução |
|------|-----------|-----------|
| Recorrente | Oracle derruba conexão por inatividade após 60min | Restart do bridge / reconexão automática |

---
*Runbook interno FB_APU04 — equipe fiscal Ferreira Costa*
