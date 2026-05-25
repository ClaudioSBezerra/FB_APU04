-- Registros SPED detalhados para o Motor Fiscal Fase 1
-- ─────────────────────────────────────────────────────
--   reg_0200 — cadastro de itens (NCM por código)
--   reg_c170 — itens da NF (cod_item, valores, CFOP, CST...)
--   import_jobs.uf — UF do destinatário (vinda do reg 0000)
--
-- Cruzamento para auditoria fiscal item-a-item:
--   reg_c170.cod_item → reg_0200.cod_item → reg_0200.cod_ncm
--   reg_c170.c100_id  → reg_c100 (cabeçalho da NF)
--   reg_c100.job_id   → import_jobs.uf (destinatário)

ALTER TABLE import_jobs ADD COLUMN IF NOT EXISTS uf VARCHAR(2);
CREATE INDEX IF NOT EXISTS idx_import_jobs_uf ON import_jobs(uf);

-- ─── reg_0200: cadastro de produtos do SPED ──────────────────────────────
-- Layout: |0200|COD_ITEM|DESCR_ITEM|COD_BARRA|COD_ANT_ITEM|UNID_INV|TIPO_ITEM
--         |COD_NCM|EX_IPI|COD_GEN|COD_LST|ALIQ_ICMS|CEST|
CREATE TABLE IF NOT EXISTS reg_0200 (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id      UUID NOT NULL REFERENCES import_jobs(id) ON DELETE CASCADE,
    cod_item    VARCHAR(60) NOT NULL,
    descr_item  TEXT,
    unid_inv    VARCHAR(6),
    tipo_item   VARCHAR(2),
    cod_ncm     VARCHAR(8),
    ex_ipi      VARCHAR(3),
    cod_gen     VARCHAR(2),
    cod_lst     VARCHAR(5),
    aliq_icms   NUMERIC(5,2),
    cest        VARCHAR(7),
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_reg_0200_job_item ON reg_0200(job_id, cod_item);
CREATE INDEX IF NOT EXISTS idx_reg_0200_ncm     ON reg_0200(cod_ncm) WHERE cod_ncm IS NOT NULL;
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'uq_reg_0200_job_cod_item') THEN
        ALTER TABLE reg_0200 ADD CONSTRAINT uq_reg_0200_job_cod_item UNIQUE (job_id, cod_item);
    END IF;
END $$;

-- ─── reg_c170: itens da NF ───────────────────────────────────────────────
-- Layout (campos principais usados pelo motor):
--   |C170|NUM_ITEM|COD_ITEM|DESCR_COMPL|QTD|UNID|VL_ITEM|VL_DESC|IND_MOV|
--   CST_ICMS|CFOP|COD_NAT|VL_BC_ICMS|ALIQ_ICMS|VL_ICMS|VL_BC_ICMS_ST|
--   ALIQ_ST|VL_ICMS_ST|IND_APUR|CST_IPI|COD_ENQ|VL_BC_IPI|ALIQ_IPI|VL_IPI|...
CREATE TABLE IF NOT EXISTS reg_c170 (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id          UUID NOT NULL REFERENCES import_jobs(id) ON DELETE CASCADE,
    c100_id         UUID NOT NULL REFERENCES reg_c100(id) ON DELETE CASCADE,
    num_item        SMALLINT NOT NULL,
    cod_item        VARCHAR(60),
    descr_compl     TEXT,
    qtd             NUMERIC(15,4),
    unid            VARCHAR(6),
    vl_item         NUMERIC(15,2) DEFAULT 0,
    vl_desc         NUMERIC(15,2) DEFAULT 0,
    ind_mov         VARCHAR(1),
    cst_icms        VARCHAR(3),
    cfop            VARCHAR(4),
    cod_nat         VARCHAR(10),
    vl_bc_icms      NUMERIC(15,2) DEFAULT 0,
    aliq_icms       NUMERIC(6,2),
    vl_icms         NUMERIC(15,2) DEFAULT 0,
    vl_bc_icms_st   NUMERIC(15,2) DEFAULT 0,
    aliq_st         NUMERIC(6,2),
    vl_icms_st      NUMERIC(15,2) DEFAULT 0,
    cst_ipi         VARCHAR(2),
    vl_bc_ipi       NUMERIC(15,2) DEFAULT 0,
    aliq_ipi        NUMERIC(6,2),
    vl_ipi          NUMERIC(15,2) DEFAULT 0,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_reg_c170_c100   ON reg_c170(c100_id);
CREATE INDEX IF NOT EXISTS idx_reg_c170_job    ON reg_c170(job_id);
CREATE INDEX IF NOT EXISTS idx_reg_c170_cfop   ON reg_c170(cfop) WHERE cfop IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_reg_c170_item   ON reg_c170(job_id, cod_item);
