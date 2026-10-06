// Sim2027PacoteFiscal.tsx — 4ª funcionalidade do módulo "Teste Pacote Fiscal":
// "Simulação 2027". Lê um XML já emitido (NF-e modelo 55 ou CT-e) e gera uma
// versão simulada como se já estivesse sob as regras plenas da Reforma
// Tributária — PIS/COFINS/IPI zerados, IBS/CBS recalculado com alíquotas de
// teste globais e editáveis (backend: pacotefiscal_sim2027_*.go). Stateless
// no frontend também: o arquivo fica em memória só durante a sessão da tela,
// nada é enviado para importação/armazenamento permanente.
import { useEffect, useState } from 'react';
import { useDropzone } from 'react-dropzone';
import { toast } from 'sonner';
import { useAuth } from '@/contexts/AuthContext';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Badge } from '@/components/ui/badge';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Upload, CloudUpload, Loader2, Download, Save } from 'lucide-react';

interface Parametros {
  aliq_ibs_uf_pct: number;
  aliq_ibs_mun_pct: number;
  aliq_cbs_pct: number;
}

interface ItemAntesDepois {
  v_pis: number;
  v_cofins: number;
  v_ipi: number;
  tem_ibs_cbs: boolean;
  v_bc_ibs_cbs?: number;
  p_ibs_uf?: number;
  v_ibs_uf?: number;
  p_ibs_mun?: number;
  v_ibs_mun?: number;
  v_ibs?: number;
  p_cbs?: number;
  v_cbs?: number;
}

interface ItemPreview {
  n_item?: number;
  c_prod?: string;
  x_prod?: string;
  antes: ItemAntesDepois;
  depois: ItemAntesDepois;
}

interface AnalisarResponse {
  tipo_doc: 'nfe' | 'cte';
  chave: string;
  numero: string;
  serie: string;
  parametros: Parametros;
  itens: ItemPreview[];
  total_antes: { v_pis: number; v_cofins: number; v_ipi: number; v_ibs_uf: number; v_ibs_mun: number; v_ibs: number; v_cbs: number };
  total_depois: { v_pis: number; v_cofins: number; v_ipi: number; v_ibs_uf: number; v_ibs_mun: number; v_ibs: number; v_cbs: number };
}

const fmt = (v: number | undefined) =>
  (v ?? 0).toLocaleString('pt-BR', { minimumFractionDigits: 2, maximumFractionDigits: 2 });

