---
status: complete
phase: 10-icms-fronteira-st-por-ncm
source: [10-01-SUMMARY.md]
started: 2026-06-28T13:41:11Z
updated: 2026-06-28T13:45:00Z
---

## Current Test

[testing complete]

## Tests

### 1. NF 159027 aparece no ST por Item (Bloco C)
expected: NF 159027 (BGL Bertoloto) aparece na aba ST do Bloco C com itens NCM 7318/8482 classificados como ST, MVA e base calculados por item
result: pass

### 2. Consistência Antecipação × ST
expected: A mesma NF 159027 NÃO aparece na aba Antecipação (foi reclassificada para ST pelo NCM). Não some das duas abas — está exatamente em uma (ST)
result: pass

### 3. Badge "NCM→ST" visível
expected: Na tabela ST do Bloco C, NFs com CFOP 6101/6102 (entrada 2101/2102) reclassificadas pelo NCM exibem o badge azul "NCM→ST" na célula CFOP
result: pass

### 4. Tela de reclassificação manual removida
expected: Não existe mais a seção de reclassificação manual no módulo — sem Select de regime, sem botão IA, sem botões validar/excluir/reset
result: pass

### 5. Cálculo por item por NCM correto
expected: Para NF 159027, NCM 7318 usa MVA 55% e NCM 8482 usa MVA 71.78%; cada item calcula sua própria base e ICMS-ST conforme a regra do seu NCM
result: pass

## Summary

total: 5
passed: 5
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

<!-- appended when issues found -->
