-- ============================================================================
-- explain_fronteira_resumo.sql  (GERADO de handlers/icms_fronteira.go)
--
-- Objetivo: confirmar o gargalo do módulo Fronteira no banco do CLIENTE antes
-- de materializar o NCM na MV (correção #2). Mede a query do /resumo.
--
-- COMO USAR (no banco do cliente, read-only — EXPLAIN ANALYZE roda a query mas
-- não altera dados):
--   1. Ajuste as duas variáveis abaixo (company_id do cliente e um período real).
--   2. psql "<conn-do-cliente>" -f backend/dev/explain_fronteira_resumo.sql > plano.txt
--   3. Me envie o plano.txt (as duas saídas: UM mês e TODOS os meses).
--
-- O que procurar no plano:
--   • "Nested Loop" + "SubPlan"/"LATERAL" com loops = nº de notas  → custo por linha
--   • tempo dominante nas subconsultas de nfe_entradas_itens / reg_c170 / reg_0200
--   • diferença de tempo entre a query de 1 mês vs. todos os meses (efeito da #1)
-- ============================================================================

\set company_id 'COLE-AQUI-O-UUID-DA-EMPRESA'
\set periodo '03/2026'

\echo '==================== RESUMO — UM MÊS (:periodo) ===================='
EXPLAIN (ANALYZE, BUFFERS, VERBOSE, TIMING)
WITH
classified AS (
    SELECT
        c100.chv_nfe                                        AS chave_nfe,
        c100.dt_doc::text                                   AS data_emissao,
        COALESCE(c100.num_doc, '')                          AS numero_nfe,
        COALESCE(part.cnpj, ne.forn_cnpj, '')               AS forn_cnpj,
        COALESCE(part.nome, ne.forn_nome, '')               AS forn_nome,
        -- forn_uf: 1º o XML (mais preciso), senão a UF resolvida via município
        -- do participante (reg 0150 do SPED → cod_mun → municipios_ibge.uf).
        -- Sem o fallback, o Bloco A (NFs de meses anteriores no SPED) ficava
        -- com UF vazia quando o XML do fornecedor não estava importado.
        COALESCE(NULLIF(ne.forn_uf, ''), NULLIF(m_part.uf, ''), '') AS forn_uf,
        l.cfop                                              AS cfop,
        l.v_prod_disp                                       AS v_prod,
        COALESCE(l.ipi_eff, 0)                              AS v_ipi,
        -- Crédito interestadual: prioriza vl_icms_inter (SUM de bc×aliq por linha
        -- do C190, cap 12%) que preserva mix de alíquotas dentro do mesmo CFOP.
        -- Fallback: v_prod × aliq quando vl_bc_icms não foi preenchido no SPED.
        COALESCE(
            NULLIF(l.vl_icms_inter, 0),
            l.v_prod_disp * COALESCE(NULLIF(l.aliq_icms, 0), 12.0) / 100.0
        ) AS v_icms,
        COALESCE(l.vl_bc_st, 0)                             AS v_bc_st,
        COALESCE(l.vl_icms_st, 0)                           AS v_st,
        COALESCE(NULLIF(l.aliq_icms, 0), 12.0)              AS aliq_inter,
        COALESCE(regra.aliquota_interna, 20.5)              AS aliq_interna,
        -- ST só se aplica quando a regra NCM tem segmento_codigo cadastrado E a
        -- empresa tem esse segmento registrado (company_segmentos). Sem match →
        -- reclassifica como ANTECIPAÇÃO (decisão do contador, 2026-05).
        CASE
            WHEN l.cfop IN ('2551','2556')
                THEN 'DIFAL'
            WHEN l.cfop IN ('2403','2409','2651','2652')
                THEN CASE
                    WHEN regra.segmento_codigo IS NOT NULL
                      AND EXISTS (
                          SELECT 1 FROM company_segmentos cs
                          WHERE cs.company_id = (:'company_id')::uuid::uuid
                            AND cs.segmento_codigo = regra.segmento_codigo
                            AND cs.uf = COALESCE(j.uf, 'PE')
                      )
                    THEN 'ST'
                    ELSE 'ANTECIPACAO'
                END
            WHEN l.cfop IN ('2101','2102','2152')
                THEN 'ANTECIPACAO'
        END                                                 AS regime,
        CASE
            WHEN (:'periodo')::text::text = ''
              OR (EXTRACT(MONTH FROM c100.dt_doc)::int = SPLIT_PART((:'periodo')::text::text,'/',1)::int
                  AND EXTRACT(YEAR  FROM c100.dt_doc)::int = SPLIT_PART((:'periodo')::text::text,'/',2)::int)
            THEN 'mes_atual'
            ELSE 'mes_anterior'
        END                                                 AS bloco,
        -- ICMS devido estimado por regime. Base = l.base_calc (já inclui IPI/
        -- frete quando há XML, ou vl_opr do SPED quando não há).
        CASE
            -- PRODEPE / regime especial de central de distribuição (art. 11-A do
            -- Dec. 21.959/1999): a filial beneficiada é DISPENSADA de antecipação
            -- E de ST nas aquisições → ICMS fronteira = 0. Identificação por CNPJ
            -- da filial recebedora (import_jobs.cnpj) com vigência cobrindo a data
            -- do documento. DIFAL (2551/2556) fica FORA da dispensa. EXISTS evita
            -- multiplicar linhas quando há mais de um enquadramento p/ o mesmo CNPJ.
            -- O regime classificado é preservado — só o valor é zerado.
            WHEN l.cfop NOT IN ('2551','2556')
             AND EXISTS (
                 SELECT 1 FROM prodepe_enquadramentos pe
                 WHERE pe.company_id = (:'company_id')::uuid
                   AND pe.ativo = true
                   AND pe.dispensa_antecipacao = true
                   AND regexp_replace(pe.cnpj, '[^0-9]', '', 'g')
                       = regexp_replace(COALESCE(j.cnpj, ''), '[^0-9]', '', 'g')
                   AND (pe.vigencia_inicio IS NULL OR c100.dt_doc >= pe.vigencia_inicio)
                   AND (pe.vigencia_fim    IS NULL OR c100.dt_doc <= pe.vigencia_fim)
             )
                THEN 0
            WHEN l.cfop IN ('2551','2556')
                THEN CASE WHEN COALESCE(ufb.base_por_dentro, false)
                    -- DIFAL por dentro (PE): base = (operação − crédito inter.) /
                    -- (1 − alíq_interna), aplicada à diferença de alíquotas, sem dedução.
                    THEN GREATEST(0,
                        ((l.base_calc - COALESCE(NULLIF(l.vl_icms_inter,0), l.v_prod_disp * COALESCE(NULLIF(l.aliq_icms,0),12.0) / 100.0))
                         / NULLIF(1.0 - COALESCE(regra.aliquota_interna,20.5)/100.0, 0))
                        * (COALESCE(regra.aliquota_interna,20.5) - COALESCE(NULLIF(l.aliq_icms,0),12.0)) / 100.0)
                    ELSE GREATEST(0,
                        l.base_calc * (
                            COALESCE(regra.aliquota_interna, 20.5)
                            - COALESCE(NULLIF(l.aliq_icms, 0), 12.0)
                        ) / 100.0)
                END
            WHEN l.cfop IN ('2403','2409','2651','2652')
                THEN CASE
                    -- ST: segmento da empresa coincide com o da regra NCM
                    WHEN regra.segmento_codigo IS NOT NULL
                      AND EXISTS (
                          SELECT 1 FROM company_segmentos cs
                          WHERE cs.company_id = (:'company_id')::uuid::uuid
                            AND cs.segmento_codigo = regra.segmento_codigo
                            AND cs.uf = COALESCE(j.uf, 'PE')
                      )
                    THEN CASE
                        -- MVA efetivo: ajustado pré-calc por alíquota interestadual real,
                        -- fallback Convênio 110/07 a partir do MVA original, fallback MVA original.
                        WHEN COALESCE(
                            CASE COALESCE(NULLIF(l.aliq_icms,0),12.0)
                                WHEN 4.0  THEN regra.mva_ajustado_4pct
                                WHEN 7.0  THEN regra.mva_ajustado_7pct
                                WHEN 12.0 THEN regra.mva_ajustado_12pct
                            END,
                            CASE WHEN regra.mva_original IS NOT NULL AND COALESCE(regra.aliquota_interna,20.5) < 100 THEN
                                ((1.0 + regra.mva_original/100.0) * (1.0 - COALESCE(NULLIF(l.aliq_icms,0),12.0)/100.0)
                                 / NULLIF(1.0 - COALESCE(regra.aliquota_interna,20.5)/100.0, 0) - 1.0) * 100.0
                            END,
                            regra.mva_original
                        ) IS NOT NULL
                            THEN GREATEST(0,
                                l.base_calc
                                * (1.0 + COALESCE(
                                    CASE COALESCE(NULLIF(l.aliq_icms,0),12.0)
                                        WHEN 4.0  THEN regra.mva_ajustado_4pct
                                        WHEN 7.0  THEN regra.mva_ajustado_7pct
                                        WHEN 12.0 THEN regra.mva_ajustado_12pct
                                    END,
                                    CASE WHEN regra.mva_original IS NOT NULL AND COALESCE(regra.aliquota_interna,20.5) < 100 THEN
                                        ((1.0 + regra.mva_original/100.0) * (1.0 - COALESCE(NULLIF(l.aliq_icms,0),12.0)/100.0)
                                         / NULLIF(1.0 - COALESCE(regra.aliquota_interna,20.5)/100.0, 0) - 1.0) * 100.0
                                    END,
                                    regra.mva_original
                                )/100.0)
                                * COALESCE(regra.aliquota_interna, 20.5)/100.0
                                - COALESCE(NULLIF(l.vl_icms_inter,0), l.v_prod_disp * COALESCE(NULLIF(l.aliq_icms, 0), 12.0) / 100.0))
                        ELSE COALESCE(l.vl_icms_st, 0)
                    END
                    -- Sem segmento cadastrado → reclassificado como ANTECIPAÇÃO
                    ELSE CASE WHEN COALESCE(ufb.base_por_dentro, false)
                        THEN GREATEST(0,
                            ((l.base_calc - COALESCE(NULLIF(l.vl_icms_inter,0), l.v_prod_disp * COALESCE(NULLIF(l.aliq_icms,0),12.0) / 100.0))
                             / NULLIF(1.0 - COALESCE(regra.aliquota_interna,20.5)/100.0, 0))
                            * COALESCE(regra.aliquota_interna,20.5)/100.0
                            - COALESCE(NULLIF(l.vl_icms_inter,0), l.v_prod_disp * COALESCE(NULLIF(l.aliq_icms,0),12.0) / 100.0))
                        ELSE GREATEST(0,
                            l.base_calc * COALESCE(regra.aliquota_interna, 20.5)/100.0
                            - COALESCE(NULLIF(l.vl_icms_inter,0), l.v_prod_disp * COALESCE(NULLIF(l.aliq_icms, 0), 12.0) / 100.0))
                    END
                END
            WHEN l.cfop IN ('2101','2102','2152')
                -- Antecipação. Por dentro (PE): base = (operação − crédito inter.) /
                -- (1 − alíq_interna), depois × alíq_interna − crédito inter.
                THEN CASE WHEN COALESCE(ufb.base_por_dentro, false)
                    THEN GREATEST(0,
                        ((l.base_calc - COALESCE(NULLIF(l.vl_icms_inter,0), l.v_prod_disp * COALESCE(NULLIF(l.aliq_icms,0),12.0) / 100.0))
                         / NULLIF(1.0 - COALESCE(regra.aliquota_interna,20.5)/100.0, 0))
                        * COALESCE(regra.aliquota_interna,20.5)/100.0
                        - COALESCE(NULLIF(l.vl_icms_inter,0), l.v_prod_disp * COALESCE(NULLIF(l.aliq_icms,0),12.0) / 100.0))
                    ELSE GREATEST(0,
                        l.base_calc * COALESCE(regra.aliquota_interna, 20.5)/100.0
                        - COALESCE(NULLIF(l.vl_icms_inter,0), l.v_prod_disp * COALESCE(NULLIF(l.aliq_icms, 0), 12.0) / 100.0))
                END
            ELSE 0
        END                                                 AS icms_devido_est,
        COALESCE(j.uf, 'PE')                                AS uf_filial,
        -- Campos crus expostos para o relatório "Incentivo" recalcular o
        -- icms_que_seria_devido (sem o branch PRODEPE) e fazer JOIN por CNPJ.
        -- Nenhum SELECT atual referencia estas colunas — adição inócua.
        COALESCE(j.cnpj, '')                                AS cnpj_filial,
        l.base_calc                                         AS base_calc,
        regra.aliquota_interna                              AS regra_aliq_interna,
        regra.mva_original                                  AS regra_mva_original,
        regra.mva_ajustado_4pct                             AS regra_mva_4,
        regra.mva_ajustado_7pct                             AS regra_mva_7,
        regra.mva_ajustado_12pct                            AS regra_mva_12,
        regra.segmento_codigo                               AS regra_seg_codigo,
        COALESCE(ufb.base_por_dentro, false)                AS base_por_dentro
    FROM mv_icms_fronteira_linhas l
    JOIN reg_c100 c100 ON c100.id = l.c100_id
    JOIN import_jobs j ON j.id = c100.job_id
    LEFT JOIN participants part
        ON part.job_id = c100.job_id AND part.cod_part = c100.cod_part
    LEFT JOIN municipios_ibge m_part ON m_part.codigo_ibge = part.cod_mun
    LEFT JOIN nfe_entradas ne ON ne.company_id = j.company_id AND ne.chave_nfe = c100.chv_nfe
    LEFT JOIN LATERAL (
        SELECT COALESCE(
            (SELECT nii.ncm
             FROM nfe_entradas_itens nii
             WHERE nii.nfe_id = ne.id AND NULLIF(nii.ncm, '') IS NOT NULL
             ORDER BY nii.v_prod DESC NULLS LAST
             LIMIT 1),
            (SELECT LEFT(regexp_replace(p.cod_ncm, '[^0-9]', '', 'g'), 8)
             FROM reg_c170 ci
             JOIN reg_0200 p ON p.job_id = c100.job_id AND p.cod_item = ci.cod_item
             WHERE ci.c100_id = c100.id AND ci.cfop = l.cfop
               AND NULLIF(regexp_replace(p.cod_ncm, '[^0-9]', '', 'g'), '') IS NOT NULL
             ORDER BY ci.vl_item DESC NULLS LAST
             LIMIT 1)
        ) AS ncm
    ) top_item ON true
    LEFT JOIN LATERAL (
        SELECT r.aliquota_interna, r.mva_original,
               r.mva_ajustado_4pct, r.mva_ajustado_7pct, r.mva_ajustado_12pct,
               r.segmento_codigo
        FROM icms_fronteira_regras_ncm r
        WHERE (r.company_id = (:'company_id')::uuid OR r.company_id IS NULL)
          AND r.uf_estado = COALESCE(j.uf, 'PE')
          AND top_item.ncm IS NOT NULL
          AND LEFT(top_item.ncm, LENGTH(r.ncm_prefixo)) = r.ncm_prefixo
          AND LENGTH(r.ncm_prefixo) >= 4
        ORDER BY r.company_id NULLS LAST, LENGTH(r.ncm_prefixo) DESC
        LIMIT 1
    ) regra ON true
    LEFT JOIN uf_beneficios_fiscais ufb
        ON ufb.company_id = (:'company_id')::uuid AND ufb.uf = COALESCE(j.uf, 'PE')
    WHERE l.company_id = (:'company_id')::uuid
      AND c100.cod_sit NOT IN ('02','03','04','05')
      AND ((:'periodo')::text::text = '' OR j.mes_ano = (:'periodo')::text
          OR (j.mes_ano IS NULL AND (
              EXTRACT(MONTH FROM j.dt_ini)::int = SPLIT_PART((:'periodo')::text::text,'/',1)::int
              AND EXTRACT(YEAR  FROM j.dt_ini)::int = SPLIT_PART((:'periodo')::text::text,'/',2)::int
          ))
      )
)
SELECT
    regime,
    COUNT(DISTINCT chave_nfe) AS qtd_notas,
    SUM(v_prod)         AS v_prod_total,
    SUM(v_ipi)          AS v_ipi_total,
    SUM(v_st)           AS v_st_retido,
    SUM(icms_devido_est) AS icms_devido_est
FROM classified
WHERE regime IS NOT NULL
GROUP BY regime
ORDER BY regime;

\echo '==================== RESUMO — TODOS OS MESES (periodo = '''') ===================='
\set periodo ''
EXPLAIN (ANALYZE, BUFFERS, VERBOSE, TIMING)
WITH
classified AS (
    SELECT
        c100.chv_nfe                                        AS chave_nfe,
        c100.dt_doc::text                                   AS data_emissao,
        COALESCE(c100.num_doc, '')                          AS numero_nfe,
        COALESCE(part.cnpj, ne.forn_cnpj, '')               AS forn_cnpj,
        COALESCE(part.nome, ne.forn_nome, '')               AS forn_nome,
        -- forn_uf: 1º o XML (mais preciso), senão a UF resolvida via município
        -- do participante (reg 0150 do SPED → cod_mun → municipios_ibge.uf).
        -- Sem o fallback, o Bloco A (NFs de meses anteriores no SPED) ficava
        -- com UF vazia quando o XML do fornecedor não estava importado.
        COALESCE(NULLIF(ne.forn_uf, ''), NULLIF(m_part.uf, ''), '') AS forn_uf,
        l.cfop                                              AS cfop,
        l.v_prod_disp                                       AS v_prod,
        COALESCE(l.ipi_eff, 0)                              AS v_ipi,
        -- Crédito interestadual: prioriza vl_icms_inter (SUM de bc×aliq por linha
        -- do C190, cap 12%) que preserva mix de alíquotas dentro do mesmo CFOP.
        -- Fallback: v_prod × aliq quando vl_bc_icms não foi preenchido no SPED.
        COALESCE(
            NULLIF(l.vl_icms_inter, 0),
            l.v_prod_disp * COALESCE(NULLIF(l.aliq_icms, 0), 12.0) / 100.0
        ) AS v_icms,
        COALESCE(l.vl_bc_st, 0)                             AS v_bc_st,
        COALESCE(l.vl_icms_st, 0)                           AS v_st,
        COALESCE(NULLIF(l.aliq_icms, 0), 12.0)              AS aliq_inter,
        COALESCE(regra.aliquota_interna, 20.5)              AS aliq_interna,
        -- ST só se aplica quando a regra NCM tem segmento_codigo cadastrado E a
        -- empresa tem esse segmento registrado (company_segmentos). Sem match →
        -- reclassifica como ANTECIPAÇÃO (decisão do contador, 2026-05).
        CASE
            WHEN l.cfop IN ('2551','2556')
                THEN 'DIFAL'
            WHEN l.cfop IN ('2403','2409','2651','2652')
                THEN CASE
                    WHEN regra.segmento_codigo IS NOT NULL
                      AND EXISTS (
                          SELECT 1 FROM company_segmentos cs
                          WHERE cs.company_id = (:'company_id')::uuid::uuid
                            AND cs.segmento_codigo = regra.segmento_codigo
                            AND cs.uf = COALESCE(j.uf, 'PE')
                      )
                    THEN 'ST'
                    ELSE 'ANTECIPACAO'
                END
            WHEN l.cfop IN ('2101','2102','2152')
                THEN 'ANTECIPACAO'
        END                                                 AS regime,
        CASE
            WHEN (:'periodo')::text::text = ''
              OR (EXTRACT(MONTH FROM c100.dt_doc)::int = SPLIT_PART((:'periodo')::text::text,'/',1)::int
                  AND EXTRACT(YEAR  FROM c100.dt_doc)::int = SPLIT_PART((:'periodo')::text::text,'/',2)::int)
            THEN 'mes_atual'
            ELSE 'mes_anterior'
        END                                                 AS bloco,
        -- ICMS devido estimado por regime. Base = l.base_calc (já inclui IPI/
        -- frete quando há XML, ou vl_opr do SPED quando não há).
        CASE
            -- PRODEPE / regime especial de central de distribuição (art. 11-A do
            -- Dec. 21.959/1999): a filial beneficiada é DISPENSADA de antecipação
            -- E de ST nas aquisições → ICMS fronteira = 0. Identificação por CNPJ
            -- da filial recebedora (import_jobs.cnpj) com vigência cobrindo a data
            -- do documento. DIFAL (2551/2556) fica FORA da dispensa. EXISTS evita
            -- multiplicar linhas quando há mais de um enquadramento p/ o mesmo CNPJ.
            -- O regime classificado é preservado — só o valor é zerado.
            WHEN l.cfop NOT IN ('2551','2556')
             AND EXISTS (
                 SELECT 1 FROM prodepe_enquadramentos pe
                 WHERE pe.company_id = (:'company_id')::uuid
                   AND pe.ativo = true
                   AND pe.dispensa_antecipacao = true
                   AND regexp_replace(pe.cnpj, '[^0-9]', '', 'g')
                       = regexp_replace(COALESCE(j.cnpj, ''), '[^0-9]', '', 'g')
                   AND (pe.vigencia_inicio IS NULL OR c100.dt_doc >= pe.vigencia_inicio)
                   AND (pe.vigencia_fim    IS NULL OR c100.dt_doc <= pe.vigencia_fim)
             )
                THEN 0
            WHEN l.cfop IN ('2551','2556')
                THEN CASE WHEN COALESCE(ufb.base_por_dentro, false)
                    -- DIFAL por dentro (PE): base = (operação − crédito inter.) /
                    -- (1 − alíq_interna), aplicada à diferença de alíquotas, sem dedução.
                    THEN GREATEST(0,
                        ((l.base_calc - COALESCE(NULLIF(l.vl_icms_inter,0), l.v_prod_disp * COALESCE(NULLIF(l.aliq_icms,0),12.0) / 100.0))
                         / NULLIF(1.0 - COALESCE(regra.aliquota_interna,20.5)/100.0, 0))
                        * (COALESCE(regra.aliquota_interna,20.5) - COALESCE(NULLIF(l.aliq_icms,0),12.0)) / 100.0)
                    ELSE GREATEST(0,
                        l.base_calc * (
                            COALESCE(regra.aliquota_interna, 20.5)
                            - COALESCE(NULLIF(l.aliq_icms, 0), 12.0)
                        ) / 100.0)
                END
            WHEN l.cfop IN ('2403','2409','2651','2652')
                THEN CASE
                    -- ST: segmento da empresa coincide com o da regra NCM
                    WHEN regra.segmento_codigo IS NOT NULL
                      AND EXISTS (
                          SELECT 1 FROM company_segmentos cs
                          WHERE cs.company_id = (:'company_id')::uuid::uuid
                            AND cs.segmento_codigo = regra.segmento_codigo
                            AND cs.uf = COALESCE(j.uf, 'PE')
                      )
                    THEN CASE
                        -- MVA efetivo: ajustado pré-calc por alíquota interestadual real,
                        -- fallback Convênio 110/07 a partir do MVA original, fallback MVA original.
                        WHEN COALESCE(
                            CASE COALESCE(NULLIF(l.aliq_icms,0),12.0)
                                WHEN 4.0  THEN regra.mva_ajustado_4pct
                                WHEN 7.0  THEN regra.mva_ajustado_7pct
                                WHEN 12.0 THEN regra.mva_ajustado_12pct
                            END,
                            CASE WHEN regra.mva_original IS NOT NULL AND COALESCE(regra.aliquota_interna,20.5) < 100 THEN
                                ((1.0 + regra.mva_original/100.0) * (1.0 - COALESCE(NULLIF(l.aliq_icms,0),12.0)/100.0)
                                 / NULLIF(1.0 - COALESCE(regra.aliquota_interna,20.5)/100.0, 0) - 1.0) * 100.0
                            END,
                            regra.mva_original
                        ) IS NOT NULL
                            THEN GREATEST(0,
                                l.base_calc
                                * (1.0 + COALESCE(
                                    CASE COALESCE(NULLIF(l.aliq_icms,0),12.0)
                                        WHEN 4.0  THEN regra.mva_ajustado_4pct
                                        WHEN 7.0  THEN regra.mva_ajustado_7pct
                                        WHEN 12.0 THEN regra.mva_ajustado_12pct
                                    END,
                                    CASE WHEN regra.mva_original IS NOT NULL AND COALESCE(regra.aliquota_interna,20.5) < 100 THEN
                                        ((1.0 + regra.mva_original/100.0) * (1.0 - COALESCE(NULLIF(l.aliq_icms,0),12.0)/100.0)
                                         / NULLIF(1.0 - COALESCE(regra.aliquota_interna,20.5)/100.0, 0) - 1.0) * 100.0
                                    END,
                                    regra.mva_original
                                )/100.0)
                                * COALESCE(regra.aliquota_interna, 20.5)/100.0
                                - COALESCE(NULLIF(l.vl_icms_inter,0), l.v_prod_disp * COALESCE(NULLIF(l.aliq_icms, 0), 12.0) / 100.0))
                        ELSE COALESCE(l.vl_icms_st, 0)
                    END
                    -- Sem segmento cadastrado → reclassificado como ANTECIPAÇÃO
                    ELSE CASE WHEN COALESCE(ufb.base_por_dentro, false)
                        THEN GREATEST(0,
                            ((l.base_calc - COALESCE(NULLIF(l.vl_icms_inter,0), l.v_prod_disp * COALESCE(NULLIF(l.aliq_icms,0),12.0) / 100.0))
                             / NULLIF(1.0 - COALESCE(regra.aliquota_interna,20.5)/100.0, 0))
                            * COALESCE(regra.aliquota_interna,20.5)/100.0
                            - COALESCE(NULLIF(l.vl_icms_inter,0), l.v_prod_disp * COALESCE(NULLIF(l.aliq_icms,0),12.0) / 100.0))
                        ELSE GREATEST(0,
                            l.base_calc * COALESCE(regra.aliquota_interna, 20.5)/100.0
                            - COALESCE(NULLIF(l.vl_icms_inter,0), l.v_prod_disp * COALESCE(NULLIF(l.aliq_icms, 0), 12.0) / 100.0))
                    END
                END
            WHEN l.cfop IN ('2101','2102','2152')
                -- Antecipação. Por dentro (PE): base = (operação − crédito inter.) /
                -- (1 − alíq_interna), depois × alíq_interna − crédito inter.
                THEN CASE WHEN COALESCE(ufb.base_por_dentro, false)
                    THEN GREATEST(0,
                        ((l.base_calc - COALESCE(NULLIF(l.vl_icms_inter,0), l.v_prod_disp * COALESCE(NULLIF(l.aliq_icms,0),12.0) / 100.0))
                         / NULLIF(1.0 - COALESCE(regra.aliquota_interna,20.5)/100.0, 0))
                        * COALESCE(regra.aliquota_interna,20.5)/100.0
                        - COALESCE(NULLIF(l.vl_icms_inter,0), l.v_prod_disp * COALESCE(NULLIF(l.aliq_icms,0),12.0) / 100.0))
                    ELSE GREATEST(0,
                        l.base_calc * COALESCE(regra.aliquota_interna, 20.5)/100.0
                        - COALESCE(NULLIF(l.vl_icms_inter,0), l.v_prod_disp * COALESCE(NULLIF(l.aliq_icms, 0), 12.0) / 100.0))
                END
            ELSE 0
        END                                                 AS icms_devido_est,
        COALESCE(j.uf, 'PE')                                AS uf_filial,
        -- Campos crus expostos para o relatório "Incentivo" recalcular o
        -- icms_que_seria_devido (sem o branch PRODEPE) e fazer JOIN por CNPJ.
        -- Nenhum SELECT atual referencia estas colunas — adição inócua.
        COALESCE(j.cnpj, '')                                AS cnpj_filial,
        l.base_calc                                         AS base_calc,
        regra.aliquota_interna                              AS regra_aliq_interna,
        regra.mva_original                                  AS regra_mva_original,
        regra.mva_ajustado_4pct                             AS regra_mva_4,
        regra.mva_ajustado_7pct                             AS regra_mva_7,
        regra.mva_ajustado_12pct                            AS regra_mva_12,
        regra.segmento_codigo                               AS regra_seg_codigo,
        COALESCE(ufb.base_por_dentro, false)                AS base_por_dentro
    FROM mv_icms_fronteira_linhas l
    JOIN reg_c100 c100 ON c100.id = l.c100_id
    JOIN import_jobs j ON j.id = c100.job_id
    LEFT JOIN participants part
        ON part.job_id = c100.job_id AND part.cod_part = c100.cod_part
    LEFT JOIN municipios_ibge m_part ON m_part.codigo_ibge = part.cod_mun
    LEFT JOIN nfe_entradas ne ON ne.company_id = j.company_id AND ne.chave_nfe = c100.chv_nfe
    LEFT JOIN LATERAL (
        SELECT COALESCE(
            (SELECT nii.ncm
             FROM nfe_entradas_itens nii
             WHERE nii.nfe_id = ne.id AND NULLIF(nii.ncm, '') IS NOT NULL
             ORDER BY nii.v_prod DESC NULLS LAST
             LIMIT 1),
            (SELECT LEFT(regexp_replace(p.cod_ncm, '[^0-9]', '', 'g'), 8)
             FROM reg_c170 ci
             JOIN reg_0200 p ON p.job_id = c100.job_id AND p.cod_item = ci.cod_item
             WHERE ci.c100_id = c100.id AND ci.cfop = l.cfop
               AND NULLIF(regexp_replace(p.cod_ncm, '[^0-9]', '', 'g'), '') IS NOT NULL
             ORDER BY ci.vl_item DESC NULLS LAST
             LIMIT 1)
        ) AS ncm
    ) top_item ON true
    LEFT JOIN LATERAL (
        SELECT r.aliquota_interna, r.mva_original,
               r.mva_ajustado_4pct, r.mva_ajustado_7pct, r.mva_ajustado_12pct,
               r.segmento_codigo
        FROM icms_fronteira_regras_ncm r
        WHERE (r.company_id = (:'company_id')::uuid OR r.company_id IS NULL)
          AND r.uf_estado = COALESCE(j.uf, 'PE')
          AND top_item.ncm IS NOT NULL
          AND LEFT(top_item.ncm, LENGTH(r.ncm_prefixo)) = r.ncm_prefixo
          AND LENGTH(r.ncm_prefixo) >= 4
        ORDER BY r.company_id NULLS LAST, LENGTH(r.ncm_prefixo) DESC
        LIMIT 1
    ) regra ON true
    LEFT JOIN uf_beneficios_fiscais ufb
        ON ufb.company_id = (:'company_id')::uuid AND ufb.uf = COALESCE(j.uf, 'PE')
    WHERE l.company_id = (:'company_id')::uuid
      AND c100.cod_sit NOT IN ('02','03','04','05')
      AND ((:'periodo')::text::text = '' OR j.mes_ano = (:'periodo')::text
          OR (j.mes_ano IS NULL AND (
              EXTRACT(MONTH FROM j.dt_ini)::int = SPLIT_PART((:'periodo')::text::text,'/',1)::int
              AND EXTRACT(YEAR  FROM j.dt_ini)::int = SPLIT_PART((:'periodo')::text::text,'/',2)::int
          ))
      )
)
SELECT
    regime,
    COUNT(DISTINCT chave_nfe) AS qtd_notas,
    SUM(v_prod)         AS v_prod_total,
    SUM(v_ipi)          AS v_ipi_total,
    SUM(v_st)           AS v_st_retido,
    SUM(icms_devido_est) AS icms_devido_est
FROM classified
WHERE regime IS NOT NULL
GROUP BY regime
ORDER BY regime;
