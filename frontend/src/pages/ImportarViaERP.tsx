import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useAuth } from '@/contexts/AuthContext';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Badge } from '@/components/ui/badge';
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select';
import { Loader2, RefreshCw, Database, Send } from 'lucide-react';
import { toast } from 'sonner';

export interface ERPXMLJob {
  id: string;
  data_ini: string;
  data_fim: string;
  tipos: string;
  status: string;
  total_enviados: number;
  total_erros: number;
  error_message?: string;
  created_at: string;
  started_at?: string;
  finished_at?: string;
  docs_total: number;
  importados: number;
  rejeitados: number;
  batches_total: number;
  batches_andamento: number;
}

// Status efetivo: combina o status do job (fase de envio pelo conector) com o
// progresso real dos lotes (fase de importação pelo worker).
function effectiveStatus(j: ERPXMLJob): { label: string; cls: string } {
  if (j.status === 'error') return { label: 'Erro', cls: 'bg-red-100 text-red-700' };
  if (j.status === 'canceled') return { label: 'Cancelado', cls: 'bg-slate-100 text-slate-600' };
  if (j.status === 'pending') return { label: 'Na fila', cls: 'bg-amber-100 text-amber-700' };
  if (j.batches_andamento > 0) return { label: 'Importando', cls: 'bg-blue-100 text-blue-700' };
  if (j.status === 'running' && j.batches_total === 0) return { label: 'Lendo ERP…', cls: 'bg-blue-100 text-blue-700' };
  return { label: 'Concluído', cls: 'bg-green-100 text-green-700' };
}

function fmtDateBR(iso: string): string {
  if (!iso) return '';
  const d = iso.slice(0, 10).split('-');
  return d.length === 3 ? `${d[2]}/${d[1]}/${d[0]}` : iso;
}

