---
phase: 04-concilia-o-bridge-vs-xml
plan: "02"
subsystem: frontend
tags: [react, tsx, conciliacao, xml, bridge, fiscal, recharts, xlsx, csv, pdf, navigation]
dependency_graph:
  requires:
    - "04-01: ConciliacaoHandler /api/xml/conciliacao + CoberturaHandler /api/xml/cobertura + ConciliacaoCSVHandler /api/xml/conciliacao/csv"
    - "02-03: PainelXMLs.tsx (dense table pattern), RelatorioSaneamento.tsx (card/filter pattern)"
  provides:
    - "ConciliacaoBridgeXML: GET /conciliacao/bridge-xml — página de conciliação fiscal completa"
    - "navigation.ts: tab Conciliação Bridge vs XML no módulo notas"
    - "navigation.ts: getActiveModule retorna 'notas' para /conciliacao/*"
    - "App.tsx: Route /conciliacao/bridge-xml → ConciliacaoBridgeXML"
  affects:
    - "EXP-01: relatório de divergências tributárias com exportação auditável (Excel+CSV+PDF)"
    - "EXP-02: dashboard de cobertura XML por mês com gráfico BarChart"
tech_stack:
  added: []
  patterns:
    - "useQuery TanStack para /api/xml/conciliacao e /api/xml/cobertura — queryKey com [mesAnoAtivo, tipo]"
    - "buildUrl helper com URLSearchParams — filtros opcionais ignorados quando string vazia"
    - "Dense table text-[11px] com overflow-x-auto rounded-md border — padrão PainelXMLs.tsx"
    - "Coloração condicional bg-red-50 hover:bg-red-100 para delta_total > 0.01"
    - "Badge variant=outline com classes condicionais vermelho/cinza para células de delta"
    - "exportToExcel client-side com 18 colunas PT-BR + toast.success"
    - "fetch /api/xml/conciliacao/csv → blob → URL.createObjectURL → anchor.click — download sem nova aba"
    - "window.print() para PDF + @media print .no-print oculta botões e filtros"
    - "BarChart recharts height=300 pct_xml fill=#22c55e — cobertura XML por mês"
key_files:
  created:
    - frontend/src/pages/ConciliacaoBridgeXML.tsx
  modified:
    - frontend/src/lib/navigation.ts
    - frontend/src/App.tsx
    - frontend/src/index.css
decisions:
  - "buildUrl com Record<string,string> em vez de apenas mesAno: suporta filtro composto mes_ano+tipo sem adapter especial"
  - "downloadingCSV state separado de loadingDiv: loading de exportação não bloqueia re-fetch da tabela"
  - "pctXml computado de cobertura[0] (ORDER BY mes_ano DESC no backend): primeiro registro = mês mais recente"
  - "Legenda threshold '(divergências > R$ 0,01)' como texto estático abaixo da tabela — threshold fixo, não configurável"
  - "Footnote 'Notas canceladas excluídas da contagem.' sempre presente na aba Cobertura — independe de dados"
metrics:
  duration_minutes: 15
  completed_date: "2026-05-16"
  tasks_completed: 2
  tasks_total: 2
  files_created: 1
  files_modified: 3
---

# Phase 04 Plan 02: Conciliação Bridge vs XML — Frontend Summary

**One-liner:** Página React ConciliacaoBridgeXML com tabela densa 13 colunas, gráfico BarChart de cobertura, exportação Excel/CSV/PDF e navegação integrada ao módulo Notas Importadas.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Criar ConciliacaoBridgeXML.tsx + @media print | 25877e4 | frontend/src/pages/ConciliacaoBridgeXML.tsx (criado), frontend/src/index.css (modificado) |
| 2 | Adicionar tab em navigation.ts e rota em App.tsx | a89635f | frontend/src/lib/navigation.ts (modificado), frontend/src/App.tsx (modificado) |

## What Was Built

### frontend/src/pages/ConciliacaoBridgeXML.tsx (337 linhas)

Página completa com:

**Interfaces TypeScript:**
- `DivergenciaRow` — 20 campos: chave_nfe, forn_cnpj, forn_nome, mes_ano, data_emissao, cfop, xml_pis/cofins/icms/ipi/v_nf, bridge_pis/cofins/icms/ipi, delta_pis/cofins/icms/ipi, delta_total
- `CoberturaRow` — 5 campos: mes_ano, total_nfes, com_xml, so_bridge, pct_xml

**Helpers:** fmtBRL, fmtCNPJ, buildUrl (idênticos aos de RelatorioSaneamento.tsx)

**Estado:** mesAnoFiltro, mesAnoAtivo, tipo ('entradas'|'saidas'), downloadingCSV

**Queries:**
- `divergencias`: queryKey ['xml-conciliacao', mesAnoAtivo, tipo] → GET /api/xml/conciliacao
- `cobertura`: queryKey ['xml-cobertura', tipo] → GET /api/xml/cobertura

**Handlers:** handleBuscar, handleLimpar, handleExportExcel (18 colunas PT-BR), handleExportCSV (blob download)

