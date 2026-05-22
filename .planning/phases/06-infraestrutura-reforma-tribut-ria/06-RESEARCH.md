# Phase 6: Infraestrutura Reforma Tributária — Research

**Researched:** 2026-05-22
**Domain:** PostgreSQL schema migrations, Go REST handlers, React/TypeScript frontend, EFD C190 parsing
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** Criar novo módulo `reforma` no `navigation.ts` com label "Reforma Tributária" e path base `/reforma`.
- **D-02:** Na Phase 6, o módulo tem apenas uma tab ativa: "Parâmetros" em `/reforma/parametros`. As tabs dos módulos 1.x e 2.x serão adicionadas nas Phases 7 e 8 com `disabled: true` como placeholder já visível na sidebar.
- **D-03:** A função `getActiveModule` em `navigation.ts` deve retornar `'reforma'` para rotas que começam com `/reforma`.
- **D-04:** A rota `/reforma/parametros` e `/config/reforma-parametros` renderizam **o mesmo componente** (`ReformaParametros.tsx`). A tab no módulo "Reforma Tributária" aponta para `/reforma/parametros`; uma aba nova "Parâmetros Reforma" no módulo `config` em `navigation.ts` aponta para `/config/reforma-parametros`.
- **D-05:** UX: card com campos inline editáveis + botão Salvar, seguindo padrão visual de `TabelaAliquotas` e `ERPBridgeConfig`. Não é modal.
- **D-06:** Usuários não-admin veem a página em somente leitura — campos desabilitados, botão Salvar oculto. Sem redirecionamento.
- **D-07:** Validação de acesso de escrita via role `admin` no backend (`PUT /api/reforma/parametros` retorna 403 para não-admins).
- **D-08:** Exibir tooltip/ícone ⓘ ao lado do label do campo `fator_simples_pct` com o texto: "Valor estimado. Alíquota definitiva ainda não publicada pelo CG-IBS." Não é banner, não é modal, não é alerta fixo.
- **D-09:** Registros históricos de `reg_c190` sem `cst_icms`/`aliq_icms` (NULL) e de `nfe_saidas` sem `ind_final` (NULL) não precisam de aviso visual. NULL é tratado como ausência de dado.
- **D-10:** Migrations seguem sequência existente (`085_...sql` → `086_...sql`, etc.) com arquivos separados por concern: `086_add_cst_aliq_icms_to_reg_c190.sql`, `087_create_reforma_parametros.sql`, `088_add_ind_final_to_nfe_saidas.sql`, `089_seed_cfop_transferencias.sql`.
- **D-11:** Handler `reforma_config.go` segue padrão de `config.go` — closure `db *sql.DB`, extrai `company_id` do contexto via middleware existente, retorna JSON.
- **D-12:** Hook `useReformaParametros.ts` usa `useQuery` do `@tanstack/react-query`. Compartilhado globalmente (não por módulo).

### Claude's Discretion

- Nomes exatos das migrations (numeração, nomenclatura) — seguir sequência e convenção existente.
- Estrutura interna de `ReformaParametros.tsx` — implementar seguindo `TabelaAliquotas.tsx` como referência mais próxima.
- Posição exata da tab "Parâmetros Reforma" no módulo `config` (antes ou depois de "Alíquotas").
- Tabs placeholder para módulos 1.x/2.x: labels e paths seguirão o ROADMAP, mas nomes exatos são discrição do implementador.

### Deferred Ideas (OUT OF SCOPE)

