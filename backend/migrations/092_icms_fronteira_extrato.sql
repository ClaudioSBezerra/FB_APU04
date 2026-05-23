-- 092_icms_fronteira_extrato.sql
-- Armazena linhas do Extrato SEFAZ-PE importadas via CSV/XLSX.
-- Permite comparação entre o calculado pelo sistema e o cobrado pela SEFAZ.

CREATE TABLE IF NOT EXISTS icms_fronteira_extrato_sefaz (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id       UUID         NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    periodo          VARCHAR(7)   NOT NULL,           -- MM/YYYY
    registro_nota    VARCHAR(60),
    cnpj_emitente    VARCHAR(14),
    nome_emitente    VARCHAR(200),
    uf_emitente      VARCHAR(2),
    numero_nf        VARCHAR(20),
    chave_nfe        VARCHAR(44),
    icms_devido      NUMERIC(15,2),
    observacao       TEXT,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_icms_extrato_company_periodo
    ON icms_fronteira_extrato_sefaz(company_id, periodo);
