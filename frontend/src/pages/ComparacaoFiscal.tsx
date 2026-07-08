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
import { Fragment, useEffect, useMemo, useRef, useState } from 'react';
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

// Tipos e regras de divergência compartilhados com o grid (NfeSearchList):
// fonte única em lib/fiscalComparacao.ts.
import {
  type ComparacaoRow,
  type Simulacao,
  type TaxKey,
  type ResumoKey,
  TAX_LABELS,
  RESUMO_LABELS,
  getTaxPairs,
  isPairDivergente,
  rowTolerancia,
  getRowBadge,
  computeNotaResumo,
} from '@/lib/fiscalComparacao';

// Agregado por tributo da simulação — memória de cálculo do card
interface SimTributoAgg {
  baseOrig: number;
  baseSim: number;
  basePac: number;
  valOrig: number;
  valSim: number;
  valPac: number;
  aliqs: Set<number>;
}

type SimTaxKey = 'icms' | 'icms_st' | 'fcp' | 'difal';
const SIM_LABELS: Record<SimTaxKey, string> = {
  icms: 'ICMS', icms_st: 'ICMS-ST', fcp: 'FCP', difal: 'DIFAL',
};

interface FiscalDebugEntry {
  timestamp: string;
  item_id?: string;
  produto?: string;
  etapa: string;
  mensagem: string;
}

interface ExecuteSummary {
  total: number;
  ok: number;
  sem_grupo_fiscal: number;
  error: number;
  debug: FiscalDebugEntry[];
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
function fmtBRL(v: number | null | undefined): string {
  if (v == null) return '—';
  return v.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' });
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

          {row.simulacao && <SimulacaoItem sim={row.simulacao} />}

          {row.full_result && <RetornoPacote fr={row.full_result} />}
        </div>
      </DialogContent>
    </Dialog>
  );
}

