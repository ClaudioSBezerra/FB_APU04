-- Adiciona coluna `periodo` (MM/YYYY) em fiscal_calculations.
-- Substitui o filtro por data_emissao (que filtrava pela data da NF, não pelo
-- período de escrituração). Necessária porque NFs de meses anteriores
-- aparecem no SPED do período corrente — o usuário pesquisa pelo período
-- escriturado, não pelo mês de emissão.

ALTER TABLE fiscal_calculations
    ADD COLUMN IF NOT EXISTS periodo VARCHAR(7);

CREATE INDEX IF NOT EXISTS idx_fiscal_calc_periodo
    ON fiscal_calculations(company_id, periodo);
