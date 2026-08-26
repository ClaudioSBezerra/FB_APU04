-- 163_cnpj_cadastro_publico.sql
-- Cache de dados públicos de CNPJ (BrasilAPI, fonte: Receita Federal) para
-- enriquecer fornecedores/clientes já importados via NF-e: situação
-- cadastral, CNAE, Simples Nacional/MEI.
--
-- NOTA: não confundir com backend/services/rfb.go / rfb_processor.go — aquilo
-- é a integração com a API da Receita Federal para a Reforma Tributária
-- (IBS/CBS, apuração), completamente independente disso aqui. Esta tabela é
-- só consulta pública de cadastro de CNPJ.
--
-- Global (não tem company_id): o dado retornado pela Receita é o mesmo
-- independente de qual empresa está perguntando, então o cache é
-- compartilhado entre todas as empresas do sistema — evita reconsultar o
-- mesmo CNPJ (ex.: um grande fornecedor comum a várias empresas clientes).
CREATE TABLE IF NOT EXISTS cnpj_cadastro_publico (
    cnpj                VARCHAR(14)  PRIMARY KEY,
    razao_social         TEXT,
    nome_fantasia        TEXT,
    situacao_cadastral   VARCHAR(60),
    data_situacao_cadastral DATE,
    natureza_juridica    TEXT,
    porte                VARCHAR(30),
    cnae_codigo          VARCHAR(10),
    cnae_descricao       TEXT,
    uf                   VARCHAR(2),
    municipio            VARCHAR(100),
    data_inicio_atividade DATE,
    simples_nacional     BOOLEAN,
    data_opcao_simples   DATE,
    data_exclusao_simples DATE,
    mei                  BOOLEAN,
    data_opcao_mei       DATE,
    data_exclusao_mei    DATE,
    erro                 TEXT,        -- preenchido quando a consulta falhou (CNPJ inválido, timeout, etc.)
    consultado_em        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Job de enriquecimento em lote — acompanha o progresso de uma rodada de
-- consultas (a API pública é rate-limited, então roda em background).
CREATE TABLE IF NOT EXISTS cnpj_consulta_jobs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id   UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    status       VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending | processing | completed | error
    total        INT NOT NULL DEFAULT 0,
    processados  INT NOT NULL DEFAULT 0,
    encontrados  INT NOT NULL DEFAULT 0,
    erros        INT NOT NULL DEFAULT 0,
    mensagem     TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_cnpj_consulta_jobs_company
    ON cnpj_consulta_jobs(company_id, created_at DESC);
