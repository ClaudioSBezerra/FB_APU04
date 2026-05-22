# Roadmap: FB_APU04 — Simulador Fiscal

## Overview

Estabilizar o produto pós-incidente de 2026-05-07 (perda de 4 meses de produção do APU02), entregar a nova capacidade de upload XML como segunda fonte de dados, depois evoluir incrementalmente nas frentes de qualidade, observabilidade e expansão fiscal — sem comprometer estabilidade conquistada.

## Phases

- [x] **Phase 1: Estabilização Crítica (Reset + Cache)** - Proteger ResetDatabase e resolver bug de cache em simu.fcxlabs.com/login (completed 2026-05-08)
- [x] **Phase 2: Upload de XMLs (Drag-and-Drop)** - Segunda fonte de dados alimentando as mesmas tabelas do ERP Bridge, com prioridade do XML em conflitos (completed 2026-05-16)
- [x] **Phase 3: Estabilização Adicional** - Tirar credenciais do código, bootstrap de testes Go/React, retry/reconnect no Bridge SAP S4 (completed 2026-05-16)
- [x] **Phase 4: Conciliação Bridge vs XML** - Relatório de divergências e dashboard de cobertura por fonte (completed 2026-05-16)
- [x] **Phase 5: Observabilidade e Alertas** - Provisionar Prometheus + Grafana + Alertmanager, instrumentar Go/Python, dashboards e alertas críticos via SMTP (completed 2026-05-17)

---
*Milestone v5.00 — Análise da Reforma Tributária (início: 2026-05-22)*

- [ ] **Phase 6: Infraestrutura Reforma Tributária** - Schema blockers (reg_c190 + cst_icms/aliq_icms, ind_final em nfe_saidas), tabela reforma_parametros, seed CFOPs transferência, endpoints config, hook frontend, navegação e página de configuração de parâmetros
- [ ] **Phase 7: Módulos 1.x — Exposição Tributária Direta** - Créditos ICMS bloqueados (1.1), ranking fornecedores IBS/CBS (1.3), reprecificação de produtos (1.2), split payment capital de giro (1.4)
- [ ] **Phase 8: Módulos 2.x — Analytics Dimensional** - Por CFOP (2.2), por NCM (2.1), por UF/destino com mapa coroplético (2.3), segmentação B2B vs. B2C (2.4)

## Phase Details

### Phase 1: Estabilização Crítica (Reset + Cache)

**Goal**: Tornar impossível repetir o incidente de 2026-05-07 e resolver o bug onde simu.fcxlabs.com/login serve a página do app anterior (FC Bots) na primeira visita.
**Depends on**: Nothing (first phase)
**Requirements**: STAB-01, STAB-02, STAB-03, STAB-04, STAB-05, STAB-10
**Success Criteria** (what must be TRUE):

  1. ResetDatabase não executa sem token de confirmação textual digitado pelo usuário
  2. Backup automático gerado antes de qualquer TRUNCATE em /backups/reset-{timestamp}.sql
  3. Audit log de toda execução de reset gravado em admin_destructive_actions
  4. Apenas role admin global executa reset completo (Environment Admin restrito a ResetCompanyData)
  5. Tentativas de reset em <1h retornam erro 429
  6. Usuário acessando simu.fcxlabs.com/login pela primeira vez vê tela do FB_APU04 (não do FC Bots) sem precisar de Ctrl+Shift+R

**Plans**: 3 plans

Plans:

- [x] 01-01-PLAN.md — Proteções backend no ResetDatabaseHandler (confirmação, backup, audit, role, rate-limit)
- [x] 01-02-PLAN.md — UI de confirmação destrutiva no frontend com avisos visuais explícitos
- [x] 01-03-PLAN.md — Diagnóstico e correção do cache stale em simu.fcxlabs.com/login

### Phase 2: Upload de XMLs (Drag-and-Drop)

