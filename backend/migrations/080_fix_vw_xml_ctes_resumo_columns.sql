-- 080_fix_vw_xml_ctes_resumo_columns.sql
-- Corrige vw_xml_ctes_resumo para expor os mesmos nomes de colunas que
-- vw_xml_entradas_resumo e vw_xml_saidas_resumo, permitindo que XMLPainelHandler
-- use a mesma SELECT em todas as views.
--
-- Mudanças em relação a 078:
--   qtd_ctes     → qtd_notas       (nome uniforme entre as views)
--   v_total_frete → v_total         (nome uniforme)
--   adicionado 0 AS v_pis, v_cofins, v_ipi  (CT-e não possui esses campos)

DROP VIEW IF EXISTS vw_xml_ctes_resumo;
CREATE VIEW vw_xml_ctes_resumo AS
SELECT
    ce.company_id,
    ce.emit_cnpj                                    AS forn_cnpj,
    ce.emit_nome                                    AS forn_nome,
    ce.mes_ano,
    ce.source,
    COUNT(*)                                        AS qtd_notas,
    SUM(COALESCE(ce.v_rec, 0))                      AS v_total,
    SUM(COALESCE(ce.v_bc_icms, 0))                  AS v_bc_icms,
    SUM(COALESCE(ce.v_icms, 0))                     AS v_icms,
    0::numeric                                      AS v_pis,
    0::numeric                                      AS v_cofins,
    0::numeric                                      AS v_ipi,
    SUM(COALESCE(ce.v_ibs, 0))                      AS v_ibs,
    SUM(COALESCE(ce.v_cbs, 0))                      AS v_cbs
FROM cte_entradas ce
GROUP BY ce.company_id, ce.emit_cnpj, ce.emit_nome, ce.mes_ano, ce.source;
