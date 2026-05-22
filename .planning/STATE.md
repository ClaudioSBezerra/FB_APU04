---
gsd_state_version: 1.0
milestone: v5.00
milestone_name: Análise da Reforma Tributária
status: executing
last_updated: "2026-05-22T21:41:06.329Z"
last_activity: 2026-05-22 -- Phase 06 planning complete
progress:
  total_phases: 8
  completed_phases: 0
  total_plans: 4
  completed_plans: 0
  percent: 0
---

# State: FB_APU04

## Project Reference

See: `.planning/PROJECT.md` (updated 2026-05-22)

**Core value:** Escrituração fiscal completa e auditável — todos os valores tributários (PIS, COFINS, IPI, ICMS) corretos por nota, com rastreabilidade até o documento original (XML ou ERP), pronta para fiscalização da Receita Federal.

**Current focus:** Milestone v5.00 — Análise da Reforma Tributária (definindo requisitos)

## Current Position

Phase: Phase 6 — Infraestrutura Reforma Tributária (not started)
Plan: —
Status: Ready to execute
Last activity: 2026-05-22 -- Phase 06 planning complete

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
*Last updated: 2026-05-22 — Milestone v5.00 roadmap approved*
