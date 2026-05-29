-- 130_create_inaplic_regras.sql
--
-- Fase 1 do motor de inaplicabilidade de ICMS Fronteira.
-- Tabela unificada que recebe as regras das 3 planilhas do contador (PE/BA/CE),
-- em status "pendente" até aprovação do contador na tela. Substitui, na prática,
-- a tabela icms_fronteira_inaplicabilidades (migration 097, só NCM) — que era
-- insuficiente para os 8 tipos de gatilho reais (CST/CFOP/CEST/VL_ICMS_ST/NCM/
-- CNAE/CNPJ-raiz/credenciamento).
--
-- Idempotente: CREATE TABLE/INDEX IF NOT EXISTS.

CREATE TABLE IF NOT EXISTS icms_fronteira_inaplic_regras (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    uf_estado        VARCHAR(2)  NOT NULL,                 -- PE | BA | CE
    id_regra         VARCHAR(16) NOT NULL,                 -- AP01, ST-BA02b, AP-CE01...
    instituto        VARCHAR(20) NOT NULL DEFAULT 'ANTECIPACAO', -- ANTECIPACAO | ANT_PARCIAL | ANT_PROPRIA | ST
    grupo            TEXT,
    hipotese         TEXT,
    tipo_verif       VARCHAR(20),                          -- CST | CFOP | CEST | VL_ICMS_ST | NCM | CNAE | CNPJ_RAIZ | CREDENC | COMBINADA | NATUREZA
    registro_sped    VARCHAR(16),
    campo_sped       VARCHAR(40),
    valores_gatilho  TEXT,
    registro_sped_2  VARCHAR(16),
    campo_sped_2     VARCHAR(40),
    valores_2        TEXT,
    logica           VARCHAR(5),                           -- AND | OR | N/A
    resultado        TEXT,                                 -- texto da planilha (NÃO CALCULAR / CALCULAR_OUTRO / ...)
    instrucao        TEXT,
    base_legal       TEXT,
    vigencia_inicio  DATE,
    vigencia_fim     DATE,
    auto_aplicavel   BOOLEAN NOT NULL DEFAULT false,       -- gatilho 100% SPED-derivável
    status_aprovacao VARCHAR(12) NOT NULL DEFAULT 'pendente', -- pendente | aprovada | rejeitada
    aprovado_por     TEXT,
    aprovado_em      TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Uma regra (id_regra) é única por UF + instituto (permite mesmo ID em UFs distintas
-- e separa antecipação parcial vs ST quando o contador reutiliza prefixos).
CREATE UNIQUE INDEX IF NOT EXISTS uq_inaplic_regra_uf_inst
    ON icms_fronteira_inaplic_regras(uf_estado, instituto, id_regra);

CREATE INDEX IF NOT EXISTS idx_inaplic_regra_uf_status
    ON icms_fronteira_inaplic_regras(uf_estado, status_aprovacao);
