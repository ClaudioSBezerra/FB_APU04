-- 118_create_uf_beneficios_fiscais.sql
-- Benefícios/parâmetros fiscais por UF (eixo UF do módulo de fronteira).
-- Cada empresa cadastra, por UF onde tem filial, os parâmetros que o estado
-- aplica na entrada interestadual: alíquota interna, FECP, redução de base de
-- cálculo, MVA padrão, e flags de inaplicabilidade de ST / antecipação.
--
-- É a parte "manual" do hub por UF. A parte "automática" (decretos/RICMS
-- interpretados pela IA) continua em legislacao_fronteira → icms_fronteira_regras_ncm.
-- A UF de cada filial vem do reg 0000 do SPED (import_jobs.uf).

CREATE TABLE IF NOT EXISTS uf_beneficios_fiscais (
    id                     UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id             UUID         NOT NULL,
    uf                     VARCHAR(2)   NOT NULL,
    aliquota_interna       NUMERIC(5,2),              -- alíquota interna da UF (ex: 18.00)
    fecp_percentual        NUMERIC(5,2),              -- Fundo Estadual de Combate à Pobreza (%)
    reducao_bc_percentual  NUMERIC(5,2),              -- redução de base de cálculo (%)
    mva_ajustada_padrao    NUMERIC(6,2),              -- MVA padrão quando não há regra NCM específica
    inaplicabilidade_st    BOOLEAN      NOT NULL DEFAULT false, -- ST não se aplica nesta UF
    antecipacao_aplicavel  BOOLEAN      NOT NULL DEFAULT true,  -- regime de antecipação aplicável
    observacoes            TEXT,
    created_at             TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT uq_uf_beneficios UNIQUE (company_id, uf)
);

CREATE INDEX IF NOT EXISTS idx_uf_beneficios_company
    ON uf_beneficios_fiscais(company_id);

COMMENT ON TABLE uf_beneficios_fiscais IS
    'Parâmetros/benefícios fiscais por UF (manual) do módulo de fronteira. '
    'Complementa a legislação interpretada por IA (legislacao_fronteira).';
