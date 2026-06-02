# Plano: importar NF-e de entrada + CT-e via ERP_BRIDGE (split do BLOB XML)

> Status: em implementação (Fase 1 backend). Iniciado em 2026-06-02.

## Objetivo

Para o cliente de alto volume (Ferreira Costa, company `8c1b9f7b-c1d5-4493-a0a2-7055b6816438`),
trazer NF-e de entrada e CT-e para o módulo ICMS Fronteira lendo o XML cru armazenado em
CLOB no banco do cliente (Oracle FCCORP), via o ERP_BRIDGE. A importação direta (upload do
C: do usuário) já funciona bem; falta esse caminho automático a partir do banco do cliente.

## Fonte confirmada (FCCORP — sondagem 2026-06-02)

Tabelas legado Totvs presentes no FCCORP (`10.131.1.118:1521/FCCORP`, user `fcosta`):

- `sfc_nfe_imp`: `CHAVE_NFE` (VARCHAR2), `EMAIL_XML_NFE` (**CLOB**, XML completo nfeProc v4.00), `DATA_IMPORTACAO` (DATE)
- `sfc_cte_imp`: `CHAVE_CTE`, `CHAVE_NFE` (bônus — link CT-e→NF-e direto), `XML_CTE` (**CLOB**, cteProc v3.00), `DATA_IMPORTACAO` (TIMESTAMP)

São EXATAMENTE as tabelas que o modo `oracle_xml` do bridge já sabe ler (FONTES em bridge.py:261-290).
XML é completo (tem `<det>`/`<NCM>` na NF-e; `<toma3><toma>` no CT-e). Cliente roda em modo `sap_s4hana`.

⚠️ **Volume colossal**: NF-e ~450–620 mil/mês (2024–2025), ~20–30M histórico. CT-e ~25–53 mil/mês.
Decisão: importação **parametrizada por data, sob demanda** (NÃO importar histórico inteiro). Carga
incremental via watermark. NF-e + CT-e juntos.

## Decisões de arquitetura

1. **Split do XML no parser Go** (não em Python). Reusa `processSingleXML`/`processSingleCTe`
   (xml_upload.go) — parser provado da importação direta. Bridge só lê o CLOB e manda XML cru.
   Ganho de graça: itens (`nfe_entradas_itens`), refs CT-e (`cte_entradas_nfe_refs`), `toma`.
2. **Transporte**: novo endpoint `POST /api/erp-bridge/import/xml` auth **X-API-Key** (igual `/import/batch`),
   enfileira no pipeline assíncrono (`xml_upload_batches` + `xml_worker`) → aguenta volume, idempotente.
2b. **Conector ISOLADO** (`erp-bridge-simulador/`) em vez de alterar o `bridge.py` compartilhado —
   o bridge atual também serve o FB_APU02 (reforma) lendo SAP s4i_* do mesmo FCCORP; isolar evita
   risco de produção no APU02. O conector é read-only nas `sfc_*_imp`.
3. **source**: v1 reusa `source='xml_upload'` (zero alteração no parser; Fronteira casa por `chave`,
   não filtra por source). `source='erp_xml'` distinto = melhoria futura de observabilidade.

## Fluxo

```
FCCORP sfc_nfe_imp / sfc_cte_imp (CLOB XML)
   → bridge.py (lê CLOB via clob_para_str, janela :data_ini/:data_fim, lotes)
   → POST /api/erp-bridge/import/xml  (X-API-Key, {tipo, competencia, xmls:[{name,content}]})
   → cria xml_upload_batches (status pending, xml_data zip) → xml_worker
   → ProcessXMLBatch → processSingleXML/processSingleCTe
   → nfe_entradas(+itens) / cte_entradas(+refs+toma)
```

## Fases

- [x] **Fase 1 — Backend**: handler `ERPBridgeXMLImportHandler` (erp_bridge_xml.go) + rota
      `/api/erp-bridge/import/xml`. Reusa auth X-API-Key, `chunkXMLFiles`, enfileiramento assíncrono
      (xml_upload_batches → xml_worker → ProcessXMLBatch → parser). Compila OK. (concluída 2026-06-02)
- [x] **Fase 2 — Conector ISOLADO** (`erp-bridge-simulador/`): decisão de NÃO mexer no
      `erp-bridge-aws/bridge.py` (compartilhado com APU02/reforma, lê SAP s4i_* do mesmo FCCORP —
      risco de produção). Conector autônomo `bridge_simulador.py` lê `sfc_nfe_imp`/`sfc_cte_imp`
      (SELECT read-only), CLOB em streaming (fetch_lobs=False, memória limitada), dedup via SQLite,
      envia em lotes ao `/api/erp-bridge/import/xml`. CLI parametrizado por data (--data-ini/--data-fim),
      --dry-run, --reset-tracker. Dockerfile + config.example.yaml + README. py_compile OK. (concluída 2026-06-02)
- [ ] **Fase 3 — Volume/resiliência**: janelas de data, async worker (existe), reconexão DPY-4011 (existe),
      watermark só avança com zero erros (existe).
- [ ] **Fase 4 — Validação**: Bloco C, fretes/CT-e e resumo refletindo entradas/CTEs num período conhecido;
      comparar com importação direta.

## Campos mínimos exigidos pela Fronteira (do parser)

- `nfe_entradas`: chave_nfe, data_emissao, mes_ano, forn_cnpj, dest_uf, dest_cnpj_cpf, v_prod, v_icms, v_frete, v_outro, v_ipi
- `nfe_entradas_itens`: nfe_id, n_item, cfop, ncm, v_prod (Bloco C faz INNER JOIN — itens obrigatórios)
- `cte_entradas`: chave_cte, data_emissao, mes_ano, emit_cnpj, v_prest, v_icms, **toma**, toma4_cnpj, dest_cnpj_cpf
- `cte_entradas_nfe_refs`: cte_id, chave_nfe (única ponte CT-e→NF-e p/ frete)

(O parser Go já popula tudo isso.)

## Pontos de atenção

- `source` CHECK só aceita oracle_bridge|xml_upload|manual — usar 'erp_xml' exigiria migração.
- CT-e sem `toma` é descartado do frete — parser já extrai, validar nos XMLs do cliente.
- Carga inicial pesada → sempre por janelas de data + assíncrono.
