-- 165_pacotefiscal_sim2027_parametros.sql
--
-- Parâmetros GLOBAIS (não por empresa) da 4ª funcionalidade do módulo "Teste
-- Pacote Fiscal": "Simulação 2027". Pedido do usuário (2026-10-06): simular um
-- XML de NF-e (modelo 55) ou CT-e já emitido como se já estivesse sob as
-- regras plenas da Reforma Tributária (PIS/COFINS/IPI extintos, IBS/CBS
-- cheios), usando alíquotas de teste editáveis.
--
-- Não reaproveita reforma_parametros (migration 087): aquela tabela é por
-- empresa e deriva aliq_ibs/aliq_cbs de tabela_aliquotas (alíquotas oficiais
-- da transição, ~0,9% de CBS em 2026/2027) — serve a um módulo de simulação
-- financeira diferente. Aqui o usuário quer um conjunto único e global de
-- alíquotas de teste (default CBS 9,90%, bem acima do oficial da transição),
-- igual para qualquer empresa/execução.
--
-- Singleton (id fixo = 1, via CHECK) — não há seletor de empresa nesta tela.
CREATE TABLE IF NOT EXISTS pacotefiscal_sim2027_parametros (
    id               SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    aliq_ibs_uf_pct  NUMERIC(7,4) NOT NULL DEFAULT 0.05,
    aliq_ibs_mun_pct NUMERIC(7,4) NOT NULL DEFAULT 0.05,
    aliq_cbs_pct     NUMERIC(7,4) NOT NULL DEFAULT 9.90,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by       UUID REFERENCES users(id)
);

INSERT INTO pacotefiscal_sim2027_parametros (id)
VALUES (1)
ON CONFLICT (id) DO NOTHING;
