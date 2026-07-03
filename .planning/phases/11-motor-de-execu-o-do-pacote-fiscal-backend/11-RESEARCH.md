# Phase 11: Motor de Execução do Pacote Fiscal (Backend) - Research

**Researched:** 2026-07-03
**Domain:** Go backend integration with Oracle PL/SQL package (fiscal tax engine) + batch execution with concurrency control, porting validated code from a sibling repo (FB_TESTESFC) into FB_APU04's existing conventions.
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** Reaproveitar `nfe_saidas`/`nfe_saidas_itens` já existentes como
  fonte dos parâmetros de entrada do pacote fiscal. **Não** portar o
  pipeline de import de XML do FB_TESTESFC — o XML já é importado hoje pelo
  fluxo normal do FB_APU04.
- **D-02 (confirmado por inspeção de código, 2026-07-03):** TPF-02 é
  necessário. `insertNFeItens` (`backend/handlers/nfe_saidas.go:373`) já
  parseia `VDesc` (`xml:"vDesc"`) na struct `det`, mas **não persiste**
  `v_desc` em `nfe_saidas_itens` nem inclui no INSERT. `VOutro` (despesas
  acessórias) **nem está na struct ainda** — precisa adicionar o parsing.
  Escopo confirmado: adicionar `v_desc`/`v_outro` na struct (onde faltar),
  na tabela `nfe_saidas_itens` (migration nova) e no INSERT de
  `insertNFeItens`.
- **D-03:** O backend Go abre sua **própria conexão go-ora direta e
  síncrona** ao Oracle `prod`/`PRODB`, lendo as credenciais **já
  armazenadas** (criptografadas) em `erp_bridge_config` por `company_id`.
  Não duplica cadastro de credencial — reaproveita o armazenamento
  existente. Diferença importante: hoje essas credenciais só são
  consumidas pelo bridge Python externo (assíncrono, roda fora do processo
  Go); a Fase 11 introduz o **primeiro caminho onde o próprio backend Go
  abre uma conexão Oracle síncrona em tempo de requisição** — não existe
  hoje, é capacidade nova.
- **D-04:** Seguir o padrão simples já usado em `nfe_saidas_itens`: schema
  `public`, FK para `nfe_saidas_itens(id)` com `ON DELETE CASCADE`,
  `UNIQUE` constraint em `nfe_item_id` para permitir
  `INSERT ... ON CONFLICT (nfe_item_id) DO UPDATE` (upsert por item, sem
  transação única pro lote inteiro — cada item é seu próprio insert/update,
  conforme TPF-05). Sem particionamento — volume esperado é baixo (uso
  administrativo de validação, não o fluxo de todas as notas).

### Claude's Discretion

- Nome exato dos campos/colunas da migration de `fiscal_execution_items`
  (os ~88 campos de saída — mapear 1:1 com `RDADOS_FISCAIS_PRODUTO`).
  **Research finding (see "Critical Finding" below): the validated
  FB_TESTESFC implementation did NOT create 88 individual columns — it
  used a hybrid model. This is the recommended approach; see Architecture
  Patterns.**
- Estrutura interna do pool de conexão Oracle dedicado (se compartilha pool
  entre companies ou abre por request) — otimizar depois se necessário,
  não é requisito bloqueante da Fase 11.

### Deferred Ideas (OUT OF SCOPE)

- Tela "Comparação Fiscal" (TPF-06/07/08) — Fase 12, depende desta fase
  estar executada primeiro.
- Sistema de permissão granular por módulo (substituindo o gate `adminOnly`
  binário) — milestone futura, fora desta fase e desta milestone inteira.
- Otimização de pool de conexão Oracle (compartilhado vs. por-request) —
  Claude's Discretion nesta fase, pode virar fase de otimização futura se
  necessário.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| TPF-01 | Lookup de grupo fiscal via Oracle (`prod`/`PRODB`) por item de `nfe_saidas_itens`, portado de `fiscal_group_lookup.go` | Full source of `fiscal_group_lookup.go` read directly from FB_TESTESFC (still on disk) — see Code Examples. `resolveCodEmpresa` + `lookupGrupoFiscal` ready to adapt verbatim; only import path/package name changes. |
| TPF-02 | Extensão de `nfe_saidas_itens`/`insertNFeItens` para persistir despesas/desconto por item | Confirmed via direct inspection of `nfe_saidas.go`: `v_desc` parsed but not persisted; `v_outro` not parsed at all. Exact struct/INSERT/migration diffs identified — see Architecture Patterns → Pattern 2. |
| TPF-03 | Serviço de execução do `PKG_FISCAL_FCTAX.calcula_imposto_produto` via bloco PL/SQL estático com bind seguro | Full source of `oracle_fiscal.go` read directly — reflection-based static block builder, 23 IN params, 88 OUT fields, both go-ora gotchas documented in comments in the source itself. Ready to port as a new `backend/services` package (FB_APU04 has no `services/` package yet — new addition). |
| TPF-04 | Nova tabela `fiscal_execution_items` com os ~88 campos de saída, incluindo status | Actual production migration (`008_fiscal_execution_items.sql`) read directly — hybrid schema (11 dedicated columns + `full_result JSONB`), not 88 individual columns. No naming collision in FB_APU04 (`fiscal_calculations` is a different, unrelated table from the ICMS Fronteira module). |
| TPF-05 | Endpoint de execução em lote com concorrência limitada, timeout por item e isolamento de erro | Full source of `fiscal_execution.go` read directly — semaphore `chan struct{}` cap 5, `sync.WaitGroup`, `defer recover()` per goroutine, `context.WithTimeout` 15s per item, per-item upsert. Confirmed this pattern does **not** exist anywhere else in FB_APU04 today (new pattern for this codebase). |
</phase_requirements>

## Summary

This phase is a **direct code port**, not new design. The discontinued sibling
project `FB_TESTESFC` (`/home/claudiobezerra/projetos/FB_TESTESFC`, still on
disk at research time — not yet deleted) contains a fully working, previously
validated-against-real-Oracle implementation of everything TPF-01 through
TPF-05 require. This research read the **actual source files** directly
(`backend/services/oracle_fiscal.go`, `backend/handlers/fiscal_group_lookup.go`,
`backend/handlers/fiscal_execution.go`, `backend/migrations/008_fiscal_execution_items.sql`,
`backend/handlers/fiscal_comparison.go`) rather than relying solely on the
summarized handoff in `.continue-here.md` — this surfaced one important
correction to the CONTEXT.md guidance (see Critical Finding below) and
confirmed every other claim in CONTEXT.md verbatim.

FB_APU04 already has 100% of the supporting infrastructure this phase needs:
an AES-256-GCM field-encryption helper (`EncryptField`/`DecryptField` in
`backend/handlers/crypto.go`) already used to store Oracle credentials in
`erp_bridge_config`, a `getDB()`/global-`*sql.DB` pattern in `main.go` that a
second Oracle pool can follow, a `withAuth(handler, role)` route-registration
convention for role-gating, and 145 sequential numbered migrations with an
established idempotent (`IF NOT EXISTS`) style. The **only genuinely new**
elements this phase introduces to the codebase are: (1) a `go-ora` Oracle
driver dependency (does not exist in `go.mod` today), (2) a synchronous
Oracle connection opened by the Go backend itself at request time (today
Oracle is only touched by the external Python bridge), and (3) a
goroutine-fan-out-with-semaphore concurrency pattern for batch processing
(no other handler in FB_APU04 uses this pattern — closest analogues are
single-goroutine background jobs in `xml_upload.go`/`worker.go`, not
per-item fan-out).

