# Project Research Summary

**Project:** FB_APU04 — Simulador Fiscal v5.00 — Análise da Reforma Tributária
**Domain:** Brazilian fiscal reform analytics (LC 214/2025 — IBS/CBS transition)
**Researched:** 2026-05-22
**Confidence:** HIGH (architecture and pitfalls derived from direct codebase inspection)

---

## Executive Summary

This milestone adds 8 new analytical modules to the existing FB_APU04 platform. The stack (Go 1.22 / PostgreSQL 15 / React 18 + Vite + TypeScript) is locked. No new backend libraries are required. The only new frontend dependency is `react-simple-maps` for the UF heat map. All analytical queries run directly against existing tables (`nfe_saidas_itens`, `nfe_entradas_itens`, `reg_c190`, `reg_c100`) using GROUP BY aggregations — no new materialized views are needed, and no additional database engines (ClickHouse, Redis) are warranted at current data volumes.

The recommended build order is: Phase A (schema blockers + config infrastructure) → Phase B (Módulos 1.x — direct tax exposure analysis) → Phase C (Módulos 2.x — dimensional analytics). This order is non-negotiable: Phase A migrations and the `reforma_parametros` config table are prerequisites for every analytics handler that needs configurable rates. Within Phases B and C, modules are largely independent and can be built in any sequence within the phase.

The most serious risks are data correctness risks, not performance risks. Four critical issues found in the existing schema must be resolved before any module is built: (1) `reg_c190` is missing the `cst_icms` and `aliq_icms` columns needed by Módulo 1.1; (2) cancelled documents (`cod_sit IN ('02','03','04','05')`) are not filtered in EFD analytics queries, silently inflating all totals; (3) `nfe_saidas` has no `ind_final` column, forcing B2B/B2C detection to rely on CPF/CNPJ length heuristic; (4) the Simples Nacional credit factor for Módulo 1.3 has no finalized regulatory value and MUST be user-configurable with a mandatory disclaimer, never hardcoded.

---

## Key Findings

### Recommended Stack

The existing stack handles all v5.00 requirements without additions. PostgreSQL 15 direct aggregations with `(company_id, ncm)` and `(company_id, mes_ano)` indexes already present on the item tables are sufficient for GROUP BY analytics over 12-month windows for a single-company tenant. The one new frontend library — `react-simple-maps` v3.0.0 — is needed for the UF choropleth map in Módulo 2.3. D3-scale is already in `node_modules` as a transitive dependency of Recharts. Radix UI Slider (already installed at `@radix-ui/react-slider ^1.3.6`) covers all configurable parameter inputs. Client-side simulation arithmetic in Módulos 1.2 and 1.4 is handled via `useMemo` with no server round-trip.

**Core technologies:**
- PostgreSQL 15 direct SQL aggregations — GROUP BY + CTEs; existing indexes sufficient; no new MVs
- `react-simple-maps` v3.0.0 — Brazil state choropleth map for Módulo 2.3; `npm install react-simple-maps`
- Brazil states TopoJSON — commit to `frontend/public/brazil-states.json` (~175 KB simplified IBGE file)
- `tabela_aliquotas` (existing) + new `reforma_parametros` table — rate and parameter storage per company
- `useMemo` + inline `useDebounce` hook (10 lines, no library) — client-side simulation
- Standard Go `time` + `math` packages — CDI-based float calculations in Módulo 1.4

**Explicitly rejected:** ClickHouse, TimescaleDB, Redis caching, Apache ECharts, react-leaflet, `shopspring/decimal`, `use-debounce` npm package.

### Expected Features

