-- 155_fronteira_nao_sped_indexes.sql
--
-- Índices para acelerar a naoSpedQuery (Bloco C do ICMS Fronteira), que ficou
-- pesada no volume da Ferreira Costa (100k+ notas XML/mês). 2026-07-10.
--
-- 1) Lookup de regra por NCM (LATERAL que roda POR LINHA): filtra por uf_estado
--    e prefixo de NCM. O índice existente é só em (ncm_prefixo); com uf_estado
--    na frente o planner descarta rapidamente as regras de outras UFs.
CREATE INDEX IF NOT EXISTS idx_icms_fronteira_regras_ncm_uf_prefixo
    ON icms_fronteira_regras_ncm(uf_estado, ncm_prefixo);

-- 2) Agrupamento dos itens por (NF, CFOP, NCM) em items_grouped. O índice
--    (nfe_id) já existe; o composto cobre o GROUP BY sem re-ordenar.
CREATE INDEX IF NOT EXISTS idx_nfe_entradas_itens_nfe_cfop_ncm
    ON nfe_entradas_itens(nfe_id, cfop, ncm);

-- Obs.: o filtro de período do xml_falt passou a usar nfe_entradas.mes_ano
-- (índice idx_nfe_entradas_company_mes, migration 059) em vez de
-- EXTRACT(data_emissao) — ver naoSpedQuery.
