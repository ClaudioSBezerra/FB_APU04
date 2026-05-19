-- Migration 081: View vw_xml_operacoes_resumo
-- Expõe operações de entrada e saída de XMLs no mesmo shape que /api/reports/mercadorias
-- para alimentar o painel /mercadorias/xml (Simulador da Reforma Tributária - XMLs).
-- Filtro source='xml_upload' garante que apenas XMLs importados manualmente são exibidos.
--
-- Colunas alinhadas com a interface AggregatedData do frontend:
--   filial_cnpj, filial_nome, mes_ano, valor, icms,
--   vl_ipi, vl_pis, vl_cofins,
--   vl_icms_projetado (=0 — sem lógica SPED),
--   vl_ibs_projetado (= v_ibs quando disponível),
--   vl_cbs_projetado (= v_cbs quando disponível),
--   tipo ('ENTRADA' | 'SAIDA'),
--   tipo_cfop (NULL — XMLs não têm CFOP classificado),
--   origem (NULL),
--   tipo_operacao ('Entrada_XML' | 'Saida_XML')

CREATE OR REPLACE VIEW vw_xml_operacoes_resumo AS

-- Entradas: destinatário é a empresa cadastrada
SELECT
    ne.company_id,
    ne.dest_cnpj_cpf                        AS filial_cnpj,
    ne.dest_nome                            AS filial_nome,
    ne.mes_ano,
    SUM(COALESCE(ne.v_nf, 0))              AS valor,
    SUM(COALESCE(ne.v_icms, 0))            AS icms,
    SUM(COALESCE(ne.v_ipi, 0))             AS vl_ipi,
    SUM(COALESCE(ne.v_pis, 0))             AS vl_pis,
    SUM(COALESCE(ne.v_cofins, 0))          AS vl_cofins,
    0::numeric                             AS vl_icms_projetado,
    SUM(COALESCE(ne.v_ibs, 0))             AS vl_ibs_projetado,
    SUM(COALESCE(ne.v_cbs, 0))             AS vl_cbs_projetado,
    'ENTRADA'                              AS tipo,
    NULL::text                             AS tipo_cfop,
    NULL::text                             AS origem,
    'Entrada_XML'                          AS tipo_operacao
FROM nfe_entradas ne
WHERE ne.source = 'xml_upload'
GROUP BY ne.company_id, ne.dest_cnpj_cpf, ne.dest_nome, ne.mes_ano

UNION ALL

-- Saídas: emitente é a empresa cadastrada
SELECT
    ns.company_id,
    ns.emit_cnpj                            AS filial_cnpj,
    ns.emit_nome                            AS filial_nome,
    ns.mes_ano,
    SUM(COALESCE(ns.v_nf, 0))              AS valor,
    SUM(COALESCE(ns.v_icms, 0))            AS icms,
    SUM(COALESCE(ns.v_ipi, 0))             AS vl_ipi,
    SUM(COALESCE(ns.v_pis, 0))             AS vl_pis,
    SUM(COALESCE(ns.v_cofins, 0))          AS vl_cofins,
    0::numeric                             AS vl_icms_projetado,
    SUM(COALESCE(ns.v_ibs, 0))             AS vl_ibs_projetado,
    SUM(COALESCE(ns.v_cbs, 0))             AS vl_cbs_projetado,
    'SAIDA'                                AS tipo,
    NULL::text                             AS tipo_cfop,
    NULL::text                             AS origem,
    'Saida_XML'                            AS tipo_operacao
FROM nfe_saidas ns
WHERE ns.source = 'xml_upload'
GROUP BY ns.company_id, ns.emit_cnpj, ns.emit_nome, ns.mes_ano;
