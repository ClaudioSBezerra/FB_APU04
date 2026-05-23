# Phase 8: Cadastro de Empresas + Ambiente Administrativo por UF — Pattern Map

**Mapeado:** 2026-05-23
**Arquivos analisados:** 8 (3 migrations novas, 2 handlers Go, 1 rota main.go, 2 componentes frontend)
**Análogos encontrados:** 8 / 8

---

## File Classification

| Arquivo Novo/Modificado | Role | Data Flow | Análogo Mais Próximo | Qualidade |
|--------------------------|------|-----------|----------------------|-----------|
| `backend/migrations/096_add_fields_to_companies.sql` | migration | batch | `backend/migrations/077_add_regime_tributario_to_companies.sql` | exact |
| `backend/migrations/097_add_uf_estado_to_fronteira_regras.sql` | migration | batch | `backend/migrations/091_icms_fronteira.sql` | exact |
| `backend/migrations/098_seed_ba_ce_fronteira.sql` | migration | batch | `backend/migrations/091_icms_fronteira.sql` (bloco seed) | exact |
| `backend/handlers/environment.go` (modificação) | handler | CRUD | si mesmo (arquivo existente) | exact |
| `backend/handlers/icms_fronteira_regras.go` (modificação) | handler | CRUD + file-I/O | si mesmo (arquivo existente) | exact |
| `backend/main.go` (nova rota PUT regras) | config/route | request-response | bloco `/api/icms-fronteira/regras` linhas 691-702 | exact |
| `frontend/src/pages/GestaoAmbiente.tsx` (modificação) | component | CRUD | si mesmo (arquivo existente) | exact |
| `frontend/src/pages/IcmsFronteira.tsx` (modificação) | component | CRUD + file-I/O | si mesmo — padrão `RegrasTab` linhas 617-944 | exact |
| `frontend/src/lib/navigation.ts` (modificação label) | config | — | si mesmo — bloco `fronteira` linhas 59-70 | exact |

---

## Pattern Assignments

### `backend/migrations/096_add_fields_to_companies.sql` (migration, batch)

**Análogo:** `backend/migrations/077_add_regime_tributario_to_companies.sql`

**Cabeçalho e comentário** (linhas 1-8 do análogo):
```sql
-- 077_add_regime_tributario_to_companies.sql
-- Adiciona coluna regime_tributario à tabela companies (per D-03).
-- Idempotente: ADD COLUMN IF NOT EXISTS + DO block para constraint.
```

**Padrão ADD COLUMN IF NOT EXISTS** (linhas 9-10 do análogo):
```sql
ALTER TABLE companies
    ADD COLUMN IF NOT EXISTS regime_tributario TEXT NOT NULL DEFAULT 'nao_informado';
```

**Padrão DO block idempotente para constraint** (linhas 12-21 do análogo):
```sql
DO $$ BEGIN
    ALTER TABLE companies ADD CONSTRAINT chk_companies_regime_tributario
        CHECK (regime_tributario IN (
            'lucro_real',
            'lucro_presumido',
            'simples_nacional',
            'nao_informado'
        ));
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;
```

**Padrão COMMENT ON COLUMN** (linha 23 do análogo):
```sql
COMMENT ON COLUMN companies.regime_tributario IS
    'Regime tributário da empresa: ...';
```

**Aplicar para a migration 096:**
```sql
-- 096_add_fields_to_companies.sql
-- Idempotente: ADD COLUMN IF NOT EXISTS. Sem UNIQUE, sem NOT NULL (empresas existentes sem CNPJ).
ALTER TABLE companies ADD COLUMN IF NOT EXISTS cnpj               VARCHAR(18);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS inscricao_estadual VARCHAR(30);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS cnae_principal     VARCHAR(7);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS cnae_secundario    TEXT[];
ALTER TABLE companies ADD COLUMN IF NOT EXISTS municipio          VARCHAR(100);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS segmento_economico VARCHAR(100);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS incentivos_fiscais JSONB;
```

