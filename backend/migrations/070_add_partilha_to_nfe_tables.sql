-- 070_add_partilha_to_nfe_tables.sql
-- Adiciona base_partilha e icms_partilha em nfe_saidas e nfe_entradas.
-- Estas colunas são necessárias pelo INSERT do ERP Bridge (erp_bridge_batch.go)
-- desde o commit 81195c5 do APU02. A migration 066 do APU04 adicionou base_icms..cofins
-- mas omitiu essas duas colunas de partilha. Sem elas, todo INSERT falha com
-- "column base_partilha does not exist".
-- Idempotente via IF NOT EXISTS — seguro mesmo se migration 103 do APU02 já rodou.

ALTER TABLE nfe_saidas
  ADD COLUMN IF NOT EXISTS base_partilha NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS icms_partilha NUMERIC(15,2) DEFAULT 0;

ALTER TABLE nfe_entradas
  ADD COLUMN IF NOT EXISTS base_partilha NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS icms_partilha NUMERIC(15,2) DEFAULT 0;
