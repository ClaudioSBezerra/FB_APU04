import { useState, useEffect } from 'react'
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
import {
  PieChart,
  Pie,
  Cell,
  Tooltip as ChartTooltip,
  Legend,
  ResponsiveContainer,
} from 'recharts'
import { Info } from 'lucide-react'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------
interface Modulo24Row {
  segmento: string  // 'b2b_credit' | 'b2c' | 'sem_classificacao'
  qtd_notas: number
  valor_total: number
  ibs_projetado: number
  cbs_projetado: number
}

interface Modulo24Response {
  rows: Modulo24Row[]
  qtd_sem_ind_final: number
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

function segmentoLabel(s: string): string {
  return s === 'b2b_credit'
    ? 'B2B (Creditável)'
    : s === 'b2c'
    ? 'B2C (Consumidor Final)'
    : 'Sem Classificação'
}

const SEGMENTO_COLORS: Record<string, string> = {
  b2b_credit: '#3b82f6',
  b2c: '#22c55e',
  sem_classificacao: '#9ca3af',
}

// ---------------------------------------------------------------------------
// Reforma24B2bB2c
// ---------------------------------------------------------------------------
export default function Reforma24B2bB2c() {
  const [data, setData] = useState<Modulo24Response | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    const load = async () => {
      try {
        const res = await fetch('/api/reforma/modulo2/b2b-b2c')
        if (!res.ok) throw new Error(`Erro ${res.status}`)
        const json: Modulo24Response = await res.json()
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

  const chartData = (data?.rows ?? []).map(row => ({
    name: segmentoLabel(row.segmento),
    value: row.valor_total,
    color: SEGMENTO_COLORS[row.segmento] ?? '#9ca3af',
  }))

  return (
    <div className="space-y-6 p-6">
      {/* Page header */}
      <div>
        <h1 className="text-xl font-semibold">Módulo 2.4 — Segmentação B2B vs B2C</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Classificação das saídas por perfil de destinatário: creditável (B2B), consumidor final (B2C) e sem classificação.
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
      ) : !data ? null : (
        <>
          {/* Alerta notas sem ind_final */}
          {data.qtd_sem_ind_final > 0 && (
            <Alert variant="default" className="border-amber-300 bg-amber-50 text-amber-900 dark:bg-amber-950 dark:text-amber-200 dark:border-amber-700">
              <Info className="h-4 w-4 shrink-0" />
              <AlertDescription className="text-xs">
                <strong>{data.qtd_sem_ind_final.toLocaleString('pt-BR')} nota(s)</strong> sem campo{' '}
                <code>ind_final</code> foram classificadas pelo CPF/CNPJ do destinatário (fallback).
                Notas históricas importadas antes da Phase 6 podem ter classificação menos precisa.
              </AlertDescription>
            </Alert>
          )}

          {/* Gráfico + tabela */}
          {data.rows.length === 0 ? (
            <p className="text-sm text-muted-foreground text-center py-8">
              Nenhum dado de segmentação encontrado para a empresa selecionada.
            </p>
          ) : (
            <>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                {/* PieChart */}
                <Card>
                  <CardHeader>
                    <CardTitle className="text-base font-semibold">Distribuição por Valor Total</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <ResponsiveContainer width="100%" height={280} aria-label="Distribuição B2B vs B2C por valor total">
                      <PieChart>
                        <Pie
                          data={chartData}
                          dataKey="value"
                          nameKey="name"
                          cx="50%"
                          cy="50%"
                          outerRadius={100}
                          label={({ name, percent }) =>
                            `${name}: ${(percent * 100).toFixed(1)}%`
                          }
                          labelLine={false}
                        >
                          {chartData.map((entry, index) => (
                            <Cell key={`cell-${index}`} fill={entry.color} />
                          ))}
                        </Pie>
                        <ChartTooltip formatter={(v) => fmtBRL(Number(v))} />
                        <Legend />
                      </PieChart>
                    </ResponsiveContainer>
                  </CardContent>
                </Card>

                {/* Tabela */}
                <Card>
                  <CardHeader>
                    <CardTitle className="text-base font-semibold">Detalhamento por Segmento</CardTitle>
                  </CardHeader>
                  <CardContent className="p-0">
                    <div className="overflow-x-auto">
                      <Table>
                        <TableHeader>
                          <TableRow className="hover:bg-transparent bg-muted/30">
                            <TableHead className="text-xs font-semibold uppercase tracking-wide pl-4">Segmento</TableHead>
                            <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">Qtd Notas</TableHead>
                            <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">Valor Total</TableHead>
                            <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">IBS Proj</TableHead>
                            <TableHead className="text-xs font-semibold uppercase tracking-wide text-right pr-4">CBS Proj</TableHead>
                          </TableRow>
                        </TableHeader>
                        <TableBody>
                          {data.rows.map((row, idx) => (
                            <TableRow key={`${row.segmento}-${idx}`}>
                              <TableCell className="text-xs pl-4">
                                <span
                                  className="inline-block w-2 h-2 rounded-full mr-2"
                                  style={{ backgroundColor: SEGMENTO_COLORS[row.segmento] ?? '#9ca3af' }}
                                />
                                {segmentoLabel(row.segmento)}
                              </TableCell>
                              <TableCell className="text-xs font-mono text-right">
                                {row.qtd_notas.toLocaleString('pt-BR')}
                              </TableCell>
                              <TableCell className="text-xs font-mono text-right">{fmtBRL(row.valor_total)}</TableCell>
                              <TableCell className="text-xs font-mono text-right">{fmtBRL(row.ibs_projetado)}</TableCell>
                              <TableCell className="text-xs font-mono text-right pr-4">{fmtBRL(row.cbs_projetado)}</TableCell>
                            </TableRow>
                          ))}
                        </TableBody>
                      </Table>
                    </div>
                  </CardContent>
                </Card>
              </div>

              {/* Nota de rodapé */}
              <p className="text-xs text-muted-foreground">
                B2B Creditável: destinatário CNPJ (não Simples). B2C: ind_final=1 ou CPF destinatário.
                b2b_nocredit não determinável sem dados do regime tributário do cliente.
                Alíq. IBS: {data.aliq_ibs_pct.toFixed(1)}% | Alíq. CBS: {data.aliq_cbs_pct.toFixed(1)}%.
              </p>
            </>
          )}
        </>
      )}
    </div>
  )
}
