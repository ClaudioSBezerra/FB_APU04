# Architecture Patterns — Reforma Tributária Analytics Modules

**Domain:** Fiscal analytics on existing EFD/XML data
**Researched:** 2026-05-22
**Confidence:** HIGH — derived from direct codebase inspection

---

## Context

This document describes how 8 new analytics modules integrate into the existing FB_APU04 Go+PostgreSQL+React stack. All findings are derived from reading the actual source files, not from assumptions.

---

## Key Findings from Codebase Inspection

### What already exists

- `tabela_aliquotas` — global (no company_id), keyed by `ano` (2027–2033). Contains `perc_ibs_uf`, `perc_ibs_mun`, `perc_cbs`, `perc_reduc_icms`. Currently hardcoded at `ano = 2033` in most handlers.
- `ncm_cclasstrib_reforma` — global reference table, 95 NCMs seeded, columns `ncm_digits`, `ibs_reducao_pct`, `cbs_reducao_pct`, `cclasstrib`.
- `forn_simples (cnpj)` — global table, populated at XML parse time when `emit.CRT == "1"`. Used by existing MVs.
- `reg_c190` — linked to `reg_c100` via `id_pai_c100`, linked to `import_jobs` via `job_id`. Has `cfop`, `vl_opr`, `vl_bc_icms`, `vl_icms`, `vl_ipi`. **No `cst_icms` column** — CST does not exist in C190 (C190 aggregates by CFOP, not by CST).
- `reg_c170` — **does not exist in the codebase**. The worker parses C100 and C190; C170 item-level lines are not parsed. Módulo 1.1 must use C190 (CFOP-based, not CST-based).
- `nfe_entradas_itens` / `nfe_saidas_itens` — created in migration 075. Have `company_id`, `ncm`, `cfop`, `cst_icms`, `v_prod`, `v_bc_icms`, `v_icms`, `v_pis`, `v_cofins`, `v_ibs`, `v_cbs`. Indexed on `(company_id, ncm)`.
- `nfe_entradas` — has `forn_cnpj`, `forn_uf`, `data_emissao`, `mes_ano`, `company_id`. **No `forn_crt` column** — supplier CRT is tracked only via the `forn_simples` CNPJ lookup table.
- `nfe_saidas` — has `emit_cnpj`, `emit_uf`, `dest_uf`, `dest_cnpj_cpf`, `data_emissao`, `mes_ano`, `company_id`. **No `ind_final` column** — B2B/B2C must be inferred from whether `dest_cnpj_cpf` is 14-digit CNPJ vs 11-digit CPF.
- `ide` XML struct — does not parse `indFinal`. Adding it requires extending the struct and adding a column to `nfe_saidas`.
- Handler pattern: `func XHandler(db *sql.DB) http.HandlerFunc` registered via `withAuth(handlers.XHandler, "")` in `main.go`.
- No period-window abstraction exists. Handlers use `mes_ano = $N` for single-period or accept an `?ano=` query param. The 12-month window must be implemented as a query-level filter.

---

## Recommended Architecture

### 1. Handler File Grouping

**Decision: Group by milestone block, not one file per module.**

The existing pattern uses one file per functional domain (`xml_comparativo.go` = 5 handlers, `creditos_perdidos.go` = 1 handler). The 8 new modules split into two natural groups:

| File (new) | Modules | Handler count |
|---|---|---|
| `backend/handlers/reforma_modulo1.go` | 1.1 Créditos Bloqueados, 1.2 Reprecificação, 1.3 Fornecedores, 1.4 Split Payment | 4 |
| `backend/handlers/reforma_modulo2.go` | 2.1 Por NCM, 2.2 Por CFOP, 2.3 Por UF, 2.4 B2B/B2C | 4 |

Rationale: Modules 1.x operate on different data sources but share the same "what is my direct tax exposure?" frame. Modules 2.x are cross-cutting dimensional cuts over the same underlying data. Splitting into exactly two files keeps each file under 300 lines and groups handlers that share helper functions (rate lookup, period window builder).

### 2. Rate Table Strategy

**Decision: Hybrid — global `tabela_aliquotas` for phase rates + new `reforma_parametros` per-company config for adjustable inputs.**

The existing `tabela_aliquotas` (global, by year) is the right place for the IBS/CBS transition schedule. Do not duplicate it.

However, the new modules need three configurable inputs that vary by company and analysis scenario:

