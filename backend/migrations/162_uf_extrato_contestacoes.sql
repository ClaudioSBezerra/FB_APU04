-- 162_uf_extrato_contestacoes.sql
-- icms_fronteira_extrato_sefaz e icms_fronteira_contestacoes nasceram
-- pensadas só para PE (comentários originais: "Extrato SEFAZ-PE",
-- "contestações junto à SEFAZ-PE") e nunca tiveram uma coluna de UF —
-- empresas com filiais em mais de uma UF (ex.: PE + BA + PA) não tinham como
-- separar o extrato/contestação de uma UF da de outra.
--
-- uf em extrato_sefaz: UF de destino cujo extrato SEFAZ está sendo
-- comparado (informada no upload, pois o CSV/XLSX da SEFAZ não traz essa
-- informação estruturada).
ALTER TABLE icms_fronteira_extrato_sefaz ADD COLUMN IF NOT EXISTS uf VARCHAR(2);
CREATE INDEX IF NOT EXISTS idx_icms_extrato_company_uf ON icms_fronteira_extrato_sefaz(company_id, uf);

-- uf em contestacoes: UF de destino da nota contestada (derivada da nota via
-- chave_nfe quando disponível; ver icms_fronteira_contestacoes.go).
ALTER TABLE icms_fronteira_contestacoes ADD COLUMN IF NOT EXISTS uf VARCHAR(2);
CREATE INDEX IF NOT EXISTS idx_icms_contestacoes_uf ON icms_fronteira_contestacoes(company_id, uf);
