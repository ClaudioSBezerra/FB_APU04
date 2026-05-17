import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { toast } from 'sonner';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from 'recharts';
import { Download, AlertTriangle, CheckCircle, FileSpreadsheet, Printer } from 'lucide-react';
import { exportToExcel } from '@/lib/exportToExcel';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------
interface DivergenciaRow {
  chave_nfe: string;
  forn_cnpj: string;
  forn_nome: string;
  mes_ano: string;
  data_emissao: string;
  cfop: string;
  xml_pis: number;
  xml_cofins: number;
  xml_icms: number;
  xml_ipi: number;
  xml_v_nf: number;
  bridge_pis: number;
  bridge_cofins: number;
  bridge_icms: number;
  bridge_ipi: number;
  delta_pis: number;
  delta_cofins: number;
  delta_icms: number;
  delta_ipi: number;
  delta_total: number;
}

interface CoberturaRow {
  mes_ano: string;
  total_nfes: number;
  com_xml: number;
  so_bridge: number;
  pct_xml: number;
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
function fmtBRL(v: number | null | undefined): string {
  if (v == null) return '—';
  return v.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' });
}

function fmtCNPJ(v: string): string {
  if (!v || v.length !== 14) return v || '—';
  return `${v.slice(0, 2)}.${v.slice(2, 5)}.${v.slice(5, 8)}/${v.slice(8, 12)}-${v.slice(12)}`;
}

function buildUrl(base: string, params: Record<string, string>): string {
  const q = new URLSearchParams(Object.entries(params).filter(([, v]) => v !== ''));
  return q.toString() ? `${base}?${q}` : base;
}

// ---------------------------------------------------------------------------
// ConciliacaoBridgeXML
// ---------------------------------------------------------------------------
export default function ConciliacaoBridgeXML() {
  const [mesAnoFiltro, setMesAnoFiltro] = useState('');
  const [mesAnoAtivo, setMesAnoAtivo] = useState('');
  const [tipo, setTipo] = useState<'entradas' | 'saidas'>('entradas');
  const [downloadingCSV, setDownloadingCSV] = useState(false);

  const {
    data: divergencias,
    isLoading: loadingDiv,
    isError: errorDiv,
    refetch: refetchDiv,
  } = useQuery<DivergenciaRow[]>({
    queryKey: ['xml-conciliacao', mesAnoAtivo, tipo],
    queryFn: async () => {
      const res = await fetch(
        buildUrl('/api/xml/conciliacao', { mes_ano: mesAnoAtivo, tipo })
      );
      if (!res.ok) throw new Error(`Erro ${res.status}`);
      return res.json();
    },
  });

  const {
    data: cobertura,
    isLoading: loadingCob,
    isError: errorCob,
    refetch: refetchCob,
  } = useQuery<CoberturaRow[]>({
    queryKey: ['xml-cobertura', tipo],
    queryFn: async () => {
      const res = await fetch(buildUrl('/api/xml/cobertura', { tipo }));
      if (!res.ok) throw new Error(`Erro ${res.status}`);
      return res.json();
    },
  });

  const handleBuscar = () => {
    setMesAnoAtivo(mesAnoFiltro.trim());
  };

  const handleLimpar = () => {
    setMesAnoFiltro('');
    setMesAnoAtivo('');
  };

  const handleExportExcel = () => {
    if (!divergencias) return;
    const data = divergencias.map(r => ({
      'Chave NF-e':      r.chave_nfe,
      'CNPJ Fornecedor': r.forn_cnpj,
      'Fornecedor':      r.forn_nome,
      'Mês/Ano':         r.mes_ano,
      'Data Emissão':    r.data_emissao,
      'CFOP':            r.cfop,
      'PIS XML':         r.xml_pis   ?? 0,
      'PIS Bridge':      r.bridge_pis ?? 0,
      'Delta PIS':       r.delta_pis  ?? 0,
      'COFINS XML':      r.xml_cofins   ?? 0,
      'COFINS Bridge':   r.bridge_cofins ?? 0,
      'Delta COFINS':    r.delta_cofins  ?? 0,
      'ICMS XML':        r.xml_icms   ?? 0,
      'ICMS Bridge':     r.bridge_icms ?? 0,
      'Delta ICMS':      r.delta_icms  ?? 0,
      'IPI XML':         r.xml_ipi   ?? 0,
      'IPI Bridge':      r.bridge_ipi ?? 0,
      'Delta Total':     r.delta_total ?? 0,
    }));
    exportToExcel(data, `conciliacao-bridge-xml-${mesAnoAtivo || 'geral'}`, 'Divergências');
    toast.success('Excel exportado com sucesso');
  };

  const handleExportCSV = async () => {
    setDownloadingCSV(true);
    try {
      const res = await fetch(
        buildUrl('/api/xml/conciliacao/csv', { mes_ano: mesAnoAtivo, tipo })
      );
      if (!res.ok) throw new Error(`Erro ${res.status}`);
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `conciliacao-bridge-xml.csv`;
      a.click();
      URL.revokeObjectURL(url);
      toast.success('CSV exportado com sucesso');
    } catch (err) {
      toast.error('Erro ao exportar: ' + (err instanceof Error ? err.message : 'Desconhecido'));
    } finally {
      setDownloadingCSV(false);
    }
  };

  // Métricas de resumo calculadas dos dados das queries
  const totalDivergencias = divergencias?.length ?? 0;
  const deltaTotal = divergencias
    ? divergencias.reduce((acc, r) => acc + (r.delta_total ?? 0), 0)
    : 0;
  const pctXml = cobertura && cobertura.length > 0 ? cobertura[0].pct_xml : null;

  return (
    <div className="space-y-6">
      {/* Heading */}
      <div>
        <h1 className="text-xl font-semibold">Conciliação Bridge vs XML</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Compare os valores tributários do ERP Bridge com os documentos fiscais SEFAZ
          para identificar divergências e medir a cobertura de autenticidade.
        </p>
      </div>

      {/* Cards de resumo — 3 colunas */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-semibold text-muted-foreground">
              NF-es com divergência
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-xl font-semibold">{loadingDiv ? '...' : totalDivergencias}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-semibold text-muted-foreground">
              Delta tributário total
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-xl font-semibold">{loadingDiv ? '...' : fmtBRL(deltaTotal)}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-semibold text-muted-foreground">
              Cobertura XML (entradas)
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-xl font-semibold">
              {loadingCob ? '...' : pctXml != null ? `${pctXml.toFixed(1)}%` : '—'}
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Tabs principais */}
      <Tabs defaultValue="divergencias">
        <TabsList>
          <TabsTrigger value="divergencias">Divergências</TabsTrigger>
          <TabsTrigger value="cobertura">Cobertura XML</TabsTrigger>
        </TabsList>

        {/* Tab: Divergências */}
        <TabsContent value="divergencias">
          {/* Filtros */}
          <div className="flex items-center gap-3 flex-wrap mb-4">
            <div className="flex items-center gap-2">
              <label className="text-xs text-muted-foreground whitespace-nowrap">Mês/Ano</label>
              <Input
                type="text"
                placeholder="MM/YYYY"
                value={mesAnoFiltro}
                onChange={e => setMesAnoFiltro(e.target.value)}
                className="h-8 w-28 text-sm"
              />
            </div>
            <Select value={tipo} onValueChange={v => setTipo(v as 'entradas' | 'saidas')}>
              <SelectTrigger className="h-8 w-40">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="entradas">NF-e Entradas</SelectItem>
                <SelectItem value="saidas">NF-e Saídas</SelectItem>
              </SelectContent>
            </Select>
            <Button size="sm" onClick={handleBuscar} disabled={loadingDiv}>
              {loadingDiv ? 'Buscando...' : 'Buscar Divergências'}
            </Button>
            {mesAnoAtivo && (
              <Button
                size="sm"
                variant="ghost"
                className="text-xs text-muted-foreground"
                onClick={handleLimpar}
              >
                Limpar
              </Button>
            )}
          </div>

          {/* Estados: loading / error / vazio / dados */}
          {loadingDiv ? (
            <p className="text-sm text-muted-foreground text-center py-8">
              Carregando divergências...
            </p>
          ) : errorDiv ? (
            <p className="text-sm text-destructive px-4 py-6">
              Erro ao carregar dados de conciliação.{' '}
              <button className="underline" onClick={() => refetchDiv()}>Tentar novamente</button>
            </p>
          ) : !divergencias || divergencias.length === 0 ? (
            <div className="flex items-center gap-2 px-4 py-6 text-sm text-muted-foreground">
              <CheckCircle className="w-4 h-4 text-green-500" />
              Nenhuma divergência encontrada. Todas as NF-es com origem XML têm valores
              tributários compatíveis com o ERP Bridge no período selecionado.
            </div>
          ) : (
            <div className="overflow-x-auto rounded-md border">
              <Table>
                <TableHeader>
                  <TableRow className="hover:bg-transparent bg-muted/30">
                    <TableHead className="py-1.5 px-2 text-[11px]">Fornecedor</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px]">Mês/Ano</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px]">Data Emissão</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px] text-right">PIS XML</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px] text-right">PIS Bridge</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px] text-right">Delta PIS</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px] text-right">COFINS XML</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px] text-right">COFINS Bridge</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px] text-right">Delta COFINS</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px] text-right">ICMS XML</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px] text-right">ICMS Bridge</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px] text-right">Delta ICMS</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px] text-right">Delta Total</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {divergencias.map((row, idx) => (
                    <TableRow
                      key={`${row.chave_nfe}-${idx}`}
                      className={row.delta_total > 0.01 ? 'bg-red-50 hover:bg-red-100' : ''}
                    >
                      {/* Fornecedor — dois níveis: nome + CNPJ */}
                      <TableCell className="py-1 px-2">
                        <div className="text-[11px] font-medium leading-tight truncate max-w-[180px]">
                          {row.forn_nome || '—'}
                        </div>
                        <div className="text-[10px] text-muted-foreground font-mono leading-tight">
                          {fmtCNPJ(row.forn_cnpj)}
                        </div>
                      </TableCell>
                      <TableCell className="py-1 px-2 text-[11px] whitespace-nowrap">{row.mes_ano}</TableCell>
                      <TableCell className="py-1 px-2 text-[11px] whitespace-nowrap">{row.data_emissao}</TableCell>
                      {/* PIS */}
                      <TableCell className="py-1 px-2 text-right text-[11px] font-semibold">{fmtBRL(row.xml_pis)}</TableCell>
                      <TableCell className="py-1 px-2 text-right text-[11px] text-muted-foreground">{fmtBRL(row.bridge_pis)}</TableCell>
                      <TableCell className="py-1 px-2 text-right">
                        <Badge
                          variant="outline"
                          className={row.delta_pis > 0.01
                            ? 'text-[10px] px-1.5 py-0 bg-red-50 text-red-700 border-red-200'
                            : 'text-[10px] px-1.5 py-0 bg-gray-50 text-muted-foreground'}
                        >
                          {fmtBRL(row.delta_pis)}
                        </Badge>
                      </TableCell>
                      {/* COFINS */}
                      <TableCell className="py-1 px-2 text-right text-[11px] font-semibold">{fmtBRL(row.xml_cofins)}</TableCell>
                      <TableCell className="py-1 px-2 text-right text-[11px] text-muted-foreground">{fmtBRL(row.bridge_cofins)}</TableCell>
                      <TableCell className="py-1 px-2 text-right">
                        <Badge
                          variant="outline"
                          className={row.delta_cofins > 0.01
                            ? 'text-[10px] px-1.5 py-0 bg-red-50 text-red-700 border-red-200'
                            : 'text-[10px] px-1.5 py-0 bg-gray-50 text-muted-foreground'}
                        >
                          {fmtBRL(row.delta_cofins)}
                        </Badge>
                      </TableCell>
                      {/* ICMS */}
                      <TableCell className="py-1 px-2 text-right text-[11px] font-semibold">{fmtBRL(row.xml_icms)}</TableCell>
                      <TableCell className="py-1 px-2 text-right text-[11px] text-muted-foreground">{fmtBRL(row.bridge_icms)}</TableCell>
                      <TableCell className="py-1 px-2 text-right">
                        <Badge
                          variant="outline"
                          className={row.delta_icms > 0.01
                            ? 'text-[10px] px-1.5 py-0 bg-red-50 text-red-700 border-red-200'
                            : 'text-[10px] px-1.5 py-0 bg-gray-50 text-muted-foreground'}
                        >
                          {fmtBRL(row.delta_icms)}
                        </Badge>
                      </TableCell>
                      {/* Delta Total */}
                      <TableCell className="py-1 px-2 text-right text-[11px] font-semibold">
                        {fmtBRL(row.delta_total)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}

          {/* Botões de exportação — abaixo da tabela, no-print */}
          <div className="flex items-center gap-2 mt-4 no-print">
            <Button size="sm" variant="outline" onClick={handleExportExcel}>
              <FileSpreadsheet className="w-4 h-4 mr-1" /> Exportar Excel
            </Button>
            <Button size="sm" variant="outline" onClick={handleExportCSV} disabled={downloadingCSV}>
              <Download className="w-4 h-4 mr-1" />
              {downloadingCSV ? 'Exportando...' : 'Exportar CSV'}
            </Button>
            <Button size="sm" variant="ghost" onClick={() => window.print()}>
              <Printer className="w-4 h-4 mr-1" /> Imprimir PDF
            </Button>
          </div>

          {/* Legenda do threshold */}
          <p className="text-[11px] text-muted-foreground mt-2">
            (divergências &gt; R$ 0,01)
          </p>
        </TabsContent>

        {/* Tab: Cobertura XML */}
        <TabsContent value="cobertura">
          {loadingCob ? (
            <p className="text-sm text-muted-foreground text-center py-8">
              Carregando cobertura...
            </p>
          ) : errorCob ? (
            <p className="text-sm text-destructive px-4 py-6">
              Erro ao carregar dados de cobertura.{' '}
              <button className="underline" onClick={() => refetchCob()}>Tentar novamente</button>
            </p>
          ) : (
            <>
              {/* Gráfico de barras */}
              <ResponsiveContainer width="100%" height={300} minWidth={0}>
                <BarChart data={cobertura ?? []}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis dataKey="mes_ano" tick={{ fontSize: 11 }} />
                  <YAxis tickFormatter={(v) => `${v}%`} tick={{ fontSize: 11 }} />
                  <Tooltip formatter={(v) => `${Number(v).toFixed(1)}%`} />
                  <Legend />
                  <Bar dataKey="pct_xml" name="XML (Autêntico)" fill="#22c55e" stackId="a" />
                </BarChart>
              </ResponsiveContainer>

              {/* Tabela de cobertura por mês */}
              <div className="overflow-x-auto rounded-md border mt-4">
                <Table>
                  <TableHeader>
                    <TableRow className="hover:bg-transparent bg-muted/30">
                      <TableHead className="py-1.5 px-2 text-[11px]">Mês/Ano</TableHead>
                      <TableHead className="py-1.5 px-2 text-[11px] text-right">Total NF-es</TableHead>
                      <TableHead className="py-1.5 px-2 text-[11px] text-right">Com XML</TableHead>
                      <TableHead className="py-1.5 px-2 text-[11px] text-right">Só Bridge</TableHead>
                      <TableHead className="py-1.5 px-2 text-[11px] text-right">% Cobertura XML</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {(cobertura ?? []).map((row, idx) => (
                      <TableRow key={`${row.mes_ano}-${idx}`}>
                        <TableCell className="py-1 px-2 text-[11px]">{row.mes_ano}</TableCell>
                        <TableCell className="py-1 px-2 text-right text-[11px]">
                          {row.total_nfes.toLocaleString('pt-BR')}
                        </TableCell>
                        <TableCell className="py-1 px-2 text-right">
                          <Badge variant="outline" className="text-[10px] px-1.5 py-0 bg-green-50 text-green-700 border-green-200">
                            {row.com_xml.toLocaleString('pt-BR')}
                          </Badge>
                        </TableCell>
                        <TableCell className="py-1 px-2 text-right">
                          <Badge variant="outline" className="text-[10px] px-1.5 py-0 bg-blue-50 text-blue-700 border-blue-200">
                            {row.so_bridge.toLocaleString('pt-BR')}
                          </Badge>
                        </TableCell>
                        <TableCell className="py-1 px-2 text-right text-[11px]">
                          <span className={row.pct_xml > 80 ? 'font-semibold' : ''}>
                            {row.pct_xml.toFixed(1)}%
                          </span>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>

              {/* Footnote obrigatória */}
              <p className="text-[11px] text-muted-foreground mt-2">
                Notas canceladas excluídas da contagem.
              </p>
            </>
          )}
        </TabsContent>
      </Tabs>
    </div>
  );
}