**Must have (table stakes — v5.00 MVP):**
- Módulo 1.1 — Blocked ICMS credits from EFD C190 by CFOP type (uso/consumo vs. ativo permanente); projected IBS/CBS credit equivalent
- Módulo 1.2 — Product repricing: current ICMS-embedded price vs. IBS/CBS por fora; configurable combined rate; ST and base-reduction CST handled separately
- Módulo 1.3 — Supplier ranking by projected IBS/CBS credit generated; regime normal vs. Simples Nacional; configurable credit factor with mandatory regulatory-pending disclaimer
- Módulo 1.4 — Working capital impact of split payment; configurable DSO and CDI per company; annual financing cost of lost float
- Módulo 2.1 — Revenue and purchase volume by NCM; current effective ICMS rate; projected IBS+CBS delta; IS (Imposto Seletivo) applicability flag
- Módulo 2.2 — Operations by CFOP functional category; transfer CFOPs excluded; IBS/CBS impact by group
- Módulo 2.3 — Sales volume by UF of destination; ICMS origin vs. IBS destination redistribution (tabular view; map deferred)
- Módulo 2.4 — B2B/B2C segmentation: b2b_credit / b2b_nocredit / b2c three-way; revenue split by segment

**Should have (differentiators — include if time allows):**
- CST/CFOP coherence validation flag in Módulo 1.1
- Three-way B2B segmentation using `ind_final` (Phase A migration)
- Hybrid regime simulation for Simples suppliers in Módulo 1.3
- Sensitivity table in Módulo 1.4 (DSO x CDI matrix)
- UF choropleth map in Módulo 2.3 using `react-simple-maps`
- AI-generated executive summary per module (existing Z.AI GLM integration available)
- Excel/CSV export on each module

**Defer to v2+:**
- UF map visualization (can ship tabular first; reduces Phase C risk)
- CIAP balance projection (requires separate CIAP control table not yet planned)
- Transition glide paths per year 2026–2032 (ship single end-state 2033 first)
- Cross-module drill-down navigation
- Per-municipality IBS analysis (rates not published; dimension premature)
- CNPJ raiz grouping for holding company supplier analysis

### Architecture Approach

All 8 new modules follow the established handler pattern without exception. New Go handlers live in two files (`reforma_modulo1.go`, `reforma_modulo2.go`) plus a config handler (`reforma_config.go`). A single new table (`reforma_parametros`) stores per-company configurable parameters. No materialized views are added — the XML upload handler has no MV refresh path, and adding one would create refresh-timing races with the SPED worker. The frontend adds one new navigation module (`reforma`) with 8 sub-pages and a shared `useReformaParametros` hook.

**Major components:**
1. `reforma_parametros` table (migration 086) — company config for all rate parameters; GET/PUT via `reforma_config.go`
2. `reforma_modulo1.go` — 4 handlers for Módulos 1.1–1.4 (credit blocking, repricing, supplier ranking, split payment float)
3. `reforma_modulo2.go` — 4 handlers for Módulos 2.1–2.4 (NCM, CFOP, UF, B2B/B2C dimensional analytics)
4. 8 React page files (`Reforma*.tsx`) + `useReformaParametros.ts` shared hook
5. Schema additions: `cst_icms`/`aliq_icms` on `reg_c190`; `ind_final` on `nfe_saidas`; transfer CFOP seed

**Confirmed data sources:**
- `reg_c100` / `reg_c190` — EFD ICMS/IPI (C170 is NOT parsed; worker never implemented it — use C190 only)
- `nfe_entradas` / `nfe_entradas_itens` — purchase XML with NCM, CFOP, CST, value fields
- `nfe_saidas` / `nfe_saidas_itens` — sales XML; `dest_cnpj_cpf` present; `ind_final` absent (needs migration)
- `tabela_aliquotas` — global IBS/CBS/ICMS transition rates by year (2027–2033)
- `ncm_cclasstrib_reforma` — 95 NCMs seeded with reduction percentages; variable-length prefix key `ncm_digits`
- `forn_simples` — global CNPJ lookup for Simples Nacional supplier identification

### Critical Pitfalls

1. **`reg_c190` missing `cst_icms` and `aliq_icms`** — These columns exist in the raw EFD file at `parts[2]` and `parts[4]` but were never stored. Migration must add them AND the worker must populate them. Phase A blocker — without this, Módulo 1.1 cannot query CST-based credit classification.