**ATENÇÃO:** Sem UNIQUE no CNPJ (a constraint original `VARCHAR(14) NOT NULL UNIQUE` da migration 013 foi removida na 023). A nova coluna é `VARCHAR(18)` nullable, sem UNIQUE.

---

### `backend/migrations/097_add_uf_estado_to_fronteira_regras.sql` (migration, batch)

**Análogo:** `backend/migrations/091_icms_fronteira.sql` (estrutura) + `077` (padrão idempotente)

**Padrão CREATE TABLE IF NOT EXISTS** (linhas 5-17 de 091):
```sql
CREATE TABLE IF NOT EXISTS icms_fronteira_regras_ncm (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    ...
    CONSTRAINT uq_icms_fronteira_regras UNIQUE NULLS NOT DISTINCT (company_id, ncm_prefixo)
);
```

**Padrão CREATE INDEX IF NOT EXISTS** (linhas 19-22 de 091):
```sql
CREATE INDEX IF NOT EXISTS idx_icms_fronteira_regras_ncm
    ON icms_fronteira_regras_ncm(ncm_prefixo);
```

**Aplicar para a migration 097 — ADD COLUMN + recriar constraint + nova tabela:**
```sql
-- 097_add_uf_estado_to_fronteira_regras.sql
-- Adiciona uf_estado + MVA ajustado; expande constraint UNIQUE; cria icms_fronteira_inaplicabilidades

-- 1) Coluna uf_estado com DEFAULT 'PE' preserva registros existentes (seed PE da 091)
ALTER TABLE icms_fronteira_regras_ncm
    ADD COLUMN IF NOT EXISTS uf_estado VARCHAR(2) NOT NULL DEFAULT 'PE';

-- 2) MVA ajustado pré-calculado para 3 alíquotas interestaduais
ALTER TABLE icms_fronteira_regras_ncm
    ADD COLUMN IF NOT EXISTS mva_ajustado_4pct  NUMERIC(8,4);
ALTER TABLE icms_fronteira_regras_ncm
    ADD COLUMN IF NOT EXISTS mva_ajustado_7pct  NUMERIC(8,4);
ALTER TABLE icms_fronteira_regras_ncm
    ADD COLUMN IF NOT EXISTS mva_ajustado_12pct NUMERIC(8,4);

-- 3) Recriar constraint UNIQUE incluindo uf_estado
--    (DROP sem IF NOT EXISTS pois a constraint existe desde 091)
ALTER TABLE icms_fronteira_regras_ncm
    DROP CONSTRAINT IF EXISTS uq_icms_fronteira_regras;

DO $$ BEGIN
    ALTER TABLE icms_fronteira_regras_ncm
        ADD CONSTRAINT uq_icms_fronteira_regras_uf
        UNIQUE NULLS NOT DISTINCT (company_id, ncm_prefixo, uf_estado);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- 4) Tabela de inaplicabilidades (nova)
CREATE TABLE IF NOT EXISTS icms_fronteira_inaplicabilidades (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    ncm_digits      VARCHAR(8)  NOT NULL,
    uf_estado       VARCHAR(2)  NOT NULL,
    motivo          TEXT        NOT NULL,
    vigencia_inicio DATE,
    vigencia_fim    DATE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_icms_fronteira_inapl_uf_ncm
    ON icms_fronteira_inaplicabilidades(uf_estado, ncm_digits);
```

---

### `backend/migrations/098_seed_ba_ce_fronteira.sql` (migration, batch)

**Análogo:** `backend/migrations/091_icms_fronteira.sql` — bloco INSERT ... ON CONFLICT DO NOTHING (linhas 24-68)

**Padrão seed global com ON CONFLICT DO NOTHING** (linhas 24-28 de 091):
```sql
-- Seed global (company_id IS NULL = aplica a todas as empresas)
INSERT INTO icms_fronteira_regras_ncm
    (ncm_prefixo, descricao, regime, aliquota_interna, mva_original)
VALUES
    ('2202', 'Refrigerantes e bebidas não alcoólicas', 'ST', 20.5, 140.00),
    ...
ON CONFLICT DO NOTHING;
```

