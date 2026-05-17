import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { toast } from 'sonner';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Download } from 'lucide-react';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------
interface PainelRow {
  forn_cnpj: string;
  forn_nome: string;
  mes_ano: string;
  source: string;
  qtd_notas: number;
  v_total: number;
  v_bc_icms: number;
  v_icms: number;
  v_pis: number;
  v_cofins: number;
  v_ipi: number;
  v_ibs: number;
  v_cbs: number;
}

interface PainelResponse {
  total: number;
  limit: number;
  offset: number;
  tipo: string;
  items: PainelRow[] | null;
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
  return `${v.slice(0,2)}.${v.slice(2,5)}.${v.slice(5,8)}/${v.slice(8,12)}-${v.slice(12)}`;
}

function SourceBadge({ source }: { source: string }) {
  const map: Record<string, { label: string; className: string }> = {
    xml_upload:    { label: 'XML',    className: 'bg-green-100 text-green-700 border-green-200' },
    oracle_bridge: { label: 'ERP',    className: 'bg-blue-100 text-blue-700 border-blue-200' },
    manual:        { label: 'Manual', className: 'bg-amber-100 text-amber-700 border-amber-200' },
  };
  const s = map[source] ?? { label: source, className: 'bg-gray-100 text-gray-600' };
  return (
    <Badge variant="outline" className={`text-[10px] px-1.5 py-0 ${s.className}`}>{s.label}</Badge>
  );
}