2. **Cancelled EFD documents not filtered** — `worker.go` inserts all `reg_c100` rows regardless of `cod_sit`. Every EFD analytics query must add `AND c100.cod_sit NOT IN ('02','03','04','05')`. Without this, all totals (credits, volumes, tax amounts) are inflated by cancelled/denied documents.

3. **Simples Nacional credit factor has no published regulatory value** — LC 214/2025 delegates the credit percentage to a future CG-IBS joint act not yet issued (as of May 2026). The `fator_simples_pct` in `reforma_parametros` must default to a labeled estimate with a mandatory UI disclaimer. Never hardcode.

4. **NCM prefix matching produces multiple matches** — `ncm_cclasstrib_reforma.ncm_digits` has variable lengths (4–10 chars). A naive JOIN returns multiple rows per item NCM. Use `DISTINCT ON (item_id)` with `ORDER BY length(ncm_digits) DESC` or a `LATERAL` subquery with `LIMIT 1` (longest-prefix-wins). Affects Módulos 1.2 and 2.1.

5. **Transfer CFOPs inflate IBS/CBS credit analysis** — LC 214/2025 Art. 6 exempts transfers between same-taxpayer establishments from IBS/CBS. All CFOP-based analytics must exclude transfer codes. The `cfop` table `tipo='T'` filter is not fully seeded — add transfer CFOPs (5151, 5152, 6151, 6152, 1151, 1152, 2151, 2152) in Phase A.

---

## Implications for Roadmap

### Phase A: Blockers and Infrastructure

**Rationale:** Four schema gaps and one missing config table will cause every analytics module to fail or produce wrong data. No user-visible output ships from Phase A — it is entirely prerequisite work that unblocks Phases B and C.

**Delivers:**
- `cst_icms VARCHAR(3)` and `aliq_icms NUMERIC(6,2)` added to `reg_c190` + worker updated to populate them (migration 086)
- `reforma_parametros` table (company_id PK, target_ano, fator_simples_pct, taxa_cdi_anual_pct, prazo_medio_dias) (migration 086 or 087)
- `ind_final SMALLINT` added to `nfe_saidas` + XML parse struct extended + `xml_upload.go` persists it (migration 087 or 088)
- Transfer CFOPs seeded into `cfop` table with `tipo='T'`
- `GET /api/reforma/parametros` and `PUT /api/reforma/parametros` endpoints (`reforma_config.go`)
- `useReformaParametros.ts` shared frontend hook
- Navigation entry for "Análise Reforma Tributária" module in `navigation.ts` and `App.tsx`

**Avoids:** CRITICAL-1 (missing CST column), CRITICAL-2 (cancelled docs), MODERATE-3 (transfer CFOP seed), MODERATE-4 (ind_final for B2B/B2C)

**Research flag:** Standard patterns — all schema changes verified against codebase. No additional research phase needed.

---

### Phase B: Módulos 1.x — Direct Tax Exposure

**Rationale:** The 1.x group answers the highest-value question first: "what is our direct tax exposure and cost under the reform?" These modules operate on different data sources but share `reforma_parametros` config. They have lower inter-dependency than the 2.x group and can be built in the order below.

**Recommended build order within phase:** 1.1 → 1.3 → 1.2 → 1.4

**Delivers:**
- Módulo 1.1 — Blocked ICMS credits dashboard (EFD C190 with `cst_icms` from Phase A; CFOP-type classification; cancelled-doc filter)
- Módulo 1.3 — Supplier credit ranking (regime normal vs. Simples Nacional; configurable `fator_simples_pct` from `reforma_parametros`)
- Módulo 1.2 — Product repricing table (ICMS por dentro → IBS/CBS por fora; NCM LATERAL prefix join; three CST calculation paths: normal/ST/base-reduction)
- Módulo 1.4 — Split payment working capital impact (configurable DSO and CDI; CDI-based float cost; year-by-year optional if time allows)

