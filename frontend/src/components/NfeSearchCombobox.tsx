// NfeSearchCombobox — combobox de busca server-side debounced de NF-e de
// saída por número ou chave de acesso (Fase 12, TPF-06/07).
//
// Composto de: shell Command/Popover (frontend/src/components/FilialSelector.tsx)
// + convenção de debounce/useQuery já usada no projeto. Desvio crítico de
// FilialSelector: shouldFilter={false} porque a filtragem agora é server-side
// (o próprio backend já retorna só os candidatos relevantes via ILIKE).
//
// Fonte autoritativa: .planning/phases/12-tela-compara-o-fiscal-navega-o/12-RESEARCH.md
// Pattern 1 (linhas 164-241) — copiado verbatim, adaptado apenas ao lint do projeto.
import { useState, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Command, CommandInput, CommandList, CommandEmpty, CommandGroup, CommandItem } from '@/components/ui/command';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Button } from '@/components/ui/button';
import { Search } from 'lucide-react';
import { useAuth } from '@/contexts/AuthContext';

export interface NfeSearchResult {
  id: string;
  chave_nfe: string;
  numero_nfe: string;
  serie: string;
  dest_nome: string;
  data_emissao: string;
}

export function NfeSearchCombobox({ onSelect }: { onSelect: (nfe: NfeSearchResult) => void }) {
  const { token, companyId } = useAuth();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [debounced, setDebounced] = useState('');

  // Debounce: 300ms é a convenção-padrão adotada nesta fase (sem precedente
  // exato no codebase — primeiro combobox server-driven do projeto).
  useMemo(() => {
    const t = setTimeout(() => setDebounced(query), 300);
    return () => clearTimeout(t);
  }, [query]);

  const { data, isFetching } = useQuery<NfeSearchResult[]>({
    queryKey: ['nfe-saidas-search', debounced, companyId],
    queryFn: async () => {
      const res = await fetch(`/api/fiscal/comparacao/search?q=${encodeURIComponent(debounced)}`, {
        headers: { Authorization: `Bearer ${token}`, 'X-Company-ID': companyId || '' },
      });
      if (!res.ok) throw new Error(res.statusText);
      return res.json();
    },
    enabled: debounced.length >= 3, // evita disparar com 1-2 caracteres
  });

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button variant="outline" role="combobox" aria-expanded={open} className="w-80 justify-start">
          <Search className="h-3.5 w-3.5 mr-2 opacity-50" />
          Buscar NF-e por número ou chave...
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-80 p-0">
        <Command shouldFilter={false}>
          <CommandInput placeholder="Número ou chave de acesso..." value={query} onValueChange={setQuery} />
          <CommandList>
            <CommandEmpty className="text-xs py-3 text-center">
              {isFetching ? 'Buscando...' : debounced.length < 3 ? 'Digite ao menos 3 caracteres.' : 'Nenhuma nota encontrada.'}
            </CommandEmpty>
            <CommandGroup>
              {(data ?? []).map(nfe => (
                <CommandItem key={nfe.id} value={nfe.id} onSelect={() => { onSelect(nfe); setOpen(false); }}>
                  <span className="text-xs">
                    Nº {nfe.numero_nfe}/{nfe.serie} — {nfe.dest_nome} — {nfe.data_emissao}
                  </span>
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}