**Goal**: Adicionar segunda fonte de dados (XML SEFAZ) alimentando as mesmas tabelas do ERP Bridge, com prioridade do XML em conflitos por chave de acesso.
**Depends on**: Phase 1
**Requirements**: XML-01, XML-02, XML-03, XML-04, XML-05, XML-06, XML-07, XML-08
**Success Criteria** (what must be TRUE):

  1. Upload de 1 XML válido aparece nas tabelas nfe_entradas/saidas em <5s
  2. Upload de ZIP com 1000 XMLs processa em <2min sem bloquear UI
  3. XMLs malformados rejeitados com motivo legível
  4. Conflito Oracle vs XML: campos tributários sobrescritos por XML, source atualizado para xml_upload
  5. Histórico de uploads consultável por filial/período

**Plans**: 4 plans

Plans:

- [x] 02-01-PLAN.md — Schema migrations (source, itens, batches, regime_tributario, views XML)
- [x] 02-02-PLAN.md — Backend parser NFe estendido (itens, CRT), handler upload unificado, worker assíncrono, lógica de prioridade XML>Oracle
- [x] 02-03-PLAN.md — Frontend drag-and-drop, painel XML, campo regime tributário, navegação atualizada
- [x] 02-04-PLAN.md — Relatórios de saneamento CCLASSTRIB + exportação CSV + fornecedores com classificação incorreta (wave 4, depende de 02-03)

### Phase 3: Estabilização Adicional

**Goal**: Reduzir dívida técnica em segredos, testes e resilência do bridge — pré-requisitos para escalar com confiança.
**Depends on**: Phase 1
**Requirements**: STAB-06, STAB-07, STAB-08, STAB-09
**Success Criteria** (what must be TRUE):

  1. Nenhum segredo real em arquivos .env* versionados
  2. Pipeline CI executa testes Go e React em cada PR
  3. Cobertura Go >=30% no pacote handlers/ (conforme STAB-07)
  4. Bridge SAP sobrevive a DPY-4011 reconectando sem intervenção humana

**Plans**: 4 plans

Plans:

- [x] 03-01-PLAN.md — Remoção de credenciais hardcoded: backend/.env, installer/.env, erp-bridge-aws/config-apu04.yaml
- [x] 03-02-PLAN.md — Bootstrap de testes Go: AuthMiddleware, ERPBridgeBatchImportHandler, rateLimiter (cobertura >=30%)
- [x] 03-03-PLAN.md — Bootstrap de testes React: formatFilial.ts (11 funções puras) e navigation.ts (getActiveModule)
- [x] 03-04-PLAN.md — Retry/reconnect Oracle no bridge.py: _connect_oracle() + _is_dpy4011() + substituição nos dois sites

### Phase 4: Conciliação Bridge vs XML

**Goal**: Aproveitar as duas fontes de dados (Bridge + XML) para gerar valor fiscal direto: conciliação, divergências, cobertura.
**Depends on**: Phase 2
**Requirements**: EXP-01, EXP-02
**Success Criteria** (what must be TRUE):

  1. Relatório de divergências gerado para qualquer período/filial em <10s
  2. Dashboard de cobertura mostra % NF-es com fonte XML por filial/mês
  3. Auditor consegue exportar relatório completo em PDF/Excel
  4. Divergências mostram delta tributário detalhado (PIS, COFINS, IPI, ICMS)

**Plans**: 2 plans

Plans:

- [x] 04-01-PLAN.md — Backend: ConciliacaoHandler + CoberturaHandler + ConciliacaoCSVHandler em xml_conciliacao.go + 3 rotas em main.go (wave 1)
- [x] 04-02-PLAN.md — Frontend: ConciliacaoBridgeXML.tsx (tabs Divergências + Cobertura XML, exportação Excel/CSV/PDF) + navigation.ts + App.tsx (wave 2, depende de 04-01)

**Wave 2** *(bloqueada até Wave 1 concluída)*

Cross-cutting constraints:

- `company_id` isolamento via `GetEffectiveCompanyID` obrigatório em ambos handlers
- Threshold de divergência fixo em R$ 0,01 (exibido na UI como legenda)
- Notas canceladas (`cancelado != 'S'`) excluídas de divergências e cobertura

### Phase 5: Observabilidade e Alertas

**Goal**: Provisionar do zero a stack Prometheus + Grafana + Alertmanager (research confirmou que NÃO está provisionada apesar do que ROADMAP/PROJECT.md afirmavam), instrumentar backend Go e bridge Python, entregar dashboards visíveis para a equipe fiscal sem SSH e alertas críticos em <1min via SMTP.
**Depends on**: Phase 3
**Requirements**: OBS-01, OBS-02
**Success Criteria** (what must be TRUE):

  1. Dashboards Grafana funcionais (runs Bridge, latência API, erro 5xx, tamanho DB)
  2. Cada alerta crítico dispara em <1min após o evento
  3. Runbook escrito para cada tipo de alerta com passos de mitigação
  4. Equipe fiscal vê status do Bridge sem acesso SSH

**Plans**: 2 plans

Plans:
**Wave 1**

- [x] 05-01-PLAN.md — Provisionar Prometheus/Grafana/Alertmanager/postgres-exporter + instrumentar Go (/metrics + counters em erp_bridge/xml_upload/admin) + instrumentar bridge.py (porta 8086) + 3 dashboards JSON (wave 1)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 05-02-PLAN.md — 6 alertas críticos (DPY-4011, BridgeOffline, BridgeDaemonDown, XMLUploadFalha, ResetBancoExecutado, DBTamanhoAlto) + Alertmanager SMTP via awk-envsubst + 5 runbooks pt-BR + validação end-to-end (3 emails SMTP enviados, 0 falhas) (wave 2)

### Phase 6: Infraestrutura Reforma Tributária

**Goal**: Resolver os 4 gaps críticos de schema e criar a infraestrutura de configuração que desbloqueia todas as análises dos módulos 1.x e 2.x. Nenhuma saída visível ao usuário além da navegação e página de parâmetros — fundação para fases 7 e 8.
**Depends on**: Phase 5
**Requirements**: RFMA-01, RFMA-02, RFMA-03, RFMA-04, RFMA-05, RFMA-06, RFMA-07, RFMA-08
**Success Criteria** (what must be TRUE):

  1. `reg_c190` possui `cst_icms` e `aliq_icms` populados por reimportação de EFD
  2. `reforma_parametros` aceita e persiste alíquotas por empresa
  3. `nfe_saidas` possui `ind_final`; novas importações XML populam o campo
  4. CFOPs 1151/1152/2151/2152/5151/5152/6151/6152 têm `tipo='T'` na tabela `cfop`
  5. `GET /api/reforma/parametros` e `PUT /api/reforma/parametros` respondem corretamente
  6. Frontend exibe aba "Análise Reforma" na navegação com página de configuração editável

**Plans:** 4 plans

Plans:
**Wave 1**

- [ ] 06-01-PLAN.md — Migrations 086–089 (DDL): cst_icms/aliq_icms em reg_c190, reforma_parametros, ind_final em nfe_saidas, seed CFOPs transferência (wave 1)

**Wave 2** *(blocked on Wave 1 completion)*

- [ ] 06-02-PLAN.md — Parsers: worker.go grava cst_icms/aliq_icms do C190 + nfe_saidas.go grava ind_final do XML (wave 2)
- [ ] 06-03-PLAN.md — Backend: reforma_config.go (GET/PUT /api/reforma/parametros) + rota em main.go (wave 2)

**Wave 3** *(blocked on Wave 2 completion)*

- [ ] 06-04-PLAN.md — Frontend: useReformaParametros.ts + ReformaParametros.tsx + navigation.ts + App.tsx + react-simple-maps + brazil-states.json (wave 3)

### Phase 7: Módulos 1.x — Exposição Tributária Direta

