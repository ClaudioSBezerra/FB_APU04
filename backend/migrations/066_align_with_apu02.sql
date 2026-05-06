-- 066_align_with_apu02.sql
-- Alinha schema do APU04 com as colunas adicionadas pelo APU02 nas migrations 081-103.
-- Todas as colunas usam ADD COLUMN IF NOT EXISTS — sem efeito quando APU04 aponta
-- para o banco do APU02 (as colunas já existem lá). Necessário para que o
-- erp_bridge_batch funcione em deploy standalone do APU04.

ALTER TABLE nfe_saidas
  ADD COLUMN IF NOT EXISTS data_autorizacao DATE,
  ADD COLUMN IF NOT EXISTS cancelado        TEXT           NOT NULL DEFAULT 'N',
  ADD COLUMN IF NOT EXISTS cfop             VARCHAR(4),
  ADD COLUMN IF NOT EXISTS tipo_cfop        VARCHAR(1)     DEFAULT 'O',
  ADD COLUMN IF NOT EXISTS base_icms        NUMERIC(15,2)  DEFAULT 0,
  ADD COLUMN IF NOT EXISTS icms             NUMERIC(15,2)  DEFAULT 0,
  ADD COLUMN IF NOT EXISTS icms_st          NUMERIC(15,2)  DEFAULT 0,
  ADD COLUMN IF NOT EXISTS ipi              NUMERIC(15,2)  DEFAULT 0,
  ADD COLUMN IF NOT EXISTS base_pis         NUMERIC(15,2)  DEFAULT 0,
  ADD COLUMN IF NOT EXISTS pis              NUMERIC(15,2)  DEFAULT 0,
  ADD COLUMN IF NOT EXISTS base_cofins      NUMERIC(15,2)  DEFAULT 0,
  ADD COLUMN IF NOT EXISTS cofins           NUMERIC(15,2)  DEFAULT 0;

ALTER TABLE nfe_entradas
  ADD COLUMN IF NOT EXISTS data_autorizacao DATE,
  ADD COLUMN IF NOT EXISTS cancelado        TEXT           NOT NULL DEFAULT 'N',
  ADD COLUMN IF NOT EXISTS cfop             VARCHAR(4),
  ADD COLUMN IF NOT EXISTS tipo_cfop        VARCHAR(1)     NOT NULL DEFAULT 'C',
  ADD COLUMN IF NOT EXISTS base_icms        NUMERIC(15,2)  DEFAULT 0,
  ADD COLUMN IF NOT EXISTS icms             NUMERIC(15,2)  DEFAULT 0,
  ADD COLUMN IF NOT EXISTS icms_st          NUMERIC(15,2)  DEFAULT 0,
  ADD COLUMN IF NOT EXISTS ipi              NUMERIC(15,2)  DEFAULT 0,
  ADD COLUMN IF NOT EXISTS base_pis         NUMERIC(15,2)  DEFAULT 0,
  ADD COLUMN IF NOT EXISTS pis              NUMERIC(15,2)  DEFAULT 0,
  ADD COLUMN IF NOT EXISTS base_cofins      NUMERIC(15,2)  DEFAULT 0,
  ADD COLUMN IF NOT EXISTS cofins           NUMERIC(15,2)  DEFAULT 0;

ALTER TABLE cte_entradas
  ADD COLUMN IF NOT EXISTS data_autorizacao DATE,
  ADD COLUMN IF NOT EXISTS cancelado        TEXT NOT NULL DEFAULT 'N';

CREATE INDEX IF NOT EXISTS idx_nfe_saidas_tipo_cfop   ON nfe_saidas(company_id, tipo_cfop);
CREATE INDEX IF NOT EXISTS idx_nfe_saidas_cfop        ON nfe_saidas(company_id, cfop);
CREATE INDEX IF NOT EXISTS idx_nfe_entradas_tipo_cfop ON nfe_entradas(company_id, tipo_cfop);
CREATE INDEX IF NOT EXISTS idx_nfe_entradas_cfop      ON nfe_entradas(company_id, cfop);