**Aplicar para 098:** mesmo padrão, acrescentando coluna `uf_estado`:
```sql
-- 098_seed_ba_ce_fronteira.sql
-- Seed regras BA e CE — deve rodar APÓS 097 (que adiciona uf_estado e recria constraint)
INSERT INTO icms_fronteira_regras_ncm
    (ncm_prefixo, descricao, regime, aliquota_interna, mva_original, uf_estado)
VALUES
    ('2202', 'Refrigerantes e bebidas não alcoólicas', 'ST', 26.0, 140.00, 'BA'),
    ('2203', 'Cervejas de malte',                      'ST', 26.0, 140.00, 'BA'),
    ...
ON CONFLICT DO NOTHING;

INSERT INTO icms_fronteira_regras_ncm
    (ncm_prefixo, descricao, regime, aliquota_interna, mva_original, uf_estado)
VALUES
    ('2202', 'Refrigerantes e bebidas não alcoólicas', 'ST', 25.0, 140.00, 'CE'),
    ...
ON CONFLICT DO NOTHING;
```

**ATENÇÃO:** Os valores BA/CE do RESEARCH.md são `[ASSUMED]` — devem ser revisados contra RICMS/BA e RICMS/CE antes da execução em produção.

---

### `backend/handlers/environment.go` — struct Company + CreateCompanyHandler + UpdateCompanyHandler (modificação)

**Análogo:** si mesmo — padrão atual extraído diretamente do arquivo

**Struct Company atual** (linhas 28-36):
```go
type Company struct {
    ID      string `json:"id"`
    GroupID string `json:"group_id"`
    // CNPJ      string `json:"cnpj"` // Deprecated
    Name              string `json:"name"`
    TradeName         string `json:"trade_name"`
    RegimeTributario  string `json:"regime_tributario"`
    CreatedAt         string `json:"created_at"`
}
```

**Struct Company expandido (CADU-01/02):**
```go
type Company struct {
    ID                string           `json:"id"`
    GroupID           string           `json:"group_id"`
    Name              string           `json:"name"`
    TradeName         string           `json:"trade_name"`
    RegimeTributario  string           `json:"regime_tributario"`
    CNPJ              string           `json:"cnpj,omitempty"`
    InscricaoEstadual string           `json:"inscricao_estadual,omitempty"`
    CNAEPrincipal     string           `json:"cnae_principal,omitempty"`
    CNAESecundario    []string         `json:"cnae_secundario,omitempty"`
    Municipio         string           `json:"municipio,omitempty"`
    SegmentoEconomico string           `json:"segmento_economico,omitempty"`
    IncentivosFiscais *json.RawMessage `json:"incentivos_fiscais,omitempty"`
    CreatedAt         string           `json:"created_at"`
}
```

**Necessário adicionar `"encoding/json"` no import block** (linha 6 atual):
```go
import (
    "database/sql"
    "encoding/json"   // ← adicionar
    "log"
    "net/http"
    "regexp"          // ← adicionar para validação CNPJ

    "github.com/golang-jwt/jwt/v5"
)
```

**Padrão GetCompaniesHandler — scan atual** (linhas 234-267):
```go
query := "SELECT id, group_id, name, COALESCE(trade_name, ''), COALESCE(regime_tributario, 'nao_informado'), created_at FROM companies"
// ...
rows.Scan(&c.ID, &c.GroupID, &c.Name, &c.TradeName, &c.RegimeTributario, &c.CreatedAt)
```
Expandir para incluir `COALESCE(cnpj,''), COALESCE(inscricao_estadual,''), ...` no SELECT e adicionar campos correspondentes no Scan.

**UpdateCompanyHandler atual — struct anônimo restrito** (linhas 317-358):
```go
var payload struct {
    RegimeTributario string `json:"regime_tributario"`
}
// ...
_, err := db.Exec(
    "UPDATE companies SET regime_tributario = $1, updated_at = NOW() WHERE id = $2",
    payload.RegimeTributario, id,
)
```

