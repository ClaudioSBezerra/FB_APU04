# Phase 7: Módulos 1.x — Exposição Tributária Direta — Pattern Map

**Mapped:** 2026-05-22
**Files analyzed:** 7 (1 new Go file, 4 new React pages, 2 modified files)
**Analogs found:** 7 / 7

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `backend/handlers/reforma_modulo1.go` | handler (multi) | request-response | `backend/handlers/creditos_perdidos.go` | exact |
| `backend/main.go` | config/router | request-response | `backend/main.go` lines 523-538 | exact |
| `frontend/src/pages/Reforma11CreditosBloqueados.tsx` | component/page | request-response | `frontend/src/pages/ConciliacaoBridgeXML.tsx` | exact |
| `frontend/src/pages/Reforma13RankingFornecedores.tsx` | component/page | request-response | `frontend/src/pages/ConciliacaoBridgeXML.tsx` | exact |
| `frontend/src/pages/Reforma12Reprecificacao.tsx` | component/page | request-response | `frontend/src/pages/ConciliacaoBridgeXML.tsx` | role-match |
| `frontend/src/pages/Reforma14SplitPayment.tsx` | component/page | request-response | `frontend/src/pages/ReformaParametros.tsx` | role-match |
| `frontend/src/lib/navigation.ts` | config | — | `frontend/src/lib/navigation.ts` lines 49-58 | exact |
| `frontend/src/App.tsx` | router | — | `frontend/src/App.tsx` lines 174-176 | exact |

---

## Pattern Assignments

### `backend/handlers/reforma_modulo1.go` (handler, request-response)

**Analog:** `backend/handlers/creditos_perdidos.go`

**Imports pattern** (lines 1-10):
```go
package handlers

import (
    "database/sql"
    "encoding/csv"
    "encoding/json"
    "log"
    "net/http"

    "github.com/golang-jwt/jwt/v5"
)
```

**Auth + companyID pattern** (lines 93-113 of creditos_perdidos.go):
```go
func CreditosBloqueadosHandler(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")

        if r.Method != http.MethodGet {
            jsonErr(w, http.StatusMethodNotAllowed, "Method not allowed")
            return
        }

        claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
        if !ok {
            jsonErr(w, http.StatusUnauthorized, "Unauthorized")
            return
        }
        userID := claims["user_id"].(string)

        companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
        if err != nil {
            jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa: "+err.Error())
            return
        }
```

**reforma_parametros read with sql.ErrNoRows guard** (lines 43-55 of reforma_config.go):
```go
var p ReformaParametros
err = db.QueryRow(`
    SELECT company_id, target_ano, aliq_ibs_pct, aliq_cbs_pct,
           fator_simples_pct, taxa_cdi_anual_pct, prazo_medio_dias
    FROM reforma_parametros
    WHERE company_id = $1
`, companyID).Scan(&p.CompanyID, &p.TargetAno, &p.AliqIBSPct, &p.AliqCBSPct,
    &p.FatorSimplesPct, &p.TaxaCDIAnualPct, &p.PrazoMedioDias)

if err == sql.ErrNoRows {
    // Use defaults when company has not configured parameters
    p.AliqIBSPct = 26.5
    p.AliqCBSPct = 9.9
    p.FatorSimplesPct = 20.0
    p.TaxaCDIAnualPct = 10.5
    p.PrazoMedioDias = 30
}
if err != nil && err != sql.ErrNoRows {
    http.Error(w, "Erro ao ler parâmetros: "+err.Error(), http.StatusInternalServerError)
    return
}
```

**Multi-query core pattern — rows scan + nil-guard** (lines 143-183 of creditos_perdidos.go):
```go
rows, err := db.Query(`...`, companyID)
if err != nil {
    log.Printf("Handler query error: %v", err)
    jsonErr(w, http.StatusInternalServerError, "Erro ao consultar dados")
    return
}
defer rows.Close()

var list []MyRow
for rows.Next() {
    var r MyRow
    if err := rows.Scan(&r.FieldA, &r.FieldB, &r.FieldC); err != nil {
        continue
    }
    // derived fields calculated in Go
    r.IBSEstimado = r.ValorTotal * (ibsRate / 100.0)
    r.CBSEstimado = r.ValorTotal * (cbsRate / 100.0)
    list = append(list, r)
}

if list == nil {
    list = []MyRow{} // always return empty array, never null
}

json.NewEncoder(w).Encode(resp)
```

