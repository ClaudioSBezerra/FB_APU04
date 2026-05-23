-- 093_icms_fronteira_contestacoes.sql
-- Registra contestações de ICMS Fronteira junto à SEFAZ-PE.

CREATE TABLE IF NOT EXISTS icms_fronteira_contestacoes (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id       UUID         NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    chave_nfe        VARCHAR(44),
    numero_nf        VARCHAR(20),
    forn_cnpj        VARCHAR(14),
    forn_nome        VARCHAR(200),
    periodo          VARCHAR(7),
    valor_contestado NUMERIC(15,2),
    motivo           TEXT         NOT NULL,
    status           VARCHAR(20)  NOT NULL DEFAULT 'pendente'
                     CHECK (status IN ('pendente','enviada','deferida','indeferida','cancelada')),
    resposta_sefaz   TEXT,
    data_registro    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    data_resposta    TIMESTAMPTZ,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_icms_contestacoes_company
    ON icms_fronteira_contestacoes(company_id);
CREATE INDEX IF NOT EXISTS idx_icms_contestacoes_status
    ON icms_fronteira_contestacoes(company_id, status);
