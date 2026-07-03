# Phase 12: Tela Comparação Fiscal + Navegação - Research

**Researched:** 2026-07-03
**Domain:** Full-stack CRUD comparison screen (Go/Postgres backend + React/TS frontend), reusing an existing execution endpoint from Phase 11
**Confidence:** HIGH (backend schema/handlers read directly; frontend patterns read directly from the two binding-precedent screens named in CONTEXT.md)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Gatilho de execução**
- **D-01:** A tela dispara a execução do lote — não é só visualização passiva. Usuário busca uma nota, clica em "Executar"/"Rodar motor fiscal", a tela chama `POST /api/fiscal/execute` com o `nfe_id`, aguarda a resposta, e então recarrega automaticamente os itens executados dessa nota (cards + tabela) na mesma tela — sem navegação extra nem toast-e-clique-de-novo.

**Escopo de seleção de nota**
- **D-02:** Fluxo é por nota específica, não listagem agregada de tudo já executado. Usuário busca 1 NF-e (número da nota ou chave de acesso, campo de texto com autocomplete — mesmo padrão de busca de NF-e já usado em outras telas do FB_APU04), seleciona, e a tela mostra só os itens daquela nota. É uma ferramenta de validação pontual, não um relatório de massa.
- **D-03:** Reexecutar uma nota já executada é permitido (o backend já faz upsert por `nfe_item_id` via `ON CONFLICT`, migration 147) — não precisa de lógica nova de "já foi executada" na tela, o upsert do backend cobre isso.

**Exportação Excel**
- **D-04:** Incluir botão "Exportar Excel" na tela, seguindo o mesmo padrão já usado em `ConciliacaoBridgeXML.tsx`/`ComparativoEFDvsXML.tsx` (endpoint CSV/Excel dedicado do lado backend). Não estava em TPF-06/07/08 literalmente, mas o usuário confirmou que entra nesta fase por consistência com as telas análogas.

**Ruído conhecido em IBS/CBS**
- **D-05:** `nfe_saidas_itens.v_ibs`/`v_cbs` costumam estar NULL/0 hoje (gap de parser documentado em `.continue-here.md`), então o comparativo vai marcar quase todo item como "divergente" nesses 2 impostos mesmo quando o pacote fiscal está correto. Tratamento: **tooltip de aviso apenas** — IBS e CBS continuam contando normalmente para o filtro "só divergentes" e para o resumo agregado, sem tratamento especial além do aviso contextual (já desenhado no UI-SPEC). Não esconder nem excluir esses campos do cálculo.

### Claude's Discretion
- Exato componente/endpoint de exportação Excel a reaproveitar/criar (CSV vs. xlsx real) — seguir o padrão mais próximo já existente no codebase (`ConciliacaoBridgeXML.tsx`/`ComparativoEFDvsXML.tsx`).
- Layout exato do estado de loading durante a execução do lote (spinner inline no botão vs. skeleton na área de resultado) — UI-SPEC já cobre estados vazio/erro, mas não o loading de execução em si; usar convenção já estabelecida no UI-SPEC (spinner + texto).
- Paginação/virtualização da tabela item a item se uma nota tiver muitos itens — não é um requisito explícito, decidir com base no volume real (notas de venda tipicamente não passam de dezenas de itens).

### Deferred Ideas (OUT OF SCOPE)
- Sistema de permissão granular por módulo (substituindo o gate `adminOnly` binário) — milestone futura, já documentado como fora de escopo desde a Fase 11.
- Execução em lote de múltiplas notas de uma vez (ex.: todas as notas de um período) — fora de escopo desta fase, que é validação pontual nota a nota; poderia virar fase futura se o volume de uso justificar.
- Paginação/virtualização de tabela para notas com muitos itens — deixado como Claude's Discretion nesta fase, pode virar ajuste futuro se necessário.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| TPF-06 | Tela "Comparação Fiscal" — esperado (`nfe_saidas_itens`) vs. calculado (`fiscal_execution_items`), divergências destacadas em ICMS/ICMS-ST/PIS/COFINS/IBS/CBS | Exact field-pairing table below (Pattern 3); new `GET /api/fiscal/comparacao` handler design (Pattern 2); UI-SPEC layout blocks 5-6 |
| TPF-07 | Filtro "só divergentes" e resumo agregado de divergências na tela | Divergence rule confirmed (zero-tolerance, per UI-SPEC); aggregate counts computable client-side from the comparison payload — no new backend aggregation endpoint needed (dataset is capped at "one note's items," typically <100 rows) |
| TPF-08 | Item de navegação novo "Teste Pacote Fiscal" com gate `adminOnly: true` | Exact `navigation.ts`/`AppSidebar.tsx`/`App.tsx` wiring confirmed by reading current file contents (Pattern 5) |
</phase_requirements>

## Summary

Phase 12 is a pure "read + trigger + compare" screen with almost zero new backend logic beyond three new HTTP handlers (search, comparison read, CSV export) and zero new npm/Go dependencies. The Phase 11 execution endpoint (`POST /api/fiscal/execute`) and its schema (`fiscal_execution_items`, migration 147) are already correct and complete — this phase only needs to call it and render its output next to `nfe_saidas_itens`.

The most important finding is that **two of CONTEXT.md's stated "existing patterns" do not fully exist as described** and must be built new in this phase, not merely reused:

