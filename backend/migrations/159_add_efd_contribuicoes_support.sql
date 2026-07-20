-- Suporte ao fluxo "Importar EFD Contribuições" (enriquecimento de PIS/COFINS
-- em nfe_entradas/nfe_saidas a partir do C100 do EFD Contribuições).
--
-- 1. Discrimina o tipo de arquivo de cada import_job para que o worker saiba
--    qual pipeline de parsing rodar (EFD ICMS/IPI, hoje o único, continua
--    sendo o default para não quebrar jobs já existentes/pendentes).
ALTER TABLE import_jobs ADD COLUMN IF NOT EXISTS tipo_arquivo VARCHAR(30) NOT NULL DEFAULT 'efd_icms_ipi';
CREATE INDEX IF NOT EXISTS idx_import_jobs_tipo_arquivo ON import_jobs(tipo_arquivo);

-- 2. Trilha de auditoria imutável: um registro por C100 processado, casado
--    ou não, para permitir conferência do resumo exibido ao usuário.
CREATE TABLE IF NOT EXISTS efd_contribuicoes_matches (
    id          BIGSERIAL PRIMARY KEY,
    job_id      UUID NOT NULL REFERENCES import_jobs(id) ON DELETE CASCADE,
    company_id  UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    chave_nfe   VARCHAR(44) NOT NULL,
    tipo_nota   VARCHAR(10) NOT NULL, -- 'entrada' (IND_OPER=0) ou 'saida' (IND_OPER=1)
    matched     BOOLEAN NOT NULL,
    vl_pis      NUMERIC(15,2) DEFAULT 0,
    vl_cofins   NUMERIC(15,2) DEFAULT 0,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_efd_contrib_matches_job          ON efd_contribuicoes_matches(job_id);
CREATE INDEX IF NOT EXISTS idx_efd_contrib_matches_company_chave ON efd_contribuicoes_matches(company_id, chave_nfe);
