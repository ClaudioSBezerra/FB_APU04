# Phase 4: Conciliação Bridge vs XML — Pattern Map

**Mapped:** 2026-05-16
**Files analyzed:** 6 (2 new, 4 modified)
**Analogs found:** 6 / 6

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `backend/handlers/xml_conciliacao.go` | handler | request-response + CSV | `backend/handlers/xml_reports.go` | exact |
| `frontend/src/pages/ConciliacaoBridgeXML.tsx` | page/component | request-response | `frontend/src/pages/RelatorioSaneamento.tsx` + `PainelXMLs.tsx` | exact (composite) |
| `backend/main.go` | config/route | — | self (lines 568–571) | exact |
| `frontend/src/lib/navigation.ts` | config | — | self (lines 14–53, 55–68) | exact |
| `frontend/src/App.tsx` | config/route | — | self (lines 154–164) | exact |
| `backend/migrations/080_*.sql` | migration | — | `backend/migrations/074_add_source_to_nfe_tables.sql` | role-match |

---

## Pattern Assignments

### `backend/handlers/xml_conciliacao.go` (handler, request-response + CSV)

**Analog:** `backend/handlers/xml_reports.go`

**Imports pattern** (xml_reports.go lines 1–13):
```go
package handlers

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)
```

**File header comment pattern** (xml_reports.go lines 15–26):
```go
// ---------------------------------------------------------------------------
// xml_conciliacao.go — Conciliação Bridge vs XML
//
// GET /api/xml/conciliacao?mes_ano=MM/YYYY&tipo=entradas|saidas → ConciliacaoHandler
// GET /api/xml/cobertura?mes_ano=MM/YYYY&tipo=entradas|saidas   → CoberturaHandler
// GET /api/xml/conciliacao/csv                                   → ConciliacaoCSVHandler
//
// Todos os handlers:
//   - Usam GetEffectiveCompanyID (company_id do usuário autenticado)
//   - Usam parâmetros $N nas queries (nunca concatenação de strings com input do usuário)
//   - Não setam CORS headers (tratados pelo SecurityMiddleware em main.go)
// ---------------------------------------------------------------------------
```

**Struct declaration pattern** (xml_reports.go lines 29–46):
```go
// conciliacaoRow representa uma NF-e com divergência entre valores Bridge e XML.
type conciliacaoRow struct {
	ChaveNfe    string  `json:"chave_nfe"`
	FornCNPJ    string  `json:"forn_cnpj"`
	FornNome    string  `json:"forn_nome"`
	MesAno      string  `json:"mes_ano"`
	DataEmissao string  `json:"data_emissao"`
	CFOP        string  `json:"cfop"`
	// Valores XML
	XmlPis    float64 `json:"xml_pis"`
	XmlCofins float64 `json:"xml_cofins"`
	XmlIcms   float64 `json:"xml_icms"`
	XmlIpi    float64 `json:"xml_ipi"`
	XmlVNf    float64 `json:"xml_v_nf"`
	// Valores Bridge
	BridgePis    float64 `json:"bridge_pis"`
	BridgeCofins float64 `json:"bridge_cofins"`
	BridgeIcms   float64 `json:"bridge_icms"`
	BridgeIpi    float64 `json:"bridge_ipi"`
	// Deltas
	DeltaPis    float64 `json:"delta_pis"`
	DeltaCofins float64 `json:"delta_cofins"`
	DeltaIcms   float64 `json:"delta_icms"`
	DeltaIpi    float64 `json:"delta_ipi"`
	DeltaTotal  float64 `json:"delta_total"`
}

// coberturaRow representa agregação de cobertura XML por mês.
type coberturaRow struct {
	MesAno     string  `json:"mes_ano"`
	TotalNfes  int     `json:"total_nfes"`
	ComXml     int     `json:"com_xml"`
	SoBridge   int     `json:"so_bridge"`
	PctXml     float64 `json:"pct_xml"`
}
```

