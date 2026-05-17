-- 074_add_source_to_nfe_tables.sql
-- Adiciona coluna `source` em nfe_entradas, nfe_saidas e cte_entradas.
--
-- Propósito: diferenciar a origem dos documentos fiscais (per D-17, D-12):
--   oracle_bridge → importado via ERP Bridge (dados históricos)
--   xml_upload    → importado manualmente via upload de XML (Phase 02)
--   manual        → lançamento manual (uso futuro)
--
-- Idempotente: ADD COLUMN IF NOT EXISTS + DO block para constraint.

-- ── nfe_entradas ──────────────────────────────────────────────────────────────
ALTER TABLE nfe_entradas
    ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'oracle_bridge';

DO $$ BEGIN
    ALTER TABLE nfe_entradas ADD CONSTRAINT chk_nfe_entradas_source
        CHECK (source IN ('oracle_bridge', 'xml_upload', 'manual'));
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE INDEX IF NOT EXISTS idx_nfe_entradas_source
    ON nfe_entradas(company_id, source);

-- ── nfe_saidas ────────────────────────────────────────────────────────────────
ALTER TABLE nfe_saidas
    ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'oracle_bridge';

DO $$ BEGIN
    ALTER TABLE nfe_saidas ADD CONSTRAINT chk_nfe_saidas_source
        CHECK (source IN ('oracle_bridge', 'xml_upload', 'manual'));
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE INDEX IF NOT EXISTS idx_nfe_saidas_source
    ON nfe_saidas(company_id, source);

-- ── cte_entradas ──────────────────────────────────────────────────────────────
ALTER TABLE cte_entradas
    ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'oracle_bridge';

DO $$ BEGIN
    ALTER TABLE cte_entradas ADD CONSTRAINT chk_cte_entradas_source
        CHECK (source IN ('oracle_bridge', 'xml_upload', 'manual'));
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE INDEX IF NOT EXISTS idx_cte_entradas_source
    ON cte_entradas(company_id, source);