**Goal**: Entregar os 4 módulos que respondem "qual é a nossa exposição tributária direta na reforma?" — créditos bloqueados, ranking de fornecedores, reprecificação e impacto de capital de giro do split payment.
**Depends on**: Phase 6
**Requirements**: RFMB-01, RFMB-02, RFMB-03, RFMB-04
**Success Criteria** (what must be TRUE):

  1. Módulo 1.1 exibe créditos ICMS bloqueados por CFOP com projeção IBS/CBS; exporta CSV
  2. Módulo 1.3 exibe ranking de fornecedores com alerta Simples Nacional e disclaimer regulatório
  3. Módulo 1.2 calcula reprecificação por produto com três caminhos de CST; exporta CSV
  4. Módulo 1.4 calcula float tributário e custo CDI com tabela de sensibilidade DSO × CDI
  5. Todos os handlers filtram cancelados (`cod_sit NOT IN ('02','03','04','05')`) e transferências (`tipo != 'T'`)

Plans:

- [ ] 07-01-PLAN.md — Backend: reforma_modulo1.go (4 handlers: creditosBloqueados, rankingFornecedores, reprecificacao, splitPayment)
- [ ] 07-02-PLAN.md — Frontend: Reforma11CreditosBloqueados.tsx + Reforma13RankingFornecedores.tsx + Reforma12Reprecificacao.tsx + Reforma14SplitPayment.tsx

### Phase 8: Módulos 2.x — Analytics Dimensional

**Goal**: Entregar os 4 módulos de análise dimensional cruzada — por CFOP, NCM, UF/destino com mapa coroplético, e segmentação B2B vs. B2C.
**Depends on**: Phase 7
**Requirements**: RFMC-01, RFMC-02, RFMC-03, RFMC-04
**Success Criteria** (what must be TRUE):

  1. Módulo 2.2 agrupa operações por grupo CFOP funcional com impacto IBS/CBS por grupo
  2. Módulo 2.1 exibe alíquota ICMS efetiva vs. IBS+CBS projetada por NCM com flag IS
  3. Módulo 2.3 exibe tabela UF + mapa coroplético colorido por volume de impacto
  4. Módulo 2.4 segmenta B2B/B2C em três vias (b2b_credit/b2b_nocredit/b2c) com nota sobre notas históricas sem `ind_final`

Plans:

- [ ] 08-01-PLAN.md — Backend: reforma_modulo2.go (4 handlers: analiseCfop, analiseNcm, analiseUf, segmentacaoBb2Bc)
- [ ] 08-02-PLAN.md — Frontend: Reforma22Cfop.tsx + Reforma21Ncm.tsx + Reforma23Uf.tsx (tabela + mapa) + Reforma24B2bB2c.tsx

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Estabilização Crítica (Reset + Cache) | 3/3 | Complete | 2026-05-08 |
| 2. Upload de XMLs (Drag-and-Drop) | 4/4 | Complete | 2026-05-16 |
| 3. Estabilização Adicional | 4/4 | Complete | 2026-05-16 |
| 4. Conciliação Bridge vs XML | 2/2 | Complete | 2026-05-16 |
| 5. Observabilidade e Alertas | 2/2 | Complete | 2026-05-17 |
| 6. Infraestrutura Reforma Tributária | 0/4 | Planned | — |
| 7. Módulos 1.x — Exposição Tributária Direta | 0/2 | Planned | — |
| 8. Módulos 2.x — Analytics Dimensional | 0/2 | Planned | — |

---
*Roadmap created: 2026-05-08*
*Phase 2 planned: 2026-05-16*
*Phase 3 planned: 2026-05-16*
*Phase 4 planned: 2026-05-16*
*Phase 5 planned: 2026-05-16*
*Phases 6–8 planned: 2026-05-22 (milestone v5.00)*
*Phase 6 plans finalized: 2026-05-22 (4 plans, 3 waves)*
