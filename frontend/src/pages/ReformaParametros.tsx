import { useEffect, useState } from 'react'
import { Info, Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { useAuth } from '@/contexts/AuthContext'
import {
  useReformaParametros,
  useUpdateReformaParametros,
} from '@/hooks/useReformaParametros'

export default function ReformaParametros() {
  const { user } = useAuth()
  const isAdmin = user?.role === 'admin'

  const { data, isLoading } = useReformaParametros()
  const mutation = useUpdateReformaParametros()

  const [targetAno, setTargetAno] = useState<number>(2027)
  const [fatorSimples, setFatorSimples] = useState<number>(20)
  const [taxaCdi, setTaxaCdi] = useState<number>(10.5)
  const [prazoMedio, setPrazoMedio] = useState<number>(30)

  useEffect(() => {
    if (data?.parametros) {
      const p = data.parametros
      setTargetAno(p.target_ano)
      setFatorSimples(p.fator_simples_pct)
      setTaxaCdi(p.taxa_cdi_anual_pct)
      setPrazoMedio(p.prazo_medio_dias)
    }
  }, [data?.parametros])

  function handleSalvar() {
    mutation.mutate(
      {
        target_ano: targetAno,
        fator_simples_pct: fatorSimples,
        taxa_cdi_anual_pct: taxaCdi,
        prazo_medio_dias: prazoMedio,
      },
      {
        onSuccess: () => toast.success('Parâmetros salvos com sucesso.'),
        onError: (e: Error) => toast.error(`Erro ao salvar: ${e.message}`),
      }
    )
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-40">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  return (
    <Card className="max-w-lg">
      <CardHeader>
        <CardTitle>Parâmetros da Reforma Tributária</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="space-y-1">
          <label className="text-sm font-medium">Ano-alvo</label>
          <Input
            type="number"
            value={targetAno}
            onChange={e => setTargetAno(Number(e.target.value))}
            disabled={!isAdmin}
          />
        </div>

        {data?.parametros && (
          <div className="rounded-md border bg-muted/40 px-3 py-2 text-sm space-y-1">
            <div className="flex justify-between">
              <span className="text-muted-foreground">Alíquota IBS (tabela)</span>
              <span className="font-medium">{data.parametros.aliq_ibs_pct.toFixed(2)}%</span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">Alíquota CBS (tabela)</span>
              <span className="font-medium">{data.parametros.aliq_cbs_pct.toFixed(2)}%</span>
            </div>
          </div>
        )}

        <div className="space-y-1">
          <label className="text-sm font-medium">
            <TooltipProvider delayDuration={200}>
              <Tooltip>
                <TooltipTrigger asChild>
                  <span className="inline-flex items-center gap-1 cursor-help">
                    Fator Simples Nacional (%)
                    <Info className="h-3.5 w-3.5 text-muted-foreground" />
                  </span>
                </TooltipTrigger>
                <TooltipContent>
                  Valor estimado. Alíquota definitiva ainda não publicada pelo CG-IBS.
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </label>
          <Input
            type="number"
            step="0.01"
            value={fatorSimples}
            onChange={e => setFatorSimples(Number(e.target.value))}
            disabled={!isAdmin}
          />
        </div>

        <div className="space-y-1">
          <label className="text-sm font-medium">Taxa CDI Anual (%)</label>
          <Input
            type="number"
            step="0.01"
            value={taxaCdi}
            onChange={e => setTaxaCdi(Number(e.target.value))}
            disabled={!isAdmin}
          />
        </div>

        <div className="space-y-1">
          <label className="text-sm font-medium">Prazo Médio (dias)</label>
          <Input
            type="number"
            value={prazoMedio}
            onChange={e => setPrazoMedio(Number(e.target.value))}
            disabled={!isAdmin}
          />
        </div>

        {isAdmin && (
          <Button
            onClick={handleSalvar}
            disabled={mutation.isPending}
            className="w-full"
          >
            {mutation.isPending ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                Salvando...
              </>
            ) : (
              'Salvar'
            )}
          </Button>
        )}
      </CardContent>
    </Card>
  )
}
