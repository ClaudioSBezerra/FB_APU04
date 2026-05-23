# Phase 8: Cadastro de Empresas + Ambiente Administrativo por UF — Research

**Pesquisado:** 2026-05-23
**Domínio:** PostgreSQL migrations, Go handlers, React/TypeScript frontend, ICMS Fronteira por UF
**Confiança:** HIGH — baseado inteiramente em leitura direta do código-fonte do projeto

---

<phase_requirements>
## Phase Requirements

| ID | Descrição | Suporte de Pesquisa |
|----|-----------|---------------------|
| CADU-01 | Migration tabela `companies`: adicionar cnpj, inscricao_estadual, cnae_principal, cnae_secundario, municipio, segmento_economico, incentivos_fiscais | Esquema atual auditado em migrations 013/023/077; próxima migration deve ser 096 |
| CADU-02 | Backend: atualizar struct `Company`, `CreateCompanyHandler` e `UpdateCompanyHandler` com novos campos e validação de CNPJ | Código atual em `backend/handlers/environment.go` mapeado integralmente |
| CADU-03 | Frontend: tela de cadastro/edição de empresa com todos os novos campos | Padrão Dialog/Form de `GestaoAmbiente.tsx` e `IcmsFronteira.tsx` identificado |
| CADU-04 | Migration `icms_fronteira_regras_ncm`: adicionar uf_estado; criar `icms_fronteira_inaplicabilidades`; adicionar mva_ajustado_4/7/12pct | Esquema 091 mapeado; constraint UNIQUE atual deve ser expandida para incluir uf_estado |
| CADU-05 | Seed BA e CE: NCMs mais frequentes com alíquotas internas e MVAs estaduais | Valores de referência documentados nesta pesquisa com fonte [ASSUMED] |
| CADU-06 | Backend: handlers CRUD de regras por UF + upload planilha por UF; rotas em main.go | Padrão `IcmsFronteiraRegrasImportarHandler` e rotas GET/POST/DELETE já mapeadas |
| CADU-07 | Frontend: tela de configuração ICMS-Fronteira com abas PE/BA/CE, edição inline, upload planilha | Padrão `Tabs` do IcmsFronteira.tsx identificado; módulo `fronteira` em navigation.ts mapeado |
</phase_requirements>

---

## Resumo

Esta fase tem três funcionalidades independentes que podem ser planejadas em waves distintos: (1) completar o cadastro de empresas, (2) expandir o ambiente administrativo ICMS-Fronteira para múltiplas UFs, e (3) criar uma tela de gestão multi-empresa. As três se sobrepõem no mesmo conjunto de arquivos — `environment.go`, `GestaoAmbiente.tsx`, `navigation.ts`, `App.tsx` e as migrations — portanto o plano deve serializar as waves para evitar conflitos de merge.

O código existente já tem todo o scaffolding necessário: rota `/api/config/companies` (GET/POST/PUT/PATCH/DELETE), handlers `GetCompaniesHandler`, `CreateCompanyHandler`, `UpdateCompanyHandler`, `DeleteCompanyHandler`, a página `GestaoAmbiente.tsx` com modal de criação de empresa, e os handlers `IcmsFronteiraRegrasList/Create/Delete/Importar` com suporte a CSV e XLSX. Nenhuma nova dependência de biblioteca é necessária — `excelize/v2` já está importado no handler de regras.

A principal armadilha é a constraint UNIQUE atual em `icms_fronteira_regras_ncm`: `UNIQUE NULLS NOT DISTINCT (company_id, ncm_prefixo)`. Ela precisará ser substituída por `UNIQUE NULLS NOT DISTINCT (company_id, ncm_prefixo, uf_estado)` para suportar a mesma regra NCM em UFs diferentes. A migration deve fazer DROP da constraint antiga e ADD da nova.

**Recomendação primária:** Wave 1 = migrations (096 para companies + 097 para fronteira UF + 098 seed BA/CE). Wave 2 = backend (Company struct + handlers + novas rotas por UF). Wave 3 = frontend (GestaoAmbiente expand + IcmsFronteira abas UF + navigation).

---

## Architectural Responsibility Map

