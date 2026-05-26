// Aba "Administrativo" do Módulo ICMS Fronteira.
//
// Centraliza para o usuário (admin ou não) as configurações que afetam a
// apuração de fronteira da empresa em foco:
//
//   • Filiais  — read-only, lista de CNPJs com UF/município/IE vindos do
//                reg 0000 do SPED a cada importação.
//   • UFs      — hub híbrido (legislação importada + benefícios manuais),
//                lê/escreve em /api/uf-hub e /api/uf-beneficios.
//   • Empresa  — formulário de edição da empresa em foco (regime, CNPJ,
//                CNAE, segmento, logo). Sem criar/excluir.
//
// As abas Filiais e UFs migram de /config/ambiente para cá. Lá só ficou a
// gestão hierárquica (Ambiente > Grupo > Empresa) para admin.

import { useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import { useAuth } from "@/contexts/AuthContext";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Check, FileText, Factory, MapPin, Building, ImageUp, Tag, Save, Pencil, Trash2, Plus, X } from "lucide-react";

// ---------------------------------------------------------------------------
// Tipos compartilhados (mesmos shapes que o backend devolve).
// ---------------------------------------------------------------------------

export interface Branch {
  cnpj: string;
  company_name: string;
  uf: string;
  inscricao_estadual: string;
  cod_municipio: string;
  municipio_nome: string;
  uf_nome: string;
}

export interface UFBeneficios {
  aliquota_interna: number | null;
  fecp_percentual: number | null;
  reducao_bc_percentual: number | null;
  mva_ajustada_padrao: number | null;
  inaplicabilidade_st: boolean;
  antecipacao_aplicavel: boolean;
  observacoes: string;
  configurado: boolean;
}

export interface UFHubItem {
  uf: string;
  uf_nome: string;
  num_filiais: number;
  legislacao: Record<string, number>;
  beneficios: UFBeneficios;
}

interface CompanyEditable {
  id: string;
  cnpj?: string;
  name?: string;
  trade_name?: string;
  regime_tributario?: string;
  cnae_principal?: string;
  segmento_economico?: string;
}

interface UserHierarchy {
  company: CompanyEditable;
  branches: Branch[];
}

