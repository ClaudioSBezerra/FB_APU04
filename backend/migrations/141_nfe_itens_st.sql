-- 141_nfe_itens_st.sql
--
-- ICMS-ST por ITEM (vBCST / vICMSST do grupo ICMS de cada det da NF-e). Necessário
-- para o demonstrativo de Substituição Tributária POR ITEM (Bloco C / XML): o
-- cabeçalho da NF (nfe_entradas.v_st) só tem o total da nota. NULL = item importado
-- antes desta coluna (ver reimport).

ALTER TABLE nfe_entradas_itens
    ADD COLUMN IF NOT EXISTS v_bc_st NUMERIC,
    ADD COLUMN IF NOT EXISTS v_st    NUMERIC;

ALTER TABLE nfe_saidas_itens
    ADD COLUMN IF NOT EXISTS v_bc_st NUMERIC,
    ADD COLUMN IF NOT EXISTS v_st    NUMERIC;
