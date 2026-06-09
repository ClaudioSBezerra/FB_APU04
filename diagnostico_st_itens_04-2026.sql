-- ============================================================================
-- DIAGNÓSTICO — ICMS-ST por item (Fronteira)
-- ----------------------------------------------------------------------------
-- Objetivo: confirmar a causa dos 3 pontos do Gilson (MVA / ICMS retido / Bloco C)
-- e validar o "apagar e reimportar". Rode ANTES e DEPOIS do reimport e compare.
--
-- COMO USAR: substitua (find/replace) os dois placeholders abaixo e rode tudo:
--   c30ceda9-56d1-4011-851a-cfa7031796d0  -> uuid da empresa/filial (ex.: '0c3f...-...')
--   04/2026     -> período do relatório (ex.: '04/2026')
--
-- Tabelas confirmadas na stItensQuery (icms_fronteira_st_itens.go).
-- ============================================================================


-- ----------------------------------------------------------------------------
-- (1) MVA — regras por NCM disponíveis por UF
--     Causa esperada: 82% das notas de ST são filial PE e NÃO há regras de PE.
--     Se a linha 'PE' não aparecer (ou vier 0), a MVA fica vazia -> AÇÃO = Gilson
--     importar as regras de PE. (Não é código.)
-- ----------------------------------------------------------------------------
SELECT COALESCE(uf_estado,'(sem UF)') AS uf_estado,
       count(*)                       AS qtd_regras
FROM icms_fronteira_regras_ncm
WHERE company_id = 'c30ceda9-56d1-4011-851a-cfa7031796d0' OR company_id IS NULL
GROUP BY uf_estado
ORDER BY qtd_regras DESC;


-- ----------------------------------------------------------------------------
-- (2) ICMS RETIDO — estado da coluna v_st POR ITEM (nfe_entradas_itens)
--     Causa esperada (ANTES do reimport): v_st NULL nas notas antigas -> retido 0.
--     DEPOIS do reimport esperamos: nulos -> 0 e com_st > 0.
-- ----------------------------------------------------------------------------
SELECT count(*)                                          AS itens_no_periodo,
       count(*) FILTER (WHERE nii.v_st IS NULL)          AS v_st_nulo,
       count(*) FILTER (WHERE COALESCE(nii.v_st,0) > 0)  AS v_st_com_valor,
       COALESCE(sum(nii.v_st),0)                         AS soma_v_st
FROM nfe_entradas_itens nii
JOIN nfe_entradas ne ON ne.id = nii.nfe_id
WHERE ne.company_id = 'c30ceda9-56d1-4011-851a-cfa7031796d0'
  AND ne.data_emissao >= to_date('04/2026','MM/YYYY')
  AND ne.data_emissao <  (to_date('04/2026','MM/YYYY') + interval '1 month');


-- ----------------------------------------------------------------------------
-- (3) BLOCO C — CFOPs das notas de entrada (reclassificados 6->2 / 5->1)
--     A lista de ST do relatório é 2403/2409/2651/2652. CFOP x404 (ST já retido)
--     reclassifica p/ 2404/1405 -> FICA DE FORA. Use isto p/ ver se as notas da
--     Rolimec entram (2403...) ou caem fora (2404/1405) -> decisão de escopo.
-- ----------------------------------------------------------------------------
SELECT nii.cfop                                              AS cfop_xml,
       CASE WHEN LEFT(nii.cfop,1)='6' THEN '2'||SUBSTRING(nii.cfop FROM 2)
            WHEN LEFT(nii.cfop,1)='5' THEN '1'||SUBSTRING(nii.cfop FROM 2)
            ELSE nii.cfop END                                AS cfop_entrada,
       CASE WHEN (CASE WHEN LEFT(nii.cfop,1)='6' THEN '2'||SUBSTRING(nii.cfop FROM 2)
                       WHEN LEFT(nii.cfop,1)='5' THEN '1'||SUBSTRING(nii.cfop FROM 2)
                       ELSE nii.cfop END) IN ('2403','2409','2651','2652')
            THEN 'DENTRO (entra no relatório)'
            ELSE 'FORA (não entra)' END                      AS situacao,
       count(*)                                              AS itens,
       COALESCE(sum(nii.v_st),0)                             AS soma_v_st
