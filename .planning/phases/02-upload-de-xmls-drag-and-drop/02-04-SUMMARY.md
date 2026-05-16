---
phase: 02-upload-de-xmls-drag-and-drop
plan: "04"
subsystem: backend-xml-reports
tags: [go, react, typescript, postgresql, csv, cclasstrib, pis, cofins, relatorio, saneamento, nfe]

dependency_graph:
  requires:
    - phase: 02-upload-de-xmls-drag-and-drop
      plan: "02"
      provides:
        - schema/nfe_entradas_itens
        - handler/XMLUploadHandler
    - phase: 02-upload-de-xmls-drag-and-drop
      plan: "03"
      provides:
        - page/painel/xmls
        - frontend/src/lib/navigation.ts (com entradas 02-03 preservadas)
        - frontend/src/App.tsx (com rotas 02-03 preservadas)
  provides:
    - api/xml/reports/saneamento (GET)
    - api/xml/reports/saneamento/csv (GET)
    - api/xml/reports/fornecedores-cclasstrib (GET)
    - page/relatorios/saneamento-cclasstrib
  affects:
    - phase-03 (relatórios de apuração usam a mesma base nfe_entradas_itens)

tech-stack:
  added:
    - encoding/csv (stdlib Go — geração de CSV sem biblioteca externa)
  patterns:
    - "Factory func Handler(db *sql.DB) http.HandlerFunc — sem CORS manual, tratado pelo SecurityMiddleware"
    - "executeSaneamentoQuery reutilizada por handler JSON e CSV — DRY"
    - "parsePgArray helper converte {a,b,c} PostgreSQL array literal em []string sem lib extra"
    - "Download CSV via Blob+URL.createObjectURL — window.fetch interceptado pelo AuthContext injeta headers"
    - "Registrar /csv antes de /saneamento no mux stdlib Go (mais específico primeiro)"

key-files:
  created:
    - backend/handlers/xml_reports.go
    - backend/migrations/079_seed_ncm_cclasstrib_reforma.sql
    - frontend/src/pages/RelatorioSaneamento.tsx
  modified:
    - backend/main.go
    - frontend/src/lib/navigation.ts
    - frontend/src/App.tsx

key-decisions:
  - "executeSaneamentoQuery extraída como helper reutilizado por JSON e CSV — evita duplicação da query longa"
  - "parsePgArray converte array literal PostgreSQL sem driver lib extra — simples e sem dependência"
  - "getActiveModule: /relatorios/saneamento mapeado para 'notas' com regra específica antes da genérica 'simulador'"
  - "Sugestão CCLASSTRIB populada automaticamente via LEFT JOIN LATERAL com ncm_cclasstrib_reforma (maior prefixo ncm_digits que faz match) — 95 NCMs da Reforma Tributária semeados na migration 079"
  - "Prefixo match: item.ncm LIKE ref.ncm_digits || '%' ORDER BY length(ncm_digits) DESC LIMIT 1 — captura códigos de capítulo (ex: 0201) e posições mais específicas (ex: 02061000)"

requirements-completed: [XML-03]

duration: ~20min
completed: "2026-05-16"
---

# Phase 02 Plan 04: Relatórios de Saneamento CCLASSTRIB — Summary

**3 endpoints Go de análise tributária (divergência CST por NCM, export CSV com coluna de sugestão para reimportação, fornecedores com CCLASSTRIB ausente) + página React com painel de resumo, tabelas priorizadas por valor e download CSV direto**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-05-16T15:36:00Z
- **Completed:** 2026-05-16T15:56:00Z
- **Tasks:** 2
- **Files modified:** 5 (2 criados, 3 modificados)

## Accomplishments

- Criou `backend/handlers/xml_reports.go` com 3 handlers usando query parametrizada sobre `nfe_entradas_itens` com filtro por `company_id` (GetEffectiveCompanyID) e filtro opcional `mes_ano`
- CSV exportado com cabeçalho PT-BR e coluna "Sugestão CCLASSTRIB" vazia para preenchimento manual pelo contador e reimportação ao sistema (D-16b)
- Relatório de fornecedores JOIN `nfe_entradas` para obter CNPJ/nome, ordenado por `v_pis_cofins_total DESC` — prioriza saneamento pelo impacto financeiro
- Página `RelatorioSaneamento.tsx` com painel de 3 cards de resumo, tabela NCMs com badge vermelho para CCLASSTRIB ausente, tabela fornecedores e mensagem informativa quando não há divergências
- Rotas e navegação adicionadas de forma cirúrgica (append only) — rotas e tabs do 02-03 preservadas intactas

