---
gsd_state_version: 1.0
milestone: v4.00
milestone_name: milestone
status: ready_to_plan
last_updated: "2026-05-16T15:24:04.369Z"
progress:
  total_phases: 5
  completed_phases: 1
  total_plans: 7
  completed_plans: 5
  percent: 71
---

# State: FB_APU04

## Project Reference

See: `.planning/PROJECT.md` (updated 2026-05-08)

**Core value:** Escrituração fiscal completa e auditável — todos os valores tributários (PIS, COFINS, IPI, ICMS) corretos por nota, com rastreabilidade até o documento original (XML ou ERP), pronta para fiscalização da Receita Federal.

**Current focus:** Phase 02 — upload-de-xmls-drag-and-drop

## Status

- **Initialized:** 2026-05-08
- **Codebase mapped:** 2026-05-08 (7 documents, 1920 lines in `.planning/codebase/`)
- **Roadmap:** 5 phases (Coarse granularity)
- **Active phase:** Phase 02 — Upload de XMLs Drag-and-Drop (Plan 2 de 4 concluído)
- **Completed phases:** 1
- **Last session:** 2026-05-16T16:00:00.000Z

## Current Phase

**Phase 02 — Upload de XMLs (Drag-and-Drop)**

- Goal: Permitir upload manual de XMLs (NF-e, CT-e) como complemento ao ERP Bridge
- Requirements: XML-01 a XML-08
- Status: IN PROGRESS — Plans 01+02 concluídos — Schema (074-078) + Handlers Go (XMLUploadHandler, XMLPainelHandler, StartXMLWorker)

## Decisions Made

- **Token de confirmação estático `DELETE-FB_APU04`:** simplicidade auditável; defesa real é combinação de 5 gates independentes (aceite T-01-02 do threat model)
- **Backup falha recusa truncar sem exceção:** fail-safe sobre disponibilidade — integridade dos dados prioritária
- **Gate DB allowlist reseta rate limiter quando disparado:** guard estrutural não penaliza o usuário
- **Vitest 1.6.x (não 4.x):** compatibilidade com Node 18.19 — 4.x requer styleText de node:util indisponível no Node 18
- **Modal AlertDialog sem AlertDialogTrigger:** abertura programática via setState do componente pai (ImportarEFD.tsx)
- **Views XML usam v_icms (não v_icms_dest/v_icms_remet):** campos inexistentes confirmados via schema migration 059 — v_icms é o campo correto do ICMSTot
- **ProcessXMLBatch exportado de handlers:** uso pelo worker sem duplicação de lógica de parse
- **NamedXML exportado como tipo público:** worker/xml_worker.go importa handlers para reutilizar o tipo sem duplicação

## Recent History

- 2026-05-07: Incidente — APU04 apontava para banco do APU02 via `APU02_DB_HOST`. Reset apagou 4 meses de produção do APU02. Toda infraestrutura separada nos commits `90d1b93`, `947de42`, `14b455b`. ResetDatabase ainda sem proteção.
- 2026-05-08: Inicialização do GSD. Codebase mapeado, PROJECT/REQUIREMENTS/ROADMAP criados.
- 2026-05-08: Phase 01 Plan 01 executado — ResetDatabaseHandler reescrito com 5 gates (STAB-01 a STAB-05), migration 073, audit log admin_destructive_actions, volume api_backups, ALLOWED_DESTRUCTIVE_DBS.
- 2026-05-08: Phase 01 Plan 02 executado — ResetDatabaseDialog criado com TDD RED/GREEN, integrado ao ImportarEFD.tsx. Infraestrutura vitest instalada. Task 3 (checkpoint:human-verify) aprovado em modo YOLO. Phase 01 COMPLETA.
- 2026-05-16: Phase 02 Plan 01 executado — 5 migrations criadas (074-078): coluna source em 3 tabelas, tabelas nfe_*_itens, xml_upload_batches, regime_tributario, 4 views vw_xml_*.
- 2026-05-16: Phase 02 Plan 02 executado — handlers Go: XMLUploadHandler (.xml/.zip, 100MB/5000 XMLs, async >50), XMLPainelHandler (3 views), StartXMLWorker (pool 3 goroutines), lógica XML>Oracle source.

## Configuration

- **Mode:** YOLO (autonomous)
- **Granularity:** Coarse (5 phases, 1-3 plans each)
- **Parallelization:** Sequential
- **Models:** Balanced (Sonnet)
- **Workflow agents:** Research + Plan Check + Verifier (all enabled)
- **Auto-advance:** Enabled
- **Commit docs:** Yes (planning tracked in git)

## Next Action

Phase 02 Plan 02 COMPLETO. Handlers Go implementados. Próximo: Phase 02 Plan 03 — frontend React para upload drag-and-drop e painel de XMLs importados.

---
*Last updated: 2026-05-08*
