-- 143_mv_fronteira_fix_vl_prod_ipi.sql
--
-- Problema: quando a empresa gravou o SPED com VL_ITEM = vProd + vIPI
-- (IPI embutido no valor do item) e VL_IPI = 0, o ipi_eff caía no
-- fallback XML e somava o IPI novamente sobre um sum_item que já o continha.
-- Exemplo confirmado: SCHAEFFLER NF 804746 — sped_vl_ipi=0 para todos
-- os itens, diferença entre sped_vl_item e xml_v_prod = exatamente o xml_v_ipi.
--
-- Fix: substituir SUM(c170.vl_item) por SUM(COALESCE(xi.v_prod, c170.vl_item))
-- como base de sum_item. O campo v_prod do NF-e XML é sempre o valor do
-- produto SEM IPI (obrigatório pela especificação NF-e). Quando não há XML,
-- mantém c170.vl_item como antes.

DROP MATERIALIZED VIEW IF EXISTS mv_icms_fronteira_linhas;

CREATE MATERIALIZED VIEW mv_icms_fronteira_linhas AS
WITH
c190_consol AS (
    SELECT id_pai_c100, cfop,
           MAX(NULLIF(aliq_icms, 0))       AS aliq_icms,
           SUM(COALESCE(vl_icms, 0))       AS vl_icms,
           SUM(COALESCE(vl_bc_icms_st, 0)) AS vl_bc_st,
           SUM(COALESCE(vl_icms_st, 0))    AS vl_icms_st,
           SUM(COALESCE(vl_ipi, 0))        AS vl_ipi,
           SUM(
               COALESCE(vl_bc_icms, 0) *
               LEAST(COALESCE(NULLIF(aliq_icms, 0), 12.0), 12.0) / 100.0
           )                               AS vl_icms_inter
    FROM reg_c190
    GROUP BY id_pai_c100, cfop
),
fonte AS (
    SELECT
        jb.company_id                                                              AS company_id,
        c170.c100_id                                                               AS c100_id,
        c170.cfop                                                                  AS cfop,
        LEFT(regexp_replace(COALESCE(p.cod_ncm, ''), '[^0-9]', '', 'g'), 8)       AS ncm_8,
        -- Usa xi.v_prod (XML) quando disponível: o campo v_prod do NF-e é sempre
        -- o valor do produto SEM IPI. Cai para vl_item do SPED apenas sem XML —
        -- evita dupla contagem quando a empresa grava VL_ITEM = vProd + vIPI no SPED.
        SUM(COALESCE(xi.v_prod, c170.vl_item, 0))                                 AS sum_item,
        -- IPI: C170 item (mais preciso) → C190 por CFOP → XML → 0
        COALESCE(
            NULLIF(SUM(COALESCE(c170.vl_ipi, 0)), 0),
            NULLIF(MAX(cc.vl_ipi), 0),
            SUM(COALESCE(xi.v_ipi, 0)),
            0
        )                                                                          AS ipi_eff,
        BOOL_OR(xi.id IS NOT NULL)                                                 AS tem_xml,
        MAX(COALESCE(ne.v_frete, 0))                                               AS nf_frete,
        MAX(COALESCE(ne.v_outro, 0))                                               AS nf_outro,
        MAX(NULLIF(cc.aliq_icms, 0))                                               AS aliq_icms,
        MAX(COALESCE(cc.vl_icms, 0))                                               AS vl_icms_cfop,
        MAX(COALESCE(cc.vl_bc_st, 0))                                              AS vl_bc_st_cfop,
        MAX(COALESCE(cc.vl_icms_st, 0))                                            AS vl_icms_st_cfop,
        COALESCE(MAX(cc.vl_icms_inter), 0)                                         AS vl_icms_inter
    FROM reg_c170 c170
    JOIN reg_c100 c100b ON c100b.id = c170.c100_id
    JOIN import_jobs jb  ON jb.id   = c100b.job_id
    LEFT JOIN reg_0200 p
           ON p.job_id = c100b.job_id AND p.cod_item = c170.cod_item
    LEFT JOIN c190_consol cc
           ON cc.id_pai_c100 = c100b.id AND cc.cfop = c170.cfop
    LEFT JOIN nfe_entradas ne
           ON ne.company_id = jb.company_id AND ne.chave_nfe = c100b.chv_nfe
    LEFT JOIN nfe_entradas_itens xi
           ON xi.nfe_id = ne.id AND xi.n_item = c170.num_item
    WHERE c170.cfop = ANY(ARRAY['2101','2102','2152','2403','2409','2651','2652','2551','2556'])
    GROUP BY jb.company_id, c170.c100_id, c170.cfop,
             LEFT(regexp_replace(COALESCE(p.cod_ncm, ''), '[^0-9]', '', 'g'), 8)
)
SELECT
    f.company_id,
    f.c100_id,
    f.cfop,
    f.ncm_8,
    f.sum_item                                                                     AS v_prod_disp,
    f.ipi_eff,
    f.tem_xml,
    f.aliq_icms,
    f.vl_icms_cfop                                                                 AS vl_icms,
    CASE WHEN SUM(f.sum_item) OVER (PARTITION BY f.c100_id, f.cfop) > 0
         THEN f.vl_bc_st_cfop * f.sum_item / SUM(f.sum_item) OVER (PARTITION BY f.c100_id, f.cfop)
         ELSE f.vl_bc_st_cfop END                                                  AS vl_bc_st,
    CASE WHEN SUM(f.sum_item) OVER (PARTITION BY f.c100_id, f.cfop) > 0
         THEN f.vl_icms_st_cfop * f.sum_item / SUM(f.sum_item) OVER (PARTITION BY f.c100_id, f.cfop)
         ELSE f.vl_icms_st_cfop END                                                AS vl_icms_st,
    f.vl_icms_inter,
    CASE WHEN SUM(f.sum_item) OVER (PARTITION BY f.c100_id) > 0
         THEN f.nf_frete * f.sum_item / SUM(f.sum_item) OVER (PARTITION BY f.c100_id)
         ELSE 0 END                                                                AS frete_rat,
    CASE WHEN SUM(f.sum_item) OVER (PARTITION BY f.c100_id) > 0
         THEN f.nf_outro * f.sum_item / SUM(f.sum_item) OVER (PARTITION BY f.c100_id)
         ELSE 0 END                                                                AS outro_rat,
    f.sum_item + f.ipi_eff
        + CASE WHEN SUM(f.sum_item) OVER (PARTITION BY f.c100_id) > 0
               THEN f.nf_frete * f.sum_item / SUM(f.sum_item) OVER (PARTITION BY f.c100_id)
               ELSE 0 END
        + CASE WHEN SUM(f.sum_item) OVER (PARTITION BY f.c100_id) > 0
               THEN f.nf_outro * f.sum_item / SUM(f.sum_item) OVER (PARTITION BY f.c100_id)
               ELSE 0 END                                                          AS base_calc
FROM fonte f
WITH NO DATA;

CREATE UNIQUE INDEX idx_mv_fronteira_linhas_key
    ON mv_icms_fronteira_linhas(c100_id, cfop, COALESCE(ncm_8, ''));

CREATE INDEX idx_mv_fronteira_linhas_company
    ON mv_icms_fronteira_linhas(company_id);

REFRESH MATERIALIZED VIEW mv_icms_fronteira_linhas;
