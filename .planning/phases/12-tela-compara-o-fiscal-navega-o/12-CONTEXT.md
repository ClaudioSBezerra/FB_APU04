# Phase 12: Tela Comparação Fiscal + Navegação - Context

**Gathered:** 2026-07-03
**Status:** Ready for planning

<domain>
## Phase Boundary

Um usuário admin busca uma NF-e específica (por número ou chave de acesso),
dispara a execução do pacote fiscal para essa nota (reaproveitando o endpoint
`POST /api/fiscal/execute` da Fase 11), e vê o resultado na mesma tela: cards
de resumo, tabela item a item com esperado (`nfe_saidas_itens`) vs. calculado
(`fiscal_execution_items`) para ICMS/ICMS-ST/PIS/COFINS/IBS/CBS, divergências
destacadas visualmente, filtro "só divergentes", exportação Excel, e um item
de navegação novo com gate `adminOnly: true`. Cobre TPF-06 a TPF-08.

Este é o primeiro fluxo end-to-end do módulo "Teste Pacote Fiscal": a Fase 11
construiu a fundação de dados (backend), a Fase 12 fecha o ciclo com a
interface que dispara e exibe os resultados — sem essa tela, o único jeito de
popular `fiscal_execution_items` é via curl manual (como foi feito para
validar a Fase 11).

</domain>

<decisions>
## Implementation Decisions

### Gatilho de execução
- **D-01:** A tela dispara a execução do lote — não é só visualização
  passiva. Usuário busca uma nota, clica em "Executar"/"Rodar motor fiscal",
  a tela chama `POST /api/fiscal/execute` com o `nfe_id`, aguarda a resposta,
  e então recarrega automaticamente os itens executados dessa nota (cards +
  tabela) na mesma tela — sem navegação extra nem toast-e-clique-de-novo.

### Escopo de seleção de nota
- **D-02:** Fluxo é por nota específica, não listagem agregada de tudo já
  executado. Usuário busca 1 NF-e (número da nota ou chave de acesso, campo
  de texto com autocomplete — mesmo padrão de busca de NF-e já usado em
  outras telas do FB_APU04), seleciona, e a tela mostra só os itens daquela
  nota. É uma ferramenta de validação pontual, não um relatório de massa.
- **D-03:** Reexecutar uma nota já executada é permitido (o backend já faz
  upsert por `nfe_item_id` via `ON CONFLICT`, migration 147) — não precisa de
  lógica nova de "já foi executada" na tela, o upsert do backend cobre isso.

### Exportação Excel
- **D-04:** Incluir botão "Exportar Excel" na tela, seguindo o mesmo padrão
  já usado em `ConciliacaoBridgeXML.tsx`/`ComparativoEFDvsXML.tsx` (endpoint
  CSV/Excel dedicado do lado backend). Não estava em TPF-06/07/08
  literalmente, mas o usuário confirmou que entra nesta fase por consistência
  com as telas análogas.

### Ruído conhecido em IBS/CBS
- **D-05:** `nfe_saidas_itens.v_ibs`/`v_cbs` costumam estar NULL/0 hoje (gap
  de parser documentado em `.continue-here.md`), então o comparativo vai
  marcar quase todo item como "divergente" nesses 2 impostos mesmo quando o
  pacote fiscal está correto. Tratamento: **tooltip de aviso apenas** — IBS e
  CBS continuam contando normalmente para o filtro "só divergentes" e para o
  resumo agregado, sem tratamento especial além do aviso contextual (já
  desenhado no UI-SPEC). Não esconder nem excluir esses campos do cálculo.

### Claude's Discretion
- Exato componente/endpoint de exportação Excel a reaproveitar/criar
  (CSV vs. xlsx real) — seguir o padrão mais próximo já existente no
  codebase (`ConciliacaoBridgeXML.tsx`/`ComparativoEFDvsXML.tsx`).
- Layout exato do estado de loading durante a execução do lote (spinner
  inline no botão vs. skeleton na área de resultado) — UI-SPEC já cobre
  estados vazio/erro, mas não o loading de execução em si; usar convenção
  já estabelecida no UI-SPEC (spinner + texto).
- Paginação/virtualização da tabela item a item se uma nota tiver muitos
  itens — não é um requisito explícito, decidir com base no volume real
  (notas de venda tipicamente não passam de dezenas de itens).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Design contract (fonte primária de UI/UX desta fase)
- `.planning/phases/12-tela-compara-o-fiscal-navega-o/12-UI-SPEC.md` —
  contrato de design aprovado (6/6 dimensões): spacing, typography, color,
  copywriting, layout de 8 blocos, componentes shadcn a reusar. **Leitura
  obrigatória antes de planejar** — decisões visuais já estão travadas ali,
  não devem ser re-decididas no planejamento.

### Backend consumido por esta fase (Fase 11, já implementada)
- `backend/handlers/fiscal_execution.go` — `FiscalExecutionRunHandler`
  (`POST /api/fiscal/execute`, body `{"nfe_id": "..."}`, resposta
  `{total, ok, sem_grupo_fiscal, error}`) é o endpoint que o botão
  "Executar" desta fase vai chamar.