- Módulos analíticos (1.1 créditos, 1.2 reprecificação, 1.3 ranking, 1.4 split payment, 2.1–2.4 analíticos) — Phases 7 e 8.
- Backfill de `cst_icms`/`aliq_icms` para registros históricos do EFD.
- Aviso visual para registros históricos com NULL.
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| RFMA-01 | Adicionar `cst_icms VARCHAR(3)` e `aliq_icms NUMERIC(6,2)` em `reg_c190`; atualizar `worker.go` para popular a partir de `parts[2]` e `parts[4]` do registro C190 | Confirmado: `stmtC190` atual em worker.go usa `parts[3]` (cfop), `parts[5..12]` sem `parts[2]`/`parts[4]`. C190 layout: `|REG|CST_ICMS|CFOP|ALIQ_ICMS|VL_OPR|...` — posições 2 e 4 corretas para CST e aliq |
| RFMA-02 | Criar tabela `reforma_parametros` com `company_id` PK, campos IBS/CBS/fator/CDI/prazo | Confirmado: tabela não existe; padrão UPSERT com `ON CONFLICT (company_id)` idêntico ao `rfb_credentials` |
| RFMA-03 | Adicionar `ind_final SMALLINT` em `nfe_saidas`; estender struct `ide` em `nfe_saidas.go` para ler `ide/indFinal`; persistir em novas importações | Confirmado: struct `ide` não tem `IndFinal`; INSERT de `nfe_saidas` em `xml_upload.go` não passa o campo |
| RFMA-04 | Seed de CFOPs de transferência na tabela `cfop` com `tipo='T'`: 1151/1152/2151/2152/5151/5152/6151/6152 | Confirmado: nenhuma migration seeds esses CFOPs; tabela `cfop` tem coluna `tipo` com valores A/C/R/T/O/S |
| RFMA-05 | Endpoints `GET /api/reforma/parametros` e `PUT /api/reforma/parametros` no backend | Confirmado: rota não existe; `withAuth(handler, "admin")` para PUT, `withAuth(handler, "")` para GET |
| RFMA-06 | Hook `useReformaParametros.ts` usando `useQuery` do `@tanstack/react-query` | Confirmado: `@tanstack/react-query` v5.90.20 já instalado; padrão de hook isolado em `src/hooks/` |
| RFMA-07 | Entrada "Análise Reforma Tributária" no `navigation.ts` e rota em `App.tsx`; página `ReformaParametros.tsx` editável só por admins; disclaimer sobre fator Simples Nacional | Confirmado: `navigation.ts` tem estrutura `modules` Record extensível; tooltip radix-ui disponível |
| RFMA-08 | Instalar `react-simple-maps` v3.0.0 e commitar TopoJSON em `frontend/public/brazil-states.json` | Confirmado: pacote existe no npm (v3.0.0, MIT, criado 2017); não está instalado; sem conflito de peerdeps com React 18 |
</phase_requirements>

---

## Summary

A Phase 6 é puramente de infraestrutura: quatro migrations PostgreSQL, um novo handler Go, um hook React e uma página de configuração. Nenhum módulo analítico é entregue. Toda a base que os módulos 1.x (Phase 7) e 2.x (Phase 8) consumirão é criada aqui.

O codebase está em estado limpo para receber as mudanças. A migration mais recente é `085_fix_vw_xml_entradas_informativos_pis_cofins.sql`, portanto as novas migrations serão `086`–`089`. O `stmtC190` em `worker.go` já faz o INSERT em `reg_c190`, mas não inclui `cst_icms` nem `aliq_icms` — a extensão é cirúrgica (adicionar 2 colunas ao Prepare + Exec). O struct `ide` em `nfe_saidas.go` não tem `IndFinal` — é necessário adicionar o campo Go + coluna na tabela + parâmetro no INSERT/ON CONFLICT de `xml_upload.go`.

O handler `reforma_config.go` deve seguir exatamente o padrão de `config.go` (GET apenas) mas com GET+PUT seguindo `rfb_credentials.go` (closure db, extração de company_id via `GetEffectiveCompanyID`, validação de role). O frontend tem `@tanstack/react-query` v5 instalado e o padrão de hook isolado existe em `src/hooks/`. O Tooltip Radix UI já está em uso (`ConciliacaoBridgeXML.tsx`).

**Recomendação principal:** Executar as 4 migrations primeiro (pré-requisito de tudo), depois o handler Go, depois o hook e a página React. O setup de `react-simple-maps` (RFMA-08) é independente e pode ser feito em qualquer ordem.

---

## Architectural Responsibility Map

