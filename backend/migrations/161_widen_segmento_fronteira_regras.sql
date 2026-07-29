-- 161_widen_segmento_fronteira_regras.sql
-- icms_fronteira_regras_ncm.segmento (VARCHAR(80), migration 100) é um rótulo
-- livre de segmento/CNAE por linha do arquivo importado — sem relação com
-- segmento_codigo (FK numérica para segmentos_uf, usada pelo motor de ST).
-- Arquivos reais trazem descrições de segmento mais longas que 80 chars
-- (ex.: descrição completa copiada de segmentos_uf.descricao, também TEXT),
-- causando "value too long for type character varying(80)" (22001) e
-- rejeitando a linha inteira na importação. Amplia para TEXT, sem limite
-- artificial, igual à coluna descricao já usa.
ALTER TABLE icms_fronteira_regras_ncm ALTER COLUMN segmento TYPE TEXT;
