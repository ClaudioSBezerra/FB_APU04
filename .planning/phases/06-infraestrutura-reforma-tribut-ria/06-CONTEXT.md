# Phase 6: Infraestrutura Reforma Tributária - Context

**Gathered:** 2026-05-22
**Status:** Ready for planning

<domain>
## Phase Boundary

Entregar a infraestrutura de dados e configuração que os módulos analíticos das Phases 7 e 8 consumirão: novas colunas em `reg_c190` e `nfe_saidas`, tabela `reforma_parametros`, seed de CFOPs de transferência, endpoints backend e hook frontend para parâmetros, e a entrada da "Reforma Tributária" na navegação com página de configuração.

Nenhum módulo analítico (créditos, reprecificação, ranking, split payment, CFOP/NCM/UF) é entregue nesta fase — só as fundações.

</domain>

<decisions>
## Implementation Decisions

### Navegação

- **D-01:** Criar novo módulo `reforma` no `navigation.ts` com label **"Reforma Tributária"** e path base `/reforma`.
- **D-02:** Na Phase 6, o módulo tem apenas uma tab ativa: **"Parâmetros"** em `/reforma/parametros`. As tabs dos módulos 1.x e 2.x serão adicionadas nas Phases 7 e 8 com `disabled: true` como placeholder já visível na sidebar.
- **D-03:** A função `getActiveModule` em `navigation.ts` deve retornar `'reforma'` para rotas que começam com `/reforma`.

### Página de Parâmetros

- **D-04:** A rota `/reforma/parametros` e `/config/reforma-parametros` renderizam **o mesmo componente** (`ReformaParametros.tsx`). A tab no módulo "Reforma Tributária" aponta para `/reforma/parametros`; uma aba nova "Parâmetros Reforma" no módulo `config` (em `navigation.ts`) aponta para `/config/reforma-parametros`.
- **D-05:** UX: **card com campos inline editáveis + botão Salvar**, seguindo o padrão visual de `TabelaAliquotas` e `ERPBridgeConfig`. Não é modal.
- **D-06:** Usuários não-admin veem a página em **somente leitura** — campos desabilitados, botão Salvar oculto. Sem redirecionamento.
- **D-07:** Validação de acesso de escrita via role `admin` no backend (`PUT /api/reforma/parametros` retorna 403 para não-admins).

### Disclaimer fator Simples Nacional

- **D-08:** Exibir **tooltip/ícone ⓘ** ao lado do label do campo `fator_simples_pct` com o texto: _"Valor estimado. Alíquota definitiva ainda não publicada pelo CG-IBS."_. Não é banner, não é modal, não é alerta fixo.

### Dados Históricos (reg_c190 e nfe_saidas)

- **D-09:** Registros históricos de `reg_c190` sem `cst_icms`/`aliq_icms` (NULL) e de `nfe_saidas` sem `ind_final` (NULL) **não precisam de aviso visual** nos módulos futuros. NULL é tratado como ausência de dado — os módulos 1.x/2.x usarão fallback (CPF/CNPJ para `ind_final`, como já documentado em RFMA-03).

### Schema e Backend

- **D-10:** As migrations seguem a sequência existente (`085_...sql` → `086_...sql`, etc.) com arquivos separados por concern (ex.: `086_add_cst_aliq_icms_to_reg_c190.sql`, `087_create_reforma_parametros.sql`, `088_add_ind_final_to_nfe_saidas.sql`, `089_seed_cfop_transferencias.sql`).
- **D-11:** O handler `reforma_config.go` segue o padrão de `config.go` — closure `db *sql.DB`, extrai `company_id` do contexto via middleware existente, retorna JSON.
- **D-12:** O hook `useReformaParametros.ts` usa `useQuery` do `@tanstack/react-query` — mesmo padrão dos hooks existentes. Compartilhado globalmente (não por módulo).

### Claude's Discretion

