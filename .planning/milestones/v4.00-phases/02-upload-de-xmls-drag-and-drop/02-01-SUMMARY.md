---
phase: 02-upload-de-xmls-drag-and-drop
plan: "01"
subsystem: database-schema
tags: [migrations, postgresql, nfe, xml-upload, schema]
dependency_graph:
  requires: []
  provides:
    - schema/nfe_entradas.source
    - schema/nfe_saidas.source
    - schema/cte_entradas.source
    - schema/nfe_entradas_itens
    - schema/nfe_saidas_itens
    - schema/xml_upload_batches
    - schema/companies.regime_tributario
    - views/vw_xml_entradas_resumo
    - views/vw_xml_saidas_resumo
    - views/vw_xml_ctes_resumo
    - views/vw_xml_itens_ncm
  affects:
    - backend/main.go (migrator auto-aplica ao startup)
    - plan 02-02 (handlers XML usam estas tabelas)
    - plan 02-03 (frontend consome as views)
tech_stack:
  added: []
  patterns:
    - "ADD COLUMN IF NOT EXISTS com DEFAULT — virtual default PostgreSQL 15 (sem lock)"
    - "CREATE OR REPLACE VIEW — idempotente para views"
    - "partial index WHERE status IN (...) — worker assíncrono eficiente"
key_files:
  created:
    - backend/migrations/074_add_source_to_nfe_tables.sql
    - backend/migrations/075_create_nfe_itens_tables.sql
    - backend/migrations/076_create_xml_upload_batches.sql
    - backend/migrations/077_add_regime_tributario_to_companies.sql
    - backend/migrations/078_create_vw_xml_panels.sql
  modified: []
decisions:
  - "Usar v_icms diretamente nas views (não v_icms_dest/v_icms_remet — campos inexistentes confirmados via schema 059)"
  - "cte_entradas_itens não criada — CT-e de carga não tem itens no formato NF-e"
  - "xml_data BYTEA em xml_upload_batches — armazena XMLs comprimidos para batches assíncronos >50 arquivos"
metrics:
  duration: "~15 minutos"
  completed_date: "2026-05-16"
  tasks_completed: 3
  tasks_total: 3
  files_created: 5
  files_modified: 0
---

# Phase 02 Plan 01: Migrations de Schema para Upload de XMLs — Summary

Schema estabelecido com 5 migrations idempotentes (074-078) para suportar upload de XMLs, rastreamento por origem, análise por item e painel de gestão.

## Migrations Criadas

### 074 — Coluna `source` em nfe_entradas, nfe_saidas, cte_entradas

**Propósito:** Rastrear a origem de cada documento fiscal (ERP Bridge vs. XML upload manual).

**O que foi adicionado:**
- `source TEXT NOT NULL DEFAULT 'oracle_bridge'` nas 3 tabelas
- CHECK constraints: `chk_nfe_entradas_source`, `chk_nfe_saidas_source`, `chk_cte_entradas_source`
- Índices compostos: `idx_nfe_entradas_source(company_id, source)`, `idx_nfe_saidas_source(company_id, source)`, `idx_cte_entradas_source(company_id, source)`
- COMMENT ON COLUMN documentando os 3 valores válidos

**Valores válidos:** `oracle_bridge` | `xml_upload` | `manual`

---

### 075 — Tabelas `nfe_entradas_itens` e `nfe_saidas_itens`

**Propósito:** Armazenar dados por linha de nota para o relatório CCLASSTRIB e análise de NCM.

**Schema de `nfe_entradas_itens` (e `nfe_saidas_itens`, estrutura idêntica):**

| Coluna | Tipo | Notas |
|--------|------|-------|
| id | UUID PK | gen_random_uuid() |
| nfe_id | UUID NOT NULL | FK → nfe_entradas(id) ON DELETE CASCADE |
| company_id | UUID NOT NULL | desnormalizado — queries sem JOIN |
| n_item | SMALLINT NOT NULL | número sequencial do item (<nItem>) |
| c_prod | VARCHAR(60) | código do produto (<cProd>) |
| x_prod | VARCHAR(120) NOT NULL | descrição (<xProd>) |
| ncm | VARCHAR(8) | NCM/SH (<NCM>) |
| cfop | VARCHAR(4) | CFOP do item |
| cst_icms | VARCHAR(3) | CST ICMS ou CSOSN |
| cst_pis | VARCHAR(2) | CST PIS |
| cst_cofins | VARCHAR(2) | CST COFINS |
| v_prod | NUMERIC(15,2) DEFAULT 0 | valor do produto |
| v_total_item | NUMERIC(15,2) DEFAULT 0 | valor total com impostos |
| v_bc_icms | NUMERIC(15,2) DEFAULT 0 | base ICMS |
| v_icms | NUMERIC(15,2) DEFAULT 0 | valor ICMS |
| v_ipi | NUMERIC(15,2) DEFAULT 0 | valor IPI |
| v_bc_pis | NUMERIC(15,2) DEFAULT 0 | base PIS |
| v_pis | NUMERIC(15,2) DEFAULT 0 | valor PIS |
| v_bc_cofins | NUMERIC(15,2) DEFAULT 0 | base COFINS |
| v_cofins | NUMERIC(15,2) DEFAULT 0 | valor COFINS |
| v_ibs | NUMERIC(15,2) DEFAULT 0 | IBS do item (Reforma) |
| v_cbs | NUMERIC(15,2) DEFAULT 0 | CBS do item (Reforma) |
| cclasstrib | VARCHAR(20) | classificação tributária; nullable |

