-- 134_add_v_carga_to_cte_entradas.sql
--
-- Bancos herdados do APU02 criaram cte_entradas sem a coluna v_carga (valor da
-- carga do CT-e, de infCTeNorm/infCarga/vCarga). O parser de XML (processSingleCTe)
-- e o módulo Fronteira (frete) dependem dela — sem a coluna, todo INSERT de CT-e
-- via XML falha com "column v_carga does not exist". Idempotente.
ALTER TABLE cte_entradas ADD COLUMN IF NOT EXISTS v_carga NUMERIC(15,2) DEFAULT 0;
