-- Frete por item (<det><prod><vFrete> do XML) — alimenta pDespesas do pacote
-- fiscal junto com v_outro (despesas acessórias compõem a base de cálculo).
-- Itens importados antes desta migration ficam com 0; reimportar o XML
-- atualiza (upsert por nfe_id+n_item).
ALTER TABLE pacotefiscal_nfe_saidas_itens
    ADD COLUMN IF NOT EXISTS v_frete NUMERIC(15,2) DEFAULT 0;
