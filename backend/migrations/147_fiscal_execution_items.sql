-- 147_fiscal_execution_items.sql
-- Resultado calculado pelo pacote fiscal (PKG_FISCAL_FCTAX.calcula_imposto_produto),
-- um registro por item de nfe_saidas_itens (TPF-04). Modelo híbrido: colunas
-- dedicadas para os campos usados na comparação visual da Fase 12 (ICMS,
-- ICMS-ST, PIS, COFINS, DIFAL, FCP) + full_result JSONB para o retorno
-- completo (~88 campos, incluindo o bloco da Reforma Tributária).
-- Porte do modelo validado em FB_TESTESFC (008_fiscal_execution_items.sql),
-- acrescido das colunas IBS/CBS que a Fase 12 vai precisar (11-RESEARCH.md
-- Pattern 3).
--
-- Status distinto de erro:
--   sem_grupo_fiscal = lookup prod/PRODB não encontrou o produto (TPF-01)
--   error            = falha na chamada do pacote fiscal em si (TPF-03)
--
-- Idempotente via CREATE TABLE IF NOT EXISTS / CREATE INDEX IF NOT EXISTS.
-- Nota: fiscal_calculations é uma tabela não relacionada (módulo ICMS
-- Fronteira) — sem colisão de nome.

CREATE TABLE IF NOT EXISTS fiscal_execution_items (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id          UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    nfe_item_id         UUID NOT NULL REFERENCES nfe_saidas_itens(id) ON DELETE CASCADE,

    -- Status de execução (isolamento de erro por item — TPF-05)
    status              TEXT NOT NULL DEFAULT 'pending', -- pending | ok | error | sem_grupo_fiscal
    error_message       TEXT,
    executed_at         TIMESTAMPTZ,

    -- Parâmetros de entrada efetivamente usados (auditoria — o que foi enviado ao pacote)
    grupo_fiscal_codigo TEXT,             -- pCodigoGrupoFiscal resolvido via prod/PRODB
    input_params        JSONB,            -- snapshot dos 23 parâmetros de entrada enviados

    -- Campos usados na comparação visual da Fase 12 (colunas dedicadas — acesso rápido/indexável)
    base_calculo_icms           NUMERIC(15,2),  -- result.BaseCalculo
    valor_icms                  NUMERIC(15,2),  -- result.ValorImposto (quando TipoImposto = ICMS)
    base_substituicao           NUMERIC(15,2),  -- result.BaseSubstituicao
    valor_substituicao          NUMERIC(15,2),  -- result.ValorSubstituicao
    base_calculo_pis            NUMERIC(15,2),  -- result.BaseCalculoPIS
    valor_pis                   NUMERIC(15,2),  -- result.ValorPIS
    base_calculo_cofins         NUMERIC(15,2),  -- result.BaseCalculoCOFINS
    valor_cofins                NUMERIC(15,2),  -- result.ValorCOFINS
    percentual_difal            NUMERIC(7,4),   -- result.PercentualDifal
    valor_icms_partilha_destino NUMERIC(15,2),  -- result.ValorIcmsPartilhaDestino (DIFAL)
    valor_icms_pobreza          NUMERIC(15,2),  -- result.ValorIcmsPobreza (FCP)

    -- Colunas adicionais (além do modelo original FB_TESTESFC) para IBS/CBS
    -- da Reforma Tributária, necessárias à Fase 12 (TPF-06)
    valor_ibs_uf         NUMERIC(15,2), -- result.ValorIbsUF
    valor_ibs_mun        NUMERIC(15,2), -- result.ValorIbsMUN
    valor_cbs            NUMERIC(15,2), -- result.ValorCbs

    -- Retorno completo (~88 campos) para auditoria/depuração
    full_result JSONB NOT NULL,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_fiscal_execution_item UNIQUE (nfe_item_id)
);

CREATE INDEX IF NOT EXISTS idx_fiscal_execution_status ON fiscal_execution_items(company_id, status);
CREATE INDEX IF NOT EXISTS idx_fiscal_execution_nfe_item ON fiscal_execution_items(nfe_item_id);
