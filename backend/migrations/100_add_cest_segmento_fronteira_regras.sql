-- 100_add_cest_segmento_fronteira_regras.sql
-- Adiciona CEST e segmento às regras de fronteira para suportar o formato
-- "Base de Regras Unificada" (planilha do cliente: UF, Segmento, NCM, CEST,
-- Descrição, Alíq Int, MVA Orig, MVA 4%, MVA 7%, MVA 12%).
--
-- segmento habilita a seleção de regra por CNAE/segmento da empresa (G12):
-- mesmo NCM pode existir em segmentos diferentes (ex.: material de construção
-- vs autopeças) com MVAs distintos.
--
-- Idempotente: ADD COLUMN IF NOT EXISTS.

ALTER TABLE icms_fronteira_regras_ncm
    ADD COLUMN IF NOT EXISTS cest VARCHAR(10);

ALTER TABLE icms_fronteira_regras_ncm
    ADD COLUMN IF NOT EXISTS segmento VARCHAR(80);

COMMENT ON COLUMN icms_fronteira_regras_ncm.cest IS
    'Código Especificador da Substituição Tributária (CEST) — informativo, '
    'vindo da tabela de MVA. Formato livre (ex.: 06.007.00).';

COMMENT ON COLUMN icms_fronteira_regras_ncm.segmento IS
    'Segmento/grupo da regra (ex.: LUBRIFICANTES, AUTOPECAS, MATERIAL DE '
    'CONSTRUCAO). Usado para desambiguar MVA por CNAE da empresa quando o '
    'mesmo NCM existe em mais de um segmento (G12).';