- `fator_reducao_simples` — the effective IBS/CBS credit rate for Simples Nacional suppliers (legal, varies by product category)
- `taxa_cdi_anual` — CDI rate used in split-payment float calculation (market rate, company may want to override)
- `target_ano` — which year from `tabela_aliquotas` to use for projections (default 2033)

These go in a new per-company config table:

```sql
CREATE TABLE IF NOT EXISTS reforma_parametros (
    company_id          UUID PRIMARY KEY REFERENCES companies(id) ON DELETE CASCADE,
    target_ano          INTEGER NOT NULL DEFAULT 2033,
    fator_simples_pct   NUMERIC(5,2) NOT NULL DEFAULT 20.00,  -- % de crédito Simples
    taxa_cdi_anual_pct  NUMERIC(5,2) NOT NULL DEFAULT 10.50,  -- CDI % ao ano
    updated_at          TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
```

Handlers query this table with a fallback to defaults if no row exists (same pattern as `tabela_aliquotas` fallback in `creditos_perdidos.go:119`).

### 3. Query Strategy: Direct SQL Aggregations vs Materialized Views

**Decision: Direct SQL aggregations for all 8 modules. Do not add new materialized views.**

Rationale from codebase evidence:
- Existing MVs (`mv_mercadorias_agregada`, `mv_operacoes_simples`, `mv_compras_fornecedores`) are refreshed by the SPED worker after import queue drains. They are SPED-only data. The new modules aggregate XML item data (`nfe_saidas_itens`, `nfe_entradas_itens`) which follows a different import path (XML upload handler, not SPED worker).
- Adding new MVs would require hooking refresh triggers into the XML upload handler path — currently `xml_upload.go` has no MV refresh call, only the worker does.
- The `(company_id, ncm)` index already exists on `nfe_saidas_itens` and `nfe_entradas_itens`. The `(company_id, mes_ano)` indexes exist on `nfe_entradas` and `nfe_saidas`. These are sufficient for GROUP BY aggregations over 12-month windows for a single company.
- Ferreira Costa is currently a single-company tenant. Query performance at their scale (months of data, one company) does not require materialized views for aggregated analytics.

**Exception:** If profiling shows queries over `nfe_saidas_itens GROUP BY ncm` taking more than 3 seconds for 12 months, add a view then. Premature optimization here would bloat the worker and create refresh-timing bugs.

### 4. Configurable Parameters Architecture

**Decision: `reforma_parametros` table (described above) + GET/PUT config endpoint.**

New handler file: `backend/handlers/reforma_config.go`

```
GET  /api/reforma/parametros          → returns current config (or defaults)
PUT  /api/reforma/parametros          → saves per-company overrides
```

Registered as:
```go
http.HandleFunc("/api/reforma/parametros", withAuth(handlers.ReformaParametrosHandler, ""))
```

This handler internally switches on `r.Method` (following the pattern documented in ARCHITECTURE.md anti-patterns — do not put method dispatch in `main.go`).

### 5. Frontend Structure

**Decision: New top-level module "reforma" in `navigation.ts` with tabs per module group, not per individual module.**

The current `modules` object in `navigation.ts` has `simulador`, `notas`, `painel`, `config`. Add a new `reforma` module:

```typescript
reforma: {
  label: 'Análise Reforma Tributária',
  tabs: [
    { label: 'Créditos Bloqueados (EFD)',   path: '/reforma/creditos-bloqueados' },
    { label: 'Reprecificação de Produtos',   path: '/reforma/reprecificacao' },
    { label: 'Ranking Fornecedores IBS/CBS', path: '/reforma/fornecedores' },
    { label: 'Split Payment — Float',        path: '/reforma/split-payment' },
    { label: 'Análise por NCM',              path: '/reforma/por-ncm' },
    { label: 'Análise por CFOP',             path: '/reforma/por-cfop' },
    { label: 'Análise por UF/Destino',       path: '/reforma/por-uf' },
    { label: 'Segmentação B2B vs B2C',       path: '/reforma/b2b-b2c' },
  ],
},
```

Add `getActiveModule` mapping for `/reforma/` → `'reforma'`.

New page files in `frontend/src/pages/`:
- `ReformaCredBloqueados.tsx`
- `ReformaReprecificacao.tsx`
- `ReformaFornecedores.tsx`
- `ReformaSplitPayment.tsx`
- `ReformaPorNCM.tsx`
- `ReformaPorCFOP.tsx`
- `ReformaPorUF.tsx`
- `ReformaB2BC.tsx`

One page per module is the established pattern (38 existing pages, not grouped into mega-pages). A shared `useReformaParametros` hook fetches the company config and is imported by all 8 pages.

