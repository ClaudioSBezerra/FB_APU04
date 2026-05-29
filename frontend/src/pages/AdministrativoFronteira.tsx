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
import { RegrasTab } from "./IcmsFronteira";
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
import { Check, FileText, Factory, MapPin, Building, Building2, ImageUp, Tag, Save, Pencil, Trash2, Plus, X, Upload, Download, AlertTriangle, ShieldCheck, List, Search } from "lucide-react";

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
  base_por_dentro: boolean;
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
export function FiliaisTab({ branches, uf }: { branches: Branch[]; uf?: string }) {
  const filtered = uf ? branches.filter(b => b.uf === uf) : branches;
  return (
    <div className="border rounded-md p-4 bg-white">
      {filtered.length === 0 ? (
        <p className="text-muted-foreground">
          {uf ? `Nenhuma filial da empresa nesta UF (${uf}).` : 'Nenhuma filial identificada nas importações.'}
        </p>
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
            {filtered.map((branch, idx) => (
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
        {uf ? `Exibindo ${filtered.length} de ${branches.length} filial(is) filtradas por UF ${uf}. ` : ''}
        Filiais e dados (UF, IE, município) são extraídos do registro 0000 do SPED a cada importação.
        Imports anteriores podem ter campos vazios — reimporte o SPED para preencher.
      </p>
    </div>
  );
}

// ---------------------------------------------------------------------------
// UFsHubTab — hub híbrido por UF (legislação IA + benefícios manuais).
// ---------------------------------------------------------------------------
export function UFsHubTab({ uf: ufProp }: { uf?: string }) {
  const { token } = useAuth();
  const [items, setItems] = useState<UFHubItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedUF, setSelectedUF] = useState<string>(ufProp ?? "");
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
      // Usa ufProp se disponível e presente na lista; senão seleciona a primeira.
      setSelectedUF(prev => {
        if (prev && ufs.some(u => u.uf === prev)) return prev;
        return ufs[0]?.uf ?? "";
      });
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
              <label className="flex items-center gap-2 text-sm cursor-pointer" title="PE: base = (operação − ICMS destacado) ÷ (1 − alíq. interna). BA/CE: deixar desmarcado.">
                <Checkbox
                  checked={form.base_por_dentro}
                  onCheckedChange={v => setField("base_por_dentro", !!v)}
                />
                Base "por dentro" (antecipação/DIFAL — PE)
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
//   regime_tributario, CNPJ, CNAE principal, logo.
//
// O backend valida ownership (apenas users com acesso à empresa via owner_id
// ou user_environments podem alterar — admin pode tudo).
// ---------------------------------------------------------------------------
export function EmpresaEditTab({ company }: { company: CompanyEditable }) {
  const { token } = useAuth();
  const [regime, setRegime] = useState(company.regime_tributario || "lucro_real");
  const [cnpj, setCnpj] = useState(company.cnpj || "");
  const [cnae, setCnae] = useState(company.cnae_principal || "");
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

interface CompanyOption { id: string; name: string; cnpj: string; }

export function SegmentosTab({ uf }: { uf: string }) {
  const { token, user } = useAuth();
  const isAdmin = user?.role === 'admin';

  // Admin: seletor de empresa alvo
  const [companies, setCompanies] = useState<CompanyOption[]>([]);
  const [targetCompanyId, setTargetCompanyId] = useState<string>('');

  useEffect(() => {
    if (!isAdmin) return;
    fetch('/api/config/companies', { headers: { Authorization: `Bearer ${token}` } })
      .then(r => r.json())
      .then((data: CompanyOption[]) => setCompanies(data ?? []))
      .catch(() => toast.error('Não foi possível carregar empresas'));
  }, [isAdmin, token]);

  const companyHeader = (extra: Record<string, string> = {}): Record<string, string> => {
    const h: Record<string, string> = { Authorization: `Bearer ${token}`, ...extra };
    if (isAdmin && targetCompanyId) h['X-Company-ID'] = targetCompanyId;
    return h;
  };

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
  // CSV import state — arquivo enviado ao backend (parsing server-side robusto).
  const [showImport, setShowImport] = useState(false);
  const [importFile, setImportFile] = useState<File | null>(null);
  const [importing, setImporting] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const load = async () => {
    if (!uf) return;
    // Sem early-return para admin sem empresa: o backend retorna o catálogo global
    // com ativo=false para todos (empresa padrão do admin está vazia).
    setLoading(true);
    try {
      const res = await fetch(`/api/icms-fronteira/segmentos?uf=${uf}`, {
        headers: companyHeader(),
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

  useEffect(() => { load(); }, [uf, token, targetCompanyId]); // eslint-disable-line react-hooks/exhaustive-deps

  const handleEdit = async (seg: SegmentoUF) => {
    if (editingCodigo === seg.codigo) {
      try {
        const res = await fetch('/api/icms-fronteira/segmentos/item', {
          method: 'PUT',
          headers: companyHeader({ 'Content-Type': 'application/json' }),
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
        headers: companyHeader(),
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
        headers: companyHeader({ 'Content-Type': 'application/json' }),
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
    if (isAdmin && !targetCompanyId) { toast.error('Selecione uma empresa primeiro'); return; }
    setSaving(true);
    try {
      const res = await fetch("/api/icms-fronteira/company-segmentos", {
        method: "PUT",
        headers: companyHeader({ "Content-Type": "application/json" }),
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

  // ---------------------------------------------------------------------------
  // CSV import — envia o arquivo ao backend, que faz o parsing (robusto a
  // vírgula/ponto-e-vírgula/TAB, BOM e cabeçalho) e o upsert em massa.
  // ---------------------------------------------------------------------------
  const handleImport = async () => {
    if (!importFile) { toast.error('Selecione um arquivo'); return; }
    if (!uf) { toast.error('UF não definida'); return; }
    setImporting(true);
    try {
      const fd = new FormData();
      fd.append('file', importFile);
      fd.append('uf', uf);
      const res = await fetch('/api/icms-fronteira/segmentos/importar', {
        method: 'POST',
        headers: companyHeader(), // sem Content-Type: o browser define o boundary
        body: fd,
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
      const imported = data.imported ?? 0;
      const skipped = data.skipped ?? 0;
      if (imported > 0) {
        toast.success(`${imported} segmento(s) importado(s) para ${uf}${skipped ? ` (${skipped} ignorado(s))` : ''}`);
        setShowImport(false); setImportFile(null);
      } else {
        toast.warning(`Nenhum segmento importado (${skipped} ignorado(s)). Verifique o arquivo.`);
      }
      load();
    } catch (e) {
      toast.error('Erro ao importar: ' + (e instanceof Error ? e.message : ''));
    } finally {
      setImporting(false);
    }
  };

  const downloadTemplate = () => {
    const content = `codigo,descricao\n1,Alimentos e bebidas não alcoólicas\n2,Produtos de higiene e limpeza\n`;
    const a = document.createElement('a');
    a.href = URL.createObjectURL(new Blob([content], { type: 'text/csv;charset=utf-8' }));
    a.download = `segmentos_${uf}_template.csv`;
    a.click();
  };

  if (!uf) {
    return (
      <div className="border rounded-md p-6 bg-white">
        <p className="text-muted-foreground text-sm">Selecione uma UF no seletor acima para gerenciar os segmentos.</p>
      </div>
    );
  }

  const targetCompanyLabel = companies.find(c => c.id === targetCompanyId)?.name ?? '';

  return (
    <div className="space-y-4">
      {/* Seletor de empresa (admin only) */}
      {isAdmin && (
        <div className="border rounded-md bg-white p-4 space-y-1">
          <Label className="text-xs font-semibold">Empresa alvo</Label>
          <Select value={targetCompanyId} onValueChange={setTargetCompanyId}>
            <SelectTrigger className="w-full">
              <SelectValue placeholder="Selecione a empresa cujos segmentos deseja gerenciar..." />
            </SelectTrigger>
            <SelectContent>
              {companies.map(c => (
                <SelectItem key={c.id} value={c.id}>
                  {c.name}{c.cnpj ? ` — ${c.cnpj}` : ''}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      )}

      <div className="border rounded-md bg-white p-4">
        <div className="flex items-center justify-between mb-3">
          <div>
            <p className="text-sm font-semibold">
              Segmentos sujeitos à ST — {uf}
              {isAdmin && targetCompanyLabel && <span className="ml-2 text-muted-foreground font-normal">({targetCompanyLabel})</span>}
            </p>
            <p className="text-xs text-muted-foreground mt-0.5">
              Marque os segmentos em que a empresa opera. O cálculo só aplica ST para NCMs cujo segmento esteja marcado; os demais são antecipação.
            </p>
          </div>
          <div className="flex gap-2 flex-wrap">
            <Button size="sm" variant="outline" onClick={() => { setShowAdd(v => !v); setShowImport(false); }}>
              <Plus className="h-3.5 w-3.5 mr-1" />
              Novo
            </Button>
            <Button size="sm" variant="outline" onClick={() => { setShowImport(v => !v); setShowAdd(false); }}>
              <Upload className="h-3.5 w-3.5 mr-1" />
              Importar CSV
            </Button>
            <Button size="sm" onClick={handleSave} disabled={saving || loading || (isAdmin && !targetCompanyId)}>
              <Save className="h-3.5 w-3.5 mr-1" />
              {saving ? "Salvando..." : "Salvar seleção"}
            </Button>
          </div>
        </div>

        {/* Observação: vínculo dos segmentos às filiais da UF */}
        <div className="mb-3 flex items-start gap-2 rounded-md border border-amber-200 bg-amber-50 p-3 text-xs text-amber-800">
          <AlertTriangle className="h-4 w-4 shrink-0 mt-0.5" />
          <p>
            <strong>Importante:</strong> os segmentos marcados e salvos aqui ficam vinculados às
            filiais de <strong>{uf}</strong>. O motor só calcula ST para um NCM quando o segmento
            da regra estiver marcado para esta UF; sem o vínculo, a operação é tratada como
            antecipação. Marque os segmentos e clique em <strong>“Salvar seleção”</strong> para cada UF.
          </p>
        </div>

        {/* Formulário de novo segmento */}
        {showAdd && (
          <div className="flex gap-2 mb-3 p-3 border rounded bg-slate-50 items-end">
            <div className="space-y-1 w-24">
              <Label className="text-xs">Código</Label>
              <Input value={newCodigo} onChange={e => setNewCodigo(e.target.value)} placeholder="Ex: 22" type="number" min={1} className="h-8" />
            </div>
            <div className="space-y-1 flex-1">
              <Label className="text-xs">Descrição</Label>
              <Input value={newDesc} onChange={e => setNewDesc(e.target.value)} placeholder="Descrição do segmento" className="h-8" onKeyDown={e => e.key === 'Enter' && handleAdd()} />
            </div>
            <Button size="sm" onClick={handleAdd} className="h-8">Criar</Button>
            <Button size="sm" variant="ghost" onClick={() => setShowAdd(false)} className="h-8"><X className="h-4 w-4" /></Button>
          </div>
        )}

        {/* Painel de importação CSV */}
        {showImport && (
          <div className="mb-3 p-4 border rounded bg-slate-50 space-y-3">
            <div className="flex items-center justify-between">
              <p className="text-xs font-semibold text-slate-700">Importar segmentos via CSV</p>
              <Button size="sm" variant="ghost" className="h-6 px-1" onClick={() => { setShowImport(false); setImportFile(null); }}>
                <X className="h-4 w-4" />
              </Button>
            </div>
            <p className="text-xs text-muted-foreground">
              O arquivo (CSV ou XLSX) deve ter duas colunas: <code className="bg-muted px-1 rounded">codigo</code> e <code className="bg-muted px-1 rounded">descricao</code>.
              Separador: vírgula, ponto-e-vírgula ou tabulação. Cabeçalho é ignorado automaticamente.
            </p>
            <div className="flex gap-2 items-center flex-wrap">
              <input
                ref={fileInputRef}
                type="file"
                accept=".csv,.txt,.xlsx"
                className="hidden"
                onChange={e => { setImportFile(e.target.files?.[0] ?? null); e.target.value = ''; }}
              />
              <Button size="sm" variant="outline" onClick={() => fileInputRef.current?.click()}>
                <Upload className="h-3.5 w-3.5 mr-1" /> Selecionar arquivo
              </Button>
              <Button size="sm" variant="ghost" onClick={downloadTemplate}>
                <Download className="h-3.5 w-3.5 mr-1" /> Baixar template
              </Button>
            </div>

            {importFile && (
              <div className="space-y-2">
                <p className="text-xs text-muted-foreground">
                  Arquivo selecionado: <span className="font-medium text-slate-700">{importFile.name}</span>
                </p>
                <Button
                  size="sm"
                  onClick={handleImport}
                  disabled={importing}
                  className="w-full"
                >
                  {importing ? 'Importando...' : `Importar segmentos de ${importFile.name} para ${uf}`}
                </Button>
              </div>
            )}
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
                <TableHead className="w-20 text-right">Ações</TableHead>
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
                      <Input value={editDesc} onChange={e => setEditDesc(e.target.value)} className="h-7 text-sm" autoFocus onKeyDown={e => e.key === 'Enter' && handleEdit(seg)} />
                    ) : seg.descricao}
                  </TableCell>
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
// ProdepeTab — cadastro PRODEPE / regime especial de CD por CNPJ da filial.
// Quando há enquadramento ativo na data do documento, o motor de fronteira zera
// a antecipação e a ST (DIFAL fica fora). A lista de NCMs é documental — não
// filtra o cálculo de fronteira (Leitura A do Art. 11-A Dec. 21.959/1999).
// ---------------------------------------------------------------------------
interface ProdepeFilial { cnpj: string; nome: string; }
type ProdepePrograma = "PRODEPE" | "PROIND";
interface ProdepeEnquadramento {
  id: string;
  cnpj: string;
  inscricao_estadual: string;
  programa: ProdepePrograma;
  num_ato: string;
  enquadramento: string;
  credito_presumido_pct: number;
  vigencia_inicio: string;
  vigencia_fim: string;
  dispensa_antecipacao: boolean;
  observacoes: string;
  ativo: boolean;
  ncm_count: number;
}

// Defaults por programa: ao trocar PRODEPE↔PROIND a descrição livre acompanha
// o caso típico (sem sobrescrever se o usuário já editou manualmente).
const programaDefaults: Record<ProdepePrograma, string> = {
  PRODEPE: "Central de Distribuição",
  PROIND: "Indústria",
};

const emptyEnquadramento: Omit<ProdepeEnquadramento, "id" | "ncm_count"> = {
  cnpj: "",
  inscricao_estadual: "",
  programa: "PRODEPE",
  num_ato: "",
  enquadramento: programaDefaults.PRODEPE,
  credito_presumido_pct: 0,
  vigencia_inicio: "",
  vigencia_fim: "",
  dispensa_antecipacao: true,
  observacoes: "",
  ativo: true,
};

export function ProdepeTab() {
  const { token } = useAuth();

  const [list, setList] = useState<ProdepeEnquadramento[]>([]);
  const [filiais, setFiliais] = useState<ProdepeFilial[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState({ ...emptyEnquadramento });
  // editingId === null   → form em modo criação
  // editingId === string → form em modo edição (do enquadramento com esse id)
  const [editingId, setEditingId] = useState<string | null>(null);

  // Import CSV de NCMs (por enquadramento já criado)
  const [importTarget, setImportTarget] = useState<ProdepeEnquadramento | null>(null);
  const [importFile, setImportFile] = useState<File | null>(null);
  const [importing, setImporting] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Listagem dos NCMs já cadastrados (para auditoria do que foi importado)
  const [ncmsTarget, setNcmsTarget] = useState<ProdepeEnquadramento | null>(null);
  const [ncmsList, setNcmsList] = useState<{ ncm: string; descricao: string }[]>([]);
  const [ncmsLoading, setNcmsLoading] = useState(false);
  const [ncmsFilter, setNcmsFilter] = useState("");

  const headers = (extra: Record<string, string> = {}): Record<string, string> => ({
    Authorization: `Bearer ${token}`,
    ...extra,
  });

  const load = async () => {
    setLoading(true);
    try {
      const [rList, rFil] = await Promise.all([
        fetch("/api/icms-fronteira/prodepe", { headers: headers() }),
        fetch("/api/icms-fronteira/prodepe/filiais", { headers: headers() }),
      ]);
      if (!rList.ok) throw new Error(`HTTP ${rList.status}`);
      const dList = await rList.json();
      setList(dList.enquadramentos ?? []);
      if (rFil.ok) {
        const dFil = await rFil.json();
        setFiliais(dFil.filiais ?? []);
      }
    } catch {
      toast.error("Erro ao carregar enquadramentos PRODEPE");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, [token]); // eslint-disable-line react-hooks/exhaustive-deps

  const handleSave = async () => {
    const cnpj = form.cnpj.replace(/\D/g, "");
    if (!cnpj) { toast.error("Selecione o CNPJ da filial beneficiada"); return; }
    if (!form.num_ato.trim()) { toast.error("Informe o número do ato/decreto"); return; }
    setSaving(true);
    try {
      const res = await fetch("/api/icms-fronteira/prodepe", {
        method: "POST",
        headers: headers({ "Content-Type": "application/json" }),
        body: JSON.stringify({ ...form, cnpj }),
      });
      if (!res.ok) {
        const d = await res.json().catch(() => ({}));
        throw new Error(d.error || `HTTP ${res.status}`);
      }
      toast.success(editingId ? "Enquadramento atualizado" : "Enquadramento criado");
      setShowForm(false);
      setEditingId(null);
      setForm({ ...emptyEnquadramento });
      load();
    } catch (e) {
      toast.error("Erro ao salvar: " + (e instanceof Error ? e.message : ""));
    } finally {
      setSaving(false);
    }
  };

  // Abre o form em modo edição com os valores carregados. O upsert do backend
  // (ON CONFLICT company_id+cnpj+num_ato) atualiza a mesma linha quando salvar.
  const handleEdit = (e: ProdepeEnquadramento) => {
    setEditingId(e.id);
    setForm({
      cnpj: e.cnpj,
      inscricao_estadual: e.inscricao_estadual || "",
      programa: e.programa,
      num_ato: e.num_ato || "",
      enquadramento: e.enquadramento || programaDefaults[e.programa],
      credito_presumido_pct: Number(e.credito_presumido_pct || 0),
      vigencia_inicio: e.vigencia_inicio || "",
      vigencia_fim: e.vigencia_fim || "",
      dispensa_antecipacao: e.dispensa_antecipacao,
      observacoes: e.observacoes || "",
      ativo: e.ativo,
    });
    setShowForm(true);
    setImportTarget(null); // fecha painel de import se estiver aberto
  };

  const handleCancelForm = () => {
    setShowForm(false);
    setEditingId(null);
    setForm({ ...emptyEnquadramento });
  };

  const handleDelete = async (e: ProdepeEnquadramento) => {
    if (!confirm(`Excluir enquadramento da filial ${e.cnpj} (ato ${e.num_ato || "—"})?\nOs NCMs associados serão removidos em cascata.`)) return;
    try {
      const res = await fetch(`/api/icms-fronteira/prodepe/item?id=${encodeURIComponent(e.id)}`, {
        method: "DELETE",
        headers: headers(),
      });
      if (!res.ok && res.status !== 204) throw new Error(`HTTP ${res.status}`);
      toast.success("Enquadramento excluído");
      load();
    } catch {
      toast.error("Erro ao excluir enquadramento");
    }
  };

  const loadNcms = async (enq: ProdepeEnquadramento) => {
    setNcmsLoading(true);
    setNcmsFilter("");
    try {
      const res = await fetch(`/api/icms-fronteira/prodepe/ncms?enquadramento_id=${encodeURIComponent(enq.id)}`, {
        headers: headers(),
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      setNcmsList(data.ncms ?? []);
    } catch {
      toast.error("Erro ao carregar NCMs");
      setNcmsList([]);
    } finally {
      setNcmsLoading(false);
    }
  };

  const handleOpenNcms = (enq: ProdepeEnquadramento) => {
    setNcmsTarget(enq);
    setImportTarget(null); // fecha import se estiver aberto
    setShowForm(false);
    loadNcms(enq);
  };

  const handleDeleteNcm = async (ncm: string) => {
    if (!ncmsTarget) return;
    if (!confirm(`Excluir NCM ${ncm} deste enquadramento?`)) return;
    try {
      const res = await fetch(
        `/api/icms-fronteira/prodepe/ncms?enquadramento_id=${encodeURIComponent(ncmsTarget.id)}&ncm=${encodeURIComponent(ncm)}`,
        { method: "DELETE", headers: headers() }
      );
      if (!res.ok && res.status !== 204) throw new Error(`HTTP ${res.status}`);
      toast.success(`NCM ${ncm} removido`);
      loadNcms(ncmsTarget);
      load(); // atualiza contagem na tabela principal
    } catch {
      toast.error("Erro ao excluir NCM");
    }
  };

  const handleClearAllNcms = async () => {
    if (!ncmsTarget) return;
    if (!confirm(`Limpar TODOS os ${ncmsList.length} NCMs deste enquadramento?\nÚtil para re-importar do zero se a importação ficou bagunçada.`)) return;
    try {
      const res = await fetch(
        `/api/icms-fronteira/prodepe/ncms?enquadramento_id=${encodeURIComponent(ncmsTarget.id)}&all=1`,
        { method: "DELETE", headers: headers() }
      );
      if (!res.ok && res.status !== 204) throw new Error(`HTTP ${res.status}`);
      toast.success("Todos os NCMs foram removidos");
      loadNcms(ncmsTarget);
      load();
    } catch {
      toast.error("Erro ao limpar NCMs");
    }
  };

  const handleImportNcms = async () => {
    if (!importTarget || !importFile) return;
    setImporting(true);
    try {
      const fd = new FormData();
      fd.append("file", importFile);
      fd.append("enquadramento_id", importTarget.id);
      const res = await fetch("/api/icms-fronteira/prodepe/ncms/importar", {
        method: "POST",
        headers: headers(), // sem Content-Type — multipart define o boundary
        body: fd,
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
      const imported = data.imported ?? 0;
      const skipped = data.skipped ?? 0;
      const mode = data.mode ?? "csv"; // csv | xlsx | pdf | text
      if (imported > 0) {
        const ctx = (mode === "pdf" || mode === "text")
          ? ` (extraído de ${mode === "pdf" ? "PDF" : "texto"})`
          : (skipped ? ` (${skipped} ignorado(s))` : "");
        toast.success(`${imported} NCM(s) importado(s)${ctx}`);
        setImportTarget(null);
        setImportFile(null);
      } else {
        toast.warning(
          (mode === "pdf" || mode === "text")
            ? `Nenhum NCM extraído do ${mode === "pdf" ? "PDF" : "texto"}. Pode ser PDF digitalizado/imagem (sem texto pesquisável).`
            : `Nenhum NCM importado (${skipped} ignorado(s)). Verifique o arquivo.`
        );
      }
      load();
    } catch (e) {
      toast.error("Erro ao importar: " + (e instanceof Error ? e.message : ""));
    } finally {
      setImporting(false);
    }
  };

  const downloadTemplate = () => {
    const content = `ncm,descricao\n22030000,Cerveja de malte\n22021000,Refrigerante\n`;
    const a = document.createElement("a");
    a.href = URL.createObjectURL(new Blob([content], { type: "text/csv;charset=utf-8" }));
    a.download = "prodepe_ncms_template.csv";
    a.click();
  };

  const fmtCnpj = (c: string) => {
    const d = (c || "").replace(/\D/g, "").padStart(14, "0");
    return `${d.slice(0,2)}.${d.slice(2,5)}.${d.slice(5,8)}/${d.slice(8,12)}-${d.slice(12,14)}`;
  };

  return (
    <div className="space-y-4">
      <div className="border rounded-md bg-white p-4">
        <div className="flex items-start justify-between mb-3 gap-3">
          <div>
            <p className="text-sm font-semibold flex items-center gap-2">
              <ShieldCheck className="h-4 w-4 text-emerald-600" />
              PRODEPE / PROIND — Regime especial por estabelecimento
            </p>
            <p className="text-xs text-muted-foreground mt-1">
              Dispensa de antecipação e ST por <strong>CNPJ da filial recebedora</strong> durante a vigência do ato/decreto.
              <strong> PRODEPE</strong> (central de distribuição, Art. 11-A do Dec. 21.959/1999) e <strong>PROIND</strong> (indústria) zeram a antecipação de fronteira nas aquisições do estabelecimento.
              DIFAL (CFOPs 2551/2556) fica fora da dispensa. Os NCMs são apenas documentais — não filtram o cálculo de fronteira (Leitura A).
              <span className="block mt-1 italic text-amber-700">Exceções (combustíveis/lubrificantes/camarão) ficarão em uma lista negativa por NCM na Fase B.1 — hoje todos os NCMs do estabelecimento beneficiado são dispensados.</span>
            </p>
          </div>
          <Button size="sm" onClick={() => {
            if (showForm) { handleCancelForm(); }
            else { setEditingId(null); setForm({ ...emptyEnquadramento }); setShowForm(true); }
          }}>
            <Plus className="h-3.5 w-3.5 mr-1" />
            Novo enquadramento
          </Button>
        </div>

        {showForm && (
          <div className="mb-4 p-4 border rounded bg-slate-50 space-y-3">
            <div className="flex items-center justify-between -mt-1 -mb-1">
              <p className="text-xs font-semibold text-slate-700">
                {editingId ? "Editar enquadramento" : "Novo enquadramento"}
              </p>
              {editingId && (
                <span className="text-[11px] text-muted-foreground">
                  Atualizando linha existente (mesmo CNPJ + Nº ato).
                </span>
              )}
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1">
                <Label className="text-xs">Programa *</Label>
                <Select
                  value={form.programa}
                  onValueChange={v => setForm(f => {
                    const next = v as ProdepePrograma;
                    // Se o usuário ainda não mexeu na descrição (está no default do programa
                    // anterior), atualiza para o default do novo programa. Caso contrário preserva.
                    const enquadramento = (f.enquadramento === programaDefaults[f.programa])
                      ? programaDefaults[next]
                      : f.enquadramento;
                    return { ...f, programa: next, enquadramento };
                  })}
                >
                  <SelectTrigger className="h-8"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="PRODEPE">PRODEPE — Central de Distribuição</SelectItem>
                    <SelectItem value="PROIND">PROIND — Indústria</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1">
                <Label className="text-xs">Descrição do enquadramento</Label>
                <Input
                  value={form.enquadramento}
                  onChange={e => setForm(f => ({ ...f, enquadramento: e.target.value }))}
                  placeholder={programaDefaults[form.programa]}
                  className="h-8"
                />
              </div>
              <div className="space-y-1">
                <Label className="text-xs">
                  CNPJ da filial beneficiada *
                  {editingId && <span className="ml-1 text-[10px] text-muted-foreground">(não editável — chave do registro)</span>}
                </Label>
                {editingId ? (
                  <Input value={fmtCnpj(form.cnpj)} disabled className="h-8 font-mono text-sm" />
                ) : filiais.length > 0 ? (
                  <Select value={form.cnpj} onValueChange={v => setForm(f => ({ ...f, cnpj: v }))}>
                    <SelectTrigger className="h-8"><SelectValue placeholder="Selecione um CNPJ..." /></SelectTrigger>
                    <SelectContent>
                      {filiais.map(fl => (
                        <SelectItem key={fl.cnpj} value={fl.cnpj}>
                          {fmtCnpj(fl.cnpj)}{fl.nome ? ` — ${fl.nome}` : ""}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                ) : (
                  <Input
                    value={form.cnpj}
                    onChange={e => setForm(f => ({ ...f, cnpj: e.target.value.replace(/\D/g, "") }))}
                    placeholder="14 dígitos"
                    maxLength={14}
                    className="h-8"
                  />
                )}
              </div>
              <div className="space-y-1">
                <Label className="text-xs">Inscrição estadual (CACEPE)</Label>
                <Input
                  value={form.inscricao_estadual}
                  onChange={e => setForm(f => ({ ...f, inscricao_estadual: e.target.value }))}
                  placeholder="Ex: 0232142-44"
                  className="h-8"
                />
              </div>
              <div className="space-y-1">
                <Label className="text-xs">Nº do ato / decreto *</Label>
                <Input
                  value={form.num_ato}
                  onChange={e => setForm(f => ({ ...f, num_ato: e.target.value }))}
                  placeholder="Ex: 57.972/2024"
                  className="h-8"
                />
              </div>
              <div className="space-y-1">
                <Label className="text-xs">Vigência início</Label>
                <Input type="date" value={form.vigencia_inicio} onChange={e => setForm(f => ({ ...f, vigencia_inicio: e.target.value }))} className="h-8" />
              </div>
              <div className="space-y-1">
                <Label className="text-xs">Vigência fim</Label>
                <Input type="date" value={form.vigencia_fim} onChange={e => setForm(f => ({ ...f, vigencia_fim: e.target.value }))} className="h-8" />
              </div>
              <div className="space-y-1">
                <Label className="text-xs">Crédito presumido % (saídas — documental)</Label>
                <Input
                  type="number"
                  step="0.01"
                  min={0}
                  max={100}
                  value={form.credito_presumido_pct}
                  onChange={e => setForm(f => ({ ...f, credito_presumido_pct: parseFloat(e.target.value) || 0 }))}
                  className="h-8"
                />
              </div>
              <div className="space-y-1 flex items-end gap-4">
                <label className="flex items-center gap-2 text-xs">
                  <Checkbox checked={form.dispensa_antecipacao} onCheckedChange={v => setForm(f => ({ ...f, dispensa_antecipacao: !!v }))} />
                  Dispensa antecipação/ST
                </label>
                <label className="flex items-center gap-2 text-xs">
                  <Checkbox checked={form.ativo} onCheckedChange={v => setForm(f => ({ ...f, ativo: !!v }))} />
                  Ativo
                </label>
              </div>
            </div>
            <div className="space-y-1">
              <Label className="text-xs">Observações</Label>
              <Textarea
                value={form.observacoes}
                onChange={e => setForm(f => ({ ...f, observacoes: e.target.value }))}
                placeholder="Ex.: TARE Rolimec — Decreto 57.972/2024, vigência 01/01/2025 a 31/12/2032."
                rows={2}
              />
            </div>
            <div className="flex justify-end gap-2">
              <Button size="sm" variant="ghost" onClick={handleCancelForm}>
                <X className="h-3.5 w-3.5 mr-1" /> Cancelar
              </Button>
              <Button size="sm" onClick={handleSave} disabled={saving}>
                <Save className="h-3.5 w-3.5 mr-1" />
                {saving ? "Salvando..." : (editingId ? "Atualizar enquadramento" : "Criar enquadramento")}
              </Button>
            </div>
          </div>
        )}

        {ncmsTarget && (
          <div className="mb-4 p-4 border rounded bg-slate-50 space-y-3">
            <div className="flex items-center justify-between">
              <p className="text-xs font-semibold text-slate-700">
                NCMs cadastrados — {fmtCnpj(ncmsTarget.cnpj)} ({ncmsTarget.num_ato || "—"})
                <span className="ml-2 text-muted-foreground font-normal">{ncmsList.length} NCM(s)</span>
              </p>
              <div className="flex gap-2">
                {ncmsList.length > 0 && (
                  <Button size="sm" variant="outline" className="h-7 text-red-600 hover:text-red-800" onClick={handleClearAllNcms}>
                    <Trash2 className="h-3.5 w-3.5 mr-1" /> Limpar todos
                  </Button>
                )}
                <Button size="sm" variant="ghost" className="h-7 px-1" onClick={() => { setNcmsTarget(null); setNcmsList([]); }}>
                  <X className="h-4 w-4" />
                </Button>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <div className="relative flex-1 max-w-xs">
                <Search className="absolute left-2 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
                <Input
                  value={ncmsFilter}
                  onChange={e => setNcmsFilter(e.target.value)}
                  placeholder="Filtrar por NCM ou descrição..."
                  className="h-8 pl-7 text-sm"
                />
              </div>
              {ncmsFilter && (
                <span className="text-[11px] text-muted-foreground">
                  {ncmsList.filter(n => n.ncm.includes(ncmsFilter) || n.descricao.toLowerCase().includes(ncmsFilter.toLowerCase())).length} de {ncmsList.length}
                </span>
              )}
            </div>
            {ncmsLoading ? (
              <p className="text-sm text-muted-foreground">Carregando NCMs...</p>
            ) : ncmsList.length === 0 ? (
              <p className="text-sm text-muted-foreground">Nenhum NCM cadastrado neste enquadramento. Use o botão de import (📤) na linha para subir um CSV.</p>
            ) : (
              <div className="max-h-80 overflow-y-auto border rounded bg-white">
                <Table>
                  <TableHeader className="sticky top-0 bg-white">
                    <TableRow>
                      <TableHead className="w-28">NCM</TableHead>
                      <TableHead>Descrição</TableHead>
                      <TableHead className="w-12 text-right">—</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {ncmsList
                      .filter(n => !ncmsFilter
                        || n.ncm.includes(ncmsFilter)
                        || n.descricao.toLowerCase().includes(ncmsFilter.toLowerCase()))
                      .map(n => (
                        <TableRow key={n.ncm}>
                          <TableCell className="font-mono text-xs">{n.ncm}</TableCell>
                          <TableCell className="text-xs">{n.descricao || <span className="text-muted-foreground">—</span>}</TableCell>
                          <TableCell className="text-right">
                            <Button
                              size="icon"
                              variant="ghost"
                              className="h-6 w-6 text-red-500 hover:text-red-700"
                              title={`Excluir NCM ${n.ncm}`}
                              onClick={() => handleDeleteNcm(n.ncm)}
                            >
                              <Trash2 className="h-3 w-3" />
                            </Button>
                          </TableCell>
                        </TableRow>
                      ))}
                  </TableBody>
                </Table>
              </div>
            )}
          </div>
        )}

        {importTarget && (
          <div className="mb-4 p-4 border rounded bg-emerald-50 space-y-3">
            <div className="flex items-center justify-between">
              <p className="text-xs font-semibold text-emerald-900">
                Importar NCMs do decreto — {fmtCnpj(importTarget.cnpj)} ({importTarget.num_ato || "—"})
              </p>
              <Button size="sm" variant="ghost" className="h-6 px-1" onClick={() => { setImportTarget(null); setImportFile(null); }}>
                <X className="h-4 w-4" />
              </Button>
            </div>
            <p className="text-xs text-muted-foreground">
              Aceita <strong>PDF do decreto</strong> (extração automática de NCMs do texto),{" "}
              <strong>CSV/XLSX</strong> (2 colunas: NCM, descrição) ou <strong>TXT</strong> (texto bruto colado).
              No PDF/TXT o servidor varre todo o texto procurando padrões <code className="bg-muted px-1 rounded">9999.99.99</code>{" "}
              ou <code className="bg-muted px-1 rounded">99999999</code> — não é necessário planilha bem-formada.
            </p>
            <div className="flex gap-2 items-center flex-wrap">
              <input
                ref={fileInputRef}
                type="file"
                accept=".csv,.txt,.xlsx,.pdf"
                className="hidden"
                onChange={e => { setImportFile(e.target.files?.[0] ?? null); e.target.value = ""; }}
              />
              <Button size="sm" variant="outline" onClick={() => fileInputRef.current?.click()}>
                <Upload className="h-3.5 w-3.5 mr-1" /> Selecionar arquivo
              </Button>
              <Button size="sm" variant="ghost" onClick={downloadTemplate}>
                <Download className="h-3.5 w-3.5 mr-1" /> Baixar template CSV
              </Button>
              {importFile && (
                <Button size="sm" onClick={handleImportNcms} disabled={importing}>
                  {importing ? "Importando..." : `Importar ${importFile.name}`}
                </Button>
              )}
            </div>
          </div>
        )}

        {loading ? (
          <p className="text-sm text-muted-foreground">Carregando...</p>
        ) : list.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            Nenhum enquadramento cadastrado. Clique em <strong>“Novo enquadramento”</strong> para cadastrar a primeira filial beneficiada.
          </p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-24">Programa</TableHead>
                <TableHead>CNPJ</TableHead>
                <TableHead>IE</TableHead>
                <TableHead>Ato/Decreto</TableHead>
                <TableHead>Enquadramento</TableHead>
                <TableHead>Vigência</TableHead>
                <TableHead className="text-right">Créd. pres. %</TableHead>
                <TableHead className="text-right">NCMs</TableHead>
                <TableHead className="w-10">Ativo</TableHead>
                <TableHead className="w-24 text-right">Ações</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {list.map(e => (
                <TableRow key={e.id} className={e.ativo && e.dispensa_antecipacao ? "bg-emerald-50/40" : ""}>
                  <TableCell>
                    <span className={
                      "inline-block rounded px-2 py-0.5 text-[10px] font-semibold " +
                      (e.programa === "PROIND"
                        ? "bg-indigo-100 text-indigo-800"
                        : "bg-emerald-100 text-emerald-800")
                    }>
                      {e.programa}
                    </span>
                  </TableCell>
                  <TableCell className="font-mono text-xs">{fmtCnpj(e.cnpj)}</TableCell>
                  <TableCell className="font-mono text-xs">{e.inscricao_estadual || <span className="text-muted-foreground">—</span>}</TableCell>
                  <TableCell className="text-sm">{e.num_ato || <span className="text-muted-foreground">—</span>}</TableCell>
                  <TableCell className="text-sm">{e.enquadramento || <span className="text-muted-foreground">—</span>}</TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {(e.vigencia_inicio || "—")} → {(e.vigencia_fim || "—")}
                  </TableCell>
                  <TableCell className="text-right text-sm">{Number(e.credito_presumido_pct || 0).toFixed(2)}</TableCell>
                  <TableCell className="text-right text-sm">{e.ncm_count}</TableCell>
                  <TableCell>
                    {e.ativo && e.dispensa_antecipacao ? (
                      <Check className="h-4 w-4 text-emerald-600" />
                    ) : (
                      <span className="text-xs text-muted-foreground">off</span>
                    )}
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="flex justify-end gap-1">
                      <Button
                        size="icon"
                        variant="ghost"
                        className="h-7 w-7"
                        title="Editar"
                        onClick={() => handleEdit(e)}
                      >
                        <Pencil className="h-3.5 w-3.5" />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        className="h-7 w-7"
                        title={`Ver NCMs (${e.ncm_count})`}
                        onClick={() => handleOpenNcms(e)}
                      >
                        <List className="h-3.5 w-3.5" />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        className="h-7 w-7"
                        title="Importar NCMs"
                        onClick={() => { setImportTarget(e); setImportFile(null); setNcmsTarget(null); }}
                      >
                        <Upload className="h-3.5 w-3.5" />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        className="h-7 w-7 text-red-500 hover:text-red-700"
                        title="Excluir"
                        onClick={() => handleDelete(e)}
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}

        <p className="text-[11px] text-muted-foreground mt-3 flex items-center gap-1">
          <Building2 className="h-3 w-3" />
          {list.length} enquadramento(s) — {list.filter(e => e.programa === "PRODEPE").length} PRODEPE,{" "}
          {list.filter(e => e.programa === "PROIND").length} PROIND;{" "}
          {list.filter(e => e.ativo && e.dispensa_antecipacao).length} com dispensa ativa.
        </p>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Inaplicabilidade de ICMS Fronteira — Fase 1: cadastro + aprovação
// ---------------------------------------------------------------------------
interface InaplicRegra {
  id: string;
  uf_estado: string;
  id_regra: string;
  instituto: string;
  grupo: string;
  hipotese: string;
  tipo_verif: string;
  registro_sped: string;
  campo_sped: string;
  valores_gatilho: string;
  registro_sped_2: string;
  valores_2: string;
  logica: string;
  resultado: string;
  instrucao: string;
  base_legal: string;
  vigencia_inicio: string | null;
  vigencia_fim: string | null;
  auto_aplicavel: boolean;
  status_aprovacao: string;
}

const INSTITUTO_LABEL: Record<string, string> = {
  ANTECIPACAO: "Antecipação ICMS Fronteira",
  ANT_PARCIAL: "Antecipação Parcial",
  ANT_PROPRIA: "Antecipação Própria",
  ST: "Substituição Tributária",
};

export function InaplicabilidadeTab({ uf }: { uf: string }) {
  const { token } = useAuth();
  const [list, setList] = useState<InaplicRegra[]>([]);
  const [loading, setLoading] = useState(false);
  const [importing, setImporting] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);

  const headers = (extra: Record<string, string> = {}): Record<string, string> => ({
    Authorization: `Bearer ${token}`,
    ...extra,
  });

  const load = async () => {
    if (!token || !uf) return;
    setLoading(true);
    try {
      const res = await fetch(`/api/icms-fronteira/inaplicabilidade?uf=${uf}`, { headers: headers() });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const d = await res.json();
      setList(d.regras ?? []);
    } catch (e) {
      toast.error("Falha ao carregar regras: " + (e instanceof Error ? e.message : ""));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); /* eslint-disable-next-line */ }, [uf, token]);

  const handleImport = async (files: FileList | null) => {
    if (!files || files.length === 0) return;
    setImporting(true);
    try {
      const fd = new FormData();
      Array.from(files).forEach((f) => fd.append("files", f));
      const res = await fetch("/api/icms-fronteira/inaplicabilidade/importar", {
        method: "POST", headers: headers(), body: fd,
      });
      if (!res.ok) throw new Error(await res.text());
      const d = await res.json();
      const byUf = Object.entries(d.by_uf ?? {}).map(([u, n]) => `${u}: ${n}`).join(", ");
      toast.success(`${d.imported} regra(s) importada(s) (${byUf || "—"})`);
      if (d.warnings?.length) toast.warning(d.warnings.join(" | "));
      if (fileRef.current) fileRef.current.value = "";
      load();
    } catch (e) {
      toast.error("Falha ao importar: " + (e instanceof Error ? e.message : ""));
    } finally {
      setImporting(false);
    }
  };

  const setStatus = async (id: string, status: string) => {
    try {
      const res = await fetch(`/api/icms-fronteira/inaplicabilidade/${id}`, {
        method: "PUT", headers: headers({ "Content-Type": "application/json" }),
        body: JSON.stringify({ status }),
      });
      if (!res.ok) throw new Error(await res.text());
      setList((prev) => prev.map((r) => (r.id === id ? { ...r, status_aprovacao: status } : r)));
    } catch (e) {
      toast.error("Falha ao atualizar: " + (e instanceof Error ? e.message : ""));
    }
  };

  const total = list.length;
  const aprovadas = list.filter((r) => r.status_aprovacao === "aprovada").length;
  const pendentes = list.filter((r) => r.status_aprovacao === "pendente").length;
  const rejeitadas = list.filter((r) => r.status_aprovacao === "rejeitada").length;

  const grupos = Array.from(new Set(list.map((r) => r.instituto)));

  return (
    <div className="space-y-4">
      <div className="p-3 border-l-4 border-amber-400 bg-amber-50 text-xs text-amber-900 rounded">
        <strong>Inaplicabilidade de ICMS Fronteira.</strong> Importe as planilhas do contador
        (PE/BA/CE). As regras entram como <em>pendentes</em> para sua aprovação. Apenas regras
        <strong> aprovadas e auto-aplicáveis</strong> (gatilho 100% derivável do SPED) serão usadas
        pelo motor numa fase futura — esta tela é só cadastro e aprovação.
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <input
          ref={fileRef}
          type="file"
          accept=".xlsx"
          multiple
          className="hidden"
          onChange={(e) => handleImport(e.target.files)}
        />
        <Button onClick={() => fileRef.current?.click()} disabled={importing}>
          <Upload className="h-4 w-4 mr-2" />
          {importing ? "Importando..." : "Importar planilhas (XLSX)"}
        </Button>
        <Button variant="outline" onClick={load} disabled={loading}>
          {loading ? "Atualizando..." : "Atualizar"}
        </Button>
        <span className="text-xs text-muted-foreground ml-auto">
          UF <strong>{uf}</strong> · {total} regra(s) — {pendentes} pendente(s), {aprovadas} aprovada(s), {rejeitadas} rejeitada(s)
        </span>
      </div>

      {total === 0 ? (
        <p className="text-sm text-muted-foreground py-6 text-center">
          Nenhuma regra para {uf}. Importe a planilha de inaplicabilidade desta UF.
        </p>
      ) : (
        grupos.map((inst) => {
          const regras = list.filter((r) => r.instituto === inst);
          return (
            <div key={inst} className="rounded-md border overflow-x-auto">
              <div className="px-3 py-2 bg-slate-50 text-sm font-semibold border-b">
                {INSTITUTO_LABEL[inst] ?? inst} <span className="text-xs text-muted-foreground">({regras.length})</span>
              </div>
              <Table className="text-xs">
                <TableHeader>
                  <TableRow>
                    <TableHead className="text-xs">Regra</TableHead>
                    <TableHead className="text-xs">Hipótese</TableHead>
                    <TableHead className="text-xs">Gatilho</TableHead>
                    <TableHead className="text-xs">Resultado</TableHead>
                    <TableHead className="text-xs">Base legal</TableHead>
                    <TableHead className="text-xs">Viab.</TableHead>
                    <TableHead className="text-xs">Status</TableHead>
                    <TableHead className="text-xs text-right">Ação</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {regras.map((r) => (
                    <TableRow key={r.id}>
                      <TableCell className="font-mono whitespace-nowrap">{r.id_regra}</TableCell>
                      <TableCell className="max-w-[240px] whitespace-normal leading-snug">
                        {r.hipotese || r.grupo}
                        {r.vigencia_inicio && (
                          <span className="block text-[10px] text-muted-foreground">
                            vig. {r.vigencia_inicio}{r.vigencia_fim ? `–${r.vigencia_fim}` : ""}
                          </span>
                        )}
                      </TableCell>
                      <TableCell className="max-w-[180px] whitespace-normal leading-snug">
                        <span className="font-medium">{r.tipo_verif}</span>
                        {r.valores_gatilho && <span className="block text-[10px] text-muted-foreground">{r.valores_gatilho}</span>}
                      </TableCell>
                      <TableCell className="max-w-[160px] whitespace-normal leading-snug">{r.resultado}</TableCell>
                      <TableCell className="max-w-[160px] whitespace-normal leading-snug text-muted-foreground">{r.base_legal}</TableCell>
                      <TableCell>
                        {r.auto_aplicavel ? (
                          <span className="inline-block px-1.5 py-0.5 rounded bg-green-100 text-green-800 text-[10px]">auto</span>
                        ) : (
                          <span className="inline-block px-1.5 py-0.5 rounded bg-gray-100 text-gray-600 text-[10px]">manual</span>
                        )}
                      </TableCell>
                      <TableCell>
                        <span className={
                          "inline-block px-1.5 py-0.5 rounded text-[10px] " +
                          (r.status_aprovacao === "aprovada" ? "bg-green-100 text-green-800" :
                           r.status_aprovacao === "rejeitada" ? "bg-red-100 text-red-800" :
                           "bg-amber-100 text-amber-800")
                        }>
                          {r.status_aprovacao}
                        </span>
                      </TableCell>
                      <TableCell className="text-right whitespace-nowrap">
                        {r.status_aprovacao !== "aprovada" && (
                          <Button size="sm" variant="outline" className="h-7 mr-1" onClick={() => setStatus(r.id, "aprovada")}>
                            <Check className="h-3 w-3" />
                          </Button>
                        )}
                        {r.status_aprovacao !== "rejeitada" && (
                          <Button size="sm" variant="outline" className="h-7" onClick={() => setStatus(r.id, "rejeitada")}>
                            <X className="h-3 w-3" />
                          </Button>
                        )}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          );
        })
      )}
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

  const allBranches = data.branches ?? [];
  const branchesFiltered = uf ? allBranches.filter(b => b.uf === uf) : allBranches;
  const branchesCount = branchesFiltered.length;

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
        <TabsTrigger value="prodepe" className="flex items-center gap-2">
          <ShieldCheck className="h-4 w-4" />
          PRODEPE
        </TabsTrigger>
        <TabsTrigger value="regras" className="flex items-center gap-2">
          <FileText className="h-4 w-4" />
          Regras NCM por Decreto
        </TabsTrigger>
        <TabsTrigger value="inaplicabilidade" className="flex items-center gap-2">
          <AlertTriangle className="h-4 w-4" />
          Inaplicabilidade
        </TabsTrigger>
        <TabsTrigger value="empresa" className="flex items-center gap-2">
          <Building className="h-4 w-4" />
          Empresa
        </TabsTrigger>
      </TabsList>

      <TabsContent value="filiais">
        <FiliaisTab branches={allBranches} uf={uf} />
      </TabsContent>

      <TabsContent value="ufs">
        <UFsHubTab uf={uf} />
      </TabsContent>

      <TabsContent value="segmentos">
        <SegmentosTab uf={uf} />
      </TabsContent>

      <TabsContent value="prodepe">
        <ProdepeTab />
      </TabsContent>

      <TabsContent value="regras">
        <div className="mb-4 p-3 border-l-4 border-emerald-400 bg-emerald-50 text-xs text-emerald-900 rounded">
          <strong>Regras NCM por Decreto.</strong> Cadastre prefixos NCM com regime fiscal
          (alíquota interna, MVA original/ajustado, segmento ST) <em>amparados em decreto</em>{" "}
          — esses parâmetros alimentam o motor de cálculo de fronteira. Regras globais (vindas
          de seed) podem ser sobrescritas por regras da empresa. Vincule sempre a uma UF e,
          quando aplicável, a um decreto/protocolo na coluna de origem.
        </div>
        <RegrasTab token={token} />
      </TabsContent>

      <TabsContent value="inaplicabilidade">
        <InaplicabilidadeTab uf={uf} />
      </TabsContent>

      <TabsContent value="empresa">
        <EmpresaEditTab company={data.company} />
      </TabsContent>
    </Tabs>
  );
}
