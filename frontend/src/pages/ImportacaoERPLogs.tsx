import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { History } from 'lucide-react';
import { ERPXMLJobsTable } from './ImportarViaERP';

export default function ImportacaoERPLogs() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight flex items-center gap-2">
          <History className="h-6 w-6" /> Logs de Importação — XML via ERP
        </h1>
        <p className="text-sm text-muted-foreground mt-1">
          Histórico das importações de XML (NF-e entrada + CT-e) enfileiradas para o conector ERP.
        </p>
      </div>
      <Card>
        <CardHeader><CardTitle className="text-base">Jobs de importação</CardTitle></CardHeader>
        <CardContent><ERPXMLJobsTable /></CardContent>
      </Card>
    </div>
  );
}