### 6. Period Window Handling

**Decision: Query parameter `?meses=12` in handler, SQL filter built in the handler layer.**

No application-layer date manipulation. Pass the window as a query param, default 12. The handler computes the cutoff date in Go and passes it as a `$N` parameter:

```go
meses := 12
if m := r.URL.Query().Get("meses"); m != "" {
    if parsed, err := strconv.Atoi(m); err == nil && parsed > 0 && parsed <= 60 {
        meses = parsed
    }
}
cutoff := time.Now().AddDate(0, -meses, 0)
// pass cutoff as $3 to SQL:
// AND data_emissao >= $3
```

This is consistent with how `ai_reports.go` handles `periodo` — a string parameter the handler validates before passing to SQL as a bound argument.

---

## Component Boundaries

### New Files to Create

| File | Type | Purpose |
|---|---|---|
| `backend/handlers/reforma_modulo1.go` | Handler | Módulos 1.1–1.4 (4 handlers) |
| `backend/handlers/reforma_modulo2.go` | Handler | Módulos 2.1–2.4 (4 handlers) |
| `backend/handlers/reforma_config.go` | Handler | GET/PUT reforma_parametros |
| `backend/migrations/086_create_reforma_parametros.sql` | Migration | `reforma_parametros` table |
| `backend/migrations/087_add_ind_final_to_nfe_saidas.sql` | Migration | `ADD COLUMN ind_final SMALLINT` to `nfe_saidas` (needed by Módulo 2.4) |
| `frontend/src/pages/ReformaCredBloqueados.tsx` | Page | Módulo 1.1 UI |
| `frontend/src/pages/ReformaReprecificacao.tsx` | Page | Módulo 1.2 UI |
| `frontend/src/pages/ReformaFornecedores.tsx` | Page | Módulo 1.3 UI |
| `frontend/src/pages/ReformaSplitPayment.tsx` | Page | Módulo 1.4 UI |
| `frontend/src/pages/ReformaPorNCM.tsx` | Page | Módulo 2.1 UI |
| `frontend/src/pages/ReformaPorCFOP.tsx` | Page | Módulo 2.2 UI |
| `frontend/src/pages/ReformaPorUF.tsx` | Page | Módulo 2.3 UI |
| `frontend/src/pages/ReformaB2BC.tsx` | Page | Módulo 2.4 UI |
| `frontend/src/hooks/useReformaParametros.ts` | Hook | Shared config fetch |

### Existing Files to Modify

| File | Change |
|---|---|
| `backend/main.go` | Register 9 new routes (8 module endpoints + 1 config endpoint) |
| `frontend/src/lib/navigation.ts` | Add `reforma` module entry and `getActiveModule` mapping |
| `frontend/src/App.tsx` | Add 8 new `<Route>` entries under `/reforma/*` |
| `backend/handlers/nfe_saidas.go` | Add `IndFinal string` to `ide` struct (for Módulo 2.4 B2B/B2C) |
| `backend/handlers/xml_upload.go` | Persist `ind_final` when parsing saída XMLs (for Módulo 2.4) |

---

## Data Flow

### Módulo 1.1 — Créditos ICMS Bloqueados (EFD C190)

```
reg_c190
  JOIN reg_c100 ON id_pai_c100
  JOIN import_jobs ON job_id
  JOIN cfop ON c190.cfop (tipo IN ('C','A') = uso/consumo, ativo)
WHERE import_jobs.company_id = $1
  AND import_jobs.mes_ano corresponds to last N months
GROUP BY cfop.tipo, c190.cfop
→ handler: CreditosBloqueadosEFDHandler (reforma_modulo1.go)
```

Note: C190 has no CST. Credit blocking in the EFD is by CFOP type (C=consumo, A=ativo). The `cfop` table already classifies this via the `tipo` column.

### Módulo 1.2 — Reprecificação (nfe_saidas_itens)

```
nfe_saidas_itens si
  JOIN nfe_saidas s ON s.id = si.nfe_id
  LEFT JOIN ncm_cclasstrib_reforma ref ON ref.ncm_digits = LEFT(si.ncm, length(ref.ncm_digits))
  JOIN tabela_aliquotas ta ON ta.ano = $target_ano
WHERE si.company_id = $1
  AND s.data_emissao >= $cutoff
GROUP BY si.ncm, ref.ibs_reducao_pct, ref.cbs_reducao_pct
→ calculates: ICMS-inside removal, IBS/CBS-outside addition, net price delta
→ handler: ReprecificacaoHandler (reforma_modulo1.go)
```

