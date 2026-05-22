---
phase: 06-infraestrutura-reforma-tribut-ria
plan: 02
subsystem: backend-parsers
tags: [go, worker, nfe-saidas, efd-c190, reforma-tributaria, RFMA-01, RFMA-03]
dependency_graph:
  requires: ["06-01"]
  provides: ["RFMA-01-parser", "RFMA-03-parser"]
  affects: ["reg_c190", "nfe_saidas"]
tech_stack:
  added: []
  patterns: ["prepared-statement-extension", "nullable-smallint-helper", "xml-struct-field"]
key_files:
  modified:
    - backend/worker/worker.go
    - backend/handlers/nfe_saidas.go
decisions:
  - "toNullSmallInt retorna nil/0/1 via interface{}: compatível com driver pq para SMALLINT nullable sem dependência extra"
  - "ind_final adicionado como $43 no INSERT nfe_saidas; source permanece literal 'xml_upload' sem placeholder"
  - "Guard C190 alterado de >= 12 para >= 13 (segurança: acesso até parts[12]=cod_obs + parts[4]=aliq_icms)"
metrics:
  duration: "<5 minutos"
  completed_date: "2026-05-22T21:49:56Z"
  tasks_completed: 2
  tasks_total: 2
  files_modified: 2
---

# Phase 06 Plan 02: Parsers EFD C190 + XML NF-e ind_final Summary

Parser EFD C190 estendido para gravar `cst_icms`/`aliq_icms` e parser XML NF-e de saída estendido para ler `ide/indFinal` e persistir em `ind_final` — ambas as colunas criadas na plan 06-01.

## Tasks Completed

| # | Task | Commit | Files |
|---|------|--------|-------|
| 1 | worker.go — popular cst_icms e aliq_icms no C190 (RFMA-01) | 9a7f9ca | backend/worker/worker.go |
| 2 | nfe_saidas.go — ler e persistir ind_final do XML (RFMA-03) | 5d0f4c3 | backend/handlers/nfe_saidas.go |

## What Was Built

### Task 1: worker.go — C190 com cst_icms e aliq_icms

Três modificações cirúrgicas em `backend/worker/worker.go`:

1. **stmtC190 Prepare** (linha ~494): INSERT estendido de 11 para 13 colunas, adicionando `cst_icms, aliq_icms` com `$12, $13` em VALUES.

2. **Guard case C190** (linha ~740): `len(parts) >= 12` alterado para `>= 13` — segurança para acesso a `parts[12]` (cod_obs) e `parts[4]` (aliq_icms). Mitiga T-06-05 (DoS por linha malformada).

3. **stmtC190.Exec** (linha ~752): adicionados `parts[2]` (CST_ICMS, string) e `parseDecimal(parts[4])` (ALIQ_ICMS) como 12º e 13º argumentos. Count de placeholders == count de argumentos (13).

Layout EFD C190 confirmado: `parts[2]=CST_ICMS`, `parts[3]=CFOP`, `parts[4]=ALIQ_ICMS`, ..., `parts[12]=COD_OBS`.

### Task 2: nfe_saidas.go — ind_final do XML NF-e de saída

Quatro modificações em `backend/handlers/nfe_saidas.go`:

1. **struct `ide`** (linha ~119): adicionado `IndFinal string \`xml:"indFinal"\`` com comentário semântico ("0"=B2B/normal, "1"=consumidor final, ""=NF-e antiga).

2. **INSERT INTO nfe_saidas** (~linha 570): adicionados `ind_final` à lista de colunas (após `source`) e `$43` à lista VALUES (após literal `'xml_upload'`).

3. **ON CONFLICT DO UPDATE SET** (~linha 620): adicionado `ind_final = EXCLUDED.ind_final` — reimportações de XMLs atualizam o campo corretamente (Pitfall 3 mitigado).

4. **QueryRow Exec** (~linha 630): adicionado `toNullSmallInt(inf.Ide.IndFinal)` como 43º argumento.

**Helper criado:** `toNullSmallInt(s string) interface{}` — converte `""` → `nil` (NULL, D-09), `"1"` → `1` (consumidor final), qualquer outro → `0` (B2B, fallback seguro T-06-07).

## Verification

- `go build ./...` no diretório backend: **PASSOU** (sem erros de compilação)
- `go build ./worker/`: OK
- `go build ./handlers/`: OK
- Placeholders $13 em stmtC190 == 13 argumentos no Exec: **CONFIRMADO**
- Placeholders $43 no INSERT nfe_saidas == 43 argumentos no QueryRow: **CONFIRMADO**
- Nenhuma deleção acidental de arquivos rastreados

## Deviations from Plan

Nenhuma — plano executado exatamente como escrito.

## Threat Model Coverage

| Threat ID | Mitigação Aplicada |
|-----------|-------------------|
| T-06-04 | parts[2]/parts[4] gravados via $12/$13 parametrizados — SQL injection impossível |
| T-06-05 | Guard `len(parts) >= 13` evita panic antes de acessar parts[12] |
| T-06-06 | inf.Ide.IndFinal gravado via $43 parametrizado |
| T-06-07 | toNullSmallInt normaliza valores inesperados para 0 (fallback B2B) |

## Known Stubs

Nenhum. As colunas ficam NULL para registros históricos (D-09) — comportamento intencional documentado no RESEARCH.md.

## Threat Flags

Nenhum novo surface de segurança introduzido além do já modelado no threat register do plano.

## Self-Check: PASSED

- [x] backend/worker/worker.go modificado e commitado (9a7f9ca)
- [x] backend/handlers/nfe_saidas.go modificado e commitado (5d0f4c3)
- [x] `go build ./...` passa sem erros
- [x] RFMA-01 parser: worker.go popula cst_icms/aliq_icms em novas importações EFD
- [x] RFMA-03 parser: nfe_saidas.go popula ind_final em novas importações XML