**Key implementation rules for all Phase B handlers:**
- EFD queries: `AND c100.cod_sit NOT IN ('02','03','04','05')`
- XML queries: `AND cancelado = 'N'`
- Transfer exclusion: `JOIN cfop cf ON c190.cfop = cf.cfop WHERE cf.tipo != 'T'`
- NCM prefix match: `LATERAL (SELECT * FROM ncm_cclasstrib_reforma WHERE starts_with(ncm, ncm_digits) ORDER BY length(ncm_digits) DESC LIMIT 1)`
- Simples factor: always read from `reforma_parametros.fator_simples_pct`; never hardcoded

**Avoids:** CRITICAL-2, CRITICAL-4, CRITICAL-5, CRITICAL-6, MODERATE-3, MODERATE-5

**Research flag:** Módulo 1.2 CST path implementation needs a test dataset with known CST distributions (CST 00, 10, 20, 60). Request sample EFD from fiscal team before coding. No additional research phase needed — PITFALLS.md contains the exact formulas.

---

### Phase C: Módulos 2.x — Dimensional Analytics

**Rationale:** The 2.x group provides cross-cutting dimensional views. They are primarily aggregation queries with less formula complexity than Phase B. Module 2.4 is built last because it requires `ind_final` (Phase A) and benefits from the Simples Nacional flag established by Phase B Módulo 1.3.

**Recommended build order within phase:** 2.2 → 2.1 → 2.3 → 2.4

**Delivers:**
- Módulo 2.2 — CFOP functional group analysis (same EFD C190 pattern as 1.1; validates Phase A CFOP seed)
- Módulo 2.1 — NCM analysis (sales items ranked by volume; LATERAL NCM join; IS flag; effective rate delta)
- Módulo 2.3 — UF/destination analysis (ICMS origin → IBS destination; tabular output first; choropleth map if time allows)
- Módulo 2.4 — B2B/B2C three-way segmentation using `ind_final` + CPF/CNPJ heuristic fallback

**Avoids:** MODERATE-3, MODERATE-4, MODERATE-5, MINOR-1, MINOR-2

**Research flag:** Módulo 2.3 UF map needs `react-simple-maps` integration. Pattern is documented in STACK.md. Verify `geo.properties.sigla` field name in the chosen TopoJSON file before wiring the colorScale. No additional research phase needed.

---

### Phase Ordering Rationale

- Phase A first because `reforma_parametros` is imported by every handler in B and C. Building without it means hardcoding defaults that must later be ripped out.
- The `cst_icms`/`aliq_icms` migration must precede any C190-based handler — otherwise queries fail at runtime with "column does not exist."
- The cancelled-document filter (`cod_sit NOT IN ('02','03','04','05')`) established as a convention in Phase A prevents each Phase B/C developer from independently remembering to add it.
- Módulos 1.x before 2.x because Phase B produces the Simples Nacional flags (1.3) and credit amounts (1.1) that feed dimensional analytics as filter inputs.
- Within each phase, EFD-based modules before XML-based modules so C190 correctness post-migration is validated before more complex item-level XML queries.

### Research Flags

**Needs implementation care (no additional research phase, but careful execution):**
- Phase B — Módulo 1.2: three CST formula paths must be explicitly tested with a real EFD sample before shipping
- Phase B — Módulo 1.3: monitor for CG-IBS joint act publication; update `fator_simples_pct` default when published (~Q3-Q4 2026)
- Phase C — Módulo 2.3: verify TopoJSON `geo.properties.sigla` field name matches two-letter UF codes before connecting the colorScale

