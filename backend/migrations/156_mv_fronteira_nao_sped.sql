-- 156_mv_fronteira_nao_sped.sql
--
-- Materializa a parte PESADA e ESTÁVEL do Bloco C (não-SPED / XML) do ICMS
-- Fronteira. Antes, a naoSpedQuery lia o dado BRUTO a cada carregamento:
-- varria nfe_entradas (anti-join com reg_c100), agrupava 100k+ itens por
-- (NF, CFOP, NCM), rateava frete/outro/ICMS do cabeçalho e resolvia a UF
-- efetiva — tudo em tempo real. Na Ferreira Costa (100k+ notas XML/mês) isso
-- levava minutos e estourava a shared memory do container (64MB).
--
-- O que fica FORA da MV (aplicado em tempo real na naoSpedQuery, pois o
-- usuário edita e precisa refletir na hora — e são lookups leves/indexados):
--   - nao_sped_cfop_override (CFOP corrigido pelo usuário → recalcula regime)
--   - icms_fronteira_regras_ncm (alíquota interna / MVA / segmento)
--   - company_segmentos (decide ST × ANTECIPACAO)
--   - prodepe_enquadramentos (dispensa)
--   - uf_beneficios_fiscais (base por dentro)
--   - icms_fronteira_classificacao_manual (regime/status manual)
--   - cte_por_nfe (frete CT-e; tabela pequena e indexada)
--
-- Ciclo de vida = mesmo da mv_icms_fronteira_linhas: refrescada pelo botão
-- "Recalcular" (POST /api/icms-fronteira/recalcular) após importar XML/SPED.
-- Chave única por (nfe_id, cfop_xml, ncm) — granularidade do items_grouped.

DROP MATERIALIZED VIEW IF EXISTS mv_fronteira_nao_sped;

CREATE MATERIALIZED VIEW mv_fronteira_nao_sped AS
WITH emp_uf AS (
    -- UF dominante por empresa (fallback 3 do eff_uf)
    SELECT company_id,
           MAX(uf) FILTER (WHERE uf IS NOT NULL AND uf <> '') AS uf
    FROM import_jobs
    GROUP BY company_id
), xml_falt AS (
    -- NF-e de entrada presentes no XML mas AUSENTES de qualquer SPED da empresa
    SELECT
        ne.company_id, ne.mes_ano,
        ne.id, ne.chave_nfe, ne.data_emissao, ne.forn_cnpj, ne.forn_nome,
        ne.forn_uf, ne.dest_uf, ne.dest_cnpj_cpf, COALESCE(ne.numero_nfe,'') AS numero_nfe,
        COALESCE(ne.v_prod,0) AS v_prod, COALESCE(ne.v_frete,0) AS v_frete,
        COALESCE(ne.v_outro,0) AS v_outro,
        COALESCE(ne.v_ipi,0)  AS v_ipi,
        COALESCE(ne.v_icms,0) AS v_icms,
        COALESCE(ne.status, 'ATIVO') AS nf_status
    FROM nfe_entradas ne
    WHERE NOT EXISTS (
        SELECT 1 FROM reg_c100 c100 JOIN import_jobs j ON j.id = c100.job_id
        WHERE j.company_id = ne.company_id AND c100.chv_nfe = ne.chave_nfe
    )
), items_grouped AS (
    -- Agrupa por (NF, CFOP, NCM): cada combinação distinta vira linha própria
    SELECT nii.nfe_id,
           COALESCE(nii.cfop,'') AS cfop_xml,
           COALESCE(nii.ncm,'')  AS ncm,
           SUM(COALESCE(nii.v_prod, 0)) AS item_sum,
           SUM(COALESCE(nii.v_ipi,  0)) AS item_ipi
    FROM nfe_entradas_itens nii
    JOIN xml_falt xf ON xf.id = nii.nfe_id
    GROUP BY nii.nfe_id, nii.cfop, nii.ncm
), nf_total AS (
    SELECT nfe_id, SUM(item_sum) AS total_sum
    FROM items_grouped
    GROUP BY nfe_id
)
SELECT
    xf.company_id,
    xf.mes_ano,
    xf.id            AS nfe_id,
    xf.chave_nfe,
    xf.data_emissao,
    xf.numero_nfe,
    xf.forn_cnpj, xf.forn_nome, xf.forn_uf,
    xf.dest_uf, xf.dest_cnpj_cpf,
    ig.cfop_xml,
    ig.ncm,
    ig.item_sum AS v_prod,
    CASE WHEN nt.total_sum > 0 THEN xf.v_frete * ig.item_sum / nt.total_sum ELSE 0 END AS v_frete,
    CASE WHEN nt.total_sum > 0 THEN xf.v_outro * ig.item_sum / nt.total_sum ELSE 0 END AS v_outro,
    ig.item_ipi AS v_ipi,
    CASE WHEN nt.total_sum > 0 THEN xf.v_icms  * ig.item_sum / nt.total_sum ELSE 0 END AS v_icms,
    CASE WHEN nt.total_sum > 0 THEN ig.item_sum / nt.total_sum              ELSE 1 END AS item_ratio,
    xf.nf_status,
    -- eff_uf: 1) dest_uf do XML; 2) UF da filial pelo CNPJ destino
    -- (import_jobs.cnpj); 3) UF dominante da empresa; 4) 'PE' (legado).
    COALESCE(
        NULLIF(xf.dest_uf, ''),
        (SELECT j.uf
         FROM import_jobs j
         WHERE j.company_id = xf.company_id
           AND j.status = 'completed'
           AND j.uf IS NOT NULL AND j.uf <> ''
           AND regexp_replace(COALESCE(j.cnpj,''), '[^0-9]', '', 'g')
               = regexp_replace(COALESCE(xf.dest_cnpj_cpf,''), '[^0-9]', '', 'g')
         LIMIT 1),
        eu.uf,
        'PE'
    ) AS eff_uf
FROM xml_falt xf
JOIN items_grouped ig ON ig.nfe_id = xf.id
JOIN nf_total      nt ON nt.nfe_id = xf.id
LEFT JOIN emp_uf   eu ON eu.company_id = xf.company_id
WITH NO DATA;

-- Chave única (granularidade do items_grouped) — habilita REFRESH CONCURRENTLY
CREATE UNIQUE INDEX idx_mv_nao_sped_key
    ON mv_fronteira_nao_sped(nfe_id, cfop_xml, ncm);

-- Caminho de leitura da naoSpedQuery: empresa + mês
CREATE INDEX idx_mv_nao_sped_company_mes
    ON mv_fronteira_nao_sped(company_id, mes_ano);

REFRESH MATERIALIZED VIEW mv_fronteira_nao_sped;
