---
status: partial
phase: 11-motor-de-execu-o-do-pacote-fiscal-backend
source: [11-VERIFICATION.md]
started: 2026-07-03T17:28:06Z
updated: 2026-07-03T17:28:06Z
---

## Current Test

[awaiting human testing]

## Tests

### 1. Execução real de lote contra Oracle prod/PRODB com credenciais válidas
expected: POST /api/fiscal/execute com credenciais Oracle REAIS (não placeholder) contra uma nfe_id real de Recife/PE (CNPJ raiz 10230480) com vários itens retorna JSON com ok > 0; `SELECT status, count(*) FROM fiscal_execution_items GROUP BY status` mostra ao menos uma linha status='ok' com base_calculo_icms/valor_icms preenchidos; full_result mostra campos OUT string preenchidos (sem ORA-06502 buffer too small) e IdRegraCalculo* como texto tipo "IVA_..." (sem ORA-06502 character-to-number)
result: [pending]

### 2. Item de filial não mapeada não aborta o lote
expected: POST /api/fiscal/execute para uma nota de filial fora de Recife/PE (fora do mapa codEmpresaPorCNPJRaiz) retorna summary agregado correto (total=N, error>=1 para os itens da filial não mapeada) sem crash/abort do processamento dos demais itens
result: [pending]

## Summary

total: 2
passed: 0
issues: 0
pending: 2
skipped: 0
blocked: 0

## Gaps
