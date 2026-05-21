-- 084_add_competencia_and_fix_vw_xml_operacoes.sql
--
-- Duas melhorias na importação de XMLs:
--
-- 1. Adiciona coluna `competencia` em xml_upload_batches (MM/YYYY).
--    Quando informada pelo usuário, substitui o mes_ano derivado de dhEmi para
--    todas as NF-es do lote, resolvendo o problema de notas emitidas em meses
--    anteriores mas recebidas/lançadas no mês de competência atual.
--
-- 2. Reescreve vw_xml_operacoes_resumo para agregar pelos itens da nota,
--    fazendo JOIN com a tabela cfop para popular tipo_cfop (R/C/A/T/O/S).
--    Resolve o problema onde tipo_cfop era sempre NULL, bloqueando os filtros
--    de Revenda, Consumo, Ativo Imobilizado, etc. na página Mercadorias XML.
--
-- Estratégia da view:
--   - NF-es COM itens: agrega de nfe_*_itens JOIN cfop → tipo_cfop preenchido
--     valor = SUM(v_prod por tipo), icms/ipi/pis/cofins por item
--   - NF-es SEM itens (fallback): agrega do cabeçalho → tipo_cfop = NULL
--     valor = v_nf do cabeçalho, inclui vl_ibs/vl_cbs do cabeçalho
--
-- Limitação aceita: IBS/CBS são campos de cabeçalho (não por item); nas linhas
-- classificadas por tipo_cfop, vl_ibs_projetado/vl_cbs_projetado = 0.
-- Como as projeções são calculadas sobre `valor × alíquota` no handler, os
-- totais de IBS/CBS projetados continuam corretos.

-- ── 1. Competência em xml_upload_batches ──────────────────────────────────────
ALTER TABLE xml_upload_batches
    ADD COLUMN IF NOT EXISTS competencia VARCHAR(7);

COMMENT ON COLUMN xml_upload_batches.competencia IS
    'Mês de competência informado pelo usuário (MM/YYYY). Quando presente, '
    'substitui o mes_ano derivado de dhEmi para todas as NF-es/CT-es do lote.';

-- ── 2. Reescrever vw_xml_operacoes_resumo ─────────────────────────────────────
CREATE OR REPLACE VIEW vw_xml_operacoes_resumo AS

-- ── Entradas COM itens — classificadas por tipo_cfop ─────────────────────────
SELECT
    ne.company_id,
    ne.dest_cnpj_cpf                        AS filial_cnpj,
    ne.dest_nome                            AS filial_nome,
    ne.mes_ano,
    SUM(COALESCE(ei.v_prod,   0))           AS valor,
    SUM(COALESCE(ei.v_icms,   0))           AS icms,
    SUM(COALESCE(ei.v_ipi,    0))           AS vl_ipi,
    SUM(COALESCE(ei.v_pis,    0))           AS vl_pis,
    SUM(COALESCE(ei.v_cofins, 0))           AS vl_cofins,
    0::numeric                              AS vl_icms_projetado,
    0::numeric                              AS vl_ibs_projetado,
    0::numeric                              AS vl_cbs_projetado,
    'ENTRADA'                               AS tipo,
    cf.tipo                                 AS tipo_cfop,
    NULL::text                              AS origem,
    CASE cf.tipo
        WHEN 'R' THEN 'Entrada_Revenda'
        WHEN 'C' THEN 'Entrada_Consumo'
        WHEN 'A' THEN 'Entrada_Imobilizado'
        WHEN 'T' THEN 'Entrada_Transferencia'
        WHEN 'O' THEN 'Entrada_Outros'
        WHEN 'S' THEN 'Entrada_Servico'
        ELSE         'Entrada_XML'
    END                                     AS tipo_operacao
FROM nfe_entradas_itens ei
JOIN nfe_entradas ne
    ON ne.id = ei.nfe_id
   AND ne.source = 'xml_upload'
LEFT JOIN cfop cf ON cf.cfop = ei.cfop
GROUP BY ne.company_id, ne.dest_cnpj_cpf, ne.dest_nome, ne.mes_ano, cf.tipo

UNION ALL

