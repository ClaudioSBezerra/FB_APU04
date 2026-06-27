import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
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
import { Download, LayoutList, BarChart3, ChevronLeft, ChevronRight, XCircle, RefreshCcw } from 'lucide-react';
import { useAuth } from '@/contexts/AuthContext';

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

interface NotaRow {
  chave: string;
  numero: string;
  serie: string;
  data_emissao: string;
  mes_ano: string;
  nat_op: string;
  par_cnpj: string;
  par_nome: string;
  dest_cnpj: string;
  dest_nome: string;
  dest_uf: string;
  v_total: number;
  v_bc_icms: number;
  v_icms: number;
  v_pis: number;
  v_cofins: number;
  v_ipi: number;
  v_ibs: number;
  v_cbs: number;
  source: string;
  status?: string; // ATIVO | CANCELADO (apenas entradas)
}

interface NotasResponse {
  total: number;
  limit: number;
  offset: number;
  tipo: string;
  items: NotaRow[] | null;
}

// Filtros compartilhados entre Resumo e Nota a Nota
interface SharedFilters {
  destUF: string;
  cnpjFilial: string;
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

function shortChave(chave: string): string {
  if (!chave || chave.length < 8) return chave || '—';
  return `...${chave.slice(-8)}`;
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

// ---------------------------------------------------------------------------
// CancelNFButton — cancela/reativa uma NF-e entrada diretamente do grid
// ---------------------------------------------------------------------------
function CancelNFButton({ chave, status, token, queryKey }: {
  chave: string;
  status?: string;
  token: string | null;
  queryKey: unknown[];
}) {
  const qc = useQueryClient();
  const [loading, setLoading] = useState(false);
  const isCancelado = status === 'CANCELADO';

  const toggle = async () => {
    if (!token) return;
    const newStatus = isCancelado ? 'ATIVO' : 'CANCELADO';
    if (!isCancelado && !window.confirm(`Cancelar NF ${chave.slice(-8)}?\nEla continuará visível mas não será somada nos totais.`)) return;
    setLoading(true);
    try {
      const res = await fetch('/api/admin/nf/cancelamento', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({ chave_nfe: chave, status: newStatus }),
      });
      if (!res.ok) throw new Error((await res.json()).error || res.statusText);
      toast.success(isCancelado ? 'NF reativada com sucesso' : 'NF marcada como cancelada');
      qc.invalidateQueries({ queryKey });
    } catch (e: unknown) {
      toast.error(`Erro: ${e instanceof Error ? e.message : String(e)}`);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Button
      size="sm"
      variant="ghost"
      className={`h-6 px-1.5 text-[10px] ${isCancelado ? 'text-emerald-600 hover:text-emerald-700' : 'text-rose-500 hover:text-rose-600'}`}
      onClick={toggle}
      disabled={loading}
      title={isCancelado ? 'Reativar NF' : 'Cancelar NF (deleção lógica)'}
    >
      {isCancelado ? <RefreshCcw className="h-3 w-3" /> : <XCircle className="h-3 w-3" />}
    </Button>
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

function exportNotasCSV(rows: NotaRow[], tipo: string) {
  const headers = ['Chave', 'Número', 'Série', 'Emissão', 'Mês/Ano', 'Nat.Op', 'CNPJ Emitente', 'Emitente', 'CNPJ Dest', 'Destinatário', 'UF Dest', 'V.Total', 'BC ICMS', 'VLR ICMS', 'VLR PIS', 'VLR COFINS', 'VLR IPI', 'VLR IBS', 'VLR CBS', 'Fonte'];
  const lines = rows.map(r => [
    r.chave, r.numero, r.serie, r.data_emissao, r.mes_ano,
    `"${r.nat_op}"`, r.par_cnpj, `"${r.par_nome}"`,
    r.dest_cnpj, `"${r.dest_nome}"`, r.dest_uf,
    r.v_total, r.v_bc_icms, r.v_icms, r.v_pis, r.v_cofins, r.v_ipi, r.v_ibs, r.v_cbs,
    r.source,
  ].join(';'));
  const csv = [headers.join(';'), ...lines].join('\n');
  const blob = new Blob(['﻿' + csv], { type: 'text/csv;charset=utf-8;' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `notas_xml_${tipo}_${new Date().toISOString().slice(0, 10)}.csv`;
  a.click();
  URL.revokeObjectURL(url);
}

// ---------------------------------------------------------------------------
// Tabela resumida (agregada por fornecedor + mês)
// ---------------------------------------------------------------------------
function TabelaPainel({
  tipo,
  shared,
}: {
  tipo: 'entradas' | 'saidas' | 'ctes';
  shared: SharedFilters;
}) {
  const [mesAno, setMesAno] = useState('');
  const [mesAnoFilter, setMesAnoFilter] = useState('');

  const { data, isLoading, isError, refetch } = useQuery<PainelResponse>({
    queryKey: ['xml-painel', tipo, mesAnoFilter, shared.destUF, shared.cnpjFilial],
    queryFn: async () => {
      const params = new URLSearchParams({ limit: '100' });
      if (mesAnoFilter) params.set('mes_ano', mesAnoFilter);
      if (shared.destUF) params.set('dest_uf', shared.destUF);
      if (shared.cnpjFilial) params.set('cnpj_filial', shared.cnpjFilial);
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
            onKeyDown={e => e.key === 'Enter' && handleSearch()}
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
// Tabela nota a nota (linhas individuais)
// ---------------------------------------------------------------------------
const PAGE_SIZE = 100;

function TabelaNotas({
  tipo,
  shared,
}: {
  tipo: 'entradas' | 'saidas' | 'ctes';
  shared: SharedFilters;
}) {
  const { token } = useAuth();
  const [dataInicio, setDataInicio] = useState('');
  const [dataFim, setDataFim] = useState('');
  const [chaveNf, setChaveNf] = useState('');
  const [filtro, setFiltro] = useState({ inicio: '', fim: '', chave: '' });
  const [offset, setOffset] = useState(0);

  const qKey = ['xml-notas', tipo, filtro.inicio, filtro.fim, filtro.chave, offset, shared.destUF, shared.cnpjFilial];
  const { data, isLoading, isError } = useQuery<NotasResponse>({
    queryKey: qKey,
    queryFn: async () => {
      const params = new URLSearchParams({ limit: String(PAGE_SIZE), offset: String(offset) });
      if (filtro.inicio) params.set('data_inicio', filtro.inicio);
      if (filtro.fim) params.set('data_fim', filtro.fim);
      if (filtro.chave) params.set('chave', filtro.chave);
      if (shared.destUF) params.set('dest_uf', shared.destUF);
      if (shared.cnpjFilial) params.set('cnpj_filial', shared.cnpjFilial);
      const res = await fetch(`/api/xml/notas/${tipo}?${params}`);
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
  const total = data?.total ?? 0;
  const totalPages = Math.ceil(total / PAGE_SIZE);
  const currentPage = Math.floor(offset / PAGE_SIZE) + 1;

  const handleSearch = () => {
    setOffset(0);
    setFiltro({ inicio: dataInicio, fim: dataFim, chave: chaveNf.trim() });
  };

  if (isError) {
    toast.error(`Erro ao carregar notas de ${tipo}`);
  }

  const isCtes = tipo === 'ctes';
  const isSaidas = tipo === 'saidas';

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3 flex-wrap">
        <div className="flex items-center gap-2">
          <label className="text-xs text-muted-foreground whitespace-nowrap">De</label>
          <Input
            type="date"
            value={dataInicio}
            onChange={e => setDataInicio(e.target.value)}
            className="h-8 w-36 text-sm"
          />
        </div>
        <div className="flex items-center gap-2">
          <label className="text-xs text-muted-foreground whitespace-nowrap">Até</label>
          <Input
            type="date"
            value={dataFim}
            onChange={e => setDataFim(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && handleSearch()}
            className="h-8 w-36 text-sm"
          />
        </div>
        <div className="flex items-center gap-2">
          <label className="text-xs text-muted-foreground whitespace-nowrap">Chave / Nº NF</label>
          <Input
            type="text"
            placeholder="Chave ou número"
            value={chaveNf}
            onChange={e => setChaveNf(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && handleSearch()}
            className="h-8 w-44 text-xs font-mono"
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
            onClick={() => exportNotasCSV(rows, tipo)}
          >
            <Download className="h-3.5 w-3.5 mr-1.5" />
            Exportar CSV
          </Button>
        )}
        {data && (
          <span className="text-xs text-muted-foreground">
            {total} {isCtes ? 'CT-e(s)' : 'nota(s)'} encontrada(s)
          </span>
        )}
      </div>

      {isLoading ? (
        <p className="text-sm text-muted-foreground text-center py-8">Carregando...</p>
      ) : rows.length === 0 ? (
        <p className="text-sm text-muted-foreground text-center py-8">
          Nenhuma nota encontrada.{(filtro.inicio || filtro.fim) ? ` Período: ${filtro.inicio || '?'} → ${filtro.fim || '?'}` : ' Importe XMLs para visualizar dados.'}
        </p>
      ) : (
        <>
          <div className="overflow-x-auto rounded-md border">
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent bg-muted/30">
                  <TableHead className="py-1.5 px-2 text-[11px]">Nº / Chave</TableHead>
                  <TableHead className="py-1.5 px-2 text-[11px]">Emissão</TableHead>
                  <TableHead className="py-1.5 px-2 text-[11px]">{isCtes ? 'Transportadora' : isSaidas ? 'Filial/Emitente' : 'Fornecedor'}</TableHead>
                  {!isCtes && (
                    <TableHead className="py-1.5 px-2 text-[11px]">{isSaidas ? 'Cliente' : 'Destinatário'}</TableHead>
                  )}
                  {!isCtes && <TableHead className="py-1.5 px-2 text-[11px]">Nat. Op.</TableHead>}
                  <TableHead className="py-1.5 px-2 text-[11px]">Fonte</TableHead>
                  <TableHead className="py-1.5 px-2 text-[11px] text-right">V. Total</TableHead>
                  <TableHead className="py-1.5 px-2 text-[11px] text-right">VLR ICMS</TableHead>
                  {!isCtes && <TableHead className="py-1.5 px-2 text-[11px] text-right">VLR PIS</TableHead>}
                  {!isCtes && <TableHead className="py-1.5 px-2 text-[11px] text-right">VLR COFINS</TableHead>}
                  <TableHead className="py-1.5 px-2 text-[11px] text-right">VLR IBS</TableHead>
                  <TableHead className="py-1.5 px-2 text-[11px] text-right">VLR CBS</TableHead>
                  {tipo === 'entradas' && <TableHead className="py-1.5 px-2 text-[11px] text-center w-10">Ação</TableHead>}
                </TableRow>
              </TableHeader>
              <TableBody>
                {rows.map((row, idx) => {
                  const isCancelado = row.status === 'CANCELADO';
                  return (
                  <TableRow key={`${row.chave}-${idx}`} className={`h-8 ${isCancelado ? 'opacity-50 bg-rose-50/40 line-through' : ''}`}>
                    <TableCell className="py-1 px-2">
                      <div className="flex items-center gap-1">
                        <div className="text-[11px] font-mono font-medium leading-tight">
                          {row.numero ? `${row.numero}${row.serie ? `/${row.serie}` : ''}` : '—'}
                        </div>
                        {isCancelado && <Badge variant="outline" className="text-[9px] px-1 py-0 h-3.5 bg-rose-100 text-rose-600 border-rose-300 no-underline">cancelada</Badge>}
                      </div>
                      <div className="text-[10px] text-muted-foreground font-mono leading-tight" title={row.chave}>
                        {shortChave(row.chave)}
                      </div>
                    </TableCell>
                    <TableCell className="py-1 px-2 text-[11px] whitespace-nowrap">{row.data_emissao}</TableCell>
                    <TableCell className="py-1 px-2">
                      <div className="text-[11px] font-medium leading-tight truncate max-w-[150px]">{row.par_nome || '—'}</div>
                      <div className="text-[10px] text-muted-foreground font-mono leading-tight">{fmtCNPJ(row.par_cnpj)}</div>
                    </TableCell>
                    {!isCtes && (
                      <TableCell className="py-1 px-2">
                        {row.dest_nome ? (
                          <>
                            <div className="text-[11px] leading-tight truncate max-w-[150px]">{row.dest_nome}</div>
                            <div className="text-[10px] text-muted-foreground font-mono leading-tight">
                              {row.dest_cnpj ? fmtCNPJ(row.dest_cnpj) : '—'}{row.dest_uf ? ` · ${row.dest_uf}` : ''}
                            </div>
                          </>
                        ) : (
                          <span className="text-[11px] text-muted-foreground">—</span>
                        )}
                      </TableCell>
                    )}
                    {!isCtes && (
                      <TableCell className="py-1 px-2 text-[11px] max-w-[120px] truncate" title={row.nat_op}>
                        {row.nat_op || '—'}
                      </TableCell>
                    )}
                    <TableCell className="py-1 px-2"><SourceBadge source={row.source} /></TableCell>
                    <TableCell className="py-1 px-2 text-right text-[11px] font-semibold">{fmtBRL(row.v_total)}</TableCell>
                    <TableCell className="py-1 px-2 text-right text-[11px]">{fmtBRL(row.v_icms)}</TableCell>
                    {!isCtes && <TableCell className="py-1 px-2 text-right text-[11px]">{fmtBRL(row.v_pis)}</TableCell>}
                    {!isCtes && <TableCell className="py-1 px-2 text-right text-[11px]">{fmtBRL(row.v_cofins)}</TableCell>}
                    <TableCell className="py-1 px-2 text-right text-[11px]">{fmtBRL(row.v_ibs)}</TableCell>
                    <TableCell className="py-1 px-2 text-right text-[11px]">{fmtBRL(row.v_cbs)}</TableCell>
                    {tipo === 'entradas' && (
                      <TableCell className="py-1 px-2 text-center">
                        <CancelNFButton chave={row.chave} status={row.status} token={token} queryKey={qKey} />
                      </TableCell>
                    )}
                  </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </div>

          {totalPages > 1 && (
            <div className="flex items-center justify-between pt-1">
              <span className="text-xs text-muted-foreground">
                Página {currentPage} de {totalPages}
              </span>
              <div className="flex gap-2">
                <Button
                  size="sm" variant="outline" className="h-7 px-2"
                  disabled={offset === 0}
                  onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
                >
                  <ChevronLeft className="h-3.5 w-3.5" />
                </Button>
                <Button
                  size="sm" variant="outline" className="h-7 px-2"
                  disabled={offset + PAGE_SIZE >= total}
                  onClick={() => setOffset(offset + PAGE_SIZE)}
                >
                  <ChevronRight className="h-3.5 w-3.5" />
                </Button>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Tab wrapper com toggle Resumo / Nota a Nota + filtros compartilhados
// ---------------------------------------------------------------------------
function TabPainel({ tipo }: { tipo: 'entradas' | 'saidas' | 'ctes' }) {
  const [view, setView] = useState<'resumo' | 'notas'>('resumo');

  // Filtros compartilhados: aplicados em ambas as views (Resumo e Nota a Nota)
  const [destUFInput, setDestUFInput] = useState('');
  const [cnpjFilialInput, setCnpjFilialInput] = useState('');
  const [shared, setShared] = useState<SharedFilters>({ destUF: '', cnpjFilial: '' });

  const handleApplyShared = () => {
    setShared({
      destUF: destUFInput.toUpperCase().trim(),
      cnpjFilial: cnpjFilialInput.replace(/\D/g, ''),
    });
  };

  const handleClearShared = () => {
    setDestUFInput('');
    setCnpjFilialInput('');
    setShared({ destUF: '', cnpjFilial: '' });
  };

  const hasActiveShared = shared.destUF !== '' || shared.cnpjFilial !== '';

  return (
    <div className="space-y-3">
      {/* Filtros Filial / UF — compartilhados entre Resumo e Nota a Nota */}
      <div className="flex items-center gap-3 flex-wrap p-3 bg-muted/20 rounded-md border border-dashed">
        <span className="text-[11px] font-medium text-muted-foreground whitespace-nowrap">Filtrar por:</span>
        <div className="flex items-center gap-1.5">
          <label className="text-[11px] text-muted-foreground whitespace-nowrap">UF Dest.</label>
          <Input
            type="text"
            placeholder="PE"
            value={destUFInput}
            onChange={e => setDestUFInput(e.target.value.toUpperCase().slice(0, 2))}
            onKeyDown={e => e.key === 'Enter' && handleApplyShared()}
            className="h-7 w-14 text-xs uppercase"
            maxLength={2}
          />
        </div>
        <div className="flex items-center gap-1.5">
          <label className="text-[11px] text-muted-foreground whitespace-nowrap">
            {tipo === 'saidas' ? 'CNPJ Emitente' : 'CNPJ Filial'}
          </label>
          <Input
            type="text"
            placeholder="8 primeiros dígitos"
            value={cnpjFilialInput}
            onChange={e => setCnpjFilialInput(e.target.value.replace(/\D/g, '').slice(0, 14))}
            onKeyDown={e => e.key === 'Enter' && handleApplyShared()}
            className="h-7 w-40 text-xs font-mono"
            maxLength={14}
          />
        </div>
        <Button size="sm" variant="outline" onClick={handleApplyShared} className="h-7 text-[11px]">
          Aplicar
        </Button>
        {hasActiveShared && (
          <Button size="sm" variant="ghost" onClick={handleClearShared} className="h-7 text-[11px] text-muted-foreground">
            Limpar
          </Button>
        )}
        {hasActiveShared && (
          <span className="text-[10px] text-amber-600 font-medium">
            {[shared.destUF && `UF: ${shared.destUF}`, shared.cnpjFilial && `Filial: ${shared.cnpjFilial}`].filter(Boolean).join(' · ')}
          </span>
        )}
      </div>

      {/* Toggle Resumo / Nota a Nota */}
      <div className="flex gap-1 border rounded-md p-0.5 w-fit bg-muted/30">
        <button
          onClick={() => setView('resumo')}
          className={`flex items-center gap-1.5 text-[11px] px-3 py-1 rounded-sm transition-colors ${
            view === 'resumo'
              ? 'bg-background shadow-sm font-medium'
              : 'text-muted-foreground hover:text-foreground'
          }`}
        >
          <BarChart3 className="h-3 w-3" />
          Resumo
        </button>
        <button
          onClick={() => setView('notas')}
          className={`flex items-center gap-1.5 text-[11px] px-3 py-1 rounded-sm transition-colors ${
            view === 'notas'
              ? 'bg-background shadow-sm font-medium'
              : 'text-muted-foreground hover:text-foreground'
          }`}
        >
          <LayoutList className="h-3 w-3" />
          Nota a Nota
        </button>
      </div>

      {view === 'resumo' ? (
        <TabelaPainel tipo={tipo} shared={shared} />
      ) : (
        <TabelaNotas tipo={tipo} shared={shared} />
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
              <TabPainel tipo="entradas" />
            </TabsContent>
            <TabsContent value="saidas">
              <TabPainel tipo="saidas" />
            </TabsContent>
            <TabsContent value="ctes">
              <TabPainel tipo="ctes" />
            </TabsContent>
          </Tabs>
        </CardContent>
      </Card>
    </div>
  );
}
