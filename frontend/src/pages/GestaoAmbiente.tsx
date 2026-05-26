import { useState, useEffect, useRef } from "react";
import { useNavigate } from "react-router-dom";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { ImageUp, Plus, Trash2, Building, Layers, Factory, Pencil, MapPin } from "lucide-react";
import { toast } from "sonner";
import { useAuth } from "@/contexts/AuthContext";

interface Environment {
  id: string;
  name: string;
  description: string;
  created_at: string;
}

interface EnterpriseGroup {
  id: string;
  environment_id: string;
  name: string;
  description: string;
  created_at: string;
}

interface Company {
  id: string;
  group_id: string;
  cnpj: string;
  name: string;
  trade_name: string;
  regime_tributario: string;
  inscricao_estadual?: string;
  cnae_principal?: string;
  cnae_secundario?: string[];
  municipio?: string;
  segmento_economico?: string;
  incentivos_fiscais?: unknown;
  created_at: string;
}

interface Branch {
  cnpj: string;
  company_name: string;
  uf: string;
  inscricao_estadual: string;
  cod_municipio: string;
  municipio_nome: string;
  uf_nome: string;
}

interface UserHierarchy {
  environment: Environment;
  group: EnterpriseGroup;
  company: Company;
  branches: Branch[];
}

