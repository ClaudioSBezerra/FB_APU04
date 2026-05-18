import { useState } from 'react';
import { AlertTriangle, Trash2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { toast } from 'sonner';
import { useAuth } from '@/contexts/AuthContext';
import { ResetDatabaseDialog } from '@/components/ResetDatabaseDialog';

const REQUIRED_TOKEN = 'DELETE-FB_APU04';

const COMPANY_TABLES: { id: string; label: string; description: string }[] = [
  { id: 'import_jobs',        label: 'import_jobs',        description: 'Jobs de importação SPED/EFD e arquivos físicos de upload' },
  { id: 'nfe_entradas',       label: 'nfe_entradas',       description: 'NF-e de entradas (XML + ERP Bridge)' },
  { id: 'nfe_saidas',         label: 'nfe_saidas',         description: 'NF-e de saídas (XML)' },
  { id: 'cte_entradas',       label: 'cte_entradas',       description: 'CT-e de entradas (XML)' },
  { id: 'xml_upload_batches', label: 'xml_upload_batches', description: 'Histórico de uploads de XML' },
  { id: 'filial_apelidos',    label: 'filial_apelidos',    description: 'Apelidos de filiais' },
  { id: 'parceiros',          label: 'parceiros',          description: 'Parceiros do ERP Bridge' },
  { id: 'erp_bridge_runs',    label: 'erp_bridge_runs',    description: 'Runs do ERP Bridge (items deletados via CASCADE)' },
];

export default function LimparDados() {
  const { companyId, company, cnpj, user } = useAuth();
  const isAdmin = user?.role === 'admin';

  // --- per-company state ---
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [token, setToken] = useState('');
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<Record<string, number> | null>(null);

  // --- global reset state ---
  const [resetOpen, setResetOpen] = useState(false);
  const [resetLoading, setResetLoading] = useState(false);

  function toggleAll() {
    if (selected.size === COMPANY_TABLES.length) {
      setSelected(new Set());
    } else {
      setSelected(new Set(COMPANY_TABLES.map((t) => t.id)));
    }
  }

  function toggleTable(id: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  async function handleConfirm() {
    if (!companyId) {
      toast.error('Empresa não identificada. Tente logar novamente.');
      return;
    }
    if (selected.size === 0) {
      toast.error('Selecione ao menos uma tabela.');
      return;
    }
    if (token !== REQUIRED_TOKEN) {
      toast.error('Token de confirmação incorreto.');
      return;
    }

    setLoading(true);
    setResult(null);
    try {
      const res = await fetch('/api/company/reset-data', {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          company_id: companyId,
          tables: Array.from(selected),
          confirmation: token,
        }),
      });
      const data = await res.json().catch(() => ({}));
      if (res.ok) {
        toast.success(data.message ?? 'Dados da empresa limpos com sucesso.');
        setResult(data.rows_deleted ?? {});
        setToken('');
        setSelected(new Set());
        return;
      }
      if (res.status === 400) toast.error('Token de confirmação rejeitado ou tabela inválida.');
      else if (res.status === 403) toast.error('Sem permissão para limpar dados desta empresa.');
      else toast.error(`Erro ${res.status}: ${data.error ?? 'falha ao limpar dados'}.`);
    } catch {
      toast.error('Erro de conexão com o servidor.');
    } finally {
      setLoading(false);
    }
  }

  async function handleGlobalReset(body: { confirmation: string }) {
    setResetLoading(true);
    try {
      const res = await fetch('/api/admin/reset-db', {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      const text = await res.text();
      if (res.ok) {
        toast.success('Base de dados zerada com sucesso. Backup gerado em /backups/.');
        setResetOpen(false);
        return;
      }
      if (res.status === 400)       toast.error('Token de confirmação rejeitado pelo servidor.');
      else if (res.status === 403)  toast.error('Apenas administradores globais podem zerar a base.');
      else if (res.status === 429)  toast.error('Limite de 1 reset por hora atingido.');
      else if (res.status === 503)  toast.error(`Servidor recusou: ${text || 'banco fora do allowlist'}.`);
      else                          toast.error(`Erro ${res.status}: ${text || 'falha ao zerar base'}.`);
    } catch {
      toast.error('Erro de conexão.');
    } finally {
      setResetLoading(false);
    }
  }

  const allSelected = selected.size === COMPANY_TABLES.length;
  const tokenOk = token === REQUIRED_TOKEN;
  const canConfirm = selected.size > 0 && tokenOk && !loading;
  const companyLabel = company ?? cnpj ?? companyId?.substring(0, 8) ?? 'empresa';

  return (
    <div className="container mx-auto max-w-3xl p-6 space-y-6">
      <h1 className="text-2xl font-bold flex items-center gap-2">
        <Trash2 className="h-6 w-6 text-destructive" />
        Limpar Dados
      </h1>

      {/* ── Card per-company ── */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-destructive">
            <AlertTriangle className="h-5 w-5" />
            Limpar dados da empresa atual
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-sm text-muted-foreground">
            Remove apenas os registros da empresa <strong>{companyLabel}</strong>. As demais
            empresas não são afetadas.
          </p>

          {/* select-all + checkboxes */}
          <div className="space-y-2">
            <div className="flex items-center gap-2">
              <Checkbox
                id="select-all"
                checked={allSelected}
                onCheckedChange={toggleAll}
                disabled={loading}
              />
              <Label htmlFor="select-all" className="font-semibold cursor-pointer">
                Selecionar todas
              </Label>
            </div>
            <div className="border rounded-md divide-y">
              {COMPANY_TABLES.map((t) => (
                <div key={t.id} className="flex items-start gap-3 p-3">
                  <Checkbox
                    id={t.id}
                    checked={selected.has(t.id)}
                    onCheckedChange={() => toggleTable(t.id)}
                    disabled={loading}
                    className="mt-0.5"
                  />
                  <Label htmlFor={t.id} className="cursor-pointer leading-snug">
                    <span className="font-mono text-sm">{t.label}</span>
                    <span className="block text-xs text-muted-foreground">{t.description}</span>
                  </Label>
                </div>
              ))}
            </div>
          </div>

          {/* token input */}
          <div className="space-y-1">
            <Label htmlFor="token-empresa">
              Digite{' '}
              <code className="rounded bg-muted px-1 py-0.5 font-mono text-destructive text-xs">
                {REQUIRED_TOKEN}
              </code>{' '}
              para confirmar:
            </Label>
            <Input
              id="token-empresa"
              value={token}
              onChange={(e) => setToken(e.target.value)}
              placeholder={REQUIRED_TOKEN}
              autoComplete="off"
              spellCheck={false}
              disabled={loading}
            />
          </div>

          <Button
            variant="destructive"
            disabled={!canConfirm}
            onClick={handleConfirm}
            className="w-full"
          >
            {loading ? 'Executando...' : `Limpar dados de ${companyLabel}`}
          </Button>

          {/* resultado */}
          {result && (
            <div className="rounded-md border border-green-200 bg-green-50 p-3 text-sm space-y-1">
              <p className="font-semibold text-green-800">Linhas removidas:</p>
              {Object.entries(result).map(([table, rows]) => (
                <div key={table} className="flex justify-between font-mono text-xs text-green-700">
                  <span>{table}</span>
                  <span>{rows}</span>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* ── Card global (admin only) ── */}
      {isAdmin && (
        <Card className="border-destructive/50">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-destructive">
              <AlertTriangle className="h-5 w-5" />
              Zerar base global
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <p className="text-sm text-muted-foreground">
              Trunca as tabelas de <strong>todas as empresas</strong>. Gera backup automático
              antes de executar. Operação irreversível.
            </p>
            <Button
              variant="destructive"
              onClick={() => setResetOpen(true)}
              className="w-full"
            >
              Zerar TODA a base de dados
            </Button>
          </CardContent>
        </Card>
      )}

      <ResetDatabaseDialog
        open={resetOpen}
        onOpenChange={setResetOpen}
        onConfirm={handleGlobalReset}
        loading={resetLoading}
      />
    </div>
  );
}
