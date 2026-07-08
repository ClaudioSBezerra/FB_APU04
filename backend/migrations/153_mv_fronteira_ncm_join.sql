-- 153_mv_fronteira_ncm_join.sql
--
-- Pareamento XML × SPED por NCM em vez de POSIÇÃO (xi.n_item = c170.num_item).
--
-- Problema (caso real, NF 170022 ITW Chemical, 2026-07-07 — achado do Gilson):
-- o ERP NÃO preserva a ordem dos itens do XML na escrituração. Item 2 do SPED
-- era ROXIL/34029039, item 2 do XML era ARCLEAN N/27101290 — o join posicional
-- cruzava valor de um produto com NCM de outro, e o Bloco C distribuía as
-- bases entre NCMs errados (troca a regra/MVA aplicada por NCM).
--
-- Fix: o XML é agregado por (nota, NCM) e juntado ao lado SPED na MESMA
-- granularidade — imune à ordem. Quando o mesmo (nota, NCM) se divide em mais
-- de um CFOP de entrada no SPED, o valor XML é rateado pró-rata pelo vl_item.
-- Quando o NCM do cadastro 0200 diverge do NCM do XML (cadastro ERP errado),
-- o join não encontra e a linha cai no vl_item do SPED (tem_xml=false),
-- comportamento idêntico a uma nota sem XML.
--
-- Mantém as regras das migrations anteriores: vProd do XML preferido (143),
-- IPI C170 → C190 → XML (127/142), NCM granular (142), rateios de frete/outro
-- e ST por participação do item (128/142).

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
fonte AS (
    SELECT
        s.company_id,
        s.c100_id,
        s.cfop,
        s.ncm_8,
        -- vProd do XML (sem IPI — spec NF-e) pareado por NCM; rateio pró-rata
        -- quando o mesmo (nota, NCM) aparece em mais de um CFOP de entrada.
        -- Sem XML (ou NCM do 0200 divergente do XML): vl_item do SPED.
        COALESCE(
            xn.v_prod * s.sped_vl_item
                / NULLIF(SUM(s.sped_vl_item) OVER (PARTITION BY s.c100_id, s.ncm_8), 0),
            s.sped_vl_item
        )                                                                     AS sum_item,
        -- IPI: C170 do item (mais preciso) → C190 por CFOP → XML por NCM → 0
        COALESCE(
            s.sped_vl_ipi,
            s.c190_vl_ipi,
            xn.v_ipi * s.sped_vl_item
                / NULLIF(SUM(s.sped_vl_item) OVER (PARTITION BY s.c100_id, s.ncm_8), 0),
            0
        )                                                                     AS ipi_eff,
        (xn.nfe_id IS NOT NULL)                                               AS tem_xml,
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
