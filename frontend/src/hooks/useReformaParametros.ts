import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'

export interface ReformaParametros {
  company_id: string
  target_ano: number
  aliq_ibs_pct: number
  aliq_cbs_pct: number
  fator_simples_pct: number
  taxa_cdi_anual_pct: number
  prazo_medio_dias: number
}

export function useReformaParametros() {
  return useQuery<{ parametros: ReformaParametros | null }>({
    queryKey: ['reforma-parametros'],
    queryFn: async () => {
      const res = await fetch('/api/reforma/parametros')
      if (!res.ok) throw new Error(await res.text())
      return res.json()
    },
  })
}

export function useUpdateReformaParametros() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: Partial<ReformaParametros>) => {
      const res = await fetch('/api/reforma/parametros', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
      })
      if (!res.ok) throw new Error(await res.text())
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['reforma-parametros'] })
    },
  })
}
