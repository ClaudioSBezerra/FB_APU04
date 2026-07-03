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

- [x] **Phase 6: Infraestrutura Reforma Tributária** - Schema blockers (reg_c190 + cst_icms/aliq_icms, ind_final em nfe_saidas), tabela reforma_parametros, seed CFOPs transferência, endpoints config, hook frontend, navegação e página de configuração de parâmetros (completed 2026-05-22)
- [x] **Phase 7: Módulos 1.x — Exposição Tributária Direta** - Créditos ICMS bloqueados (1.1), ranking fornecedores IBS/CBS (1.3), reprecificação de produtos (1.2), split payment capital de giro (1.4) (completed 2026-05-23)
- [x] **Phase 8: Cadastro de Empresas + Ambiente Administrativo por UF** - Cadastro completo de empresa (CNPJ/IE/CNAE), gestão multi-empresa na UI, ambiente administrativo por UF com regras ICMS-Fronteira (PE/BA/CE) (completed 2026-05-23)
- [x] **Phase 9: Módulos 2.x — Analytics Dimensional** - Por CFOP (2.2), por NCM (2.1), por UF/destino com mapa coroplético (2.3), segmentação B2B vs. B2C (2.4) (completed 2026-05-23)

---
*Milestone v6.00 — ICMS Fronteira (Substituição Tributária / Antecipação)*

- [x] **Phase 10: ICMS Fronteira — ST por NCM no Bloco C** - Classificação automática de ST pelo NCM (regra + segmento da empresa na UF) para NFs em XML sem SPED, independentemente do CFOP do fornecedor; fecha o caso de fornecedor sem protocolo CONFAZ (CFOP 6101/6102); remove a tela de reclassificação manual (completed 2026-06-27, UAT 5/5 em 2026-06-28)

---
*Milestone v6.00 — Módulo Teste Pacote Fiscal (início: 2026-07-03)*

- [x] **Phase 11: Motor de Execução do Pacote Fiscal (Backend)** - Lookup de grupo fiscal via Oracle, execução do PKG_FISCAL_FCTAX via PL/SQL estático com bind seguro, tabela fiscal_execution_items, endpoint de execução em lote com concorrência limitada e isolamento de erro por item (completed 2026-07-03)
- [x] **Phase 12: Tela Comparação Fiscal + Navegação** - Tela item a item esperado vs. calculado com divergências destacadas, filtro "só divergentes", resumo agregado e item de navegação com gate adminOnly (completed 2026-07-03)

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

**Plans:** 4/4 plans complete

Plans:
**Wave 1**

- [x] 06-01-PLAN.md — Migrations 086–089 (DDL): cst_icms/aliq_icms em reg_c190, reforma_parametros, ind_final em nfe_saidas, seed CFOPs transferência (wave 1)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 06-02-PLAN.md — Parsers: worker.go grava cst_icms/aliq_icms do C190 + nfe_saidas.go grava ind_final do XML (wave 2)
- [x] 06-03-PLAN.md — Backend: reforma_config.go (GET/PUT /api/reforma/parametros) + rota em main.go (wave 2)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 06-04-PLAN.md — Frontend: useReformaParametros.ts + ReformaParametros.tsx + navigation.ts + App.tsx + react-simple-maps + brazil-states.json (wave 3)

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

**Plans:** 2/2 plans complete

Plans:
**Wave 1**

- [x] 07-01-PLAN.md — Backend: reforma_modulo1.go (4 handlers JSON + 3 CSV: creditosBloqueados, rankingFornecedores, reprecificacao, splitPayment) + 7 rotas em main.go

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 07-02-PLAN.md — Frontend: 4 páginas (Reforma11/12/13/14) + ativação de tabs em navigation.ts + 4 rotas em App.tsx + checkpoint de verificação visual

### Phase 8: Cadastro de Empresas + Ambiente Administrativo por UF

