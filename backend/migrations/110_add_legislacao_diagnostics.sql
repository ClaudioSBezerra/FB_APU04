-- 110_add_legislacao_diagnostics.sql
-- Etapa 5 (cont.): diagnóstico da importação de legislação.
--
-- Após resolver "texto reduzido de 156117 para 0 chars" trocando a extração
-- de PDF, queremos persistir o que foi efetivamente enviado para a IA e a
-- resposta crua que ela devolveu, para permitir reprocessamento sem upload
-- e diagnóstico offline quando a interpretação vier vazia ou incompleta.

ALTER TABLE legislacao_fronteira
    ADD COLUMN IF NOT EXISTS texto_ia        TEXT,
    ADD COLUMN IF NOT EXISTS resposta_ia_raw TEXT,
    ADD COLUMN IF NOT EXISTS ia_model        VARCHAR(50),
    ADD COLUMN IF NOT EXISTS chunks_count    INTEGER NOT NULL DEFAULT 1;

COMMENT ON COLUMN legislacao_fronteira.texto_ia IS
    'Texto efetivamente enviado para a IA após extração do PDF e filtragem '
    '(linhas com NCM/MVA/%). Pode ser a concatenação de múltiplos chunks se '
    'o texto foi dividido para caber no contexto.';

COMMENT ON COLUMN legislacao_fronteira.resposta_ia_raw IS
    'Resposta crua da IA (content + reasoning_content concatenados quando '
    'houver) antes do parseLegislacaoJSON. Útil para diagnóstico offline '
    'quando a interpretação vier vazia.';

COMMENT ON COLUMN legislacao_fronteira.ia_model IS
    'Modelo IA utilizado (ex: glm-4.7-flash, glm-4.5-flash). Permite '
    'rastrear se um fallback foi acionado por rate limit.';

COMMENT ON COLUMN legislacao_fronteira.chunks_count IS
    'Quantidade de chamadas à IA. 1 = texto coube em uma chamada. >1 = '
    'texto longo foi processado em chunks e os resultados mesclados.';
