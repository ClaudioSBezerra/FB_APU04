-- 148_pacotefiscal_isolated_schema.sql
-- Estrutura de dados ISOLADA e dedicada ao módulo "Teste Pacote Fiscal"
-- (Fase 12+). Decisão do usuário (2026-07): não reaproveitar mais
-- nfe_saidas/nfe_saidas_itens (decisão original da Fase 11/12) — o módulo
-- passa a ter seu próprio pipeline de importação de XML, suas próprias
-- tabelas, e captura o cabeçalho COMPLETO (emitente e destinatário: razão
-- social, IE, endereço completo, contato) que nfe_saidas nunca teve.
-- Motivo do usuário: reduzir o raio de impacto — um bug na importação ou
-- schema deste módulo não pode afetar Painel XMLs, Conciliação, Auditoria
-- Fiscal etc., que continuam lendo nfe_saidas/nfe_saidas_itens normalmente.
--
-- Isolamento é de DADOS (tabelas próprias) — o parser Go reutiliza helpers
-- genéricos já testados (charset reader, extração de ZIP anti-bomb,
-- conversão decimal) do pacote handlers, mas usa tipos de struct e nomes de
-- tabela inteiramente próprios (prefixo pacotefiscal_), sem tocar em
-- nfe_saidas.go/xml_upload.go.

CREATE TABLE IF NOT EXISTS pacotefiscal_nfe_saidas (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id      UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,

    -- Identificação
    chave_nfe       VARCHAR(44) NOT NULL,
    modelo          SMALLINT NOT NULL,
    serie           VARCHAR(3),
    numero_nfe      VARCHAR(9),
    data_emissao    DATE NOT NULL,
    mes_ano         VARCHAR(7) NOT NULL,
    nat_op          VARCHAR(60),
    tp_nf           VARCHAR(1),              -- <tpNF> 0=entrada 1=saída
    ind_final       SMALLINT,                -- <indFinal>
    ind_pres        VARCHAR(1),              -- <indPres> presença do comprador
    fin_nfe         VARCHAR(1),              -- <finNFe> finalidade da emissão

    -- Emitente — cabeçalho completo (diferente de nfe_saidas: inclui IE, IEST,
    -- CRT, nome fantasia, endereço completo e telefone)
    emit_cnpj       VARCHAR(14),
    emit_cpf        VARCHAR(11),
    emit_xnome      VARCHAR(60),
    emit_xfant      VARCHAR(60),
    emit_ie         VARCHAR(14),
    emit_iest       VARCHAR(14),
    emit_crt        VARCHAR(1),
    emit_fone       VARCHAR(20),
    emit_logradouro VARCHAR(60),
    emit_numero     VARCHAR(60),
    emit_complemento VARCHAR(60),
    emit_bairro     VARCHAR(60),
    emit_c_mun      VARCHAR(7),
    emit_x_mun      VARCHAR(60),
    emit_uf         VARCHAR(2),
    emit_cep        VARCHAR(8),
    emit_c_pais     VARCHAR(4),
    emit_x_pais     VARCHAR(60),

    -- Destinatário — cabeçalho completo (diferente de nfe_saidas: inclui IE,
    -- indIEDest, email, endereço completo e telefone)
    dest_cnpj       VARCHAR(14),
    dest_cpf        VARCHAR(11),
    dest_xnome      VARCHAR(60),
    dest_ie         VARCHAR(14),
    dest_ind_ie     VARCHAR(1),              -- <indIEDest> 1=contrib. 2=isento 9=não contrib.
    dest_email      VARCHAR(60),
    dest_fone       VARCHAR(20),
    dest_logradouro VARCHAR(60),
    dest_numero     VARCHAR(60),
    dest_complemento VARCHAR(60),
    dest_bairro     VARCHAR(60),
    dest_c_mun      VARCHAR(7),
    dest_x_mun      VARCHAR(60),
    dest_uf         VARCHAR(2),
    dest_cep        VARCHAR(8),
    dest_c_pais     VARCHAR(4),
    dest_x_pais     VARCHAR(60),

    -- ICMSTot — totais do cabeçalho (mesmos nomes/semântica de nfe_saidas
    -- para reduzir fricção de quem já conhece a tabela antiga)
    v_bc            NUMERIC(15,2) DEFAULT 0,
    v_icms          NUMERIC(15,2) DEFAULT 0,
    v_icms_deson    NUMERIC(15,2) DEFAULT 0,
    v_fcp           NUMERIC(15,2) DEFAULT 0,
    v_bc_st         NUMERIC(15,2) DEFAULT 0,
    v_st            NUMERIC(15,2) DEFAULT 0,
    v_fcp_st        NUMERIC(15,2) DEFAULT 0,
    v_fcp_st_ret    NUMERIC(15,2) DEFAULT 0,
    v_prod          NUMERIC(15,2) DEFAULT 0,
    v_frete         NUMERIC(15,2) DEFAULT 0,
    v_seg           NUMERIC(15,2) DEFAULT 0,
    v_desc          NUMERIC(15,2) DEFAULT 0,
    v_ii            NUMERIC(15,2) DEFAULT 0,
    v_ipi           NUMERIC(15,2) DEFAULT 0,
    v_ipi_devol     NUMERIC(15,2) DEFAULT 0,
    v_pis           NUMERIC(15,2) DEFAULT 0,
    v_cofins        NUMERIC(15,2) DEFAULT 0,
    v_outro         NUMERIC(15,2) DEFAULT 0,
    v_nf            NUMERIC(15,2) DEFAULT 0,

    -- IBSCBSTot — totais da Reforma Tributária
    v_bc_ibs_cbs    NUMERIC(15,2),
    v_ibs_uf        NUMERIC(15,2),
    v_ibs_mun       NUMERIC(15,2),
    v_ibs           NUMERIC(15,2),
    v_cred_pres_ibs NUMERIC(15,2),
    v_cbs           NUMERIC(15,2),
    v_cred_pres_cbs NUMERIC(15,2),

    source          TEXT NOT NULL DEFAULT 'xml_upload',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_pacotefiscal_nfe_saidas_company_chave UNIQUE (company_id, chave_nfe)
);

