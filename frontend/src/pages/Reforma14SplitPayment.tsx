import { useQuery } from '@tanstack/react-query'
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
import { Info } from 'lucide-react'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------
interface Modulo14Response {
  float_tributario: number
  custo_cdi: number
  total_saidas: number
  aliq_total: number
  taxa_cdi_anual_pct: number
  prazo_medio_dias: number
  cdi_colunas: number[]
  dso_linhas: number[]
  sensibilidade: { dso: number; custos: number[] }[]
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
function fmtBRL(v: number | null | undefined): string {
  if (v == null) return '—'
  return v.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' })
}

// ---------------------------------------------------------------------------
// Reforma14SplitPayment
// ---------------------------------------------------------------------------
export default function Reforma14SplitPayment() {
  const { data, isLoading, isError } = useQuery<Modulo14Response>({
    queryKey: ['reforma/modulo1/split'],
    queryFn: async () => {
      const res = await fetch('/api/reforma/modulo1/split')
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      return res.json()
    },
  })

  const isEmpty =
    !isLoading && !isError && (!data || data.total_saidas === 0)

  return (
    <div className="space-y-6 p-6">
      {/* Page header — sem botão Exportar CSV por design */}
      <div>
        <h1 className="text-xl font-semibold">Split Payment e Capital de Giro — Módulo 1.4</h1>
        <p className="text-sm text-muted-foreground">
          Float tributário e custo de reposição CDI com tabela de sensibilidade DSO × CDI
        </p>
      </div>

      <Separator />

      {/* KPI cards — 2 cols */}
      <div className="grid grid-cols-2 gap-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm text-muted-foreground">Float Tributário</CardTitle>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <Skeleton className="h-8 w-32" />
            ) : (
              <>
                <p className="text-2xl font-semibold">{fmtBRL(data?.float_tributario)}</p>
                <p className="text-sm text-muted-foreground">
                  IBS+CBS × Saídas × {data?.prazo_medio_dias ?? 30} dias / 365
                </p>
              </>
            )}
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm text-muted-foreground">
              Custo CDI Estimado (R$/ano)
            </CardTitle>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <Skeleton className="h-8 w-32" />
            ) : (
              <>
                <p className="text-2xl font-semibold">{fmtBRL(data?.custo_cdi)}</p>
                <p className="text-sm text-muted-foreground">
                  Float × {data?.taxa_cdi_anual_pct ?? 10.5}% CDI
                </p>
              </>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Disclaimer regulatório */}
      <Alert variant="default">
        <Info className="h-4 w-4" />
        <AlertDescription>
          Split payment entra em vigor gradualmente entre 2026 e 2033 conforme cronograma da
          Reforma. Os valores simulam o impacto no regime de transição plena.
        </AlertDescription>
      </Alert>

      {/* Sensitivity matrix */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base font-semibold">
            Tabela de Sensibilidade — DSO × CDI
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
          ) : isEmpty ? (
            <p className="text-sm text-muted-foreground text-center py-8">
              Nenhuma nota fiscal de saída encontrada. Os módulos de split payment requerem dados
              de NF-e de saída importados via XML.
            </p>
          ) : (
            <div className="overflow-x-auto rounded-md border">
              <Table>
                <TableHeader>
                  <TableRow className="hover:bg-transparent bg-muted/30">
                    <TableHead className="text-xs font-semibold uppercase tracking-wide">
                      DSO (dias)
                    </TableHead>
                    {(data?.cdi_colunas ?? []).map((cdi) => (
                      <TableHead
                        key={cdi}
                        className="text-xs font-semibold uppercase tracking-wide text-right"
                      >
                        CDI {cdi}%
                      </TableHead>
                    ))}
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {(data?.sensibilidade ?? []).map((senRow) => (
                    <TableRow key={senRow.dso}>
                      <TableCell className="text-xs font-mono">{senRow.dso}</TableCell>
                      {senRow.custos.map((custo, i) => {
                        const cdi = data?.cdi_colunas[i]
                        const isCurrentCell =
                          senRow.dso === data?.prazo_medio_dias &&
                          Math.abs((cdi ?? 0) - (data?.taxa_cdi_anual_pct ?? 0)) < 0.001
                        return (
                          <TableCell
                            key={i}
                            className={`text-xs font-mono text-right${isCurrentCell ? ' bg-primary/10 font-semibold' : ''}`}
                            aria-current={isCurrentCell ? 'true' : undefined}
                          >
                            {fmtBRL(custo)}
                          </TableCell>
                        )
                      })}
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
