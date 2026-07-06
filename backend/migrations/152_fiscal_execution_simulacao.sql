-- Simulação "IBS/CBS na base do ICMS" (fase BOA, 2026-07): quando a execução
-- roda com incluir_ibs_cbs_base=true, o item ganha uma 2ª chamada ao pacote
-- com pPrecoTotal = original + IBS + CBS, e este JSONB guarda a comparação
-- simulado interno × pacote (ICMS, ICMS-ST, FCP, DIFAL + fator/acréscimo).
-- NULL = execução sem simulação.
ALTER TABLE fiscal_execution_items
    ADD COLUMN IF NOT EXISTS simulacao JSONB;
