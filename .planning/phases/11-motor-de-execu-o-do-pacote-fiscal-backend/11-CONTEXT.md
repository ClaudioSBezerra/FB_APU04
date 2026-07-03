# Phase 11: Motor de Execução do Pacote Fiscal (Backend) - Context

**Gathered:** 2026-07-03
**Status:** Ready for planning

<domain>
## Phase Boundary

Dado um item de `nfe_saidas_itens`, o sistema resolve seu grupo fiscal no
Oracle (`prod`/`PRODB`), executa `PKG_FISCAL_FCTAX.calcula_imposto_produto`
com bind seguro e persiste os ~88 campos de saída em `fiscal_execution_items`,
em lote, com isolamento de erro e limites de concorrência/timeout. **Sem
nenhuma tela ainda** — só a fundação de dados que a Fase 12 vai exibir na
tela "Comparação Fiscal". Cobre TPF-01 a TPF-05.

Este módulo é um **porte** do validador unitário já implementado e validado
contra Oracle real no projeto irmão descontinuado `FB_TESTESFC`
(2026-06-30 a 2026-07-02) — não é uma implementação do zero. Adaptar o
código existente, não reescrever.

</domain>

<decisions>
## Implementation Decisions

### Fonte de dados
- **D-01:** Reaproveitar `nfe_saidas`/`nfe_saidas_itens` já existentes como
  fonte dos parâmetros de entrada do pacote fiscal. **Não** portar o
  pipeline de import de XML do FB_TESTESFC — o XML já é importado hoje pelo
  fluxo normal do FB_APU04.
- **D-02 (confirmado por inspeção de código, 2026-07-03):** TPF-02 é
  necessário. `insertNFeItens` (`backend/handlers/nfe_saidas.go:373`) já
  parseia `VDesc` (`xml:"vDesc"`) na struct `det`, mas **não persiste**
  `v_desc` em `nfe_saidas_itens` nem inclui no INSERT. `VOutro` (despesas
  acessórias) **nem está na struct ainda** — precisa adicionar o parsing.
  Escopo confirmado: adicionar `v_desc`/`v_outro` na struct (onde faltar),
  na tabela `nfe_saidas_itens` (migration nova) e no INSERT de
  `insertNFeItens`.

### Conexão Oracle dedicada
- **D-03:** O backend Go abre sua **própria conexão go-ora direta e
  síncrona** ao Oracle `prod`/`PRODB`, lendo as credenciais **já
  armazenadas** (criptografadas) em `erp_bridge_config` por `company_id`.
  Não duplica cadastro de credencial — reaproveita o armazenamento
  existente. Diferença importante em relação ao uso atual de
  `erp_bridge_config`: hoje essas credenciais só são consumidas pelo bridge
  Python externo (assíncrono, roda fora do processo Go); a Fase 11 introduz
  o **primeiro caminho onde o próprio backend Go abre uma conexão Oracle
  síncrona em tempo de requisição** — não existe hoje, é capacidade nova.

### Schema de `fiscal_execution_items`
- **D-04:** Seguir o padrão simples já usado em `nfe_saidas_itens`: schema
  `public`, FK para `nfe_saidas_itens(id)` com `ON DELETE CASCADE`,
  `UNIQUE` constraint em `nfe_item_id` para permitir
  `INSERT ... ON CONFLICT (nfe_item_id) DO UPDATE` (upsert por item, sem
  transação única pro lote inteiro — cada item é seu próprio insert/update,
  conforme TPF-05). Sem particionamento — volume esperado é baixo (uso
  administrativo de validação, não o fluxo de todas as notas).

### Claude's Discretion
- Nome exato dos campos/colunas da migration de `fiscal_execution_items`
  (os ~88 campos de saída — mapear 1:1 com `RDADOS_FISCAIS_PRODUTO`, ver
  `código_reutilizavel_do_fb_testesfc` abaixo).
- Estrutura interna do pool de conexão Oracle dedicado (se compartilha pool
  entre companies ou abre por request) — otimizar depois se necessário,
  não é requisito bloqueante da Fase 11.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Handoff / decisões já tomadas (fonte primária desta fase)
- `.planning/.continue-here.md` — handoff completo da sessão que decidiu
  portar o FB_TESTESFC para cá. Contém: contrato exato dos 23 parâmetros IN
  do pacote Oracle, mapa de `cod_empresa` por CNPJ raiz, query de lookup de
  grupo fiscal já validada contra Oracle real, padrão de
  concorrência/isolamento por item, defaults de parâmetros não capturados
  pelo schema atual, e as duas "pegadinhas" do driver go-ora (buffer size
  em binds OUT string; `IdRegraCalculo*` são VARCHAR2, nunca NUMBER).
  **Leitura obrigatória — tem código-fonte embutido, não depende do
  FB_TESTESFC existir mais no disco/GitHub.**
