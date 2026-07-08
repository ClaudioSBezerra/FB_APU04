// DiagnosticoPacoteFiscal.tsx — Relatório Diagnóstico dos testes do pacote
// fiscal (pedido 2026-07-08): sumário de tudo que já foi executado — período,
// notas/itens, status, divergências por tributo (mesma régua da Comparação),
// distribuição por CFOP, CSTs de ICMS/PIS, parâmetros usados (centro fiscal,
// tipo contribuinte) e erros mais frequentes. Exportável para Excel.
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { toast } from 'sonner';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table';
import { FileSpreadsheet, FileText, Loader2, Search } from 'lucide-react';
import { exportToExcel } from '@/lib/exportToExcel';
import { useAuth } from '@/contexts/AuthContext';

interface DiagCfopRow {
  cfop: string; notas: number; itens: number; v_prod: number;
  ok: number; sem_grupo_fiscal: number; error: number;
  div_icms: number; div_st: number; div_pis: number;
  div_cofins: number; div_ibs: number; div_cbs: number;
}
interface DiagDistRow { chave: string; itens: number; v_prod: number }
interface DiagErroRow { mensagem: string; itens: number }
interface DiagFilialRow { cnpj: string; nome: string; uf: string; notas: number }

interface Diagnostico {
  periodo_inicio: string; periodo_fim: string;
  notas_executadas: number; itens_executados: number;
  itens_ok: number; itens_sem_grupo: number; itens_erro: number;
  com_simulacao: number; v_prod_total: number;
  div_icms: number; div_st: number; div_pis: number;
  div_cofins: number; div_ibs: number; div_cbs: number;
  por_cfop: DiagCfopRow[];
  por_cst_icms: DiagDistRow[];
  por_cst_pis: DiagDistRow[];
  por_centro_fiscal: DiagDistRow[];
  por_contribuinte: DiagDistRow[];
  erros: DiagErroRow[];
  filiais: DiagFilialRow[];
}

const fmtBRL = (v: number) => v.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' });
const fmtN = (v: number) => v.toLocaleString('pt-BR');

const DIV_LABELS: { key: keyof Diagnostico; label: string }[] = [
  { key: 'div_icms', label: 'ICMS' },
  { key: 'div_st', label: 'ICMS-ST' },
  { key: 'div_pis', label: 'PIS' },
  { key: 'div_cofins', label: 'COFINS' },
  { key: 'div_ibs', label: 'IBS' },
  { key: 'div_cbs', label: 'CBS' },
];

function StatCard({ title, value, sub }: { title: string; value: string; sub?: string }) {
  return (
    <Card>
      <CardHeader className="pb-1"><CardTitle className="text-xs font-semibold text-muted-foreground">{title}</CardTitle></CardHeader>
      <CardContent>
        <p className="text-xl font-semibold">{value}</p>
        {sub && <p className="text-[11px] text-muted-foreground mt-0.5">{sub}</p>}
      </CardContent>
    </Card>
  );
}

