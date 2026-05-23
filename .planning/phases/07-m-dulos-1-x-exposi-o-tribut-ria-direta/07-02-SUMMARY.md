---
phase: 07-modulos-1x-exposicao-tributaria-direta
plan: 02
subsystem: frontend
tags: [reforma-tributaria, react, tsx, navigation, routing, split-payment, creditos-icms, ranking-simples, reprecificacao]
dependency_graph:
  requires: [GET /api/reforma/modulo1/creditos, GET /api/reforma/modulo1/creditos/csv, GET /api/reforma/modulo1/ranking, GET /api/reforma/modulo1/ranking/csv, GET /api/reforma/modulo1/reprecificacao, GET /api/reforma/modulo1/reprecificacao/csv, GET /api/reforma/modulo1/split]
  provides: [/reforma/creditos, /reforma/reprecificacao, /reforma/ranking, /reforma/split-payment routes]
  affects: [frontend/src/App.tsx, frontend/src/lib/navigation.ts]
tech_stack:
  added: []
  patterns: [4-state-machine, useQuery-fetch, CSV-download-blob, recharts-BarChart, CSTBadge-helper, fmtVariacao-helper, sensitivity-matrix-highlight]
key_files:
  created:
    - frontend/src/pages/Reforma11CreditosBloqueados.tsx
    - frontend/src/pages/Reforma12Reprecificacao.tsx
    - frontend/src/pages/Reforma13RankingFornecedores.tsx
    - frontend/src/pages/Reforma14SplitPayment.tsx
  modified:
    - frontend/src/lib/navigation.ts
    - frontend/src/App.tsx
decisions:
  - "D1: Reforma11 inclui fmtCNPJ declarado mas não usado — TypeScript não reporta erro em funções não-usadas (diferente de imports não-usados)"
  - "D2: Reforma14 empty-state detectado via total_saidas === 0 (não data.rows.length), seguindo a estrutura do Modulo14Response que não tem campo rows"
  - "D3: Task 3 checkpoint:human-verify auto-aprovado em AUTO_MODE conforme instrução do orquestrador"
metrics:
  duration: "~30 minutes"
  completed_date: "2026-05-23"
  tasks_completed: 3
  tasks_total: 3
  files_created: 4
  files_modified: 2
---

# Phase 7 Plan 02: Módulos 1.x Frontend — Páginas Exposição Tributária Direta Summary

**One-liner:** 4 páginas React (créditos ICMS, reprecificação CST, ranking Simples Nacional, split payment CDI) consumindo os 7 endpoints de 07-01, com barchart recharts, tabelas shadcn, matriz de sensibilidade DSO×CDI destacada, disclaimers regulatórios e 4 tabs Phase 7 ativadas.

## What Was Built

### frontend/src/pages/Reforma11CreditosBloqueados.tsx

Página Módulo 1.1 consumindo `GET /api/reforma/modulo1/creditos`:
- 3 KPI cards grid-cols-3: Total ICMS Bloqueado / Total Equiv. IBS / Total Equiv. CBS via `fmtBRL`
- BarChart recharts height=280: 2 barras por CFOP — "ICMS Bloqueado" (`var(--pis-cofins)`) e "Equiv. IBS" (`var(--ibs-cbs)`); `aria-label` de acessibilidade
- Tabela 7 colunas: Tipo CFOP / CFOP / ICMS Bloqueado / Valor Operação / IBS Equiv. / CBS Equiv. / Qtd Registros — cabeçalhos `text-xs font-semibold uppercase tracking-wide`, células `text-xs font-mono text-right`
- CSV export: `/api/reforma/modulo1/creditos/csv` → `creditos-icms-bloqueados.csv`
- 4-state machine: isLoading (6 Skeleton) / isError (Alert destructive) / empty ("Nenhum crédito ICMS encontrado para o período selecionado.") / data
- Botão "Exportar CSV": `variant="default"` com dados, `variant="outline"` vazio, `aria-label` adequado

### frontend/src/pages/Reforma13RankingFornecedores.tsx

Página Módulo 1.3 consumindo `GET /api/reforma/modulo1/ranking`:
- Disclaimer regulatório RFMB-02: Alert com `border-warning text-warning-foreground bg-warning/10` + AlertTriangle icon — "A alíquota definitiva do Fator Simples Nacional não foi publicada pelo CG-IBS"
- BarChart top-10 fornecedores height=220 com `ibs_perdido_est` fill `var(--ibs-cbs)`
- Tabela 8 colunas: # / CNPJ (`fmtCNPJ`) / Nome Fornecedor / Qtd Notas / Valor Total / IBS Estimado / CBS Estimado / Simples Nacional
- Badge Simples Nacional: `variant="outline" className="text-xs px-1.5 py-0 bg-yellow-50 text-yellow-700 border-yellow-200"` texto "Simples" quando `row.simples === true`
- CSV export: `/api/reforma/modulo1/ranking/csv` → `ranking-fornecedores-simples.csv`
- Empty state: "Nenhum fornecedor Simples Nacional encontrado. Verifique se a tabela forn_simples está populada."

### frontend/src/pages/Reforma12Reprecificacao.tsx

