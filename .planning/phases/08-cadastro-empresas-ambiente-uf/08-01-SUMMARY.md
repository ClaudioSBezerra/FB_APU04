---
phase: 08-cadastro-empresas-ambiente-uf
plan: "01"
subsystem: database-migrations
tags: [migrations, schema, icms-fronteira, companies, postgresql]
dependency_graph:
  requires: []
  provides:
    - "schema: companies com CNPJ + 6 campos de cadastro mestre (nullable)"
    - "schema: icms_fronteira_regras_ncm com uf_estado + MVA ajustado"
    - "schema: icms_fronteira_inaplicabilidades (nova tabela)"
    - "seed: 14 regras NCM para BA e CE (7 por UF)"
  affects:
    - backend/handlers/environment.go (plano 08-02)
    - backend/handlers/icms_fronteira_regras.go (plano 08-02)
    - frontend/src/pages/GestaoAmbiente.tsx (plano 08-03)
    - frontend/src/pages/IcmsFronteira.tsx (plano 08-03)
tech_stack:
  added: []
  patterns:
    - "ALTER TABLE ... ADD COLUMN IF NOT EXISTS (idempotente)"
    - "DROP CONSTRAINT IF EXISTS + DO $$ EXCEPTION WHEN duplicate_object (idempotente)"
    - "CREATE TABLE IF NOT EXISTS / CREATE INDEX IF NOT EXISTS"
    - "INSERT ... ON CONFLICT DO NOTHING (seed idempotente)"
key_files:
  created:
    - backend/migrations/096_add_fields_to_companies.sql
    - backend/migrations/097_add_uf_estado_to_fronteira_regras.sql
    - backend/migrations/098_seed_ba_ce_fronteira.sql
  modified: []
decisions:
  - "cnpj VARCHAR(18) sem UNIQUE: multi-filial pode compartilhar CNPJs relacionados (CADU-01)"
  - "uf_estado DEFAULT 'PE' em 097: preserva registros existentes da migration 091 (seed PE)"
  - "mva_ajustado_* NULL no seed 098: preenchimento via UI/planilha nos planos 02/03"
  - "Seed 098 com valores [ASSUMED]: revisão obrigatória contra RICMS/BA e RICMS/CE antes de produção"
metrics:
  duration: "~15 minutos"
  completed: "2026-05-23"
  tasks_completed: 3
  tasks_total: 3
  files_created: 3
  files_modified: 0
---

# Phase 08 Plan 01: Migrations 096/097/098 — Schema de Cadastro Mestre e Fronteira Multi-UF

**One-liner:** 3 migrations SQL idempotentes que re-adicionam CNPJ + 6 campos de cadastro mestre em `companies`, expandem `icms_fronteira_regras_ncm` com `uf_estado` + MVA ajustado (4/7/12%) e criam `icms_fronteira_inaplicabilidades`, e inserem 14 regras NCM seed (7 BA + 7 CE).

## Tasks Executadas

| Task | Nome | Commit | Arquivos |
|------|------|--------|----------|
| 1 | Migration 096 — re-adicionar CNPJ + 6 campos em companies | b47a09f | backend/migrations/096_add_fields_to_companies.sql |
| 2 | Migration 097 — uf_estado + MVA ajustado + tabela inaplicabilidades | fe0697b | backend/migrations/097_add_uf_estado_to_fronteira_regras.sql |
| 3 | Migration 098 — seed inicial BA e CE | be5af10 | backend/migrations/098_seed_ba_ce_fronteira.sql |

## O Que Foi Construído

### Migration 096 — companies: 7 campos de cadastro mestre (CADU-01)

Adiciona via `ADD COLUMN IF NOT EXISTS` os campos:
- `cnpj VARCHAR(18)` — re-adicionado (removido na migration 023) sem UNIQUE, nullable
- `inscricao_estadual VARCHAR(30)` — nullable
- `cnae_principal VARCHAR(7)` — nullable
- `cnae_secundario TEXT[]` — array nativo PostgreSQL, nullable
- `municipio VARCHAR(100)` — nullable
- `segmento_economico VARCHAR(100)` — nullable
- `incentivos_fiscais JSONB` — schema livre, nullable

7 `COMMENT ON COLUMN` descritivos em PT-BR. Nenhuma constraint nova além das existentes.

### Migration 097 — icms_fronteira_regras_ncm: uf_estado + MVA + inaplicabilidades (CADU-04)

- `uf_estado VARCHAR(2) NOT NULL DEFAULT 'PE'` — registros existentes ficam com valor PE automaticamente
- `mva_ajustado_4pct`, `mva_ajustado_7pct`, `mva_ajustado_12pct` — todos `NUMERIC(8,4)` nullable
- `DROP CONSTRAINT IF EXISTS uq_icms_fronteira_regras` + nova constraint `uq_icms_fronteira_regras_uf` com `UNIQUE NULLS NOT DISTINCT (company_id, ncm_prefixo, uf_estado)`
- Nova tabela `icms_fronteira_inaplicabilidades` com índice composto `(uf_estado, ncm_digits)`

### Migration 098 — seed inicial BA e CE (CADU-05)

14 regras NCM globais (company_id IS NULL), 7 por UF:

| NCM | Descrição | BA aliquota_interna | CE aliquota_interna |
|-----|-----------|---------------------|---------------------|
| 2202 | Refrigerantes | 26% | 25% |
| 2203 | Cervejas | 26% | 25% |
| 3004 | Medicamentos humanos | 12% | 12% |
| 3303 | Cosméticos/higiene | 20,5% | 20,5% |
| 4011 | Pneumáticos novos | 20,5% | 20,5% |
| 2523 | Cimento | 17,5% | 17% |
| 8517 | Aparelhos telefônicos | 20,5% | 20,5% |

Todos com `ON CONFLICT DO NOTHING`. `mva_ajustado_*` permanecem NULL.

## Deviations from Plan

None - plan executed exactly as written.

## Known Stubs

Nenhum. As migrations são DDL/DML puros — não há stubs de UI ou dados falsos que fluam para renderização.

**Nota sobre valores [ASSUMED] no seed 098:** Os valores de `aliquota_interna` e `mva_original` para BA e CE são baseados em legislação estadual conhecida mas marcados como `[ASSUMED]` no RESEARCH. O cabeçalho da migration 098 documenta explicitamente esta situação e a necessidade de revisão contra RICMS/BA e RICMS/CE antes de produção. Isso não é um "stub" — é uma flag de risco fiscal documentada.

## Threat Surface Scan

Nenhuma nova superfície de rede ou autenticação introduzida. Todas as 3 migrations são DDL/seed executadas pelo migration runner com acesso admin ao PostgreSQL — dentro da trust boundary já existente `migration runner → PostgreSQL`. Ameaças mapeadas no threat model do plano (T-08-01 a T-08-SC) foram endereçadas conforme especificado.

## Self-Check: PASSED

- [x] `backend/migrations/096_add_fields_to_companies.sql` existe — confirmado
- [x] `backend/migrations/097_add_uf_estado_to_fronteira_regras.sql` existe — confirmado
- [x] `backend/migrations/098_seed_ba_ce_fronteira.sql` existe — confirmado
- [x] Commit b47a09f existe — confirmado (Task 1)
- [x] Commit fe0697b existe — confirmado (Task 2)
- [x] Commit be5af10 existe — confirmado (Task 3)
