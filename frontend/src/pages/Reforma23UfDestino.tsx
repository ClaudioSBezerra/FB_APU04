import { useState, useEffect } from 'react'
import { ComposableMap, Geographies, Geography } from 'react-simple-maps'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------
interface Modulo23Row {
  dest_uf: string
  qtd_notas: number
  valor_total: number
  vl_icms: number
  ibs_projetado: number
  cbs_projetado: number
}

interface Modulo23Response {
  rows: Modulo23Row[]
  aliq_ibs_pct: number
  aliq_cbs_pct: number
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
function fmtBRL(v: number | null | undefined): string {
  if (v == null) return '—'
  return v.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' })
}

/**
 * Interpolação linear hexadecimal entre #dbeafe (mínimo) e #1d4ed8 (máximo).
 * Se val <= 0 retorna #e5e7eb (cinza neutro).
 */
function colorScale(val: number, minVal: number, maxVal: number): string {
  if (val <= 0 || maxVal <= minVal) return '#e5e7eb'
  const t = Math.max(0, Math.min(1, (val - minVal) / (maxVal - minVal)))
  // from: #dbeafe (219, 234, 254) to: #1d4ed8 (29, 78, 216)
  const r = Math.round(219 + (29 - 219) * t)
  const g = Math.round(234 + (78 - 234) * t)
  const b = Math.round(254 + (216 - 254) * t)
  return `rgb(${r},${g},${b})`
}

const geoUrl = '/brazil-states.json'

// ---------------------------------------------------------------------------
// Reforma23UfDestino
// ---------------------------------------------------------------------------
export default function Reforma23UfDestino() {
  const [data, setData] = useState<Modulo23Response | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    const load = async () => {
      try {
        const res = await fetch('/api/reforma/modulo2/uf-destino')
        if (!res.ok) throw new Error(`Erro ${res.status}`)
        const json: Modulo23Response = await res.json()
        if (!cancelled) setData(json)
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : 'Erro desconhecido')
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    load()
    return () => { cancelled = true }
  }, [])

  const minVal = data ? Math.min(...data.rows.map(r => r.valor_total)) : 0
  const maxVal = data ? Math.max(...data.rows.map(r => r.valor_total)) : 0

  return (
    <div className="space-y-6 p-6">
      {/* Page header */}
      <div>
        <h1 className="text-xl font-semibold">Módulo 2.3 — Análise por UF de Destino</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Distribuição geográfica das saídas por estado de destino, com projeção IBS/CBS.
        </p>
      </div>

      <Separator />

      {loading ? (
        <div className="space-y-2">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-8 w-full" />
          ))}
        </div>
      ) : error ? (
        <Alert variant="destructive">
          <AlertDescription>
            {error} — Verifique sua conexão e tente novamente.
          </AlertDescription>
        </Alert>
      ) : !data || data.rows.length === 0 ? (
        <p className="text-sm text-muted-foreground text-center py-8">
          Nenhum dado de UF de destino encontrado para a empresa selecionada.
        </p>
      ) : (
        <>
          {/* Layout 2 colunas: mapa + tabela */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            {/* Mapa coroplético */}
            <Card>
              <CardHeader>
                <CardTitle className="text-base font-semibold">Mapa de Calor — Volume de Vendas por UF</CardTitle>
              </CardHeader>
              <CardContent>
                <ComposableMap
                  projection="geoMercator"
                  projectionConfig={{ center: [-54, -15], scale: 800 }}
                  style={{ width: '100%', height: 'auto' }}
                >
                  <Geographies geography={geoUrl}>
                    {({ geographies }) =>
                      geographies.map(geo => {
                        const uf = geo.properties.sigla as string
                        const row = data.rows.find(r => r.dest_uf === uf)
                        const fill = row ? colorScale(row.valor_total, minVal, maxVal) : '#e5e7eb'
                        return (
                          <Geography
                            key={geo.rsmKey}
                            geography={geo}
                            fill={fill}
                            stroke="#ffffff"
                            strokeWidth={0.5}
                            title={`${geo.properties.name}: ${row ? fmtBRL(row.valor_total) : 'Sem dados'}`}
                          />
                        )
                      })
                    }
                  </Geographies>
                </ComposableMap>
                <p className="text-xs text-muted-foreground mt-2 text-center">
                  Cor mais escura = maior volume de vendas
                </p>
              </CardContent>
            </Card>

            {/* Tabela UF */}
            <Card>
              <CardHeader>
                <CardTitle className="text-base font-semibold">Detalhamento por UF</CardTitle>
              </CardHeader>
              <CardContent className="p-0">
                <div className="overflow-x-auto">
                  <Table>
                    <TableHeader>
                      <TableRow className="hover:bg-transparent bg-muted/30">
                        <TableHead className="text-xs font-semibold uppercase tracking-wide pl-4">UF</TableHead>
                        <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">Qtd Notas</TableHead>
                        <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">Valor Total</TableHead>
                        <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">ICMS Real</TableHead>
                        <TableHead className="text-xs font-semibold uppercase tracking-wide text-right pr-4">IBS Proj</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {data.rows.map((row, idx) => (
                        <TableRow key={`${row.dest_uf}-${idx}`}>
                          <TableCell className="text-xs font-semibold pl-4">{row.dest_uf}</TableCell>
                          <TableCell className="text-xs font-mono text-right">
                            {row.qtd_notas.toLocaleString('pt-BR')}
                          </TableCell>
                          <TableCell className="text-xs font-mono text-right">{fmtBRL(row.valor_total)}</TableCell>
                          <TableCell className="text-xs font-mono text-right">{fmtBRL(row.vl_icms)}</TableCell>
                          <TableCell className="text-xs font-mono text-right pr-4">{fmtBRL(row.ibs_projetado)}</TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              </CardContent>
            </Card>
          </div>

          {/* CBS info */}
          <p className="text-xs text-muted-foreground">
            Alíq. IBS: {data.aliq_ibs_pct.toFixed(1)}% | Alíq. CBS: {data.aliq_cbs_pct.toFixed(1)}%. Dados ordenados por Valor Total DESC.
          </p>
        </>
      )}
    </div>
  )
}
