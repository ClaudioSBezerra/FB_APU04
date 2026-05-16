-- 074_add_source_to_nfe_tables.sql
-- Adiciona coluna `source` em nfe_entradas, nfe_saidas e cte_entradas.
--
-- Propósito: diferenciar a origem dos documentos fiscais (per D-17, D-12):
--   oracle_bridge → importado via ERP Bridge (dados históricos)
--   xml_upload    → importado manualmente via upload de XML (Phase 02)
--   manual        → lançamento manual (uso futuro)
--
-- PostgreSQL 15+: ADD COLUMN com DEFAULT em tabela existente é operação virtual
-- (não reescreve linhas), portanto segura em produção sem lock prolongado.
-- Idempotente via ADD COLUMN IF NOT EXISTS e ADD CONSTRAINT IF NOT EXISTS.

-- ── nfe_entradas ──────────────────────────────────────────────────────────────
ALTER TABLE nfe_entradas
    ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'oracle_bridge';

ALTER TABLE nfe_entradas
    ADD CONSTRAINT IF NOT EXISTS chk_nfe_entradas_source
        CHECK (source IN ('oracle_bridge', 'xml_upload', 'manual'));

CREATE INDEX IF NOT EXISTS idx_nfe_entradas_source
    ON nfe_entradas(company_id, source);

COMMENT ON COLUMN nfe_entradas.source IS
    'Origem do documento: oracle_bridge (ERP), xml_upload (Phase 02), manual';

-- ── nfe_saidas ────────────────────────────────────────────────────────────────
ALTER TABLE nfe_saidas
    ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'oracle_bridge';

ALTER TABLE nfe_saidas
    ADD CONSTRAINT IF NOT EXISTS chk_nfe_saidas_source
        CHECK (source IN ('oracle_bridge', 'xml_upload', 'manual'));

CREATE INDEX IF NOT EXISTS idx_nfe_saidas_source
    ON nfe_saidas(company_id, source);

COMMENT ON COLUMN nfe_saidas.source IS
    'Origem do documento: oracle_bridge (ERP), xml_upload (Phase 02), manual';

-- ── cte_entradas ──────────────────────────────────────────────────────────────
ALTER TABLE cte_entradas
    ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'oracle_bridge';

ALTER TABLE cte_entradas
    ADD CONSTRAINT IF NOT EXISTS chk_cte_entradas_source
        CHECK (source IN ('oracle_bridge', 'xml_upload', 'manual'));

CREATE INDEX IF NOT EXISTS idx_cte_entradas_source
    ON cte_entradas(company_id, source);

COMMENT ON COLUMN cte_entradas.source IS
    'Origem do documento: oracle_bridge (ERP), xml_upload (Phase 02), manual';
