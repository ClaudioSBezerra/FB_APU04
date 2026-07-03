---
phase: 11-motor-de-execu-o-do-pacote-fiscal-backend
plan: 04
subsystem: api
tags: [go-ora, oracle, plsql, reflection, bind-variables]

# Dependency graph
requires:
  - phase: 11-01
    provides: "go-ora v2.9.0 vendored driver + openFiscalOracleConn (proven Oracle reachability)"
provides:
  - "services.FiscalInput / services.FiscalResult — 23 IN / 88 OUT Go structs mapping the PKG_FISCAL_FCTAX.calcula_imposto_produto contract"
  - "services.BuildCalculaImpostoBlock() — 100% static PL/SQL anonymous block text, generated only from fixed metadata tables"
  - "services.CallFiscalPackage(ctx, oracleDB, in) (*FiscalResult, error) — bind-safe executor, exported for Plan 11-05's batch handler"
affects: [11-05, 11-06]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Reflection-over-fixed-metadata-table PL/SQL block generation (fiscalInParams/fiscalOutFields) — the core anti-injection control for this integration; zero string concatenation of input values into SQL text"
    - "go_ora.Out{Dest,Size} for string OUT binds vs plain sql.Out for numeric OUT binds"

key-files:
  created:
    - backend/services/oracle_fiscal.go
  modified: []

key-decisions:
  - "CallFiscalPackage returns (*FiscalResult, error) per plan's explicit acceptance criteria, diverging from the FB_TESTESFC original's value-return (FiscalResult, error) — pointer signature was required for Plan 11-05's handler to consume via services.CallFiscalPackage"
  - "Split the single-file port into two atomic commits matching the plan's two tasks (contracts/metadata tables, then builder/bind/call) rather than one combined commit, since Task 1 has its own independent build-passing verification gate"

patterns-established:
  - "backend/services/ is now a populated package for non-HTTP Oracle integration code (previously only ai.go/crypto.go/email.go/rfb.go/text_to_sql.go); oracle_fiscal.go follows the same package services convention"

requirements-completed: [TPF-03]

# Metrics
duration: 20min
completed: 2026-07-03
---

# Phase 11 Plan 04: Motor de Execução do Pacote Fiscal — Caller Oracle PL/SQL Summary

**`backend/services/oracle_fiscal.go` portado verbatim do FB_TESTESFC: 23 parâmetros IN / 88 campos OUT do `PKG_FISCAL_FCTAX.calcula_imposto_produto` mapeados via duas tabelas de metadados fixas, bloco PL/SQL anônimo gerado 100% por reflection sobre essas tabelas (zero concatenação de input), e `CallFiscalPackage` exportada com bind seguro (`sql.Named` + `go_ora.Out{Size:4000}`).**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-07-03T16:35:00Z
- **Completed:** 2026-07-03T16:55:00Z
- **Tasks:** 2
- **Files modified:** 1 (created)

## Accomplishments
- Ported `FiscalInput` (23 fields, `FornecedorSimplesNacional` correctly without the `p` prefix) and `FiscalResult` (88 fields, all five `IdRegraCalculo*` typed `string` per Pitfall 2) from FB_TESTESFC's validated `oracle_fiscal.go`, read directly off disk (file still present at `/home/claudiobezerra/projetos/FB_TESTESFC/backend/services/oracle_fiscal.go`)
- Ported `fiscalInParams` (23 entries) and `fiscalOutFields` (88 entries) metadata tables — verified counts by direct grep (`23`/`88`) before committing
- Ported `BuildCalculaImpostoBlock`, `buildBindArgs`, and `CallFiscalPackage` — confirmed by direct inspection that the only two `fmt.Fprintf` calls in the block builder consume exclusively `p.OracleParam`/`f.GoField`/`f.OracleField` from the fixed metadata tables, never a `FiscalInput` value
- Adapted `CallFiscalPackage`'s return type to `(*FiscalResult, error)` (plan's explicit contract) instead of the FB_TESTESFC original's `(FiscalResult, error)`, so Plan 11-05's batch handler can consume it as `services.CallFiscalPackage`
- `go build ./...` and `go vet ./services/` pass after each task

