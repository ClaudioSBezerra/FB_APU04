# Phase 12: Tela Comparação Fiscal + Navegação - Pattern Map

**Mapped:** 2026-07-03
**Files analyzed:** 8 (3 new backend, 2 new frontend, 3 modified frontend config)
**Analogs found:** 8 / 8 (all files have at least a role-match analog; 2 are explicit compositions of 2 analogs each, called out below)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|--------------------|------|-----------|-----------------|----------------|
| `backend/handlers/fiscal_comparacao.go` (search handler) | controller (Go HTTP handler) | request-response (ILIKE search) | `backend/handlers/admin_nf_cancelamento.go` | exact (same ILIKE-search-by-param shape, different table) |
| `backend/handlers/fiscal_comparacao.go` (comparison-read handler) | controller (Go HTTP handler) | CRUD (read, LEFT JOIN) | `backend/handlers/fiscal_execution.go` (IDOR/company_id-scoped item query) + `backend/handlers/xml_conciliacao.go` (JSON list handler shape) | role-match, composed from 2 |
| `backend/handlers/fiscal_comparacao_csv.go` | controller (Go HTTP handler) | file-I/O (CSV streamed download) | `backend/handlers/xml_conciliacao.go` (`ConciliacaoCSVHandler`) | exact |
| `backend/main.go` (route registration, +3 lines) | config | request-response (route wiring) | `backend/main.go` lines 532/535 (`/api/fiscal/oracle-ping`, `/api/fiscal/execute`) | exact |
| `frontend/src/pages/ComparacaoFiscal.tsx` | component (page) | CRUD (fetch) + event-driven (trigger-then-reload mutation) | `frontend/src/pages/ConciliacaoBridgeXML.tsx` (table/badge/filter/export shell) + `frontend/src/pages/ImportarViaERP.tsx` (trigger mutation) + `frontend/src/pages/ConsultaNFeSaidas.tsx` (detail Dialog) | role-match, composed from 3 |
| `frontend/src/components/NfeSearchCombobox.tsx` | component | request-response (debounced server search) | `frontend/src/components/FilialSelector.tsx` (Command/Popover shell, in-memory filter today) | role-match (shell reused, data source swapped to debounced fetch — no exact debounced-server-search precedent exists in codebase) |
| `frontend/src/lib/navigation.ts` (+module entry) | config | — | `auditoria` module entry (single-tab shape, lines 66-71) | exact |
| `frontend/src/components/AppSidebar.tsx` (+section) | config | — | `"malha"` section (lines 113-123, `adminOnly: true` whole-section gate) | exact |
| `frontend/src/App.tsx` (+route, +import) | route | — | `/importacoes/erp-bridge-xml` route (line 197, `AdminRoute` wrapper) | exact |

## Pattern Assignments

### `backend/handlers/fiscal_comparacao.go` — search sub-handler (controller, request-response)

**Analog:** `backend/handlers/admin_nf_cancelamento.go` (GET branch, lines 22-92)

**Imports pattern** (lines 1-10):
```go
package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)
```

**Auth + IDOR pattern** (lines 26-37):
```go
claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
if !ok {
	jsonErr(w, http.StatusUnauthorized, "Unauthorized")
	return
}
userID, _ := claims["user_id"].(string)

companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
if err != nil {
	jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa: "+err.Error())
	return
}
```

