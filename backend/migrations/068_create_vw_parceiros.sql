-- 068_create_vw_parceiros.sql
-- VIEW unificada de participantes/parceiros, combinando duas fontes:
--   1. parceiros  (APU02/ERP Bridge): keyed por (company_id, cnpj)
--   2. participants (SPED 0150):       keyed por (job_id, cnpj) → via import_jobs
--
-- Prioridade: parceiros (ERP Bridge) > participants (SPED 0150).
-- Usada pelos handlers de NF-e para enriquecer forn_nome/dest_nome quando
-- o registro foi importado via bridge sem nome.

CREATE OR REPLACE VIEW vw_parceiros AS
SELECT DISTINCT ON (company_id, cnpj)
    company_id,
    cnpj,
    nome
FROM (
    SELECT company_id, cnpj, nome, 1 AS prio
    FROM parceiros
    WHERE cnpj  IS NOT NULL AND cnpj  <> ''
      AND nome  IS NOT NULL AND nome  <> ''

    UNION ALL

    SELECT ij.company_id, p.cnpj, p.nome, 2 AS prio
    FROM participants p
    JOIN import_jobs ij ON ij.id = p.job_id
    WHERE p.cnpj  IS NOT NULL AND p.cnpj  <> ''
      AND p.nome  IS NOT NULL AND p.nome  <> ''
      AND ij.company_id IS NOT NULL
) src
ORDER BY company_id, cnpj, prio;
