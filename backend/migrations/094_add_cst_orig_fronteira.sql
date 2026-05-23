-- Migration 094: Origem da mercadoria (CST Tabela A) para cálculo da alíquota interestadual
--
-- Tabela A do CST:
--   0 = Nacional               → 7% (Sul/SE) ou 12% (demais)
--   1 = Estrangeira direta     → 4%
--   2 = Estrangeira mercado interno → 4%
--   3 = Nacional c/ import >40%≤70% → 4%
--   4 = Nacional processo básico    → 7%/12% (origem nacional)
--   5 = Nacional c/ import ≤40%    → 7%/12%
--   6 = Estrangeira s/ similar (Camex) → 4%
--   7 = Estrangeira s/ similar (mercado interno) → 4%
--   8 = Nacional c/ import >70%    → 4%

ALTER TABLE nfe_entradas_itens
    ADD COLUMN IF NOT EXISTS cst_orig VARCHAR(1);

ALTER TABLE nfe_saidas_itens
    ADD COLUMN IF NOT EXISTS cst_orig VARCHAR(1);

-- cst_orig_pred: código de origem mais "estrangeiro" encontrado em qualquer item da NF
-- Preenchido no upload de XML; NULL = não processado ou todos nacionais
ALTER TABLE nfe_entradas
    ADD COLUMN IF NOT EXISTS cst_orig_pred VARCHAR(1);

COMMENT ON COLUMN nfe_entradas.cst_orig_pred IS
    'Origem predominante (CST Tab A) dos itens: 1,2,3,6,7,8 → alíquota interestadual 4%';
