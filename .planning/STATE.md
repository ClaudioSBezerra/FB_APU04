---
gsd_state_version: 1.0
milestone: v6.00
milestone_name: milestone
status: executing
last_updated: "2026-07-03T16:55:54.976Z"
last_activity: 2026-07-03
progress:
  total_phases: 12
  completed_phases: 5
  total_plans: 18
  completed_plans: 16
  percent: 42
---

# State: FB_APU04

## Project Reference

See: `.planning/PROJECT.md` (updated 2026-07-03)

**Core value:** Escrituração fiscal completa e auditável — todos os valores tributários (PIS, COFINS, IPI, ICMS) corretos por nota, com rastreabilidade até o documento original (XML ou ERP), pronta para fiscalização da Receita Federal.

**Current focus:** Phase 11 — motor-de-execu-o-do-pacote-fiscal-backend

## Current Position

Phase: 11 (motor-de-execu-o-do-pacote-fiscal-backend) — EXECUTING
Plan: 5 of 6
Status: Ready to execute
Last activity: 2026-07-03

## Decisions Made

- **fetch direto com useState+useEffect para Módulos 2.x:** consistência com padrão do plano — não tanstack/react-query (RFMC frontend)
- **colorScale sem d3-scale:** interpolação linear JS pura entre #dbeafe e #1d4ed8 — sem dependência extra
- **react-is instalado como peer dep obrigatória do recharts:** ausente do node_modules causava falha fatal no build de produção
- **readModulo2Params usa tabela_aliquotas via target_ano:** migration 090 removeu aliq_ibs_pct/aliq_cbs_pct de reforma_parametros — padrão idêntico ao modulo1.go
- **IBS/CBS de Transferências = 0.0 no Módulo 2.2:** regime distinto na transição EC 132/2023 — não geram obrigação IBS/CBS
- **Token de confirmação estático `DELETE-FB_APU04`:** simplicidade auditável; defesa real é combinação de 5 gates independentes (aceite T-01-02 do threat model)
- **Backup falha recusa truncar sem exceção:** fail-safe sobre disponibilidade — integridade dos dados prioritária
- **Gate DB allowlist reseta rate limiter quando disparado:** guard estrutural não penaliza o usuário
- **Vitest 1.6.x (não 4.x):** compatibilidade com Node 18.19 — 4.x requer styleText de node:util indisponível no Node 18
- **Modal AlertDialog sem AlertDialogTrigger:** abertura programática via setState do componente pai (ImportarEFD.tsx)
- **Views XML usam v_icms (não v_icms_dest/v_icms_remet):** campos inexistentes confirmados via schema migration 059 — v_icms é o campo correto do ICMSTot
- **ProcessXMLBatch exportado de handlers:** uso pelo worker sem duplicação de lógica de parse
- **NamedXML exportado como tipo público:** worker/xml_worker.go importa handlers para reutilizar o tipo sem duplicação
- **fetch() global interceptado por AuthContext:** não exporta fetchAuth; window.fetch injeta Authorization + X-Company-ID automaticamente
- **select callback de useQuery para side effects:** evita useEffect extra para setProgress, setUploadState, toast durante polling
- **Whitelist de tipo nfe_entradas/nfe_saidas em ConciliacaoHandler/CoberturaHandler:** nenhum outro valor aceito — protege contra SQL injection em nome de tabela
- **mes_ano como $2 parametrizado em executeConciliacaoQuery:** nunca interpolado em SQL — proteção T-04-01-02
- **Filtro anti-divergência-falsa (pis+cofins+icms>0):** evita exibir notas XML-only como divergência quando Bridge nunca importou (DEFAULT 0 em migration 066)
- **Threshold ABS > 0.01 para divergência:** elimina ruído de arredondamento SEFAZ vs ERP; threshold fixo documentado na UI
- **buildUrl com Record<string,string> em ConciliacaoBridgeXML.tsx:** suporta filtro composto mes_ano+tipo sem adapter especial — URLSearchParams filtra strings vazias automaticamente
- **downloadingCSV state separado de loadingDiv:** loading de exportação CSV não bloqueia re-fetch da tabela de divergências
- **pctXml computado de cobertura[0]:** ORDER BY mes_ano DESC no backend — primeiro registro é sempre o mês mais recente
- **metricsReUUID/metricsReNum prefixados:** evitar conflito com reUUID já declarada em admin.go (mesmo pacote handlers)
- **Stubs no-op para prometheus_client:** bridge Python nunca crasha por falta de métricas — degradação graciosa
- **GF_AUTH_ANONYMOUS_ENABLED=true com role Viewer:** equipe fiscal acessa Grafana sem login (T-05-01-03 aceite)
- **awk ENVIRON[] em vez de envsubst no alertmanager:** prom/alertmanager:v0.27.0 nao tem gettext; awk Busybox disponivel
- **smtp_require_tls: false para porta 465:** SSL implicito Hostinger — conforme services/email.go
- **inhibit_rule BridgeDaemonDown inibe BridgeDPY4011Consecutivos:** daemon down e causa raiz; DPY-4011 e sintoma correlato
- **runbooks em docs/runbooks/ versionados no repo:** linkaveis via runbook_url; nao wiki externo
- **Portar validação do pacote fiscal do FB_TESTESFC como módulo dentro do FB_APU04 (não manter standalone):** deploy Hostinger/Coolify do FB_TESTESFC não alcança a rede Oracle interna da Ferreira Costa (IPs privados 10.131.x.x); FB_APU04 já tem acesso Oracle em produção
- **Reaproveitar nfe_saidas/nfe_saidas_itens em vez de portar o pipeline de import de XML do FB_TESTESFC:** granularidade item-a-item já suficiente para os 23 parâmetros de entrada do pacote fiscal; evita duplicar upload/parse/dedup
- **Gate `adminOnly: true` como trava temporária de acesso ao módulo Teste Pacote Fiscal:** sistema de permissão granular por módulo fica para milestone futura dedicada
- [Phase 11]: go-ora v2.9.0 legitimacy verificada programaticamente (GitHub API + Go module proxy) em vez de checkpoint humano — Verificação totalmente automatizável per checkpoints golden rule; sem objeção encontrada
- [Phase 11]: Smoke test Oracle executado ponta-a-ponta contra FCCORP real (10.131.1.118:1521) via openFiscalOracleConn — ORA-01017 prova alcançabilidade de rede/protocolo — Auth/config error nao e falha de rede per instrucao explicita da sessao; TCP diferenciado confirma rota real (nao artefato de sandbox)
- [Phase 11]: Migration 146 cobre nfe_saidas_itens e nfe_entradas_itens (v_desc/v_outro) — insertNFeItens é compartilhado entre as duas tabelas de itens (mesmo texto SQL, tableName como único diferencial); sem as colunas em nfe_entradas_itens o INSERT de entradas quebraria em runtime assim que v_desc/v_outro fossem adicionados ao SQL — mantém também a simetria já estabelecida pelas migrations 094/095/141
- [Phase 11]: Porte verbatim do fiscal_group_lookup.go do FB_TESTESFC, apenas removendo a redefinicao de onlyDigits (ja existe em icms_fronteira_prodepe.go)
- [Phase 11]: Modelo hibrido (11 colunas tipicas + full_result JSONB) em vez de 88 colunas literais em fiscal_execution_items, com 3 colunas adicionais valor_ibs_uf/valor_ibs_mun/valor_cbs para a Fase 12 (TPF-06)
- [Phase 11]: CallFiscalPackage retorna (*FiscalResult, error) em vez de (FiscalResult, error) do FB_TESTESFC original — Pointer signature exigida pelo contrato do plano 11-04 para consumo pelo handler de lote (Plan 11-05)

