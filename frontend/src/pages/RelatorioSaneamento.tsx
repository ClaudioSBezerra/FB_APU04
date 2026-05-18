import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { toast } from 'sonner';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Download, AlertTriangle, CheckCircle } from 'lucide-react';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------
interface SaneamentoRow {
  ncm: string;
  variantes_cst_pis: number;
  variantes_cst_cofins: number;
  variantes_cclasstrib: number;
  qtd_itens: number;
  v_pis_total: number;
  v_cofins_total: number;
  csts_pis: string[] | null;
  csts_cofins: string[] | null;
  // Referência Reforma Tributária
  cclasstrib_reforma: number | null;
  descricao_reforma: string | null;
  ibs_reducao_pct: number | null;
  cbs_reducao_pct: number | null;
  anexo_reforma: string | null;
}

interface FornecedorRow {
  forn_cnpj: string;
  forn_nome: string;
  ncm: string;
  variantes_cclasstrib: number;
  v_pis_cofins_total: number;
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

function buildUrl(base: string, mesAno: string): string {
  if (!mesAno) return base;
  return `${base}?mes_ano=${encodeURIComponent(mesAno)}`;
}

// ---------------------------------------------------------------------------
// RelatorioSaneamento
// ---------------------------------------------------------------------------
export default function RelatorioSaneamento() {
  const [mesAnoFiltro, setMesAnoFiltro] = useState('');
  const [mesAnoAtivo, setMesAnoAtivo] = useState('');
  const [downloadingCSV, setDownloadingCSV] = useState(false);

  // Query: saneamento por NCM
  const {
    data: saneamentoData,
    isLoading: loadingSaneamento,
    isError: errorSaneamento,
    refetch: refetchSaneamento,
  } = useQuery<SaneamentoRow[]>({
    queryKey: ['xml-saneamento', mesAnoAtivo],
    queryFn: async () => {
      const res = await fetch(buildUrl('/api/xml/reports/saneamento', mesAnoAtivo));
      if (!res.ok) throw new Error(`Erro ${res.status}`);
      return res.json();
    },
  });

  // Query: fornecedores com CCLASSTRIB inconsistente
  const {
    data: fornData,
    isLoading: loadingForn,
    isError: errorForn,
    refetch: refetchForn,
  } = useQuery<FornecedorRow[]>({
    queryKey: ['xml-fornecedores-cclasstrib', mesAnoAtivo],
    queryFn: async () => {
      const res = await fetch(buildUrl('/api/xml/reports/fornecedores-cclasstrib', mesAnoAtivo));
      if (!res.ok) throw new Error(`Erro ${res.status}`);
      return res.json();
    },
  });

  const handleAtualizar = () => {
    setMesAnoAtivo(mesAnoFiltro.trim());
  };

  const handleDownloadCSV = async () => {
    setDownloadingCSV(true);
    try {
      const res = await fetch(buildUrl('/api/xml/reports/saneamento/csv', mesAnoAtivo));
      if (!res.ok) throw new Error(`Erro ${res.status}`);
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'saneamento-cclasstrib.csv';
      a.click();
      URL.revokeObjectURL(url);
      toast.success('CSV exportado com sucesso');
    } catch (err) {
      toast.error('Erro ao exportar CSV: ' + (err instanceof Error ? err.message : 'Desconhecido'));
    } finally {
      setDownloadingCSV(false);
    }
  };

  // Totais para painel de resumo
  const totalNcmsDivergentes = saneamentoData?.length ?? 0;
  const totalFornecedoresProblema = fornData
    ? new Set(fornData.map(r => r.forn_cnpj)).size
    : 0;
  const totalVPisCofins = saneamentoData
    ? saneamentoData.reduce((acc, r) => acc + (r.v_pis_total ?? 0) + (r.v_cofins_total ?? 0), 0)
    : 0;

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold">Saneamento CCLASSTRIB</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Identifica divergências de CCLASSTRIB entre notas que já possuem essa classificação
          nos XMLs. Apenas notas com CCLASSTRIB preenchido são consideradas.
        </p>
      </div>

      {/* Filtros globais */}
      <div className="flex items-center gap-3">
        <div className="flex flex-col gap-1">
          <label className="text-xs text-muted-foreground">Mês/Ano (opcional)</label>
          <Input
            placeholder="Ex: 2025-01"
            value={mesAnoFiltro}
            onChange={e => setMesAnoFiltro(e.target.value)}
            className="w-40 h-8 text-sm"
          />
        </div>
        <Button
          size="sm"
          className="mt-4"
          onClick={handleAtualizar}
        >
          Atualizar
        </Button>
        {mesAnoAtivo && (
          <Button
            size="sm"
            variant="ghost"
            className="mt-4 text-xs text-muted-foreground"
            onClick={() => { setMesAnoFiltro(''); setMesAnoAtivo(''); }}
          >
            Limpar filtro
          </Button>
        )}
      </div>

      {/* Seção 3 — Painel de resumo */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              NCMs com divergência de CST
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">{loadingSaneamento ? '…' : totalNcmsDivergentes}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Fornecedores com problemas
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">{loadingForn ? '…' : totalFornecedoresProblema}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              V. PIS+COFINS potencialmente incorreto
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">{loadingSaneamento ? '…' : fmtBRL(totalVPisCofins)}</p>
          </CardContent>
        </Card>
      </div>

      {/* Seção 1 — Saneamento por NCM */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between pb-3">
          <CardTitle className="text-base">Divergências por NCM</CardTitle>
          <Button
            size="sm"
            variant="outline"
            onClick={handleDownloadCSV}
            disabled={downloadingCSV}
          >
            <Download className="w-4 h-4 mr-1" />
            {downloadingCSV ? 'Exportando…' : 'Exportar CSV'}
          </Button>
        </CardHeader>
        <CardContent className="p-0">
          {loadingSaneamento ? (
            <p className="text-sm text-muted-foreground px-4 py-6">Carregando…</p>
          ) : errorSaneamento ? (
            <p className="text-sm text-destructive px-4 py-6">
              Erro ao carregar dados.{' '}
              <button className="underline" onClick={() => refetchSaneamento()}>Tentar novamente</button>
            </p>
          ) : !saneamentoData || saneamentoData.length === 0 ? (
            <div className="flex items-center gap-2 px-4 py-6 text-sm text-muted-foreground">
              <CheckCircle className="w-4 h-4 text-green-500" />
              Nenhuma divergência encontrada. Isso significa que todos os itens importados
              têm classificação tributária consistente.
            </div>
          ) : (
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>NCM</TableHead>
                    <TableHead className="text-center">CSTs PIS</TableHead>
                    <TableHead className="text-center">CSTs COFINS</TableHead>
                    <TableHead className="text-center">Variantes CCLASSTRIB</TableHead>
                    <TableHead className="text-right">Qtd Itens</TableHead>
                    <TableHead className="text-right">V. PIS+COFINS</TableHead>
                    <TableHead>CSTs PIS Encontrados</TableHead>
                    <TableHead>CSTs COFINS Encontrados</TableHead>
                    <TableHead className="text-center">CCLASSTRIB Reforma</TableHead>
                    <TableHead>Descrição / Anexo</TableHead>
                    <TableHead className="text-center">Redução IBS/CBS</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {saneamentoData.map(row => (
                    <TableRow key={row.ncm}>
                      <TableCell className="font-mono text-xs">{row.ncm}</TableCell>
                      <TableCell className="text-center">
                        <Badge variant="outline" className={row.variantes_cst_pis > 1 ? 'bg-orange-50 text-orange-700' : ''}>
                          {row.variantes_cst_pis}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-center">
                        <Badge variant="outline" className={row.variantes_cst_cofins > 1 ? 'bg-orange-50 text-orange-700' : ''}>
                          {row.variantes_cst_cofins}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-center">
                        <Badge variant="outline" className="bg-orange-50 text-orange-700">
                          <AlertTriangle className="w-3 h-3 mr-1 inline" />
                          {row.variantes_cclasstrib}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-right text-xs">{row.qtd_itens.toLocaleString('pt-BR')}</TableCell>
                      <TableCell className="text-right text-xs">
                        {fmtBRL((row.v_pis_total ?? 0) + (row.v_cofins_total ?? 0))}
                      </TableCell>
                      <TableCell className="text-xs text-muted-foreground">
                        {(row.csts_pis ?? []).join(', ') || '—'}
                      </TableCell>
                      <TableCell className="text-xs text-muted-foreground">
                        {(row.csts_cofins ?? []).join(', ') || '—'}
                      </TableCell>
                      <TableCell className="text-center">
                        {row.cclasstrib_reforma != null ? (
                          <Badge variant="outline" className="bg-blue-50 text-blue-700 border-blue-200 font-mono">
                            {row.cclasstrib_reforma}
                          </Badge>
                        ) : (
                          <span className="text-xs text-muted-foreground">—</span>
                        )}
                      </TableCell>
                      <TableCell className="text-xs max-w-[160px]">
                        {row.descricao_reforma
                          ? <span title={row.descricao_reforma}>{row.descricao_reforma.slice(0, 25)}{row.descricao_reforma.length > 25 ? '…' : ''}</span>
                          : '—'}
                        {row.anexo_reforma && (
                          <span className="ml-1 text-muted-foreground">({row.anexo_reforma})</span>
                        )}
                      </TableCell>
                      <TableCell className="text-center text-xs">
                        {row.ibs_reducao_pct != null
                          ? `${row.ibs_reducao_pct.toFixed(0)}%`
                          : '—'}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Seção 2 — Fornecedores com problemas */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">Fornecedores com Problemas de CCLASSTRIB</CardTitle>
          <p className="text-xs text-muted-foreground mt-0.5">
            Ordenado por valor PIS+COFINS decrescente — priorize os fornecedores mais críticos para saneamento.
          </p>
        </CardHeader>
        <CardContent className="p-0">
          {loadingForn ? (
            <p className="text-sm text-muted-foreground px-4 py-6">Carregando…</p>
          ) : errorForn ? (
            <p className="text-sm text-destructive px-4 py-6">
              Erro ao carregar dados.{' '}
              <button className="underline" onClick={() => refetchForn()}>Tentar novamente</button>
            </p>
          ) : !fornData || fornData.length === 0 ? (
            <div className="flex items-center gap-2 px-4 py-6 text-sm text-muted-foreground">
              <CheckCircle className="w-4 h-4 text-green-500" />
              Nenhuma divergência encontrada. Todos os fornecedores têm CCLASSTRIB preenchido e consistente.
            </div>
          ) : (
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>CNPJ Fornecedor</TableHead>
                    <TableHead>Nome Fornecedor</TableHead>
                    <TableHead>NCM</TableHead>
                    <TableHead className="text-right">Variantes CCLASSTRIB</TableHead>
                    <TableHead className="text-right">V. PIS+COFINS Total</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {fornData.map((row, idx) => (
                    <TableRow key={`${row.forn_cnpj}-${row.ncm}-${idx}`}>
                      <TableCell className="font-mono text-xs">{fmtCNPJ(row.forn_cnpj)}</TableCell>
                      <TableCell className="text-xs max-w-[180px] truncate" title={row.forn_nome}>
                        {row.forn_nome || '—'}
                      </TableCell>
                      <TableCell className="font-mono text-xs">{row.ncm}</TableCell>
                      <TableCell className="text-right">
                        <Badge variant="outline" className="bg-orange-50 text-orange-700">
                          <AlertTriangle className="w-3 h-3 mr-1 inline" />
                          {row.variantes_cclasstrib}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-right text-xs font-medium">
                        {fmtBRL(row.v_pis_cofins_total)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