**CSV handler pattern** (lines 310-386 of xml_conciliacao.go):
```go
func CreditosBloqueadosCSVHandler(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodGet {
            http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
            return
        }

        claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
        if !ok {
            http.Error(w, "Não autenticado", http.StatusUnauthorized)
            return
        }
        userID, _ := claims["user_id"].(string)

        companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
        if err != nil {
            http.Error(w, "Erro ao obter empresa: "+err.Error(), http.StatusInternalServerError)
            return
        }

        // ... run same query as JSON handler ...

        w.Header().Set("Content-Type", "text/csv; charset=utf-8")
        w.Header().Set("Content-Disposition", `attachment; filename="creditos-icms-bloqueados.csv"`)

        cw := csv.NewWriter(w)
        header := []string{"Tipo CFOP", "CFOP", "ICMS Bloqueado (R$)", "VL Operações (R$)", "IBS Equiv. (R$)", "CBS Equiv. (R$)", "Qtd Registros"}
        if err := cw.Write(header); err != nil {
            log.Printf("[CSV] write header error: %v", err)
            return
        }

        for _, row := range data {
            record := []string{
                row.TipoCFOP, row.CFOP,
                fmt.Sprintf("%.2f", row.VlIcmsTotal),
                fmt.Sprintf("%.2f", row.VlOprTotal),
                fmt.Sprintf("%.2f", row.IBSEquiv),
                fmt.Sprintf("%.2f", row.CBSEquiv),
                fmt.Sprintf("%d", row.QtdRegistros),
            }
            if err := cw.Write(record); err != nil {
                log.Printf("[CSV] write row error: %v", err)
                return
            }
        }

        cw.Flush()
        if err := cw.Error(); err != nil {
            log.Printf("[CSV] flush error: %v", err)
        }
    }
}
```

**Key SQL join chain for reg_c190 (no company_id directly):**
```sql
-- Pattern confirmed: migrations/043 (mv_operacoes_simples)
FROM reg_c190 c190
JOIN reg_c100 c100 ON c100.id = c190.id_pai_c100
JOIN import_jobs j  ON j.id = c100.job_id
LEFT JOIN cfop cf   ON cf.cfop = c190.cfop
WHERE j.company_id = $1
  AND c100.cod_sit NOT IN ('02','03','04','05')
  AND COALESCE(cf.tipo, 'O') != 'T'
```

**Key SQL join for nfe_entradas (has company_id directly):**
```sql
-- Pattern confirmed: creditos_perdidos.go lines 144-157
FROM nfe_entradas ne
JOIN forn_simples fs ON fs.cnpj = ne.forn_cnpj
WHERE ne.company_id = $1
  AND ne.cancelado = 'N'
```

**CNPJ normalization join for forn_simples** (research pitfall 5):
```sql
JOIN forn_simples fs ON fs.cnpj = REGEXP_REPLACE(p.cnpj, '[^0-9]', '', 'g')
```

**Struct declaration pattern** (lines 16-88 of creditos_perdidos.go):
```go
// One response struct per handler, grouped at top of file
type Modulo11Row struct {
    TipoCFOP     string  `json:"tipo_cfop"`
    CFOP         string  `json:"cfop"`
    VlIcmsTotal  float64 `json:"vl_icms_total"`
    VlOprTotal   float64 `json:"vl_opr_total"`
    IBSEquiv     float64 `json:"ibs_equiv"`
    CBSEquiv     float64 `json:"cbs_equiv"`
    QtdRegistros int     `json:"qtd_registros"`
}

type Modulo11Response struct {
    Rows        []Modulo11Row `json:"rows"`
    TotalIcms   float64       `json:"total_icms"`
    TotalIBS    float64       `json:"total_ibs"`
    TotalCBS    float64       `json:"total_cbs"`
}
```

