import { useState, useEffect, useMemo } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
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
import { RefreshCw, Loader2, Search, X } from 'lucide-react'
import { toast } from 'sonner'
import { useAuth } from '@/contexts/AuthContext'
import { formatCurrency } from '@/lib/utils'

// ---------------------------------------------------------------------------
// Fornecedores/Clientes — enriquecimento com CNPJ público (BrasilAPI/Receita
// Federal). Não confunde com o módulo de Reforma Tributária (RFB paga) —
// aqui é só situação cadastral, CNAE e Simples Nacional/MEI de quem já
// aparece como fornecedor (nfe_entradas) ou cliente PJ (nfe_saidas).
// ---------------------------------------------------------------------------

interface FornecedorClienteRow {
  cnpj: string
  tipo: 'fornecedor' | 'cliente'
  nome_nota: string
  razao_social_rfb: string
  nome_fantasia: string
  situacao_cadastral: string
  data_situacao_cadastral: string | null
  natureza_juridica: string
  porte: string
  cnae_codigo: string
  cnae_descricao: string
  uf: string
  municipio: string
  simples_nacional: boolean | null
  mei: boolean | null
  ano: number
  valor_acumulado: number
  qtd_notas: number
  consultado_rfb: boolean
}

interface RelatorioResponse {
  rows: FornecedorClienteRow[]
  count: number
}

interface JobStatus {
  id: string
  status: 'pending' | 'processing' | 'completed' | 'error' | 'cancelled'
  total: number
  processados: number
  encontrados: number
  erros: number
  mensagem: string
}

function formatDataSituacao(data: string | null) {
  if (!data) return null
  const [ano, mes, dia] = data.split('-')
  if (!ano || !mes || !dia) return data
  return `${dia}/${mes}/${ano}`
}

function situacaoBadge(situacao: string, dataSituacao: string | null) {
  if (!situacao) return <span className="text-muted-foreground text-xs">—</span>
  const cls = situacao === 'ATIVA'
    ? 'bg-green-50 text-green-700 border-green-200'
    : 'bg-red-50 text-red-700 border-red-200'
  const dataFormatada = formatDataSituacao(dataSituacao)
  return (
    <div className="flex flex-col gap-0.5">
      <Badge variant="outline" className={`text-[10px] px-1.5 py-0 w-fit ${cls}`}>{situacao}</Badge>
      {dataFormatada && (
        <span className="text-[10px] text-muted-foreground">desde {dataFormatada}</span>
      )}
    </div>
  )
}

function boolBadge(value: boolean | null, labelTrue: string, labelFalse: string) {
  if (value === null || value === undefined) return <span className="text-muted-foreground text-xs">—</span>
  return value
    ? <Badge variant="outline" className="text-[10px] px-1.5 py-0 bg-blue-50 text-blue-700 border-blue-200">{labelTrue}</Badge>
    : <span className="text-xs text-muted-foreground">{labelFalse}</span>
}