export default function GestaoAmbiente() {
  const [environments, setEnvironments] = useState<Environment[]>([]);
  const [selectedEnv, setSelectedEnv] = useState<Environment | null>(null);
  const [groups, setGroups] = useState<EnterpriseGroup[]>([]);
  const [selectedGroup, setSelectedGroup] = useState<EnterpriseGroup | null>(null);
  const [companies, setCompanies] = useState<Company[]>([]);
  
  // Modal states
  const [isEnvModalOpen, setIsEnvModalOpen] = useState(false);
  const [isGroupModalOpen, setIsGroupModalOpen] = useState(false);
  const [isCompanyModalOpen, setIsCompanyModalOpen] = useState(false);
  
  // Form states
  const [newEnvName, setNewEnvName] = useState("");
  const [newEnvDesc, setNewEnvDesc] = useState("");
  const [newGroupName, setNewGroupName] = useState("");
  const [newGroupDesc, setNewGroupDesc] = useState("");
  const [newCompanyCNPJ, setNewCompanyCNPJ] = useState("");
  const [newCompanyName, setNewCompanyName] = useState("");
  const [newCompanyTradeName, setNewCompanyTradeName] = useState("");
  const [newCompanyRegime, setNewCompanyRegime] = useState("lucro_real");
  const [newCompanyCNAE, setNewCompanyCNAE] = useState("");
  const [newCompanySegmento, setNewCompanySegmento] = useState("");

  const [editingGroup, setEditingGroup] = useState<EnterpriseGroup | null>(null);
  const [editGroupName, setEditGroupName] = useState("");
  const [editGroupDesc, setEditGroupDesc] = useState("");

  const [editingCompany, setEditingCompany] = useState<Company | null>(null);
  const [editRegime, setEditRegime] = useState("lucro_real");
  const [editCNPJ, setEditCNPJ] = useState("");
  const [editCNAE, setEditCNAE] = useState("");
  const [editSegmento, setEditSegmento] = useState("");

  const [loading, setLoading] = useState(false);
  const { user, token } = useAuth();
  const navigate = useNavigate();
  const [userHierarchy, setUserHierarchy] = useState<UserHierarchy | null>(null);

  // Logo da empresa em edição
  const [editLogoPreview, setEditLogoPreview] = useState<string | null>(null);
  const [uploadingLogo, setUploadingLogo] = useState(false);
  const editLogoInputRef = useRef<HTMLInputElement>(null);

  // Initial Load
  useEffect(() => {
    if (!user) return;
    // Admin tem a UI hierárquica (3 colunas) + abas Filiais/UFs; o não-admin
    // tem só os cards de cabeçalho + abas. Em ambos os casos precisamos da
    // hierarquia do usuário para alimentar a aba "Filiais".
    if (user.role === 'admin') {
      fetchEnvironments();
    }
    fetchUserHierarchy();
  }, [user]);

  // Load Groups when Env selected — clear state first to avoid stale flash,
  // then guard against out-of-order responses with a cancellation flag.
  useEffect(() => {
    setGroups([]);
    setSelectedGroup(null);
    setCompanies([]);
    if (!selectedEnv) return;

    let cancelled = false;
    fetch(`/api/config/groups?environment_id=${selectedEnv.id}`)
      .then((res) => {
        if (!res.ok) throw new Error("Failed to fetch groups");
        return res.json();
      })
      .then((data) => {
        if (!cancelled) setGroups(data);
      })
      .catch((error) => {
        if (!cancelled) {
          console.error(error);
          toast.error("Erro ao carregar grupos de empresas");
        }
      });

    return () => { cancelled = true; };
  }, [selectedEnv]);

  // Load Companies when Group selected
  useEffect(() => {
    if (selectedGroup) {
      fetchCompanies(selectedGroup.id);
    } else {
      setCompanies([]);
    }
  }, [selectedGroup]);

  const fetchUserHierarchy = async () => {
    try {
      setLoading(true);

      const res = await fetch("/api/user/hierarchy", {
      });
      if (!res.ok) throw new Error("Failed to fetch hierarchy");
      const data = await res.json();
      setUserHierarchy(data);
    } catch (error) {
      console.error(error);
      toast.error("Erro ao carregar dados do usuário");
    } finally {
      setLoading(false);
    }
  };

  const fetchEnvironments = async () => {
    try {
      setLoading(true);

      const res = await fetch("/api/config/environments", {
      });
      if (!res.ok) throw new Error("Failed to fetch environments");
      const data = await res.json();
      setEnvironments(data);
      // Select first one by default if none selected and data exists
      if (!selectedEnv && data.length > 0) {
        setSelectedEnv(data[0]);
      }
    } catch (error) {
      console.error(error);
      toast.error("Erro ao carregar ambientes");
    } finally {
      setLoading(false);
    }
  };

  const fetchGroups = async (envId: string) => {
    try {

      const res = await fetch(`/api/config/groups?environment_id=${envId}`, {
      });
      if (!res.ok) throw new Error("Failed to fetch groups");
      const data = await res.json();
      setGroups(data);
    } catch (error) {
      console.error(error);
      toast.error("Erro ao carregar grupos de empresas");
    }
  };

  const fetchCompanies = async (groupId: string) => {
    try {

      const res = await fetch(`/api/config/companies?group_id=${groupId}`, {
      });
      if (!res.ok) throw new Error("Failed to fetch companies");
      const data = await res.json();
      setCompanies(data);
    } catch (error) {
      console.error(error);
      toast.error("Erro ao carregar empresas");
    }
  };

  const handleCreateEnvironment = async () => {
    if (!newEnvName) {
      toast.error("Nome do ambiente é obrigatório");
      return;
    }

    try {

      const res = await fetch("/api/config/environments", {
        method: "POST",
        headers: { 
          "Content-Type": "application/json"
        },
        body: JSON.stringify({ name: newEnvName, description: newEnvDesc }),
      });

      if (!res.ok) throw new Error("Failed to create");
      
      toast.success("Ambiente criado com sucesso!");
      setIsEnvModalOpen(false);
      setNewEnvName("");
      setNewEnvDesc("");
      fetchEnvironments();
    } catch (error) {
      toast.error("Erro ao criar ambiente");
    }
  };

  const handleCreateGroup = async () => {
    if (!selectedEnv) return;
    if (!newGroupName) {
      toast.error("Nome do grupo é obrigatório");
      return;
    }

    try {

      const res = await fetch("/api/config/groups", {
        method: "POST",
        headers: { 
          "Content-Type": "application/json"
        },
        body: JSON.stringify({
          environment_id: selectedEnv.id,
          name: newGroupName,
          description: newGroupDesc
        }),
      });

      if (!res.ok) throw new Error("Failed to create");
      
      toast.success("Grupo criado com sucesso!");
      setIsGroupModalOpen(false);
      setNewGroupName("");
      setNewGroupDesc("");
      fetchGroups(selectedEnv.id);
    } catch (error) {
      toast.error("Erro ao criar grupo");
    }
  };

  const handleCreateCompany = async () => {
    if (!selectedGroup) return;
    if (!newCompanyName) {
      toast.error("Razão Social é obrigatória");
      return;
    }

    try {

      const res = await fetch("/api/config/companies", {
        method: "POST",
        headers: { 
          "Content-Type": "application/json"
        },
        body: JSON.stringify({
          group_id: selectedGroup.id,
          cnpj: newCompanyCNPJ,
          name: newCompanyName,
          trade_name: newCompanyTradeName,
          regime_tributario: newCompanyRegime,
          cnae_principal: newCompanyCNAE,
          segmento_economico: newCompanySegmento,
        }),
      });

      if (!res.ok) throw new Error("Failed to create");
      
      toast.success("Empresa cadastrada com sucesso!");
      setIsCompanyModalOpen(false);
      setNewCompanyCNPJ("");
      setNewCompanyName("");
      setNewCompanyTradeName("");
      setNewCompanyRegime("lucro_real");
      setNewCompanyCNAE("");
      setNewCompanySegmento("");
      fetchCompanies(selectedGroup.id);
    } catch (error) {
      toast.error("Erro ao criar empresa");
    }
  };

  const loadEmpresaAssets = async (companyId: string) => {
    if (!token) return;
    setEditLogoPreview(null);
    const headers: Record<string, string> = { Authorization: `Bearer ${token}`, 'X-Company-ID': companyId };
    // metadados
    const meta = await fetch('/api/config/empresa/parametros', { headers }).then(r => r.ok ? r.json() : null).catch(() => null);
    if (meta?.tem_logo) {
      const r = await fetch('/api/config/empresa/logo', { headers });
      if (r.ok) { const blob = await r.blob(); setEditLogoPreview(URL.createObjectURL(blob)); }
    }
  };

  const uploadEmpresaLogo = async (companyId: string, file: File) => {
    if (!token) return;
    setUploadingLogo(true);
    const form = new FormData(); form.append('logo', file);
    const headers: Record<string, string> = { Authorization: `Bearer ${token}`, 'X-Company-ID': companyId };
    try {
      const res = await fetch('/api/config/empresa/logo', { method: 'POST', headers, body: form });
      if (!res.ok) throw new Error();
      toast.success('Logo salva');
      await loadEmpresaAssets(companyId);
    } catch { toast.error('Erro ao salvar logo'); }
    finally { setUploadingLogo(false); }
  };

  const handleUpdateGroup = async () => {
    if (!editingGroup) return;
    if (!editGroupName.trim()) { toast.error("Nome do grupo é obrigatório"); return; }
    try {
      const res = await fetch(`/api/config/groups?id=${editingGroup.id}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: editGroupName.trim(), description: editGroupDesc }),
      });
      if (!res.ok) throw new Error("Falha ao atualizar");
      toast.success("Grupo atualizado");
      setEditingGroup(null);
      if (selectedEnv) fetchGroups(selectedEnv.id);
    } catch {
      toast.error("Erro ao atualizar grupo");
    }
  };

  const handleDeleteEnvironment = async (id: string) => {
    if (!confirm("Tem certeza? Isso apagará TODOS os grupos e empresas vinculados.")) return;
    
    try {

      const res = await fetch(`/api/config/environments?id=${id}`, { 
        method: "DELETE",
      });
      if (!res.ok) throw new Error("Failed to delete");
      toast.success("Ambiente removido");
      if (selectedEnv?.id === id) setSelectedEnv(null);
      fetchEnvironments();
    } catch (error) {
      toast.error("Erro ao remover ambiente");
    }
  };

  const handleDeleteGroup = async (id: string) => {
    if (!confirm("Tem certeza? Isso apagará TODAS as empresas vinculadas.")) return;
    
    try {

      const res = await fetch(`/api/config/groups?id=${id}`, { 
        method: "DELETE",
      });
      if (!res.ok) throw new Error("Failed to delete");
      toast.success("Grupo removido");
      if (selectedGroup?.id === id) setSelectedGroup(null);
      if (selectedEnv) fetchGroups(selectedEnv.id);
    } catch (error) {
      toast.error("Erro ao remover grupo");
    }
  };

  const handleUpdateCompany = async () => {
    if (!editingCompany) return;
    try {
      const res = await fetch(`/api/config/companies?id=${editingCompany.id}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          regime_tributario: editRegime,
          cnpj: editCNPJ,
          cnae_principal: editCNAE,
          segmento_economico: editSegmento,
        }),
      });
      if (!res.ok) {
        const errText = await res.text();
        throw new Error(errText || "Failed to update");
      }
      toast.success("Empresa atualizada");
      setEditingCompany(null);
      if (selectedGroup) fetchCompanies(selectedGroup.id);
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Erro ao atualizar empresa";
      toast.error(msg);
    }
  };

  const handleDeleteCompany = async (id: string) => {
    if (!confirm("Tem certeza?")) return;
    
    try {

      const res = await fetch(`/api/config/companies?id=${id}`, { 
        method: "DELETE",
      });
      if (!res.ok) throw new Error("Failed to delete");
      toast.success("Empresa removida");
      if (selectedGroup) fetchCompanies(selectedGroup.id);
    } catch (error) {
      toast.error("Erro ao remover empresa");
    }
  };

  if (user?.role !== 'admin') {
    return (
      <div className="container mx-auto p-4 space-y-6">
        <div>
            <h1 className="text-3xl font-bold text-gray-900">Meu Ambiente</h1>
            <p className="text-gray-500 mt-1">
                Visualização dos dados vinculados ao seu usuário
            </p>
        </div>

        {loading ? (
             <p>Carregando...</p>
        ) : !userHierarchy ? (
             <p>Nenhum dado encontrado. Contate o administrador.</p>
        ) : (
            <>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
                {/* Environment */}
                <Card>
                    <CardHeader>
                        <CardTitle className="flex items-center gap-2">
                            <Layers className="h-5 w-5" />
                            Ambiente
                        </CardTitle>
                    </CardHeader>
                    <CardContent>
                         <div className="text-lg font-medium">{userHierarchy.environment.name}</div>
                         <div className="text-sm text-muted-foreground">{userHierarchy.environment.description}</div>
                    </CardContent>
                </Card>

                {/* Group */}
                <Card>
                    <CardHeader>
                        <CardTitle className="flex items-center gap-2">
                            <Building className="h-5 w-5" />
                            Grupo
                        </CardTitle>
                    </CardHeader>
                    <CardContent>
                         <div className="text-lg font-medium">{userHierarchy.group.name}</div>
                         <div className="text-sm text-muted-foreground">{userHierarchy.group.description}</div>
                    </CardContent>
                </Card>

                {/* Company */}
                <Card>
                    <CardHeader>
                        <CardTitle className="flex items-center gap-2">
                            <Factory className="h-5 w-5" />
                            Empresa
                        </CardTitle>
                    </CardHeader>
                    <CardContent>
                         <div className="text-lg font-medium">{userHierarchy.company.name}</div>
                         <p className="text-[10px] text-gray-400 font-mono truncate mb-1" title={userHierarchy.company.id}>ID: {userHierarchy.company.id}</p>
                         {userHierarchy.company.cnpj && <div className="text-sm text-muted-foreground">CNPJ: {userHierarchy.company.cnpj}</div>}
                    </CardContent>
                </Card>
            </div>

            <div className="rounded-md border bg-blue-50 border-blue-200 p-4">
                <p className="text-sm font-medium text-blue-900 flex items-center gap-2">
                    <MapPin className="h-4 w-4" />
                    Filiais, parâmetros por UF e edição da empresa agora ficam no Módulo ICMS Fronteira.
                </p>
                <p className="text-xs text-blue-700 mt-1">
                    Acesse <strong>ICMS Fronteira → aba Administrativo</strong> para ver as filiais importadas,
                    configurar benefícios fiscais por UF e ajustar os dados da empresa.
                </p>
                <Button
                    size="sm"
                    className="mt-3"
                    onClick={() => navigate('/icms-fronteira/administrativo')}
                >
                    Ir para Administrativo
                </Button>
            </div>
            </>
        )}
      </div>
    );
  }

  return (
    <div className="container mx-auto p-4 space-y-6">
      <div>
        <h1 className="text-3xl font-bold text-gray-900">Gestão de Ambientes</h1>
        <p className="text-gray-500 mt-1">
          Configuração Hierárquica: Ambiente &gt; Grupo &gt; Empresa
        </p>
      </div>

      <div className="rounded-md border bg-blue-50 border-blue-200 p-3 text-xs text-blue-800">
        Filiais importadas, parâmetros por UF e edição da empresa migraram para
        <strong> ICMS Fronteira → aba Administrativo</strong>. Esta página cuida apenas
        da estrutura hierárquica (Ambiente &gt; Grupo &gt; Empresa).
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-6 h-[calc(100vh-12rem)]">
        {/* Column 1: Environments */}
        <div className="flex flex-col space-y-4 h-full">
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-semibold flex items-center gap-2">
              <Layers className="w-5 h-5" /> Ambientes
            </h2>
            <Dialog open={isEnvModalOpen} onOpenChange={setIsEnvModalOpen}>
              <DialogTrigger asChild>
                <Button size="sm" variant="outline"><Plus className="w-4 h-4" /></Button>
              </DialogTrigger>
              <DialogContent>
                <DialogHeader>
                  <DialogTitle>Novo Ambiente</DialogTitle>
                  <DialogDescription>Crie um novo ambiente (Ex: Produção, Homologação).</DialogDescription>
                </DialogHeader>
                <div className="space-y-4 py-4">
                  <div className="space-y-2">
                    <Label>Nome</Label>
                    <Input value={newEnvName} onChange={(e) => setNewEnvName(e.target.value)} placeholder="Ex: Ambiente Produção" />
                  </div>
                  <div className="space-y-2">
                    <Label>Descrição</Label>
                    <Input value={newEnvDesc} onChange={(e) => setNewEnvDesc(e.target.value)} placeholder="Opcional" />
                  </div>
                </div>
                <DialogFooter>
                  <Button onClick={handleCreateEnvironment}>Criar</Button>
                </DialogFooter>
              </DialogContent>
            </Dialog>
          </div>
          
          <div className="flex-1 overflow-y-auto space-y-2 border rounded-md p-2 bg-gray-50/50">
            {loading && <p className="text-sm text-muted-foreground p-2">Carregando...</p>}
            {!loading && environments.length === 0 && (
              <p className="text-sm text-muted-foreground p-2">Nenhum ambiente.</p>
            )}
            {environments.map((env) => (
              <div
                key={env.id}
                className={`flex items-center justify-between p-3 rounded-md border cursor-pointer transition-all ${
                  selectedEnv?.id === env.id
                    ? "bg-white border-primary shadow-sm ring-1 ring-primary"
                    : "bg-white border-gray-200 hover:border-primary/50"
                }`}
                onClick={() => setSelectedEnv(env)}
              >
                <div className="overflow-hidden">
                  <p className="font-medium text-sm truncate">{env.name}</p>
                  {env.description && <p className="text-xs text-gray-500 truncate">{env.description}</p>}
                </div>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-6 w-6 text-gray-400 hover:text-red-500"
                  onClick={(e) => {
                    e.stopPropagation();
                    handleDeleteEnvironment(env.id);
                  }}
                >
                  <Trash2 className="w-3 h-3" />
                </Button>
              </div>
            ))}
          </div>
        </div>

        {/* Column 2: Groups */}
        <div className="flex flex-col space-y-4 h-full">
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-semibold flex items-center gap-2">
              <Building className="w-5 h-5" /> Grupos
            </h2>
            <Dialog open={isGroupModalOpen} onOpenChange={setIsGroupModalOpen}>
              <DialogTrigger asChild>
                <Button size="sm" variant="outline" disabled={!selectedEnv}><Plus className="w-4 h-4" /></Button>
              </DialogTrigger>
              <DialogContent>
                <DialogHeader>
                  <DialogTitle>Novo Grupo</DialogTitle>
                  <DialogDescription>Vinculado a: {selectedEnv?.name}</DialogDescription>
                </DialogHeader>
                <div className="space-y-4 py-4">
                  <div className="space-y-2">
                    <Label>Nome do Grupo</Label>
                    <Input value={newGroupName} onChange={(e) => setNewGroupName(e.target.value)} placeholder="Ex: Grupo Varejo X" />
                  </div>
                  <div className="space-y-2">
                    <Label>Descrição</Label>
                    <Input value={newGroupDesc} onChange={(e) => setNewGroupDesc(e.target.value)} placeholder="Opcional" />
                  </div>
                </div>
                <DialogFooter>
                  <Button onClick={handleCreateGroup}>Criar</Button>
                </DialogFooter>
              </DialogContent>
            </Dialog>
          </div>

          <div className="flex-1 overflow-y-auto space-y-2 border rounded-md p-2 bg-gray-50/50">
            {!selectedEnv ? (
              <div className="h-full flex items-center justify-center text-gray-400 text-sm">
                Selecione um ambiente
              </div>
            ) : groups.length === 0 ? (
               <div className="h-full flex items-center justify-center text-gray-400 text-sm">
                Nenhum grupo cadastrado
              </div>
            ) : (
              groups.map((group) => (
                <div key={group.id}>
                <div
                  className={`flex items-center justify-between p-3 rounded-md border cursor-pointer transition-all ${
                    selectedGroup?.id === group.id
                      ? "bg-white border-primary shadow-sm ring-1 ring-primary"
                      : "bg-white border-gray-200 hover:border-primary/50"
                  }`}
                  onClick={() => setSelectedGroup(group)}
                >
                  <div className="overflow-hidden">
                    <p className="font-medium text-sm truncate">{group.name}</p>
                    <p className="text-[10px] text-gray-400 font-mono truncate" title={group.id}>ID: {group.id}</p>
                    {group.description && <p className="text-xs text-gray-500 truncate">{group.description}</p>}
                  </div>
                  <div className="flex items-center gap-1 shrink-0">
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-6 w-6 text-gray-400 hover:text-blue-500"
                      title="Renomear grupo"
                      onClick={(e) => {
                        e.stopPropagation();
                        setEditingGroup(group);
                        setEditGroupName(group.name);
                        setEditGroupDesc(group.description ?? "");
                      }}
                    >
                      <Pencil className="w-3 h-3" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-6 w-6 text-gray-400 hover:text-red-500"
                      onClick={(e) => {
                        e.stopPropagation();
                        handleDeleteGroup(group.id);
                      }}
                    >
                      <Trash2 className="w-3 h-3" />
                    </Button>
                  </div>
                </div>

                {/* Painel inline de edição do grupo */}
                {editingGroup?.id === group.id && (
                  <div className="mt-2 p-3 border border-blue-200 rounded-md bg-blue-50">
                    <p className="text-xs font-medium text-blue-800 mb-2">Renomear Grupo</p>
                    <div className="space-y-2 mb-2">
                      <div>
                        <p className="text-[10px] text-blue-700 mb-0.5">Nome</p>
                        <Input
                          value={editGroupName}
                          onChange={(e) => setEditGroupName(e.target.value)}
                          placeholder="Nome do grupo"
                          className="h-7 text-xs"
                          onKeyDown={(e) => { if (e.key === 'Enter') handleUpdateGroup(); if (e.key === 'Escape') setEditingGroup(null); }}
                          autoFocus
                        />
                      </div>
                      <div>
                        <p className="text-[10px] text-blue-700 mb-0.5">Descrição (opcional)</p>
                        <Input
                          value={editGroupDesc}
                          onChange={(e) => setEditGroupDesc(e.target.value)}
                          placeholder="Opcional"
                          className="h-7 text-xs"
                        />
                      </div>
                    </div>
                    <div className="flex gap-2">
                      <Button size="sm" className="h-7 text-xs flex-1" onClick={handleUpdateGroup}>
                        Salvar
                      </Button>
                      <Button size="sm" variant="outline" className="h-7 text-xs" onClick={() => setEditingGroup(null)}>
                        Cancelar
                      </Button>
                    </div>
                  </div>
                )}
                </div>
              ))
            )}
          </div>
        </div>

        {/* Column 3: Companies */}
        <div className="flex flex-col space-y-4 h-full">
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-semibold flex items-center gap-2">
              <Factory className="w-5 h-5" /> Empresas
            </h2>
            <Dialog open={isCompanyModalOpen} onOpenChange={setIsCompanyModalOpen}>
              <DialogTrigger asChild>
                <Button size="sm" variant="outline" disabled={!selectedGroup}><Plus className="w-4 h-4" /></Button>
              </DialogTrigger>
              <DialogContent>
                <DialogHeader>
                  <DialogTitle>Nova Empresa</DialogTitle>
                  <DialogDescription>Vinculada a: {selectedGroup?.name}</DialogDescription>
                </DialogHeader>
                <div className="space-y-4 py-4">
                  <div className="space-y-2">
                    <Label>CNPJ (apenas números)</Label>
                    <Input value={newCompanyCNPJ} onChange={(e) => setNewCompanyCNPJ(e.target.value)} placeholder="Opcional" maxLength={14} />
                  </div>
                  <div className="space-y-2">
                    <Label>Razão Social</Label>
                    <Input value={newCompanyName} onChange={(e) => setNewCompanyName(e.target.value)} placeholder="Empresa S/A" />
                  </div>
                  <div className="space-y-2">
                    <Label>Nome Fantasia</Label>
                    <Input value={newCompanyTradeName} onChange={(e) => setNewCompanyTradeName(e.target.value)} placeholder="Empresa X" />
                  </div>
                  <div className="space-y-2">
                    <Label>Regime Tributário</Label>
                    <Select value={newCompanyRegime} onValueChange={setNewCompanyRegime}>
                      <SelectTrigger>
                        <SelectValue placeholder="Selecione o regime" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="nao_informado">Não informado</SelectItem>
                        <SelectItem value="lucro_real">Lucro Real</SelectItem>
                        <SelectItem value="lucro_presumido">Lucro Presumido</SelectItem>
                        <SelectItem value="simples_nacional">Simples Nacional</SelectItem>
                      </SelectContent>
                    </Select>
                    {(newCompanyRegime === 'lucro_real' || newCompanyRegime === 'lucro_presumido') && (
                      <p className="text-[11px] text-amber-600">
                        Lucro Real e Presumido: importação de EFD ICMS obrigatória.
                      </p>
                    )}
                  </div>
                  <div className="space-y-2">
                    <Label>CNAE Principal</Label>
                    <Input value={newCompanyCNAE} onChange={(e) => setNewCompanyCNAE(e.target.value)} placeholder="Ex: 4711301" maxLength={7} />
                  </div>
                  <div className="space-y-2">
                    <Label>Segmento Econômico</Label>
                    <Input value={newCompanySegmento} onChange={(e) => setNewCompanySegmento(e.target.value)} placeholder="Ex: Varejo de móveis" maxLength={100} />
                  </div>
                </div>
                <DialogFooter>
                  <Button onClick={handleCreateCompany}>Cadastrar</Button>
                </DialogFooter>
              </DialogContent>
            </Dialog>
          </div>

          <div className="flex-1 overflow-y-auto space-y-2 border rounded-md p-2 bg-gray-50/50">
             {!selectedGroup ? (
              <div className="h-full flex items-center justify-center text-gray-400 text-sm">
                Selecione um grupo
              </div>
            ) : companies.length === 0 ? (
               <div className="h-full flex items-center justify-center text-gray-400 text-sm">
                Nenhuma empresa cadastrada
              </div>
            ) : (
              companies.map((company) => (
                <div key={company.id}>
                <div
                  className="flex items-center justify-between p-3 rounded-md border bg-white border-gray-200 hover:border-primary/50 transition-all"
                >
                  <div className="overflow-hidden">
                    <p className="font-medium text-sm truncate">{company.name}</p>
                    <p className="text-[10px] text-gray-400 font-mono truncate" title={company.id}>ID: {company.id}</p>
                    {company.cnpj && <p className="text-xs text-gray-500 font-mono">{company.cnpj}</p>}
                    {company.trade_name && <p className="text-xs text-gray-400 truncate">{company.trade_name}</p>}
                    <p className="text-[10px] mt-0.5">
                      <span className={`inline-block px-1.5 py-0.5 rounded text-white font-medium ${
                        company.regime_tributario === 'simples_nacional' ? 'bg-green-500' :
                        company.regime_tributario === 'lucro_real' ? 'bg-blue-500' :
                        company.regime_tributario === 'lucro_presumido' ? 'bg-purple-500' :
                        'bg-gray-400'
                      }`}>
                        {{
                          simples_nacional: 'Simples Nacional',
                          lucro_real: 'Lucro Real',
                          lucro_presumido: 'Lucro Presumido',
                          nao_informado: 'Regime não informado',
                        }[company.regime_tributario] ?? 'Regime não informado'}
                      </span>
                    </p>
                  </div>
                  <div className="flex items-center gap-1 shrink-0">
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-6 w-6 text-gray-400 hover:text-blue-500"
                      title="Alterar regime tributário"
                      onClick={() => {
                        setEditingCompany(company);
                        setEditRegime(company.regime_tributario || 'lucro_real');
                        setEditCNPJ(company.cnpj || '');
                        setEditCNAE(company.cnae_principal || '');
                        setEditSegmento(company.segmento_economico || '');
                        loadEmpresaAssets(company.id);
                      }}
                    >
                      <Pencil className="w-3 h-3" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-6 w-6 text-gray-400 hover:text-red-500"
                      onClick={() => handleDeleteCompany(company.id)}
                    >
                      <Trash2 className="w-3 h-3" />
                    </Button>
                  </div>
                </div>

                {/* Painel inline de edição de empresa */}
                {editingCompany?.id === company.id && (
                  <div className="mt-2 p-3 border border-blue-200 rounded-md bg-blue-50">
                    <p className="text-xs font-medium text-blue-800 mb-2">Editar Empresa</p>
                    <div className="grid grid-cols-2 gap-2 mb-2">
                      <div>
                        <p className="text-[10px] text-blue-700 mb-0.5">CNPJ (só números)</p>
                        <Input
                          value={editCNPJ}
                          onChange={(e) => setEditCNPJ(e.target.value)}
                          placeholder="14 dígitos"
                          maxLength={14}
                          className="h-7 text-xs"
                        />
                      </div>
                      <div>
                        <p className="text-[10px] text-blue-700 mb-0.5">CNAE Principal</p>
                        <Input
                          value={editCNAE}
                          onChange={(e) => setEditCNAE(e.target.value)}
                          placeholder="Ex: 4711301"
                          maxLength={7}
                          className="h-7 text-xs"
                        />
                      </div>
                      <div className="col-span-2">
                        <p className="text-[10px] text-blue-700 mb-0.5">Segmento Econômico</p>
                        <Input
                          value={editSegmento}
                          onChange={(e) => setEditSegmento(e.target.value)}
                          placeholder="Ex: Varejo de móveis"
                          maxLength={100}
                          className="h-7 text-xs"
                        />
                      </div>
                    </div>
                    <div>
                      <p className="text-[10px] text-blue-700 mb-0.5">Regime Tributário</p>
                      <Select value={editRegime} onValueChange={setEditRegime}>
                        <SelectTrigger className="h-8 text-xs">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="lucro_real">Lucro Real</SelectItem>
                          <SelectItem value="lucro_presumido">Lucro Presumido</SelectItem>
                          <SelectItem value="simples_nacional">Simples Nacional</SelectItem>
                          <SelectItem value="nao_informado">Não informado</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                    {/* Logo */}
                    <div className="mt-3 pt-3 border-t border-blue-200">
                      <p className="text-[10px] text-blue-700 mb-1 font-medium flex items-center gap-1"><ImageUp className="w-3 h-3"/>Logo da Empresa</p>
                      {editLogoPreview && (
                        <img src={editLogoPreview} alt="Logo" className="h-10 max-w-[120px] object-contain rounded border bg-white p-0.5 mb-1" />
                      )}
                      <input ref={editLogoInputRef} type="file" accept="image/jpeg,image/png,image/webp" className="hidden"
                        onChange={e => { const f = e.target.files?.[0]; if (f && editingCompany) uploadEmpresaLogo(editingCompany.id, f); }} />
                      <Button size="sm" variant="outline" className="h-6 text-[10px]" disabled={uploadingLogo}
                        onClick={() => editLogoInputRef.current?.click()}>
                        {editLogoPreview ? 'Substituir logo' : 'Enviar logo'}
                      </Button>
                    </div>

                    <div className="flex gap-2 mt-3">
                      <Button size="sm" className="h-7 text-xs flex-1" onClick={handleUpdateCompany}>
                        Salvar
                      </Button>
                      <Button size="sm" variant="outline" className="h-7 text-xs" onClick={() => setEditingCompany(null)}>
                        Cancelar
                      </Button>
                    </div>
                  </div>
                )}
                </div>
              ))
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
