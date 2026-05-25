import { useEffect, useRef, useState } from 'react'
import { Building2, FileText, ImageUp, Loader2, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { useAuth } from '@/contexts/AuthContext'

interface EmpresaParametrosInfo {
  tem_logo: boolean
  logo_mime?: string
  logo_nome?: string
  tem_template_antecip: boolean
  template_antecip_nome?: string
}

async function fetchParametros(token: string, companyId?: string): Promise<EmpresaParametrosInfo> {
  const headers: Record<string, string> = { Authorization: `Bearer ${token}` }
  if (companyId) headers['X-Company-ID'] = companyId
  const res = await fetch('/api/config/empresa/parametros', { headers })
  if (!res.ok) throw new Error('Erro ao carregar parâmetros')
  return res.json()
}

export default function EmpresaParametros() {
  const { user, token, company: companyId } = useAuth()
  const isAdmin = user?.role === 'admin'

  const [info, setInfo] = useState<EmpresaParametrosInfo | null>(null)
  const [loading, setLoading] = useState(true)
  const [logoPreview, setLogoPreview] = useState<string | null>(null)
  const [uploadingLogo, setUploadingLogo] = useState(false)
  const [uploadingTemplate, setUploadingTemplate] = useState(false)

  const logoInputRef = useRef<HTMLInputElement>(null)
  const templateInputRef = useRef<HTMLInputElement>(null)

  const load = async () => {
    if (!token) return
    try {
      const data = await fetchParametros(token, companyId ?? undefined)
      setInfo(data)
      if (data.tem_logo) {
        // carrega a logo para preview
        const headers: Record<string, string> = { Authorization: `Bearer ${token}` }
        if (companyId) headers['X-Company-ID'] = companyId
        const res = await fetch('/api/config/empresa/logo', { headers })
        if (res.ok) {
          const blob = await res.blob()
          setLogoPreview(URL.createObjectURL(blob))
        }
      } else {
        setLogoPreview(null)
      }
    } catch {
      toast.error('Erro ao carregar parâmetros')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [token, companyId]) // eslint-disable-line react-hooks/exhaustive-deps

  const uploadLogo = async (file: File) => {
    if (!token) return
    setUploadingLogo(true)
    const form = new FormData()
    form.append('logo', file)
    const headers: Record<string, string> = { Authorization: `Bearer ${token}` }
    if (companyId) headers['X-Company-ID'] = companyId
    try {
      const res = await fetch('/api/config/empresa/logo', { method: 'POST', headers, body: form })
      if (!res.ok) throw new Error((await res.json()).error ?? 'Erro')
      toast.success('Logo salva com sucesso')
      await load()
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Erro ao salvar logo')
    } finally {
      setUploadingLogo(false)
    }
  }

  const uploadTemplate = async (file: File) => {
    if (!token) return
    setUploadingTemplate(true)
    const form = new FormData()
    form.append('template', file)
    const headers: Record<string, string> = { Authorization: `Bearer ${token}` }
    if (companyId) headers['X-Company-ID'] = companyId
    try {
      const res = await fetch('/api/config/empresa/template-antecipacao', { method: 'POST', headers, body: form })
      if (!res.ok) throw new Error((await res.json()).error ?? 'Erro')
      toast.success('Template salvo com sucesso')
      await load()
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Erro ao salvar template')
    } finally {
      setUploadingTemplate(false)
    }
  }

  const downloadTemplate = async () => {
    if (!token) return
    const headers: Record<string, string> = { Authorization: `Bearer ${token}` }
    if (companyId) headers['X-Company-ID'] = companyId
    const res = await fetch('/api/config/empresa/template-antecipacao', { headers })
    if (!res.ok) { toast.error('Template não encontrado'); return }
    const blob = await res.blob()
    const nome = info?.template_antecip_nome ?? 'template-antecipacao.pdf'
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url; a.download = nome; a.click()
    URL.revokeObjectURL(url)
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Loader2 className="w-6 h-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  return (
    <div className="p-6 max-w-2xl mx-auto space-y-6">
      <div className="flex items-center gap-2 mb-2">
        <Building2 className="w-5 h-5 text-muted-foreground" />
        <h1 className="text-xl font-semibold">Parâmetros de Empresa</h1>
      </div>

      {/* ── Logo ─────────────────────────────────────────────────────── */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base flex items-center gap-2">
            <ImageUp className="w-4 h-4" />
            Logo da Empresa
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {logoPreview ? (
            <div className="flex items-center gap-4">
              <img
                src={logoPreview}
                alt="Logo"
                className="h-16 max-w-[200px] object-contain rounded border bg-white p-1"
              />
              <div className="text-sm text-muted-foreground">
                <p className="font-medium">{info?.logo_nome}</p>
                <p>{info?.logo_mime}</p>
              </div>
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">Nenhuma logo cadastrada.</p>
          )}

          {isAdmin && (
            <>
              <input
                ref={logoInputRef}
                type="file"
                accept="image/jpeg,image/png,image/webp,image/svg+xml"
                className="hidden"
                onChange={e => { const f = e.target.files?.[0]; if (f) uploadLogo(f) }}
              />
              <div className="flex gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => logoInputRef.current?.click()}
                  disabled={uploadingLogo}
                >
                  {uploadingLogo ? <Loader2 className="w-4 h-4 mr-1 animate-spin" /> : <ImageUp className="w-4 h-4 mr-1" />}
                  {info?.tem_logo ? 'Substituir logo' : 'Enviar logo'}
                </Button>
                {info?.tem_logo && (
                  <Button size="sm" variant="ghost" className="text-destructive" disabled>
                    <Trash2 className="w-4 h-4 mr-1" /> Remover
                  </Button>
                )}
              </div>
            </>
          )}
        </CardContent>
      </Card>

      {/* ── Template de Antecipação ───────────────────────────────────── */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base flex items-center gap-2">
            <FileText className="w-4 h-4" />
            Modelo de Relatório — Antecipação
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {info?.tem_template_antecip ? (
            <div className="flex items-center gap-3">
              <FileText className="w-8 h-8 text-red-500 shrink-0" />
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium truncate">{info.template_antecip_nome}</p>
                <p className="text-xs text-muted-foreground">Template PDF de antecipação cadastrado</p>
              </div>
              <Button size="sm" variant="outline" onClick={downloadTemplate}>
                Baixar
              </Button>
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">Nenhum template cadastrado.</p>
          )}

          {isAdmin && (
            <>
              <input
                ref={templateInputRef}
                type="file"
                accept="application/pdf"
                className="hidden"
                onChange={e => { const f = e.target.files?.[0]; if (f) uploadTemplate(f) }}
              />
              <Button
                size="sm"
                variant="outline"
                onClick={() => templateInputRef.current?.click()}
                disabled={uploadingTemplate}
              >
                {uploadingTemplate ? <Loader2 className="w-4 h-4 mr-1 animate-spin" /> : <FileText className="w-4 h-4 mr-1" />}
                {info?.tem_template_antecip ? 'Substituir template' : 'Enviar template PDF'}
              </Button>
            </>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