export default function FornecedoresClientesRFB() {
  const { token, companyId } = useAuth()
  const queryClient = useQueryClient()
  const [tipoFiltro, setTipoFiltro] = useState<'todos' | 'fornecedor' | 'cliente'>('todos')
  const [busca, setBusca] = useState('')
  const [jobId, setJobId] = useState<string | null>(null)
  const [enriquecendo, setEnriquecendo] = useState(false)

  const { data, isLoading, isError } = useQuery<RelatorioResponse>({
    queryKey: ['fornecedores-clientes/relatorio', tipoFiltro, companyId],
    queryFn: async () => {
      const params = new URLSearchParams()
      if (tipoFiltro !== 'todos') params.set('tipo', tipoFiltro)
      const res = await fetch(`/api/fornecedores-clientes/relatorio?${params}`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      return res.json()
    },
    enabled: !!token,
  })

  const { data: jobStatus } = useQuery<JobStatus>({
    queryKey: ['fornecedores-clientes/job', jobId],
    queryFn: async () => {
      const res = await fetch(`/api/fornecedores-clientes/jobs/${jobId}`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) throw new Error(`Erro ${res.status}`)
      return res.json()
    },
    enabled: !!jobId && enriquecendo,
    refetchInterval: 1500,
  })

  useEffect(() => {
    if (!jobStatus) return
    if (jobStatus.status === 'completed' || jobStatus.status === 'error' || jobStatus.status === 'cancelled') {
      setEnriquecendo(false)
      if (jobStatus.status === 'completed') {
        toast.success(jobStatus.mensagem || 'Enriquecimento concluído')
      } else if (jobStatus.status === 'cancelled') {
        toast.info(jobStatus.mensagem || 'Enriquecimento cancelado')
      } else {
        toast.error(jobStatus.mensagem || 'Erro no enriquecimento')
      }
      queryClient.invalidateQueries({ queryKey: ['fornecedores-clientes/relatorio'] })
    }
  }, [jobStatus, queryClient])

  async function handleEnriquecer() {
    setEnriquecendo(true)
    try {
      const res = await fetch('/api/fornecedores-clientes/enriquecer', {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) {
        const err = await res.json().catch(() => ({}))
        throw new Error(err.error || `Erro ${res.status}`)
      }
      const result = await res.json()
      setJobId(result.job_id)
      if (result.a_consultar === 0) {
        toast.success(`Todos os ${result.total_cnpjs} CNPJs já estavam em cache`)
        setEnriquecendo(false)
      } else {
        toast.info(`Consultando ${result.a_consultar} CNPJ(s) na Receita Federal (${result.ja_em_cache} já em cache)...`)
      }
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Erro ao iniciar enriquecimento')
      setEnriquecendo(false)
    }
  }

  async function handleCancelar() {
    if (!jobId) return
    try {
      const res = await fetch(`/api/fornecedores-clientes/jobs/${jobId}/cancelar`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) {
        const err = await res.json().catch(() => ({}))
        throw new Error(err.error || `Erro ${res.status}`)
      }
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Erro ao cancelar')
    }
  }

  const rows = data?.rows ?? []
  const filtered = useMemo(() => {
    const q = busca.trim().toLowerCase()
    if (!q) return rows
    return rows.filter(r =>
      r.cnpj.includes(q) ||
      r.nome_nota.toLowerCase().includes(q) ||
      r.razao_social_rfb.toLowerCase().includes(q)
    )
  }, [rows, busca])

  const naoConsultados = rows.filter(r => !r.consultado_rfb).length

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Fornecedores e Clientes — Compliance RFB</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Situação cadastral, CNAE e Simples Nacional/MEI (Receita Federal via BrasilAPI, gratuito) cruzados com
          o valor acumulado de compra/venda por ano de cada fornecedor e cliente PJ já importado.
        </p>
      </div>

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-semibold">Enriquecimento com dados públicos</CardTitle>
          <CardDescription className="text-xs">
            Consulta a Receita Federal para cada CNPJ que ainda não está em cache ou está desatualizado
            (mais de 30 dias) — roda em segundo plano, ~1 CNPJ por segundo.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex items-center gap-3 flex-wrap">
          <Button size="sm" onClick={handleEnriquecer} disabled={enriquecendo}>
            {enriquecendo ? <Loader2 className="h-3.5 w-3.5 mr-1.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5 mr-1.5" />}
            {enriquecendo ? 'Consultando...' : 'Enriquecer com dados da Receita Federal'}
          </Button>
          {enriquecendo && (
            <Button size="sm" variant="outline" onClick={handleCancelar}>
              <X className="h-3.5 w-3.5 mr-1.5" />
              Cancelar
            </Button>
          )}
          {enriquecendo && jobStatus && (
            <span className="text-xs text-muted-foreground">
              {jobStatus.processados}/{jobStatus.total} processados — {jobStatus.encontrados} encontrados, {jobStatus.erros} com erro
            </span>
          )}
          {!enriquecendo && naoConsultados > 0 && (
            <span className="text-xs text-amber-700">
              {naoConsultados} CNPJ(s) na lista ainda sem consulta RFB
            </span>
          )}
        </CardContent>
      </Card>

      <div className="flex items-center gap-2 flex-wrap">
        <Select value={tipoFiltro} onValueChange={(v) => setTipoFiltro(v as typeof tipoFiltro)}>
          <SelectTrigger className="w-44 text-xs">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="todos">Fornecedores e Clientes</SelectItem>
            <SelectItem value="fornecedor">Só Fornecedores</SelectItem>
            <SelectItem value="cliente">Só Clientes</SelectItem>
          </SelectContent>
        </Select>
        <div className="relative w-64">
          <Search className="absolute left-2 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
          <Input
            placeholder="Buscar por CNPJ ou nome..."
            value={busca}
            onChange={(e) => setBusca(e.target.value)}
            className="pl-7 text-xs h-8"
          />
        </div>
      </div>

      {isLoading && (
        <div className="space-y-2">
          {[0, 1, 2, 3].map((i) => <Skeleton key={i} className="h-8 w-full" />)}
        </div>
      )}
      {isError && (
        <Alert variant="destructive">
          <AlertDescription>Erro ao carregar o relatório de fornecedores/clientes.</AlertDescription>
        </Alert>
      )}

      {data && (
        <div className="space-y-2">
          <div className="text-xs text-muted-foreground">{filtered.length} registro(s)</div>
          <div className="rounded-md border overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow className="bg-muted/30 hover:bg-transparent">
                  <TableHead className="text-xs font-semibold uppercase tracking-wide">Tipo</TableHead>
                  <TableHead className="text-xs font-semibold uppercase tracking-wide">CNPJ</TableHead>
                  <TableHead className="text-xs font-semibold uppercase tracking-wide">Razão Social (RFB)</TableHead>
                  <TableHead className="text-xs font-semibold uppercase tracking-wide">Situação</TableHead>
                  <TableHead className="text-xs font-semibold uppercase tracking-wide">CNAE</TableHead>
                  <TableHead className="text-xs font-semibold uppercase tracking-wide">Simples</TableHead>
                  <TableHead className="text-xs font-semibold uppercase tracking-wide">MEI</TableHead>
                  <TableHead className="text-xs font-semibold uppercase tracking-wide">UF</TableHead>
                  <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">Ano</TableHead>
                  <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">Valor Acumulado</TableHead>
                  <TableHead className="text-xs font-semibold uppercase tracking-wide text-right">Notas</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filtered.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={11} className="text-center text-xs text-muted-foreground py-6">
                      Nenhum registro encontrado
                    </TableCell>
                  </TableRow>
                ) : (
                  filtered.map((row, i) => (
                    <TableRow key={`${row.cnpj}-${row.tipo}-${row.ano}-${i}`}>
                      <TableCell className="text-xs">
                        <Badge variant="outline" className="text-[10px] px-1.5 py-0">
                          {row.tipo === 'fornecedor' ? 'Fornecedor' : 'Cliente'}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-xs font-mono">{row.cnpj}</TableCell>
                      <TableCell className="text-xs max-w-[220px]">
                        <div className="truncate" title={row.razao_social_rfb || row.nome_nota}>
                          {row.consultado_rfb ? (row.razao_social_rfb || row.nome_nota) : row.nome_nota}
                        </div>
                        {!row.consultado_rfb && (
                          <span className="text-[10px] text-amber-700">não consultado na RFB</span>
                        )}
                      </TableCell>
                      <TableCell className="text-xs">{situacaoBadge(row.situacao_cadastral, row.data_situacao_cadastral)}</TableCell>
                      <TableCell className="text-xs max-w-[220px]">
                        {row.cnae_codigo && (
                          <div className="truncate" title={row.cnae_descricao}>
                            <span className="font-mono">{row.cnae_codigo}</span> — {row.cnae_descricao}
                          </div>
                        )}
                      </TableCell>
                      <TableCell className="text-xs">{boolBadge(row.simples_nacional, 'Optante', 'Não optante')}</TableCell>
                      <TableCell className="text-xs">{boolBadge(row.mei, 'MEI', '—')}</TableCell>
                      <TableCell className="text-xs font-mono">{row.uf}</TableCell>
                      <TableCell className="text-xs text-right font-mono">{row.ano}</TableCell>
                      <TableCell className="text-xs text-right font-mono">{formatCurrency(row.valor_acumulado)}</TableCell>
                      <TableCell className="text-xs text-right font-mono">{row.qtd_notas}</TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>
        </div>
      )}
    </div>
  )
}
