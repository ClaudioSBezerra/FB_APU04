-- 113_create_cest_ncm_ref.sql
-- Base de Conhecimento CEST→NCM auto-construída.
--
-- O CEST (Código Especificador da Substituição Tributária) liga cada produto
-- ST a um segmento (2 primeiros dígitos) do Convênio ICMS 52/2017. Esse
-- mapeamento CEST→NCM já chega ao sistema a cada importação — vem do SPED
-- (reg_0200.cest + cod_ncm) e do XML (nfe_entradas_itens.cest + ncm).
--
-- Em vez de baixar a tabela CEST nacional do CONFAZ (PDF/HTML frágil), esta
-- KB se constrói sozinha dos dados que o cliente importa, cobrindo exatamente
-- os NCMs que ele realmente movimenta. Serve para:
--   1. consultar "que NCMs tenho no segmento X" (CEST prefixo);
--   2. expandir decretos que remetem a anexo externo (segmento → CESTs → NCMs);
--   3. validação cruzada com o CEST↔NCM que vem inline nos decretos.

CREATE TABLE IF NOT EXISTS cest_ncm_ref (
    id          BIGSERIAL    PRIMARY KEY,
    company_id  UUID         NOT NULL,
    cest        VARCHAR(9)   NOT NULL,   -- ex: '0104900' (com ou sem pontuação normalizada)
    ncm         VARCHAR(8)   NOT NULL,
    descricao   TEXT,
    fonte       VARCHAR(12)  NOT NULL DEFAULT 'sped',  -- 'sped' (0200) | 'xml'
    ocorrencias INTEGER      NOT NULL DEFAULT 1,        -- quantas vezes visto (confiança)
    first_seen  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    last_seen   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT uq_cest_ncm_ref UNIQUE (company_id, cest, ncm)
);

-- Consulta por segmento (prefixo do CEST) é o caso de uso principal.
CREATE INDEX IF NOT EXISTS idx_cest_ncm_ref_company_cest
    ON cest_ncm_ref(company_id, cest);
-- Consulta reversa NCM→CEST (validação cruzada com decreto).
CREATE INDEX IF NOT EXISTS idx_cest_ncm_ref_company_ncm
    ON cest_ncm_ref(company_id, ncm);

COMMENT ON TABLE cest_ncm_ref IS
    'Base de conhecimento CEST→NCM auto-construída a partir dos dados importados '
    '(reg_0200 + nfe_entradas_itens). Alimentada por refreshCestNcmKB após cada '
    'importação. ocorrencias indica confiança/frequência do par.';
