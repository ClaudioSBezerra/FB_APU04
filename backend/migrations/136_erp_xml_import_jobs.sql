-- 136_erp_xml_import_jobs.sql
--
-- Fila de jobs de importação de XML via ERP (conector erp-bridge-simulador, modo
-- --drain). A UI ("Importar via ERP") enfileira um job por período+tipos; o conector
-- consulta os pendentes, executa a janela (lendo sfc_nfe_imp/sfc_cte_imp do FCCORP e
-- postando em /api/erp-bridge/import/xml) e reporta o resultado. Padrão fila + drain:
-- sem daemon sempre-ligado.

CREATE TABLE IF NOT EXISTS erp_xml_import_jobs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id      UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    data_ini        DATE NOT NULL,
    data_fim        DATE NOT NULL,                       -- inclusive (UI); conector trata o +1
    tipos           TEXT NOT NULL DEFAULT 'entradas,ctes',
    status          TEXT NOT NULL DEFAULT 'pending',     -- pending|running|done|error|canceled
    total_enviados  INT  NOT NULL DEFAULT 0,
    total_erros     INT  NOT NULL DEFAULT 0,
    error_message   TEXT,
    created_by      UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at      TIMESTAMPTZ,
    finished_at     TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_erp_xml_jobs_status CHECK (status IN ('pending','running','done','error','canceled'))
);

CREATE INDEX IF NOT EXISTS idx_erp_xml_jobs_company ON erp_xml_import_jobs(company_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_erp_xml_jobs_pending ON erp_xml_import_jobs(status) WHERE status = 'pending';
