// ComparacaoFiscal.tsx — tela "Comparação Fiscal" (Fase 12, TPF-06/TPF-07).
//
// Busca NF-e de saída por período/número (NfeSearchList — filtro visível +
// seleção múltipla + execução em lote), dispara a execução do pacote fiscal
// (POST /api/fiscal/execute, Fase 11) e recarrega automaticamente a
// comparação esperado (nfe_saidas_itens) x calculado (fiscal_execution_items)
// na mesma tela para a nota em visualização — sem navegação extra (D-01/D-02,
// CONTEXT.md).
//
// Composição de 3 análogos: shell de ConciliacaoBridgeXML.tsx (cards, filtro,
// tabela densa, badges, export) + mutation trigger-then-reload de
// ImportarViaERP.tsx + Dialog de detalhe de ConsultaNFeSaidas.tsx.
//
// Regra de divergência (BINDING, UI-SPEC "Divergence Visual Rules"):
// tolerância ZERO — abs(esperado - calculado) !== 0. NÃO reusar o threshold
// de um centavo de ConciliacaoBridgeXML.tsx (essa tela é validação de pacote
// fiscal, não reconciliação ERP-vs-XML).
import { Fragment, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { useAuth } from '@/contexts/AuthContext';
import { NfeSearchList, type NfeSearchResult } from '@/components/NfeSearchList';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import { Label } from '@/components/ui/label';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import {
  Search,
  HelpCircle,
  AlertTriangle,
  CheckCircle,
  Download,
  FileSpreadsheet,
  Loader2,
  Send,
} from 'lucide-react';
import { exportToExcel } from '@/lib/exportToExcel';

// ---------------------------------------------------------------------------
// Types (espelham backend/handlers/fiscal_comparacao.go — ComparacaoRow)
// ---------------------------------------------------------------------------
type StatusFiscal = 'ok' | 'error' | 'sem_grupo_fiscal' | 'pending' | 'not_executed';

interface ComparacaoRow {
  id: string;
  n_item: number;
  c_prod: string;
  x_prod: string;
  ncm: string;
  cfop: string;
  // Esperado (nfe_saidas_itens)
  v_bc_icms: number;
  v_icms: number;
  v_bc_st: number;
  v_st: number;
  v_bc_pis: number;
  v_pis: number;
  v_bc_cofins: number;
  v_cofins: number;
  v_ibs: number;
  v_cbs: number;
  // Calculado (fiscal_execution_items) — null quando nunca executado
  status: StatusFiscal;
  error_message: string | null;
  executed_at: string | null;
  base_calculo_icms: number | null;
  valor_icms: number | null;
  base_substituicao: number | null;
  valor_substituicao: number | null;
  base_calculo_pis: number | null;
  valor_pis: number | null;
  base_calculo_cofins: number | null;
  valor_cofins: number | null;
  valor_ibs_total: number | null;
  valor_cbs: number | null;
  percentual_difal: number | null;
  valor_icms_partilha_destino: number | null;
  valor_icms_pobreza: number | null;
  grupo_fiscal_codigo: string | null;
  // Base de cálculo compartilhada entre IBS e CBS — só existe lado
  // calculado (extraído do full_result do pacote Oracle); sem "esperado"
  // correspondente no XML, por isso não entra em getTaxPairs/divergência.
  base_calculo_ibs_cbs: number | null;
}

interface ExecuteSummary {
  total: number;
  ok: number;
  sem_grupo_fiscal: number;
  error: number;
}

type TaxKey = 'icms' | 'icms_st' | 'pis' | 'cofins' | 'ibs' | 'cbs';
type RowBadge = 'ok' | 'divergente' | 'nao_calculado' | 'nunca_executado';

interface TaxPairDef {
  key: TaxKey;
  label: string;
  baseEsperado?: number;
  baseCalculado?: number | null;
  valorEsperado: number;
  valorCalculado: number | null;
}

const TAX_LABELS: Record<TaxKey, string> = {
  icms: 'ICMS',
  icms_st: 'ICMS-ST',
  pis: 'PIS',
  cofins: 'COFINS',
  ibs: 'IBS',
  cbs: 'CBS',
};

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
function fmtBRL(v: number | null | undefined): string {
  if (v == null) return '—';
  return v.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' });
}

function getTaxPairs(row: ComparacaoRow): TaxPairDef[] {
  return [
    { key: 'icms', label: 'ICMS', baseEsperado: row.v_bc_icms, baseCalculado: row.base_calculo_icms, valorEsperado: row.v_icms, valorCalculado: row.valor_icms },
    { key: 'icms_st', label: 'ICMS-ST', baseEsperado: row.v_bc_st, baseCalculado: row.base_substituicao, valorEsperado: row.v_st, valorCalculado: row.valor_substituicao },
    { key: 'pis', label: 'PIS', baseEsperado: row.v_bc_pis, baseCalculado: row.base_calculo_pis, valorEsperado: row.v_pis, valorCalculado: row.valor_pis },
    { key: 'cofins', label: 'COFINS', baseEsperado: row.v_bc_cofins, baseCalculado: row.base_calculo_cofins, valorEsperado: row.v_cofins, valorCalculado: row.valor_cofins },
    { key: 'ibs', label: 'IBS', valorEsperado: row.v_ibs, valorCalculado: row.valor_ibs_total },
    { key: 'cbs', label: 'CBS', valorEsperado: row.v_cbs, valorCalculado: row.valor_cbs },
  ];
}

// Tolerância ZERO — abs(esperado - calculado) !== 0 (base e valor).
// NÃO usar o threshold de um centavo de ConciliacaoBridgeXML.tsx — não se aplica aqui.
function isPairDivergente(pair: TaxPairDef): boolean {
  const diffValor = Math.abs((pair.valorEsperado ?? 0) - (pair.valorCalculado ?? 0));
  if (diffValor !== 0) return true;
  if (pair.baseEsperado !== undefined) {
    const diffBase = Math.abs((pair.baseEsperado ?? 0) - (pair.baseCalculado ?? 0));
    if (diffBase !== 0) return true;
  }
  return false;
}

// Precedência de status: item com status != 'ok' NUNCA é avaliado como
// divergente (evita falso positivo) — 4 estados de badge (not_executed é o
// 4º estado, distinto de "Não calculado", herança do LEFT JOIN implícito).
function getRowBadge(row: ComparacaoRow): { badge: RowBadge; divergentTaxes: TaxKey[] } {
  if (row.status === 'not_executed') return { badge: 'nunca_executado', divergentTaxes: [] };
  if (row.status !== 'ok') return { badge: 'nao_calculado', divergentTaxes: [] };
  const divergentTaxes = getTaxPairs(row).filter(isPairDivergente).map(p => p.key);
  return { badge: divergentTaxes.length > 0 ? 'divergente' : 'ok', divergentTaxes };
}

function StatusBadge({ row }: { row: ComparacaoRow }) {
  const { badge } = getRowBadge(row);
  if (badge === 'ok') {
    return <Badge variant="outline" className="text-[10px] px-1.5 py-0 bg-gray-50 text-muted-foreground">OK</Badge>;
  }
  if (badge === 'divergente') {
    return <Badge variant="outline" className="text-[10px] px-1.5 py-0 bg-red-50 text-red-700 border-red-200">Divergente</Badge>;
  }
  if (badge === 'nunca_executado') {
    return (
      <TooltipProvider delayDuration={150}>
        <Tooltip>
          <TooltipTrigger asChild>
            <Badge variant="outline" className="text-[10px] px-1.5 py-0 bg-gray-50 text-gray-400 border-gray-200 border-dashed cursor-help">
              Nunca executado
            </Badge>
          </TooltipTrigger>
          <TooltipContent side="top" className="text-xs max-w-xs">
            Nota ainda não executada — clique em Executar.
          </TooltipContent>
        </Tooltip>
      </TooltipProvider>
    );
  }
  // nao_calculado (error / sem_grupo_fiscal / pending)
  return (
    <TooltipProvider delayDuration={150}>
      <Tooltip>
        <TooltipTrigger asChild>
          <Badge variant="outline" className="text-[10px] px-1.5 py-0 bg-slate-100 text-slate-600 border-slate-200 cursor-help">
            Não calculado
          </Badge>
        </TooltipTrigger>
        <TooltipContent side="top" className="text-xs max-w-xs">
          {row.status}
          {row.error_message ? ` — ${row.error_message}` : ''}
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}

function DiferencaBadge({ diferenca, divergente }: { diferenca: number; divergente: boolean }) {
  return (
    <Badge
      variant="outline"
      className={divergente
        ? 'text-[10px] px-1.5 py-0 bg-red-50 text-red-700 border-red-200'
        : 'text-[10px] px-1.5 py-0 bg-gray-50 text-muted-foreground'}
    >
      {fmtBRL(diferenca)}
    </Badge>
  );
}

// ---------------------------------------------------------------------------
// Detail Dialog — 3 seções: Identificação / Comparação / Só calculado
// ---------------------------------------------------------------------------
function DetalheItem({ row, onClose }: { row: ComparacaoRow; onClose: () => void }) {
  const Linha = ({ label, value }: { label: string; value: string | number | null | undefined }) => (
    <div className="flex justify-between py-0.5 border-b border-dashed last:border-0">
      <span className="text-[11px] text-muted-foreground w-40 shrink-0">{label}</span>
      <span className="text-[11px] font-medium text-right">{value ?? '—'}</span>
    </div>
  );

  const Secao = ({ title, children }: { title: string; children: React.ReactNode }) => (
    <div className="mb-2">
      <h3 className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground mb-1 pb-0.5 border-b">
        {title}
      </h3>
      {children}
    </div>
  );

  const pairs = getTaxPairs(row);
  const naoOk = row.status !== 'ok';

  return (
    <Dialog open onOpenChange={onClose}>
      <DialogContent className="max-w-2xl max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="text-xs">
            Item {row.n_item} — {row.x_prod}
          </DialogTitle>
        </DialogHeader>
        <div className="space-y-1 mt-1">
          <Secao title="Identificação">
            <Linha label="Item" value={row.n_item} />
            <Linha label="Produto" value={row.c_prod ? `${row.c_prod} — ${row.x_prod}` : row.x_prod} />
            <Linha label="NCM" value={row.ncm} />
            <Linha label="CFOP" value={row.cfop} />
          </Secao>

          <Secao title="Comparação (precisão total)">
            {naoOk ? (
              <p className="text-[11px] text-muted-foreground italic">
                Item não calculado — sem comparação de valores.
              </p>
            ) : (
              pairs.map(pair => {
                const diferenca = (pair.valorEsperado ?? 0) - (pair.valorCalculado ?? 0);
                return (
                  <div key={pair.key} className="mb-1.5">
                    <p className="text-[10px] font-semibold text-muted-foreground">{pair.label}</p>
                    {pair.baseEsperado !== undefined && (
                      <Linha label="Base Esperada / Calculada" value={`${fmtBRL(pair.baseEsperado)} / ${fmtBRL(pair.baseCalculado)}`} />
                    )}
                    <Linha label="Valor Esperado" value={fmtBRL(pair.valorEsperado)} />
                    <Linha label="Valor Calculado" value={fmtBRL(pair.valorCalculado)} />
                    <Linha label="Diferença" value={fmtBRL(diferenca)} />
                  </div>
                );
              })
            )}
          </Secao>

          <Secao title="Só calculado">
            <p className="text-[10px] text-muted-foreground italic mb-1">
              Campos abaixo só existem no retorno do pacote fiscal — sem valor esperado correspondente no XML.
            </p>
            <Linha label="% DIFAL" value={row.percentual_difal != null ? `${row.percentual_difal}%` : null} />
            <Linha label="ICMS Partilha Destino" value={fmtBRL(row.valor_icms_partilha_destino)} />
            <Linha label="ICMS Pobreza (FCP)" value={fmtBRL(row.valor_icms_pobreza)} />
            <Linha label="Base IBS/CBS" value={fmtBRL(row.base_calculo_ibs_cbs)} />
            <Linha label="Grupo Fiscal" value={row.grupo_fiscal_codigo} />
          </Secao>
        </div>
      </DialogContent>
    </Dialog>
  );
}

// ---------------------------------------------------------------------------
// ComparacaoFiscal — página principal
// ---------------------------------------------------------------------------
export default function ComparacaoFiscal() {
  const { token, companyId } = useAuth();
  const queryClient = useQueryClient();

  const [selectedNfe, setSelectedNfe] = useState<NfeSearchResult | null>(null);
  const [showOnlyDivergent, setShowOnlyDivergent] = useState(false);
  const [selectedItem, setSelectedItem] = useState<ComparacaoRow | null>(null);
  const [downloadingCSV, setDownloadingCSV] = useState(false);

  const authHeaders = { Authorization: `Bearer ${token}`, 'X-Company-ID': companyId || '' };
  const nfeId = selectedNfe?.id ?? null;

  const {
    data,
    isLoading,
    isError,
    refetch,
  } = useQuery<ComparacaoRow[]>({
    queryKey: ['fiscal-comparacao', nfeId],
    queryFn: async () => {
      const res = await fetch(`/api/fiscal/comparacao?nfe_id=${encodeURIComponent(nfeId as string)}`, {
        headers: authHeaders,
      });
      if (!res.ok) throw new Error(`Erro ${res.status}`);
      return res.json();
    },
    enabled: !!nfeId,
  });

  const executar = useMutation({
    mutationFn: async (id: string) => {
      const res = await fetch('/api/fiscal/execute', {
        method: 'POST',
        headers: { ...authHeaders, 'Content-Type': 'application/json' },
        body: JSON.stringify({ nfe_id: id }),
      });
      if (!res.ok) throw new Error((await res.text()) || 'Erro ao executar');
      return res.json() as Promise<ExecuteSummary>;
    },
    onSuccess: (summary, id) => {
      toast.success(`Execução concluída: ${summary.ok}/${summary.total} OK.`);
      queryClient.invalidateQueries({ queryKey: ['fiscal-comparacao', id] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const rows = useMemo(() => data ?? [], [data]);

  const summary = useMemo(() => {
    let semDivergencia = 0;
    let divergentes = 0;
    let naoCalculados = 0;
    const porImposto: Record<TaxKey, number> = { icms: 0, icms_st: 0, pis: 0, cofins: 0, ibs: 0, cbs: 0 };

    rows.forEach(row => {
      const { badge, divergentTaxes } = getRowBadge(row);
      if (badge === 'ok') semDivergencia++;
      else if (badge === 'divergente') divergentes++;
      else naoCalculados++;

      divergentTaxes.forEach(key => { porImposto[key]++; });
    });

    return { total: rows.length, semDivergencia, divergentes, naoCalculados, porImposto };
  }, [rows]);

  const displayRows = useMemo(
    () => (showOnlyDivergent ? rows.filter(r => getRowBadge(r).badge === 'divergente') : rows),
    [rows, showOnlyDivergent],
  );

  const handleExportExcel = () => {
    if (!rows.length) return;
    const excelData = rows.map(row => {
      const pairs = getTaxPairs(row);
      const record: Record<string, unknown> = {
        'Nº Nota': selectedNfe ? `${selectedNfe.numero_nfe}${selectedNfe.serie ? `/${selectedNfe.serie}` : ''}` : '',
        'Cliente': selectedNfe?.dest_nome ?? '',
        'Item': row.n_item,
        'Produto': row.x_prod,
        'NCM': row.ncm,
        'CFOP': row.cfop,
        'Status': row.status,
      };
      pairs.forEach(pair => {
        const diferenca = (pair.valorEsperado ?? 0) - (pair.valorCalculado ?? 0);
        record[`${pair.label} Esperado`] = pair.valorEsperado ?? 0;
        record[`${pair.label} Calculado`] = pair.valorCalculado ?? 0;
        record[`${pair.label} Diferença`] = diferenca;
      });
      record['Base IBS/CBS Calculado'] = row.base_calculo_ibs_cbs ?? 0;
      return record;
    });
    exportToExcel(excelData, `comparacao-fiscal-${selectedNfe?.numero_nfe ?? nfeId}`, 'Comparação Fiscal');
    toast.success('Excel exportado com sucesso');
  };

  const handleExportCSV = async () => {
    if (!nfeId) return;
    setDownloadingCSV(true);
    try {
      const res = await fetch(`/api/fiscal/comparacao/csv?nfe_id=${encodeURIComponent(nfeId)}`, {
        headers: authHeaders,
      });
      if (!res.ok) throw new Error(`Erro ${res.status}`);
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'comparacao-fiscal.csv';
      a.click();
      URL.revokeObjectURL(url);
      toast.success('CSV exportado com sucesso');
    } catch (err) {
      toast.error('Erro ao exportar: ' + (err instanceof Error ? err.message : 'Desconhecido'));
    } finally {
      setDownloadingCSV(false);
    }
  };

  const IbsCbsHeaderTooltip = () => (
    <TooltipProvider delayDuration={150}>
      <Tooltip>
        <TooltipTrigger asChild>
          <HelpCircle className="h-3 w-3 ml-1 inline text-muted-foreground cursor-help" />
        </TooltipTrigger>
        <TooltipContent side="top" className="text-xs max-w-xs">
          Campo v_ibs/v_cbs do XML pode aparecer zerado — parser atual de nfe_saidas_itens
          nem sempre popula esses campos. Divergência aqui pode refletir dado de origem
          ausente, não necessariamente um erro do pacote fiscal.
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );

  const BaseIbsCbsHeaderTooltip = () => (
    <TooltipProvider delayDuration={150}>
      <Tooltip>
        <TooltipTrigger asChild>
          <HelpCircle className="h-3 w-3 ml-1 inline text-muted-foreground cursor-help" />
        </TooltipTrigger>
        <TooltipContent side="top" className="text-xs max-w-xs">
          Base de cálculo compartilhada entre IBS e CBS, retornada pelo pacote fiscal
          Oracle. Não existe um valor "esperado" equivalente no XML da nota — é só
          informativo, sem comparação de divergência.
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <div className="flex items-center gap-2">
          <h1 className="text-xl font-semibold">Comparação Fiscal</h1>
          <TooltipProvider delayDuration={200}>
            <Tooltip>
              <TooltipTrigger asChild>
                <HelpCircle className="h-4 w-4 text-muted-foreground hover:text-foreground cursor-help shrink-0" />
              </TooltipTrigger>
              <TooltipContent side="right" className="max-w-sm text-xs space-y-2 p-3">
                <p className="font-semibold">O que é esta tela?</p>
                <p>
                  Compara o valor <strong>esperado</strong> (XML da nota, `nfe_saidas_itens`) com o
                  valor <strong>calculado</strong> pelo pacote fiscal Oracle (`PKG_FISCAL_FCTAX`)
                  para ICMS, ICMS-ST, PIS, COFINS, IBS e CBS, item a item.
                </p>
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
        </div>
        <p className="text-sm text-muted-foreground mt-1">
          Compare o valor esperado (XML da nota) com o valor calculado pelo pacote fiscal
          (PKG_FISCAL_FCTAX) para ICMS, ICMS-ST, PIS, COFINS, IBS e CBS.
        </p>
      </div>

      {/* Busca de NF-e: filtro por período/número visível na página, lista com
          seleção múltipla e execução em lote do pacote fiscal */}
      <NfeSearchList onViewDetail={setSelectedNfe} activeId={nfeId} />

      {selectedNfe && (
        <div className="flex items-center gap-3 flex-wrap border-t pt-3">
          <span className="text-xs text-muted-foreground">
            Visualizando: Nº {selectedNfe.numero_nfe}/{selectedNfe.serie} — {selectedNfe.dest_nome} — {selectedNfe.data_emissao}
            <span className="font-mono ml-2">{selectedNfe.chave_nfe.slice(0, 8)}...{selectedNfe.chave_nfe.slice(-6)}</span>
          </span>
          <Button onClick={() => nfeId && executar.mutate(nfeId)} disabled={!nfeId || executar.isPending} size="sm" variant="outline">
            {executar.isPending
              ? <Loader2 className="h-4 w-4 mr-1.5 animate-spin" />
              : <Send className="h-4 w-4 mr-1.5" />}
            Executar esta nota
          </Button>
        </div>
      )}

      {!nfeId ? null : isLoading ? (
        <p className="text-sm text-muted-foreground text-center py-8">Carregando comparação fiscal...</p>
      ) : isError ? (
        <p className="text-sm text-destructive px-4 py-6">
          Erro ao carregar dados de comparação fiscal.{' '}
          <button className="underline" onClick={() => refetch()}>Tentar novamente</button>
        </p>
      ) : rows.length === 0 ? (
        <p className="text-sm text-muted-foreground px-4 py-6">
          Nenhum item de `nfe_saidas_itens` possui resultado do pacote fiscal para os filtros atuais.
          Execute o processamento em lote (endpoint `/api/fiscal/execute`) para gerar comparações.
        </p>
      ) : (
        <>
          {/* 4 cards de resumo */}
          <div className="grid grid-cols-1 sm:grid-cols-4 gap-4">
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-semibold text-muted-foreground">Total de Itens</CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-xl font-semibold">{summary.total}</p>
              </CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-semibold text-muted-foreground">Sem Divergência</CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-xl font-semibold">{summary.semDivergencia}</p>
              </CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-semibold text-muted-foreground">Divergentes</CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-xl font-semibold">{summary.divergentes}</p>
              </CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-semibold text-muted-foreground">Não Calculados</CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-xl font-semibold">{summary.naoCalculados}</p>
              </CardContent>
            </Card>
          </div>

          {/* 6 chips por imposto */}
          <div className="flex items-center gap-2 flex-wrap">
            {(Object.keys(TAX_LABELS) as TaxKey[]).map(key => {
              const count = summary.porImposto[key];
              const pct = summary.total > 0 ? (count / summary.total) * 100 : 0;
              return (
                <Badge
                  key={key}
                  variant="outline"
                  className={count > 0
                    ? 'text-[11px] px-2 py-0.5 bg-red-50 text-red-700 border-red-200'
                    : 'text-[11px] px-2 py-0.5 bg-gray-50 text-muted-foreground'}
                >
                  {TAX_LABELS[key]}: {count} ({pct.toFixed(1)}%)
                </Badge>
              );
            })}
          </div>

          {/* Filtro só divergentes */}
          <div className="flex items-center gap-2 rounded-md border px-2 py-1 w-fit">
            <Switch id="so-divergentes" checked={showOnlyDivergent} onCheckedChange={setShowOnlyDivergent} />
            <Label htmlFor="so-divergentes" className="text-xs cursor-pointer whitespace-nowrap">Só divergentes</Label>
          </div>

          {/* Tabela ou estado de filtro vazio */}
          {showOnlyDivergent && displayRows.length === 0 ? (
            <div className="flex items-center gap-2 px-4 py-6 text-sm text-muted-foreground">
              <CheckCircle className="w-4 h-4 text-green-500" />
              Nenhuma divergência encontrada. Todos os itens executados batem entre o XML e o pacote fiscal.
            </div>
          ) : (
            <div className="overflow-x-auto rounded-md border">
              <Table>
                <TableHeader>
                  <TableRow className="hover:bg-transparent bg-muted/30">
                    <TableHead className="py-1.5 px-2 text-[11px]">Nº Nota</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px]">Cliente</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px] text-center">Item</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px]">Produto</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px] border-l">Status</TableHead>
                    {(Object.keys(TAX_LABELS) as TaxKey[]).map(key => (
                      <TableHead key={key} colSpan={3} className="py-1.5 px-2 text-[11px] text-center border-l">
                        {TAX_LABELS[key]}
                        {(key === 'ibs' || key === 'cbs') && <IbsCbsHeaderTooltip />}
                      </TableHead>
                    ))}
                    <TableHead className="py-1.5 px-2 text-[11px] text-right border-l">
                      Base IBS/CBS
                      <BaseIbsCbsHeaderTooltip />
                    </TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px]"></TableHead>
                  </TableRow>
                  <TableRow className="hover:bg-transparent bg-muted/10">
                    <TableHead className="py-1 px-2 text-[10px]"></TableHead>
                    <TableHead className="py-1 px-2 text-[10px]"></TableHead>
                    <TableHead className="py-1 px-2 text-[10px]"></TableHead>
                    <TableHead className="py-1 px-2 text-[10px]"></TableHead>
                    <TableHead className="py-1 px-2 text-[10px] border-l"></TableHead>
                    {(Object.keys(TAX_LABELS) as TaxKey[]).map(key => (
                      <Fragment key={key}>
                        <TableHead className="py-1 px-2 text-[10px] text-right border-l">Esperado</TableHead>
                        <TableHead className="py-1 px-2 text-[10px] text-right">Calculado</TableHead>
                        <TableHead className="py-1 px-2 text-[10px] text-right">Diferença</TableHead>
                      </Fragment>
                    ))}
                    <TableHead className="py-1 px-2 text-[10px] text-right border-l">Calculado</TableHead>
                    <TableHead className="py-1 px-2 text-[10px]"></TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {displayRows.map(row => {
                    const { badge } = getRowBadge(row);
                    const rowTint = badge === 'divergente'
                      ? 'bg-red-50 hover:bg-red-100'
                      : badge === 'nao_calculado' || badge === 'nunca_executado'
                        ? 'bg-slate-50'
                        : '';
                    const naoOk = row.status !== 'ok';
                    const pairs = getTaxPairs(row);
                    return (
                      <TableRow key={row.id} className={rowTint}>
                        <TableCell className="py-1 px-2 text-[11px] font-medium whitespace-nowrap">
                          {selectedNfe ? `${selectedNfe.numero_nfe}${selectedNfe.serie ? `/${selectedNfe.serie}` : ''}` : '—'}
                        </TableCell>
                        <TableCell className="py-1 px-2 text-[11px] truncate max-w-[180px]">
                          {selectedNfe?.dest_nome ?? '—'}
                        </TableCell>
                        <TableCell className="py-1 px-2 text-[11px] text-center">{row.n_item}</TableCell>
                        <TableCell className="py-1 px-2 text-[11px] truncate max-w-[200px]">{row.x_prod}</TableCell>
                        <TableCell className="py-1 px-2 border-l">
                          <StatusBadge row={row} />
                        </TableCell>
                        {pairs.map(pair => {
                          const diferenca = (pair.valorEsperado ?? 0) - (pair.valorCalculado ?? 0);
                          const divergente = !naoOk && isPairDivergente(pair);
                          return (
                            <Fragment key={pair.key}>
                              <TableCell className="py-1 px-2 text-right text-[11px] font-semibold border-l">
                                {fmtBRL(pair.valorEsperado)}
                              </TableCell>
                              <TableCell className="py-1 px-2 text-right text-[11px] text-muted-foreground">
                                {naoOk ? '—' : fmtBRL(pair.valorCalculado)}
                              </TableCell>
                              <TableCell className="py-1 px-2 text-right">
                                {naoOk ? (
                                  <span className="text-[11px] text-muted-foreground">—</span>
                                ) : (
                                  <DiferencaBadge diferenca={diferenca} divergente={divergente} />
                                )}
                              </TableCell>
                            </Fragment>
                          );
                        })}
                        <TableCell className="py-1 px-2 text-right text-[11px] text-muted-foreground border-l">
                          {naoOk ? '—' : fmtBRL(row.base_calculo_ibs_cbs)}
                        </TableCell>
                        <TableCell className="py-1 px-2">
                          <Button
                            variant="ghost"
                            size="sm"
                            className="h-6 px-1.5"
                            onClick={() => setSelectedItem(row)}
                            aria-label="Ver detalhes"
                          >
                            <Search className="h-3 w-3" />
                          </Button>
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </div>
          )}

          {/* Export */}
          <div className="flex items-center gap-2 no-print">
            <Button size="sm" variant="outline" onClick={handleExportExcel}>
              <FileSpreadsheet className="w-4 h-4 mr-1" /> Exportar Excel
            </Button>
            <Button size="sm" variant="outline" onClick={handleExportCSV} disabled={downloadingCSV}>
              <Download className="w-4 h-4 mr-1" />
              {downloadingCSV ? 'Exportando...' : 'Exportar CSV'}
            </Button>
          </div>

          {/* Legenda do threshold */}
          <p className="text-[11px] text-muted-foreground flex items-center gap-1">
            <AlertTriangle className="h-3 w-3" />
            (divergência = qualquer diferença ≠ R$ 0,00 — sem tolerância de arredondamento)
          </p>
        </>
      )}

      {selectedItem && (
        <DetalheItem row={selectedItem} onClose={() => setSelectedItem(null)} />
      )}
    </div>
  );
}
