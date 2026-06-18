import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { useAuth } from '@/contexts/AuthContext'
import { toast } from 'sonner'
import { FileText, FileCheck2, Loader2, ShieldCheck } from 'lucide-react'

// Auditoria Fiscal EFD ICMS/IPI × Guias (DARE). Faz upload do SPED (.txt) +
// guias (.pdf), o backend concilia o Bloco E com as guias e devolve o
// relatório executivo (HTML), aberto em nova aba (imprimível como PDF).
export default function AuditoriaEFD() {
  const { token } = useAuth()
  const [sped, setSped] = useState<File | null>(null)
  const [guias, setGuias] = useState<File[]>([])
  const [loading, setLoading] = useState(false)

  async function handleAuditar() {
    if (!sped) {
      toast.error('Selecione o arquivo SPED (.txt) da EFD ICMS/IPI.')
      return
    }
    if (guias.length === 0) {
      toast.error('Selecione ao menos uma guia (DARE em PDF).')
      return
    }
    setLoading(true)
    try {
      const fd = new FormData()
      fd.append('sped', sped)
      guias.forEach((g) => fd.append('guias', g))
      const res = await fetch('/api/auditoria-efd', {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` },
        body: fd,
      })
      if (!res.ok) {
        const msg = await res.text().catch(() => '')
        throw new Error(msg || `Erro ${res.status}`)
      }
      const html = await res.blob()
      const url = URL.createObjectURL(html)
      window.open(url, '_blank')
      toast.success('Auditoria gerada — confira a nova aba.')
    } catch (e) {
      toast.error('Falha na auditoria: ' + (e instanceof Error ? e.message : 'erro'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="max-w-3xl mx-auto p-4 space-y-4">
      <div className="flex items-center gap-2">
        <ShieldCheck className="h-6 w-6 text-blue-800" />
        <div>
          <h1 className="text-xl font-bold text-blue-900">Auditoria Fiscal — EFD ICMS/IPI × Guias</h1>
          <p className="text-sm text-muted-foreground">Concilia a apuração do Bloco E (SPED) com as guias de recolhimento (DARE) e gera um relatório executivo.</p>
        </div>
      </div>

      <Card>
        <CardHeader><CardTitle className="text-base flex items-center gap-2"><FileText className="h-4 w-4" /> 1. Arquivo SPED (EFD ICMS/IPI)</CardTitle></CardHeader>
        <CardContent className="space-y-2">
          <input
            type="file"
            accept=".txt"
            className="block w-full text-sm"
            onChange={(e) => setSped(e.target.files?.[0] ?? null)}
          />
          {sped && <p className="text-xs text-green-700">✔ {sped.name} ({(sped.size / 1048576).toFixed(1)} MB)</p>}
        </CardContent>
      </Card>

      <Card>
        <CardHeader><CardTitle className="text-base flex items-center gap-2"><FileCheck2 className="h-4 w-4" /> 2. Guias de recolhimento (DARE em PDF)</CardTitle></CardHeader>
        <CardContent className="space-y-2">
          <input
            type="file"
            accept=".pdf"
            multiple
            className="block w-full text-sm"
            onChange={(e) => setGuias(Array.from(e.target.files ?? []))}
          />
          {guias.length > 0 && (
            <ul className="text-xs text-green-700 list-disc pl-5">
              {guias.map((g, i) => <li key={i}>{g.name}</li>)}
            </ul>
          )}
        </CardContent>
      </Card>

      <Button onClick={handleAuditar} disabled={loading} className="bg-blue-800 hover:bg-blue-900">
        {loading ? <><Loader2 className="h-4 w-4 mr-2 animate-spin" />Processando…</> : <><ShieldCheck className="h-4 w-4 mr-2" />Gerar Auditoria</>}
      </Button>

      <p className="text-xs text-muted-foreground">
        O relatório abre em nova aba. Para salvar em PDF, use <b>Imprimir → Salvar como PDF</b> (layout A4, 1 página).
      </p>
    </div>
  )
}