CREATE INDEX IF NOT EXISTS idx_pacotefiscal_nfe_saidas_company_mes  ON pacotefiscal_nfe_saidas(company_id, mes_ano);
CREATE INDEX IF NOT EXISTS idx_pacotefiscal_nfe_saidas_company_data ON pacotefiscal_nfe_saidas(company_id, data_emissao);
CREATE INDEX IF NOT EXISTS idx_pacotefiscal_nfe_saidas_chave        ON pacotefiscal_nfe_saidas(company_id, chave_nfe);

CREATE TABLE IF NOT EXISTS pacotefiscal_nfe_saidas_itens (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nfe_id          UUID NOT NULL REFERENCES pacotefiscal_nfe_saidas(id) ON DELETE CASCADE,
    company_id      UUID NOT NULL,  -- desnormalizado, mesmo padrão de nfe_saidas_itens (guard IDOR)
    n_item          INTEGER NOT NULL,

    -- Produto
    c_prod          VARCHAR(60),
    c_ean           VARCHAR(20),
    x_prod          VARCHAR(120) NOT NULL,
    ncm             VARCHAR(8),
    cest            VARCHAR(7),
    cfop            VARCHAR(4),
    u_com           VARCHAR(6),
    q_com           NUMERIC(15,4) DEFAULT 0,
    v_un_com        NUMERIC(21,10) DEFAULT 0,
    v_prod          NUMERIC(15,2) DEFAULT 0,
    u_trib          VARCHAR(6),
    q_trib          NUMERIC(15,4) DEFAULT 0,
    v_un_trib       NUMERIC(21,10) DEFAULT 0,
    v_desc          NUMERIC(15,2) DEFAULT 0,  -- pDesconto do pacote fiscal (Fase 11)
    v_outro         NUMERIC(15,2) DEFAULT 0,  -- pDespesas do pacote fiscal (Fase 11)

    -- ICMS
    cst_orig        VARCHAR(1),
    cst_icms        VARCHAR(3),   -- CST (regime normal) ou CSOSN (Simples Nacional)
    v_bc_icms       NUMERIC(15,2) DEFAULT 0,
    p_icms          NUMERIC(7,4) DEFAULT 0,
    v_icms          NUMERIC(15,2) DEFAULT 0,
    v_bc_st         NUMERIC(15,2) DEFAULT 0,
    p_mva_st        NUMERIC(7,4) DEFAULT 0,
    v_st            NUMERIC(15,2) DEFAULT 0,

    -- IPI
    v_bc_ipi        NUMERIC(15,2) DEFAULT 0,
    p_ipi           NUMERIC(7,4) DEFAULT 0,
    v_ipi           NUMERIC(15,2) DEFAULT 0,

    -- PIS
    cst_pis         VARCHAR(2),
    v_bc_pis        NUMERIC(15,2) DEFAULT 0,
    p_pis           NUMERIC(7,4) DEFAULT 0,
    v_pis           NUMERIC(15,2) DEFAULT 0,

    -- COFINS
    cst_cofins      VARCHAR(2),
    v_bc_cofins     NUMERIC(15,2) DEFAULT 0,
    p_cofins        NUMERIC(7,4) DEFAULT 0,
    v_cofins        NUMERIC(15,2) DEFAULT 0,

    -- IBS/CBS por item (Reforma Tributária) — melhor esforço: caminho de tags
    -- <imposto><IBSCBS><gIBSCBS>...</gIBSCBS></IBSCBS> não verificado contra
    -- um XML real de produção com Reforma Tributária ainda (schema em
    -- evolução) — conferir no primeiro import real e ajustar se necessário.
    cst_ibscbs      VARCHAR(3),
    cclasstrib      VARCHAR(6),
    v_bc_ibs_cbs    NUMERIC(15,2),
    v_ibs           NUMERIC(15,2),
    v_cbs           NUMERIC(15,2),

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_pacotefiscal_nfe_saidas_itens UNIQUE (nfe_id, n_item)
);

