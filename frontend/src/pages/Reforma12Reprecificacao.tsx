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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { Download, Info } from 'lucide-react'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------
interface Modulo12Row {
  ncm: string
  x_prod: string
  cst_icms: string
  cst_path: string
  preco_atual: number
  icms_atual: number
  ibs_projetado: number
  cbs_projetado: number
  preco_sugerido: number
  variacao_pct: number
}

interface Modulo12Response {
  rows: Modulo12Row[]
  aliq_ibs_pct: number
  aliq_cbs_pct: number
  ano: number
  anos_disponiveis: number[]
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
function fmtBRL(v: number | null | undefined): string {
  if (v == null) return '—'
  return v.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' })
}

function CSTBadge({ cst }: { cst: string | null | undefined }) {
  if (!cst) return <span className="text-xs text-muted-foreground">—</span>
  const isNormal = cst === '00'
  const isST = ['10', '30', '60', '70'].includes(cst)
  return (
    <Badge
      variant={isNormal ? 'secondary' : isST ? 'outline' : 'default'}
      className="font-mono text-xs"
    >
      {cst}
    </Badge>
  )
}

function fmtVariacao(v: number | null | undefined): React.ReactNode {
  if (v == null) return <span className="text-xs text-muted-foreground">—</span>
  const positive = v > 0
  const neutral = v === 0
  return (
    <span
      className={`text-xs font-mono ${neutral ? 'text-muted-foreground' : positive ? 'text-green-600' : 'text-red-600'}`}
    >
      {positive ? '+' : ''}
      {v.toFixed(2)}%
    </span>
  )
}

// ---------------------------------------------------------------------------
// Reforma12Reprecificacao
// ---------------------------------------------------------------------------
export default function Reforma12Reprecificacao() {
  const [downloadingCSV, setDownloadingCSV] = useState(false)
  const [cstFilter, setCstFilter] = useState('todos')
  const [anoBase, setAnoBase] = useState<string>('') // '' = usa default do backend

  const anoQS = anoBase ? `?ano=${anoBase}` : ''

  const { data, isLoading, isError } = useQuery<Modulo12Response>({
    queryKey: ['reforma/modulo1/reprecificacao', anoBase],
    queryFn: async () => {
      const res = await fetch(`/api/reforma/modulo1/reprecificacao${anoQS}`)
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      return res.json()
    },
  })

  const handleExportCSV = async () => {
    setDownloadingCSV(true)
    try {
      const res = await fetch(`/api/reforma/modulo1/reprecificacao/csv${anoQS}`)
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = 'reprecificacao-produtos.csv'
      a.click()
      URL.revokeObjectURL(url)
    } catch (_err) {
      // silent
    } finally {
      setDownloadingCSV(false)
    }
  }

  // Client-side CST filter
  const filteredRows =
    data && cstFilter !== 'todos'
      ? data.rows.filter((row) => {
          if (cstFilter === 'st') {
            return row.cst_path === 'st'
          }
          if (cstFilter === 'base_reduzida') {
            return row.cst_path === 'base_reduzida'
          }
          return row.cst_path === cstFilter
        })
      : data?.rows ?? []

  const hasData = data && data.rows.length > 0

  return (
    <div className="space-y-6 p-6">
      {/* Page header */}
      <div className="flex items-center justify-between">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-xl font-semibold">Reprecificação de Produtos — Módulo 1.2</h1>
            <TooltipProvider delayDuration={200}>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Info className="h-4 w-4 text-muted-foreground cursor-help shrink-0" />
                </TooltipTrigger>
                <TooltipContent className="max-w-xs text-left" side="bottom">
                  <p className="font-medium mb-1">O que é</p>
                  <p className="text-xs text-muted-foreground">
                    Compara o ICMS embutido no preço atual de cada produto com o IBS/CBS projetado
                    no novo regime, por NCM e CST. O ICMS é "por dentro" (compõe a base); o IBS/CBS
                    é "por fora" (incide sobre o preço líquido).
                  </p>
                  <p className="font-medium mb-1 mt-2">Como usar</p>
                  <p className="text-xs text-muted-foreground">
                    Ordene pela coluna Variação % para ver quais produtos encarecem ou barateiam.
                    Ajuste o ano-alvo nos Parâmetros para simular diferentes momentos da transição
                    (2027–2033).
                  </p>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </div>
          <p className="text-sm text-muted-foreground">
            Impacto da troca ICMS por dentro por IBS/CBS por fora por produto
          </p>
        </div>
        <Button
          variant={hasData ? 'default' : 'outline'}
          size="sm"
          onClick={handleExportCSV}
          disabled={downloadingCSV || !hasData}
          aria-label="Exportar tabela de reprecificação como CSV"
        >
          <Download className="mr-2 h-4 w-4" />
          {downloadingCSV ? 'Exportando...' : 'Exportar CSV'}
        </Button>
      </div>

      <Separator />

      {/* Main card */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base font-semibold">Produtos por CST ICMS</CardTitle>
        </CardHeader>
        <CardContent>
          {/* Filter row */}
          <div className="flex items-center gap-4 flex-wrap mb-4">
            <div className="flex items-center gap-2">
              <span className="text-xs text-muted-foreground whitespace-nowrap">Ano-base:</span>
              <Select
                value={anoBase || (data?.ano ? String(data.ano) : '')}
                onValueChange={(v) => setAnoBase(v)}
              >
                <SelectTrigger className="h-8 w-28">
                  <SelectValue placeholder="Ano" />
                </SelectTrigger>
                <SelectContent>
                  {(data?.anos_disponiveis ?? []).map((a) => (
                    <SelectItem key={a} value={String(a)}>{a}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {data && (
                <span className="text-xs text-muted-foreground">
                  IBS {data.aliq_ibs_pct.toFixed(2)}% + CBS {data.aliq_cbs_pct.toFixed(2)}%
                </span>
              )}
            </div>
            <Select value={cstFilter} onValueChange={setCstFilter}>
              <SelectTrigger className="h-8 w-52">
                <SelectValue placeholder="Filtrar por CST" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="todos">Todos</SelectItem>
                <SelectItem value="normal">Normal (00)</SelectItem>
                <SelectItem value="st">Substituição Tributária</SelectItem>
                <SelectItem value="base_reduzida">Base Reduzida</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <Separator className="mb-4" />

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
            <div className="flex flex-col items-center gap-3 py-10 text-center">
              <p className="text-sm font-medium text-foreground">
                Nenhum produto encontrado
              </p>
              <p className="text-xs text-muted-foreground max-w-md">
                Este módulo requer NF-e importadas via upload de XML. Dados sincronizados
                pelo ERP Bridge (SAP/Oracle) não incluem itens por produto — apenas totais
                por nota. Importe os XMLs das notas de entrada para visualizar a
                reprecificação produto a produto.
              </p>
            </div>
          ) : (
            <div className="overflow-x-auto rounded-md border">
              <Table>
                <TableHeader>
                  <TableRow className="hover:bg-transparent bg-muted/30">
                    <TableHead className="text-xs font-semibold uppercase tracking-wide">
                      NCM
                    </TableHead>
                    <TableHead className="text-xs font-semibold uppercase tracking-wide">
                      Descrição Produto
                    </TableHead>
                    <TableHead className="text-xs font-semibold uppercase tracking-wide">
                      CST ICMS
                    </TableHead>
                    <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">
                      Preço Atual (R$)
                    </TableHead>
                    <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">
                      ICMS Atual (R$)
                    </TableHead>
                    <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">
                      IBS Projetado (R$)
                    </TableHead>
                    <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">
                      CBS Projetado (R$)
                    </TableHead>
                    <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">
                      Preço Sugerido (R$)
                    </TableHead>
                    <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">
                      Variação (%)
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filteredRows.map((row, idx) => (
                    <TableRow key={`${row.ncm}-${idx}`}>
                      <TableCell className="text-xs font-mono">{row.ncm || '—'}</TableCell>
                      <TableCell className="text-xs max-w-[200px] truncate">
                        {row.x_prod || '—'}
                      </TableCell>
                      <TableCell>
                        <CSTBadge cst={row.cst_icms} />
                      </TableCell>
                      <TableCell className="text-xs font-mono text-right">
                        {fmtBRL(row.preco_atual)}
                      </TableCell>
                      <TableCell className="text-xs font-mono text-right">
                        {fmtBRL(row.icms_atual)}
                      </TableCell>
                      <TableCell className="text-xs font-mono text-right">
                        {row.cst_icms ? fmtBRL(row.ibs_projetado) : '—'}
                      </TableCell>
                      <TableCell className="text-xs font-mono text-right">
                        {row.cst_icms ? fmtBRL(row.cbs_projetado) : '—'}
                      </TableCell>
                      <TableCell className="text-xs font-mono text-right font-semibold">
                        {row.cst_icms ? fmtBRL(row.preco_sugerido) : '—'}
                      </TableCell>
                      <TableCell className="text-right">
                        {fmtVariacao(row.variacao_pct)}
                      </TableCell>
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
