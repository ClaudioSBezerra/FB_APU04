---
phase: 04-concilia-o-bridge-vs-xml
plan: "01"
subsystem: backend
tags: [go, handlers, conciliacao, xml, bridge, fiscal, csv, api]
dependency_graph:
  requires:
    - "02-02: XMLUploadHandler (xml_upload source pattern)"
    - "02-04: xml_reports.go (handler factory pattern, CSV pattern)"
  provides:
    - "ConciliacaoHandler: GET /api/xml/conciliacao"
    - "CoberturaHandler: GET /api/xml/cobertura"
    - "ConciliacaoCSVHandler: GET /api/xml/conciliacao/csv"
  affects:
    - "04-02: ConciliacaoBridgeXML.tsx consumirá os três endpoints"
tech_stack:
  added: []
  patterns:
    - "Handler factory pattern: func Handler(db *sql.DB) http.HandlerFunc"
    - "executeConciliacaoQuery helper: tabela como parâmetro whitelist-validado"
    - "mes_ano como $2 parametrizado; tabela via fmt.Sprintf (não user input)"
    - "Filtro anti-divergência-falsa: (pis+cofins+icms)>0"
    - "Threshold R$ 0,01: ABS(xml-bridge) > 0.01"
key_files:
  created:
    - backend/handlers/xml_conciliacao.go
  modified:
    - backend/main.go
decisions:
  - "Tabela whitelist inline: tabela := 'nfe_entradas'; if tipo == 'saidas' { tabela = 'nfe_saidas' } — nenhum outro valor aceito"
  - "mes_ano como $2 parametrizado (nunca interpolado) — protege contra SQL injection"
  - "Filtro (COALESCE(pis,0)+COALESCE(cofins,0)+COALESCE(icms,0))>0 — evita divergência falsa para notas sem dados Bridge"
  - "Threshold ABS > 0.01 (R$ 0,01) — elimina ruído de arredondamento entre ERP e SEFAZ"
  - "LIMIT 500 na divergência, LIMIT 24 na cobertura — proteção DoS, padrão do projeto"
  - "/csv registrado antes de /conciliacao no mux stdlib — longest-prefix match do Go"
metrics:
  duration_minutes: 15
  completed_date: "2026-05-16"
  tasks_completed: 2
  tasks_total: 2
  files_created: 1
  files_modified: 1
---

# Phase 04 Plan 01: Conciliação Bridge vs XML — Backend Summary

**One-liner:** Handler Go de conciliação intra-row com 3 endpoints (JSON divergências, JSON cobertura, CSV auditor) usando filtros source/cancelado/threshold para precisão fiscal.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Criar xml_conciliacao.go | 32ca6d6 | backend/handlers/xml_conciliacao.go (criado) |
| 2 | Registrar rotas em main.go | dccb780 | backend/main.go (modificado) |

## What Was Built

### backend/handlers/xml_conciliacao.go (384 linhas)

Arquivo novo com 3 handlers públicos e 2 query helpers:

**Structs:**
- `conciliacaoRow` — 20 campos JSON: chave_nfe, forn_cnpj, forn_nome, mes_ano, data_emissao, cfop, xml_pis/cofins/icms/ipi/v_nf, bridge_pis/cofins/icms/ipi, delta_pis/cofins/icms/ipi, delta_total
- `coberturaRow` — 5 campos: mes_ano, total_nfes, com_xml, so_bridge, pct_xml

**Query helpers:**
- `executeConciliacaoQuery(db, companyID, mesAno, tabela)` — query de divergências com todos os filtros de segurança
- `executeCoberturaQuery(db, companyID, tabela)` — cobertura XML por mês com COUNT FILTER

**Handlers:**
- `ConciliacaoHandler(db)` — GET /api/xml/conciliacao, parâmetros mes_ano + tipo
- `CoberturaHandler(db)` — GET /api/xml/cobertura, parâmetro tipo
- `ConciliacaoCSVHandler(db)` — GET /api/xml/conciliacao/csv, download com cabeçalho PT-BR 19 colunas

### backend/main.go (5 linhas adicionadas)

Três rotas inseridas após o bloco `/api/xml/reports/`, antes de `/api/nfe-saidas/`:
```
/api/xml/conciliacao/csv  (registrado ANTES — longest-prefix match)
/api/xml/conciliacao
/api/xml/cobertura
```

## Security Controls Implemented

Todos os controles do threat model T-04-01-01 a T-04-01-06 foram implementados:

| Threat ID | Controle |
|-----------|---------|
| T-04-01-01 (Tampering: tipo → tabela) | Whitelist explícita nfe_entradas/nfe_saidas |
| T-04-01-02 (Tampering: mes_ano → SQL) | Parametrizado como $2, nunca interpolado |
| T-04-01-03 (IDOR: ver dados de outra empresa) | GetEffectiveCompanyID + WHERE company_id=$1 |
| T-04-01-04 (DoS: query sem LIMIT) | LIMIT 500 (divergência) + LIMIT 24 (cobertura) |
| T-04-01-05 (CSV de outra empresa) | GetEffectiveCompanyID idêntico ao JSON handler |
| T-04-01-06 (Spoofing: sem JWT) | ClaimsKey assert + jsonErr 401 antes de qualquer acesso DB |

## Verification Results

```
go build ./handlers/... → OK (handlers compilam sem erros)
go build .              → OK (main.go compila com novas rotas)
func ConciliacaoHandler: linha 199
func CoberturaHandler:   linha 254
func ConciliacaoCSVHandler: linha 308
source = 'xml_upload':  3 ocorrências (query conciliação + query cobertura + comentário)
cancelado != 'S':       2 ocorrências (uma por query)
> 0.01:                 3 ocorrências (threshold em 3 condições do filtro de divergência)
LIMIT 500:              1 ocorrência
LIMIT 24:               1 ocorrência
text/csv; charset=utf-8: 1 ocorrência
Delta Total:            1 ocorrência (último header CSV)
Access-Control-Allow-Origin: 0 (sem CORS headers — correto)
/csv na linha 574, /conciliacao na linha 575 (ordenação correta)
```

## Deviations from Plan

None — plan executed exactly as written.

## Known Stubs

None. Os handlers retornam dados reais do banco de dados. Nenhum placeholder ou mock data.

## Threat Flags

Nenhuma nova superfície de ataque além das documentadas no threat model do plano.

## Self-Check: PASSED

- [x] backend/handlers/xml_conciliacao.go existe (criado, 384 linhas)
- [x] backend/main.go modificado com 3 novas rotas
- [x] Commit 32ca6d6 existe (Task 1)
- [x] Commit dccb780 existe (Task 2)
- [x] go build ./handlers/... passa
- [x] go build . passa
- [x] Todos os filtros de segurança presentes nas queries
- [x] /csv registrado antes de /conciliacao em main.go