| Capability | Tier Primário | Tier Secundário | Rationale |
|------------|--------------|-----------------|-----------|
| Novos campos companies (cnpj, IE, CNAE etc.) | Database / Storage | API Backend | Campos de cadastro mestre — residem em PostgreSQL, expostos via API REST |
| Validação de CNPJ (14 dígitos) | API Backend | — | Regra de negócio fiscal; nunca delegar ao frontend exclusivamente |
| CRUD de empresas (criação/edição/exclusão) | API Backend | Browser/Client | Handlers Go existentes; frontend é UI sobre essa API |
| Regras ICMS por UF | Database / Storage | API Backend | Nova coluna `uf_estado` na tabela; query filtra por UF |
| MVA ajustado (4%/7%/12%) | Database / Storage | API Backend | Colunas numéricas na tabela; cálculo pode usar diretamente no handler fronteira |
| Tabela inaplicabilidades | Database / Storage | API Backend | Nova tabela; lida pelo handler que calcula ICMS-Fronteira |
| Upload planilha regras por UF | API Backend | Browser/Client | Handler Go existente `IcmsFronteiraRegrasImportarHandler` precisa de parâmetro uf_estado |
| Abas PE/BA/CE na UI | Browser/Client | — | Componente React puro; não impacta backend |
| Tela gestão multi-empresa | Browser/Client | API Backend | Expansão de `GestaoAmbiente.tsx`; backend já pronto via endpoint existente |

---

## Standard Stack

### Core (tudo já presente no projeto)

| Biblioteca | Versão | Propósito | Por que é o padrão |
|------------|--------|-----------|---------------------|
| Go stdlib `database/sql` | Go 1.24 | Queries PostgreSQL | Padrão do projeto — todos os handlers usam [VERIFIED: codebase grep] |
| `github.com/xuri/excelize/v2` | v2.x | Parse XLSX | Já importado em `icms_fronteira_regras.go` [VERIFIED: codebase grep] |
| React 18 + TypeScript | 18.x | Frontend SPA | Padrão do projeto [VERIFIED: codebase grep] |
| `@tanstack/react-query` | v5 | Fetch/cache data | Usado em `IcmsFronteira.tsx` e outros [VERIFIED: codebase grep] |
| `@radix-ui/react-tabs` (via shadcn) | — | Tabs UI | Usado em `IcmsFronteira.tsx` (`Tabs, TabsContent, TabsList, TabsTrigger`) [VERIFIED: codebase grep] |
| `@radix-ui/react-dialog` (via shadcn) | — | Modais | Usado em `GestaoAmbiente.tsx` [VERIFIED: codebase grep] |
| `sonner` | — | Toasts | `toast.success/error` em todos os handlers frontend [VERIFIED: codebase grep] |
| `lucide-react` | — | Ícones | Padrão do projeto [VERIFIED: codebase grep] |

**Sem novas dependências a instalar.** Todas as bibliotecas necessárias já estão no projeto.

---

## Package Legitimacy Audit

> Nenhum novo pacote externo a ser instalado nesta fase. Todas as dependências já constam no `go.mod` e `package.json` existentes.

| Pacote | Registro | Status | Disposição |
|--------|----------|--------|-----------|
| `excelize/v2` | Go modules | Já instalado | Aprovado — em uso |
| shadcn/ui Tabs | npm (bundled) | Já instalado | Aprovado — em uso |

**Pacotes removidos por slopcheck:** nenhum
**Pacotes suspeitos:** nenhum

---

## Estado Atual Detalhado do Código

### 1. Tabela `companies` — Schema Real (após migrations aplicadas)

A migration 013 criou `companies` com `cnpj VARCHAR(14) NOT NULL UNIQUE`. Migrations subsequentes removeram essa estrutura:

- **014**: `ALTER TABLE companies DROP CONSTRAINT ... cnpj NOT NULL` (torna opcional)
- **023**: `ALTER TABLE companies DROP COLUMN IF EXISTS cnpj` — **CNPJ foi removido**
- **077**: `ADD COLUMN IF NOT EXISTS regime_tributario TEXT NOT NULL DEFAULT 'nao_informado'`

**Schema real atual da tabela `companies`:**
```sql
id               UUID        PK
group_id         UUID        FK enterprise_groups
name             VARCHAR(255) NOT NULL
trade_name       VARCHAR(255)
owner_id         UUID        FK users  (migration 017)
regime_tributario TEXT        NOT NULL DEFAULT 'nao_informado'
created_at       TIMESTAMPTZ
updated_at       TIMESTAMPTZ
```

CNPJ não existe mais. O comentário `// CNPJ      string \`json:"cnpj"\` // Deprecated` no struct Go confirma isso. [VERIFIED: codebase grep das migrations]

### 2. Struct `Company` em Go — Estado Atual

```go
type Company struct {
    ID               string `json:"id"`
    GroupID          string `json:"group_id"`
    Name             string `json:"name"`
    TradeName        string `json:"trade_name"`
    RegimeTributario string `json:"regime_tributario"`
    CreatedAt        string `json:"created_at"`
}
```

`UpdateCompanyHandler` aceita **somente** `regime_tributario`. `CreateCompanyHandler` aceita `name`, `trade_name`, `group_id`, `regime_tributario`. Todos os outros campos serão adicionados. [VERIFIED: leitura direta de environment.go]

