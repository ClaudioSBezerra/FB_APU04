# Runbook: DBTamanhoAlto

**Severidade:** warning
**Dispara quando:** `pg_database_size_bytes{datname="fb_apu04"} > 50 * 1024 * 1024 * 1024` (> 50 GB)
**Detectado em:** 5 minutos sustentados acima do threshold (for: 5m — evitar falso positivo em import temporário)
**Alerta Prometheus:** DBTamanhoAlto
**Métrica:** `pg_database_size_bytes` (Gauge, postgres-exporter:9187/metrics)

---

## Sintomas

- Email de alerta `[FB_APU04][WARNING] DBTamanhoAlto` chegou
- Dashboard Grafana "DB Size" mostra banco `fb_apu04` acima de 50 GB
- Queries de relatório ficando mais lentas (SPED, apuração, conciliação)
- Backups do banco (pg_dump antes do reset) demorando mais do que o habitual

## Causa Mais Provável

1. **Crescimento natural** — volume de NF-e importados pelo Bridge ao longo do tempo
2. **Bulk import sem cleanup** — importação massiva de XMLs históricos sem deletar registros duplicados
3. **Table bloat** — tabelas com muitas linhas deletadas mas espaço não retornado ao SO (requer VACUUM FULL)
4. **Índices inchados** — índices B-tree fragmentados após muitas atualizações (REINDEX)
5. **Dados de saneamento não purgados** — tabela `ncm_cclasstrib_reforma` com 95 NCMs semeados (migration 079) — normalmente pequeno

## Passos de Mitigação

1. **Verificar tamanho atual do banco:**
   ```sql
   SELECT pg_size_pretty(pg_database_size('fb_apu04'));
   ```

2. **Identificar as tabelas que mais ocupam espaço:**
   ```sql
   SELECT
     schemaname,
     tablename,
     pg_size_pretty(pg_total_relation_size(schemaname || '.' || tablename)) AS total_size,
     pg_size_pretty(pg_relation_size(schemaname || '.' || tablename)) AS table_size,
     pg_size_pretty(pg_indexes_size(schemaname || '.' || tablename)) AS index_size
   FROM pg_tables
   WHERE schemaname = 'public'
   ORDER BY pg_total_relation_size(schemaname || '.' || tablename) DESC
   LIMIT 10;
   ```

3. **Verificar bloat das principais tabelas (linhas mortas):**
   ```sql
   SELECT relname, n_dead_tup, n_live_tup,
          ROUND(n_dead_tup::numeric / NULLIF(n_live_tup + n_dead_tup, 0) * 100, 1) AS dead_pct
   FROM pg_stat_user_tables
   WHERE n_dead_tup > 10000
   ORDER BY n_dead_tup DESC LIMIT 10;
   ```

4. **Executar VACUUM ANALYZE nas tabelas maiores (fora do horário fiscal):**
   ```sql
   VACUUM ANALYZE nfe_entradas;
   VACUUM ANALYZE nfe_saidas;
   VACUUM ANALYZE nfe_entradas_itens;
   VACUUM ANALYZE nfe_saidas_itens;
   VACUUM ANALYZE cte_entradas;
   VACUUM ANALYZE parceiros;
   VACUUM ANALYZE xml_upload_batches;
   ```

5. **Se bloat for muito alto (dead_pct > 20%), usar VACUUM FULL (bloqueia tabela):**
   ```sql
   -- ATENÇÃO: bloqueia writes. Executar fora do horário de uso (madrugada/fim de semana).
   VACUUM FULL ANALYZE <tabela_mais_inchada>;
   ```

6. **Avaliar arquivamento de dados antigos (> 2 anos) se crescimento for natural:**
   Discutir estratégia de archive com a equipe fiscal antes de executar deletions em massa.

## Verificação Pós-Mitigação

1. Tamanho do banco deve reduzir após VACUUM FULL:
   ```sql
   SELECT pg_size_pretty(pg_database_size('fb_apu04'));
   ```

2. Dashboard Grafana "DB Size" deve mostrar redução dentro de 5 minutos (próximo scrape).

3. `pg_database_size_bytes{datname="fb_apu04"}` no Prometheus deve cair abaixo de 50 GB.

4. Alerta resolve automaticamente quando tamanho ficar abaixo do threshold por 5 minutos.

## Escalar Para

- **claudio.bezerra@ferreiracosta.com.br:** Para decisões sobre archive ou particionamento
- **DBA PostgreSQL:** Se VACUUM FULL não resolver ou houver risco de indisponibilidade

---
*Runbook interno FB_APU04 — equipe fiscal Ferreira Costa*
