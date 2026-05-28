-- 125_fronteira_performance_indexes.sql
--
-- Índices de performance para o fronteiraBaseQuery e naoSpedQuery.
-- Sem esses índices o PostgreSQL faz seq-scan em nfe_entradas e reg_c170
-- para cada nota do SPED, tornando o relatório lento em volumes maiores.

-- ── nfe_entradas: lookup por chave_nfe no LEFT JOIN do fronteiraBaseQuery ─────
-- fonte CTE: LEFT JOIN nfe_entradas ne ON ne.company_id = jb.company_id
--                                      AND ne.chave_nfe = c100b.chv_nfe
CREATE INDEX IF NOT EXISTS idx_nfe_entradas_company_chave
    ON nfe_entradas(company_id, chave_nfe)
    WHERE chave_nfe IS NOT NULL;

-- ── reg_c170: filtro cfop + join por c100_id na fonte CTE ────────────────────
-- fonte CTE: FROM reg_c170 WHERE cfop = ANY(ARRAY['2101','2102',...])
-- O índice simples idx_reg_c170_c100 existe, mas o composto evita o
-- re-filtro de cfop após o join, reduzindo linhas intermediárias.
CREATE INDEX IF NOT EXISTS idx_reg_c170_c100_cfop
    ON reg_c170(c100_id, cfop);

-- ── prodepe_enquadramentos: EXISTS subquery por CNPJ ─────────────────────────
-- WHEN EXISTS (SELECT 1 FROM prodepe_enquadramentos pe
--              WHERE pe.company_id=$1 AND pe.ativo=true AND pe.dispensa_antecipacao=true ...)
CREATE INDEX IF NOT EXISTS idx_prodepe_company_ativo_cnpj
    ON prodepe_enquadramentos(company_id, ativo, dispensa_antecipacao, cnpj);