**Critical Finding — `fiscal_execution_items` schema:** CONTEXT.md's
"Claude's Discretion" section says to "map 1:1 with `RDADOS_FISCAIS_PRODUTO`"
implying ~88 individual columns. **The actual validated FB_TESTESFC migration
(`008_fiscal_execution_items.sql`) does not do this.** It uses a **hybrid
model**: 11 dedicated typed columns for the fields the comparison screen
needs fast/indexable access to (ICMS, ICMS-ST, PIS, COFINS, DIFAL, FCP), plus
one `full_result JSONB NOT NULL` column holding the complete ~88-field
struct for audit/detail-view purposes. This is the pattern that was actually
built, tested, and worked against real Oracle — it should be the one ported,
not a literal 88-column table. See "Architecture Patterns" and "Assumptions
Log" for the one adjustment this phase needs to make to that inherited
schema (IBS/CBS columns), since FB_APU04's TPF-06 (Phase 12) has slightly
broader comparison scope than FB_TESTESFC's original comparison screen did.

**Primary recommendation:** Port the three FB_TESTESFC Go files essentially
verbatim (package path and DB-pool wiring adapted to FB_APU04 conventions),
port the hybrid-model migration verbatim plus 2-3 additional typed columns
for IBS/CBS, add `v_desc`/`v_outro` to `nfe_saidas_itens` via a new
idempotent migration, and reuse `EncryptField`/`DecryptField` from
`crypto.go` exactly as `erp_bridge.go` already does — no new encryption
scheme needed.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Grupo fiscal lookup (Oracle `prod`/`PRODB`) | API / Backend | Database / Storage (Oracle, external) | Pure server-side Oracle query; no UI, no client involvement — TPF-01 |
| `nfe_saidas_itens` schema extension (v_desc/v_outro) | Database / Storage | API / Backend (parser update) | Schema + parser change only; XML parsing already happens server-side during upload — TPF-02 |
| PL/SQL package execution (`calcula_imposto_produto`) | API / Backend | Database / Storage (Oracle, external) | Static bind-safe PL/SQL block built and executed entirely server-side — TPF-03 |
| `fiscal_execution_items` persistence | Database / Storage | API / Backend (upsert logic) | New Postgres table, written exclusively by the backend batch job — TPF-04 |
| Batch execution endpoint (concurrency + isolation) | API / Backend | — | Single HTTP handler orchestrating goroutines; no browser/CDN involvement — TPF-05 |

No browser/client-tier or CDN/static-tier capabilities in this phase — it is
explicitly "sem nenhuma tela ainda" (backend-only foundation for Phase 12).

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/sijms/go-ora/v2` | v2.9.0 (latest, confirmed via Go module proxy 2026-07-03) [ASSUMED — package name originates from CONTEXT.md/prior validated use in FB_TESTESFC, not from this session's independent discovery; existence + version confirmed via authoritative Go proxy] | Pure-Go Oracle driver — no OCI/Instant Client install needed | Already used and validated against real Oracle (`FCCORP_BKP`) in FB_TESTESFC 2026-06-30→07-02. Listed by Oracle's own developer blog and the official go.dev Wiki "SQL Database Drivers" page as a legitimate pure-Go Oracle driver option [CITED: oracle.com/developer, go.dev/wiki/SQLDrivers]. ~911 GitHub stars, actively released (v2.9.0 released within the last month) [CITED: WebSearch, GitHub releases page]. |

**Installation:**
```bash
go get github.com/sijms/go-ora/v2@v2.9.0
```

**Version verification performed:**
```bash
go list -m -versions github.com/sijms/go-ora/v2
# → ... v2.8.24 v2.9.0   (queried live against proxy.golang.org, 2026-07-03)
```
No other Go Oracle driver (`godror`, `goracle`, etc.) is referenced anywhere
in the FB_APU04 codebase, docs, or comments — `go-ora` is the only candidate
and matches what was already validated in the sibling project. `godror`
(the CGO/OCI-based alternative) was **not** considered because it requires
installing Oracle Instant Client binaries on the deploy host (Coolify/Hostinger
container) — a materially heavier deployment change than a pure-Go driver,
and FB_TESTESFC's validated code already proves go-ora works end-to-end
against the exact same Oracle instance FB_APU04 needs to reach.

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `database/sql` (stdlib) | Go 1.24.1 (already the project's Go version) | `sql.Named`, `sql.Out` for IN params and non-string OUT params | Already the pattern used everywhere else in FB_APU04's Postgres access code |
| `reflect` (stdlib) | Go 1.24.1 | Drives the static PL/SQL block generation and bind-argument construction from the two metadata tables (`fiscalInParams`, `fiscalOutFields`) | Only inside the ported `oracle_fiscal.go` — not a new pattern to introduce elsewhere |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `go-ora` (pure Go) | `godror` (CGO + Oracle Instant Client) | Requires OS-level Oracle Client libraries in the deploy image; heavier ops burden; not what was validated in FB_TESTESFC. Rejected. |
| Hybrid schema (11 typed cols + JSONB) | 88 individual typed columns | Bigger migration, harder to maintain, and Phase 12's comparison screen only ever needs a handful of typed fields for fast diffing — the rest is audit/detail-only. The hybrid model is what was actually built and validated; recommended over the literal reading of CONTEXT.md's discretion note. |
| `go-ora`'s own connection pool per Oracle call | One dedicated `*sql.DB` Oracle pool held for the process lifetime, `SetMaxOpenConns(5)` | Opening a fresh `sql.Open` per batch run (as FB_TESTESFC did, scoped to the HTTP request) is simpler and avoids idle-Oracle-session concerns for a low-volume admin tool; left as Claude's Discretion per CONTEXT.md — no strong reason to deviate from FB_TESTESFC's per-request pattern for this phase. |

## Package Legitimacy Audit

> Required — this phase adds an external Go module dependency (`go-ora`).

**Tool run:** `slopcheck install github.com/sijms/go-ora/v2 --ecosystem go`
(slopcheck v… installed via `pip install slopcheck --break-system-packages`,
2026-07-03). **Side-effect warning:** `slopcheck install` actually executes
`go get` as part of its check (unlike npm/pip equivalents, which can dry-run).
This modified `backend/go.mod`/`backend/go.sum` during research; those
changes were reverted with `git checkout -- backend/go.mod backend/go.sum`
immediately after. **The planner must have the implementation task run
`go get github.com/sijms/go-ora/v2@v2.9.0` itself** — it is not already
present from this research session.

| Package | Registry | Age | Downloads | Source Repo | slopcheck | Disposition |
|---------|----------|-----|-----------|-------------|-----------|-------------|
| `github.com/sijms/go-ora/v2` | Go module proxy | Active since ~2019, v2.9.0 released within the last month (2026-06) | N/A (Go proxy has no download counters) — ~911 GitHub stars [CITED: WebSearch] | github.com/sijms/go-ora (public, active) | `[OK]` (slopcheck note: "No source repository linked" in Go proxy metadata — informational only; the GitHub repo itself is real and verifiable at github.com/sijms/go-ora) | Approved |

**Packages removed due to slopcheck `[SLOP]` verdict:** none.
**Packages flagged as suspicious `[SUS]`:** none.

This package was not "discovered" during this research session — it is the
exact package already used, tested, and validated against real Oracle
infrastructure in the sibling FB_TESTESFC project (per CONTEXT.md D-03 and
confirmed by reading `oracle_fiscal.go` directly). Per the package-name
provenance rule, it is still tagged `[ASSUMED]` in the Standard Stack table
above rather than `[VERIFIED]`, since that prior validation happened in a
different repo/session, not via Context7/official docs in *this* session —
the planner should treat the package choice itself as settled (do not
re-litigate go-ora vs. godror) but keep the human-verify checkpoint for the
actual `go get` + first successful Oracle round-trip in the new environment.

## Architecture Patterns

### System Architecture Diagram

```
 HTTP request (JWT, role=admin)
        │
        ▼
 ┌─────────────────────────────┐
 │ FiscalExecutionRunHandler    │  (new, backend/handlers/fiscal_execution.go)
 │  - resolve company_id (JWT)  │
 │  - load nfe_saidas header    │──────▶ Postgres (nfe_saidas, nfe_saidas_itens)
 │  - load nfe_saidas_itens     │◀──────
 │  - resolveCodEmpresa(CNPJ)   │
 │  - open Oracle conn (D-03)   │──────▶ erp_bridge_config (encrypted creds)
 └───────────────┬──────────────┘         via EncryptField/DecryptField
                 │ processFiscalBatch()
                 ▼
      ┌─────────────────────────┐   sem := make(chan struct{}, 5)
      │  per-item goroutine      │   (cap 5 concurrent Oracle calls)
      │  ┌─────────────────────┐│
      │  │ context.WithTimeout ││   15s per item
      │  │  (ctx, 15*time.Sec) ││
      │  ├─────────────────────┤│
      │  │ lookupGrupoFiscal   ││──────▶ Oracle prod/PRODB
      │  │ (prod, prodb join)  ││◀──────  (sql.ErrNoRows → sem_grupo_fiscal,
      │  ├─────────────────────┤│          not fatal)
      │  │ CallFiscalPackage   ││──────▶ Oracle FCCORP_BKP
      │  │ (static PL/SQL blk) ││◀──────  PKG_FISCAL_FCTAX.calcula_imposto_produto
      │  ├─────────────────────┤│
      │  │ defer recover()     ││   panic isolated → status="error", other
      │  │ (panic isolation)   ││   items keep running
      │  ├─────────────────────┤│
      │  │ persistFiscalItem   ││──────▶ Postgres fiscal_execution_items
      │  │ Result (per-item    ││          INSERT ... ON CONFLICT (nfe_item_id)
      │  │ upsert, own stmt)   ││          DO UPDATE  (never one batch txn)
      │  └─────────────────────┘│
      └─────────────────────────┘
                 │  wg.Wait()
                 ▼
        fiscalExecutionSummary
        {total, ok, sem_grupo_fiscal, error}
                 │
                 ▼
           JSON response
