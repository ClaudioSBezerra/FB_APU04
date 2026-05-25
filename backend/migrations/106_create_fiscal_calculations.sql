-- Motor de Cálculo Fiscal — Fase 1 (Substituição Tributária BA)
-- ---------------------------------------------------------------
-- Persiste cada cálculo ST item-a-item para auditoria.
-- Fórmula aplicada (CFOP 2403, destino BA):
--   Base ST = (V.Item + IPI + Frete Proporcional + Frete CT-e Rateado + Outras Desp.)
--             * (1 + MVA/100)
--   ICMS ST = Base ST * Alíq.Interna% − V.ICMS destacado no item
-- A persistência permite ao usuário auditar produto a produto e
-- reexecutar o cálculo quando regras forem alteradas.

CREATE TABLE IF NOT EXISTS fiscal_calculations (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id               UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,

    -- Origem
    nfe_id                   UUID NOT NULL REFERENCES nfe_entradas(id) ON DELETE CASCADE,
    item_id                  UUID NOT NULL REFERENCES nfe_entradas_itens(id) ON DELETE CASCADE,
    chave_nfe                VARCHAR(44) NOT NULL,
    numero_nfe               VARCHAR(20),
    data_emissao             DATE NOT NULL,
    n_item                   SMALLINT NOT NULL,
    cfop                     VARCHAR(4) NOT NULL,
    ncm                      VARCHAR(8),
    cst_icms                 VARCHAR(3),
    dest_uf                  VARCHAR(2) NOT NULL,
    forn_uf                  VARCHAR(2),

    -- Componentes da base
    v_item                   NUMERIC(15,2) NOT NULL DEFAULT 0,
    v_ipi                    NUMERIC(15,2) NOT NULL DEFAULT 0,
    v_frete_proporcional     NUMERIC(15,2) NOT NULL DEFAULT 0,  -- rateado do v_frete da NF
    v_frete_cte_rateado      NUMERIC(15,2) NOT NULL DEFAULT 0,  -- rateado do CT-e do destinatário
    v_outras_desp            NUMERIC(15,2) NOT NULL DEFAULT 0,
    v_icms_item              NUMERIC(15,2) NOT NULL DEFAULT 0,  -- destacado no item

    -- Inteligência aplicada
    ncm_regra_id             UUID REFERENCES icms_fronteira_regras_ncm(id) ON DELETE SET NULL,
    ncm_prefixo_aplicado     VARCHAR(8),
    mva_aplicada             NUMERIC(7,2),
    mva_tipo                 VARCHAR(20),  -- 'ajustada_4pct' | 'ajustada_7pct' | 'ajustada_12pct' | 'original'
    aliq_inter               NUMERIC(5,2) NOT NULL DEFAULT 0,
    aliq_interna             NUMERIC(5,2) NOT NULL DEFAULT 0,

    -- Resultado
    base_st                  NUMERIC(15,2) NOT NULL DEFAULT 0,
    icms_st_estimado         NUMERIC(15,2) NOT NULL DEFAULT 0,

    -- Auditoria
    fase                     VARCHAR(40) NOT NULL DEFAULT 'F1_ST_BA',
    created_at               TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    updated_at               TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),

    CONSTRAINT uq_fiscal_calc_item_fase UNIQUE (item_id, fase)
);

CREATE INDEX IF NOT EXISTS idx_fiscal_calc_company  ON fiscal_calculations(company_id);
CREATE INDEX IF NOT EXISTS idx_fiscal_calc_data     ON fiscal_calculations(company_id, data_emissao);
CREATE INDEX IF NOT EXISTS idx_fiscal_calc_chave    ON fiscal_calculations(chave_nfe);
CREATE INDEX IF NOT EXISTS idx_fiscal_calc_destuf   ON fiscal_calculations(dest_uf, cfop);
