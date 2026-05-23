import { useState } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog'
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
import {
  Info,
  TrendingUp,
  AlertTriangle,
  ArrowLeftRight,
  FileDown,
  FileSpreadsheet,
  Printer,
  RefreshCw,
  Trash2,
  Plus,
  Upload,
  BarChart2,
} from 'lucide-react'
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip as RechartsTooltip,
  Legend,
  ResponsiveContainer,
} from 'recharts'
import { useAuth } from '@/contexts/AuthContext'

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

interface RegraNCM {
  id: number
  ncm_prefixo: string
  descricao: string
  regime: string
  aliquota_interna: number
  mva_original: number | null
  reducao_bc_pct: number
  is_global: boolean
}

interface RegrasResponse {
  rows: RegraNCM[]
  count: number
}

interface ExtratoRow {
  id: number
  periodo: string
  registro_nota: string
  cnpj_emitente: string
  nome_emitente: string
  uf_emitente: string
  numero_nf: string
  chave_nfe: string
  icms_devido: number
}

interface ExtratoResponse {
  rows: ExtratoRow[]
  total: number
  count: number
}

interface FronteiraItemRow {
  chave_nfe: string
  data_emissao: string
  numero_nfe: string
  forn_cnpj: string
  forn_nome: string
  forn_uf: string
  forn_simples: boolean
  cfop: string
  regime: string
  n_item: number
  c_prod: string
  x_prod: string
  ncm: string
  cest: string
  v_prod_item: number
  v_ipi_item: number
  v_outro_rateado: number
  v_operacao: number
  v_icms_item: number
  aliq_inter: number
  aliq_interna: number
  bc: number
  icms_calculado: number
  icms_retido: number
  mva_original: number | null
  bc_st: number
}

interface FronteiraItensResponse {
  rows: FronteiraItemRow[]
  total: number
  count: number
}

interface DivergenciaRow {
  chave_nfe: string
  periodo: string
  numero_nf: string
  forn_cnpj: string
  forn_nome: string
  forn_uf: string
  data_emissao: string
  regime: string
  icms_sefaz: number
  icms_calculado: number
  diferenca: number
  status: string
}

interface DivergenciasResponse {
  rows: DivergenciaRow[]
  total_sefaz: number
  total_calculado: number
  total_diferenca: number
  count: number
}

interface ContestacaoRow {
  id: number
  chave_nfe: string
  numero_nf: string
  forn_cnpj: string
  forn_nome: string
  periodo: string
  valor_contestado: number
  motivo: string
  status: string
  resposta_sefaz: string | null
  data_registro: string
  data_resposta: string | null
}