---

### `backend/main.go` (config/router — modification only)

**Analog:** `backend/main.go` lines 523-538 (reforma parametros block)

**Route registration pattern** (lines 523-538):
```go
// ── Reforma Tributária — Módulos 1.x ──
http.HandleFunc("/api/reforma/modulo1/creditos", func(w http.ResponseWriter, r *http.Request) {
    database := getDB()
    if database == nil {
        jsonServiceUnavailable(w)
        return
    }
    handlers.AuthMiddleware(handlers.CreditosBloqueadosHandler(database), "")(w, r)
})
http.HandleFunc("/api/reforma/modulo1/creditos/csv", func(w http.ResponseWriter, r *http.Request) {
    database := getDB()
    if database == nil {
        jsonServiceUnavailable(w)
        return
    }
    handlers.AuthMiddleware(handlers.CreditosBloqueadosCSVHandler(database), "")(w, r)
})
// Repeat pattern for: ranking, reprecificacao, split (split has no /csv route per UI-SPEC)
```

Note: all 4 JSON handlers use role `""` (any authenticated user). CSV handlers also role `""`. The PUT reforma/parametros uses `"admin"` — that precedent does NOT apply here (read-only analytics).

---

### `frontend/src/pages/Reforma11CreditosBloqueados.tsx` (component/page, request-response)

**Analog:** `frontend/src/pages/ConciliacaoBridgeXML.tsx`

**Imports pattern** (lines 1-35 of ConciliacaoBridgeXML.tsx):
```typescript
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import {
  BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer,
} from 'recharts'
import { Download, AlertTriangle, Info } from 'lucide-react'
import {
  Tooltip as UITooltip, TooltipContent, TooltipProvider, TooltipTrigger,
} from '@/components/ui/tooltip'
```

Note: recharts `Tooltip` and shadcn `Tooltip` conflict — alias one (as shown above with `Tooltip as UITooltip`).

**Type definition pattern** (lines 40-61 of ConciliacaoBridgeXML.tsx):
```typescript
interface Modulo11Row {
  tipo_cfop: string
  cfop: string
  vl_icms_total: number
  vl_opr_total: number
  ibs_equiv: number
  cbs_equiv: number
  qtd_registros: number
}

interface Modulo11Response {
  rows: Modulo11Row[]
  total_icms: number
  total_ibs: number
  total_cbs: number
}
```

**Helper functions pattern** (lines 74-87 of ConciliacaoBridgeXML.tsx):
```typescript
function fmtBRL(v: number | null | undefined): string {
  if (v == null) return '—'
  return v.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' })
}

function fmtCNPJ(v: string): string {
  if (!v || v.length !== 14) return v || '—'
  return `${v.slice(0, 2)}.${v.slice(2, 5)}.${v.slice(5, 8)}/${v.slice(8, 12)}-${v.slice(12)}`
}
```

**useQuery data fetch pattern** (lines 98-126 of ConciliacaoBridgeXML.tsx):
```typescript
const {
  data,
  isLoading,
  isError,
} = useQuery<Modulo11Response>({
  queryKey: ['reforma/modulo1/creditos'],
  queryFn: async () => {
    const res = await fetch('/api/reforma/modulo1/creditos')
    if (!res.ok) throw new Error(`Erro ${res.status}`)
    return res.json()
  },
})
```

**CSV download pattern** (lines 163-183 of ConciliacaoBridgeXML.tsx):
```typescript
const [downloadingCSV, setDownloadingCSV] = useState(false)

const handleExportCSV = async () => {
  setDownloadingCSV(true)
  try {
    const res = await fetch('/api/reforma/modulo1/creditos/csv')
    if (!res.ok) throw new Error(`Erro ${res.status}`)
    const blob = await res.blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'creditos-icms-bloqueados.csv'
    a.click()
    URL.revokeObjectURL(url)
  } catch (err) {
    // handle error (toast or alert)
  } finally {
    setDownloadingCSV(false)
  }
}
```

