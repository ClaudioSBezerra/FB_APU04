// ImportarXMLPacoteFiscal.tsx — upload de XML ISOLADO e dedicado ao módulo
// Teste Pacote Fiscal (2026-07). Grava em pacotefiscal_nfe_saidas/_itens
// (backend/handlers/pacotefiscal_xml_import.go) — tabelas exclusivas deste
// módulo, não compartilhadas com Painel XMLs/Conciliação/Auditoria Fiscal.
//
// Resposta do endpoint é síncrona (sem batch/polling, diferente de
// ImportarXMLsSaida.tsx) — volume esperado deste módulo é bem menor
// (validação pontual de notas contra o pacote fiscal, não import em massa).
import { useState } from 'react';
import { useDropzone } from 'react-dropzone';
import { toast } from 'sonner';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Upload, CloudUpload, CheckCircle, XCircle, Loader2 } from 'lucide-react';
import { useAuth } from '@/contexts/AuthContext';

interface PFImportErro {
  arquivo: string;
  erro: string;
}

interface PFImportResult {
  importados: number;
  ignorados: number;
  erros: PFImportErro[];
}

export default function ImportarXMLPacoteFiscal() {
  const { token, companyId } = useAuth();
  const [uploading, setUploading] = useState(false);
  const [result, setResult] = useState<PFImportResult | null>(null);

  const handleUpload = async (files: File[]) => {
    if (files.length === 0) return;
    setUploading(true);
    setResult(null);

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

      const data: PFImportResult = await res.json();
      if (!res.ok) {
        toast.error('Erro no upload: ' + res.statusText);
        return;
      }

      setResult(data);
      toast.success(`Upload concluído: ${data.importados} NF-e(s) importada(s) para o Teste Pacote Fiscal.`);
    } catch (err: unknown) {
      toast.error('Erro inesperado: ' + String(err));
    } finally {
      setUploading(false);
    }
  };

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
    disabled: uploading,
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
                : uploading
                  ? 'border-muted bg-muted/30 cursor-not-allowed text-muted-foreground'
                  : 'border-muted-foreground/25 hover:border-primary/50 hover:bg-muted/20 text-muted-foreground',
            ].join(' ')}
          >
            <input {...getInputProps()} />
            {uploading ? (
              <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
            ) : isDragActive ? (
              <CloudUpload className="h-8 w-8 text-blue-500" />
            ) : (
              <Upload className="h-8 w-8" />
            )}
            <div className="text-center">
              {uploading && <p className="text-sm font-medium">Enviando e processando...</p>}
              {!uploading && isDragActive && <p className="text-sm font-medium">Solte os arquivos aqui</p>}
              {!uploading && !isDragActive && (
                <>
                  <p className="text-sm font-medium">Arraste XMLs ou um .zip aqui, ou clique</p>
                  <p className="text-xs text-muted-foreground mt-1">Aceita .xml e .zip — máximo 512MB</p>
                </>
              )}
            </div>
          </div>

          {result && (
            <div className="mt-4 rounded-lg border p-4 space-y-3">
              <div className="flex gap-4 flex-wrap items-center">
                <div className="flex items-center gap-2">
                  <CheckCircle className="h-4 w-4 text-green-600" />
                  <span className="text-sm font-medium">Importados:</span>
                  <Badge className="bg-green-600">{result.importados}</Badge>
                </div>
                {result.ignorados > 0 && (
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium text-muted-foreground">Ignorados:</span>
                    <Badge variant="outline">{result.ignorados}</Badge>
                  </div>
                )}
                {result.erros.length > 0 && (
                  <div className="flex items-center gap-2">
                    <XCircle className="h-4 w-4 text-red-500" />
                    <span className="text-sm font-medium">Com erro:</span>
                    <Badge variant="destructive">{result.erros.length}</Badge>
                  </div>
                )}
              </div>
              {result.erros.length > 0 && (
                <div className="text-xs space-y-1 max-h-40 overflow-auto bg-red-50 rounded p-2">
                  {result.erros.map((e, i) => (
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
