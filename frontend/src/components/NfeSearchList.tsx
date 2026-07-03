// NfeSearchList.tsx — lista de NF-e de saída com filtro de período/número
// visível na página (não escondido em popover) + seleção múltipla + execução
// em lote do pacote fiscal (Fase 12, revisão pós-deploy).
//
// Substitui NfeSearchCombobox.tsx: o combobox digite-e-selecione-1 escondia
// os resultados dentro de um popover pequeno e só permitia executar uma nota
// por vez. Aqui os filtros ficam visíveis, os resultados aparecem numa
// tabela com checkbox por linha + "selecionar todas", e o botão "Executar
// Selecionadas" dispara POST /api/fiscal/execute para cada nota marcada
// (3 em paralelo — o backend já limita a 5 itens em paralelo DENTRO de
// cada nota, então 3 notas simultâneas é um teto conservador no navegador).
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Badge } from '@/components/ui/badge';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Search, Loader2, Send, Eye } from 'lucide-react';
import { useAuth } from '@/contexts/AuthContext';

export interface NfeSearchResult {
  id: string;
  chave_nfe: string;
  numero_nfe: string;
  serie: string;
  dest_nome: string;
  data_emissao: string;
  // Totais do cabeçalho da nota (bloco <ICMSTot> do XML) — usados no
  // "Resumo da Nota" (acumulado dos itens vs. total declarado da NF).
  v_icms: number;
  v_st: number;
  v_pis: number;
  v_cofins: number;
  v_ibs: number;
  v_cbs: number;
}

interface ExecuteSummary {
  total: number;
  ok: number;
  sem_grupo_fiscal: number;
  error: number;
}

type ExecStatus = 'idle' | 'running' | 'ok' | 'partial' | 'failed';

// Teto de concorrência no navegador para a execução em lote de várias notas.
const CONCURRENCY = 3;

function ExecStatusBadge({ status }: { status: ExecStatus }) {
  if (status === 'running') {
    return (
      <Badge variant="outline" className="text-[10px] px-1.5 py-0 gap-1">
        <Loader2 className="h-2.5 w-2.5 animate-spin" /> Executando
      </Badge>
    );
  }
  if (status === 'ok') {
    return <Badge variant="outline" className="text-[10px] px-1.5 py-0 bg-emerald-50 text-emerald-700 border-emerald-200">OK</Badge>;
  }
  if (status === 'partial') {
    return <Badge variant="outline" className="text-[10px] px-1.5 py-0 bg-amber-50 text-amber-700 border-amber-200">Parcial</Badge>;
  }
  if (status === 'failed') {
    return <Badge variant="outline" className="text-[10px] px-1.5 py-0 bg-red-50 text-red-700 border-red-200">Falhou</Badge>;
  }
  return <Badge variant="outline" className="text-[10px] px-1.5 py-0 bg-gray-50 text-muted-foreground">—</Badge>;
}

