---
phase: 10-icms-fronteira-st-por-ncm
plan: "01"
subsystem: fullstack
tags: [icms-fronteira, substituicao-tributaria, ncm, bloco-c, go, react, retroactive]
retroactive: true
dependency_graph:
  requires: []
  provides:
    - "Classificação ST por NCM no Bloco C (FRST-01)"
    - "ST por Item inclui CFOP 6101/6102 com NCM de ST (FRST-02)"
    - "Remoção da tela de reclassificação manual (FRST-03)"
  affects:
    - backend/handlers/icms_fronteira_nao_sped.go
    - backend/handlers/icms_fronteira_reconciliacao.go
    - backend/handlers/icms_fronteira_st_itens.go
    - frontend/src/pages/IcmsFronteira.tsx
tech_stack:
  added: []
  patterns:
    - "Classificação NCM-first: DIFAL → ST(NCM+segmento) → ANTECIPACAO → NAO_FRONTEIRA"
    - "LATERAL join longest-prefix-wins em icms_fronteira_regras_ncm (prefixo >= 4 díg)"
    - "Guard de segmento: EXISTS em company_segmentos por (segmento_codigo, eff_uf)"
    - "eff_uf em 3 camadas: dest_uf XML → CNPJ match import_jobs → emp_uf fallback"
    - "class_status='ncm' marca NF reclassificada de antecipação para ST pelo NCM"
key_files:
  created: []
  modified:
    - backend/handlers/icms_fronteira_nao_sped.go
    - backend/handlers/icms_fronteira_reconciliacao.go
    - backend/handlers/icms_fronteira_st_itens.go
    - frontend/src/pages/IcmsFronteira.tsx
decisions:
  - "ST classificado pelo NCM, não pelo CFOP do fornecedor (orientação Gilson 2026-06-27)"
  - "Sem protocolo CONFAZ o fornecedor usa CFOP normal (6101/6102) e cabe à empresa recolher ST"
  - "Trava de segmento: só vira ST se a empresa tem o segmento da regra cadastrado para a UF"
  - "Tela de reclassificação manual removida — NCM virou regra automática do sistema"
  - "Override manual (icms_fronteira_classificacao_manual) mantido no SQL via COALESCE mas sem UI"
  - "Prefixo NCM amplo (ex.: 8482 com 4 díg) cobre todo o capítulo — volume alto é esperado e vem do cadastro de regras, não do código"
metrics:
  duration: "~1 sessão"
  completed_date: "2026-06-27"
  tasks_completed: 4
  tasks_total: 4
  files_created: 0
  files_modified: 4
requirements_satisfied: [FRST-01, FRST-02, FRST-03]
restore_tags:
  current: "st-ncm-v2.2.1 (com ST por NCM)"
  rollback: "st-cfop-only-restore (ST só por CFOP, estado anterior)"
---

# Phase 10 Plan 01: ICMS Fronteira — ST por NCM no Bloco C — Summary

**One-liner:** NFs em XML sem SPED passam a ser classificadas como ST pelo NCM (regra + segmento da empresa na UF) independentemente do CFOP do fornecedor, fechando o gap das notas 6101/6102 sem protocolo CONFAZ nas três views do Bloco C, com a tela de reclassificação manual removida.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | NCM-first no naoSpedQuery (aba Antecipação) | 670699b | icms_fronteira_nao_sped.go |
| 2 | NCM-first no reconXmlQuery (Reconciliação) | 670699b | icms_fronteira_reconciliacao.go |
| 4 | Remover tela de reclassificação manual | 670699b | frontend/src/pages/IcmsFronteira.tsx |
| 3 | Incluir 6101/6102 no ST por Item | 28574ea | icms_fronteira_st_itens.go |
| — | Log de debug temporário (NF 159027) | 7c67464 | icms_fronteira_nao_sped.go |

## What Was Built

### naoSpedQuery (icms_fronteira_nao_sped.go)

