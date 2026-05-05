-- Migration 065: ERP Bridge — módulo completo de importação via daemon
-- Consolida migrations 074-091 do FB_APU02 (erp_bridge)

-- Tabelas principais de configuração e histórico
CREATE TABLE IF NOT EXISTS erp_bridge_config (
  company_id        UUID PRIMARY KEY REFERENCES companies(id) ON DELETE CASCADE,
  ativo             BOOLEAN NOT NULL DEFAULT false,
  horario           TIME NOT NULL DEFAULT '02:00:00',
  dias_retroativos  INTEGER NOT NULL DEFAULT 1,
  ultimo_run_em     TIMESTAMP WITH TIME ZONE,
  updated_at        TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  reset_tracker     BOOLEAN NOT NULL DEFAULT FALSE,
  erp_type          TEXT NOT NULL DEFAULT 'oracle_xml',
  fbtax_email       TEXT,
  fbtax_password    TEXT,
  oracle_usuario    TEXT,
  oracle_senha      TEXT,
  oracle_dsn        TEXT,
  api_key           TEXT,
  api_key_hash      TEXT,
  daemon_last_seen  TIMESTAMPTZ
);

COMMENT ON COLUMN erp_bridge_config.erp_type IS
  'oracle_xml = por filial com XML (legado) | sap_s4hana = tabela s4i_nfe (novo)';

CREATE INDEX IF NOT EXISTS idx_erp_bridge_config_api_key_hash
  ON erp_bridge_config(api_key_hash)
  WHERE api_key_hash IS NOT NULL;

CREATE TABLE IF NOT EXISTS erp_bridge_runs (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id      UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  iniciado_em     TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  finalizado_em   TIMESTAMP WITH TIME ZONE,
  status          TEXT NOT NULL DEFAULT 'running',
  data_ini        DATE,
  data_fim        DATE,
  total_enviados  INTEGER NOT NULL DEFAULT 0,
  total_ignorados INTEGER NOT NULL DEFAULT 0,
  total_erros     INTEGER NOT NULL DEFAULT 0,
  erro_msg        TEXT,
  origem          TEXT NOT NULL DEFAULT 'manual',
  filiais_filter  TEXT DEFAULT NULL,
  only_parceiros  BOOLEAN NOT NULL DEFAULT FALSE
);

COMMENT ON COLUMN erp_bridge_runs.filiais_filter IS
  'JSON array de nomes de servidor, ex: ["FC - Recife"]. NULL = todos os servidores.';
COMMENT ON COLUMN erp_bridge_runs.only_parceiros IS
  'Quando TRUE, sincroniza apenas parceiros (FORN/CLIE) sem importar movimentos fiscais';

CREATE INDEX IF NOT EXISTS idx_erp_bridge_runs_company
  ON erp_bridge_runs(company_id, iniciado_em DESC);

CREATE INDEX IF NOT EXISTS idx_erp_bridge_runs_pending
  ON erp_bridge_runs(company_id, iniciado_em ASC)
  WHERE status = 'pending';

CREATE TABLE IF NOT EXISTS erp_bridge_run_items (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  run_id      UUID NOT NULL REFERENCES erp_bridge_runs(id) ON DELETE CASCADE,
  servidor    TEXT NOT NULL,
  tipo        TEXT NOT NULL,
  enviados    INTEGER NOT NULL DEFAULT 0,
  ignorados   INTEGER NOT NULL DEFAULT 0,
  erros       INTEGER NOT NULL DEFAULT 0,
  status      TEXT NOT NULL DEFAULT 'ok',
  erro_msg    TEXT
);

CREATE INDEX IF NOT EXISTS idx_erp_bridge_run_items_run
  ON erp_bridge_run_items(run_id);

-- Tabela de servidores registrados pelo daemon
CREATE TABLE IF NOT EXISTS erp_bridge_servidores (
  company_id  UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  nome        TEXT NOT NULL,
  updated_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  PRIMARY KEY (company_id, nome)
);

-- Tabela de lookup CNPJ → nome (fornecedores/clientes/transportadoras)
CREATE TABLE IF NOT EXISTS parceiros (
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    cnpj       TEXT NOT NULL,
    nome       TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (company_id, cnpj)
);

CREATE INDEX IF NOT EXISTS idx_parceiros_company ON parceiros(company_id);