### 3. Tabela `icms_fronteira_regras_ncm` — Schema Atual

```sql
id               UUID        PK
company_id       UUID        (NULL = global)
ncm_prefixo      VARCHAR(8)  NOT NULL
descricao        TEXT        NOT NULL
regime           VARCHAR(20) CHECK (ST|ANTECIPACAO|DIFAL|ISENTO|NORMAL)
aliquota_interna NUMERIC(5,2) DEFAULT 20.5
mva_original     NUMERIC(7,2)
reducao_bc_pct   NUMERIC(5,2) DEFAULT 0
created_at       TIMESTAMPTZ

CONSTRAINT uq_icms_fronteira_regras UNIQUE NULLS NOT DISTINCT (company_id, ncm_prefixo)
```

**Não existe** coluna `uf_estado` nem `mva_ajustado_*pct`. [VERIFIED: leitura direta de 091_icms_fronteira.sql]

### 4. Rotas Já Registradas em main.go (ICMS Fronteira Regras)

```
GET  /api/icms-fronteira/regras          → IcmsFronteiraRegrasListHandler
POST /api/icms-fronteira/regras          → IcmsFronteiraRegraCreateHandler
POST /api/icms-fronteira/regras/importar → IcmsFronteiraRegrasImportarHandler
DELETE /api/icms-fronteira/regras/{id}   → IcmsFronteiraRegraDeleteHandler
```

As novas rotas para CRUD por UF precisarão de parâmetro `?uf_estado=PE|BA|CE` nas existentes (filtro de query), não de novas rotas separadas — exceto UPDATE que não existe ainda. [VERIFIED: leitura direta de main.go]

### 5. Módulo `fronteira` em navigation.ts

```typescript
fronteira: {
  label: 'ICMS Fronteira — PE',   // ← precisa mudar para 'ICMS Fronteira'
  tabs: [
    { label: 'Resumo',             path: '/icms-fronteira' },
    ...
    { label: 'Regras NCM',         path: '/icms-fronteira/regras' },
    ...
  ],
},
```

A aba de Regras NCM já existe. Para abas por UF, o approach correto é manter `/icms-fronteira/regras` e usar tabs internas dentro do componente IcmsFronteira (como já faz para Resumo/Antecipação/ST/DIFAL). [VERIFIED: leitura direta de navigation.ts]

---

## Architecture Patterns

### Diagrama de Fluxo

```
[Frontend GestaoAmbiente.tsx]
       |
       | PATCH /api/config/companies?id=UUID
       v
[UpdateCompanyHandler] → PostgreSQL companies (novos campos)
       |
       | query novos campos
       v
[GetCompaniesHandler]  → retorna Company com cnpj, IE, CNAE etc.
       |
       v
[Frontend GestaoAmbiente.tsx modal de edição expandido]


[Frontend IcmsFronteira.tsx — aba Regras NCM]
       |
       | GET /api/icms-fronteira/regras?uf_estado=BA
       v
[IcmsFronteiraRegrasListHandler] → filtra por uf_estado
       |
       v
[Frontend tabela de regras filtrada por UF]
       |
       | POST /api/icms-fronteira/regras (body: {uf_estado: "BA", ...})
       v
[IcmsFronteiraRegraCreateHandler] → INSERT com uf_estado
```

### Estrutura de Arquivos Impactados

```
backend/
├── handlers/
│   └── environment.go           ← Company struct + Create/Update handlers
│   └── icms_fronteira_regras.go ← List/Create/Delete/Importar + uf_estado
├── migrations/
│   └── 096_add_fields_to_companies.sql
│   └── 097_add_uf_estado_to_fronteira_regras.sql
│   └── 098_seed_ba_ce_fronteira.sql
└── main.go                      ← nova rota PUT /api/icms-fronteira/regras/{id}

frontend/src/
├── pages/
│   ├── GestaoAmbiente.tsx       ← expandir modal empresa + edição inline
│   └── IcmsFronteira.tsx        ← abas PE/BA/CE dentro da aba Regras NCM
├── lib/
│   └── navigation.ts            ← renomear label 'ICMS Fronteira — PE'
└── App.tsx                      ← nenhuma nova rota necessária (IcmsFronteira já cobre tudo)
```

### Padrão 1: Migration Idempotente com IF NOT EXISTS

Todas as migrations do projeto usam `ADD COLUMN IF NOT EXISTS` e blocos `DO $$ BEGIN ... EXCEPTION WHEN duplicate_object THEN NULL; END $$;` para constraints. Seguir o mesmo padrão. [VERIFIED: migrations 077, 086, 087]