| Capability | Tier Principal | Tier Secundário | Racional |
|------------|---------------|-----------------|----------|
| Schema: colunas reg_c190, nfe_saidas, tabela reforma_parametros | Database/Migration | — | DDL sempre em migration numerada |
| Parser EFD C190: ler CST e alíquota ICMS | Backend (worker.go) | — | O worker processa o arquivo linha a linha |
| Parser XML NF-e: ler indFinal | Backend (xml_upload.go + nfe_saidas.go) | — | Struct de parse e INSERT no mesmo pacote handlers |
| Seed CFOPs de transferência | Database/Migration | — | Dados de referência vão em migration, não em handler |
| API reforma/parametros GET+PUT | API/Backend (Go handler) | — | Lógica de persistência e validação de role pertencem ao backend |
| Hook useReformaParametros | Frontend (React hook) | — | Camada de acesso a dados client-side; compartilhado globalmente via react-query |
| Página ReformaParametros + navegação | Frontend (React page) | — | UI pura; lê do hook, persiste via PUT |
| Instalação react-simple-maps + TopoJSON | Frontend (static asset) | — | Dependência de mapa para Phase 8; sem componente criado na Phase 6 |

---

## Standard Stack

### Core (já presente no projeto)
| Biblioteca | Versão atual | Propósito | Status |
|------------|-------------|-----------|--------|
| Go (database/sql + lib/pq) | — | Handler backend, migrations via arquivo SQL | Já em uso |
| PostgreSQL | — | Banco principal | Já em uso |
| `@tanstack/react-query` | ^5.90.20 | Fetching/caching de dados no frontend | Já instalado |
| `@radix-ui/react-tooltip` | ^1.2.8 | Tooltip ⓘ para disclaimer fator Simples | Já instalado |
| `react-router-dom` | ^6.22.3 | Roteamento SPA | Já instalado |

### Nova dependência
| Biblioteca | Versão | Propósito | Status |
|------------|--------|-----------|--------|
| `react-simple-maps` | 3.0.0 | Mapa coroplético dos estados brasileiros (Phase 8) | A instalar (RFMA-08) |

**Instalação:**
```bash
cd frontend && npm install react-simple-maps@3.0.0
```

---

## Package Legitimacy Audit

| Package | Registry | Idade | Downloads | Source Repo | slopcheck | Disposição |
|---------|----------|-------|-----------|-------------|-----------|------------|
| react-simple-maps | npm | ~8 anos (criado 2017-03-15) | Não verificado via npm view | github.com/zcreativelabs/react-simple-maps | [FALSO POSITIVO: slopcheck verificou PyPI] — npm confirmado [VERIFIED: npm registry] | Aprovado |

**Nota sobre slopcheck:** O slopcheck na versão 0.6.1 verificou PyPI por engano ao invés do registry npm. A verificação correta foi feita diretamente via `npm view react-simple-maps`: pacote existe, versão 3.0.0 (última), MIT, criado 2017, mantido até 2023, sem script `postinstall`.

**Pacotes removidos por slopcheck [SLOP]:** nenhum
**Pacotes com [SUS]:** nenhum

---

## Architecture Patterns

### System Architecture Diagram

```
EFD Upload (upload.go)                    XML Upload (xml_upload.go)
      |                                          |
      v                                          v
worker.go (C190 parser)               nfe_saidas.go struct ide
  - parts[2] = CST_ICMS         -->     + IndFinal string `xml:"indFinal"`
  - parts[4] = ALIQ_ICMS                         |
      |                                           v
      v                                 INSERT nfe_saidas
  INSERT reg_c190                         + ind_final = $N
   + cst_icms, aliq_icms                           
                                                   
─────────────────────────────────────────────────
                                                   
Browser (React SPA)                                
  |                                                
  v                                                
useReformaParametros.ts (react-query useQuery)     
  - GET /api/reforma/parametros                    
  - queryKey: ['reforma-parametros', companyId]    
      |                                            
      v                                            
ReformaParametros.tsx                              
  - Card com campos inline editáveis               
  - Tooltip ⓘ para fator_simples_pct              
  - Botão Salvar (admin only)                      
  - PUT /api/reforma/parametros                    
      |                                            
      v                                            
reforma_config.go (Go handler)                    
  - GET: SELECT FROM reforma_parametros WHERE company_id=$1
  - PUT (admin only): UPSERT INTO reforma_parametros
      |                                            
      v                                            
reforma_parametros (PostgreSQL table)              
  PK: company_id UUID                              
  + target_ano, aliq_ibs_pct, aliq_cbs_pct         
  + fator_simples_pct, taxa_cdi_anual_pct          
  + prazo_medio_dias                               
```