**Core handler factory pattern** (xml_reports.go lines 180–219 — XMLSaneamentoCCLASSTRIBHandler):
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

		mesAno := strings.TrimSpace(r.URL.Query().Get("mes_ano"))
		tipo := strings.TrimSpace(r.URL.Query().Get("tipo"))
		if tipo == "" {
			tipo = "entradas"
		}
		// whitelist de tipo para evitar injeção SQL em nome de tabela
		tabela := "nfe_entradas"
		if tipo == "saidas" {
			tabela = "nfe_saidas"
		}

		data, err := executeConciliacaoQuery(db, companyID, mesAno, tabela)
		if err != nil {
			log.Printf("[Conciliacao] query error: %v", err)
			jsonErr(w, http.StatusInternalServerError, "Erro ao consultar banco")
			return
		}

		if data == nil {
			data = []conciliacaoRow{}
		}

		if encErr := json.NewEncoder(w).Encode(data); encErr != nil {
			log.Printf("[Conciliacao] encode error: %v", encErr)
		}
	}
}
```

**CSV handler pattern** (xml_reports.go lines 227–336 — XMLSaneamentoCSVHandler):
```go
func ConciliacaoCSVHandler(db *sql.DB) http.HandlerFunc {
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

		mesAno := strings.TrimSpace(r.URL.Query().Get("mes_ano"))
		tipo := strings.TrimSpace(r.URL.Query().Get("tipo"))
		tabela := "nfe_entradas"
		if tipo == "saidas" {
			tabela = "nfe_saidas"
		}

		data, err := executeConciliacaoQuery(db, companyID, mesAno, tabela)
		if err != nil {
			log.Printf("[Conciliacao/CSV] query error: %v", err)
			http.Error(w, "Erro ao consultar banco", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="conciliacao-bridge-xml.csv"`)

		cw := csv.NewWriter(w)
		header := []string{
			"Chave NF-e", "CNPJ Fornecedor", "Fornecedor", "Mês/Ano", "Data Emissão", "CFOP",
			"PIS XML", "PIS Bridge", "Delta PIS",
			"COFINS XML", "COFINS Bridge", "Delta COFINS",
			"ICMS XML", "ICMS Bridge", "Delta ICMS",
			"IPI XML", "IPI Bridge", "Delta IPI",
			"Delta Total",
		}
		if err := cw.Write(header); err != nil {
			log.Printf("[Conciliacao/CSV] write header error: %v", err)
			return
		}

		for _, row := range data {
			record := []string{
				row.ChaveNfe, row.FornCNPJ, row.FornNome, row.MesAno, row.DataEmissao, row.CFOP,
				fmt.Sprintf("%.2f", row.XmlPis), fmt.Sprintf("%.2f", row.BridgePis), fmt.Sprintf("%.2f", row.DeltaPis),
				fmt.Sprintf("%.2f", row.XmlCofins), fmt.Sprintf("%.2f", row.BridgeCofins), fmt.Sprintf("%.2f", row.DeltaCofins),
				fmt.Sprintf("%.2f", row.XmlIcms), fmt.Sprintf("%.2f", row.BridgeIcms), fmt.Sprintf("%.2f", row.DeltaIcms),
				fmt.Sprintf("%.2f", row.XmlIpi), fmt.Sprintf("%.2f", row.BridgeIpi), fmt.Sprintf("%.2f", row.DeltaIpi),
				fmt.Sprintf("%.2f", row.DeltaTotal),
			}
			if err := cw.Write(record); err != nil {
				log.Printf("[Conciliacao/CSV] write row error: %v", err)
				return
			}
		}

		cw.Flush()
		if err := cw.Error(); err != nil {
			log.Printf("[Conciliacao/CSV] flush error: %v", err)
		}
	}
}
```

**Query helper pattern** (xml_reports.go lines 73–174 — executeSaneamentoQuery):
```go
// executeConciliacaoQuery executa a query de divergências e retorna os dados.
// tabela deve ser "nfe_entradas" ou "nfe_saidas" — validado pelo chamador via whitelist.
func executeConciliacaoQuery(db *sql.DB, companyID, mesAno, tabela string) ([]conciliacaoRow, error) {
	args := []interface{}{companyID}
	whereExtra := ""
	paramIdx := 2

	if mesAno != "" {
		whereExtra += fmt.Sprintf(" AND ne.mes_ano = $%d", paramIdx)
		args = append(args, mesAno)
		paramIdx++
	}

	// tabela já validada pelo chamador como "nfe_entradas" ou "nfe_saidas"
	query := fmt.Sprintf(`
SELECT
    ne.chave_nfe,
    COALESCE(ne.forn_cnpj, '')                         AS forn_cnpj,
    COALESCE(NULLIF(ne.forn_nome, ''), '')             AS forn_nome,
    ne.mes_ano,
    TO_CHAR(ne.data_emissao, 'DD/MM/YYYY')             AS data_emissao,
    COALESCE(ne.cfop, '')                              AS cfop,
    COALESCE(ne.v_pis, 0)    AS xml_pis,
    COALESCE(ne.v_cofins, 0) AS xml_cofins,
    COALESCE(ne.v_icms, 0)   AS xml_icms,
    COALESCE(ne.v_ipi, 0)    AS xml_ipi,
    COALESCE(ne.v_nf, 0)     AS xml_v_nf,
    COALESCE(ne.pis, 0)      AS bridge_pis,
    COALESCE(ne.cofins, 0)   AS bridge_cofins,
    COALESCE(ne.icms, 0)     AS bridge_icms,
    COALESCE(ne.ipi, 0)      AS bridge_ipi,
    ROUND(ABS(COALESCE(ne.v_pis,0)    - COALESCE(ne.pis,0)),    2) AS delta_pis,
    ROUND(ABS(COALESCE(ne.v_cofins,0) - COALESCE(ne.cofins,0)), 2) AS delta_cofins,
    ROUND(ABS(COALESCE(ne.v_icms,0)   - COALESCE(ne.icms,0)),   2) AS delta_icms,
    ROUND(ABS(COALESCE(ne.v_ipi,0)    - COALESCE(ne.ipi,0)),    2) AS delta_ipi,
    ROUND(
        ABS(COALESCE(ne.v_pis,0) - COALESCE(ne.pis,0)) +
        ABS(COALESCE(ne.v_cofins,0) - COALESCE(ne.cofins,0)) +
        ABS(COALESCE(ne.v_icms,0) - COALESCE(ne.icms,0)),
    2) AS delta_total
FROM %s ne
WHERE ne.company_id = $1
  AND ne.source = 'xml_upload'
  AND ne.cancelado != 'S'
  AND (COALESCE(ne.pis,0) + COALESCE(ne.cofins,0) + COALESCE(ne.icms,0)) > 0
  AND (ABS(COALESCE(ne.v_pis,0)    - COALESCE(ne.pis,0))    > 0.01
    OR ABS(COALESCE(ne.v_cofins,0) - COALESCE(ne.cofins,0)) > 0.01
    OR ABS(COALESCE(ne.v_icms,0)   - COALESCE(ne.icms,0))   > 0.01)
  %s
ORDER BY delta_total DESC
LIMIT 500`, tabela, whereExtra)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []conciliacaoRow
	for rows.Next() {
		var row conciliacaoRow
		if err := rows.Scan(
			&row.ChaveNfe, &row.FornCNPJ, &row.FornNome, &row.MesAno, &row.DataEmissao, &row.CFOP,
			&row.XmlPis, &row.XmlCofins, &row.XmlIcms, &row.XmlIpi, &row.XmlVNf,
			&row.BridgePis, &row.BridgeCofins, &row.BridgeIcms, &row.BridgeIpi,
			&row.DeltaPis, &row.DeltaCofins, &row.DeltaIcms, &row.DeltaIpi, &row.DeltaTotal,
		); err != nil {
			log.Printf("[Conciliacao] scan error: %v", err)
			continue
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
```

**Error handling pattern** (xml_reports.go lines 205–219):
```go
// After db.Query():
if err != nil {
    log.Printf("[Conciliacao] query error: %v", err)
    jsonErr(w, http.StatusInternalServerError, "Erro ao consultar banco")
    return
}
// After rows.Scan():
if err := rows.Scan(...); err != nil {
    log.Printf("[Conciliacao] scan error: %v", err)
    continue  // skip bad row, don't abort
}
// rows.Err() check after loop:
if err := rows.Err(); err != nil {
    return nil, err
}
// Return empty slice, not null:
if data == nil {
    data = []conciliacaoRow{}
}
```

---

### `frontend/src/pages/ConciliacaoBridgeXML.tsx` (page/component, request-response)

**Primary analog:** `frontend/src/pages/RelatorioSaneamento.tsx`
**Secondary analog:** `frontend/src/pages/PainelXMLs.tsx`

**Imports pattern** (RelatorioSaneamento.tsx lines 1–17 + PainelXMLs.tsx lines 1–17):
```typescript
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { toast } from 'sonner';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from 'recharts';
import { Download, AlertTriangle, CheckCircle, FileSpreadsheet, Printer } from 'lucide-react';
import { exportToExcel } from '@/lib/exportToExcel';
```

**Helper functions pattern** (RelatorioSaneamento.tsx lines 52–65, PainelXMLs.tsx lines 49–69):
```typescript
// Copy fmtBRL and fmtCNPJ exactly from RelatorioSaneamento.tsx — identical in both files
function fmtBRL(v: number | null | undefined): string {
  if (v == null) return '—';
  return v.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' });
}

function fmtCNPJ(v: string): string {
  if (!v || v.length !== 14) return v || '—';
  return `${v.slice(0, 2)}.${v.slice(2, 5)}.${v.slice(5, 8)}/${v.slice(8, 12)}-${v.slice(12)}`;
}

// URL builder pattern from RelatorioSaneamento.tsx lines 62–65:
function buildUrl(base: string, params: Record<string, string>): string {
  const q = new URLSearchParams(Object.entries(params).filter(([, v]) => v !== ''));
  return q.toString() ? `${base}?${q}` : base;
}
```

**TypeScript interface pattern** (RelatorioSaneamento.tsx lines 22–47):
```typescript
interface DivergenciaRow {
  chave_nfe: string;
  forn_cnpj: string;
  forn_nome: string;
  mes_ano: string;
  data_emissao: string;
  cfop: string;
  xml_pis: number;
  xml_cofins: number;
  xml_icms: number;
  xml_ipi: number;
  xml_v_nf: number;
  bridge_pis: number;
  bridge_cofins: number;
  bridge_icms: number;
  bridge_ipi: number;
  delta_pis: number;
  delta_cofins: number;
  delta_icms: number;
  delta_ipi: number;
  delta_total: number;
}

interface CoberturaRow {
  mes_ano: string;
  total_nfes: number;
  com_xml: number;
  so_bridge: number;
  pct_xml: number;
}
```

**State + useQuery pattern** (RelatorioSaneamento.tsx lines 70–103):
```typescript
export default function ConciliacaoBridgeXML() {
  const [mesAnoFiltro, setMesAnoFiltro] = useState('');
  const [mesAnoAtivo, setMesAnoAtivo] = useState('');
  const [tipo, setTipo] = useState<'entradas' | 'saidas'>('entradas');

  const {
    data: divergencias,
    isLoading: loadingDiv,
    isError: errorDiv,
    refetch: refetchDiv,
  } = useQuery<DivergenciaRow[]>({
    queryKey: ['xml-conciliacao', mesAnoAtivo, tipo],
    queryFn: async () => {
      const res = await fetch(
        buildUrl('/api/xml/conciliacao', { mes_ano: mesAnoAtivo, tipo })
      );
      if (!res.ok) throw new Error(`Erro ${res.status}`);
      return res.json();
    },
  });

  const {
    data: cobertura,
    isLoading: loadingCob,
    isError: errorCob,
    refetch: refetchCob,
  } = useQuery<CoberturaRow[]>({
    queryKey: ['xml-cobertura', tipo],
    queryFn: async () => {
      const res = await fetch(buildUrl('/api/xml/cobertura', { tipo }));
      if (!res.ok) throw new Error(`Erro ${res.status}`);
      return res.json();
    },
  });
```

**Buscar/Limpar handler pattern** (RelatorioSaneamento.tsx lines 105–127):
```typescript
  const handleBuscar = () => {
    setMesAnoAtivo(mesAnoFiltro.trim());
  };

  const handleLimpar = () => {
    setMesAnoFiltro('');
    setMesAnoAtivo('');
  };
```

**Excel export pattern** (RelatorioSaneamento.tsx lines 109–127 adapted + exportToExcel.ts):
```typescript
  const handleExportExcel = () => {
    if (!divergencias) return;
    const data = divergencias.map(r => ({
      'Chave NF-e':      r.chave_nfe,
      'CNPJ Fornecedor': r.forn_cnpj,
      'Fornecedor':      r.forn_nome,
      'Mês/Ano':         r.mes_ano,
      'Data Emissão':    r.data_emissao,
      'PIS XML':         r.xml_pis   ?? 0,
      'PIS Bridge':      r.bridge_pis ?? 0,
      'Delta PIS':       r.delta_pis  ?? 0,
      'COFINS XML':      r.xml_cofins   ?? 0,
      'COFINS Bridge':   r.bridge_cofins ?? 0,
      'Delta COFINS':    r.delta_cofins  ?? 0,
      'ICMS XML':        r.xml_icms   ?? 0,
      'ICMS Bridge':     r.bridge_icms ?? 0,
      'Delta ICMS':      r.delta_icms  ?? 0,
      'IPI XML':         r.xml_ipi   ?? 0,
      'IPI Bridge':      r.bridge_ipi ?? 0,
      'Delta IPI':       r.delta_ipi  ?? 0,
      'Delta Total':     r.delta_total ?? 0,
    }));
    exportToExcel(data, `conciliacao-bridge-xml-${mesAnoAtivo || 'geral'}`, 'Divergências');
    toast.success('Excel exportado com sucesso');
  };
```

**CSV download pattern** (RelatorioSaneamento.tsx lines 109–127 — handleDownloadCSV):
```typescript
  const [downloadingCSV, setDownloadingCSV] = useState(false);

  const handleExportCSV = async () => {
    setDownloadingCSV(true);
    try {
      const res = await fetch(
        buildUrl('/api/xml/conciliacao/csv', { mes_ano: mesAnoAtivo, tipo })
      );
      if (!res.ok) throw new Error(`Erro ${res.status}`);
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `conciliacao-bridge-xml.csv`;
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

**Summary cards pattern** (RelatorioSaneamento.tsx lines 178–210):
```tsx
  // Summary metrics (computed from query results)
  const totalDivergencias = divergencias?.length ?? 0;
  const deltaTotal = divergencias
    ? divergencias.reduce((acc, r) => acc + (r.delta_total ?? 0), 0)
    : 0;
  const pctXml = cobertura && cobertura.length > 0
    ? cobertura[0].pct_xml
    : null;

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold">Conciliação Bridge vs XML</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Compare os valores tributários do ERP Bridge com os documentos fiscais SEFAZ
          para identificar divergências e medir a cobertura de autenticidade.
        </p>
      </div>

      {/* 3-col summary cards — pattern from RelatorioSaneamento.tsx lines 179–210 */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-semibold text-muted-foreground">
              NF-es com divergência
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-xl font-semibold">{loadingDiv ? '…' : totalDivergencias}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-semibold text-muted-foreground">
              Delta tributário total
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-xl font-semibold">{loadingDiv ? '…' : fmtBRL(deltaTotal)}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-semibold text-muted-foreground">
              Cobertura XML (entradas)
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-xl font-semibold">
              {loadingCob ? '…' : pctXml != null ? `${pctXml.toFixed(1)}%` : '—'}
            </p>
          </CardContent>
        </Card>
      </div>
```

**Tabs + filters pattern** (PainelXMLs.tsx lines 239–255 + RelatorioSaneamento.tsx lines 148–176):
```tsx
      <Tabs defaultValue="divergencias">
        <TabsList>
          <TabsTrigger value="divergencias">Divergências</TabsTrigger>
          <TabsTrigger value="cobertura">Cobertura XML</TabsTrigger>
        </TabsList>

        <TabsContent value="divergencias">
          {/* Filters row */}
          <div className="flex items-center gap-3 flex-wrap mb-4">
            <div className="flex items-center gap-2">
              <label className="text-xs text-muted-foreground whitespace-nowrap">Mês/Ano</label>
              <Input
                type="text"
                placeholder="MM/YYYY"
                value={mesAnoFiltro}
                onChange={e => setMesAnoFiltro(e.target.value)}
                className="h-8 w-28 text-sm"
              />
            </div>
            <Select value={tipo} onValueChange={v => setTipo(v as 'entradas' | 'saidas')}>
              <SelectTrigger className="h-8 w-40">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="entradas">NF-e Entradas</SelectItem>
                <SelectItem value="saidas">NF-e Saídas</SelectItem>
              </SelectContent>
            </Select>
            <Button size="sm" onClick={handleBuscar} disabled={loadingDiv}>
              {loadingDiv ? 'Buscando...' : 'Buscar Divergências'}
            </Button>
            {mesAnoAtivo && (
              <Button size="sm" variant="ghost" className="text-xs text-muted-foreground" onClick={handleLimpar}>
                Limpar
              </Button>
            )}
          </div>
```

**Loading/empty/error states pattern** (RelatorioSaneamento.tsx lines 226–240):
```tsx
          {/* Loading / error / empty states — copy exactly from RelatorioSaneamento.tsx */}
          {loadingDiv ? (
            <p className="text-sm text-muted-foreground text-center py-8">
              Carregando divergências...
            </p>
          ) : errorDiv ? (
            <p className="text-sm text-destructive px-4 py-6">
              Erro ao carregar dados de conciliação.{' '}
              <button className="underline" onClick={() => refetchDiv()}>Tentar novamente</button>
            </p>
          ) : !divergencias || divergencias.length === 0 ? (
            <div className="flex items-center gap-2 px-4 py-6 text-sm text-muted-foreground">
              <CheckCircle className="w-4 h-4 text-green-500" />
              Nenhuma divergência encontrada. Todas as NF-es com origem XML têm valores
              tributários compatíveis com o ERP Bridge no período selecionado.
            </div>
          ) : (
```

**Dense table pattern** (PainelXMLs.tsx lines 175–215):
```tsx
            <div className="overflow-x-auto rounded-md border">
              <Table>
                <TableHeader>
                  <TableRow className="hover:bg-transparent bg-muted/30">
                    <TableHead className="py-1.5 px-2 text-[11px]">Fornecedor</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px]">Mês/Ano</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px]">Data Emissão</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px] text-right">PIS XML</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px] text-right">PIS Bridge</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px] text-right">Delta PIS</TableHead>
                    {/* repeat for COFINS, ICMS, IPI, Delta Total */}
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {divergencias.map((row, idx) => (
                    <TableRow
                      key={`${row.chave_nfe}-${idx}`}
                      className={row.delta_total > 0.01 ? 'bg-red-50 hover:bg-red-100' : ''}
                    >
                      {/* Fornecedor cell — two-line pattern from PainelXMLs.tsx line 197–199 */}
                      <TableCell className="py-1 px-2">
                        <div className="text-[11px] font-medium leading-tight truncate max-w-[180px]">
                          {row.forn_nome || '—'}
                        </div>
                        <div className="text-[10px] text-muted-foreground font-mono leading-tight">
                          {fmtCNPJ(row.forn_cnpj)}
                        </div>
                      </TableCell>
                      <TableCell className="py-1 px-2 text-[11px] whitespace-nowrap">{row.mes_ano}</TableCell>
                      <TableCell className="py-1 px-2 text-[11px] whitespace-nowrap">{row.data_emissao}</TableCell>
                      {/* BRL values right-aligned, font-semibold for XML values */}
                      <TableCell className="py-1 px-2 text-right text-[11px] font-semibold">{fmtBRL(row.xml_pis)}</TableCell>
                      <TableCell className="py-1 px-2 text-right text-[11px] text-muted-foreground">{fmtBRL(row.bridge_pis)}</TableCell>
                      <TableCell className="py-1 px-2 text-right">
                        <Badge
                          variant="outline"
                          className={row.delta_pis > 0.01
                            ? 'text-[10px] px-1.5 py-0 bg-red-50 text-red-700 border-red-200'
                            : 'text-[10px] px-1.5 py-0 bg-gray-50 text-muted-foreground'}
                        >
                          {fmtBRL(row.delta_pis)}
                        </Badge>
                      </TableCell>
                      {/* repeat pattern for delta_cofins, delta_icms */}
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
```

**Recharts BarChart pattern** (RESEARCH.md Pattern 5, verified against OperacoesSimplesNacional.tsx lines 5–15, 219–237):
```tsx
        <TabsContent value="cobertura">
          <ResponsiveContainer width="100%" height={300}>
            <BarChart data={cobertura ?? []}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="mes_ano" tick={{ fontSize: 11 }} />
              <YAxis tickFormatter={(v) => `${v}%`} tick={{ fontSize: 11 }} />
              <Tooltip formatter={(v) => `${Number(v).toFixed(1)}%`} />
              <Legend />
              <Bar dataKey="pct_xml" name="XML (Autêntico)" fill="#22c55e" stackId="a" />
            </BarChart>
          </ResponsiveContainer>
          {/* footnote — always present */}
          <p className="text-[11px] text-muted-foreground mt-2">
            Notas canceladas excluídas da contagem.
          </p>
        </TabsContent>
```

**Export buttons pattern** (UI-SPEC.md + RelatorioSaneamento.tsx lines 215–225):
```tsx
          {/* Export buttons — BELOW the table, left-aligned, no-print class */}
          <div className="flex items-center gap-2 mt-4 no-print">
            <Button size="sm" variant="outline" onClick={handleExportExcel}>
              <FileSpreadsheet className="w-4 h-4 mr-1" /> Exportar Excel
            </Button>
            <Button size="sm" variant="outline" onClick={handleExportCSV} disabled={downloadingCSV}>
              <Download className="w-4 h-4 mr-1" />
              {downloadingCSV ? 'Exportando...' : 'Exportar CSV'}
            </Button>
            <Button size="sm" variant="ghost" onClick={() => window.print()}>
              <Printer className="w-4 h-4 mr-1" /> Imprimir PDF
            </Button>
          </div>
          {/* threshold legend */}
          <p className="text-[11px] text-muted-foreground mt-2">
            (divergências &gt; R$ 0,01)
          </p>
```

---

### `backend/main.go` (route registration, modify)

**Analog:** `backend/main.go` lines 568–571 (existing xml/reports block)

**Route registration pattern** (main.go lines 568–571):
```go
// Relatórios de Saneamento CCLASSTRIB — /csv deve ser registrado ANTES de /saneamento (mais específico primeiro)
http.HandleFunc("/api/xml/reports/saneamento/csv", withAuth(handlers.XMLSaneamentoCSVHandler, ""))
http.HandleFunc("/api/xml/reports/saneamento", withAuth(handlers.XMLSaneamentoCCLASSTRIBHandler, ""))
http.HandleFunc("/api/xml/reports/fornecedores-cclasstrib", withAuth(handlers.XMLFornecedoresCCLASSTRIBHandler, ""))
```

**Where to insert new routes** — AFTER line 571 (after existing xml/reports block, before line 573 nfe-saidas):
```go
// Conciliação Bridge vs XML — /csv deve ser registrado ANTES de /conciliacao (mais específico primeiro)
http.HandleFunc("/api/xml/conciliacao/csv", withAuth(handlers.ConciliacaoCSVHandler, ""))
http.HandleFunc("/api/xml/conciliacao", withAuth(handlers.ConciliacaoHandler, ""))
http.HandleFunc("/api/xml/cobertura", withAuth(handlers.CoberturaHandler, ""))
```

**Critical ordering note:** Go stdlib mux matches by specificity — `/api/xml/conciliacao/csv` must be registered BEFORE `/api/xml/conciliacao`. This is the same pattern used at lines 568–569 for `/csv` before the base saneamento route.

---

### `frontend/src/lib/navigation.ts` (modify)

**Analog:** `frontend/src/lib/navigation.ts` lines 28–37 (notas module tabs) + lines 61–65 (getActiveModule)

**Tab insertion pattern** (navigation.ts lines 28–37 — notas module):
```typescript
notas: {
  label: 'Notas Importadas',
  tabs: [
    { label: 'NF-e Entradas',          path: '/apuracao/entrada/notas' },
    // ... existing tabs ...
    { label: 'Saneamento CCLASSTRIB',  path: '/relatorios/saneamento-cclasstrib' },
    // ADD after Saneamento CCLASSTRIB:
    { label: 'Conciliação Bridge vs XML', path: '/conciliacao/bridge-xml' },
  ],
},
```

**getActiveModule pattern** (navigation.ts lines 55–68):
```typescript
export function getActiveModule(pathname: string): string {
  // ... existing conditions ...
  if (pathname.startsWith('/relatorios/saneamento')) return 'notas'
  // ADD before final return:
  if (pathname.startsWith('/conciliacao/')) return 'notas'
  // ...
}
```

---

### `frontend/src/App.tsx` (modify)

**Analog:** `frontend/src/App.tsx` lines 22–24 (import pattern) + lines 154–164 (Notas Importadas routes block)

**Import pattern** (App.tsx lines 22–24):
```typescript
// Add with other page imports, alphabetically near RelatorioSaneamento:
import ConciliacaoBridgeXML from './pages/ConciliacaoBridgeXML'
```

**Route registration pattern** (App.tsx lines 163–164):
```tsx
{/* Inside Notas Importadas block, after saneamento-cclasstrib: */}
<Route path="/relatorios/saneamento-cclasstrib"  element={<RelatorioSaneamento />} />
<Route path="/conciliacao/bridge-xml"            element={<ConciliacaoBridgeXML />} />
```

---

### `backend/migrations/080_*.sql` (optional, migration)

**Analog:** `backend/migrations/074_add_source_to_nfe_tables.sql` (source index pattern)

**Index creation pattern** (migration 074 pattern — composite index for combined filter):
```sql
-- 080_create_indexes_conciliacao.sql
-- Índice composto para queries de conciliação: company_id + source + mes_ano
-- Suporta filtro WHERE company_id=$1 AND source='xml_upload' AND mes_ano=$2
CREATE INDEX IF NOT EXISTS idx_nfe_entradas_source_mes
    ON nfe_entradas(company_id, source, mes_ano);

CREATE INDEX IF NOT EXISTS idx_nfe_saidas_source_mes
    ON nfe_saidas(company_id, source, mes_ano);
```

**Note:** Existing indexes `idx_nfe_entradas_source` (company_id, source) and `idx_nfe_entradas_company_mes` (company_id, mes_ano) already exist (migration 074). Migration 080 is OPTIONAL — only needed if query profiling shows index misses on combined filter.

---

## Shared Patterns

### Authentication + Company Isolation
**Source:** `backend/handlers/xml_reports.go` lines 188–200 (in XMLSaneamentoCCLASSTRIBHandler)
**Apply to:** `ConciliacaoHandler`, `CoberturaHandler`, `ConciliacaoCSVHandler`
```go
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
```

### Error handling (JSON handlers)
**Source:** `backend/handlers/ai_query.go` lines 55–65 (jsonErr definition)
**Apply to:** All new Go handlers — use `jsonErr` for all error responses, `log.Printf` with handler prefix before returning error
```go
func jsonErr(w http.ResponseWriter, status int, msg string, extra ...map[string]string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    out := map[string]string{"error": msg}
    for _, m := range extra {
        for k, v := range m {
            out[k] = v
        }
    }
    json.NewEncoder(w).Encode(out)
}
```

### Empty-slice guard
**Source:** `backend/handlers/xml_reports.go` lines 212–214
**Apply to:** All JSON list handlers — return `[]T{}` never `null`
```go
if data == nil {
    data = []conciliacaoRow{}
}
```

### fmtBRL + fmtCNPJ helpers
**Source:** `frontend/src/pages/RelatorioSaneamento.tsx` lines 52–65
**Apply to:** `ConciliacaoBridgeXML.tsx` — copy these functions verbatim (identical in RelatorioSaneamento and PainelXMLs)

### Toast notifications
**Source:** `frontend/src/pages/RelatorioSaneamento.tsx` lines 121–124
**Apply to:** All export handlers in `ConciliacaoBridgeXML.tsx`
```typescript
toast.success('CSV exportado com sucesso');
toast.error('Erro ao exportar CSV: ' + (err instanceof Error ? err.message : 'Desconhecido'));
```

### Input validation (query params)
**Source:** `backend/handlers/xml_painel.go` lines 62–70
**Apply to:** `ConciliacaoHandler` and `CoberturaHandler` — validate `tipo` with explicit whitelist, pass `mes_ano` as parameterized `$N`, never concatenate user input into SQL
```go
mesAno := strings.TrimSpace(r.URL.Query().Get("mes_ano"))
tipo   := strings.TrimSpace(r.URL.Query().Get("tipo"))
// Whitelist — never interpolate into SQL:
tabela := "nfe_entradas"
if tipo == "saidas" {
    tabela = "nfe_saidas"
}
```

---

## No Analog Found

All files have analogs. No entries in this section.

---

## Metadata

**Analog search scope:** `backend/handlers/`, `frontend/src/pages/`, `frontend/src/lib/`, `backend/main.go`
**Files scanned:** xml_reports.go, xml_painel.go, RelatorioSaneamento.tsx, PainelXMLs.tsx, navigation.ts, App.tsx, exportToExcel.ts, ai_query.go (jsonErr), auth.go (ClaimsKey/GetEffectiveCompanyID), OperacoesSimplesNacional.tsx (recharts)
**Pattern extraction date:** 2026-05-16