```sql
-- Source: migrations/077_add_regime_tributario_to_companies.sql
ALTER TABLE companies
    ADD COLUMN IF NOT EXISTS regime_tributario TEXT NOT NULL DEFAULT 'nao_informado';

DO $$ BEGIN
    ALTER TABLE companies ADD CONSTRAINT chk_companies_regime_tributario
        CHECK (...);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;
```

### Padrão 2: Handler Go com GetEffectiveCompanyID

Todos os handlers que acessam dados por empresa usam `GetEffectiveCompanyID` para resolver o company_id a partir do JWT + header `X-Company-ID`. Manter este padrão nos novos handlers. [VERIFIED: icms_fronteira_regras.go]

```go
// Source: backend/handlers/icms_fronteira_regras.go
companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
if err != nil {
    jsonErr(w, http.StatusInternalServerError, "Erro ao obter empresa: "+err.Error())
    return
}
```

### Padrão 3: CRUD Company com withAuth no main.go

Rotas de companies usam o padrão de switch por Method dentro de withAuth:

```go
// Source: backend/main.go
http.HandleFunc("/api/config/companies", withAuth(func(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        switch r.Method {
        case http.MethodGet:    handlers.GetCompaniesHandler(db)(w, r)
        case http.MethodPost:   handlers.CreateCompanyHandler(db)(w, r)
        case http.MethodPut, http.MethodPatch:  handlers.UpdateCompanyHandler(db)(w, r)
        case http.MethodDelete: handlers.DeleteCompanyHandler(db)(w, r)
        }
    }
}, ""))
```

`UpdateCompanyHandler` já está registrado para PUT/PATCH — basta expandir os campos que ele aceita.

### Padrão 4: Tabs dentro de IcmsFronteira.tsx

O componente `IcmsFronteira.tsx` já usa `<Tabs>` do shadcn para as abas internas (Resumo, Antecipação, ST, DIFAL, Planilha Itens, Divergências, Regras NCM, Extrato, Contestações). As abas de UF (PE/BA/CE) devem ser **tabs aninhadas dentro da aba "Regras NCM"**, não novas tabs de módulo na barra superior. [VERIFIED: leitura de IcmsFronteira.tsx e navigation.ts]

### Padrão 5: useQuery para fetch + useMutation para mutações

```typescript
// Source: frontend/src/pages/IcmsFronteira.tsx
const { data: regrasData, isLoading: regrasLoading, refetch: refetchRegras } =
  useQuery<RegrasResponse>({
    queryKey: ['fronteira-regras', companyId],
    queryFn: () => fetch('/api/icms-fronteira/regras', { headers: buildHeaders() })
      .then(r => r.json()),
    enabled: !!companyId,
  })
```

Para regras por UF, a `queryKey` deve incluir `uf_estado`:
```typescript
queryKey: ['fronteira-regras', companyId, selectedUF],
queryFn: () => fetch(`/api/icms-fronteira/regras?uf_estado=${selectedUF}`, ...)
```

### Anti-Patterns a Evitar

- **Criar rotas separadas por UF** (`/api/icms-fronteira/regras/pe`, `/api/icms-fronteira/regras/ba`): use query param `?uf_estado=PE`. Mais simples e consistente com o padrão de filtros do projeto.
- **Nova tabela para regras por UF**: a coluna `uf_estado` na tabela existente é suficiente. A constraint UNIQUE abarca os três — não crie tabelas separadas por estado.
- **Deixar `UpdateCompanyHandler` aceitar apenas `regime_tributario`**: o handler atual usa um struct literal anônimo com apenas esse campo — expandir para um struct nomeado com todos os campos novos.
- **Seed hardcoded no handler Go**: seeds para BA/CE devem ser SQL migrations (padrão do projeto), não lógica em handlers.

---

## Don't Hand-Roll

| Problema | Não Construir | Usar em vez Disso | Por que |
|----------|--------------|-------------------|---------|
| Validação de CNPJ 14 dígitos | função regexp custom complexa | `len(cnpj) == 14 && regexp.MustCompile(`^\d{14}$`).MatchString(cnpj)` em Go | CNPJ sem máscara: basta checar 14 dígitos numéricos conforme CADU-02 |
| Parse de XLSX para importação | parser XLSX próprio | `github.com/xuri/excelize/v2` (já instalado) | Lida com todas as variações de formato; já em uso em `icms_fronteira_regras.go` |
| Upload de arquivo no frontend | fetch manual com FormData | `<input type="file">` + FormData (já feito em `ImportarXMLsEntrada.tsx`) | Padrão existente no projeto |
| Gestão de estado de formulário | state manager | useState simples (padrão de GestaoAmbiente.tsx) | Formulários pequenos; o projeto não usa react-hook-form |
| Tabs de UF no frontend | roteamento separado | `<Tabs>` shadcn com estado local | Navigation.ts não precisa de novas entradas |