function DistTable({ title, rows, chaveLabel }: { title: string; rows: DiagDistRow[]; chaveLabel: string }) {
  return (
    <Card>
      <CardHeader className="pb-2"><CardTitle className="text-sm font-semibold">{title}</CardTitle></CardHeader>
      <CardContent>
        {rows.length === 0 ? (
          <p className="text-xs text-muted-foreground italic">Sem dados.</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent bg-muted/30">
                <TableHead className="py-1 px-2 text-[11px]">{chaveLabel}</TableHead>
                <TableHead className="py-1 px-2 text-[11px] text-right">Itens</TableHead>
                <TableHead className="py-1 px-2 text-[11px] text-right">Valor Produtos</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map(r => (
                <TableRow key={r.chave || '(vazio)'}>
                  <TableCell className="py-1 px-2 text-[11px] font-medium">{r.chave || '—'}</TableCell>
                  <TableCell className="py-1 px-2 text-[11px] text-right">{fmtN(r.itens)}</TableCell>
                  <TableCell className="py-1 px-2 text-[11px] text-right">{fmtBRL(r.v_prod)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}

export default function DiagnosticoPacoteFiscal() {
  const { token, companyId } = useAuth();
  const [dataInicio, setDataInicio] = useState('');
  const [dataFim, setDataFim] = useState('');
  // Filial = CNPJ do emitente; UF Origem = UF do emitente (2026-07-08)
  const [filial, setFilial] = useState('');
  const [ufOrigem, setUfOrigem] = useState('');
  const [applied, setApplied] = useState({ dataInicio: '', dataFim: '', filial: '', ufOrigem: '' });

  const { data: diag, isLoading, isError, refetch } = useQuery<Diagnostico>({
    queryKey: ['fiscal-diagnostico', applied],
    queryFn: async () => {
      const params = new URLSearchParams();
      if (applied.dataInicio) params.set('data_inicio', applied.dataInicio);
      if (applied.dataFim) params.set('data_fim', applied.dataFim);
      if (applied.filial) params.set('filial', applied.filial);
      if (applied.ufOrigem) params.set('uf_origem', applied.ufOrigem);
      const res = await fetch(`/api/fiscal/diagnostico?${params}`);
      if (!res.ok) throw new Error(`Erro ${res.status}`);
      return res.json();
    },
  });

  // PDF: window.open não envia headers — token via ?token= (AuthMiddleware
  // aceita) e empresa via ?company_id= (mesmo padrão dos PDFs do Fronteira)
  const handleExportPDF = () => {
    const params = new URLSearchParams();
    if (applied.dataInicio) params.set('data_inicio', applied.dataInicio);
    if (applied.dataFim) params.set('data_fim', applied.dataFim);
    if (applied.filial) params.set('filial', applied.filial);
    if (applied.ufOrigem) params.set('uf_origem', applied.ufOrigem);
    if (token) params.set('token', token);
    if (companyId) params.set('company_id', companyId);
    window.open(`/api/fiscal/diagnostico/pdf?${params}`, '_blank');
  };

  const handleExport = () => {
    if (!diag) return;
    const linhas: Record<string, unknown>[] = [
      { Secao: 'RESUMO', Chave: 'Período', Valor: `${diag.periodo_inicio} a ${diag.periodo_fim}` },
      { Secao: 'RESUMO', Chave: 'Notas executadas', Valor: diag.notas_executadas },
      { Secao: 'RESUMO', Chave: 'Itens executados', Valor: diag.itens_executados },
      { Secao: 'RESUMO', Chave: 'Itens OK', Valor: diag.itens_ok },
      { Secao: 'RESUMO', Chave: 'Sem grupo fiscal', Valor: diag.itens_sem_grupo },
      { Secao: 'RESUMO', Chave: 'Com erro', Valor: diag.itens_erro },
      { Secao: 'RESUMO', Chave: 'Com simulação IBS/CBS', Valor: diag.com_simulacao },
      { Secao: 'RESUMO', Chave: 'Valor produtos', Valor: diag.v_prod_total },
      ...DIV_LABELS.map(d => ({ Secao: 'DIVERGENCIAS', Chave: d.label, Valor: diag[d.key] as number })),
      ...diag.por_cfop.map(c => ({
        Secao: 'POR CFOP', Chave: c.cfop, Notas: c.notas, Itens: c.itens, 'Valor Produtos': c.v_prod,
        OK: c.ok, 'Sem Grupo': c.sem_grupo_fiscal, Erro: c.error,
        'Div ICMS': c.div_icms, 'Div ST': c.div_st, 'Div PIS': c.div_pis,
        'Div COFINS': c.div_cofins, 'Div IBS': c.div_ibs, 'Div CBS': c.div_cbs,
      })),
      ...diag.por_cst_icms.map(d => ({ Secao: 'CST ICMS', Chave: d.chave, Itens: d.itens, 'Valor Produtos': d.v_prod })),
      ...diag.por_cst_pis.map(d => ({ Secao: 'CST PIS', Chave: d.chave, Itens: d.itens, 'Valor Produtos': d.v_prod })),
      ...diag.por_centro_fiscal.map(d => ({ Secao: 'CENTRO FISCAL', Chave: d.chave, Itens: d.itens })),
      ...diag.por_contribuinte.map(d => ({ Secao: 'TIPO CONTRIBUINTE', Chave: d.chave, Itens: d.itens })),
      ...diag.erros.map(e => ({ Secao: 'ERROS', Chave: e.mensagem, Itens: e.itens })),
    ];
    exportToExcel(linhas, `diagnostico-pacote-fiscal`, 'Diagnóstico');
    toast.success('Excel exportado com sucesso');
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold">Relatório Diagnóstico — Teste Pacote Fiscal</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Sumário de tudo que já foi executado contra o PKG_FISCAL_FCTAX: cobertura, status,
          divergências por tributo (mesma régua da Comparação Fiscal), CFOPs, CSTs e parâmetros usados.
        </p>
      </div>

      {/* Filtro de período (pela data de emissão da nota) */}
      <div className="flex items-end gap-2 flex-wrap p-3 bg-muted/20 rounded-md border border-dashed">
        <div className="flex flex-col gap-1">
          <label className="text-[11px] text-muted-foreground">De</label>
          <Input type="date" value={dataInicio} onChange={e => setDataInicio(e.target.value)} className="h-8 w-36 text-xs" />
        </div>
        <div className="flex flex-col gap-1">
          <label className="text-[11px] text-muted-foreground">Até</label>
          <Input type="date" value={dataFim} onChange={e => setDataFim(e.target.value)} className="h-8 w-36 text-xs" />
        </div>
        <div className="flex flex-col gap-1">
          <label className="text-[11px] text-muted-foreground" title="CNPJ emitente das notas já executadas — use para diagnosticar filial a filial">Filial</label>
          <select
            value={filial}
            onChange={e => setFilial(e.target.value)}
            className="h-8 w-56 text-xs rounded-md border bg-background px-2"
          >
            <option value="">Todas</option>
            {(diag?.filiais ?? []).map(f => (
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
            className="h-8 w-16 text-xs uppercase"
            maxLength={2}
          />
        </div>
        <Button size="sm" className="h-8" onClick={() => setApplied({ dataInicio, dataFim, filial, ufOrigem })} disabled={isLoading}>
          {isLoading ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Search className="h-3.5 w-3.5 mr-1.5" />}
          Aplicar
        </Button>
        <Button size="sm" variant="outline" className="h-8 ml-auto" onClick={handleExportPDF} disabled={!diag}>
          <FileText className="w-4 h-4 mr-1" /> Exportar PDF
        </Button>
        <Button size="sm" variant="outline" className="h-8" onClick={handleExport} disabled={!diag}>
          <FileSpreadsheet className="w-4 h-4 mr-1" /> Exportar Excel
        </Button>
      </div>

      {isError ? (
        <p className="text-sm text-destructive">
          Erro ao carregar o diagnóstico. <button className="underline" onClick={() => refetch()}>Tentar novamente</button>
        </p>
      ) : isLoading || !diag ? (
        <p className="text-sm text-muted-foreground text-center py-8">Montando diagnóstico...</p>
      ) : diag.itens_executados === 0 ? (
        <p className="text-sm text-muted-foreground text-center py-8">
          Nenhuma execução encontrada no período — rode notas na Comparação Fiscal primeiro.
        </p>
      ) : (
        <>
          {/* Cards de cobertura */}
          <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3">
            <StatCard title="Período" value={`${diag.periodo_inicio}`} sub={`até ${diag.periodo_fim}`} />
            <StatCard title="Notas executadas" value={fmtN(diag.notas_executadas)} />
            <StatCard title="Itens executados" value={fmtN(diag.itens_executados)}
              sub={`${fmtN(diag.com_simulacao)} com simulação IBS/CBS`} />
            <StatCard title="Itens OK" value={fmtN(diag.itens_ok)}
              sub={diag.itens_executados > 0 ? `${(diag.itens_ok / diag.itens_executados * 100).toFixed(1)}%` : ''} />
            <StatCard title="Sem grupo / Erro" value={`${fmtN(diag.itens_sem_grupo)} / ${fmtN(diag.itens_erro)}`} />
            <StatCard title="Valor produtos" value={fmtBRL(diag.v_prod_total)} />
          </div>

          {/* Divergências por tributo */}
          <div className="flex items-center gap-2 flex-wrap">
            <span className="text-xs font-semibold text-muted-foreground">Itens divergentes por tributo:</span>
            {DIV_LABELS.map(d => {
              const count = diag[d.key] as number;
              const pct = diag.itens_ok > 0 ? (count / diag.itens_ok) * 100 : 0;
              return (
                <Badge key={d.key} variant="outline"
                  className={count > 0
                    ? 'text-[11px] px-2 py-0.5 bg-red-50 text-red-700 border-red-200'
                    : 'text-[11px] px-2 py-0.5 bg-emerald-50 text-emerald-700 border-emerald-200'}>
                  {d.label}: {fmtN(count)} ({pct.toFixed(1)}%)
                </Badge>
              );
            })}
          </div>

          {/* Por CFOP */}
          <Card>
            <CardHeader className="pb-2"><CardTitle className="text-sm font-semibold">Por CFOP</CardTitle></CardHeader>
            <CardContent>
              <div className="overflow-x-auto rounded-md border">
                <Table>
                  <TableHeader>
                    <TableRow className="hover:bg-transparent bg-muted/30">
                      <TableHead className="py-1 px-2 text-[11px]">CFOP</TableHead>
                      <TableHead className="py-1 px-2 text-[11px] text-right">Notas</TableHead>
                      <TableHead className="py-1 px-2 text-[11px] text-right">Itens</TableHead>
                      <TableHead className="py-1 px-2 text-[11px] text-right">Valor Produtos</TableHead>
                      <TableHead className="py-1 px-2 text-[11px] text-right">OK</TableHead>
                      <TableHead className="py-1 px-2 text-[11px] text-right">S/ Grupo</TableHead>
                      <TableHead className="py-1 px-2 text-[11px] text-right">Erro</TableHead>
                      <TableHead className="py-1 px-2 text-[11px] text-right border-l">ICMS</TableHead>
                      <TableHead className="py-1 px-2 text-[11px] text-right">ST</TableHead>
                      <TableHead className="py-1 px-2 text-[11px] text-right">PIS</TableHead>
                      <TableHead className="py-1 px-2 text-[11px] text-right">COFINS</TableHead>
                      <TableHead className="py-1 px-2 text-[11px] text-right">IBS</TableHead>
                      <TableHead className="py-1 px-2 text-[11px] text-right">CBS</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {diag.por_cfop.map(c => (
                      <TableRow key={c.cfop}>
                        <TableCell className="py-1 px-2 text-[11px] font-medium">{c.cfop || '—'}</TableCell>
                        <TableCell className="py-1 px-2 text-[11px] text-right">{fmtN(c.notas)}</TableCell>
                        <TableCell className="py-1 px-2 text-[11px] text-right">{fmtN(c.itens)}</TableCell>
                        <TableCell className="py-1 px-2 text-[11px] text-right">{fmtBRL(c.v_prod)}</TableCell>
                        <TableCell className="py-1 px-2 text-[11px] text-right">{fmtN(c.ok)}</TableCell>
                        <TableCell className="py-1 px-2 text-[11px] text-right">{fmtN(c.sem_grupo_fiscal)}</TableCell>
                        <TableCell className="py-1 px-2 text-[11px] text-right">{fmtN(c.error)}</TableCell>
                        {[c.div_icms, c.div_st, c.div_pis, c.div_cofins, c.div_ibs, c.div_cbs].map((v, i) => (
                          <TableCell key={i} className={`py-1 px-2 text-[11px] text-right ${i === 0 ? 'border-l' : ''} ${v > 0 ? 'text-red-700 font-semibold' : 'text-muted-foreground'}`}>
                            {fmtN(v)}
                          </TableCell>
                        ))}
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            </CardContent>
          </Card>

          {/* Distribuições */}
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            <DistTable title="Situações de ICMS (CST do XML)" rows={diag.por_cst_icms} chaveLabel="CST ICMS" />
            <DistTable title="Situações de PIS/COFINS (CST do XML)" rows={diag.por_cst_pis} chaveLabel="CST PIS" />
            <DistTable title="Centro fiscal usado na chamada" rows={diag.por_centro_fiscal} chaveLabel="pTipoCentroFiscal" />
            <DistTable title="Tipo de contribuinte na chamada" rows={diag.por_contribuinte} chaveLabel="pTipoContribuinte" />
          </div>

          {/* Erros */}
          {diag.erros.length > 0 && (
            <Card>
              <CardHeader className="pb-2"><CardTitle className="text-sm font-semibold">Erros mais frequentes</CardTitle></CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow className="hover:bg-transparent bg-muted/30">
                      <TableHead className="py-1 px-2 text-[11px]">Mensagem</TableHead>
                      <TableHead className="py-1 px-2 text-[11px] text-right">Itens</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {diag.erros.map((e, i) => (
                      <TableRow key={i}>
                        <TableCell className="py-1 px-2 text-[11px]">{e.mensagem || '—'}</TableCell>
                        <TableCell className="py-1 px-2 text-[11px] text-right">{fmtN(e.itens)}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          )}

          <p className="text-[11px] text-muted-foreground">
            Divergência usa a mesma régua da Comparação Fiscal: itens executados com simulação IBS/CBS
            comparam contra o esperado ajustado (tolerância de 1 centavo); sem simulação, contra o XML cru
            (tolerância zero). "% divergente" é sobre os itens OK.
          </p>
        </>
      )}
    </div>
  );
}
