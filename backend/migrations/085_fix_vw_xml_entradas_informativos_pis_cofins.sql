-- Migration 085: Corrige vw_xml_entradas_informativos — PIS/COFINS Simples Nacional
--
-- Problema: total_pis_simples e total_cofins_simples sempre retornavam R$ 0,00.
-- Causa: fornecedores do Simples Nacional emitem NF-e com <vPIS>=0 e <vCOFINS>=0,
--        pois recolhem PIS/COFINS no DAS (não destacam na NF-e).
--        A view anterior somava esses valores declarados (sempre zero).
--
-- Correção: calcular o crédito estimado que o comprador DEIXA DE APROVEITAR
--           ao comprar de fornecedor Simples Nacional, usando alíquotas padrão
--           do regime não cumulativo:
--             PIS   = v_nf × 1,65%
--             COFINS = v_nf × 7,60%
--
-- Isso representa o crédito de PIS/COFINS que seria aproveitável se o fornecedor
-- fosse optante pelo Lucro Real/Presumido com regime não cumulativo.
-- Exibido como "Informativo" no painel — não é crédito real, é estimativa de perda.

CREATE OR REPLACE VIEW vw_xml_entradas_informativos AS
SELECT
    ne.company_id,
    ne.mes_ano,
    SUM(COALESCE(ne.v_ipi, 0))                                                           AS total_ipi,
    SUM(CASE WHEN fs.cnpj IS NOT NULL
             THEN ROUND(COALESCE(ne.v_nf, 0) * 0.0165, 2)
             ELSE 0
        END)                                                                             AS total_pis_simples,
    SUM(CASE WHEN fs.cnpj IS NOT NULL
             THEN ROUND(COALESCE(ne.v_nf, 0) * 0.0760, 2)
             ELSE 0
        END)                                                                             AS total_cofins_simples,
    COUNT(*)                                                                             AS qtd_notas
FROM nfe_entradas ne
LEFT JOIN forn_simples fs ON fs.cnpj = ne.forn_cnpj
WHERE ne.source = 'xml_upload'
GROUP BY ne.company_id, ne.mes_ano;
