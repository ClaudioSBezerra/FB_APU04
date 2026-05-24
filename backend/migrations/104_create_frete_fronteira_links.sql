-- 104_create_frete_fronteira_links.sql
-- Tabelas para vincular CT-e a NF-e no cálculo de ICMS Fronteira do frete.
-- Estratégia de matching (em ordem de prioridade):
--   1. reg_d162: D162 do SPED vincula CTE (D100) → NF-e por chave
--   2. cte_entradas_nfe_refs: referências das NFs extraídas do XML do CT-e
-- Fallback por num_doc+vl_doc é feito em query na hora sem tabela extra.

-- Registro D162 do EFD ICMS: filho de D160 (filho de D100)
-- Cada D162 aponta para uma NF-e transportada pelo CT-e pai (D100).
CREATE TABLE IF NOT EXISTS reg_d162 (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id     UUID        NOT NULL REFERENCES import_jobs(id) ON DELETE CASCADE,
    d100_id    UUID        REFERENCES reg_d100(id) ON DELETE CASCADE,
    num_item   INTEGER,
    chv_nfe    VARCHAR(44),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_d162_job_id  ON reg_d162(job_id);
CREATE INDEX IF NOT EXISTS idx_d162_d100_id ON reg_d162(d100_id);
CREATE INDEX IF NOT EXISTS idx_d162_chv_nfe ON reg_d162(chv_nfe);

-- Referências NF-e extraídas do XML do CT-e (infCTeNorm/infDoc/infNFe/chNFe).
-- Um CT-e pode transportar várias NFs; uma NF pode ter mais de um CT-e.
CREATE TABLE IF NOT EXISTS cte_entradas_nfe_refs (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    cte_id     UUID        NOT NULL REFERENCES cte_entradas(id) ON DELETE CASCADE,
    company_id UUID        NOT NULL,
    chave_nfe  VARCHAR(44) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_cte_nfe_ref UNIQUE (cte_id, chave_nfe)
);
CREATE INDEX IF NOT EXISTS idx_cte_nfe_refs_company  ON cte_entradas_nfe_refs(company_id);
CREATE INDEX IF NOT EXISTS idx_cte_nfe_refs_chave    ON cte_entradas_nfe_refs(chave_nfe);
CREATE INDEX IF NOT EXISTS idx_cte_nfe_refs_cte_id   ON cte_entradas_nfe_refs(cte_id);
