import { useState } from 'react';
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
import { Upload, CloudUpload, CheckCircle, XCircle, Loader2 } from 'lucide-react';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------
type UploadState = 'idle' | 'uploading' | 'polling' | 'done' | 'error';

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

  // ── Polling do status do batch ─────────────────────────────────────────────
  const { data: batchStatus } = useQuery<BatchStatus>({
    queryKey: ['xml-batch', batchId],
    queryFn: async () => {
      const res = await fetch(`/api/xml/upload-batches/${batchId}/status`);
      if (!res.ok) throw new Error(res.statusText);
      return res.json();
    },
    enabled: !!batchId && uploadState === 'polling',
    refetchInterval: uploadState === 'polling' ? 2000 : false,
    select: (data) => {
      const pct = data.total_count > 0
        ? Math.round((data.processed_count / data.total_count) * 100)
        : 0;
      setProgress(pct);
      if (data.status === 'done') {
        setUploadState('done');
        setUploadResult({
          imported: data.imported_count,
          rejected: data.rejected_count,
          total: data.total_count,
          errorDetails: data.error_details,
        });
        toast.success(
          `Upload concluído: ${data.imported_count} NF-e(s) importadas, ${data.rejected_count} rejeitadas.`
        );
      } else if (data.status === 'failed') {
        setUploadState('error');
        toast.error('Processamento falhou. Verifique os detalhes abaixo.');
      }
      return data;
    },
  });

  // ── Histórico de uploads ───────────────────────────────────────────────────
  const { data: historico, refetch: refetchHistorico } = useQuery<{ items: BatchHistoryRow[] }>({
    queryKey: ['xml-historico', TIPO],
    queryFn: async () => {
      const res = await fetch(`/api/xml/upload-batches?tipo=${TIPO}&limit=10`);
      if (!res.ok) throw new Error(res.statusText);
      return res.json();
    },
    refetchInterval: uploadState === 'polling' ? 3000 : false,
  });

  // ── Upload handler ─────────────────────────────────────────────────────────
  const handleUpload = async (files: File[]) => {
    if (files.length === 0) return;

    setUploadState('uploading');
    setUploadResult(null);
    setBatchId(null);
    setProgress(0);

    try {
      const formData = new FormData();
      formData.append('tipo', TIPO);
      files.forEach(f => formData.append('file', f));

      const res = await fetch('/api/xml/upload', {
        method: 'POST',
        body: formData,
      });

      if (res.status === 413) {
        toast.error('Arquivo excede o limite de 100MB.');
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
        // Processamento assíncrono — iniciar polling
        setBatchId(data.batch_id);
        setUploadState('polling');
        toast.success('Upload recebido. Processando em background...');
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

  // ── Dropzone ───────────────────────────────────────────────────────────────
  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    accept: {
      'text/xml': ['.xml'],
      'application/zip': ['.zip'],
      'application/x-zip-compressed': ['.zip'],
    },
    maxSize: 100 * 1024 * 1024,
    multiple: true,
    onDropRejected: (rejected) => {
      toast.error(`${rejected.length} arquivo(s) rejeitado(s). Apenas XML e ZIP até 100MB.`);
    },
    onDrop: handleUpload,
    disabled: uploadState === 'uploading' || uploadState === 'polling',
  });

  const isProcessing = uploadState === 'uploading' || uploadState === 'polling';

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Importar XMLs de Entrada</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Importe NF-e (mod. 55) de entrada recebidas de fornecedores. Arraste arquivos XML ou ZIP,
          ou clique para selecionar. Limite: 100MB por envio.
        </p>
      </div>

      {/* ── Zona de drag-and-drop ── */}
      <Card>
        <CardContent className="pt-6">
          <div
            {...getRootProps()}
            className={[
              'flex flex-col items-center justify-center gap-3 rounded-lg border-2 border-dashed px-6 py-10 cursor-pointer transition-colors',
              isDragActive
                ? 'border-blue-500 bg-blue-50 text-blue-700'
                : isProcessing
                  ? 'border-muted bg-muted/30 cursor-not-allowed text-muted-foreground'
                  : 'border-muted-foreground/25 hover:border-primary/50 hover:bg-muted/20 text-muted-foreground',
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
              {uploadState === 'uploading' && <p className="text-sm font-medium">Enviando arquivos...</p>}
              {uploadState === 'polling' && <p className="text-sm font-medium">Processando XMLs...</p>}
              {!isProcessing && isDragActive && <p className="text-sm font-medium">Solte os arquivos aqui</p>}
              {!isProcessing && !isDragActive && (
                <>
                  <p className="text-sm font-medium">Arraste XMLs ou ZIP aqui, ou clique para selecionar</p>
                  <p className="text-xs text-muted-foreground mt-1">Aceita .xml e .zip — máximo 100MB</p>
                </>
              )}
            </div>
          </div>

          {/* ── Barra de progresso durante polling ── */}
          {uploadState === 'polling' && (
            <div className="mt-4 space-y-1.5">
              <div className="flex justify-between text-xs text-muted-foreground">
                <span>Processando... {batchStatus?.processed_count ?? 0} / {batchStatus?.total_count ?? '?'} XMLs</span>
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
          {!historico || historico.items.length === 0 ? (
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
                    <TableHead className="py-1.5 px-2 text-[11px]">Total</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px] text-green-700">Importados</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px] text-red-600">Rejeitados</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px]">Status</TableHead>
                    <TableHead className="py-1.5 px-2 text-[11px]">Usuário</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {historico.items.map(row => (
                    <TableRow key={row.id} className="h-8">
                      <TableCell className="py-1 px-2 text-[11px] whitespace-nowrap">{fmtDateTime(row.created_at)}</TableCell>
                      <TableCell className="py-1 px-2 text-[11px] max-w-[200px] truncate" title={row.filename}>{row.filename}</TableCell>
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
