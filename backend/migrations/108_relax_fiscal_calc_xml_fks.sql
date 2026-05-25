-- Motor Fiscal — relaxa dependência do XML
-- ------------------------------------------
-- Itens vindos do SPED (reg_c170) podem não ter XML correspondente
-- importado (ex: NFs de meses anteriores listadas no SPED mas cujo XML
-- não foi baixado). Tornamos nfe_id e item_id NULLABLE para que o
-- cálculo aceite itens "SPED-only" — frete da NF fica 0 (sem dado XML
-- de v_frete), mas o IPI/V.Item/ICMS do C170 ainda são utilizados.

ALTER TABLE fiscal_calculations
    ALTER COLUMN nfe_id  DROP NOT NULL,
    ALTER COLUMN item_id DROP NOT NULL;

-- Adiciona colunas opcionais para identificar a origem SPED quando não
-- há vínculo XML — facilita auditoria pelo contador.
ALTER TABLE fiscal_calculations
    ADD COLUMN IF NOT EXISTS sped_c170_id UUID,
    ADD COLUMN IF NOT EXISTS sped_c100_id UUID;

-- Substituímos a unique por uma que aceita sped_c170_id quando item_id é NULL.
-- Para preservar idempotência: chave = COALESCE(item_id, sped_c170_id) + fase
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname='uq_fiscal_calc_item_fase') THEN
        ALTER TABLE fiscal_calculations DROP CONSTRAINT uq_fiscal_calc_item_fase;
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_fiscal_calc_origem_fase
    ON fiscal_calculations (COALESCE(item_id::text, sped_c170_id::text), fase);