- Nomes exatos das migrations (numeração, nomenclatura) — seguir sequência e convenção existente.
- Estrutura interna de `ReformaParametros.tsx` — implementar seguindo `TabelaAliquotas.tsx` como referência mais próxima.
- Posição exata da tab "Parâmetros Reforma" no módulo `config` (antes ou depois de "Alíquotas").
- Tabs placeholder para módulos 1.x/2.x: labels e paths seguirão o ROADMAP, mas nomes exatos são discrição do implementador.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Schema existente
- `backend/migrations/010_update_schema_efd_icms.sql` — estrutura atual de `reg_c190` (colunas existentes, constraints)
- `backend/migrations/085_fix_vw_xml_entradas_informativos_pis_cofins.sql` — migration mais recente (referência de numeração)

### Padrões de handler Go
- `backend/handlers/config.go` — padrão de GET handler com `db *sql.DB`, JSON encode
- `backend/handlers/rfb_credentials.go` — padrão de GET+PUT com validação de role admin
- `backend/handlers/auth.go` — extração de company_id e role do contexto JWT

### Padrões de navegação e roteamento frontend
- `frontend/src/lib/navigation.ts` — estrutura do `modules` record, tabs, `getActiveModule`
- `frontend/src/App.tsx` — padrão de import/Route para novas páginas

### Padrões de UI
- `frontend/src/pages/TabelaAliquotas.tsx` (ou similar) — card com inline edit, referência visual mais próxima para `ReformaParametros.tsx`

### Requisitos
- `.planning/REQUIREMENTS.md` — seção RFMA-01 a RFMA-08 com especificações exatas de colunas, tabelas e campos

### Parser EFD
- `backend/handlers/` — localizar `xml_upload.go` e o worker que processa reg_c190 para RFMA-01 (posições `parts[2]` e `parts[4]` do C190)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `backend/handlers/config.go` → padrão direto para `reforma_config.go` (GET alíquotas → GET parâmetros)
- `frontend/src/lib/navigation.ts` → adicionar módulo `reforma` ao `modules` record e case em `getActiveModule`
- Componente de tooltip/ícone ⓘ já usado em outras telas (confirmar durante implementação)
- `@tanstack/react-query` `useQuery` → base para `useReformaParametros.ts`

### Established Patterns
- Migrations nomeadas `NNN_descricao_snake_case.sql` — criar 4 arquivos separados (reg_c190, reforma_parametros, nfe_saidas, seed cfop)
- Handlers Go: closure com `db *sql.DB`, sem global state, retornam JSON
- Auth middleware já injeta `company_id` no contexto — handlers extraem via helper existente
- `fetch()` global interceptado pelo `AuthContext` injeta `Authorization` e `X-Company-ID` automaticamente — hook não precisa gerenciar cabeçalhos

### Integration Points
- `xml_upload.go` / worker EFD → adicionar parse de `cst_icms` e `aliq_icms` para RFMA-01
- `main.go` ou equivalente → registrar rotas `GET /api/reforma/parametros` e `PUT /api/reforma/parametros`
- `frontend/src/App.tsx` → import + Route para `/reforma/parametros` e `/config/reforma-parametros`

</code_context>

<specifics>
## Specific Ideas

- RFMA-08 (`react-simple-maps` v3.0.0 + `frontend/public/brazil-states.json`) é puro setup de dependência para Phase 8 — instalar e commitar o TopoJSON sem criar página ou componente ainda.
- Ao instalar `react-simple-maps`, confirmar compatibilidade com Vite (pode precisar de configuração de alias ou plugin).

</specifics>

<deferred>
## Deferred Ideas

- Módulos analíticos (1.1 créditos, 1.2 reprecificação, 1.3 ranking, 1.4 split payment, 2.1–2.4 analíticos) → Phases 7 e 8.
- Backfill de `cst_icms`/`aliq_icms` para registros históricos do EFD — requereria reimportação de SPEDs; não é prioridade agora.
- Aviso visual para registros históricos com NULL — aguardar feedback dos usuários ao usar os módulos 1.x/2.x.

</deferred>

---

*Phase: 6-infraestrutura-reforma-tributaria*
*Context gathered: 2026-05-22*
