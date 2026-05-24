-- 102_create_legislacao_fronteira.sql
-- Etapa 5: Importação de Legislação (decretos, portarias, RICMS) com IA.
--
-- Fluxo: upload (PDF/texto) → extrai texto → IA gera interpretação estruturada
-- (regras propostas em JSON) → usuário revisa item-a-item → aplica nas
-- icms_fronteira_regras_ncm. Mantém auditoria com referência ao trecho.

CREATE TABLE IF NOT EXISTS legislacao_fronteira (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id        UUID         NOT NULL,
    uf_estado         VARCHAR(2)   NOT NULL,
    titulo            TEXT         NOT NULL,
    fonte             VARCHAR(40),                 -- ex: 'decreto', 'portaria', 'ricms'
    referencia        VARCHAR(120),                -- ex: 'Decreto BA 13.870/2012'
    conteudo_texto    TEXT,                        -- texto integral extraído (PDF→txt)
    interpretacao     JSONB        NOT NULL DEFAULT '{}'::jsonb,
                       -- shape: { "resumo": "...", "regras": [
                       --    { "ncm": "8482", "regime": "ANTECIPACAO",
                       --      "justificativa": "8482 NÃO consta na lista ST do decreto" } ] }
    status            VARCHAR(20)  NOT NULL DEFAULT 'pending'
                      CHECK (status IN ('pending','reviewed','applied','discarded')),
    uploaded_by       UUID,
    applied_by        UUID,
    applied_at        TIMESTAMPTZ,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_legislacao_company_uf
    ON legislacao_fronteira(company_id, uf_estado);

COMMENT ON TABLE legislacao_fronteira IS
    'Legislação tributária importada (decretos, RICMS, portarias). A IA gera uma '
    'interpretação estruturada em JSONB; o usuário valida cada regra antes de '
    'aplicá-la nas icms_fronteira_regras_ncm.';

COMMENT ON COLUMN legislacao_fronteira.interpretacao IS
    'JSONB com {"resumo": str, "regras": [{ncm,regime,mva,aliquota_int,justificativa,'
    'confirmado:bool}]}. confirmado controla quais regras vão ser aplicadas.';

COMMENT ON COLUMN legislacao_fronteira.status IS
    'pending=interpretação gerada, aguardando revisão; reviewed=usuário revisou; '
    'applied=regras aplicadas em icms_fronteira_regras_ncm; discarded=descartado.';