## Rotas Backend Registradas

| Rota | Handler | Descrição |
|------|---------|-----------|
| `GET /api/xml/reports/saneamento/csv` | `XMLSaneamentoCSVHandler` | Export CSV saneamento (D-16b) |
| `GET /api/xml/reports/saneamento` | `XMLSaneamentoCCLASSTRIBHandler` | NCMs com divergência CST/CCLASSTRIB (D-16a) |
| `GET /api/xml/reports/fornecedores-cclasstrib` | `XMLFornecedoresCCLASSTRIBHandler` | Fornecedores com CCLASSTRIB ausente/inconsistente (D-16c) |

**Ordem de registro:** `/csv` antes de `/saneamento` — mux stdlib Go faz match pelo path mais longo primeiro.

## Rota Frontend Adicionada

| Rota | Componente | Proteção |
|------|-----------|---------|
| `/relatorios/saneamento-cclasstrib` | `RelatorioSaneamento` | ProtectedRoute (via AppLayout) |

Aba adicionada no módulo `notas`: `Saneamento CCLASSTRIB → /relatorios/saneamento-cclasstrib`

## Estrutura do CSV Gerado (campos e ordem)

| # | Campo | Fonte |
|---|-------|-------|
| 1 | NCM | `ei.ncm` |
| 2 | Variantes CST PIS | `COUNT(DISTINCT cst_pis)` |
| 3 | Variantes CST COFINS | `COUNT(DISTINCT cst_cofins)` |
| 4 | Variantes CCLASSTRIB | `COUNT(DISTINCT cclasstrib) FILTER (WHERE NOT NULL)` |
| 5 | CCLASSTRIB Ausente | `BOOL_OR(cclasstrib IS NULL)` → "Sim"/"Não" |
| 6 | Qtd Itens | `COUNT(*)` |
| 7 | V. PIS Total | `SUM(v_pis)` |
| 8 | V. COFINS Total | `SUM(v_cofins)` |
| 9 | CSTs PIS Encontrados | `array_agg(DISTINCT cst_pis)` separado por "; " |
| 10 | CSTs COFINS Encontrados | `array_agg(DISTINCT cst_cofins)` separado por "; " |
| 11 | Sugestão CCLASSTRIB | `ncm_cclasstrib_reforma.cclasstrib` via LEFT JOIN LATERAL (preenchida automaticamente quando NCM está na tabela de referência da Reforma Tributária) |
| 12 | Descrição Reforma | `ncm_cclasstrib_reforma.descricao` |
| 13 | Redução IBS (%) | `ncm_cclasstrib_reforma.ibs_reducao_pct` |
| 14 | Redução CBS (%) | `ncm_cclasstrib_reforma.cbs_reducao_pct` |
| 15 | Anexo Reforma | `ncm_cclasstrib_reforma.anexo` (Anexo I/V/VIII/IX/XIII) |

## Performance das Queries (estimativa)

Com índice em `nfe_entradas_itens(company_id)`:
- Query saneamento: GROUP BY ncm com HAVING — O(n) onde n = itens da empresa; LIMIT 500 previne scan completo em tabelas grandes
- Query fornecedores: JOIN com nfe_entradas + GROUP BY (forn_cnpj, forn_nome, ncm) — índice em `nfe_entradas.id` (PK) garante O(itens × log(notas)); LIMIT 200
- Volume típico (empresa com 1000 XMLs/mês): resposta estimada em < 100ms

## Phase 2 Completa — Resumo de Requisitos XML-01 a XML-08