**Page layout pattern** (derived from UI-SPEC Page Layout Contract):
```tsx
export default function Reforma11CreditosBloqueados() {
  return (
    <div className="space-y-6 p-6">
      {/* Page header row */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold">Créditos ICMS Bloqueados — Módulo 1.1</h1>
          <p className="text-sm text-muted-foreground">
            Créditos ICMS não aproveitáveis na transição + equivalente IBS/CBS recuperável
          </p>
        </div>
        <Button
          variant={data && data.rows.length > 0 ? 'default' : 'outline'}
          size="sm"
          onClick={handleExportCSV}
          disabled={downloadingCSV || !data || data.rows.length === 0}
          aria-label="Exportar tabela de créditos como CSV"
        >
          <Download className="mr-2 h-4 w-4" />
          Exportar CSV
        </Button>
      </div>
      <Separator />

      {/* KPI summary — 3 cols */}
      <div className="grid grid-cols-3 gap-4">
        <Card><CardHeader><CardTitle className="text-sm text-muted-foreground">Total ICMS Bloqueado</CardTitle></CardHeader>
          <CardContent><p className="text-2xl font-semibold">{fmtBRL(data?.total_icms)}</p></CardContent>
        </Card>
        {/* repeat for IBS, CBS */}
      </div>

      {/* Main card: chart + table */}
      <Card>
        <CardHeader><CardTitle className="text-base font-semibold">Créditos por CFOP</CardTitle></CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-2">
              {Array.from({ length: 6 }).map((_, i) => <Skeleton key={i} className="h-8 w-full" />)}
            </div>
          ) : isError ? (
            <Alert variant="destructive">
              <AlertDescription>Erro ao carregar dados. Verifique sua conexão e tente novamente.</AlertDescription>
            </Alert>
          ) : !data || data.rows.length === 0 ? (
            <p className="text-sm text-muted-foreground text-center py-8">
              Nenhum crédito ICMS encontrado para o período selecionado.
            </p>
          ) : (
            <>
              {/* BarChart then Table */}
            </>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
```

**BarChart pattern** (lines 450-459 of ConciliacaoBridgeXML.tsx):
```tsx
<ResponsiveContainer width="100%" height={280} aria-label="Gráfico de créditos ICMS bloqueados por CFOP">
  <BarChart data={data.rows}>
    <CartesianGrid strokeDasharray="3 3" />
    <XAxis dataKey="cfop" tick={{ fontSize: 12 }} />
    <YAxis tickFormatter={(v) => fmtBRL(v)} tick={{ fontSize: 12 }} />
    <Tooltip formatter={(v) => fmtBRL(Number(v))} />
    <Legend />
    <Bar dataKey="vl_icms_total" name="ICMS Bloqueado" fill="var(--pis-cofins)" />
    <Bar dataKey="ibs_equiv"    name="Equiv. IBS+CBS"  fill="var(--ibs-cbs)" />
  </BarChart>
</ResponsiveContainer>
```

**Table header pattern** (lines 333-347 of ConciliacaoBridgeXML.tsx):
```tsx
<Table>
  <TableHeader>
    <TableRow className="hover:bg-transparent bg-muted/30">
      <TableHead className="text-xs font-semibold uppercase tracking-wide">Tipo CFOP</TableHead>
      <TableHead className="text-xs font-semibold uppercase tracking-wide">CFOP</TableHead>
      <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">ICMS Bloqueado (R$)</TableHead>
      {/* ... */}
    </TableRow>
  </TableHeader>
  <TableBody>
    {data.rows.map((row, idx) => (
      <TableRow key={`${row.cfop}-${idx}`}>
        <TableCell className="text-xs">{row.tipo_cfop}</TableCell>
        <TableCell className="text-xs font-mono">{row.cfop}</TableCell>
        <TableCell className="text-xs font-mono text-right">{fmtBRL(row.vl_icms_total)}</TableCell>
      </TableRow>
    ))}
  </TableBody>
</Table>
```

---