1. **No backend endpoint searches `nfe_saidas` by número/chave.** `GET /api/nfe-saidas` (the only existing NF-e-saída list endpoint) filters only by `mes_ano` and `emit_cnpj`, and returns up to 500 rows unfiltered by number/chave — the frontend would have to load and grep 500 notes client-side, which is not autocomplete. The closest **backend** precedent for server-side ILIKE search-by-número is `AdminNFCancelamentoHandler` (`GET /api/admin/nf/cancelamentos?num_nota=...`), but it queries `nfe_entradas`, not `nfe_saidas`. A new handler must be written, structurally copying that ILIKE-search pattern against `nfe_saidas`.
2. **No frontend component does server-driven autocomplete-by-text.** The two `cmdk`-based comboboxes in the codebase (`FilialSelector.tsx`, `CompanySwitcher.tsx`) filter an already-fully-loaded in-memory list — they are not "type 3 chars → debounced fetch → show results." `ConsultaNFeSaidas.tsx` is the closest data-shape precedent (loads NF-e-saída rows, has a detail dialog) but is a full-listing screen with client-side filters by `dest_nome`/`dest_cnpj_cpf`, not chave/número, and is not a search-select control. The vendored `Command`/`Popover`/`CommandInput` primitives (`frontend/src/components/ui/command.tsx`, `popover.tsx`) are the correct building blocks — just wired to a new debounced fetch instead of a static in-memory array. Zero new npm packages required (`cmdk` is already a transitive dependency of the vendored `command.tsx`).

The field-pairing between "esperado" (`nfe_saidas_itens`) and "calculado" (`fiscal_execution_items`) is fully determined by the two migration files (075/141/146 for the expected side, 147 for the calculated side) — see Pattern 3 below. One asymmetry requires explicit planner attention: `nfe_saidas_itens.v_ibs` is a single total column, but `fiscal_execution_items` stores IBS as two separate columns (`valor_ibs_uf` + `valor_ibs_mun`, no total column) — the comparison must sum the two calculated columns before diffing against the single expected column.

**Primary recommendation:** Build 3 new Go handlers (search, comparison-read, CSV-export) following the exact ILIKE/IDOR/CSV patterns already established in `admin_nf_cancelamento.go` and `xml_conciliacao.go`; build 1 new debounced Command/Popover search component reusing existing vendored primitives; reuse `exportToExcel()` client-side for the Excel button (zero backend work); wire navigation exactly as pre-specified in the UI-SPEC (already verified against current `navigation.ts`/`AppSidebar.tsx`/`App.tsx` contents).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| NF-e search/autocomplete (by número/chave) | API / Backend | Browser / Client | Search-as-you-type needs server-side ILIKE + `LIMIT` + company_id scoping (IDOR); client only debounces and renders |
| Trigger fiscal execution ("Executar" button) | API / Backend (existing, Phase 11) | Browser / Client | Frontend only calls `POST /api/fiscal/execute`; all business logic already lives server-side (Phase 11) |
| Esperado vs. calculado comparison read | API / Backend | Database / Storage | New `LEFT JOIN` query (`nfe_saidas_itens` ⟕ `fiscal_execution_items`) belongs server-side — keeps the SQL join logic (incl. the IBS UF+MUN sum) out of the frontend and reusable by the CSV export handler |
| Divergence computation (per-tax, per-row) | Browser / Client | — | Zero-tolerance `abs(a-b) != 0` is trivial arithmetic on already-fetched JSON; no reason to push this to the backend — avoids a second round-trip when the "só divergentes" filter toggles |
| Aggregate summary (4 KPI cards + 6 per-tax chips) | Browser / Client | — | Dataset is bounded to one note's items (tens of rows); client-side `reduce()` over the already-fetched payload is simpler and avoids a second endpoint |
| CSV export | API / Backend | — | Follows `ConciliacaoCSVHandler`/`XMLSaneamentoCSVHandler` convention: `encoding/csv` streamed with `Content-Disposition: attachment` |
| Excel export | Browser / Client | — | `exportToExcel()` (client-side `xlsx` lib) — zero backend work, exact precedent in `ConciliacaoBridgeXML.tsx` |
| Navigation gate (`adminOnly`) | Browser / Client (route guard) | API / Backend (defense-in-depth) | `AdminRoute` wrapper in `App.tsx` is the primary gate; the new backend handlers must ALSO require `"admin"` role via `withAuth(..., "admin")` (same as `/api/fiscal/execute`) — hiding the nav item alone is not a security boundary |

## Standard Stack

### Core (already in project — no new installs)
| Library | Version | Purpose | Why Standard (for this phase) |
|---------|---------|---------|--------------------------------|
| `@tanstack/react-query` | already installed | `useQuery`/`useMutation` for search, comparison fetch, and the execute-then-reload flow | Established convention for this screen family (UI-SPEC data-fetching row); `useMutation` + `queryClient.invalidateQueries` is the exact pattern already used for "trigger backend job then refresh" in `ImportarViaERP.tsx` (`trigger` mutation, lines 188-203) |
| `xlsx` (`^0.18.5`) | `^0.18.5` [VERIFIED: package.json] | `exportToExcel()` helper (`frontend/src/lib/exportToExcel.ts`) | Already used by `ConciliacaoBridgeXML.tsx`'s "Exportar Excel" button — direct reuse, zero new code needed besides the column-mapping object |
| `cmdk` (transitive, via vendored `components/ui/command.tsx`) | already vendored | Search-as-you-type combobox base (`Command`/`CommandInput`/`CommandList`/`CommandItem`) | Already used by `FilialSelector.tsx`/`CompanySwitcher.tsx` — same primitives, different data source (debounced fetch instead of static array) |
| `lucide-react` | already installed | `AlertTriangle`, `CheckCircle`, `HelpCircle`, `Download`, `FileSpreadsheet`, `Search`, `FlaskConical`, `GitCompare` icons | Confirmed already in use across every comparable screen |
| Go stdlib `encoding/csv` | stdlib | CSV export handler | Exact convention already used in `xml_conciliacao.go`/`xml_reports.go` — no third-party CSV/Excel Go library needed |