## Task Commits

Each task was committed atomically:

1. **Task 1: Structs de contrato + tabelas de metadados** - `699081d` (feat)
2. **Task 2: Builder do bloco PL/SQL estático + CallFiscalPackage (bind seguro)** - `061173e` (feat)

**Plan metadata:** (this commit) `docs(11-04): complete plan`

## Files Created/Modified
- `backend/services/oracle_fiscal.go` - `FiscalInput`/`FiscalResult` contracts, `fiscalInParams`/`fiscalOutFields` metadata tables, `BuildCalculaImpostoBlock`, `buildBindArgs`, `CallFiscalPackage` (387 lines)

## Decisions Made
- **`CallFiscalPackage` returns a pointer** (`*FiscalResult, error`), not a value, diverging from the FB_TESTESFC source — required by this plan's frontmatter/acceptance criteria so the Plan 11-05 batch handler gets a stable, allocatable result per goroutine.
- **Two commits instead of one** — Task 1 (structs/tables) and Task 2 (builder/bind/call) each have independent `go build`/`grep` verification gates in the plan, so each was committed as soon as its own verification passed, keeping the atomic-commit-per-task discipline even though both tasks touch the same file.
- **`time` import added in Task 1's commit** (not present in the plan's task breakdown as a separate concern) — `FiscalInput.PDataReferenciaFiscal *time.Time` requires it for the package to build in isolation per Task 1's own `go build ./services/` verification step; this is a direct consequence of porting the struct verbatim, not a deviation from intent.

## Deviations from Plan

None - plan executed exactly as written. The `CallFiscalPackage` pointer-return adaptation and the `time` import were both already specified/implied by the plan's own acceptance criteria and struct definitions, not unplanned discoveries.

## Issues Encountered
None. FB_TESTESFC's `oracle_fiscal.go` was still present on disk at research/execution time, so the full original source (structs, metadata tables, builder, bind logic) was read directly rather than reconstructed from `11-RESEARCH.md`/`.continue-here.md` summaries — this eliminated any risk of transcription drift in the 88-field OUT table.

## User Setup Required
None - no external service configuration required. This plan only adds Go source; no new environment variables, no new routes, no new database schema.

## Next Phase Readiness
- `services.CallFiscalPackage(ctx, oracleDB, in)` is ready for Plan 11-05's batch execution handler to call per-item, inside the semaphore-bounded goroutine fan-out (TPF-05), using the `*sql.DB` opened by `openFiscalOracleConn` (Plan 11-01).
- `services.FiscalInput` fields still need to be populated by Plan 11-05 from `nfe_saidas`/`nfe_saidas_itens` (via `resolveCodEmpresa`/`lookupGrupoFiscal` from Plan 11-03) — this plan only builds the caller, not the caller's caller.
- No blockers. The single most important invariant (zero concatenation of input values into the PL/SQL block text) was verified explicitly: `BuildCalculaImpostoBlock`'s two `fmt.Fprintf` calls read only from `fiscalInParams`/`fiscalOutFields` (fixed package-level tables), and all 23 IN values / 88 OUT destinations travel exclusively through `sql.Named`/`go_ora.Out` in `buildBindArgs`, which takes the actual `FiscalInput` value.

---
*Phase: 11-motor-de-execu-o-do-pacote-fiscal-backend*
*Completed: 2026-07-03*

## Self-Check: PASSED

- FOUND: `backend/services/oracle_fiscal.go`
- FOUND: `.planning/phases/11-motor-de-execu-o-do-pacote-fiscal-backend/11-04-SUMMARY.md`
- FOUND: commit `699081d` in git log
- FOUND: commit `061173e` in git log
- `cd backend && go build ./...` exits 0