### `frontend/src/pages/Reforma13RankingFornecedores.tsx` (component/page, request-response)

**Analog:** `frontend/src/pages/ConciliacaoBridgeXML.tsx`

Same imports, useQuery, fmtBRL, fmtCNPJ, and CSV download patterns as Reforma11 above.

**Disclaimer banner pattern** (UI-SPEC Módulo 1.3):
```tsx
<Alert variant="default" className="border-warning text-warning-foreground bg-warning/10">
  <AlertTriangle className="h-4 w-4" />
  <AlertDescription>
    Valores estimados. A alíquota definitiva do Fator Simples Nacional não foi publicada
    pelo CG-IBS. Use como referência indicativa.
  </AlertDescription>
</Alert>
```

**Simples Nacional Badge pattern** (UI-SPEC + ConciliacaoBridgeXML.tsx Badge usage):
```tsx
{/* In table cell for confirmed Simples Nacional suppliers */}
<Badge variant="outline" className="text-xs px-1.5 py-0 bg-yellow-50 text-yellow-700 border-yellow-200">
  Simples
</Badge>
```

**Bar chart top 10** (ResponsiveContainer height 220px per UI-SPEC):
```tsx
<ResponsiveContainer width="100%" height={220} aria-label="Ranking de fornecedores Simples Nacional">
  <BarChart data={data.rows.slice(0, 10)}>
    <Bar dataKey="ibs_perdido_est" name="IBS+CBS Estimado" fill="var(--ibs-cbs)" />
    {/* ... */}
  </BarChart>
</ResponsiveContainer>
```

**useReformaParametros for fator_simples_pct** — the backend reads this from DB; the frontend only needs the JSON data back. No need to call `useReformaParametros` in this component (backend handles parameter lookup).

---

### `frontend/src/pages/Reforma12Reprecificacao.tsx` (component/page, request-response)

**Analog:** `frontend/src/pages/ConciliacaoBridgeXML.tsx` (filter + table pattern)

Same imports, useQuery, fmtBRL patterns as above. No bar chart (table-only per UI-SPEC).

**Filter row with Select pattern** (lines 278-311 of ConciliacaoBridgeXML.tsx):
```tsx
<div className="flex items-center gap-4 flex-wrap mb-4">
  <Select value={cstFilter} onValueChange={setCstFilter}>
    <SelectTrigger className="h-8 w-48">
      <SelectValue placeholder="Filtrar por CST" />
    </SelectTrigger>
    <SelectContent>
      <SelectItem value="todos">Todos</SelectItem>
      <SelectItem value="00">Normal (00)</SelectItem>
      <SelectItem value="st">Substituição Tributária</SelectItem>
      <SelectItem value="base_reduzida">Base Reduzida</SelectItem>
    </SelectContent>
  </Select>
</div>
```

**CST path Badge variants** (UI-SPEC Módulo 1.2):
```tsx
function CSTBadge({ cst }: { cst: string | null }) {
  if (!cst) return <span className="text-xs text-muted-foreground">—</span>
  const isNormal = cst === '00'
  const isST = ['10','30','60','70'].includes(cst)
  return (
    <Badge variant={isNormal ? 'secondary' : isST ? 'outline' : 'default'} className="font-mono text-xs">
      {cst}
    </Badge>
  )
}
```

**Variation column with sign and color** (UI-SPEC Módulo 1.2):
```tsx
function fmtVariacao(v: number | null | undefined): React.ReactNode {
  if (v == null) return <span className="text-xs text-muted-foreground">—</span>
  const positive = v > 0
  const neutral = v === 0
  return (
    <span className={`text-xs font-mono ${neutral ? 'text-muted-foreground' : positive ? 'text-green-600' : 'text-red-600'}`}>
      {positive ? '+' : ''}{v.toFixed(2)}%
    </span>
  )
}
```

---

### `frontend/src/pages/Reforma14SplitPayment.tsx` (component/page, request-response)

**Analog:** `frontend/src/pages/ReformaParametros.tsx` (KPI card layout) + `ConciliacaoBridgeXML.tsx` (useQuery, table)