export default function Sim2027PacoteFiscal() {
  const { token, companyId, user } = useAuth();
  const isAdmin = user?.role === 'admin';

  const [params, setParams] = useState<Parametros>({ aliq_ibs_uf_pct: 0.05, aliq_ibs_mun_pct: 0.05, aliq_cbs_pct: 9.90 });
  const [loadingParams, setLoadingParams] = useState(true);
  const [savingParams, setSavingParams] = useState(false);

  const [file, setFile] = useState<File | null>(null);
  const [analisando, setAnalisando] = useState(false);
  const [gerando, setGerando] = useState(false);
  const [preview, setPreview] = useState<AnalisarResponse | null>(null);

  const authHeaders = () => ({ Authorization: `Bearer ${token}`, 'X-Company-ID': companyId || '' });

  useEffect(() => {
    (async () => {
      try {
        const res = await fetch('/api/pacotefiscal/sim2027/parametros', { headers: authHeaders() });
        if (res.ok) {
          const data: Parametros = await res.json();
          setParams(data);
        }
      } catch {
        toast.error('Erro ao carregar parâmetros — usando os valores padrão.');
      } finally {
        setLoadingParams(false);
      }
    })();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const salvarParametros = async () => {
    setSavingParams(true);
    try {
      const res = await fetch('/api/pacotefiscal/sim2027/parametros/salvar', {
        method: 'PUT',
        headers: { ...authHeaders(), 'Content-Type': 'application/json' },
        body: JSON.stringify(params),
      });
      if (!res.ok) {
        const text = await res.text();
        toast.error('Erro ao salvar parâmetros: ' + (text || res.statusText));
        return;
      }
      toast.success('Parâmetros salvos com sucesso.');
    } catch (err) {
      toast.error('Erro inesperado ao salvar: ' + String(err));
    } finally {
      setSavingParams(false);
    }
  };

  const analisar = async (f: File) => {
    setAnalisando(true);
    setPreview(null);
    try {
      const formData = new FormData();
      formData.append('xml', f);
      const res = await fetch('/api/pacotefiscal/sim2027/analisar', {
        method: 'POST',
        headers: authHeaders(),
        body: formData,
      });
      if (!res.ok) {
        const text = await res.text();
        toast.error('Erro ao analisar XML: ' + (text || res.statusText));
        setFile(null);
        return;
      }
      const data: AnalisarResponse = await res.json();
      setPreview(data);
      toast.success(`Documento analisado: ${data.tipo_doc === 'nfe' ? 'NF-e' : 'CT-e'} ${data.numero}/${data.serie}.`);
    } catch (err) {
      toast.error('Erro inesperado: ' + String(err));
      setFile(null);
    } finally {
      setAnalisando(false);
    }
  };

  const gerar = async () => {
    if (!file) return;
    setGerando(true);
    try {
      const formData = new FormData();
      formData.append('xml', file);
      const res = await fetch('/api/pacotefiscal/sim2027/gerar', {
        method: 'POST',
        headers: authHeaders(),
        body: formData,
      });
      if (!res.ok) {
        const text = await res.text();
        toast.error('Erro ao gerar XML: ' + (text || res.statusText));
        return;
      }
      const blob = await res.blob();
      const disposition = res.headers.get('Content-Disposition') || '';
      const match = /filename="?([^"]+)"?/.exec(disposition);
      const filename = match ? match[1] : (file.name.replace(/\.xml$/i, '') + '-sim2027.xml');

      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = filename;
      document.body.appendChild(a);
      a.click();
      a.remove();
      window.URL.revokeObjectURL(url);
      toast.success('XML simulado gerado e baixado.');
    } catch (err) {
      toast.error('Erro inesperado ao gerar: ' + String(err));
    } finally {
      setGerando(false);
    }
  };

  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    accept: { 'text/xml': ['.xml'], 'application/xml': ['.xml'] },
    maxSize: 10 * 1024 * 1024,
    multiple: false,
    disabled: analisando || gerando,
    onDropRejected: () => toast.error('Arquivo rejeitado — envie um único .xml de NF-e (modelo 55) ou CT-e.'),
    onDrop: (files) => {
      if (files.length === 0) return;
      const f = files[0];
      setFile(f);
      analisar(f);
    },
  });

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Simulação 2027 — IBS/CBS</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Gera, a partir de uma NF-e (modelo 55) ou CT-e já emitidos, uma versão simulada como se já
          estivessem sob as regras plenas da Reforma Tributária: PIS, COFINS e IPI zerados, e IBS/CBS
          recalculado com as alíquotas de teste abaixo, usando a base de cálculo que já existe no bloco
          IBSCBS do próprio XML. O XML gerado não é válido para transmissão — é só um artefato de teste.
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Parâmetros (alíquotas de teste, globais)</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-wrap items-end gap-4">
          <div className="space-y-1.5">
            <Label htmlFor="ibsUf">IBS UF (%)</Label>
            <Input
              id="ibsUf" type="number" step="0.01" min={0} max={100} className="w-32"
              value={params.aliq_ibs_uf_pct} disabled={!isAdmin || loadingParams}
              onChange={e => setParams(p => ({ ...p, aliq_ibs_uf_pct: Number(e.target.value) }))}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="ibsMun">IBS Municipal (%)</Label>
            <Input
              id="ibsMun" type="number" step="0.01" min={0} max={100} className="w-32"
              value={params.aliq_ibs_mun_pct} disabled={!isAdmin || loadingParams}
              onChange={e => setParams(p => ({ ...p, aliq_ibs_mun_pct: Number(e.target.value) }))}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="cbs">CBS (%)</Label>
            <Input
              id="cbs" type="number" step="0.01" min={0} max={100} className="w-32"
              value={params.aliq_cbs_pct} disabled={!isAdmin || loadingParams}
              onChange={e => setParams(p => ({ ...p, aliq_cbs_pct: Number(e.target.value) }))}
            />
          </div>
          {isAdmin ? (
            <Button onClick={salvarParametros} disabled={savingParams || loadingParams}>
              {savingParams ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : <Save className="h-4 w-4 mr-2" />}
              Salvar parâmetros
            </Button>
          ) : (
            <p className="text-xs text-muted-foreground">Somente administradores podem alterar os parâmetros.</p>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardContent className="pt-6">
          <div
            {...getRootProps()}
            className={[
              'flex flex-col items-center justify-center gap-3 rounded-lg border-2 border-dashed px-6 py-10 cursor-pointer transition-colors',
              isDragActive
                ? 'border-blue-500 bg-blue-50 text-blue-700'
                : (analisando || gerando)
                  ? 'border-muted bg-muted/30 cursor-not-allowed text-muted-foreground'
                  : 'border-muted-foreground/25 hover:border-primary/50 hover:bg-muted/20 text-muted-foreground',
            ].join(' ')}
          >
            <input {...getInputProps()} />
            {analisando ? (
              <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
            ) : isDragActive ? (
              <CloudUpload className="h-8 w-8 text-blue-500" />
            ) : (
              <Upload className="h-8 w-8" />
            )}
            <div className="text-center">
              {analisando && <p className="text-sm font-medium">Analisando documento...</p>}
              {!analisando && (
                <>
                  <p className="text-sm font-medium">Arraste um XML (NF-e 55 ou CT-e) aqui, ou clique</p>
                  <p className="text-xs text-muted-foreground mt-1">Um único arquivo .xml — máximo 10MB</p>
                </>
              )}
            </div>
          </div>
        </CardContent>
      </Card>

      {preview && (
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0">
            <div>
              <CardTitle className="text-base">
                {preview.tipo_doc === 'nfe' ? 'NF-e' : 'CT-e'} nº {preview.numero} série {preview.serie}
              </CardTitle>
              <p className="text-xs text-muted-foreground mt-1">Chave: {preview.chave}</p>
            </div>
            <Button onClick={gerar} disabled={gerando}>
              {gerando ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : <Download className="h-4 w-4 mr-2" />}
              Confirmar e baixar XML simulado
            </Button>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="overflow-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Item</TableHead>
                    <TableHead className="text-right">PIS (antes → depois)</TableHead>
                    <TableHead className="text-right">COFINS (antes → depois)</TableHead>
                    <TableHead className="text-right">IPI (antes → depois)</TableHead>
                    <TableHead className="text-right">IBS UF (antes → depois)</TableHead>
                    <TableHead className="text-right">IBS Mun (antes → depois)</TableHead>
                    <TableHead className="text-right">CBS (antes → depois)</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {preview.itens.map((it, i) => (
                    <TableRow key={i}>
                      <TableCell className="max-w-[220px] truncate" title={it.x_prod}>
                        {it.n_item ? `${it.n_item} — ${it.x_prod}` : (it.x_prod || 'Documento (CT-e)')}
                      </TableCell>
                      <TableCell className="text-right">{fmt(it.antes.v_pis)} → {fmt(it.depois.v_pis)}</TableCell>
                      <TableCell className="text-right">{fmt(it.antes.v_cofins)} → {fmt(it.depois.v_cofins)}</TableCell>
                      <TableCell className="text-right">{fmt(it.antes.v_ipi)} → {fmt(it.depois.v_ipi)}</TableCell>
                      <TableCell className="text-right">
                        {it.antes.tem_ibs_cbs ? `${fmt(it.antes.v_ibs_uf)} → ${fmt(it.depois.v_ibs_uf)}` : (
                          <Badge variant="outline">sem IBS/CBS</Badge>
                        )}
                      </TableCell>
                      <TableCell className="text-right">
                        {it.antes.tem_ibs_cbs ? `${fmt(it.antes.v_ibs_mun)} → ${fmt(it.depois.v_ibs_mun)}` : '—'}
                      </TableCell>
                      <TableCell className="text-right">
                        {it.antes.tem_ibs_cbs ? `${fmt(it.antes.v_cbs)} → ${fmt(it.depois.v_cbs)}` : '—'}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
                <TableBody>
                  <TableRow className="font-medium bg-muted/40">
                    <TableCell>Total</TableCell>
                    <TableCell className="text-right">{fmt(preview.total_antes.v_pis)} → {fmt(preview.total_depois.v_pis)}</TableCell>
                    <TableCell className="text-right">{fmt(preview.total_antes.v_cofins)} → {fmt(preview.total_depois.v_cofins)}</TableCell>
                    <TableCell className="text-right">{fmt(preview.total_antes.v_ipi)} → {fmt(preview.total_depois.v_ipi)}</TableCell>
                    <TableCell className="text-right">{fmt(preview.total_antes.v_ibs_uf)} → {fmt(preview.total_depois.v_ibs_uf)}</TableCell>
                    <TableCell className="text-right">{fmt(preview.total_antes.v_ibs_mun)} → {fmt(preview.total_depois.v_ibs_mun)}</TableCell>
                    <TableCell className="text-right">{fmt(preview.total_antes.v_cbs)} → {fmt(preview.total_depois.v_cbs)}</TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </div>
            <p className="text-xs text-muted-foreground">
              ICMSTot e a duplicata/título (cobr/dup) do documento original não são alterados — a
              simulação altera apenas PIS, COFINS, IPI e IBS/CBS.
            </p>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
