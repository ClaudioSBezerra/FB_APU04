import { useState, useEffect, useRef } from 'react';
import { useDropzone } from 'react-dropzone';
import { useQuery } from '@tanstack/react-query';
import { toast } from 'sonner';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Progress } from '@/components/ui/progress';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Upload, CloudUpload, CheckCircle, XCircle, Loader2, FolderOpen } from 'lucide-react';
import { isValidCompetencia } from '@/lib/utils';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------
type UploadState = 'idle' | 'scanning' | 'uploading' | 'polling' | 'done' | 'error';

interface BatchStatus {
  id: string;
  status: 'pending' | 'processing' | 'done' | 'failed';
  total_count: number;
  processed_count: number;
  imported_count: number;
  rejected_count: number;
  error_details: { filename: string; motivo: string }[] | null;
}

interface BatchHistoryRow {
  id: string;
  created_at: string;
  filename: string;
  tipo: string;
  total_count: number;
  imported_count: number;
  rejected_count: number;
  status: string;
  user_email: string;
  competencia?: string;
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
function fmtDateTime(iso: string): string {
  if (!iso) return '—';
  return new Date(iso).toLocaleString('pt-BR', { dateStyle: 'short', timeStyle: 'short' });
}

function StatusBadge({ status }: { status: string }) {
  const map: Record<string, { label: string; className: string }> = {
    done:       { label: 'Concluído',   className: 'bg-green-100 text-green-700 border-green-200' },
    processing: { label: 'Processando', className: 'bg-blue-100 text-blue-700 border-blue-200' },
    failed:     { label: 'Erro',        className: 'bg-red-100 text-red-700 border-red-200' },
    pending:    { label: 'Aguardando',  className: 'bg-gray-100 text-gray-500 border-gray-200' },
  };
  const s = map[status] ?? { label: status, className: 'bg-gray-100 text-gray-600' };
  return (
    <Badge variant="outline" className={`text-[10px] px-1.5 py-0 ${s.className}`}>{s.label}</Badge>
  );
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------
export default function ImportarXMLsEntrada() {
  const TIPO = 'entradas';

  const [uploadState, setUploadState] = useState<UploadState>('idle');
  const [batchId, setBatchId] = useState<string | null>(null);
  const [uploadResult, setUploadResult] = useState<{
    imported: number; rejected: number; total: number;
    errorDetails: { filename: string; motivo: string }[] | null;
  } | null>(null);
  const [progress, setProgress] = useState(0);
  const [totalBatches, setTotalBatches] = useState(1);
  const [totalXMLs, setTotalXMLs] = useState(0);
  const [uploadStartTime, setUploadStartTime] = useState<Date | null>(null);
  const [competencia, setCompetencia] = useState('');

  // ── Polling do status do batch (lote único ou primeiro lote) ──────────────
  const { data: batchStatus } = useQuery<BatchStatus>({
    queryKey: ['xml-batch', batchId],
    queryFn: async () => {
      const res = await fetch(`/api/xml/upload-batches/${batchId}/status`);
      if (!res.ok) throw new Error(res.statusText);
      return res.json();
    },
    enabled: !!batchId && uploadState === 'polling' && totalBatches <= 1,
    refetchInterval: uploadState === 'polling' && totalBatches <= 1 ? 2000 : false,
  });

  // ── Histórico de uploads ───────────────────────────────────────────────────
  const histLimit = Math.max(10, totalBatches + 5);
  const { data: historico, refetch: refetchHistorico } = useQuery<{ items: BatchHistoryRow[] }>({
    queryKey: ['xml-historico', TIPO, histLimit],
    queryFn: async () => {
      const res = await fetch(`/api/xml/upload-batches?tipo=${TIPO}&limit=${histLimit}`);
      if (!res.ok) throw new Error(res.statusText);
      return res.json();
    },
    refetchInterval: uploadState === 'polling' ? 3000 : false,
  });

  // Efeito: lote único
  useEffect(() => {
    if (!batchStatus || totalBatches > 1) return;
    const pct = batchStatus.total_count > 0
      ? Math.round((batchStatus.processed_count / batchStatus.total_count) * 100)
      : 0;
    setProgress(pct);
    if (batchStatus.status === 'done') {
      setUploadState('done');
      setUploadResult({
        imported: batchStatus.imported_count,
        rejected: batchStatus.rejected_count,
        total: batchStatus.total_count,
        errorDetails: batchStatus.error_details,
      });
      toast.success(
        `Upload concluído: ${batchStatus.imported_count} NF-e(s) importadas, ${batchStatus.rejected_count} rejeitadas.`
      );
      refetchHistorico();
    } else if (batchStatus.status === 'failed') {
      setUploadState('error');
      toast.error('Processamento falhou. Verifique os detalhes abaixo.');
    }
  }, [batchStatus, totalBatches, refetchHistorico]);

  // Efeito: multi-lote — acompanha progresso pelo histórico agregado
  useEffect(() => {
    if (totalBatches <= 1 || uploadState !== 'polling') return;
    if (!historico?.items || !uploadStartTime) return;
    const startMs = uploadStartTime.getTime() - 5000;
    const ourBatches = historico.items.filter(
      item => new Date(item.created_at).getTime() >= startMs
    );
    if (ourBatches.length === 0) return;
    const doneBatches = ourBatches.filter(b => b.status === 'done' || b.status === 'failed').length;
    const totalImported = ourBatches.reduce((sum, b) => sum + (b.imported_count ?? 0), 0);
    const totalRejected = ourBatches.reduce((sum, b) => sum + (b.rejected_count ?? 0), 0);
    setProgress(Math.min(Math.round((doneBatches / totalBatches) * 100), doneBatches >= totalBatches ? 100 : 99));
    if (doneBatches >= totalBatches) {
      setUploadState('done');
      setUploadResult({ imported: totalImported, rejected: totalRejected, total: totalXMLs, errorDetails: null });
      toast.success(`Upload concluído: ${totalImported} NF-e(s) importadas, ${totalRejected} rejeitadas.`);
      refetchHistorico();
    }
  }, [historico, totalBatches, uploadState, uploadStartTime, totalXMLs, refetchHistorico]);

  // ── Upload handler ─────────────────────────────────────────────────────────
  const handleUpload = async (files: File[]) => {
    if (files.length === 0) return;
    if (!isValidCompetencia(competencia)) {
      toast.error("Informe o mês de competência (MM/YYYY) antes de importar.");
      return;
    }

    setUploadState('scanning');
    setUploadResult(null);
    setBatchId(null);
    setProgress(0);
    setTotalBatches(1);
    setTotalXMLs(0);
    setUploadStartTime(null);

    await new Promise(resolve => setTimeout(resolve, 0));

    setUploadState('uploading');

    try {
      const formData = new FormData();
      formData.append('tipo', TIPO);
      formData.append('competencia', competencia);
      files.forEach(f => formData.append('file', f));

      const uploadStart = new Date(); // capturado antes do fetch — batches são criados durante o request
      const res = await fetch('/api/xml/upload', {
        method: 'POST',
        body: formData,
      });

      if (res.status === 413) {
        toast.error('Arquivo excede o limite de 2GB.');
        setUploadState('error');
        return;
      }

      const data = await res.json();

      if (!res.ok) {
        toast.error('Erro no upload: ' + (data.error || res.statusText));
        setUploadState('error');
        return;
      }

      if (res.status === 202 && data.batch_id) {
        const tb = data.total_batches ?? 1;
        setBatchId(data.batch_id);
        setTotalBatches(tb);
        setTotalXMLs(data.total_count ?? 0);
        setUploadStartTime(uploadStart);
        setUploadState('polling');
        toast.success(tb > 1
          ? `Upload recebido. Processando ${tb} lotes em background...`
          : 'Upload recebido. Processando em background...');
      } else {
        // Processamento síncrono (< 50 XMLs) — resultado imediato
        setUploadState('done');
        setUploadResult({
          imported: data.imported_count ?? data.importados ?? 0,
          rejected: data.rejected_count ?? data.ignorados ?? 0,
          total: data.total_count ?? data.total ?? 0,
          errorDetails: data.error_details ?? null,
        });
        toast.success(`Upload concluído: ${data.imported_count ?? data.importados ?? 0} NF-e(s) importadas.`);
        refetchHistorico();
      }
    } catch (err: unknown) {
      toast.error('Erro inesperado: ' + String(err));
      setUploadState('error');
    }
  };

  const competenciaValida = isValidCompetencia(competencia);

  // ── Dropzone ───────────────────────────────────────────────────────────────
  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    accept: {
      'text/xml': ['.xml'],
      'application/zip': ['.zip'],
      'application/x-zip-compressed': ['.zip'],
      'application/x-rar-compressed': ['.rar'],
    },
    maxSize: 2 * 1024 * 1024 * 1024,
    multiple: true,
    onDropRejected: (rejected) => {
      toast.error(`${rejected.length} arquivo(s) rejeitado(s). Apenas XML, ZIP e RAR até 2GB.`);
    },
    onDrop: handleUpload,
    disabled: uploadState === 'uploading' || uploadState === 'polling' || !competenciaValida,
  });

