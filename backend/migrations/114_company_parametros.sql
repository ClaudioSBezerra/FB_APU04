-- 114_company_parametros.sql
-- Adiciona logo e template de relatório à tabela companies.
-- Arquivos armazenados como BYTEA (funciona em container sem volume externo).

ALTER TABLE companies ADD COLUMN IF NOT EXISTS logo_data  BYTEA;
ALTER TABLE companies ADD COLUMN IF NOT EXISTS logo_mime  VARCHAR(50);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS logo_nome  VARCHAR(255);

ALTER TABLE companies ADD COLUMN IF NOT EXISTS template_antecip_data BYTEA;
ALTER TABLE companies ADD COLUMN IF NOT EXISTS template_antecip_nome VARCHAR(255);

COMMENT ON COLUMN companies.logo_data  IS 'Bytes da logo da empresa (JPEG/PNG/WebP).';
COMMENT ON COLUMN companies.logo_mime  IS 'MIME type da logo (ex.: image/jpeg).';
COMMENT ON COLUMN companies.template_antecip_data IS 'PDF de modelo de relatório de antecipação.';
COMMENT ON COLUMN companies.template_antecip_nome IS 'Nome original do arquivo PDF de modelo.';
