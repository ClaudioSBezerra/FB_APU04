-- 116_create_municipios_ibge.sql
-- Tabela de referência (nacional, sem company_id) dos municípios do IBGE.
-- O código IBGE (7 dígitos) é o mesmo COD_MUN do reg 0000 do SPED, gravado em
-- import_jobs.cod_municipio. Serve para resolver código → nome do município/UF
-- na exibição das filiais e no eixo UF do módulo de fronteira.
--
-- O seed completo (5.570 municípios) vem no migration 117_seed_municipios_ibge.sql,
-- gerado a partir da API de localidades do IBGE.

CREATE TABLE IF NOT EXISTS municipios_ibge (
    codigo_ibge VARCHAR(7)   PRIMARY KEY,   -- COD_MUN (reg 0000), 7 dígitos
    uf          VARCHAR(2)   NOT NULL,      -- sigla da UF (ex: 'BA')
    uf_nome     VARCHAR(40)  NOT NULL,      -- nome da UF (ex: 'Bahia')
    nome        VARCHAR(120) NOT NULL       -- nome do município
);

CREATE INDEX IF NOT EXISTS idx_municipios_ibge_uf ON municipios_ibge(uf);

COMMENT ON TABLE municipios_ibge IS
    'Municípios do IBGE (referência nacional). codigo_ibge = COD_MUN do reg 0000 '
    'do SPED. Seed populado pelo migration 117 a partir da API de localidades IBGE.';
