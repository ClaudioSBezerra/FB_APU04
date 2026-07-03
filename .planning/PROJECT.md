# FB_APU04 — Simulador Fiscal

## Current State

**v6.00 — Módulo Teste Pacote Fiscal: SHIPPED (2026-07-03)**

Backend (Phase 11) resolve grupo fiscal via Oracle prod/PRODB por item de `nfe_saidas_itens`, executa `PKG_FISCAL_FCTAX.calcula_imposto_produto` via bloco PL/SQL 100% estático (bind seguro, zero concatenação) e persiste ~88 campos de saída em `fiscal_execution_items`, em lote, com isolamento de erro por item. Frontend (Phase 12) entrega a tela "Comparação Fiscal": busca de NF-e, execução sob demanda, comparação esperado × calculado das 6 impostos (ICMS/ICMS-ST/PIS/COFINS/IBS/CBS) com tolerância zero, filtro "só divergentes", resumo agregado e exportação Excel/CSV. Navegação nova ("Teste Pacote Fiscal") gated por `adminOnly` — trava temporária até existir permissão granular por módulo. Ver `.planning/milestones/v6.00-ROADMAP.md` para o detalhamento completo.

## Next Milestone Goals

Nenhuma milestone nova iniciada ainda. Candidatos capturados durante o v6.00 (não compromissados):
- Resolver o achado de segurança CR-02 (Phase 08): regras fiscais globais BA/CE editáveis por qualquer usuário autenticado, não só admin
- Validar os 2 cenários de UAT da Fase 11 pendentes contra Oracle prod/PRODB real (credenciais reais só disponíveis em produção)
- Sistema de permissão granular por módulo — hoje só existe o gate binário `adminOnly` (usado por auditoria, malha e agora teste-pacote-fiscal); usuário já sinalizou que vai restringir acesso ao módulo Teste Pacote Fiscal numa fase futura
- Verificar o caminho de tags XML do bloco IBS/CBS por item (`<imposto><IBSCBS><gIBSCBS>`) em `pacotefiscal_xml_import.go` contra um XML real de produção com Reforma Tributária — implementado como melhor esforço, não validado contra amostra real (schema ainda em evolução)

<details>
<summary>Histórico de milestones anteriores</summary>

**v5.00 — Análise da Reforma Tributária: COMPLETA (2026-05-29)**
- Módulo 1.1: Créditos ICMS bloqueados — CST/CFOP de uso/consumo e ativo permanente (EFD C170/C190)
- Módulo 1.2: Reprecificação de produtos — ICMS por dentro → IBS/CBS por fora (XMLs NF-e venda)
- Módulo 1.3: Ranking de fornecedores por crédito IBS/CBS gerado, alerta Simples Nacional
- Módulo 1.4: Split payment — float tributário perdido e custo financeiro de reposição de capital
- Módulo 2.1: Análise por NCM — alíquota efetiva atual vs. IBS+CBS projetada
- Módulo 2.2: Análise por CFOP — impacto por natureza da operação (grupos: uso/consumo, ativo, transferências, exportação)
- Módulo 2.3: Análise por UF/destino — tributação na origem (ICMS) → destino (IBS)
- Módulo 2.4: Segmentação B2B vs. B2C automática por indFinal/CPF/CNPJ

</details>

## What This Is

Plataforma de escrituração e simulação fiscal de entradas/saídas com apuração de impostos (PIS, COFINS, IPI, ICMS) e geração de SPED. Atende a equipe fiscal da Ferreira Costa (e futuros clientes via tenancy lógico Ambiente → Grupo → Empresa) através do domínio público `simu.fbtax.cloud` / `simu.fcxlabs.com`. Hoje em produção com importação de notas via ERP Bridge (Oracle ERP), e evoluindo para suportar também upload manual de XMLs.

## Core Value

A escrituração fiscal precisa ser **completa e auditável** — todos os valores tributários (PIS, COFINS, IPI, ICMS) corretos para cada nota, com rastreabilidade até o documento original (XML ou ERP), pronta para fiscalização da Receita Federal.

## Requirements

### Validated

<!-- Capacidades já em produção, herdadas do mapeamento do codebase em 2026-05-08 -->

- ✓ Importação de notas via ERP Bridge (Oracle ERP, modos `oracle_xml` e `sap_s4hana`) — em produção
- ✓ Tabelas independentes de `nfe_entradas`, `nfe_saidas`, `cte_entradas`, `parceiros` — em produção
- ✓ Enriquecimento de PIS/COFINS/IPI no painel de entradas — em produção
- ✓ Painel comercial com view `vw_nfe_entradas_impostos` (CFOP, ICMS, IPI por nota) — em produção
- ✓ Apuração IBS/CBS (reforma tributária) — em produção
- ✓ Geração SPED layout 020 — em produção
- ✓ Tenancy lógico via hierarquia `environments → enterprise_groups → companies → user_environments` — em produção
- ✓ Frontend React/Vite + backend Go 1.22 + PostgreSQL `fiscal_apu04_db` + Redis — deploy via Coolify+Traefik
- ✓ Auth JWT + RBAC com `role: admin/user` + permissões via Environment Admin — em produção
- ✓ Bridge AWS containerizado (`fbtax-bridge-apu04`) isolado do bridge APU02 — separado em 2026-05-07
- ✓ Z.AI GLM API para relatórios executivos com IA — em produção
- ✓ E-mail estruturado SMTP via Hostinger — em produção
- ✓ Módulo Teste Pacote Fiscal: execução do `PKG_FISCAL_FCTAX.calcula_imposto_produto` via Oracle prod/PRODB, comparação item a item (esperado × calculado) das 6 impostos com divergências destacadas — v6.00 (2026-07-03)