| Requisito | Descrição | Plano | Status |
|-----------|-----------|-------|--------|
| XML-01 | Upload de XMLs NF-e via drag-and-drop (.xml e .zip) | 02-02, 02-03 | Completo |
| XML-02 | Processamento assíncrono para batches > 50 XMLs com polling de status | 02-02, 02-03 | Completo |
| XML-03 | Relatórios de saneamento CCLASSTRIB com exportação CSV | 02-04 | **Completo** |
| XML-04 | (não especificado no plano) | — | — |
| XML-05 | Painel de XMLs importados com 3 abas (Entradas/Saídas/CT-es) | 02-03 | Completo |
| XML-06 | (não especificado no plano) | — | — |
| XML-07 | Prioridade XML > Oracle: upsert preserva dados de XML quando Oracle tenta sobrescrever | 02-02 | Completo |
| XML-08 | Regime tributário (Simples/Lucro Real/Presumido) no cadastro de empresa | 02-03 | Completo |

## Task Commits

1. **Task 1: Backend — 3 endpoints relatório saneamento CCLASSTRIB** — `7105e66` (feat)
2. **Task 2: Frontend — RelatorioSaneamento + navegação** — `04a390f` (feat)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] parsePgArray criado como helper local em vez de Scan interface**
- **Found during:** Task 1 (compilação inicial)
- **Issue:** A implementação original usava um tipo `pgStringArrayScanner` com método `Scan()` e uma função fábrica `pgStringArray(*[]string)`, mas havia erro de redeclaração e tipo incompatível com a interface `sql.Scanner`. A tentativa de usar `&cstsPis` (type `*[]string`) como argumento da fábrica causou erro de compilação.
- **Fix:** Substituído por `parsePgArray(interface{}) []string` — função simples que recebe o valor bruto escaneado em `interface{}` e converte o literal `{a,b,c}` do PostgreSQL para `[]string`. A query usa `Scan(&rawCstsPis)` e chama `parsePgArray` depois.
- **Files modified:** backend/handlers/xml_reports.go
- **Verification:** `go build ./...` passou sem erros
- **Committed in:** 7105e66 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 — bug de compilação na abordagem de scan de array)
**Impact on plan:** Correção necessária para compilação. A funcionalidade do relatório é idêntica à especificada. Sem scope creep.

## Mitigações de Segurança (Threat Model)

| Threat ID | Status | Implementação |
|-----------|--------|---------------|
| T-02-04-01 | Mitigado | `GetEffectiveCompanyID` valida user↔company antes de qualquer query — dados de outra empresa nunca expostos |
| T-02-04-02 | Mitigado | `mes_ano` passado como `$2` paramétrico — nunca concatenado no SQL |
| T-02-04-03 | Aceito | `LIMIT 500` aplicado na query principal; `LIMIT 200` no relatório de fornecedores |
| T-02-04-04 | Aceito | Volume máximo ~500 NCMs × 11 campos ≈ ~50KB; sem risco de DoS |

## Known Stubs

Nenhum stub identificado — todos os endpoints consomem dados reais de `nfe_entradas_itens`.

## Threat Flags

Nenhuma nova superfície de segurança além das especificadas no threat model do plano.

## Self-Check: PASSED

Arquivos criados:
- `backend/handlers/xml_reports.go` — FOUND (commit 7105e66)
- `frontend/src/pages/RelatorioSaneamento.tsx` — FOUND (commit 04a390f)

Rotas main.go:
- `XMLSaneamentoCSVHandler` — FOUND linha 569
- `XMLSaneamentoCCLASSTRIBHandler` — FOUND linha 570
- `XMLFornecedoresCCLASSTRIBHandler` — FOUND linha 571

Frontend:
- `RelatorioSaneamento` importado em App.tsx — FOUND linha 22
- Rota `/relatorios/saneamento-cclasstrib` em App.tsx — FOUND linha 164
- Aba "Saneamento CCLASSTRIB" em navigation.ts — FOUND linha 36
- `getActiveModule` para `/relatorios/saneamento` → `notas` — FOUND linha 61

Build Go: `go build ./...` — PASSOU sem erros
Build TypeScript: `npm run build` — PASSOU (vite build em 7.75s)

## Next Phase Readiness

- Phase 02 completamente concluída (Plans 01+02+03+04)
- Schema XML (migrations 074-078), handlers Go, frontend React e relatórios de saneamento prontos
- Próximo: Phase 03 — apuração fiscal e relatórios baseados nos dados XML e ERP importados
- Base `nfe_entradas_itens` com CCLASSTRIB disponível para apuração diferenciada por regime tributário

---
*Phase: 02-upload-de-xmls-drag-and-drop*
*Completed: 2026-05-16*
