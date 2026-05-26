import { useState, useEffect, useRef } from "react";
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
import { ImageUp, Plus, Trash2, Building, Layers, Factory, Pencil, MapPin, FileText, Check } from "lucide-react";
import { toast } from "sonner";
import { useAuth } from "@/contexts/AuthContext";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { Checkbox } from "@/components/ui/checkbox";

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

interface UFBeneficios {
  aliquota_interna: number | null;
  fecp_percentual: number | null;
  reducao_bc_percentual: number | null;
  mva_ajustada_padrao: number | null;
  inaplicabilidade_st: boolean;
  antecipacao_aplicavel: boolean;
  observacoes: string;
  configurado: boolean;
}

interface UFHubItem {
  uf: string;
  uf_nome: string;
  num_filiais: number;
  legislacao: Record<string, number>;
  beneficios: UFBeneficios;
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

            <Tabs defaultValue="filiais" className="space-y-4">
                <TabsList>
                    <TabsTrigger value="filiais" className="flex items-center gap-2">
                        <Factory className="h-4 w-4" />
                        Filiais
                        <span className="ml-1 text-xs text-muted-foreground">({userHierarchy.branches.length})</span>
                    </TabsTrigger>
                    <TabsTrigger value="ufs" className="flex items-center gap-2">
                        <MapPin className="h-4 w-4" />
                        UFs
                    </TabsTrigger>
                </TabsList>

                <TabsContent value="filiais">
                    <FiliaisTab branches={userHierarchy.branches} />
                </TabsContent>

                <TabsContent value="ufs">
                    <UFsHubTab />
                </TabsContent>
            </Tabs>
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

      <Tabs defaultValue="hierarquia" className="space-y-4">
        <TabsList>
          <TabsTrigger value="hierarquia" className="flex items-center gap-2">
            <Layers className="h-4 w-4" /> Hierarquia
          </TabsTrigger>
          <TabsTrigger value="filiais" className="flex items-center gap-2">
            <Factory className="h-4 w-4" /> Filiais
            {userHierarchy && (
              <span className="ml-1 text-xs text-muted-foreground">({userHierarchy.branches.length})</span>
            )}
          </TabsTrigger>
          <TabsTrigger value="ufs" className="flex items-center gap-2">
            <MapPin className="h-4 w-4" /> UFs
          </TabsTrigger>
        </TabsList>

        <TabsContent value="filiais">
          {userHierarchy
            ? <FiliaisTab branches={userHierarchy.branches} />
            : <p className="text-muted-foreground">Carregando filiais...</p>}
        </TabsContent>

        <TabsContent value="ufs">
          <UFsHubTab />
        </TabsContent>

        <TabsContent value="hierarquia">
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
        </TabsContent>
      </Tabs>
    </div>
  );
}

// ---------------------------------------------------------------------------
// FiliaisTab — aba "Filiais" da Gestão de Ambiente.
//
// Tabela read-only das filiais (CNPJs) vinculadas à empresa do usuário, vindas
// do registro 0000 do SPED a cada importação (worker.go). UF/IE/COD_MUN são
// preenchidos automaticamente; nome do município/UF vem do JOIN com
// municipios_ibge no backend (hierarchy.go).
// ---------------------------------------------------------------------------
function FiliaisTab({ branches }: { branches: Branch[] }) {
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
// UFsHubTab — aba "UFs" da Gestão de Ambiente.
//
// Hub híbrido por UF: para cada UF onde a empresa tem filial (vindo do reg
// 0000 do SPED via /api/uf-hub), mostra:
//   • nº de filiais
//   • status da legislação interpretada pela IA (legislacao_fronteira)
//   • formulário de benefícios fiscais manuais (uf_beneficios_fiscais)
//
// O backend devolve as UFs já com defaults (antecipacao_aplicavel=true) para
// UFs sem registro salvo, então o formulário sempre tem estado inicial válido.
// ---------------------------------------------------------------------------
function UFsHubTab() {
  const { token } = useAuth();
  const [items, setItems] = useState<UFHubItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedUF, setSelectedUF] = useState<string>("");
  // edits[uf] guarda o estado do formulário enquanto o usuário digita.
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
      // hidrata edits com o que veio do backend
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

  // Converte input string para number|null. Vazio → null (não preenchido).
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
      {/* Coluna esquerda — lista de UFs */}
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

      {/* Coluna direita — detalhe da UF selecionada */}
      {current && form && (
        <div className="space-y-4">
          {/* Cabeçalho + status da legislação */}
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

          {/* Formulário de benefícios manuais */}
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