**No CSV export** for this module per UI-SPEC (sensitivity table is display-only).

**KPI cards grid pattern** (derived from ConciliacaoBridgeXML.tsx lines 233-266 + UI-SPEC):
```tsx
<div className="grid grid-cols-2 gap-4">
  <Card>
    <CardHeader><CardTitle className="text-sm text-muted-foreground">Float Tributário</CardTitle></CardHeader>
    <CardContent>
      <p className="text-2xl font-semibold">{fmtBRL(data?.float_tributario)}</p>
      <p className="text-sm text-muted-foreground">IBS+CBS × Saídas × {data?.prazo_medio_dias ?? 30} dias / 365</p>
    </CardContent>
  </Card>
  <Card>
    <CardHeader><CardTitle className="text-sm text-muted-foreground">Custo CDI Estimado (R$/ano)</CardTitle></CardHeader>
    <CardContent>
      <p className="text-2xl font-semibold">{fmtBRL(data?.custo_cdi)}</p>
      <p className="text-sm text-muted-foreground">Float × {data?.taxa_cdi_anual_pct ?? 10.5}% CDI</p>
    </CardContent>
  </Card>
</div>
```

**Sensitivity matrix table** (UI-SPEC Módulo 1.4):
```tsx
{/* Rows = DSO (15,30,45,60,90); Columns = CDI (8%,10%,12%,14%) */}
<Table>
  <TableHeader>
    <TableRow>
      <TableHead className="text-xs font-semibold uppercase tracking-wide">DSO (dias)</TableHead>
      {CDI_VALUES.map(cdi => (
        <TableHead key={cdi} className="text-xs font-semibold uppercase tracking-wide text-right">
          CDI {cdi}%
        </TableHead>
      ))}
    </TableRow>
  </TableHeader>
  <TableBody>
    {DSO_VALUES.map(dso => (
      <TableRow key={dso}>
        <TableCell className="text-xs font-mono">{dso}</TableCell>
        {CDI_VALUES.map(cdi => {
          const isCurrentCell = dso === data?.prazo_medio_dias && cdi === data?.taxa_cdi_anual_pct
          return (
            <TableCell
              key={cdi}
              className={`text-xs font-mono text-right ${isCurrentCell ? 'bg-primary/10 font-semibold' : ''}`}
              aria-current={isCurrentCell ? 'true' : undefined}
            >
              {fmtBRL(calcCustoCDI(data?.total_saidas ?? 0, data?.aliq_total ?? 0, dso, cdi))}
            </TableCell>
          )
        })}
      </TableRow>
    ))}
  </TableBody>
</Table>
```

**Info disclaimer banner** (UI-SPEC Módulo 1.4):
```tsx
<Alert variant="default">
  <Info className="h-4 w-4" />
  <AlertDescription>
    Split payment entra em vigor gradualmente entre 2026 e 2033 conforme cronograma da Reforma.
    Os valores simulam o impacto no regime de transição plena.
  </AlertDescription>
</Alert>
```

---

### `frontend/src/lib/navigation.ts` (config — modification only)

**Analog:** `frontend/src/lib/navigation.ts` lines 48-57 (exact — remove `disabled: true`)

**Before (current state, lines 49-52 and 55-58):**
```typescript
{ label: 'Créditos IBS/CBS',       path: '/reforma/creditos',         disabled: true },
{ label: 'Reprecificação',         path: '/reforma/reprecificacao',   disabled: true },
{ label: 'Ranking Fornecedores',   path: '/reforma/ranking',          disabled: true },
{ label: 'Split Payment',          path: '/reforma/split-payment',    disabled: true },
```

**After (Phase 7 target):**
```typescript
{ label: 'Créditos IBS/CBS',       path: '/reforma/creditos' },
{ label: 'Reprecificação',         path: '/reforma/reprecificacao' },
{ label: 'Ranking Fornecedores',   path: '/reforma/ranking' },
{ label: 'Split Payment',          path: '/reforma/split-payment' },
```

