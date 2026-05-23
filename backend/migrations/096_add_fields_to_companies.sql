-- 096_add_fields_to_companies.sql
-- Re-adiciona CNPJ e 6 campos de cadastro mestre à tabela companies (per CADU-01).
--
-- O CNPJ foi removido na migration 023 (movido para branches/filiais).
-- Agora é re-adicionado como VARCHAR(18) nullable, sem UNIQUE — multi-filial pode
-- compartilhar CNPJs relacionados (ex.: matriz/filial com mesmo CNPJ raiz).
--
-- Idempotente: todos os campos usam cláusula IF NOT EXISTS.
-- Nenhuma constraint adicional além das existentes na tabela.

ALTER TABLE companies ADD COLUMN IF NOT EXISTS cnpj VARCHAR(18);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS inscricao_estadual VARCHAR(30);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS cnae_principal     VARCHAR(7);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS cnae_secundario    TEXT[];
ALTER TABLE companies ADD COLUMN IF NOT EXISTS municipio          VARCHAR(100);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS segmento_economico VARCHAR(100);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS incentivos_fiscais JSONB;

COMMENT ON COLUMN companies.cnpj IS
    'CNPJ da empresa no formato de 14 dígitos numéricos ou formatado (18 chars com pontos/barras/traço). '
    'Nullable — empresas existentes e novas não são obrigadas a informar. Sem UNIQUE: multi-filial '
    'pode compartilhar CNPJs relacionados.';

COMMENT ON COLUMN companies.inscricao_estadual IS
    'Inscrição Estadual da empresa no estado de domicílio fiscal. '
    'Formato varia por UF; armazenado como texto livre (até 30 chars). Nullable.';

COMMENT ON COLUMN companies.cnae_principal IS
    'Código da atividade econômica principal (CNAE) no formato 7 caracteres (ex.: 4711-3/01). '
    'Determina o segmento econômico para fins de ST e regimes especiais. Nullable.';

COMMENT ON COLUMN companies.cnae_secundario IS
    'Lista de CNAEs secundários da empresa (array nativo PostgreSQL TEXT[]). '
    'Permite registrar múltiplas atividades econômicas acessórias. Nullable.';

COMMENT ON COLUMN companies.municipio IS
    'Município de domicílio fiscal da empresa (até 100 chars). '
    'Usado para verificação de benefícios fiscais municipais e endereço de entrega de SPED. Nullable.';

COMMENT ON COLUMN companies.segmento_economico IS
    'Segmento econômico da empresa para fins de categorização interna (ex.: Varejo, Indústria, Atacado). '
    'Não é o CNAE — é uma classificação de negócio usada por relatórios gerenciais. Nullable.';

COMMENT ON COLUMN companies.incentivos_fiscais IS
    'Estrutura JSONB livre para registrar incentivos fiscais da empresa (ex.: PRODEPE, PROIND, RECOOP). '
    'Schema não fixo nesta fase — revisado conforme necessidade dos relatórios de CADU-01. Nullable.';