### Supporting
None — this phase needs no new supporting libraries.

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Server-side ILIKE search endpoint | Client-side filter over `GET /api/nfe-saidas` (like `ConsultaNFeSaidas.tsx` does) | Rejected: that endpoint returns up to 500 rows and doesn't filter by número/chave at all — would require a new query param anyway, and 500 unfiltered rows defeats "autocomplete" UX for companies with more history |
| `cmdk` Command/Popover for search | Plain `Input` + "Buscar" button (like `ConciliacaoBridgeXML.tsx`'s mês/ano filter) | Rejected: D-02 explicitly requires "autocomplete," which the plain-Input+button pattern does not provide; Command/Popover is zero-new-dependency and already vendored |
| Client-side aggregate summary | New backend aggregation endpoint | Rejected: dataset is bounded to one note (typically <100 items) — a second round-trip adds latency/complexity for no benefit at this scale |

**Installation:** None required — every library above is already a project dependency.

## Package Legitimacy Audit

**Not applicable — this phase installs zero new external packages.** Every library referenced above (`@tanstack/react-query`, `xlsx`, `cmdk` via vendored `command.tsx`, `lucide-react`, Go `encoding/csv`) is already present in `frontend/package.json` / the Go standard library and already used in production by sibling screens. The Package Legitimacy Gate (slopcheck + registry verification) is skipped per its own trigger condition ("whenever this phase installs external packages").

## Architecture Patterns

### System Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│  Browser (React) — ComparacaoFiscal.tsx                              │
│                                                                        │
│  [1] Search box (Command/Popover, debounced)                          │
│         │ GET /api/fiscal/comparacao/search?q=...                     │
│         ▼                                                             │
│  [2] User selects a note → nfe_id stored in state                     │
│         │                                                             │
│  [3a] "Executar" button        [3b] auto-fires on note select         │
│         │ POST /api/fiscal/execute {nfe_id}                           │
│         ▼                                                             │
│  [4] useMutation.onSuccess → queryClient.invalidateQueries(           │
│         ['fiscal-comparacao', nfe_id])                                │
│         │                                                             │
│  [5] useQuery refetches        GET /api/fiscal/comparacao?nfe_id=...  │
│         ▼                                                             │
│  [6] Client computes: per-row divergence (zero-tolerance),            │
│      4 KPI cards, 6 per-tax chips, "só divergentes" filter            │
│         │                                                             │
│  [7] Render: summary cards → per-tax chips → filter → table → dialog  │
│         │                                                             │
│  [8] "Exportar Excel" → exportToExcel() (client-side, no request)     │
│  [9] "Exportar CSV"   → GET /api/fiscal/comparacao/csv?nfe_id=...     │
└─────────────────────────────────────────────────────────────────────┘
            │                    │                        │
            ▼                    ▼                        ▼
┌───────────────────┐  ┌──────────────────────┐  ┌─────────────────────┐
│ NEW: search handler│  │ EXISTING (Phase 11): │  │ NEW: comparison read │
│ GET /api/fiscal/   │  │ POST /api/fiscal/    │  │ + CSV export handler │
│ comparacao/search   │  │ execute (admin-only) │  │ (admin-only)         │
│ ILIKE nfe_saidas    │  │ fan-out → Oracle     │  │ LEFT JOIN            │
│ (número/chave),     │  │ PKG_FISCAL_FCTAX →   │  │ nfe_saidas_itens ⟕   │
│ company_id-scoped   │  │ upsert fiscal_       │  │ fiscal_execution_    │
│                     │  │ execution_items       │  │ items ON nfe_item_id │
└───────────────────┘  └──────────┬────────────┘  └──────────┬──────────┘
                                   ▼                           ▼
                        ┌────────────────────────────────────────────┐
                        │  Postgres: nfe_saidas / nfe_saidas_itens /  │
                        │  fiscal_execution_items                     │
                        └────────────────────────────────────────────┘
```

### Recommended Project Structure
```
backend/handlers/
├── fiscal_execution.go              # EXISTING (Phase 11) — POST /api/fiscal/execute, untouched
├── fiscal_comparacao.go             # NEW — search handler + comparison-read handler
├── fiscal_comparacao_csv.go         # NEW — CSV export handler (or same file as above, project convention allows both — xml_conciliacao.go keeps JSON+CSV handlers in one file)

frontend/src/pages/
├── ComparacaoFiscal.tsx             # NEW — main screen (search + trigger + cards + table + dialog + export)

frontend/src/components/
├── NfeSearchCombobox.tsx            # NEW — reusable debounced Command/Popover NF-e search (candidate for reuse beyond this phase)
```

### Pattern 1: Debounced server-side search combobox (NEW — no exact precedent, composed from 2 existing patterns)
**What:** A `Command`/`Popover` combobox (UI shell copied from `FilialSelector.tsx`) whose `CommandInput` `onValueChange` drives a debounced `useQuery` (data-fetching pattern copied from `ConciliacaoBridgeXML.tsx`'s `buildUrl` + `useQuery` convention) against a new search endpoint, instead of filtering an in-memory array.
**When to use:** Any future "search entity by text, few results" screen — this is the first server-driven combobox in the codebase and is worth extracting as a reusable component.
**Example:**
```tsx
// frontend/src/components/NfeSearchCombobox.tsx
// Composed from: Command/Popover shell (FilialSelector.tsx) +
// useQuery debounce convention (project-wide react-query usage)
import { useState, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Command, CommandInput, CommandList, CommandEmpty, CommandGroup, CommandItem } from '@/components/ui/command';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Button } from '@/components/ui/button';
import { Search } from 'lucide-react';
import { useAuth } from '@/contexts/AuthContext';

interface NfeSearchResult {
  id: string;
  chave_nfe: string;
  numero_nfe: string;
  serie: string;
  dest_nome: string;
  data_emissao: string;
}