**UpdateCompanyHandler expandido (CADU-02) — substituir struct anônimo + query:**
```go
var payload struct {
    RegimeTributario  string   `json:"regime_tributario"`
    CNPJ              string   `json:"cnpj"`
    InscricaoEstadual string   `json:"inscricao_estadual"`
    CNAEPrincipal     string   `json:"cnae_principal"`
    CNAESecundario    []string `json:"cnae_secundario"`
    Municipio         string   `json:"municipio"`
    SegmentoEconomico string   `json:"segmento_economico"`
}
if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
    http.Error(w, err.Error(), http.StatusBadRequest)
    return
}
// Validar CNPJ: 14 dígitos numéricos quando fornecido
if payload.CNPJ != "" {
    re := regexp.MustCompile(`^\d{14}$`)
    if !re.MatchString(payload.CNPJ) {
        http.Error(w, "CNPJ deve ter 14 dígitos numéricos", http.StatusBadRequest)
        return
    }
}
_, err := db.Exec(`
    UPDATE companies SET
        regime_tributario  = $1,
        cnpj               = NULLIF($2, ''),
        inscricao_estadual = NULLIF($3, ''),
        cnae_principal     = NULLIF($4, ''),
        municipio          = NULLIF($5, ''),
        segmento_economico = NULLIF($6, ''),
        updated_at         = NOW()
    WHERE id = $7
`, payload.RegimeTributario, payload.CNPJ, payload.InscricaoEstadual,
   payload.CNAEPrincipal, payload.Municipio, payload.SegmentoEconomico, id)
```

**Padrão de validação whitelist existente** (linhas 338-345 do UpdateCompanyHandler atual):
```go
allowed := map[string]bool{
    "lucro_real": true, "lucro_presumido": true,
    "simples_nacional": true, "nao_informado": true,
}
if !allowed[payload.RegimeTributario] {
    http.Error(w, "regime_tributario inválido", http.StatusBadRequest)
    return
}
```
Manter este bloco de validação de regime_tributario.

---

### `backend/handlers/icms_fronteira_regras.go` — uf_estado em List/Create/Delete/Importar + novo UpdateHandler (modificação)

**Análogo:** si mesmo — padrão atual extraído diretamente do arquivo

**Padrão GetEffectiveCompanyID** (linhas 58-62, repetido em 135-139, 232-236, 285-289):
```go
companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
if err != nil {
    jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa: "+err.Error())
    return
}
```
**OBRIGATÓRIO** em todos os handlers novos/modificados.

**Struct FronteiraRegraRow atual** (linhas 22-31):
```go
type FronteiraRegraRow struct {
    ID             string   `json:"id"`
    NCMPrefixo     string   `json:"ncm_prefixo"`
    Descricao      string   `json:"descricao"`
    Regime         string   `json:"regime"`
    AliquotaInterna float64 `json:"aliquota_interna"`
    MVAOriginal    *float64 `json:"mva_original"`
    ReducaoBCPct   float64  `json:"reducao_bc_pct"`
    IsGlobal       bool     `json:"is_global"`
}
```

**Struct expandido com uf_estado e MVA ajustado:**
```go
type FronteiraRegraRow struct {
    ID               string   `json:"id"`
    NCMPrefixo       string   `json:"ncm_prefixo"`
    Descricao        string   `json:"descricao"`
    Regime           string   `json:"regime"`
    AliquotaInterna  float64  `json:"aliquota_interna"`
    MVAOriginal      *float64 `json:"mva_original"`
    MVAAjustado4pct  *float64 `json:"mva_ajustado_4pct"`
    MVAAjustado7pct  *float64 `json:"mva_ajustado_7pct"`
    MVAAjustado12pct *float64 `json:"mva_ajustado_12pct"`
    ReducaoBCPct     float64  `json:"reducao_bc_pct"`
    UFEstado         string   `json:"uf_estado"`
    IsGlobal         bool     `json:"is_global"`
}
```

