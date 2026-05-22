---
phase: 06-infraestrutura-reforma-tribut-ria
plan: 01
subsystem: database-schema
tags: [migration, postgresql, reforma-tributaria, ddl, schema]
dependency_graph:
  requires: []
  provides:
    - reg_c190.cst_icms (VARCHAR 3, nullable)
    - reg_c190.aliq_icms (NUMERIC 6.2, nullable)
    - tabela reforma_parametros (PK company_id, 6 campos configuráveis)
    - nfe_saidas.ind_final (SMALLINT, nullable)
    - cfop 1151/1152/2151/2152/5151/5152/6151/6152 com tipo=T
  affects:
    - backend/migrations/
    - Phase 7 (módulos analíticos)
    - Phase 8 (mapa UF + segmentação B2B/B2C)
tech_stack:
  added: []
  patterns:
    - ADD COLUMN IF NOT EXISTS para idempotência de colunas
    - CREATE TABLE IF NOT EXISTS para idempotência de tabelas
    - ON CONFLICT DO UPDATE para seed idempotente com correção de valor
key_files:
  created:
    - backend/migrations/086_add_cst_aliq_icms_to_reg_c190.sql
    - backend/migrations/087_create_reforma_parametros.sql
    - backend/migrations/088_add_ind_final_to_nfe_saidas.sql
    - backend/migrations/089_seed_cfop_transferencias.sql
  modified: []
decisions:
  - "Colunas 086/088 nullable por D-09: histórico fica NULL sem necessidade de backfill"
  - "ON DELETE CASCADE em reforma_parametros: parâmetros são dependentes da empresa — exclusão em cascata é o comportamento correto"
  - "ON CONFLICT DO UPDATE em 089: garante correção de CFOPs com tipo errado pré-existente (Pitfall 4)"
  - "fator_simples_pct DEFAULT 20.00: estimativa pendente publicação CG-IBS; UI deve exibir disclaimer RFMA-07"
metrics:
  duration: "2m"
  completed_date: "2026-05-22"
  tasks_completed: 3
  files_created: 4
  files_modified: 0
---

# Phase 06 Plan 01: Schema Foundation Reforma Tributária — Summary

## One-liner

Quatro migrations PostgreSQL (086–089) criam a fundação de schema para análise da Reforma Tributária: colunas `cst_icms`/`aliq_icms` em `reg_c190`, tabela `reforma_parametros` com 6 parâmetros configuráveis por empresa, coluna `ind_final` em `nfe_saidas` e seed de 8 CFOPs de transferência com `tipo='T'`.

## What Was Built

| Task | Description | Commit | Files |
|------|-------------|--------|-------|
| 1 | Migrations 086 + 088: ADD COLUMN em reg_c190 e nfe_saidas | 97efd1e | 086, 088 |
| 2 | Migration 087: CREATE TABLE reforma_parametros | c38197c | 087 |
| 3 | Migration 089: Seed 8 CFOPs de transferência tipo=T | a2a3710 | 089 |

## Success Criteria Verification

- RFMA-01 (DDL): `reg_c190` ganhou `cst_icms VARCHAR(3)` + `aliq_icms NUMERIC(6,2)` — idempotente via `IF NOT EXISTS`
- RFMA-02: tabela `reforma_parametros` criada com `company_id UUID PK FK → companies(id) ON DELETE CASCADE`, 6 campos nos tipos exatos, `fator_simples_pct DEFAULT 20.00`, timestamps
- RFMA-03 (DDL): `nfe_saidas` ganhou `ind_final SMALLINT` — idempotente via `IF NOT EXISTS`
- RFMA-04: 8 CFOPs de transferência com `tipo='T'` via `ON CONFLICT DO UPDATE` (corrige valores errados)

## Deviations from Plan

None — plano executado exatamente como especificado.

## Known Stubs

None — este plano cria apenas DDL de schema; nenhum dado de UI ou lógica de negócio envolvida.

## Threat Flags

None — nenhuma nova superfície de rede ou endpoint introduzida. Migrations aplicam-se somente pelo migration runner interno (PostgreSQL), conforme trust boundary T-06-01 do plano.

## Self-Check: PASSED

- [x] `backend/migrations/086_add_cst_aliq_icms_to_reg_c190.sql` existe
- [x] `backend/migrations/087_create_reforma_parametros.sql` existe
- [x] `backend/migrations/088_add_ind_final_to_nfe_saidas.sql` existe
- [x] `backend/migrations/089_seed_cfop_transferencias.sql` existe
- [x] Commits 97efd1e, c38197c, a2a3710 presentes no git log