**Core ILIKE-search pattern** (lines 42-92, adapt table `nfe_entradas`→`nfe_saidas`, columns `forn_cnpj/forn_nome/num_nota`→`numero_nfe/chave_nfe`):
```go
forn := strings.TrimSpace(r.URL.Query().Get("forn"))
numNota := strings.TrimSpace(r.URL.Query().Get("num_nota"))

type NFRow struct {
	ChaveNFe    string `json:"chave_nfe"`
	NumeroNFe   string `json:"numero_nfe"`
	DataEmissao string `json:"data_emissao"`
	FornNome    string `json:"forn_nome"`
}

rows, err := db.Query(`
	SELECT ne.chave_nfe, COALESCE(ne.numero_nfe, ''), ne.data_emissao::text, COALESCE(ne.forn_nome,'')
	FROM nfe_entradas ne
	WHERE ne.company_id = $1
	  AND ($2::text = '' OR COALESCE(ne.forn_cnpj,'') ILIKE '%'||$2||'%' OR COALESCE(ne.forn_nome,'') ILIKE '%'||$2||'%')
	  AND ($3::text = '' OR COALESCE(ne.numero_nfe,'') ILIKE '%'||$3||'%')
	ORDER BY ne.data_emissao DESC, ne.numero_nfe
	LIMIT 200
`, companyID, forn, numNota)
if err != nil {
	jsonErr(w, http.StatusInternalServerError, "Erro ao buscar NFs: "+err.Error())
	return
}
defer rows.Close()

result := []NFRow{}
for rows.Next() {
	var row NFRow
	if err := rows.Scan(&row.ChaveNFe, &row.NumeroNFe, &row.DataEmissao, &row.FornNome); err != nil {
		continue
	}
	result = append(result, row)
}
json.NewEncoder(w).Encode(map[string]interface{}{"rows": result, "count": len(result)})
```

