-- 137_xml_batches_erp_job_id.sql
--
-- Liga os lotes de XML (xml_upload_batches) ao job de importação via ERP
-- (erp_xml_import_jobs). Assim a UI mostra o progresso REAL do worker
-- (importados/total/rejeitados) por job, em tempo real e resiliente — mesmo que o
-- conector caia depois de enviar, os lotes continuam processando e a barra anda.
-- NULL para uploads diretos (que não têm job ERP).
ALTER TABLE xml_upload_batches ADD COLUMN IF NOT EXISTS erp_job_id UUID;
CREATE INDEX IF NOT EXISTS idx_xml_batches_erp_job ON xml_upload_batches(erp_job_id) WHERE erp_job_id IS NOT NULL;
