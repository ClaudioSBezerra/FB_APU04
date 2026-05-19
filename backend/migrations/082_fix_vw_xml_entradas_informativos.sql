-- Migration 082: Corrige vw_xml_entradas_informativos removendo REGEXP_REPLACE desnecessário.
-- forn_simples.cnpj é VARCHAR(14) PK (só dígitos) e nfe_entradas.forn_cnpj é armazenado
-- diretamente de inf.Emit.CNPJ do XML NF-e (também 14 dígitos sem formatação, padrão SEFAZ).
-- O REGEXP_REPLACE anterior impedia uso do índice PK de forn_simples, causando full scan.

CREATE OR REPLACE VIEW vw_xml_entradas_informativos AS
SELECT
    ne.company_id,
    ne.mes_ano,
    SUM(COALESCE(ne.v_ipi, 0))                                                    AS total_ipi,
    SUM(CASE WHEN fs.cnpj IS NOT NULL THEN COALESCE(ne.v_pis, 0)    ELSE 0 END)   AS total_pis_simples,
    SUM(CASE WHEN fs.cnpj IS NOT NULL THEN COALESCE(ne.v_cofins, 0) ELSE 0 END)   AS total_cofins_simples,
    COUNT(*)                                                                       AS qtd_notas
FROM nfe_entradas ne
LEFT JOIN forn_simples fs ON fs.cnpj = ne.forn_cnpj
WHERE ne.source = 'xml_upload'
GROUP BY ne.company_id, ne.mes_ano;