**Métricas de resumo:** totalDivergencias, deltaTotal (reduce), pctXml (cobertura[0])

**JSX:**
- 3 cards de resumo sm:grid-cols-3
- Tabs defaultValue="divergencias" com 2 abas
- Aba Divergências: filtros (Input Mês/Ano + Select entradas/saídas + botões Buscar/Limpar), tabela 13 colunas text-[11px] com coloração condicional, botões exportação no-print, legenda threshold
- Aba Cobertura XML: BarChart height=300 Bar pct_xml fill=#22c55e + tabela de cobertura 5 colunas + footnote

### frontend/src/index.css

Adicionado bloco `@media print`:
- `.no-print { display: none !important; }` — oculta botões e filtros na impressão
- `body { background: white; }` — fundo branco para PDF
- `.overflow-x-auto { overflow: visible; }` — tabela visível sem scroll no PDF

### frontend/src/lib/navigation.ts

Duas modificações:
1. Tab inserida após Saneamento CCLASSTRIB: `{ label: 'Conciliação Bridge vs XML', path: '/conciliacao/bridge-xml' }`
2. `getActiveModule`: `if (pathname.startsWith('/conciliacao/')) return 'notas'` inserido após a condição de `/relatorios/saneamento`

### frontend/src/App.tsx

Duas modificações:
1. Import: `import ConciliacaoBridgeXML from './pages/ConciliacaoBridgeXML'` (linha 23)
2. Route: `<Route path="/conciliacao/bridge-xml" element={<ConciliacaoBridgeXML />} />` após saneamento-cclasstrib

## Verification Results

```
npm run build → ✓ built in 7.26s (Task 1) + ✓ built in 4.59s (Task 2) — sem erros TypeScript
ls ConciliacaoBridgeXML.tsx → -rw-r--r-- 20424 bytes
grep api/xml/conciliacao|cobertura|exportToExcel|window.print|bg-red-50|#22c55e → 11 ocorrências
grep canceladas excluídas|divergências > R$ 0,01 → 2 ocorrências (footnote + legenda)
grep @media print → 1 ocorrência em index.css
grep "Conciliação Bridge vs XML" navigation.ts → 1
grep "conciliacao/bridge-xml" navigation.ts → 1
grep "startsWith('/conciliacao/')" navigation.ts → 1
grep ConciliacaoBridgeXML App.tsx → 2 (import + Route)
grep "conciliacao/bridge-xml" App.tsx → 1
```

## Deviations from Plan

None — plan executed exactly as written.

## Known Stubs

None. A página renderiza dados reais dos endpoints criados no Plan 01. Todos os handlers retornam dados do banco de dados. Nenhum placeholder, mock ou hardcoded empty value que flua para renderização de UI.

## Threat Flags

Nenhuma nova superfície de ataque além das documentadas no threat model do plano (T-04-02-01 a T-04-02-05). Os controles documentados estão corretamente implementados:
- T-04-02-01: fetch interceptado por AuthContext (Authorization + X-Company-ID injetados globalmente)
- T-04-02-02: mes_ano passado como query param, nunca executado como SQL no frontend
- T-04-02-04: exportToExcel processa máximo 500 rows (LIMIT no backend do Plan 01)

## Self-Check: PASSED

- [x] frontend/src/pages/ConciliacaoBridgeXML.tsx existe (20424 bytes)
- [x] frontend/src/index.css contém @media print com .no-print
- [x] frontend/src/lib/navigation.ts contém 'Conciliação Bridge vs XML' e '/conciliacao/bridge-xml'
- [x] frontend/src/lib/navigation.ts contém getActiveModule para /conciliacao/
- [x] frontend/src/App.tsx contém import ConciliacaoBridgeXML (1 ocorrência) e Route (1 ocorrência) = 2 total
- [x] Commit 25877e4 existe (Task 1)
- [x] Commit a89635f existe (Task 2)
- [x] npm run build passa sem erros TypeScript (✓ built in 7.26s e 4.59s)
- [x] ConciliacaoBridgeXML.tsx contém "export default function ConciliacaoBridgeXML"
- [x] ConciliacaoBridgeXML.tsx contém Tabs defaultValue="divergencias"
- [x] ConciliacaoBridgeXML.tsx contém fetch /api/xml/conciliacao e /api/xml/cobertura
- [x] ConciliacaoBridgeXML.tsx contém exportToExcel(data,
- [x] ConciliacaoBridgeXML.tsx contém window.print()
- [x] ConciliacaoBridgeXML.tsx contém bg-red-50 hover:bg-red-100
- [x] ConciliacaoBridgeXML.tsx contém "Notas canceladas excluídas da contagem."
- [x] ConciliacaoBridgeXML.tsx contém "divergências > R$ 0,01"
- [x] ConciliacaoBridgeXML.tsx contém #22c55e
- [x] ConciliacaoBridgeXML.tsx contém DivergenciaRow e CoberturaRow como interfaces TypeScript
- [x] ConciliacaoBridgeXML.tsx contém fmtBRL e fmtCNPJ como funções helper
