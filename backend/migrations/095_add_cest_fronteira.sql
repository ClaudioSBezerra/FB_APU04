-- 095_add_cest_fronteira.sql
-- Bloco 2: adiciona coluna cest (Código Especificador da Substituição Tributária)
-- às tabelas de itens de NF-e para suporte ao cálculo por item no ICMS Fronteira.

ALTER TABLE nfe_entradas_itens ADD COLUMN IF NOT EXISTS cest VARCHAR(7);
ALTER TABLE nfe_saidas_itens    ADD COLUMN IF NOT EXISTS cest VARCHAR(7);