Classificação NCM-first em quatro pontos coordenados (SELECT regime, class_status,
icms_devido_est, WHERE):

- **regime**: `DIFAL (2551/2556)` → `ST se NCM casa regra + company_segmentos(eff_uf)`
  → `ANTECIPACAO (2403/2409/2651/2652/2101/2102/2152)` → `NAO_FRONTEIRA`.
- **class_status**: `'ncm'` quando CFOP 2101/2102/2152 mas NCM forçou ST; senão `'auto'`.
- **icms_devido_est**: ramo ST por NCM calcula com MVA (gross-up), incluindo CT-e na base.
- **LATERAL regra**: adicionado `r.segmento_codigo`, longest-prefix-wins, prefixo >= 4 díg.

### reconXmlQuery (icms_fronteira_reconciliacao.go)

Mesma precedência de classificação. Removido o `LEFT JOIN icms_fronteira_classificacao_manual`
e o filtro `<> 'excluded'` — não há mais override manual afetando a view. Texto do alerta
atualizado para explicar a classificação automática por NCM.

### xml_itens (icms_fronteira_st_itens.go)

A query do demonstrativo ST por Item aceitava só CFOP 2403/2409/2651/2652. Adicionado OR
para CFOP 2101/2102/2152 com `EXISTS` em `icms_fronteira_regras_ncm` + `company_segmentos`
(segmento da regra cadastrado para `dest_uf`), prefixo NCM >= 4 díg via
`LEFT(regexp_replace(ncm),...)`. Isso fechou o caso da NF 159027.

### IcmsFronteira.tsx (frontend)

`FaltandoBlockTable` simplificado para somente leitura: removidos Select de regime, botão
IA, validar/excluir/reset, modal IA e a interface `IASuggestion`. Badge "NCM→ST" exibido
na célula CFOP da tabela ST quando `row.class_status === 'ncm'`.

## Deviations from Plan

Plano é retroativo (escrito após a implementação para formalizar o trabalho que vinha
sendo rastreado fora da estrutura GSD). Sem desvios — o plano descreve o que foi entregue.

### Pendência conhecida

- **Log de debug temporário** (commit 7c67464) ainda presente no `naoSpedQuery` para
  rastrear a NF 159027 em PRD. Remover após validação da Rolimec.

## Verification

```
cd backend && go build ./...   # EXIT 0
# Diagnóstico PRD confirmou classificação correta:
#   NF 159027 CFOP 6101 → NCM 7318 (seg 104, MVA 55%) → ST
#                         NCM 8482 (seg 112, MVA 71.78%) → ST
# Regras NCM verificadas (BA): 7318(4d/seg104), 8482(4d/seg112),
#   8483(4d/seg112), 40103(5d), 4016x(8d) — todas com segmento cadastrado.
```

## Pontos de Restauração (git tags)

| Tag | Commit | Estado |
|-----|--------|--------|
| `st-ncm-v2.2.1` | 28574ea | **Atual** — ST por NCM ativo |
| `st-cfop-only-restore` | 7c67464 | Rollback — ST só por CFOP (anterior) |

Rollback seletivo do ST por item:
`git checkout st-cfop-only-restore -- backend/handlers/icms_fronteira_st_itens.go`

## Known Stubs

Nenhum stub. Todas as queries retornam dados reais do banco.

## Self-Check: PASSED

- `backend/handlers/icms_fronteira_nao_sped.go`: MODIFIED (NCM-first + debug log)
- `backend/handlers/icms_fronteira_reconciliacao.go`: MODIFIED (NCM-first)
- `backend/handlers/icms_fronteira_st_itens.go`: MODIFIED (CFOP 2101/2102/2152)
- `frontend/src/pages/IcmsFronteira.tsx`: MODIFIED (tela manual removida + badge)
- Commits 670699b, 7c67464, 28574ea: FOUND
- Tags st-ncm-v2.2.1, st-cfop-only-restore: FOUND
