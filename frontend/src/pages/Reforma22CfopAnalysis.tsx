import { useState, useEffect } from 'react'
import { toast } from 'sonner'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
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
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip as ChartTooltip,
  Legend,
  ResponsiveContainer,
} from 'recharts'
import { Download } from 'lucide-react'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------
interface Modulo22Row {
  natureza_cfop: string
  qtd_notas: number
  valor_total: number
  ibs_projetado: number
  cbs_projetado: number
}

interface Modulo22Response {
  rows: Modulo22Row[]
  total_ibs: number
  total_cbs: number
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

// ---------------------------------------------------------------------------
// Reforma22CfopAnalysis
// ---------------------------------------------------------------------------
export default function Reforma22CfopAnalysis() {
  const [data, setData] = useState<Modulo22Response | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [downloadingCSV, setDownloadingCSV] = useState(false)

  useEffect(() => {
    let cancelled = false
    const load = async () => {
      try {
        const res = await fetch('/api/reforma/modulo2/cfop')
        if (!res.ok) throw new Error(`Erro ${res.status}`)
        const json: Modulo22Response = await res.json()
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

  const handleExportCSV = async () => {
    setDownloadingCSV(true)
    try {
      const res = await fetch('/api/reforma/modulo2/cfop/csv')
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = 'analise-cfop.csv'
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(url)
    } catch (err) {
      console.error('Falha ao exportar CSV:', err)
      toast.error('Erro ao exportar CSV')
    } finally {
      setDownloadingCSV(false)
    }
  }

  const chartData = data?.rows.map(row => ({
    name: row.natureza_cfop,
    'Valor Total (R$)': row.valor_total,
    'IBS Proj (R$)': row.ibs_projetado,
    'CBS Proj (R$)': row.cbs_projetado,
  })) ?? []

  return (
    <div className="space-y-6 p-6">
      {/* Page header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold">Módulo 2.2 — Análise por CFOP</h1>
          <p className="text-sm text-muted-foreground mt-1">
            CFOPs de transferência são exibidos para contexto mas excluídos do cálculo de IBS/CBS.
          </p>
        </div>
        <Button
          variant={data && data.rows.length > 0 ? 'default' : 'outline'}
          size="sm"
          onClick={handleExportCSV}
          disabled={downloadingCSV || !data || data.rows.length === 0}
          aria-label="Exportar tabela de análise CFOP como CSV"
        >
          <Download className="mr-2 h-4 w-4" />
          {downloadingCSV ? 'Exportando...' : 'Exportar CSV'}
        </Button>
      </div>

      <Separator />

      {/* KPI cards */}
      {!loading && !error && data && (
        <div className="grid grid-cols-2 gap-4">
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm text-muted-foreground">Total IBS Projetado</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-2xl font-semibold">{fmtBRL(data.total_ibs)}</p>
              <p className="text-xs text-muted-foreground mt-1">Alíq. {data.aliq_ibs_pct.toFixed(1)}%</p>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm text-muted-foreground">Total CBS Projetado</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-2xl font-semibold">{fmtBRL(data.total_cbs)}</p>
              <p className="text-xs text-muted-foreground mt-1">Alíq. {data.aliq_cbs_pct.toFixed(1)}%</p>
            </CardContent>
          </Card>
        </div>
      )}

      {/* Main card */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base font-semibold">Distribuição por Natureza CFOP</CardTitle>
        </CardHeader>
        <CardContent>
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
              Nenhum dado de CFOP encontrado para a empresa selecionada.
            </p>
          ) : (
            <>
              {/* Gráfico */}
              <ResponsiveContainer width="100%" height={300} aria-label="Análise por natureza CFOP">
                <BarChart data={chartData} margin={{ top: 4, right: 16, left: 0, bottom: 4 }}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis
                    dataKey="name"
                    tick={{ fontSize: 11 }}
                    interval={0}
                    angle={-15}
                    textAnchor="end"
                    height={60}
                  />
                  <YAxis tickFormatter={(v) => fmtBRL(v)} tick={{ fontSize: 10 }} width={110} />
                  <ChartTooltip formatter={(v) => fmtBRL(Number(v))} />
                  <Legend />
                  <Bar dataKey="Valor Total (R$)" fill="#3b82f6" />
                  <Bar dataKey="IBS Proj (R$)"    fill="#22c55e" />
                  <Bar dataKey="CBS Proj (R$)"    fill="#f97316" />
                </BarChart>
              </ResponsiveContainer>

              {/* Tabela */}
              <div className="overflow-x-auto rounded-md border mt-4">
                <Table>
                  <TableHeader>
                    <TableRow className="hover:bg-transparent bg-muted/30">
                      <TableHead className="text-xs font-semibold uppercase tracking-wide">Natureza</TableHead>
                      <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">Qtd Notas</TableHead>
                      <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">Valor Total</TableHead>
                      <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">IBS Projetado</TableHead>
                      <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">CBS Projetado</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {data.rows.map((row, idx) => (
                      <TableRow key={`${row.natureza_cfop}-${idx}`}>
                        <TableCell className="text-xs">{row.natureza_cfop}</TableCell>
                        <TableCell className="text-xs font-mono text-right">
                          {row.qtd_notas.toLocaleString('pt-BR')}
                        </TableCell>
                        <TableCell className="text-xs font-mono text-right">{fmtBRL(row.valor_total)}</TableCell>
                        <TableCell className="text-xs font-mono text-right">{fmtBRL(row.ibs_projetado)}</TableCell>
                        <TableCell className="text-xs font-mono text-right">{fmtBRL(row.cbs_projetado)}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            </>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