### Active

<!-- Foco dos próximos meses, em ordem de prioridade -->

**Estabilizar (prioridade 1):**
- ✓ Proteção no `ResetDatabaseHandler` — 5 gates (token DELETE-FB_APU04, pg_dump backup, audit log, role gate, rate-limit 1/h) + ALLOWED_DESTRUCTIVE_DBS — Validado em Phase 1 (2026-05-15)
- ✓ Correção cache simu.fcxlabs.com/login (SW órfão FC Bots) — unregister-sw.js + nginx headers — Validado em Phase 1 (2026-05-15)

**Expandir (prioridade 2):**
- [x] Importação de XMLs via upload manual (drag-and-drop) alimentando as mesmas tabelas do ERP Bridge — fonte unificada de identificação tributária
- [x] Módulo Teste Pacote Fiscal — validação item a item do pacote fiscal Oracle contra os valores das notas já importadas — Validado em v6.00 (2026-07-03)

**Demais frentes (prioridade 3+, ordem a definir no roadmap):**
- [ ] Estabilização adicional: tirar credenciais do código (.env, configs AWS), bootstrap de testes Go/React, retry/reconnect no Bridge SAP S4 (DPY-4011)
- [ ] Expansão fiscal: novos relatórios, integrações adicionais com Receita Federal
- [ ] Escalar: performance para mais filiais/clientes, monitoramento (Prometheus/Grafana já provisionados em prod, mas sem dashboards dedicados)
- [ ] Modernizar: avaliar substituição de materialized views por outra estratégia, Redis hoje provisionado mas não consumido pelo backend

### Out of Scope

- **Multi-cliente comercial agressivo neste ciclo** — tenancy lógico já existe, mas vendas para múltiplos clientes externos não é o foco agora — equipe fiscal Ferreira Costa é o caso de uso primário
- **Migração para outro stack (não-Go, não-React, não-PostgreSQL)** — produto está em produção e estável; mudar fundações destruiria valor sem ganho proporcional
- **Reescrita do Bridge em Go** — Python+oracledb funciona e a equipe domina; a complexidade está na lógica fiscal, não na linguagem
- **Reset/limpeza por API sem UI dedicada** — depois do incidente de 2026-05-07, qualquer operação destrutiva requer fluxo UI explícito com confirmações
- **Sistema de permissão por módulo neste ciclo** — v6.00 usa gate binário `adminOnly` como trava temporária; permissão granular por usuário/módulo (config, RT-SPED, XMLs, painel, reforma, fronteira, auditoria, teste pacote fiscal) fica para uma milestone futura dedicada

## Context

**Histórico recente que molda o projeto:**

- **2026-05-07 — Incidente de perda de dados:** APU04 estava configurado com `APU02_DB_HOST` apontando para o banco do APU02. Um clique em "limpar base" no APU04 executou `TRUNCATE TABLE import_jobs CASCADE` no banco do APU02, destruindo 4 meses de NF-e entradas/saídas, CT-e e parceiros. Toda a infraestrutura foi separada nos commits `90d1b93`, `947de42`, `14b455b`. O `ResetDatabase` ainda **não tem proteção** — é a prioridade 1.

- **Bridge AWS em produção:** dois bridges rodando paralelos no servidor `172.16.249.77`:
  - `/opt/apps/fbtax/erp-bridge/` — bare Python, daemon do APU02 (`fctax.fcxlabs.com`)
  - `fbtax-bridge-apu04` Docker container — APU04 (`simu.fcxlabs.com`)
  - Erros `DPY-4011: the database or network closed the connection` aparecem nos logs do Bridge SAP S4/HANA — Oracle derruba conexão por inatividade, sem retry automático além de restart do run.

- **Codebase brownfield**: 7 documentos em `.planning/codebase/` (1920 linhas) mapeiam stack (Go/React/Postgres/Redis), arquitetura (3-tier + bridge externo), conventions, testes (cobertura mínima — 1 arquivo isolado), e concerns (8 itens críticos detalhados em CONCERNS.md).

- **Tenancy lógico já implementado:** hierarquia `environments → enterprise_groups → companies` com `user_environments` ligando usuários. Cada usuário comum **obrigatoriamente** vinculado a esse grafo. Isso permite isolar dados por cliente sem multi-tenancy de banco/schema.

**Decisões já tomadas neste questionamento:**
- XML upload é manual (drag-and-drop), não watch automatizado
- Quando NF-e chega por Oracle Bridge e XML, **XML vence** (fonte SEFAZ é autêntica)
- Estabilizar primeiro (apenas ResetDatabase como crítico), depois XML, depois resto