```

A reader can trace the full path: HTTP request → header/item load from
Postgres → per-company Oracle connection using existing encrypted
credentials → fan-out with bounded concurrency → two sequential Oracle calls
per item (lookup, then package execution) → per-item upsert into a new
Postgres table → aggregated summary response. Phase 12 (deferred) reads
`fiscal_execution_items` joined with `nfe_saidas_itens` — no write path from
Phase 12 back into this table.

### Recommended Project Structure

```
backend/
├── services/                          # NEW top-level package — does not exist yet in FB_APU04
│   └── oracle_fiscal.go               # ported from FB_TESTESFC verbatim (package services)
├── handlers/
│   ├── fiscal_group_lookup.go         # NEW — port of FB_TESTESFC file, same name
│   ├── fiscal_execution.go            # NEW — port of FB_TESTESFC file, adapted:
│   │                                  #   - openFiscalOracleConn: same logic, same
│   │                                  #     erp_bridge_config columns, same
│   │                                  #     DecryptFieldWithFallback calls
│   │                                  #   - import "fb_apu04/services" (module name is
│   │                                  #     "fb_apu04" per go.mod, not "fb_testesfc")
│   └── crypto.go                      # EXISTING — reused as-is, no changes needed
├── migrations/
│   ├── 146_nfe_saidas_itens_desc_outro.sql   # NEW — TPF-02
│   └── 147_fiscal_execution_items.sql         # NEW — TPF-04
└── main.go                            # add route registration (withAuth, role="admin")
```

FB_APU04 has no `services/` package today (`grep -rn "^package services"` in
`backend/` returns nothing) — this will be a new top-level package alongside
`handlers/`. This mirrors FB_TESTESFC's own separation (non-HTTP integration
code in `services/`, HTTP handlers in `handlers/`) and is a reasonable,
low-risk structural addition.

### Pattern 1: Static PL/SQL Block via Reflection (TPF-03)

**What:** Build the PL/SQL anonymous block string once from two fixed Go
metadata tables (`fiscalInParams`: 23 entries, `fiscalOutFields`: ~88
entries) — never from user/XML input. All 23 IN values and ~88 OUT
destinations are bound via `sql.Named`/`go_ora.Out`, reflection only locates
*which* Go struct field to read/write, never touches the SQL text.

**When to use:** Any time a fixed-shape Oracle package/procedure needs to be
called safely from Go with no possibility of SQL injection, even though the
number of parameters is large enough that hand-writing every bind call
would be error-prone and hard to keep in sync with the block string.

**Example (from FB_TESTESFC `backend/services/oracle_fiscal.go`, read in full this session):**
```go
// Source: FB_TESTESFC backend/services/oracle_fiscal.go (validated against
// real Oracle FCCORP_BKP, 2026-06-30..07-02)
const fiscalOutStringBufSize = 4000 // Pitfall 1 — see below

func BuildCalculaImpostoBlock() string {
    var b strings.Builder
    b.WriteString("declare\n")
    b.WriteString("  result PKG_FISCAL_FCTAX.RDADOS_FISCAIS_PRODUTO;\n")
    b.WriteString("begin\n")
    b.WriteString("  result := PKG_FISCAL_FCTAX.calcula_imposto_produto(\n")
    for i, p := range fiscalInParams {
        sep := ","
        if i == len(fiscalInParams)-1 { sep = "" }
        fmt.Fprintf(&b, "    %s => :%s%s\n", p.OracleParam, p.OracleParam, sep)
    }
    b.WriteString("  );\n\n")
    for _, f := range fiscalOutFields {
        fmt.Fprintf(&b, "  :o%s := result.%s;\n", f.GoField, f.OracleField)
    }
    b.WriteString("end;")
    return b.String()
}

