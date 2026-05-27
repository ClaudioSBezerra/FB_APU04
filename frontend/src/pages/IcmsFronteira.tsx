import { createContext, useContext, useEffect, useState } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Table,
  TableBody,
  TableCell,
  TableFooter,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
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
  Sparkles,
  ChevronDown,
  ChevronRight,
  Clock,
  FileQuestion,
  CheckCircle2,
  Truck,
  Calculator,
  Play,
  Loader2,
  Pencil,
  ShieldCheck,
  BadgePercent,
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
import { AdministrativoTab } from './AdministrativoFronteira'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------
interface FronteiraResumoRow {
  regime: string
  qtd_notas: number
  v_prod_total: number
  v_ipi_total: number
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
  v_ipi: number
  v_icms: number
  v_bc_st: number
  v_st: number
  aliq_inter: number
  aliq_interna: number
  icms_devido_est: number
  regime: string
  bloco: string // 'mes_atual' | 'mes_anterior'
}

interface FronteiraNotasResponse {
  rows: FronteiraNotaRow[]
  total: number
  count: number
  total_mes_atual: number
  total_mes_anterior: number
  count_mes_atual: number
  count_mes_anterior: number
}

interface FronteiraXmlNaoSpedRow {
  chave_nfe: string
  data_emissao: string
  numero_nfe: string
  forn_cnpj: string
  forn_nome: string
  forn_uf: string
  cfop_saida: string
  ncm: string
  v_prod: number
  v_ipi: number
  v_frete: number
  v_frete_cte: number
  v_outro: number
  v_opr: number
  v_icms_nf: number
  v_icms_cte: number
  aliq_inter: number
  aliq_interna: number
  mva: number
  icms_devido_est: number
  regime: string
  class_status: string // 'auto' | 'manual'
}

interface FronteiraXmlNaoSpedResponse {
  rows: FronteiraXmlNaoSpedRow[]
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
  mva_ajustado_4pct: number | null
  mva_ajustado_7pct: number | null
  mva_ajustado_12pct: number | null
  reducao_bc_pct: number
  uf_estado: string
  is_global: boolean
  segmento_codigo: number | null
}

interface RegrasResponse {
  rows: RegraNCM[]
  count: number
}