export function NfeSearchCombobox({ onSelect }: { onSelect: (nfe: NfeSearchResult) => void }) {
  const { token, companyId } = useAuth();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [debounced, setDebounced] = useState('');

  // Debounce: 300ms is the project-wide convention for search-as-you-type
  // (no existing precedent to cite verbatim — chosen as a standard UX default,
  // consistent with the codebase's absence of any faster/slower debounce elsewhere)
  useMemo(() => {
    const t = setTimeout(() => setDebounced(query), 300);
    return () => clearTimeout(t);
  }, [query]);

  const { data, isFetching } = useQuery<NfeSearchResult[]>({
    queryKey: ['nfe-saidas-search', debounced, companyId],
    queryFn: async () => {
      const res = await fetch(`/api/fiscal/comparacao/search?q=${encodeURIComponent(debounced)}`, {
        headers: { Authorization: `Bearer ${token}`, 'X-Company-ID': companyId || '' },
      });
      if (!res.ok) throw new Error(res.statusText);
      return res.json();
    },
    enabled: debounced.length >= 3, // avoid firing on 1-2 chars
  });

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button variant="outline" role="combobox" aria-expanded={open} className="w-80 justify-start">
          <Search className="h-3.5 w-3.5 mr-2 opacity-50" />
          Buscar NF-e por número ou chave...
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-80 p-0">
        <Command shouldFilter={false}>
          <CommandInput placeholder="Número ou chave de acesso..." value={query} onValueChange={setQuery} />
          <CommandList>
            <CommandEmpty className="text-xs py-3 text-center">
              {isFetching ? 'Buscando...' : debounced.length < 3 ? 'Digite ao menos 3 caracteres.' : 'Nenhuma nota encontrada.'}
            </CommandEmpty>
            <CommandGroup>
              {(data ?? []).map(nfe => (
                <CommandItem key={nfe.id} value={nfe.id} onSelect={() => { onSelect(nfe); setOpen(false); }}>
                  <span className="text-xs">
                    Nº {nfe.numero_nfe}/{nfe.serie} — {nfe.dest_nome} — {nfe.data_emissao}
                  </span>
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}
```

### Pattern 2: New search + comparison-read Go handlers (composed from admin_nf_cancelamento.go + xml_conciliacao.go conventions)
**What:** Two new handlers in `backend/handlers/fiscal_comparacao.go`, following the exact auth/IDOR/query conventions already established.
**When to use:** This phase, for TPF-06.
**Example:**
```go
// backend/handlers/fiscal_comparacao.go — NEW
// Search pattern copied from admin_nf_cancelamento.go (GET .../cancelamentos?num_nota=...)
// but scoped to nfe_saidas (not nfe_entradas) and matching chave_nfe OR numero_nfe.
package handlers

// GET /api/fiscal/comparacao/search?q=...
func FiscalComparacaoSearchHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			jsonErr(w, http.StatusMethodNotAllowed, "Método não permitido")
			return
		}
		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			jsonErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		userID, _ := claims["user_id"].(string)
		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa")
			return
		}
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		if len(q) < 3 {
			json.NewEncoder(w).Encode([]interface{}{})
			return
		}
		rows, err := db.Query(`
			SELECT id, chave_nfe, COALESCE(numero_nfe,''), COALESCE(serie,''),
			       COALESCE(dest_nome,''), TO_CHAR(data_emissao,'DD/MM/YYYY')
			FROM nfe_saidas
			WHERE company_id = $1
			  AND (numero_nfe ILIKE '%'||$2||'%' OR chave_nfe ILIKE '%'||$2||'%')
			ORDER BY data_emissao DESC
			LIMIT 20`, companyID, q)
		// ... scan into []struct, json.NewEncoder(w).Encode(...)
	}
}

