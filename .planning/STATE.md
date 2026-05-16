---
gsd_state_version: 1.0
milestone: v4.00
milestone_name: milestone
status: ready_to_plan
last_updated: "2026-05-16T23:48:49.042Z"
progress:
  total_phases: 5
  completed_phases: 4
  total_plans: 15
  completed_plans: 13
  percent: 87
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
- **Active phase:** Phase 04 — Conciliação Bridge vs XML — COMPLETA (Plans 01+02)
- **Completed phases:** 4
- **Last session:** 2026-05-16T22:00:00.000Z

## Current Phase

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

## Configuration

- **Mode:** YOLO (autonomous)
- **Granularity:** Coarse (5 phases, 1-3 plans each)
- **Parallelization:** Sequential
- **Models:** Balanced (Sonnet)
- **Workflow agents:** Research + Plan Check + Verifier (all enabled)
- **Auto-advance:** Enabled
- **Commit docs:** Yes (planning tracked in git)

## Next Action

Phase 05 PLANEJADA (2026-05-16) — 2 planos prontos para execução.
- Plan 01 (Wave 1): Infraestrutura Prometheus/Grafana + instrumentação Go + instrumentação bridge.py
- Plan 02 (Wave 2): Regras de alerta + Alertmanager SMTP + runbooks

Executar: `/gsd-execute-phase 5`

---
*Last updated: 2026-05-08*