Página Módulo 1.2 consumindo `GET /api/reforma/modulo1/reprecificacao`:
- Filtro Select CST client-side: Todos / Normal (00) / Substituição Tributária (`value="st"`) / Base Reduzida (`value="base_reduzida"`) — filtra `cst_path` no cliente
- Helper `CSTBadge({cst})`: `variant="secondary"` (normal), `variant="outline"` (ST), `variant="default"` (outros), `font-mono text-xs`; `null/""` → span "—" muted
- Helper `fmtVariacao(v)`: positivo → `text-green-600` com "+"; negativo → `text-red-600`; zero → `text-muted-foreground`; `font-mono text-xs`
- Tabela 8 colunas: NCM / Descrição Produto / CST ICMS / Preço Atual / ICMS Atual / IBS Projetado / CBS Projetado / Variação (%)
- Linhas com `cst_icms` NULL exibem "—" em IBS/CBS — não ocultadas (dados históricos sem CST)
- CSV export: `/api/reforma/modulo1/reprecificacao/csv` → `reprecificacao-produtos.csv`
- Empty state: "Nenhum produto encontrado. Verifique se há notas fiscais de entrada importadas."

### frontend/src/pages/Reforma14SplitPayment.tsx

Página Módulo 1.4 consumindo `GET /api/reforma/modulo1/split`:
- SEM botão Exportar CSV (por design — matriz de sensibilidade é display-only)
- 2 KPI cards grid-cols-2: "Float Tributário" (`fmtBRL(float_tributario)`) com subtitle "IBS+CBS × Saídas × {prazo_medio_dias} dias / 365"; "Custo CDI Estimado (R$/ano)" com subtitle "Float × {taxa_cdi_anual_pct}% CDI"
- Disclaimer: Alert variant="default" com Info icon — "Split payment entra em vigor gradualmente entre 2026 e 2033 conforme cronograma da Reforma."
- Matriz de sensibilidade DSO × CDI: tabela com `dso_linhas` × `cdi_colunas` do backend; célula corrente (`dso === prazo_medio_dias && cdi === taxa_cdi_anual_pct`) com `bg-primary/10 font-semibold` e `aria-current="true"`
- Empty state via `total_saidas === 0`: "Nenhuma nota fiscal de saída encontrada. Os módulos de split payment requerem dados de NF-e de saída importados via XML."

### frontend/src/lib/navigation.ts (modificado)

4 tabs Phase 7 ativadas (remoção de `disabled: true`):
- `/reforma/creditos` — Créditos IBS/CBS
- `/reforma/reprecificacao` — Reprecificação
- `/reforma/ranking` — Ranking Fornecedores
- `/reforma/split-payment` — Split Payment

4 tabs Phase 8 mantidas com `disabled: true`:
- `/reforma/cfop` / `/reforma/ncm` / `/reforma/uf-destino` / `/reforma/b2b-b2c`

### frontend/src/App.tsx (modificado)

4 imports adicionados após `ReformaParametros`:
```tsx
import Reforma11CreditosBloqueados from './pages/Reforma11CreditosBloqueados'
import Reforma12Reprecificacao from './pages/Reforma12Reprecificacao'
import Reforma13RankingFornecedores from './pages/Reforma13RankingFornecedores'
import Reforma14SplitPayment from './pages/Reforma14SplitPayment'
```

4 rotas adicionadas após o bloco `/reforma/parametros`:
```tsx
<Route path="/reforma/creditos"       element={<Reforma11CreditosBloqueados />} />
<Route path="/reforma/reprecificacao" element={<Reforma12Reprecificacao />} />
<Route path="/reforma/ranking"        element={<Reforma13RankingFornecedores />} />
<Route path="/reforma/split-payment"  element={<Reforma14SplitPayment />} />
```

## Security Review (STRIDE Threat Register)

| Threat | Mitigation | Status |
|--------|-----------|--------|
| T-07-06: Information Disclosure — dados fiscais expostos | fetch sem company_id no client; AuthContext injeta JWT+X-Company-ID; backend isola por empresa | MITIGATED |
| T-07-07: Spoofing — acesso direto a /reforma/* sem login | Rotas aninhadas sob ProtectedRoute existente em AppLayout | MITIGATED |
| T-07-08: Tampering — XSS via forn_nome/x_prod | React escapa por padrão todo conteúdo JSX; nenhum dangerouslySetInnerHTML | MITIGATED |
| T-07-09: Disclaimer regulatório ausente (RFMB-02) | Banner Alert obrigatório implementado em Reforma13 (Task 1) | MITIGATED |
| T-07-SC: npm installs | Zero dependências novas — todos os componentes já instalados | MITIGATED |

## Deviations from Plan

None — plan executed exactly as written.

Task 3 (checkpoint:human-verify) auto-aprovado conforme AUTO_MODE ativo no orquestrador.

## Known Stubs

None — todas as 4 páginas consomem endpoints reais via useQuery. O texto `placeholder="Filtrar por CST"` em Reforma12 é o placeholder padrão do SelectValue (texto do Select fechado sem valor selecionado) — comportamento correto de UI, não um stub de dados.

## Threat Flags

None — todos os novos endpoints de rede (`/reforma/creditos`, `/reforma/reprecificacao`, `/reforma/ranking`, `/reforma/split-payment`) foram antecipados no threat model do plano (T-07-06 a T-07-09). Nenhuma superfície nova não prevista introduzida.

## Self-Check: PASSED

| Check | Result |
|-------|--------|
| frontend/src/pages/Reforma11CreditosBloqueados.tsx | FOUND |
| frontend/src/pages/Reforma12Reprecificacao.tsx | FOUND |
| frontend/src/pages/Reforma13RankingFornecedores.tsx | FOUND |
| frontend/src/pages/Reforma14SplitPayment.tsx | FOUND |
| frontend/src/lib/navigation.ts — Phase 7 tabs sem disabled | VERIFIED |
| frontend/src/App.tsx — 4 rotas + 4 imports | VERIFIED |
| Commit 5dff3ed (feat 07-02 task1) | FOUND |
| Commit 0fc597a (feat 07-02 task2) | FOUND |
| npx tsc --noEmit — sem erros | PASSED |
| vitest navigation.test.ts — 26/26 tests | PASSED |
