-- 121_add_segmento_codigo_to_regras.sql
-- Adiciona segmento_codigo (int) a icms_fronteira_regras_ncm.
-- Este campo referencia segmentos_uf.codigo e indica a qual segmento de ST
-- a regra pertence. Quando preenchido, o cálculo só aplica ST se a empresa
-- tiver esse segmento cadastrado em company_segmentos para a UF da filial.
-- Quando NULL, a nota com CFOP de ST é reclassificada como ANTECIPAÇÃO.

ALTER TABLE icms_fronteira_regras_ncm
    ADD COLUMN IF NOT EXISTS segmento_codigo INT;

COMMENT ON COLUMN icms_fronteira_regras_ncm.segmento_codigo IS
    'Código do segmento ST (referência a segmentos_uf.codigo). NULL = nenhum segmento específico; '
    'nesses casos o CFOP de ST é reclassificado como ANTECIPAÇÃO no cálculo.';