// ---------------------------------------------------------------------------
// FiliaisTab — tabela read-only das filiais (reg 0000 + JOIN IBGE).
// ---------------------------------------------------------------------------
export function FiliaisTab({ branches }: { branches: Branch[] }) {
  return (
    <div className="border rounded-md p-4 bg-white">
      {branches.length === 0 ? (
        <p className="text-muted-foreground">Nenhuma filial identificada nas importações.</p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>CNPJ</TableHead>
              <TableHead>Razão Social (Importada)</TableHead>
              <TableHead className="w-16">UF</TableHead>
              <TableHead>Município</TableHead>
              <TableHead>Inscrição Estadual</TableHead>
              <TableHead className="w-24">COD IBGE</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {branches.map((branch, idx) => (
              <TableRow key={idx}>
                <TableCell className="font-mono">{branch.cnpj}</TableCell>
                <TableCell>{branch.company_name}</TableCell>
                <TableCell>
                  {branch.uf
                    ? <span className="inline-flex items-center rounded bg-slate-100 px-2 py-0.5 text-xs font-semibold text-slate-700" title={branch.uf_nome || branch.uf}>{branch.uf}</span>
                    : <span className="text-xs text-muted-foreground">—</span>}
                </TableCell>
                <TableCell className="text-sm">{branch.municipio_nome || <span className="text-muted-foreground">—</span>}</TableCell>
                <TableCell className="font-mono text-sm">{branch.inscricao_estadual || <span className="text-muted-foreground">—</span>}</TableCell>
                <TableCell className="font-mono text-xs text-muted-foreground">{branch.cod_municipio || "—"}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
      <p className="text-[11px] text-muted-foreground mt-3">
        Filiais e dados (UF, IE, município) são extraídos do registro 0000 do SPED a cada importação.
        Imports anteriores podem ter campos vazios — reimporte o SPED para preencher.
      </p>
    </div>
  );
}

// ---------------------------------------------------------------------------
// UFsHubTab — hub híbrido por UF (legislação IA + benefícios manuais).
// ---------------------------------------------------------------------------
export function UFsHubTab() {
  const { token } = useAuth();
  const [items, setItems] = useState<UFHubItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedUF, setSelectedUF] = useState<string>("");
  const [edits, setEdits] = useState<Record<string, UFBeneficios>>({});
  const [saving, setSaving] = useState(false);

  const load = async () => {
    setLoading(true);
    try {
      const res = await fetch("/api/uf-hub", { headers: { Authorization: `Bearer ${token}` } });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      const ufs: UFHubItem[] = data.ufs || [];
      setItems(ufs);
      if (ufs.length > 0 && !selectedUF) setSelectedUF(ufs[0].uf);
      const e: Record<string, UFBeneficios> = {};
      ufs.forEach(u => { e[u.uf] = { ...u.beneficios }; });
      setEdits(e);
    } catch (err) {
      toast.error("Erro ao carregar hub de UFs");
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { if (token) load(); /* eslint-disable-next-line react-hooks/exhaustive-deps */ }, [token]);

  const current = items.find(i => i.uf === selectedUF);
  const form = edits[selectedUF];

  const setField = <K extends keyof UFBeneficios>(field: K, value: UFBeneficios[K]) => {
    setEdits(prev => ({ ...prev, [selectedUF]: { ...prev[selectedUF], [field]: value } }));
  };

  const numOrNull = (s: string): number | null => {
    if (s.trim() === "") return null;
    const n = parseFloat(s.replace(",", "."));
    return isNaN(n) ? null : n;
  };
  const showNum = (v: number | null) => (v === null || v === undefined ? "" : String(v));

  const handleSave = async () => {
    if (!form || !selectedUF) return;
    setSaving(true);
    try {
      const res = await fetch("/api/uf-beneficios", {
        method: "PUT",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify({ uf: selectedUF, ...form }),
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      toast.success(`Benefícios da UF ${selectedUF} salvos`);
      await load();
    } catch (err) {
      toast.error("Erro ao salvar");
      console.error(err);
    } finally {
      setSaving(false);
    }
  };

  if (loading) return <p className="text-muted-foreground">Carregando UFs...</p>;
  if (items.length === 0) {
    return (
      <div className="border rounded-md p-6 bg-white">
        <p className="text-muted-foreground">
          Nenhuma UF identificada nas importações. Importe um SPED para que a UF da filial
          (registro 0000) apareça aqui.
        </p>
      </div>
    );
  }

  return (
    <div className="grid grid-cols-1 md:grid-cols-[260px_1fr] gap-4">
      <div className="border rounded-md bg-white p-2 space-y-1">
        <p className="text-xs font-medium text-muted-foreground px-2 py-1">
          UFs com filiais ({items.length})
        </p>
        {items.map(u => {
          const isActive = u.uf === selectedUF;
          const legTotal = Object.values(u.legislacao).reduce((a, b) => a + b, 0);
          return (
            <button
              key={u.uf}
              onClick={() => setSelectedUF(u.uf)}
              className={`w-full text-left rounded px-2 py-2 text-sm flex items-center gap-2 ${
                isActive ? "bg-slate-900 text-white" : "hover:bg-slate-100"
              }`}
            >
              <span className={`inline-flex items-center justify-center rounded font-bold text-xs w-8 h-6 ${
                isActive ? "bg-white text-slate-900" : "bg-slate-200 text-slate-700"
              }`}>{u.uf}</span>
              <span className="flex-1 truncate">{u.uf_nome || u.uf}</span>
              <span className="flex flex-col items-end text-[10px] leading-tight">
                <span className={isActive ? "text-slate-300" : "text-muted-foreground"}>
                  {u.num_filiais} filial{u.num_filiais === 1 ? "" : "is"}
                </span>
                {legTotal > 0 && (
                  <span className={isActive ? "text-blue-200" : "text-blue-600"}>
                    {legTotal} legisl.
                  </span>
                )}
                {u.beneficios.configurado && (
                  <span className={isActive ? "text-emerald-300" : "text-emerald-600"}>
                    <Check className="inline h-3 w-3" /> conf.
                  </span>
                )}
              </span>
            </button>
          );
        })}
      </div>

      {current && form && (
        <div className="space-y-4">
          <div className="border rounded-md bg-white p-4">
            <div className="flex items-center gap-3 mb-3">
              <div className="inline-flex items-center justify-center rounded bg-slate-900 text-white font-bold text-sm w-12 h-9">
                {current.uf}
              </div>
              <div>
                <h3 className="text-lg font-semibold">{current.uf_nome || current.uf}</h3>
                <p className="text-xs text-muted-foreground">
                  {current.num_filiais} filial{current.num_filiais === 1 ? "" : "is"} importada{current.num_filiais === 1 ? "" : "s"} nesta UF
                </p>
              </div>
            </div>
            <div className="border-t pt-3">
              <p className="text-xs font-medium text-muted-foreground flex items-center gap-1 mb-2">
                <FileText className="h-3 w-3" /> Legislação interpretada (IA)
              </p>
              {Object.keys(current.legislacao).length === 0 ? (
                <p className="text-xs text-muted-foreground">
                  Nenhuma legislação importada para esta UF. Use o módulo de Fronteira → Legislação para subir decretos/RICMS.
                </p>
              ) : (
                <div className="flex flex-wrap gap-2">
                  {Object.entries(current.legislacao).map(([status, n]) => (
                    <span key={status} className="inline-flex items-center rounded-full bg-slate-100 px-2 py-0.5 text-xs">
                      <span className="font-semibold text-slate-700 mr-1">{n}</span>
                      <span className="text-slate-600">{status}</span>
                    </span>
                  ))}
                </div>
              )}
            </div>
          </div>

          <div className="border rounded-md bg-white p-4">
            <p className="text-sm font-semibold mb-1">Benefícios fiscais (manual)</p>
            <p className="text-xs text-muted-foreground mb-4">
              Parâmetros aplicados pelo motor de fronteira nesta UF. Use a IA para extrair de decretos
              ou preencha manualmente aqui.
            </p>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="space-y-1">
                <Label className="text-xs">Alíquota interna (%)</Label>
                <Input
                  type="number" step="0.01" placeholder="Ex: 18.00"
                  value={showNum(form.aliquota_interna)}
                  onChange={e => setField("aliquota_interna", numOrNull(e.target.value))}
                />
              </div>
              <div className="space-y-1">
                <Label className="text-xs">FECP (%)</Label>
                <Input
                  type="number" step="0.01" placeholder="Ex: 2.00"
                  value={showNum(form.fecp_percentual)}
                  onChange={e => setField("fecp_percentual", numOrNull(e.target.value))}
                />
              </div>
              <div className="space-y-1">
                <Label className="text-xs">Redução de BC (%)</Label>
                <Input
                  type="number" step="0.01" placeholder="Opcional"
                  value={showNum(form.reducao_bc_percentual)}
                  onChange={e => setField("reducao_bc_percentual", numOrNull(e.target.value))}
                />
              </div>
              <div className="space-y-1">
                <Label className="text-xs">MVA ajustada padrão (%)</Label>
                <Input
                  type="number" step="0.01" placeholder="Quando não há regra por NCM"
                  value={showNum(form.mva_ajustada_padrao)}
                  onChange={e => setField("mva_ajustada_padrao", numOrNull(e.target.value))}
                />
              </div>
            </div>

            <div className="flex flex-wrap gap-6 mt-4">
              <label className="flex items-center gap-2 text-sm cursor-pointer">
                <Checkbox
                  checked={form.inaplicabilidade_st}
                  onCheckedChange={v => setField("inaplicabilidade_st", !!v)}
                />
                Inaplicabilidade de ST nesta UF
              </label>
              <label className="flex items-center gap-2 text-sm cursor-pointer">
                <Checkbox
                  checked={form.antecipacao_aplicavel}
                  onCheckedChange={v => setField("antecipacao_aplicavel", !!v)}
                />
                Antecipação aplicável nesta UF
              </label>
            </div>

            <div className="space-y-1 mt-4">
              <Label className="text-xs">Observações</Label>
              <Textarea
                rows={3}
                placeholder="Anotações livres (ex: decreto base, data de vigência)..."
                value={form.observacoes}
                onChange={e => setField("observacoes", e.target.value)}
              />
            </div>

            <div className="flex justify-end mt-4">
              <Button onClick={handleSave} disabled={saving}>
                {saving ? "Salvando..." : "Salvar benefícios"}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// EmpresaEditTab — edição dos dados da empresa em foco.
//
// Edita os mesmos campos que o admin já editava em /config/ambiente:
//   regime_tributario, CNPJ, CNAE principal, segmento econômico, logo.
//
// O backend valida ownership (apenas users com acesso à empresa via owner_id
// ou user_environments podem alterar — admin pode tudo).
// ---------------------------------------------------------------------------
export function EmpresaEditTab({ company }: { company: CompanyEditable }) {
  const { token } = useAuth();
  const [regime, setRegime] = useState(company.regime_tributario || "lucro_real");
  const [cnpj, setCnpj] = useState(company.cnpj || "");
  const [cnae, setCnae] = useState(company.cnae_principal || "");
  const [segmento, setSegmento] = useState(company.segmento_economico || "");
  const [saving, setSaving] = useState(false);
  // Logo
  const logoInputRef = useRef<HTMLInputElement>(null);
  const [logoPreview, setLogoPreview] = useState<string | null>(null);
  const [uploadingLogo, setUploadingLogo] = useState(false);

  useEffect(() => {
    // checa se a empresa tem logo salva
    if (!company.id) return;
    fetch(`/api/config/empresa/logo?company_id=${encodeURIComponent(company.id)}`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then(r => r.ok ? r.blob() : null)
      .then(b => {
        if (b && b.size > 0) setLogoPreview(URL.createObjectURL(b));
      })
      .catch(() => { /* sem logo, sem problema */ });
  }, [company.id, token]);

  const handleSave = async () => {
    setSaving(true);
    try {
      const res = await fetch(`/api/config/companies?id=${encodeURIComponent(company.id)}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify({
          regime_tributario: regime,
          cnpj: cnpj.replace(/\D/g, ""),
          cnae_principal: cnae,
          segmento_economico: segmento,
        }),
      });
      if (!res.ok) {
        if (res.status === 403) {
          toast.error("Você não tem permissão para editar esta empresa");
        } else {
          throw new Error(`HTTP ${res.status}`);
        }
        return;
      }
      toast.success("Dados da empresa atualizados");
    } catch (err) {
      toast.error("Erro ao salvar");
      console.error(err);
    } finally {
      setSaving(false);
    }
  };

  const handleLogoUpload = async (file: File) => {
    if (!file || !company.id) return;
    setUploadingLogo(true);
    try {
      const fd = new FormData();
      fd.append("file", file);
      const res = await fetch(`/api/config/empresa/logo?company_id=${encodeURIComponent(company.id)}`, {
        method: "POST",
        headers: { Authorization: `Bearer ${token}` },
        body: fd,
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      // recarrega preview
      const blob = await fetch(`/api/config/empresa/logo?company_id=${encodeURIComponent(company.id)}&t=${Date.now()}`, {
        headers: { Authorization: `Bearer ${token}` },
      }).then(r => r.blob());
      setLogoPreview(URL.createObjectURL(blob));
      toast.success("Logo enviada");
    } catch (err) {
      toast.error("Erro ao enviar logo");
      console.error(err);
    } finally {
      setUploadingLogo(false);
    }
  };

  return (
    <div className="space-y-4">
      <div className="border rounded-md bg-white p-4">
        <p className="text-sm font-semibold mb-1">Dados da empresa</p>
        <p className="text-xs text-muted-foreground mb-4">
          Identidade e classificação fiscal da empresa em foco. Campos de filial (IE, município)
          são extraídos automaticamente do SPED — veja na aba Filiais.
        </p>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div className="space-y-1 md:col-span-2">
            <Label className="text-xs">Razão Social</Label>
            <Input value={company.name || ""} disabled className="bg-slate-50" />
            <p className="text-[10px] text-muted-foreground">
              Identidade da empresa — alterações de nome requerem o administrador do ambiente.
            </p>
          </div>
          <div className="space-y-1">
            <Label className="text-xs">CNPJ (sede)</Label>
            <Input
              value={cnpj}
              onChange={e => setCnpj(e.target.value.replace(/\D/g, ""))}
              maxLength={14}
              placeholder="14 dígitos"
              inputMode="numeric"
            />
          </div>
          <div className="space-y-1">
            <Label className="text-xs">Regime Tributário</Label>
            <Select value={regime} onValueChange={setRegime}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="lucro_real">Lucro Real</SelectItem>
                <SelectItem value="lucro_presumido">Lucro Presumido</SelectItem>
                <SelectItem value="simples_nacional">Simples Nacional</SelectItem>
                <SelectItem value="nao_informado">Não informado</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1">
            <Label className="text-xs">CNAE Principal</Label>
            <Input
              value={cnae}
              onChange={e => setCnae(e.target.value)}
              maxLength={7}
              placeholder="Ex: 4711301"
            />
          </div>
          <div className="space-y-1">
            <Label className="text-xs">Segmento Econômico</Label>
            <Input
              value={segmento}
              onChange={e => setSegmento(e.target.value)}
              maxLength={100}
              placeholder="Ex: Varejo de móveis"
            />
          </div>
        </div>

        {/* Logo */}
        <div className="mt-6 pt-4 border-t">
          <p className="text-xs font-medium text-muted-foreground flex items-center gap-1 mb-2">
            <ImageUp className="h-3 w-3" /> Logo da empresa
          </p>
          <div className="flex items-center gap-4">
            {logoPreview
              ? <img src={logoPreview} alt="Logo" className="h-16 max-w-[160px] object-contain rounded border bg-white p-1" />
              : <div className="h-16 w-32 rounded border bg-slate-50 flex items-center justify-center text-xs text-muted-foreground">sem logo</div>}
            <div>
              <input
                ref={logoInputRef}
                type="file"
                accept="image/jpeg,image/png,image/webp"
                className="hidden"
                onChange={e => { const f = e.target.files?.[0]; if (f) handleLogoUpload(f); }}
              />
              <Button
                size="sm"
                variant="outline"
                disabled={uploadingLogo}
                onClick={() => logoInputRef.current?.click()}
              >
                {logoPreview ? "Substituir logo" : "Enviar logo"}
              </Button>
              <p className="text-[10px] text-muted-foreground mt-1">JPEG, PNG ou WebP.</p>
            </div>
          </div>
        </div>

        <div className="flex justify-end mt-6">
          <Button onClick={handleSave} disabled={saving || !cnpj || cnpj.length !== 14}>
            {saving ? "Salvando..." : "Salvar dados da empresa"}
          </Button>
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// SegmentosTab — gerencia os segmentos de ST cadastrados para a empresa/UF.
// Lista todos os segmentos disponíveis para a UF selecionada e permite marcar
// quais a empresa opera — o motor de cálculo usa essa lista para decidir se
// um CFOP de ST é realmente ST ou antecipação.
// ---------------------------------------------------------------------------
interface SegmentoUF {
  codigo: number;
  uf: string;
  descricao: string;
  ativo: boolean;
}

export function SegmentosTab({ uf }: { uf: string }) {
  const { token, user } = useAuth();
  const isAdmin = user?.role === 'admin';
  const [segmentos, setSegmentos] = useState<SegmentoUF[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [ativos, setAtivos] = useState<Set<number>>(new Set());
  // CRUD state
  const [editingCodigo, setEditingCodigo] = useState<number | null>(null);
  const [editDesc, setEditDesc] = useState('');
  const [showAdd, setShowAdd] = useState(false);
  const [newCodigo, setNewCodigo] = useState('');
  const [newDesc, setNewDesc] = useState('');

  const load = async () => {
    if (!uf) return;
    setLoading(true);
    try {
      const res = await fetch(`/api/icms-fronteira/segmentos?uf=${uf}`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      const list: SegmentoUF[] = data.segmentos || [];
      setSegmentos(list);
      setAtivos(new Set(list.filter(s => s.ativo).map(s => s.codigo)));
    } catch {
      toast.error("Erro ao carregar segmentos");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, [uf, token]); // eslint-disable-line react-hooks/exhaustive-deps

  const handleEdit = async (seg: SegmentoUF) => {
    if (editingCodigo === seg.codigo) {
      try {
        const res = await fetch('/api/icms-fronteira/segmentos/item', {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
          body: JSON.stringify({ codigo: seg.codigo, uf, descricao: editDesc }),
        });
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        toast.success('Segmento atualizado');
        setEditingCodigo(null);
        load();
      } catch { toast.error('Erro ao atualizar segmento'); }
    } else {
      setEditingCodigo(seg.codigo);
      setEditDesc(seg.descricao);
    }
  };

  const handleDelete = async (seg: SegmentoUF) => {
    if (!confirm(`Excluir segmento ${seg.codigo} — ${seg.descricao}?`)) return;
    try {
      const res = await fetch(`/api/icms-fronteira/segmentos/item?codigo=${seg.codigo}&uf=${uf}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${token}` },
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      toast.success('Segmento excluído');
      load();
    } catch { toast.error('Erro ao excluir segmento'); }
  };

  const handleAdd = async () => {
    const codigo = parseInt(newCodigo);
    if (!codigo || !newDesc.trim()) { toast.error('Código e descrição obrigatórios'); return; }
    try {
      const res = await fetch('/api/icms-fronteira/segmentos', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({ codigo, uf, descricao: newDesc.trim() }),
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      toast.success('Segmento criado');
      setShowAdd(false); setNewCodigo(''); setNewDesc('');
      load();
    } catch { toast.error('Erro ao criar segmento'); }
  };

  const toggle = (codigo: number) => {
    setAtivos(prev => {
      const next = new Set(prev);
      if (next.has(codigo)) next.delete(codigo); else next.add(codigo);
      return next;
    });
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      const res = await fetch("/api/icms-fronteira/company-segmentos", {
        method: "PUT",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify({ uf, codigos: Array.from(ativos) }),
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      toast.success(`Segmentos de ${uf} atualizados`);
      load();
    } catch {
      toast.error("Erro ao salvar segmentos");
    } finally {
      setSaving(false);
    }
  };

  if (!uf) {
    return (
      <div className="border rounded-md p-6 bg-white">
        <p className="text-muted-foreground text-sm">Selecione uma UF no seletor acima para gerenciar os segmentos.</p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="border rounded-md bg-white p-4">
        <div className="flex items-center justify-between mb-3">
          <div>
            <p className="text-sm font-semibold">Segmentos sujeitos à ST — {uf}</p>
            <p className="text-xs text-muted-foreground mt-0.5">
              Marque os segmentos em que a empresa opera. O cálculo só aplica ST para NCMs cujo segmento esteja marcado; os demais são antecipação.
            </p>
          </div>
          <div className="flex gap-2">
            {isAdmin && (
              <Button size="sm" variant="outline" onClick={() => setShowAdd(v => !v)}>
                <Plus className="h-3.5 w-3.5 mr-1" />
                Novo
              </Button>
            )}
            <Button size="sm" onClick={handleSave} disabled={saving || loading}>
              <Save className="h-3.5 w-3.5 mr-1" />
              {saving ? "Salvando..." : "Salvar seleção"}
            </Button>
          </div>
        </div>

        {isAdmin && showAdd && (
          <div className="flex gap-2 mb-3 p-3 border rounded bg-slate-50 items-end">
            <div className="space-y-1 w-24">
              <Label className="text-xs">Código</Label>
              <Input value={newCodigo} onChange={e => setNewCodigo(e.target.value)} placeholder="Ex: 22" type="number" min={1} className="h-8" />
            </div>
            <div className="space-y-1 flex-1">
              <Label className="text-xs">Descrição</Label>
              <Input value={newDesc} onChange={e => setNewDesc(e.target.value)} placeholder="Descrição do segmento" className="h-8" />
            </div>
            <Button size="sm" onClick={handleAdd} className="h-8">Criar</Button>
            <Button size="sm" variant="ghost" onClick={() => setShowAdd(false)} className="h-8"><X className="h-4 w-4" /></Button>
          </div>
        )}

        {loading ? (
          <p className="text-sm text-muted-foreground">Carregando...</p>
        ) : segmentos.length === 0 ? (
          <p className="text-sm text-muted-foreground">Nenhum segmento cadastrado para {uf}.</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-10">Ativo</TableHead>
                <TableHead className="w-16">Cód.</TableHead>
                <TableHead>Descrição do Segmento</TableHead>
                {isAdmin && <TableHead className="w-20 text-right">Ações</TableHead>}
              </TableRow>
            </TableHeader>
            <TableBody>
              {segmentos.map(seg => (
                <TableRow key={seg.codigo} className={ativos.has(seg.codigo) ? "bg-blue-50" : ""}>
                  <TableCell>
                    <Checkbox checked={ativos.has(seg.codigo)} onCheckedChange={() => toggle(seg.codigo)} />
                  </TableCell>
                  <TableCell className="font-mono text-sm font-semibold">{String(seg.codigo).padStart(2, '0')}</TableCell>
                  <TableCell className="text-sm">
                    {editingCodigo === seg.codigo ? (
                      <Input value={editDesc} onChange={e => setEditDesc(e.target.value)} className="h-7 text-sm" autoFocus />
                    ) : seg.descricao}
                  </TableCell>
                  {isAdmin && (
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-1">
                        <Button size="icon" variant="ghost" className="h-7 w-7" title={editingCodigo === seg.codigo ? "Salvar" : "Editar"} onClick={() => handleEdit(seg)}>
                          {editingCodigo === seg.codigo ? <Check className="h-3.5 w-3.5 text-green-600" /> : <Pencil className="h-3.5 w-3.5" />}
                        </Button>
                        {editingCodigo === seg.codigo ? (
                          <Button size="icon" variant="ghost" className="h-7 w-7" onClick={() => setEditingCodigo(null)}>
                            <X className="h-3.5 w-3.5" />
                          </Button>
                        ) : (
                          <Button size="icon" variant="ghost" className="h-7 w-7 text-red-500 hover:text-red-700" onClick={() => handleDelete(seg)}>
                            <Trash2 className="h-3.5 w-3.5" />
                          </Button>
                        )}
                      </div>
                    </TableCell>
                  )}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}

        <p className="text-[11px] text-muted-foreground mt-3">
          <strong>{ativos.size}</strong> de {segmentos.length} segmentos ativos para {uf}.
        </p>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// AdministrativoTab — entrada única, usada como TabsContent="administrativo"
// dentro do /icms-fronteira. Carrega /api/user/hierarchy e organiza 4 sub-abas.
// ---------------------------------------------------------------------------
export function AdministrativoTab({ uf }: { uf: string }) {
  const { token } = useAuth();

  const { data, isLoading, isError } = useQuery<UserHierarchy>({
    queryKey: ["user/hierarchy"],
    queryFn: async () => {
      const res = await fetch("/api/user/hierarchy", {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      return res.json();
    },
    enabled: !!token,
  });

  if (isLoading) return <p className="text-muted-foreground">Carregando dados administrativos...</p>;
  if (isError || !data) {
    return (
      <div className="border rounded-md p-6 bg-white">
        <p className="text-muted-foreground">Erro ao carregar dados administrativos.</p>
      </div>
    );
  }

  const branchesCount = data.branches?.length ?? 0;

  return (
    <Tabs defaultValue="filiais" className="space-y-4">
      <TabsList>
        <TabsTrigger value="filiais" className="flex items-center gap-2">
          <Factory className="h-4 w-4" />
          Filiais
          <span className="ml-1 text-xs text-muted-foreground">({branchesCount})</span>
        </TabsTrigger>
        <TabsTrigger value="ufs" className="flex items-center gap-2">
          <MapPin className="h-4 w-4" />
          UFs
        </TabsTrigger>
        <TabsTrigger value="segmentos" className="flex items-center gap-2">
          <Tag className="h-4 w-4" />
          Segmentos ST
        </TabsTrigger>
        <TabsTrigger value="empresa" className="flex items-center gap-2">
          <Building className="h-4 w-4" />
          Empresa
        </TabsTrigger>
      </TabsList>

      <TabsContent value="filiais">
        <FiliaisTab branches={data.branches ?? []} />
      </TabsContent>

      <TabsContent value="ufs">
        <UFsHubTab />
      </TabsContent>

      <TabsContent value="segmentos">
        <SegmentosTab uf={uf} />
      </TabsContent>

      <TabsContent value="empresa">
        <EmpresaEditTab company={data.company} />
      </TabsContent>
    </Tabs>
  );
}