**Lines that MUST keep `disabled: true`** (lines 53-57 — Phase 8 tabs):
```typescript
{ label: 'Análise CFOP',  path: '/reforma/cfop',        disabled: true },
{ label: 'Análise NCM',   path: '/reforma/ncm',         disabled: true },
{ label: 'UF Destino',    path: '/reforma/uf-destino',  disabled: true },
{ label: 'B2B vs B2C',   path: '/reforma/b2b-b2c',     disabled: true },
```

---

### `frontend/src/App.tsx` (router — modification only)

**Analog:** `frontend/src/App.tsx` lines 174-176 (reforma/parametros route block)

**Before (current state, lines 174-176):**
```tsx
{/* Análise Reforma Tributária */}
<Route path="/reforma/parametros"         element={<ReformaParametros />} />
<Route path="/config/reforma-parametros"  element={<ReformaParametros />} />
```

**After (Phase 7 additions — insert after line 176):**
```tsx
{/* Análise Reforma Tributária */}
<Route path="/reforma/parametros"         element={<ReformaParametros />} />
<Route path="/config/reforma-parametros"  element={<ReformaParametros />} />
<Route path="/reforma/creditos"           element={<Reforma11CreditosBloqueados />} />
<Route path="/reforma/reprecificacao"     element={<Reforma12Reprecificacao />} />
<Route path="/reforma/ranking"            element={<Reforma13RankingFornecedores />} />
<Route path="/reforma/split-payment"      element={<Reforma14SplitPayment />} />
```

**Import block additions** (lines 1-43 of App.tsx — add after line 31):
```tsx
import Reforma11CreditosBloqueados from './pages/Reforma11CreditosBloqueados'
import Reforma12Reprecificacao from './pages/Reforma12Reprecificacao'
import Reforma13RankingFornecedores from './pages/Reforma13RankingFornecedores'
import Reforma14SplitPayment from './pages/Reforma14SplitPayment'
```

---

## Shared Patterns

### Authentication + CompanyID Extraction
**Source:** `backend/handlers/creditos_perdidos.go` lines 93-113
**Apply to:** All 8 functions in `reforma_modulo1.go` (4 JSON + 4 CSV handlers)
```go
claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
if !ok {
    jsonErr(w, http.StatusUnauthorized, "Unauthorized")
    return
}
userID := claims["user_id"].(string)
companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
if err != nil {
    jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa: "+err.Error())
    return
}
```

### reforma_parametros Default Fallback
**Source:** `backend/handlers/reforma_config.go` lines 43-55 (pattern extended)
**Apply to:** All 4 JSON handlers in `reforma_modulo1.go`
```go
// Always guard against sql.ErrNoRows — empresa pode não ter configurado
if err == sql.ErrNoRows {
    aliqIBS, aliqCBS = 26.5, 9.9
    fatorSimples = 20.0
    taxaCDI = 10.5
    prazoMedio = 30
} else if err != nil {
    http.Error(w, "Erro ao ler parâmetros: "+err.Error(), http.StatusInternalServerError)
    return
}
```

### Empty Slice Guard (never return null JSON array)
**Source:** `backend/handlers/creditos_perdidos.go` lines 183-185, 241-243
**Apply to:** All handlers returning slices in `reforma_modulo1.go`
```go
if list == nil {
    list = []RowType{} // json.Marshal produces [] not null
}
```

### Parametrized SQL (IDOR protection)
**Source:** All existing handlers
**Apply to:** All SQL in `reforma_modulo1.go` — company_id always via `$1`, never string concatenation
```go
db.Query(`... WHERE j.company_id = $1 ...`, companyID)
// NEVER: fmt.Sprintf("WHERE company_id = '%s'", companyID)
```

### Frontend useQuery Pattern
**Source:** `frontend/src/pages/ConciliacaoBridgeXML.tsx` lines 98-126
**Apply to:** All 4 new React page components
```typescript
const { data, isLoading, isError } = useQuery<ResponseType>({
  queryKey: ['reforma/modulo1/ENDPOINT'],
  queryFn: async () => {
    const res = await fetch('/api/reforma/modulo1/ENDPOINT')
    if (!res.ok) throw new Error(`Erro ${res.status}`)
    return res.json()
  },
})
```

