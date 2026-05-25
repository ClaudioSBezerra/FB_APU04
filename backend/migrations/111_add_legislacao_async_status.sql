-- 111_add_legislacao_async_status.sql
-- Etapa 5 (cont.): processamento assíncrono da interpretação de legislação.
--
-- O free-tier do Z.AI rate-limita pesado; processar um decreto inteiro em
-- múltiplos chunks de forma síncrona estourava o timeout do frontend e
-- falhava chunks no meio. Agora o upload retorna na hora e uma goroutine
-- processa os chunks em background, salvando regras incrementalmente.
-- Estas colunas rastreiam o ciclo de processamento para o frontend pollar.

ALTER TABLE legislacao_fronteira
    ADD COLUMN IF NOT EXISTS proc_status       VARCHAR(20) NOT NULL DEFAULT 'done',
    ADD COLUMN IF NOT EXISTS proc_done_chunks  INTEGER     NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS proc_total_chunks INTEGER     NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS proc_error        TEXT;

-- Linhas antigas (já interpretadas de forma síncrona) ficam 'done'.
-- Novas começam 'processing' e o worker as move para 'done'/'error'.

COMMENT ON COLUMN legislacao_fronteira.proc_status IS
    'Ciclo de processamento em background: processing | done | error. '
    'Default done para retrocompat com linhas síncronas antigas.';

COMMENT ON COLUMN legislacao_fronteira.proc_done_chunks IS
    'Chunks já processados pela IA. Permite barra de progresso no frontend.';

COMMENT ON COLUMN legislacao_fronteira.proc_total_chunks IS
    'Total de chunks a processar (texto filtrado / chunkLimit).';

COMMENT ON COLUMN legislacao_fronteira.proc_error IS
    'Mensagem de erro quando proc_status=error (todos os chunks falharam).';
