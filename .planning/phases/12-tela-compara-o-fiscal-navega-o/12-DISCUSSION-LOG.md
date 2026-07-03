# Phase 12: Tela Comparação Fiscal + Navegação - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-03
**Phase:** 12-tela-compara-o-fiscal-navega-o
**Areas discussed:** Gatilho de execução do lote, Escopo de seleção de nota(s), Exportação Excel, Ruído conhecido em IBS/CBS

---

## Gatilho de execução do lote

| Option | Description | Selected |
|--------|-------------|----------|
| Botão "Executar" na tela | Usuário escolhe uma nota e clica em "Rodar motor fiscal" — chama POST /api/fiscal/execute, mostra progresso/resultado, depois carrega a comparação. | ✓ |
| Só visualização (execução fica de fora) | A tela só lê fiscal_execution_items já populada. Execução continua manual via API/script. | |

**User's choice:** Botão "Executar" na tela
**Notes:** Fecha o ciclo completo sem depender de curl manual — hoje a única forma de popular fiscal_execution_items era via curl (validado no Plan 11-06).

---

## Escopo de seleção de nota(s)

| Option | Description | Selected |
|--------|-------------|----------|
| Seletor por nota específica | Campo de busca/select — usuário escolhe 1 nota, clica Executar, vê os itens dela. | ✓ |
| Listagem agregada de tudo já executado | Mostra de cara todos os itens de fiscal_execution_items de todas as notas já rodadas. | |

**User's choice:** Seletor por nota específica

| Option | Description | Selected |
|--------|-------------|----------|
| Busca por número da nota ou chave de acesso | Campo de texto com autocomplete/busca, mesmo padrão já usado em outras telas do FB_APU04. | ✓ |
| Dropdown das notas mais recentes | Lista as N notas mais recentes da empresa num select. | |

**User's choice:** Busca por número da nota ou chave de acesso

| Option | Description | Selected |
|--------|-------------|----------|
| Mostra o resumo + tabela na mesma tela | Após a resposta do POST, a tela recarrega automaticamente os itens executados dessa nota. | ✓ |
| Mostra só um toast de sucesso, usuário clica em algo pra ver o resultado | Confirmação simples, ação extra necessária pra ver resultado. | |

**User's choice:** Mostra o resumo + tabela na mesma tela
**Notes:** É uma ferramenta de validação pontual, não um relatório de massa — fluxo busca→executar→ver deve ser contínuo, sem navegação extra.

---

## Exportação Excel

| Option | Description | Selected |
|--------|-------------|----------|
| Sim, incluir | Mantém consistência com telas análogas (ConciliacaoBridgeXML, ComparativoEFDvsXML) que já têm esse botão. | ✓ |
| Não, fora de escopo por ora | TPF-06/07/08 não pedem exportação — fica estritamente no requirement escrito. | |

**User's choice:** Sim, incluir
**Notes:** Não estava em TPF-06/07/08 literalmente, mas confirmado por consistência de padrão com as telas análogas — não é capacidade nova, é convenção já estabelecida no codebase.

---

## Ruído conhecido em IBS/CBS

| Option | Description | Selected |
|--------|-------------|----------|
| Tooltip de aviso basta | Mantém IBS/CBS no cálculo de divergência normalmente, só com tooltip explicativo (já previsto no UI-SPEC). | ✓ |
| Excluir IBS/CBS do filtro "só divergentes" e do resumo agregado | IBS/CBS continuam visíveis na tabela mas não contam pro filtro/resumo. | |

**User's choice:** Tooltip de aviso basta
**Notes:** Não esconder informação, só contextualizar — nfe_saidas_itens.v_ibs/v_cbs costumam estar NULL/0 hoje (gap de parser conhecido, documentado em .continue-here.md).

---

## Claude's Discretion

- Exato componente/endpoint de exportação Excel a reaproveitar/criar (CSV vs. xlsx real).
- Layout exato do estado de loading durante a execução do lote (spinner inline vs. skeleton).
- Paginação/virtualização da tabela item a item para notas com muitos itens.

## Deferred Ideas

- Sistema de permissão granular por módulo — milestone futura (já documentado desde a Fase 11).
- Execução em lote de múltiplas notas de uma vez — fora de escopo desta fase (validação pontual nota a nota).
- Paginação/virtualização de tabela — deixado como Claude's Discretion, pode virar ajuste futuro.