---

## Common Pitfalls

### Pitfall 1: Constraint UNIQUE de icms_fronteira_regras_ncm

**O que vai errado:** A constraint atual é `UNIQUE NULLS NOT DISTINCT (company_id, ncm_prefixo)`. Se adicionarmos `uf_estado NOT NULL DEFAULT 'PE'` sem alterar a constraint, regras PE existentes continuarão com `uf_estado = 'PE'` e novas regras BA/CE funcionarão. Mas se a constraint não for atualizada para incluir `uf_estado`, não será possível criar a mesma regra NCM para duas UFs diferentes.

**Por que acontece:** PostgreSQL não permite dois registros com o mesmo `(company_id, ncm_prefixo)` mesmo que `uf_estado` seja diferente, enquanto a constraint antiga não incluir `uf_estado`.

**Como evitar:**
```sql
-- Na migration 097:
ALTER TABLE icms_fronteira_regras_ncm
    DROP CONSTRAINT IF EXISTS uq_icms_fronteira_regras;

ALTER TABLE icms_fronteira_regras_ncm
    ADD CONSTRAINT uq_icms_fronteira_regras_uf
    UNIQUE NULLS NOT DISTINCT (company_id, ncm_prefixo, uf_estado);
```

**Sinais de alerta:** INSERTs de regras BA/CE retornam erro de duplicate key mesmo com NCM diferente de PE.

### Pitfall 2: Seed BA/CE pode conflitar com seed PE existente

**O que vai errado:** O seed em 091 usa `ON CONFLICT DO NOTHING`. Após adicionar `uf_estado DEFAULT 'PE'`, os registros existentes do seed PE ganharão `uf_estado = 'PE'`. O seed BA/CE em migration 098 precisa inserir os mesmos NCMs com `uf_estado = 'BA'` ou `'CE'` — o novo UNIQUE (company_id, ncm_prefixo, uf_estado) permite isso, mas a migration 098 deve rodar **após** a 097 que adiciona a coluna e altera a constraint.

**Como evitar:** Garantir ordem de execução: 096 → 097 → 098. O sistema de migrations do projeto já garante ordem alfabética/numérica.

### Pitfall 3: CNPJ já foi removido (migration 023)

**O que vai errado:** CADU-01 fala em "adicionar CNPJ" mas `cnpj` foi removido em migration 023. A re-adição precisa ser `ADD COLUMN IF NOT EXISTS` e **sem** a constraint UNIQUE original (que causou problemas — daí a remoção).

**Por que acontece:** Histórico de decisões: CNPJ foi considerado campo de empresa, depois movido para branches/filiais, depois decidiu-se re-expor no cadastro mestre para fins de CADU.

**Como evitar:** A migration 096 deve usar `VARCHAR(18)` (não `VARCHAR(14)` da versão original) conforme CADU-01, sem constraint UNIQUE, e sem NOT NULL (para não quebrar empresas existentes sem CNPJ cadastrado).

```sql
-- Migration 096:
ALTER TABLE companies ADD COLUMN IF NOT EXISTS cnpj               VARCHAR(18);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS inscricao_estadual VARCHAR(30);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS cnae_principal     VARCHAR(7);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS cnae_secundario    TEXT[];
ALTER TABLE companies ADD COLUMN IF NOT EXISTS municipio          VARCHAR(100);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS segmento_economico VARCHAR(100);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS incentivos_fiscais JSONB;
```

### Pitfall 4: GestaoAmbiente.tsx usa interface `Company` local com `cnpj: string`

**O que vai errado:** A interface local `Company` em `GestaoAmbiente.tsx` já tem `cnpj: string` (linha 56), embora a coluna não exista no banco. Isso não causa erros porque `COALESCE` retorna string vazia quando NULL — mas após a migration 096 adicionar o campo, o frontend precisará mostrar os novos campos.

**Como evitar:** Expandir a interface local `Company` para incluir todos os novos campos como opcionais (`cnpj?: string`, etc.). O handler `GetCompaniesHandler` também precisará de scan dos novos campos na query.

### Pitfall 5: `UpdateCompanyHandler` usa struct literal anônimo

**O que vai errado:** O handler atual decodifica apenas `regime_tributario`:
```go
var payload struct {
    RegimeTributario string `json:"regime_tributario"`
}
```
Se o frontend enviar os novos campos, eles serão silenciosamente ignorados.

**Como evitar:** Substituir o struct anônimo por um struct nomeado com todos os campos novos, e atualizar a query SQL de UPDATE para incluir todos os campos.

---

## Code Examples

### Migration 096 — Adicionar campos a companies