-- ── Entradas SEM itens — fallback com tipo_cfop = NULL ───────────────────────
SELECT
    ne.company_id,
    ne.dest_cnpj_cpf                        AS filial_cnpj,
    ne.dest_nome                            AS filial_nome,
    ne.mes_ano,
    SUM(COALESCE(ne.v_nf,     0))           AS valor,
    SUM(COALESCE(ne.v_icms,   0))           AS icms,
    SUM(COALESCE(ne.v_ipi,    0))           AS vl_ipi,
    SUM(COALESCE(ne.v_pis,    0))           AS vl_pis,
    SUM(COALESCE(ne.v_cofins, 0))           AS vl_cofins,
    0::numeric                              AS vl_icms_projetado,
    SUM(COALESCE(ne.v_ibs,    0))           AS vl_ibs_projetado,
    SUM(COALESCE(ne.v_cbs,    0))           AS vl_cbs_projetado,
    'ENTRADA'                               AS tipo,
    NULL::text                              AS tipo_cfop,
    NULL::text                              AS origem,
    'Entrada_XML'                           AS tipo_operacao
FROM nfe_entradas ne
WHERE ne.source = 'xml_upload'
  AND NOT EXISTS (
      SELECT 1 FROM nfe_entradas_itens ei2 WHERE ei2.nfe_id = ne.id
  )
GROUP BY ne.company_id, ne.dest_cnpj_cpf, ne.dest_nome, ne.mes_ano

UNION ALL

-- ── Saídas COM itens — classificadas por tipo_cfop ───────────────────────────
SELECT
    ns.company_id,
    ns.emit_cnpj                            AS filial_cnpj,
    ns.emit_nome                            AS filial_nome,
    ns.mes_ano,
    SUM(COALESCE(si.v_prod,   0))           AS valor,
    SUM(COALESCE(si.v_icms,   0))           AS icms,
    SUM(COALESCE(si.v_ipi,    0))           AS vl_ipi,
    SUM(COALESCE(si.v_pis,    0))           AS vl_pis,
    SUM(COALESCE(si.v_cofins, 0))           AS vl_cofins,
    0::numeric                              AS vl_icms_projetado,
    0::numeric                              AS vl_ibs_projetado,
    0::numeric                              AS vl_cbs_projetado,
    'SAIDA'                                 AS tipo,
    cf.tipo                                 AS tipo_cfop,
    NULL::text                              AS origem,
    CASE cf.tipo
        WHEN 'R' THEN 'Saida_Revenda'
        WHEN 'C' THEN 'Saida_Consumo'
        WHEN 'A' THEN 'Saida_Imobilizado'
        WHEN 'T' THEN 'Saida_Transferencia'
        WHEN 'O' THEN 'Saida_Outros'
        WHEN 'S' THEN 'Saida_Servico'
        ELSE         'Saida_XML'
    END                                     AS tipo_operacao
FROM nfe_saidas_itens si
JOIN nfe_saidas ns
    ON ns.id = si.nfe_id
   AND ns.source = 'xml_upload'
LEFT JOIN cfop cf ON cf.cfop = si.cfop
GROUP BY ns.company_id, ns.emit_cnpj, ns.emit_nome, ns.mes_ano, cf.tipo

UNION ALL

-- ── Saídas SEM itens — fallback com tipo_cfop = NULL ─────────────────────────
SELECT
    ns.company_id,
    ns.emit_cnpj                            AS filial_cnpj,
    ns.emit_nome                            AS filial_nome,
    ns.mes_ano,
    SUM(COALESCE(ns.v_nf,     0))           AS valor,
    SUM(COALESCE(ns.v_icms,   0))           AS icms,
    SUM(COALESCE(ns.v_ipi,    0))           AS vl_ipi,
    SUM(COALESCE(ns.v_pis,    0))           AS vl_pis,
    SUM(COALESCE(ns.v_cofins, 0))           AS vl_cofins,
    0::numeric                              AS vl_icms_projetado,
    SUM(COALESCE(ns.v_ibs,    0))           AS vl_ibs_projetado,
    SUM(COALESCE(ns.v_cbs,    0))           AS vl_cbs_projetado,
    'SAIDA'                                 AS tipo,
    NULL::text                              AS tipo_cfop,
    NULL::text                              AS origem,
    'Saida_XML'                             AS tipo_operacao
FROM nfe_saidas ns
WHERE ns.source = 'xml_upload'
  AND NOT EXISTS (
      SELECT 1 FROM nfe_saidas_itens si2 WHERE si2.nfe_id = ns.id
  )
GROUP BY ns.company_id, ns.emit_cnpj, ns.emit_nome, ns.mes_ano;
