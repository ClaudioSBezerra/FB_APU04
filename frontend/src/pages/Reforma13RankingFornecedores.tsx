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
import { Download, AlertTriangle, Info } from 'lucide-react'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------
interface Modulo13Row {
  forn_cnpj: string
  forn_nome: string
  qtd_notas: number
  valor_total: number
  ibs_perdido_est: number
  cbs_perdido_est: number
  simples: boolean
}

interface Modulo13Response {
  rows: Modulo13Row[]
  fator_simples_pct: number
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
// Reforma13RankingFornecedores
// ---------------------------------------------------------------------------
export default function Reforma13RankingFornecedores() {
  const [downloadingCSV, setDownloadingCSV] = useState(false)

  const { data, isLoading, isError } = useQuery<Modulo13Response>({
    queryKey: ['reforma/modulo1/ranking'],
    queryFn: async () => {
      const res = await fetch('/api/reforma/modulo1/ranking')
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      return res.json()
    },
  })

  const handleExportCSV = async () => {
    setDownloadingCSV(true)
    try {
      const res = await fetch('/api/reforma/modulo1/ranking/csv')
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = 'ranking-fornecedores-simples.csv'
      a.click()
      URL.revokeObjectURL(url)
    } catch (_err) {
      // silent
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
          <div className="flex items-center gap-2">
            <h1 className="text-xl font-semibold">Ranking de Fornecedores — Módulo 1.3</h1>
            <TooltipProvider delayDuration={200}>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Info className="h-4 w-4 text-muted-foreground cursor-help shrink-0" />
                </TooltipTrigger>
                <TooltipContent className="max-w-xs text-left" side="bottom">
                  <p className="font-medium mb-1">O que é</p>
                  <p className="text-xs text-muted-foreground">
                    Lista os fornecedores optantes pelo Simples Nacional ordenados por volume de
                    compras. No novo regime o tomador não pode creditar o IBS/CBS desses
                    fornecedores — o crédito estimado representa um custo adicional não recuperável.
                  </p>
                  <p className="font-medium mb-1 mt-2">Como usar</p>
                  <p className="text-xs text-muted-foreground">
                    Priorize negociações de preço com os fornecedores de maior IBS/CBS perdido, ou
                    avalie substituição por fornecedores do regime regular que gerem crédito pleno.
                    O Fator Simples (Parâmetros) ajusta a estimativa de crédito não aproveitável.
                  </p>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </div>
          <p className="text-sm text-muted-foreground">
            Fornecedores Simples Nacional com crédito IBS/CBS estimado por empresa
          </p>
        </div>
        <Button
          variant={hasData ? 'default' : 'outline'}
          size="sm"
          onClick={handleExportCSV}
          disabled={downloadingCSV || !hasData}
          aria-label="Exportar ranking de fornecedores como CSV"
        >
          <Download className="mr-2 h-4 w-4" />
          {downloadingCSV ? 'Exportando...' : 'Exportar CSV'}
        </Button>
      </div>

      <Separator />

      {/* Disclaimer regulatório — RFMB-02 obrigatório */}
      <Alert variant="default" className="border-warning text-warning-foreground bg-warning/10">
        <AlertTriangle className="h-4 w-4" />
        <AlertDescription>
          Valores estimados. A alíquota definitiva do Fator Simples Nacional não foi publicada
          pelo CG-IBS. Use como referência indicativa.
        </AlertDescription>
      </Alert>

      {/* Main card: chart + table */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base font-semibold">
            Ranking por IBS+CBS Estimado Perdido
          </CardTitle>
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
              Nenhum fornecedor Simples Nacional encontrado. Verifique se a tabela forn_simples está populada.
            </p>
          ) : (
            <>
              {/* Bar chart top 10 */}
              <ResponsiveContainer
                width="100%"
                height={220}
                aria-label="Ranking de fornecedores Simples Nacional"
              >
                <BarChart data={data.rows.slice(0, 10)}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis
                    dataKey="forn_cnpj"
                    tick={{ fontSize: 11 }}
                    tickFormatter={(v: string) => v.slice(0, 12)}
                  />
                  <YAxis tickFormatter={(v) => fmtBRL(v)} tick={{ fontSize: 11 }} />
                  <ChartTooltip formatter={(v) => fmtBRL(Number(v))} />
                  <Legend />
                  <Bar dataKey="ibs_perdido_est" name="IBS+CBS Estimado" fill="var(--ibs-cbs)" />
                </BarChart>
              </ResponsiveContainer>

              {/* Table */}
              <div className="overflow-x-auto rounded-md border mt-4">
                <Table>
                  <TableHeader>
                    <TableRow className="hover:bg-transparent bg-muted/30">
                      <TableHead className="text-xs font-semibold uppercase tracking-wide">
                        #
                      </TableHead>
                      <TableHead className="text-xs font-semibold uppercase tracking-wide">
                        CNPJ
                      </TableHead>
                      <TableHead className="text-xs font-semibold uppercase tracking-wide">
                        Nome Fornecedor
                      </TableHead>
                      <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">
                        Qtd Notas
                      </TableHead>
                      <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">
                        Valor Total (R$)
                      </TableHead>
                      <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">
                        IBS Estimado (R$)
                      </TableHead>
                      <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">
                        CBS Estimado (R$)
                      </TableHead>
                      <TableHead className="text-xs font-semibold uppercase tracking-wide">
                        Simples Nacional
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {data.rows.map((row, idx) => (
                      <TableRow key={`${row.forn_cnpj}-${idx}`}>
                        <TableCell className="text-xs font-mono">{idx + 1}</TableCell>
                        <TableCell className="text-xs font-mono">
                          {fmtCNPJ(row.forn_cnpj)}
                        </TableCell>
                        <TableCell className="text-xs">{row.forn_nome || '—'}</TableCell>
                        <TableCell className="text-xs font-mono text-right">
                          {row.qtd_notas.toLocaleString('pt-BR')}
                        </TableCell>
                        <TableCell className="text-xs font-mono text-right">
                          {fmtBRL(row.valor_total)}
                        </TableCell>
                        <TableCell className="text-xs font-mono text-right">
                          {fmtBRL(row.ibs_perdido_est)}
                        </TableCell>
                        <TableCell className="text-xs font-mono text-right">
                          {fmtBRL(row.cbs_perdido_est)}
                        </TableCell>
                        <TableCell>
                          {row.simples ? (
                            <Badge
                              variant="outline"
                              className="text-xs px-1.5 py-0 bg-yellow-50 text-yellow-700 border-yellow-200"
                            >
                              Simples
                            </Badge>
                          ) : null}
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