// Tabela de jobs reaproveitada pela página de Logs.
export function ERPXMLJobsTable({ autoRefresh = true }: { autoRefresh?: boolean }) {
  const { token, companyId } = useAuth();
  const authHeaders = { Authorization: `Bearer ${token}`, 'X-Company-ID': companyId || '' };

  const { data, isFetching, refetch } = useQuery<{ jobs: ERPXMLJob[] }>({
    queryKey: ['erp-xml-jobs', companyId],
    queryFn: async () => {
      const res = await fetch('/api/erp-bridge/xml-import/jobs', { headers: authHeaders });
      if (!res.ok) throw new Error(res.statusText);
      return res.json();
    },
    enabled: !!token && !!companyId,
    refetchInterval: autoRefresh ? 10_000 : false,
  });

  const jobs = data?.jobs ?? [];

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <span className="text-sm text-muted-foreground">{jobs.length} job(s)</span>
        <Button size="sm" variant="outline" onClick={() => refetch()} disabled={isFetching}>
          <RefreshCw className={`h-3.5 w-3.5 mr-1.5 ${isFetching ? 'animate-spin' : ''}`} />
          Atualizar
        </Button>
      </div>
      <div className="rounded-md border overflow-x-auto">
        <table className="w-full text-sm">
          <thead className="bg-muted/40">
            <tr className="text-left">
              <th className="px-3 py-2 font-semibold">Período</th>
              <th className="px-3 py-2 font-semibold">Tipos</th>
              <th className="px-3 py-2 font-semibold">Status</th>
              <th className="px-3 py-2 font-semibold w-64">Progresso (importados)</th>
              <th className="px-3 py-2 font-semibold">Criado</th>
            </tr>
          </thead>
          <tbody>
            {jobs.length === 0 && (
              <tr><td colSpan={5} className="px-3 py-6 text-center text-muted-foreground">Nenhum job ainda.</td></tr>
            )}
            {jobs.map((j) => {
              const sc = effectiveStatus(j);
              const total = j.docs_total || 0;
              const imp = j.importados || 0;
              const pct = total > 0 ? Math.min(100, Math.round((imp / total) * 100)) : 0;
              const emAndamento = j.batches_andamento > 0;
              return (
                <tr key={j.id} className="border-t hover:bg-muted/20">
                  <td className="px-3 py-2 whitespace-nowrap">{fmtDateBR(j.data_ini)} – {fmtDateBR(j.data_fim)}</td>
                  <td className="px-3 py-2">{j.tipos}</td>
                  <td className="px-3 py-2">
                    <span className={`inline-block rounded px-2 py-0.5 text-xs font-medium ${sc.cls}`}>{sc.label}</span>
                    {j.error_message && (
                      <span className="block text-[11px] text-red-600 mt-0.5 max-w-xs truncate" title={j.error_message}>
                        {j.error_message}
                      </span>
                    )}
                  </td>
                  <td className="px-3 py-2">
                    {total > 0 ? (
                      <div className="space-y-1">
                        <div className="h-2 w-full rounded bg-muted overflow-hidden">
                          <div className={`h-full ${emAndamento ? 'bg-blue-500' : 'bg-green-500'}`} style={{ width: `${pct}%` }} />
                        </div>
                        <div className="text-[11px] text-muted-foreground tabular-nums">
                          {imp.toLocaleString('pt-BR')} / {total.toLocaleString('pt-BR')} ({pct}%)
                          {j.rejeitados > 0 && <span className="text-red-600"> · {j.rejeitados} rej.</span>}
                        </div>
                      </div>
                    ) : (
                      <span className="text-[11px] text-muted-foreground">
                        {sc.label === 'Lendo ERP…' ? 'lendo do ERP…' : '—'}
                      </span>
                    )}
                  </td>
                  <td className="px-3 py-2 whitespace-nowrap text-muted-foreground">
                    {new Date(j.created_at).toLocaleString('pt-BR')}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

export default function ImportarViaERP() {
  const { token, companyId } = useAuth();
  const queryClient = useQueryClient();
  const [dataIni, setDataIni] = useState('');
  const [dataFim, setDataFim] = useState('');
  const [tipos, setTipos] = useState('entradas,ctes');

  const trigger = useMutation({
    mutationFn: async () => {
      const res = await fetch('/api/erp-bridge/xml-import/trigger', {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}`, 'X-Company-ID': companyId || '', 'Content-Type': 'application/json' },
        body: JSON.stringify({ data_ini: dataIni, data_fim: dataFim, tipos }),
      });
      if (!res.ok) throw new Error((await res.text()) || 'Erro ao enfileirar');
      return res.json();
    },
    onSuccess: () => {
      toast.success('Importação enfileirada. O conector vai processar quando rodar (modo --drain).');
      queryClient.invalidateQueries({ queryKey: ['erp-xml-jobs'] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const canSubmit = !!dataIni && !!dataFim && dataIni <= dataFim && !trigger.isPending;

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight flex items-center gap-2">
          <Database className="h-6 w-6" /> Importar via ERP — XML (NF-e entrada + CT-e)
        </h1>
        <p className="text-sm text-muted-foreground mt-1">
          Enfileira a importação dos XMLs do ERP (FCCORP) por período. O conector
          <code className="mx-1 px-1 bg-muted rounded">erp-bridge-simulador</code>
          processa os jobs pendentes ao rodar em modo <code className="px-1 bg-muted rounded">--drain</code>.
        </p>
      </div>

      <Card>
        <CardHeader><CardTitle className="text-base">Nova importação</CardTitle></CardHeader>
        <CardContent>
          <div className="flex flex-wrap items-end gap-4">
            <div className="space-y-1">
              <Label htmlFor="di" className="text-xs">Data início</Label>
              <Input id="di" type="date" value={dataIni} onChange={(e) => setDataIni(e.target.value)} className="w-44" />
            </div>
            <div className="space-y-1">
              <Label htmlFor="df" className="text-xs">Data fim</Label>
              <Input id="df" type="date" value={dataFim} onChange={(e) => setDataFim(e.target.value)} className="w-44" />
            </div>
            <div className="space-y-1">
              <Label className="text-xs">Tipos</Label>
              <Select value={tipos} onValueChange={setTipos}>
                <SelectTrigger className="w-52"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="entradas,ctes">NF-e entradas + CT-e</SelectItem>
                  <SelectItem value="entradas">Só NF-e entradas</SelectItem>
                  <SelectItem value="ctes">Só CT-e</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <Button onClick={() => trigger.mutate()} disabled={!canSubmit}>
              {trigger.isPending
                ? <Loader2 className="h-4 w-4 mr-1.5 animate-spin" />
                : <Send className="h-4 w-4 mr-1.5" />}
              Enfileirar importação
            </Button>
          </div>
          <p className="text-xs text-muted-foreground mt-3">
            ⚠ Volume alto: nos meses pesados, enfileire janelas menores (por dia/semana). O backend é
            idempotente e o conector tem dedup — re-enfileirar o mesmo período não duplica dados.
          </p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader><CardTitle className="text-base">Importações</CardTitle></CardHeader>
        <CardContent><ERPXMLJobsTable /></CardContent>
      </Card>
    </div>
  );
}
