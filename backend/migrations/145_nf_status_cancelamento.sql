-- 145_nf_status_cancelamento.sql
--
-- Adiciona coluna status (ATIVO | CANCELADO) em nfe_entradas e reg_c100.
-- Cancelamento é lógico: a NF continua visível nas telas e relatórios,
-- mas seus valores NÃO são somados nos totais. Apenas informativo.
-- A ação é realizada pelo usuário no módulo Administrativo > Cancelamentos.

ALTER TABLE nfe_entradas
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'ATIVO';

ALTER TABLE reg_c100
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'ATIVO';

-- Índices parciais: apenas as linhas canceladas (minoria) são indexadas.
CREATE INDEX IF NOT EXISTS idx_nfe_entradas_canceladas
    ON nfe_entradas(company_id) WHERE status = 'CANCELADO';

CREATE INDEX IF NOT EXISTS idx_reg_c100_canceladas
    ON reg_c100(status) WHERE status = 'CANCELADO';
