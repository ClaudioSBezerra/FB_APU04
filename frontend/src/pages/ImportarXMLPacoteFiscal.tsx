// ImportarXMLPacoteFiscal.tsx — upload de XML ISOLADO e dedicado ao módulo
// Teste Pacote Fiscal (2026-07). Grava em pacotefiscal_nfe_saidas/_itens
// (backend/handlers/pacotefiscal_xml_import.go) — tabelas exclusivas deste
// módulo, não compartilhadas com Painel XMLs/Conciliação/Auditoria Fiscal.
//
// Upload assíncrono com barra de progresso (2026-07): o endpoint responde na
// hora com um job_id e o processamento roda em background no servidor — a
// tela acompanha via polling em /api/pacotefiscal/xml/upload/status. Com
// milhares de XMLs a versão síncrona estourava o timeout do proxy/navegador
// e a tela ficava sem resposta, sem saber se a importação continuava.
import { useEffect, useRef, useState } from 'react';
import { useDropzone } from 'react-dropzone';
import { toast } from 'sonner';
import { Card, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Progress } from '@/components/ui/progress';
import { Upload, CloudUpload, CheckCircle, XCircle, Loader2 } from 'lucide-react';
import { useAuth } from '@/contexts/AuthContext';

interface PFImportErro {
  arquivo: string;
  erro: string;
}

interface PFImportJobStatus {
  job_id: string;
  total: number;
  processed: number;
  importados: number;
  ignorados: number;
  erros: PFImportErro[];
  done: boolean;
}

const POLL_INTERVAL_MS = 1500;

