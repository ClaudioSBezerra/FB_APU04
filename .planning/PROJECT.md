# FB_APU04 — Simulador Fiscal

## Current Milestone: v5.00 — Análise da Reforma Tributária

**Goal:** Entregar dois módulos analíticos sobre dados já importados (EFD ICMS/IPI + XMLs NF-e), identificando oportunidades de crédito IBS/CBS e impactos por produto, CFOP, UF e segmento de cliente.

**Target features:**
- Módulo 1.1: Créditos ICMS bloqueados — CST/CFOP de uso/consumo e ativo permanente (EFD C170/C190)
- Módulo 1.2: Reprecificação de produtos — ICMS por dentro → IBS/CBS por fora (XMLs NF-e venda)
- Módulo 1.3: Ranking de fornecedores por crédito IBS/CBS gerado, alerta Simples Nacional
- Módulo 1.4: Split payment — float tributário perdido e custo financeiro de reposição de capital
- Módulo 2.1: Análise por NCM — alíquota efetiva atual vs. IBS+CBS projetada
- Módulo 2.2: Análise por CFOP — impacto por natureza da operação (grupos: uso/consumo, ativo, transferências, exportação)
- Módulo 2.3: Análise por UF/destino — tributação na origem (ICMS) → destino (IBS)
- Módulo 2.4: Segmentação B2B vs. B2C automática por indFinal/CPF/CNPJ

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

### Active

<!-- Foco dos próximos meses, em ordem de prioridade -->

**Estabilizar (prioridade 1):**
- ✓ Proteção no `ResetDatabaseHandler` — 5 gates (token DELETE-FB_APU04, pg_dump backup, audit log, role gate, rate-limit 1/h) + ALLOWED_DESTRUCTIVE_DBS — Validado em Phase 1 (2026-05-15)
- ✓ Correção cache simu.fcxlabs.com/login (SW órfão FC Bots) — unregister-sw.js + nginx headers — Validado em Phase 1 (2026-05-15)

**Expandir (prioridade 2):**
- [ ] Importação de XMLs via upload manual (drag-and-drop) alimentando as mesmas tabelas do ERP Bridge — fonte unificada de identificação tributária

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
*Last updated: 2026-05-22 — Milestone v5.00 iniciado*
