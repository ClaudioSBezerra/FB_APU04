-- 146_nfe_itens_desc_outro.sql
--
-- Desconto (vDesc) e despesas acessórias (vOutro) por ITEM da NF-e. Necessário
-- como pDesconto/pDespesas — dois dos 23 parâmetros IN do pacote fiscal Oracle
-- (PKG_FISCAL_FCTAX.calcula_imposto_produto, Fase 11). Sem estas colunas, o
-- motor de execução da Fase 11 não teria de onde ler esses dois inputs a
-- partir de nfe_saidas_itens.
--
-- Ambas as tabelas de itens recebem as colunas: nfe_saidas_itens é o
-- requisito direto (TPF-02), e nfe_entradas_itens é obrigatório porque
-- insertNFeItens (nfe_saidas.go) é COMPARTILHADO entre as duas tabelas — o
-- mesmo texto de INSERT roda para ambas, então sem as colunas em
-- nfe_entradas_itens o INSERT de entradas quebraria em runtime. Também
-- mantém a simetria já estabelecida pelas migrations 094/095/141.

ALTER TABLE nfe_saidas_itens
    ADD COLUMN IF NOT EXISTS v_desc  NUMERIC(15,2) DEFAULT 0,
    ADD COLUMN IF NOT EXISTS v_outro NUMERIC(15,2) DEFAULT 0;

ALTER TABLE nfe_entradas_itens
    ADD COLUMN IF NOT EXISTS v_desc  NUMERIC(15,2) DEFAULT 0,
    ADD COLUMN IF NOT EXISTS v_outro NUMERIC(15,2) DEFAULT 0;
