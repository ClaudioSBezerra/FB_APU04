import { useNavigate, useLocation } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { Info, TrendingUp, AlertTriangle, ArrowLeftRight } from 'lucide-react'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------
interface FronteiraResumoRow {
  regime: string
  qtd_notas: number
  v_prod_total: number
  v_st_retido: number
  icms_devido_est: number
}

interface FronteiraResumoResponse {
  rows: FronteiraResumoRow[]
  total_devido: number
  total_prod: number
}

interface FronteiraNotaRow {
  chave_nfe: string
  data_emissao: string
  numero_nfe: string
  forn_cnpj: string
  forn_nome: string
  forn_uf: string
  cfop: string
  v_prod: number
  v_icms: number
  v_bc_st: number
  v_st: number
  aliq_inter: number
  aliq_interna: number
  icms_devido_est: number
  regime: string
}

interface FronteiraNotasResponse {
  rows: FronteiraNotaRow[]
  total: number
  count: number
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
function fmtBRL(v: number | null | undefined): string {
  if (v == null) return '—'
  return v.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' })
}

function fmtPct(v: number | null | undefined): string {
  if (v == null) return '—'
  return v.toFixed(1) + '%'
}

function RegimeBadge({ regime }: { regime: string }) {
  const map: Record<string, { label: string; variant: 'default' | 'secondary' | 'outline' | 'destructive' }> = {
    ANTECIPACAO: { label: 'Antecipação', variant: 'default' },
    ST:          { label: 'ST',          variant: 'outline' },
    DIFAL:       { label: 'DIFAL',       variant: 'secondary' },
  }
  const cfg = map[regime] ?? { label: regime, variant: 'secondary' as const }
  return (
    <Badge variant={cfg.variant} className="font-mono text-xs">
      {cfg.label}
    </Badge>
  )
}

function formatCNPJ(cnpj: string): string {
  if (!cnpj || cnpj.length !== 14) return cnpj || '—'
  return cnpj.replace(/(\d{2})(\d{3})(\d{3})(\d{4})(\d{2})/, '$1.$2.$3/$4-$5')
}

// ---------------------------------------------------------------------------
// Resumo cards
// ---------------------------------------------------------------------------
function ResumoTab() {
  const { data, isLoading, isError } = useQuery<FronteiraResumoResponse>({
    queryKey: ['icms-fronteira/resumo'],
    queryFn: async () => {
      const res = await fetch('/api/icms-fronteira/resumo')
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      return res.json()
    },
  })

  if (isLoading) {
    return (
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {[0, 1, 2].map((i) => <Skeleton key={i} className="h-28 w-full" />)}
      </div>
    )
  }

  if (isError) {
    return (
      <Alert variant="destructive">
        <AlertDescription>Erro ao carregar resumo ICMS Fronteira.</AlertDescription>
      </Alert>
    )
  }

  if (!data || data.rows.length === 0) {
    return <EmptyState />
  }

  const regimeIcons: Record<string, React.ReactNode> = {
    ANTECIPACAO: <TrendingUp className="h-5 w-5 text-blue-500" />,
    ST:          <AlertTriangle className="h-5 w-5 text-amber-500" />,
    DIFAL:       <ArrowLeftRight className="h-5 w-5 text-purple-500" />,
  }

  const regimeLabels: Record<string, string> = {
    ANTECIPACAO: 'Antecipação ICMS',
    ST:          'Substituição Tributária',
    DIFAL:       'Diferencial de Alíquota',
  }

  return (
    <div className="space-y-6">
      {/* KPI cards */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {data.rows.map((row) => (
          <Card key={row.regime}>
            <CardContent className="pt-5 pb-4">
              <div className="flex items-center justify-between mb-3">
                <span className="text-sm font-medium text-muted-foreground">
                  {regimeLabels[row.regime] ?? row.regime}
                </span>
                {regimeIcons[row.regime]}
              </div>
              <p className="text-2xl font-bold tabular-nums">
                {fmtBRL(row.icms_devido_est)}
              </p>
              <p className="text-xs text-muted-foreground mt-1">
                {row.qtd_notas} nota{row.qtd_notas !== 1 ? 's' : ''} •{' '}
                Prod.: {fmtBRL(row.v_prod_total)}
              </p>
            </CardContent>
          </Card>
        ))}
      </div>

      {/* Total */}
      <Card className="bg-muted/30">
        <CardContent className="pt-4 pb-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-muted-foreground">Total ICMS Fronteira estimado</p>
              <p className="text-3xl font-bold tabular-nums mt-1">{fmtBRL(data.total_devido)}</p>
            </div>
            <div className="text-right">
              <p className="text-sm text-muted-foreground">Total mercadorias (v_prod)</p>
              <p className="text-xl font-semibold tabular-nums mt-1">{fmtBRL(data.total_prod)}</p>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Summary table */}
      <div className="rounded-md border overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow className="bg-muted/30 hover:bg-transparent">
              <TableHead className="text-xs font-semibold uppercase tracking-wide">Regime</TableHead>
              <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">Notas</TableHead>
              <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">V. Produtos</TableHead>
              <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">V. ST Retido</TableHead>
              <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">ICMS Devido Est.</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {data.rows.map((row) => (
              <TableRow key={row.regime}>
                <TableCell><RegimeBadge regime={row.regime} /></TableCell>
                <TableCell className="text-xs text-right tabular-nums">{row.qtd_notas}</TableCell>
                <TableCell className="text-xs text-right tabular-nums">{fmtBRL(row.v_prod_total)}</TableCell>
                <TableCell className="text-xs text-right tabular-nums">{fmtBRL(row.v_st_retido)}</TableCell>
                <TableCell className="text-xs text-right tabular-nums font-semibold">{fmtBRL(row.icms_devido_est)}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Notes table (shared by Antecipação, ST, DIFAL tabs)
// ---------------------------------------------------------------------------
function NotasTab({ endpoint, regime }: { endpoint: string; regime: string }) {
  const { data, isLoading, isError } = useQuery<FronteiraNotasResponse>({
    queryKey: ['icms-fronteira', regime],
    queryFn: async () => {
      const res = await fetch(endpoint)
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      return res.json()
    },
  })

  if (isLoading) {
    return (
      <div className="space-y-2">
        {Array.from({ length: 6 }).map((_, i) => (
          <Skeleton key={i} className="h-8 w-full" />
        ))}
      </div>
    )
  }

  if (isError) {
    return (
      <Alert variant="destructive">
        <AlertDescription>Erro ao carregar notas. Verifique sua conexão.</AlertDescription>
      </Alert>
    )
  }

  if (!data || data.rows.length === 0) {
    return <EmptyState />
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between text-sm text-muted-foreground">
        <span>{data.count} nota{data.count !== 1 ? 's' : ''} (máx. 500)</span>
        <span className="font-semibold text-foreground">Total: {fmtBRL(data.total)}</span>
      </div>

      <div className="rounded-md border overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow className="bg-muted/30 hover:bg-transparent">
              <TableHead className="text-xs font-semibold uppercase tracking-wide">Data</TableHead>
              <TableHead className="text-xs font-semibold uppercase tracking-wide">NF-e</TableHead>
              <TableHead className="text-xs font-semibold uppercase tracking-wide">Fornecedor</TableHead>
              <TableHead className="text-xs font-semibold uppercase tracking-wide">UF</TableHead>
              <TableHead className="text-xs font-semibold uppercase tracking-wide">CFOP</TableHead>
              <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">V. Prod.</TableHead>
              <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">Alíq. Inter.</TableHead>
              <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">Alíq. Int.</TableHead>
              <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">ICMS Devido Est.</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {data.rows.map((row, idx) => (
              <TableRow key={`${row.chave_nfe}-${idx}`}>
                <TableCell className="text-xs font-mono whitespace-nowrap">
                  {row.data_emissao ? row.data_emissao.slice(0, 10) : '—'}
                </TableCell>
                <TableCell className="text-xs font-mono">{row.numero_nfe || '—'}</TableCell>
                <TableCell className="text-xs max-w-[180px]">
                  <div className="truncate" title={row.forn_nome}>{row.forn_nome || '—'}</div>
                  <div className="text-muted-foreground text-[10px] font-mono">{formatCNPJ(row.forn_cnpj)}</div>
                </TableCell>
                <TableCell className="text-xs font-mono font-semibold">{row.forn_uf || '—'}</TableCell>
                <TableCell className="text-xs font-mono">{row.cfop || '—'}</TableCell>
                <TableCell className="text-xs text-right tabular-nums">{fmtBRL(row.v_prod)}</TableCell>
                <TableCell className="text-xs text-right tabular-nums">{fmtPct(row.aliq_inter)}</TableCell>
                <TableCell className="text-xs text-right tabular-nums">{fmtPct(row.aliq_interna)}</TableCell>
                <TableCell className="text-xs text-right tabular-nums font-semibold">
                  {fmtBRL(row.icms_devido_est)}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Empty state
// ---------------------------------------------------------------------------
function EmptyState() {
  return (
    <div className="flex flex-col items-center gap-3 py-10 text-center">
      <p className="text-sm font-medium text-foreground">Nenhuma nota interestadual encontrada</p>
      <p className="text-xs text-muted-foreground max-w-md">
        O ICMS Fronteira é calculado sobre NF-e de entrada com fornecedor de outro estado.
        Importe XMLs de notas de entrada ou sincronize via ERP Bridge para visualizar os dados.
      </p>
    </div>
  )
}

// ---------------------------------------------------------------------------
// IcmsFronteira — main page
// ---------------------------------------------------------------------------
export default function IcmsFronteira() {
  const location = useLocation()
  const navigate = useNavigate()

  const pathToTab: Record<string, string> = {
    '/icms-fronteira':             'resumo',
    '/icms-fronteira/antecipacao': 'antecipacao',
    '/icms-fronteira/st':          'st',
    '/icms-fronteira/difal':       'difal',
  }
  const tabToPath: Record<string, string> = {
    resumo:       '/icms-fronteira',
    antecipacao:  '/icms-fronteira/antecipacao',
    st:           '/icms-fronteira/st',
    difal:        '/icms-fronteira/difal',
  }

  const tab = pathToTab[location.pathname] ?? 'resumo'

  function handleTabChange(value: string) {
    navigate(tabToPath[value] ?? '/icms-fronteira')
  }

  return (
    <div className="space-y-6 p-6">
      {/* Page header */}
      <div className="flex items-center gap-2">
        <h1 className="text-xl font-semibold">ICMS Fronteira — PE</h1>
        <TooltipProvider delayDuration={200}>
          <Tooltip>
            <TooltipTrigger asChild>
              <Info className="h-4 w-4 text-muted-foreground cursor-help shrink-0" />
            </TooltipTrigger>
            <TooltipContent className="max-w-xs text-left" side="bottom">
              <p className="font-medium mb-1">O que é</p>
              <p className="text-xs text-muted-foreground">
                Apura o ICMS devido na entrada de mercadorias interestaduais no estado de PE.
                Classifica cada NF-e em Antecipação, Substituição Tributária (ST) ou DIFAL
                com base no CFOP e nos valores da nota.
              </p>
              <p className="font-medium mb-1 mt-2">Regimes</p>
              <p className="text-xs text-muted-foreground">
                <strong>Antecipação:</strong> compra interestadual normal. ICMS = v_prod × (alíq. interna − alíq. inter.) / 100.<br />
                <strong>ST:</strong> fornecedor recolheu ICMS-ST antecipadamente (v_st {'>'} 0).<br />
                <strong>DIFAL:</strong> compra p/ uso/consumo ou ativo fixo (CFOP 1/2551, 2556).
              </p>
              <p className="font-medium mb-1 mt-2">Alíquotas</p>
              <p className="text-xs text-muted-foreground">
                Interestadual: 7% (PR/RS/SC/MG/RJ/SP) ou 12% (demais). Interna PE: 20,5% padrão.
              </p>
            </TooltipContent>
          </Tooltip>
        </TooltipProvider>
        <p className="text-sm text-muted-foreground ml-1">
          Apuração de ICMS na entrada interestadual de mercadorias
        </p>
      </div>

      <Separator />

      {/* Tabs */}
      <Tabs value={tab} onValueChange={handleTabChange}>
        <TabsList>
          <TabsTrigger value="resumo">Resumo</TabsTrigger>
          <TabsTrigger value="antecipacao">Antecipação</TabsTrigger>
          <TabsTrigger value="st">Subst. Tributária</TabsTrigger>
          <TabsTrigger value="difal">DIFAL</TabsTrigger>
        </TabsList>

        <TabsContent value="resumo" className="mt-6">
          <ResumoTab />
        </TabsContent>

        <TabsContent value="antecipacao" className="mt-6">
          <Card>
            <CardHeader>
              <CardTitle className="text-base font-semibold flex items-center gap-2">
                <TrendingUp className="h-4 w-4 text-blue-500" />
                Antecipação ICMS
              </CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-xs text-muted-foreground mb-4">
                Notas interestaduais sem ST e sem CFOP de uso/consumo. ICMS estimado =
                V.Prod × (alíq. interna − alíq. interestadual).
              </p>
              <NotasTab endpoint="/api/icms-fronteira/antecipacao" regime="antecipacao" />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="st" className="mt-6">
          <Card>
            <CardHeader>
              <CardTitle className="text-base font-semibold flex items-center gap-2">
                <AlertTriangle className="h-4 w-4 text-amber-500" />
                Substituição Tributária
              </CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-xs text-muted-foreground mb-4">
                Notas com ICMS-ST já retido pelo fornecedor (v_st {'>'} 0). O valor exibido
                é o ST efetivamente destacado na nota.
              </p>
              <NotasTab endpoint="/api/icms-fronteira/st" regime="st" />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="difal" className="mt-6">
          <Card>
            <CardHeader>
              <CardTitle className="text-base font-semibold flex items-center gap-2">
                <ArrowLeftRight className="h-4 w-4 text-purple-500" />
                Diferencial de Alíquota (DIFAL)
              </CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-xs text-muted-foreground mb-4">
                Notas com CFOP de compra para uso/consumo ou ativo imobilizado interestadual
                (1551, 1556, 2551, 2556). DIFAL = V.Prod × (alíq. interna − alíq. inter.).
              </p>
              <NotasTab endpoint="/api/icms-fronteira/difal" regime="difal" />
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  )
}
