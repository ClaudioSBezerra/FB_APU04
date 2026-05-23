import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
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
  Tooltip,
  Legend,
  ResponsiveContainer,
} from 'recharts'
import { Download } from 'lucide-react'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------
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

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
function fmtBRL(v: number | null | undefined): string {
  if (v == null) return '—'
  return v.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' })
}

function fmtCNPJ(v: string): string {
  if (!v || v.length !== 14) return v || '—'
  return `${v.slice(0, 2)}.${v.slice(2, 5)}.${v.slice(5, 8)}/${v.slice(8, 12)}-${v.slice(12)}`
}

// ---------------------------------------------------------------------------
// Reforma11CreditosBloqueados
// ---------------------------------------------------------------------------
export default function Reforma11CreditosBloqueados() {
  const [downloadingCSV, setDownloadingCSV] = useState(false)

  const { data, isLoading, isError } = useQuery<Modulo11Response>({
    queryKey: ['reforma/modulo1/creditos'],
    queryFn: async () => {
      const res = await fetch('/api/reforma/modulo1/creditos')
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      return res.json()
    },
  })

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
    } catch (_err) {
      // silent — CSV errors shown via disabled state
    } finally {
      setDownloadingCSV(false)
    }
  }

  const hasData = data && data.rows.length > 0

  return (
    <div className="space-y-6 p-6">
      {/* Page header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold">Créditos ICMS Bloqueados — Módulo 1.1</h1>
          <p className="text-sm text-muted-foreground">
            Créditos ICMS não aproveitáveis na transição + equivalente IBS/CBS recuperável
          </p>
        </div>
        <Button
          variant={hasData ? 'default' : 'outline'}
          size="sm"
          onClick={handleExportCSV}
          disabled={downloadingCSV || !hasData}
          aria-label="Exportar tabela de créditos como CSV"
        >
          <Download className="mr-2 h-4 w-4" />
          {downloadingCSV ? 'Exportando...' : 'Exportar CSV'}
        </Button>
      </div>

      <Separator />

      {/* KPI summary — 3 cols */}
      <div className="grid grid-cols-3 gap-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm text-muted-foreground">Total ICMS Bloqueado</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-semibold">{fmtBRL(data?.total_icms)}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm text-muted-foreground">Total Equiv. IBS</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-semibold">{fmtBRL(data?.total_ibs)}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm text-muted-foreground">Total Equiv. CBS</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-semibold">{fmtBRL(data?.total_cbs)}</p>
          </CardContent>
        </Card>
      </div>

      {/* Main card: chart + table */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base font-semibold">Créditos por CFOP</CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-2">
              {Array.from({ length: 6 }).map((_, i) => (
                <Skeleton key={i} className="h-8 w-full" />
              ))}
            </div>
          ) : isError ? (
            <Alert variant="destructive">
              <AlertDescription>
                Erro ao carregar dados. Verifique sua conexão e tente novamente.
              </AlertDescription>
            </Alert>
          ) : !data || data.rows.length === 0 ? (
            <p className="text-sm text-muted-foreground text-center py-8">
              Nenhum crédito ICMS encontrado para o período selecionado.
            </p>
          ) : (
            <>
              {/* Bar chart */}
              <ResponsiveContainer
                width="100%"
                height={280}
                aria-label="Gráfico de créditos ICMS bloqueados por CFOP"
              >
                <BarChart data={data.rows}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis dataKey="cfop" tick={{ fontSize: 12 }} />
                  <YAxis tickFormatter={(v) => fmtBRL(v)} tick={{ fontSize: 12 }} />
                  <Tooltip formatter={(v) => fmtBRL(Number(v))} />
                  <Legend />
                  <Bar dataKey="vl_icms_total" name="ICMS Bloqueado" fill="var(--pis-cofins)" />
                  <Bar dataKey="ibs_equiv" name="Equiv. IBS" fill="var(--ibs-cbs)" />
                </BarChart>
              </ResponsiveContainer>

              {/* Table */}
              <div className="overflow-x-auto rounded-md border mt-4">
                <Table>
                  <TableHeader>
                    <TableRow className="hover:bg-transparent bg-muted/30">
                      <TableHead className="text-xs font-semibold uppercase tracking-wide">
                        Tipo CFOP
                      </TableHead>
                      <TableHead className="text-xs font-semibold uppercase tracking-wide">
                        CFOP
                      </TableHead>
                      <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">
                        ICMS Bloqueado (R$)
                      </TableHead>
                      <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">
                        Valor Operação (R$)
                      </TableHead>
                      <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">
                        IBS Equiv. (R$)
                      </TableHead>
                      <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">
                        CBS Equiv. (R$)
                      </TableHead>
                      <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">
                        Qtd Registros
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {data.rows.map((row, idx) => (
                      <TableRow key={`${row.cfop}-${idx}`}>
                        <TableCell className="text-xs">{row.tipo_cfop || '—'}</TableCell>
                        <TableCell className="text-xs font-mono">{row.cfop}</TableCell>
                        <TableCell className="text-xs font-mono text-right">
                          {fmtBRL(row.vl_icms_total)}
                        </TableCell>
                        <TableCell className="text-xs font-mono text-right">
                          {fmtBRL(row.vl_opr_total)}
                        </TableCell>
                        <TableCell className="text-xs font-mono text-right">
                          {fmtBRL(row.ibs_equiv)}
                        </TableCell>
                        <TableCell className="text-xs font-mono text-right">
                          {fmtBRL(row.cbs_equiv)}
                        </TableCell>
                        <TableCell className="text-xs font-mono text-right">
                          {row.qtd_registros.toLocaleString('pt-BR')}
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
    </div>
  )
}

