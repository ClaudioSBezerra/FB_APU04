import { useState, useEffect } from 'react';
import { AlertTriangle } from 'lucide-react';
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
} from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

const REQUIRED_TOKEN = 'DELETE-FB_APU04';

const AFFECTED_TABLES = [
  'import_jobs (e dependentes via CASCADE)',
  'filial_apelidos',
  'nfe_entradas',
  'nfe_saidas',
  'cte_entradas',
  'parceiros',
  'erp_bridge_run_items',
  'erp_bridge_runs',
];

export interface ResetDatabaseDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Chamado SOMENTE quando o usuário digita o token correto e clica Confirmar.
   *  Recebe o body que deve ser enviado ao backend. */
  onConfirm: (body: { confirmation: string }) => Promise<void> | void;
  /** Indica que a requisição está em voo — desabilita CTA e mostra spinner */
  loading?: boolean;
}

export function ResetDatabaseDialog({
  open,
  onOpenChange,
  onConfirm,
  loading = false,
}: ResetDatabaseDialogProps) {
  const [typed, setTyped] = useState('');

  // Reset input quando abre/fecha (evita estado vazado entre aberturas)
  useEffect(() => {
    if (!open) setTyped('');
  }, [open]);

  const tokenOk = typed === REQUIRED_TOKEN;

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent className="max-w-lg">
        <AlertDialogHeader>
          <AlertDialogTitle className="flex items-center gap-2 text-destructive">
            <AlertTriangle className="h-5 w-5" />
            Zerar TODA a base de dados
          </AlertDialogTitle>
          <AlertDialogDescription asChild>
            <div className="space-y-3 text-sm">
              <p className="font-semibold text-destructive">
                Esta ação é IRREVERSÍVEL e destruirá os dados de TODAS as empresas.
              </p>
              <p>Tabelas que serão truncadas:</p>
              <ul className="list-disc pl-5 space-y-0.5 font-mono text-xs">
                {AFFECTED_TABLES.map((t) => (
                  <li key={t}>{t}</li>
                ))}
              </ul>
              <p className="text-muted-foreground">
                Um backup automático <code>/backups/reset-&lt;timestamp&gt;.sql</code> será
                gerado antes do TRUNCATE; se o backup falhar, a operação é abortada.
              </p>
              <p className="pt-2">
                Para confirmar, digite exatamente{' '}
                <code className="rounded bg-muted px-1 py-0.5 font-mono text-destructive">
                  {REQUIRED_TOKEN}
                </code>{' '}
                abaixo:
              </p>
              <Input
                aria-label="Token de confirmação"
                value={typed}
                onChange={(e) => setTyped(e.target.value)}
                placeholder={REQUIRED_TOKEN}
                autoComplete="off"
                spellCheck={false}
                disabled={loading}
              />
            </div>
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <Button
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={loading}
          >
            Cancelar
          </Button>
          <Button
            variant="destructive"
            disabled={!tokenOk || loading}
            onClick={() => onConfirm({ confirmation: REQUIRED_TOKEN })}
          >
            {loading ? 'Executando...' : 'Confirmar destruição'}
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

export default ResetDatabaseDialog;
