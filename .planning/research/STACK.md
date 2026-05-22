# Technology Stack — Fiscal Reform Analytics Additions

**Project:** FB_APU04 v5.00 — Análise da Reforma Tributária  
**Researched:** 2026-05-22  
**Scope:** Stack additions only — existing Go 1.22 / PostgreSQL 15 / React 18+Vite+TypeScript stack is locked.

---

## 1. PostgreSQL Analytical Query Patterns

### Decision: Materialized Views (existing pattern) + CTEs for ad-hoc, no new infrastructure

**Rationale:** The codebase already has three working materialized views (`mv_mercadorias_agregada`, `mv_operacoes_simples`, `mv_compras_fornecedores`) with the established pattern of `REFRESH MATERIALIZED VIEW CONCURRENTLY` (falling back to non-concurrent if no unique index). All Go refresh code uses raw `database/sql` `Exec()` calls — no new libraries needed.

For the new analytical modules the pattern is:

**Use materialized views when:**
- The query aggregates across the full tenant dataset (NCM ranking, CFOP impact, UF heat map totals)
- Results are expensive to compute (>100 ms) and data changes only on import events
- A unique index on `(company_id, <grouping_key>)` can be defined — required for `CONCURRENTLY`

**Use inline CTEs when:**
- The query is parameterized (e.g., filter by mes_ano range, single NCM, single CFOP group)
- The query is only called once per user action, not in a polling loop
- PostgreSQL 12+ CTE inlining (non-materialized by default) is sufficient

**Pattern: B-tree index for prefix NCM matching**

`nfe_entradas_itens.ncm` and `ncm_cclasstrib_reforma.ncm_digits` already use this. For new queries joining items to the reforma table:

```sql
-- Already indexed: idx_ncm_cclasstrib_reforma_digits ON ncm_cclasstrib_reforma(ncm_digits)
-- For prefix match: use LEFT(ncm, 4) or ncm_digits LIKE '0207%' (supported by B-tree)
-- For ranking: GROUP BY ei.ncm, r.descricao with SUM/COUNT aggregates
```

**Pattern: Window functions for ranking**

```sql
WITH ranked AS (
  SELECT
    ei.ncm,
    SUM(ei.v_prod) AS v_total,
    SUM(ei.v_pis + ei.v_cofins) AS tributos_atuais,
    RANK() OVER (ORDER BY SUM(ei.v_prod) DESC) AS ranking
  FROM nfe_entradas_itens ei
  WHERE ei.company_id = $1
  GROUP BY ei.ncm
)
SELECT * FROM ranked WHERE ranking <= 50
```

**Pattern: UF aggregation for heat map**

`nfe_saidas` has `dest_uf` and `emit_uf` columns. `nfe_entradas` has `forn_uf`. These are VARCHAR(2) — no new columns needed for UF-level aggregation.

**Materialized view refresh trigger:** Keep the existing Go pattern — `REFRESH MATERIALIZED VIEW CONCURRENTLY` called at end of XML upload handler and via the admin refresh endpoint. New MVs for the analytics modules follow the same pattern.

**Confidence:** HIGH — based on direct codebase inspection of existing MV patterns and PostgreSQL 15 documentation.

---

## 2. Brazil State Map Visualization

### Decision: `react-simple-maps` v3.0.0 + IBGE TopoJSON file (committed to repo)

**Library:** `react-simple-maps` v3.0.0  
**Install:** `npm install react-simple-maps`  
**TypeScript types:** Built-in (included in package)  
**React 18 compatibility:** YES — requires React >=16.8 (hooks), confirmed compatible with React 18.

**Why react-simple-maps, not alternatives:**