- `.planning/PROJECT.md` §"Current Milestone: v6.00" — goal e target
  features da milestone; também documenta o incidente de 2026-05-07
  (TRUNCATE cross-database por env var errada) que motivou a decisão
  "Out of Scope: Reset/limpeza por API sem UI dedicada" — relevante como
  lembrete de cautela ao desenhar qualquer operação destrutiva/upsert em
  massa nesta fase.
- `.planning/REQUIREMENTS.md` — TPF-01 a TPF-05 (linhas 99-103), mapeamento
  de requirements para fases (linha 150).
- `.planning/ROADMAP.md` §"Phase 11" — goal e success criteria exatos.

### Padrões de código existentes no FB_APU04
- `backend/handlers/erp_bridge.go` — CRUD de `erp_bridge_config`
  (credenciais Oracle criptografadas por `company_id`); usar o mesmo
  padrão de leitura/descriptografia para a nova conexão Go direta (D-03).
- `backend/handlers/nfe_saidas.go:373` (`insertNFeItens`) — struct `det` e
  INSERT atual de `nfe_saidas_itens`; ponto de partida pra TPF-02.
- `frontend/src/lib/navigation.ts` — padrão `adminOnly?: boolean` já usado
  em outros itens de menu (ex.: linhas 37, 38, 82, 83); reaproveitar
  idêntico para TPF-08 (Fase 12, fora do escopo desta fase, mas mantém
  consistência de padrão).

**No external specs beyond the above** — requisitos totalmente capturados
no handoff + REQUIREMENTS.md.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets (do FB_TESTESFC, ver `.continue-here.md` pro código-fonte completo)
- `oracle_fiscal.go` — contrato dos 23 parâmetros IN / ~88 campos OUT do
  `PKG_FISCAL_FCTAX.calcula_imposto_produto`, geração de bloco PL/SQL
  estático via reflection (duas tabelas Go mapeando parâmetro Oracle ↔
  campo struct Go).
- `fiscal_group_lookup.go` — `resolveCodEmpresa` (mapa CNPJ raiz → código)
  e a query `SELECT pb.grupo_fiscal, p.especial AS origem, p.ncm FROM prodb
  pb, prod p WHERE ...` já validada contra Oracle real.
- `fiscal_execution.go` — padrão de concorrência (semáforo `chan struct{}`
  cap 5, `SetMaxOpenConns(5)`), timeout por item (`context.WithTimeout`
  15s), isolamento de erro (`defer recover()` por goroutine de item), e
  upsert por item (nunca transação única do lote).

### Established Patterns
- `erp_bridge_config`: armazenamento de credencial Oracle criptografada por
  `company_id` — já em produção, só nunca consumido por uma conexão Go
  síncrona (essa é a novidade da Fase 11).
- `nfe_saidas_itens`: schema `public`, sem particionamento, convenção de
  nomes de coluna (`v_prod`, `v_bc_icms`, `v_icms`, etc.) — `
  fiscal_execution_items` deve seguir a mesma convenção pros ~88 campos.

### Integration Points
- `fiscal_execution_items.nfe_item_id` → FK pra `nfe_saidas_itens(id)` ON
  DELETE CASCADE.
- Conexão Oracle nova consome credenciais de `erp_bridge_config` mas abre
  seu próprio `go_ora` connection pool — não reusa a infraestrutura do
  bridge Python.

</code_context>

<specifics>
## Specific Ideas

**Gaps conhecidos, já assumidos como aceitáveis para esta fase (não
bloqueiam implementação — viram erro explícito por item):**
- Só a filial de Recife/PE tem `cod_empresa` mapeado por CNPJ raiz hoje.
  Notas de outras filiais (ex. Garanhuns/PE) retornam erro explícito por
  item até confirmação da raiz de CNPJ correspondente contra o Oracle real.
- Defaults de parâmetros (`defaultTipoContribuinte`, `defaultTipoCentroFiscal`,
  etc., ver `.continue-here.md`) só foram validados contra Oracle real no
  caminho "normal" de venda — Simples Nacional/prestação de serviço podem
  expor default incorreto, mas isso vira divergência visível na tela da
  Fase 12, não trava a Fase 11.

</specifics>

<deferred>
## Deferred Ideas

- Tela "Comparação Fiscal" (TPF-06/07/08) — Fase 12, depende desta fase
  estar executada primeiro.
- Sistema de permissão granular por módulo (substituindo o gate `adminOnly`
  binário) — milestone futura, fora desta fase e desta milestone inteira.
- Otimização de pool de conexão Oracle (compartilhado vs. por-request) —
  Claude's Discretion nesta fase, pode virar fase de otimização futura se
  necessário.

</deferred>

---

*Phase: 11-motor-de-execu-o-do-pacote-fiscal-backend*
*Context gathered: 2026-07-03*
