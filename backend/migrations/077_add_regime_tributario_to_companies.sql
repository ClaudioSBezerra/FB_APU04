-- 077_add_regime_tributario_to_companies.sql
-- Adiciona coluna regime_tributario à tabela companies (per D-03).
--
-- O regime tributário determina como os XMLs são classificados e qual
-- a base de cálculo aplicável (Simples Nacional usa CSOSN; demais regimes usam CST).
--
-- DEFAULT 'nao_informado' para empresas já cadastradas sem o campo.
-- Idempotente via ADD COLUMN IF NOT EXISTS.

ALTER TABLE companies
    ADD COLUMN IF NOT EXISTS regime_tributario TEXT NOT NULL DEFAULT 'nao_informado';

ALTER TABLE companies
    ADD CONSTRAINT IF NOT EXISTS chk_companies_regime_tributario
        CHECK (regime_tributario IN (
            'lucro_real',
            'lucro_presumido',
            'simples_nacional',
            'nao_informado'
        ));

COMMENT ON COLUMN companies.regime_tributario IS
    'Regime tributário da empresa: lucro_real, lucro_presumido, simples_nacional, nao_informado. '
    'Determina como os XMLs são classificados (CST vs CSOSN) e as bases de cálculo aplicáveis.';
