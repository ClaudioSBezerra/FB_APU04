-- Adiciona o tomador do serviço de transporte ao CT-e.
-- toma = 0 (Remetente) | 1 (Expedidor) | 2 (Recebedor) | 3 (Destinatário) | 4 (Outros)
-- Para o cálculo de ICMS fronteira sobre frete, apenas CT-es com toma='3'
-- (destinatário = nossa empresa) ou toma='4' apontando para o CNPJ da empresa
-- devem ser considerados — pois é quando o frete é por conta do destinatário.

ALTER TABLE cte_entradas
    ADD COLUMN IF NOT EXISTS toma       VARCHAR(1),
    ADD COLUMN IF NOT EXISTS toma4_cnpj VARCHAR(14);

CREATE INDEX IF NOT EXISTS idx_cte_entradas_toma
    ON cte_entradas(company_id, toma) WHERE toma IS NOT NULL;