### Estrutura de Arquivos Recomendada

```
backend/
├── migrations/
│   ├── 086_add_cst_aliq_icms_to_reg_c190.sql  # ADD COLUMN (RFMA-01)
│   ├── 087_create_reforma_parametros.sql        # CREATE TABLE (RFMA-02)
│   ├── 088_add_ind_final_to_nfe_saidas.sql      # ADD COLUMN (RFMA-03)
│   └── 089_seed_cfop_transferencias.sql         # INSERT ON CONFLICT (RFMA-04)
├── handlers/
│   └── reforma_config.go                        # GET + PUT /api/reforma/parametros (RFMA-05)
└── worker/
    └── worker.go                                # Modificar stmtC190 (RFMA-01)

frontend/
├── src/
│   ├── hooks/
│   │   └── useReformaParametros.ts              # react-query hook (RFMA-06)
│   ├── pages/
│   │   └── ReformaParametros.tsx                # Página parâmetros (RFMA-07)
│   ├── lib/
│   │   └── navigation.ts                        # Adicionar módulo 'reforma' (RFMA-07)
│   └── App.tsx                                  # Adicionar 2 Routes (RFMA-07)
└── public/
    └── brazil-states.json                       # TopoJSON Brasil (RFMA-08)
```

---

## Don't Hand-Roll

| Problema | Não construir | Usar em vez disso | Por quê |
|----------|--------------|-------------------|---------|
| Tooltip de disclaimer | Tag `<span>` com CSS customizado | `TooltipProvider/Tooltip/TooltipTrigger/TooltipContent` de `@radix-ui/react-tooltip` | Já em uso em `ConciliacaoBridgeXML.tsx`; acessibilidade automática |
| Caching de parâmetros no frontend | useState + useEffect manual | `useQuery` de `@tanstack/react-query` | Deduplicação de requests, cache global, stale-while-revalidate |
| Validação de role no PUT | Verificar role dentro do handler manualmente | `withAuth(handler, "admin")` no `main.go` | `AuthMiddleware` já rejeita com 403 antes de entrar no handler |
| Extração de company_id | Ler header manualmente | `GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))` | Função já existente em `auth.go`; cobre hierarquia de empresa |
| Verificação de admin no frontend | Lógica duplicada no componente | `user?.role === 'admin'` via `useAuth()` (padrão já em `App.tsx`) | Padrão `AdminRoute` já estabelecido |
| UPSERT manual com UPDATE | INSERT + UPDATE condicional | `INSERT ... ON CONFLICT (company_id) DO UPDATE SET ...` | Padrão já em `rfb_credentials.go` |

---

## Pitfalls Comuns

### Pitfall 1: Posições do C190 no EFD vs. índices de `parts[]`
**O que dá errado:** O layout EFD C190 é `|REG|CST_ICMS|CFOP|ALIQ_ICMS|VL_BC_ICMS|VL_ICMS|VL_BC_ICMS_ST|VL_ICMS_ST|VL_RED_BC|VL_IPI|COD_OBS|` — campos indexados 0-based após o split. `parts[0]="C190"`, `parts[1]=""` (pipe inicial), portanto os índices dependem de como o split ocorre.
**Por que ocorre:** O worker faz `strings.Split(line, "|")` — a linha começa com `|C190|`, então `parts[0]=""`, `parts[1]="C190"`. No código atual: `case "C190"` captura pelo valor depois do trim, e `parts[3]` é cfop. Isso confirma que `parts[2]=CST_ICMS` e `parts[4]=ALIQ_ICMS`. [VERIFIED: codebase — linha 752 worker.go usa `parts[3]` para cfop, `parts[5]` para vl_bc_icms, consistente com layout]
**Como evitar:** Validar `len(parts) >= 13` (precisamos até `parts[12]` = cod_obs). Alterar o guard `if len(parts) >= 12` para `>= 13` ao adicionar `parts[4]` (aliq_icms).
**Sinal de alerta:** `stmtC190.Exec` com placeholder count divergente do INSERT — o compilador Go não pega, o driver Postgres retorna erro em runtime.