### Módulo 1.3 — Ranking Fornecedores (nfe_entradas + forn_simples)

```
nfe_entradas ne
  LEFT JOIN forn_simples fs ON fs.cnpj = ne.forn_cnpj
  JOIN tabela_aliquotas ta ON ta.ano = $target_ano
  JOIN reforma_parametros rp ON rp.company_id = ne.company_id  (or defaults)
WHERE ne.company_id = $1
  AND ne.data_emissao >= $cutoff
  AND ne.cancelado = 'N'
GROUP BY ne.forn_cnpj, ne.forn_nome, (fs.cnpj IS NOT NULL)
→ projected IBS/CBS credit: full rate if not Simples, rp.fator_simples_pct if Simples
→ handler: FornecedoresIBSHandler (reforma_modulo1.go)
```

### Módulo 1.4 — Split Payment Float (nfe_saidas)

```
nfe_saidas
WHERE company_id = $1
  AND data_emissao >= $cutoff
  AND cancelado = 'N'
GROUP BY mes_ano
→ float_total = SUM(v_icms) + SUM(v_pis) + SUM(v_cofins)  [current]
→ float_reforma = SUM(v_prod) * (ibs_rate + cbs_rate)       [projected, from tabela_aliquotas]
→ custo_financeiro = float_reforma * (taxa_cdi / 12) * days_avg_payment
→ handler: SplitPaymentHandler (reforma_modulo1.go)
```

### Módulo 2.1 — Por NCM (nfe_saidas_itens + ncm_cclasstrib_reforma)

```
nfe_saidas_itens si
  JOIN nfe_saidas s ON s.id = si.nfe_id
  LEFT JOIN ncm_cclasstrib_reforma ref ON prefix match
  JOIN tabela_aliquotas ta ON ta.ano = $target_ano
WHERE si.company_id = $1
  AND s.data_emissao >= $cutoff
GROUP BY si.ncm, ref.descricao, ref.ibs_reducao_pct
→ columns: ncm, descricao, valor_total, icms_efetivo_pct, ibs_cbs_projetado_pct, delta_carga
→ handler: AnalisePorNCMHandler (reforma_modulo2.go)
```

### Módulo 2.2 — Por CFOP (reg_c190 + cfop)

```
reg_c190
  JOIN reg_c100 ON id_pai_c100
  JOIN import_jobs ON job_id
  JOIN cfop ON c190.cfop
  JOIN tabela_aliquotas ta ON ta.ano = $target_ano
WHERE import_jobs.company_id = $1
GROUP BY c190.cfop, cfop.descricao_cfop, cfop.tipo
→ handler: AnalisePorCFOPHandler (reforma_modulo2.go)
```

### Módulo 2.3 — Por UF/Destino (nfe_saidas)

```
nfe_saidas
  JOIN tabela_aliquotas ta ON ta.ano = $target_ano
WHERE company_id = $1
  AND data_emissao >= $cutoff
  AND cancelado = 'N'
GROUP BY dest_uf, emit_uf
→ columns: uf_origem, uf_destino, valor_total, icms_atual, ibs_projetado
   (ICMS origem → IBS destino is the core reform shift this module shows)
→ handler: AnalisePorUFHandler (reforma_modulo2.go)
```

### Módulo 2.4 — B2B vs B2C (nfe_saidas + ind_final)

```
nfe_saidas
WHERE company_id = $1
  AND data_emissao >= $cutoff
  AND cancelado = 'N'
GROUP BY
  CASE
    WHEN ind_final = 1 THEN 'B2C'
    WHEN LENGTH(dest_cnpj_cpf) = 11 THEN 'B2C'  -- CPF = consumidor final
    WHEN LENGTH(dest_cnpj_cpf) = 14 THEN 'B2B'  -- CNPJ = empresa
    ELSE 'indefinido'
  END
→ handler: AnaliseB2BCHandler (reforma_modulo2.go)
```

Note: `ind_final` is not currently stored. The fallback (CPF vs CNPJ length) works for most cases. Adding `ind_final` as a column and extending the XML parse struct is migration 087 — should be built in the same phase as Módulo 2.4.

---

## Build Order (Module Dependencies)

