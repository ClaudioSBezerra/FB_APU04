-- 123_add_base_por_dentro_uf.sql
-- Parâmetro por UF: cálculo da base de antecipação/DIFAL "por dentro".
--
-- Em Pernambuco, a base da antecipação (e do DIFAL de uso/consumo/ativo) é
-- calculada excluindo o ICMS interestadual destacado e reincorporando o ICMS
-- pela alíquota interna (gross-up): base = (operação − ICMS_destacado) /
-- (1 − alíq_interna). Bahia e Ceará NÃO fazem esse processo. Por isso o
-- comportamento é parametrizado por UF, configurável no módulo administrativo
-- (aba UFs → benefícios), em vez de hardcoded.
--
-- ST não usa este processo (base da ST é o valor da operação puro).

ALTER TABLE uf_beneficios_fiscais
    ADD COLUMN IF NOT EXISTS base_por_dentro BOOLEAN NOT NULL DEFAULT false;

COMMENT ON COLUMN uf_beneficios_fiscais.base_por_dentro IS
    'Quando true, a base de antecipação/DIFAL é calculada por dentro (gross-up): '
    'base = (operação − ICMS destacado) / (1 − alíq_interna). PE = true; BA/CE = false.';

-- Linhas já existentes de PE passam a usar o cálculo por dentro (regra do estado).
UPDATE uf_beneficios_fiscais SET base_por_dentro = true WHERE uf = 'PE';