### Pitfall 2: ON CONFLICT com UPSERT em `reforma_parametros` — não usar `DO NOTHING`
**O que dá errado:** Se o PUT usar `INSERT ... ON CONFLICT DO NOTHING`, atualizações de parâmetros existentes são silenciosamente descartadas.
**Por que ocorre:** Confusão com o padrão de seeds (que usam `DO NOTHING`).
**Como evitar:** PUT deve usar `ON CONFLICT (company_id) DO UPDATE SET aliq_ibs_pct=$2, ... updated_at=CURRENT_TIMESTAMP` — exatamente como `rfb_credentials.go` linha 149.

### Pitfall 3: `ind_final` no ON CONFLICT/UPDATE de `nfe_saidas`
**O que dá errado:** Adicionar `ind_final` ao INSERT mas esquecer de incluí-lo no `ON CONFLICT ... DO UPDATE SET`, causando que reimportações de XMLs não atualizem o campo.
**Por que ocorre:** O bloco `ON CONFLICT` de `nfe_saidas` em `xml_upload.go` (linhas 407-427) tem lista longa de campos — fácil esquecer o novo.
**Como evitar:** Adicionar `ind_final = EXCLUDED.ind_final` à lista do `DO UPDATE SET`.

### Pitfall 4: CFOPs de transferência — ON CONFLICT DO NOTHING vs. DO UPDATE
**O que dá errado:** Se `5151` já existir na tabela `cfop` com `tipo='R'` (improvável, mas possível), `ON CONFLICT (cfop) DO NOTHING` preservaria o tipo errado.
**Por que ocorre:** Seeds anteriores (062) poderiam ter inserido esses CFOPs com tipo diferente.
**Como evitar:** Usar `ON CONFLICT (cfop) DO UPDATE SET tipo='T', descricao_cfop=EXCLUDED.descricao_cfop` para garantir idempotência e corretude.

### Pitfall 5: react-simple-maps e ESM/Vite
**O que dá errado:** `react-simple-maps` v3.0.0 exporta ESM; em alguns setups Vite pode não resolver corretamente dependências transitivas (`d3-geo`, etc.).
**Por que ocorre:** A biblioteca depende de `d3-geo` que pode ter problemas de dual CJS/ESM em Vite 5.
**Como evitar:** Ao instalar, verificar `npm install react-simple-maps@3.0.0` e testar `vite build` — se falhar com "does not provide an export named", adicionar ao `vite.config.ts`:
```ts
optimizeDeps: { include: ['react-simple-maps'] }
```
Na Phase 6 apenas instalamos e comitamos o TopoJSON — nenhum componente é criado, então build errors só aparecem na Phase 8.

### Pitfall 6: Tooltip no frontend exige `TooltipProvider` no escopo
**O que dá errado:** `<Tooltip>` sem `<TooltipProvider>` wrapper lança erro runtime.
**Por que ocorre:** Radix Tooltip requer provider de contexto.
**Como evitar:** Envolver o campo `fator_simples_pct` em `<TooltipProvider delayDuration={200}>` — exatamente como em `ConciliacaoBridgeXML.tsx` linhas 198-224.

### Pitfall 7: Rota duplicada `/reforma/parametros` e `/config/reforma-parametros`
**O que dá errado:** Adicionar apenas uma das rotas no `App.tsx`, deixando a outra sem componente registrado.
**Por que ocorre:** D-04 especifica que ambas renderizam o mesmo componente — é fácil registrar só uma.
**Como evitar:** Adicionar explicitamente ambas as rotas em `App.tsx`:
```tsx
<Route path="/reforma/parametros" element={<ReformaParametros />} />
<Route path="/config/reforma-parametros" element={<ReformaParametros />} />
```

---

## Code Examples

