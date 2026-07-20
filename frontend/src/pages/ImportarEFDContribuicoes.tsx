import { useState, useRef, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { CheckCircle, Clock, FileText, Loader2, Upload, XCircle, FolderOpen, ShieldCheck, CheckCheck, AlertTriangle } from 'lucide-react';
import { toast } from 'sonner';
import { UploadProgressDisplay, UploadProgressType } from '@/components/UploadProgress';
import { useAuth } from '@/contexts/AuthContext';

interface ImportJob {
  id: string;
  filename: string;
  status: 'pending' | 'processing' | 'completed' | 'error' | 'cancelled';
  created_at: string;
  updated_at: string;
  message?: string;
}

// Extrai "N casadas / M não encontradas" da mensagem final gravada pelo
// worker (processEFDContribuicoesFile) em import_jobs.message, ex.:
// "Importação concluída: 42 casadas, 3 não encontradas (C100 processados: 45)"
function parseMatchSummary(message?: string): { matched: number; notMatched: number } | null {
  if (!message) return null;
  const m = message.match(/(\d+)\s+casadas,\s*(\d+)\s+não encontradas/);
  if (!m) return null;
  return { matched: parseInt(m[1], 10), notMatched: parseInt(m[2], 10) };
}

export default function ImportarEFDContribuicoes() {
  const navigate = useNavigate();
  const { token, companyId } = useAuth();
  const [selectedFiles, setSelectedFiles] = useState<File[]>([]);
  const [isDragOver, setIsDragOver] = useState(false);

  const [uploadProgress, setUploadProgress] = useState<UploadProgressType>({
    status: 'idle',
    percentage: 0,
    bytesUploaded: 0,
    bytesTotal: 0,
    speed: 0,
    remainingTime: 0
  });

  const [currentFileIndex, setCurrentFileIndex] = useState<number>(-1);
  const [isUploading, setIsUploading] = useState(false);
  const [jobs, setJobs] = useState<ImportJob[]>([]);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Poll jobs list — só os jobs deste fluxo (tipo=efd_contribuicoes)
  useEffect(() => {
    let pollInterval: NodeJS.Timeout;

    const fetchJobs = async () => {
      if (!token) return;
      try {
        const res = await fetch('/api/jobs?tipo=efd_contribuicoes', {
          headers: {
            'Cache-Control': 'no-cache',
            'Pragma': 'no-cache'
          }
        });
        if (res.ok) {
          const data = await res.json();
          setJobs(data);
        } else if (res.status === 401) {
          console.error("Unauthorized polling jobs - redirecting");
          clearInterval(pollInterval);
          navigate('/login');
          return;
        }
      } catch (error) {
        console.error("Error polling jobs:", error);
      }
    };

    fetchJobs();
    pollInterval = setInterval(fetchJobs, 2000);
    return () => clearInterval(pollInterval);
  }, [token, navigate]);

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files.length > 0) {
      const allFiles = Array.from(e.target.files);
      const txtFiles = allFiles.filter(file => file.name.toLowerCase().endsWith('.txt'));

      if (txtFiles.length < allFiles.length) {
        toast.info(`${allFiles.length - txtFiles.length} arquivos ignorados (apenas .txt são permitidos).`);
      }

      if (txtFiles.length > 0) {
        setSelectedFiles(prev => [...prev, ...txtFiles]);
      } else if (allFiles.length > 0) {
        toast.warning("Nenhum arquivo .txt encontrado na seleção.");
      }
    }
    if (fileInputRef.current) {
      fileInputRef.current.value = '';
    }
  };

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragOver(true);
  };

  const handleDragLeave = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragOver(false);
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragOver(false);
    if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
      const allFiles = Array.from(e.dataTransfer.files);
      const txtFiles = allFiles.filter(file => file.name.toLowerCase().endsWith('.txt'));

      if (txtFiles.length < allFiles.length) {
        toast.info(`${allFiles.length - txtFiles.length} arquivos ignorados (apenas .txt são permitidos).`);
      }

      if (txtFiles.length > 0) {
        setSelectedFiles(prev => [...prev, ...txtFiles]);
      } else if (allFiles.length > 0) {
        toast.warning("Nenhum arquivo .txt válido encontrado.");
      }
    }
  };

  const removeFile = (index: number) => {
    if (isUploading) return;
    setSelectedFiles(prev => prev.filter((_, i) => i !== index));
  };

  // Upload chunked do arquivo COMPLETO (sem filtro client-side de linhas —
  // o arquivo de EFD Contribuições só nos interessa pelo C100, mas ele é
  // bem menor que um SPED ICMS/IPI completo, então não vale a complexidade
  // de pré-filtrar no browser).
  const processSingleFile = async (file: File): Promise<string | null> => {
    let createdJobId: string | null = null;

    setUploadProgress({
      status: 'uploading',
      percentage: 0,
      bytesUploaded: 0,
      bytesTotal: file.size,
      speed: 0,
      remainingTime: 0
    });

    try {
      const UPLOAD_CHUNK_SIZE = 2 * 1024 * 1024; // 2MB Upload Chunks
      const totalChunks = Math.ceil(file.size / UPLOAD_CHUNK_SIZE);
      const uploadId = `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;

      let startTimeUpload = Date.now();

      for (let chunkIndex = 0; chunkIndex < totalChunks; chunkIndex++) {
        const start = chunkIndex * UPLOAD_CHUNK_SIZE;
        const end = Math.min(start + UPLOAD_CHUNK_SIZE, file.size);
        const chunk = file.slice(start, end);

        const formData = new FormData();
        formData.append('file', chunk, file.name);
        formData.append('is_chunked', 'true');
        formData.append('upload_id', uploadId);
        formData.append('chunk_index', chunkIndex.toString());
        formData.append('total_chunks', totalChunks.toString());
        formData.append('expected_size', file.size.toString());

        if (companyId) {
          formData.append('company_id', companyId);
        }

        const responseJson = await new Promise<any>((resolve, reject) => {
          const xhr = new XMLHttpRequest();
          xhr.open('POST', '/api/efd-contribuicoes/upload', true);

          if (token) {
            xhr.setRequestHeader('Authorization', `Bearer ${token}`);
          }
          if (companyId) {
            xhr.setRequestHeader('X-Company-ID', companyId);
          }

          xhr.onload = () => {
            if (xhr.status >= 200 && xhr.status < 300) {
              try {
                resolve(JSON.parse(xhr.responseText));
              } catch (e) {
                resolve(null);
              }
            } else {
              reject(new Error(`Upload failed: ${xhr.statusText}`));
            }
          };
          xhr.onerror = () => reject(new Error(`Network error at chunk ${chunkIndex}`));
          xhr.send(formData);
        });

        if (chunkIndex === totalChunks - 1 && responseJson?.job_id) {
          createdJobId = responseJson.job_id;
        }

        const percentUpload = ((chunkIndex + 1) / totalChunks) * 100;
        const bytesUploadedSoFar = end;
        const elapsedTime = (Date.now() - startTimeUpload) / 1000;
        const speed = bytesUploadedSoFar / elapsedTime;
        const remainingBytes = file.size - bytesUploadedSoFar;
        const remainingTime = speed > 0 ? remainingBytes / speed : 0;

        setUploadProgress({
          status: 'uploading',
          percentage: Math.round(percentUpload),
          bytesUploaded: bytesUploadedSoFar,
          bytesTotal: file.size,
          speed,
          remainingTime
        });
      }

      setUploadProgress(prev => ({ ...prev, status: 'completed', percentage: 100 }));
      toast.success(`Arquivo ${file.name} enviado!`);
      return createdJobId;

    } catch (error) {
      console.error(`Upload error for ${file.name}:`, error);
      setUploadProgress(prev => ({ ...prev, status: 'error', errorMessage: String(error) }));
      toast.error(`Erro ao enviar ${file.name}.`);
      throw error;
    }
  };

  const handleUploadAll = async () => {
    if (selectedFiles.length === 0) return;

    setIsUploading(true);
    const batchJobIds: string[] = [];

    for (let i = 0; i < selectedFiles.length; i++) {
      setCurrentFileIndex(i);
      try {
        const jobId = await processSingleFile(selectedFiles[i]);
        if (jobId) batchJobIds.push(jobId);
      } catch (err) {
        console.error(`Failed to process file ${i}`, err);
      }
    }

    if (batchJobIds.length === 0) {
      setIsUploading(false);
      setCurrentFileIndex(-1);
      return;
    }

    let allCompleted = false;
    const processingToastId = toast.loading(`Processando ${batchJobIds.length} arquivo(s) no servidor...`);

    while (!allCompleted) {
      await new Promise(r => setTimeout(r, 2000));

      try {
        const res = await fetch('/api/jobs?tipo=efd_contribuicoes', {
          headers: {
            'Cache-Control': 'no-cache',
            'Pragma': 'no-cache'
          }
        });
        if (res.ok) {
          const data: ImportJob[] = await res.json();
          setJobs(data);

          const relevantJobs = data.filter(j => batchJobIds.includes(j.id));
          const pendingCount = relevantJobs.filter(j => j.status === 'pending' || j.status === 'processing').length;
          const foundIds = relevantJobs.map(j => j.id);
          const allFound = batchJobIds.every(id => foundIds.includes(id));

          if (allFound && pendingCount === 0) {
            allCompleted = true;
          }
        } else if (res.status === 401) {
          console.error("Unauthorized polling jobs - redirecting");
          navigate('/login');
          return;
        }
      } catch (e) {
        console.error("Error polling jobs", e);
      }
    }

    toast.dismiss(processingToastId);
    toast.success("Importação concluída! Confira o resumo no histórico abaixo.");
    setIsUploading(false);
    setCurrentFileIndex(-1);
    setSelectedFiles([]);
  };

  return (
    <div className="container mx-auto p-6 space-y-6 animate-fade-in">
      <div className="flex justify-between items-start">
        <div className="flex flex-col gap-2">
          <h1 className="text-3xl font-bold tracking-tight text-primary">Importar EFD Contribuições</h1>
          <p className="text-muted-foreground">
            Envie o arquivo oficial de EFD Contribuições para enriquecer PIS/COFINS das notas já importadas,
            casando cada nota (C100) pela chave de acesso.
          </p>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {/* Upload Card */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Upload className="h-5 w-5" />
              Novo Arquivo
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div
              className={`
                border-2 border-dashed rounded-lg p-8 text-center cursor-pointer transition-colors
                ${isDragOver ? 'border-primary bg-primary/5' : 'border-muted-foreground/25 hover:border-primary/50'}
              `}
              onDragOver={handleDragOver}
              onDragLeave={handleDragLeave}
              onDrop={handleDrop}
            >
              <input
                type="file"
                ref={fileInputRef}
                className="hidden"
                accept=".txt"
                multiple
                onChange={handleFileChange}
              />

              <div className="flex flex-col items-center gap-2">
                <div className="p-4 bg-muted rounded-full">
                  <FolderOpen className="h-8 w-8 text-muted-foreground" />
                </div>
                <div className="space-y-1">
                  <p className="text-sm font-medium">
                    Arraste o arquivo aqui
                  </p>
                  <div className="flex gap-2 justify-center mt-2">
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={(e) => {
                        e.stopPropagation();
                        fileInputRef.current?.click();
                      }}
                    >
                      Selecionar Arquivo(s)
                    </Button>
                  </div>
                  <p className="text-xs text-muted-foreground mt-2">
                    Suporta arquivos .txt (EFD Contribuições)
                  </p>
                </div>
              </div>
            </div>

            {selectedFiles.length > 0 && (
              <div className="space-y-2 max-h-60 overflow-y-auto pr-2">
                <div className="flex justify-between items-center pb-2 border-b">
                  <span className="text-sm font-semibold">{selectedFiles.length} arquivo(s) selecionado(s)</span>
                  <Button
                    size="sm"
                    onClick={handleUploadAll}
                    disabled={isUploading}
                  >
                    {isUploading ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : <Upload className="h-4 w-4 mr-2" />}
                    {isUploading ? 'Enviando...' : 'Enviar Todos'}
                  </Button>
                </div>

                {selectedFiles.map((file, idx) => (
                  <div key={idx} className={`flex items-center justify-between p-2 rounded-md text-sm ${idx === currentFileIndex ? 'bg-primary/10 border border-primary/20' : 'bg-muted/50'}`}>
                    <div className="flex items-center gap-2 overflow-hidden flex-1 min-w-0">
                      <FileText className="h-3 w-3 flex-shrink-0 text-muted-foreground" />
                      <span className="truncate flex-1 min-w-0" title={file.name}>{file.name}</span>
                      <span className="text-xs text-muted-foreground flex-shrink-0">({(file.size / 1024 / 1024).toFixed(2)} MB)</span>
                    </div>

                    {idx === currentFileIndex && isUploading && (
                      <Badge variant="secondary" className="text-xs">Processando...</Badge>
                    )}

                    {!isUploading && (
                      <Button variant="ghost" size="icon" className="h-6 w-6" onClick={() => removeFile(idx)}>
                        <XCircle className="h-4 w-4 text-muted-foreground hover:text-red-500" />
                      </Button>
                    )}
                  </div>
                ))}
              </div>
            )}

            <UploadProgressDisplay
              progress={uploadProgress}
              fileName={currentFileIndex >= 0 ? selectedFiles[currentFileIndex]?.name : ''}
            />
          </CardContent>
        </Card>

        {/* Recent Jobs Card */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Clock className="h-5 w-5" />
              Histórico de Importações
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex items-start gap-2 bg-blue-50 border border-blue-200 rounded-lg px-3 py-2.5 mb-3 text-xs text-blue-800">
              <ShieldCheck className="h-4 w-4 mt-0.5 shrink-0 text-blue-600" />
              <span>
                <strong>Sobrescrita autoritativa:</strong> ao casar uma nota pela chave de acesso,
                os valores de PIS/COFINS são sempre atualizados com o que veio da EFD Contribuições.
              </span>
            </div>

            {jobs.length === 0 ? (
              <div className="text-center py-8 text-muted-foreground">
                <p>Nenhum processamento recente.</p>
              </div>
            ) : (
              <div className="space-y-2 max-h-[500px] overflow-y-auto pr-2">
                {jobs.map((job) => {
                  const summary = job.status === 'completed' ? parseMatchSummary(job.message) : null;
                  return (
                    <div key={job.id} className="flex items-center justify-between p-3 border rounded-lg gap-3">
                      <div className="flex items-center gap-3 flex-1 min-w-0">
                        {job.status === 'completed' && <CheckCircle className="h-4 w-4 text-green-500 flex-shrink-0" />}
                        {job.status === 'processing' && <Loader2 className="h-4 w-4 text-blue-500 animate-spin flex-shrink-0" />}
                        {job.status === 'error' && <XCircle className="h-4 w-4 text-red-500 flex-shrink-0" />}
                        {job.status === 'pending' && <Clock className="h-4 w-4 text-gray-500 flex-shrink-0" />}
                        {job.status === 'cancelled' && <XCircle className="h-4 w-4 text-orange-400 flex-shrink-0" />}

                        <div className="flex flex-col flex-1 min-w-0">
                          <div className="flex justify-between items-center mb-1 gap-2">
                            <span className="text-sm font-medium truncate flex-1 min-w-0" title={job.filename}>{job.filename}</span>
                            <span className="text-xs text-muted-foreground whitespace-nowrap">
                              {new Date(job.created_at).toLocaleString('pt-BR')}
                            </span>
                          </div>

                          {summary ? (
                            <div className="flex items-center gap-3 text-xs mt-0.5">
                              <span className="flex items-center gap-1 text-green-700 font-medium">
                                <CheckCheck className="h-3 w-3" /> {summary.matched} casadas
                              </span>
                              {summary.notMatched > 0 && (
                                <span className="flex items-center gap-1 text-amber-700 font-medium">
                                  <AlertTriangle className="h-3 w-3" /> {summary.notMatched} não encontradas
                                </span>
                              )}
                            </div>
                          ) : (
                            <span className="text-xs text-muted-foreground truncate" title={job.message}>
                              {job.message || 'Aguardando...'}
                            </span>
                          )}
                        </div>
                      </div>
                      <Badge variant={
                        job.status === 'completed' ? 'default' :
                        job.status === 'error' ? 'destructive' :
                        job.status === 'cancelled' ? 'outline' : 'secondary'
                      }>
                        {job.status === 'completed' ? 'Concluído' :
                         job.status === 'processing' ? 'Processando' :
                         job.status === 'error' ? 'Erro' :
                         job.status === 'cancelled' ? 'Cancelado' : 'Aguardando'}
                      </Badge>
                    </div>
                  );
                })}
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
