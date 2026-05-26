-- 120_company_segmentos.sql
-- Segmentos cadastrados para cada empresa por UF.
-- Uma empresa pode operar em múltiplos segmentos de ST.
-- O motor de cálculo só aplica ST quando o segmento_codigo da regra NCM
-- constar nesta tabela para a empresa e UF da filial.

CREATE TABLE IF NOT EXISTS company_segmentos (
    id              UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    company_id      UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    segmento_codigo INT  NOT NULL,
    uf              VARCHAR(2) NOT NULL,
    created_at      TIMESTAMP DEFAULT now(),
    UNIQUE (company_id, segmento_codigo, uf)
);

CREATE INDEX IF NOT EXISTS idx_company_segmentos_company_uf
    ON company_segmentos (company_id, uf);
