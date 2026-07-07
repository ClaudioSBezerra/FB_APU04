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
import { useEffect, useRef, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { type ComparacaoRow, avaliarDivergenciaNota } from '@/lib/fiscalComparacao';
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
  // Identificação/valores do cabeçalho para o strip do "Resumo da Nota"
  v_prod: number;   // total dos produtos (valor da venda)
  v_desc: number;   // total de descontos
  v_frete: number;  // total do frete destacado
  v_nf: number;     // valor total da NF
  // Totais fiscais extras (colunas FCP/DIFAL/ICMS Reduzido do Resumo da Nota)
  v_fcp: number;          // <vFCP>
  v_icms_uf_dest: number; // <vICMSUFDest> (DIFAL)
  v_icms_deson: number;   // <vICMSDeson> (ICMS desonerado/reduzido)
  v_fcp_st: number;       // <vFCPST> (FCP retido por ST)
  v_fcp_uf_dest: number;  // <vFCPUFDest> (FCP do DIFAL — o pacote o embute no DIFAL)
}

// Envelope paginado da busca (espelha NfeSearchResponse do backend)
interface NfeSearchResponse {
  total: number;
  page: number;
  page_size: number; // 0 = todas
  rows: NfeSearchResult[];
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
  incluirIbsCbs = false,
  onIncluirIbsCbsChange,
}: {
  onViewDetail: (nfe: NfeSearchResult) => void;
  activeId?: string | null;
  // Simulação "IBS/CBS na base do ICMS" — estado vive na página
  // (ComparacaoFiscal) para valer também no botão "Executar esta nota"
  incluirIbsCbs?: boolean;
  onIncluirIbsCbsChange?: (v: boolean) => void;
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
  // Filtros fiscais (checkboxes): valores destacados no XML da nota
  const [comIcms, setComIcms] = useState(false);
  const [comSt, setComSt] = useState(false);
  const [comDifal, setComDifal] = useState(false);
  const [comFcp, setComFcp] = useState(false);
  const [comBaseReduzida, setComBaseReduzida] = useState(false);
  // Notas CNPJ próprio (destinatário = mesma raiz de CNPJ do emitente:
  // transferências e outras saídas entre filiais, ex 5949) geram muita
  // "sujeira" de regras — padrão é IGNORAR; "somente" isola para testá-las
  const [cnpjProprio, setCnpjProprio] = useState<'excluir' | 'incluir' | 'somente'>('excluir');
  // Paginação: 0 = todas
  const [pageSize, setPageSize] = useState(50);
  const [page, setPage] = useState(1);
  const emptyApplied = {
    dataInicio: '', dataFim: '', q: '', ufOrigem: '', ufDestino: '', cliente: '', emitente: '',
    comIcms: false, comSt: false, comDifal: false, comFcp: false, comBaseReduzida: false,
    cnpjProprio: 'excluir' as 'excluir' | 'incluir' | 'somente', pageSize: 50,
  };
  const [applied, setApplied] = useState(emptyApplied);
  const [searched, setSearched] = useState(false);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [execStatus, setExecStatus] = useState<Record<string, ExecStatus>>({});
  const [batchRunning, setBatchRunning] = useState(false);

  const { data, isLoading, isError, refetch } = useQuery<NfeSearchResponse>({
    queryKey: ['nfe-saidas-search', applied, page],
    queryFn: async () => {
      const params = new URLSearchParams();
      if (applied.q) params.set('q', applied.q);
      if (applied.dataInicio) params.set('data_inicio', applied.dataInicio);
      if (applied.dataFim) params.set('data_fim', applied.dataFim);
      if (applied.ufOrigem) params.set('uf_origem', applied.ufOrigem);
      if (applied.ufDestino) params.set('uf_destino', applied.ufDestino);
      if (applied.cliente) params.set('cliente', applied.cliente);
      if (applied.emitente) params.set('emitente', applied.emitente);
      if (applied.comIcms) params.set('com_icms', '1');
      if (applied.comSt) params.set('com_st', '1');
      if (applied.comDifal) params.set('com_difal', '1');
      if (applied.comFcp) params.set('com_fcp', '1');
      if (applied.comBaseReduzida) params.set('com_base_reduzida', '1');
      if (applied.cnpjProprio !== 'incluir') params.set('cnpj_proprio', applied.cnpjProprio);
      params.set('page', String(page));
      params.set('page_size', String(applied.pageSize));
      const res = await fetch(`/api/fiscal/comparacao/search?${params}`, { headers: authHeaders });
      if (!res.ok) throw new Error(res.statusText);
      return res.json();
    },
    // Roda a partir do primeiro clique em "Buscar" — sem exigir 3+ caracteres
    // em nenhum campo; sem filtro nenhum, lista as notas mais recentes.
    enabled: searched,
  });

  const rows = data?.rows ?? [];
  const total = data?.total ?? 0;
  const totalPages = applied.pageSize > 0 ? Math.max(1, Math.ceil(total / applied.pageSize)) : 1;

  const handleSearch = () => {
    setSelected(new Set());
    setSearched(true);
    setPage(1);
    setApplied({
      dataInicio, dataFim, q: q.trim(), ufOrigem, ufDestino,
      cliente: cliente.trim(), emitente: emitente.trim(),
      comIcms, comSt, comDifal, comFcp, comBaseReduzida, cnpjProprio, pageSize,
    });
  };

  const goToPage = (p: number) => {
    setSelected(new Set());
    setPage(Math.min(Math.max(1, p), totalPages));
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

  // Avaliação por nota: Executado (existe resultado do pacote) + Divergência
  // (SIM/NÃO, mesma régua do detalhe — avaliarDivergenciaNota da lib).
  type AvalNota = { executado: boolean; divergente: boolean | null };
  const [aval, setAval] = useState<Record<string, AvalNota>>({});
  const avalEmAndamento = useRef<Set<string>>(new Set());

  const avaliarNota = async (nfe: NfeSearchResult) => {
    if (avalEmAndamento.current.has(nfe.id)) return;
    avalEmAndamento.current.add(nfe.id);
    try {
      const res = await fetch(`/api/fiscal/comparacao?nfe_id=${encodeURIComponent(nfe.id)}`, { headers: authHeaders });
      if (!res.ok) return;
      const compRows: ComparacaoRow[] = await res.json();
      const executado = compRows.length > 0 && compRows.some(r => r.status !== 'not_executed');
      setAval(prev => ({
        ...prev,
        [nfe.id]: { executado, divergente: executado ? avaliarDivergenciaNota(nfe, compRows) : null },
      }));
    } catch {
      // silencioso — célula fica em "—"
    } finally {
      avalEmAndamento.current.delete(nfe.id);
    }
  };

  // Avalia automaticamente as notas do resultado (páginas de até 100 — acima
  // disso só sob demanda, para não disparar centenas de requests).
  useEffect(() => {
    if (rows.length === 0 || rows.length > 100) return;
    const pendentes = rows.filter(r => aval[r.id] === undefined);
    if (pendentes.length === 0) return;
    let cursor = 0;
    const worker = async () => {
      while (cursor < pendentes.length) {
        const nfe = pendentes[cursor++];
        await avaliarNota(nfe);
      }
    };
    Promise.all(Array.from({ length: Math.min(4, pendentes.length) }, worker));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rows]);

  const executeOne = async (id: string) => {
    setExecStatus(prev => ({ ...prev, [id]: 'running' }));
    try {
      const res = await fetch('/api/fiscal/execute', {
        method: 'POST',
        headers: { ...authHeaders, 'Content-Type': 'application/json' },
        body: JSON.stringify({ nfe_id: id, incluir_ibs_cbs_base: incluirIbsCbs }),
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
    // Reavalia o veredito de divergência com o resultado fresco
    const nfe = rows.find(r => r.id === id);
    if (nfe) await avaliarNota(nfe);
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
        <div className="flex flex-col gap-1">
          <label className="text-[11px] text-muted-foreground">Notas por página</label>
          <select
            value={pageSize}
            onChange={e => setPageSize(Number(e.target.value))}
            className="h-8 w-28 text-xs rounded-md border bg-background px-2"
          >
            <option value={50}>50</option>
            <option value={100}>100</option>
            <option value={200}>200</option>
            <option value={500}>500</option>
            <option value={0}>Todas</option>
          </select>
        </div>
        <div className="flex flex-col gap-1">
          <label className="text-[11px] text-muted-foreground" title="Notas cujo destinatário tem a mesma raiz de CNPJ do emitente: transferências e outras saídas entre filiais (ex: CFOP 5949) — geram muitas regras específicas; ignorar deixa a análise mais limpa">Notas CNPJ próprio</label>
          <select
            value={cnpjProprio}
            onChange={e => setCnpjProprio(e.target.value as 'excluir' | 'incluir' | 'somente')}
            className="h-8 w-32 text-xs rounded-md border bg-background px-2"
          >
            <option value="excluir">Ignorar</option>
            <option value="incluir">Incluir</option>
            <option value="somente">Somente</option>
          </select>
        </div>
        <Button size="sm" onClick={handleSearch} disabled={isLoading} className="h-8">
          {isLoading ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Search className="h-3.5 w-3.5 mr-1.5" />}
          Buscar
        </Button>
        {rows.length > 0 && (
          <div className="flex items-end gap-3 ml-auto">
            <label className="flex items-center gap-1.5 text-xs cursor-pointer whitespace-nowrap pb-1.5" title="Se SIM, a tela compara a inclusão de IBS/CBS na base: Original (XML) × Cálculo Simulado (interno: nova base = base + IBS + CBS calculados sobre o preço líquido) × Cálculo do Pacote (que já embute a inclusão na chamada)">
              <Checkbox
                checked={incluirIbsCbs}
                onCheckedChange={c => onIncluirIbsCbsChange?.(c === true)}
              />
              Inclui IBS/CBS base ICMS?
            </label>
            <Button
              size="sm"
              onClick={executeSelected}
              disabled={!someSelected || batchRunning}
              className="h-8"
            >
              {batchRunning
                ? <Loader2 className="h-3.5 w-3.5 mr-1.5 animate-spin" />
                : <Send className="h-3.5 w-3.5 mr-1.5" />}
              Executar Selecionadas ({selected.size})
            </Button>
          </div>
        )}

        {/* Filtros fiscais — valores destacados no XML da nota */}
        <div className="w-full flex items-center gap-4 flex-wrap pt-1 border-t border-dashed mt-1">
          <span className="text-[11px] text-muted-foreground font-medium">Somente notas com:</span>
          <label className="flex items-center gap-1.5 text-xs cursor-pointer">
            <Checkbox checked={comIcms} onCheckedChange={c => setComIcms(c === true)} /> ICMS &gt; 0
          </label>
          <label className="flex items-center gap-1.5 text-xs cursor-pointer">
            <Checkbox checked={comSt} onCheckedChange={c => setComSt(c === true)} /> ST &gt; 0
          </label>
          <label className="flex items-center gap-1.5 text-xs cursor-pointer">
            <Checkbox checked={comDifal} onCheckedChange={c => setComDifal(c === true)} /> DIFAL &gt; 0
          </label>
          <label className="flex items-center gap-1.5 text-xs cursor-pointer">
            <Checkbox checked={comFcp} onCheckedChange={c => setComFcp(c === true)} /> FCP &gt; 0
          </label>
          <label className="flex items-center gap-1.5 text-xs cursor-pointer">
            <Checkbox checked={comBaseReduzida} onCheckedChange={c => setComBaseReduzida(c === true)} /> Base ICMS reduzida
          </label>
        </div>
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
        <>
        <div className="flex items-center justify-between text-xs text-muted-foreground px-1">
          <span>
            {total.toLocaleString('pt-BR')} nota(s) encontrada(s)
            {applied.pageSize > 0 && totalPages > 1 && ` — página ${page} de ${totalPages}`}
          </span>
          {applied.pageSize > 0 && totalPages > 1 && (
            <div className="flex items-center gap-1">
              <Button variant="outline" size="sm" className="h-6 px-2 text-xs" disabled={page <= 1 || isLoading} onClick={() => goToPage(page - 1)}>
                ← Anterior
              </Button>
              <Button variant="outline" size="sm" className="h-6 px-2 text-xs" disabled={page >= totalPages || isLoading} onClick={() => goToPage(page + 1)}>
                Próxima →
              </Button>
            </div>
          )}
        </div>
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
                <TableHead className="py-1.5 px-2 text-[11px]">Executado</TableHead>
                <TableHead className="py-1.5 px-2 text-[11px]">Divergência</TableHead>
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
                      {status !== 'idle' ? (
                        <ExecStatusBadge status={status} />
                      ) : aval[row.id]?.executado ? (
                        <Badge variant="outline" className="text-[10px] px-1.5 py-0 bg-emerald-50 text-emerald-700 border-emerald-200">OK</Badge>
                      ) : aval[row.id] ? (
                        <Badge variant="outline" className="text-[10px] px-1.5 py-0 bg-gray-50 text-gray-400 border-dashed">Nunca</Badge>
                      ) : (
                        <span className="text-[11px] text-muted-foreground">—</span>
                      )}
                    </TableCell>
                    <TableCell className="py-1 px-2">
                      {(() => {
                        const a = aval[row.id];
                        if (status === 'running' || a === undefined) {
                          return <span className="text-[11px] text-muted-foreground">—</span>;
                        }
                        if (a.divergente === true) {
                          return <Badge variant="outline" className="text-[10px] px-1.5 py-0 bg-red-50 text-red-700 border-red-200">SIM</Badge>;
                        }
                        if (a.divergente === false) {
                          return <Badge variant="outline" className="text-[10px] px-1.5 py-0 bg-emerald-50 text-emerald-700 border-emerald-200">NÃO</Badge>;
                        }
                        return <span className="text-[11px] text-muted-foreground">—</span>;
                      })()}
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
        </>
      )}
    </div>
  );
}