**Padrão de filtro por query param + whitelist (CADU-06):**
```go
// Adicionar no início de IcmsFronteiraRegrasListHandler, após GetEffectiveCompanyID:
ufEstado := r.URL.Query().Get("uf_estado")
if ufEstado == "" {
    ufEstado = "PE" // default — preserva compatibilidade com frontend atual
}
validUFs := map[string]bool{"PE": true, "BA": true, "CE": true}
if !validUFs[ufEstado] {
    jsonErr(w, http.StatusBadRequest, "uf_estado inválido: deve ser PE, BA ou CE")
    return
}
// Usar ufEstado como $2 na query:
rows, err := db.Query(`
    SELECT id::text, ncm_prefixo, COALESCE(descricao,''), COALESCE(regime,'ST'),
           COALESCE(aliquota_interna, 20.5), mva_original,
           mva_ajustado_4pct, mva_ajustado_7pct, mva_ajustado_12pct,
           COALESCE(reducao_bc_pct, 0), uf_estado, (company_id IS NULL)
    FROM icms_fronteira_regras_ncm
    WHERE (company_id = $1 OR company_id IS NULL)
      AND uf_estado = $2
    ORDER BY ncm_prefixo
`, companyID, ufEstado)
```

**Padrão sql.NullFloat64 para campos nullable** (linhas 88-104 do List handler):
```go
var mva sql.NullFloat64
if err := rows.Scan(
    &row.ID, &row.NCMPrefixo, &row.Descricao, &row.Regime,
    &row.AliquotaInterna, &mva,
    &row.ReducaoBCPct, &row.IsGlobal,
); err != nil { ... }
if mva.Valid {
    row.MVAOriginal = &mva.Float64
}
```
Repetir para os três campos `mva_ajustado_*` usando `sql.NullFloat64` individuais.

**Padrão ON CONFLICT na query de Create** (linhas 175-199):
```go
err = db.QueryRow(`
    INSERT INTO icms_fronteira_regras_ncm
        (company_id, ncm_prefixo, descricao, regime, aliquota_interna, mva_original, reducao_bc_pct)
    VALUES ($1, $2, $3, $4, $5, $6, $7)
    ON CONFLICT (company_id, ncm_prefixo) DO UPDATE
        SET descricao = EXCLUDED.descricao, ...
    RETURNING ...
`, ...)
```
Após migration 097, o ON CONFLICT deve ser `ON CONFLICT (company_id, ncm_prefixo, uf_estado)` e incluir `uf_estado` nos campos do INSERT.

**Novo IcmsFronteiraRegraUpdateHandler (CADU-06) — copiar estrutura do DeleteHandler:**
```go
// Padrão: extrair ID do path, validar companyID, executar UPDATE
func IcmsFronteiraRegraUpdateHandler(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Mesmo padrão de auth + GetEffectiveCompanyID do Delete (linhas 219-243)
        id := strings.TrimPrefix(r.URL.Path, "/api/icms-fronteira/regras/")
        id = strings.TrimSpace(id)
        if id == "" {
            jsonErr(w, http.StatusBadRequest, "ID não informado")
            return
        }
        // Decode body, validar, executar UPDATE WHERE id=$1 AND company_id=$2
    }
}
```

**Padrão Importar — uf_estado como campo de formulário:**
```go
// No IcmsFronteiraRegrasImportarHandler, após ParseMultipartForm:
ufEstado := r.FormValue("uf_estado")
if ufEstado == "" {
    ufEstado = "PE"
}
validUFs := map[string]bool{"PE": true, "BA": true, "CE": true}
if !validUFs[ufEstado] {
    jsonErr(w, http.StatusBadRequest, "uf_estado inválido")
    return
}
// Usar ufEstado em todos os INSERTs dentro do loop de registros
```

---

### `backend/main.go` — nova rota PUT/PATCH para regras (modificação)

**Análogo:** bloco `/api/icms-fronteira/regras/` (linhas 686-689 de main.go) e bloco `/api/icms-fronteira/regras` (linhas 691-702):

