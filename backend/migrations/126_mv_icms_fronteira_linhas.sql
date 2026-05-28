-- 126_mv_icms_fronteira_linhas.sql
--
-- Materialized view que pré-computa o join pesado do fronteiraBaseQuery:
--   reg_c170 × reg_c190 × reg_c100 × import_jobs × nfe_entradas × nfe_entradas_itens
--
-- Com 500 MB de SPED, o CTE "fonte" leva >60 s a cada requisição.
-- Materializado, o handler lê de uma tabela plana (idx por company_id)
-- e aplica apenas a lógica de negócio (PRODEPE, regras NCM, ST vs ANTECIPAÇÃO).
--
-- Refresh: disparado pelo worker SPED após o último job da fila (mesmo
-- padrão de mv_mercadorias_agregada). Usa CONCURRENTLY (não bloqueia leituras).

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_icms_fronteira_linhas AS
WITH
c190_consol AS (
    SELECT id_pai_c100, cfop,
           MAX(NULLIF(aliq_icms, 0))       AS aliq_icms,
           SUM(COALESCE(vl_icms, 0))       AS vl_icms,
           SUM(COALESCE(vl_bc_icms_st, 0)) AS vl_bc_st,
           SUM(COALESCE(vl_icms_st, 0))    AS vl_icms_st
    FROM reg_c190
    GROUP BY id_pai_c100, cfop
),
fonte AS (
    SELECT
        jb.company_id                                                              AS company_id,
        c170.c100_id                                                               AS c100_id,
        c170.cfop                                                                  AS cfop,
        SUM(COALESCE(c170.vl_item, 0))                                             AS sum_item,
        COALESCE(NULLIF(SUM(COALESCE(c170.vl_ipi, 0)), 0),
                 SUM(COALESCE(xi.v_ipi, 0)), 0)                                    AS ipi_eff,
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