**Standard patterns (no additional research needed):**
- Phase A: all schema changes and handler patterns verified
- Phase B Módulo 1.1: direct extension of `creditos_perdidos.go` pattern
- Phase B Módulo 1.4: CDI arithmetic is standard `math.Pow`; complexity is in parameter UX, not calculation
- Phase C Módulo 2.2: same EFD C190 GROUP BY pattern as Módulo 1.1 with different dimensions
- Phase C Módulo 2.4: three-way segmentation SQL fully specified in PITFALLS.md MODERATE-4

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Direct codebase inspection of `go.mod`, `package.json`, all migrations; no inference |
| Features | HIGH | LC 214/2025 official text consulted; EFD field structure from official SEFAZ documentation |
| Architecture | HIGH | All patterns from direct inspection of `backend/handlers/`, `backend/worker/worker.go`, `frontend/src/` |
| Pitfalls | HIGH (schema/correctness) / MEDIUM (regulatory) | Schema gaps confirmed by SQL inspection. Regulatory pitfalls (Simples credit factor, split payment timeline) are MEDIUM — depend on future regulatory acts |

**Overall confidence:** HIGH for implementation decisions. MEDIUM for rate and regulatory assumptions.

### Gaps to Address

- **Simples Nacional credit factor default:** The 20% placeholder in `reforma_parametros.fator_simples_pct` has no regulatory basis. Track the CG-IBS joint act; update the default when published. Build the admin config UI so the update requires zero code changes.

- **NCM prefix matching performance:** The `LATERAL` longest-prefix-wins join on `ncm_cclasstrib_reforma` is untested at production item volumes. Add query timing in Phase C Módulo 2.1 development. Verify migration 079 created `idx_ncm_cclasstrib_reforma_digits` — if missing, add it.

- **`ind_final` for historical data:** Phase A migration adds the column but existing `nfe_saidas` rows will have `NULL`. The CPF/CNPJ heuristic covers `NULL` as a documented fallback. Add a UI note: "Notas anteriores à atualização do importador usam classificação por CPF/CNPJ. Reimporte os XMLs para análise com campo indFinal."

- **PIS/COFINS actual amounts:** Módulo 1.4 (split payment) ideally uses PIS/COFINS from the EFD Contribuições file — which is not imported. Handler must use `nfe_saidas.v_pis` / `v_cofins` (XML-sourced approximation) or prompt the user to enter the combined rate manually. Document this limitation clearly in the UI.

- **CFOP seed completeness for `tipo='T'`:** After Phase A migration, verify with `SELECT cfop FROM cfop WHERE tipo = 'T' ORDER BY cfop` and compare against the standard CFOP transfer list. If any transfer CFOP is missing, the exclusion filter silently fails for that code.

---

## Sources

### Primary (HIGH confidence)
- LC 214/2025 (Planalto official text) — regulatory basis for all 8 modules; split payment Arts. 31-35; transfer non-incidence Art. 6
- EFD ICMS/IPI layout (SEFAZ Goiás FAQ v7.5) — C190 field positions, CST Tabela B meanings
- VRI Consulting — C100, C190, C170 register layout structure
- Direct codebase inspection: `backend/worker/worker.go`, `backend/handlers/`, `backend/migrations/085` and earlier, `frontend/src/` — all schema, handler, and navigation patterns confirmed
- Decreto 12.955/2026 — split payment regulatory instruments (Pix, Boleto, TED, TEF, cards)

### Secondary (MEDIUM confidence)
- SimTax.com.br — IBS/CBS destination principle, Simples Nacional hybrid regime, transfer operations
- Escola Superior do Simples Nacional — formação de preço por fora vs. por dentro; regime híbrido
- react-simple-maps v3.0.0 official docs + npm — React 18 compatibility confirmed
- carolinabigonha/br-atlas — Brazil states TopoJSON (IBGE-sourced)
- CDM Contabilidade — CST ICMS Tabela B complete classification

### Tertiary (MEDIUM-LOW — pending regulatory or single source)
- Simples Nacional credit factor for IBS/CBS — regulatory act not published as of 2026-05-22; estimates are illustrative only
- State-level IBS alíquota UF/municipality split — not published; `tabela_aliquotas_uf` design is proactive but will have no real data until CG-IBS publishes rates
- CFOP seed completeness — inferred from migration 026/062 analysis; requires verification query post-Phase A

---
*Research completed: 2026-05-22*
*Ready for roadmap: yes*