| Option | Verdict | Reason |
|--------|---------|--------|
| `react-simple-maps` v3 | **USE** | SVG-based, d3-geo + topojson, declarative API, works with any TopoJSON file, fits Tailwind/Recharts visual style |
| `@react-map/brazil` v1.0.9 | Avoid | Last published 2 years ago, unmaintained |
| `@svg-maps/brazil` v2.0.0 | Skip | More recently maintained but lower abstraction level; requires manual SVG wiring without d3-geo projection support |
| Leaflet/react-leaflet | Overkill | Tile-based maps for geographic accuracy; choropleth coloring is harder; adds ~200 KB; not needed for a state-level heat map |
| D3 directly | Avoid | No need to reinvent; react-simple-maps is the standard React wrapper |

**Note on react-simple-maps maintenance:** The original package (`zcreativelabs/react-simple-maps`) is at v3.0.0 and was last published ~4 years ago. However, v3 is stable, has no known React 18 breaking issues, and there are 1M+ weekly downloads (community evidence of continued use). A React 19–compatible fork (`@vnedyalk0v/react19-simple-maps` v2.0.7) exists but is unnecessary — this project uses React 18.3.1.

**GeoJSON / TopoJSON data source:**

Use the pre-built Brazil states TopoJSON from `carolinabigonha/br-atlas` or the IBGE-sourced file from `mtrovo/br-atlas`. Commit the TopoJSON file directly to `frontend/public/brazil-states.json` (approximately 150–200 KB simplified). This avoids a CDN runtime dependency.

The relevant IBGE source: https://github.com/carolinabigonha/br-atlas

**Integration pattern:**

```tsx
import { ComposableMap, Geographies, Geography } from "react-simple-maps"
import { scaleQuantile } from "d3-scale"
// d3-scale is already a transitive dependency of recharts

const colorScale = scaleQuantile<string>()
  .domain(data.map(d => d.impacto_ibs))
  .range(["#f0f9e8","#bae4bc","#7bccc4","#43a2ca","#0868ac"])

<ComposableMap
  projection="geoMercator"
  projectionConfig={{ center: [-52, -14], scale: 800 }}
>
  <Geographies geography="/brazil-states.json">
    {({ geographies }) =>
      geographies.map(geo => {
        const uf = geo.properties.sigla  // e.g. "SP", "RJ"
        const val = ufData[uf]
        return (
          <Geography
            key={geo.rsmKey}
            geography={geo}
            fill={val ? colorScale(val) : "#EEE"}
          />
        )
      })
    }
  </Geographies>
</ComposableMap>
```

**d3-scale dependency:** `d3-scale` is a transitive dependency of recharts (already in `node_modules`). It can be imported directly without adding a new `package.json` entry, though explicitly listing it improves clarity.

**Confidence:** MEDIUM — react-simple-maps v3 is verified against official docs; React 18 compatibility confirmed by no hooks API changes; TopoJSON data source is IBGE-official.

---

## 3. Configurable Tax Rate Storage

### Decision: PostgreSQL tables (extend existing pattern), no JSON files

**Existing pattern (already in production):**

- `tabela_aliquotas` — year-level IBS/CBS/ICMS reduction rates (migration 007, handler `config.go`)
- `ncm_cclasstrib_reforma` — NCM-level IBS/CBS reduction percentages per Anexo (migration 079)

**What to add for v5.00 modules:**

The existing tables are not sufficient for modules 2.1–2.3 which need UF-level and municipality-level rates. Two additional tables are needed:

**Table A: `tabela_aliquotas_ncm`** — Per-NCM override rates (extends ncm_cclasstrib_reforma)

This table already exists implicitly as `ncm_cclasstrib_reforma` with `ibs_reducao_pct`/`cbs_reducao_pct`. The new analytics modules need to JOIN items to this table to compute effective post-reforma IBS/CBS. No new table required — extend the query to use the existing table.

**Table B: `tabela_aliquotas_uf`** — Per-UF IBS split (UF vs municipality)

```sql
CREATE TABLE IF NOT EXISTS tabela_aliquotas_uf (
    ano         INT NOT NULL,
    uf          VARCHAR(2) NOT NULL,
    ibs_uf_pct  NUMERIC(6,4) NOT NULL,   -- alíquota IBS componente estado
    ibs_mun_pct NUMERIC(6,4) NOT NULL,   -- alíquota IBS componente município
    PRIMARY KEY (ano, uf)
);
```