**Constraints:** `uq_nfe_entradas_itens_nfe_item UNIQUE (nfe_id, n_item)`

**Índices:** `idx_nfe_entradas_itens_company_ncm(company_id, ncm)`, `idx_nfe_entradas_itens_nfe_id(nfe_id)`

---

### 076 — Tabela `xml_upload_batches`

**Propósito:** Histórico e controle de uploads em lote (per D-13).

**Schema:**

| Coluna | Tipo | Notas |
|--------|------|-------|
| id | UUID PK | gen_random_uuid() |
| company_id | UUID NOT NULL | FK → companies(id) ON DELETE CASCADE |
| uploaded_by | UUID | FK → users(id) ON DELETE SET NULL |
| tipo | TEXT NOT NULL | 'entradas' \| 'saidas' \| 'ctes' |
| filename | TEXT | nome do arquivo ZIP ou XML original |
| total_count | INT DEFAULT 0 | total de XMLs no lote |
| processed_count | INT DEFAULT 0 | XMLs processados |
| imported_count | INT DEFAULT 0 | XMLs importados com sucesso |
| rejected_count | INT DEFAULT 0 | XMLs rejeitados |
| status | TEXT DEFAULT 'pending' | 'pending' \| 'processing' \| 'done' \| 'failed' |
| error_details | JSONB | erros por XML rejeitado |
| xml_data | BYTEA | XMLs comprimidos para async >50 arquivos; NULL = síncrono |
| created_at | TIMESTAMPTZ DEFAULT NOW() | |
| completed_at | TIMESTAMPTZ | |

**Constraints:** `chk_xml_upload_batches_tipo`, `chk_xml_upload_batches_status`

**Índices:**
- `idx_xml_upload_batches_company_created(company_id, created_at DESC)` — histórico
- `idx_xml_upload_batches_status_active` — partial index WHERE status IN ('pending','processing') — worker assíncrono

---

### 077 — Coluna `regime_tributario` em `companies`

**O que foi adicionado:**
- `regime_tributario TEXT NOT NULL DEFAULT 'nao_informado'`
- `chk_companies_regime_tributario CHECK (regime_tributario IN ('lucro_real', 'lucro_presumido', 'simples_nacional', 'nao_informado'))`
- COMMENT ON COLUMN documentando valores e impacto na classificação XML

---

### 078 — Views para Painel XML

**Propósito:** 4 views regulares (não materializadas) para o painel de XMLs importados.

| View | Fonte | GROUP BY | Uso |
|------|-------|----------|-----|
| `vw_xml_entradas_resumo` | nfe_entradas | company_id, forn_cnpj, forn_nome, mes_ano, source | Painel de entradas com filtro source |
| `vw_xml_saidas_resumo` | nfe_saidas | company_id, emit_cnpj, emit_nome, mes_ano, source | Painel de saídas com filtro source |
| `vw_xml_ctes_resumo` | cte_entradas | company_id, emit_cnpj, emit_nome, mes_ano, source | Painel de CT-es com filtro source |
| `vw_xml_itens_ncm` | nfe_entradas_itens | company_id, ncm | Relatório CCLASSTRIB — variância de CST por NCM |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Ajuste de Schema] Colunas v_icms_dest/v_icms_remet não existem em nfe_entradas**
- **Found during:** Task 3 (criação das views)
- **Issue:** O rascunho do plano referenciava `v_icms_dest` e `v_icms_remet` na vw_xml_entradas_resumo, mas ao verificar o schema real (migration 059), esses campos não existem — o campo correto é `v_icms` (ICMSTot).
- **Fix:** Substituído por `SUM(COALESCE(ne.v_icms, 0)) AS v_icms` nas 2 views de NF-e. Comportamento equivalente — vw_nfe_entradas_impostos (072) usa o mesmo campo.
- **Files modified:** backend/migrations/078_create_vw_xml_panels.sql
- **Commit:** d5cbe4c

## Self-Check: PASSED

Arquivos criados verificados:
- backend/migrations/074_add_source_to_nfe_tables.sql — FOUND (commit b059ea0)
- backend/migrations/075_create_nfe_itens_tables.sql — FOUND (commit bb76c6f)
- backend/migrations/076_create_xml_upload_batches.sql — FOUND (commit bb76c6f)
- backend/migrations/077_add_regime_tributario_to_companies.sql — FOUND (commit bb76c6f)
- backend/migrations/078_create_vw_xml_panels.sql — FOUND (commit d5cbe4c)

Critérios de sucesso atendidos:
- 5 arquivos .sql em backend/migrations/ com números 074-078
- Todos idempotentes (IF NOT EXISTS / CREATE OR REPLACE)
- Nenhum arquivo usa BEGIN/COMMIT explícito
- Coluna source em 3 tabelas com DEFAULT 'oracle_bridge' e CHECK constraint
- 2 tabelas de itens com UNIQUE (nfe_id, n_item) — 22 colunas cada
- Tabela xml_upload_batches com status e partial index para worker
- Campo regime_tributario em companies com 4 valores válidos
- 4 views vw_xml_* com CREATE OR REPLACE