function exportCSV(rows: PainelRow[], tipo: string) {
  const headers = ['CNPJ', 'Nome', 'Mês/Ano', 'Fonte', 'Qtd', 'V.Total', 'BC ICMS', 'VLR ICMS', 'VLR PIS', 'VLR COFINS', 'VLR IPI', 'VLR IBS', 'VLR CBS'];
  const lines = rows.map(r => [
    r.forn_cnpj,
    `"${r.forn_nome}"`,
    r.mes_ano,
    r.source,
    r.qtd_notas,
    r.v_total,
    r.v_bc_icms,
    r.v_icms,
    r.v_pis,
    r.v_cofins,
    r.v_ipi,
    r.v_ibs,
    r.v_cbs,
  ].join(';'));
  const csv = [headers.join(';'), ...lines].join('\n');
  const blob = new Blob(['﻿' + csv], { type: 'text/csv;charset=utf-8;' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `painel_xml_${tipo}_${new Date().toISOString().slice(0, 10)}.csv`;
  a.click();
  URL.revokeObjectURL(url);
}

// ---------------------------------------------------------------------------
// Tabela genérica para entradas/saídas
// ---------------------------------------------------------------------------
function TabelaPainel({ tipo }: { tipo: 'entradas' | 'saidas' | 'ctes' }) {
  const [mesAno, setMesAno] = useState('');
  const [mesAnoFilter, setMesAnoFilter] = useState('');

  const { data, isLoading, isError, refetch } = useQuery<PainelResponse>({
    queryKey: ['xml-painel', tipo, mesAnoFilter],
    queryFn: async () => {
      const params = new URLSearchParams({ limit: '100' });
      if (mesAnoFilter) params.set('mes_ano', mesAnoFilter);
      const res = await fetch(`/api/xml/painel/${tipo}?${params}`);
      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: res.statusText }));
        throw new Error(err.error || res.statusText);
      }
      return res.json();
    },
    staleTime: 60_000,
    throwOnError: false,
  });

  const rows = data?.items ?? [];

  const handleSearch = () => {
    setMesAnoFilter(mesAno);
    refetch();
  };

  if (isError) {
    toast.error(`Erro ao carregar painel de ${tipo}`);
  }

  const isCtes = tipo === 'ctes';

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3 flex-wrap">
        <div className="flex items-center gap-2">
          <label className="text-xs text-muted-foreground whitespace-nowrap">Mês/Ano</label>
          <Input
            type="text"
            placeholder="MM/YYYY"
            value={mesAno}
            onChange={e => setMesAno(e.target.value)}
            className="h-8 w-28 text-sm"
          />
        </div>
        <Button size="sm" variant="outline" onClick={handleSearch} disabled={isLoading} className="h-8">
          {isLoading ? 'Buscando...' : 'Buscar'}
        </Button>
        {rows.length > 0 && (
          <Button
            size="sm"
            variant="ghost"
            className="h-8 ml-auto text-muted-foreground"
            onClick={() => exportCSV(rows, tipo)}
          >
            <Download className="h-3.5 w-3.5 mr-1.5" />
            Exportar CSV
          </Button>
        )}
        {data && (
          <span className="text-xs text-muted-foreground">
            {data.total} {isCtes ? 'CT-e(s)' : 'nota(s)'} encontrada(s)
          </span>
        )}
      </div>

      {isLoading ? (
        <p className="text-sm text-muted-foreground text-center py-8">Carregando...</p>
      ) : rows.length === 0 ? (
        <p className="text-sm text-muted-foreground text-center py-8">
          Nenhum dado encontrado.{mesAnoFilter ? ` Período: ${mesAnoFilter}` : ' Importe XMLs para visualizar dados.'}
        </p>
      ) : (
        <div className="overflow-x-auto rounded-md border">
          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent bg-muted/30">
                <TableHead className="py-1.5 px-2 text-[11px]">{isCtes ? 'Transportadora' : tipo === 'saidas' ? 'Filial/Emitente' : 'Fornecedor'}</TableHead>
                <TableHead className="py-1.5 px-2 text-[11px]">Mês/Ano</TableHead>
                <TableHead className="py-1.5 px-2 text-[11px] text-center">Qtd</TableHead>
                <TableHead className="py-1.5 px-2 text-[11px]">Fonte</TableHead>
                <TableHead className="py-1.5 px-2 text-[11px] text-right">V. Total</TableHead>
                <TableHead className="py-1.5 px-2 text-[11px] text-right">BC ICMS</TableHead>
                <TableHead className="py-1.5 px-2 text-[11px] text-right">VLR ICMS</TableHead>
                {!isCtes && <TableHead className="py-1.5 px-2 text-[11px] text-right">VLR PIS</TableHead>}
                {!isCtes && <TableHead className="py-1.5 px-2 text-[11px] text-right">VLR COFINS</TableHead>}
                {!isCtes && <TableHead className="py-1.5 px-2 text-[11px] text-right">VLR IPI</TableHead>}
                <TableHead className="py-1.5 px-2 text-[11px] text-right">VLR IBS</TableHead>
                <TableHead className="py-1.5 px-2 text-[11px] text-right">VLR CBS</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((row, idx) => (
                <TableRow key={`${row.forn_cnpj}-${row.mes_ano}-${idx}`} className="h-8">
                  <TableCell className="py-1 px-2">
                    <div className="text-[11px] font-medium leading-tight truncate max-w-[180px]">{row.forn_nome || '—'}</div>
                    <div className="text-[10px] text-muted-foreground font-mono leading-tight">{fmtCNPJ(row.forn_cnpj)}</div>
                  </TableCell>
                  <TableCell className="py-1 px-2 text-[11px] whitespace-nowrap">{row.mes_ano}</TableCell>
                  <TableCell className="py-1 px-2 text-[11px] text-center font-medium">{row.qtd_notas}</TableCell>
                  <TableCell className="py-1 px-2"><SourceBadge source={row.source} /></TableCell>
                  <TableCell className="py-1 px-2 text-right text-[11px] font-semibold">{fmtBRL(row.v_total)}</TableCell>
                  <TableCell className="py-1 px-2 text-right text-[11px]">{fmtBRL(row.v_bc_icms)}</TableCell>
                  <TableCell className="py-1 px-2 text-right text-[11px]">{fmtBRL(row.v_icms)}</TableCell>
                  {!isCtes && <TableCell className="py-1 px-2 text-right text-[11px]">{fmtBRL(row.v_pis)}</TableCell>}
                  {!isCtes && <TableCell className="py-1 px-2 text-right text-[11px]">{fmtBRL(row.v_cofins)}</TableCell>}
                  {!isCtes && <TableCell className="py-1 px-2 text-right text-[11px]">{fmtBRL(row.v_ipi)}</TableCell>}
                  <TableCell className="py-1 px-2 text-right text-[11px]">{fmtBRL(row.v_ibs)}</TableCell>
                  <TableCell className="py-1 px-2 text-right text-[11px]">{fmtBRL(row.v_cbs)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Main component
// ---------------------------------------------------------------------------
export default function PainelXMLs() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Painel XMLs</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Visualize os dados importados via XML, ERP Bridge ou lançamento manual por tipo de nota.
        </p>
      </div>

      <Card>
        <CardHeader className="pb-0">
          <CardTitle className="text-base">Notas e CT-es Importados</CardTitle>
        </CardHeader>
        <CardContent className="pt-4">
          <Tabs defaultValue="entradas">
            <TabsList className="mb-4">
              <TabsTrigger value="entradas">NF-e Entradas</TabsTrigger>
              <TabsTrigger value="saidas">NF-e Saídas</TabsTrigger>
              <TabsTrigger value="ctes">CT-es</TabsTrigger>
            </TabsList>
            <TabsContent value="entradas">
              <TabelaPainel tipo="entradas" />
            </TabsContent>
            <TabsContent value="saidas">
              <TabelaPainel tipo="saidas" />
            </TabsContent>
            <TabsContent value="ctes">
              <TabelaPainel tipo="ctes" />
            </TabsContent>
          </Tabs>
        </CardContent>
      </Card>
    </div>
  );
}