**Padrão atual do bloco regras/** (linhas 686-689):
```go
http.HandleFunc("/api/icms-fronteira/regras/", func(w http.ResponseWriter, r *http.Request) {
    database := getDB()
    if database == nil { jsonServiceUnavailable(w); return }
    handlers.AuthMiddleware(handlers.IcmsFronteiraRegraDeleteHandler(database), "")(w, r)
})
```

**Expandir para suportar PUT (update) além de DELETE:**
```go
http.HandleFunc("/api/icms-fronteira/regras/", func(w http.ResponseWriter, r *http.Request) {
    database := getDB()
    if database == nil { jsonServiceUnavailable(w); return }
    switch r.Method {
    case http.MethodDelete:
        handlers.AuthMiddleware(handlers.IcmsFronteiraRegraDeleteHandler(database), "")(w, r)
    case http.MethodPut, http.MethodPatch:
        handlers.AuthMiddleware(handlers.IcmsFronteiraRegraUpdateHandler(database), "")(w, r)
    default:
        w.WriteHeader(http.StatusMethodNotAllowed)
    }
})
```

**Padrão switch Method** (análogo em `/api/config/companies` e `/api/icms-fronteira/regras` linhas 691-702):
```go
switch r.Method {
case "GET":
    handlers.AuthMiddleware(handlers.IcmsFronteiraRegrasListHandler(database), "")(w, r)
case "POST":
    handlers.AuthMiddleware(handlers.IcmsFronteiraRegraCreateHandler(database), "")(w, r)
default:
    w.WriteHeader(http.StatusMethodNotAllowed)
}
```

---

### `frontend/src/pages/GestaoAmbiente.tsx` — modal empresa expandido (modificação)

**Análogo:** si mesmo — padrão atual extraído diretamente do arquivo

**Interface Company atual** (linhas 54-62):
```typescript
interface Company {
  id: string;
  group_id: string;
  cnpj: string;          // ← já existe; apenas adicionar campos novos
  name: string;
  trade_name: string;
  regime_tributario: string;
  created_at: string;
}
```

**Expandir interface para novos campos (todos opcionais):**
```typescript
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
```

**Padrão de estado de formulário** (linhas 93-96 — useState simples, sem react-hook-form):
```typescript
const [newCompanyCNPJ, setNewCompanyCNPJ] = useState("");
const [newCompanyName, setNewCompanyName] = useState("");
const [newCompanyTradeName, setNewCompanyTradeName] = useState("");
const [newCompanyRegime, setNewCompanyRegime] = useState("lucro_real");
```
Adicionar um `useState` por campo novo: `newCompanyIE`, `newCompanyCNAE`, `newCompanyMunicipio`, `newCompanySegmento`.

**Padrão Dialog + Form** (linhas 617-664 — modal de Nova Empresa):
```typescript
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
      ...
    </div>
    <DialogFooter>
      <Button onClick={handleCreateCompany}>Cadastrar</Button>
    </DialogFooter>
  </DialogContent>
</Dialog>
```
Adicionar campos novos dentro do mesmo `<div className="space-y-4 py-4">`. Usar `maxLength={14}` no campo CNPJ (sem máscara — apenas números).

**Padrão de edição inline (editingCompany)** (linhas 723-748):
```typescript
{editingCompany?.id === company.id && (
  <div className="mt-2 p-3 border border-blue-200 rounded-md bg-blue-50">
    <p className="text-xs font-medium text-blue-800 mb-2">Alterar Regime Tributário</p>
    <Select value={editRegime} onValueChange={setEditRegime}>...</Select>
    <div className="flex gap-2 mt-2">
      <Button size="sm" className="h-7 text-xs flex-1" onClick={handleUpdateCompanyRegime}>Salvar</Button>
      <Button size="sm" variant="outline" className="h-7 text-xs" onClick={() => setEditingCompany(null)}>Cancelar</Button>
    </div>
  </div>
)}
```
Expandir este bloco inline para incluir campos adicionais (CNPJ, IE, CNAE, Município, Segmento) além do regime_tributario.

**Padrão PATCH para update** (linhas 335-350):
```typescript
const handleUpdateCompanyRegime = async () => {
  if (!editingCompany) return;
  try {
    const res = await fetch(`/api/config/companies?id=${editingCompany.id}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ regime_tributario: editRegime }),
    });
    if (!res.ok) throw new Error("Failed to update");
    toast.success("Regime atualizado");
    setEditingCompany(null);
    if (selectedGroup) fetchCompanies(selectedGroup.id);
  } catch {
    toast.error("Erro ao atualizar regime");
  }
};
```
Renomear para `handleUpdateCompany` e expandir o body do JSON para incluir todos os campos novos.

---

### `frontend/src/pages/IcmsFronteira.tsx` — abas PE/BA/CE dentro de RegrasTab (modificação)

**Análogo:** si mesmo — padrão `RegrasTab` (linhas 617-944) e estrutura de Tabs externa (linhas 2273-2285)

**Padrão useQuery com queryKey incluindo UF** (linhas 632-641):
```typescript
const { data, isLoading, isError } = useQuery<RegrasResponse>({
  queryKey: ['icms-fronteira/regras'],   // ← adicionar selectedUF: ['icms-fronteira/regras', selectedUF]
  queryFn: async () => {
    const res = await fetch('/api/icms-fronteira/regras', {  // ← adicionar ?uf_estado=${selectedUF}
      headers: { Authorization: `Bearer ${token}` },
    })
    if (!res.ok) throw new Error(`Erro ${res.status}`)
    return res.json()
  },
})
```

**Padrão Tabs aninhadas para UF** (baseado no padrão da estrutura principal linhas 2273-2285):
```typescript
// Dentro de RegrasTab, antes do render da tabela:
const [selectedUF, setSelectedUF] = useState<'PE' | 'BA' | 'CE'>('PE')

<Tabs value={selectedUF} onValueChange={(v) => setSelectedUF(v as 'PE' | 'BA' | 'CE')}>
  <TabsList>
    <TabsTrigger value="PE">PE — Pernambuco</TabsTrigger>
    <TabsTrigger value="BA">BA — Bahia</TabsTrigger>
    <TabsTrigger value="CE">CE — Ceará</TabsTrigger>
  </TabsList>
  <TabsContent value="PE">
    {/* tabela + botões existentes, filtrados para PE */}
  </TabsContent>
  <TabsContent value="BA">
    {/* mesma estrutura */}
  </TabsContent>
  <TabsContent value="CE">
    {/* mesma estrutura */}
  </TabsContent>
