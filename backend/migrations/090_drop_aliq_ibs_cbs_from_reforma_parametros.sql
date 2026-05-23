-- Migration 090: Remove aliq_ibs_pct e aliq_cbs_pct de reforma_parametros
--
-- As alíquotas IBS e CBS passam a ser derivadas de tabela_aliquotas usando
-- target_ano como chave. Isso elimina duplicação e garante que os valores
-- reflitam sempre a tabela oficial da transição (EC 132/2023).

ALTER TABLE reforma_parametros
    DROP COLUMN IF EXISTS aliq_ibs_pct,
    DROP COLUMN IF EXISTS aliq_cbs_pct;
