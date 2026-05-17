-- 078_create_vw_xml_panels.sql
-- Views PostgreSQL para os painéis XML da Phase 02 (per D-10, D-11, D-16a).
--
-- Padrão: DROP + CREATE (não CREATE OR REPLACE) — necessário para renomear colunas
-- entre versões (emit_cnpj→forn_cnpj em saidas/ctes). Idempotente via DROP IF EXISTS.
-- Padrão de agregação: SUM(COALESCE(col, 0)) — mesmo padrão de vw_nfe_entradas_impostos (072).
-- Nomes de colunas validados contra schema real (migrations 059, 058, 060, 066, 067, 070).
--
-- Ajuste vs. rascunho original:
--   v_icms_dest/v_icms_remet NÃO existem em nfe_entradas — o campo é v_icms (ICMSTot).
--   As views usam v_icms diretamente.
--   vw_xml_saidas_resumo e vw_xml_ctes_resumo expõem emit_cnpj/emit_nome como
--   forn_cnpj/forn_nome para que XMLPainelHandler use a mesma query em todas as views.

-- ── vw_xml_entradas_resumo ────────────────────────────────────────────────────
DROP VIEW IF EXISTS vw_xml_entradas_resumo;
-- Agrega nfe_entradas por empresa/fornecedor/mês, incluindo filtro por source.
CREATE VIEW vw_xml_entradas_resumo AS
SELECT
    ne.company_id,
    ne.forn_cnpj,
    ne.forn_nome,
    ne.mes_ano,
    ne.source,
    COUNT(*)                                        AS qtd_notas,
    SUM(COALESCE(ne.v_nf, 0))                      AS v_total,
    SUM(COALESCE(ne.v_bc, 0))                       AS v_bc_icms,
    SUM(COALESCE(ne.v_icms, 0))                     AS v_icms,
    SUM(COALESCE(ne.v_pis, 0))                      AS v_pis,
    SUM(COALESCE(ne.v_cofins, 0))                   AS v_cofins,
    SUM(COALESCE(ne.v_ipi, 0))                      AS v_ipi,
    SUM(COALESCE(ne.v_ibs, 0))                      AS v_ibs,
    SUM(COALESCE(ne.v_cbs, 0))                      AS v_cbs
FROM nfe_entradas ne
GROUP BY ne.company_id, ne.forn_cnpj, ne.forn_nome, ne.mes_ano, ne.source;

-- ── vw_xml_saidas_resumo ──────────────────────────────────────────────────────
DROP VIEW IF EXISTS vw_xml_saidas_resumo;
-- Agrega nfe_saidas por empresa/emitente/mês, incluindo filtro por source.
-- emit_cnpj/emit_nome expostos como forn_cnpj/forn_nome (convenção uniforme).
CREATE VIEW vw_xml_saidas_resumo AS
SELECT
    ns.company_id,
    ns.emit_cnpj                                    AS forn_cnpj,
    ns.emit_nome                                    AS forn_nome,
    ns.mes_ano,
    ns.source,
    COUNT(*)                                        AS qtd_notas,
    SUM(COALESCE(ns.v_nf, 0))                      AS v_total,
    SUM(COALESCE(ns.v_bc, 0))                       AS v_bc_icms,
    SUM(COALESCE(ns.v_icms, 0))                     AS v_icms,
    SUM(COALESCE(ns.v_pis, 0))                      AS v_pis,
    SUM(COALESCE(ns.v_cofins, 0))                   AS v_cofins,
    SUM(COALESCE(ns.v_ipi, 0))                      AS v_ipi,
    SUM(COALESCE(ns.v_ibs, 0))                      AS v_ibs,
    SUM(COALESCE(ns.v_cbs, 0))                      AS v_cbs
FROM nfe_saidas ns
GROUP BY ns.company_id, ns.emit_cnpj, ns.emit_nome, ns.mes_ano, ns.source;

-- ── vw_xml_ctes_resumo ────────────────────────────────────────────────────────
DROP VIEW IF EXISTS vw_xml_ctes_resumo;
-- Agrega cte_entradas por empresa/transportadora/mês, incluindo filtro por source.
-- emit_cnpj/emit_nome expostos como forn_cnpj/forn_nome (convenção uniforme).
CREATE VIEW vw_xml_ctes_resumo AS
SELECT
    ce.company_id,
    ce.emit_cnpj                                    AS forn_cnpj,
    ce.emit_nome                                    AS forn_nome,
    ce.mes_ano,
    ce.source,
    COUNT(*)                                        AS qtd_ctes,
    SUM(COALESCE(ce.v_rec, 0))                      AS v_total_frete,
    SUM(COALESCE(ce.v_bc_icms, 0))                  AS v_bc_icms,
    SUM(COALESCE(ce.v_icms, 0))                      AS v_icms,
    SUM(COALESCE(ce.v_ibs, 0))                       AS v_ibs,
    SUM(COALESCE(ce.v_cbs, 0))                       AS v_cbs
FROM cte_entradas ce
GROUP BY ce.company_id, ce.emit_cnpj, ce.emit_nome, ce.mes_ano, ce.source;

-- ── vw_xml_itens_ncm ──────────────────────────────────────────────────────────
DROP VIEW IF EXISTS vw_xml_itens_ncm;
-- Agrega nfe_entradas_itens por NCM para o relatório CCLASSTRIB (per D-16a).
CREATE VIEW vw_xml_itens_ncm AS
SELECT
    ei.company_id,
    ei.ncm,
    COUNT(DISTINCT ei.cst_pis)                                          AS variantes_cst_pis,
    COUNT(DISTINCT ei.cst_cofins)                                       AS variantes_cst_cofins,
    COUNT(DISTINCT ei.cclasstrib) FILTER (WHERE ei.cclasstrib IS NOT NULL) AS variantes_cclasstrib,
    COUNT(*)                                                            AS qtd_itens,
    SUM(COALESCE(ei.v_pis, 0))                                         AS v_pis_total,
    SUM(COALESCE(ei.v_cofins, 0))                                      AS v_cofins_total,
    BOOL_OR(ei.cclasstrib IS NULL)                                      AS tem_cclasstrib_nulo
FROM nfe_entradas_itens ei
GROUP BY ei.company_id, ei.ncm;