  const isProcessing = uploadState === 'scanning' || uploadState === 'uploading' || uploadState === 'polling';

  const folderInputRef = useRef<HTMLInputElement>(null);
  const handleFolderSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files ?? []).filter(f => f.name.toLowerCase().endsWith('.xml'));
    e.target.value = '';
    if (files.length === 0) { toast.error('Nenhum arquivo .xml encontrado na pasta.'); return; }
    handleUpload(files);
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Importar XMLs de Entrada</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Importe NF-e (mod. 55) de entrada recebidas de fornecedores. Arraste arquivos XML ou ZIP,
          ou clique para selecionar. Limite: 2GB por envio.
        </p>
      </div>

      {/* ── Zona de drag-and-drop ── */}
      <Card>
        <CardContent className="pt-6">

          {/* ── Competência ── */}
          <div className="mb-4 flex flex-wrap items-center gap-3">
            <div>
              <label className="text-sm font-medium block mb-1">
                Mês de competência
                <span className="text-red-600 text-xs ml-1">(obrigatório)</span>
              </label>
              <input
                type="text"
                placeholder="MM/YYYY"
                maxLength={7}
                required
                value={competencia}
                onChange={e => setCompetencia(e.target.value)}
                disabled={isProcessing}
                className={[
                  'border rounded-md px-2 py-1.5 text-sm bg-background disabled:opacity-50 disabled:cursor-not-allowed w-28',
                  competencia && !isValidCompetencia(competencia) ? 'border-red-500' : '',
                ].join(' ')}
              />
            </div>
            <p className="text-xs text-muted-foreground self-end pb-0.5">
              {competencia && !isValidCompetencia(competencia)
                ? <span className="text-red-600">Formato inválido — use MM/YYYY (ex: 03/2026).</span>
                : 'Mês de referência aplicado a todas as notas deste envio (substitui a data de emissão do XML).'}
            </p>
          </div>

          <div
            {...getRootProps()}
            className={[
              'flex flex-col items-center justify-center gap-3 rounded-lg border-2 border-dashed px-6 py-10 transition-colors',
              !competenciaValida
                ? 'border-muted bg-muted/30 cursor-not-allowed text-muted-foreground'
                : isDragActive
                  ? 'border-blue-500 bg-blue-50 text-blue-700 cursor-pointer'
                  : isProcessing
                    ? 'border-muted bg-muted/30 cursor-not-allowed text-muted-foreground'
                    : 'border-muted-foreground/25 hover:border-primary/50 hover:bg-muted/20 text-muted-foreground cursor-pointer',
            ].join(' ')}
          >
            <input {...getInputProps()} />
            {isProcessing ? (
              <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
            ) : isDragActive ? (
              <CloudUpload className="h-8 w-8 text-blue-500" />
            ) : (
              <Upload className="h-8 w-8" />
            )}
            <div className="text-center">
              {uploadState === 'scanning'  && <p className="text-sm font-medium">Lendo arquivos...</p>}
              {uploadState === 'uploading' && <p className="text-sm font-medium">Enviando arquivos...</p>}
              {uploadState === 'polling'   && <p className="text-sm font-medium">Processando XMLs...</p>}
              {!isProcessing && !competenciaValida && (
                <p className="text-sm font-medium">Informe o mês de competência acima para liberar o envio</p>
              )}
              {!isProcessing && competenciaValida && isDragActive && <p className="text-sm font-medium">Solte os arquivos aqui</p>}
              {!isProcessing && competenciaValida && !isDragActive && (
                <>
                  <p className="text-sm font-medium">Arraste XMLs ou compactados (.zip/.rar) aqui, ou clique</p>
                  <p className="text-xs text-muted-foreground mt-1">Aceita .xml, .zip, .rar — máximo 2GB</p>
                  <button
                    type="button"
                    onClick={e => { e.stopPropagation(); folderInputRef.current?.click(); }}
                    disabled={isProcessing}
                    className="mt-2 inline-flex items-center gap-1.5 text-xs text-primary hover:underline disabled:opacity-50"
                  >
                    <FolderOpen className="h-3.5 w-3.5" />
                    Selecionar Pasta
                  </button>
                  <input ref={folderInputRef} type="file" className="hidden" onChange={handleFolderSelect}
                    {...({ webkitdirectory: '', directory: '' } as React.InputHTMLAttributes<HTMLInputElement>)} />
                </>
              )}
            </div>
          </div>

          {/* ── Barra de progresso durante polling ── */}
          {uploadState === 'polling' && (
            <div className="mt-4 space-y-1.5">
              <div className="flex justify-between text-xs text-muted-foreground">
                {totalBatches > 1 ? (
                  <span>
                    {(() => {
                      const startMs = (uploadStartTime?.getTime() ?? 0) - 5000;
                      const done = historico?.items?.filter(b => new Date(b.created_at).getTime() >= startMs && (b.status === 'done' || b.status === 'failed')).length ?? 0;
                      return `Lotes concluídos: ${done} / ${totalBatches} (${totalXMLs.toLocaleString('pt-BR')} XMLs no total)`;
                    })()}
                  </span>
                ) : (
                  <span>Processando... {batchStatus?.processed_count ?? 0} / {batchStatus?.total_count ?? '?'} XMLs</span>
                )}
                <span>{progress}%</span>
              </div>
              <Progress value={progress} className="h-2" />
            </div>
          )}

          {/* ── Resultado do upload ── */}
          {uploadResult && (uploadState === 'done' || uploadState === 'error') && (
            <div className="mt-4 rounded-lg border p-4 space-y-3">
              <div className="flex gap-4 flex-wrap items-center">
                <div className="flex items-center gap-2">
                  <CheckCircle className="h-4 w-4 text-green-600" />
                  <span className="text-sm font-medium">Importados:</span>
                  <Badge className="bg-green-600">{uploadResult.imported}</Badge>
                </div>
                {uploadResult.rejected > 0 && (
                  <div className="flex items-center gap-2">
                    <XCircle className="h-4 w-4 text-red-500" />
                    <span className="text-sm font-medium">Rejeitados:</span>
                    <Badge variant="destructive">{uploadResult.rejected}</Badge>
                  </div>
                )}
                <span className="text-xs text-muted-foreground ml-auto">Total: {uploadResult.total}</span>
              </div>
              {uploadResult.errorDetails && uploadResult.errorDetails.length > 0 && (
                <div className="text-xs space-y-1 max-h-40 overflow-auto bg-red-50 rounded p-2">
                  {uploadResult.errorDetails.map((e, i) => (
                    <div key={i} className="text-red-700">
                      <span className="font-medium">{e.filename}:</span> {e.motivo}
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}
        </CardContent>
      </Card>

      {/* ── Histórico de uploads ── */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Histórico de Uploads — NF-e Entradas</CardTitle>
        </CardHeader>
        <CardContent>
          {!historico?.items?.length ? (
            <p className="text-sm text-muted-foreground text-center py-6">
              Nenhum upload registrado ainda.
            </p>
          ) : (
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow className="hover:bg-transparent">
                    <TableHead className="py-1.5 px-2 text-[11px]">Data</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px]">Arquivo</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px]">Competência</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px]">Total</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px] text-green-700">Importados</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px] text-red-600">Rejeitados</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px]">Status</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px]">Usuário</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {(historico.items ?? []).map(row => (
                    <TableRow key={row.id} className="h-8">
                      <TableCell className="py-1 px-2 text-[11px] whitespace-nowrap">{fmtDateTime(row.created_at)}</TableCell>
                      <TableCell className="py-1 px-2 text-[11px] max-w-[200px] truncate" title={row.filename}>{row.filename}</TableCell>
                      <TableCell className="py-1 px-2 text-[11px] text-center">
                        {row.competencia
                          ? <Badge variant="outline" className="text-[10px] px-1.5 py-0 bg-blue-50 text-blue-700 border-blue-200">{row.competencia}</Badge>
                          : <span className="text-muted-foreground">—</span>}
                      </TableCell>
                      <TableCell className="py-1 px-2 text-[11px] text-center">{row.total_count}</TableCell>
                      <TableCell className="py-1 px-2 text-[11px] text-center text-green-700 font-medium">{row.imported_count}</TableCell>
                      <TableCell className="py-1 px-2 text-[11px] text-center text-red-600">{row.rejected_count || '—'}</TableCell>
                      <TableCell className="py-1 px-2"><StatusBadge status={row.status} /></TableCell>
                      <TableCell className="py-1 px-2 text-[11px] text-muted-foreground truncate max-w-[140px]">{row.user_email || '—'}</TableCell>
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
