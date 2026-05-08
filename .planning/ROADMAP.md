# Roadmap: FB_APU04 — Simulador Fiscal

## Overview

Estabilizar o produto pós-incidente de 2026-05-07 (perda de 4 meses de produção do APU02), entregar a nova capacidade de upload XML como segunda fonte de dados, depois evoluir incrementalmente nas frentes de qualidade, observabilidade e expansão fiscal — sem comprometer estabilidade conquistada.

## Phases

- [ ] **Phase 1: Estabilização Crítica (Reset + Cache)** - Proteger ResetDatabase e resolver bug de cache em simu.fcxlabs.com/login
- [ ] **Phase 2: Upload de XMLs (Drag-and-Drop)** - Segunda fonte de dados alimentando as mesmas tabelas do ERP Bridge, com prioridade do XML em conflitos
- [ ] **Phase 3: Estabilização Adicional** - Tirar credenciais do código, bootstrap de testes Go/React, retry/reconnect no Bridge SAP S4
- [ ] **Phase 4: Conciliação Bridge vs XML** - Relatório de divergências e dashboard de cobertura por fonte
- [ ] **Phase 5: Observabilidade e Alertas** - Dashboards Grafana e alertas críticos via Prometheus já provisionado

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
**Plans**: 2-3 plans

Plans:
- [x] 01-01: Proteções backend no ResetDatabaseHandler (confirmação, backup, audit, role, rate-limit)
- [ ] 01-02: UI de confirmação destrutiva no frontend com avisos visuais explícitos
- [x] 01-03: Diagnóstico e correção do cache stale em simu.fcxlabs.com/login

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
**Plans**: 3-4 plans

Plans:
- [ ] 02-01: Schema migrations (coluna source, tabela upload_history) + parser NFe v4.00
- [ ] 02-02: Backend handler de upload + worker de processamento batch + endpoint de histórico
- [ ] 02-03: Frontend tela drag-and-drop + visualização de progresso + histórico
- [ ] 02-04: Lógica de conflito Oracle vs XML + testes de integração

### Phase 3: Estabilização Adicional
**Goal**: Reduzir dívida técnica em segredos, testes e resilência do bridge — pré-requisitos para escalar com confiança.
**Depends on**: Phase 1
**Requirements**: STAB-06, STAB-07, STAB-08, STAB-09
**Success Criteria** (what must be TRUE):
  1. Nenhum segredo real em arquivos .env* versionados
  2. Pipeline CI executa testes Go e React em cada PR
  3. Cobertura Go ≥30% nos pacotes handlers/ e services/
  4. Bridge SAP sobrevive a DPY-4011 reconectando sem intervenção humana
**Plans**: 3-4 plans

Plans:
- [ ] 03-01: Migração de secrets para Coolify env vars + auditoria de exposições
- [ ] 03-02: Bootstrap de testes Go (handlers críticos com cobertura inicial 30%)
- [ ] 03-03: Bootstrap de testes React (Vitest + componentes-chave)
- [ ] 03-04: Retry/reconnect no bridge.py para DPY-4011 com retomada via tracker

### Phase 4: Conciliação Bridge vs XML
**Goal**: Aproveitar as duas fontes de dados (Bridge + XML) para gerar valor fiscal direto: conciliação, divergências, cobertura.
**Depends on**: Phase 2
**Requirements**: EXP-01, EXP-02
**Success Criteria** (what must be TRUE):
  1. Relatório de divergências gerado para qualquer período/filial em <10s
  2. Dashboard de cobertura mostra % NF-es com fonte XML por filial/mês
  3. Auditor consegue exportar relatório completo em PDF/Excel
  4. Divergências mostram delta tributário detalhado (PIS, COFINS, IPI, ICMS)
**Plans**: 2-3 plans

Plans:
- [ ] 04-01: Backend service de conciliação + endpoint de divergências
- [ ] 04-02: Frontend dashboard de cobertura + exportação PDF/Excel

### Phase 5: Observabilidade e Alertas
**Goal**: Aproveitar Prometheus já provisionado para ganhar visibilidade operacional — dashboards e alertas para os fluxos críticos.
**Depends on**: Phase 3
**Requirements**: OBS-01, OBS-02
**Success Criteria** (what must be TRUE):
  1. Dashboards Grafana funcionais (runs Bridge, latência API, erro 5xx, tamanho DB)
  2. Cada alerta crítico dispara em <1min após o evento
  3. Runbook escrito para cada tipo de alerta com passos de mitigação
  4. Equipe fiscal vê status do Bridge sem acesso SSH
**Plans**: 2-3 plans

Plans:
- [ ] 05-01: Setup de Prometheus exporters + dashboards Grafana
- [ ] 05-02: Alertas críticos + integração SMTP/Slack + runbooks

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Estabilização Crítica (Reset + Cache) | 2/3 | In Progress|  |
| 2. Upload de XMLs (Drag-and-Drop) | 0/4 | Not started | - |
| 3. Estabilização Adicional | 0/4 | Not started | - |
| 4. Conciliação Bridge vs XML | 0/2 | Not started | - |
| 5. Observabilidade e Alertas | 0/2 | Not started | - |

---
*Roadmap created: 2026-05-08*
