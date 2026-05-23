import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
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
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { Download, Info, AlertTriangle } from 'lucide-react'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------
interface Modulo11Row {
  tipo_bloqueio: string   // 'ICMS-ST' | 'Diferido'
  tipo_cfop: string
  cfop: string
  vl_bloqueado: number
  vl_opr_total: number
  ibs_equiv: number
  cbs_equiv: number
  qtd_registros: number
}

interface Modulo11Response {
  rows: Modulo11Row[]
  total_bloqueado: number
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

const BLOQUEIO_BADGE: Record<string, { label: string; variant: 'default' | 'secondary' | 'outline' }> = {
  'ICMS-ST':  { label: 'ICMS-ST',  variant: 'default'   },
  'Diferido': { label: 'Diferido', variant: 'secondary'  },
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

  // Dados agregados por tipo para o gráfico de resumo
  const chartData = ['ICMS-ST', 'Diferido'].map(tipo => ({
    tipo,
    'Crédito Bloqueado': data?.rows.filter(r => r.tipo_bloqueio === tipo).reduce((s, r) => s + r.vl_bloqueado, 0) ?? 0,
    'Equiv. IBS': data?.rows.filter(r => r.tipo_bloqueio === tipo).reduce((s, r) => s + r.ibs_equiv, 0) ?? 0,
  }))

  return (
    <div className="space-y-6 p-6">
      {/* Page header */}
      <div className="flex items-center justify-between">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-xl font-semibold">Créditos ICMS Bloqueados — Módulo 1.1</h1>
            <TooltipProvider delayDuration={200}>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Info className="h-4 w-4 text-muted-foreground cursor-help shrink-0" />
                </TooltipTrigger>
                <TooltipContent className="max-w-xs text-left" side="bottom">
                  <p className="font-medium mb-1">O que é</p>
                  <p className="text-xs text-muted-foreground">
                    Mostra créditos de ICMS que não terão mecanismo de aproveitamento no IBS/CBS,
                    separados por tipo: <strong>ICMS-ST</strong> (pago antecipadamente nas entradas,
                    sem devolução no novo regime) e <strong>Diferido</strong> (CST 51 — créditos
                    escriturais suspensos que não serão compensados).
                  </p>
                  <p className="font-medium mb-1 mt-2">Como usar</p>
                  <p className="text-xs text-muted-foreground">
                    Compare o total bloqueado por tipo com o equivalente IBS/CBS que seria gerado
                    nas mesmas operações — a diferença é o impacto líquido real da transição para
                    cada mecanismo de bloqueio.
                  </p>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </div>
          <p className="text-sm text-muted-foreground">
            ICMS-ST nas entradas + ICMS Diferido (CST 51) sem equivalência no IBS/CBS
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

      {/* Aviso CIAP */}
      <Alert variant="default" className="border-amber-300 bg-amber-50 text-amber-900 dark:bg-amber-950 dark:text-amber-200 dark:border-amber-700">
        <AlertTriangle className="h-4 w-4" />
        <AlertDescription className="text-xs">
          <strong>CIAP (Imobilizado) não disponível.</strong> Os créditos de ICMS sobre bens do
          ativo imobilizado em apropriação parcelada requerem importação do Bloco G do EFD — ainda
          não implementado. Os valores abaixo cobrem apenas ICMS-ST e ICMS Diferido.
        </AlertDescription>
      </Alert>

      {/* KPI summary — 3 cols */}
      <div className="grid grid-cols-3 gap-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm text-muted-foreground">Total Crédito Bloqueado</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-semibold">{fmtBRL(data?.total_bloqueado)}</p>
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
          <CardTitle className="text-base font-semibold">Créditos por Tipo e CFOP</CardTitle>
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
              Nenhum crédito ICMS-ST ou Diferido encontrado nas entradas do período.
            </p>
          ) : (
            <>
              {/* Gráfico de resumo por tipo */}
              <ResponsiveContainer
                width="100%"
                height={240}
                aria-label="Comparativo de créditos bloqueados por tipo"
              >
                <BarChart data={chartData} margin={{ top: 4, right: 16, left: 0, bottom: 4 }}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis dataKey="tipo" tick={{ fontSize: 13 }} />
                  <YAxis tickFormatter={(v) => fmtBRL(v)} tick={{ fontSize: 11 }} width={110} />
                  <ChartTooltip formatter={(v) => fmtBRL(Number(v))} />
                  <Legend />
                  <Bar dataKey="Crédito Bloqueado" fill="var(--pis-cofins)" />
                  <Bar dataKey="Equiv. IBS"         fill="var(--ibs-cbs)" />
                </BarChart>
              </ResponsiveContainer>

              {/* Tabela de detalhe */}
              <div className="overflow-x-auto rounded-md border mt-4">
                <Table>
                  <TableHeader>
                    <TableRow className="hover:bg-transparent bg-muted/30">
                      <TableHead className="text-xs font-semibold uppercase tracking-wide">
                        Tipo de Crédito
                      </TableHead>
                      <TableHead className="text-xs font-semibold uppercase tracking-wide">
                        Tipo CFOP
                      </TableHead>
                      <TableHead className="text-xs font-semibold uppercase tracking-wide">
                        CFOP
                      </TableHead>
                      <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">
                        Valor Bloqueado (R$)
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
                        Qtd
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {data.rows.map((row, idx) => {
                      const badge = BLOQUEIO_BADGE[row.tipo_bloqueio] ?? { label: row.tipo_bloqueio, variant: 'outline' as const }
                      return (
                        <TableRow key={`${row.tipo_bloqueio}-${row.cfop}-${idx}`}>
                          <TableCell className="text-xs">
                            <Badge variant={badge.variant}>{badge.label}</Badge>
                          </TableCell>
                          <TableCell className="text-xs">{row.tipo_cfop || '—'}</TableCell>
                          <TableCell className="text-xs font-mono">{row.cfop}</TableCell>
                          <TableCell className="text-xs font-mono text-right">
                            {fmtBRL(row.vl_bloqueado)}
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
                      )
                    })}
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
