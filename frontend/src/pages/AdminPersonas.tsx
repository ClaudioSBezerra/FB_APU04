import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Checkbox } from "@/components/ui/checkbox";
import { toast } from "sonner";
import { Save, Info } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";

interface Persona {
  id: string;
  label: string;
  modules: string[];
}

// Módulos controláveis por persona — mesmos ids de mainItems no AppRail.
// Config fica de fora: é aberto a todos (abas sensíveis já são adminOnly).
const CONTROLLED_MODULES: { id: string; label: string }[] = [
  { id: 'simulador',    label: 'Simulador RT - SPED' },
  { id: 'notas',        label: 'Notas Importadas' },
  { id: 'painel',       label: 'Painel XMLs' },
  { id: 'reforma',      label: 'Análise Reforma Trib.' },
  { id: 'fronteira',    label: 'ICMS Fronteira' },
  { id: 'auditoria',    label: 'Auditoria Fiscal' },
  { id: 'pacotefiscal', label: 'Teste Pacote Fiscal' },
  { id: 'efdcontrib',   label: 'EFD Contribuições' },
];

export default function AdminPersonas() {
  const { token } = useAuth();
  const queryClient = useQueryClient();

  // Edições pendentes por persona (id → lista de módulos); vazio = sem mudança
  const [pending, setPending] = useState<Record<string, string[]>>({});

  const { data: personas, isLoading } = useQuery<Persona[]>({
    queryKey: ['admin-personas'],
    queryFn: async () => {
      const response = await fetch(`/api/admin/personas`);
      if (!response.ok) throw new Error(`Erro ao carregar personas (${response.status})`);
      return response.json();
    },
    enabled: !!token
  });

  const saveMutation = useMutation({
    mutationFn: async (data: { personaId: string; modules: string[] }) => {
      const response = await fetch(`/api/admin/personas/update?id=${data.personaId}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ modules: data.modules }),
      });
      if (!response.ok) {
        const text = await response.text();
        throw new Error(text || 'Falha ao salvar persona');
      }
      return response.json();
    },
    onSuccess: (_data, vars) => {
      queryClient.invalidateQueries({ queryKey: ['admin-personas'] });
      setPending(prev => {
        const next = { ...prev };
        delete next[vars.personaId];
        return next;
      });
      toast.success("Persona atualizada. Vale para os usuários em até 30 min (próximo refresh do token).");
    },
    onError: (error: Error) => toast.error(error.message || "Erro ao salvar persona"),
  });

  if (isLoading) return <div>Carregando personas...</div>;

  const effectiveModules = (p: Persona) => pending[p.id] ?? p.modules ?? [];

  const toggle = (p: Persona, moduleId: string, checked: boolean) => {
    const current = effectiveModules(p);
    const next = checked
      ? [...current, moduleId]
      : current.filter(m => m !== moduleId);
    setPending(prev => ({ ...prev, [p.id]: next }));
  };

  const isDirty = (p: Persona) => {
    const edited = pending[p.id];
    if (!edited) return false;
    const orig = [...(p.modules ?? [])].sort().join(',');
    return [...edited].sort().join(',') !== orig;
  };

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-bold tracking-tight">Personas × Módulos</h2>
        <p className="text-sm text-muted-foreground mt-1">
          Defina quais módulos cada persona libera. Um usuário com várias personas acessa a união dos módulos delas.
        </p>
      </div>

      <div className="rounded-md border overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="min-w-[160px]">Persona</TableHead>
              {CONTROLLED_MODULES.map(m => (
                <TableHead key={m.id} className="text-center text-xs whitespace-nowrap">{m.label}</TableHead>
              ))}
              <TableHead className="text-right">Ações</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {personas?.map(p => {
              const mods = effectiveModules(p);
              return (
                <TableRow key={p.id}>
                  <TableCell className="font-medium">
                    {p.label}
                    {mods.length === 0 && (
                      <Badge variant="outline" className="ml-2 bg-red-50 text-red-700 border-red-200 text-[10px]">
                        nenhum módulo
                      </Badge>
                    )}
                  </TableCell>
                  {CONTROLLED_MODULES.map(m => (
                    <TableCell key={m.id} className="text-center">
                      <Checkbox
                        checked={mods.includes(m.id)}
                        onCheckedChange={(checked) => toggle(p, m.id, checked as boolean)}
                        aria-label={`${p.label} → ${m.label}`}
                      />
                    </TableCell>
                  ))}
                  <TableCell className="text-right">
                    <Button
                      size="sm"
                      variant={isDirty(p) ? "default" : "outline"}
                      disabled={!isDirty(p) || saveMutation.isPending}
                      onClick={() => saveMutation.mutate({ personaId: p.id, modules: mods })}
                    >
                      <Save className="mr-1.5 h-3.5 w-3.5" />
                      {saveMutation.isPending ? "Salvando..." : "Salvar"}
                    </Button>
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </div>

      <div className="flex items-start gap-2 text-xs text-muted-foreground border rounded-md p-3 bg-muted/30">
        <Info className="h-4 w-4 shrink-0 mt-0.5" />
        <div className="space-y-1">
          <p><strong>Como funciona:</strong> os módulos são as áreas fixas do sistema (ícones do menu lateral) — não são cadastráveis, novos módulos surgem com novas versões do sistema. O que se configura aqui é quais deles cada persona libera.</p>
          <p><strong>Configurações</strong> não aparece na lista porque é acessível a todos os usuários; as abas sensíveis dentro dele (Usuários, Limpar Dados, etc.) continuam restritas a admin.</p>
          <p><strong>Teste Pacote Fiscal</strong>: a Comparação Fiscal é liberada pela persona; a aba de Importar XML continua restrita a admin.</p>
          <p>Alterações valem para os usuários logados em até 30 minutos (próxima renovação automática do token), sem precisar de novo login.</p>
        </div>
      </div>
    </div>
  );
}