```
Phase A — Prerequisites (no module works without these)
  1. Migration 086: reform_parametros table
  2. Handler reforma_config.go + GET/PUT /api/reforma/parametros
  3. Frontend hook useReformaParametros.ts
  4. Navigation entry for "reforma" module

Phase B — Módulos 1.x (independent, any order within phase)
  B1. Módulo 1.1 — CreditosBloqueadosEFDHandler
      Depends on: reg_c190, cfop.tipo (already seeded), tabela_aliquotas
      Risk: LOW — data exists, query is a GROUP BY with JOIN on already-indexed tables
  
  B2. Módulo 1.3 — FornecedoresIBSHandler
      Depends on: nfe_entradas, forn_simples, reforma_parametros
      Risk: LOW — all tables exist; fator_simples is the only new config param needed

  B3. Módulo 1.2 — ReprecificacaoHandler
      Depends on: nfe_saidas_itens, ncm_cclasstrib_reforma (migration 079), tabela_aliquotas
      Risk: MEDIUM — NCM prefix matching (LEFT(ncm, N)) requires care; ncm_cclasstrib_reforma
            has variable-length ncm_digits, so matching must be done with a subquery or lateral join

  B4. Módulo 1.4 — SplitPaymentHandler
      Depends on: nfe_saidas, reforma_parametros (taxa_cdi), tabela_aliquotas
      Risk: MEDIUM — CDI-based float calculation is pure arithmetic, but defining "average
            payment days" requires either a config parameter or a fixed assumption

Phase C — Módulos 2.x (independent, any order within phase)
  C1. Módulo 2.2 — AnalisePorCFOPHandler
      Depends on: reg_c190, cfop (already seeded), tabela_aliquotas
      Risk: LOW — same query pattern as Módulo 1.1 but different grouping

  C2. Módulo 2.1 — AnalisePorNCMHandler
      Depends on: nfe_saidas_itens, ncm_cclasstrib_reforma, tabela_aliquotas
      Risk: MEDIUM — same NCM prefix-match complexity as Módulo 1.2

  C3. Módulo 2.3 — AnalisePorUFHandler
      Depends on: nfe_saidas (emit_uf, dest_uf already present), tabela_aliquotas
      Risk: LOW — straightforward GROUP BY UF

  C4. Módulo 2.4 — AnaliseB2BCHandler
      Depends on: nfe_saidas (dest_cnpj_cpf for CPF/CNPJ length heuristic)
                  migration 087 + xml_upload.go change (for ind_final accuracy)
      Risk: MEDIUM — building without ind_final works as MVP; adding ind_final later
            only improves accuracy for cases currently unclassifiable by CPF/CNPJ heuristic
```

---

## Schema Changes Needed

### New Tables

**Migration 086** — `reforma_parametros`
```sql
CREATE TABLE IF NOT EXISTS reforma_parametros (
    company_id          UUID PRIMARY KEY REFERENCES companies(id) ON DELETE CASCADE,
    target_ano          INTEGER NOT NULL DEFAULT 2033,
    fator_simples_pct   NUMERIC(5,2) NOT NULL DEFAULT 20.00,
    taxa_cdi_anual_pct  NUMERIC(5,2) NOT NULL DEFAULT 10.50,
    prazo_medio_dias    INTEGER NOT NULL DEFAULT 30,
    updated_at          TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
```

### Column Additions

**Migration 087** — `ind_final` on `nfe_saidas`
```sql
ALTER TABLE nfe_saidas
    ADD COLUMN IF NOT EXISTS ind_final SMALLINT;
-- 0 = normal, 1 = consumidor final (NFC-e, B2C); NULL = not set / not parsed

CREATE INDEX IF NOT EXISTS idx_nfe_saidas_ind_final
    ON nfe_saidas(company_id, ind_final);
```

**No changes to `reg_c190`** — CST-level blocking is not achievable from C190 (aggregated by CFOP). Módulo 1.1 uses CFOP-type classification from the `cfop` table, which is already seeded with `tipo IN ('C','A','R','T','O')`.

---

## API Route Map

All new routes follow the pattern `withAuth(handlers.XHandler, "")` in `backend/main.go`:

```
GET  /api/reforma/parametros                → ReformaParametrosHandler (GET)
PUT  /api/reforma/parametros                → ReformaParametrosHandler (PUT)

GET  /api/reforma/modulo1/creditos-bloqueados   → CreditosBloqueadosEFDHandler
GET  /api/reforma/modulo1/reprecificacao        → ReprecificacaoHandler
GET  /api/reforma/modulo1/fornecedores          → FornecedoresIBSHandler
GET  /api/reforma/modulo1/split-payment         → SplitPaymentHandler

GET  /api/reforma/modulo2/por-ncm               → AnalisePorNCMHandler
GET  /api/reforma/modulo2/por-cfop              → AnalisePorCFOPHandler
GET  /api/reforma/modulo2/por-uf                → AnalisePorUFHandler
GET  /api/reforma/modulo2/b2b-b2c               → AnaliseB2BCHandler
```