**Why DB table over JSON file:**

- Queryable: `JOIN tabela_aliquotas_uf ON ano = $year AND uf = ns.dest_uf` in the analytics SQL
- Auditable: changes via migration, visible in `pg_dump` backups
- Consistent: same `company_id`-scoped pattern used throughout the app (though aliquotas are global, not tenant-specific)
- Editable by admin UI: the app already has a `TabelaAliquotas` page — extend it for UF rates
- JSON file in repo would require rebuild to update and cannot be queried in SQL

**Confidence:** HIGH — based on existing codebase pattern and direct inspection.

---

## 4. Frontend Simulator (Real-Time, No Server Round-Trip)

### Decision: Pure client-side calculation with `useMemo` + `useDebounce` (no new library)

**Rationale:** The simulator (Módulo 1.2 reprecificação, Módulo 1.4 split payment float) computes deterministic arithmetic given:
- User-controlled inputs: IBS rate, CBS rate, ICMS reduction %, CDI rate, days float
- Loaded-once data: note totals from the API

All calculations are pure math — no joins, no aggregation needed at simulation time. Running these in the browser via `useMemo` is the correct pattern.

**Pattern:**

```tsx
// Inputs from Radix Slider (already in node_modules: @radix-ui/react-slider ^1.3.6)
const [ibsRate, setIbsRate] = useState(26.0)
const [cbsRate, setCbsRate] = useState(8.8)

// Simulation result: only recomputes when inputs change
const simulation = useMemo(() => {
  return notes.map(note => ({
    ...note,
    precoReprecificado: calcReprecificacao(note.v_prod, note.v_icms, ibsRate, cbsRate),
    floatTributario:    calcFloat(note.v_ibs, note.data_emissao, cdiRate, days),
  }))
}, [notes, ibsRate, cbsRate, cdiRate, days])
```

**Debounce for text input:** If users type IBS/CBS rates as numbers (not sliders), wrap `setIbsRate` with `useCallback` + debounce. The existing `date-fns` package or a 10-line `useDebounce` hook (no new library) is sufficient. Do not add `lodash` or `use-debounce` — overhead not justified.

**Radix Slider is already installed** (`@radix-ui/react-slider ^1.3.6` in `package.json`). No new component library needed.

**Recharts for result visualization:** Already installed (`recharts ^3.7.0`). Use `BarChart` for before/after comparison, `LineChart` for float cost over time.

**Confidence:** HIGH — based on direct package.json inspection; no new package needed.

---

## 5. Go Backend — New Endpoints

### Decision: No new Go libraries needed

**Rationale:** All five new analytical module endpoints follow the exact same pattern used in the 80+ existing endpoints:
- `database/sql` with `db.Query()` / `db.QueryRow()`
- `encoding/json` for response serialization
- JWT middleware already handles auth + company_id extraction
- `http.HandleFunc` registration in `main.go`

**Float/capital giro calculations:** Use Go's standard `time` package.

```go
// Example: split payment float cost
func calcFloatCost(valorIBS float64, dataEmissao time.Time, cdiAnual float64) float64 {
    diasFloat := 30  // regime padrão split payment
    fatorDiario := math.Pow(1 + cdiAnual/100, 1.0/252)  // dias úteis
    return valorIBS * (math.Pow(fatorDiario, float64(diasFloat)) - 1)
}
```

Standard library `math` package handles all exponentiation. No external financial library needed.

**Pattern for analytics handlers:**

New handlers go in `backend/handlers/` following naming convention of existing handlers:
- `analytics_ncm.go` — Módulo 2.1 (NCM analysis)
- `analytics_cfop.go` — Módulo 2.2 (CFOP groups)
- `analytics_uf.go` — Módulo 2.3 (UF destination)
- `analytics_segmento.go` — Módulo 2.4 (B2B/B2C segmentation)
- `reprecificacao.go` — Módulo 1.2 (repricing)
- `split_payment.go` — Módulo 1.4 (float calculation)

