-- Migration 080: View vw_xml_entradas_informativos
-- Agrega IPI e PIS/COFINS de fornecedores Simples Nacional a partir de XMLs de entrada.
-- Filtro source='xml_upload' garante que SPED/bridge não contamina os totais informativos.

CREATE OR REPLACE VIEW vw_xml_entradas_informativos AS
SELECT
    ne.company_id,
    ne.mes_ano,
    SUM(COALESCE(ne.v_ipi, 0))                                                                AS total_ipi,
    SUM(CASE WHEN fs.cnpj IS NOT NULL THEN COALESCE(ne.v_pis, 0)    ELSE 0 END)               AS total_pis_simples,
    SUM(CASE WHEN fs.cnpj IS NOT NULL THEN COALESCE(ne.v_cofins, 0) ELSE 0 END)               AS total_cofins_simples,
    COUNT(*)                                                                                  AS qtd_notas
FROM nfe_entradas ne
LEFT JOIN forn_simples fs ON fs.cnpj = REGEXP_REPLACE(ne.forn_cnpj, '[^0-9]', '', 'g')
WHERE ne.source = 'xml_upload'
GROUP BY ne.company_id, ne.mes_ano;
