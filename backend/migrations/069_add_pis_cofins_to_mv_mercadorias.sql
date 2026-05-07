-- 069_add_pis_cofins_to_mv_mercadorias.sql
-- Adiciona vl_pis_origem e vl_cofins_origem à mv_mercadorias_agregada.
-- Fonte: nfe_entradas/nfe_saidas (tabelas de NF-e importadas via XML ou ERP Bridge).
-- Ligação: reg_c100.chv_nfe = nfe_entradas.chave_nfe
-- Distribuição: proporcional por linha C190 (vl_opr / c100.vl_doc), evita double-count.

DROP MATERIALIZED VIEW IF EXISTS mv_mercadorias_agregada;

CREATE MATERIALIZED VIEW mv_mercadorias_agregada AS

-- 1. Mercadorias (C100 + C190) com PIS/COFINS das NF-e
SELECT
    j.company_id,
    j.company_name AS filial_nome,
    j.cnpj         AS filial_cnpj,
    TO_CHAR(COALESCE(c.dt_e_s, c.dt_doc), 'MM/YYYY')          AS mes_ano,
    EXTRACT(YEAR FROM COALESCE(c.dt_e_s, c.dt_doc))::INTEGER   AS ano,
    CASE WHEN c.ind_oper = '0' THEN 'ENTRADA' ELSE 'SAIDA' END AS tipo,
    COALESCE(f.tipo, 'O')                                       AS tipo_cfop,
    'C100'                                                       AS origem,
    CASE
        WHEN c.ind_oper = '0' THEN
            CASE f.tipo
                WHEN 'R' THEN 'Entrada_Revenda'
                WHEN 'C' THEN 'Entradas_Consumo'
                WHEN 'T' THEN 'Entradas_Transferencia'
                WHEN 'A' THEN 'Entradas_Imobilizado'
                WHEN 'O' THEN 'Entradas_Outros'
                ELSE          'Entradas_NaoIdent'
            END
        ELSE
            CASE f.tipo
                WHEN 'R' THEN 'Saidas_Revenda'
                WHEN 'C' THEN 'Saidas_Consumo'
                WHEN 'T' THEN 'Saidas_Transferencia'
                WHEN 'A' THEN 'Saidas_Imobilizado'
                WHEN 'O' THEN 'Saidas_Outros'
                ELSE          'Saidas_NaoIdent'
            END
    END AS tipo_operacao,
    SUM(c190.vl_opr)  AS valor_contabil,
    SUM(c190.vl_icms) AS vl_icms_origem,
    SUM(c190.vl_ipi)  AS vl_ipi_origem,
    -- PIS/COFINS distribuídos proporcionalmente entre linhas C190 do mesmo documento
    SUM(CASE WHEN c.vl_doc > 0
             THEN COALESCE(nfe.vl_pis_doc, 0) * c190.vl_opr / c.vl_doc
             ELSE 0 END) AS vl_pis_origem,
    SUM(CASE WHEN c.vl_doc > 0
             THEN COALESCE(nfe.vl_cofins_doc, 0) * c190.vl_opr / c.vl_doc
             ELSE 0 END) AS vl_cofins_origem
FROM reg_c190 c190
JOIN reg_c100 c ON c.id = c190.id_pai_c100
JOIN import_jobs j ON j.id = c.job_id
LEFT JOIN cfop f ON c190.cfop = f.cfop
LEFT JOIN (
    SELECT chave_nfe, company_id,
           v_pis    + COALESCE(pis,    0) AS vl_pis_doc,
           v_cofins + COALESCE(cofins, 0) AS vl_cofins_doc
    FROM nfe_entradas
    UNION ALL
    SELECT chave_nfe, company_id,
           v_pis    + COALESCE(pis,    0),
           v_cofins + COALESCE(cofins, 0)
    FROM nfe_saidas
) nfe ON nfe.chave_nfe = c.chv_nfe AND nfe.company_id = j.company_id
GROUP BY 1, 2, 3, 4, 5, 6, 7, 8, 9

UNION ALL

-- 2. Transporte (D100) — sem PIS/COFINS
SELECT
    j.company_id,
    j.company_name AS filial_nome,
    j.cnpj         AS filial_cnpj,
    TO_CHAR(COALESCE(d.dt_a_p, d.dt_doc), 'MM/YYYY')          AS mes_ano,
    EXTRACT(YEAR FROM COALESCE(d.dt_a_p, d.dt_doc))::INTEGER   AS ano,
    CASE WHEN d.ind_oper = '0' THEN 'ENTRADA' ELSE 'SAIDA' END AS tipo,
    'R'              AS tipo_cfop,
    'D100'           AS origem,
    'Entradas_Frete' AS tipo_operacao,
    SUM(d.vl_doc)  AS valor_contabil,
    SUM(d.vl_icms) AS vl_icms_origem,
    0::numeric     AS vl_ipi_origem,
    0::numeric     AS vl_pis_origem,
    0::numeric     AS vl_cofins_origem
