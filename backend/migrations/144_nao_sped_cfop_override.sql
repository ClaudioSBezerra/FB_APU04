-- 144_nao_sped_cfop_override.sql
--
-- Permite que o usuário corrija o CFOP de uma NF do Bloco C (XML-only)
-- diretamente na tela, sem precisar alterar o XML original.
--
-- Caso de uso: fornecedor emitiu CFOP 6101 (Antecipação) para produto
-- com NCM 7318 que deveria ser 6403 (ST). O usuário ajusta o CFOP por linha
-- (chave_nfe + NCM) e o sistema recalcula o regime e o ICMS automaticamente.

CREATE TABLE nao_sped_cfop_override (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id          UUID        NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    chave_nfe           TEXT        NOT NULL,
    ncm                 TEXT        NOT NULL DEFAULT '',
    cfop_saida_override TEXT        NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (company_id, chave_nfe, ncm)
);

CREATE INDEX idx_nao_sped_cfop_override_company
    ON nao_sped_cfop_override(company_id);