// GET /api/fiscal/comparacao?nfe_id=...
// LEFT JOIN so items never executed still appear (fiscal_execution_items.* = NULL).
func FiscalComparacaoReadHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// ... auth + companyID as above ...
		nfeID := r.URL.Query().Get("nfe_id")
		rows, err := db.Query(`
			SELECT
				nsi.id, nsi.n_item, nsi.c_prod, nsi.x_prod, nsi.ncm, nsi.cfop,
				nsi.v_bc_icms, nsi.v_icms, nsi.v_bc_st, nsi.v_st,
				nsi.v_bc_pis, nsi.v_pis, nsi.v_bc_cofins, nsi.v_cofins,
				nsi.v_ibs, nsi.v_cbs,
				fei.status, fei.error_message, fei.executed_at,
				fei.base_calculo_icms, fei.valor_icms,
				fei.base_substituicao, fei.valor_substituicao,
				fei.base_calculo_pis, fei.valor_pis,
				fei.base_calculo_cofins, fei.valor_cofins,
				fei.valor_ibs_uf, fei.valor_ibs_mun, fei.valor_cbs,
				fei.percentual_difal, fei.valor_icms_partilha_destino, fei.valor_icms_pobreza,
				fei.full_result
			FROM nfe_saidas_itens nsi
			LEFT JOIN fiscal_execution_items fei ON fei.nfe_item_id = nsi.id
			WHERE nsi.nfe_id = $1 AND nsi.company_id = $2
			ORDER BY nsi.n_item ASC`, nfeID, companyID)
		// COALESCE(fei.status, 'not_executed') should be applied either in SQL
		// or in the Go scan step — see Pitfall 1 below.
	}
}
```

### Pattern 3: Esperado × Calculado field-pairing table (READ DIRECTLY from migrations — do not re-derive)
**What:** The exact column mapping between `nfe_saidas_itens` (esperado) and `fiscal_execution_items` (calculado), verified against migrations 075, 094, 095, 141, 146 (expected side) and 147 (calculated side).
**When to use:** This is the authoritative source for the comparison table columns in TPF-06/07.

| Imposto | Campo | Esperado — `nfe_saidas_itens` | Calculado — `fiscal_execution_items` | Nota |
|---------|-------|-------------------------------|----------------------------------------|------|
| ICMS | Base | `v_bc_icms` | `base_calculo_icms` | Direct 1:1 |
| ICMS | Valor | `v_icms` | `valor_icms` | Direct 1:1 |
| ICMS-ST | Base | `v_bc_st` | `base_substituicao` | `v_bc_st`/`v_st` added by migration 141; NULL for items imported before that migration |
| ICMS-ST | Valor | `v_st` | `valor_substituicao` | Direct 1:1 |
| PIS | Base | `v_bc_pis` | `base_calculo_pis` | Direct 1:1 |
| PIS | Valor | `v_pis` | `valor_pis` | Direct 1:1 |
| COFINS | Base | `v_bc_cofins` | `base_calculo_cofins` | Direct 1:1 |
| COFINS | Valor | `v_cofins` | `valor_cofins` | Direct 1:1 |
| IBS | Valor (total) | `v_ibs` (single column, `DEFAULT 0`) | `valor_ibs_uf + valor_ibs_mun` (**must be summed** — no total column exists in `fiscal_execution_items`) | **Asymmetry — see Pitfall 2.** Also the known D-05 data-quality noise (often 0/NULL on the expected side) |
| CBS | Valor | `v_cbs` (`DEFAULT 0`) | `valor_cbs` | Direct 1:1; same D-05 noise |

**"Só calculado" fields (no `nfe_saidas_itens` item-level counterpart — per UI-SPEC block 6, Dialog section):**
- `percentual_difal` — % DIFAL used in calc
- `valor_icms_partilha_destino` — DIFAL amount
- `valor_icms_pobreza` — FCP amount
- `grupo_fiscal_codigo`, `input_params`, `full_result` (JSONB, ~88 fields) — audit/debug only

**Identification/join columns:**
- `fiscal_execution_items.nfe_item_id` → FK to `nfe_saidas_itens.id` (NOT `nfe_id` — join is per-item, one `fiscal_execution_items` row per `nfe_saidas_itens` row, enforced by `UNIQUE(nfe_item_id)` in migration 147)
- `nfe_saidas_itens.nfe_id` → FK to `nfe_saidas.id` (used to scope the whole-note query)
- Row-level status source: `fiscal_execution_items.status` (`pending` | `ok` | `error` | `sem_grupo_fiscal`) — **but a `LEFT JOIN` also produces a 5th implicit state: no `fiscal_execution_items` row at all** (item never executed). See Pitfall 1.

Fields explicitly OUT of TPF-06/07 scope (present in schemas but not part of the 6-tax comparison): `v_ipi` (nfe_saidas_itens has it; `fiscal_execution_items` has no IPI column — IPI is not one of the 6 taxes named in TPF-06), `cst_icms`/`cst_pis`/`cst_cofins`/`cest`/`cclasstrib` (classification metadata, useful for the "Identificação" dialog section but not for comparison).

### Pattern 4: Trigger-then-reload via react-query mutation (EXACT precedent: `ImportarViaERP.tsx` `trigger` mutation)
**What:** `useMutation` for `POST /api/fiscal/execute`, `onSuccess` invalidates the comparison query key so it refetches automatically — exactly satisfies D-01's "aguarda a resposta, e então recarrega automaticamente."
**Example:**
```tsx
// Source: frontend/src/pages/ImportarViaERP.tsx lines 188-203 (exact pattern, adapted)
const queryClient = useQueryClient();
const executar = useMutation({
  mutationFn: async (nfeId: string) => {
    const res = await fetch('/api/fiscal/execute', {
      method: 'POST',
      headers: { Authorization: `Bearer ${token}`, 'X-Company-ID': companyId || '', 'Content-Type': 'application/json' },
      body: JSON.stringify({ nfe_id: nfeId }),
    });
    if (!res.ok) throw new Error((await res.text()) || 'Erro ao executar');
    return res.json(); // { total, ok, sem_grupo_fiscal, error }
  },
  onSuccess: (summary, nfeId) => {
    toast.success(`Execução concluída: ${summary.ok}/${summary.total} OK.`);
    queryClient.invalidateQueries({ queryKey: ['fiscal-comparacao', nfeId] });
  },
  onError: (e: Error) => toast.error(e.message),
});
```

### Pattern 5: Navigation wiring (VERIFIED against current file contents — matches UI-SPEC exactly)
**What:** `AppSidebar.tsx` already declares `NavSection[]` with `adminOnly?: boolean` at both section and item level (confirmed lines 70-120); `navigation.ts` already declares `adminOnly?: boolean` per tab (line 6) and has `getActiveModule()` path-prefix matching (lines 95-104); `App.tsx` already has a working `AdminRoute` wrapper (lines 77-84) used verbatim for `/importacoes/erp-bridge*` (lines 195-198).
**Confirmed exact insertion points:**
- `AppSidebar.tsx`: add a `NavSection` object to the `sections` array (line 85+), same shape as the `"malha"` section (id, title, sectionIcon, adminOnly, items).
- `navigation.ts`: add a `pacotefiscal` key to the modules map, same shape as `auditoria` (single tab, no `adminOnly` needed at module level since the whole section is already gated in the sidebar — but UI-SPEC's per-tab `adminOnly: true` is harmless defense-in-depth, keep it).
- `App.tsx`: add `<Route path="/pacote-fiscal/comparacao" element={<AdminRoute><ComparacaoFiscal /></AdminRoute>} />` in the same route block as the `/importacoes/erp-bridge*` routes (around line 195-198), plus the corresponding `import ComparacaoFiscal from './pages/ComparacaoFiscal'` at the top (alongside line 24-44's page imports).
- Role check for the new backend handlers: reuse `withAuth(handlers.FiscalComparacaoSearchHandler, "admin")` / `withAuth(handlers.FiscalComparacaoReadHandler, "admin")` / `withAuth(handlers.FiscalComparacaoCSVHandler, "admin")` in `main.go`, registered in the same block as `/api/fiscal/execute` (line 535) and `/api/fiscal/oracle-ping` (line 532).

### Anti-Patterns to Avoid
- **Do not reuse `GET /api/nfe-saidas` for the search box.** It has no número/chave filter and caps at 500 unordered-by-relevance rows — it would silently fail to find notes outside the first 500 by `data_emissao DESC`.
- **Do not apply the `> 0.01` divergence threshold from `ConciliacaoBridgeXML.tsx`.** UI-SPEC's Divergence Visual Rules are explicit and binding: zero tolerance (`abs(esperado - calculado) != 0`), because this is a package validator, not an ERP-vs-XML reconciliation.
- **Do not compare `v_ibs` directly against a single `valor_ibs` column** — that column does not exist in `fiscal_execution_items`. Must sum `valor_ibs_uf + valor_ibs_mun` server- or client-side before diffing (see Pattern 3 and Pitfall 2).
- **Do not build a new CSV/Excel library integration.** `xlsx` (client-side) and Go `encoding/csv` (server-side) are already the established, working conventions — introducing e.g. `excelize` (Go) or a backend `.xlsx` generator would contradict D-04's explicit instruction to match `ConciliacaoBridgeXML.tsx`'s pattern.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Debounced search input | Custom `setTimeout` + raw `<input>` + manual dropdown | `Command`/`CommandInput`/`Popover` (already vendored) + `useQuery` with `enabled: debounced.length >= 3` | Keyboard nav (arrow keys, Enter to select, Esc to close) is already handled by `cmdk`; reinventing loses accessibility for free |
| Excel file generation | Backend `.xlsx` writer (e.g. `excelize` Go library) | `exportToExcel()` (`frontend/src/lib/exportToExcel.ts`, client-side `xlsx` package) | Already proven working in 2 sibling screens; adding a Go Excel library for this phase would be a net-new backend dependency for zero benefit |
| CSV writer / escaping | Manual string-join CSV | Go stdlib `encoding/csv` (`csv.NewWriter(w)`) | Already the exact convention in `xml_conciliacao.go`/`xml_reports.go` — handles quoting/escaping edge cases (commas, quotes in `x_prod` product descriptions) automatically |
| Admin-only route gating | Custom auth check per-page | `AdminRoute` wrapper (already exists, `App.tsx` line 77) | Zero new code — direct reuse, already battle-tested for `/importacoes/erp-bridge*`, `/config/usuarios`, etc. |

**Key insight:** This phase's entire backend surface (3 new handlers) can be written by copy-adapting 3 existing handlers verbatim (`admin_nf_cancelamento.go` for search, a new LEFT JOIN query for read, `xml_conciliacao.go`'s `ConciliacaoCSVHandler` for CSV) — there is no genuinely novel backend problem in this phase, only new column names.

## Common Pitfalls

### Pitfall 1: LEFT JOIN produces a 5th implicit status ("never executed") not covered by the documented state machine
**What goes wrong:** `fiscal_execution_items.status` is documented (migration 147 comment) as `pending | ok | error | sem_grupo_fiscal`. But the comparison query is `nfe_saidas_itens LEFT JOIN fiscal_execution_items`, so for a note that was never executed, every `fei.*` column — including `status` — comes back `NULL` in Go/JSON, not the string `"pending"`. If the frontend does `status === 'pending' ? 'Não calculado' : ...` without also handling `status == null`, the row will render with empty/undefined badges instead of the correct "Não calculado" (or a distinct "Não executado ainda") state.
**Why it happens:** `pending` is a DB column default (`DEFAULT 'pending'`) that only applies when a row is actually inserted; a `LEFT JOIN` with no matching row does not insert-with-defaults, it returns SQL `NULL`.
**How to avoid:** Either `COALESCE(fei.status, 'not_executed')` in the SQL, or explicitly branch on `status == null` in the Go struct (using `*string`) and again in the frontend TypeScript type (`status: string | null`). Recommend deciding explicitly in planning whether "never executed" gets its own visual treatment or collapses into the same "Não calculado" badge as `error`/`sem_grupo_fiscal`/`pending` — UI-SPEC doesn't address this 5th state directly, only 3 (OK/Divergente/Não calculado), and its "Não calculado" bucket description ("status != 'ok'") technically only covers non-null statuses.
**Warning signs:** Table rows for a freshly-searched, never-executed note showing blank/undefined instead of a clear "Não calculado" or "Não executado" badge.

### Pitfall 2: IBS comparison requires summing two calculated columns against one expected column
**What goes wrong:** A naive column-by-column diff (`v_ibs` vs. some `valor_ibs` field) will either fail to compile (no such column) or, if a developer mistakenly compares `v_ibs` against only `valor_ibs_uf` (ignoring `valor_ibs_mun`), the comparison will show false divergences for every item where MUN IBS is nonzero.
**Why it happens:** `nfe_saidas_itens` (parsed from the XML's flat `<vIBS>` total tag) stores IBS as one number; `fiscal_execution_items` (populated from the Oracle package's structured output) stores it split into UF and municipal components (`result.ValorIbsUF`, `result.ValorIbsMUN` — see `fiscal_execution.go` line 367-368), mirroring the reform's IBS revenue-split model.
**How to avoid:** Compute `calculado_ibs_total := valor_ibs_uf + valor_ibs_mun` (treating NULL as 0) before diffing against `v_ibs`. Decide once (in the backend SQL, for consistency between the JSON endpoint and the CSV export) rather than duplicating this arithmetic in the frontend AND the CSV handler.
**Warning signs:** IBS divergence count in the aggregate summary looks implausibly high even for items where the pacote fiscal's IBS split matches the invoice total exactly.

### Pitfall 3: `nfe_saidas_itens.v_ibs`/`v_cbs` default to `0`, not `NULL` — "zero" is indistinguishable from "not parsed"
**What goes wrong:** Because these columns are `NUMERIC(15,2) DEFAULT 0` (migration 075), a genuinely-zero IBS (e.g., an operation exempt from IBS) is stored identically to a never-populated field. The D-05 tooltip caveat is the only signal to the user that "0" might mean "parser didn't populate this," not "this item really has zero IBS."
**Why it happens:** The XML upload parser (`nfe_saidas.go`, `insertNFeItens`) doesn't currently extract `<gIBS>`/`<gCBS>` at the item (`<det>`) level at all — those tags are only parsed at the note header level (`total.IBSCBSTot`, used for `nfe_saidas.v_ibs`/`v_cbs`). Item-level `v_ibs`/`v_cbs` columns exist in the schema (migration 075) but `insertNFeItens`'s INSERT statement (verified in `nfe_saidas.go` lines 402-425) does **not include `v_ibs`/`v_cbs` in its column list at all** — meaning these columns are ALWAYS their table default (0), never populated from any XML, for every single note ever imported.
**How to avoid:** This is not a Phase 12 bug to fix — it's the exact gap D-05 already accounts for (tooltip warning, no special-casing). But the planner should know the caveat is not "sometimes NULL" — it's "always 0, for 100% of rows, permanently, until a future parser change." The tooltip copy in UI-SPEC ("pode aparecer zerado") slightly understates this — it's not intermittent, it is universal today.
**Warning signs:** N/A for this phase — this is expected, documented behavior, not something to debug.

### Pitfall 4: `withAuth(..., "")` vs `withAuth(..., "admin")` — role string is empty for "any authenticated user," not omitted
**What goes wrong:** Copying `main.go`'s route registration pattern carelessly could result in a route accidentally requiring no specific role (`""`) when it should require `"admin"`, exposing fiscal comparison data (which includes Oracle-sourced fiscal calculation details) to non-admin users — even though the nav item is hidden, the API would still respond.
**Why it happens:** The codebase's `withAuth` signature takes a role string where `""` means "authenticated, any role" (seen for `/api/xml/conciliacao`, `/api/nfe-saidas`) and `"admin"` means role-restricted (seen for `/api/fiscal/execute`, `/api/admin/*`). It's easy to default to `""` by copy-pasting a non-admin sibling handler's registration line.
**How to avoid:** All 3 new handlers in this phase must register with `"admin"` explicitly — `/api/fiscal/execute` (the endpoint they trigger) is already `"admin"`-gated, so the read/search/export siblings must match for consistency and to avoid a privilege-escalation gap (a non-admin user directly hitting the comparison API even though the nav entry and `AdminRoute` hide it from the UI).
**Warning signs:** A non-admin test user can `curl` the new endpoints successfully.

## Code Examples

See Pattern 1 (search combobox), Pattern 2 (Go handlers), Pattern 4 (trigger+reload mutation) above — all code examples are inline with their patterns since each is short and directly tied to one architectural decision.

## State of the Art

Not applicable in the "old vs. new industry practice" sense — this is an internal, first-of-its-kind screen for this codebase (first server-driven autocomplete, first Oracle-package-output comparison UI). No external framework/library version changes are relevant.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | 300ms debounce and 3-character minimum for the search box are reasonable UX defaults | Pattern 1 | Low — purely a UX tuning parameter, easy to adjust post-hoc, no architectural impact |
| A2 | Aggregate summary (4 KPI cards + 6 per-tax chips) can be computed entirely client-side from the comparison payload without a dedicated backend aggregation endpoint | Architectural Responsibility Map, Phase Requirements (TPF-07) | Low-Medium — holds as long as a single note's item count stays in the tens; if some notes have hundreds of items, client-side `reduce()` is still fine (cheap arithmetic), but the underlying table might then need the pagination/virtualization already flagged as Claude's Discretion in CONTEXT.md |
| A3 | The "never executed" (`fiscal_execution_items` row absent) state should probably render identically to "Não calculado," not as a distinct 4th visual state | Pitfall 1 | Medium — if the user/planner disagrees, the UI-SPEC and this research's Pitfall 1 need a 4th explicit state added before implementation; flagged as an open question below |

## Open Questions

1. **Should "nunca executado" (no `fiscal_execution_items` row) get a visually distinct state from "Não calculado" (`status = error/sem_grupo_fiscal/pending`)?**
   - What we know: UI-SPEC documents exactly 3 states (OK / Divergente / Não calculado) and defines "Não calculado" as `status != 'ok'`. That definition presumes a `fiscal_execution_items` row exists.
   - What's unclear: On first search of a note that has never been run through `/api/fiscal/execute`, every item's `fiscal_execution_items.*` columns are SQL `NULL` via the `LEFT JOIN` — technically not any of the 4 documented `status` values.
   - Recommendation: Treat `NULL` status the same as "Não calculado" for the badge/color (simplest, matches UI-SPEC's 3-state model), but consider a distinct tooltip message ("Nota ainda não executada — clique em Executar" vs. the existing "sem_grupo_fiscal — produto não encontrado..." style messages) so users don't confuse "never run" with "ran and failed." This is a planning-time decision, not a research gap — flagging so the plan explicitly assigns it rather than leaving it to task-time improvisation.

2. **Does the CSV export need the same `full_result` JSONB "Só calculado" fields (DIFAL/FCP) as columns, or only the 6-tax esperado/calculado/diferença set shown in the table?**
   - What we know: `ConciliacaoBridgeXML.tsx`'s CSV export mirrors exactly what's in the visible table (not the dialog's extra detail).
   - What's unclear: TPF-06/07 don't explicitly require DIFAL/FCP in the export; UI-SPEC's Detail Dialog section 6 lists them as dialog-only ("Só calculado" section), implying they're NOT meant for the main table/export.
   - Recommendation: Match the table exactly (6 taxes × 3 columns + identification), excluding DIFAL/FCP/grupo_fiscal_codigo from CSV/Excel — consistent with D-04's instruction to mirror the sibling screens' export scope (which also exports only what's in their visible table, not dialog-only detail).

## Environment Availability

Skipped — this phase has no new external dependencies (no new tools, services, runtimes, or package managers). All required infrastructure (Postgres, the existing Go backend, the existing React frontend build) is already running and verified by Phase 11's completion.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-------------------|
| V2 Authentication | Yes (inherited) | Existing JWT bearer-token auth (`ClaimsKey` context, `withAuth` middleware) — no new auth mechanism introduced |
| V3 Session Management | No | Unchanged — no new session handling in this phase |
| V4 Access Control | Yes | `adminOnly` gate at 3 layers: (1) `AppSidebar.tsx`/`navigation.ts` hide the nav item for non-admins, (2) `AdminRoute` wrapper blocks direct URL navigation, (3) **all 3 new backend handlers must use `withAuth(handler, "admin")`** — layer 3 is the only one that actually stops an API call, layers 1-2 are UX-only (see Pitfall 4) |
| V5 Input Validation | Yes | `q` search param passed into `ILIKE '%'||$N||'%'` — MUST remain parameterized (`$1`/`$2` placeholders, never string-concatenated into SQL), exactly matching `admin_nf_cancelamento.go`'s existing pattern; `nfe_id` param must be validated as a well-formed UUID or rely on Postgres's own UUID type-check failing safely (existing `nfe_saidas` queries already do this — `WHERE id = $1` against a `UUID` column) |
| V6 Cryptography | No | Not applicable — no new secrets/crypto in this phase |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|----------------------|
| IDOR — `nfe_id` from another company accessed via the comparison-read endpoint | Tampering / Information Disclosure | `WHERE nsi.nfe_id = $1 AND nsi.company_id = $2` — same double-guard pattern already applied in `fiscal_execution.go` (both header AND item queries scoped by `company_id`); the new read/search/CSV handlers must replicate this, not just filter by `nfe_id` alone |
| SQL injection via search query text | Tampering | Parameterized `ILIKE '%'||$N||'%'` (never `fmt.Sprintf` the user's `q` into the query string) — exact precedent already in `admin_nf_cancelamento.go` line 67-69 |
| Privilege escalation — non-admin user calls the new endpoints directly (bypassing hidden nav/`AdminRoute`) | Elevation of Privilege | `withAuth(handler, "admin")` server-side role check on all 3 new handlers (see Pitfall 4 and V4 above) — this is the only real boundary, the frontend gates are cosmetic |

## Sources

### Primary (HIGH confidence — read directly from this codebase)
- `backend/migrations/147_fiscal_execution_items.sql` — exact "calculado" schema
- `backend/migrations/075_create_nfe_itens_tables.sql`, `094_add_cst_orig_fronteira.sql`, `095_add_cest_fronteira.sql`, `141_nfe_itens_st.sql`, `146_nfe_itens_desc_outro.sql` — exact "esperado" schema, built up across 5 migrations
- `backend/migrations/058_create_nfe_saidas.sql` — header table schema (search source)
- `backend/handlers/fiscal_execution.go` — exact `POST /api/fiscal/execute` contract, IDOR guard pattern, IBS/CBS field mapping (`ValorIbsUF`/`ValorIbsMUN`/`ValorCbs`)
- `backend/handlers/nfe_saidas.go` — confirmed `GET /api/nfe-saidas` has no número/chave search filter; confirmed `insertNFeItens` never populates item-level `v_ibs`/`v_cbs`
- `backend/handlers/admin_nf_cancelamento.go` — exact ILIKE server-side search precedent (for `nfe_entradas`, adapted for `nfe_saidas` in this phase)
- `backend/handlers/xml_conciliacao.go` — exact CSV export handler convention (`encoding/csv`, `Content-Disposition` header)
- `frontend/src/pages/ConciliacaoBridgeXML.tsx`, `ComparativoEFDvsXML.tsx` — binding visual/interaction precedent (per CONTEXT.md), read in full
- `frontend/src/pages/ConsultaNFeSaidas.tsx` — closest NF-e-saída data-shape precedent (confirmed: no número/chave search, no autocomplete)
- `frontend/src/components/FilialSelector.tsx` — confirmed `cmdk` Command/Popover primitives already vendored and in use (client-side filter only)
- `frontend/src/lib/exportToExcel.ts` — confirmed client-side `xlsx` export helper, zero backend dependency
- `frontend/src/pages/ImportarViaERP.tsx` (lines 150-203) — exact `useMutation` trigger-then-invalidate pattern precedent
- `frontend/src/App.tsx` (lines 1-84, 195-198) — confirmed `AdminRoute` implementation and route registration convention
- `frontend/src/lib/navigation.ts`, `frontend/src/components/AppSidebar.tsx` — confirmed `adminOnly` gate shape at both files, matches UI-SPEC's proposed code verbatim
- `backend/main.go` (lines 519-535, 1289-1318) — confirmed route registration conventions, `withAuth(..., "admin")` vs `withAuth(..., "")` usage
- `.planning/phases/12-tela-compara-o-fiscal-navega-o/12-UI-SPEC.md` — approved design contract (visual/interaction decisions, treated as locked)
- `.planning/phases/12-tela-compara-o-fiscal-navega-o/12-CONTEXT.md` — locked decisions D-01 through D-05
- `.planning/phases/11-motor-de-execu-o-do-pacote-fiscal-backend/11-05-SUMMARY.md` — Phase 11 handoff notes, confirms `fiscal_execution_items` is "ready to be read by Phase 12"
- `.planning/config.json` — confirmed `nyquist_validation: false` (Validation Architecture section omitted), no `security_enforcement` key present (Security Domain section included per absent=enabled rule)

### Secondary (MEDIUM confidence)
None — every claim in this research was verified by direct file inspection; no WebSearch was needed since this is a self-contained internal-codebase research task with zero new external libraries.

### Tertiary (LOW confidence)
None.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every library is already installed and its exact usage pattern was read from working code, not inferred
- Architecture: HIGH — schema and existing handler/route code was read directly; the 2 new patterns (search combobox, LEFT JOIN comparison query) are compositions of 2+ verified existing patterns each, not speculative
- Pitfalls: HIGH — Pitfall 1 (LEFT JOIN NULL status) and Pitfall 2 (IBS split) are derived directly from comparing the two migration files column-by-column, not guessed; Pitfall 3 (v_ibs/v_cbs never populated at item level) was confirmed by reading `insertNFeItens`'s actual INSERT column list

**Research date:** 2026-07-03
**Valid until:** No external time pressure — this research is based on this project's own frozen schema/code, not a fast-moving external library. Valid until Phase 11's schema/handlers are modified by a future phase (unlikely — Phase 11 is marked complete/closed).
