-- 076_create_xml_upload_batches.sql
-- Cria tabela xml_upload_batches para histórico e controle de uploads de XML (per D-13).
--
-- Cada registro representa um lote de upload: pode ser síncrono (<= 50 XMLs)
-- ou assíncrono (> 50 XMLs, com xml_data armazenando os bytes comprimidos).
--
-- O campo status permite ao worker assíncrono consultar batches pendentes
-- via partial index, sem varrer toda a tabela.
--
-- Idempotente via CREATE TABLE IF NOT EXISTS.

CREATE TABLE IF NOT EXISTS xml_upload_batches (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id          UUID        NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    uploaded_by         UUID        REFERENCES users(id) ON DELETE SET NULL,

    -- Tipo de documento do lote
    tipo                TEXT        NOT NULL,   -- 'entradas' | 'saidas' | 'ctes'

    -- Metadados do arquivo original
    filename            TEXT,                   -- nome do arquivo ZIP ou XML enviado

    -- Contadores de progresso
    total_count         INT         NOT NULL DEFAULT 0,
    processed_count     INT         NOT NULL DEFAULT 0,
    imported_count      INT         NOT NULL DEFAULT 0,
    rejected_count      INT         NOT NULL DEFAULT 0,

    -- Estado do batch
    status              TEXT        NOT NULL DEFAULT 'pending',

    -- Detalhes de erros por XML rejeitado (JSONB para flexibilidade)
    error_details       JSONB,

    -- Bytes dos XMLs comprimidos para processamento assíncrono (>50 XMLs)
    -- NULL para batches síncronos (já processados no momento do upload)
    xml_data            BYTEA,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at        TIMESTAMPTZ,

    CONSTRAINT chk_xml_upload_batches_tipo
        CHECK (tipo IN ('entradas', 'saidas', 'ctes')),

    CONSTRAINT chk_xml_upload_batches_status
        CHECK (status IN ('pending', 'processing', 'done', 'failed'))
);

-- Índice para histórico de uploads por empresa (mais recentes primeiro)
CREATE INDEX IF NOT EXISTS idx_xml_upload_batches_company_created
    ON xml_upload_batches(company_id, created_at DESC);

-- Partial index para o worker assíncrono (consulta apenas batches ativos)
CREATE INDEX IF NOT EXISTS idx_xml_upload_batches_status_active
    ON xml_upload_batches(status)
    WHERE status IN ('pending', 'processing');