</Tabs>
```

**Padrão useMutation para create** (linhas 643-663):
```typescript
const createMutation = useMutation({
  mutationFn: async (body: object) => {
    const res = await fetch('/api/icms-fronteira/regras', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
      body: JSON.stringify(body),
    })
    if (!res.ok) throw new Error(`Erro ${res.status}`)
    return res.json()
  },
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['icms-fronteira/regras'] })
    toast.success('Regra criada com sucesso')
    ...
  },
  onError: () => toast.error('Erro ao criar regra'),
})
```
Ao criar regra, incluir `uf_estado: selectedUF` no body.

**Padrão import de arquivo + FormData** (linhas 700-718):
```typescript
async function handleImport() {
  if (!importFile) return
  setImportLoading(true)
  try {
    const fd = new FormData()
    fd.append('file', importFile)
    const res = await fetch('/api/icms-fronteira/regras/importar', {
      method: 'POST',
      headers: { Authorization: `Bearer ${token}` },
      body: fd,
    })
    ...
    toast.success(`Importadas: ${result.imported}, ignoradas: ${result.skipped}`)
  } ...
}
```
Adicionar `fd.append('uf_estado', selectedUF)` antes do fetch.

**Interface RegraNCM — adicionar campos novos:**
```typescript
interface RegraNCM {
  id: number
  ncm_prefixo: string
  descricao: string
  regime: string
  aliquota_interna: number
  mva_original: number | null
  mva_ajustado_4pct: number | null   // ← novo
  mva_ajustado_7pct: number | null   // ← novo
  mva_ajustado_12pct: number | null  // ← novo
  reducao_bc_pct: number
  uf_estado: string                  // ← novo
  is_global: boolean
}
```

---

### `frontend/src/lib/navigation.ts` — renomear label fronteira (modificação)

**Análogo:** si mesmo — bloco `fronteira` (linhas 59-70)

**Estado atual** (linha 60):
```typescript
fronteira: {
  label: 'ICMS Fronteira — PE',   // ← somente PE
  ...
}
```

**Modificação mínima:**
```typescript
fronteira: {
  label: 'ICMS Fronteira',        // ← sem UF específica (agora multi-UF)
  ...
}
```
Nenhuma aba nova no nível de navigation.ts — as abas de UF são tabs aninhadas dentro do componente `IcmsFronteira.tsx`.

---

## Shared Patterns

### Autenticação + Empresa — Backend

**Fonte:** `backend/handlers/icms_fronteira_regras.go` (linhas 51-62, padrão repetido em todos os handlers)
**Aplicar a:** todos os handlers novos/modificados em `environment.go` e `icms_fronteira_regras.go`

```go
claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
if !ok {
    jsonErr(w, http.StatusUnauthorized, "Unauthorized")
    return
}
userID, _ := claims["user_id"].(string)

companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
if err != nil {
    jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa: "+err.Error())
    return
}
```

### Tratamento de erro JSON — Backend

**Fonte:** `backend/handlers/icms_fronteira_regras.go` — uso de `jsonErr(w, statusCode, message)` em todos os handlers
**Aplicar a:** todos os novos handlers (nunca usar `http.Error` em handlers que já definiram `Content-Type: application/json`)

```go
w.Header().Set("Content-Type", "application/json")
// ...
jsonErr(w, http.StatusBadRequest, "mensagem descritiva em português")
```

### Whitelist de valores — Backend

**Fonte:** `backend/handlers/environment.go` linhas 338-345 (validação regime) e padrão de uf_estado no RESEARCH.md
**Aplicar a:** qualquer campo que aceite conjunto finito de valores (regime_tributario, uf_estado)

```go
allowed := map[string]bool{"PE": true, "BA": true, "CE": true}
if !allowed[value] {
    http.Error(w, "valor inválido", http.StatusBadRequest)
    return
}
```

### Toast feedback — Frontend

**Fonte:** `frontend/src/pages/GestaoAmbiente.tsx` (linhas 221, 258, 290, 313, 328, 343, 360, 364) e `IcmsFronteira.tsx`
**Aplicar a:** todas as ações CRUD no frontend

```typescript
toast.success("mensagem de sucesso")
toast.error("mensagem de erro")
```

### Invalidação de query após mutação — Frontend

**Fonte:** `frontend/src/pages/IcmsFronteira.tsx` linhas 657, 674
**Aplicar a:** `onSuccess` de todos os `useMutation`

```typescript
onSuccess: () => {
  queryClient.invalidateQueries({ queryKey: ['icms-fronteira/regras', selectedUF] })
  toast.success('...')
}
```

### Fetch com Authorization Bearer — Frontend

**Fonte:** `frontend/src/pages/IcmsFronteira.tsx` — todas as funções de fetch usam `{ token }` de `useAuth()`
**Aplicar a:** todos os fetches em `GestaoAmbiente.tsx` (atualmente sem header; se autenticação for necessária, seguir o padrão de IcmsFronteira)

```typescript
const { token } = useAuth()
// ...
headers: { Authorization: `Bearer ${token}` }
```

---

## Sem Análogo

Todos os arquivos desta fase têm análogos diretos no codebase. Nenhum arquivo sem correspondência.

---

## Metadata

**Escopo de busca de análogos:**
- `backend/handlers/` — todos os arquivos `.go`
- `backend/migrations/` — migrations 077, 091 (mais próximas por função)
- `frontend/src/pages/GestaoAmbiente.tsx` e `IcmsFronteira.tsx`
- `frontend/src/lib/navigation.ts`

**Arquivos lidos:** 8 arquivos de código-fonte + RESEARCH.md + REQUIREMENTS.md
**Data de mapeamento:** 2026-05-23
