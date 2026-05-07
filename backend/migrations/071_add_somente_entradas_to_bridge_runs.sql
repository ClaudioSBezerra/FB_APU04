-- 071_add_somente_entradas_to_bridge_runs.sql
-- Adiciona flag somente_entradas em erp_bridge_runs.
-- Quando true, o daemon Bridge importa apenas NF-e de entrada (DIRECT=1),
-- ignorando NF-e de saída (DIRECT=2). Útil para APU04 que não precisa de saídas.

ALTER TABLE erp_bridge_runs
  ADD COLUMN IF NOT EXISTS somente_entradas BOOLEAN NOT NULL DEFAULT FALSE;
