import { useState } from 'react';
import * as XLSX from 'xlsx';
import { useQuery } from '@tanstack/react-query';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
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
  BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip as RechartsTooltip,
  Legend, ResponsiveContainer, Cell,
} from 'recharts';
import { AlertTriangle, CheckCircle, FileBarChart, RefreshCw, Info, Download, Loader2 } from 'lucide-react';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------
interface ResumoRow {
  mes_ano: string;
  qtd_efd: number;
  qtd_xml: number;
  total_efd: number;
  total_xml: number;
  diferenca: number;
  pct_cobertura: number;
}

interface LacunaRow {
  mes_ano: string;
  chv_nfe: string;
  num_doc: string;
  dt_doc: string;
  cod_mod: string;
  cod_sit: string;
  vl_doc: number;
}

interface LacunaMensalRow {
  mes_ano: string;
  qtd_falta: number;
  valor_falta: number;
}

interface ModeloRow {
  cod_mod: string;
  descricao: string;
  ind_oper: string;
  qtd: number;
  total: number;
}

interface LacunaExportRow {
  mes_ano: string;
  filial_cnpj: string;
  cod_part: string;
  ser: string;
  num_doc: string;
  chv_nfe: string;
  dt_doc: string;
  dt_e_s: string;
  cod_mod: string;
  cod_sit: string;
  cfops: string;
  vl_doc: number;
  vl_icms: number;
  vl_bc_icms: number;
  vl_pis: number;
  vl_cofins: number;
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
function fmtBRL(v: number | null | undefined): string {
  if (v == null) return '—';
  return v.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL', maximumFractionDigits: 2 });
}

function fmtPct(v: number): string {
  return `${v.toFixed(1)}%`;
}

function buildUrl(base: string, params: Record<string, string>): string {
  const q = new URLSearchParams(Object.entries(params).filter(([, v]) => v !== ''));
  return q.toString() ? `${base}?${q}` : base;
}

function CoberturaChip({ pct }: { pct: number }) {
  if (pct >= 95) return <Badge className="bg-green-600 text-white text-[10px]">{fmtPct(pct)}</Badge>;
  if (pct >= 70) return <Badge className="bg-yellow-500 text-white text-[10px]">{fmtPct(pct)}</Badge>;
  return <Badge variant="destructive" className="text-[10px]">{fmtPct(pct)}</Badge>;
}

const SIT_LABELS: Record<string, string> = {
  '00': 'Regular',
  '01': 'Regular Extemporânea',
  '06': 'Complementar',
  '07': 'Complementar Extemporânea',
  '08': 'Regime Especial',
};

function SitChip({ sit }: { sit: string }) {
  return <span className="text-muted-foreground text-[10px]">{SIT_LABELS[sit] ?? sit}</span>;
}

// ---------------------------------------------------------------------------
// Excel export helper
// ---------------------------------------------------------------------------
async function downloadLacunasExcel(
  tipo: 'saidas' | 'entradas',
  mesAno?: string,
  onStart?: () => void,
  onEnd?: () => void,
) {
  onStart?.();
  try {
    const url = buildUrl('/api/xml/comparativo/lacunas/export', {
      tipo,
      ...(mesAno ? { mes_ano: mesAno } : {}),
    });
    const res = await fetch(url);
    if (!res.ok) throw new Error(res.statusText);
    const data = await res.json();
    const rows: LacunaExportRow[] = data.items ?? [];

    const headers = [
      'Período',
      'Filial CNPJ',
      'Cód. Parceiro',
      'Série',
      'Nº NF',
      'Data Emissão',
      'Data Entrada/Saída',
      'Modelo',
      'Situação',
      'CFOPs',
      'Valor Total (R$)',
      'Base ICMS (R$)',
      'Valor ICMS (R$)',
      'Valor PIS (R$)',
      'Valor COFINS (R$)',
      'Chave NF-e',
    ];

    const wsData = [
      headers,
      ...rows.map(r => [
        r.mes_ano,
        r.filial_cnpj,
        r.cod_part,
        r.ser,
        r.num_doc,
        r.dt_doc,
        r.dt_e_s,
        r.cod_mod,
        r.cod_sit ? `${r.cod_sit} – ${SIT_LABELS[r.cod_sit] ?? r.cod_sit}` : '',
        r.cfops,
        r.vl_doc,
        r.vl_bc_icms,
        r.vl_icms,
        r.vl_pis,
        r.vl_cofins,
        r.chv_nfe,
      ]),
    ];

    const wb = XLSX.utils.book_new();
    const ws = XLSX.utils.aoa_to_sheet(wsData);

    ws['!cols'] = [
      { wch: 10 }, // Período
      { wch: 16 }, // Filial CNPJ
      { wch: 22 }, // Cód. Parceiro
      { wch: 7 },  // Série
      { wch: 12 }, // Nº NF
      { wch: 14 }, // Data Emissão
      { wch: 18 }, // Data Entrada/Saída
      { wch: 8 },  // Modelo
      { wch: 28 }, // Situação
      { wch: 16 }, // CFOPs
      { wch: 18 }, // Valor Total
      { wch: 16 }, // Base ICMS
      { wch: 16 }, // Valor ICMS
      { wch: 14 }, // Valor PIS
      { wch: 16 }, // Valor COFINS
      { wch: 48 }, // Chave NF-e
    ];

    // Format currency columns (J:O → indices 10-14) as numbers
    for (let row = 1; row < wsData.length; row++) {
      const cols = [10, 11, 12, 13, 14];
      for (const col of cols) {
        const cellRef = XLSX.utils.encode_cell({ r: row, c: col });
        if (ws[cellRef]) ws[cellRef].t = 'n';
      }
    }

    const label = tipo === 'saidas' ? 'Saidas' : 'Entradas';
    XLSX.utils.book_append_sheet(wb, ws, `Lacunas ${label}`);

    const filename = mesAno
      ? `lacunas_${tipo}_${mesAno.replace('/', '-')}.xlsx`
      : `lacunas_${tipo}_todos_periodos.xlsx`;

    XLSX.writeFile(wb, filename);
  } finally {
    onEnd?.();
  }
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------
export default function ComparativoEFDvsXML() {
  const [tipo, setTipo] = useState<'saidas' | 'entradas'>('saidas');
  const [mesSelecionado, setMesSelecionado] = useState<string | null>(null);
  const [exportingAll, setExportingAll] = useState(false);
  const [exportingMonth, setExportingMonth] = useState<string | null>(null);

  // ── Queries ──────────────────────────────────────────────────────────────
  const { data: resumoData, isLoading: loadingResumo, refetch: refetchResumo } = useQuery<{ items: ResumoRow[] }>({
    queryKey: ['comparativo-resumo', tipo],
    queryFn: async () => {
      const res = await fetch(buildUrl('/api/xml/comparativo/resumo', { tipo }));
      if (!res.ok) throw new Error(res.statusText);
      return res.json();
    },
  });

  const { data: lacunasMensalData, isLoading: loadingLacunaMensal } = useQuery<{ items: LacunaMensalRow[] }>({
    queryKey: ['comparativo-lacunas-mensal', tipo],
    queryFn: async () => {
      const res = await fetch(buildUrl('/api/xml/comparativo/lacunas/mensal', { tipo }));
      if (!res.ok) throw new Error(res.statusText);
      return res.json();
    },
  });

  const { data: lacunasData, isLoading: loadingLacunas } = useQuery<{ items: LacunaRow[]; total: number }>({
    queryKey: ['comparativo-lacunas-detalhe', tipo, mesSelecionado],
    queryFn: async () => {
      const res = await fetch(buildUrl('/api/xml/comparativo/lacunas', { tipo, mes_ano: mesSelecionado! }));
      if (!res.ok) throw new Error(res.statusText);
      return res.json();
    },
    enabled: !!mesSelecionado,
  });

  const { data: modelosData, isLoading: loadingModelos } = useQuery<{ items: ModeloRow[] }>({
    queryKey: ['comparativo-modelos'],
    queryFn: async () => {
      const res = await fetch('/api/xml/comparativo/modelos');
      if (!res.ok) throw new Error(res.statusText);
      return res.json();
    },
  });

  // ── Derived ───────────────────────────────────────────────────────────────
  const resumo = resumoData?.items ?? [];
  const lacunasMensal = lacunasMensalData?.items ?? [];
  const lacunas = lacunasData?.items ?? [];
  const modelos = modelosData?.items ?? [];

  const totalEFD  = resumo.reduce((s, r) => s + r.total_efd, 0);
  const totalXML  = resumo.reduce((s, r) => s + r.total_xml, 0);
  const totalDiff = totalEFD - totalXML;
  const pctGeral  = totalEFD > 0 ? (totalXML / totalEFD) * 100 : 0;
  const totalFalta = lacunasMensal.reduce((s, r) => s + r.valor_falta, 0);

  const chartData = resumo.map(r => ({
    mes: r.mes_ano,
    EFD: parseFloat((r.total_efd / 1_000_000).toFixed(2)),
    XML: parseFloat((r.total_xml / 1_000_000).toFixed(2)),
  }));

  const modelosSaida   = modelos.filter(m => m.ind_oper === '1');
  const modelosEntrada = modelos.filter(m => m.ind_oper === '0');

  const handleExportAll = () =>
    downloadLacunasExcel(tipo, undefined, () => setExportingAll(true), () => setExportingAll(false));

  const handleExportMonth = (mes: string) =>
    downloadLacunasExcel(tipo, mes, () => setExportingMonth(mes), () => setExportingMonth(null));

  return (
    <div className="space-y-6">
      {/* ── Cabeçalho ── */}
      <div className="flex items-start justify-between flex-wrap gap-3">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Comparativo EFD ICMS vs XMLs</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Cruza os documentos do EFD ICMS (reg_c100) com os XMLs importados para identificar lacunas.
          </p>
        </div>
        <Select value={tipo} onValueChange={v => setTipo(v as 'saidas' | 'entradas')}>
          <SelectTrigger className="w-44 h-8 text-sm">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="saidas">Saídas</SelectItem>
            <SelectItem value="entradas">Entradas</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {/* ── Cards de resumo ── */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <Card>
          <CardContent className="pt-4 pb-3">
            <p className="text-[11px] text-muted-foreground uppercase tracking-wide">Total EFD</p>
            <p className="text-lg font-bold mt-1">{fmtBRL(totalEFD)}</p>
            <p className="text-[10px] text-muted-foreground">NF-e mod 55/65 no SPED</p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-4 pb-3">
            <p className="text-[11px] text-muted-foreground uppercase tracking-wide">Total XML</p>
            <p className="text-lg font-bold mt-1">{fmtBRL(totalXML)}</p>
            <p className="text-[10px] text-muted-foreground">XMLs importados</p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-4 pb-3">
            <p className="text-[11px] text-muted-foreground uppercase tracking-wide">Diferença</p>
            <p className={`text-lg font-bold mt-1 ${totalDiff > 0 ? 'text-red-600' : totalDiff < 0 ? 'text-orange-500' : 'text-green-600'}`}>
              {fmtBRL(totalDiff)}
            </p>
            <p className="text-[10px] text-muted-foreground">EFD − XML</p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-4 pb-3">
            <p className="text-[11px] text-muted-foreground uppercase tracking-wide">Cobertura XML</p>
            <p className={`text-lg font-bold mt-1 ${pctGeral >= 95 ? 'text-green-600' : pctGeral >= 70 ? 'text-yellow-600' : 'text-red-600'}`}>
              {fmtPct(pctGeral)}
            </p>
            <p className="text-[10px] text-muted-foreground">% do EFD coberto</p>
          </CardContent>
        </Card>
      </div>

      {/* ── Tabs ── */}
      <Tabs defaultValue="resumo">
        <TabsList>
          <TabsTrigger value="resumo"><FileBarChart className="h-3.5 w-3.5 mr-1.5" />Por Mês</TabsTrigger>
          <TabsTrigger value="lacunas"><AlertTriangle className="h-3.5 w-3.5 mr-1.5" />Lacunas</TabsTrigger>
          <TabsTrigger value="modelos"><CheckCircle className="h-3.5 w-3.5 mr-1.5" />Modelos EFD</TabsTrigger>
        </TabsList>

        {/* ── Tab: Resumo por Mês ── */}
        <TabsContent value="resumo" className="space-y-4 mt-4">
          {chartData.length > 0 && (
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm">Faturamento Mensal: EFD vs XML (R$ milhões)</CardTitle>
              </CardHeader>
              <CardContent>
                <ResponsiveContainer width="100%" height={220}>
                  <BarChart data={chartData} margin={{ top: 4, right: 8, bottom: 4, left: 8 }}>
                    <CartesianGrid strokeDasharray="3 3" stroke="#e5e7eb" />
                    <XAxis dataKey="mes" tick={{ fontSize: 11 }} />
                    <YAxis tick={{ fontSize: 11 }} tickFormatter={v => `R$${v}M`} />
                    <RechartsTooltip
                      formatter={(v: number | undefined) => v != null ? `R$ ${v.toLocaleString('pt-BR')}M` : '—'}
                      contentStyle={{ fontSize: 12 }}
                    />
                    <Legend wrapperStyle={{ fontSize: 11 }} />
                    <Bar dataKey="EFD" fill="#94a3b8" radius={[3,3,0,0]} />
                    <Bar dataKey="XML" radius={[3,3,0,0]}>
                      {chartData.map((entry, idx) => (
                        <Cell key={idx} fill={entry.XML >= entry.EFD * 0.95 ? '#16a34a' : entry.XML >= entry.EFD * 0.70 ? '#ca8a04' : '#dc2626'} />
                      ))}
                    </Bar>
                  </BarChart>
                </ResponsiveContainer>
              </CardContent>
            </Card>
          )}

          <Card>
            <CardHeader className="pb-2 flex flex-row items-center justify-between">
              <CardTitle className="text-sm">Detalhe Mensal</CardTitle>
              <Button variant="ghost" size="sm" onClick={() => refetchResumo()} className="h-7 px-2">
                <RefreshCw className="h-3.5 w-3.5" />
              </Button>
            </CardHeader>
            <CardContent className="p-0">
              {loadingResumo ? (
                <p className="text-sm text-center py-8 text-muted-foreground">Carregando...</p>
              ) : resumo.length === 0 ? (
                <p className="text-sm text-center py-8 text-muted-foreground">
                  Nenhum dado no EFD para {tipo === 'saidas' ? 'saídas' : 'entradas'}.
                </p>
              ) : (
                <div className="overflow-x-auto">
                  <Table>
                    <TableHeader>
                      <TableRow className="hover:bg-transparent">
                        <TableHead className="py-2 px-3 text-[11px]">Mês</TableHead>
                        <TableHead className="py-2 px-3 text-[11px] text-right">Qtd EFD</TableHead>
                        <TableHead className="py-2 px-3 text-[11px] text-right">Qtd XML</TableHead>
                        <TableHead className="py-2 px-3 text-[11px] text-right">Total EFD</TableHead>
                        <TableHead className="py-2 px-3 text-[11px] text-right">Total XML</TableHead>
                        <TableHead className="py-2 px-3 text-[11px] text-right">Diferença</TableHead>
                        <TableHead className="py-2 px-3 text-[11px] text-center">Cobertura</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {resumo.map(row => (
                        <TableRow key={row.mes_ano} className="h-9">
                          <TableCell className="py-1 px-3 text-[12px] font-medium">{row.mes_ano}</TableCell>
                          <TableCell className="py-1 px-3 text-[11px] text-right tabular-nums">{row.qtd_efd.toLocaleString('pt-BR')}</TableCell>
                          <TableCell className="py-1 px-3 text-[11px] text-right tabular-nums">{row.qtd_xml.toLocaleString('pt-BR')}</TableCell>
                          <TableCell className="py-1 px-3 text-[11px] text-right tabular-nums">{fmtBRL(row.total_efd)}</TableCell>
                          <TableCell className="py-1 px-3 text-[11px] text-right tabular-nums">{fmtBRL(row.total_xml)}</TableCell>
                          <TableCell className={`py-1 px-3 text-[11px] text-right tabular-nums font-medium ${row.diferenca > 0 ? 'text-red-600' : row.diferenca < 0 ? 'text-orange-500' : 'text-green-600'}`}>
                            {fmtBRL(row.diferenca)}
                          </TableCell>
                          <TableCell className="py-1 px-3 text-center">
                            <CoberturaChip pct={row.pct_cobertura} />
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        {/* ── Tab: Lacunas ── */}
        <TabsContent value="lacunas" className="space-y-4 mt-4">
          {/* Resumo mensal */}
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm flex items-center justify-between flex-wrap gap-2">
                <span>NF-e no EFD sem XML importado — por mês</span>
                <div className="flex items-center gap-2">
                  {totalFalta > 0 && (
                    <span className="text-red-600 text-xs font-normal">
                      Total em falta: {fmtBRL(totalFalta)}
                    </span>
                  )}
                  {lacunasMensal.length > 0 && (
                    <Button
                      size="sm"
                      variant="outline"
                      className="h-7 px-2 text-[11px] gap-1.5"
                      onClick={handleExportAll}
                      disabled={exportingAll}
                    >
                      {exportingAll
                        ? <Loader2 className="h-3.5 w-3.5 animate-spin" />
                        : <Download className="h-3.5 w-3.5" />}
                      Exportar todos os períodos
                    </Button>
                  )}
                </div>
              </CardTitle>
            </CardHeader>
            <CardContent className="p-0">
              {loadingLacunaMensal ? (
                <p className="text-sm text-center py-6 text-muted-foreground">Carregando resumo...</p>
              ) : lacunasMensal.length === 0 ? (
                <div className="flex flex-col items-center gap-2 py-6 text-muted-foreground">
                  <CheckCircle className="h-7 w-7 text-green-500" />
                  <p className="text-sm">Nenhuma lacuna encontrada — todos os XMLs foram importados.</p>
                </div>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow className="hover:bg-transparent">
                      <TableHead className="py-1.5 px-3 text-[11px]">Mês</TableHead>
                      <TableHead className="py-1.5 px-3 text-[11px] text-right">NF-e faltando</TableHead>
                      <TableHead className="py-1.5 px-3 text-[11px] text-right">Valor em falta</TableHead>
                      <TableHead className="py-1.5 px-3 text-[11px] text-center" colSpan={2}>Ações</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {lacunasMensal.map(row => (
                      <TableRow key={row.mes_ano} className="h-9">
                        <TableCell className="py-1 px-3 text-[12px] font-medium">{row.mes_ano}</TableCell>
                        <TableCell className="py-1 px-3 text-[11px] text-right tabular-nums text-red-600 font-medium">
                          {row.qtd_falta.toLocaleString('pt-BR')}
                        </TableCell>
                        <TableCell className="py-1 px-3 text-[11px] text-right tabular-nums text-red-600 font-medium">
                          {fmtBRL(row.valor_falta)}
                        </TableCell>
                        <TableCell className="py-1 px-3 text-center">
                          <Button
                            size="sm"
                            variant={mesSelecionado === row.mes_ano ? 'default' : 'outline'}
                            className="h-6 px-2 text-[10px]"
                            onClick={() => setMesSelecionado(mesSelecionado === row.mes_ano ? null : row.mes_ano)}
                          >
                            {mesSelecionado === row.mes_ano ? 'Fechar' : 'Ver notas'}
                          </Button>
                        </TableCell>
                        <TableCell className="py-1 px-2 text-center">
                          <Button
                            size="sm"
                            variant="ghost"
                            className="h-6 w-6 p-0"
                            title={`Exportar Excel — ${row.mes_ano}`}
                            onClick={() => handleExportMonth(row.mes_ano)}
                            disabled={exportingMonth === row.mes_ano}
                          >
                            {exportingMonth === row.mes_ano
                              ? <Loader2 className="h-3.5 w-3.5 animate-spin" />
                              : <Download className="h-3.5 w-3.5" />}
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>

          {/* Detalhe do mês selecionado */}
          {mesSelecionado && (
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm flex items-center gap-2 flex-wrap">
                  <AlertTriangle className="h-4 w-4 text-red-500" />
                  <span>NF-e sem XML — {mesSelecionado}</span>
                  {lacunasData?.total === 500 && (
                    <Badge variant="outline" className="text-[10px]">limite 500 notas</Badge>
                  )}
                  <div className="ml-auto">
                    <Button
                      size="sm"
                      variant="outline"
                      className="h-7 px-2 text-[11px] gap-1.5"
                      onClick={() => handleExportMonth(mesSelecionado)}
                      disabled={exportingMonth === mesSelecionado}
                    >
                      {exportingMonth === mesSelecionado
                        ? <Loader2 className="h-3.5 w-3.5 animate-spin" />
                        : <Download className="h-3.5 w-3.5" />}
                      Exportar Excel
                    </Button>
                  </div>
                </CardTitle>
              </CardHeader>
              <CardContent className="p-0">
                {loadingLacunas ? (
                  <p className="text-sm text-center py-6 text-muted-foreground">Buscando notas...</p>
                ) : lacunas.length === 0 ? (
                  <p className="text-sm text-center py-6 text-muted-foreground">Nenhuma lacuna neste mês.</p>
                ) : (
                  <div className="overflow-x-auto">
                    <Table>
                      <TableHeader>
                        <TableRow className="hover:bg-transparent">
                          <TableHead className="py-1.5 px-2 text-[11px]">Nº NF</TableHead>
                          <TableHead className="py-1.5 px-2 text-[11px]">Data</TableHead>
                          <TableHead className="py-1.5 px-2 text-[11px]">Mod</TableHead>
                          <TableHead className="py-1.5 px-2 text-[11px]">Situação</TableHead>
                          <TableHead className="py-1.5 px-2 text-[11px] text-right">Valor</TableHead>
                          <TableHead className="py-1.5 px-2 text-[11px]">Chave</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {lacunas.map((row, i) => (
                          <TableRow key={i} className="h-8">
                            <TableCell className="py-1 px-2 text-[11px] font-medium">{row.num_doc || '—'}</TableCell>
                            <TableCell className="py-1 px-2 text-[11px] whitespace-nowrap">{row.dt_doc}</TableCell>
                            <TableCell className="py-1 px-2 text-[11px]">{row.cod_mod}</TableCell>
                            <TableCell className="py-1 px-2"><SitChip sit={row.cod_sit} /></TableCell>
                            <TableCell className="py-1 px-2 text-[11px] text-right tabular-nums font-medium">{fmtBRL(row.vl_doc)}</TableCell>
                            <TableCell className="py-1 px-2 text-[10px] font-mono text-muted-foreground max-w-[160px] truncate" title={row.chv_nfe}>{row.chv_nfe || '—'}</TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>
                )}
              </CardContent>
            </Card>
          )}
        </TabsContent>

        {/* ── Tab: Modelos EFD ── */}
        <TabsContent value="modelos" className="space-y-4 mt-4">
          <div className="grid md:grid-cols-2 gap-4">
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm">Modelos de Documento — Saídas (EFD)</CardTitle>
              </CardHeader>
              <CardContent className="p-0">
                {loadingModelos ? (
                  <p className="text-sm text-center py-6 text-muted-foreground">Carregando...</p>
                ) : modelosSaida.length === 0 ? (
                  <p className="text-sm text-center py-6 text-muted-foreground">Sem dados de saída no EFD.</p>
                ) : (
                  <Table>
                    <TableHeader>
                      <TableRow className="hover:bg-transparent">
                        <TableHead className="py-1.5 px-3 text-[11px]">Modelo</TableHead>
                        <TableHead className="py-1.5 px-3 text-[11px]">Descrição</TableHead>
                        <TableHead className="py-1.5 px-3 text-[11px] text-right">Qtd</TableHead>
                        <TableHead className="py-1.5 px-3 text-[11px] text-right">Total</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {modelosSaida.map(row => (
                        <TableRow key={row.cod_mod} className="h-8">
                          <TableCell className="py-1 px-3 text-[11px] font-mono font-bold">{row.cod_mod}</TableCell>
                          <TableCell className="py-1 px-3 text-[11px]">
                            <span>{row.descricao}</span>
                            {['55','65','57','58'].includes(row.cod_mod) && (
                              <Badge className="ml-1.5 text-[9px] bg-green-100 text-green-700 border-green-200" variant="outline">XML</Badge>
                            )}
                          </TableCell>
                          <TableCell className="py-1 px-3 text-[11px] text-right tabular-nums">{row.qtd.toLocaleString('pt-BR')}</TableCell>
                          <TableCell className="py-1 px-3 text-[11px] text-right tabular-nums">{fmtBRL(row.total)}</TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                )}
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm">Modelos de Documento — Entradas (EFD)</CardTitle>
              </CardHeader>
              <CardContent className="p-0">
                {loadingModelos ? (
                  <p className="text-sm text-center py-6 text-muted-foreground">Carregando...</p>
                ) : modelosEntrada.length === 0 ? (
                  <p className="text-sm text-center py-6 text-muted-foreground">Sem dados de entrada no EFD.</p>
                ) : (
                  <Table>
                    <TableHeader>
                      <TableRow className="hover:bg-transparent">
                        <TableHead className="py-1.5 px-3 text-[11px]">Modelo</TableHead>
                        <TableHead className="py-1.5 px-3 text-[11px]">Descrição</TableHead>
                        <TableHead className="py-1.5 px-3 text-[11px] text-right">Qtd</TableHead>
                        <TableHead className="py-1.5 px-3 text-[11px] text-right">Total</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {modelosEntrada.map(row => (
                        <TableRow key={row.cod_mod} className="h-8">
                          <TableCell className="py-1 px-3 text-[11px] font-mono font-bold">{row.cod_mod}</TableCell>
                          <TableCell className="py-1 px-3 text-[11px]">
                            <span>{row.descricao}</span>
                            {['55','65','57','58'].includes(row.cod_mod) && (
                              <Badge className="ml-1.5 text-[9px] bg-green-100 text-green-700 border-green-200" variant="outline">XML</Badge>
                            )}
                          </TableCell>
                          <TableCell className="py-1 px-3 text-[11px] text-right tabular-nums">{row.qtd.toLocaleString('pt-BR')}</TableCell>
                          <TableCell className="py-1 px-3 text-[11px] text-right tabular-nums">{fmtBRL(row.total)}</TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                )}
              </CardContent>
            </Card>
          </div>
          <p className="text-xs text-muted-foreground flex items-center gap-1">
            <Info className="h-3 w-3" />
            Documentos com badge <Badge className="text-[9px] bg-green-100 text-green-700 border-green-200 mx-1" variant="outline">XML</Badge>
            são importáveis como XML. Os demais (mod 01, 04, 06...) só existem no EFD e explicam parte da diferença.
          </p>
        </TabsContent>
      </Tabs>
    </div>
  );
}
