-- 157_mv_fronteira_pareamento_consistente.sql
--
-- Corrige DUPLA CONTAGEM na mv_icms_fronteira_linhas (migration 153) quando o
-- NCM do 0200 diverge do NCM do XML na MESMA nota. Caso real (FC/BA 05/2026,
-- NF 48445, achado pela Contraprova em 2026-07-10): o 0200 classificou 3 itens
-- como 85061011 e o XML os traz em 85061019 → a MV somava o vl_item deles pelo
-- FALLBACK (grupo 85061011, sem par no XML) E o vProd deles pela ALOCAÇÃO
-- (grupo 85061019, que casou) → 70.054,83 numa nota de 49.259,49 (vl_item).
-- Antecipação calculada A MAIOR nessas notas.
--
-- Fix conservador: o pareamento XML só é usado quando é CONSISTENTE na nota
-- inteira —
--   (a) todo grupo (CFOP,NCM) do SPED encontra o NCM no XML;
--   (b) todo NCM do XML é encontrado no SPED (conjuntos iguais);
--   (c) o vProd total do XML não excede o vl_item total do SPED (excesso
--       indica nota parcialmente escriturada fora dos CFOPs de fronteira ou
--       desconto escriturado — a alocação inflaria).
-- Nota inconsistente cai INTEIRA no SPED puro (vl_item, comportamento
-- pré-143/153 — conservador, sem dupla contagem). Notas consistentes (a
-- imensa maioria) mantêm a precisão do vProd por NCM do XML.

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
-- Itens do XML agregados por (nota, NCM) — a chave de pareamento com o SPED
xml_ncm AS (
    SELECT
        nii.nfe_id,
        LEFT(regexp_replace(COALESCE(nii.ncm, ''), '[^0-9]', '', 'g'), 8) AS ncm_8,
        SUM(COALESCE(nii.v_prod, 0)) AS v_prod,
        SUM(COALESCE(nii.v_ipi, 0))  AS v_ipi
    FROM nfe_entradas_itens nii
    GROUP BY nii.nfe_id,
             LEFT(regexp_replace(COALESCE(nii.ncm, ''), '[^0-9]', '', 'g'), 8)
),
xml_tot AS (
    SELECT nfe_id, COUNT(*) AS n_grupos, SUM(v_prod) AS v_prod_tot
    FROM xml_ncm
    GROUP BY nfe_id
),
-- Lado SPED agregado por (nota, cfop, NCM) — sem tocar no XML ainda
sped AS (
    SELECT
        jb.company_id                                                         AS company_id,
        c170.c100_id                                                          AS c100_id,
        c170.cfop                                                             AS cfop,
        LEFT(regexp_replace(COALESCE(p.cod_ncm, ''), '[^0-9]', '', 'g'), 8)  AS ncm_8,
        ne.id                                                                 AS nfe_id,
        SUM(COALESCE(c170.vl_item, 0))                                        AS sped_vl_item,
        NULLIF(SUM(COALESCE(c170.vl_ipi, 0)), 0)                              AS sped_vl_ipi,
        NULLIF(MAX(cc.vl_ipi), 0)                                             AS c190_vl_ipi,
        MAX(COALESCE(ne.v_frete, 0))                                          AS nf_frete,
        MAX(COALESCE(ne.v_outro, 0))                                          AS nf_outro,
        MAX(NULLIF(cc.aliq_icms, 0))                                          AS aliq_icms,
        MAX(COALESCE(cc.vl_icms, 0))                                          AS vl_icms_cfop,
        MAX(COALESCE(cc.vl_bc_st, 0))                                         AS vl_bc_st_cfop,
        MAX(COALESCE(cc.vl_icms_st, 0))                                       AS vl_icms_st_cfop,
        COALESCE(MAX(cc.vl_icms_inter), 0)                                    AS vl_icms_inter
    FROM reg_c170 c170
    JOIN reg_c100 c100b ON c100b.id = c170.c100_id
    JOIN import_jobs jb  ON jb.id   = c100b.job_id
    LEFT JOIN reg_0200 p
           ON p.job_id = c100b.job_id AND p.cod_item = c170.cod_item
    LEFT JOIN c190_consol cc
           ON cc.id_pai_c100 = c100b.id AND cc.cfop = c170.cfop
    LEFT JOIN nfe_entradas ne
           ON ne.company_id = jb.company_id AND ne.chave_nfe = c100b.chv_nfe
    WHERE c170.cfop = ANY(ARRAY['2101','2102','2152','2403','2409','2651','2652','2551','2556'])
    GROUP BY jb.company_id, c170.c100_id, c170.cfop, ne.id,
             LEFT(regexp_replace(COALESCE(p.cod_ncm, ''), '[^0-9]', '', 'g'), 8)
),
-- Consistência do pareamento POR NOTA (guarda anti dupla-contagem)
nota_par AS (
    SELECT s.c100_id,
           (BOOL_AND(xn.nfe_id IS NOT NULL)
            AND COUNT(DISTINCT xn.ncm_8) = MAX(xt.n_grupos)
            AND MAX(xt.v_prod_tot) <= SUM(s.sped_vl_item) + 0.01)             AS pareavel
    FROM sped s
    LEFT JOIN xml_ncm xn ON xn.nfe_id = s.nfe_id AND xn.ncm_8 = s.ncm_8
    LEFT JOIN xml_tot xt ON xt.nfe_id = s.nfe_id
    GROUP BY s.c100_id
),
fonte AS (
    SELECT
        s.company_id,
        s.c100_id,
        s.cfop,
        s.ncm_8,
        -- vProd do XML (sem IPI — spec NF-e) pareado por NCM, SÓ quando a nota
        -- é pareável (ver nota_par); rateio pró-rata quando o mesmo (nota,NCM)
        -- aparece em mais de um CFOP de entrada. Senão: vl_item do SPED.
        CASE WHEN np.pareavel THEN
            COALESCE(
                xn.v_prod * s.sped_vl_item
                    / NULLIF(SUM(s.sped_vl_item) OVER (PARTITION BY s.c100_id, s.ncm_8), 0),
                s.sped_vl_item
            )
        ELSE s.sped_vl_item END                                               AS sum_item,
        -- IPI: C170 do item (mais preciso) → C190 por CFOP → XML por NCM → 0
        COALESCE(
            s.sped_vl_ipi,
            s.c190_vl_ipi,
            CASE WHEN np.pareavel THEN
                xn.v_ipi * s.sped_vl_item
                    / NULLIF(SUM(s.sped_vl_item) OVER (PARTITION BY s.c100_id, s.ncm_8), 0)
            END,
            0
        )                                                                     AS ipi_eff,
        (xn.nfe_id IS NOT NULL AND np.pareavel)                               AS tem_xml,
        s.nf_frete,
        s.nf_outro,
        s.aliq_icms,
        s.vl_icms_cfop,
        s.vl_bc_st_cfop,
        s.vl_icms_st_cfop,
        s.vl_icms_inter
    FROM sped s
    LEFT JOIN xml_ncm xn
           ON xn.nfe_id = s.nfe_id AND xn.ncm_8 = s.ncm_8
    LEFT JOIN nota_par np
           ON np.c100_id = s.c100_id
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
