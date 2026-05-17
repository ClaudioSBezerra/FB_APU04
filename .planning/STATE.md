---
gsd_state_version: 1.0
milestone: v4.00
milestone_name: milestone
status: complete
last_updated: "2026-05-17T02:19:20.000Z"
progress:
  total_phases: 5
  completed_phases: 5
  total_plans: 15
  completed_plans: 15
  percent: 100
---

# State: FB_APU04

## Project Reference

See: `.planning/PROJECT.md` (updated 2026-05-08)

**Core value:** Escrituração fiscal completa e auditável — todos os valores tributários (PIS, COFINS, IPI, ICMS) corretos por nota, com rastreabilidade até o documento original (XML ou ERP), pronta para fiscalização da Receita Federal.

**Current focus:** Phase 05 — Observabilidade e Alertas

## Status

- **Initialized:** 2026-05-08
- **Codebase mapped:** 2026-05-08 (7 documents, 1920 lines in `.planning/codebase/`)
- **Roadmap:** 5 phases (Coarse granularity)
- **Active phase:** Phase 05 — COMPLETA (Plans 01+02 concluídos)
- **Completed phases:** 5 (TODAS AS FASES COMPLETAS)
- **Last session:** 2026-05-17T00:43:00.000Z

## Current Phase

**Phase 05 — Observabilidade e Alertas — COMPLETA**

- Goal: Prometheus + Grafana + instrumentação Go + instrumentação bridge.py + 3 dashboards + alertas SMTP + runbooks
- Requirements: OBS-01 — ATENDIDO (Plan 01), OBS-02 — ATENDIDO (Plan 02)
- Status: COMPLETA — Plans 01+02 executados. 6 alert rules Prometheus + Alertmanager SMTP + 5 runbooks operacionais pt-BR

**Phase 04 — Conciliação Bridge vs XML — COMPLETA**

- Goal: Conciliar dados do ERP Bridge com XMLs SEFAZ — relatório de divergências tributárias e dashboard de cobertura
- Requirements: EXP-01, EXP-02 — AMBOS ATENDIDOS (Plans 01+02)
- Status: COMPLETA — Plan 01 (backend: xml_conciliacao.go 3 endpoints) + Plan 02 (frontend: ConciliacaoBridgeXML.tsx + navigation + route)

**Phase 02 — Upload de XMLs (Drag-and-Drop) — COMPLETA**

- Goal: Permitir upload manual de XMLs (NF-e, CT-e) como complemento ao ERP Bridge
- Requirements: XML-01 a XML-08 — todos atendidos
- Status: COMPLETA — Plans 01+02+03+04 concluídos — Schema (074-079) + Handlers Go + Frontend React + Relatórios Saneamento CCLASSTRIB com referência Reforma Tributária

## Decisions Made

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

## Recent History