**Goal**: Completar o cadastro de empresas (CNPJ/IE/CNAE/município/segmento), criar tela de gestão multi-empresa, e expandir o ambiente administrativo de ICMS-Fronteira para suportar múltiplos estados (BA, CE além do PE já existente) com regras por UF, MVA ajustado e inaplicabilidades.
**Depends on**: Phase 7
**Requirements**: CADU-01, CADU-02, CADU-03, CADU-04, CADU-05, CADU-06, CADU-07

**Success Criteria** (what must be TRUE):

  1. Tabela `companies` possui cnpj, inscricao_estadual, cnae_principal, municipio, segmento_economico, incentivos_fiscais
  2. `CreateCompanyHandler` e `UpdateCompanyHandler` aceitam e persitem os novos campos
  3. Frontend exibe tela de cadastro/edição de empresa com todos os novos campos
  4. `icms_fronteira_regras_ncm` possui coluna `uf_estado` e colunas MVA ajustado (4/7/12%)
  5. Tabela `icms_fronteira_inaplicabilidades` criada com seed para inaplicabilidades conhecidas
  6. Seed inicial de regras para BA e CE (além do PE já existente)
  7. Frontend de configuração ICMS-Fronteira exibe abas por UF (PE / BA / CE) com edição inline

Plans:

### Phase 9: Módulos 2.x — Analytics Dimensional

**Goal**: Entregar os 4 módulos de análise dimensional cruzada — por CFOP, NCM, UF/destino com mapa coroplético, e segmentação B2B vs. B2C.
**Depends on**: Phase 8
**Requirements**: RFMC-01, RFMC-02, RFMC-03, RFMC-04
**Success Criteria** (what must be TRUE):

  1. Módulo 2.2 agrupa operações por grupo CFOP funcional com impacto IBS/CBS por grupo
  2. Módulo 2.1 exibe alíquota ICMS efetiva vs. IBS+CBS projetada por NCM com flag IS
  3. Módulo 2.3 exibe tabela UF + mapa coroplético colorido por volume de impacto
  4. Módulo 2.4 segmenta B2B/B2C em três vias (b2b_credit/b2b_nocredit/b2c) com nota sobre notas históricas sem `ind_final`

**Plans:** 2/2 plans complete

Plans:

- [x] 09-01-PLAN.md — Backend: reforma_modulo2.go (4 handlers JSON + 2 CSV: cfopAnalysis, ncmAnalysis, ufDestino, b2bB2c) + 6 rotas em main.go (wave 1)
- [x] 09-02-PLAN.md — Frontend: 4 páginas (Reforma22/21/23/24) + ativação de 4 tabs em navigation.ts + 4 rotas em App.tsx (wave 2, depende de 09-01)

### Phase 10: ICMS Fronteira — ST por NCM no Bloco C

**Goal**: Classificar como ICMS-ST, pelo NCM, as NFs presentes em XML mas ausentes do SPED (Bloco C) quando o NCM tem regra de ST cadastrada e a empresa tem o segmento da regra para a UF destino — independentemente do CFOP do fornecedor. Resolve o caso do fornecedor sem protocolo CONFAZ, que emite com CFOP de venda normal (6101/6102) mas cuja mercadoria é de ST.
**Depends on**: Phase 8 (cadastro de regras por UF + segmentos)
**Requirements**: FRST-01, FRST-02, FRST-03
**Success Criteria** (what must be TRUE):

  1. NF CFOP 6101/6102 (entrada 2101/2102/2152) com NCM de ST + segmento da empresa na UF → classificada como ST (não Antecipação)
  2. Demonstrativo ST por Item (Bloco C) inclui essas NFs, com MVA/base/alíquota por item
  3. Aba Antecipação e aba ST classificam a mesma NF de forma consistente (não some de ambas)
  4. Tela de reclassificação manual removida — classificação por NCM é regra automática do sistema
  5. Badge "NCM→ST" indica visualmente as NFs reclassificadas pelo NCM

**Plans:** 1/1 plan complete (retroativo)

Plans:

- [x] 10-01-PLAN.md — Classificação NCM-first nas 3 views do Bloco C (nao-sped, reconciliação, ST por item) + remoção da tela de reclassificação manual (retroativo)

### Phase 11: Motor de Execução do Pacote Fiscal (Backend)

**Goal**: Dado um item de `nfe_saidas_itens`, o sistema resolve seu grupo fiscal no Oracle (prod/PRODB), executa `PKG_FISCAL_FCTAX.calcula_imposto_produto` com bind seguro e persiste os ~88 campos de saída em `fiscal_execution_items`, em lote, com isolamento de erro e limites de concorrência/timeout — sem nenhuma tela ainda, apenas a fundação de dados que a Phase 12 vai exibir.
**Depends on**: Nothing dentro deste milestone (reaproveita `nfe_saidas`/`nfe_saidas_itens` já existentes e a conexão ERP_BRIDGE Oracle já em produção)
**Requirements**: TPF-01, TPF-02, TPF-03, TPF-04, TPF-05
**Success Criteria** (what must be TRUE):

  1. Para um item de `nfe_saidas_itens`, o sistema resolve o grupo fiscal correspondente via Oracle (`prod`/`PRODB`), ou retorna erro explícito quando o mapeamento (ex.: filial fora de Recife/PE) não existe
  2. Quando confirmado necessário, despesas/desconto por item ficam disponíveis em `nfe_saidas_itens` como input do pacote fiscal
  3. O pacote `PKG_FISCAL_FCTAX.calcula_imposto_produto` é executado via bloco PL/SQL estático com bind seguro (`sql.Named`/`go_ora.Out`, nunca concatenação de string) e os ~88 campos de saída são persistidos em `fiscal_execution_items` com status `ok`/erro por item
  4. Em um lote com N itens, um item que falha (grupo fiscal ausente, timeout, erro Oracle) não impede o processamento dos demais itens do lote
  5. A execução em lote respeita um limite de concorrência e um timeout por item configuráveis, evitando saturar a conexão Oracle

**Plans**: 6 plans (4 waves)

Plans:
**Wave 1**

- [x] 11-01-PLAN.md — Driver go-ora + conexão Oracle síncrona (openFiscalOracleConn) + rota admin de smoke test de alcançabilidade (checkpoints: legitimidade go-ora + reachability)
- [x] 11-02-PLAN.md — TPF-02: v_desc/v_outro por item (migration 146 nas duas tabelas de itens + struct prod/insertNFeItens)
- [x] 11-03-PLAN.md — TPF-01 lookup de grupo fiscal (fiscal_group_lookup.go) + TPF-04 tabela fiscal_execution_items (migration 147, schema híbrido + IBS/CBS)

**Wave 2** *(bloqueada até go-ora instalado)*

- [x] 11-04-PLAN.md — TPF-03: services/oracle_fiscal.go (bloco PL/SQL estático via reflection, 23 IN/~88 OUT, bind seguro)

**Wave 3** *(bloqueada até Waves 1-2)*

- [x] 11-05-PLAN.md — TPF-05: endpoint de lote /api/fiscal/execute (fan-out sem cap 5, timeout 15s/item, isolamento por item, upsert) + guard tests

**Wave 4** *(bloqueada até Wave 3)*

- [x] 11-06-PLAN.md — Checkpoint end-to-end: execução real de lote contra Oracle validando as pegadinhas do go-ora com dados reais

### Phase 12: Tela Comparação Fiscal + Navegação

