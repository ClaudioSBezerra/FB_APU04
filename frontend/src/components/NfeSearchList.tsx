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
import { toast } from 'sonner';
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
  // Status agregado da execução (calculado no servidor)
  total_itens: number;
  exec_itens: number;
  itens_problema: number; // executados com status != ok
  divergente: boolean;
}

// Status do lote server-side (espelha fiscalLoteStatus do backend)
interface LoteStatus {
  ativo: boolean;
  total: number;
  processed: number;
  notas_ok: number;
  notas_parciais: number;
  notas_erro: number;
  incluir_ibs_cbs: boolean;
  done: boolean;
  iniciado_em: string;
  terminado_em?: string;
}

// Filial (CNPJ emitente) presente nas notas importadas — GET /api/fiscal/filiais
export interface FilialInfo {
  cnpj: string;
  nome: string;
  uf: string;
  notas: number;
}

// Envelope paginado da busca (espelha NfeSearchResponse do backend)
interface NfeSearchResponse {
  total: number;
  page: number;
  page_size: number; // 0 = todas
  rows: NfeSearchResult[];
}

export function NfeSearchList({
  onViewDetail,
  activeId,
  incluirIbsCbs = false,
  onIncluirIbsCbsChange,
  codEmpresa = '',
  onCodEmpresaChange,
}: {
  onViewDetail: (nfe: NfeSearchResult) => void;
  activeId?: string | null;
  // Simulação "IBS/CBS na base do ICMS" — estado vive na página
  // (ComparacaoFiscal) para valer também no botão "Executar esta nota"
  incluirIbsCbs?: boolean;
  onIncluirIbsCbsChange?: (v: boolean) => void;
  // COD_EMPRESA (filial na PRODB) p/ o lookup do grupo fiscal — string p/ o
  // input controlado; vira número (0 = derivar do CNPJ) no POST
  codEmpresa?: string;
  onCodEmpresaChange?: (v: string) => void;
}) {
  const { token, companyId } = useAuth();
  // Sem Authorization/X-Company-ID explícitos: o interceptor global do
  // AuthContext injeta o token SEMPRE FRESCO (tokenRef) — headers fixos aqui
  // congelavam o token da renderização e lotes longos morriam com 401.

  const [dataInicio, setDataInicio] = useState('');
  const [dataFim, setDataFim] = useState('');
  const [q, setQ] = useState('');
  // Filial = CNPJ do emitente (2026-07-08: "vamos testar outras filiais")
  const [filial, setFilial] = useState('');
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
  // Somente vendas (padrão): o pacote fiscal domina operações de VENDA —
  // remessas/devoluções/bonificações/transferências etc. geram "falso erro"
  const [somenteVendas, setSomenteVendas] = useState(true);
  // Resultado da execução (filtro pós-processamento): isola divergentes/erros
  type Resultado = '' | 'divergentes' | 'com_erro' | 'ok' | 'nao_executadas';
  const [resultado, setResultado] = useState<Resultado>('');
  // Paginação: 0 = todas
  const [pageSize, setPageSize] = useState(50);
  const [page, setPage] = useState(1);
  const emptyApplied = {
    dataInicio: '', dataFim: '', q: '', filial: '', ufOrigem: '', ufDestino: '', cliente: '', emitente: '',
    comIcms: false, comSt: false, comDifal: false, comFcp: false, comBaseReduzida: false,
    somenteVendas: true, resultado: '' as Resultado, pageSize: 50,
  };
  const [applied, setApplied] = useState(emptyApplied);
  const [searched, setSearched] = useState(false);
  const [selected, setSelected] = useState<Set<string>>(new Set());

  // Filiais (CNPJs emitentes) presentes nas notas importadas — popula o select
  const { data: filiais } = useQuery<FilialInfo[]>({
    queryKey: ['fiscal-filiais', companyId],
    queryFn: async () => {
      const res = await fetch('/api/fiscal/filiais');
      if (!res.ok) throw new Error(res.statusText);
      return res.json();
    },
    staleTime: 5 * 60 * 1000,
  });

  const { data, isLoading, isError, refetch } = useQuery<NfeSearchResponse>({
    queryKey: ['nfe-saidas-search', applied, page],
    queryFn: async () => {
      const params = new URLSearchParams();
      if (applied.q) params.set('q', applied.q);
      if (applied.dataInicio) params.set('data_inicio', applied.dataInicio);
      if (applied.dataFim) params.set('data_fim', applied.dataFim);
      if (applied.filial) params.set('filial', applied.filial);
      if (applied.ufOrigem) params.set('uf_origem', applied.ufOrigem);
      if (applied.ufDestino) params.set('uf_destino', applied.ufDestino);
      if (applied.cliente) params.set('cliente', applied.cliente);
      if (applied.emitente) params.set('emitente', applied.emitente);
      if (applied.comIcms) params.set('com_icms', '1');
      if (applied.comSt) params.set('com_st', '1');
      if (applied.comDifal) params.set('com_difal', '1');
      if (applied.comFcp) params.set('com_fcp', '1');
      if (applied.comBaseReduzida) params.set('com_base_reduzida', '1');
      if (applied.somenteVendas) params.set('somente_vendas', '1');
      if (applied.resultado) params.set('resultado', applied.resultado);
      params.set('page', String(page));
      params.set('page_size', String(applied.pageSize));
      const res = await fetch(`/api/fiscal/comparacao/search?${params}`);
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
      dataInicio, dataFim, q: q.trim(), filial, ufOrigem, ufDestino,
      cliente: cliente.trim(), emitente: emitente.trim(),
      comIcms, comSt, comDifal, comFcp, comBaseReduzida, somenteVendas, resultado, pageSize,
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

  // ── Lote SERVER-SIDE (2026-07-08) ──────────────────────────────────────────
  // O lote roda como job no servidor (sobrevive a logout/refresh). A tela só
  // dispara e acompanha por polling; ao logar de novo, o status do job da
  // empresa é retomado automaticamente. As colunas Executado/Divergência vêm
  // prontas do servidor na própria busca (qualquer volume).
  const [lote, setLote] = useState<LoteStatus | null>(null);
  const pollRef = useRef<number | null>(null);

  const pararPolling = () => {
    if (pollRef.current !== null) {
      window.clearInterval(pollRef.current);
      pollRef.current = null;
    }
  };

  const consultarLote = async (): Promise<LoteStatus | null> => {
    try {
      const res = await fetch('/api/fiscal/execute-lote/status');
      if (!res.ok) return null;
      return await res.json();
    } catch {
      return null;
    }
  };

  const iniciarPolling = () => {
    pararPolling();
    let ticks = 0;
    pollRef.current = window.setInterval(async () => {
      const st = await consultarLote();
      if (!st) return;
      setLote(st);
      ticks++;
      // Atualiza o grid (colunas do servidor) a cada ~10s durante o lote
      if (st.done || ticks % 5 === 0) refetch();
      if (st.done) {
        pararPolling();
        toast.success(`Lote concluído: ${st.notas_ok} OK, ${st.notas_parciais} parciais, ${st.notas_erro} com erro.`);
      }
    }, 2000);
  };

  // Retomada: ao montar a tela (login/refresh), verifica se há lote em
  // andamento para a empresa e religa a barra de progresso.
  useEffect(() => {
    let cancelado = false;
    consultarLote().then(st => {
      if (cancelado || !st) return;
      setLote(st);
      if (!st.done) iniciarPolling();
    });
    return () => { cancelado = true; pararPolling(); };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const executeSelected = async () => {
    const ids = Array.from(selected);
    if (ids.length === 0) return;
    try {
      const res = await fetch('/api/fiscal/execute-lote', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ nfe_ids: ids, incluir_ibs_cbs_base: incluirIbsCbs, cod_empresa: Number(codEmpresa) || 0 }),
      });
      const st: LoteStatus = await res.json();
      if (res.status === 409) {
        toast.error('Já existe um lote em andamento para esta empresa — acompanhe a barra de progresso.');
        setLote(st);
        iniciarPolling();
        return;
      }
      if (!res.ok) {
        toast.error('Falha ao iniciar o lote.');
        return;
      }
      setLote(st);
      setSelected(new Set());
      iniciarPolling();
      toast.success(`Lote de ${ids.length} nota(s) iniciado no servidor — pode fechar a tela, ele continua.`);
    } catch {
      toast.error('Falha ao iniciar o lote.');
    }
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
          <label className="text-[11px] text-muted-foreground" title="CNPJ emitente das notas importadas — use para testar o pacote filial a filial">Filial</label>
          <select
            value={filial}
            onChange={e => setFilial(e.target.value)}
            className="h-8 w-52 text-xs rounded-md border bg-background px-2"
          >
            <option value="">Todas</option>
            {(filiais ?? []).map(f => (
              <option key={f.cnpj} value={f.cnpj}>
                {f.nome ? `${f.nome} — ` : ''}{f.cnpj} ({f.uf}) · {f.notas}
              </option>
            ))}
          </select>
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
          <label className="text-[11px] text-muted-foreground" title="O pacote fiscal domina operações de VENDA (CFOP 5.1xx/6.1xx exceto transferências + vendas ST). Remessas, devoluções, bonificações, consertos e transferências geram 'falso erro' — nota com qualquer item fora de venda fica de fora quando 'Somente vendas' está ativo">Operações</label>
          <select
            value={somenteVendas ? 'vendas' : 'todas'}
            onChange={e => setSomenteVendas(e.target.value === 'vendas')}
            className="h-8 w-36 text-xs rounded-md border bg-background px-2"
          >
            <option value="vendas">Somente vendas</option>
            <option value="todas">Todas</option>
          </select>
        </div>
        <div className="flex flex-col gap-1">
          <label className="text-[11px] text-muted-foreground" title="Filtra pelo VEREDITO da execução — mesmo critério das colunas Executado/Divergência. Use após processar um lote para isolar o que precisa de atenção">Resultado</label>
          <select
            value={resultado}
            onChange={e => setResultado(e.target.value as Resultado)}
            className="h-8 w-40 text-xs rounded-md border bg-background px-2"
          >
            <option value="">Todos</option>
            <option value="divergentes">Somente divergentes</option>
            <option value="com_erro">Com erro / sem grupo</option>
            <option value="ok">OK sem divergência</option>
            <option value="nao_executadas">Não executadas</option>
          </select>
        </div>
        <Button size="sm" onClick={handleSearch} disabled={isLoading} className="h-8">
          {isLoading ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Search className="h-3.5 w-3.5 mr-1.5" />}
          Buscar
        </Button>
        {rows.length > 0 && (
          <div className="flex items-end gap-3 ml-auto">
            <div className="flex flex-col gap-1">
              <label className="text-[11px] text-muted-foreground" title="COD_EMPRESA da filial na PRODB — define qual empresa é usada na busca do GRUPO_FISCAL (que carrega a regra no pacote). Deixe vazio para derivar do CNPJ. A raiz do CNPJ é a mesma em todas as filiais FC, então informe o código da filial que está testando.">COD_EMPRESA (filial)</label>
              <Input
                type="number"
                inputMode="numeric"
                placeholder="deriva do CNPJ"
                value={codEmpresa}
                onChange={e => onCodEmpresaChange?.(e.target.value)}
                className="h-8 w-32 text-xs"
              />
            </div>
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
              disabled={!someSelected || (lote !== null && !lote.done)}
              className="h-8"
            >
              {lote !== null && !lote.done
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

      {/* Lote server-side em andamento/concluído — sobrevive a logout/refresh */}
      {lote && (
        <div className={`rounded-md border px-3 py-2 text-xs ${lote.done ? 'bg-emerald-50 border-emerald-200' : 'bg-sky-50 border-sky-200'}`}>
          <div className="flex items-center justify-between mb-1">
            <span className="font-medium">
              {lote.done
                ? `Lote concluído: ${lote.processed.toLocaleString('pt-BR')} nota(s) — ${lote.notas_ok} OK, ${lote.notas_parciais} parciais, ${lote.notas_erro} com erro`
                : `Lote em andamento no servidor: ${lote.processed.toLocaleString('pt-BR')} de ${lote.total.toLocaleString('pt-BR')} nota(s) — continua mesmo se você sair da tela`}
            </span>
            {!lote.done && <Loader2 className="h-3.5 w-3.5 animate-spin text-sky-600" />}
          </div>
          <div className="h-2 w-full rounded bg-white border overflow-hidden">
            <div
              className={`h-full ${lote.done ? 'bg-emerald-500' : 'bg-sky-500'}`}
              style={{ width: `${lote.total > 0 ? Math.round(lote.processed / lote.total * 100) : 0}%` }}
            />
          </div>
        </div>
      )}

      {!searched ? (
        <p className="text-sm text-muted-foreground text-center py-6">
          Clique em "Buscar" para listar as notas mais recentes, ou aplique um filtro primeiro.
        </p>
      ) : isError && rows.length === 0 ? (
        <p className="text-sm text-destructive text-center py-6">
          Erro ao buscar NF-e. <button className="underline" onClick={() => refetch()}>Tentar novamente</button>
        </p>
      ) : isLoading ? (
        <p className="text-sm text-muted-foreground text-center py-6">Buscando...</p>
      ) : rows.length === 0 ? (
        <p className="text-sm text-muted-foreground text-center py-6">Nenhuma nota encontrada para os filtros informados.</p>
      ) : (
        <>
        {isError && (
          <p className="text-xs text-amber-700 bg-amber-50 border border-amber-200 rounded px-2 py-1">
            A última atualização da busca falhou (transitório) — exibindo o resultado anterior.{' '}
            <button className="underline" onClick={() => refetch()}>Atualizar</button>
          </p>
        )}
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
                const executado = row.exec_itens > 0;
                const completo = executado && row.exec_itens >= row.total_itens;
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
                      {!executado ? (
                        <Badge variant="outline" className="text-[10px] px-1.5 py-0 bg-gray-50 text-gray-400 border-dashed">Nunca</Badge>
                      ) : row.itens_problema > 0 ? (
                        <Badge variant="outline" className="text-[10px] px-1.5 py-0 bg-amber-50 text-amber-700 border-amber-200" title={`${row.itens_problema} item(ns) com erro/sem grupo fiscal`}>Parcial</Badge>
                      ) : completo ? (
                        <Badge variant="outline" className="text-[10px] px-1.5 py-0 bg-emerald-50 text-emerald-700 border-emerald-200">OK</Badge>
                      ) : (
                        <Badge variant="outline" className="text-[10px] px-1.5 py-0 bg-amber-50 text-amber-700 border-amber-200">{row.exec_itens}/{row.total_itens}</Badge>
                      )}
                    </TableCell>
                    <TableCell className="py-1 px-2">
                      {!executado ? (
                        <span className="text-[11px] text-muted-foreground">—</span>
                      ) : row.divergente ? (
                        <Badge variant="outline" className="text-[10px] px-1.5 py-0 bg-red-50 text-red-700 border-red-200">SIM</Badge>
                      ) : (
                        <Badge variant="outline" className="text-[10px] px-1.5 py-0 bg-emerald-50 text-emerald-700 border-emerald-200">NÃO</Badge>
                      )}
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
