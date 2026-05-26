-- 115_add_ie_municipio_to_import_jobs.sql
-- Inscrição Estadual e código de município (IBGE) do estabelecimento, vindos do
-- reg 0000 do SPED, por filial/job. Migra esses dados do cadastro da empresa
-- (companies) para a filial (eixo UF). A UF já existia em import_jobs.uf.
--
-- O parser (worker.go, case "0000") popula em novas importações:
--   campo 9 = UF, campo 10 = IE, campo 11 = COD_MUN.
-- Imports anteriores ficam com estes campos nulos até reimportar o SPED.

ALTER TABLE import_jobs ADD COLUMN IF NOT EXISTS inscricao_estadual VARCHAR(30);
ALTER TABLE import_jobs ADD COLUMN IF NOT EXISTS cod_municipio      VARCHAR(7);

COMMENT ON COLUMN import_jobs.inscricao_estadual IS
    'Inscrição Estadual do estabelecimento (reg 0000, campo IE do SPED).';
COMMENT ON COLUMN import_jobs.cod_municipio IS
    'Código IBGE do município do estabelecimento (reg 0000, campo COD_MUN do SPED).';