FROM reg_d100 d
JOIN import_jobs j ON j.id = d.job_id
GROUP BY 1, 2, 3, 4, 5, 6, 7, 8, 9

UNION ALL

-- 3. Energia/Água/Gás (C500) — sem PIS/COFINS
SELECT
    j.company_id,
    j.company_name AS filial_nome,
    j.cnpj         AS filial_cnpj,
    TO_CHAR(COALESCE(c5.dt_e_s, c5.dt_doc), 'MM/YYYY')       AS mes_ano,
    EXTRACT(YEAR FROM COALESCE(c5.dt_e_s, c5.dt_doc))::INTEGER AS ano,
    'ENTRADA'              AS tipo,
    'C'                    AS tipo_cfop,
    'C500'                 AS origem,
    'Entradas_Energia_Agua' AS tipo_operacao,
    SUM(c5.vl_doc)  AS valor_contabil,
    SUM(c5.vl_icms) AS vl_icms_origem,
    0::numeric      AS vl_ipi_origem,
    0::numeric      AS vl_pis_origem,
    0::numeric      AS vl_cofins_origem
FROM reg_c500 c5
JOIN import_jobs j ON j.id = c5.job_id
GROUP BY 1, 2, 3, 4, 5, 6, 7, 8, 9

UNION ALL

-- 4. Comunicação (D500) — sem PIS/COFINS
SELECT
    j.company_id,
    j.company_name AS filial_nome,
    j.cnpj         AS filial_cnpj,
    TO_CHAR(COALESCE(d5.dt_a_p, d5.dt_doc), 'MM/YYYY')        AS mes_ano,
    EXTRACT(YEAR FROM COALESCE(d5.dt_a_p, d5.dt_doc))::INTEGER  AS ano,
    CASE WHEN d5.ind_oper = '0' THEN 'ENTRADA' ELSE 'SAIDA' END AS tipo,
    'C'                     AS tipo_cfop,
    'D500'                  AS origem,
    'Entradas_Comunicações' AS tipo_operacao,
    SUM(d5.vl_doc)  AS valor_contabil,
    SUM(d5.vl_icms) AS vl_icms_origem,
    0::numeric      AS vl_ipi_origem,
    0::numeric      AS vl_pis_origem,
    0::numeric      AS vl_cofins_origem
FROM reg_d500 d5
JOIN import_jobs j ON j.id = d5.job_id
GROUP BY 1, 2, 3, 4, 5, 6, 7, 8, 9

UNION ALL

-- 5. Consolidação Energia (C600) — sem PIS/COFINS
SELECT
    j.company_id,
    j.company_name AS filial_nome,
    j.cnpj         AS filial_cnpj,
    TO_CHAR(c6.dt_doc, 'MM/YYYY')          AS mes_ano,
    EXTRACT(YEAR FROM c6.dt_doc)::INTEGER   AS ano,
    'SAIDA'              AS tipo,
    'O'                  AS tipo_cfop,
    'C600'               AS origem,
    'Saidas_Energia_Agua' AS tipo_operacao,
    SUM(c6.vl_doc)  AS valor_contabil,
    0               AS vl_icms_origem,
    0::numeric      AS vl_ipi_origem,
    0::numeric      AS vl_pis_origem,
    0::numeric      AS vl_cofins_origem
FROM reg_c600 c6
JOIN import_jobs j ON j.id = c6.job_id
GROUP BY 1, 2, 3, 4, 5, 6, 7, 8, 9;

CREATE INDEX idx_mv_mercadorias_agregada_filial   ON mv_mercadorias_agregada(filial_nome);
CREATE INDEX idx_mv_mercadorias_agregada_cnpj     ON mv_mercadorias_agregada(filial_cnpj);
CREATE INDEX idx_mv_mercadorias_agregada_mes      ON mv_mercadorias_agregada(mes_ano);
CREATE INDEX idx_mv_mercadorias_agregada_company  ON mv_mercadorias_agregada(company_id);
CREATE INDEX idx_mv_mercadorias_agregada_tipo_op  ON mv_mercadorias_agregada(tipo_operacao);

CREATE UNIQUE INDEX idx_mv_mercadorias_agregada_v4
ON mv_mercadorias_agregada (company_id, filial_nome, filial_cnpj, mes_ano, ano, tipo, tipo_cfop, origem, tipo_operacao);
