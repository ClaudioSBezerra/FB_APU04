-- 101_create_classificacao_manual.sql
-- Etapa 2b da Reconciliação SPED×XML: persistir a validação manual da
-- classificação do bloco "Faltando" (notas no XML ausentes do SPED).
--
-- A classificação inicial é automática (CFOP saída→entrada). O usuário pode:
--   - validar a sugestão (status='manual', regime mantido)
--   - sobrescrever o regime (status='manual', regime editado)
--   - excluir a nota do cálculo (status='excluded')
--
-- Uma vez gravada, a UI deixa de mostrar como "sugestão"; passa a respeitar
-- a decisão humana. Auditoria via validated_by + validated_at.

CREATE TABLE IF NOT EXISTS icms_fronteira_classificacao_manual (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id    UUID         NOT NULL,
    chave_nfe     VARCHAR(44)  NOT NULL,
    regime        VARCHAR(20)  NOT NULL
                  CHECK (regime IN ('ANTECIPACAO','ST','DIFAL','NAO_FRONTEIRA')),
    status        VARCHAR(20)  NOT NULL DEFAULT 'manual'
                  CHECK (status IN ('manual','excluded')),
    notes         TEXT,
    validated_by  UUID,
    validated_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT uq_classif_manual UNIQUE (company_id, chave_nfe)
);

CREATE INDEX IF NOT EXISTS idx_classif_manual_company
    ON icms_fronteira_classificacao_manual(company_id);

COMMENT ON TABLE icms_fronteira_classificacao_manual IS
    'Validação manual da classificação de notas no bloco "Faltando" da reconciliação SPED×XML. '
    'Substitui a classificação automática (CFOP saída→entrada) quando presente.';

COMMENT ON COLUMN icms_fronteira_classificacao_manual.status IS
    'manual = nota validada/editada para entrar no cálculo; '
    'excluded = nota explicitamente removida do cálculo do mês.';