Query params consistent across all module endpoints:
- `?meses=12` — lookback window (default 12, max 60)
- `?ano=2033` — IBS/CBS rate year from `tabela_aliquotas` (default 2033, overridden by `reforma_parametros.target_ano`)

---

## Tenancy Compliance

Every query follows the established pattern:

```go
companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
```

Then passes `companyID` as `$1` in all SQL queries. Tables accessed:

| Table | company_id path |
|---|---|
| `nfe_saidas` | direct `company_id` column |
| `nfe_entradas` | direct `company_id` column |
| `nfe_saidas_itens` | direct `company_id` column (denormalized) |
| `nfe_entradas_itens` | direct `company_id` column (denormalized) |
| `reg_c190` | via `import_jobs.company_id` (JOIN) |
| `reg_c100` | via `import_jobs.company_id` (JOIN) |
| `reforma_parametros` | direct `company_id` column (PK) |
| `tabela_aliquotas` | global table, no company_id (by design) |
| `ncm_cclasstrib_reforma` | global reference table (by design) |
| `cfop` | global reference table (by design) |
| `forn_simples` | global CNPJ lookup (by design — cross-company reference data) |

---

## Anti-Patterns to Avoid

### Do Not Set CORS Headers in New Handlers
Per the codebase anti-patterns doc: new handlers must NOT call `w.Header().Set("Access-Control-Allow-Origin", ...)`. `SecurityMiddleware` handles this.

### Do Not Interpolate Table Names or Company IDs Into SQL Strings
All handlers use `$N` parameters for user-supplied values. The only exception is table name whitelisting via a Go switch (as in `xml_comparativo.go:88`), which is acceptable.

### Do Not Add Method Dispatch to main.go
New handlers must internally switch on `r.Method`. Only the route prefix goes in `main.go`.

### Do Not Refresh Materialized Views From XML Upload Handler
The MV refresh belongs in the worker. Adding MV refresh to the XML upload path would create double-refresh races when both SPED import and XML upload happen concurrently.

---

## Confidence Assessment

| Area | Confidence | Evidence |
|---|---|---|
| Handler file structure | HIGH | Matches established codebase pattern verified in source |
| Rate table strategy | HIGH | `tabela_aliquotas` and `forn_simples` inspected directly |
| Query strategy (no MVs) | HIGH | Worker refresh logic inspected; no XML upload refresh path exists |
| Period window via query param | HIGH | Matches pattern in `ai_reports.go` |
| `reforma_parametros` schema | MEDIUM | Design matches codebase conventions; exact defaults (CDI, fator_simples) need fiscal team input |
| B2B/B2C via CPF/CNPJ heuristic | MEDIUM | `ind_final` not in source; heuristic is a known NF-e pattern but has edge cases |
| NCM prefix matching strategy | MEDIUM | `ncm_cclasstrib_reforma` inspected; matching approach needs query-level validation |
| C170 absence | HIGH | Worker source inspected — C170 is not parsed, C190 is the only item-level EFD data |

---

## Gaps to Address in Phase Research

1. **NCM prefix matching SQL** — `ncm_cclasstrib_reforma.ncm_digits` has variable lengths (4 to 10 chars). The join `LEFT(si.ncm, length(ref.ncm_digits)) = ref.ncm_digits` with a LATERAL or subquery needs testing; a simpler fixed-8-digit strategy may be sufficient.

2. **fator_simples default** — The 20% default is a placeholder. The actual credit factor for Simples Nacional suppliers under EC 132/2023 depends on annexe classification. Phase research for Módulo 1.3 should confirm the legal value.

3. **Split payment "prazo_medio_dias"** — Módulo 1.4 needs an assumption about average days between NF-e emission and tax payment by the buyer under split payment. Default 30 days is an approximation; the fiscal team should validate.

4. **ind_final population for historical data** — Migration 087 adds the column but historical `nfe_saidas` rows will have `NULL`. The CPF/CNPJ heuristic covers the NULL case as a fallback; this is documented behavior.

---

*Sources: Direct inspection of backend/handlers/, backend/migrations/, backend/worker/worker.go, frontend/src/lib/navigation.ts, frontend/src/App.tsx — all 2026-05-22.*