- `backend/migrations/147_fiscal_execution_items.sql` — schema exato de
  `fiscal_execution_items` (colunas típicas indexáveis + `full_result`
  JSONB com os ~88 campos do pacote Oracle) — fonte do lado "calculado" da
  comparação.
- `.planning/phases/11-motor-de-execu-o-do-pacote-fiscal-backend/11-05-SUMMARY.md`
  — mapeamento exato de campos e decisões do endpoint de execução.

### Handoff / decisões já tomadas (herdadas da Fase 11)
- `.planning/.continue-here.md` §4 — descrição da tela de referência
  `ComparacaoFiscal.tsx` do projeto descontinuado FB_TESTESFC (cards de
  resumo, filtro "só divergentes", dialog de detalhe com seção "Só
  calculado", regra de divergência zero-tolerância). Tratado como prior art
  validado, incorporado ao UI-SPEC.
- `.planning/REQUIREMENTS.md` — TPF-06 a TPF-08 (linhas 105-107).
- `.planning/ROADMAP.md` §"Phase 12" — goal e success criteria exatos.

### Padrões de código existentes no FB_APU04
- `frontend/src/pages/ConciliacaoBridgeXML.tsx` e
  `frontend/src/pages/ComparativoEFDvsXML.tsx` — telas mais próximas que já
  resolvem o mesmo problema visual (esperado vs. calculado, divergência
  destacada); tratadas como precedente vinculante para tabela/badge/filtro,
  não só inspiração (confirmado no UI-SPEC).
- `frontend/src/lib/navigation.ts` — padrão `adminOnly?: boolean` já usado
  em outros itens de menu; reaproveitar idêntico para TPF-08 (item de
  navegação novo "Teste Pacote Fiscal").
- `frontend/src/components/AppSidebar.tsx` — seção de navegação onde o novo
  item entra, mesmo padrão de gate `adminOnly` a nível de seção.

**No external specs beyond the above** — requisitos totalmente capturados no
UI-SPEC + REQUIREMENTS.md + handoff da Fase 11.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `ConciliacaoBridgeXML.tsx`/`ComparativoEFDvsXML.tsx`: componentes de
  tabela de comparação, badges de status, filtro, exportação — base direta
  para a tela nova, adaptar em vez de criar do zero.
- `navigation.ts` + `AppSidebar.tsx`: gate `adminOnly` já implementado e em
  produção — TPF-08 é reuso puro, zero sistema de permissão novo.
- Componente de busca/autocomplete de NF-e já usado em outras telas do
  FB_APU04 — reaproveitar para o seletor de nota (D-02).

### Established Patterns
- Regra de divergência "zero tolerância" (qualquer diferença ≠ 0, sem
  tolerância de arredondamento) — diferente do padrão de
  `ConciliacaoBridgeXML.tsx` que usa tolerância de R$0,01 (essa tela é
  reconciliação ERP-vs-XML com ruído esperado; a Fase 12 é validação de
  pacote fiscal, onde 1 centavo pode importar).
- Item com `status != 'ok'` em `fiscal_execution_items` é "Não calculado",
  nunca "divergente" — evita falso positivo (herdado do FB_TESTESFC,
  confirmado no UI-SPEC).

### Integration Points
- Botão "Executar" → `POST /api/fiscal/execute` (Fase 11) → reload da
  listagem de itens da mesma nota via leitura de `fiscal_execution_items`
  JOIN `nfe_saidas_itens`.
- Item de navegação novo → `navigation.ts` + `AppSidebar.tsx` (gate
  `adminOnly`) → rota nova envolvida no `AdminRoute` existente (padrão já
  documentado no UI-SPEC).

</code_context>

<specifics>
## Specific Ideas

- Fluxo completo: buscar nota → clicar Executar → ver resultado na mesma
  tela, sem navegação extra. Isso é uma decisão explícita do usuário nesta
  discussão — evita que o planner desenhe um fluxo assíncrono/multi-tela.
- IBS/CBS: aviso via tooltip é suficiente, não excluir do filtro/resumo
  agregado — decisão explícita para não mascarar dados incompletos, apenas
  contextualizá-los.

</specifics>

<deferred>
## Deferred Ideas

- Sistema de permissão granular por módulo (substituindo o gate `adminOnly`
  binário) — milestone futura, já documentado como fora de escopo desde a
  Fase 11.
- Execução em lote de múltiplas notas de uma vez (ex.: todas as notas de um
  período) — fora de escopo desta fase, que é validação pontual nota a
  nota; poderia virar fase futura se o volume de uso justificar.
- Paginação/virtualização de tabela para notas com muitos itens — deixado
  como Claude's Discretion nesta fase (ver acima), pode virar ajuste futuro
  se necessário.

### Reviewed Todos (not folded)
None — discussion stayed within phase scope

</deferred>

---

*Phase: 12-tela-compara-o-fiscal-navega-o*
*Context gathered: 2026-07-03*
