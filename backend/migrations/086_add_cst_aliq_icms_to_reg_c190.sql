-- Migration 086: Adiciona cst_icms e aliq_icms em reg_c190 (RFMA-01)
--
-- Motivação: O módulo de análise de créditos ICMS da Reforma Tributária (Phase 7)
-- precisa de CST ICMS e alíquota efetiva por registro C190 do SPED para calcular
-- o aproveitamento de créditos IBS/CBS. Sem essas colunas não há como segmentar
-- operações por CST nem derivar base de crédito ICMS.
--
-- Colunas nullable por D-09: registros históricos importados antes desta migration
-- ficam com NULL (retroalimentação via reimport é opcional).

ALTER TABLE reg_c190
    ADD COLUMN IF NOT EXISTS cst_icms VARCHAR(3),
    ADD COLUMN IF NOT EXISTS aliq_icms NUMERIC(6,2);