**Goal**: Um usuário admin acessa a tela "Teste Pacote Fiscal" e vê, item a item, o valor esperado (`nfe_saidas_itens`, vindo do XML) contra o valor calculado pelo pacote fiscal (`fiscal_execution_items`), com divergências de ICMS/ICMS-ST/PIS/COFINS/IBS/CBS destacadas visualmente, podendo filtrar só os itens divergentes e ver um resumo agregado — usuários sem role admin não veem essa opção de navegação.
**Depends on**: Phase 11 (precisa de `fiscal_execution_items` populada para exibir algo)
**Requirements**: TPF-06, TPF-07, TPF-08
**Success Criteria** (what must be TRUE):

  1. Usuário com role admin vê o item de navegação "Teste Pacote Fiscal"; usuário sem role admin não vê essa opção (gate `adminOnly: true`)
  2. A tela "Comparação Fiscal" lista itens executados mostrando lado a lado o valor esperado (XML/`nfe_saidas_itens`) e o calculado (`fiscal_execution_items`) para ICMS, ICMS-ST, PIS, COFINS, IBS e CBS
  3. Itens com divergência entre esperado e calculado são destacados visualmente (cor/badge) por imposto
  4. Usuário pode ativar um filtro "só divergentes" que oculta itens sem nenhuma divergência
  5. A tela exibe um resumo agregado (contagem e/ou percentual de itens divergentes) por tipo de imposto

**Plans**: 3 plans (3 waves)

Plans:
**Wave 1**

- [x] 12-01-PLAN.md — Backend: fiscal_comparacao.go (search + comparison-read LEFT JOIN, 4º estado not_executed, soma IBS) + fiscal_comparacao_csv.go + 3 rotas admin-gated em main.go (TPF-06/07)

**Wave 2** *(bloqueada até Wave 1)*

- [x] 12-02-PLAN.md — Frontend: NfeSearchCombobox.tsx (busca server-side debounced) + ComparacaoFiscal.tsx (busca→executar→recarrega, 6 impostos, 4 estados de badge, filtro só divergentes, resumo agregado, export Excel/CSV) (TPF-06/07)

**Wave 3** *(bloqueada até Wave 2)*

- [x] 12-03-PLAN.md — Navegação: navigation.ts + AppSidebar.tsx + App.tsx (item adminOnly 'Teste Pacote Fiscal') + checkpoint de verificação end-to-end (TPF-08)

**UI hint**: yes

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9 → 10 → 11 → 12

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Estabilização Crítica (Reset + Cache) | 3/3 | Complete | 2026-05-08 |
| 2. Upload de XMLs (Drag-and-Drop) | 4/4 | Complete | 2026-05-16 |
| 3. Estabilização Adicional | 4/4 | Complete | 2026-05-16 |
| 4. Conciliação Bridge vs XML | 2/2 | Complete | 2026-05-16 |
| 5. Observabilidade e Alertas | 2/2 | Complete | 2026-05-17 |
| 6. Infraestrutura Reforma Tributária | 4/4 | Verified | 2026-05-22 |
| 7. Módulos 1.x — Exposição Tributária Direta | 2/2 | Verified | 2026-05-23 |
| 8. Cadastro de Empresas + Ambiente Adm por UF | 3/3 | Verified | 2026-05-23 |
| 9. Módulos 2.x — Analytics Dimensional | 2/2 | Verified | 2026-05-23 |
| 10. ICMS Fronteira — ST por NCM no Bloco C | 1/1 | Verified (UAT 5/5) | 2026-06-28 |
| 11. Motor de Execução do Pacote Fiscal (Backend) | 6/6 | Complete    | 2026-07-03 |
| 12. Tela Comparação Fiscal + Navegação | 3/3 | Complete   | 2026-07-03 |

> **Milestone v5.00 fechado em 2026-05-29.** Fases 6–9 marcadas como *Verified* por
> fechamento administrativo (sem UAT formal), a pedido do usuário — o trabalho ativo
> migrou para o módulo ICMS Fronteira, rastreado fora da estrutura de fases GSD.

---
*Roadmap created: 2026-05-08*
*Phase 2 planned: 2026-05-16*
*Phase 3 planned: 2026-05-16*
*Phase 4 planned: 2026-05-16*
*Phase 5 planned: 2026-05-16*
*Phases 6–8 planned: 2026-05-22 (milestone v5.00)*
*Phase 6 plans finalized: 2026-05-22 (4 plans, 3 waves)*
*Phases 11–12 planned: 2026-07-03 (milestone v6.00 — Módulo Teste Pacote Fiscal)*