CREATE INDEX IF NOT EXISTS idx_pacotefiscal_itens_nfe_id ON pacotefiscal_nfe_saidas_itens(nfe_id);
CREATE INDEX IF NOT EXISTS idx_pacotefiscal_itens_company ON pacotefiscal_nfe_saidas_itens(company_id);

-- Repontar fiscal_execution_items (Fase 11, migration 147) para o novo item
-- table isolado. fiscal_execution_items é EXCLUSIVO deste módulo (nenhum
-- outro handler grava/lê essa tabela) — repontar a FK aqui é seguro.
--
-- TRUNCATE necessário: linhas existentes referenciam nfe_saidas_itens.id,
-- que não existe em pacotefiscal_nfe_saidas_itens — o novo FK rejeitaria
-- essas linhas de qualquer forma. São dados de teste da Fase 11/12 (poucas
-- execuções, sem valor histórico a preservar); o usuário reimporta via XML
-- e reexecuta pelo botão "Executar" normalmente.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE constraint_name = 'fiscal_execution_items_nfe_item_id_fkey'
          AND table_name = 'fiscal_execution_items'
    ) THEN
        TRUNCATE TABLE fiscal_execution_items;
        ALTER TABLE fiscal_execution_items
            DROP CONSTRAINT fiscal_execution_items_nfe_item_id_fkey;
    END IF;
END $$;

ALTER TABLE fiscal_execution_items
    ADD CONSTRAINT fiscal_execution_items_nfe_item_id_fkey
    FOREIGN KEY (nfe_item_id) REFERENCES pacotefiscal_nfe_saidas_itens(id) ON DELETE CASCADE;