```sql
-- Source: padrão estabelecido em migrations/077_add_regime_tributario_to_companies.sql
ALTER TABLE companies ADD COLUMN IF NOT EXISTS cnpj               VARCHAR(18);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS inscricao_estadual VARCHAR(30);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS cnae_principal     VARCHAR(7);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS cnae_secundario    TEXT[];
ALTER TABLE companies ADD COLUMN IF NOT EXISTS municipio          VARCHAR(100);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS segmento_economico VARCHAR(100);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS incentivos_fiscais JSONB;
```

### Migration 097 — Adicionar uf_estado à tabela de regras + nova constraint

```sql
-- Adicionar coluna uf_estado com DEFAULT 'PE' (preserva registros existentes)
ALTER TABLE icms_fronteira_regras_ncm
    ADD COLUMN IF NOT EXISTS uf_estado VARCHAR(2) NOT NULL DEFAULT 'PE';

-- Adicionar colunas MVA ajustado
ALTER TABLE icms_fronteira_regras_ncm
    ADD COLUMN IF NOT EXISTS mva_ajustado_4pct  NUMERIC(8,4);
ALTER TABLE icms_fronteira_regras_ncm
    ADD COLUMN IF NOT EXISTS mva_ajustado_7pct  NUMERIC(8,4);
ALTER TABLE icms_fronteira_regras_ncm
    ADD COLUMN IF NOT EXISTS mva_ajustado_12pct NUMERIC(8,4);

-- Recriar constraint UNIQUE incluindo uf_estado
ALTER TABLE icms_fronteira_regras_ncm
    DROP CONSTRAINT IF EXISTS uq_icms_fronteira_regras;

DO $$ BEGIN
    ALTER TABLE icms_fronteira_regras_ncm
        ADD CONSTRAINT uq_icms_fronteira_regras_uf
        UNIQUE NULLS NOT DISTINCT (company_id, ncm_prefixo, uf_estado);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Criar tabela de inaplicabilidades
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

### Struct Company expandido em Go

```go
// Source: padrão de backend/handlers/environment.go
type Company struct {
    ID                string   `json:"id"`
    GroupID           string   `json:"group_id"`
    Name              string   `json:"name"`
    TradeName         string   `json:"trade_name"`
    RegimeTributario  string   `json:"regime_tributario"`
    CNPJ              string   `json:"cnpj,omitempty"`
    InscricaoEstadual string   `json:"inscricao_estadual,omitempty"`
    CNAEPrincipal     string   `json:"cnae_principal,omitempty"`
    CNAESecundario    []string `json:"cnae_secundario,omitempty"`
    Municipio         string   `json:"municipio,omitempty"`
    SegmentoEconomico string   `json:"segmento_economico,omitempty"`
    InventivosFiscais *json.RawMessage `json:"incentivos_fiscais,omitempty"`
    CreatedAt         string   `json:"created_at"`
}
```

### FronteiraRegraRow expandido com uf_estado e MVA ajustado

```go
// Source: padrão de backend/handlers/icms_fronteira_regras.go
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

### Filtro por UF no ListHandler

```go
// Source: adaptação de IcmsFronteiraRegrasListHandler
ufEstado := r.URL.Query().Get("uf_estado")
if ufEstado == "" {
    ufEstado = "PE" // default para compatibilidade
}
// Validar whitelist
validUFs := map[string]bool{"PE": true, "BA": true, "CE": true}
if !validUFs[ufEstado] {
    jsonErr(w, http.StatusBadRequest, "uf_estado inválido")
    return
}
// Usar $3 na query
rows, err := db.Query(`
    SELECT ... FROM icms_fronteira_regras_ncm
    WHERE (company_id = $1 OR company_id IS NULL)
      AND uf_estado = $2
    ORDER BY ncm_prefixo
`, companyID, ufEstado)
```

### Tabs PE/BA/CE no Frontend

```typescript
// Source: padrão de Tabs usado em IcmsFronteira.tsx
const [selectedUF, setSelectedUF] = useState<'PE' | 'BA' | 'CE'>('PE')

// Dentro da aba Regras NCM existente:
<Tabs value={selectedUF} onValueChange={(v) => setSelectedUF(v as 'PE' | 'BA' | 'CE')}>
  <TabsList>
    <TabsTrigger value="PE">PE — Pernambuco</TabsTrigger>
    <TabsTrigger value="BA">BA — Bahia</TabsTrigger>
    <TabsTrigger value="CE">CE — Ceará</TabsTrigger>
  </TabsList>
  <TabsContent value="PE">...</TabsContent>
  <TabsContent value="BA">...</TabsContent>
  <TabsContent value="CE">...</TabsContent>
</Tabs>
```

### UpdateCompanyHandler expandido

