-- Migration 087: Cria tabela reforma_parametros (RFMA-02)
--
-- Motivação: Centraliza os parâmetros configuráveis por empresa para os módulos
-- de análise da Reforma Tributária (Phases 7 e 8). Sem esta tabela não há como
-- personalizar alíquotas IBS/CBS por empresa, aplicar o fator estimado do Simples
-- Nacional ou configurar parâmetros de capital de giro (CDI, prazo médio).
--
-- Design: PK = company_id (one-to-one com companies). ON DELETE CASCADE garante
-- que a deleção de uma empresa remove automaticamente seus parâmetros.
--
-- Nota sobre fator_simples_pct DEFAULT 20.00: valor estimado pendente publicação
-- pelo CG-IBS. UI deve exibir disclaimer obrigatório sobre esta incerteza (RFMA-07).

CREATE TABLE IF NOT EXISTS reforma_parametros (
    company_id         UUID PRIMARY KEY REFERENCES companies(id) ON DELETE CASCADE,
    target_ano         INTEGER          NOT NULL DEFAULT 2027,
    aliq_ibs_pct       NUMERIC(5,2),
    aliq_cbs_pct       NUMERIC(5,2),
    fator_simples_pct  NUMERIC(5,2)     DEFAULT 20.00,
    taxa_cdi_anual_pct NUMERIC(5,2),
    prazo_medio_dias   INTEGER,
    created_at         TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at         TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
