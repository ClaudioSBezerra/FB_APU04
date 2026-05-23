-- 097_add_uf_estado_to_fronteira_regras.sql
-- Expande icms_fronteira_regras_ncm com uf_estado + MVA ajustado (4/7/12%),
-- recria a constraint UNIQUE incluindo uf_estado, e cria a tabela
-- icms_fronteira_inaplicabilidades (per CADU-04).
--
-- CRÍTICO: DROP da constraint uq_icms_fronteira_regras é obrigatório antes de
-- adicionar uf_estado, senão regras para BA/CE com mesma NCM colidem com regras
-- PE existentes (Pitfall 1 do RESEARCH). A constraint nova (uq_icms_fronteira_regras_uf)
-- inclui uf_estado para permitir a mesma NCM em UFs distintas.
--
-- ATENÇÃO: Requires PostgreSQL >= 15 for UNIQUE NULLS NOT DISTINCT (Bloco 3).
-- Em PostgreSQL 14 ou anterior, a sintaxe NULLS NOT DISTINCT não é suportada
-- e a migration falhará. Verifique a versão do banco antes de executar:
--   SELECT version();
--
-- Idempotente: ADD COLUMN IF NOT EXISTS, DROP CONSTRAINT IF EXISTS,
-- DO block com EXCEPTION WHEN duplicate_object, CREATE TABLE/INDEX IF NOT EXISTS.

-- Bloco 1: adicionar coluna uf_estado com DEFAULT 'PE' para preservar registros
-- existentes da migration 091 (todos referentes a Pernambuco)
ALTER TABLE icms_fronteira_regras_ncm
    ADD COLUMN IF NOT EXISTS uf_estado VARCHAR(2) NOT NULL DEFAULT 'PE';

-- Bloco 2: adicionar 3 colunas de MVA ajustado pré-calculado (nullable)
-- Nullable — cálculo dinâmico via fórmula Convênio ICMS 110/07 é fallback quando NULL
ALTER TABLE icms_fronteira_regras_ncm
    ADD COLUMN IF NOT EXISTS mva_ajustado_4pct  NUMERIC(8,4);
ALTER TABLE icms_fronteira_regras_ncm
    ADD COLUMN IF NOT EXISTS mva_ajustado_7pct  NUMERIC(8,4);
ALTER TABLE icms_fronteira_regras_ncm
    ADD COLUMN IF NOT EXISTS mva_ajustado_12pct NUMERIC(8,4);

-- Bloco 3: recriar constraint UNIQUE incluindo uf_estado
-- DROP sem IF EXISTS: o nome exato está na migration 091 linha 16
ALTER TABLE icms_fronteira_regras_ncm
    DROP CONSTRAINT IF EXISTS uq_icms_fronteira_regras;

DO $$ BEGIN
    ALTER TABLE icms_fronteira_regras_ncm
        ADD CONSTRAINT uq_icms_fronteira_regras_uf
        UNIQUE NULLS NOT DISTINCT (company_id, ncm_prefixo, uf_estado);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Bloco 4: criar tabela de inaplicabilidades de fronteira por UF/NCM
CREATE TABLE IF NOT EXISTS icms_fronteira_inaplicabilidades (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    ncm_digits      VARCHAR(8)  NOT NULL,
    uf_estado       VARCHAR(2)  NOT NULL,
    motivo          TEXT        NOT NULL,
    vigencia_inicio DATE,
    vigencia_fim    DATE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Bloco 5: índice composto para consultas por UF + NCM (filtro mais frequente)
CREATE INDEX IF NOT EXISTS idx_icms_fronteira_inapl_uf_ncm
    ON icms_fronteira_inaplicabilidades(uf_estado, ncm_digits);

-- Bloco 6: comentários descritivos em PT-BR
COMMENT ON COLUMN icms_fronteira_regras_ncm.uf_estado IS
    'UF de destino para a qual a regra de fronteira se aplica (ex.: PE, BA, CE). '
    'DEFAULT PE preserva os registros da migration 091 (seed original Pernambuco). '
    'Incluída na constraint UNIQUE junto com company_id e ncm_prefixo.';

COMMENT ON COLUMN icms_fronteira_regras_ncm.mva_ajustado_4pct IS
    'MVA ajustado pré-calculado para alíquota interestadual de 4% '
    '(operações com produtos importados — Resolução Senado 13/2012). '
    'NULL = calcular dinamicamente via fórmula do Convênio ICMS 110/07.';

COMMENT ON COLUMN icms_fronteira_regras_ncm.mva_ajustado_7pct IS
    'MVA ajustado pré-calculado para alíquota interestadual de 7% '
    '(origem Sul/Sudeste para Norte/Nordeste/CO — exceto ES). '
    'NULL = calcular dinamicamente via fórmula do Convênio ICMS 110/07.';

COMMENT ON COLUMN icms_fronteira_regras_ncm.mva_ajustado_12pct IS
    'MVA ajustado pré-calculado para alíquota interestadual de 12% '
    '(demais operações interestaduais). '
    'NULL = calcular dinamicamente via fórmula do Convênio ICMS 110/07.';

COMMENT ON TABLE icms_fronteira_inaplicabilidades IS
    'Registra NCMs e UFs onde a sistemática de fronteira (ST/Antecipação) não se aplica '
    'por força de legislação estadual específica, convênios ICMS, ou decisões judiciais. '
    'Tabela vazia na inicialização — preenchida via UI conforme casos identificados (CADU-04).';
