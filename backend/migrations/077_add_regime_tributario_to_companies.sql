-- 077_add_regime_tributario_to_companies.sql
-- Adiciona coluna regime_tributario à tabela companies (per D-03).
--
-- O regime tributário determina como os XMLs são classificados e qual
-- a base de cálculo aplicável (Simples Nacional usa CSOSN; demais regimes usam CST).
--
-- Idempotente: ADD COLUMN IF NOT EXISTS + DO block para constraint.

ALTER TABLE companies
    ADD COLUMN IF NOT EXISTS regime_tributario TEXT NOT NULL DEFAULT 'nao_informado';

DO $$ BEGIN
    ALTER TABLE companies ADD CONSTRAINT chk_companies_regime_tributario
        CHECK (regime_tributario IN (
            'lucro_real',
            'lucro_presumido',
            'simples_nacional',
            'nao_informado'
        ));
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

COMMENT ON COLUMN companies.regime_tributario IS
    'Regime tributário da empresa: lucro_real, lucro_presumido, simples_nacional, nao_informado. '
    'Determina como os XMLs são classificados (CST vs CSOSN) e as bases de cálculo aplicáveis.';