FROM nfe_entradas_itens nii
JOIN nfe_entradas ne ON ne.id = nii.nfe_id
WHERE ne.company_id = 'c30ceda9-56d1-4011-851a-cfa7031796d0'
  AND ne.data_emissao >= to_date('04/2026','MM/YYYY')
  AND ne.data_emissao <  (to_date('04/2026','MM/YYYY') + interval '1 month')
  AND (COALESCE(nii.v_st,0) > 0 OR nii.cfop LIKE '_40_' OR nii.cfop LIKE '_65_')
GROUP BY 1, 2, 3
ORDER BY situacao, itens DESC;


-- ----------------------------------------------------------------------------
-- (3b) ROLIMEC — visão direta: notas, CFOP, v_st e se estão ou não no SPED
--     (NÃO no SPED = candidatas a Bloco C). Ajuste o ILIKE se o nome variar.
-- ----------------------------------------------------------------------------
SELECT ne.numero_nfe,
       ne.forn_nome,
       nii.cfop,
       COALESCE(nii.v_st,0)                                  AS v_st_item,
       CASE WHEN EXISTS (
                SELECT 1 FROM reg_c100 c100
                JOIN import_jobs j ON j.id = c100.job_id
                WHERE j.company_id = ne.company_id
                  AND c100.chv_nfe = ne.chave_nfe
            ) THEN 'no SPED (Bloco A/B)'
            ELSE 'fora do SPED (Bloco C)' END                AS origem
FROM nfe_entradas_itens nii
JOIN nfe_entradas ne ON ne.id = nii.nfe_id
WHERE ne.company_id = 'c30ceda9-56d1-4011-851a-cfa7031796d0'
  AND ne.data_emissao >= to_date('04/2026','MM/YYYY')
  AND ne.data_emissao <  (to_date('04/2026','MM/YYYY') + interval '1 month')
  AND ne.forn_nome ILIKE '%rolimec%'
ORDER BY ne.numero_nfe, nii.n_item;


-- ----------------------------------------------------------------------------
-- (4) PRÉVIA DO RESULTADO — Bloco A/B: o retido vai aparecer após o reimport?
--     Reproduz o COALESCE(NULLIF(xi.v_st,0), ci.vl_icms_st, 0) do relatório,
--     casando SPED x XML por (chave, item). Se 'v_st_xml' vier preenchido e
--     'casou_item' = sim, o retido aparecerá no demonstrativo.
--     ATENÇÃO: a junção é por n_item = num_item; se o reimport não preencher o
--     n_item igual ao do SPED, o retido NÃO casa mesmo com v_st > 0.
-- ----------------------------------------------------------------------------
SELECT c100.num_doc                                          AS numero_nfe,
       ci.num_item,
       ci.cfop,
       ci.vl_icms_st                                         AS retido_sped,
       xi.v_st                                               AS v_st_xml,
       COALESCE(NULLIF(xi.v_st,0), ci.vl_icms_st, 0)         AS retido_final,
       CASE WHEN xi.nfe_id IS NULL THEN 'nao (sem XML/n_item)'
            ELSE 'sim' END                                   AS casou_item
FROM reg_c170 ci
JOIN reg_c100 c100 ON c100.id = ci.c100_id
JOIN import_jobs j ON j.id = c100.job_id
LEFT JOIN nfe_entradas ne ON ne.company_id = j.company_id AND ne.chave_nfe = c100.chv_nfe
LEFT JOIN nfe_entradas_itens xi ON xi.nfe_id = ne.id AND xi.n_item = ci.num_item
WHERE j.company_id = 'c30ceda9-56d1-4011-851a-cfa7031796d0'
  AND ci.cfop IN ('2403','2409','2651','2652')
  AND c100.cod_sit NOT IN ('02','03','04','05')
  AND (j.mes_ano = '04/2026'
       OR (j.mes_ano IS NULL
           AND EXTRACT(MONTH FROM j.dt_ini)::int = SPLIT_PART('04/2026','/',1)::int
           AND EXTRACT(YEAR  FROM j.dt_ini)::int = SPLIT_PART('04/2026','/',2)::int))
ORDER BY c100.num_doc, ci.num_item;