func buildBindArgs(in FiscalInput, result *FiscalResult) []interface{} {
    args := make([]interface{}, 0, len(fiscalInParams)+len(fiscalOutFields))
    inVal := reflect.ValueOf(in)
    for _, p := range fiscalInParams {
        fv := inVal.FieldByName(p.GoField)
        args = append(args, sql.Named(p.OracleParam, fv.Interface()))
    }
    resVal := reflect.ValueOf(result).Elem()
    for _, f := range fiscalOutFields {
        fv := resVal.FieldByName(f.GoField)
        if fv.Kind() == reflect.String {
            // Pitfall 1: MUST use go_ora.Out with explicit Size, not sql.Out
            args = append(args, sql.Named("o"+f.GoField,
                go_ora.Out{Dest: fv.Addr().Interface(), Size: fiscalOutStringBufSize}))
        } else {
            args = append(args, sql.Named("o"+f.GoField, sql.Out{Dest: fv.Addr().Interface()}))
        }
    }
    return args
}
```

### Pattern 2: `nfe_saidas_itens` extension for TPF-02

**What:** `prod` struct (in `backend/handlers/nfe_saidas.go`) already parses
`VDesc string `xml:"vDesc"`` but `insertNFeItens` never writes it. `VOutro`
(`xml:"vOutro"`, accessory expenses) doesn't exist on the item-level `prod`
struct at all today — only at the header level (`icmsTot.VOutro`).

**Confirmed exact edit locations** (all in `backend/handlers/nfe_saidas.go`):
1. `prod` struct (line 82-90): add `VOutro string `xml:"vOutro"`` alongside existing `VDesc`.
2. `insertNFeItens` (line 373-459): add `v_desc` and `v_outro` to the column
   list, `VALUES` placeholders, `ON CONFLICT DO UPDATE SET` clause, and the
   final positional args (`toDecimal(d.Prod.VDesc)`, `toDecimal(d.Prod.VOutro)`).
3. New migration adds the two nullable/defaulted columns to `nfe_saidas_itens`.

**Example migration (idempotent, matches existing style seen in migrations 094/095/141):**
```sql
-- 146_nfe_saidas_itens_desc_outro.sql
-- TPF-02: despesas acessórias (vOutro) e desconto (vDesc) por item, necessários
-- como pDespesas/pDesconto de entrada do pacote fiscal (PKG_FISCAL_FCTAX).
ALTER TABLE nfe_saidas_itens
    ADD COLUMN IF NOT EXISTS v_desc  NUMERIC(15,2) DEFAULT 0,
    ADD COLUMN IF NOT EXISTS v_outro NUMERIC(15,2) DEFAULT 0;
```

**Note on scope:** TPF-02's requirement text names `nfe_saidas_itens`
specifically (this module only processes saída notes). FB_APU04's past
convention (migrations 094, 095, 141) has always mirrored item-level column
additions across *both* `nfe_entradas_itens` and `nfe_saidas_itens` for
schema symmetry, even when only one side was immediately needed. This is a
low-cost, low-risk consistency call left to the planner — adding the same
two columns to `nfe_entradas_itens` in the same migration costs nothing and
matches established precedent, but is not a hard requirement of TPF-02.

### Pattern 3: Hybrid schema for `fiscal_execution_items` (TPF-04)

**What:** Instead of ~88 individual typed columns, use a small number of
dedicated columns for whatever the comparison screen (Phase 12) needs to
sort/filter/diff quickly, plus one `full_result JSONB NOT NULL` column
holding the complete struct for audit/"only calculated" detail views.

**Source (from FB_TESTESFC `backend/migrations/008_fiscal_execution_items.sql`, read in full):**
```sql
CREATE TABLE IF NOT EXISTS fiscal_execution_items (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id          UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    nfe_item_id         UUID NOT NULL REFERENCES nfe_saidas_itens(id) ON DELETE CASCADE,

    status              TEXT NOT NULL DEFAULT 'pending', -- pending | ok | error | sem_grupo_fiscal
    error_message       TEXT,
    executed_at         TIMESTAMPTZ,

    grupo_fiscal_codigo TEXT,
    input_params        JSONB,            -- snapshot of the 23 IN params actually sent

    base_calculo_icms           NUMERIC(15,2),
    valor_icms                  NUMERIC(15,2),
    base_substituicao           NUMERIC(15,2),
    valor_substituicao          NUMERIC(15,2),
    base_calculo_pis            NUMERIC(15,2),
    valor_pis                   NUMERIC(15,2),
    base_calculo_cofins         NUMERIC(15,2),
    valor_cofins                NUMERIC(15,2),
    percentual_difal            NUMERIC(7,4),
    valor_icms_partilha_destino NUMERIC(15,2),
    valor_icms_pobreza          NUMERIC(15,2),

    full_result         JSONB NOT NULL,   -- complete ~88-field result

    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_fiscal_execution_item UNIQUE (nfe_item_id)
);

CREATE INDEX IF NOT EXISTS idx_fiscal_execution_status ON fiscal_execution_items(company_id, status);
CREATE INDEX IF NOT EXISTS idx_fiscal_execution_nfe_item ON fiscal_execution_items(nfe_item_id);
```

**Recommended adjustment for FB_APU04 (TPF-04/TPF-06 gap — see Open Questions):**
FB_APU04's TPF-06 requires divergence detection on IBS/CBS in addition to
ICMS/ICMS-ST/PIS/COFINS — FB_TESTESFC's original comparison screen did not
cover IBS/CBS as typed columns (its own migration comment says so
explicitly: *"campos da Reforma Tributária (IBS UF, IBS Município, CBS)
ainda não mapeados para colunas dedicadas"*). Recommend adding a small
number of additional typed columns during **this** phase (schema is owned
by Phase 11, and Phase 12 cannot retroactively add columns without another
migration) so Phase 12 doesn't have to extract IBS/CBS values out of
`full_result` JSONB for its comparison table:
```sql
-- Additional columns beyond the FB_TESTESFC original, for TPF-06 (IBS/CBS
-- divergence support in Phase 12):
    valor_ibs_uf         NUMERIC(15,2),  -- result.ValorIbsUF
    valor_ibs_mun        NUMERIC(15,2),  -- result.ValorIbsMUN
    valor_cbs            NUMERIC(15,2),  -- result.ValorCbs