export default function ImportarXMLPacoteFiscal() {
  const { token, companyId } = useAuth();
  const [uploading, setUploading] = useState(false);   // enviando o arquivo (rede)
  const [status, setStatus] = useState<PFImportJobStatus | null>(null); // job em andamento/concluído
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // Encerra o polling ao desmontar a página
  useEffect(() => () => { if (pollRef.current) clearInterval(pollRef.current); }, []);

  const startPolling = (jobId: string) => {
    if (pollRef.current) clearInterval(pollRef.current);
    pollRef.current = setInterval(async () => {
      try {
        const res = await fetch(`/api/pacotefiscal/xml/upload/status?job_id=${encodeURIComponent(jobId)}`, {
          headers: { Authorization: `Bearer ${token}`, 'X-Company-ID': companyId || '' },
        });
        if (res.status === 404) {
          if (pollRef.current) clearInterval(pollRef.current);
          toast.error('Job de importação não encontrado — o servidor pode ter reiniciado. Confira os totais e reenvie se necessário (reimportar é seguro, não duplica).');
          setStatus(prev => prev ? { ...prev, done: true } : prev);
          return;
        }
        if (!res.ok) return; // erro transitório de rede — tenta de novo no próximo tick
        const data: PFImportJobStatus = await res.json();
        setStatus(data);
        if (data.done) {
          if (pollRef.current) clearInterval(pollRef.current);
          toast.success(`Importação concluída: ${data.importados} NF-e(s) importada(s), ${data.ignorados} ignorada(s), ${data.erros.length} erro(s).`);
        }
      } catch {
        // rede oscilou — mantém o polling; o job continua no servidor
      }
    }, POLL_INTERVAL_MS);
  };

  const handleUpload = async (files: File[]) => {
    if (files.length === 0) return;
    setUploading(true);
    setStatus(null);

    try {
      const formData = new FormData();
      files.forEach(f => formData.append('xmls', f));

      const res = await fetch('/api/pacotefiscal/xml/upload', {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}`, 'X-Company-ID': companyId || '' },
        body: formData,
      });

      if (res.status === 413) {
        toast.error('Arquivo excede o limite permitido.');
        return;
      }
      if (!res.ok) {
        const text = await res.text();
        toast.error('Erro no upload: ' + (text || res.statusText));
        return;
      }

      const data: { job_id: string; total: number } = await res.json();
      setStatus({ job_id: data.job_id, total: data.total, processed: 0, importados: 0, ignorados: 0, erros: [], done: false });
      toast.info(`Upload recebido: ${data.total} XML(s) na fila de processamento.`);
      startPolling(data.job_id);
    } catch (err: unknown) {
      toast.error('Erro inesperado: ' + String(err));
    } finally {
      setUploading(false);
    }
  };

  const processing = !!status && !status.done;
  const pct = status && status.total > 0 ? Math.round((status.processed / status.total) * 100) : 0;

  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    accept: {
      'text/xml': ['.xml'],
      'application/zip': ['.zip'],
      'application/x-zip-compressed': ['.zip'],
    },
    maxSize: 512 * 1024 * 1024,
    multiple: true,
    onDropRejected: rejected => {
      toast.error(`${rejected.length} arquivo(s) rejeitado(s). Apenas XML ou ZIP.`);
    },
    onDrop: handleUpload,
    disabled: uploading || processing,
  });

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Importar XML — Teste Pacote Fiscal</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Importação isolada, exclusiva deste módulo — não afeta o Painel XMLs, a Conciliação
          ou a Auditoria Fiscal. Captura cabeçalho completo (emitente e destinatário: razão
          social, IE, endereço, contato) e itens (produtos e impostos) para a Comparação Fiscal.
        </p>
      </div>

      <Card>
        <CardContent className="pt-6">
          <div
            {...getRootProps()}
            className={[
              'flex flex-col items-center justify-center gap-3 rounded-lg border-2 border-dashed px-6 py-10 cursor-pointer transition-colors',
              isDragActive
                ? 'border-blue-500 bg-blue-50 text-blue-700'
                : (uploading || processing)
                  ? 'border-muted bg-muted/30 cursor-not-allowed text-muted-foreground'
                  : 'border-muted-foreground/25 hover:border-primary/50 hover:bg-muted/20 text-muted-foreground',
            ].join(' ')}
          >
            <input {...getInputProps()} />
            {uploading || processing ? (
              <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
            ) : isDragActive ? (
              <CloudUpload className="h-8 w-8 text-blue-500" />
            ) : (
              <Upload className="h-8 w-8" />
            )}
            <div className="text-center">
              {uploading && <p className="text-sm font-medium">Enviando arquivos...</p>}
              {processing && <p className="text-sm font-medium">Processando no servidor — pode fechar esta tela, a importação continua.</p>}
              {!uploading && !processing && isDragActive && <p className="text-sm font-medium">Solte os arquivos aqui</p>}
              {!uploading && !processing && !isDragActive && (
                <>
                  <p className="text-sm font-medium">Arraste XMLs ou um .zip aqui, ou clique</p>
                  <p className="text-xs text-muted-foreground mt-1">Aceita .xml e .zip — máximo 512MB</p>
                </>
              )}
            </div>
          </div>

          {status && (
            <div className="mt-4 rounded-lg border p-4 space-y-3">
              {/* Barra de progresso */}
              <div className="space-y-1.5">
                <div className="flex justify-between text-xs text-muted-foreground">
                  <span>
                    {status.done
                      ? 'Importação concluída'
                      : `Processando ${status.processed.toLocaleString('pt-BR')} de ${status.total.toLocaleString('pt-BR')} XMLs...`}
                  </span>
                  <span className="font-medium">{pct}%</span>
                </div>
                <Progress value={pct} className="h-2" />
              </div>

              <div className="flex gap-4 flex-wrap items-center">
                <div className="flex items-center gap-2">
                  <CheckCircle className="h-4 w-4 text-green-600" />
                  <span className="text-sm font-medium">Importados:</span>
                  <Badge className="bg-green-600">{status.importados.toLocaleString('pt-BR')}</Badge>
                </div>
                {status.ignorados > 0 && (
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium text-muted-foreground">Ignorados:</span>
                    <Badge variant="outline">{status.ignorados.toLocaleString('pt-BR')}</Badge>
                  </div>
                )}
                {status.erros.length > 0 && (
                  <div className="flex items-center gap-2">
                    <XCircle className="h-4 w-4 text-red-500" />
                    <span className="text-sm font-medium">Com erro:</span>
                    <Badge variant="destructive">{status.erros.length}</Badge>
                  </div>
                )}
              </div>
              {status.erros.length > 0 && (
                <div className="text-xs space-y-1 max-h-40 overflow-auto bg-red-50 rounded p-2">
                  {status.erros.map((e, i) => (
                    <div key={i} className="text-red-700">
                      <span className="font-medium">{e.arquivo}:</span> {e.erro}
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
