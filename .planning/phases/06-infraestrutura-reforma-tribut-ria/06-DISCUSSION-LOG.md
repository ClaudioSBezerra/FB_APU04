# Phase 6: Infraestrutura Reforma Tributária - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-22
**Phase:** 06-infraestrutura-reforma-tributaria
**Areas discussed:** Posição na navegação, Página de parâmetros, Disclaimer fator Simples Nacional, Dados históricos com NULL

---

## Posição na Navegação

| Option | Description | Selected |
|--------|-------------|----------|
| Módulo separado | Novo módulo `reforma` independente na sidebar | ✓ |
| Abas dentro de `simulador` | Reforma como sub-seção do módulo existente | |

**User's choice:** Módulo separado — label "Reforma Tributária", path `/reforma`

| Option | Description | Selected |
|--------|-------------|----------|
| Só 'Parâmetros' + disabled placeholders | Tab única ativa, demais já aparecem desabilitadas | ✓ |
| Só 'Parâmetros', adicionar resto nas phases seguintes | Módulo limpo sem placeholders | |

**User's choice:** Só "Parâmetros" ativa, futuras tabs disabled: true visíveis desde já

---

## Página de Parâmetros

| Option | Description | Selected |
|--------|-------------|----------|
| Dois caminhos — mesma página | /config/reforma-parametros e /reforma/parametros renderizam o mesmo componente | ✓ |
| Só /reforma/parametros | Página apenas no módulo reforma | |

**User's choice:** Dois caminhos para o mesmo componente `ReformaParametros.tsx`

| Option | Description | Selected |
|--------|-------------|----------|
| Card com campos inline editáveis + botão Salvar | Padrão visual de TabelaAliquotas e ERPBridgeConfig | ✓ |
| Formulário com separação IBS/CBS vs. financeiro | Dois grupos de campos | |

**User's choice:** Card com inline edit + Salvar, seguindo padrão existente

---

## Disclaimer fator Simples Nacional

| Option | Description | Selected |
|--------|-------------|----------|
| Tooltip curto "Valor estimado..." | Texto: "Valor estimado. Alíquota definitiva ainda não publicada pelo CG-IBS." | ✓ |
| Texto longo com referência legal | Citar LC 214/2025 e data prevista CG-IBS | |

**User's choice:** Tooltip/ícone ⓘ com texto curto

| Option | Description | Selected |
|--------|-------------|----------|
| Sim, somente leitura | Não-admin vê campos desabilitados, sem botão Salvar | ✓ |
| Não, redirecionados | Acesso bloqueado para não-admin | |

**User's choice:** Não-admin acessa em somente leitura

---

## Dados Históricos com NULL

**User's choice:** NULL é suficiente — sem aviso visual nos módulos 1.x/2.x. Fallback CPF/CNPJ para `ind_final` já documentado em RFMA-03.

---

## Claude's Discretion

- Nomes exatos das migrations (numeração, nomenclatura)
- Estrutura interna de `ReformaParametros.tsx`
- Posição da aba "Parâmetros Reforma" no módulo `config`
- Labels e paths dos placeholders disabled das Phases 7 e 8

## Deferred Ideas

- Módulos analíticos 1.1–1.4 e 2.1–2.4 → Phases 7 e 8
- Backfill histórico de cst_icms/aliq_icms → não é prioridade
- Aviso visual para registros históricos NULL → aguardar feedback dos usuários