### Pattern: stmtC190 em worker.go (RFMA-01)
```go
// Source: backend/worker/worker.go linha 494 (atual)
// ANTES:
stmtC190, err = tx.Prepare(`INSERT INTO reg_c190 
  (job_id, id_pai_c100, cfop, vl_opr, vl_bc_icms, vl_icms, vl_bc_icms_st, vl_icms_st, vl_red_bc, vl_ipi, cod_obs) 
  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`)

// DEPOIS (adicionar cst_icms, aliq_icms):
stmtC190, err = tx.Prepare(`INSERT INTO reg_c190 
  (job_id, id_pai_c100, cfop, vl_opr, vl_bc_icms, vl_icms, vl_bc_icms_st, vl_icms_st, vl_red_bc, vl_ipi, cod_obs, cst_icms, aliq_icms) 
  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`)

// Exec correspondente (linha 752 atual):
// ANTES:
stmtC190.Exec(jobID, currentC100ID, parts[3], parseDecimal(parts[5]), ...)
// DEPOIS: adicionar parts[2] e parseDecimal(parts[4]):
stmtC190.Exec(jobID, currentC100ID, parts[3], parseDecimal(parts[5]), parseDecimal(parts[6]), parseDecimal(parts[7]), parseDecimal(parts[8]), parseDecimal(parts[9]), parseDecimal(parts[10]), vlIpi, parts[12], parts[2], parseDecimal(parts[4]))
// Guard: mudar len(parts) >= 12 para >= 13
```

### Pattern: Handler GO GET+PUT (RFMA-05)
```go
// Source: backend/handlers/rfb_credentials.go (padrão de referência)
// Para reforma_config.go:

type ReformaParametros struct {
    CompanyID       string  `json:"company_id"`
    TargetAno       int     `json:"target_ano"`
    AliqIBSPct      float64 `json:"aliq_ibs_pct"`
    AliqCBSPct      float64 `json:"aliq_cbs_pct"`
    FatorSimplesPct float64 `json:"fator_simples_pct"`
    TaxaCDIAnualPct float64 `json:"taxa_cdi_anual_pct"`
    PrazoMedioDias  int     `json:"prazo_medio_dias"`
}

func GetReformaParametrosHandler(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
        if !ok { http.Error(w, "Unauthorized", http.StatusUnauthorized); return }
        userID := claims["user_id"].(string)
        companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
        // ... SELECT FROM reforma_parametros WHERE company_id = $1
    }
}

func PutReformaParametrosHandler(db *sql.DB) http.HandlerFunc {
    // Requer role admin — registrar como withAuth(handler, "admin") no main.go
    return func(w http.ResponseWriter, r *http.Request) {
        // ... UPSERT ON CONFLICT (company_id) DO UPDATE SET ...
    }
}
```

### Pattern: Rotas no main.go (RFMA-05)
```go
// Source: backend/main.go padrão withAuth
// GET: sem restrição de role (qualquer autenticado vê os parâmetros)
http.HandleFunc("/api/reforma/parametros", func(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        withAuth(handlers.GetReformaParametrosHandler, "")(w, r)
    case http.MethodPut:
        withAuth(handlers.PutReformaParametrosHandler, "admin")(w, r)
    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
})
```

### Pattern: useQuery hook (RFMA-06)
```typescript
// Source: padrão de frontend/src/pages/ERPBridgeConfig.tsx linha 114
// frontend/src/hooks/useReformaParametros.ts

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
  return useQuery<ReformaParametros>({
    queryKey: ['reforma-parametros'],
    queryFn: async () => {
      const res = await fetch('/api/reforma/parametros')
      if (!res.ok) throw new Error(res.statusText)
      return res.json()
    },
  })
}

// Nota: fetch() global já interceptado por AuthContext — 
// não é necessário passar Authorization nem X-Company-ID manualmente.
// [VERIFIED: codebase — AuthContext.tsx linhas 46-55]
```

### Pattern: Tooltip Radix UI (D-08)
```typescript
// Source: frontend/src/pages/ConciliacaoBridgeXML.tsx linhas 198-224
import {
  Tooltip, TooltipContent, TooltipProvider, TooltipTrigger,
} from '@/components/ui/tooltip'
import { Info } from 'lucide-react'

// Uso no label do campo fator_simples_pct:
<TooltipProvider delayDuration={200}>
  <Tooltip>
    <TooltipTrigger asChild>
      <span className="inline-flex items-center gap-1 cursor-help">
        Fator Simples Nacional (%)
        <Info className="h-3.5 w-3.5 text-muted-foreground" />
      </span>
    </TooltipTrigger>
    <TooltipContent side="right" className="max-w-sm text-xs p-3">
      Valor estimado. Alíquota definitiva ainda não publicada pelo CG-IBS.
    </TooltipContent>
  </Tooltip>
</TooltipProvider>
```

