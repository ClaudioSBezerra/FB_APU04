-- DIFAL no cabeçalho (<ICMSTot><vICMSUFDest>/<vFCPUFDest>) — alimenta os
-- filtros "DIFAL > 0" da busca de notas do Teste Pacote Fiscal.
-- Notas importadas antes desta migration ficam com 0; reimportar atualiza.
ALTER TABLE pacotefiscal_nfe_saidas
    ADD COLUMN IF NOT EXISTS v_icms_uf_dest NUMERIC(15,2) DEFAULT 0,
    ADD COLUMN IF NOT EXISTS v_fcp_uf_dest  NUMERIC(15,2) DEFAULT 0;