**Adaptation notes for the new search handler:**
- Replace `nfe_entradas` with `nfe_saidas`; the `q` param (min 3 chars per RESEARCH.md Pattern 1) matches against `numero_nfe ILIKE` OR `chave_nfe ILIKE` (both columns, single param, per RESEARCH.md Pattern 2's exact SQL) rather than the two independent `forn`/`num_nota` params this analog uses — RESEARCH.md's Pattern 2 code block already has the exact query to copy verbatim.
- Return `[]interface{}{}` early when `len(q) < 3` (RESEARCH.md Pattern 2) instead of running the query — mirrors this analog's pattern of always returning a non-nil empty slice, never `null`.

---

### `backend/handlers/fiscal_comparacao.go` — comparison-read sub-handler (controller, CRUD read)

**Analog 1 (IDOR guard + item query shape):** `backend/handlers/fiscal_execution.go` lines 122-139, 150-159

**IDOR pattern to copy verbatim** (fiscal_execution.go lines 122-139):
```go
// Guard IDOR (T-11-14): a nota só é carregada se pertencer à company_id
// resolvida via JWT — nunca confiar em company_id vindo do corpo/cliente.
var emitCNPJ, emitUF, destUF, destCMun string
var dataEmissao time.Time
err = db.QueryRow(`
	SELECT emit_cnpj, COALESCE(emit_uf,''), COALESCE(dest_uf,''), COALESCE(dest_c_mun,''), data_emissao
	FROM nfe_saidas
	WHERE id = $1 AND company_id = $2`, req.NfeID, companyID,
).Scan(&emitCNPJ, &emitUF, &destUF, &destCMun, &dataEmissao)
if err == sql.ErrNoRows {
	jsonErr(w, http.StatusNotFound, "Nota não encontrada")
	return
}
```

**Item query double-guard pattern** (fiscal_execution.go lines 150-154 — the exact `WHERE nfe_id = $1 AND company_id = $2` double-scope the security-domain notes in RESEARCH.md flag as mandatory for the new LEFT JOIN query):
```go
itemRows, err := db.Query(`
	SELECT id, COALESCE(c_prod,''), COALESCE(cfop,''), COALESCE(v_prod,0), COALESCE(v_desc,0), COALESCE(v_outro,0), COALESCE(v_ipi,0)
	FROM nfe_saidas_itens
	WHERE nfe_id = $1 AND company_id = $2
	ORDER BY n_item ASC`, req.NfeID, companyID)
```

**Analog 2 (LEFT JOIN query + full field-pairing SQL — already fully written, copy from RESEARCH.md verbatim):** RESEARCH.md Pattern 2, `FiscalComparacaoReadHandler` code block (`.planning/phases/12-tela-compara-o-fiscal-navega-o/12-RESEARCH.md` lines 291-317) — the exact `SELECT ... LEFT JOIN fiscal_execution_items fei ON fei.nfe_item_id = nsi.id WHERE nsi.nfe_id = $1 AND nsi.company_id = $2` query with all 6-tax columns is already drafted there; use it as the base and add `COALESCE(fei.status, 'not_executed')` per Pitfall 1.

**Field-pairing table (READ FROM, do not re-derive — this is the authoritative column mapping):** RESEARCH.md Pattern 3, table at lines 323-334. Key points to carry into the Go struct + SQL:
- ICMS/ICMS-ST/PIS/COFINS: direct 1:1 column pairs (`v_bc_icms`↔`base_calculo_icms`, `v_icms`↔`valor_icms`, `v_bc_st`↔`base_substituicao`, `v_st`↔`valor_substituicao`, `v_bc_pis`↔`base_calculo_pis`, `v_pis`↔`valor_pis`, `v_bc_cofins`↔`base_calculo_cofins`, `v_cofins`↔`valor_cofins`).
- IBS: `v_ibs` (single column) vs. `valor_ibs_uf + valor_ibs_mun` (**must SUM in SQL** — no total column exists; do this once server-side, reused by both JSON and CSV handlers, per RESEARCH.md Pitfall 2).
- CBS: `v_cbs` ↔ `valor_cbs`, direct 1:1.
- "Só calculado" fields (no expected-side counterpart, dialog-only per UI-SPEC): `percentual_difal`, `valor_icms_partilha_destino`, `valor_icms_pobreza`, `grupo_fiscal_codigo`.

**Error handling pattern** (xml_conciliacao.go lines 201-249, `ConciliacaoHandler` — same shape for both new JSON handlers):
```go
func ConciliacaoHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			jsonErr(w, http.StatusMethodNotAllowed, "Método não permitido")
			return
		}
		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			jsonErr(w, http.StatusUnauthorized, "Não autenticado")
			return
		}
		userID, _ := claims["user_id"].(string)
		companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa: "+err.Error())
			return
		}
		// ... query, scan, always default to []T{} not nil before Encode
	}
}
```

---

### `backend/handlers/fiscal_comparacao_csv.go` (controller, file-I/O)

**Analog:** `backend/handlers/xml_conciliacao.go` `ConciliacaoCSVHandler` (lines 310-386)

**Full pattern to copy (headers + csv.Writer + row loop + flush):**
```go
w.Header().Set("Content-Type", "text/csv; charset=utf-8")
w.Header().Set("Content-Disposition", `attachment; filename="conciliacao-bridge-xml.csv"`) // rename to comparacao-fiscal.csv

cw := csv.NewWriter(w)

header := []string{
	"Chave NF-e", "CNPJ Fornecedor", "Fornecedor", "Mês/Ano", "Data Emissão", "CFOP",
	"PIS XML", "PIS Bridge", "Delta PIS",
	// ... (adapt to the 6-tax × 3-column set from Pattern 3, per UI-SPEC scope decision:
	// only the visible table's 6 taxes × esperado/calculado/diferença — NOT DIFAL/FCP,
	// per RESEARCH.md Open Question 2 recommendation)
}
if err := cw.Write(header); err != nil {
	log.Printf("[FiscalComparacao/CSV] write header error: %v", err)
	return
}
for _, row := range data {
	record := []string{ /* ... fmt.Sprintf("%.2f", ...) per numeric field ... */ }
	if err := cw.Write(record); err != nil {
		log.Printf("[FiscalComparacao/CSV] write row error: %v", err)
		return
	}
}
cw.Flush()
if err := cw.Error(); err != nil {
	log.Printf("[FiscalComparacao/CSV] flush error: %v", err)
}
```

**Note:** Compute the IBS sum (`valor_ibs_uf + valor_ibs_mun`) in the SAME shared query function used by the JSON read handler — do not duplicate the arithmetic (RESEARCH.md Pitfall 2, "decide once ... rather than duplicating in frontend AND CSV handler").

---

### `backend/main.go` (config, route registration)

**Analog:** lines 532, 535 (existing `admin`-gated fiscal routes) + line 1305-1306 (existing CSV+JSON pair pattern):
```go
http.HandleFunc("/api/fiscal/oracle-ping", withAuth(handlers.FiscalOraclePingHandler, "admin"))
http.HandleFunc("/api/fiscal/execute",     withAuth(handlers.FiscalExecutionRunHandler, "admin"))
```

**Add (same block, all 3 as `"admin"` — never `""`, per RESEARCH.md Pitfall 4/V4):**
```go
http.HandleFunc("/api/fiscal/comparacao/search", withAuth(handlers.FiscalComparacaoSearchHandler, "admin"))
http.HandleFunc("/api/fiscal/comparacao",        withAuth(handlers.FiscalComparacaoReadHandler, "admin"))
http.HandleFunc("/api/fiscal/comparacao/csv",    withAuth(handlers.FiscalComparacaoCSVHandler, "admin"))
```

---

### `frontend/src/pages/ComparacaoFiscal.tsx` (component, CRUD + event-driven)

**Analog 1 (page shell — heading/tooltip, summary cards, filter row, dense table, badges, export buttons, empty/error states):** `frontend/src/pages/ConciliacaoBridgeXML.tsx`, full file read (511 lines).

**Header + HelpCircle tooltip pattern** (lines 195-230):
```tsx
<div>
  <div className="flex items-center gap-2">
    <h1 className="text-xl font-semibold">Conciliação Bridge vs XML</h1>
    <TooltipProvider delayDuration={200}>
      <Tooltip>
        <TooltipTrigger asChild>
          <HelpCircle className="h-4 w-4 text-muted-foreground hover:text-foreground cursor-help shrink-0" />
        </TooltipTrigger>
        <TooltipContent side="right" className="max-w-sm text-xs space-y-2 p-3">
          {/* ... explanation ... */}
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  </div>
  <p className="text-sm text-muted-foreground mt-1">{/* subtitle */}</p>
</div>
```
Use UI-SPEC copy verbatim: H1 "Comparação Fiscal", subtitle "Compare o valor esperado (XML da nota) com o valor calculado pelo pacote fiscal (PKG_FISCAL_FCTAX) para ICMS, ICMS-ST, PIS, COFINS, IBS e CBS."

**Summary cards pattern (adapt from 3-card to 4-card grid per UI-SPEC)** (lines 233-266):
```tsx
<div className="grid grid-cols-1 sm:grid-cols-4 gap-4">
  <Card>
    <CardHeader className="pb-2">
      <CardTitle className="text-sm font-semibold text-muted-foreground">Total de Itens</CardTitle>
    </CardHeader>
    <CardContent>
      <p className="text-xl font-semibold">{loading ? '...' : total}</p>
    </CardContent>
  </Card>
  {/* repeat for "Sem Divergência", "Divergentes", "Não Calculados" */}
</div>
```

**Divergence Badge pattern (zero-tolerance variant — change `> 0.01` to `!= 0`)** (lines 369-378, this project's `ConciliacaoBridgeXML.tsx` uses R$0.01 tolerance — **UI-SPEC explicitly forbids reusing that threshold for this screen**):
```tsx
<Badge
  variant="outline"
  className={diferenca !== 0
    ? 'text-[10px] px-1.5 py-0 bg-red-50 text-red-700 border-red-200'
    : 'text-[10px] px-1.5 py-0 bg-gray-50 text-muted-foreground'}
>
  {fmtBRL(diferenca)}
</Badge>
```
**Adaptation:** replace `row.delta_pis > 0.01` (ERP-vs-XML tolerance) with `Math.abs(esperado - calculado) !== 0` (fiscal-package zero-tolerance rule, UI-SPEC "Divergence Visual Rules" #1). Row-level tint pattern (line 353, `row.delta_total > 0.01 ? 'bg-red-50 hover:bg-red-100' : ''`) must be adapted the same way, AND must first check `status !== 'ok'` → render `bg-slate-50` "Não calculado" tint instead (never evaluate divergence for non-`ok` rows, UI-SPEC rule #2).

**Export buttons pattern** (lines 416-428):
```tsx
<div className="flex items-center gap-2 mt-4 no-print">
  <Button size="sm" variant="outline" onClick={handleExportExcel}>
    <FileSpreadsheet className="w-4 h-4 mr-1" /> Exportar Excel
  </Button>
  <Button size="sm" variant="outline" onClick={handleExportCSV} disabled={downloadingCSV}>
    <Download className="w-4 h-4 mr-1" />
    {downloadingCSV ? 'Exportando...' : 'Exportar CSV'}
  </Button>
</div>
```

**Excel export handler pattern** (lines 137-161, `handleExportExcel` — column-mapping object shape to copy, replacing the 4-column Bridge/XML pairs with the 6-tax esperado/calculado/diferença set):
```tsx
const handleExportExcel = () => {
  if (!rows) return;
  const data = rows.map(r => ({
    'Chave NF-e': r.chave_nfe,
    'Produto': r.x_prod,
    'ICMS Esperado': r.v_icms ?? 0,
    'ICMS Calculado': r.valor_icms ?? 0,
    'ICMS Diferença': (r.v_icms ?? 0) - (r.valor_icms ?? 0),
    // ... repeat per tax
  }));
  exportToExcel(data, `comparacao-fiscal-${nfeId}`, 'Comparação');
  toast.success('Excel exportado com sucesso');
};
```

**CSV export handler pattern (blob download)** (lines 163-183):
```tsx
const handleExportCSV = async () => {
  setDownloadingCSV(true);
  try {
    const res = await fetch(`/api/fiscal/comparacao/csv?nfe_id=${nfeId}`, { headers: authHeaders });
    if (!res.ok) throw new Error(`Erro ${res.status}`);
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `comparacao-fiscal.csv`;
    a.click();
    URL.revokeObjectURL(url);
    toast.success('CSV exportado com sucesso');
  } catch (err) {
    toast.error('Erro ao exportar: ' + (err instanceof Error ? err.message : 'Desconhecido'));
  } finally {
    setDownloadingCSV(false);
  }
};
```

**Empty/error state copy pattern** (lines 313-328) — adapt text per UI-SPEC Copywriting Contract table (page title, empty states, error state, "Não calculado" badge label all pre-written there, use verbatim).

---

**Analog 2 (trigger-then-reload mutation — D-01's core requirement):** `frontend/src/pages/ImportarViaERP.tsx` lines 188-203

**Exact pattern to copy** (adapt endpoint/body/invalidate key):
```tsx
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
Button usage pattern (ImportarViaERP.tsx lines 242-247, spinner-in-button loading convention, matches CONTEXT.md's Claude's Discretion note "usar convenção já estabelecida no UI-SPEC — spinner + texto"):
```tsx
<Button onClick={() => executar.mutate(nfeId)} disabled={!nfeId || executar.isPending}>
  {executar.isPending
    ? <Loader2 className="h-4 w-4 mr-1.5 animate-spin" />
    : <Send className="h-4 w-4 mr-1.5" />}
  Executar
</Button>
```

---

**Analog 3 (per-row detail Dialog — "Ver detalhes" / 3-section breakdown):** `frontend/src/pages/ConsultaNFeSaidas.tsx` lines 108-205 (`DetalheNFe` component)

**Dialog shell + section/line-item sub-component pattern to copy verbatim (structure), adapt content to the 3 UI-SPEC sections (Identificação / Comparação / Só calculado):**
```tsx
function DetalheItem({ item, onClose }: { item: ComparacaoRow; onClose: () => void }) {
  const Linha = ({ label, value }: { label: string; value: string | number | null | undefined }) => (
    <div className="flex justify-between py-0.5 border-b border-dashed last:border-0">
      <span className="text-[11px] text-muted-foreground w-36 shrink-0">{label}</span>
      <span className="text-[11px] font-medium text-right">{value ?? '—'}</span>
    </div>
  );
  const Secao = ({ title, children }: { title: string; children: React.ReactNode }) => (
    <div className="mb-2">
      <h3 className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground mb-1 pb-0.5 border-b">
        {title}
      </h3>
      {children}
    </div>
  );

  return (
    <Dialog open onOpenChange={onClose}>
      <DialogContent className="max-w-2xl max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="text-xs">
            Item {item.n_item} — {item.x_prod}
          </DialogTitle>
        </DialogHeader>
        <div className="space-y-1 mt-1">
          <Secao title="Identificação">{/* chave, item, produto, NCM, CFOP */}</Secao>
          <Secao title="Comparação">{/* esperado/calculado/diferença, all 6 taxes, full precision */}</Secao>
          <Secao title="Só calculado">
            <p className="text-[10px] text-muted-foreground italic mb-1">
              Campos abaixo só existem no retorno do pacote fiscal — sem valor esperado correspondente no XML
            </p>
            {/* DIFAL %, valor_icms_partilha_destino, valor_icms_pobreza */}
          </Secao>
        </div>
      </DialogContent>
    </Dialog>
  );
}
```
Row trigger pattern (ConsultaNFeSaidas.tsx line 449): `onClick={() => setSelected(row)}` on the trailing "Ver detalhes" icon button (per UI-SPEC screen layout item 5).

---

### `frontend/src/components/NfeSearchCombobox.tsx` (component, request-response)

**Analog:** `frontend/src/components/FilialSelector.tsx` (Command/Popover shell — currently filters an in-memory array, must be swapped to a debounced `useQuery` against the new search endpoint).

**Full working code already drafted in RESEARCH.md Pattern 1** (`.planning/phases/12-tela-compara-o-fiscal-navega-o/12-RESEARCH.md` lines 164-241) — copy verbatim, this is the authoritative implementation:
```tsx
import { useState, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Command, CommandInput, CommandList, CommandEmpty, CommandGroup, CommandItem } from '@/components/ui/command';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Button } from '@/components/ui/button';
import { Search } from 'lucide-react';
import { useAuth } from '@/contexts/AuthContext';

export function NfeSearchCombobox({ onSelect }: { onSelect: (nfe: NfeSearchResult) => void }) {
  const { token, companyId } = useAuth();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [debounced, setDebounced] = useState('');

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
    enabled: debounced.length >= 3,
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
                  <span className="text-xs">Nº {nfe.numero_nfe}/{nfe.serie} — {nfe.dest_nome} — {nfe.data_emissao}</span>
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
**Key deviation from `FilialSelector.tsx`:** pass `shouldFilter={false}` to `<Command>` (FilialSelector.tsx omits this because it relies on `cmdk`'s built-in client-side text filter over an already-loaded array — this new component must disable that built-in filter since filtering now happens server-side via the debounced `useQuery`).

---

### `frontend/src/lib/navigation.ts` (config)

**Analog:** `auditoria` module entry (lines 66-71):
```ts
auditoria: {
  label: 'Auditoria Fiscal — EFD ICMS/IPI × Guias',
  tabs: [
    { label: 'EFD ICMS/IPI × Guias', path: '/auditoria-efd' },
  ],
},
```

**Add (UI-SPEC exact code, already written):**
```ts
pacotefiscal: {
  label: 'Teste Pacote Fiscal',
  tabs: [
    { label: 'Comparação Fiscal', path: '/pacote-fiscal/comparacao' },
  ],
},
```
And extend `getActiveModule()` (pattern at lines 89-106, insert alongside `if (pathname.startsWith('/auditoria-efd')) return 'auditoria'` at line 104):
```ts
if (pathname.startsWith('/pacote-fiscal')) return 'pacotefiscal'
```

---

### `frontend/src/components/AppSidebar.tsx` (config)

**Analog:** `"malha"` section (lines 113-123, whole-section `adminOnly` gate, single-item pattern):
```ts
{
  id: "malha",
  title: "Malha Fina",
  sectionIcon: SearchX,
  adminOnly: true,
  items: [
    { title: "NF-e Entradas",   url: "/malha-fina/nfe-entradas", icon: FileText },
    { title: "CT-e",            url: "/malha-fina/cte",          icon: Truck },
    { title: "NFS-e Entradas",  url: "#",                        icon: FileText, disabled: true },
  ],
},
```

**Add (UI-SPEC exact code, add to `sections` array, requires importing `FlaskConical`/`GitCompare` from `lucide-react`):**
```ts
{
  id: "pacotefiscal",
  title: "Teste Pacote Fiscal",
  sectionIcon: FlaskConical,
  adminOnly: true,
  items: [
    { title: "Comparação Fiscal", url: "/pacote-fiscal/comparacao", icon: GitCompare },
  ],
},
```
`NavSection`/`NavItem` types (already declared, lines 65-80) require no changes — `adminOnly?: boolean` already exists at both levels. Section-level gating logic already implemented at lines 273-275 (`if (section.adminOnly && !isAdmin) return null`).

---

### `frontend/src/App.tsx` (route)

**Analog:** `/importacoes/erp-bridge-xml` route registration (line 197) + `AdminRoute` wrapper definition (lines 77-84):
```tsx
function AdminRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, loading, user } = useAuth()
  const location = useLocation()
  if (loading) return null
  if (!isAuthenticated) return <Navigate to="/login" state={{ from: location }} replace />
  if (user?.role !== 'admin') return <Navigate to="/" replace />
  return <>{children}</>
}
// ...
<Route path="/importacoes/erp-bridge-xml" element={<AdminRoute><ImportarViaERP /></AdminRoute>} />
```

**Add:**
- Import (alongside line 30 `import ImportarViaERP from './pages/ImportarViaERP'`): `import ComparacaoFiscal from './pages/ComparacaoFiscal'`
- Route (same block as lines 195-198):
```tsx
<Route path="/pacote-fiscal/comparacao" element={<AdminRoute><ComparacaoFiscal /></AdminRoute>} />
```

---

## Shared Patterns

### Authentication / IDOR guard (backend)
**Source:** `backend/handlers/fiscal_execution.go` lines 100-112, 122-139; `backend/handlers/admin_nf_cancelamento.go` lines 26-37
**Apply to:** All 3 new Go handlers (`FiscalComparacaoSearchHandler`, `FiscalComparacaoReadHandler`, `FiscalComparacaoCSVHandler`)
```go
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
// every subsequent query MUST include `AND ... company_id = $N` — never trust
// a bare nfe_id from the client (T-11-14 IDOR guard pattern, RESEARCH.md V4/IDOR row).
```
Register all 3 with `withAuth(handler, "admin")` — never `""` (RESEARCH.md Pitfall 4).

### Divergence rule (frontend, zero-tolerance — NOT the same as sibling screens)
**Source:** UI-SPEC "Divergence Visual Rules" (binding), contrasted with `ConciliacaoBridgeXML.tsx` line 353/372 (`> 0.01` tolerance — do NOT reuse for this screen)
**Apply to:** `ComparacaoFiscal.tsx` table cell/row rendering, CSV/Excel export delta columns
```ts
const diferenca = Math.abs((esperado ?? 0) - (calculado ?? 0));
const divergente = diferenca !== 0; // zero tolerance — no `> 0.01` check
```
Row-level status precedence: if `status !== 'ok'` (or `status == null`, "never executed") → render "Não calculado" (`bg-slate-100 text-slate-600`), regardless of any per-tax diff — never evaluate divergence for these rows (UI-SPEC rule #2, RESEARCH.md Pitfall 1).

### Trigger-then-reload mutation (frontend)
**Source:** `frontend/src/pages/ImportarViaERP.tsx` lines 188-203 (`trigger` useMutation)
**Apply to:** `ComparacaoFiscal.tsx`'s "Executar" button (D-01)
```tsx
onSuccess: (summary, nfeId) => {
  toast.success(`Execução concluída: ${summary.ok}/${summary.total} OK.`);
  queryClient.invalidateQueries({ queryKey: ['fiscal-comparacao', nfeId] });
},
```

### Excel export (frontend, zero backend work)
**Source:** `frontend/src/lib/exportToExcel.ts` (8 lines, full file — client-side `xlsx`)
```ts
import * as XLSX from 'xlsx';
export function exportToExcel(data: Record<string, unknown>[], fileName: string, sheetName: string = 'Dados') {
  const ws = XLSX.utils.json_to_sheet(data);
  const wb = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(wb, ws, sheetName);
  XLSX.writeFile(wb, `${fileName}.xlsx`);
}
```
**Apply to:** `ComparacaoFiscal.tsx`'s "Exportar Excel" button — import directly, build a column-mapping object per row (see `ConciliacaoBridgeXML.tsx` `handleExportExcel`, lines 137-161, for the object shape convention).

### CSV export (backend, `encoding/csv` streamed)
**Source:** `backend/handlers/xml_conciliacao.go` `ConciliacaoCSVHandler` (lines 310-386)
**Apply to:** `FiscalComparacaoCSVHandler` — `Content-Type: text/csv; charset=utf-8` + `Content-Disposition: attachment` headers, `csv.NewWriter(w)`, PT-BR header row, `fmt.Sprintf("%.2f", ...)` per numeric cell, `cw.Flush()` + `cw.Error()` check at the end.

### Dense comparison-table styling (frontend)
**Source:** UI-SPEC "Spacing Scale" + `ConciliacaoBridgeXML.tsx` table (lines 330-413)
**Apply to:** `ComparacaoFiscal.tsx`'s item table — `text-[11px]` cell content, `py-1 px-2` cell padding, `py-1.5 px-2 text-[11px]` header cells, `bg-muted/30` header row background, `overflow-x-auto rounded-md border` table container. Do not loosen to standard `md` padding — UI-SPEC explicitly calls this out as required density given ~18 numeric columns per row.

### Navigation adminOnly gate (frontend, 3-file wiring)
**Source:** `frontend/src/lib/navigation.ts` (`auditoria` entry) + `frontend/src/components/AppSidebar.tsx` (`"malha"` section) + `frontend/src/App.tsx` (`AdminRoute` + `/importacoes/erp-bridge-xml` route)
**Apply to:** TPF-08 nav entry — exact code already drafted in UI-SPEC "Navigation Entry" section (lines 142-178), copy verbatim into the 3 files.

## No Analog Found

None — every file has at least a role-match analog. The two components with no *exact* precedent (`NfeSearchCombobox.tsx` for debounced server-side search, and the LEFT JOIN comparison-read query) are explicitly documented above as compositions of 2 existing, verified patterns each — RESEARCH.md already drafted full working code for both, treat those code blocks as the primary source, this file's excerpts as the secondary/supporting analog evidence.

## Metadata

**Analog search scope:** `backend/handlers/` (admin_nf_cancelamento.go, xml_conciliacao.go, fiscal_execution.go), `frontend/src/pages/` (ConciliacaoBridgeXML.tsx, ComparativoEFDvsXML.tsx, ImportarViaERP.tsx, ConsultaNFeSaidas.tsx), `frontend/src/components/` (FilialSelector.tsx, AppSidebar.tsx), `frontend/src/lib/` (navigation.ts, exportToExcel.ts), `frontend/src/App.tsx`, `backend/main.go` (route registration block)
**Files scanned:** 12 read in full or targeted sections; 0 files > 2,000 lines encountered (largest was `backend/main.go` at 1,449 lines, read via targeted `grep` + offset/limit, not loaded whole)
**Pattern extraction date:** 2026-07-03