**B2B/B2C segmentation:** The `nfe_saidas` table has `dest_cnpj_cpf VARCHAR(14)`. B2C detection: `LENGTH(dest_cnpj_cpf) = 11` (CPF) OR `dest_cnpj_cpf IS NULL`. No `ind_final` column exists in the current schema — it was not imported. Migration needed to add it, OR derive from CPF length. The CPF-length heuristic is sufficient for the analytics use case.

**Confidence:** HIGH — verified against go.mod, existing handler patterns, and migration schema.

---

## Summary: What Actually Needs to Be Added

| Category | Addition | Type |
|----------|----------|------|
| Frontend library | `react-simple-maps` ^3.0.0 | `npm install react-simple-maps` |
| Frontend asset | Brazil states TopoJSON | Commit to `frontend/public/brazil-states.json` |
| DB migration | `tabela_aliquotas_uf` table | New `.sql` migration file |
| DB migration | Materialized views for NCM/CFOP/UF aggregations | New `.sql` migration file |
| Go code | Analytics handlers (6 files) | New `.go` files, no new imports |
| Frontend pages | 8 new analytical pages | Follow existing page patterns |

**Nothing else.** No new Go libraries. No new state management. No new charting library. No additional database infrastructure (no ClickHouse, no TimescaleDB, no Redis activation — PostgreSQL 15 with materialized views is sufficient for the volumes described).

---

## Alternatives Explicitly Rejected

| Candidate | Rejected Because |
|-----------|-----------------|
| ClickHouse / TimescaleDB | Overkill for single-tenant analytics at <1M rows/company; adds deployment complexity incompatible with Coolify+Docker setup |
| Redis for caching analytics | Redis is already provisioned but unused; materialized views in PostgreSQL are simpler and already the established pattern; not worth activating Redis for read caching when MV refresh is fast |
| Apache ECharts / ApexCharts | Recharts 3.7 already installed and used; switching adds bundle size and breaks the shadcn/ui chart.tsx wrapper |
| react-leaflet | Tile-map overhead; choropleth on state level does not need geo tile rendering |
| `use-debounce` npm package | 10-line `useDebounce` hook covers the use case; no external dependency justified |
| `shopspring/decimal` Go library | NUMERIC(15,2) values from PostgreSQL round-trip correctly through `float64` at the scale used (millions of BRL); fiscal reform is a simulation/estimation tool, not a ledger |

---

## Installation

```bash
# Frontend only — one new package
npm install react-simple-maps
# No --save needed, npm 5+ saves by default

# No Go packages to install
# No docker-compose changes
```

---

## Sources

- PostgreSQL 15 Materialized Views: https://www.postgresql.org/docs/current/rules-materializedviews.html
- REFRESH MATERIALIZED VIEW CONCURRENTLY: https://www.postgresql.org/docs/current/sql-refreshmaterializedview.html
- react-simple-maps official docs: https://www.react-simple-maps.io/docs/getting-started/
- react-simple-maps npm: https://www.npmjs.com/package/react-simple-maps
- Brazil TopoJSON (IBGE-sourced): https://github.com/carolinabigonha/br-atlas
- Brazil GeoJSON states: https://github.com/tbrugz/geodata-br
- Radix UI Slider: https://www.radix-ui.com/primitives/docs/components/slider
- PostgreSQL CTEs and Window Functions: https://www.crunchydata.com/blog/postgres-subquery-powertools-subqueries-ctes-materialized-views-window-functions-and-lateral
- Codebase inspection: `/home/claudiobezerra/projetos/FB_APU04/backend/go.mod` — confirms no new Go deps needed
- Codebase inspection: `/home/claudiobezerra/projetos/FB_APU04/frontend/package.json` — confirms Recharts 3.7 and Radix Slider already present