### Frontend 4-State Machine (Loading/Error/Empty/Data)
**Source:** `frontend/src/pages/ConciliacaoBridgeXML.tsx` lines 314-413
**Apply to:** All 4 new React page components
```tsx
{isLoading ? (
  <div className="space-y-2">
    {Array.from({ length: 6 }).map((_, i) => <Skeleton key={i} className="h-8 w-full" />)}
  </div>
) : isError ? (
  <Alert variant="destructive">
    <AlertDescription>Erro ao carregar dados. Verifique sua conexão e tente novamente.</AlertDescription>
  </Alert>
) : !data || data.rows.length === 0 ? (
  <p className="text-sm text-muted-foreground text-center py-8">{EMPTY_STATE_COPY}</p>
) : (
  /* render table/chart */
)}
```

### Frontend fmtBRL + fmtCNPJ Helpers
**Source:** `frontend/src/pages/ConciliacaoBridgeXML.tsx` lines 74-87
**Apply to:** All 4 new React page components — copy these helpers verbatim into each page file
```typescript
function fmtBRL(v: number | null | undefined): string {
  if (v == null) return '—'
  return v.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' })
}
function fmtCNPJ(v: string): string {
  if (!v || v.length !== 14) return v || '—'
  return `${v.slice(0, 2)}.${v.slice(2, 5)}.${v.slice(5, 8)}/${v.slice(8, 12)}-${v.slice(12)}`
}
```

### CSV Export Button State
**Source:** UI-SPEC State Machine Contract
**Apply to:** Reforma11, Reforma12, Reforma13 (Reforma14 has no CSV)
```tsx
<Button
  variant={data && data.rows.length > 0 ? 'default' : 'outline'}
  size="sm"
  onClick={handleExportCSV}
  disabled={downloadingCSV || !data || data.rows.length === 0}
>
  <Download className="mr-2 h-4 w-4" />
  {downloadingCSV ? 'Exportando...' : 'Exportar CSV'}
</Button>
```

---

## No Analog Found

All files have close analogs in the codebase. No entries in this section.

---

## Critical Pitfalls (from RESEARCH.md — planner must include in plan actions)

| Pitfall | File(s) Affected | Mitigation |
|---------|-----------------|------------|
| `reg_c190` has no `company_id` | `reforma_modulo1.go` (Módulo 1.1) | Always join via `c190 → c100 (id_pai_c100) → import_jobs (job_id)` |
| `cod_sit` is on `reg_c100`, not `reg_c190` | `reforma_modulo1.go` (Módulo 1.1) | `WHERE c100.cod_sit NOT IN ('02','03','04','05')` after JOIN |
| CFOP is on items table, not NF-e header | `reforma_modulo1.go` (1.2, 1.3, 1.4) | Always join `nfe_entradas_itens`/`nfe_saidas_itens` for CFOP filter |
| `forn_simples.cnpj` is 14-digit pure number | `reforma_modulo1.go` (Módulo 1.3) | `JOIN forn_simples fs ON fs.cnpj = REGEXP_REPLACE(p.cnpj, '[^0-9]', '', 'g')` |
| `reforma_parametros` may be empty | `reforma_modulo1.go` (all handlers) | `sql.ErrNoRows` guard with hardcoded defaults |
| Phase 8 tabs must stay `disabled: true` | `navigation.ts` | Only remove disabled from 4 Phase 7 paths; keep 4 Phase 8 paths disabled |
| recharts `Tooltip` conflicts with shadcn `Tooltip` | All pages with charts | `import { Tooltip as UITooltip } from '@/components/ui/tooltip'` |

---

## Metadata

**Analog search scope:** `backend/handlers/`, `frontend/src/pages/`, `frontend/src/lib/`, `frontend/src/hooks/`, `backend/main.go`
**Files scanned:** 8 source files read directly
**Pattern extraction date:** 2026-05-22
