-- 112_create_legislacao_regras_staging.sql
-- Etapa 5 (cont.): staging de regras extraídas por micro-chunk.
--
-- O free-tier Z.AI não atende requests grandes (timeout "awaiting headers").
-- Estratégia: quebrar o texto filtrado em grupos pequenos de linhas, chamar a
-- IA por grupo (request minúsculo = mais chance de ser atendido), e inserir as
-- regras extraídas AQUI a cada grupo. Assim o progresso parcial é durável: se
-- a goroutine morrer no meio, as regras já extraídas permanecem. Ao final, o
-- worker consolida (dedup por NCM) daqui para legislacao_fronteira.interpretacao
-- e limpa estas linhas.

CREATE TABLE IF NOT EXISTS legislacao_regras_staging (
    id               BIGSERIAL    PRIMARY KEY,
    legislacao_id    UUID         NOT NULL,
    company_id       UUID         NOT NULL,
    chunk_idx        INTEGER      NOT NULL,
    ncm              VARCHAR(20),
    regime           VARCHAR(20),
    descricao        TEXT,
    aliquota_interna NUMERIC(7,2),
    mva_original     NUMERIC(8,4),
    mva_4pct         NUMERIC(8,4),
    mva_7pct         NUMERIC(8,4),
    mva_12pct        NUMERIC(8,4),
    justificativa    TEXT,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_legislacao_staging_leg
    ON legislacao_regras_staging(legislacao_id);

COMMENT ON TABLE legislacao_regras_staging IS
    'Buffer durável de regras extraídas por micro-chunk de legislação. O worker '
    'insere aqui a cada grupo de linhas processado pela IA e, ao concluir, '
    'consolida (dedup por NCM) em legislacao_fronteira.interpretacao e limpa. '
    'Linhas remanescentes indicam processamento interrompido (reinício do worker).';