```

### Pattern 4: Bounded-concurrency batch with per-item error isolation (TPF-05)

**What:** Semaphore-capped goroutine fan-out, per-item timeout, panic
recovery, per-item upsert — never one shared transaction, never one shared
timeout for the whole batch.

**This exact concurrency shape does not exist anywhere else in FB_APU04
today.** Closest existing patterns are single background goroutines
(`xml_upload.go:225`, `worker.go:44`) that do sequential work inside one
goroutine, and short (2-5s) `context.WithTimeout` calls in `auth.go`/`job.go`
for single synchronous operations — none of them fan out N goroutines with a
semaphore. This is a **new pattern being introduced**, not an established
FB_APU04 convention being reused. The planner should treat this as new code
to review carefully rather than "just like elsewhere in the codebase."

**Example (from FB_TESTESFC `backend/handlers/fiscal_execution.go`, read in full):**
```go
// Source: FB_TESTESFC backend/handlers/fiscal_execution.go
func processFiscalBatch(ctx context.Context, oracleDB *sql.DB, pgDB *sql.DB,
    companyID string, nfe fiscalNotaContext, itens []fiscalItemInput) fiscalExecutionSummary {
    summary := fiscalExecutionSummary{Total: len(itens)}
    var mu sync.Mutex
    sem := make(chan struct{}, 5)
    var wg sync.WaitGroup

    for _, item := range itens {
        wg.Add(1)
        sem <- struct{}{}
        go func(it fiscalItemInput) {
            defer wg.Done()
            defer func() { <-sem }()
            defer func() {
                if rec := recover(); rec != nil {
                    // panic isolated to this item; persist as status="error"
                    // and continue — never abort the batch
                }
            }()
            itemCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
            defer cancel()
            // ... lookupGrupoFiscal → CallFiscalPackage → persistFiscalItemResult
        }(item)
    }
    wg.Wait()
    return summary
}
```

### Anti-Patterns to Avoid

- **Concatenating any value into the PL/SQL block string:** the entire
  security model of this integration rests on the block being built purely
  from fixed metadata tables. Never `fmt.Sprintf` a user/XML value into the
  block text — always `sql.Named`/`go_ora.Out`.
- **Using `sql.Out{Dest: ptr}` for string OUT binds:** produces
  `ORA-06502: buffer too small` under go-ora because the generic
  `database/sql.Out` sends `size=0`. Always use `go_ora.Out{Dest: ptr, Size: 4000}`
  for string fields (see Pitfall 1).
- **Typing `IdRegraCalculo*` fields as `float64`/NUMBER:** they are VARCHAR2
  in the real Oracle object and will throw `ORA-06502: character to number
  conversion error` if bound as numeric. Always `string`.
- **One transaction for the whole batch:** violates TPF-05 explicitly — a
  single failed item must never roll back or block the others. Each item is
  its own `INSERT ... ON CONFLICT DO UPDATE`.
- **Guessing `cod_empresa`:** `resolveCodEmpresa` must return an explicit
  error (never a fabricated value) when the CNPJ root isn't in
  `codEmpresaPorCNPJRaiz`. Today only Recife/PE (`10230480`) is mapped.
- **A single timeout for the entire batch instead of per-item:** the
  15-second timeout in FB_TESTESFC is applied per goroutine (`itemCtx`), not
  once for the whole `processFiscalBatch` call. The outer `ctx` passed in
  from the HTTP handler had a much longer bound (10 minutes in FB_TESTESFC)
  purely as an overall backstop.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Oracle credential encryption at rest | A new encryption scheme for the Go-side Oracle connection | `EncryptField`/`DecryptField`/`DecryptFieldWithFallback` from `backend/handlers/crypto.go` (AES-256-GCM, key derived from `ENCRYPTION_KEY` env var) | Already in production, already used for these exact `erp_bridge_config` columns (`oracle_dsn`, `oracle_usuario`, `oracle_senha`) by `erp_bridge.go`. Building a second scheme would fragment key management and create an inconsistency for zero benefit. |
| Bounded-concurrency worker pool | A custom worker-pool package/library | Plain `chan struct{}` semaphore + `sync.WaitGroup` (as in FB_TESTESFC) | This is already how the validated code does it; it's ~15 lines and fully auditable — no need for a third-party pool library for a fixed cap of 5. |
| Dynamic SQL/PL·SQL parameter binding | Any form of string templating or `fmt.Sprintf` into the PL/SQL block | The reflection-over-fixed-metadata-table pattern in `oracle_fiscal.go` | This is the core security control (T-02-02 in FB_TESTESFC's own threat model comments) — any hand-rolled alternative reintroduces injection risk for no gain, since the metadata-table approach already handles the "23 in / 88 out is a lot to hand-wire" problem cleanly. |
| Postgres migration numbering/idempotency tooling | A migration framework (golang-migrate, goose, etc.) | The existing bespoke `schema_migrations` + `filepath.Glob("*.sql")` runner already in `main.go`/`onDBConnected()` | FB_APU04 already has 145 migrations running through this mechanism; introducing a migration framework mid-project for one phase would be a much bigger, unrelated change. |

**Key insight:** Every piece of infrastructure this phase needs (encryption,
migration runner, route/auth wiring, DB pool pattern) already exists in
FB_APU04 in a form that's directly reusable. The only genuinely new
capabilities are the Oracle driver dependency and the bounded-concurrency
batch pattern — both of which already have a working, previously-validated
reference implementation to port rather than design from scratch.

## Common Pitfalls

### Pitfall 1: go-ora OUT string bind buffer size

**What goes wrong:** `ORA-06502: PL/SQL: numeric or value error: character
string buffer too small` when calling the package.
**Why it happens:** `database/sql`'s generic `sql.Out{Dest: ptr}` sends
`size=0` to the go-ora driver for string OUT binds, so the driver allocates
a zero-length buffer.
**How to avoid:** Use the driver-native type `go_ora.Out{Dest: ptr, Size: 4000}`
(import `go_ora "github.com/sijms/go-ora/v2"`) for every string-typed OUT
field. Numeric OUT fields can keep using the generic `sql.Out`.
**Warning signs:** Works fine when a string OUT field happens to come back
empty in testing, fails only once Oracle tries to write a non-empty value
into it — easy to miss in a quick smoke test with sparse test data.

### Pitfall 2: `IdRegraCalculo*` fields are VARCHAR2, not NUMBER

**What goes wrong:** `ORA-06502: character to number conversion error`.
**Why it happens:** The field names (`IdRegraCalculoIcms`,
`IdRegraCalculoPisCofins`, `IdRegraCalculoIpi`, `IdRegraCalculoIbs`,
`IdRegraCalculoCbs`) look numeric ("Id") but the real Oracle object type
declares them VARCHAR2 — confirmed against real Oracle in FB_TESTESFC on
2026-07-01 (values look like `"IVA_N_FC01PEPE1SNVRJNE6811810030002IC61"`).
**How to avoid:** Type all five fields as `string` in the Go struct, never
`float64`/`int`.
**Warning signs:** Any `Id*` field name is a trap — verify the actual Oracle
object type definition rather than trusting the name.

### Pitfall 3: `cod_empresa` mapping is incomplete by design

**What goes wrong:** Notes from filiais other than Recife/PE (CNPJ root
`10230480`) will always fail TPF-01's lookup with an explicit error.
**Why it happens:** Only one CNPJ root is confirmed against real Oracle
today; Garanhuns/PE (`cod_empresa=1`) has no confirmed root.
**How to avoid:** This is an accepted, documented gap for this phase (see
CONTEXT.md `<specifics>` and REQUIREMENTS.md Out of Scope table) — the
correct behavior is an explicit per-item error, not a guess. Do not "fix"
this by defaulting to `cod_empresa=2` for unmapped filiais.
**Warning signs:** If someone reports "every item from filial X comes back
`sem_grupo_fiscal` or `error`," check whether X's CNPJ root is in
`codEmpresaPorCNPJRaiz` before assuming a bug in the lookup query itself.

### Pitfall 4: Oracle session/connection exhaustion under load

**What goes wrong:** Oracle-side connection/session limits hit if the
concurrency cap and pool size aren't kept in lockstep.
**Why it happens:** The semaphore caps in-flight goroutines at 5, but if the
underlying `*sql.DB`'s `SetMaxOpenConns` is left at its default (unlimited)
or set higher than 5, `database/sql` may still open more physical
connections than the intended cap under certain retry/backoff conditions.
**How to avoid:** Always pair the semaphore cap with
`oracleDB.SetMaxOpenConns(5)` on the same dedicated Oracle pool — both
numbers must match (FB_TESTESFC hardcodes both to 5).
**Warning signs:** Oracle-side `ORA-12520`/session-limit errors under
concurrent batch runs even though the semaphore "looks" correct in code
review — check the pool's `SetMaxOpenConns` value too.

### Pitfall 5: `slopcheck install` mutates `go.mod`/`go.sum` as a side effect

**What goes wrong:** Running `slopcheck install <pkg> --ecosystem go` (per
the standard Package Legitimacy Gate protocol) actually executes `go get`
under the hood, unlike the npm/pip equivalents which can dry-run.
**Why it happens:** slopcheck's Go-ecosystem support shells out to `go get`
to resolve/verify the module rather than only querying the proxy API.
**How to avoid:** If re-running this check outside of the actual
implementation task, revert `backend/go.mod`/`backend/go.sum` afterward
with `git checkout -- backend/go.mod backend/go.sum` (as done during this
research session). The implementation plan should treat the `go get`
invocation as the real, intentional dependency-add step, not something to
redo separately.

## Code Examples

All three examples below are read directly from
`/home/claudiobezerra/projetos/FB_TESTESFC` during this research session
(full files, not the summarized handoff) and are ready to copy/adapt with
only import-path and package-name changes for FB_APU04 (module name
`fb_apu04`, not `fb_testesfc`).

### Grupo fiscal lookup (TPF-01)

```go
// Source: FB_TESTESFC backend/handlers/fiscal_group_lookup.go (full file read 2026-07-03)
var errSemGrupoFiscal = errors.New("produto não encontrado em prod/PRODB")

