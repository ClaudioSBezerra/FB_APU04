-- 124_prodepe_enquadramentos.sql
-- PRODEPE / regime especial de central de distribuição, POR ESTABELECIMENTO (CNPJ).
--
-- Base legal: art. 11-A do Decreto 21.959/1999 (incluído pelo Dec. 47.864/2019):
-- a central de distribuição beneficiada é detentora de regime especial → NÃO
-- aplicabilidade da antecipação tributária nas aquisições e responsabilidade pela
-- ST nas SAÍDAS (que é apuração, fora do fronteira). Logo, na ENTRADA, as
-- aquisições do CD ficam dispensadas de antecipação E de ST.
--
-- O benefício é do ESTABELECIMENTO (CNPJ/CACEPE), não da UF: numa mesma UF a
-- empresa pode ter um CD com TARE e filiais de varejo normais. Por isso a chave
-- é o CNPJ da filial recebedora (Leitura A confirmada pelo contador, 2026-05-27).
--
-- A lista de NCMs (produtos beneficiados do decreto) é guardada para
-- rastreabilidade e para o crédito presumido das saídas — NÃO filtra o cálculo
-- de fronteira na Leitura A (dispensa vale para todas as aquisições do CD).

CREATE TABLE IF NOT EXISTS prodepe_enquadramentos (
    id                     UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id             UUID         NOT NULL,
    cnpj                   VARCHAR(14)  NOT NULL,          -- estabelecimento beneficiado
    inscricao_estadual     VARCHAR(20),                    -- CACEPE
    programa               VARCHAR(20)  NOT NULL DEFAULT 'PRODEPE'  -- PRODEPE (CD) | PROIND (indústria)
                           CHECK (programa IN ('PRODEPE','PROIND')),
    num_ato                VARCHAR(60),                    -- nº do decreto/ato concessório
    enquadramento          VARCHAR(80),                    -- descritivo livre (ex.: central de distribuição, indústria)
    credito_presumido_pct  NUMERIC(5,2),                   -- doc (ex.: 3,00) — apuração, não fronteira
    vigencia_inicio        DATE,
    vigencia_fim           DATE,
    dispensa_antecipacao   BOOLEAN      NOT NULL DEFAULT true,  -- regime especial art. 11-A / PROIND
    observacoes            TEXT,
    ativo                  BOOLEAN      NOT NULL DEFAULT true,
    created_at             TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT uq_prodepe_enq UNIQUE (company_id, cnpj, num_ato)
);

CREATE INDEX IF NOT EXISTS idx_prodepe_enq_company_cnpj
    ON prodepe_enquadramentos (company_id, cnpj);

COMMENT ON TABLE prodepe_enquadramentos IS
    'Enquadramentos PRODEPE (central de distribuição) e PROIND (indústria) por '
    'estabelecimento (CNPJ). Dispensam antecipação/ST de fronteira nas aquisições '
    'durante a vigência. Exceções (combustíveis/lubrificantes/camarão) ficam em '
    'tabela separada na Fase B.1.';
COMMENT ON COLUMN prodepe_enquadramentos.programa IS
    'PRODEPE = regime especial de central de distribuição (art. 11-A Dec. 21.959/1999). '
    'PROIND = Programa de Estímulo à Indústria do Estado de PE. Ambos: dispensa de '
    'antecipação e ST nas aquisições do estabelecimento beneficiário.';

CREATE TABLE IF NOT EXISTS prodepe_ncms (
    id                BIGSERIAL   PRIMARY KEY,
    enquadramento_id  UUID        NOT NULL REFERENCES prodepe_enquadramentos(id) ON DELETE CASCADE,
    ncm               VARCHAR(8)  NOT NULL,
    descricao         TEXT,
    CONSTRAINT uq_prodepe_ncm UNIQUE (enquadramento_id, ncm)
);

COMMENT ON TABLE prodepe_ncms IS
    'Produtos beneficiados (NCM) do decreto PRODEPE — rastreabilidade e base do '
    'crédito presumido das saídas. Na Leitura A não filtra o cálculo de fronteira.';