### Pattern: Struct ide com IndFinal (RFMA-03)
```go
// Source: backend/handlers/nfe_saidas.go linha 119 (estado atual)
// ANTES:
type ide struct {
    Mod   string `xml:"mod"`
    Serie string `xml:"serie"`
    NNF   string `xml:"nNF"`
    DhEmi string `xml:"dhEmi"`
    TpNF  string `xml:"tpNF"`
    NatOp string `xml:"natOp"`
}

// DEPOIS (adicionar IndFinal):
type ide struct {
    Mod      string `xml:"mod"`
    Serie    string `xml:"serie"`
    NNF      string `xml:"nNF"`
    DhEmi    string `xml:"dhEmi"`
    TpNF     string `xml:"tpNF"`
    NatOp    string `xml:"natOp"`
    IndFinal string `xml:"indFinal"` // "0"=normal, "1"=consumidor final; "" para NF-e B2B
}
```

### Pattern: Migration para nova coluna nullable (RFMA-01, RFMA-03)
```sql
-- Source: padrão de migrations existentes
-- 086_add_cst_aliq_icms_to_reg_c190.sql
ALTER TABLE reg_c190
    ADD COLUMN IF NOT EXISTS cst_icms  VARCHAR(3),
    ADD COLUMN IF NOT EXISTS aliq_icms NUMERIC(6,2);

-- 088_add_ind_final_to_nfe_saidas.sql
ALTER TABLE nfe_saidas
    ADD COLUMN IF NOT EXISTS ind_final SMALLINT;
```

### Pattern: Migration seed CFOP com DO UPDATE (RFMA-04)
```sql
-- 089_seed_cfop_transferencias.sql
INSERT INTO cfop (cfop, descricao_cfop, tipo) VALUES
('1151', 'Transferência para industrialização', 'T'),
('1152', 'Transferência para comercialização', 'T'),
('2151', 'Transferência para industrialização - interestadual', 'T'),
('2152', 'Transferência para comercialização - interestadual', 'T'),
('5151', 'Transferência de produção do estabelecimento', 'T'),
('5152', 'Transferência de mercadoria adquirida ou recebida de terceiros', 'T'),
('6151', 'Transferência interestadual de produção do estabelecimento', 'T'),
('6152', 'Transferência interestadual de mercadoria adquirida ou recebida de terceiros', 'T')
ON CONFLICT (cfop) DO UPDATE SET tipo = 'T', descricao_cfop = EXCLUDED.descricao_cfop;
```

---

## State of the Art

| Abordagem Antiga | Abordagem Atual | Impacto |
|-----------------|-----------------|---------|
| `TabelaAliquotas.tsx` usa `useEffect + fetch` manual | `useQuery` de `@tanstack/react-query` v5 | Hook centralizado com deduplicação e cache |
| Handlers GET simples sem role check | `withAuth(handler, "admin")` para escrita | RBAC consistente via middleware único |

---

## Assumptions Log

| # | Claim | Seção | Risco se errado |
|---|-------|-------|-----------------|
| A1 | IndFinal no NF-e v4.00 XML está em `ide/indFinal` | RFMA-03 / Struct ide | Campo poderia estar em `infNFe/indFinal` — verificar no schema NF-e oficial antes de implementar |
| A2 | O layout C190 do SPED EFD tem `|REG|CST_ICMS|CFOP|ALIQ_ICMS|...` com CST em posição 2 e aliq em posição 4 | RFMA-01 | Se layout variar entre versões do SPED, posições poderiam ser diferentes — confirmar com arquivo EFD real |

---

## Open Questions

1. **Localização exata do `IndFinal` no XML NF-e v4.00**
   - O que sabemos: o campo existe na NF-e v4.00 como `indFinal` no grupo `ide`
   - O que está incerto: o tag XML exato poderia ser `indFinal` ou outro; struct `ide` em `nfe_saidas.go` não tem o campo ainda
   - Recomendação: verificar um XML de amostra ou o schema XSD da NF-e v4.00 durante implementação. Se necessário, adicionar `IndFinal string \`xml:"indFinal"\`` ao struct `ide`.

2. **Verificação do guard `len(parts)` para C190**
   - O que sabemos: guard atual é `>= 12` (para 12 campos). Com `aliq_icms` em `parts[4]`, precisamos de `>= 13` (para o `cod_obs` em `parts[12]`).
   - Recomendação: validar com arquivo EFD real — contar os pipes de uma linha C190.