export function NfeSearchList({
  onViewDetail,
  activeId,
}: {
  onViewDetail: (nfe: NfeSearchResult) => void;
  activeId?: string | null;
}) {
  const { token, companyId } = useAuth();
  const authHeaders = { Authorization: `Bearer ${token}`, 'X-Company-ID': companyId || '' };

  const [dataInicio, setDataInicio] = useState('');
  const [dataFim, setDataFim] = useState('');
  const [q, setQ] = useState('');
  const [ufOrigem, setUfOrigem] = useState('');
  const [ufDestino, setUfDestino] = useState('');
  const [cliente, setCliente] = useState('');
  const [emitente, setEmitente] = useState('');
  const emptyApplied = { dataInicio: '', dataFim: '', q: '', ufOrigem: '', ufDestino: '', cliente: '', emitente: '' };
  const [applied, setApplied] = useState(emptyApplied);
  const [searched, setSearched] = useState(false);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [execStatus, setExecStatus] = useState<Record<string, ExecStatus>>({});
  const [batchRunning, setBatchRunning] = useState(false);

  const { data, isLoading, isError, refetch } = useQuery<NfeSearchResult[]>({
    queryKey: ['nfe-saidas-search', applied],
    queryFn: async () => {
      const params = new URLSearchParams();
      if (applied.q) params.set('q', applied.q);
      if (applied.dataInicio) params.set('data_inicio', applied.dataInicio);
      if (applied.dataFim) params.set('data_fim', applied.dataFim);
      if (applied.ufOrigem) params.set('uf_origem', applied.ufOrigem);
      if (applied.ufDestino) params.set('uf_destino', applied.ufDestino);
      if (applied.cliente) params.set('cliente', applied.cliente);
      if (applied.emitente) params.set('emitente', applied.emitente);
      const res = await fetch(`/api/fiscal/comparacao/search?${params}`, { headers: authHeaders });
      if (!res.ok) throw new Error(res.statusText);
      return res.json();
    },
    // Roda a partir do primeiro clique em "Buscar" — sem exigir 3+ caracteres
    // em nenhum campo; sem filtro nenhum, lista as 50 notas mais recentes.
    enabled: searched,
  });

  const rows = data ?? [];

  const handleSearch = () => {
    setSelected(new Set());
    setSearched(true);
    setApplied({ dataInicio, dataFim, q: q.trim(), ufOrigem, ufDestino, cliente: cliente.trim(), emitente: emitente.trim() });
  };

  const allSelected = rows.length > 0 && rows.every(r => selected.has(r.id));
  const someSelected = selected.size > 0;

  const toggleAll = () => {
    setSelected(allSelected ? new Set() : new Set(rows.map(r => r.id)));
  };

  const toggleOne = (id: string) => {
    setSelected(prev => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      return next;
    });
  };

  const executeOne = async (id: string) => {
    setExecStatus(prev => ({ ...prev, [id]: 'running' }));
    try {
      const res = await fetch('/api/fiscal/execute', {
        method: 'POST',
        headers: { ...authHeaders, 'Content-Type': 'application/json' },
        body: JSON.stringify({ nfe_id: id }),
      });
      if (!res.ok) throw new Error(await res.text());
      const summary: ExecuteSummary = await res.json();
      setExecStatus(prev => ({
        ...prev,
        [id]: (summary.error > 0 || summary.sem_grupo_fiscal > 0) ? 'partial' : 'ok',
      }));
    } catch {
      setExecStatus(prev => ({ ...prev, [id]: 'failed' }));
    }
  };

  const executeSelected = async () => {
    const ids = Array.from(selected);
    if (ids.length === 0) return;
    setBatchRunning(true);
    let cursor = 0;
    const worker = async () => {
      while (cursor < ids.length) {
        const id = ids[cursor++];
        await executeOne(id);
      }
    };
    await Promise.all(Array.from({ length: Math.min(CONCURRENCY, ids.length) }, worker));
    setBatchRunning(false);
    // Nota já executada continua com seus itens carregados no detalhe — força
    // reload se o usuário estiver olhando o detalhe de uma nota recém-executada.
  };

  return (
    <div className="space-y-3">
      {/* Filtros visíveis na página — não escondidos em popover/combobox */}
      <div className="flex items-end gap-2 flex-wrap p-3 bg-muted/20 rounded-md border border-dashed">
        <div className="flex flex-col gap-1">
          <label className="text-[11px] text-muted-foreground">De</label>
          <Input type="date" value={dataInicio} onChange={e => setDataInicio(e.target.value)} className="h-8 w-36 text-xs" />
        </div>
        <div className="flex flex-col gap-1">
          <label className="text-[11px] text-muted-foreground">Até</label>
          <Input
            type="date"
            value={dataFim}
            onChange={e => setDataFim(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && handleSearch()}
            className="h-8 w-36 text-xs"
          />
        </div>
        <div className="flex flex-col gap-1">
          <label className="text-[11px] text-muted-foreground">Número ou chave</label>
          <Input
            type="text"
            placeholder="opcional"
            value={q}
            onChange={e => setQ(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && handleSearch()}
            className="h-8 w-48 text-xs font-mono"
          />
        </div>
        <div className="flex flex-col gap-1">
          <label className="text-[11px] text-muted-foreground">UF Origem</label>
          <Input
            type="text"
            placeholder="PE"
            value={ufOrigem}
            onChange={e => setUfOrigem(e.target.value.toUpperCase().slice(0, 2))}
            onKeyDown={e => e.key === 'Enter' && handleSearch()}
            className="h-8 w-16 text-xs uppercase"
            maxLength={2}
          />
        </div>
        <div className="flex flex-col gap-1">
          <label className="text-[11px] text-muted-foreground">UF Destino</label>
          <Input
            type="text"
            placeholder="SP"
            value={ufDestino}
            onChange={e => setUfDestino(e.target.value.toUpperCase().slice(0, 2))}
            onKeyDown={e => e.key === 'Enter' && handleSearch()}
            className="h-8 w-16 text-xs uppercase"
            maxLength={2}
          />
        </div>
        <div className="flex flex-col gap-1">
          <label className="text-[11px] text-muted-foreground">Cliente</label>
          <Input
            type="text"
            placeholder="nome do destinatário"
            value={cliente}
            onChange={e => setCliente(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && handleSearch()}
            className="h-8 w-44 text-xs"
          />
        </div>
        <div className="flex flex-col gap-1">
          <label className="text-[11px] text-muted-foreground">Emitente</label>
          <Input
            type="text"
            placeholder="nome do emitente"
            value={emitente}
            onChange={e => setEmitente(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && handleSearch()}
            className="h-8 w-44 text-xs"
          />
        </div>
        <Button size="sm" onClick={handleSearch} disabled={isLoading} className="h-8">
          {isLoading ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Search className="h-3.5 w-3.5 mr-1.5" />}
          Buscar
        </Button>
        {rows.length > 0 && (
          <Button
            size="sm"
            onClick={executeSelected}
            disabled={!someSelected || batchRunning}
            className="h-8 ml-auto"
          >
            {batchRunning
              ? <Loader2 className="h-3.5 w-3.5 mr-1.5 animate-spin" />
              : <Send className="h-3.5 w-3.5 mr-1.5" />}
            Executar Selecionadas ({selected.size})
          </Button>
        )}
      </div>

      {!searched ? (
        <p className="text-sm text-muted-foreground text-center py-6">
          Clique em "Buscar" para listar as notas mais recentes, ou aplique um filtro primeiro.
        </p>
      ) : isError ? (
        <p className="text-sm text-destructive text-center py-6">
          Erro ao buscar NF-e. <button className="underline" onClick={() => refetch()}>Tentar novamente</button>
        </p>
      ) : isLoading ? (
        <p className="text-sm text-muted-foreground text-center py-6">Buscando...</p>
      ) : rows.length === 0 ? (
        <p className="text-sm text-muted-foreground text-center py-6">Nenhuma nota encontrada para os filtros informados.</p>
      ) : (
        <div className="overflow-x-auto rounded-md border">
          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent bg-muted/30">
                <TableHead className="w-10 py-1.5 px-2">
                  <Checkbox checked={allSelected} onCheckedChange={toggleAll} aria-label="Selecionar todas" />
                </TableHead>
                <TableHead className="py-1.5 px-2 text-[11px]">Nº / Série</TableHead>
                <TableHead className="py-1.5 px-2 text-[11px]">Destinatário</TableHead>
                <TableHead className="py-1.5 px-2 text-[11px]">Emissão</TableHead>
                <TableHead className="py-1.5 px-2 text-[11px]">Chave</TableHead>
                <TableHead className="py-1.5 px-2 text-[11px]">Status Execução</TableHead>
                <TableHead className="py-1.5 px-2 text-[11px] w-10"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map(row => {
                const status = execStatus[row.id] ?? 'idle';
                return (
                  <TableRow key={row.id} className={row.id === activeId ? 'bg-primary/5' : ''}>
                    <TableCell className="py-1 px-2">
                      <Checkbox
                        checked={selected.has(row.id)}
                        onCheckedChange={() => toggleOne(row.id)}
                        aria-label={`Selecionar NF ${row.numero_nfe}`}
                      />
                    </TableCell>
                    <TableCell className="py-1 px-2 text-[11px] font-medium whitespace-nowrap">
                      {row.numero_nfe}{row.serie ? `/${row.serie}` : ''}
                    </TableCell>
                    <TableCell className="py-1 px-2 text-[11px] truncate max-w-[180px]">{row.dest_nome}</TableCell>
                    <TableCell className="py-1 px-2 text-[11px] whitespace-nowrap">{row.data_emissao}</TableCell>
                    <TableCell className="py-1 px-2 text-[10px] font-mono text-muted-foreground">
                      {row.chave_nfe.slice(0, 8)}...{row.chave_nfe.slice(-6)}
                    </TableCell>
                    <TableCell className="py-1 px-2">
                      <ExecStatusBadge status={status} />
                    </TableCell>
                    <TableCell className="py-1 px-2">
                      <Button
                        variant="ghost"
                        size="sm"
                        className="h-6 px-1.5"
                        onClick={() => onViewDetail(row)}
                        aria-label="Ver comparação"
                      >
                        <Eye className="h-3.5 w-3.5" />
                      </Button>
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  );
}
