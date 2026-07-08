-- 154_pacotefiscal_xml_content.sql
--
-- Guarda o XML BRUTO da NF-e no upload do Teste Pacote Fiscal, para permitir
-- visualizar o XML original na conferência de divergências (pedido do Claudio,
-- 2026-07-08: "visualizar o XML quando houver divergências, facilita a
-- conferência"). Antes só guardávamos os campos já parseados — se houvesse
-- dúvida sobre um valor, não dava para inspecionar a fonte.
--
-- TEXT (não bytea): o XML da NF-e é sempre texto UTF-8/latin1 e o Postgres já
-- comprime via TOAST. Notas importadas ANTES desta migration ficam com
-- xml_content NULL — o visualizador orienta a reimportar para habilitar.
ALTER TABLE pacotefiscal_nfe_saidas
    ADD COLUMN IF NOT EXISTS xml_content TEXT;