---

## Environment Availability

| Dependência | Requerida por | Disponível | Versão | Fallback |
|-------------|--------------|-----------|--------|----------|
| npm (Node.js) | Instalar react-simple-maps | ✓ | — | — |
| PostgreSQL | Rodar migrations | ✓ (container Docker) | — | — |
| Go toolchain | Compilar handler | ✓ | — | — |

**Dependências ausentes sem fallback:** nenhuma

---

## Security Domain

`security_enforcement` não configurado em `.planning/config.json` — tratado como habilitado.

### Categorias ASVS Aplicáveis

| Categoria ASVS | Aplica | Controle Padrão |
|----------------|--------|-----------------|
| V2 Authentication | Sim | JWT via `AuthMiddleware` já existente |
| V3 Session Management | Não | — |
| V4 Access Control | Sim | `withAuth(handler, "admin")` para PUT — 403 para não-admins |
| V5 Input Validation | Sim | Validar campos numéricos no PUT handler (aliq_ibs_pct ∈ [0,100], etc.) |
| V6 Cryptography | Não | Parâmetros fiscais não são secrets |

### Ameaças Conhecidas para o Stack

| Padrão | STRIDE | Mitigação Padrão |
|--------|--------|-----------------|
| Usuário não-admin salva alíquotas arbitrárias | Tampering | `withAuth(handler, "admin")` no PUT; backend rejeita com 403 |
| SQL injection nos campos numéricos | Tampering | `$1, $2...` parametrizados — nunca interpolação de string |
| IDOR: PUT com company_id de outra empresa | Elevation of Privilege | `GetEffectiveCompanyID` extrai company_id do JWT, não do body |

---

## Sources

### Primárias (HIGH confidence)
- Codebase `backend/worker/worker.go` — layout C190, stmtC190 atual [VERIFIED: codebase]
- Codebase `backend/handlers/xml_upload.go` — INSERT nfe_saidas, struct uses [VERIFIED: codebase]
- Codebase `backend/handlers/nfe_saidas.go` — struct `ide` sem IndFinal [VERIFIED: codebase]
- Codebase `backend/handlers/auth.go` — `AuthMiddleware`, `GetEffectiveCompanyID` [VERIFIED: codebase]
- Codebase `backend/handlers/rfb_credentials.go` — padrão GET+PUT com UPSERT [VERIFIED: codebase]
- Codebase `backend/handlers/config.go` — padrão GET simples [VERIFIED: codebase]
- Codebase `frontend/src/lib/navigation.ts` — estrutura modules Record [VERIFIED: codebase]
- Codebase `frontend/src/App.tsx` — padrão de import/Route [VERIFIED: codebase]
- Codebase `frontend/src/pages/ConciliacaoBridgeXML.tsx` — padrão Tooltip Radix [VERIFIED: codebase]
- Codebase `frontend/package.json` — dependências instaladas [VERIFIED: codebase]
- npm registry `react-simple-maps@3.0.0` [VERIFIED: npm registry]

### Secundárias (MEDIUM confidence)
- `backend/migrations/062_reseed_cfops.sql` — confirma `ON CONFLICT (cfop) DO NOTHING` e estrutura da tabela [VERIFIED: codebase]
- `backend/migrations/009_create_cfop_table.sql` — confirma coluna `tipo` com valores A/C/R/T/O/S [VERIFIED: codebase]

---

## Metadata

**Breakdown de confiança:**
- Schema e migrations: HIGH — baseado em migrations existentes verificadas no codebase
- Padrões de handler Go: HIGH — baseado em `rfb_credentials.go` e `config.go` verificados
- Padrões React/TypeScript: HIGH — baseado em `ERPBridgeConfig.tsx` e `ConciliacaoBridgeXML.tsx` verificados
- Layout EFD C190: MEDIUM — inferido do código existente de worker.go; A2 é [ASSUMED]
- IndFinal no XML NF-e: MEDIUM — campo padrão NF-e v4.00, mas tag exato não verificado [ASSUMED A1]

**Research date:** 2026-05-22
**Valid until:** 2026-06-22 (stack estável — nenhuma dependência em rápida evolução)