interface SegmentoUFOption {
  codigo: number
  uf: string
  descricao: string
  ativo: boolean
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

interface FreteRow {
  chave_nfe: string
  numero_nfe: string
  data_emissao: string
  forn_nome: string
  forn_cnpj: string
  forn_uf: string
  regime: string
  chave_cte: string
  numero_cte: string
  emit_nome: string
  emit_cnpj: string
  v_prest: number
  v_icms_cte: number
  icms_fronteira: number
  fonte: string
  toma: string
}

interface FretesResponse {
  rows: FreteRow[]
  count: number
  total_v_prest: number
  total_icms_fronteira: number
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

// Aba Incentivo — relatório das notas dispensadas pelo motor (PRODEPE/PROIND).
// Espelha as colunas de Antecipação/ST e ADICIONA programa+ato+vigência+economia.
interface IncentivoRow {
  chave_nfe: string
  data_emissao: string
  numero_nfe: string
  forn_cnpj: string
  forn_nome: string
  forn_uf: string
  cfop: string
  v_prod: number
  v_ipi: number
  v_icms: number
  aliq_inter: number
  aliq_interna: number
  regime: string
  bloco: string
  cnpj_filial: string
  programa: string
  num_ato: string
  vigencia_inicio: string
  vigencia_fim: string
  icms_seria_devido: number
}

interface IncentivoResponse {
  rows: IncentivoRow[]
  count: number
  total_dispensado: number
  total_mes_atual: number
  total_mes_anterior: number
  total_nao_sped: number
  count_mes_atual: number
  count_mes_anterior: number
  count_nao_sped: number
  por_programa: Record<string, number>
  por_filial: Record<string, number>
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

// Normaliza o período para MM/YYYY. Aceita MM/YYYY (digitado direto) e o legado
// YYYY-MM do <input type="month">. Retorna '' se inválido (não dispara query).
function monthToPeriodo(m: string): string {
  if (!m) return ''
  const s = m.trim()
  if (/^\d{2}\/\d{4}$/.test(s)) return s
  const [y, mo] = s.split('-')
  return mo && y ? `${mo}/${y}` : ''
}

// Célula compacta de chave de 44 dígitos: mostra os últimos 9 dígitos
// (cNF + DV) com tooltip da chave completa e clique para copiar.
function TomadorBadge({ toma }: { toma: string }) {
  const map: Record<string, { label: string; cls: string }> = {
    '0': { label: 'Remetente',    cls: 'bg-amber-100 text-amber-800 border-amber-200' },
    '1': { label: 'Expedidor',    cls: 'bg-amber-100 text-amber-800 border-amber-200' },
    '2': { label: 'Recebedor',    cls: 'bg-amber-100 text-amber-800 border-amber-200' },
    '3': { label: 'Destinatário', cls: 'bg-green-100 text-green-800 border-green-200' },
    '4': { label: 'Outros',       cls: 'bg-slate-100 text-slate-700 border-slate-200' },
  }
  const m = map[toma] ?? { label: 'n/d', cls: 'bg-gray-100 text-gray-500 border-gray-200' }
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded text-[10px] font-medium border ${m.cls}`}>
      {m.label}
    </span>
  )
}

function ChaveCell({ chave, label }: { chave: string; label?: string }) {
  if (!chave) return <span className="text-muted-foreground">—</span>
  const short = chave.length === 44 ? chave.slice(-9) : chave
  return (
    <TooltipProvider delayDuration={200}>
      <Tooltip>
        <TooltipTrigger asChild>
          <button
            type="button"
            className="text-[10px] font-mono text-muted-foreground hover:text-foreground cursor-pointer"
            onClick={() => { navigator.clipboard?.writeText(chave) }}
            title="Clique para copiar"
          >
            …{short}
          </button>
        </TooltipTrigger>
        <TooltipContent side="top" className="max-w-md">
          <div className="text-[10px] font-mono break-all">{chave}</div>
          {label && <div className="text-[10px] text-muted-foreground mt-1">{label}</div>}
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
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
// Filtros de notas (fornecedor / número da nota / intervalo de data)
// ---------------------------------------------------------------------------
interface FronteiraFiltros {
  forn?: string
  num_nota?: string
  data_ini?: string
  data_fim?: string
  uf?: string
}

// aplicaFiltros adiciona os filtros não-vazios a um URLSearchParams.
function aplicaFiltros(params: URLSearchParams, f?: FronteiraFiltros) {
  if (!f) return
  if (f.uf?.trim()) params.set('uf', f.uf.trim())
  if (f.forn?.trim()) params.set('forn', f.forn.trim())
  if (f.num_nota?.trim()) params.set('num_nota', f.num_nota.trim())
  if (f.data_ini) params.set('data_ini', f.data_ini)
  if (f.data_fim) params.set('data_fim', f.data_fim)
}

// UF selecionada para o módulo inteiro (eixo UF). Provida pelo componente raiz.
const FronteiraUFContext = createContext<string>('')
const useFronteiraUF = () => useContext(FronteiraUFContext)

// ---------------------------------------------------------------------------
// Export buttons (shared by tabs)
// ---------------------------------------------------------------------------
function ExportButtons({ regime, token, periodo, filtros }: { regime: string; token: string | null; periodo?: string; filtros?: FronteiraFiltros }) {
  const uf = useFronteiraUF()
  async function downloadFile(format: 'csv' | 'xlsx') {
    try {
      const params = new URLSearchParams({ regime })
      if (periodo) params.set('periodo', periodo)
      if (token) params.set('token', token)
      if (uf) params.set('uf', uf)
      aplicaFiltros(params, filtros)
      const res = await fetch(`/api/icms-fronteira/exportar/${format}?${params}`)
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
    if (token) params.set('token', token)
    if (uf) params.set('uf', uf)
    aplicaFiltros(params, filtros)
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
  const uf = useFronteiraUF()

  const { data, isLoading, isError } = useQuery<FronteiraResumoResponse>({
    queryKey: ['icms-fronteira/resumo', periodo, uf],
    queryFn: async () => {
      const params = new URLSearchParams()
      if (periodo) params.set('periodo', periodo)
      if (uf) params.set('uf', uf)
      const qs = params.toString()
      const res = await fetch(`/api/icms-fronteira/resumo${qs ? `?${qs}` : ''}`, {
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
            type="text"
            placeholder="MM/AAAA"
            maxLength={7}
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
              <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">V. IPI</TableHead>
              <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">V. Operação</TableHead>
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
                <TableCell className="text-xs text-right tabular-nums">{fmtBRL(row.v_ipi_total)}</TableCell>
                <TableCell className="text-xs text-right tabular-nums font-medium">{fmtBRL((row.v_prod_total || 0) + (row.v_ipi_total || 0))}</TableCell>
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
          type="text"
          placeholder="MM/AAAA"
          maxLength={7}
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
              <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">V. IPI</TableHead>
              <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">V. Operação</TableHead>
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
                <TableCell className="text-xs text-right tabular-nums">{fmtBRL(row.v_ipi)}</TableCell>
                <TableCell className="text-xs text-right tabular-nums font-medium">{fmtBRL((row.v_prod || 0) + (row.v_ipi || 0))}</TableCell>
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
// NotasTabBlocos — 3 blocks: mes_anterior (SPED), mes_atual (SPED), nao_sped (XML)
// ---------------------------------------------------------------------------

type RegimedStr = 'antecipacao' | 'st' | 'difal'
const REGIME_PARAM: Record<RegimedStr, string> = {
  antecipacao: 'ANTECIPACAO',
  st: 'ST',
  difal: 'DIFAL',
}

function TabelaNotasSped({
  rows,
  showAliq,
}: {
  rows: FronteiraNotaRow[]
  showAliq: boolean
}) {
  const totalVProd = rows.reduce((a, r) => a + (r.v_prod || 0), 0)
  const totalVIpi  = rows.reduce((a, r) => a + (r.v_ipi || 0), 0)
  const totalVOpr  = totalVProd + totalVIpi
  const totalVIcms = rows.reduce((a, r) => a + (r.v_icms || 0), 0)
  const totalIcms  = rows.reduce((a, r) => a + (r.icms_devido_est || 0), 0)
  return (
    <div className="rounded-md border overflow-x-auto">
      <Table>
        <TableHeader>
          <TableRow className="bg-muted/30 hover:bg-transparent">
            <TableHead className="text-xs font-semibold uppercase tracking-wide">Data</TableHead>
            <TableHead className="text-xs font-semibold uppercase tracking-wide">NF-e</TableHead>
            <TableHead className="text-xs font-semibold uppercase tracking-wide">Fornecedor</TableHead>
            <TableHead className="text-xs font-semibold uppercase tracking-wide">UF</TableHead>
            <TableHead className="text-xs font-semibold uppercase tracking-wide">CFOP</TableHead>
            <TableHead className="text-xs font-semibold uppercase tracking-wide">Chave NF-e</TableHead>
            <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">V. Prod.</TableHead>
            <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">V. IPI</TableHead>
            <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">V. Operação</TableHead>
            <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">ICMS NF</TableHead>
            {showAliq && (
              <>
                <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">Alíq. Inter.</TableHead>
                <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">Alíq. Int.</TableHead>
              </>
            )}
            <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">ICMS fronteira</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((row, idx) => (
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
              <TableCell className="text-xs"><ChaveCell chave={row.chave_nfe} label={`NF-e ${row.numero_nfe}`} /></TableCell>
              <TableCell className="text-xs text-right tabular-nums">{fmtBRL(row.v_prod)}</TableCell>
              <TableCell className="text-xs text-right tabular-nums">{fmtBRL(row.v_ipi)}</TableCell>
              <TableCell className="text-xs text-right tabular-nums font-medium">{fmtBRL((row.v_prod || 0) + (row.v_ipi || 0))}</TableCell>
              <TableCell className="text-xs text-right tabular-nums">{fmtBRL(row.v_icms)}</TableCell>
              {showAliq && (
                <>
                  <TableCell className="text-xs text-right tabular-nums">{fmtPct(row.aliq_inter)}</TableCell>
                  <TableCell className="text-xs text-right tabular-nums">{fmtPct(row.aliq_interna)}</TableCell>
                </>
              )}
              <TableCell className="text-xs text-right tabular-nums font-semibold">
                {fmtBRL(row.icms_devido_est)}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
        {rows.length > 0 && (
          <TableFooter>
            <TableRow className="bg-muted/60 hover:bg-muted/60">
              <TableCell colSpan={6} className="text-xs font-bold uppercase">Total — {rows.length} nota{rows.length !== 1 ? 's' : ''}</TableCell>
              <TableCell className="text-xs text-right tabular-nums font-bold">{fmtBRL(totalVProd)}</TableCell>
              <TableCell className="text-xs text-right tabular-nums font-bold">{fmtBRL(totalVIpi)}</TableCell>
              <TableCell className="text-xs text-right tabular-nums font-bold">{fmtBRL(totalVOpr)}</TableCell>
              <TableCell className="text-xs text-right tabular-nums font-bold">{fmtBRL(totalVIcms)}</TableCell>
              {showAliq && <TableCell colSpan={2} />}
              <TableCell className="text-xs text-right tabular-nums font-bold">{fmtBRL(totalIcms)}</TableCell>
            </TableRow>
          </TableFooter>
        )}
      </Table>
    </div>
  )
}

// TabelaNotasXml (Bloco C) — MESMAS colunas e regras de preenchimento do Bloco
// A/B (TabelaNotasSped), acrescidas das colunas de frete de CT-e (V.Frete CT-e
// e ICMS CT-e), que integram o ICMS fronteira quando tomador=destinatário.
function TabelaNotasXml({ rows, showAliq }: { rows: FronteiraXmlNaoSpedRow[]; showAliq: boolean }) {
  const totalVProd     = rows.reduce((acc, r) => acc + (r.v_prod || 0), 0)
  const totalVIpi      = rows.reduce((acc, r) => acc + (r.v_ipi || 0), 0)
  const totalVOpr      = totalVProd + totalVIpi
  const totalVFreteCTe = rows.reduce((acc, r) => acc + (r.v_frete_cte || 0), 0)
  const totalVIcms     = rows.reduce((acc, r) => acc + (r.v_icms_nf || 0), 0)
  const totalVIcmsCTe  = rows.reduce((acc, r) => acc + (r.v_icms_cte || 0), 0)
  const totalIcms      = rows.reduce((acc, r) => acc + (r.icms_devido_est || 0), 0)
  return (
    <div className="rounded-md border overflow-x-auto">
      <Table>
        <TableHeader>
          <TableRow className="bg-muted/30 hover:bg-transparent">
            <TableHead className="text-xs font-semibold uppercase tracking-wide">Data</TableHead>
            <TableHead className="text-xs font-semibold uppercase tracking-wide">NF-e</TableHead>
            <TableHead className="text-xs font-semibold uppercase tracking-wide">Fornecedor</TableHead>
            <TableHead className="text-xs font-semibold uppercase tracking-wide">UF</TableHead>
            <TableHead className="text-xs font-semibold uppercase tracking-wide">CFOP</TableHead>
            <TableHead className="text-xs font-semibold uppercase tracking-wide">Chave NF-e</TableHead>
            <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">V. Prod.</TableHead>
            <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">V. IPI</TableHead>
            <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">V. Operação</TableHead>
            <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">ICMS NF</TableHead>
            <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">V. Frete CT-e</TableHead>
            <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">ICMS CT-e</TableHead>
            {showAliq && (
              <>
                <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">Alíq. Inter.</TableHead>
                <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">Alíq. Int.</TableHead>
              </>
            )}
            <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">ICMS fronteira</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((row, idx) => (
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
              <TableCell className="text-xs font-mono">{row.cfop_saida || '—'}</TableCell>
              <TableCell className="text-xs"><ChaveCell chave={row.chave_nfe} label={`NF-e ${row.numero_nfe}`} /></TableCell>
              <TableCell className="text-xs text-right tabular-nums">{fmtBRL(row.v_prod)}</TableCell>
              <TableCell className="text-xs text-right tabular-nums">{fmtBRL(row.v_ipi)}</TableCell>
              <TableCell className="text-xs text-right tabular-nums font-medium">{fmtBRL((row.v_prod || 0) + (row.v_ipi || 0))}</TableCell>
              <TableCell className="text-xs text-right tabular-nums">{fmtBRL(row.v_icms_nf)}</TableCell>
              <TableCell className="text-xs text-right tabular-nums text-emerald-700">{fmtBRL(row.v_frete_cte)}</TableCell>
              <TableCell className="text-xs text-right tabular-nums text-emerald-700">{fmtBRL(row.v_icms_cte)}</TableCell>
              {showAliq && (
                <>
                  <TableCell className="text-xs text-right tabular-nums">{fmtPct(row.aliq_inter)}</TableCell>
                  <TableCell className="text-xs text-right tabular-nums">{fmtPct(row.aliq_interna)}</TableCell>
                </>
              )}
              <TableCell className="text-xs text-right tabular-nums font-semibold">
                {fmtBRL(row.icms_devido_est)}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
        {rows.length > 0 && (
          <TableFooter>
            <TableRow className="bg-muted/60 hover:bg-muted/60">
              <TableCell colSpan={6} className="text-xs font-bold uppercase">Total — {rows.length} nota{rows.length !== 1 ? 's' : ''}</TableCell>
              <TableCell className="text-xs text-right tabular-nums font-bold">{fmtBRL(totalVProd)}</TableCell>
              <TableCell className="text-xs text-right tabular-nums font-bold">{fmtBRL(totalVIpi)}</TableCell>
              <TableCell className="text-xs text-right tabular-nums font-bold">{fmtBRL(totalVOpr)}</TableCell>
              <TableCell className="text-xs text-right tabular-nums font-bold">{fmtBRL(totalVIcms)}</TableCell>
              <TableCell className="text-xs text-right tabular-nums font-bold text-emerald-700">{fmtBRL(totalVFreteCTe)}</TableCell>
              <TableCell className="text-xs text-right tabular-nums font-bold text-emerald-700">{fmtBRL(totalVIcmsCTe)}</TableCell>
              {showAliq && <TableCell colSpan={2} />}
              <TableCell className="text-xs text-right tabular-nums font-bold">{fmtBRL(totalIcms)}</TableCell>
            </TableRow>
          </TableFooter>
        )}
      </Table>
    </div>
  )
}

function BlocoHeader({
  open,
  icon,
  label,
  count,
  total,
  colorClass,
  onClick,
}: {
  open: boolean
  icon: React.ReactNode
  label: string
  count: number
  total: number
  colorClass: string
  onClick?: () => void
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`flex items-center justify-between w-full px-3 py-2 rounded-md border ${colorClass} cursor-pointer select-none text-left`}
    >
      <div className="flex items-center gap-2">
        {open ? <ChevronDown className="h-4 w-4 shrink-0" /> : <ChevronRight className="h-4 w-4 shrink-0" />}
        {icon}
        <span className="text-sm font-semibold">{label}</span>
        <Badge variant="secondary" className="text-xs">{count} nota{count !== 1 ? 's' : ''}</Badge>
      </div>
      <span className="text-sm font-semibold tabular-nums">{fmtBRL(total)}</span>
    </button>
  )
}

function NotasTabBlocos({
  endpointSped,
  regime,
  token,
}: {
  endpointSped: string
  regime: RegimedStr
  token: string | null
  // ExportButtons rendered inside with access to periodo
}) {
  const [monthInput, setMonthInput] = useState('')
  const periodo = monthToPeriodo(monthInput)

  // Filtros opcionais (fornecedor / número da nota / intervalo de data).
  const [forn, setForn] = useState('')
  const [numNota, setNumNota] = useState('')
  const [dataIni, setDataIni] = useState('')
  const [dataFim, setDataFim] = useState('')
  const uf = useFronteiraUF()
  const filtros: FronteiraFiltros = { forn, num_nota: numNota, data_ini: dataIni, data_fim: dataFim, uf }
  // chave de cache estável dos filtros (só dispara refetch quando mudam)
  const filtrosKey = `${uf}|${forn}|${numNota}|${dataIni}|${dataFim}`

  const [openA, setOpenA] = useState(true)
  const [openB, setOpenB] = useState(true)
  const [openC, setOpenC] = useState(true)

  const regimeParam = REGIME_PARAM[regime]
  const showAliq = regime !== 'st'

  // Bloco A + B — SPED
  const spedQuery = useQuery<FronteiraNotasResponse>({
    queryKey: ['icms-fronteira', regime, periodo, 'sped', filtrosKey],
    queryFn: async () => {
      const params = new URLSearchParams()
      if (periodo) params.set('periodo', periodo)
      aplicaFiltros(params, filtros)
      const qs = params.toString()
      const url = qs ? `${endpointSped}?${qs}` : endpointSped
      const res = await fetch(url, { headers: { Authorization: `Bearer ${token}` } })
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      return res.json()
    },
    enabled: !!periodo,
  })

  // Bloco C — XML não lançado no SPED
  const xmlQuery = useQuery<FronteiraXmlNaoSpedResponse>({
    queryKey: ['icms-fronteira-nao-sped', regime, periodo, uf],
    queryFn: async () => {
      if (!periodo) return { rows: [], total: 0, count: 0 }
      let url = `/api/icms-fronteira/nao-sped?periodo=${encodeURIComponent(periodo)}&regime=${encodeURIComponent(regimeParam)}`
      if (uf) url += `&uf=${encodeURIComponent(uf)}`
      const res = await fetch(url, { headers: { Authorization: `Bearer ${token}` } })
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      return res.json()
    },
    enabled: !!periodo,
  })

  const rowsAnterior = spedQuery.data?.rows.filter(r => r.bloco === 'mes_anterior') ?? []
  const rowsAtual    = spedQuery.data?.rows.filter(r => r.bloco === 'mes_atual') ?? []
  const rowsXml      = xmlQuery.data?.rows ?? []

  const totalAnterior = spedQuery.data?.total_mes_anterior ?? 0
  const totalAtual    = spedQuery.data?.total_mes_atual ?? 0
  const totalXml      = xmlQuery.data?.total ?? 0
  const totalGeral    = totalAtual + totalXml // mes_anterior não entra no total (já recolhido)

  return (
    <div className="space-y-4">
      {/* Filtro período + export */}
      <div className="flex items-center gap-2 flex-wrap justify-between">
        <div className="flex items-center gap-2">
          <Label htmlFor={`notas-periodo-${regime}`} className="text-xs whitespace-nowrap">Período (SPED):</Label>
          <Input
            id={`notas-periodo-${regime}`}
            type="text"
            placeholder="MM/AAAA"
            maxLength={7}
            className="w-36 text-xs h-8"
            value={monthInput}
            onChange={(e) => setMonthInput(e.target.value)}
          />
          {periodo && (
            <span className="text-xs text-muted-foreground">{periodo}</span>
          )}
        </div>
        {periodo && <ExportButtons regime={regime} token={token} periodo={periodo} filtros={filtros} />}
      </div>

      {/* Filtros: fornecedor / número da nota / intervalo de data */}
      {periodo && (
        <div className="flex items-end gap-2 flex-wrap rounded-md border bg-muted/20 p-2">
          <div className="flex flex-col gap-1">
            <Label className="text-[10px] uppercase text-muted-foreground">Fornecedor (nome/CNPJ)</Label>
            <Input
              type="text"
              placeholder="Buscar fornecedor..."
              className="w-52 text-xs h-8"
              value={forn}
              onChange={(e) => setForn(e.target.value)}
            />
          </div>
          <div className="flex flex-col gap-1">
            <Label className="text-[10px] uppercase text-muted-foreground">Nº Nota</Label>
            <Input
              type="text"
              placeholder="Número"
              className="w-28 text-xs h-8"
              value={numNota}
              onChange={(e) => setNumNota(e.target.value)}
            />
          </div>
          <div className="flex flex-col gap-1">
            <Label className="text-[10px] uppercase text-muted-foreground">Data de</Label>
            <Input type="date" className="w-36 text-xs h-8" value={dataIni} onChange={(e) => setDataIni(e.target.value)} />
          </div>
          <div className="flex flex-col gap-1">
            <Label className="text-[10px] uppercase text-muted-foreground">Data até</Label>
            <Input type="date" className="w-36 text-xs h-8" value={dataFim} onChange={(e) => setDataFim(e.target.value)} />
          </div>
          {(forn || numNota || dataIni || dataFim) && (
            <Button
              size="sm"
              variant="ghost"
              className="h-8 text-xs"
              onClick={() => { setForn(''); setNumNota(''); setDataIni(''); setDataFim('') }}
            >
              Limpar filtros
            </Button>
          )}
        </div>
      )}

      {!periodo && (
        <Alert>
          <AlertDescription className="text-xs">
            Informe o período no formato MM/AAAA para carregar os cálculos.
          </AlertDescription>
        </Alert>
      )}

      {periodo && (
        <>
          {/* Total geral */}
          {!spedQuery.isLoading && !xmlQuery.isLoading && (
            <div className="flex items-center justify-between px-1">
              <span className="text-xs text-muted-foreground">
                Total do mês ({rowsAtual.length + rowsXml.length} nota{rowsAtual.length + rowsXml.length !== 1 ? 's' : ''})
              </span>
              <span className="text-base font-bold tabular-nums">{fmtBRL(totalGeral)}</span>
            </div>
          )}

          {/* ── Bloco A: NFs mês anterior no SPED ── */}
          <div>
            <BlocoHeader
              open={openA}
              icon={<Clock className="h-4 w-4 text-amber-500" />}
              label="Bloco A — NFs de meses anteriores no SPED"
              count={rowsAnterior.length}
              total={totalAnterior}
              colorClass="bg-amber-50 border-amber-200 text-amber-900 hover:bg-amber-100"
              onClick={() => setOpenA(v => !v)}
            />
            {openA && (
              <div className="mt-2 space-y-2">
                <Alert className="border-amber-200 bg-amber-50">
                  <AlertTriangle className="h-4 w-4 text-amber-600" />
                  <AlertDescription className="text-xs text-amber-800">
                    Estas notas têm data de emissão em meses anteriores mas entraram no SPED deste período.
                    O imposto correspondente <strong>pode já ter sido recolhido</strong> no mês de emissão.
                    Verifique o comprovante de pagamento antes de incluir no cálculo.
                  </AlertDescription>
                </Alert>
                {spedQuery.isLoading ? (
                  <div className="space-y-2">{Array.from({ length: 3 }).map((_, i) => <Skeleton key={i} className="h-8 w-full" />)}</div>
                ) : rowsAnterior.length === 0 ? (
                  <p className="text-xs text-muted-foreground py-2 text-center">Nenhuma nota de mês anterior neste SPED.</p>
                ) : (
                  <TabelaNotasSped rows={rowsAnterior} showAliq={showAliq} />
                )}
              </div>
            )}
          </div>

          {/* ── Bloco B: NFs do mês no SPED ── */}
          <div>
            <BlocoHeader
              open={openB}
              icon={<CheckCircle2 className="h-4 w-4 text-green-600" />}
              label="Bloco B — NFs do mês presentes no SPED"
              count={rowsAtual.length}
              total={totalAtual}
              colorClass="bg-green-50 border-green-200 text-green-900 hover:bg-green-100"
              onClick={() => setOpenB(v => !v)}
            />
            {openB && (
              <div className="mt-2">
                {spedQuery.isLoading ? (
                  <div className="space-y-2">{Array.from({ length: 6 }).map((_, i) => <Skeleton key={i} className="h-8 w-full" />)}</div>
                ) : spedQuery.isError ? (
                  <Alert variant="destructive"><AlertDescription>Erro ao carregar notas do SPED.</AlertDescription></Alert>
                ) : rowsAtual.length === 0 ? (
                  <p className="text-xs text-muted-foreground py-2 text-center">Nenhuma nota do mês encontrada no SPED.</p>
                ) : (
                  <TabelaNotasSped rows={rowsAtual} showAliq={showAliq} />
                )}
              </div>
            )}
          </div>

          {/* ── Bloco C: NFs XML não lançadas no SPED ── */}
          <div>
            <BlocoHeader
              open={openC}
              icon={<FileQuestion className="h-4 w-4 text-slate-500" />}
              label="Bloco C — NFs do mês não localizadas no SPED (apenas XML)"
              count={rowsXml.length}
              total={totalXml}
              colorClass="bg-slate-50 border-slate-200 text-slate-800 hover:bg-slate-100"
              onClick={() => setOpenC(v => !v)}
            />
            {openC && (
              <div className="mt-2 space-y-2">
                <Alert className="border-slate-200 bg-slate-50">
                  <Info className="h-4 w-4 text-slate-500" />
                  <AlertDescription className="text-xs text-slate-700">
                    Notas emitidas neste mês encontradas no XML mas <strong>ausentes do SPED</strong>.
                    Podem ter sido recebidas no mês seguinte ou excluídas da escrituração.
                    A classificação de regime é automática pelo CFOP do fornecedor —
                    <strong> valide na aba Reconciliação antes de incluir no cálculo oficial</strong>.
                  </AlertDescription>
                </Alert>
                {xmlQuery.isLoading ? (
                  <div className="space-y-2">{Array.from({ length: 4 }).map((_, i) => <Skeleton key={i} className="h-8 w-full" />)}</div>
                ) : xmlQuery.isError ? (
                  <Alert variant="destructive"><AlertDescription>Erro ao carregar notas XML.</AlertDescription></Alert>
                ) : rowsXml.length === 0 ? (
                  <p className="text-xs text-muted-foreground py-2 text-center">Todas as notas XML do mês estão no SPED.</p>
                ) : (
                  <TabelaNotasXml rows={rowsXml} showAliq={showAliq} />
                )}
              </div>
            )}
          </div>
        </>
      )}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Fretes tab — CT-e vinculados às NFs de mercadoria
// ---------------------------------------------------------------------------

function FonteBadge({ fonte }: { fonte: string }) {
  const map: Record<string, string> = {
    'D162':    'bg-green-100 text-green-800 border-green-200',
    'XML-CTE': 'bg-blue-100 text-blue-800 border-blue-200',
    'CTE-REM': 'bg-amber-100 text-amber-800 border-amber-200',
  }
  const cls = map[fonte] ?? 'bg-gray-100 text-gray-600 border-gray-200'
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded text-[10px] font-medium border ${cls}`}>
      {fonte}
    </span>
  )
}

function FretesTab({ token }: { token: string | null }) {
  const [monthInput, setMonthInput] = useState('')
  const periodo = monthToPeriodo(monthInput)

  const { data, isLoading, isError } = useQuery<FretesResponse>({
    queryKey: ['icms-fronteira/fretes', periodo],
    queryFn: async () => {
      const url = periodo
        ? `/api/icms-fronteira/fretes?periodo=${encodeURIComponent(periodo)}`
        : '/api/icms-fronteira/fretes'
      const res = await fetch(url, { headers: { Authorization: `Bearer ${token}` } })
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      return res.json()
    },
  })

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2 flex-wrap justify-between">
        <div className="flex items-center gap-2">
          <Label htmlFor="fretes-periodo" className="text-xs whitespace-nowrap">Período:</Label>
          <Input
            id="fretes-periodo"
            type="text"
            placeholder="MM/AAAA"
            maxLength={7}
            className="w-36 text-xs h-8"
            value={monthInput}
            onChange={(e) => setMonthInput(e.target.value)}
          />
          {periodo && <span className="text-xs text-muted-foreground">{periodo}</span>}
        </div>
      </div>

      {isLoading && (
        <div className="space-y-2">
          {Array.from({ length: 6 }).map((_, i) => <Skeleton key={i} className="h-8 w-full" />)}
        </div>
      )}

      {isError && (
        <Alert variant="destructive">
          <AlertDescription>Erro ao carregar fretes.</AlertDescription>
        </Alert>
      )}

      {!isLoading && !isError && data && data.count === 0 && (
        <div className="flex flex-col items-center gap-3 py-10 text-center">
          <Truck className="h-8 w-8 text-muted-foreground/40" />
          <p className="text-sm text-muted-foreground">Nenhum CT-e vinculado encontrado para o período.</p>
          <p className="text-xs text-muted-foreground max-w-md">
            Os fretes são identificados via SPED (reg_d162), XML de CT-e importados ou
            correspondência documental D100. Importe os arquivos correspondentes para visualizar.
          </p>
        </div>
      )}

      {data && data.count > 0 && (
        <>
          <div className="flex items-center justify-between px-1">
            <span className="text-xs text-muted-foreground">
              {data.count} CT-e vinculado{data.count !== 1 ? 's' : ''}
            </span>
            <div className="text-right text-xs">
              <span className="text-muted-foreground">Frete total: </span>
              <span className="font-semibold tabular-nums">{fmtBRL(data.total_v_prest)}</span>
              <span className="text-muted-foreground ml-3">ICMS fronteira s/ frete: </span>
              <span className="font-bold tabular-nums text-foreground">{fmtBRL(data.total_icms_fronteira)}</span>
            </div>
          </div>

          <div className="rounded-md border overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow className="bg-muted/30 hover:bg-transparent">
                  <TableHead className="text-xs font-semibold uppercase tracking-wide">Data NF</TableHead>
                  <TableHead className="text-xs font-semibold uppercase tracking-wide">NF-e</TableHead>
                  <TableHead className="text-xs font-semibold uppercase tracking-wide">Fornecedor</TableHead>
                  <TableHead className="text-xs font-semibold uppercase tracking-wide">UF</TableHead>
                  <TableHead className="text-xs font-semibold uppercase tracking-wide">Regime</TableHead>
                  <TableHead className="text-xs font-semibold uppercase tracking-wide">CT-e</TableHead>
                  <TableHead className="text-xs font-semibold uppercase tracking-wide">Transportadora</TableHead>
                  <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">V. Frete</TableHead>
                  <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">ICMS CT-e</TableHead>
                  <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">ICMS Fronteira</TableHead>
                  <TableHead className="text-xs font-semibold uppercase tracking-wide">Tomador</TableHead>
                  <TableHead className="text-xs font-semibold uppercase tracking-wide">Fonte</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.rows.map((row, idx) => (
                  <TableRow key={`${row.chave_nfe}-${row.chave_cte}-${idx}`}>
                    <TableCell className="text-xs font-mono whitespace-nowrap">
                      {row.data_emissao ? row.data_emissao.slice(0, 10) : '—'}
                    </TableCell>
                    <TableCell className="text-xs font-mono">{row.numero_nfe || '—'}</TableCell>
                    <TableCell className="text-xs max-w-[160px]">
                      <div className="truncate" title={row.forn_nome}>{row.forn_nome || '—'}</div>
                      <div className="text-muted-foreground text-[10px] font-mono">{formatCNPJ(row.forn_cnpj)}</div>
                    </TableCell>
                    <TableCell className="text-xs font-mono font-semibold">{row.forn_uf || '—'}</TableCell>
                    <TableCell><RegimeBadge regime={row.regime} /></TableCell>
                    <TableCell className="text-xs font-mono">{row.numero_cte || '—'}</TableCell>
                    <TableCell className="text-xs max-w-[160px]">
                      <div className="truncate" title={row.emit_nome}>{row.emit_nome || '—'}</div>
                      <div className="text-muted-foreground text-[10px] font-mono">{formatCNPJ(row.emit_cnpj)}</div>
                    </TableCell>
                    <TableCell className="text-xs text-right tabular-nums">{fmtBRL(row.v_prest)}</TableCell>
                    <TableCell className="text-xs text-right tabular-nums">{fmtBRL(row.v_icms_cte)}</TableCell>
                    <TableCell className="text-xs text-right tabular-nums font-semibold">
                      {fmtBRL(row.icms_fronteira)}
                    </TableCell>
                    <TableCell><TomadorBadge toma={row.toma || ''} /></TableCell>
                    <TableCell><FonteBadge fonte={row.fonte} /></TableCell>
                  </TableRow>
                ))}
              </TableBody>
              {data.rows.length > 0 && (
                <TableFooter>
                  <TableRow className="bg-muted/60 hover:bg-muted/60">
                    <TableCell colSpan={7} className="text-xs font-bold uppercase">
                      Total — {data.count} CT-e{data.count !== 1 ? 's' : ''}
                    </TableCell>
                    <TableCell className="text-xs text-right tabular-nums font-bold">{fmtBRL(data.total_v_prest)}</TableCell>
                    <TableCell className="text-xs text-right tabular-nums font-bold">
                      {fmtBRL(data.rows.reduce((a, r) => a + (r.v_icms_cte || 0), 0))}
                    </TableCell>
                    <TableCell className="text-xs text-right tabular-nums font-bold">{fmtBRL(data.total_icms_fronteira)}</TableCell>
                    <TableCell colSpan={2} />
                  </TableRow>
                </TableFooter>
              )}
            </Table>
          </div>

          <Alert className="border-blue-200 bg-blue-50">
            <Info className="h-4 w-4 text-blue-600" />
            <AlertDescription className="text-xs text-blue-800">
              <strong>Tomador:</strong> são considerados <strong>apenas</strong> CT-es cujo tomador é o
              destinatário (= empresa do cliente). CT-es com frete por conta do remetente
              (FOB), de outros tomadores, ou sem o campo informado, são ignorados.
              <br />
              O ICMS fronteira sobre o frete usa o mesmo regime da NF de mercadoria correspondente.
              Fontes: <strong>D162</strong> = vínculo direto no SPED (mais confiável);{' '}
              <strong>XML-CTE</strong> = CT-e importado com referência à NF.
            </AlertDescription>
          </Alert>
        </>
      )}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Motor de Cálculo Fiscal — Fase 1 (Substituição Tributária BA)
// ---------------------------------------------------------------------------
interface MotorFiscalRow {
  id: string
  chave_nfe: string
  numero_nfe: string
  data_emissao: string
  n_item: number
  cfop: string
  ncm: string
  cst_icms: string
  dest_uf: string
  forn_uf: string
  v_item: number
  v_ipi: number
  v_frete_proporcional: number
  v_frete_cte_rateado: number
  v_outras_desp: number
  v_icms_item: number
  ncm_prefixo_aplicado: string
  mva_aplicada: number
  mva_tipo: string
  aliq_inter: number
  aliq_interna: number
  base_st: number
  icms_st_estimado: number
}

interface MotorFiscalResponse {
  rows: MotorFiscalRow[]
  count: number
  total_base_st: number
  total_icms_st: number
  periodo: string
  fase: string
  mensagem?: string
}

function MotorFiscalTab({ token }: { token: string | null }) {
  const [monthInput, setMonthInput] = useState('')
  const periodo = monthToPeriodo(monthInput)
  const [calcMsg, setCalcMsg] = useState<string | null>(null)
  const [calculating, setCalculating] = useState(false)
  const queryClient = useQueryClient()

  const { data, isLoading, isError } = useQuery<MotorFiscalResponse>({
    queryKey: ['motor-fiscal/resultados', periodo],
    queryFn: async () => {
      const url = periodo
        ? `/api/icms-fronteira/motor-fiscal/resultados?periodo=${encodeURIComponent(periodo)}`
        : '/api/icms-fronteira/motor-fiscal/resultados'
      const res = await fetch(url, { headers: { Authorization: `Bearer ${token}` } })
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      return res.json()
    },
  })

  async function rodarCalculo() {
    if (!periodo) { setCalcMsg('Informe o período MM/AAAA antes de calcular.'); return }
    setCalculating(true)
    setCalcMsg(null)
    try {
      const res = await fetch(`/api/icms-fronteira/motor-fiscal/calcular?periodo=${encodeURIComponent(periodo)}`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) {
        const txt = await res.text()
        throw new Error(`HTTP ${res.status}: ${txt.slice(0, 200)}`)
      }
      const json: MotorFiscalResponse = await res.json()
      setCalcMsg(json.mensagem ?? `${json.count} itens processados`)
      queryClient.invalidateQueries({ queryKey: ['motor-fiscal/resultados'] })
    } catch (err) {
      setCalcMsg('Falha: ' + (err instanceof Error ? err.message : String(err)))
    } finally {
      setCalculating(false)
    }
  }

  const rows = data?.rows ?? []
  const totalBase = data?.total_base_st ?? 0
  const totalIcms = data?.total_icms_st ?? 0

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2 flex-wrap justify-between">
        <div className="flex items-center gap-2">
          <Label htmlFor="motor-periodo" className="text-xs whitespace-nowrap">Período:</Label>
          <Input
            id="motor-periodo"
            type="text"
            placeholder="MM/AAAA"
            maxLength={7}
            className="w-36 text-xs h-8"
            value={monthInput}
            onChange={(e) => setMonthInput(e.target.value)}
          />
          {periodo && <span className="text-xs text-muted-foreground">{periodo}</span>}
        </div>
        <Button size="sm" onClick={rodarCalculo} disabled={!periodo || calculating}>
          <Play className="h-3 w-3 mr-1" />
          {calculating ? 'Calculando…' : 'Executar Cálculo'}
        </Button>
      </div>

      {calcMsg && (
        <Alert>
          <AlertDescription className="text-xs">{calcMsg}</AlertDescription>
        </Alert>
      )}

      {isLoading && (
        <div className="space-y-2">
          {Array.from({ length: 6 }).map((_, i) => <Skeleton key={i} className="h-8 w-full" />)}
        </div>
      )}

      {isError && (
        <Alert variant="destructive">
          <AlertDescription>Erro ao carregar resultados.</AlertDescription>
        </Alert>
      )}

      {!isLoading && !isError && rows.length === 0 && (
        <div className="flex flex-col items-center gap-3 py-10 text-center">
          <Calculator className="h-8 w-8 text-muted-foreground/40" />
          <p className="text-sm text-muted-foreground">Nenhum cálculo persistido para o período.</p>
          <p className="text-xs text-muted-foreground max-w-md">
            Informe o período e clique em <strong>Executar Cálculo</strong>. O motor processa
            itens com CFOP 2403 e UF de destino BA, aplicando a MVA da regra NCM correspondente.
          </p>
        </div>
      )}

      {rows.length > 0 && (
        <>
          <div className="flex items-center justify-between px-1">
            <span className="text-xs text-muted-foreground">
              {data!.count} item{data!.count !== 1 ? 's' : ''} calculado{data!.count !== 1 ? 's' : ''}
            </span>
            <div className="text-right text-xs">
              <span className="text-muted-foreground">Base ST total: </span>
              <span className="font-semibold tabular-nums">{fmtBRL(totalBase)}</span>
              <span className="text-muted-foreground ml-3">ICMS ST estimado: </span>
              <span className="font-bold tabular-nums text-foreground">{fmtBRL(totalIcms)}</span>
            </div>
          </div>

          <div className="rounded-md border overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow className="bg-muted/30 hover:bg-transparent">
                  <TableHead className="text-xs font-semibold uppercase">Data</TableHead>
                  <TableHead className="text-xs font-semibold uppercase">NF-e</TableHead>
                  <TableHead className="text-xs font-semibold uppercase">Item</TableHead>
                  <TableHead className="text-xs font-semibold uppercase">CFOP</TableHead>
                  <TableHead className="text-xs font-semibold uppercase">NCM</TableHead>
                  <TableHead className="text-xs font-semibold uppercase">UF</TableHead>
                  <TableHead className="text-xs font-semibold uppercase text-right">V. Item</TableHead>
                  <TableHead className="text-xs font-semibold uppercase text-right">IPI</TableHead>
                  <TableHead className="text-xs font-semibold uppercase text-right">Frete prop.</TableHead>
                  <TableHead className="text-xs font-semibold uppercase text-right">Frete CT-e</TableHead>
                  <TableHead className="text-xs font-semibold uppercase text-right">Outras</TableHead>
                  <TableHead className="text-xs font-semibold uppercase text-right">MVA%</TableHead>
                  <TableHead className="text-xs font-semibold uppercase text-right">Alíq.Int</TableHead>
                  <TableHead className="text-xs font-semibold uppercase text-right">Base ST</TableHead>
                  <TableHead className="text-xs font-semibold uppercase text-right">ICMS ST</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {rows.map((r) => (
                  <TableRow key={r.id}>
                    <TableCell className="text-xs font-mono whitespace-nowrap">
                      {r.data_emissao ? r.data_emissao.slice(0, 10) : '—'}
                    </TableCell>
                    <TableCell className="text-xs font-mono">{r.numero_nfe || '—'}</TableCell>
                    <TableCell className="text-xs">{r.n_item}</TableCell>
                    <TableCell className="text-xs font-mono">{r.cfop}</TableCell>
                    <TableCell className="text-xs font-mono">{r.ncm || '—'}</TableCell>
                    <TableCell className="text-xs font-mono font-semibold">{r.dest_uf}</TableCell>
                    <TableCell className="text-xs text-right tabular-nums">{fmtBRL(r.v_item)}</TableCell>
                    <TableCell className="text-xs text-right tabular-nums">{fmtBRL(r.v_ipi)}</TableCell>
                    <TableCell className="text-xs text-right tabular-nums">{fmtBRL(r.v_frete_proporcional)}</TableCell>
                    <TableCell className="text-xs text-right tabular-nums">{fmtBRL(r.v_frete_cte_rateado)}</TableCell>
                    <TableCell className="text-xs text-right tabular-nums">{fmtBRL(r.v_outras_desp)}</TableCell>
                    <TableCell className="text-xs text-right tabular-nums">
                      {(r.mva_aplicada || 0).toFixed(2)}
                      <span className="text-[9px] text-muted-foreground ml-1">{r.mva_tipo}</span>
                    </TableCell>
                    <TableCell className="text-xs text-right tabular-nums">{(r.aliq_interna || 0).toFixed(2)}%</TableCell>
                    <TableCell className="text-xs text-right tabular-nums font-semibold">{fmtBRL(r.base_st)}</TableCell>
                    <TableCell className="text-xs text-right tabular-nums font-bold">{fmtBRL(r.icms_st_estimado)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
              <TableFooter>
                <TableRow className="bg-muted/60 hover:bg-muted/60">
                  <TableCell colSpan={13} className="text-xs font-bold uppercase">
                    Total — {data!.count} ite{data!.count !== 1 ? 'ns' : 'm'}
                  </TableCell>
                  <TableCell className="text-xs text-right tabular-nums font-bold">{fmtBRL(totalBase)}</TableCell>
                  <TableCell className="text-xs text-right tabular-nums font-bold">{fmtBRL(totalIcms)}</TableCell>
                </TableRow>
              </TableFooter>
            </Table>
          </div>

          <Alert className="border-indigo-200 bg-indigo-50">
            <Info className="h-4 w-4 text-indigo-600" />
            <AlertDescription className="text-xs text-indigo-900">
              <strong>Fórmula aplicada:</strong> Base ST = (V.Item + IPI + Frete prop. + Frete CT-e + Outras Desp.) × (1 + MVA%).
              ICMS ST = Base ST × Alíq.Interna − ICMS destacado no item. Frete CT-e considera apenas
              CT-es onde o tomador é o destinatário. Cálculo persistido em
              <code className="text-[10px] bg-white px-1 rounded ml-1">fiscal_calculations</code>
              (fase F1_ST_BA) para auditoria.
            </AlertDescription>
          </Alert>
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
// Reconciliação SPED × XML — notas sobrando e faltando
// ---------------------------------------------------------------------------
interface ReconNota {
  chave_nfe: string
  data_emissao: string
  data_entrada: string
  numero_nfe: string
  forn_cnpj: string
  forn_nome: string
  forn_uf: string
  cfop: string
  cfop_entrada: string
  regime: string
  class_status?: string  // 'auto' | 'manual' | 'excluded'
  v_opr: number
  icms_devido_est: number
  origem: string
  alerta?: string
}

interface IASuggestion {
  regime_sugerido: string
  confianca: string
  justificativa: string
  contexto_usado?: Record<string, unknown>
  historico_fornecedor?: Array<{ regime: string; cfop: string; qtd: number }>
}
interface ReconBlock { rows: ReconNota[]; total: number; count: number }
interface ReconResponse {
  periodo: string
  normal: ReconBlock
  emitida_mes_anterior: ReconBlock
  nao_localizada_sped: ReconBlock
}

function ReconBlockTable({ block, showCfopMap }: { block: ReconBlock; showCfopMap?: boolean }) {
  if (!block.rows.length) {
    return <p className="text-xs text-muted-foreground py-4">Nenhuma nota neste bloco.</p>
  }
  return (
    <div className="rounded-md border overflow-x-auto">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="text-xs">Emissão</TableHead>
            <TableHead className="text-xs">NF</TableHead>
            <TableHead className="text-xs">Fornecedor</TableHead>
            <TableHead className="text-xs">UF</TableHead>
            {showCfopMap
              ? <TableHead className="text-xs">CFOP saída→entrada</TableHead>
              : <TableHead className="text-xs">CFOP</TableHead>}
            <TableHead className="text-xs">Regime</TableHead>
            <TableHead className="text-xs text-right">V. Operação</TableHead>
            <TableHead className="text-xs text-right">ICMS estimado</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {block.rows.map((r, i) => (
            <TableRow key={`${r.chave_nfe}-${i}`}>
              <TableCell className="text-xs">{r.data_emissao ? r.data_emissao.slice(0, 10) : '—'}</TableCell>
              <TableCell className="text-xs">{r.numero_nfe || '—'}</TableCell>
              <TableCell className="text-xs">{r.forn_nome || formatCNPJ(r.forn_cnpj)}</TableCell>
              <TableCell className="text-xs">{r.forn_uf || '—'}</TableCell>
              <TableCell className="text-xs tabular-nums">
                {showCfopMap ? `${r.cfop} → ${r.cfop_entrada}` : r.cfop}
              </TableCell>
              <TableCell className="text-xs"><RegimeBadge regime={r.regime} /></TableCell>
              <TableCell className="text-xs text-right tabular-nums">{fmtBRL(r.v_opr)}</TableCell>
              <TableCell className="text-xs text-right tabular-nums font-semibold">{fmtBRL(r.icms_devido_est)}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

// Tabela do bloco "Faltando" — com edição, validação e botão IA por linha.
function FaltandoBlockTable({
  block, token, periodo, queryClient,
}: {
  block: ReconBlock
  token: string | null
  periodo: string
  queryClient: ReturnType<typeof useQueryClient>
}) {
  const [iaModal, setIaModal] = useState<{ chave: string; sugestao: IASuggestion | null; loading: boolean } | null>(null)

  async function saveManual(chave: string, regime: string, status: 'manual' | 'excluded') {
    try {
      const res = await fetch('/api/icms-fronteira/reconciliacao/classificacao', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({ chave_nfe: chave, regime, status }),
      })
      if (!res.ok) throw new Error(await res.text())
      toast.success(status === 'excluded' ? 'Nota excluída do cálculo' : 'Classificação validada')
      queryClient.invalidateQueries({ queryKey: ['icms-fronteira/reconciliacao', periodo] })
    } catch (e) {
      toast.error('Falha ao salvar: ' + (e instanceof Error ? e.message : ''))
    }
  }

  async function resetManual(chave: string) {
    try {
      const res = await fetch(`/api/icms-fronteira/reconciliacao/classificacao?chave=${encodeURIComponent(chave)}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) throw new Error(await res.text())
      toast.success('Voltou à classificação automática')
      queryClient.invalidateQueries({ queryKey: ['icms-fronteira/reconciliacao', periodo] })
    } catch (e) {
      toast.error('Falha: ' + (e instanceof Error ? e.message : ''))
    }
  }

  async function sugerirIA(chave: string) {
    setIaModal({ chave, sugestao: null, loading: true })
    try {
      const res = await fetch('/api/icms-fronteira/reconciliacao/sugerir-ia', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({ chave_nfe: chave }),
      })
      if (!res.ok) throw new Error((await res.json()).error || 'IA indisponível')
      const d = await res.json()
      setIaModal({ chave, sugestao: d, loading: false })
    } catch (e) {
      toast.error('IA: ' + (e instanceof Error ? e.message : ''))
      setIaModal(null)
    }
  }

  if (!block.rows.length) {
    return <p className="text-xs text-muted-foreground py-4">Nenhuma nota neste bloco.</p>
  }

  return (
    <>
      <div className="rounded-md border overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="text-xs">Emissão</TableHead>
              <TableHead className="text-xs">NF</TableHead>
              <TableHead className="text-xs">Fornecedor</TableHead>
              <TableHead className="text-xs">UF</TableHead>
              <TableHead className="text-xs">CFOP saída→entrada</TableHead>
              <TableHead className="text-xs">Regime (editável)</TableHead>
              <TableHead className="text-xs">Status</TableHead>
              <TableHead className="text-xs text-right">V. Operação</TableHead>
              <TableHead className="text-xs text-right">ICMS estimado</TableHead>
              <TableHead className="text-xs text-right">Ações</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {block.rows.map((r, i) => (
              <TableRow key={`${r.chave_nfe}-${i}`}>
                <TableCell className="text-xs">{r.data_emissao ? r.data_emissao.slice(0, 10) : '—'}</TableCell>
                <TableCell className="text-xs">{r.numero_nfe || '—'}</TableCell>
                <TableCell className="text-xs">{r.forn_nome || formatCNPJ(r.forn_cnpj)}</TableCell>
                <TableCell className="text-xs">{r.forn_uf || '—'}</TableCell>
                <TableCell className="text-xs tabular-nums">{r.cfop} → {r.cfop_entrada}</TableCell>
                <TableCell className="text-xs">
                  <Select
                    value={r.regime}
                    onValueChange={v => saveManual(r.chave_nfe, v, 'manual')}
                  >
                    <SelectTrigger className="h-7 text-xs w-32"><SelectValue /></SelectTrigger>
                    <SelectContent>
                      <SelectItem value="ANTECIPACAO">Antecipação</SelectItem>
                      <SelectItem value="ST">ST</SelectItem>
                      <SelectItem value="DIFAL">DIFAL</SelectItem>
                      <SelectItem value="NAO_FRONTEIRA">Não Fronteira</SelectItem>
                    </SelectContent>
                  </Select>
                </TableCell>
                <TableCell className="text-xs">
                  {r.class_status === 'manual'
                    ? <Badge variant="outline" className="bg-emerald-50 text-emerald-700 border-emerald-200 text-[10px]">manual</Badge>
                    : <Badge variant="outline" className="bg-slate-50 text-slate-600 border-slate-200 text-[10px]">auto</Badge>}
                </TableCell>
                <TableCell className="text-xs text-right tabular-nums">{fmtBRL(r.v_opr)}</TableCell>
                <TableCell className="text-xs text-right tabular-nums font-semibold">{fmtBRL(r.icms_devido_est)}</TableCell>
                <TableCell className="text-xs text-right whitespace-nowrap">
                  <Button size="sm" variant="ghost" className="h-6 px-2 text-[11px]"
                    title="Sugerir com IA" onClick={() => sugerirIA(r.chave_nfe)}>
                    <Sparkles className="h-3 w-3 mr-1" />IA
                  </Button>
                  <Button size="sm" variant="ghost" className="h-6 px-2 text-[11px]"
                    title="Validar (mantém o regime atual)" onClick={() => saveManual(r.chave_nfe, r.regime, 'manual')}>
                    ✓
                  </Button>
                  <Button size="sm" variant="ghost" className="h-6 px-2 text-[11px] text-red-600"
                    title="Excluir do cálculo" onClick={() => saveManual(r.chave_nfe, r.regime, 'excluded')}>
                    ×
                  </Button>
                  {r.class_status === 'manual' && (
                    <Button size="sm" variant="ghost" className="h-6 px-2 text-[11px] text-slate-500"
                      title="Voltar à classificação automática" onClick={() => resetManual(r.chave_nfe)}>
                      ↺
                    </Button>
                  )}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      {/* Modal de sugestão IA */}
      <Dialog open={!!iaModal} onOpenChange={o => !o && setIaModal(null)}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>Sugestão da IA</DialogTitle>
            <DialogDescription className="text-xs">
              Chave NF: ...{iaModal?.chave.slice(-12)}
            </DialogDescription>
          </DialogHeader>
          {iaModal?.loading && <p className="text-sm py-6 text-center text-muted-foreground">Consultando IA...</p>}
          {iaModal?.sugestao && (
            <div className="space-y-3 text-sm">
              <div className="flex items-center gap-2">
                <span className="text-xs text-muted-foreground">Regime sugerido:</span>
                <RegimeBadge regime={iaModal.sugestao.regime_sugerido} />
                <Badge variant="outline" className="text-[10px]">conf: {iaModal.sugestao.confianca}</Badge>
              </div>
              <div className="rounded-md border bg-muted/30 p-3 text-xs">{iaModal.sugestao.justificativa}</div>
              {iaModal.sugestao.historico_fornecedor && iaModal.sugestao.historico_fornecedor.length > 0 && (
                <div className="text-xs">
                  <span className="text-muted-foreground">Histórico: </span>
                  {iaModal.sugestao.historico_fornecedor.map(h => `${h.qtd}× ${h.regime}`).join(', ')}
                </div>
              )}
            </div>
          )}
          <DialogFooter className="gap-2">
            <Button variant="outline" size="sm" onClick={() => setIaModal(null)}>Descartar</Button>
            <Button
              size="sm"
              disabled={!iaModal?.sugestao}
              onClick={async () => {
                if (!iaModal?.sugestao) return
                await saveManual(iaModal.chave, iaModal.sugestao.regime_sugerido, 'manual')
                setIaModal(null)
              }}
            >Aplicar sugestão</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}

// ---------------------------------------------------------------------------
// Aba Legislação — upload de decreto + interpretação IA + aplicar regras
// ---------------------------------------------------------------------------
interface LegislacaoRegraItem {
  ncm: string
  regime: string
  descricao?: string
  justificativa?: string
  aliquota_interna?: number
  mva_original?: number
  mva_4pct?: number
  mva_7pct?: number
  mva_12pct?: number
  confirmado: boolean
}

interface LegislacaoInterp {
  resumo: string
  regras: LegislacaoRegraItem[]
}

interface LegislacaoListRow {
  id: string
  uf_estado: string
  titulo: string
  status: string
  created_at: string
  applied_at?: string | null
  proc_status?: string          // 'processing' | 'done' | 'error'
  proc_done_chunks?: number
  proc_total_chunks?: number
  proc_error?: string | null
}

interface LegislacaoDetail extends LegislacaoListRow {
  conteudo_texto: string
  interpretacao: LegislacaoInterp
}

function LegislacaoTab({ token }: { token: string | null }) {
  const queryClient = useQueryClient()
  const [openId, setOpenId] = useState<string | null>(null)
  const [uploadOpen, setUploadOpen] = useState(false)
  const [uf, setUf] = useState('BA')
  const [titulo, setTitulo] = useState('')
  const [texto, setTexto] = useState('')
  const [uploadFile, setUploadFile] = useState<File | null>(null)
  const [uploadMode, setUploadMode] = useState<'file' | 'text'>('file')
  const [busy, setBusy] = useState(false)
  const [detail, setDetail] = useState<LegislacaoDetail | null>(null)
  const [editedRegras, setEditedRegras] = useState<LegislacaoRegraItem[]>([])

  const { data: lista, isLoading } = useQuery<LegislacaoListRow[]>({
    queryKey: ['icms-fronteira/legislacao'],
    queryFn: async () => {
      const res = await fetch('/api/icms-fronteira/legislacao', {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) throw new Error('falha ao listar')
      return res.json()
    },
    enabled: !!token,
    // Enquanto algum decreto estiver sendo processado pela IA em background,
    // refaz a consulta a cada 4s para atualizar o progresso (N/M chunks).
    refetchInterval: (query) => {
      const rows = query.state.data as LegislacaoListRow[] | undefined
      return rows?.some(r => r.proc_status === 'processing') ? 4000 : false
    },
  })

  async function uploadDecreto() {
    if (!titulo.trim()) {
      toast.error('Informe o título do decreto')
      return
    }
    setBusy(true)
    try {
      let res: Response
      if (uploadMode === 'file' && uploadFile) {
        const fd = new FormData()
        fd.append('titulo', titulo)
        fd.append('uf_estado', uf)
        fd.append('file', uploadFile)
        res = await fetch(`/api/icms-fronteira/legislacao/upload?uf_estado=${uf}`, {
          method: 'POST',
          headers: { Authorization: `Bearer ${token}` },
          body: fd,
        })
      } else {
        if (texto.trim().length < 50) {
          toast.error('Texto muito curto — cole o conteúdo do decreto')
          setBusy(false)
          return
        }
        res = await fetch(`/api/icms-fronteira/legislacao/upload?uf_estado=${uf}`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
          body: JSON.stringify({ titulo, conteudo_texto: texto }),
        })
      }
      if (!res.ok) throw new Error((await res.json()).error || 'falha')
      const r = await res.json()
      const nChunks = r.total_chunks ?? 1
      toast.success(
        `Decreto enviado. A IA está extraindo as regras em background` +
        (nChunks > 1 ? ` (${nChunks} partes)` : '') +
        `. O progresso aparece na lista.`
      )
      setUploadOpen(false)
      setTitulo(''); setTexto(''); setUploadFile(null)
      queryClient.invalidateQueries({ queryKey: ['icms-fronteira/legislacao'] })
    } catch (e) {
      toast.error('Upload falhou: ' + (e instanceof Error ? e.message : ''))
    } finally {
      setBusy(false)
    }
  }

  async function loadDetail(id: string) {
    setOpenId(id)
    setDetail(null)
    try {
      const res = await fetch(`/api/icms-fronteira/legislacao?id=${encodeURIComponent(id)}`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) throw new Error('falha')
      const d: LegislacaoDetail = await res.json()
      setDetail(d)
      setEditedRegras(d.interpretacao?.regras ?? [])
    } catch (e) {
      toast.error('Falha ao abrir: ' + (e instanceof Error ? e.message : ''))
      setOpenId(null)
    }
  }

  async function salvarRevisao() {
    if (!detail) return
    setBusy(true)
    try {
      const res = await fetch(`/api/icms-fronteira/legislacao?id=${encodeURIComponent(detail.id)}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({
          interpretacao: { resumo: detail.interpretacao.resumo, regras: editedRegras },
          status: 'reviewed',
        }),
      })
      if (!res.ok) throw new Error('falha')
      toast.success('Revisão salva')
      queryClient.invalidateQueries({ queryKey: ['icms-fronteira/legislacao'] })
    } catch (e) {
      toast.error('Falha: ' + (e instanceof Error ? e.message : ''))
    } finally {
      setBusy(false)
    }
  }

  async function aplicarRegras() {
    if (!detail) return
    const confirmadas = editedRegras.filter(r => r.confirmado).length
    if (confirmadas === 0) {
      toast.error('Confirme pelo menos uma regra antes de aplicar')
      return
    }
    if (!confirm(`Aplicar ${confirmadas} regra(s) confirmada(s) nas Regras NCM da empresa?`)) return
    setBusy(true)
    try {
      // 1) salva edição
      await fetch(`/api/icms-fronteira/legislacao?id=${encodeURIComponent(detail.id)}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({
          interpretacao: { resumo: detail.interpretacao.resumo, regras: editedRegras },
          status: 'reviewed',
        }),
      })
      // 2) aplica
      const res = await fetch(`/api/icms-fronteira/legislacao/aplicar?id=${encodeURIComponent(detail.id)}`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` },
      })
      const r = await res.json()
      if (!res.ok) throw new Error(r.error || 'falha')
      toast.success(`Aplicadas ${r.applied} regra(s); ${r.skipped} ignoradas`)
      setOpenId(null)
      queryClient.invalidateQueries({ queryKey: ['icms-fronteira/legislacao'] })
    } catch (e) {
      toast.error('Falha: ' + (e instanceof Error ? e.message : ''))
    } finally {
      setBusy(false)
    }
  }

  async function descartar(id: string) {
    if (!confirm('Descartar esta legislação?')) return
    try {
      await fetch(`/api/icms-fronteira/legislacao?id=${encodeURIComponent(id)}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${token}` },
      })
      toast.success('Removida')
      queryClient.invalidateQueries({ queryKey: ['icms-fronteira/legislacao'] })
    } catch {
      toast.error('Falha ao remover')
    }
  }

  function setRegra(i: number, patch: Partial<LegislacaoRegraItem>) {
    setEditedRegras(prev => prev.map((r, idx) => idx === i ? { ...r, ...patch } : r))
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base font-semibold flex items-center gap-2">
          <Sparkles className="h-4 w-4 text-indigo-600" />
          Legislação — Importação com IA
        </CardTitle>
      </CardHeader>
      <CardContent>
        <p className="text-xs text-muted-foreground mb-4">
          Faça upload do PDF ou TXT do decreto/RICMS. A IA extrai automaticamente os NCMs sujeitos a ST,
          alíquotas e MVAs. Você revisa item a item, confirma o que está correto e aplica nas Regras NCM.
        </p>
        <div className="flex justify-end mb-3">
          <Button size="sm" onClick={() => setUploadOpen(true)}>
            <Upload className="h-3.5 w-3.5 mr-1.5" /> Importar legislação
          </Button>
        </div>

        {isLoading && <p className="text-sm text-muted-foreground py-6">Carregando...</p>}
        {!isLoading && (lista?.length ?? 0) === 0 && (
          <p className="text-sm text-muted-foreground py-6 text-center">
            Nenhuma legislação importada ainda.
          </p>
        )}
        {!isLoading && (lista?.length ?? 0) > 0 && (
          <div className="rounded-md border overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="text-xs">Título</TableHead>
                  <TableHead className="text-xs">UF</TableHead>
                  <TableHead className="text-xs">Status</TableHead>
                  <TableHead className="text-xs">Criado</TableHead>
                  <TableHead className="text-xs text-right">Ações</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {lista!.map(r => {
                  const processing = r.proc_status === 'processing'
                  const procError = r.proc_status === 'error'
                  return (
                  <TableRow key={r.id}>
                    <TableCell className="text-xs">{r.titulo}</TableCell>
                    <TableCell className="text-xs">{r.uf_estado}</TableCell>
                    <TableCell className="text-xs">
                      {processing ? (
                        <Badge variant="outline" className="text-[10px] gap-1 border-blue-300 text-blue-700">
                          <Loader2 className="h-3 w-3 animate-spin" />
                          processando{(r.proc_total_chunks ?? 0) > 1 ? ` ${r.proc_done_chunks ?? 0}/${r.proc_total_chunks}` : ''}
                        </Badge>
                      ) : procError ? (
                        <Badge variant="outline" className="text-[10px] border-red-300 text-red-700" title={r.proc_error ?? ''}>
                          erro
                        </Badge>
                      ) : (
                        <Badge variant="outline" className="text-[10px]">{r.status}</Badge>
                      )}
                    </TableCell>
                    <TableCell className="text-xs">{r.created_at?.slice(0, 10)}</TableCell>
                    <TableCell className="text-right">
                      <Button
                        size="sm"
                        variant="ghost"
                        className="h-6 px-2 text-[11px]"
                        disabled={processing}
                        title={processing ? 'Aguarde a IA terminar de extrair as regras' : undefined}
                        onClick={() => loadDetail(r.id)}
                      >
                        Revisar
                      </Button>
                      <Button size="sm" variant="ghost" className="h-6 px-2 text-[11px] text-red-600" onClick={() => descartar(r.id)}>
                        <Trash2 className="h-3 w-3" />
                      </Button>
                    </TableCell>
                  </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          </div>
        )}

        {/* Modal upload */}
        <Dialog open={uploadOpen} onOpenChange={o => { setUploadOpen(o); if (!o) { setTitulo(''); setTexto(''); setUploadFile(null); setUploadMode('file') } }}>
          <DialogContent className="max-w-2xl">
            <DialogHeader>
              <DialogTitle>Importar legislação</DialogTitle>
              <DialogDescription className="text-xs">
                Faça upload do arquivo PDF ou TXT do decreto. A IA vai extrair os NCMs sujeitos a ST automaticamente.
              </DialogDescription>
            </DialogHeader>

            {/* UF + Título */}
            <div className="grid grid-cols-4 gap-3">
              <div className="col-span-1">
                <Label className="text-xs">UF</Label>
                <Select value={uf} onValueChange={setUf}>
                  <SelectTrigger className="text-xs"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="PE">PE</SelectItem>
                    <SelectItem value="BA">BA</SelectItem>
                    <SelectItem value="CE">CE</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="col-span-3">
                <Label className="text-xs">Título</Label>
                <Input className="text-xs" value={titulo} onChange={e => setTitulo(e.target.value)} placeholder="Ex: Decreto BA 13.870/2012 — Lista ST" />
              </div>
            </div>

            {/* Tabs file / text */}
            <div className="flex gap-1 border-b pb-2">
              <button
                type="button"
                onClick={() => setUploadMode('file')}
                className={`px-3 py-1 text-xs rounded-t ${uploadMode === 'file' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:text-foreground'}`}
              >
                Upload de arquivo
              </button>
              <button
                type="button"
                onClick={() => setUploadMode('text')}
                className={`px-3 py-1 text-xs rounded-t ${uploadMode === 'text' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:text-foreground'}`}
              >
                Colar texto
              </button>
            </div>

            {uploadMode === 'file' ? (
              <div className="space-y-2">
                <Label className="text-xs">Arquivo (PDF ou TXT)</Label>
                <Input
                  type="file"
                  accept=".pdf,.txt,.text"
                  className="text-xs"
                  onChange={e => {
                    const f = e.target.files?.[0] ?? null
                    setUploadFile(f)
                    if (f && !titulo.trim()) {
                      setTitulo(f.name.replace(/\.[^.]+$/, ''))
                    }
                  }}
                />
                {uploadFile && (
                  <p className="text-[10px] text-muted-foreground">
                    {uploadFile.name} — {(uploadFile.size / 1024).toFixed(0)} KB
                  </p>
                )}
                <Alert className="py-2">
                  <Info className="h-3 w-3" />
                  <AlertDescription className="text-[11px]">
                    PDFs com texto selecionável são extraídos automaticamente. PDFs escaneados (imagem) não funcionam — use a aba "Colar texto" nesses casos.
                  </AlertDescription>
                </Alert>
              </div>
            ) : (
              <div>
                <Label className="text-xs">Texto do decreto</Label>
                <Textarea className="text-xs h-56 font-mono" value={texto} onChange={e => setTexto(e.target.value)} placeholder="Cole o conteúdo aqui (ou o trecho que contém a lista de NCMs sujeitos a ST)..." />
                <p className="text-[10px] text-muted-foreground mt-1">Limite ~200k caracteres.</p>
              </div>
            )}

            <DialogFooter>
              <Button variant="outline" size="sm" onClick={() => setUploadOpen(false)}>Cancelar</Button>
              <Button
                size="sm"
                disabled={busy || (uploadMode === 'file' ? !uploadFile : texto.trim().length < 50)}
                onClick={uploadDecreto}
              >
                {busy ? 'Processando...' : 'Importar e interpretar'}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        {/* Modal revisão */}
        <Dialog open={!!openId} onOpenChange={o => { if (!o) { setOpenId(null); setDetail(null) } }}>
          <DialogContent className="max-w-5xl">
            <DialogHeader>
              <DialogTitle>{detail?.titulo ?? 'Carregando...'}</DialogTitle>
              <DialogDescription className="text-xs">
                Revise cada regra extraída pela IA. Confirme as corretas e clique em "Aplicar".
              </DialogDescription>
            </DialogHeader>
            {!detail && <p className="text-sm py-6 text-center text-muted-foreground">Carregando...</p>}
            {detail && (
              <div className="space-y-4">
                <Alert>
                  <Info className="h-4 w-4" />
                  <AlertDescription className="text-xs">
                    <strong>Resumo da IA:</strong> {detail.interpretacao?.resumo || '—'}
                  </AlertDescription>
                </Alert>
                <div className="rounded-md border overflow-x-auto max-h-96">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead className="text-xs w-10">Confirmar</TableHead>
                        <TableHead className="text-xs">NCM</TableHead>
                        <TableHead className="text-xs">Descrição</TableHead>
                        <TableHead className="text-xs">Regime</TableHead>
                        <TableHead className="text-xs text-right">Alíq Int</TableHead>
                        <TableHead className="text-xs text-right">MVA orig</TableHead>
                        <TableHead className="text-xs text-right">MVA 4%</TableHead>
                        <TableHead className="text-xs text-right">MVA 7%</TableHead>
                        <TableHead className="text-xs text-right">MVA 12%</TableHead>
                        <TableHead className="text-xs">Justificativa</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {editedRegras.map((rg, i) => (
                        <TableRow key={i}>
                          <TableCell className="text-xs">
                            <Checkbox checked={rg.confirmado} onCheckedChange={v => setRegra(i, { confirmado: !!v })} />
                          </TableCell>
                          <TableCell className="text-xs">
                            <Input className="text-xs h-7 w-20" value={rg.ncm} onChange={e => setRegra(i, { ncm: e.target.value })} />
                          </TableCell>
                          <TableCell className="text-xs">
                            <Input className="text-xs h-7" value={rg.descricao ?? ''} onChange={e => setRegra(i, { descricao: e.target.value })} />
                          </TableCell>
                          <TableCell className="text-xs">
                            <Select value={rg.regime} onValueChange={v => setRegra(i, { regime: v })}>
                              <SelectTrigger className="h-7 text-xs w-32"><SelectValue /></SelectTrigger>
                              <SelectContent>
                                <SelectItem value="ANTECIPACAO">Antecipação</SelectItem>
                                <SelectItem value="ST">ST</SelectItem>
                                <SelectItem value="DIFAL">DIFAL</SelectItem>
                                <SelectItem value="NORMAL">Normal</SelectItem>
                              </SelectContent>
                            </Select>
                          </TableCell>
                          {(['aliquota_interna','mva_original','mva_4pct','mva_7pct','mva_12pct'] as const).map(k => (
                            <TableCell key={k} className="text-xs text-right">
                              <Input className="text-xs h-7 w-20 text-right" type="number" step="0.01"
                                value={rg[k] ?? ''}
                                onChange={e => setRegra(i, { [k]: e.target.value === '' ? undefined : Number(e.target.value) } as Partial<LegislacaoRegraItem>)} />
                            </TableCell>
                          ))}
                          <TableCell className="text-xs">
                            <span className="text-[10px] text-muted-foreground">{rg.justificativa}</span>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              </div>
            )}
            <DialogFooter className="gap-2">
              <Button variant="outline" size="sm" onClick={() => { setOpenId(null); setDetail(null) }}>Fechar</Button>
              <Button variant="outline" size="sm" disabled={busy || !detail} onClick={salvarRevisao}>Salvar revisão</Button>
              <Button size="sm" disabled={busy || !detail} onClick={aplicarRegras}>
                Aplicar nas Regras NCM
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </CardContent>
    </Card>
  )
}

function ReconciliacaoTab({ token }: { token: string | null }) {
  const [monthInput, setMonthInput] = useState('')
  const periodo = monthToPeriodo(monthInput)
  const queryClient = useQueryClient()

  const { data, isLoading, error } = useQuery<ReconResponse>({
    queryKey: ['icms-fronteira/reconciliacao', periodo],
    queryFn: async () => {
      const res = await fetch(`/api/icms-fronteira/reconciliacao?periodo=${encodeURIComponent(periodo)}`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) throw new Error('Erro ao carregar reconciliação')
      return res.json()
    },
    enabled: !!token && !!periodo,
  })

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base font-semibold flex items-center gap-2">
          <ArrowLeftRight className="h-4 w-4 text-teal-600" />
          Reconciliação SPED × XML (notas sobrando e faltando)
        </CardTitle>
      </CardHeader>
      <CardContent>
        <p className="text-xs text-muted-foreground mb-4">
          O ICMS antecipado é devido pela <strong>data de emissão</strong>, mas o SPED registra por
          recebimento. Informe o <strong>mês de análise (MM/YYYY)</strong> para segregar as notas em
          três blocos.
        </p>
        <div className="flex items-center gap-2 mb-4">
          <Label htmlFor="recon-periodo" className="text-xs whitespace-nowrap">Mês de análise:</Label>
          <Input
            id="recon-periodo"
            type="text"
            placeholder="MM/AAAA"
            maxLength={7}
            value={monthInput}
            onChange={e => setMonthInput(e.target.value)}
            className="w-40 h-8 text-xs"
          />
          {periodo && <span className="text-xs text-muted-foreground">{periodo}</span>}
        </div>

        {!periodo && (
          <p className="text-sm text-muted-foreground py-6 text-center">
            Selecione o mês de análise para gerar a reconciliação.
          </p>
        )}
        {periodo && isLoading && <p className="text-sm text-muted-foreground py-6">Carregando...</p>}
        {periodo && error && (
          <Alert variant="destructive"><AlertDescription>Erro ao carregar reconciliação.</AlertDescription></Alert>
        )}

        {periodo && data && (
          <div className="space-y-6">
            {/* Bloco 1 — Normal */}
            <div>
              <div className="flex items-center justify-between mb-2">
                <h3 className="text-sm font-semibold flex items-center gap-2">
                  <Badge variant="outline" className="bg-green-50 text-green-700 border-green-200">Normal</Badge>
                  Emitidas no mês e presentes no SPED ({data.normal.count})
                </h3>
                <span className="text-xs font-semibold">ICMS: {fmtBRL(data.normal.total)}</span>
              </div>
              <ReconBlockTable block={data.normal} />
            </div>

            {/* Bloco 2 — Emitida mês anterior */}
            <div>
              <div className="flex items-center justify-between mb-2">
                <h3 className="text-sm font-semibold flex items-center gap-2">
                  <Badge variant="outline" className="bg-amber-50 text-amber-700 border-amber-200">Sobrando</Badge>
                  Emitidas em mês anterior — verificar recolhimento ({data.emitida_mes_anterior.count})
                </h3>
                <span className="text-xs font-semibold">ICMS: {fmtBRL(data.emitida_mes_anterior.total)}</span>
              </div>
              <Alert className="mb-2">
                <AlertTriangle className="h-4 w-4" />
                <AlertDescription className="text-xs">
                  Estas notas entraram no SPED deste mês mas foram emitidas antes. O ICMS antecipado
                  provavelmente já foi recolhido no mês de emissão — <strong>verificar</strong> antes de recolher novamente.
                </AlertDescription>
              </Alert>
              <ReconBlockTable block={data.emitida_mes_anterior} />
            </div>

            {/* Bloco 3 — Não localizada no SPED */}
            <div>
              <div className="flex items-center justify-between mb-2">
                <h3 className="text-sm font-semibold flex items-center gap-2">
                  <Badge variant="outline" className="bg-blue-50 text-blue-700 border-blue-200">Faltando</Badge>
                  Não localizadas no SPED — do XML ({data.nao_localizada_sped.count})
                </h3>
                <span className="text-xs font-semibold">ICMS: {fmtBRL(data.nao_localizada_sped.total)}</span>
              </div>
              <Alert className="mb-2">
                <Info className="h-4 w-4" />
                <AlertDescription className="text-xs">
                  Notas emitidas no mês, presentes nos XMLs mas ausentes do SPED. Classificadas
                  automaticamente pelo CFOP de saída do fornecedor (6xxx→2xxx). ICMS é
                  <strong> estimado</strong> — validar a classificação com o contador antes de incluir no cálculo oficial.
                </AlertDescription>
              </Alert>
              <FaltandoBlockTable
                block={data.nao_localizada_sped}
                token={token}
                periodo={periodo}
                queryClient={queryClient}
              />
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

// ---------------------------------------------------------------------------
// Regras NCM tab
// ---------------------------------------------------------------------------
export function RegrasTab({ token }: { token: string | null }) {
  const queryClient = useQueryClient()
  const [search, setSearch] = useState('')
  const [openCreate, setOpenCreate] = useState(false)
  const [importFile, setImportFile] = useState<File | null>(null)
  const [importLoading, setImportLoading] = useState(false)
  const [selectedUF, setSelectedUF] = useState<'PE' | 'BA' | 'CE'>('PE')
  // Segmento × UF: ao trocar a UF, a lista de segmentos e a seleção são resetadas.
  const [importSegmento, setImportSegmento] = useState<string>('')
  const [createSegmento, setCreateSegmento] = useState<string>('')
  // Resultado da última importação. Aberto em Dialog enquanto não-nulo, para
  // que o usuário possa ler o detalhe dos erros sem perder pela transição de toast.
  const [importResult, setImportResult] = useState<{ imported: number; skipped: number; errors: string[] } | null>(null)

  // Form state
  const [ncmPrefixo, setNcmPrefixo] = useState('')
  const [descricao, setDescricao] = useState('')
  const [regimenForm, setRegimenForm] = useState('ANTECIPACAO')
  const [aliqInterna, setAliqInterna] = useState('20.5')
  const [mva, setMva] = useState('')
  const [reducaoBC, setReducaoBC] = useState('0')

  const { data, isLoading, isError } = useQuery<RegrasResponse>({
    queryKey: ['icms-fronteira/regras', selectedUF],
    queryFn: async () => {
      const res = await fetch(`/api/icms-fronteira/regras?uf_estado=${selectedUF}`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      return res.json()
    },
  })

  // Segmentos disponíveis para a UF selecionada (catálogo segmentos_uf).
  const { data: segmentosData } = useQuery<{ segmentos: SegmentoUFOption[] }>({
    queryKey: ['icms-fronteira/segmentos', selectedUF],
    queryFn: async () => {
      const res = await fetch(`/api/icms-fronteira/segmentos?uf=${selectedUF}`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      return res.json()
    },
    enabled: !!token,
  })
  const segmentos = segmentosData?.segmentos ?? []
  const segmentoLabel = (codigo: number | null): string => {
    if (codigo == null) return '—'
    const s = segmentos.find(x => x.codigo === codigo)
    return s ? `${String(s.codigo).padStart(2, '0')} — ${s.descricao}` : String(codigo)
  }

  // Reset das seleções de segmento ao trocar de UF (segmento é vinculado à UF).
  useEffect(() => { setImportSegmento(''); setCreateSegmento('') }, [selectedUF])

  const createMutation = useMutation({
    mutationFn: async (body: object) => {
      const res = await fetch('/api/icms-fronteira/regras', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ ...body, uf_estado: selectedUF }),
      })
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      return res.json()
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['icms-fronteira/regras', selectedUF] })
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
      queryClient.invalidateQueries({ queryKey: ['icms-fronteira/regras', selectedUF] })
      toast.success('Regra removida')
    },
    onError: () => toast.error('Erro ao remover regra'),
  })

  // Edição de regra existente (ajuste manual dos dados importados).
  const [editing, setEditing] = useState<RegraNCM | null>(null)
  const updateMutation = useMutation({
    mutationFn: async (r: RegraNCM) => {
      const res = await fetch(`/api/icms-fronteira/regras/${r.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({
          descricao: r.descricao,
          regime: r.regime,
          aliquota_interna: r.aliquota_interna,
          mva_original: r.mva_original,
          mva_ajustado_4pct: r.mva_ajustado_4pct,
          mva_ajustado_7pct: r.mva_ajustado_7pct,
          mva_ajustado_12pct: r.mva_ajustado_12pct,
          reducao_bc_pct: r.reducao_bc_pct,
          segmento_codigo: r.segmento_codigo,
        }),
      })
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      return res.json()
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['icms-fronteira/regras', selectedUF] })
      toast.success('Regra atualizada')
      setEditing(null)
    },
    onError: () => toast.error('Erro ao atualizar regra'),
  })

  function resetForm() {
    setNcmPrefixo('')
    setDescricao('')
    setRegimenForm('ANTECIPACAO')
    setAliqInterna('20.5')
    setMva('')
    setReducaoBC('0')
    setCreateSegmento('')
  }

  function handleCreate() {
    if (!createSegmento) { toast.error('Selecione o segmento da regra'); return }
    createMutation.mutate({
      ncm_prefixo: ncmPrefixo,
      descricao,
      regime: regimenForm,
      aliquota_interna: parseFloat(aliqInterna) || 20.5,
      mva_original: mva ? parseFloat(mva) : null,
      reducao_bc_pct: parseFloat(reducaoBC) || 0,
      segmento_codigo: parseInt(createSegmento, 10),
    })
  }

  async function handleImport() {
    if (!importFile) return
    if (!importSegmento) { toast.error('Selecione o segmento antes de importar'); return }
    setImportLoading(true)
    try {
      const fd = new FormData()
      fd.append('file', importFile)
      fd.append('uf_estado', selectedUF)
      fd.append('segmento_codigo', importSegmento)
      const res = await fetch('/api/icms-fronteira/regras/importar', {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` },
        body: fd,
      })
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      const result = await res.json() as { imported: number; skipped: number; errors?: string[] }
      const errors = result.errors ?? []
      // Mostra um único toast com summary; o Dialog abaixo entrega o detalhe.
      if (errors.length === 0) {
        toast.success(`${result.imported} regra(s) importada(s). ${result.skipped} ignorada(s).`)
      } else {
        toast.warning(`${result.imported} importada(s), ${errors.length} com erro — abra os detalhes.`)
      }
      setImportResult({ imported: result.imported, skipped: result.skipped, errors })
      queryClient.invalidateQueries({ queryKey: ['icms-fronteira/regras', selectedUF] })
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
      {/* Seletor de UF */}
      <Tabs value={selectedUF} onValueChange={(v) => setSelectedUF(v as 'PE' | 'BA' | 'CE')}>
        <TabsList>
          <TabsTrigger value="PE">PE — Pernambuco</TabsTrigger>
          <TabsTrigger value="BA">BA — Bahia</TabsTrigger>
          <TabsTrigger value="CE">CE — Ceará</TabsTrigger>
        </TabsList>
      </Tabs>

      {/* Import card */}
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-semibold">Importar Regras (CSV/XLSX) — {selectedUF}</CardTitle>
        </CardHeader>
        <CardContent>
          <Alert className="mb-3">
            <AlertDescription className="text-xs">
              Formato esperado: <code>ncm_prefixo; descricao; regime; aliquota_interna; mva_original; reducao_bc_pct</code>.
              Todas as regras importadas ficam vinculadas ao <strong>segmento selecionado</strong> nesta UF.
            </AlertDescription>
          </Alert>
          {segmentos.length === 0 ? (
            <Alert variant="destructive">
              <AlertDescription className="text-xs">
                Nenhum segmento cadastrado para {selectedUF}. Cadastre os segmentos em
                <strong> Administrativo → Segmentos ST</strong> antes de importar regras.
              </AlertDescription>
            </Alert>
          ) : (
            <div className="flex items-end gap-2 flex-wrap">
              <div className="grid gap-1.5">
                <Label className="text-xs">Segmento (obrigatório)</Label>
                <Select value={importSegmento} onValueChange={setImportSegmento}>
                  <SelectTrigger className="w-72 text-xs">
                    <SelectValue placeholder="Selecione o segmento desta UF..." />
                  </SelectTrigger>
                  <SelectContent>
                    {segmentos.map(s => (
                      <SelectItem key={s.codigo} value={String(s.codigo)}>
                        {String(s.codigo).padStart(2, '0')} — {s.descricao}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <Input
                type="file"
                accept=".csv,.xlsx,.xls"
                className="max-w-sm text-xs"
                onChange={(e) => setImportFile(e.target.files?.[0] ?? null)}
              />
              <Button
                size="sm"
                onClick={handleImport}
                disabled={!importFile || !importSegmento || importLoading}
              >
                <Upload className="h-3.5 w-3.5 mr-1" />
                {importLoading ? 'Importando...' : 'Importar'}
              </Button>
            </div>
          )}
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
                <TableHead className="text-xs font-semibold uppercase tracking-wide">Segmento</TableHead>
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
                  <TableCell colSpan={9} className="text-center text-xs text-muted-foreground py-6">
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
                    <TableCell className="text-xs max-w-[180px]">
                      {row.segmento_codigo == null
                        ? <span className="text-amber-600" title="Sem segmento — não gera ST">⚠ sem segmento</span>
                        : <div className="truncate" title={segmentoLabel(row.segmento_codigo)}>{segmentoLabel(row.segmento_codigo)}</div>}
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
                      <div className="flex items-center gap-1">
                        {/* Editar: disponível para todas as regras (da empresa e globais/base). */}
                        <Button
                          size="sm"
                          variant="ghost"
                          className="h-7 w-7 p-0"
                          title={row.is_global ? 'Editar regra base (global)' : 'Editar regra'}
                          onClick={() => setEditing({ ...row })}
                        >
                          <Pencil className="h-3.5 w-3.5" />
                        </Button>
                        {/* Remover: só regras da empresa — as globais são seed compartilhado. */}
                        {!row.is_global && (
                          <Button
                            size="sm"
                            variant="ghost"
                            className="h-7 w-7 p-0 text-destructive hover:text-destructive"
                            title="Remover regra"
                            onClick={() => {
                              if (confirm('Remover esta regra?')) deleteMutation.mutate(row.id)
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
              <Label>Segmento ({selectedUF}) — obrigatório</Label>
              {segmentos.length === 0 ? (
                <p className="text-xs text-amber-600">
                  Nenhum segmento cadastrado para {selectedUF}. Cadastre em Administrativo → Segmentos ST.
                </p>
              ) : (
                <Select value={createSegmento} onValueChange={setCreateSegmento}>
                  <SelectTrigger>
                    <SelectValue placeholder="Selecione o segmento desta UF..." />
                  </SelectTrigger>
                  <SelectContent>
                    {segmentos.map(s => (
                      <SelectItem key={s.codigo} value={String(s.codigo)}>
                        {String(s.codigo).padStart(2, '0')} — {s.descricao}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
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
              disabled={createMutation.isPending || !ncmPrefixo || !descricao || !createSegmento}
            >
              {createMutation.isPending ? 'Salvando...' : 'Criar Regra'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Edit Dialog — ajuste manual de uma regra NCM importada */}
      <Dialog open={!!editing} onOpenChange={(o) => { if (!o) setEditing(null) }}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>Editar Regra NCM {editing?.ncm_prefixo}</DialogTitle>
            <DialogDescription className="text-xs">
              Ajuste os dados desta regra para a UF {selectedUF}. Campos em branco
              de MVA significam "não se aplica".
            </DialogDescription>
          </DialogHeader>
          {editing && (
            <div className="grid gap-3 py-2">
              <div className="grid gap-1.5">
                <Label className="text-xs">Descrição</Label>
                <Input
                  value={editing.descricao}
                  onChange={(e) => setEditing({ ...editing, descricao: e.target.value })}
                />
              </div>
              <div className="grid gap-1.5">
                <Label className="text-xs">Segmento ({selectedUF})</Label>
                <Select
                  value={editing.segmento_codigo != null ? String(editing.segmento_codigo) : ''}
                  onValueChange={(v) => setEditing({ ...editing, segmento_codigo: parseInt(v, 10) })}
                >
                  <SelectTrigger><SelectValue placeholder="Selecione o segmento..." /></SelectTrigger>
                  <SelectContent>
                    {segmentos.map(s => (
                      <SelectItem key={s.codigo} value={String(s.codigo)}>
                        {String(s.codigo).padStart(2, '0')} — {s.descricao}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div className="grid gap-1.5">
                  <Label className="text-xs">Regime</Label>
                  <Select
                    value={editing.regime}
                    onValueChange={(v) => setEditing({ ...editing, regime: v })}
                  >
                    <SelectTrigger><SelectValue /></SelectTrigger>
                    <SelectContent>
                      <SelectItem value="ANTECIPACAO">Antecipação</SelectItem>
                      <SelectItem value="ST">Substituição Tributária</SelectItem>
                      <SelectItem value="DIFAL">DIFAL</SelectItem>
                      <SelectItem value="ISENTO">Isento</SelectItem>
                      <SelectItem value="NORMAL">Normal</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="grid gap-1.5">
                  <Label className="text-xs">Alíquota Interna %</Label>
                  <Input
                    type="number" step="0.01"
                    value={editing.aliquota_interna}
                    onChange={(e) => setEditing({ ...editing, aliquota_interna: parseFloat(e.target.value) || 0 })}
                  />
                </div>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div className="grid gap-1.5">
                  <Label className="text-xs">MVA Original %</Label>
                  <Input
                    type="number" step="0.01" placeholder="—"
                    value={editing.mva_original ?? ''}
                    onChange={(e) => setEditing({ ...editing, mva_original: e.target.value === '' ? null : parseFloat(e.target.value) })}
                  />
                </div>
                <div className="grid gap-1.5">
                  <Label className="text-xs">Redução BC %</Label>
                  <Input
                    type="number" step="0.01"
                    value={editing.reducao_bc_pct}
                    onChange={(e) => setEditing({ ...editing, reducao_bc_pct: parseFloat(e.target.value) || 0 })}
                  />
                </div>
              </div>
              <div className="grid grid-cols-3 gap-3">
                <div className="grid gap-1.5">
                  <Label className="text-xs">MVA Aj. 4%</Label>
                  <Input
                    type="number" step="0.01" placeholder="—"
                    value={editing.mva_ajustado_4pct ?? ''}
                    onChange={(e) => setEditing({ ...editing, mva_ajustado_4pct: e.target.value === '' ? null : parseFloat(e.target.value) })}
                  />
                </div>
                <div className="grid gap-1.5">
                  <Label className="text-xs">MVA Aj. 7%</Label>
                  <Input
                    type="number" step="0.01" placeholder="—"
                    value={editing.mva_ajustado_7pct ?? ''}
                    onChange={(e) => setEditing({ ...editing, mva_ajustado_7pct: e.target.value === '' ? null : parseFloat(e.target.value) })}
                  />
                </div>
                <div className="grid gap-1.5">
                  <Label className="text-xs">MVA Aj. 12%</Label>
                  <Input
                    type="number" step="0.01" placeholder="—"
                    value={editing.mva_ajustado_12pct ?? ''}
                    onChange={(e) => setEditing({ ...editing, mva_ajustado_12pct: e.target.value === '' ? null : parseFloat(e.target.value) })}
                  />
                </div>
              </div>
            </div>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={() => setEditing(null)}>Cancelar</Button>
            <Button
              onClick={() => editing && updateMutation.mutate(editing)}
              disabled={updateMutation.isPending}
            >
              {updateMutation.isPending ? 'Salvando...' : 'Salvar alterações'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Detalhe da última importação — toast some, este dialog persiste */}
      <Dialog open={!!importResult} onOpenChange={(v) => !v && setImportResult(null)}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>Resultado da importação</DialogTitle>
            <DialogDescription>
              Resumo do processamento do arquivo na UF {selectedUF}.
            </DialogDescription>
          </DialogHeader>
          {importResult && (
            <div className="space-y-3">
              <div className="grid grid-cols-3 gap-3 text-sm">
                <div className="rounded border bg-emerald-50 px-3 py-2">
                  <div className="text-[11px] text-emerald-700 uppercase">Importadas</div>
                  <div className="text-2xl font-bold text-emerald-700">{importResult.imported}</div>
                </div>
                <div className="rounded border bg-slate-50 px-3 py-2">
                  <div className="text-[11px] text-slate-700 uppercase">Ignoradas</div>
                  <div className="text-2xl font-bold text-slate-700">{importResult.skipped}</div>
                </div>
                <div className={`rounded border px-3 py-2 ${importResult.errors.length > 0 ? 'bg-amber-50' : 'bg-slate-50'}`}>
                  <div className={`text-[11px] uppercase ${importResult.errors.length > 0 ? 'text-amber-700' : 'text-slate-700'}`}>Com erro</div>
                  <div className={`text-2xl font-bold ${importResult.errors.length > 0 ? 'text-amber-700' : 'text-slate-700'}`}>{importResult.errors.length}</div>
                </div>
              </div>
              {importResult.errors.length > 0 && (
                <div className="space-y-1">
                  <p className="text-xs font-semibold text-amber-700">Detalhes:</p>
                  <ul className="max-h-72 overflow-y-auto rounded border bg-amber-50/50 p-2 text-xs font-mono space-y-1">
                    {importResult.errors.map((err, i) => (
                      <li key={i} className="text-amber-900">{err}</li>
                    ))}
                  </ul>
                </div>
              )}
            </div>
          )}
          <DialogFooter>
            <Button onClick={() => setImportResult(null)}>Fechar</Button>
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
    const s = m.trim()
    if (/^\d{2}\/\d{4}$/.test(s)) return s
    const [y, mo] = s.split('-')
    return mo && y ? `${mo}/${y}` : ''
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
            type="text"
            placeholder="MM/AAAA"
            maxLength={7}
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
    const s = m.trim()
    if (/^\d{2}\/\d{4}$/.test(s)) return s
    const [y, mo] = s.split('-')
    return mo && y ? `${mo}/${y}` : ''
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
            type="text"
            placeholder="MM/AAAA"
            maxLength={7}
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
          <Button size="sm" variant="outline" onClick={async () => {
            try {
              const url = `/api/icms-fronteira/divergencias/exportar/csv${periodo ? `?periodo=${encodeURIComponent(periodo)}` : ''}`
              const res = await fetch(url, { headers: { Authorization: `Bearer ${token}` } })
              if (!res.ok) throw new Error(`Erro ${res.status}`)
              const blob = await res.blob()
              const blobUrl = URL.createObjectURL(blob)
              const a = document.createElement('a')
              a.href = blobUrl
              a.download = `divergencias${periodo ? '-' + periodo.replace('/','-') : ''}.csv`
              a.click()
              URL.revokeObjectURL(blobUrl)
            } catch { toast.error('Erro ao exportar CSV') }
          }}>
            <FileDown className="h-3.5 w-3.5 mr-1" />CSV
          </Button>
          <Button size="sm" variant="outline" onClick={async () => {
            try {
              const url = `/api/icms-fronteira/divergencias/exportar/xlsx${periodo ? `?periodo=${encodeURIComponent(periodo)}` : ''}`
              const res = await fetch(url, { headers: { Authorization: `Bearer ${token}` } })
              if (!res.ok) throw new Error(`Erro ${res.status}`)
              const blob = await res.blob()
              const blobUrl = URL.createObjectURL(blob)
              const a = document.createElement('a')
              a.href = blobUrl
              a.download = `divergencias${periodo ? '-' + periodo.replace('/','-') : ''}.xlsx`
              a.click()
              URL.revokeObjectURL(blobUrl)
            } catch { toast.error('Erro ao exportar Excel') }
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
            type="text"
            placeholder="MM/AAAA"
            maxLength={7}
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
          <Button size="sm" variant="outline" onClick={async () => {
            try {
              const params = new URLSearchParams({ regime: regimeFilter })
              if (periodo) params.set('periodo', periodo)
              const res = await fetch(`/api/icms-fronteira/itens/exportar/csv?${params}`, {
                headers: { Authorization: `Bearer ${token}` },
              })
              if (!res.ok) throw new Error(`Erro ${res.status}`)
              const blob = await res.blob()
              const blobUrl = URL.createObjectURL(blob)
              const a = document.createElement('a')
              a.href = blobUrl
              a.download = `icms-fronteira-itens-${regimeFilter}.csv`
              a.click()
              URL.revokeObjectURL(blobUrl)
            } catch { toast.error('Erro ao exportar CSV') }
          }}>
            <FileDown className="h-3.5 w-3.5 mr-1" />CSV
          </Button>
          <Button size="sm" variant="outline" onClick={async () => {
            try {
              const params = new URLSearchParams({ regime: regimeFilter })
              if (periodo) params.set('periodo', periodo)
              const res = await fetch(`/api/icms-fronteira/itens/exportar/xlsx?${params}`, {
                headers: { Authorization: `Bearer ${token}` },
              })
              if (!res.ok) throw new Error(`Erro ${res.status}`)
              const blob = await res.blob()
              const blobUrl = URL.createObjectURL(blob)
              const a = document.createElement('a')
              a.href = blobUrl
              a.download = `icms-fronteira-itens-${regimeFilter}.xlsx`
              a.click()
              URL.revokeObjectURL(blobUrl)
            } catch { toast.error('Erro ao exportar Excel') }
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
          <div style={{ width: '100%', height: 300 }}>
          <ResponsiveContainer width="100%" height="100%">
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
          </div>
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
// IncentivoTab — relatório das notas dispensadas pelo motor (PRODEPE/PROIND).
// Mesma estrutura visual de Antecipação/ST/DIFAL: cards de total + tabela.
// O endpoint /api/icms-fronteira/incentivo retorna A (SPED mês), B (SPED anterior)
// e C (XML sem SPED) — todos os blocos exibidos em uma só tabela com `bloco` na coluna.
// ---------------------------------------------------------------------------
function IncentivoTab({ token }: { token: string | null }) {
  const uf = useFronteiraUF()
  const [monthInput, setMonthInput] = useState('')
  const periodo = monthToPeriodo(monthInput)

  const { data, isLoading, isError } = useQuery<IncentivoResponse>({
    queryKey: ['icms-fronteira/incentivo', periodo, uf],
    queryFn: async () => {
      const params = new URLSearchParams()
      if (periodo) params.set('periodo', periodo)
      if (uf) params.set('uf', uf)
      const qs = params.toString()
      const res = await fetch(`/api/icms-fronteira/incentivo${qs ? `?${qs}` : ''}`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      return res.json()
    },
    enabled: !!token,
  })

  const programaLabel = (p: string) => {
    const cls = p === 'PROIND'
      ? 'bg-indigo-100 text-indigo-800'
      : 'bg-emerald-100 text-emerald-800'
    return <span className={`inline-block rounded px-2 py-0.5 text-[10px] font-semibold ${cls}`}>{p}</span>
  }

  const blocoLabel = (b: string) => {
    const map: Record<string, { label: string; cls: string }> = {
      mes_atual:     { label: 'Mês',       cls: 'bg-blue-50 text-blue-700' },
      mes_anterior:  { label: 'Anterior',  cls: 'bg-slate-100 text-slate-700' },
      nao_sped:      { label: 'Sem SPED',  cls: 'bg-amber-50 text-amber-800' },
    }
    const v = map[b] ?? { label: b, cls: 'bg-gray-100 text-gray-700' }
    return <span className={`inline-block rounded px-1.5 py-0.5 text-[10px] font-medium ${v.cls}`}>{v.label}</span>
  }

  return (
    <div className="space-y-4">
      {/* Filtros */}
      <div className="flex items-center gap-3 flex-wrap">
        <Label htmlFor="notas-periodo-incentivo" className="text-xs whitespace-nowrap">Período (mês/ano):</Label>
        <Input
          id="notas-periodo-incentivo"
          type="month"
          value={monthInput}
          onChange={e => setMonthInput(e.target.value)}
          className="w-40 h-8"
        />
        {periodo && <span className="text-xs text-muted-foreground">{periodo}</span>}
        {!periodo && <span className="text-xs text-muted-foreground">Sem filtro = todos os meses</span>}
        {uf && <span className="text-xs text-muted-foreground">UF: <strong>{uf}</strong></span>}
      </div>

      {/* Cards de total */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-3">
        <Card className="border-emerald-200">
          <CardContent className="p-4">
            <p className="text-[11px] text-muted-foreground uppercase tracking-wide">Total dispensado</p>
            <p className="text-2xl font-bold text-emerald-700 mt-1">
              {fmtBRL(data?.total_dispensado ?? 0)}
            </p>
            <p className="text-[11px] text-muted-foreground mt-1">
              {data?.count ?? 0} nota(s) com dispensa ativa
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4">
            <p className="text-[11px] text-muted-foreground uppercase tracking-wide">Mês selecionado</p>
            <p className="text-lg font-semibold mt-1">{fmtBRL(data?.total_mes_atual ?? 0)}</p>
            <p className="text-[11px] text-muted-foreground">{data?.count_mes_atual ?? 0} nota(s)</p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4">
            <p className="text-[11px] text-muted-foreground uppercase tracking-wide">Meses anteriores</p>
            <p className="text-lg font-semibold mt-1">{fmtBRL(data?.total_mes_anterior ?? 0)}</p>
            <p className="text-[11px] text-muted-foreground">{data?.count_mes_anterior ?? 0} nota(s)</p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4">
            <p className="text-[11px] text-muted-foreground uppercase tracking-wide">XML sem SPED</p>
            <p className="text-lg font-semibold mt-1">{fmtBRL(data?.total_nao_sped ?? 0)}</p>
            <p className="text-[11px] text-muted-foreground">{data?.count_nao_sped ?? 0} nota(s)</p>
          </CardContent>
        </Card>
      </div>

      {/* Quebra por programa */}
      {data && Object.keys(data.por_programa ?? {}).length > 0 && (
        <div className="flex items-center gap-3 flex-wrap text-xs">
          <span className="text-muted-foreground">Por programa:</span>
          {Object.entries(data.por_programa).map(([prog, valor]) => (
            <span key={prog} className="inline-flex items-center gap-1.5 rounded border bg-white px-2 py-1">
              {programaLabel(prog)}
              <span className="font-semibold">{fmtBRL(valor)}</span>
            </span>
          ))}
        </div>
      )}

      {/* Tabela */}
      <Card>
        <CardContent className="p-0">
          {isLoading ? (
            <p className="p-6 text-sm text-muted-foreground">Carregando notas dispensadas…</p>
          ) : isError ? (
            <p className="p-6 text-sm text-red-600">Erro ao consultar /incentivo.</p>
          ) : !data || data.rows.length === 0 ? (
            <div className="p-6">
              <p className="text-sm text-muted-foreground">
                Nenhuma nota dispensada por incentivo no recorte atual.
                {!periodo && uf && ' Selecione um período para ver as do mês.'}
              </p>
              <p className="text-xs text-muted-foreground mt-2">
                Verifique se há enquadramento PRODEPE/PROIND ativo na aba Administrativo →
                PRODEPE para a empresa selecionada. DIFAL (CFOPs 2551/2556) não entra na
                dispensa e por isso não aparece aqui.
              </p>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-16">Bloco</TableHead>
                    <TableHead>Chave</TableHead>
                    <TableHead className="w-24">Data</TableHead>
                    <TableHead className="w-20">Nº NF</TableHead>
                    <TableHead>Fornecedor</TableHead>
                    <TableHead className="w-12">UF</TableHead>
                    <TableHead className="w-16">CFOP</TableHead>
                    <TableHead className="text-right w-24">V.Prod</TableHead>
                    <TableHead className="text-right w-24">V.ICMS NF</TableHead>
                    <TableHead className="text-right w-28">ICMS dispensado</TableHead>
                    <TableHead className="w-20">Programa</TableHead>
                    <TableHead className="w-28">Nº Ato</TableHead>
                    <TableHead className="w-44">Vigência</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {data.rows.map((r, i) => (
                    <TableRow key={`${r.chave_nfe}-${r.cfop}-${i}`}>
                      <TableCell>{blocoLabel(r.bloco)}</TableCell>
                      <TableCell><ChaveCell chave={r.chave_nfe} /></TableCell>
                      <TableCell className="text-xs">{r.data_emissao?.slice(0, 10)}</TableCell>
                      <TableCell className="text-xs font-mono">{r.numero_nfe}</TableCell>
                      <TableCell className="text-xs">
                        <div className="leading-tight">
                          <div className="truncate max-w-[200px]">{r.forn_nome || <span className="text-muted-foreground">—</span>}</div>
                          <div className="font-mono text-[10px] text-muted-foreground">{r.forn_cnpj && formatCNPJ(r.forn_cnpj)}</div>
                        </div>
                      </TableCell>
                      <TableCell className="text-xs">{r.forn_uf}</TableCell>
                      <TableCell className="text-xs font-mono">{r.cfop}</TableCell>
                      <TableCell className="text-right text-xs">{fmtBRL(r.v_prod)}</TableCell>
                      <TableCell className="text-right text-xs">{fmtBRL(r.v_icms)}</TableCell>
                      <TableCell className="text-right text-xs font-semibold text-emerald-700">
                        {fmtBRL(r.icms_seria_devido)}
                      </TableCell>
                      <TableCell>{programaLabel(r.programa)}</TableCell>
                      <TableCell className="text-xs">{r.num_ato || <span className="text-muted-foreground">—</span>}</TableCell>
                      <TableCell className="text-[11px] text-muted-foreground">
                        {r.vigencia_inicio || '—'} → {r.vigencia_fim || '—'}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
                <TableFooter>
                  <TableRow>
                    <TableCell colSpan={9} className="text-right font-semibold text-sm">Total dispensado:</TableCell>
                    <TableCell className="text-right font-bold text-sm text-emerald-700">{fmtBRL(data.total_dispensado)}</TableCell>
                    <TableCell colSpan={3} />
                  </TableRow>
                </TableFooter>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>
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

  // Eixo UF: UF de trabalho do módulo (filiais daquela UF). Lista vem do SPED.
  const { data: ufsData } = useQuery<{ ufs: string[] }>({
    queryKey: ['icms-fronteira/ufs'],
    queryFn: async () => {
      const res = await fetch('/api/icms-fronteira/ufs', { headers: { Authorization: `Bearer ${token}` } })
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      return res.json()
    },
    enabled: !!token,
  })
  const ufs = ufsData?.ufs ?? []
  const [uf, setUf] = useState('')
  useEffect(() => {
    if (!uf && ufs.length > 0) setUf(ufs[0])
  }, [ufs, uf])

  const pathToTab: Record<string, string> = {
    '/icms-fronteira':              'resumo',
    '/icms-fronteira/antecipacao':  'antecipacao',
    '/icms-fronteira/st':           'st',
    '/icms-fronteira/difal':        'difal',
    '/icms-fronteira/incentivo':    'incentivo',
    '/icms-fronteira/planilha':     'planilha',
    '/icms-fronteira/fretes':       'fretes',
    '/icms-fronteira/motor-fiscal': 'motor-fiscal',
    '/icms-fronteira/divergencias': 'divergencias',
    '/icms-fronteira/reconciliacao': 'reconciliacao',
    '/icms-fronteira/legislacao':    'legislacao',
    '/icms-fronteira/regras':       'administrativo', // legacy → agora vive em Administrativo
    '/icms-fronteira/extrato':      'extrato',
    '/icms-fronteira/contestacoes': 'contestacoes',
    '/icms-fronteira/apuracao':     'apuracao',
    '/icms-fronteira/administrativo': 'administrativo',
  }
  const tabToPath: Record<string, string> = {
    resumo:        '/icms-fronteira',
    antecipacao:   '/icms-fronteira/antecipacao',
    st:            '/icms-fronteira/st',
    difal:         '/icms-fronteira/difal',
    incentivo:     '/icms-fronteira/incentivo',
    planilha:      '/icms-fronteira/planilha',
    fretes:        '/icms-fronteira/fretes',
    'motor-fiscal':'/icms-fronteira/motor-fiscal',
    divergencias:  '/icms-fronteira/divergencias',
    reconciliacao: '/icms-fronteira/reconciliacao',
    legislacao:    '/icms-fronteira/legislacao',
    extrato:       '/icms-fronteira/extrato',
    contestacoes:  '/icms-fronteira/contestacoes',
    apuracao:      '/icms-fronteira/apuracao',
    administrativo:'/icms-fronteira/administrativo',
  }

  const tab = pathToTab[location.pathname] ?? 'resumo'

  function handleTabChange(value: string) {
    navigate(tabToPath[value] ?? '/icms-fronteira')
  }

  return (
    <div className="space-y-6 p-6">
      {/* Page header */}
      <div className="flex items-center gap-2">
        <h1 className="text-xl font-semibold">Módulo ICMS Fronteira</h1>
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

      {/* Menu do módulo: 1ª linha = UF de trabalho (eixo obrigatório, colado à
          esquerda e compacto, destaque vermelho); 2ª linha = abas. Todo o módulo
          opera sobre a UF selecionada. */}
      <FronteiraUFContext.Provider value={uf}>
      <div className="space-y-2">
        <div className="flex items-center gap-2 flex-wrap">
          <span className="text-sm font-bold uppercase tracking-wide text-red-700">
            UF de trabalho:
          </span>
          <Select value={uf} onValueChange={setUf} disabled={ufs.length === 0}>
            <SelectTrigger className="h-9 w-20 border-2 border-red-500 bg-white text-base font-extrabold text-red-700 focus:ring-red-500">
              <SelectValue placeholder={ufs.length === 0 ? '—' : 'UF'} />
            </SelectTrigger>
            <SelectContent>
              {ufs.map((u) => (
                <SelectItem key={u} value={u} className="font-bold">
                  {u}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <span className="text-xs text-muted-foreground">
            {ufs.length > 1
              ? <>* obrigatório — apurando as filiais de <strong>{uf}</strong>; troque para ver as demais.</>
              : <>* obrigatório — toda apuração é feita sobre as filiais desta UF.</>}
          </span>
        </div>

      <Tabs value={tab} onValueChange={handleTabChange}>
        {/* Tabs compactas — text-[11px] + px-2 para caber tudo em uma linha em telas ≥1280px.
            Em telas menores faz quebra (flex-wrap). Regras NCM foi movida para dentro de Administrativo
            (acessível via Administrativo → Regras NCM por Decreto). */}
        <TabsList className="flex-wrap h-auto gap-0.5 text-[11px]">
          <TabsTrigger value="resumo" className="px-2 py-1.5 text-[11px]">Resumo</TabsTrigger>
          <TabsTrigger value="antecipacao" className="px-2 py-1.5 text-[11px]">Antecipação</TabsTrigger>
          <TabsTrigger value="st" className="px-2 py-1.5 text-[11px]">Subst. Tributária</TabsTrigger>
          <TabsTrigger value="difal" className="px-2 py-1.5 text-[11px]">DIFAL</TabsTrigger>
          <TabsTrigger value="incentivo" className="px-2 py-1.5 text-[11px] data-[state=active]:bg-emerald-50 data-[state=active]:text-emerald-700">
            <ShieldCheck className="h-3 w-3 mr-1" />Incentivo
          </TabsTrigger>
          <TabsTrigger value="planilha" className="px-2 py-1.5 text-[11px]">Planilha</TabsTrigger>
          <TabsTrigger value="fretes" className="px-2 py-1.5 text-[11px]">Fretes</TabsTrigger>
          <TabsTrigger value="motor-fiscal" className="px-2 py-1.5 text-[11px]">Motor Fiscal</TabsTrigger>
          <TabsTrigger value="divergencias" className="px-2 py-1.5 text-[11px]">Divergências</TabsTrigger>
          <TabsTrigger value="reconciliacao" className="px-2 py-1.5 text-[11px]">Reconciliação</TabsTrigger>
          <TabsTrigger value="legislacao" className="px-2 py-1.5 text-[11px]">Legislação</TabsTrigger>
          <TabsTrigger value="apuracao" className="px-2 py-1.5 text-[11px]">Apuração Mensal</TabsTrigger>
          <TabsTrigger value="extrato" className="px-2 py-1.5 text-[11px]">Extrato SEFAZ</TabsTrigger>
          <TabsTrigger value="contestacoes" className="px-2 py-1.5 text-[11px]">Contestações</TabsTrigger>
          <TabsTrigger value="administrativo" className="px-2 py-1.5 text-[11px]">Administrativo</TabsTrigger>
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
                V.Prod × (alíq. interna − alíq. interestadual). Três blocos: meses anteriores
                no SPED, mês atual no SPED, e XML não lançadas no SPED.
              </p>
              <div className="flex justify-end items-center mb-2">
                <RecalcularButton />
              </div>
              <NotasTabBlocos endpointSped="/api/icms-fronteira/antecipacao" regime="antecipacao" token={token} />
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
                Notas com ICMS-ST. Na aba Planilha de Itens cada nota é detalhada por produto,
                pois o MVA pode diferir por NCM e uma mesma NF pode ter itens de regimes distintos.
                Três blocos: meses anteriores no SPED, mês atual no SPED, e XML não lançadas.
              </p>
              <div className="flex justify-end mb-2">
                <RecalcularButton />
              </div>
              <NotasTabBlocos endpointSped="/api/icms-fronteira/st" regime="st" token={token} />
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
                (2551, 2556). DIFAL = V.Prod × (alíq. interna − alíq. inter.). Três blocos:
                meses anteriores no SPED, mês atual, e XML não lançadas.
              </p>
              <div className="flex justify-end mb-2">
                <RecalcularButton />
              </div>
              <NotasTabBlocos endpointSped="/api/icms-fronteira/difal" regime="difal" token={token} />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="incentivo" className="mt-6">
          <Card>
            <CardHeader>
              <CardTitle className="text-base font-semibold flex items-center gap-2">
                <BadgePercent className="h-4 w-4 text-emerald-600" />
                Incentivo — Notas dispensadas por PRODEPE / PROIND
              </CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-xs text-muted-foreground mb-4">
                Notas em que o motor zerou a antecipação/ST por enquadramento ativo em PRODEPE
                ou PROIND no CNPJ recebedor. <strong>ICMS dispensado</strong> é o valor que seria devido
                se não houvesse incentivo — funciona como prova fiscal da economia. DIFAL
                (CFOPs 2551/2556) não entra na dispensa e por isso não aparece aqui.
              </p>
              <IncentivoTab token={token} />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="fretes" className="mt-6">
          <Card>
            <CardHeader>
              <CardTitle className="text-base font-semibold flex items-center gap-2">
                <Truck className="h-4 w-4 text-slate-600" />
                Fretes — CT-e vinculados às NFs de mercadoria
              </CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-xs text-muted-foreground mb-4">
                CT-e de entrada vinculados às NFs de mercadoria interestadual. O ICMS fronteira
                sobre o frete é calculado com o mesmo regime da NF correspondente.
              </p>
              <FretesTab token={token} />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="motor-fiscal" className="mt-6">
          <Card>
            <CardHeader>
              <CardTitle className="text-base font-semibold flex items-center gap-2">
                <Calculator className="h-4 w-4 text-indigo-600" />
                Motor de Cálculo Fiscal — Fase 1 (Substituição Tributária BA)
              </CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-xs text-muted-foreground mb-4">
                Processa <strong>itens C170 do SPED</strong> (entradas, CFOP 2403) para empresas
                com <strong>UF = BA</strong>. NCM via cruzamento <code className="text-[10px] bg-muted px-1 rounded">reg_c170.cod_item → reg_0200.cod_ncm</code>.
                MVA cruzada por NCM × alíquota interestadual × UF em
                <code className="text-[10px] bg-muted px-1 rounded">icms_fronteira_regras_ncm</code>.
                Frete (NF + CT-e tomador=destinatário) rateado proporcional ao item.
                Persistência em <code className="text-[10px] bg-muted px-1 rounded">fiscal_calculations</code> (auditoria item-a-item).
              </p>
              <MotorFiscalTab token={token} />
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

        <TabsContent value="reconciliacao" className="mt-6">
          <ReconciliacaoTab token={token} />
        </TabsContent>

        <TabsContent value="legislacao" className="mt-6">
          <LegislacaoTab token={token} />
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

        {/* Aba "Regras NCM" foi movida para Administrativo → Regras por Decreto (ver
            AdministrativoFronteira.tsx). Mantemos o redirect de URL no pathToTab acima
            para que /icms-fronteira/regras continue funcionando como atalho legacy. */}

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

        <TabsContent value="administrativo" className="mt-6">
          <Card>
            <CardHeader>
              <CardTitle className="text-base font-semibold">
                Administrativo
              </CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-xs text-muted-foreground mb-4">
                Filiais importadas, parâmetros por UF (benefícios) e edição dos dados da empresa em foco.
                Substitui a antiga aba "Filiais" e "UFs" da Gestão de Ambiente.
              </p>
              <AdministrativoTab uf={uf} />
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
      </div>
      </FronteiraUFContext.Provider>
    </div>
  )
}
