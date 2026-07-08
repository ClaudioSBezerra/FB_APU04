// XmlViewerDialog.tsx — visualizador do XML BRUTO da NF-e para conferência de
// divergências (pedido do Claudio, 2026-07-08). Busca o XML em
// /api/fiscal/comparacao/xml, formata (indenta) e destaca as linhas dos itens
// que deram divergência, para facilitar bater o valor contra a fonte.
//
// Notas importadas antes da migration 154 não têm xml_content → o backend
// devolve 404 com mensagem orientando reimportar; aqui mostramos essa mensagem.
import { useEffect, useMemo, useRef, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  Dialog, DialogContent, DialogHeader, DialogTitle,
} from '@/components/ui/dialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Loader2, ExternalLink, Search, AlertTriangle } from 'lucide-react';
import { useAuth } from '@/contexts/AuthContext';

// Item divergente destacado no XML (código do produto + rótulo p/ o chip)
export interface XmlDivergentItem {
  nItem: number;
  cProd: string;
  xProd: string;
}

// Formata XML de uma linha só em várias linhas indentadas. Formatador simples
// (sem dependência externa por causa do CSP dos artifacts/produção): quebra
// entre '><' e recalcula a indentação por linha.
function formatXml(xml: string): string {
  const withBreaks = xml.replace(/>\s*</g, '>\n<').trim();
  let pad = 0;
  return withBreaks
    .split('\n')
    .map(raw => {
      const line = raw.trim();
      if (!line) return '';
      let indent = 0;
      if (/^<\?/.test(line)) {
        indent = 0; // declaração <?xml ...?>
      } else if (/^<\/.+>$/.test(line)) {
        pad = Math.max(pad - 1, 0); // fechamento </tag>
      } else if (/^<[^!?][^>]*[^/]>.*<\/.+>$/.test(line)) {
        indent = 0; // <tag>valor</tag> na mesma linha
      } else if (/^<[^!?][^>]*[^/]>$/.test(line)) {
        indent = 1; // abertura <tag>
      }
      const result = '  '.repeat(pad) + line;
      pad += indent;
      return result;
    })
    .join('\n');
}

export function XmlViewerDialog({
  nfeId,
  chave,
  numero,
  divergentItems = [],
  open,
  onClose,
}: {
  nfeId: string;
  chave: string;
  numero: string;
  divergentItems?: XmlDivergentItem[];
  open: boolean;
  onClose: () => void;
}) {
  const { token, companyId } = useAuth();
  const [q, setQ] = useState('');
  const preRef = useRef<HTMLPreElement>(null);

  const { data, isLoading, isError, error } = useQuery<string>({
    queryKey: ['fiscal-xml', nfeId],
    queryFn: async () => {
      const res = await fetch(`/api/fiscal/comparacao/xml?nfe_id=${encodeURIComponent(nfeId)}`);
      if (!res.ok) {
        let msg = `Erro ${res.status}`;
        try {
          const j = await res.json();
          if (j?.error) msg = j.error;
        } catch { /* corpo não-JSON */ }
        throw new Error(msg);
      }
      return res.text();
    },
    enabled: open && !!nfeId,
    staleTime: 5 * 60 * 1000,
  });

  const formatted = useMemo(() => (data ? formatXml(data) : ''), [data]);

  // Códigos dos produtos divergentes (para destacar as linhas <cProd>)
  const codigos = useMemo(
    () => divergentItems.map(d => d.cProd).filter(Boolean),
    [divergentItems],
  );

  const linhas = useMemo(() => formatted.split('\n'), [formatted]);
  const busca = q.trim().toLowerCase();

  const linhaDestacada = (linha: string) => {
    if (busca && linha.toLowerCase().includes(busca)) return 'busca';
    if (codigos.some(c => linha.includes(c))) return 'divergente';
    return '';
  };

  // Rola até a primeira linha do produto clicado
  const scrollToCProd = (cProd: string) => {
    const el = preRef.current?.querySelector(`[data-cprod="${CSS.escape(cProd)}"]`);
    el?.scrollIntoView({ behavior: 'smooth', block: 'center' });
  };

  useEffect(() => {
    if (!open) setQ('');
  }, [open]);

  const abrirNovaAba = () => {
    const params = new URLSearchParams();
    params.set('nfe_id', nfeId);
    if (token) params.set('token', token);
    if (companyId) params.set('company_id', companyId);
    window.open(`/api/fiscal/comparacao/xml?${params}`, '_blank');
  };

  return (
    <Dialog open={open} onOpenChange={o => !o && onClose()}>
      <DialogContent className="max-w-4xl max-h-[90vh] flex flex-col">
        <DialogHeader>
          <DialogTitle className="text-sm flex items-center gap-2 flex-wrap">
            XML da NF-e {numero}
            <span className="font-mono text-[10px] text-muted-foreground">{chave}</span>
          </DialogTitle>
        </DialogHeader>

        {/* Chips dos itens divergentes — clique rola até o produto no XML */}
        {divergentItems.length > 0 && (
          <div className="flex items-center gap-1.5 flex-wrap">
            <span className="text-[11px] text-muted-foreground">Itens divergentes:</span>
            {divergentItems.map(d => (
              <button key={`${d.nItem}-${d.cProd}`} onClick={() => scrollToCProd(d.cProd)}>
                <Badge
                  variant="outline"
                  className="text-[10px] px-1.5 py-0 bg-red-50 text-red-700 border-red-200 hover:bg-red-100 cursor-pointer"
                  title={d.xProd}
                >
                  #{d.nItem} · {d.cProd}
                </Badge>
              </button>
            ))}
          </div>
        )}

        {/* Busca + abrir em nova aba */}
        <div className="flex items-center gap-2">
          <div className="relative flex-1">
            <Search className="absolute left-2 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
            <Input
              value={q}
              onChange={e => setQ(e.target.value)}
              placeholder="localizar no XML (ex.: vBC, cProd, vICMS)"
              className="h-8 pl-7 text-xs font-mono"
            />
          </div>
          <Button size="sm" variant="outline" className="h-8" onClick={abrirNovaAba} disabled={!data}>
            <ExternalLink className="h-3.5 w-3.5 mr-1.5" /> Abrir em nova aba
          </Button>
        </div>

        {/* Corpo */}
        <div className="flex-1 min-h-0 overflow-auto rounded-md border bg-muted/20">
          {isLoading ? (
            <div className="flex items-center justify-center py-12 text-sm text-muted-foreground">
              <Loader2 className="h-4 w-4 mr-2 animate-spin" /> Carregando XML...
            </div>
          ) : isError ? (
            <div className="flex items-start gap-2 p-4 text-sm text-amber-700">
              <AlertTriangle className="h-4 w-4 mt-0.5 shrink-0" />
              <span>{(error as Error)?.message || 'Não foi possível carregar o XML.'}</span>
            </div>
          ) : (
            <pre ref={preRef} className="text-[11px] leading-relaxed font-mono p-3 whitespace-pre">
              {linhas.map((linha, i) => {
                const tipo = linhaDestacada(linha);
                const cProd = codigos.find(c => linha.includes(c));
                return (
                  <div
                    key={i}
                    data-cprod={linha.includes('<cProd>') && cProd ? cProd : undefined}
                    className={
                      tipo === 'divergente'
                        ? 'bg-red-100/70 -mx-3 px-3'
                        : tipo === 'busca'
                          ? 'bg-yellow-200/70 -mx-3 px-3'
                          : ''
                    }
                  >
                    {linha || ' '}
                  </div>
                );
              })}
            </pre>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