interface ContestacaoResponse {
  rows: ContestacaoRow[]
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

// Converts <input type="month"> value (YYYY-MM) to API format MM/YYYY
function monthToPeriodo(m: string): string {
  if (!m) return ''
  const [y, mo] = m.split('-')
  return `${mo}/${y}`
}

function formatCNPJ(cnpj: string): string {
  if (!cnpj || cnpj.length !== 14) return cnpj || '—'
  return cnpj.replace(/(\d{2})(\d{3})(\d{3})(\d{4})(\d{2})/, '$1.$2.$3/$4-$5')
}

function RegimeBadge({ regime }: { regime: string }) {
  const map: Record<string, { label: string; variant: 'default' | 'secondary' | 'outline' | 'destructive' }> = {
    ANTECIPACAO: { label: 'Antecipação', variant: 'default' },
    ST:          { label: 'ST',          variant: 'outline' },
    DIFAL:       { label: 'DIFAL',       variant: 'secondary' },
    ISENTO:      { label: 'Isento',      variant: 'secondary' },
    NORMAL:      { label: 'Normal',      variant: 'secondary' },
  }
  const cfg = map[regime] ?? { label: regime, variant: 'secondary' as const }
  return (
    <Badge variant={cfg.variant} className="font-mono text-xs">
      {cfg.label}
    </Badge>
  )
}

function StatusBadge({ status }: { status: string }) {
  const map: Record<string, { label: string; className: string }> = {
    pendente:    { label: 'Pendente',    className: 'bg-yellow-100 text-yellow-800 border-yellow-200' },
    enviada:     { label: 'Enviada',     className: 'bg-blue-100 text-blue-800 border-blue-200' },
    deferida:    { label: 'Deferida',    className: 'bg-green-100 text-green-800 border-green-200' },
    indeferida:  { label: 'Indeferida',  className: 'bg-red-100 text-red-800 border-red-200' },
    cancelada:   { label: 'Cancelada',   className: 'bg-gray-100 text-gray-600 border-gray-200' },
  }
  const cfg = map[status] ?? { label: status, className: 'bg-gray-100 text-gray-600 border-gray-200' }
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium border ${cfg.className}`}>
      {cfg.label}
    </span>
  )
}

// ---------------------------------------------------------------------------
// Export buttons (shared by tabs)
// ---------------------------------------------------------------------------
function ExportButtons({ regime, token, periodo }: { regime: string; token: string | null; periodo?: string }) {
  async function downloadFile(format: 'csv' | 'xlsx') {
    try {
      const params = new URLSearchParams({ regime })
      if (periodo) params.set('periodo', periodo)
      const res = await fetch(`/api/icms-fronteira/exportar/${format}?${params}`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `icms-fronteira-${regime}.${format}`
      a.click()
      URL.revokeObjectURL(url)
      toast.success(`Arquivo ${format.toUpperCase()} gerado com sucesso`)
    } catch {
      toast.error(`Erro ao exportar ${format.toUpperCase()}`)
    }
  }

  function openPDF() {
    const params = new URLSearchParams({ regime })
    if (periodo) params.set('periodo', periodo)
    window.open(`/api/icms-fronteira/exportar/pdf?${params}`, '_blank')
  }

  return (
    <div className="flex items-center gap-2">
      <Button size="sm" variant="outline" onClick={() => downloadFile('csv')}>
        <FileDown className="h-3.5 w-3.5 mr-1" />
        CSV
      </Button>
      <Button size="sm" variant="outline" onClick={() => downloadFile('xlsx')}>
        <FileSpreadsheet className="h-3.5 w-3.5 mr-1" />
        Excel
      </Button>
      <Button size="sm" variant="outline" onClick={openPDF}>
        <Printer className="h-3.5 w-3.5 mr-1" />
        Imprimir/PDF
      </Button>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Recalcular button
// ---------------------------------------------------------------------------
function RecalcularButton() {
  const queryClient = useQueryClient()
  const [loading, setLoading] = useState(false)

  async function handleRecalcular() {
    setLoading(true)
    await queryClient.invalidateQueries({ queryKey: ['icms-fronteira'] })
    await queryClient.invalidateQueries({ queryKey: ['icms-fronteira/resumo'] })
    await queryClient.invalidateQueries({ queryKey: ['icms-fronteira/regras'] })
    setLoading(false)
    toast.success('Dados atualizados')
  }

  return (
    <Button size="sm" variant="outline" onClick={handleRecalcular} disabled={loading}>
      <RefreshCw className={`h-3.5 w-3.5 mr-1 ${loading ? 'animate-spin' : ''}`} />
      Recalcular
    </Button>
  )
}

// ---------------------------------------------------------------------------
// Resumo tab
// ---------------------------------------------------------------------------
function ResumoTab({ token }: { token: string | null }) {
  const [monthInput, setMonthInput] = useState('')
  const periodo = monthToPeriodo(monthInput)

  const { data, isLoading, isError } = useQuery<FronteiraResumoResponse>({
    queryKey: ['icms-fronteira/resumo', periodo],
    queryFn: async () => {
      const url = periodo
        ? `/api/icms-fronteira/resumo?periodo=${encodeURIComponent(periodo)}`
        : '/api/icms-fronteira/resumo'
      const res = await fetch(url, {
        headers: { Authorization: `Bearer ${token}` },
      })
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
      {/* Actions row */}
      <div className="flex items-center gap-2 justify-between flex-wrap">
        <div className="flex items-center gap-2">
          <Label htmlFor="resumo-periodo" className="text-xs whitespace-nowrap">Período:</Label>
          <Input
            id="resumo-periodo"
            type="month"
            className="w-36 text-xs h-8"
            value={monthInput}
            onChange={(e) => setMonthInput(e.target.value)}
          />
        </div>
        <div className="flex items-center gap-2">
          <ExportButtons regime="todos" token={token} periodo={periodo} />
          <RecalcularButton />
        </div>
      </div>

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
function NotasTab({ endpoint, regime, token }: { endpoint: string; regime: string; token: string | null }) {
  const [monthInput, setMonthInput] = useState('')
  const periodo = monthToPeriodo(monthInput)

  const { data, isLoading, isError } = useQuery<FronteiraNotasResponse>({
    queryKey: ['icms-fronteira', regime, periodo],
    queryFn: async () => {
      const url = periodo
        ? `${endpoint}?periodo=${encodeURIComponent(periodo)}`
        : endpoint
      const res = await fetch(url, {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      return res.json()
    },
  })

  return (
    <div className="space-y-3">
      {/* Period filter */}
      <div className="flex items-center gap-2">
        <Label htmlFor={`notas-periodo-${regime}`} className="text-xs whitespace-nowrap">Período:</Label>
        <Input
          id={`notas-periodo-${regime}`}
          type="month"
          className="w-36 text-xs h-8"
          value={monthInput}
          onChange={(e) => setMonthInput(e.target.value)}
        />
        {periodo && (
          <span className="text-xs text-muted-foreground">{periodo}</span>
        )}
      </div>

      {isLoading && (
        <div className="space-y-2">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-8 w-full" />
          ))}
        </div>
      )}

      {isError && (
        <Alert variant="destructive">
          <AlertDescription>Erro ao carregar notas. Verifique sua conexão.</AlertDescription>
        </Alert>
      )}

      {!isLoading && !isError && (!data || data.rows.length === 0) && <EmptyState />}

      {data && data.rows.length > 0 && (
        <>
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
        </>
      )}
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
// Regras NCM tab
// ---------------------------------------------------------------------------
function RegrasTab({ token }: { token: string | null }) {
  const queryClient = useQueryClient()
  const [search, setSearch] = useState('')
  const [openCreate, setOpenCreate] = useState(false)
  const [importFile, setImportFile] = useState<File | null>(null)
  const [importLoading, setImportLoading] = useState(false)

  // Form state
  const [ncmPrefixo, setNcmPrefixo] = useState('')
  const [descricao, setDescricao] = useState('')
  const [regimenForm, setRegimenForm] = useState('ANTECIPACAO')
  const [aliqInterna, setAliqInterna] = useState('20.5')
  const [mva, setMva] = useState('')
  const [reducaoBC, setReducaoBC] = useState('0')

  const { data, isLoading, isError } = useQuery<RegrasResponse>({
    queryKey: ['icms-fronteira/regras'],
    queryFn: async () => {
      const res = await fetch('/api/icms-fronteira/regras', {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      return res.json()
    },
  })

  const createMutation = useMutation({
    mutationFn: async (body: object) => {
      const res = await fetch('/api/icms-fronteira/regras', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify(body),
      })
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      return res.json()
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['icms-fronteira/regras'] })
      toast.success('Regra criada com sucesso')
      setOpenCreate(false)
      resetForm()
    },
    onError: () => toast.error('Erro ao criar regra'),
  })

  const deleteMutation = useMutation({
    mutationFn: async (id: number) => {
      const res = await fetch(`/api/icms-fronteira/regras/${id}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) throw new Error(`Erro ${res.status}`)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['icms-fronteira/regras'] })
      toast.success('Regra removida')
    },
    onError: () => toast.error('Erro ao remover regra'),
  })

  function resetForm() {
    setNcmPrefixo('')
    setDescricao('')
    setRegimenForm('ANTECIPACAO')
    setAliqInterna('20.5')
    setMva('')
    setReducaoBC('0')
  }

  function handleCreate() {
    createMutation.mutate({
      ncm_prefixo: ncmPrefixo,
      descricao,
      regime: regimenForm,
      aliquota_interna: parseFloat(aliqInterna) || 20.5,
      mva_original: mva ? parseFloat(mva) : null,
      reducao_bc_pct: parseFloat(reducaoBC) || 0,
    })
  }

  async function handleImport() {
    if (!importFile) return
    setImportLoading(true)
    try {
      const fd = new FormData()
      fd.append('file', importFile)
      const res = await fetch('/api/icms-fronteira/regras/importar', {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` },
        body: fd,
      })
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      const result = await res.json()
      toast.success(`Importadas: ${result.imported}, ignoradas: ${result.skipped}`)
      if (result.errors?.length) {
        toast.warning(`${result.errors.length} erro(s) na importação`)
      }
      queryClient.invalidateQueries({ queryKey: ['icms-fronteira/regras'] })
      setImportFile(null)
    } catch {
      toast.error('Erro ao importar arquivo')
    } finally {
      setImportLoading(false)
    }
  }

  const filtered = (data?.rows ?? []).filter((r) => {
    const q = search.toLowerCase()
    return (
      r.ncm_prefixo.toLowerCase().includes(q) ||
      r.descricao.toLowerCase().includes(q) ||
      r.regime.toLowerCase().includes(q)
    )
  })

  return (
    <div className="space-y-4">
      {/* Import card */}
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-semibold">Importar Regras (CSV/XLSX)</CardTitle>
        </CardHeader>
        <CardContent>
          <Alert className="mb-3">
            <AlertDescription className="text-xs">
              Formato esperado: <code>ncm_prefixo; descricao; regime; aliquota_interna; mva_original; reducao_bc_pct</code>
            </AlertDescription>
          </Alert>
          <div className="flex items-center gap-2">
            <Input
              type="file"
              accept=".csv,.xlsx,.xls"
              className="max-w-sm text-xs"
              onChange={(e) => setImportFile(e.target.files?.[0] ?? null)}
            />
            <Button
              size="sm"
              onClick={handleImport}
              disabled={!importFile || importLoading}
            >
              <Upload className="h-3.5 w-3.5 mr-1" />
              {importLoading ? 'Importando...' : 'Importar'}
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Controls */}
      <div className="flex items-center justify-between gap-2">
        <Input
          placeholder="Buscar por NCM, descrição ou regime..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="max-w-xs text-xs"
        />
        <Button size="sm" onClick={() => setOpenCreate(true)}>
          <Plus className="h-3.5 w-3.5 mr-1" />
          Nova Regra
        </Button>
      </div>

      {/* Table */}
      {isLoading && (
        <div className="space-y-2">
          {[0, 1, 2, 3].map((i) => <Skeleton key={i} className="h-8 w-full" />)}
        </div>
      )}
      {isError && (
        <Alert variant="destructive">
          <AlertDescription>Erro ao carregar regras NCM.</AlertDescription>
        </Alert>
      )}
      {data && (
        <div className="rounded-md border overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow className="bg-muted/30 hover:bg-transparent">
                <TableHead className="text-xs font-semibold uppercase tracking-wide">NCM</TableHead>
                <TableHead className="text-xs font-semibold uppercase tracking-wide">Descrição</TableHead>
                <TableHead className="text-xs font-semibold uppercase tracking-wide">Regime</TableHead>
                <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">Alíq. Int. %</TableHead>
                <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">MVA %</TableHead>
                <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">Redução BC %</TableHead>
                <TableHead className="text-xs font-semibold uppercase tracking-wide text-center">Global</TableHead>
                <TableHead className="text-xs font-semibold uppercase tracking-wide">Ações</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={8} className="text-center text-xs text-muted-foreground py-6">
                    Nenhuma regra encontrada
                  </TableCell>
                </TableRow>
              ) : (
                filtered.map((row) => (
                  <TableRow key={row.id}>
                    <TableCell className="text-xs font-mono">{row.ncm_prefixo}</TableCell>
                    <TableCell className="text-xs max-w-[200px]">
                      <div className="truncate" title={row.descricao}>{row.descricao}</div>
                    </TableCell>
                    <TableCell><RegimeBadge regime={row.regime} /></TableCell>
                    <TableCell className="text-xs text-right tabular-nums">{fmtPct(row.aliquota_interna)}</TableCell>
                    <TableCell className="text-xs text-right tabular-nums">{row.mva_original != null ? fmtPct(row.mva_original) : '—'}</TableCell>
                    <TableCell className="text-xs text-right tabular-nums">{fmtPct(row.reducao_bc_pct)}</TableCell>
                    <TableCell className="text-center">
                      {row.is_global ? (
                        <Badge variant="secondary" className="text-[10px]">Global</Badge>
                      ) : (
                        <span className="text-xs text-muted-foreground">—</span>
                      )}
                    </TableCell>
                    <TableCell>
                      {!row.is_global && (
                        <Button
                          size="sm"
                          variant="ghost"
                          className="h-7 w-7 p-0 text-destructive hover:text-destructive"
                          onClick={() => {
                            if (confirm('Remover esta regra?')) deleteMutation.mutate(row.id)
                          }}
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      )}

      {/* Create Dialog */}
      <Dialog open={openCreate} onOpenChange={(o) => { setOpenCreate(o); if (!o) resetForm() }}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Nova Regra NCM</DialogTitle>
          </DialogHeader>
          <div className="grid gap-4 py-2">
            <div className="grid gap-1.5">
              <Label htmlFor="ncm-prefixo">NCM Prefixo</Label>
              <Input
                id="ncm-prefixo"
                placeholder="ex: 8471"
                value={ncmPrefixo}
                onChange={(e) => setNcmPrefixo(e.target.value)}
              />
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="descricao-regra">Descrição</Label>
              <Input
                id="descricao-regra"
                placeholder="Descrição da regra"
                value={descricao}
                onChange={(e) => setDescricao(e.target.value)}
              />
            </div>
            <div className="grid gap-1.5">
              <Label>Regime</Label>
              <Select value={regimenForm} onValueChange={setRegimenForm}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="ANTECIPACAO">Antecipação</SelectItem>
                  <SelectItem value="ST">Substituição Tributária (ST)</SelectItem>
                  <SelectItem value="DIFAL">DIFAL</SelectItem>
                  <SelectItem value="ISENTO">Isento</SelectItem>
                  <SelectItem value="NORMAL">Normal</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="grid grid-cols-3 gap-3">
              <div className="grid gap-1.5">
                <Label htmlFor="aliq-interna">Alíq. Interna %</Label>
                <Input
                  id="aliq-interna"
                  type="number"
                  step="0.1"
                  value={aliqInterna}
                  onChange={(e) => setAliqInterna(e.target.value)}
                />
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="mva">MVA % (opt.)</Label>
                <Input
                  id="mva"
                  type="number"
                  step="0.1"
                  placeholder="—"
                  value={mva}
                  onChange={(e) => setMva(e.target.value)}
                />
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="reducao-bc">Redução BC %</Label>
                <Input
                  id="reducao-bc"
                  type="number"
                  step="0.1"
                  value={reducaoBC}
                  onChange={(e) => setReducaoBC(e.target.value)}
                />
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpenCreate(false)}>Cancelar</Button>
            <Button
              onClick={handleCreate}
              disabled={createMutation.isPending || !ncmPrefixo || !descricao}
            >
              {createMutation.isPending ? 'Salvando...' : 'Criar Regra'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Extrato SEFAZ tab
// ---------------------------------------------------------------------------
function ExtratoTab({ token }: { token: string | null }) {
  const queryClient = useQueryClient()
  // month input gives YYYY-MM, convert to MM/YYYY for API
  const [monthInput, setMonthInput] = useState('')
  const [importFile, setImportFile] = useState<File | null>(null)
  const [importPeriodo, setImportPeriodo] = useState('')
  const [importLoading, setImportLoading] = useState(false)
  const [confirmClear, setConfirmClear] = useState(false)

  function monthToPeriodo(m: string): string {
    if (!m) return ''
    const [y, mo] = m.split('-')
    return `${mo}/${y}`
  }

  const periodo = monthToPeriodo(monthInput)

  const { data, isLoading, isError } = useQuery<ExtratoResponse>({
    queryKey: ['icms-fronteira/extrato', periodo],
    queryFn: async () => {
      const url = periodo
        ? `/api/icms-fronteira/extrato?periodo=${encodeURIComponent(periodo)}`
        : '/api/icms-fronteira/extrato'
      const res = await fetch(url, {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      return res.json()
    },
    enabled: true,
  })

  async function handleImport() {
    if (!importFile) return
    setImportLoading(true)
    try {
      const fd = new FormData()
      fd.append('file', importFile)
      fd.append('periodo', importPeriodo)
      const res = await fetch('/api/icms-fronteira/extrato/importar', {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` },
        body: fd,
      })
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      const result = await res.json()
      toast.success(`${result.imported} registros importados para ${result.periodo}`)
      queryClient.invalidateQueries({ queryKey: ['icms-fronteira/extrato'] })
      setImportFile(null)
    } catch {
      toast.error('Erro ao importar extrato')
    } finally {
      setImportLoading(false)
    }
  }

  async function handleClearPeriodo() {
    if (!periodo) return
    try {
      const res = await fetch(`/api/icms-fronteira/extrato?periodo=${encodeURIComponent(periodo)}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      toast.success(`Período ${periodo} removido`)
      queryClient.invalidateQueries({ queryKey: ['icms-fronteira/extrato'] })
    } catch {
      toast.error('Erro ao limpar período')
    } finally {
      setConfirmClear(false)
    }
  }

  return (
    <div className="space-y-4">
      {/* Import card */}
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-semibold">Importar Extrato SEFAZ</CardTitle>
        </CardHeader>
        <CardContent>
          <Alert className="mb-3">
            <AlertDescription className="text-xs">
              Formato esperado: <code>registro_nota; cnpj_emitente; nome_emitente; uf_emitente; numero_nf; chave_nfe; icms_devido</code>
            </AlertDescription>
          </Alert>
          <div className="flex items-center gap-2 flex-wrap">
            <Input
              type="file"
              accept=".csv,.xlsx,.xls"
              className="max-w-xs text-xs"
              onChange={(e) => setImportFile(e.target.files?.[0] ?? null)}
            />
            <div className="flex items-center gap-1.5">
              <Label className="text-xs whitespace-nowrap">Período (MM/YYYY):</Label>
              <Input
                placeholder="ex: 05/2025"
                value={importPeriodo}
                onChange={(e) => setImportPeriodo(e.target.value)}
                className="w-32 text-xs"
              />
            </div>
            <Button
              size="sm"
              onClick={handleImport}
              disabled={!importFile || !importPeriodo || importLoading}
            >
              <Upload className="h-3.5 w-3.5 mr-1" />
              {importLoading ? 'Importando...' : 'Importar'}
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Period selector + actions */}
      <div className="flex items-center gap-2 flex-wrap">
        <div className="flex items-center gap-1.5">
          <Label className="text-xs whitespace-nowrap">Período:</Label>
          <Input
            type="month"
            value={monthInput}
            onChange={(e) => setMonthInput(e.target.value)}
            className="w-40 text-xs"
          />
        </div>
        {periodo && (
          <Button
            size="sm"
            variant="destructive"
            onClick={() => setConfirmClear(true)}
          >
            <Trash2 className="h-3.5 w-3.5 mr-1" />
            Limpar período
          </Button>
        )}
      </div>

      {/* Table */}
      {isLoading && (
        <div className="space-y-2">
          {[0, 1, 2, 3].map((i) => <Skeleton key={i} className="h-8 w-full" />)}
        </div>
      )}
      {isError && (
        <Alert variant="destructive">
          <AlertDescription>Erro ao carregar extrato SEFAZ.</AlertDescription>
        </Alert>
      )}
      {data && (
        <div className="space-y-2">
          <div className="text-xs text-muted-foreground">{data.count} registro(s)</div>
          <div className="rounded-md border overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow className="bg-muted/30 hover:bg-transparent">
                  <TableHead className="text-xs font-semibold uppercase tracking-wide">Período</TableHead>
                  <TableHead className="text-xs font-semibold uppercase tracking-wide">CNPJ</TableHead>
                  <TableHead className="text-xs font-semibold uppercase tracking-wide">Nome Emitente</TableHead>
                  <TableHead className="text-xs font-semibold uppercase tracking-wide">UF</TableHead>
                  <TableHead className="text-xs font-semibold uppercase tracking-wide">NF</TableHead>
                  <TableHead className="text-xs font-semibold uppercase tracking-wide">Chave NF-e</TableHead>
                  <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">ICMS Devido</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.rows.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={7} className="text-center text-xs text-muted-foreground py-6">
                      Nenhum registro encontrado
                    </TableCell>
                  </TableRow>
                ) : (
                  data.rows.map((row) => (
                    <TableRow key={row.id}>
                      <TableCell className="text-xs font-mono">{row.periodo}</TableCell>
                      <TableCell className="text-xs font-mono">{formatCNPJ(row.cnpj_emitente)}</TableCell>
                      <TableCell className="text-xs max-w-[160px]">
                        <div className="truncate" title={row.nome_emitente}>{row.nome_emitente}</div>
                      </TableCell>
                      <TableCell className="text-xs font-mono font-semibold">{row.uf_emitente}</TableCell>
                      <TableCell className="text-xs font-mono">{row.numero_nf}</TableCell>
                      <TableCell className="text-xs font-mono text-[10px] max-w-[140px]">
                        <div className="truncate" title={row.chave_nfe}>{row.chave_nfe}</div>
                      </TableCell>
                      <TableCell className="text-xs text-right tabular-nums font-semibold">
                        {fmtBRL(row.icms_devido)}
                      </TableCell>
                    </TableRow>
                  ))
                )}
                {data.rows.length > 0 && (
                  <TableRow className="bg-muted/30 font-semibold">
                    <TableCell colSpan={6} className="text-xs text-right">Total</TableCell>
                    <TableCell className="text-xs text-right tabular-nums">{fmtBRL(data.total)}</TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
        </div>
      )}

      {/* Confirm clear dialog */}
      <Dialog open={confirmClear} onOpenChange={setConfirmClear}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>Confirmar exclusão</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            Deseja remover todos os registros do período <strong>{periodo}</strong>? Esta ação não pode ser desfeita.
          </p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirmClear(false)}>Cancelar</Button>
            <Button variant="destructive" onClick={handleClearPeriodo}>Confirmar</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Contestações tab
// ---------------------------------------------------------------------------
function ContestacoesTab({ token }: { token: string | null }) {
  const queryClient = useQueryClient()
  const [filterStatus, setFilterStatus] = useState('todos')
  const [filterPeriodo, setFilterPeriodo] = useState('')
  const [openCreate, setOpenCreate] = useState(false)
  const [responderRow, setResponderRow] = useState<ContestacaoRow | null>(null)

  // Create form
  const [fChaveNfe, setFChaveNfe] = useState('')
  const [fNumeroNf, setFNumeroNf] = useState('')
  const [fFornCnpj, setFFornCnpj] = useState('')
  const [fFornNome, setFFornNome] = useState('')
  const [fPeriodo, setFPeriodo] = useState('')
  const [fValor, setFValor] = useState('')
  const [fMotivo, setFMotivo] = useState('')

  // Responder form
  const [rStatus, setRStatus] = useState('')
  const [rResposta, setRResposta] = useState('')

  const qParams = new URLSearchParams()
  if (filterStatus && filterStatus !== 'todos') qParams.set('status', filterStatus)
  if (filterPeriodo) qParams.set('periodo', filterPeriodo)

  const { data, isLoading, isError } = useQuery<ContestacaoResponse>({
    queryKey: ['icms-fronteira/contestacoes', filterStatus, filterPeriodo],
    queryFn: async () => {
      const res = await fetch(`/api/icms-fronteira/contestacoes?${qParams.toString()}`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      return res.json()
    },
  })

  const createMutation = useMutation({
    mutationFn: async (body: object) => {
      const res = await fetch('/api/icms-fronteira/contestacoes', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify(body),
      })
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      return res.json()
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['icms-fronteira/contestacoes'] })
      toast.success('Contestação criada com sucesso')
      setOpenCreate(false)
      resetCreateForm()
    },
    onError: () => toast.error('Erro ao criar contestação'),
  })

  const updateMutation = useMutation({
    mutationFn: async ({ id, body }: { id: number; body: object }) => {
      const res = await fetch(`/api/icms-fronteira/contestacoes/${id}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify(body),
      })
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      return res.json()
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['icms-fronteira/contestacoes'] })
      toast.success('Contestação atualizada')
      setResponderRow(null)
    },
    onError: () => toast.error('Erro ao atualizar contestação'),
  })

  const deleteMutation = useMutation({
    mutationFn: async (id: number) => {
      const res = await fetch(`/api/icms-fronteira/contestacoes/${id}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) throw new Error(`Erro ${res.status}`)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['icms-fronteira/contestacoes'] })
      toast.success('Contestação removida')
    },
    onError: () => toast.error('Erro ao remover contestação'),
  })

  function resetCreateForm() {
    setFChaveNfe(''); setFNumeroNf(''); setFFornCnpj(''); setFFornNome('')
    setFPeriodo(''); setFValor(''); setFMotivo('')
  }

  function handleCreate() {
    createMutation.mutate({
      chave_nfe: fChaveNfe,
      numero_nf: fNumeroNf,
      forn_cnpj: fFornCnpj,
      forn_nome: fFornNome,
      periodo: fPeriodo,
      valor_contestado: parseFloat(fValor) || 0,
      motivo: fMotivo,
    })
  }

  function handleResponder() {
    if (!responderRow) return
    updateMutation.mutate({
      id: responderRow.id,
      body: { status: rStatus, resposta_sefaz: rResposta },
    })
  }

  function openResponder(row: ContestacaoRow) {
    setResponderRow(row)
    setRStatus(row.status)
    setRResposta(row.resposta_sefaz ?? '')
  }

  return (
    <div className="space-y-4">
      {/* Controls */}
      <div className="flex items-center gap-2 flex-wrap justify-between">
        <div className="flex items-center gap-2 flex-wrap">
          <Select value={filterStatus} onValueChange={setFilterStatus}>
            <SelectTrigger className="w-36 text-xs">
              <SelectValue placeholder="Status" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="todos">Todos</SelectItem>
              <SelectItem value="pendente">Pendente</SelectItem>
              <SelectItem value="enviada">Enviada</SelectItem>
              <SelectItem value="deferida">Deferida</SelectItem>
              <SelectItem value="indeferida">Indeferida</SelectItem>
              <SelectItem value="cancelada">Cancelada</SelectItem>
            </SelectContent>
          </Select>
          <Input
            placeholder="Período (MM/YYYY)"
            value={filterPeriodo}
            onChange={(e) => setFilterPeriodo(e.target.value)}
            className="w-36 text-xs"
          />
        </div>
        <Button size="sm" onClick={() => setOpenCreate(true)}>
          <Plus className="h-3.5 w-3.5 mr-1" />
          Nova Contestação
        </Button>
      </div>

      {/* Table */}
      {isLoading && (
        <div className="space-y-2">
          {[0, 1, 2, 3].map((i) => <Skeleton key={i} className="h-8 w-full" />)}
        </div>
      )}
      {isError && (
        <Alert variant="destructive">
          <AlertDescription>Erro ao carregar contestações.</AlertDescription>
        </Alert>
      )}
      {data && (
        <div className="rounded-md border overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow className="bg-muted/30 hover:bg-transparent">
                <TableHead className="text-xs font-semibold uppercase tracking-wide">NF</TableHead>
                <TableHead className="text-xs font-semibold uppercase tracking-wide">CNPJ</TableHead>
                <TableHead className="text-xs font-semibold uppercase tracking-wide">Fornecedor</TableHead>
                <TableHead className="text-xs font-semibold uppercase tracking-wide">Período</TableHead>
                <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">Valor</TableHead>
                <TableHead className="text-xs font-semibold uppercase tracking-wide">Motivo</TableHead>
                <TableHead className="text-xs font-semibold uppercase tracking-wide">Status</TableHead>
                <TableHead className="text-xs font-semibold uppercase tracking-wide whitespace-nowrap">Data Reg.</TableHead>
                <TableHead className="text-xs font-semibold uppercase tracking-wide">Ações</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {data.rows.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={9} className="text-center text-xs text-muted-foreground py-6">
                    Nenhuma contestação encontrada
                  </TableCell>
                </TableRow>
              ) : (
                data.rows.map((row) => (
                  <TableRow key={row.id}>
                    <TableCell className="text-xs font-mono">{row.numero_nf}</TableCell>
                    <TableCell className="text-xs font-mono text-[10px]">{formatCNPJ(row.forn_cnpj)}</TableCell>
                    <TableCell className="text-xs max-w-[140px]">
                      <div className="truncate" title={row.forn_nome}>{row.forn_nome}</div>
                    </TableCell>
                    <TableCell className="text-xs font-mono">{row.periodo}</TableCell>
                    <TableCell className="text-xs text-right tabular-nums">{fmtBRL(row.valor_contestado)}</TableCell>
                    <TableCell className="text-xs max-w-[140px]">
                      <div className="truncate" title={row.motivo}>{row.motivo}</div>
                    </TableCell>
                    <TableCell><StatusBadge status={row.status} /></TableCell>
                    <TableCell className="text-xs font-mono whitespace-nowrap">
                      {row.data_registro ? row.data_registro.slice(0, 10) : '—'}
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1">
                        <Button
                          size="sm"
                          variant="ghost"
                          className="h-7 px-2 text-xs"
                          onClick={() => openResponder(row)}
                        >
                          Responder
                        </Button>
                        {row.status === 'pendente' && (
                          <Button
                            size="sm"
                            variant="ghost"
                            className="h-7 w-7 p-0 text-destructive hover:text-destructive"
                            onClick={() => {
                              if (confirm('Remover esta contestação?')) deleteMutation.mutate(row.id)
                            }}
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </Button>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      )}

      {/* Create dialog */}
      <Dialog open={openCreate} onOpenChange={(o) => { setOpenCreate(o); if (!o) resetCreateForm() }}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>Nova Contestação</DialogTitle>
          </DialogHeader>
          <div className="grid gap-4 py-2">
            <div className="grid gap-1.5">
              <Label htmlFor="c-chave">Chave NF-e (44 dígitos)</Label>
              <Input
                id="c-chave"
                maxLength={44}
                placeholder="44 dígitos"
                value={fChaveNfe}
                onChange={(e) => setFChaveNfe(e.target.value)}
                className="font-mono text-xs"
              />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="grid gap-1.5">
                <Label htmlFor="c-numero">Número NF</Label>
                <Input id="c-numero" value={fNumeroNf} onChange={(e) => setFNumeroNf(e.target.value)} />
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="c-periodo">Período (MM/YYYY)</Label>
                <Input id="c-periodo" placeholder="05/2025" value={fPeriodo} onChange={(e) => setFPeriodo(e.target.value)} />
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="grid gap-1.5">
                <Label htmlFor="c-cnpj">CNPJ Fornecedor</Label>
                <Input id="c-cnpj" value={fFornCnpj} onChange={(e) => setFFornCnpj(e.target.value)} />
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="c-valor">Valor Contestado (R$)</Label>
                <Input id="c-valor" type="number" step="0.01" value={fValor} onChange={(e) => setFValor(e.target.value)} />
              </div>
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="c-nome">Nome Fornecedor</Label>
              <Input id="c-nome" value={fFornNome} onChange={(e) => setFFornNome(e.target.value)} />
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="c-motivo">Motivo *</Label>
              <Textarea
                id="c-motivo"
                rows={3}
                placeholder="Descreva o motivo da contestação..."
                value={fMotivo}
                onChange={(e) => setFMotivo(e.target.value)}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpenCreate(false)}>Cancelar</Button>
            <Button
              onClick={handleCreate}
              disabled={createMutation.isPending || !fChaveNfe || !fMotivo}
            >
              {createMutation.isPending ? 'Salvando...' : 'Criar'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Responder dialog */}
      <Dialog open={!!responderRow} onOpenChange={(o) => { if (!o) setResponderRow(null) }}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Atualizar Contestação</DialogTitle>
          </DialogHeader>
          {responderRow && (
            <div className="space-y-1 text-xs text-muted-foreground mb-2">
              <p>NF: <span className="font-mono text-foreground">{responderRow.numero_nf}</span></p>
              <p>Fornecedor: {responderRow.forn_nome}</p>
              <p>Valor: {fmtBRL(responderRow.valor_contestado)}</p>
            </div>
          )}
          <div className="grid gap-4 py-2">
            <div className="grid gap-1.5">
              <Label>Status</Label>
              <Select value={rStatus} onValueChange={setRStatus}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="pendente">Pendente</SelectItem>
                  <SelectItem value="enviada">Enviada</SelectItem>
                  <SelectItem value="deferida">Deferida</SelectItem>
                  <SelectItem value="indeferida">Indeferida</SelectItem>
                  <SelectItem value="cancelada">Cancelada</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="r-resposta">Resposta SEFAZ</Label>
              <Textarea
                id="r-resposta"
                rows={4}
                placeholder="Cole aqui a resposta da SEFAZ..."
                value={rResposta}
                onChange={(e) => setRResposta(e.target.value)}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setResponderRow(null)}>Cancelar</Button>
            <Button onClick={handleResponder} disabled={updateMutation.isPending}>
              {updateMutation.isPending ? 'Salvando...' : 'Salvar'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Divergências tab — calculado × SEFAZ
// ---------------------------------------------------------------------------

const STATUS_CFG: Record<string, { label: string; className: string; priority: number }> = {
  COBRADO_A_MAIS:  { label: 'Cobrado a mais',  className: 'bg-red-100 text-red-800 border-red-200',       priority: 1 },
  SEM_NOTA:        { label: 'Sem nota',         className: 'bg-orange-100 text-orange-800 border-orange-200', priority: 2 },
  COBRADO_A_MENOS: { label: 'Cobrado a menos', className: 'bg-yellow-100 text-yellow-800 border-yellow-200', priority: 3 },
  NAO_COBRADO:     { label: 'Não cobrado',      className: 'bg-blue-100 text-blue-800 border-blue-200',   priority: 4 },
  OK:              { label: 'OK',               className: 'bg-green-100 text-green-800 border-green-200', priority: 5 },
}

function StatusDivBadge({ status }: { status: string }) {
  const cfg = STATUS_CFG[status] ?? { label: status, className: 'bg-gray-100 text-gray-600 border-gray-200', priority: 9 }
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium border ${cfg.className}`}>
      {cfg.label}
    </span>
  )
}

function DivergenciasTab({ token }: { token: string | null }) {
  const queryClient = useQueryClient()
  const [monthInput, setMonthInput] = useState('')
  const [statusFilter, setStatusFilter] = useState('todos')
  const [contestarRow, setContestarRow] = useState<DivergenciaRow | null>(null)

  // Contestar form
  const [cMotivo, setCMotivo] = useState('')

  function monthToPeriodo(m: string): string {
    if (!m) return ''
    const [y, mo] = m.split('-')
    return `${mo}/${y}`
  }

  const periodo = monthToPeriodo(monthInput)

  const { data, isLoading, isError } = useQuery<DivergenciasResponse>({
    queryKey: ['icms-fronteira/divergencias', periodo],
    queryFn: async () => {
      const url = periodo
        ? `/api/icms-fronteira/divergencias?periodo=${encodeURIComponent(periodo)}`
        : '/api/icms-fronteira/divergencias'
      const res = await fetch(url, { headers: { Authorization: `Bearer ${token}` } })
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      return res.json()
    },
  })

  const contestarMutation = useMutation({
    mutationFn: async (body: object) => {
      const res = await fetch('/api/icms-fronteira/contestacoes', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify(body),
      })
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      return res.json()
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['icms-fronteira/contestacoes'] })
      toast.success('Contestação criada com sucesso')
      setContestarRow(null)
      setCMotivo('')
    },
    onError: () => toast.error('Erro ao criar contestação'),
  })

  function handleContestar() {
    if (!contestarRow) return
    contestarMutation.mutate({
      chave_nfe:        contestarRow.chave_nfe,
      numero_nf:        contestarRow.numero_nf,
      forn_cnpj:        contestarRow.forn_cnpj,
      forn_nome:        contestarRow.forn_nome,
      periodo:          contestarRow.periodo,
      valor_contestado: Math.abs(contestarRow.diferenca),
      motivo:           cMotivo,
    })
  }

  const rows = (data?.rows ?? []).filter(
    (r) => statusFilter === 'todos' || r.status === statusFilter,
  )

  const countByStatus = (data?.rows ?? []).reduce<Record<string, number>>((acc, r) => {
    acc[r.status] = (acc[r.status] ?? 0) + 1
    return acc
  }, {})

  return (
    <div className="space-y-4">
      {/* Filters */}
      <div className="flex items-center gap-3 flex-wrap">
        <div className="flex items-center gap-1.5">
          <Label className="text-xs whitespace-nowrap">Período:</Label>
          <Input
            type="month"
            value={monthInput}
            onChange={(e) => setMonthInput(e.target.value)}
            className="w-40 text-xs"
          />
        </div>
        <Select value={statusFilter} onValueChange={setStatusFilter}>
          <SelectTrigger className="w-44 text-xs">
            <SelectValue placeholder="Status" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="todos">Todos</SelectItem>
            {Object.entries(STATUS_CFG)
              .sort((a, b) => a[1].priority - b[1].priority)
              .map(([k, v]) => (
                <SelectItem key={k} value={k}>
                  {v.label} {countByStatus[k] ? `(${countByStatus[k]})` : ''}
                </SelectItem>
              ))}
          </SelectContent>
        </Select>
        <div className="flex items-center gap-2 ml-auto">
          <Button size="sm" variant="outline" onClick={() => {
            const a = document.createElement('a')
            a.href = `/api/icms-fronteira/divergencias/exportar/csv${periodo ? `?periodo=${encodeURIComponent(periodo)}` : ''}`
            a.download = `divergencias${periodo ? '-' + periodo.replace('/','-') : ''}.csv`
            document.body.appendChild(a); a.click(); document.body.removeChild(a)
          }}>
            <FileDown className="h-3.5 w-3.5 mr-1" />CSV
          </Button>
          <Button size="sm" variant="outline" onClick={() => {
            const a = document.createElement('a')
            a.href = `/api/icms-fronteira/divergencias/exportar/xlsx${periodo ? `?periodo=${encodeURIComponent(periodo)}` : ''}`
            a.download = `divergencias${periodo ? '-' + periodo.replace('/','-') : ''}.xlsx`
            document.body.appendChild(a); a.click(); document.body.removeChild(a)
          }}>
            <FileSpreadsheet className="h-3.5 w-3.5 mr-1" />Excel
          </Button>
          {data && (
            <span className="text-xs text-muted-foreground">
              {rows.length} registro{rows.length !== 1 ? 's' : ''}
            </span>
          )}
        </div>
      </div>

      {/* KPI summary */}
      {data && (
        <div className="grid grid-cols-3 gap-3">
          <Card>
            <CardContent className="pt-4 pb-3">
              <p className="text-xs text-muted-foreground">SEFAZ cobrou</p>
              <p className="text-lg font-bold tabular-nums">{fmtBRL(data.total_sefaz)}</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-4 pb-3">
              <p className="text-xs text-muted-foreground">Calculamos</p>
              <p className="text-lg font-bold tabular-nums">{fmtBRL(data.total_calculado)}</p>
            </CardContent>
          </Card>
          <Card className={data.total_diferenca > 0.05 ? 'border-red-300' : data.total_diferenca < -0.05 ? 'border-yellow-300' : ''}>
            <CardContent className="pt-4 pb-3">
              <p className="text-xs text-muted-foreground">Diferença total</p>
              <p className={`text-lg font-bold tabular-nums ${data.total_diferenca > 0.05 ? 'text-red-600' : data.total_diferenca < -0.05 ? 'text-yellow-600' : 'text-green-600'}`}>
                {fmtBRL(data.total_diferenca)}
              </p>
            </CardContent>
          </Card>
        </div>
      )}

      {isLoading && (
        <div className="space-y-2">
          {Array.from({ length: 6 }).map((_, i) => <Skeleton key={i} className="h-8 w-full" />)}
        </div>
      )}

      {isError && (
        <Alert variant="destructive">
          <AlertDescription>Erro ao carregar divergências. Verifique sua conexão.</AlertDescription>
        </Alert>
      )}

      {data && rows.length === 0 && (
        <div className="flex flex-col items-center gap-2 py-10 text-center">
          <p className="text-sm font-medium">Nenhuma divergência encontrada</p>
          <p className="text-xs text-muted-foreground max-w-sm">
            Importe o Extrato SEFAZ-PE na tab correspondente e selecione um período para cruzar os dados.
          </p>
        </div>
      )}

      {rows.length > 0 && (
        <div className="rounded-md border overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow className="bg-muted/30 hover:bg-transparent">
                <TableHead className="text-xs font-semibold uppercase tracking-wide">Período</TableHead>
                <TableHead className="text-xs font-semibold uppercase tracking-wide">NF</TableHead>
                <TableHead className="text-xs font-semibold uppercase tracking-wide">Fornecedor</TableHead>
                <TableHead className="text-xs font-semibold uppercase tracking-wide">UF</TableHead>
                <TableHead className="text-xs font-semibold uppercase tracking-wide">Data</TableHead>
                <TableHead className="text-xs font-semibold uppercase tracking-wide">Regime</TableHead>
                <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">SEFAZ</TableHead>
                <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">Calculado</TableHead>
                <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">Diferença</TableHead>
                <TableHead className="text-xs font-semibold uppercase tracking-wide">Status</TableHead>
                <TableHead className="text-xs font-semibold uppercase tracking-wide">Ações</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((row, idx) => (
                <TableRow key={`${row.chave_nfe}-${idx}`}>
                  <TableCell className="text-xs font-mono">{row.periodo || '—'}</TableCell>
                  <TableCell className="text-xs font-mono">{row.numero_nf || '—'}</TableCell>
                  <TableCell className="text-xs max-w-[160px]">
                    <div className="truncate" title={row.forn_nome}>{row.forn_nome || '—'}</div>
                    <div className="text-[10px] text-muted-foreground font-mono">{formatCNPJ(row.forn_cnpj)}</div>
                  </TableCell>
                  <TableCell className="text-xs font-mono font-semibold">{row.forn_uf || '—'}</TableCell>
                  <TableCell className="text-xs font-mono whitespace-nowrap">
                    {row.data_emissao ? row.data_emissao.slice(0, 10) : '—'}
                  </TableCell>
                  <TableCell>
                    {row.regime ? <RegimeBadge regime={row.regime} /> : <span className="text-xs text-muted-foreground">—</span>}
                  </TableCell>
                  <TableCell className="text-xs text-right tabular-nums">{fmtBRL(row.icms_sefaz)}</TableCell>
                  <TableCell className="text-xs text-right tabular-nums">{fmtBRL(row.icms_calculado)}</TableCell>
                  <TableCell className={`text-xs text-right tabular-nums font-semibold ${row.diferenca > 0.05 ? 'text-red-600' : row.diferenca < -0.05 ? 'text-yellow-600' : 'text-green-600'}`}>
                    {fmtBRL(row.diferenca)}
                  </TableCell>
                  <TableCell><StatusDivBadge status={row.status} /></TableCell>
                  <TableCell>
                    {row.status === 'COBRADO_A_MAIS' && (
                      <Button
                        size="sm"
                        variant="outline"
                        className="h-7 px-2 text-xs"
                        onClick={() => { setContestarRow(row); setCMotivo('') }}
                      >
                        Contestar
                      </Button>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      {/* Contestar dialog */}
      <Dialog open={!!contestarRow} onOpenChange={(o) => { if (!o) { setContestarRow(null); setCMotivo('') } }}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Nova Contestação</DialogTitle>
          </DialogHeader>
          {contestarRow && (
            <div className="space-y-3">
              <div className="rounded-md bg-muted/40 p-3 text-xs space-y-1">
                <p><span className="text-muted-foreground">NF:</span> <span className="font-mono">{contestarRow.numero_nf}</span></p>
                <p><span className="text-muted-foreground">Fornecedor:</span> {contestarRow.forn_nome}</p>
                <p><span className="text-muted-foreground">Período:</span> {contestarRow.periodo}</p>
                <p>
                  <span className="text-muted-foreground">Diferença:</span>{' '}
                  <span className="font-semibold text-red-600">{fmtBRL(Math.abs(contestarRow.diferenca))}</span>
                  {' '}(SEFAZ cobrou {fmtBRL(contestarRow.icms_sefaz)}, calculamos {fmtBRL(contestarRow.icms_calculado)})
                </p>
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="c-motivo-div">Motivo da contestação *</Label>
                <Textarea
                  id="c-motivo-div"
                  rows={4}
                  placeholder="Descreva o motivo da contestação..."
                  value={cMotivo}
                  onChange={(e) => setCMotivo(e.target.value)}
                />
              </div>
            </div>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={() => { setContestarRow(null); setCMotivo('') }}>Cancelar</Button>
            <Button
              onClick={handleContestar}
              disabled={contestarMutation.isPending || !cMotivo.trim()}
            >
              {contestarMutation.isPending ? 'Salvando...' : 'Criar Contestação'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Planilha tab — item-level view
// ---------------------------------------------------------------------------
function PlanilhaTab({ token }: { token: string | null }) {
  const [regimeFilter, setRegimeFilter] = useState('todos')
  const [monthInput, setMonthInput] = useState('')
  const periodo = monthToPeriodo(monthInput)

  const { data, isLoading, isError } = useQuery<FronteiraItensResponse>({
    queryKey: ['icms-fronteira/itens', regimeFilter, periodo],
    queryFn: async () => {
      const params = new URLSearchParams({ regime: regimeFilter })
      if (periodo) params.set('periodo', periodo)
      const res = await fetch(`/api/icms-fronteira/itens?${params}`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      return res.json()
    },
  })

  // Group rows by chave_nfe for subtotal display
  type NfGroup = { key: string; rows: FronteiraItemRow[]; subtotal: number }
  const groups: NfGroup[] = []
  if (data?.rows) {
    const map = new Map<string, NfGroup>()
    for (const row of data.rows) {
      if (!map.has(row.chave_nfe)) {
        map.set(row.chave_nfe, { key: row.chave_nfe, rows: [], subtotal: 0 })
      }
      const g = map.get(row.chave_nfe)!
      g.rows.push(row)
      g.subtotal += row.icms_calculado
    }
    groups.push(...map.values())
  }

  return (
    <div className="space-y-4">
      {/* Controls */}
      <div className="flex items-center gap-3 flex-wrap justify-between">
        <div className="flex items-center gap-2 flex-wrap">
          <span className="text-xs text-muted-foreground whitespace-nowrap">Período:</span>
          <Input
            type="month"
            className="w-36 text-xs h-8"
            value={monthInput}
            onChange={(e) => setMonthInput(e.target.value)}
          />
          <span className="text-xs text-muted-foreground whitespace-nowrap">Regime:</span>
          <Select value={regimeFilter} onValueChange={setRegimeFilter}>
            <SelectTrigger className="w-44 text-xs">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="todos">Todos</SelectItem>
              <SelectItem value="ANTECIPACAO">Antecipação</SelectItem>
              <SelectItem value="ST">Subst. Tributária</SelectItem>
              <SelectItem value="DIFAL">DIFAL</SelectItem>
            </SelectContent>
          </Select>
          <Button size="sm" variant="outline" onClick={() => {
            const params = new URLSearchParams({ regime: regimeFilter })
            if (periodo) params.set('periodo', periodo)
            const a = document.createElement('a')
            a.href = `/api/icms-fronteira/itens/exportar/csv?${params}`
            a.download = `icms-fronteira-itens-${regimeFilter}.csv`
            document.body.appendChild(a); a.click(); document.body.removeChild(a)
          }}>
            <FileDown className="h-3.5 w-3.5 mr-1" />CSV
          </Button>
          <Button size="sm" variant="outline" onClick={() => {
            const params = new URLSearchParams({ regime: regimeFilter })
            if (periodo) params.set('periodo', periodo)
            const a = document.createElement('a')
            a.href = `/api/icms-fronteira/itens/exportar/xlsx?${params}`
            a.download = `icms-fronteira-itens-${regimeFilter}.xlsx`
            document.body.appendChild(a); a.click(); document.body.removeChild(a)
          }}>
            <FileSpreadsheet className="h-3.5 w-3.5 mr-1" />Excel
          </Button>
        </div>
        {data && (
          <span className="text-xs text-muted-foreground">
            {data.count} item{data.count !== 1 ? 's' : ''} (máx. 2000) —{' '}
            <span className="font-semibold text-foreground">ICMS total: {fmtBRL(data.total)}</span>
          </span>
        )}
      </div>

      {isLoading && (
        <div className="space-y-2">
          {Array.from({ length: 8 }).map((_, i) => <Skeleton key={i} className="h-7 w-full" />)}
        </div>
      )}

      {isError && (
        <Alert variant="destructive">
          <AlertDescription>Erro ao carregar planilha de itens. Verifique sua conexão.</AlertDescription>
        </Alert>
      )}

      {data && data.rows.length === 0 && <EmptyState />}

      {data && data.rows.length > 0 && (
        <div className="rounded-md border overflow-x-auto text-xs">
          <Table>
            <TableHeader>
              <TableRow className="bg-muted/30 hover:bg-transparent">
                <TableHead className="font-semibold uppercase tracking-wide whitespace-nowrap">Data</TableHead>
                <TableHead className="font-semibold uppercase tracking-wide whitespace-nowrap">NF-e</TableHead>
                <TableHead className="font-semibold uppercase tracking-wide">Fornecedor</TableHead>
                <TableHead className="font-semibold uppercase tracking-wide">UF</TableHead>
                <TableHead className="font-semibold uppercase tracking-wide">CFOP</TableHead>
                <TableHead className="font-semibold uppercase tracking-wide">Regime</TableHead>
                <TableHead className="font-semibold uppercase tracking-wide text-right">#</TableHead>
                <TableHead className="font-semibold uppercase tracking-wide">Cód.</TableHead>
                <TableHead className="font-semibold uppercase tracking-wide">Descrição</TableHead>
                <TableHead className="font-semibold uppercase tracking-wide">NCM</TableHead>
                <TableHead className="font-semibold uppercase tracking-wide">CEST</TableHead>
                <TableHead className="font-semibold uppercase tracking-wide text-right">V.Prod</TableHead>
                <TableHead className="font-semibold uppercase tracking-wide text-right">V.IPI</TableHead>
                <TableHead className="font-semibold uppercase tracking-wide text-right">V.Outro</TableHead>
                <TableHead className="font-semibold uppercase tracking-wide text-right">V.Operação</TableHead>
                <TableHead className="font-semibold uppercase tracking-wide text-right">V.ICMS</TableHead>
                <TableHead className="font-semibold uppercase tracking-wide text-right">A.Inter%</TableHead>
                <TableHead className="font-semibold uppercase tracking-wide text-right">A.Int%</TableHead>
                <TableHead className="font-semibold uppercase tracking-wide text-right">BC</TableHead>
                <TableHead className="font-semibold uppercase tracking-wide text-right">MVA%</TableHead>
                <TableHead className="font-semibold uppercase tracking-wide text-right">BC-ST</TableHead>
                <TableHead className="font-semibold uppercase tracking-wide text-right">ICMS Calc.</TableHead>
                <TableHead className="font-semibold uppercase tracking-wide text-right">ICMS Ret.</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {groups.map((group) => (
                <>
                  {group.rows.map((row, idx) => (
                    <TableRow key={`${row.chave_nfe}-${row.n_item}`} className={idx % 2 === 0 ? '' : 'bg-muted/10'}>
                      <TableCell className="font-mono whitespace-nowrap">
                        {idx === 0 ? (row.data_emissao ? row.data_emissao.slice(0, 10) : '—') : ''}
                      </TableCell>
                      <TableCell className="font-mono whitespace-nowrap">
                        {idx === 0 ? (row.numero_nfe || '—') : ''}
                      </TableCell>
                      <TableCell className="max-w-[140px]">
                        {idx === 0 ? (
                          <div>
                            <div className="truncate" title={row.forn_nome}>{row.forn_nome || '—'}</div>
                            <div className="text-[10px] text-muted-foreground font-mono">{formatCNPJ(row.forn_cnpj)}</div>
                          </div>
                        ) : null}
                      </TableCell>
                      <TableCell className="font-mono font-semibold">
                        {idx === 0 ? row.forn_uf : ''}
                      </TableCell>
                      <TableCell className="font-mono">
                        {idx === 0 ? row.cfop : ''}
                      </TableCell>
                      <TableCell>
                        {idx === 0 ? (
                          <div className="flex items-center gap-1">
                            <RegimeBadge regime={row.regime} />
                            {row.forn_simples && (
                              <span className="inline-flex items-center px-1 py-0.5 rounded text-[9px] font-medium bg-green-100 text-green-700 border border-green-200">SN</span>
                            )}
                          </div>
                        ) : null}
                      </TableCell>
                      <TableCell className="text-right tabular-nums">{row.n_item}</TableCell>
                      <TableCell className="font-mono">{row.c_prod || '—'}</TableCell>
                      <TableCell className="max-w-[160px]">
                        <div className="truncate" title={row.x_prod}>{row.x_prod || '—'}</div>
                      </TableCell>
                      <TableCell className="font-mono">{row.ncm || '—'}</TableCell>
                      <TableCell className="font-mono">{row.cest || '—'}</TableCell>
                      <TableCell className="text-right tabular-nums">{fmtBRL(row.v_prod_item)}</TableCell>
                      <TableCell className="text-right tabular-nums">{row.v_ipi_item > 0 ? fmtBRL(row.v_ipi_item) : '—'}</TableCell>
                      <TableCell className="text-right tabular-nums">{row.v_outro_rateado > 0 ? fmtBRL(row.v_outro_rateado) : '—'}</TableCell>
                      <TableCell className="text-right tabular-nums">{fmtBRL(row.v_operacao)}</TableCell>
                      <TableCell className="text-right tabular-nums">{row.v_icms_item > 0 ? fmtBRL(row.v_icms_item) : '—'}</TableCell>
                      <TableCell className="text-right tabular-nums">{fmtPct(row.aliq_inter)}</TableCell>
                      <TableCell className="text-right tabular-nums">{fmtPct(row.aliq_interna)}</TableCell>
                      <TableCell className="text-right tabular-nums">{fmtBRL(row.bc)}</TableCell>
                      <TableCell className="text-right tabular-nums text-muted-foreground">{row.mva_original != null ? fmtPct(row.mva_original) : '—'}</TableCell>
                      <TableCell className="text-right tabular-nums text-muted-foreground">{row.bc_st > 0 ? fmtBRL(row.bc_st) : '—'}</TableCell>
                      <TableCell className="text-right tabular-nums font-semibold">{fmtBRL(row.icms_calculado)}</TableCell>
                      <TableCell className="text-right tabular-nums">{row.icms_retido > 0 ? fmtBRL(row.icms_retido) : '—'}</TableCell>
                    </TableRow>
                  ))}
                  {/* Subtotal per NF */}
                  <TableRow className="bg-muted/40 border-t border-border">
                    <TableCell colSpan={3} className="text-[10px] font-mono text-muted-foreground truncate max-w-[120px]">
                      NF: {group.key.slice(0, 20)}…
                    </TableCell>
                    <TableCell colSpan={19} className="text-right text-xs font-semibold tabular-nums">
                      Subtotal NF: {fmtBRL(group.subtotal)}
                    </TableCell>
                  </TableRow>
                </>
              ))}
              {/* Grand total */}
              <TableRow className="bg-muted/60 border-t-2 border-border">
                <TableCell colSpan={21} className="text-right text-xs font-bold">
                  Total ICMS Calculado
                </TableCell>
                <TableCell className="text-right text-xs font-bold tabular-nums">
                  {fmtBRL(data.total)}
                </TableCell>
                <TableCell />
              </TableRow>
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Apuração Mensal tab (Bloco D)
// ---------------------------------------------------------------------------
interface FronteiraMensalRow {
  periodo: string
  regime: string
  qtd_notas: number
  v_prod_total: number
  icms_devido: number
}
interface FronteiraMensalResponse {
  rows: FronteiraMensalRow[]
  total_devido: number
  total_prod: number
}

// Palette: matches the app's CSS variables where possible
const REGIME_COLORS: Record<string, string> = {
  ANTECIPACAO: '#3b82f6',
  ST:          '#f59e0b',
  DIFAL:       '#8b5cf6',
}

function ApuracaoMensalTab({ token }: { token: string | null }) {
  const { data, isLoading, isError } = useQuery<FronteiraMensalResponse>({
    queryKey: ['icms-fronteira/mensal'],
    queryFn: async () => {
      const res = await fetch('/api/icms-fronteira/mensal', {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      return res.json()
    },
  })

  if (isLoading) {
    return (
      <div className="space-y-3">
        {[0, 1, 2].map((i) => <Skeleton key={i} className="h-10 w-full" />)}
      </div>
    )
  }

  if (isError) {
    return (
      <Alert variant="destructive">
        <AlertDescription>Erro ao carregar apuração mensal. Verifique sua conexão.</AlertDescription>
      </Alert>
    )
  }

  if (!data || data.rows.length === 0) {
    return <EmptyState />
  }

  // Pivot rows into { periodo, ANTECIPACAO, ST, DIFAL } for recharts
  const periodos = [...new Set(data.rows.map((r) => r.periodo))].sort((a, b) => {
    const [ma, ya] = a.split('/').map(Number)
    const [mb, yb] = b.split('/').map(Number)
    return ya !== yb ? ya - yb : ma - mb
  })
  const chartData = periodos.map((p) => {
    const entry: Record<string, number | string> = { periodo: p }
    for (const row of data.rows.filter((r) => r.periodo === p)) {
      entry[row.regime] = row.icms_devido
    }
    return entry
  })

  const regimes = [...new Set(data.rows.map((r) => r.regime))].sort()

  // KPI cards: last period per regime
  const lastPeriodo = periodos[periodos.length - 1]
  const lastRows = data.rows.filter((r) => r.periodo === lastPeriodo)

  return (
    <div className="space-y-6">
      {/* KPI: last period */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {lastRows.map((row) => (
          <Card key={row.regime}>
            <CardContent className="pt-5 pb-4">
              <div className="flex items-center justify-between mb-2">
                <span className="text-xs text-muted-foreground">{lastPeriodo} — {row.regime}</span>
                <RegimeBadge regime={row.regime} />
              </div>
              <p className="text-2xl font-bold tabular-nums">{fmtBRL(row.icms_devido)}</p>
              <p className="text-xs text-muted-foreground mt-1">
                {row.qtd_notas} nota{row.qtd_notas !== 1 ? 's' : ''} · Prod.: {fmtBRL(row.v_prod_total)}
              </p>
            </CardContent>
          </Card>
        ))}
      </div>

      {/* Chart */}
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-semibold flex items-center gap-2">
            <BarChart2 className="h-4 w-4 text-muted-foreground" />
            Evolução Mensal — ICMS Devido Estimado por Regime
          </CardTitle>
        </CardHeader>
        <CardContent>
          <ResponsiveContainer width="100%" height={300}>
            <BarChart data={chartData} margin={{ top: 4, right: 16, left: 8, bottom: 4 }}>
              <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
              <XAxis dataKey="periodo" tick={{ fontSize: 11 }} />
              <YAxis tickFormatter={(v) => `R$${(v / 1000).toFixed(0)}k`} tick={{ fontSize: 11 }} width={64} />
              <RechartsTooltip
                formatter={(value: number, name: string) => [fmtBRL(value), name]}
                labelStyle={{ fontWeight: 600 }}
              />
              <Legend wrapperStyle={{ fontSize: 12 }} />
              {regimes.map((regime) => (
                <Bar
                  key={regime}
                  dataKey={regime}
                  name={regime === 'ANTECIPACAO' ? 'Antecipação' : regime === 'ST' ? 'Subst. Tributária' : 'DIFAL'}
                  fill={REGIME_COLORS[regime] ?? '#6b7280'}
                  stackId="a"
                />
              ))}
            </BarChart>
          </ResponsiveContainer>
        </CardContent>
      </Card>

      {/* Summary table */}
      <div className="rounded-md border overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow className="bg-muted/30 hover:bg-transparent">
              <TableHead className="text-xs font-semibold uppercase tracking-wide">Período</TableHead>
              <TableHead className="text-xs font-semibold uppercase tracking-wide">Regime</TableHead>
              <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">Notas</TableHead>
              <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">V. Produtos</TableHead>
              <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">ICMS Devido Est.</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {data.rows.map((row, i) => (
              <TableRow key={`${row.periodo}-${row.regime}-${i}`}>
                <TableCell className="text-xs font-mono">{row.periodo}</TableCell>
                <TableCell><RegimeBadge regime={row.regime} /></TableCell>
                <TableCell className="text-xs text-right tabular-nums">{row.qtd_notas}</TableCell>
                <TableCell className="text-xs text-right tabular-nums">{fmtBRL(row.v_prod_total)}</TableCell>
                <TableCell className="text-xs text-right tabular-nums font-semibold">{fmtBRL(row.icms_devido)}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// IcmsFronteira — main page
// ---------------------------------------------------------------------------
export default function IcmsFronteira() {
  const location = useLocation()
  const navigate = useNavigate()
  const { token } = useAuth()

  const pathToTab: Record<string, string> = {
    '/icms-fronteira':              'resumo',
    '/icms-fronteira/antecipacao':  'antecipacao',
    '/icms-fronteira/st':           'st',
    '/icms-fronteira/difal':        'difal',
    '/icms-fronteira/planilha':     'planilha',
    '/icms-fronteira/divergencias': 'divergencias',
    '/icms-fronteira/regras':       'regras',
    '/icms-fronteira/extrato':      'extrato',
    '/icms-fronteira/contestacoes': 'contestacoes',
    '/icms-fronteira/apuracao':     'apuracao',
  }
  const tabToPath: Record<string, string> = {
    resumo:        '/icms-fronteira',
    antecipacao:   '/icms-fronteira/antecipacao',
    st:            '/icms-fronteira/st',
    difal:         '/icms-fronteira/difal',
    planilha:      '/icms-fronteira/planilha',
    divergencias:  '/icms-fronteira/divergencias',
    regras:        '/icms-fronteira/regras',
    extrato:       '/icms-fronteira/extrato',
    contestacoes:  '/icms-fronteira/contestacoes',
    apuracao:      '/icms-fronteira/apuracao',
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
        <TabsList className="flex-wrap h-auto gap-1">
          <TabsTrigger value="resumo">Resumo</TabsTrigger>
          <TabsTrigger value="antecipacao">Antecipação</TabsTrigger>
          <TabsTrigger value="st">Subst. Tributária</TabsTrigger>
          <TabsTrigger value="difal">DIFAL</TabsTrigger>
          <TabsTrigger value="planilha">Planilha</TabsTrigger>
          <TabsTrigger value="divergencias">Divergências</TabsTrigger>
          <TabsTrigger value="apuracao">Apuração Mensal</TabsTrigger>
          <TabsTrigger value="regras">Regras NCM</TabsTrigger>
          <TabsTrigger value="extrato">Extrato SEFAZ</TabsTrigger>
          <TabsTrigger value="contestacoes">Contestações</TabsTrigger>
        </TabsList>

        <TabsContent value="resumo" className="mt-6">
          <ResumoTab token={token} />
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
              <div className="flex items-center gap-2 mb-4 justify-end flex-wrap">
                <ExportButtons regime="antecipacao" token={token} />
                <RecalcularButton />
              </div>
              <NotasTab endpoint="/api/icms-fronteira/antecipacao" regime="antecipacao" token={token} />
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
              <div className="flex items-center gap-2 mb-4 justify-end flex-wrap">
                <ExportButtons regime="st" token={token} />
                <RecalcularButton />
              </div>
              <NotasTab endpoint="/api/icms-fronteira/st" regime="st" token={token} />
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
              <div className="flex items-center gap-2 mb-4 justify-end flex-wrap">
                <ExportButtons regime="difal" token={token} />
                <RecalcularButton />
              </div>
              <NotasTab endpoint="/api/icms-fronteira/difal" regime="difal" token={token} />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="divergencias" className="mt-6">
          <Card>
            <CardHeader>
              <CardTitle className="text-base font-semibold flex items-center gap-2">
                <AlertTriangle className="h-4 w-4 text-red-500" />
                Divergências — Calculado × SEFAZ
              </CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-xs text-muted-foreground mb-4">
                Cruzamento entre o ICMS calculado pelo sistema e o cobrado no Extrato SEFAZ-PE.
                Selecione um período para identificar cobranças indevidas e gerar contestações.
              </p>
              <DivergenciasTab token={token} />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="planilha" className="mt-6">
          <Card>
            <CardHeader>
              <CardTitle className="text-base font-semibold flex items-center gap-2">
                <FileSpreadsheet className="h-4 w-4 text-slate-500" />
                Planilha de Itens
              </CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-xs text-muted-foreground mb-4">
                Detalhamento por item com BC correta, ICMS calculado e ICMS retido.
                Um item por linha, com subtotal por NF e total geral.
              </p>
              <PlanilhaTab token={token} />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="apuracao" className="mt-6">
          <Card>
            <CardHeader>
              <CardTitle className="text-base font-semibold flex items-center gap-2">
                <BarChart2 className="h-4 w-4 text-blue-500" />
                Apuração Mensal
              </CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-xs text-muted-foreground mb-4">
                Evolução do ICMS Fronteira estimado por regime ao longo dos meses, com gráfico de barras empilhadas.
              </p>
              <ApuracaoMensalTab token={token} />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="regras" className="mt-6">
          <Card>
            <CardHeader>
              <CardTitle className="text-base font-semibold">
                Regras NCM
              </CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-xs text-muted-foreground mb-4">
                Defina regras de regime tributário por prefixo NCM. Regras globais são pré-definidas pelo sistema e não podem ser removidas.
              </p>
              <RegrasTab token={token} />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="extrato" className="mt-6">
          <Card>
            <CardHeader>
              <CardTitle className="text-base font-semibold">
                Extrato SEFAZ
              </CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-xs text-muted-foreground mb-4">
                Importe e consulte o extrato oficial da SEFAZ-PE de ICMS devido na fronteira por período.
              </p>
              <ExtratoTab token={token} />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="contestacoes" className="mt-6">
          <Card>
            <CardHeader>
              <CardTitle className="text-base font-semibold">
                Contestações
              </CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-xs text-muted-foreground mb-4">
                Gerencie contestações de cobranças indevidas de ICMS Fronteira junto à SEFAZ-PE.
              </p>
              <ContestacoesTab token={token} />
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  )
}
