# Runbook: ResetBancoExecutado

**Severidade:** critical
**Dispara quando:** `increase(database_reset_total[5m]) > 0` (for: 0s — IMEDIATO)
**Detectado em:** até 30 segundos após o reset (scrape 15s + eval 15s + for 0s)
**Alerta Prometheus:** ResetBancoExecutado
**Métrica:** `database_reset_total` (Counter, api:8084/metrics)

---

## ACAO IMEDIATA — NAO AGUARDAR

Este alerta existe especificamente para detectar a repetição do **incidente de 2026-05-07**,
quando um clique em "limpar base" destruiu 4 meses de NF-e de entradas/saídas, CT-e e parceiros
do APU02. O ResetDatabaseHandler está protegido por 5 gates (STAB-01..05), mas SE este alerta
disparar, um reset foi de fato executado com sucesso. Agir imediatamente.

**Verificar agora:** `SELECT * FROM admin_destructive_actions ORDER BY executed_at DESC LIMIT 1;`

---

## Sintomas

- Email de alerta `[FB_APU04][CRITICAL] ResetBancoExecutado` chegou na caixa
- Usuários relatam dados zerados: "perdi os dados", "tabelas estão vazias", "nota não aparece mais"
- Painel de entradas/saídas retorna listas vazias
- Logs do backend mostram `ResetDatabase: user=... db=... backup=...`
- Counter `database_reset_total` incrementou pelo menos 1

## Causa Mais Provável

1. **Reset autorizado** — admin executou o reset intencionalmente pelo painel (5 gates passados: token DELETE-FB_APU04, pg_dump backup, audit log, role gate, rate-limit 1/h)
2. **Reset não autorizado** — alguém com acesso de admin executou reset sem autorização adequada (verificar audit log)
3. **Acesso comprometido** — credencial de admin vazou e terceiro executou o reset remotamente

## Passos de Mitigação

1. **IMEDIATO: Verificar quem executou e quando:**
   ```sql
   SELECT user_id, user_email, action, scope, status, client_ip, executed_at, error_message
   FROM admin_destructive_actions
   ORDER BY executed_at DESC LIMIT 5;
   ```

2. **Confirmar detalhes do usuário, IP e horário:**
   - O IP é o `client_ip` no registro acima
   - Verificar se o `user_email` é um admin legítimo
   - Verificar se o horário faz sentido (horário comercial vs. madrugada)

3. **Se reset NÃO autorizado — Resposta a Incidente:**
   - **Revogar tokens JWT imediatamente:**
     Alterar `JWT_SECRET` no Coolify → reiniciar serviço `api` (invalida todos os tokens)
   - **Suspender usuário comprometido:**
     ```sql
     UPDATE users SET active = false WHERE email = '<email_do_invasor>';
     ```
   - **Ler audit completo das últimas 24h:**
     ```sql
     SELECT * FROM admin_destructive_actions
     WHERE executed_at > NOW() - INTERVAL '24 hours'
     ORDER BY executed_at DESC;
     ```
   - **Abrir post-mortem imediatamente** (notificar gerência)

4. **Restaurar backup do banco:**
   O backup é criado automaticamente pelo ResetDatabaseHandler antes do truncate:
   ```
   docker compose exec api ls -la /backups/
   ```
   O arquivo segue o padrão `/backups/reset-{timestamp}.sql`. Para restaurar:
   ```
   docker compose exec api psql $DATABASE_URL < /backups/reset-<timestamp>.sql
   ```
   Ou via host:
   ```
   docker compose cp api:/backups/reset-<timestamp>.sql ./backup_restore.sql
   docker compose exec db psql -U $DB_USER $DB_NAME < /backup_restore.sql
   ```

5. **Verificar integridade após restauração:**
   ```sql
   SELECT
     (SELECT COUNT(*) FROM nfe_entradas) AS nfe_entradas,
     (SELECT COUNT(*) FROM nfe_saidas)   AS nfe_saidas,
     (SELECT COUNT(*) FROM cte_entradas) AS cte_entradas,
     (SELECT COUNT(*) FROM parceiros)    AS parceiros;
   ```
   Comparar com snapshot pré-reset (se disponível) ou com último relatório gerado.

6. **Abrir post-mortem documentando:**
   - Quem executou, de qual IP, em qual horário
   - Se autorizado ou não autorizado
   - Tempo de detecção (alerta chegou em < 30s?)
   - Tempo de restauração
   - Ações preventivas para evitar repetição

## Verificação Pós-Mitigação

1. Contagens de tabelas batem com snapshot pré-reset:
   ```sql
   SELECT COUNT(*) FROM nfe_entradas;
   SELECT COUNT(*) FROM nfe_saidas;
   SELECT COUNT(*) FROM parceiros;
   ```

2. Painel de entradas/saídas mostra dados novamente

3. Usuários confirmam que dados estão presentes

4. Backup `/backups/reset-<timestamp>.sql` mantido por auditoria (não deletar)

## Escalar Para

- **Gerência imediata:** Se reset não autorizado ou dados não recuperados
- **claudio.bezerra@ferreiracosta.com.br:** Responsável técnico — acionar imediatamente
- **Suporte Coolify/AWS:** Se restore requer acesso ao servidor de produção

## Histórico de Incidentes

| Data | Descrição | Resolução |
|------|-----------|-----------|
| **2026-05-07** | APU04 apontava para banco APU02. Reset executado apagou 4 meses de NF-e de entradas/saídas, CT-e e parceiros de PRODUÇÃO do APU02. | Separação total de infraestrutura (commits 90d1b93, 947de42, 14b455b). Implementação dos 5 gates STAB-01..05 em Phase 1. |

Este alerta existe exatamente para garantir que o incidente de 2026-05-07 nunca passe
despercebido por mais de 30 segundos. Se disparou, agir agora.

---
*Runbook interno FB_APU04 — equipe fiscal Ferreira Costa*