```go
// Source: padrão de UpdateCompanyHandler em environment.go
func UpdateCompanyHandler(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        id := r.URL.Query().Get("id")
        if id == "" {
            http.Error(w, "Missing id parameter", http.StatusBadRequest)
            return
        }
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
        ...
    }
}
```

---

## Seed BA e CE — Referência

Os valores abaixo são [ASSUMED] baseados em legislação estadual conhecida em treinamento. Devem ser revisados com as tabelas oficiais dos RICMS estaduais antes da execução.

### Regras Bahia (BA) — RICMS/BA [ASSUMED]

| NCM | Descrição | Regime | Alíquota Interna | MVA Original |
|-----|-----------|--------|-----------------|--------------|
| 2202 | Refrigerantes | ST | 26.0 | 140.00 |
| 2203 | Cervejas | ST | 26.0 | 140.00 |
| 3004 | Medicamentos humanos | ST | 12.0 | 33.00 |
| 3303–3307 | Cosméticos/higiene | ST | 20.5 | 45.00 |
| 4011 | Pneumáticos | ST | 20.5 | 42.00 |
| 2523 | Cimento | ST | 17.5 | 25.00 |
| 8517 | Celulares | ST | 20.5 | 15.00 |

### Regras Ceará (CE) — RICMS/CE [ASSUMED]

| NCM | Descrição | Regime | Alíquota Interna | MVA Original |
|-----|-----------|--------|-----------------|--------------|
| 2202 | Refrigerantes | ST | 25.0 | 140.00 |
| 2203 | Cervejas | ST | 25.0 | 140.00 |
| 3004 | Medicamentos humanos | ST | 12.0 | 33.00 |
| 3303–3307 | Cosméticos/higiene | ST | 20.5 | 45.00 |
| 4011 | Pneumáticos | ST | 20.5 | 42.00 |
| 2523 | Cimento | ST | 17.0 | 25.00 |
| 8517 | Celulares | ST | 20.5 | 15.00 |

**MVA ajustado** é calculado a partir do MVA original usando a fórmula do Convênio ICMS 110/07:
```
MVA_ajustado = ((1 + MVA_original/100) × (1 - aliq_interestadual/100) / (1 - aliq_interna/100)) - 1
```
Onde `aliq_interestadual` é 4%, 7% ou 12% conforme a UF de origem. As colunas `mva_ajustado_4pct`, `mva_ajustado_7pct`, `mva_ajustado_12pct` devem ser preenchidas com esses valores pré-calculados no seed (ou deixadas NULL para cálculo dinâmico no handler).

---

## State of the Art

| Abordagem Antiga | Abordagem Atual | Quando Mudou | Impacto |
|-----------------|-----------------|-------------|---------|
| CNPJ em `companies` | CNPJ removido (migration 023) | Fase inicial | CADU-01 re-adiciona como campo opcional sem UNIQUE |
| Regras ICMS apenas para PE | Regras multi-UF com `uf_estado` | Phase 8 | Constraint UNIQUE deve ser expandida |
| `UpdateCompanyHandler` atualiza só regime | Handler atualiza todos os campos | Phase 8 | Struct anônimo → struct nomeado |

---

## Assumptions Log

| # | Afirmação | Seção | Risco se Errado |
|---|-----------|-------|-----------------|
| A1 | Alíquotas internas BA (26%) e CE (25%) para refrigerantes/cervejas | Seed BA e CE | Seed incorreto — pode ser corrigido via upload de planilha pelo admin, sem bloqueio funcional |
| A2 | MVA original BA e CE são iguais ao PE para a maioria dos NCMs | Seed BA e CE | Idem — seed é editável via UI |
| A3 | `cnae_secundario TEXT[]` é o tipo correto para array de CNAEs | Migration 096 | PostgreSQL suporta TEXT[] nativamente; se frontend não enviar array, basta enviar `[]` |
| A4 | `incentivos_fiscais JSONB` não precisa de schema fixo nesta fase | Migration 096 | Schema livre pode dificultar queries futuras, mas cobre o requisito CADU-01 |

---

## Open Questions

1. **MVA ajustado calculado ou armazenado?**
   - O que sabemos: CADU-04 exige colunas `mva_ajustado_4pct`, `mva_ajustado_7pct`, `mva_ajustado_12pct` na tabela
   - O que não está claro: o handler de cálculo de ICMS Fronteira deve ler esses valores ou calculá-los dinamicamente a partir de `mva_original` e `aliq_interna`?
   - Recomendação: armazenar pré-calculados (como quer CADU-04) e deixar o seed/import preencher via upload de planilha. O handler de cálculo usa o valor armazenado como fallback.

