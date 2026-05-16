---
status: partial
phase: 04-concilia-o-bridge-vs-xml
source: [04-VERIFICATION.md]
started: 2026-05-16T22:10:00Z
updated: 2026-05-16T22:10:00Z
---

## Current Test

[awaiting human testing]

## Tests

### 1. Tab Navigation Highlight
expected: Sidebar module "Notas Importadas" highlights; sub-tab "Conciliação Bridge vs XML" aparece ativo em /conciliacao/bridge-xml
result: [pending]

### 2. Row Coloring com Divergências Reais
expected: Linhas com delta_total > R$ 0,01 exibem bg-red-50; linhas sem divergência ficam brancas
result: [pending]

### 3. Exportação Excel
expected: .xlsx com 18 colunas PT-BR, valores com 2 casas decimais, sem células corrompidas
result: [pending]

### 4. Impressão PDF (window.print)
expected: Print preview mostra dados fiscais; botões de exportação (classe no-print) ocultos; tabela visível sem scroll horizontal
result: [pending]

### 5. SLA de Resposta < 10s
expected: GET /api/xml/conciliacao?tipo=entradas retorna em < 10s em produção sem filtro de mes_ano
result: [pending]

## Summary

total: 5
passed: 0
issues: 0
pending: 5
skipped: 0
blocked: 0

## Gaps
