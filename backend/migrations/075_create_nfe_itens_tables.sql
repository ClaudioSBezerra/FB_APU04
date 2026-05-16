-- 075_create_nfe_itens_tables.sql
-- Cria tabelas de itens de NF-e para armazenar dados por linha de nota (per D-07, D-09).
--
-- Cada NF-e pode ter 1..N itens. As tabelas são separadas por direção (entradas/saidas)
-- para facilitar queries e indexação independente.
--
-- Nota: cte_entradas NÃO tem tabela de itens — CT-es de carga não têm itens
-- no formato NF-e (apenas uma prestação de serviço de transporte).
--
-- Idempotente via CREATE TABLE IF NOT EXISTS e UNIQUE (nfe_id, n_item).

-- ── nfe_entradas_itens ────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS nfe_entradas_itens (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    nfe_id          UUID        NOT NULL REFERENCES nfe_entradas(id) ON DELETE CASCADE,
    company_id      UUID        NOT NULL,   -- desnormalizado para queries sem JOIN
    n_item          SMALLINT    NOT NULL,   -- número sequencial do item (<nItem>), 1..N

    -- Identificação do produto/serviço
    c_prod          VARCHAR(60),            -- código do produto (<cProd>)
    x_prod          VARCHAR(120) NOT NULL,  -- descrição do produto (<xProd>)
    ncm             VARCHAR(8),             -- NCM/SH (<NCM>)
    cfop            VARCHAR(4),             -- CFOP do item (<CFOP>)

    -- CST / CSOSN de cada tributo
    cst_icms        VARCHAR(3),             -- CST ICMS ou CSOSN (grupo ICMS)
    cst_pis         VARCHAR(2),             -- CST PIS (grupo PIS)
    cst_cofins      VARCHAR(2),             -- CST COFINS (grupo COFINS)

    -- Valores monetários do item
    v_prod          NUMERIC(15,2) DEFAULT 0,   -- valor do produto (<vProd>)
    v_total_item    NUMERIC(15,2) DEFAULT 0,   -- valor total do item com impostos (<vItem>)

    -- ICMS do item
    v_bc_icms       NUMERIC(15,2) DEFAULT 0,   -- base de cálculo ICMS
    v_icms          NUMERIC(15,2) DEFAULT 0,   -- valor ICMS

    -- IPI do item
    v_ipi           NUMERIC(15,2) DEFAULT 0,   -- valor IPI

    -- PIS do item
    v_bc_pis        NUMERIC(15,2) DEFAULT 0,   -- base de cálculo PIS
    v_pis           NUMERIC(15,2) DEFAULT 0,   -- valor PIS

    -- COFINS do item
    v_bc_cofins     NUMERIC(15,2) DEFAULT 0,   -- base de cálculo COFINS
    v_cofins        NUMERIC(15,2) DEFAULT 0,   -- valor COFINS

    -- IBS/CBS do item (Reforma Tributária — nullable quando não presente no XML)
    v_ibs           NUMERIC(15,2) DEFAULT 0,   -- valor IBS do item
    v_cbs           NUMERIC(15,2) DEFAULT 0,   -- valor CBS do item

    -- Classificação tributária (per D-07)
    cclasstrib      VARCHAR(20),   -- classificação tributária; nullable

    CONSTRAINT uq_nfe_entradas_itens_nfe_item UNIQUE (nfe_id, n_item)
);

CREATE INDEX IF NOT EXISTS idx_nfe_entradas_itens_company_ncm
    ON nfe_entradas_itens(company_id, ncm);

CREATE INDEX IF NOT EXISTS idx_nfe_entradas_itens_nfe_id
    ON nfe_entradas_itens(nfe_id);

-- ── nfe_saidas_itens ──────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS nfe_saidas_itens (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    nfe_id          UUID        NOT NULL REFERENCES nfe_saidas(id) ON DELETE CASCADE,
    company_id      UUID        NOT NULL,   -- desnormalizado para queries sem JOIN
    n_item          SMALLINT    NOT NULL,   -- número sequencial do item (<nItem>), 1..N

    -- Identificação do produto/serviço
    c_prod          VARCHAR(60),            -- código do produto (<cProd>)
    x_prod          VARCHAR(120) NOT NULL,  -- descrição do produto (<xProd>)
    ncm             VARCHAR(8),             -- NCM/SH (<NCM>)
    cfop            VARCHAR(4),             -- CFOP do item (<CFOP>)

    -- CST / CSOSN de cada tributo
    cst_icms        VARCHAR(3),             -- CST ICMS ou CSOSN (grupo ICMS)
    cst_pis         VARCHAR(2),             -- CST PIS (grupo PIS)
    cst_cofins      VARCHAR(2),             -- CST COFINS (grupo COFINS)

    -- Valores monetários do item
    v_prod          NUMERIC(15,2) DEFAULT 0,   -- valor do produto (<vProd>)
    v_total_item    NUMERIC(15,2) DEFAULT 0,   -- valor total do item com impostos (<vItem>)

    -- ICMS do item
    v_bc_icms       NUMERIC(15,2) DEFAULT 0,   -- base de cálculo ICMS
    v_icms          NUMERIC(15,2) DEFAULT 0,   -- valor ICMS

    -- IPI do item
    v_ipi           NUMERIC(15,2) DEFAULT 0,   -- valor IPI

    -- PIS do item
    v_bc_pis        NUMERIC(15,2) DEFAULT 0,   -- base de cálculo PIS
    v_pis           NUMERIC(15,2) DEFAULT 0,   -- valor PIS

    -- COFINS do item
    v_bc_cofins     NUMERIC(15,2) DEFAULT 0,   -- base de cálculo COFINS
    v_cofins        NUMERIC(15,2) DEFAULT 0,   -- valor COFINS

    -- IBS/CBS do item (Reforma Tributária — nullable quando não presente no XML)
    v_ibs           NUMERIC(15,2) DEFAULT 0,   -- valor IBS do item
    v_cbs           NUMERIC(15,2) DEFAULT 0,   -- valor CBS do item

    -- Classificação tributária (per D-07)
    cclasstrib      VARCHAR(20),   -- classificação tributária; nullable

    CONSTRAINT uq_nfe_saidas_itens_nfe_item UNIQUE (nfe_id, n_item)
);

CREATE INDEX IF NOT EXISTS idx_nfe_saidas_itens_company_ncm
    ON nfe_saidas_itens(company_id, ncm);

CREATE INDEX IF NOT EXISTS idx_nfe_saidas_itens_nfe_id
    ON nfe_saidas_itens(nfe_id);