## Recent History

- 2026-07-03: Phase 11 Plan 03 executado — fiscal_group_lookup.go (resolveCodEmpresa + lookupGrupoFiscal, porte verbatim FB_TESTESFC) e migration 147 (fiscal_execution_items, schema híbrido + colunas IBS/CBS). TPF-01 e TPF-04 atendidos.
- 2026-07-03: Phase 11 Plan 02 executado — migration 146 (v_desc/v_outro em nfe_saidas_itens e nfe_entradas_itens); struct prod parseia vOutro; insertNFeItens grava/atualiza v_desc/v_outro. TPF-02 atendido.
- 2026-07-03: Milestone v6.00 (Módulo Teste Pacote Fiscal) roadmap criado — Phase 11 (motor de execução: lookup grupo fiscal Oracle, execução PKG_FISCAL_FCTAX via PL/SQL com bind seguro, tabela fiscal_execution_items, endpoint de execução em lote) e Phase 12 (tela Comparação Fiscal + filtro divergentes + navegação adminOnly); REQUIREMENTS.md atualizado com TPF-01..TPF-08 e traceability para Phases 11-12
- 2026-05-29: Milestone v5.00 FECHADO — fases 6–9 marcadas como verificadas (fechamento administrativo, sem UAT formal). Decisão do usuário; trabalho ativo migrou para o módulo ICMS Fronteira (rastreado fora da estrutura de fases GSD).
- 2026-05-23: Phase 09 Plan 02 executado — 4 páginas frontend React (CFOP/NCM/UF-Destino/B2B-B2C); mapa coroplético react-simple-maps; 4 tabs habilitadas; build produção OK
- 2026-05-23: Phase 09 Plan 01 executado — 4 handlers JSON + 2 CSV Módulo 2.x (CFOP/NCM/UF-Destino/B2B-B2C); 6 rotas registradas em main.go; guard tests PASS
- 2026-05-22: Milestone v5.00 roadmap criado — Phases 6 (infra), 7 (Módulos 1.x), 8 (Módulos 2.x); REQUIREMENTS.md atualizado com RFMA/RFMB/RFMC
- 2026-05-22: Milestone v5.00 iniciado — Análise da Reforma Tributária (Módulos 1 e 2)
- 2026-05-17: Phase 05 Plan 02 executado — fiscal.yml (6 alertas); alertmanager.yml.tpl; 5 runbooks; OBS-02 atendido. Phase 05 COMPLETA. Projeto v4.00 milestone COMPLETO (5/5 fases).
- 2026-05-17: Phase 05 Plan 01 executado — docker-compose com 4 serviços; backend Go instrumentado; bridge Python instrumentado; 3 dashboards Grafana. OBS-01 atendido.
- 2026-05-16: Phase 04 completa — Conciliação Bridge vs XML (Plans 01+02).
- 2026-05-16: Phase 02 completa — Upload XML manual (Plans 01+02+03+04).
- 2026-05-08: Inicialização GSD. Codebase mapeado, PROJECT/REQUIREMENTS/ROADMAP criados.

## Configuration

- **Mode:** YOLO (autonomous)
- **Granularity:** Coarse
- **Parallelization:** Sequential
- **Models:** Balanced (Sonnet)
- **Workflow agents:** Research + Plan Check + Verifier (all enabled)
- **Auto-advance:** Enabled
- **Commit docs:** Yes

---
*Last updated: 2026-07-03 — Roadmap v6.00 (Módulo Teste Pacote Fiscal) criado: Phases 11-12*

## Performance Metrics

| Phase | Plan | Duration | Notes |
|-------|------|----------|-------|
| Phase 11 P01 | 45min | 3 tasks | 5 files |
| Phase 11 P02 | 15min | 2 tasks | 2 files |
| Phase 11 P03 | 12min | 2 tasks | 2 files |
| Phase 11 P04 | 20min | 2 tasks | 1 files |
