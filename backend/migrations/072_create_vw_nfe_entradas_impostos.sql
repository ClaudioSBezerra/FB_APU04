-- 072_create_vw_nfe_entradas_impostos.sql
-- View agregada de NF-e de entrada por empresa/filial/mês com totais de impostos.
-- Fonte exclusiva: tabela nfe_entradas (ERP Bridge ou upload XML).
-- Independente do SPED — sem JOIN com mv_mercadorias ou reg_c100.
--
-- Colunas de imposto:
--   total_ipi        = ipi (ERP Bridge) ou v_ipi (XML) — mutuamente exclusivos
--   total_icms_st    = icms_st (Bridge) ou v_st (XML)
--   total_icms_part  = icms_partilha
--   total_pis        = pis (Bridge) ou v_pis (XML)
--   total_cofins     = cofins (Bridge) ou v_cofins (XML)

CREATE OR REPLACE VIEW vw_nfe_entradas_impostos AS
SELECT
    company_id,
    dest_cnpj_cpf                                                   AS filial_cnpj,
    mes_ano,
    SUM(COALESCE(ipi,0)          + COALESCE(v_ipi,0))              AS total_ipi,
    SUM(COALESCE(icms_st,0)      + COALESCE(v_st,0))               AS total_icms_st,
    SUM(COALESCE(icms_partilha,0))                                  AS total_icms_part,
    SUM(COALESCE(pis,0)          + COALESCE(v_pis,0))              AS total_pis,
    SUM(COALESCE(cofins,0)       + COALESCE(v_cofins,0))           AS total_cofins,
    COUNT(*)                                                        AS qtd_notas
FROM nfe_entradas
WHERE COALESCE(cancelado,'N') <> 'S'
GROUP BY company_id, dest_cnpj_cpf, mes_ano;