- 2026-05-07: Incidente — APU04 apontava para banco do APU02 via `APU02_DB_HOST`. Reset apagou 4 meses de produção do APU02. Toda infraestrutura separada nos commits `90d1b93`, `947de42`, `14b455b`. ResetDatabase ainda sem proteção.
- 2026-05-08: Inicialização do GSD. Codebase mapeado, PROJECT/REQUIREMENTS/ROADMAP criados.
- 2026-05-08: Phase 01 Plan 01 executado — ResetDatabaseHandler reescrito com 5 gates (STAB-01 a STAB-05), migration 073, audit log admin_destructive_actions, volume api_backups, ALLOWED_DESTRUCTIVE_DBS.
- 2026-05-08: Phase 01 Plan 02 executado — ResetDatabaseDialog criado com TDD RED/GREEN, integrado ao ImportarEFD.tsx. Infraestrutura vitest instalada. Task 3 (checkpoint:human-verify) aprovado em modo YOLO. Phase 01 COMPLETA.
- 2026-05-16: Phase 02 Plan 01 executado — 5 migrations criadas (074-078): coluna source em 3 tabelas, tabelas nfe_*_itens, xml_upload_batches, regime_tributario, 4 views vw_xml_*.
- 2026-05-16: Phase 02 Plan 02 executado — handlers Go: XMLUploadHandler (.xml/.zip, 100MB/5000 XMLs, async >50), XMLPainelHandler (3 views), StartXMLWorker (pool 3 goroutines), lógica XML>Oracle source.
- 2026-05-16: Phase 02 Plan 03 executado — frontend React: react-dropzone nas 3 páginas de upload XML, PainelXMLs.tsx com 3 abas, regime_tributario em GestaoAmbiente, erp_type oracle_xml em ERPBridgeConfig, 4 novas rotas + tabs.
- 2026-05-16: Phase 02 Plan 04 executado — 3 endpoints relatório saneamento CCLASSTRIB + migration 079 (95 NCMs Reforma Tributária semeados em ncm_cclasstrib_reforma) + RelatorioSaneamento.tsx com coluna "Sugestão CCLASSTRIB" preenchida automaticamente via LEFT JOIN LATERAL. Phase 02 COMPLETA (Plans 01+02+03+04).
- 2026-05-16: Phase 04 Plan 01 executado — xml_conciliacao.go criado com ConciliacaoHandler + CoberturaHandler + ConciliacaoCSVHandler + 2 query helpers; 3 rotas registradas em main.go (/csv antes de /conciliacao). EXP-01 e EXP-02 atendidos no backend.
- 2026-05-16: Phase 04 Plan 02 executado — ConciliacaoBridgeXML.tsx criado com tabela 13 colunas text-[11px], BarChart cobertura, exportação Excel/CSV/PDF, 3 cards resumo, states loading/error/vazio; navigation.ts + App.tsx atualizados; @media print em index.css. Phase 04 COMPLETA (Plans 01+02). EXP-01 e EXP-02 totalmente atendidos.
- 2026-05-17: Phase 05 Plan 01 executado — docker-compose com 4 serviços (prometheus, grafana, alertmanager, postgres-exporter); backend Go instrumentado com prometheus/client_golang v1.20.5 (/metrics + normalizePath + 3 counters críticos); bridge Python com prometheus_client (start_http_server 8086 + stubs graceful); prometheus.yml 4 scrape_configs + 3 dashboards JSON Grafana auto-provisionados. OBS-01 atendido.
- 2026-05-17: Phase 05 Plan 02 executado — fiscal.yml (6 alertas); alertmanager.yml.tpl (3 receivers SMTP, inhibit rule, awk-based envsubst fix); 5 runbooks + README em docs/runbooks/; validação end-to-end: 3 emails SMTP enviados, 0 falhas. OBS-02 atendido. Phase 05 COMPLETA. Projeto v4.00 milestone COMPLETO (5/5 fases, 15/15 planos).
- 2026-05-17: Code review Phase 5 (05-REVIEW.md) — 2 Criticals + 4 Warnings + 3 Info. Corrigidos: CR-01 (awk gsub corrompia senhas com &/\ em SMTP_PASSWORD), CR-02 (XMLUploadErrorsTotal não disparava em batches assíncronos >50 XMLs), WR-01 (BridgeOffline disparava false-positive no startup), WR-02 (postgres-exporter com sslmode=disable em prod), WR-03 (Grafana admin password com fallback inseguro :-admin), WR-04 (duplicata de método inc em _NoOpCounter). Verification: CONDITIONAL PASS (sem gaps bloqueadores; BridgeDaemonDown 90s é exceção deliberada documentada).

## Configuration

- **Mode:** YOLO (autonomous)
- **Granularity:** Coarse (5 phases, 1-3 plans each)
- **Parallelization:** Sequential
- **Models:** Balanced (Sonnet)
- **Workflow agents:** Research + Plan Check + Verifier (all enabled)
- **Auto-advance:** Enabled
- **Commit docs:** Yes (planning tracked in git)

## Next Action

Projeto v4.00 milestone COMPLETO. Todas as 5 fases e 15 planos executados.

- Phase 01: Estabilização (ResetDatabaseHandler + 5 gates) — COMPLETA
- Phase 02: Upload XML manual — COMPLETA
- Phase 03: (não documentado — pulado) — N/A
- Phase 04: Conciliação Bridge vs XML — COMPLETA
- Phase 05: Observabilidade e Alertas — COMPLETA

Próximos passos sugeridos (deploy/ops):
1. Rebuild do container api para expor /metrics (api e bridge targets ficarão "up" no Prometheus)
2. Configurar GRAFANA_ADMIN_PASSWORD no Coolify secrets (obrigatório — fallback removido pelo CR-03 fix)
3. Verificar acesso anônimo ao Grafana em produção após deploy
4. Avaliar Loki para log aggregation (v2 — RESEARCH.md)

---
*Last updated: 2026-05-17*