var codEmpresaPorCNPJRaiz = map[string]int{
    "10230480": 2, // Ferreira Costa — Recife/PE (única raiz confirmada)
}

func resolveCodEmpresa(emitCNPJ, emitUF string) (int, error) {
    digits := onlyDigits(emitCNPJ)
    if len(digits) < 8 {
        return 0, fmt.Errorf("CNPJ do emitente inválido para resolução de cod_empresa")
    }
    raiz := digits[:8]
    if cod, ok := codEmpresaPorCNPJRaiz[raiz]; ok {
        return cod, nil
    }
    return 0, fmt.Errorf("cod_empresa não mapeado para a filial do emitente (CNPJ raiz %s, UF %s)", raiz, emitUF)
}

func lookupGrupoFiscal(ctx context.Context, oracleDB *sql.DB, codigoProduto string, codEmpresa int) (grupoFiscal, origem, ncm string, err error) {
    const query = `
        SELECT pb.grupo_fiscal, p.especial AS origem, p.ncm
        FROM prodb pb, prod p
        WHERE p.codigo = pb.codigo
          AND pb.codigo = :codigoProduto
          AND pb.cod_empresa = :codEmpresa`
    var grupoFiscalNS, origemNS, ncmNS sql.NullString
    row := oracleDB.QueryRowContext(ctx, query,
        sql.Named("codigoProduto", codigoProduto),
        sql.Named("codEmpresa", codEmpresa))
    if scanErr := row.Scan(&grupoFiscalNS, &origemNS, &ncmNS); scanErr != nil {
        if scanErr == sql.ErrNoRows {
            return "", "", "", errSemGrupoFiscal
        }
        return "", "", "", scanErr // never expose scanErr.Error() raw to the client
    }
    return grupoFiscalNS.String, origemNS.String, ncmNS.String, nil
}
```

### Opening the dedicated Oracle connection reusing `erp_bridge_config` (D-03)

```go
// Source: FB_TESTESFC backend/handlers/fiscal_execution.go (adapted names for
// FB_APU04's existing crypto.go, which already exports the same function names)
func openFiscalOracleConn(db *sql.DB, companyID string) (*sql.DB, error) {
    var oracleDsn, oracleUsuario, oracleSenha sql.NullString
    err := db.QueryRow(`
        SELECT oracle_dsn, oracle_usuario, oracle_senha
        FROM erp_bridge_config WHERE company_id = $1`, companyID,
    ).Scan(&oracleDsn, &oracleUsuario, &oracleSenha)
    if err != nil {
        return nil, fmt.Errorf("credenciais Oracle não configuradas para a empresa")
    }
    if !oracleDsn.Valid || strings.TrimSpace(oracleDsn.String) == "" {
        return nil, fmt.Errorf("DSN Oracle não configurado")
    }

    dsnPlain := DecryptFieldWithFallback(oracleDsn.String)      // already in backend/handlers/crypto.go
    usuarioPlain := DecryptFieldWithFallback(oracleUsuario.String)
    senhaPlain := DecryptFieldWithFallback(oracleSenha.String)

    var connStr string
    if strings.HasPrefix(dsnPlain, "oracle://") {
        connStr = dsnPlain
    } else {
        connStr = fmt.Sprintf("oracle://%s:%s@%s", usuarioPlain, senhaPlain, dsnPlain)
    }

    conn, err := sql.Open("oracle", connStr) // driver name registered by go_ora's init()
    if err != nil {
        return nil, fmt.Errorf("falha ao inicializar conexão Oracle")
    }
    conn.SetMaxOpenConns(5) // must match the semaphore cap — Pitfall 4
    return conn, nil
}
```

Note: `sql.Open("oracle", ...)` works because `github.com/sijms/go-ora/v2`
registers itself as the `"oracle"` driver via `database/sql`'s driver
registry in its package `init()` — no explicit driver object needs to be
passed, exactly like `lib/pq` registering `"postgres"` in FB_APU04 today.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| Oracle only reachable via external Python bridge (async, out-of-process) | Go backend opens its own synchronous Oracle connection at request time | This phase (2026-07) | First time FB_APU04's Go process talks to Oracle directly; new failure mode to monitor (Oracle reachability from the Go container, not just from the bridge host) |

No deprecated/outdated library concerns — `go-ora` v2.x is the current
major version and actively released (v2.9.0, 2026-06).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `github.com/sijms/go-ora/v2` is the correct/only Oracle driver to add — no alternative already referenced anywhere in FB_APU04 | Standard Stack | Low — confirmed by exhaustive grep of the codebase for `go-ora`/`sijms`/`godror`/`goracle`; only hits were this phase's own planning docs. Also independently confirmed as a legitimate, actively maintained package via Go proxy + Oracle's own developer blog + go.dev Wiki. |
| A2 | The hybrid `fiscal_execution_items` schema (11 typed columns + JSONB) is preferable to a literal 88-column table for FB_APU04, despite CONTEXT.md's discretion note leaning toward "map 1:1" | Architecture Patterns → Pattern 3 | Medium — if the planner/user actually wants full per-field typed columns (e.g. for future SQL-level filtering on any of the 88 fields, not just the ones Phase 12 currently needs), this recommendation would need to be revisited. Low implementation cost either way since it's Claude's Discretion, but worth a quick confirmation before Phase 11 execution given it diverges from a literal reading of CONTEXT.md. |
| A3 | Adding `valor_ibs_uf`/`valor_ibs_mun`/`valor_cbs` as extra typed columns (beyond FB_TESTESFC's original 11) is worth doing now, in Phase 11, rather than deferring | Architecture Patterns → Pattern 3 | Low-medium — if skipped, Phase 12 can still get IBS/CBS values out of `full_result` JSONB with `->>'ValorIbsUF'`-style queries, just less ergonomically/indexably. Not a blocking risk, but cheap to add now vs. a later migration. |
| A4 | `nfe_saidas_itens` currently has **no** per-item IBS/CBS columns populated by the XML parser (only header-level `nfe_saidas.v_ibs`/`v_cbs` are ever written, and even those may be 0 for most 2026 XMLs given the Reforma Tributária phase-in) — meaning the "esperado" side of any future IBS/CBS divergence check in Phase 12 may have no reliable ground truth at item granularity | Summary / Open Questions | Medium — this is a real gap discovered by reading the XML struct definitions directly (`det`/`prod`/`detImposto` has no per-item IBS/CBS group at all); it affects how meaningful Phase 12's IBS/CBS divergence column will be, but does not block Phase 11 (Phase 11 just needs to calculate and store the value; Phase 12 decides how to display "esperado" for a field that may not exist per-item in the XML). |
| A5 | The five `defaultTipoContribuinte`/`defaultTipoCentroFiscal`/etc. constants ported from FB_TESTESFC remain appropriate defaults for FB_APU04's saída notes | Code Examples (implied, per CONTEXT.md `<specifics>`) | Medium — explicitly flagged already in CONTEXT.md as validated only for the "normal sale" path; Simples Nacional / serviço paths may expose wrong defaults. This surfaces as a visible divergence in Phase 12, not a Phase 11 blocker (already accepted risk per CONTEXT.md). |

## Open Questions (RESOLVED)

> Resolucao: todas as 3 questoes foram resolvidas durante o planejamento da Fase 11 e os planos ja implementam as recomendacoes abaixo. Notas inline em **RESOLVIDO** por item.

1. **Should `fiscal_execution_items` get literal 88 columns or the hybrid model?**
   - **RESOLVIDO:** Adotado o modelo hibrido (Pattern 3 — 11 colunas tipadas + `full_result` JSONB), implementado em Plan 11-03 (migration 147) e consumido por Plan 11-05. Captura 100% dos ~88 campos para auditoria; desvio de leitura literal do CONTEXT.md sinalizado ao usuario no plan review.
   - What we know: FB_TESTESFC's validated, working implementation used the
     hybrid model (11 typed + JSONB). CONTEXT.md's discretion note suggests
     "map 1:1."
   - What's unclear: Whether the "map 1:1" phrasing was a firm intent or
     just shorthand for "make sure every field is captured somewhere"
     (which the hybrid model already satisfies via `full_result`).
   - Recommendation: Use the hybrid model (Pattern 3) — it's proven, and
     the JSONB column captures 100% of the 88 fields for audit purposes
     regardless. Flag this choice explicitly to the user during plan
     review since it's a visible deviation from a literal reading of
     CONTEXT.md.

2. **Does Phase 12's IBS/CBS divergence check need item-level "esperado" data that doesn't exist today?**
   - **RESOLVIDO (para a Fase 11):** Fora de escopo desta fase — Plan 11-04/11-05 apenas calcula e armazena `valor_ibs_uf`/`valor_ibs_mun`/`valor_cbs` do resultado do pacote Oracle. A decisao de como exibir o "esperado" IBS/CBS foi deferida para o planejamento da Fase 12 (comparacao calculado-vs-esperado).
   - What we know: `nfe_saidas_itens` has `v_ibs`/`v_cbs` columns (added in
     migration 075) but `insertNFeItens` never populates them; the XML
     `det`/`prod` struct has no per-item IBS/CBS group to parse from in the
     first place (IBS/CBS only appears at the NF-e header/total level today).
   - What's unclear: Whether the NF-e 4.00 transitional layout for 2026 even
     defines a per-item IBS/CBS group yet, or whether IBS/CBS is
     header-total-only during this phase-in period.
   - Recommendation: Out of scope for Phase 11 either way (Phase 11 only
     needs to *calculate and store* `valor_ibs_uf`/`valor_ibs_mun`/`valor_cbs`
     from the Oracle package result). Flag for Phase 12 planning: the
     "esperado" side of an IBS/CBS comparison may need to come from the
     header total divided/estimated, or simply be omitted/shown as "N/A"
     rather than "0" to avoid a false-positive divergence.

3. **Should the `v_desc`/`v_outro` migration also touch `nfe_entradas_itens` for symmetry?**
   - **RESOLVIDO:** Migration dual-table adotada em Plan 11-02 (adiciona `v_desc`/`v_outro` a saidas e entradas), mantendo a convencao estabelecida no codebase (094/095/141).
   - What we know: Every prior item-level column addition (094, 095, 141)
     touched both entradas and saídas tables together, even when only one
     side had an immediate consumer.
   - What's unclear: Whether that symmetry is a hard team convention or
     coincidental (all three of those additions happened to be useful to
     both entradas and saídas features).
   - Recommendation: Low-cost either way; lean toward adding both for
     consistency with the codebase's established pattern, but this is not
     a TPF-02 requirement — purely a style/consistency call for the planner.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `github.com/sijms/go-ora/v2` (Go module) | TPF-01, TPF-03 | Not yet in `go.mod` — confirmed absent | N/A (to be added: v2.9.0) | None needed — `go get` during implementation is the intended path, not a blocker |
| Oracle `prod`/`PRODB` network reachability from the Go backend's runtime environment | TPF-01, TPF-03 | **Unknown at research time** — this research session ran on the local dev machine, not the Coolify/Hostinger deploy target; cannot verify network path from the actual backend container to Oracle's internal `10.131.x.x` range | — | If unreachable in production, this mirrors the exact problem that caused FB_TESTESFC to be discontinued (per `.continue-here.md`) — the stated reason FB_APU04 was chosen as the new home is that "FB_APU04 já roda com acesso Oracle (bridge AWS existente)," implying the Go backend's host *does* have network access, but this has not been independently re-verified in this research session. **Recommend a `checkpoint:human-verify` task early in the plan** (e.g., a minimal `sql.Open("oracle", ...); conn.Ping()` smoke test against real credentials) before investing in the full port. |
| `erp_bridge_config` populated with valid `oracle_dsn`/`oracle_usuario`/`oracle_senha` for at least one company | TPF-01, TPF-03, TPF-05 (end-to-end testing) | Presumed available (existing production feature used by the Python bridge daily) — not independently re-verified in this session | — | None needed if presumption holds; if the values are stale/wrong, TPF-05's endpoint will surface `"Falha ao conectar ao Oracle. Verifique as credenciais ERP configuradas."` immediately and clearly, per the ported `openFiscalOracleConn` error message |

**Missing dependencies with no fallback:**
- Oracle network reachability from the Go backend container — must be
  smoke-tested early (see recommendation above); this is the single biggest
  risk to this phase succeeding, and it's exactly the risk category that
  killed the FB_TESTESFC standalone product.

**Missing dependencies with fallback:**
- `go-ora` module itself — trivially added via `go get`, no fallback needed.

## Validation Architecture

Skipped — `.planning/config.json` has `workflow.nyquist_validation: false`
explicitly set.

For reference (not a required section, but useful context for the planner):
FB_APU04's existing Go tests use plain stdlib `testing` +
`net/http/httptest` with a "guard test" convention (verify 405 on wrong
method and 401 on missing auth by passing a `nil` `*sql.DB`, since those
checks short-circuit before any DB access — see
`backend/handlers/icms_fronteira_st_itens_guards_test.go`). No `testify` or
other assertion library is used anywhere in the module. The new
`FiscalExecutionRunHandler` should follow this same guard-test convention
for its method/auth checks; anything touching the real Oracle connection is
inherently better suited to manual/checkpoint verification than automated
unit tests, given there's no Oracle test double in this codebase.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | Yes | Existing JWT (`ClaimsKey` context value) + `AuthMiddleware` — reused as-is via `withAuth(handler, "admin")` in `main.go`, no new auth mechanism |
| V3 Session Management | No | No new session concept introduced; rides on existing JWT session handling |
| V4 Access Control | Yes | New endpoint should be registered with role `"admin"` (matching the pattern for other operationally-sensitive endpoints like `/api/erp-bridge/config/generate-api-key`, `/api/admin/reset-db`); company scoping enforced via `erpBridgeGetCompany`/`GetEffectiveCompanyID` exactly as `erp_bridge.go` does today |
| V5 Input Validation | Yes | `nfe_id` from request body must be validated as belonging to the authenticated `company_id` before any Oracle work starts (exact pattern already in FB_TESTESFC's `FiscalExecutionRunHandler`: the `WHERE id = $1 AND company_id = $2` guard) |
| V6 Cryptography | Yes | Oracle credentials at rest — reuse `EncryptField`/`DecryptField` (AES-256-GCM) from `backend/handlers/crypto.go`; never hand-roll a second scheme for this connection |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Dynamic PL/SQL block construction from user/XML input | Tampering | 100% static block built only from fixed Go metadata tables (`fiscalInParams`/`fiscalOutFields`); all values travel via `sql.Named`/`go_ora.Out` bind variables, never string-concatenated (this is the core control already designed into the ported code — see Pattern 1) |
| Cross-company data access (IDOR) via `nfe_id` in request body | Tampering / Information Disclosure | `WHERE id = $1 AND company_id = $2` guard on every load, exactly as FB_TESTESFC's `FiscalExecutionRunHandler` already does; `company_id` always derived server-side from the JWT via `erpBridgeGetCompany`, never accepted from the client body |
| Oracle DSN/credentials leaking into error responses | Information Disclosure | `openFiscalOracleConn` never returns `err.Error()` from `sql.Open`/connection failures directly to the HTTP client — always a generic message ("Falha ao conectar ao Oracle...") while the detailed error is only `log.Printf`'d server-side; same discipline already applied in `lookupGrupoFiscal`'s scan-error handling |
| Oracle session/connection exhaustion from an unbounded batch (DoS against the shared Oracle instance) | Denial of Service | Semaphore cap (5) + matching `SetMaxOpenConns(5)` + per-item 15s timeout, so a single company's batch run can never monopolize more than 5 concurrent Oracle sessions or hang indefinitely on one stuck item |
| Admin-only endpoint reachable by non-admin authenticated users | Elevation of Privilege | Register the route with `withAuth(handler, "admin")`, matching the existing convention for other sensitive endpoints in `main.go` |

## Sources

### Primary (HIGH confidence)

- `/home/claudiobezerra/projetos/FB_TESTESFC/backend/services/oracle_fiscal.go` — read in full; source of the 23 IN params, ~88 OUT fields, static block builder, both go-ora pitfalls
- `/home/claudiobezerra/projetos/FB_TESTESFC/backend/handlers/fiscal_group_lookup.go` — read in full; `resolveCodEmpresa`, `lookupGrupoFiscal`
- `/home/claudiobezerra/projetos/FB_TESTESFC/backend/handlers/fiscal_execution.go` — read in full; batch handler, concurrency pattern, `openFiscalOracleConn`, `persistFiscalItemResult`
- `/home/claudiobezerra/projetos/FB_TESTESFC/backend/handlers/fiscal_comparison.go` — read in full; confirmed which `fiscal_execution_items` columns Phase 12's predecessor actually queried
- `/home/claudiobezerra/projetos/FB_TESTESFC/backend/migrations/008_fiscal_execution_items.sql` — read in full; actual validated hybrid schema
- `/home/claudiobezerra/projetos/FB_APU04/backend/handlers/erp_bridge.go` — read in full; confirmed `EncryptField`/`DecryptField`/`DecryptFieldWithFallback` usage pattern for Oracle credentials
- `/home/claudiobezerra/projetos/FB_APU04/backend/handlers/crypto.go` — read in full; AES-256-GCM implementation, function signatures
- `/home/claudiobezerra/projetos/FB_APU04/backend/handlers/nfe_saidas.go` — read (structs + `insertNFeItens` in full); confirmed TPF-02 exact gap
- `/home/claudiobezerra/projetos/FB_APU04/backend/main.go` (relevant sections) — `getDB()`/`dbMutex` pattern, `withAuth`/`withDB` route wiring convention
- `/home/claudiobezerra/projetos/FB_APU04/backend/migrations/{065,075,094,095,106,141}_*.sql` — read in full; confirmed `erp_bridge_config`, `nfe_saidas_itens` schema history, no naming collision with `fiscal_calculations`
- `go list -m -versions github.com/sijms/go-ora/v2` — live query against proxy.golang.org, 2026-07-03, confirmed v2.9.0 is latest
- `slopcheck install github.com/sijms/go-ora/v2 --ecosystem go` — run 2026-07-03, `[OK]` verdict

### Secondary (MEDIUM confidence)

- WebSearch: "github.com/sijms/go-ora latest release version go.mod" — corroborated v2.9.0, June 2026 release date
- WebSearch: "sijms/go-ora github stars pure go oracle driver production ready" — corroborated ~911 stars, referenced by Oracle's own developer blog and go.dev's official SQL Drivers wiki page as a legitimate pure-Go Oracle driver option

### Tertiary (LOW confidence)

None — every claim in this document is backed by either direct source-code
inspection (primary) or a cross-verified web source (secondary).

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — go-ora is the only candidate, already validated against real Oracle in the sibling project, and independently corroborated as a legitimate package via authoritative sources
- Architecture: HIGH — all three core files read in full from the actual working implementation, not summarized; the one deviation recommended (hybrid schema + IBS/CBS columns) is clearly flagged as a recommendation, not asserted as fact
- Pitfalls: HIGH — both go-ora gotchas are documented directly in the source code's own comments, with the exact Oracle error codes they produce

**Research date:** 2026-07-03
**Valid until:** 30 days (stable domain — Oracle package contract and FB_APU04 codebase conventions are not fast-moving; re-verify go-ora version if implementation is delayed beyond that window)
