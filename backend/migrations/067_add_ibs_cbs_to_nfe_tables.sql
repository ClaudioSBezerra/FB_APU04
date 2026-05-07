-- 067_add_ibs_cbs_to_nfe_tables.sql
-- Restaura colunas XML e IBS/CBS removidas pelo APU02 nas migrations 081/082.
-- Necessário quando APU04 aponta para o banco do APU02 (cliente-db), que
-- simplificou o schema para SAP S4/HANA. Todas as colunas usam IF NOT EXISTS.

-- ── nfe_saidas ────────────────────────────────────────────────────────────────
ALTER TABLE nfe_saidas
  -- Colunas de identificação removidas em 081
  ADD COLUMN IF NOT EXISTS nat_op           VARCHAR(60),
  ADD COLUMN IF NOT EXISTS emit_nome        VARCHAR(60),
  ADD COLUMN IF NOT EXISTS emit_uf          VARCHAR(2),
  ADD COLUMN IF NOT EXISTS emit_municipio   VARCHAR(60),
  ADD COLUMN IF NOT EXISTS dest_nome        VARCHAR(60),
  ADD COLUMN IF NOT EXISTS dest_uf          VARCHAR(2),
  ADD COLUMN IF NOT EXISTS dest_c_mun       VARCHAR(7),
  -- Colunas ICMSTot removidas em 081
  ADD COLUMN IF NOT EXISTS v_bc             NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_icms           NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_icms_deson     NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_fcp            NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_bc_st          NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_st             NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_fcp_st         NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_fcp_st_ret     NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_prod           NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_frete          NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_seg            NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_desc           NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_ii             NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_ipi            NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_ipi_devol      NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_pis            NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_cofins         NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_outro          NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_nf             NUMERIC(15,2) DEFAULT 0,
  -- IBS/CBS removidas em 081
  ADD COLUMN IF NOT EXISTS v_bc_ibs_cbs     NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_ibs_uf         NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_ibs_mun        NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_ibs            NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_cred_pres_ibs  NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_cbs            NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_cred_pres_cbs  NUMERIC(15,2) DEFAULT 0;

-- ── nfe_entradas ──────────────────────────────────────────────────────────────
ALTER TABLE nfe_entradas
  -- Colunas de identificação removidas em 082
  ADD COLUMN IF NOT EXISTS nat_op           VARCHAR(60),
  ADD COLUMN IF NOT EXISTS forn_nome        VARCHAR(60),
  ADD COLUMN IF NOT EXISTS forn_uf          VARCHAR(2),
  ADD COLUMN IF NOT EXISTS forn_municipio   VARCHAR(60),
  ADD COLUMN IF NOT EXISTS dest_nome        VARCHAR(60),
  ADD COLUMN IF NOT EXISTS dest_uf          VARCHAR(2),
  ADD COLUMN IF NOT EXISTS dest_c_mun       VARCHAR(7),
  -- Colunas ICMSTot removidas em 082
  ADD COLUMN IF NOT EXISTS v_bc             NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_icms           NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_icms_deson     NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_fcp            NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_bc_st          NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_st             NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_fcp_st         NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_fcp_st_ret     NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_prod           NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_frete          NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_seg            NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_desc           NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_ii             NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_ipi            NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_ipi_devol      NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_pis            NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_cofins         NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_outro          NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_nf             NUMERIC(15,2) DEFAULT 0,
  -- IBS/CBS removidas em 082
  ADD COLUMN IF NOT EXISTS v_bc_ibs_cbs     NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_ibs_uf         NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_ibs_mun        NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_ibs            NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_cred_pres_ibs  NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_cbs            NUMERIC(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS v_cred_pres_cbs  NUMERIC(15,2) DEFAULT 0;