**v6.00 (Módulo Teste Pacote Fiscal) — shipped 2026-07-03:**
- Backend Go abre conexão síncrona própria ao Oracle via `erp_bridge_config` para o pacote fiscal — não reaproveita o bridge Python, que segue isolado para import de NF-e/CT-e
- `fiscal_execution_items` no modelo híbrido (11 colunas típicas indexáveis + `full_result` JSONB) em vez de ~88 colunas literais — decisão validada em produção nas Fases 11-12
- 4º estado `not_executed` (distinto de `ok`/`error`/`sem_grupo_fiscal`) resolvido via `COALESCE(fei.status, 'not_executed')` no LEFT JOIN — evita confundir "nunca rodou" com "rodou e falhou"
- IBS calculado exposto como soma única `valor_ibs_uf + valor_ibs_mun` (não existe coluna de total na tabela) — resolvido uma vez no SQL, reusado pelo JSON e pelo CSV
- Dívida conhecida ficou registrada em STATE.md § Deferred Items em vez de bloquear o fechamento da milestone (ver seção "Next Milestone Goals" acima)

## Constraints

- **Tech stack**: Go 1.22 (backend), React 18+/Vite/TypeScript (frontend), PostgreSQL 15, Redis 7, Python 3 (bridge) — stack travado, mudar exige justificativa forte
- **Deployment**: Coolify + Traefik + Docker em produção; GitHub Container Registry (`ghcr.io/claudiosbezerra/fb_apu04-*`); Watchtower para auto-update
- **Compatibility**: Bridge AWS roda em Linux, Oracle thin mode (sem Oracle Client) — não pode introduzir dependências que quebrem isso
- **Performance**: SPED de uma filial pode ter 350k+ linhas (visto nos logs); operações de import precisam ser stream/batch, não carregamento total em memória
- **Security**: Pós-incidente, qualquer operação destrutiva (TRUNCATE, DELETE em massa) requer salvaguardas em código, não confiança no usuário
- **Tenancy**: Tudo que tocar dados precisa respeitar `company_id` e o grafo `environments/groups/companies`
- **Idioma**: Comunicação interna e UI em PT-BR; código e nomes técnicos em inglês

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Separar APU04 do APU02 (banco, processo, container, deploy) | Incidente 2026-05-07 destruiu 4 meses de produção | ✓ Good |
| XML wins over Oracle Bridge em conflitos de chave | XML é fonte original SEFAZ, autêntica | — Pending |
| Upload XML manual (não pasta watch) | Volume controlado, mais simples, sem requisitos novos de infra | — Pending |
| Estabilização foca SOMENTE em ResetDatabase | Outros itens (secrets, testes, retry bridge) são importantes mas não bloqueantes | — Pending |
| Multi-cliente externo fora do escopo atual | Tenancy lógico já cobre o caso interno; venda externa é decisão comercial separada | — Pending |
| `--config` no bridge.py permite isolar instâncias APU02/APU04 | Cada config tem seu tracker.db e logs separados | ✓ Good |
| Portar validação do pacote fiscal do FB_TESTESFC para dentro do FB_APU04 (novo módulo) em vez de manter produto standalone | Deploy em Hostinger/Coolify (FB_TESTESFC) não alcança a rede interna Oracle da Ferreira Costa (IPs privados `10.131.x.x`); FB_APU04 já roda com acesso Oracle | ✓ Good — shipped v6.00 |
| ~~Reaproveitar `nfe_saidas`/`nfe_saidas_itens` existentes como fonte de dados~~ — **REVERTIDA em 2026-07** | Decisão original: granularidade item-a-item já suficiente, evita duplicar upload/parse. Motivo da reversão (usuário): reduzir raio de impacto — bug na importação/schema deste módulo não pode afetar Painel XMLs/Conciliação/Auditoria; acesso a este módulo será restringido em fase futura, mais barato isolar agora do que depois | ⚠️ Revisit — substituída por pipeline isolado (ver linha abaixo) |
| Pipeline de importação de XML isolado: `pacotefiscal_nfe_saidas`/`pacotefiscal_nfe_saidas_itens` dedicados (migration 148), com cabeçalho completo de emit/dest (razão social, IE, endereço, contato — nfe_saidas só tinha nome/UF/município) | Isolamento de dados/tipos (não Go package separado — reusa helpers genéricos já testados de charset/ZIP/decimal). `fiscal_execution_items.nfe_item_id` repontado para a nova tabela de itens (TRUNCATE de execuções de teste pré-existentes, sem valor histórico) | ✓ Good — migration validada e2e contra Postgres descartável antes do deploy |
| Gate `adminOnly` binário para o módulo Teste Pacote Fiscal, em vez de permissão granular | Evitar bloquear a entrega atrás de uma infra de permissões maior e separada | ⚠️ Revisit — candidato a milestone futura (ver Next Milestone Goals) |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-07-03 — após milestone v6.00 (Módulo Teste Pacote Fiscal) shipped*
