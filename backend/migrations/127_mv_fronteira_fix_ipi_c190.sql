-- 127_mv_fronteira_fix_ipi_c190.sql
--
-- Corrige IPI zerado nas linhas do Bloco A/B (SPED).
--
-- Problema: ipi_eff usava apenas c170.vl_ipi (item a item) e xi.v_ipi (XML).
-- Muitos SPEDs não populam vl_ipi em C170; o C190 (sumarizador por CFOP) tem
-- o total do IPI em vl_ipi, mas não era incluído no c190_consol.
--
-- Fix: adiciona SUM(vl_ipi) em c190_consol e usa como fallback intermediário:
--   1. C170 item a item  → mais preciso quando disponível
--   2. C190 por CFOP     → presente na maioria dos SPEDs
--   3. XML (nfe_entradas_itens) → fallback quando não há SPED
--   4. 0

DROP MATERIALIZED VIEW IF EXISTS mv_icms_fronteira_linhas;

CREATE MATERIALIZED VIEW mv_icms_fronteira_linhas AS
WITH
c190_consol AS (
    SELECT id_pai_c100, cfop,
           MAX(NULLIF(aliq_icms, 0))       AS aliq_icms,
           SUM(COALESCE(vl_icms, 0))       AS vl_icms,
           SUM(COALESCE(vl_bc_icms_st, 0)) AS vl_bc_st,
           SUM(COALESCE(vl_icms_st, 0))    AS vl_icms_st,
           SUM(COALESCE(vl_ipi, 0))        AS vl_ipi
    FROM reg_c190
    GROUP BY id_pai_c100, cfop
),
fonte AS (
    SELECT
        jb.company_id                                                              AS company_id,
        c170.c100_id                                                               AS c100_id,
        c170.cfop                                                                  AS cfop,
        SUM(COALESCE(c170.vl_item, 0))                                             AS sum_item,
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
        SUM(COALESCE(cc.vl_icms, 0))                                               AS vl_icms,
        SUM(COALESCE(cc.vl_bc_st, 0))                                              AS vl_bc_st,
        SUM(COALESCE(cc.vl_icms_st, 0))                                            AS vl_icms_st
    FROM reg_c170 c170
    JOIN reg_c100 c100b ON c100b.id = c170.c100_id
    JOIN import_jobs jb  ON jb.id   = c100b.job_id
    LEFT JOIN c190_consol cc
           ON cc.id_pai_c100 = c100b.id AND cc.cfop = c170.cfop
    LEFT JOIN nfe_entradas ne
           ON ne.company_id = jb.company_id AND ne.chave_nfe = c100b.chv_nfe
    LEFT JOIN nfe_entradas_itens xi
           ON xi.nfe_id = ne.id AND xi.n_item = c170.num_item
    WHERE c170.cfop = ANY(ARRAY['2101','2102','2152','2403','2409','2651','2652','2551','2556'])
    GROUP BY jb.company_id, c170.c100_id, c170.cfop
)
SELECT
    f.company_id,
    f.c100_id,
    f.cfop,
    f.sum_item                                                                     AS v_prod_disp,
    f.ipi_eff,
    f.tem_xml,
    f.aliq_icms,
    f.vl_icms,
    f.vl_bc_st,
    f.vl_icms_st,
    -- frete/outro rateados pela participação do cfop no total da nota
    CASE WHEN SUM(f.sum_item) OVER (PARTITION BY f.c100_id) > 0
         THEN f.nf_frete * f.sum_item / SUM(f.sum_item) OVER (PARTITION BY f.c100_id)
         ELSE 0 END                                                                AS frete_rat,
    CASE WHEN SUM(f.sum_item) OVER (PARTITION BY f.c100_id) > 0
         THEN f.nf_outro * f.sum_item / SUM(f.sum_item) OVER (PARTITION BY f.c100_id)
         ELSE 0 END                                                                AS outro_rat,
    -- base_calc = produto + IPI + frete_rat + outro_rat
    f.sum_item + f.ipi_eff
        + CASE WHEN SUM(f.sum_item) OVER (PARTITION BY f.c100_id) > 0
               THEN f.nf_frete * f.sum_item / SUM(f.sum_item) OVER (PARTITION BY f.c100_id)
               ELSE 0 END
        + CASE WHEN SUM(f.sum_item) OVER (PARTITION BY f.c100_id) > 0
               THEN f.nf_outro * f.sum_item / SUM(f.sum_item) OVER (PARTITION BY f.c100_id)
               ELSE 0 END                                                          AS base_calc
FROM fonte f
WITH DATA;

-- Índice único requerido pelo REFRESH CONCURRENTLY
CREATE UNIQUE INDEX idx_mv_fronteira_linhas_key
    ON mv_icms_fronteira_linhas(c100_id, cfop);

-- Índice de filtragem por empresa (principal filtro do handler)
CREATE INDEX idx_mv_fronteira_linhas_company
    ON mv_icms_fronteira_linhas(company_id);