// SimulacaoItem — comparação por item da simulação "IBS/CBS na base":
// Original × Simulado interno (método aditivo) × Pacote (chamada única) × Diferença.
function SimulacaoItem({ sim }: { sim: Simulacao }) {
  if (sim.erro) {
    return (
      <div className="mb-2">
        <h3 className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground mb-1 pb-0.5 border-b">
          Simulação IBS/CBS na base
        </h3>
        <p className="text-[11px] text-amber-700 bg-amber-50 border border-amber-200 rounded p-2">{sim.erro}</p>
      </div>
    );
  }
  const fmtPct = (v: number | null | undefined) =>
    v != null ? `${v.toLocaleString('pt-BR')}%` : '—';
  const linhas: { label: string; aliq: string; baseOrig: number | null; baseSim: number | null; basePac: number | null; original: number; simulado: number; pacote: number }[] = [
    { label: 'ICMS', aliq: fmtPct(sim.aliquota_icms), baseOrig: sim.base_icms_original, baseSim: sim.base_icms_simulada, basePac: sim.base_icms_pacote, original: sim.icms_original, simulado: sim.icms_simulado, pacote: sim.icms_pacote },
    { label: 'ICMS-ST', aliq: 'MVA', baseOrig: sim.base_st_original ?? 0, baseSim: sim.base_st_simulada ?? 0, basePac: sim.base_st_pacote ?? 0, original: sim.st_original, simulado: sim.st_simulado, pacote: sim.st_pacote },
    { label: 'FCP', aliq: fmtPct(sim.aliquota_fcp), baseOrig: null, baseSim: null, basePac: null, original: sim.fcp_original, simulado: sim.fcp_simulado, pacote: sim.fcp_pacote },
    { label: 'DIFAL', aliq: fmtPct(sim.percentual_difal), baseOrig: null, baseSim: null, basePac: null, original: sim.difal_original, simulado: sim.difal_simulado, pacote: sim.difal_pacote },
  ];
  return (
    <div className="mb-2">
      <h3 className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground mb-1 pb-0.5 border-b">
        Simulação IBS/CBS na base
      </h3>
      <p className="text-[10px] text-muted-foreground mb-1">
        IBS {fmtBRL(sim.valor_ibs_simulado)} ({sim.aliquota_ibs?.toLocaleString('pt-BR')}%) + CBS {fmtBRL(sim.valor_cbs_simulado)} ({sim.aliquota_cbs?.toLocaleString('pt-BR')}%) = acréscimo {fmtBRL(sim.acrescimo_ibs_cbs)} sobre a base
      </p>
      <p className="text-[10px] text-muted-foreground mb-1">
        Preço Líquido (venda − desc + frete + desp − ICMS − ICMS-ST − PIS − COFINS − ISS): <span className="font-semibold text-foreground">{fmtBRL(sim.preco_liquido)}</span>
        {' '}· Base IBS/CBS do pacote: <span className={`font-semibold ${Math.abs((sim.preco_liquido ?? 0) - (sim.base_ibs_cbs_pacote ?? 0)) > 0.01 ? 'text-red-700' : 'text-foreground'}`}>{fmtBRL(sim.base_ibs_cbs_pacote)}</span>
      </p>
      <div className="overflow-x-auto">
        <table className="w-full text-[11px]">
          <thead>
            <tr className="text-muted-foreground border-b">
              <th className="text-left font-normal py-0.5"></th>
              <th className="text-right font-normal py-0.5">Base antes</th>
              <th className="text-right font-normal py-0.5">Nova base (sim)</th>
              <th className="text-right font-normal py-0.5">Nova base (pacote)</th>
              <th className="text-center font-normal py-0.5">Alíq.</th>
              <th className="text-right font-normal py-0.5">Valor antes</th>
              <th className="text-right font-normal py-0.5">Novo (sim)</th>
              <th className="text-right font-normal py-0.5">Novo (pacote)</th>
              <th className="text-right font-normal py-0.5">Dif.</th>
            </tr>
          </thead>
          <tbody>
            {linhas.map(l => {
              const dif = Math.round((l.simulado - l.pacote) * 100) / 100;
              return (
                <tr key={l.label} className="border-b border-dashed last:border-0">
                  <td className="py-0.5 text-muted-foreground whitespace-nowrap">{l.label}</td>
                  <td className="py-0.5 text-right">{l.baseOrig != null ? fmtBRL(l.baseOrig) : '—'}</td>
                  <td className="py-0.5 text-right font-medium">{l.baseSim != null ? fmtBRL(l.baseSim) : '—'}</td>
                  <td className="py-0.5 text-right">{l.basePac != null ? fmtBRL(l.basePac) : '—'}</td>
                  <td className="py-0.5 text-center text-muted-foreground">{l.aliq}</td>
                  <td className="py-0.5 text-right">{fmtBRL(l.original)}</td>
                  <td className="py-0.5 text-right font-medium">{fmtBRL(l.simulado)}</td>
                  <td className="py-0.5 text-right">{fmtBRL(l.pacote)}</td>
                  <td className={`py-0.5 text-right font-medium ${dif !== 0 ? 'text-red-700' : 'text-muted-foreground'}`}>{fmtBRL(dif)}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

// RetornoPacote — diagnóstico do retorno do pacote fiscal: as mensagens e
// metadados que explicam POR QUE o pacote calculou o que calculou (natureza
// da operação, CST, alíquotas, leis aplicadas e id das regras). É onde se
// entende uma divergência como "ICMS calculado = 0" (isento/outras).
function RetornoPacote({ fr }: { fr: Record<string, unknown> }) {
  const s = (k: string): string | null => {
    const v = fr[k];
    return typeof v === 'string' && v.trim() !== '' ? v : null;
  };
  const n = (k: string): number | null => {
    const v = fr[k];
    return typeof v === 'number' ? v : null;
  };
  const pct = (k: string): string | null => {
    const v = n(k);
    return v != null ? `${v.toLocaleString('pt-BR')}%` : null;
  };
  const brl = (k: string): string | null => {
    const v = n(k);
    return v != null ? v.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' }) : null;
  };

  const mensagens = ['Mensagem1', 'Mensagem2', 'Mensagem3', 'Mensagem4']
    .map(k => s(k))
    .filter((m): m is string => m !== null);

  const Linha = ({ label, value }: { label: string; value: string | null }) => (
    value == null ? null : (
      <div className="flex justify-between py-0.5 border-b border-dashed last:border-0 gap-3">
        <span className="text-[11px] text-muted-foreground w-40 shrink-0">{label}</span>
        <span className="text-[11px] font-medium text-right break-words min-w-0">{value}</span>
      </div>
    )
  );

  return (
    <div className="mb-2">
      <h3 className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground mb-1 pb-0.5 border-b">
        Retorno do Pacote (diagnóstico)
      </h3>
      {mensagens.length > 0 && (
        <div className="rounded bg-amber-50 border border-amber-200 p-2 mb-1.5 space-y-1">
          {mensagens.map((m, i) => (
            <p key={i} className="text-[11px] text-amber-900">{m}</p>
          ))}
        </div>
      )}
      <Linha label="Tipo Imposto" value={s('TipoImposto')} />
      <Linha label="CST / Cód. Tributação" value={s('CodigoTributFiscal')} />
      <Linha label="Natureza da Operação" value={s('NaturezaOperacao') ?? s('NaturezaOperacaoRetorno')} />
      <Linha label="Alíquota ICMS" value={pct('AliquotaImposto')} />
      <Linha label="Valor Isentas" value={brl('ValorIsentas')} />
      <Linha label="Valor Outras" value={brl('ValorOutras')} />
      <Linha label="Alíquota PIS / COFINS" value={n('AliquotaPIS') != null ? `${pct('AliquotaPIS')} / ${pct('AliquotaCOFINS')}` : null} />
      <Linha label="Lei ICMS" value={s('ICMSLaw')} />
      <Linha label="Lei PIS" value={s('PISLaw')} />
      <Linha label="Lei COFINS" value={s('COFINSLaw')} />
      <Linha label="Regra ICMS / PIS-COFINS" value={s('IdRegraCalculoIcms') != null || s('IdRegraCalculoPisCofins') != null ? `${s('IdRegraCalculoIcms') ?? '—'} / ${s('IdRegraCalculoPisCofins') ?? '—'}` : null} />
      <details className="mt-1.5">
        <summary className="text-[10px] text-muted-foreground cursor-pointer">Retorno completo (JSON, ~88 campos)</summary>
        <pre className="mt-1 max-h-56 overflow-auto rounded bg-muted/40 p-2 text-[10px] leading-tight">
          {JSON.stringify(fr, null, 2)}
        </pre>
      </details>
    </div>
  );
}

// ---------------------------------------------------------------------------
// ComparacaoFiscal — página principal
// ---------------------------------------------------------------------------
export default function ComparacaoFiscal() {
  const { token, companyId } = useAuth();
  const queryClient = useQueryClient();

  const [selectedNfe, setSelectedNfe] = useState<NfeSearchResult | null>(null);
  // Já vem MARCADO (2026-07-07): o pacote novo embute IBS/CBS na base em toda
  // chamada, então executar sem a simulação compara laranja com banana
  const [incluirIbsCbsBase, setIncluirIbsCbsBase] = useState(true);
  const [showOnlyDivergent, setShowOnlyDivergent] = useState(false);
  const [selectedItem, setSelectedItem] = useState<ComparacaoRow | null>(null);
  const [downloadingCSV, setDownloadingCSV] = useState(false);
  // Ao clicar no "olho" numa nota da lista, a seção de detalhe abaixo é
  // preenchida — mas como ela pode aparecer bem abaixo de uma lista longa de
  // resultados, sem scroll automático parece que o clique "não fez nada".
  const detailRef = useRef<HTMLDivElement>(null);

  // Sem Authorization/X-Company-ID explícitos: o interceptor global do
  // AuthContext injeta o token SEMPRE FRESCO (tokenRef) — headers fixos aqui
  // congelavam o token da renderização e lotes longos morriam com 401.
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
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ nfe_id: id, incluir_ibs_cbs_base: incluirIbsCbsBase }),
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

  useEffect(() => {
    if (selectedNfe && detailRef.current) {
      detailRef.current.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
  }, [selectedNfe]);

  const rows = useMemo(() => data ?? [], [data]);

  // Resumo da Nota — regra compartilhada em lib/fiscalComparacao.ts
  // (computeNotaResumo): calculado somando itens 'ok' × esperado do cabeçalho
  // (ajustado p/ inclusão IBS/CBS quando a nota rodou com o toggle).
  const notaSummary = useMemo(
    () => (selectedNfe ? computeNotaResumo(selectedNfe, rows) : null),
    [rows, selectedNfe],
  );

  // Acumulado da simulação "IBS/CBS na base" — soma dos itens que rodaram em
  // modo simulação sem erro. Original × Simulado interno × Pacote (chamada única).
  const simSummary = useMemo(() => {
    const comSim = rows.filter(r => r.simulacao && !r.simulacao.erro);
    const comErro = rows.filter(r => r.simulacao?.erro).length;
    if (comSim.length === 0 && comErro === 0) return null;
    const mk = (): SimTributoAgg => ({ baseOrig: 0, baseSim: 0, basePac: 0, valOrig: 0, valSim: 0, valPac: 0, aliqs: new Set<number>() });
    const trib: Record<SimTaxKey, SimTributoAgg> = { icms: mk(), icms_st: mk(), fcp: mk(), difal: mk() };
    let acrescimoTotal = 0;
    let precoLiquidoTotal = 0;
    let baseIbsCbsPacoteTotal = 0;
    let ibsSimTotal = 0;
    let cbsSimTotal = 0;
    comSim.forEach(r => {
      const s = r.simulacao!;
      precoLiquidoTotal += s.preco_liquido ?? 0;
      baseIbsCbsPacoteTotal += s.base_ibs_cbs_pacote ?? 0;
      acrescimoTotal += s.acrescimo_ibs_cbs;
      ibsSimTotal += s.valor_ibs_simulado ?? 0;
      cbsSimTotal += s.valor_cbs_simulado ?? 0;
      trib.icms.baseOrig += s.base_icms_original; trib.icms.baseSim += s.base_icms_simulada; trib.icms.basePac += s.base_icms_pacote;
      trib.icms.valOrig += s.icms_original; trib.icms.valSim += s.icms_simulado; trib.icms.valPac += s.icms_pacote;
      trib.icms.aliqs.add(s.aliquota_icms ?? 0);
      trib.icms_st.baseOrig += s.base_st_original ?? 0; trib.icms_st.baseSim += s.base_st_simulada ?? 0; trib.icms_st.basePac += s.base_st_pacote ?? 0;
      trib.icms_st.valOrig += s.st_original; trib.icms_st.valSim += s.st_simulado; trib.icms_st.valPac += s.st_pacote;
      trib.fcp.valOrig += s.fcp_original; trib.fcp.valSim += s.fcp_simulado; trib.fcp.valPac += s.fcp_pacote;
      trib.fcp.aliqs.add(s.aliquota_fcp ?? 0);
      trib.difal.valOrig += s.difal_original; trib.difal.valSim += s.difal_simulado; trib.difal.valPac += s.difal_pacote;
      trib.difal.aliqs.add(s.percentual_difal ?? 0);
    });
    return {
      trib, acrescimoTotal,
      precoLiquidoTotal: Math.round(precoLiquidoTotal * 100) / 100,
      baseIbsCbsPacoteTotal: Math.round(baseIbsCbsPacoteTotal * 100) / 100,
      ibsSimTotal: Math.round(ibsSimTotal * 100) / 100,
      cbsSimTotal: Math.round(cbsSimTotal * 100) / 100,
      itens: comSim.length, comErro,
    };
  }, [rows]);

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
      <NfeSearchList
        onViewDetail={setSelectedNfe}
        activeId={nfeId}
        incluirIbsCbs={incluirIbsCbsBase}
        onIncluirIbsCbsChange={setIncluirIbsCbsBase}
      />

      {selectedNfe && (
        <div ref={detailRef} className="flex items-center gap-3 flex-wrap border-t pt-3">
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

      {executar.data && executar.data.debug && executar.data.debug.length > 0 && (
        <details className="rounded-md border bg-muted/20 px-3 py-2">
          <summary className="text-xs font-medium cursor-pointer text-muted-foreground">
            Debug da última execução ({executar.data.debug.length} eventos — conexão Oracle,
            lookup de grupo fiscal, chamada do pacote fiscal por item)
          </summary>
          <div className="mt-2 max-h-64 overflow-y-auto space-y-1 font-mono text-[10px]">
            {executar.data.debug.map((entry, i) => (
              <div key={i} className="flex gap-2 border-b border-dashed pb-1 last:border-0">
                <span className="text-muted-foreground shrink-0">{entry.timestamp}</span>
                <span className="shrink-0 uppercase text-primary/70">[{entry.etapa}]</span>
                {entry.produto && <span className="shrink-0 text-muted-foreground truncate max-w-[220px]">{entry.produto}</span>}
                <span>{entry.mensagem}</span>
              </div>
            ))}
          </div>
        </details>
      )}

      {notaSummary && (
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-semibold flex items-center gap-2 flex-wrap">
              Resumo da Nota — Acumulado dos Itens
              {notaSummary.itensNaoOk > 0 && (
                <Badge variant="outline" className="text-[10px] px-1.5 py-0 bg-amber-50 text-amber-700 border-amber-200">
                  {notaSummary.itensNaoOk} de {notaSummary.totalItens} itens não calculados — total parcial
                </Badge>
              )}
            </CardTitle>
          </CardHeader>
          <CardContent>
            {/* Identificação da nota + valores comerciais do cabeçalho (<ICMSTot>) */}
            {selectedNfe && (
              <div className="flex flex-wrap gap-x-6 gap-y-1 mb-3 text-[11px] border rounded-md bg-muted/20 px-3 py-2">
                <div>
                  <span className="text-muted-foreground">Chave: </span>
                  <span className="font-mono select-all">{selectedNfe.chave_nfe}</span>
                </div>
                <div>
                  <span className="text-muted-foreground">Nº NF: </span>
                  <span className="font-semibold">{selectedNfe.numero_nfe}{selectedNfe.serie ? `/${selectedNfe.serie}` : ''}</span>
                </div>
                <div>
                  <span className="text-muted-foreground">Valor da Venda: </span>
                  <span className="font-semibold">{fmtBRL(selectedNfe.v_prod)}</span>
                </div>
                <div>
                  <span className="text-muted-foreground">Descontos: </span>
                  <span className="font-semibold">{fmtBRL(selectedNfe.v_desc)}</span>
                </div>
                <div>
                  <span className="text-muted-foreground">Frete: </span>
                  <span className="font-semibold">{fmtBRL(selectedNfe.v_frete)}</span>
                </div>
                <div>
                  <span className="text-muted-foreground">Total da NF: </span>
                  <span className="font-semibold">{fmtBRL(selectedNfe.v_nf)}</span>
                </div>
              </div>
            )}
            <div className="overflow-x-auto rounded-md border">
              <Table>
                <TableHeader>
                  <TableRow className="hover:bg-transparent bg-muted/30">
                    <TableHead className="py-1.5 px-2 text-[11px]"></TableHead>
                    {(Object.keys(RESUMO_LABELS) as ResumoKey[]).map(key => (
                      <TableHead key={key} className="py-1.5 px-2 text-[11px] text-center">{RESUMO_LABELS[key]}</TableHead>
                    ))}
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <TableRow>
                    <TableCell className="py-1 px-2 text-[11px] font-medium whitespace-nowrap">
                      {notaSummary.temSimulacao ? 'Esperado (ajustado p/ inclusão IBS/CBS)' : 'Esperado (total NF)'}
                    </TableCell>
                    {(Object.keys(RESUMO_LABELS) as ResumoKey[]).map(key => (
                      <TableCell key={key} className="py-1 px-2 text-right text-[11px] font-semibold">
                        {fmtBRL(notaSummary.esperado[key])}
                      </TableCell>
                    ))}
                  </TableRow>
                  <TableRow>
                    <TableCell className="py-1 px-2 text-[11px] font-medium whitespace-nowrap">Calculado (soma dos itens)</TableCell>
                    {(Object.keys(RESUMO_LABELS) as ResumoKey[]).map(key => (
                      <TableCell key={key} className="py-1 px-2 text-right text-[11px] text-muted-foreground">
                        {fmtBRL(notaSummary.acumuladoCalculado[key])}
                      </TableCell>
                    ))}
                  </TableRow>
                  <TableRow>
                    <TableCell className="py-1 px-2 text-[11px] font-medium whitespace-nowrap">Diferença</TableCell>
                    {(Object.keys(RESUMO_LABELS) as ResumoKey[]).map(key => {
                      const diferenca = Math.round((notaSummary.esperado[key] - notaSummary.acumuladoCalculado[key]) * 100) / 100;
                      return (
                        <TableCell key={key} className="py-1 px-2 text-right">
                          {notaSummary.itensNaoOk > 0 ? (
                            <span className="text-[11px] text-muted-foreground">—</span>
                          ) : (
                            <DiferencaBadge diferenca={diferenca} divergente={Math.abs(diferenca) > notaSummary.tolerancia} />
                          )}
                        </TableCell>
                      );
                    })}
                  </TableRow>
                </TableBody>
              </Table>
            </div>
          </CardContent>
        </Card>
      )}

      {simSummary && (
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-semibold flex items-center gap-2 flex-wrap">
              Simulação — IBS/CBS na base do ICMS
              <Badge variant="outline" className="text-[10px] px-1.5 py-0 bg-sky-50 text-sky-700 border-sky-200">
                {simSummary.itens} item(ns) simulados — acréscimo IBS+CBS {fmtBRL(simSummary.acrescimoTotal)}
              </Badge>
              {simSummary.comErro > 0 && (
                <Badge variant="outline" className="text-[10px] px-1.5 py-0 bg-amber-50 text-amber-700 border-amber-200">
                  {simSummary.comErro} item(ns) sem simulação (ver detalhe)
                </Badge>
              )}
            </CardTitle>
          </CardHeader>
          <CardContent>
            {/* Preço Líquido — base legal do IBS/CBS na transição */}
            <div className="flex flex-wrap gap-x-6 gap-y-1 mb-3 text-[11px] border rounded-md bg-sky-50/50 border-sky-200 px-3 py-2">
              <div>
                <span className="text-muted-foreground">Preço Líquido (venda − descontos + frete + despesas − ICMS − ICMS-ST − PIS − COFINS − ISS): </span>
                <span className="font-semibold">{fmtBRL(simSummary.precoLiquidoTotal)}</span>
                <span className="text-muted-foreground"> ← base do IBS/CBS</span>
              </div>
              <div>
                <span className="text-muted-foreground">Base IBS/CBS usada pelo pacote: </span>
                <span className={`font-semibold ${Math.abs(simSummary.precoLiquidoTotal - simSummary.baseIbsCbsPacoteTotal) > 0.01 ? 'text-red-700' : ''}`}>
                  {fmtBRL(simSummary.baseIbsCbsPacoteTotal)}
                </span>
              </div>
              <div>
                <span className="text-muted-foreground">Diferença: </span>
                <span className={`font-semibold ${Math.abs(simSummary.precoLiquidoTotal - simSummary.baseIbsCbsPacoteTotal) > 0.01 ? 'text-red-700' : 'text-muted-foreground'}`}>
                  {fmtBRL(Math.round((simSummary.precoLiquidoTotal - simSummary.baseIbsCbsPacoteTotal) * 100) / 100)}
                </span>
              </div>
              <div>
                <span className="text-muted-foreground">IBS simulado: </span>
                <span className="font-semibold">{fmtBRL(simSummary.ibsSimTotal)}</span>
                <span className="text-muted-foreground"> + CBS simulado: </span>
                <span className="font-semibold">{fmtBRL(simSummary.cbsSimTotal)}</span>
                <span className="text-muted-foreground"> = acréscimo na base</span>
              </div>
            </div>
            {/* Memória de cálculo: base antes → nova base (com IBS/CBS) → alíquota → valor antes → novo valor */}
            <div className="overflow-x-auto rounded-md border">
              <Table>
                <TableHeader>
                  <TableRow className="hover:bg-transparent bg-muted/30">
                    <TableHead className="py-1.5 px-2 text-[11px]">Tributo</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px] text-right border-l">Base antes (XML)</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px] text-right">Nova base (simulada)</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px] text-right">Nova base (pacote)</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px] text-center border-l">Alíq.</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px] text-right border-l">Valor antes</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px] text-right">Novo valor (simulado)</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px] text-right">Novo valor (pacote)</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px] text-right border-l">Diferença</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {(Object.keys(SIM_LABELS) as SimTaxKey[]).map(key => {
                    const t = simSummary.trib[key];
                    const temBase = key === 'icms' || key === 'icms_st';
                    const aliq = t.aliqs.size === 1 ? [...t.aliqs][0] : null;
                    const diferenca = Math.round((t.valSim - t.valPac) * 100) / 100;
                    return (
                      <TableRow key={key}>
                        <TableCell className="py-1 px-2 text-[11px] font-medium whitespace-nowrap">{SIM_LABELS[key]}</TableCell>
                        <TableCell className="py-1 px-2 text-right text-[11px] text-muted-foreground border-l">
                          {temBase ? fmtBRL(t.baseOrig) : '—'}
                        </TableCell>
                        <TableCell className="py-1 px-2 text-right text-[11px] font-semibold">
                          {temBase ? fmtBRL(t.baseSim) : '—'}
                        </TableCell>
                        <TableCell className="py-1 px-2 text-right text-[11px]">
                          {temBase ? fmtBRL(t.basePac) : '—'}
                        </TableCell>
                        <TableCell className="py-1 px-2 text-center text-[11px] border-l">
                          {key === 'icms_st'
                            ? 'MVA'
                            : aliq != null
                              ? `${aliq.toLocaleString('pt-BR')}%`
                              : 'várias'}
                        </TableCell>
                        <TableCell className="py-1 px-2 text-right text-[11px] text-muted-foreground border-l">{fmtBRL(t.valOrig)}</TableCell>
                        <TableCell className="py-1 px-2 text-right text-[11px] font-semibold">{fmtBRL(t.valSim)}</TableCell>
                        <TableCell className="py-1 px-2 text-right text-[11px]">{fmtBRL(t.valPac)}</TableCell>
                        <TableCell className="py-1 px-2 text-right border-l">
                          <DiferencaBadge diferenca={diferenca} divergente={diferenca !== 0} />
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </div>
            <p className="text-[11px] text-muted-foreground mt-2">
              Simulado (método aditivo): IBS/CBS = preço líquido × alíquotas; nova base = base do XML + IBS + CBS
              (acréscimo integral, precisão cheia); novo valor = nova base × alíquota do item. Pacote: a versão nova
              do PKG_FISCAL_FCTAX já embute IBS/CBS na base na própria chamada — colunas "pacote" vêm da chamada
              única (BaseCalculo/ValorImposto). Diferença = novo valor simulado − pacote.
            </p>
          </CardContent>
        </Card>
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
                          const divergente = !naoOk && isPairDivergente(pair, rowTolerancia(row));
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