2. **Gestão multi-empresa: nova página ou expandir GestaoAmbiente.tsx?**
   - O que sabemos: existe `/config/ambiente` → `GestaoAmbiente.tsx` com CRUD de environments/groups/companies. CADU-03 pede "tela de cadastro/edição de empresa com todos os novos campos".
   - O que não está claro: tela separada `/config/empresas` ou modal expandido na GestaoAmbiente existente?
   - Recomendação: expandir o modal existente em `GestaoAmbiente.tsx` para incluir os novos campos — menos impacto em navigation.ts e App.tsx.

3. **Upload de planilha de regras BA/CE: mesma rota `/api/icms-fronteira/regras/importar` com campo `uf_estado` no CSV/header ou rota diferente?**
   - Recomendação: mesma rota com `uf_estado` como coluna obrigatória no CSV/XLSX (coluna 0 = uf_estado, coluna 1 = ncm_prefixo, ...) ou via form field adicional. Mais simples que nova rota.

---

## Environment Availability

> Nenhuma dependência nova — todas as ferramentas já disponíveis no projeto.

| Dependência | Requerida por | Disponível | Versão | Fallback |
|-------------|--------------|-----------|--------|----------|
| PostgreSQL 15 | Migrations 096-098 | Verificado via migrations existentes | 15.x | — |
| Go 1.24 | Backend handlers | Verificado via go.mod | 1.24 | — |
| `excelize/v2` | Import XLSX | Já em go.mod | v2.x | CSV (já suportado) |
| React 18 + Vite | Frontend | Já em package.json | 18.x | — |

---

## Validation Architecture

> `nyquist_validation: false` em `.planning/config.json` — seção omitida conforme configuração.

---

## Security Domain

### ASVS Aplicáveis

| Categoria ASVS | Aplica | Controle |
|---------------|--------|----------|
| V5 Input Validation | sim | Validação CNPJ 14 dígitos; whitelist uf_estado (PE/BA/CE); NULLIF para campos vazios |
| V4 Access Control | sim | `withAuth` em todas as rotas; UpdateCompanyHandler deve checar company_id pertence ao user |
| V2 Authentication | não | Auth existente via JWT — sem mudança |

### Ameaças por Padrão

| Padrão | STRIDE | Mitigação |
|--------|--------|-----------|
| CNPJ inválido no cadastro | Tampering | Validação regexp `^\d{14}$` no backend |
| uf_estado fora da whitelist | Tampering | `validUFs := map[string]bool{"PE": true, "BA": true, "CE": true}` antes de usar em query |
| company_id forjado no header | Spoofing | `GetEffectiveCompanyID` já trata isso — manter uso obrigatório |
| Upload XLSX com fórmulas maliciosas | Tampering | `excelize.OpenReader` não executa macros; risco baixo |

---

## Sources

### Primary (HIGH confidence)
- `backend/handlers/environment.go` — Company struct, CreateCompanyHandler, UpdateCompanyHandler completos
- `backend/migrations/013_create_environment_hierarchy.sql` — schema original de companies
- `backend/migrations/023_remove_cnpj_from_companies.sql` — remoção histórica do CNPJ
- `backend/migrations/077_add_regime_tributario_to_companies.sql` — padrão de migration idempotente
- `backend/migrations/091_icms_fronteira.sql` — schema completo de icms_fronteira_regras_ncm + seed PE
- `backend/handlers/icms_fronteira_regras.go` — List/Create/Delete/Importar handlers completos
- `backend/main.go` — todas as rotas registradas
- `frontend/src/lib/navigation.ts` — todos os módulos e tabs
- `frontend/src/App.tsx` — todas as rotas React
- `frontend/src/pages/GestaoAmbiente.tsx` — padrão completo de CRUD environment/group/company
- `frontend/src/pages/IcmsFronteira.tsx` — padrão de Tabs e useQuery
- `.planning/config.json` — nyquist_validation: false

### Secondary (MEDIUM confidence)
- `.planning/.continue-here.md` — diagnóstico das 3 funcionalidades ausentes da sessão anterior
- `.planning/REQUIREMENTS.md` — definição oficial de CADU-01 a CADU-07

### Tertiary (LOW / ASSUMED)
- Valores de alíquotas BA/CE e MVA estaduais — baseados em treinamento, não verificados contra legislação atual

---

## Metadata

**Breakdown de confiança:**
- Schema atual do banco: HIGH — lido diretamente das migrations
- Padrões de código Go: HIGH — lido diretamente dos handlers existentes
- Padrões de código React/TS: HIGH — lido diretamente dos componentes existentes
- Valores de alíquotas BA/CE para seed: LOW — [ASSUMED], editáveis via UI
- Estratégia de constraint UNIQUE: HIGH — comportamento PostgreSQL verificado via leitura da DDL

**Data de pesquisa:** 2026-05-23
**Válido até:** 2026-06-22 (estável — stack bem estabelecida, sem dependências externas novas)
