import { useState, useEffect } from 'react'
import { toast } from 'sonner'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
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
interface Modulo21Row {
  ncm: string
  x_prod: string
  vl_prod: number
  vl_icms: number
  aliq_icms_efet: number
  ibs_projetado: number
  cbs_projetado: number
  is_flag: boolean
}

interface Modulo21Response {
  rows: Modulo21Row[]
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
// Reforma21NcmAnalysis
// ---------------------------------------------------------------------------
export default function Reforma21NcmAnalysis() {
  const [data, setData] = useState<Modulo21Response | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [downloadingCSV, setDownloadingCSV] = useState(false)

  useEffect(() => {
    let cancelled = false
    const load = async () => {
      try {
        const res = await fetch('/api/reforma/modulo2/ncm')
        if (!res.ok) throw new Error(`Erro ${res.status}`)
        const json: Modulo21Response = await res.json()
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
      const res = await fetch('/api/reforma/modulo2/ncm/csv')
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = 'analise-ncm.csv'
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

  // Top 10 NCMs para o gráfico (truncar NCM a 8 chars)
  const chartData = (data?.rows ?? []).slice(0, 10).map(row => ({
    name: row.ncm.substring(0, 8),
    'ICMS Atual (R$)': row.vl_icms,
    'IBS+CBS Proj (R$)': row.ibs_projetado + row.cbs_projetado,
  }))

  return (
    <div className="space-y-6 p-6">
      {/* Page header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold">Módulo 2.1 — Análise por NCM</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Comparativo ICMS atual vs IBS+CBS projetado por NCM. Limitado aos 100 NCMs de maior volume.
          </p>
        </div>
        <Button
          variant={data && data.rows.length > 0 ? 'default' : 'outline'}
          size="sm"
          onClick={handleExportCSV}
          disabled={downloadingCSV || !data || data.rows.length === 0}
          aria-label="Exportar tabela de análise NCM como CSV"
        >
          <Download className="mr-2 h-4 w-4" />
          {downloadingCSV ? 'Exportando...' : 'Exportar CSV'}
        </Button>
      </div>

      <Separator />

      {/* Main card */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base font-semibold">NCMs — ICMS Atual vs IBS+CBS Projetado</CardTitle>
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
              Nenhum dado de NCM encontrado para a empresa selecionada.
            </p>
          ) : (
            <>
              {/* Gráfico top 10 */}
              <ResponsiveContainer width="100%" height={300} aria-label="ICMS atual vs IBS+CBS projetado por NCM (top 10)">
                <BarChart data={chartData} margin={{ top: 4, right: 16, left: 0, bottom: 4 }}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis dataKey="name" tick={{ fontSize: 11 }} />
                  <YAxis tickFormatter={(v) => fmtBRL(v)} tick={{ fontSize: 10 }} width={110} />
                  <ChartTooltip formatter={(v) => fmtBRL(Number(v))} />
                  <Legend />
                  <Bar dataKey="ICMS Atual (R$)"    fill="#3b82f6" />
                  <Bar dataKey="IBS+CBS Proj (R$)"  fill="#22c55e" />
                </BarChart>
              </ResponsiveContainer>

              {/* Tabela */}
              <div className="overflow-x-auto rounded-md border mt-4">
                <Table>
                  <TableHeader>
                    <TableRow className="hover:bg-transparent bg-muted/30">
                      <TableHead className="text-xs font-semibold uppercase tracking-wide">NCM</TableHead>
                      <TableHead className="text-xs font-semibold uppercase tracking-wide">Descrição do Produto</TableHead>
                      <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">VL Prod</TableHead>
                      <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">VL ICMS</TableHead>
                      <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">Alíq ICMS Efet (%)</TableHead>
                      <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">IBS Proj (R$)</TableHead>
                      <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">CBS Proj (R$)</TableHead>
                      <TableHead className="text-xs font-semibold uppercase tracking-wide text-center">IS</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {data.rows.map((row, idx) => (
                      <TableRow key={`${row.ncm}-${idx}`}>
                        <TableCell className="text-xs font-mono">{row.ncm}</TableCell>
                        <TableCell className="text-xs max-w-[200px] truncate" title={row.x_prod}>
                          {row.x_prod || '—'}
                        </TableCell>
                        <TableCell className="text-xs font-mono text-right">{fmtBRL(row.vl_prod)}</TableCell>
                        <TableCell className="text-xs font-mono text-right">{fmtBRL(row.vl_icms)}</TableCell>
                        <TableCell className="text-xs font-mono text-right">
                          {row.aliq_icms_efet.toFixed(1)}%
                        </TableCell>
                        <TableCell className="text-xs font-mono text-right">{fmtBRL(row.ibs_projetado)}</TableCell>
                        <TableCell className="text-xs font-mono text-right">{fmtBRL(row.cbs_projetado)}</TableCell>
                        <TableCell className="text-xs text-center">
                          {row.is_flag
                            ? <Badge variant="destructive">IS</Badge>
                            : <span className="text-muted-foreground">—</span>
                          }
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            </>
          )}
        </CardContent>
      </Card>

      {/* Nota de rodapé */}
      <p className="text-xs text-muted-foreground">
        Limitado aos 100 NCMs de maior volume. IS = Imposto Seletivo (conforme ncm_cclasstrib_reforma).
        Alíq. IBS: {data?.aliq_ibs_pct.toFixed(1) ?? '—'}% | Alíq. CBS: {data?.aliq_cbs_pct.toFixed(1) ?? '—'}%.
      </p>
    </div>
  )
}
