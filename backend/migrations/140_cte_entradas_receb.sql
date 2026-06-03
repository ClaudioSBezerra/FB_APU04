-- 140_cte_entradas_receb.sql
--
-- Recebedor do CT-e (<receb>): o estabelecimento que RECEBE a carga no
-- redespacho — tipicamente a transportadora do trecho seguinte. É a pista que
-- liga uma NF ao frete final ainda não emitido: quando um CT-e tem
-- receb=Transportadora T mas não existe CT-e emitido por T para a mesma NF,
-- o frete final de T está PENDENTE (ver relatório de fretes pendentes).
--   receb_cnpj_cpf — CNPJ/CPF do recebedor (14/11 díg., sem máscara)
--   receb_nome     — razão social do recebedor
-- NULL = CT-e sem <receb> ou importado antes desta coluna (ver backfill).

ALTER TABLE cte_entradas
    ADD COLUMN IF NOT EXISTS receb_cnpj_cpf VARCHAR(14),
    ADD COLUMN IF NOT EXISTS receb_nome     VARCHAR(255);
