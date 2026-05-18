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

const GROUPS: { id: string; label: string; description: string; tables: string[] }[] = [
  {
    id: 'sped',
    label: 'SPED / EFD',
    description: 'Jobs de importação SPED e dados processados (reg_c100, reg_c190… via CASCADE)',
    tables: ['import_jobs'],
  },
  {
    id: 'xml',
    label: 'XML Upload',
    description: 'Documentos importados via upload manual de arquivos XML',
    tables: ['nfe_entradas[xml]', 'nfe_saidas[xml]', 'cte_entradas[xml]', 'xml_upload_batches'],
  },
  {
    id: 'erp_bridge',
    label: 'ERP Bridge',
    description: 'Documentos e runs importados via integração ERP Bridge (oracle_bridge)',
    tables: ['nfe_entradas[erp_bridge]', 'nfe_saidas[erp_bridge]', 'cte_entradas[erp_bridge]', 'erp_bridge_runs', 'parceiros'],
  },
  {
    id: 'config',
    label: 'Configurações',
    description: 'Apelidos de filiais',
    tables: ['filial_apelidos'],
  },
];

export default function LimparDados() {
  const { companyId, company, cnpj, user } = useAuth();
  const isAdmin = user?.role === 'admin';

  // --- per-company state ---
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [token, setToken] = useState('');
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<Record<string, number> | null>(null);
  const [rowsBefore, setRowsBefore] = useState<Record<string, number> | null>(null);
  const [rowsTotal, setRowsTotal] = useState<Record<string, number> | null>(null);
  const [usedCompanyId, setUsedCompanyId] = useState<string | null>(null);

  // --- global reset state ---
  const [resetOpen, setResetOpen] = useState(false);
  const [resetLoading, setResetLoading] = useState(false);

  function toggleAll() {
    if (selected.size === GROUPS.length) {
      setSelected(new Set());
    } else {
      setSelected(new Set(GROUPS.map((g) => g.id)));
    }
  }

  function toggleGroup(id: string) {
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
      toast.error('Selecione ao menos um grupo.');
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
          groups: Array.from(selected),
          confirmation: token,
        }),
      });
      const data = await res.json().catch(() => ({}));
      if (res.ok) {
        const deleted: Record<string, number> = data.rows_deleted ?? {};
        const before: Record<string, number> = data.rows_before ?? {};
        const totals: Record<string, number> = data.rows_total ?? {};
        const totalDeleted = Object.values(deleted).reduce((a: number, b) => a + (b as number), 0);
        const totalExisting = Object.values(totals).reduce((a: number, b) => a + (b as number), 0);
        if (totalDeleted > 0) {
          toast.success(`${totalDeleted} registro(s) removido(s) com sucesso.`);
        } else if (totalExisting === 0) {
          toast.warning('Esta empresa não possui registros nas tabelas selecionadas. Verifique se está na empresa correta.');
        } else {
          toast.warning('Registros existem mas com source diferente. Selecione o grupo correto de origem.');
        }
        setResult(deleted);
        setRowsBefore(before);
        setRowsTotal(totals);
        setUsedCompanyId(data.company_id ?? null);
        setToken('');
        setSelected(new Set());
        return;
      }
      if (res.status === 400) toast.error('Token de confirmação rejeitado ou grupo inválido.');
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

  const allSelected = selected.size === GROUPS.length;
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
            Remove apenas os registros da empresa <strong>{companyLabel}</strong> para os grupos
            selecionados. As demais empresas não são afetadas.
          </p>

          {/* select-all + group checkboxes */}
          <div className="space-y-2">
            <div className="flex items-center gap-2">
              <Checkbox
                id="select-all"
                checked={allSelected}
                onCheckedChange={toggleAll}
                disabled={loading}
              />
              <Label htmlFor="select-all" className="font-semibold cursor-pointer">
                Selecionar todos os grupos
              </Label>
            </div>
            <div className="border rounded-md divide-y">
              {GROUPS.map((g) => (
                <div key={g.id} className="p-3 space-y-1">
                  <div className="flex items-start gap-3">
                    <Checkbox
                      id={g.id}
                      checked={selected.has(g.id)}
                      onCheckedChange={() => toggleGroup(g.id)}
                      disabled={loading}
                      className="mt-0.5"
                    />
                    <Label htmlFor={g.id} className="cursor-pointer leading-snug flex-1">
                      <span className="font-semibold">{g.label}</span>
                      <span className="block text-xs text-muted-foreground">{g.description}</span>
                    </Label>
                  </div>
                  <div className="pl-7 flex flex-wrap gap-1">
                    {g.tables.map((t) => (
                      <span key={t} className="font-mono text-xs bg-muted px-1.5 py-0.5 rounded">
                        {t}
                      </span>
                    ))}
                  </div>
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

          {/* diagnóstico de company_id */}
          {usedCompanyId && (
            <p className="text-xs text-muted-foreground font-mono">
              company_id usado: <span className="font-bold">{usedCompanyId}</span>
              {companyId && usedCompanyId !== companyId && (
                <span className="text-red-600 ml-2">⚠ diverge do contexto ({companyId})</span>
              )}
            </p>
          )}

          {/* resultado */}
          {result && (() => {
            const total = Object.values(result).reduce((a, b) => a + b, 0);
            const boxCls = total > 0
              ? 'rounded-md border border-green-200 bg-green-50 p-3 text-sm space-y-1'
              : 'rounded-md border border-yellow-200 bg-yellow-50 p-3 text-sm space-y-1';
            const titleCls  = total > 0 ? 'font-semibold text-green-800' : 'font-semibold text-yellow-800';
            const rowCls    = total > 0 ? 'flex justify-between font-mono text-xs text-green-700' : 'flex justify-between font-mono text-xs text-yellow-700';
            return (
              <div className={boxCls}>
                <p className={titleCls}>
                  Linhas removidas: <span className="font-bold">{total}</span>
                </p>
                {Object.entries(result).map(([key, rows]) => (
                  <div key={key} className={rowCls}>
                    <span>{key}</span>
                    <span>
                      {rows}
                      {rows === 0 && rowsTotal && (rowsTotal[key] ?? 0) > 0 && (
                        <span className="ml-2 text-orange-600">
                          ({rowsTotal[key]} total com outro source)
                        </span>
                      )}
                      {rows === 0 && rowsTotal && (rowsTotal[key] ?? 0) === 0 && (
                        <span className="ml-2 text-gray-400">(vazio nesta empresa)</span>
                      )}
                    </span>
                  </div>
                ))}
              </div>
            );
          })()}
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
