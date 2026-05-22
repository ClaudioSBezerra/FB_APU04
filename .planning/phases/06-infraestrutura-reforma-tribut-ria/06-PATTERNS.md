# Phase 6: Infraestrutura Reforma Tributária — Pattern Map

**Mapped:** 2026-05-22
**Files analyzed:** 12 (4 migrations, 2 backend handlers/modifications, 2 frontend modifications, 2 frontend new, 1 static asset, 1 backend worker modification)
**Analogs found:** 11 / 12

---

## File Classification

| Arquivo Novo/Modificado | Role | Data Flow | Analog Mais Próximo | Qualidade |
|--------------------------|------|-----------|---------------------|-----------|
| `backend/migrations/086_add_cst_aliq_icms_to_reg_c190.sql` | migration | DDL | `backend/migrations/084_add_competencia_and_fix_vw_xml_operacoes.sql` | exact |
| `backend/migrations/087_create_reforma_parametros.sql` | migration | DDL | `backend/migrations/010_update_schema_efd_icms.sql` | role-match |
| `backend/migrations/088_add_ind_final_to_nfe_saidas.sql` | migration | DDL | `backend/migrations/084_add_competencia_and_fix_vw_xml_operacoes.sql` | exact |
| `backend/migrations/089_seed_cfop_transferencias.sql` | migration | DDL/seed | `backend/migrations/062_reseed_cfops.sql` | exact |
| `backend/handlers/reforma_config.go` (novo) | handler | request-response | `backend/handlers/rfb_credentials.go` | exact |
| `backend/worker/worker.go` (modificar) | worker/parser | batch/transform | self (linha 738–752) | self-referential |
| `backend/handlers/nfe_saidas.go` (modificar) | handler/parser | file-I/O | self (linhas 119–126 e 542–616) | self-referential |
| `backend/main.go` (modificar) | config | request-response | `backend/main.go` linhas 358–368 (withAuth) | self-referential |
| `frontend/src/hooks/useReformaParametros.ts` (novo) | hook | request-response | `frontend/src/pages/ERPBridgeConfig.tsx` linhas 114–122 | role-match |
| `frontend/src/pages/ReformaParametros.tsx` (novo) | page/component | request-response | `frontend/src/pages/ERPBridgeConfig.tsx` | exact |
| `frontend/src/lib/navigation.ts` (modificar) | config | — | self (linhas 14–76) | self-referential |
| `frontend/src/App.tsx` (modificar) | config/router | — | self (linhas 163–182) | self-referential |
| `frontend/public/brazil-states.json` (novo) | static asset | — | nenhum | sem analog |

---

## Pattern Assignments

### `backend/migrations/086_add_cst_aliq_icms_to_reg_c190.sql` (migration, DDL)

**Analog:** `backend/migrations/084_add_competencia_and_fix_vw_xml_operacoes.sql` linhas 26–28

**Padrão ADD COLUMN com IF NOT EXISTS:**
```sql
-- Padrão exato do codebase (084, linha 27):
ALTER TABLE xml_upload_batches
    ADD COLUMN IF NOT EXISTS competencia VARCHAR(7);

-- Aplicar para reg_c190:
ALTER TABLE reg_c190
    ADD COLUMN IF NOT EXISTS cst_icms  VARCHAR(3),
    ADD COLUMN IF NOT EXISTS aliq_icms NUMERIC(6,2);
```

**Cabeçalho de migration (padrão do codebase — ver 085):**
```sql
-- Migration 086: Adiciona cst_icms e aliq_icms em reg_c190 (RFMA-01)
--
-- Motivo: registrar o CST ICMS (posição parts[2]) e a alíquota ICMS (parts[4])
-- do registro EFD C190 para alimentar os módulos analíticos da Reforma Tributária.
-- Colunas nullable: registros históricos sem dado ficam NULL — tratado como ausência.
```

---

### `backend/migrations/087_create_reforma_parametros.sql` (migration, DDL)

**Analog:** `backend/migrations/010_update_schema_efd_icms.sql` linhas 1–18 (CREATE TABLE IF NOT EXISTS)

**Padrão CREATE TABLE:**
```sql
-- Da migration 010 (padrão):
CREATE TABLE IF NOT EXISTS reg_c190 (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES import_jobs(id) ON DELETE CASCADE,
    ...
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Adaptar para reforma_parametros (PK = company_id, sem FK para import_jobs):
CREATE TABLE IF NOT EXISTS reforma_parametros (
    company_id         UUID PRIMARY KEY REFERENCES companies(id) ON DELETE CASCADE,
    target_ano         INTEGER         NOT NULL DEFAULT 2027,
    aliq_ibs_pct       NUMERIC(6,4)    NOT NULL DEFAULT 0.26,
    aliq_cbs_pct       NUMERIC(6,4)    NOT NULL DEFAULT 0.086,
    fator_simples_pct  NUMERIC(6,4)    NOT NULL DEFAULT 0.20,
    taxa_cdi_anual_pct NUMERIC(6,4)    NOT NULL DEFAULT 0.10,
    prazo_medio_dias   INTEGER         NOT NULL DEFAULT 30,
    created_at         TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at         TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
```

**Nota:** A PK `company_id` é também FK — verificar coluna `id` em `companies` durante implementação.

---

### `backend/migrations/088_add_ind_final_to_nfe_saidas.sql` (migration, DDL)

**Analog:** `backend/migrations/084_add_competencia_and_fix_vw_xml_operacoes.sql` linhas 26–28

**Padrão idêntico ao 086 (ADD COLUMN IF NOT EXISTS):**
```sql
ALTER TABLE nfe_saidas
    ADD COLUMN IF NOT EXISTS ind_final SMALLINT;
-- NULL = dado ausente (NF-e histórica); 0 = B2B; 1 = consumidor final.
```

---

### `backend/migrations/089_seed_cfop_transferencias.sql` (migration, seed)

**Analog:** `backend/migrations/062_reseed_cfops.sql` linhas 1–33

**Padrão de seed com ON CONFLICT — do 062:**
```sql
INSERT INTO cfop (cfop, descricao_cfop, tipo) VALUES
('5101', 'Venda de produção do estabelecimento', 'R'),
...
ON CONFLICT (cfop) DO NOTHING;
```

**Diferença crítica para transferências (Pitfall 4 do RESEARCH.md — usar DO UPDATE, não DO NOTHING):**
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
-- DO UPDATE (não DO NOTHING) garante que CFOPs eventualmente cadastrados com tipo errado
-- sejam corrigidos para 'T' na aplicação da migration.
```

---

### `backend/handlers/reforma_config.go` (handler, request-response) — NOVO

**Analog:** `backend/handlers/rfb_credentials.go` (arquivo completo)

**Padrão de imports — do rfb_credentials.go linhas 1–11:**
```go
package handlers

import (
    "database/sql"
    "encoding/json"
    "net/http"

    "github.com/golang-jwt/jwt/v5"
)
// Nota: não importar "time" se os campos de timestamp forem lidos mas não manipulados.
// Importar "strings" apenas se houver sanitização de input.
```

**Padrão de struct — adaptar de RFBCredential (rfb_credentials.go linhas 14–24):**
```go
type ReformaParametros struct {
    CompanyID       string  `json:"company_id"`
    TargetAno       int     `json:"target_ano"`
    AliqIBSPct      float64 `json:"aliq_ibs_pct"`
    AliqCBSPct      float64 `json:"aliq_cbs_pct"`
    FatorSimplesPct float64 `json:"fator_simples_pct"`
    TaxaCDIAnualPct float64 `json:"taxa_cdi_anual_pct"`
    PrazoMedioDias  int     `json:"prazo_medio_dias"`
}
```

**Padrão GET handler — do GetRFBCredentialHandler (rfb_credentials.go linhas 27–73):**
```go
func GetReformaParametrosHandler(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.Header().Set("Access-Control-Allow-Origin", "*")

        claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
        if !ok {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        userID := claims["user_id"].(string)

        companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
        if err != nil {
            http.Error(w, "Error getting company: "+err.Error(), http.StatusInternalServerError)
            return
        }

        var p ReformaParametros
        err = db.QueryRow(`
            SELECT company_id, target_ano, aliq_ibs_pct, aliq_cbs_pct,
                   fator_simples_pct, taxa_cdi_anual_pct, prazo_medio_dias
            FROM reforma_parametros
            WHERE company_id = $1
        `, companyID).Scan(&p.CompanyID, &p.TargetAno, &p.AliqIBSPct, &p.AliqCBSPct,
            &p.FatorSimplesPct, &p.TaxaCDIAnualPct, &p.PrazoMedioDias)

        if err == sql.ErrNoRows {
            // Retornar defaults — empresa ainda não configurou parâmetros
            json.NewEncoder(w).Encode(map[string]interface{}{"parametros": nil})
            return
        }
        if err != nil {
            http.Error(w, "Error querying parametros: "+err.Error(), http.StatusInternalServerError)
            return
        }

        json.NewEncoder(w).Encode(map[string]interface{}{"parametros": p})
    }
}
```

**Padrão PUT handler com UPSERT — do SaveRFBCredentialHandler (rfb_credentials.go linhas 76–157):**
```go
func PutReformaParametrosHandler(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.Header().Set("Access-Control-Allow-Origin", "*")

        if r.Method != http.MethodPut {
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
            return
        }

        claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
        if !ok {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        userID := claims["user_id"].(string)

        companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
        if err != nil {
            http.Error(w, "Error getting company: "+err.Error(), http.StatusInternalServerError)
            return
        }

        var req ReformaParametros
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, "Invalid request body", http.StatusBadRequest)
            return
        }

        // Validação de ranges (V5 ASVS — input validation)
        // aliq_ibs_pct, aliq_cbs_pct, fator_simples_pct, taxa_cdi_anual_pct ∈ [0, 1]
        // prazo_medio_dias ∈ [1, 3650]

        // UPSERT — DO UPDATE SET (nunca DO NOTHING para parâmetros mutáveis)
        // Padrão direto do rfb_credentials.go linha 146–152
        _, err = db.Exec(`
            INSERT INTO reforma_parametros
              (company_id, target_ano, aliq_ibs_pct, aliq_cbs_pct,
               fator_simples_pct, taxa_cdi_anual_pct, prazo_medio_dias)
            VALUES ($1, $2, $3, $4, $5, $6, $7)
            ON CONFLICT (company_id) DO UPDATE SET
              target_ano         = $2,
              aliq_ibs_pct       = $3,
              aliq_cbs_pct       = $4,
              fator_simples_pct  = $5,
              taxa_cdi_anual_pct = $6,
              prazo_medio_dias   = $7,
              updated_at         = CURRENT_TIMESTAMP
        `, companyID, req.TargetAno, req.AliqIBSPct, req.AliqCBSPct,
           req.FatorSimplesPct, req.TaxaCDIAnualPct, req.PrazoMedioDias)
        if err != nil {
            http.Error(w, "Error saving parametros: "+err.Error(), http.StatusInternalServerError)
            return
        }

        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]string{"message": "Parâmetros salvos com sucesso"})
    }
}
```

---

### `backend/worker/worker.go` — modificação cirúrgica (RFMA-01)

**Self-referential analog:** linhas 494–496 (stmtC190 Prepare) e linhas 738–752 (case "C190" Exec)

**Estado atual (linhas 494–496):**
```go
stmtC190, err = tx.Prepare(`INSERT INTO reg_c190 (job_id, id_pai_c100, cfop, vl_opr, vl_bc_icms, vl_icms, vl_bc_icms_st, vl_icms_st, vl_red_bc, vl_ipi, cod_obs) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`)
```

**Estado atual (linhas 738–752):**
```go
case "C190":
    parts := strings.Split(line, "|")
    if len(parts) >= 12 && currentC100ID != "" {
        // ... debug IPI ...
        stmtC190.Exec(jobID, currentC100ID, parts[3], parseDecimal(parts[5]), parseDecimal(parts[6]), parseDecimal(parts[7]), parseDecimal(parts[8]), parseDecimal(parts[9]), parseDecimal(parts[10]), vlIpi, parts[12])
    }
```

**Modificações necessárias — 2 pontos cirúrgicos:**

1. Linha 494: adicionar `cst_icms, aliq_icms` ao INSERT e `$12, $13` ao VALUES
2. Linha 740: mudar guard de `>= 12` para `>= 13`
3. Linha 752: adicionar `parts[2], parseDecimal(parts[4])` ao Exec

```go
// DEPOIS — linha 494:
stmtC190, err = tx.Prepare(`INSERT INTO reg_c190
  (job_id, id_pai_c100, cfop, vl_opr, vl_bc_icms, vl_icms, vl_bc_icms_st, vl_icms_st, vl_red_bc, vl_ipi, cod_obs, cst_icms, aliq_icms)
  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`)

// DEPOIS — linha 740 (guard):
if len(parts) >= 13 && currentC100ID != "" {

// DEPOIS — linha 752 (Exec):
stmtC190.Exec(jobID, currentC100ID, parts[3], parseDecimal(parts[5]), parseDecimal(parts[6]), parseDecimal(parts[7]), parseDecimal(parts[8]), parseDecimal(parts[9]), parseDecimal(parts[10]), vlIpi, parts[12], parts[2], parseDecimal(parts[4]))
//                                                                                                                                                                                                              ^^^ cod_obs  ^^^^^^^^^ cst_icms  ^^^^^^^^^^^^^^^^^^^^^aliq_icms
```

**Notas de posição:** Layout EFD C190 após `strings.Split(line, "|")`:
- `parts[0]=""` (pipe inicial), `parts[1]="C190"`, `parts[2]=CST_ICMS`, `parts[3]=CFOP`, `parts[4]=ALIQ_ICMS`, `parts[5]=VL_BC_ICMS`, ..., `parts[12]=COD_OBS`
- Fonte: RESEARCH.md Pitfall 1 + codebase worker.go linha 746 (`parts[11]` = VL_IPI, `parts[12]` = COD_OBS confirmado)

---

### `backend/handlers/nfe_saidas.go` — modificação (RFMA-03)

**Self-referential analog — 3 pontos cirúrgicos:**

**Ponto 1: struct `ide` (linhas 119–126) — adicionar IndFinal:**
```go
// ANTES:
type ide struct {
    Mod   string `xml:"mod"`
    Serie string `xml:"serie"`
    NNF   string `xml:"nNF"`
    DhEmi string `xml:"dhEmi"`
    TpNF  string `xml:"tpNF"`
    NatOp string `xml:"natOp"`
}

// DEPOIS:
type ide struct {
    Mod      string `xml:"mod"`
    Serie    string `xml:"serie"`
    NNF      string `xml:"nNF"`
    DhEmi    string `xml:"dhEmi"`
    TpNF     string `xml:"tpNF"`
    NatOp    string `xml:"natOp"`
    IndFinal string `xml:"indFinal"` // "0"=B2B/normal, "1"=consumidor final; "" para NF-e antigas
}
```

**Ponto 2: INSERT INTO nfe_saidas (linhas 543–566) — adicionar `ind_final`:**
```go
// Adicionar `ind_final` à lista de colunas após `source`:
INSERT INTO nfe_saidas (
    company_id, chave_nfe, ..., source, ind_final
) VALUES (
    $1, $2, ..., 'xml_upload', $43
)
// Adicionar parâmetro $43 correspondendo a toNullSmallInt(inf.Ide.IndFinal) no Exec
```

**Ponto 3: ON CONFLICT DO UPDATE SET (linhas 568–603) — adicionar `ind_final` (Pitfall 3):**
```go
// Adicionar na lista DO UPDATE SET (após source = 'xml_upload'):
ind_final = EXCLUDED.ind_final
```

**Helper necessário (criar localmente ou usar padrão existente):**
```go
// Converter string "0"/"1"/"" para *int (NULL se vazio)
func toNullSmallInt(s string) interface{} {
    s = strings.TrimSpace(s)
    if s == "" { return nil }
    if s == "1" { return 1 }
    return 0
}
```

---

### `backend/main.go` — modificação (registrar rotas)

**Self-referential analog:** linhas 358–368 (`withAuth`) e linhas 526–542 (padrão multi-method switch)

**Padrão de rota multi-method (main.go linhas 526–542):**
```go
http.HandleFunc("/api/rfb/credentials", func(w http.ResponseWriter, r *http.Request) {
    database := getDB()
    if database == nil { jsonServiceUnavailable(w); return }
    switch r.Method {
    case http.MethodGet:
        handlers.AuthMiddleware(handlers.GetRFBCredentialHandler(database), "")(w, r)
    case http.MethodPost:
        handlers.AuthMiddleware(handlers.SaveRFBCredentialHandler(database), "")(w, r)
    case http.MethodDelete:
        handlers.AuthMiddleware(handlers.DeleteRFBCredentialHandler(database), "")(w, r)
    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
})
```

**Adaptar para `/api/reforma/parametros`:**
```go
http.HandleFunc("/api/reforma/parametros", func(w http.ResponseWriter, r *http.Request) {
    database := getDB()
    if database == nil { jsonServiceUnavailable(w); return }
    switch r.Method {
    case http.MethodGet:
        handlers.AuthMiddleware(handlers.GetReformaParametrosHandler(database), "")(w, r)
    case http.MethodPut:
        handlers.AuthMiddleware(handlers.PutReformaParametrosHandler(database), "admin")(w, r)
    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
})
```

---

### `frontend/src/hooks/useReformaParametros.ts` (hook, request-response) — NOVO

**Analog:** `frontend/src/pages/ERPBridgeConfig.tsx` linhas 114–122 (padrão `useQuery`)

**Padrão de hook isolado — baseado no useQuery do ERPBridgeConfig:**
```typescript
// ERPBridgeConfig.tsx linhas 1-3 (imports base):
import { useState, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useAuth } from '@/contexts/AuthContext';

// ERPBridgeConfig.tsx linhas 114-122 (padrão useQuery):
const { data: cfg, isLoading } = useQuery<BridgeConfig>({
    queryKey: ['erp-bridge-config', companyId],
    queryFn: async () => {
        const res = await fetch('/api/erp-bridge/config', { headers: authHeaders });
        if (!res.ok) throw new Error(res.statusText);
        return res.json();
    },
    enabled: !!token && !!companyId,
});
```

**Diferença importante para useReformaParametros:** O RESEARCH.md (linha 96) confirma que `fetch()` global é interceptado pelo `AuthContext` — **não precisar passar `headers: authHeaders` manualmente**. Verificado em `AuthContext.tsx` linhas 46–55.

```typescript
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
  return useQuery<{ parametros: ReformaParametros | null }>({
    queryKey: ['reforma-parametros'],
    queryFn: async () => {
      const res = await fetch('/api/reforma/parametros')
      if (!res.ok) throw new Error(res.statusText)
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
      return res.json()
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['reforma-parametros'] }),
  })
}
```

---

### `frontend/src/pages/ReformaParametros.tsx` (page/component, request-response) — NOVO

**Analog:** `frontend/src/pages/ERPBridgeConfig.tsx` (estrutura geral) + `frontend/src/pages/ConciliacaoBridgeXML.tsx` linhas 198–224 (tooltip)

**Padrão de imports — do ERPBridgeConfig.tsx linhas 1–17:**
```typescript
import { useState, useEffect } from 'react'
import { useAuth } from '@/contexts/AuthContext'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'
```

**Padrão de tooltip — do ConciliacaoBridgeXML.tsx linhas 29–34 (imports):**
```typescript
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { Info } from 'lucide-react'
```

**Padrão de verificação de role admin — do App.tsx linhas 62–69 (`AdminRoute`):**
```typescript
// App.tsx:
function AdminRoute({ children }: { children: React.ReactNode }) {
    const { isAuthenticated, loading, user } = useAuth()
    if (user?.role !== 'admin') return <Navigate to="/" replace />
    ...
}

// No ReformaParametros.tsx — mesma verificação inline:
const { user } = useAuth()
const isAdmin = user?.role === 'admin'
// Usar isAdmin para: disabled={!isAdmin} nos inputs e {isAdmin && <Button>Salvar</Button>}
```

**Padrão de card com campo inline editável — do ERPBridgeConfig.tsx linhas 575–715:**
```typescript
// Card de configuração com fields + botão Salvar (padrão ERPBridgeConfig.tsx):
<Card>
  <CardHeader className="py-3 px-4">
    <CardTitle className="text-sm flex items-center gap-2">
      <Settings2 className="h-4 w-4" /> Parâmetros da Reforma Tributária
    </CardTitle>
  </CardHeader>
  <CardContent className="px-4 pb-4 space-y-5">
    {/* Campos inline */}
    <div className="flex flex-col gap-1">
      <label className="text-xs text-muted-foreground">IBS (%)</label>
      <Input type="number" value={aliqIBS} onChange={...} disabled={!isAdmin} className="h-8 w-32 text-sm" />
    </div>
    {/* Botão Salvar — oculto para não-admins (D-06) */}
    {isAdmin && (
      <div className="flex justify-end pt-1">
        <Button size="sm" onClick={() => saveMutation.mutate()} disabled={saveMutation.isPending}>
          {saveMutation.isPending && <Loader2 className="h-3 w-3 mr-1.5 animate-spin" />}
          Salvar
        </Button>
      </div>
    )}
  </CardContent>
</Card>
```

**Padrão de tooltip ⓘ para fator_simples_pct — do ConciliacaoBridgeXML.tsx linhas 198–224:**
```typescript
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

**Padrão de sincronização de state com dados do hook — do ERPBridgeConfig.tsx linhas 170–178:**
```typescript
// ERPBridgeConfig.tsx — useEffect para popular state local quando dados carregam:
useEffect(() => {
    if (cfg) {
        setAtivo(cfg.ativo)
        setHorario(cfg.horario)
        setDiasRetro(cfg.dias_retroativos)
    }
}, [cfg])

// Adaptar para ReformaParametros.tsx:
useEffect(() => {
    if (data?.parametros) {
        setTargetAno(data.parametros.target_ano)
        setAliqIBS(data.parametros.aliq_ibs_pct)
        // ... etc
    }
}, [data])
```

---

### `frontend/src/lib/navigation.ts` — modificação (RFMA-07)

**Self-referential analog:** linhas 14–76 (estrutura `modules` Record e `getActiveModule`)

**Padrão de novo módulo — copiar estrutura do módulo `config` (linhas 45–58):**
```typescript
// navigation.ts linhas 45–58 (padrão de módulo com tabs):
config: {
    label: 'Configurações',
    tabs: [
        { label: 'Alíquotas',          path: '/config/aliquotas' },
        { label: 'CFOP',               path: '/config/cfop' },
        // ...
        { label: 'Limpar Dados', path: '/config/limpar-dados', adminOnly: true, danger: true },
    ],
},

// Adicionar módulo reforma logo após simulador (antes de notas):
reforma: {
    label: 'Análise Reforma Tributária',
    tabs: [
        { label: 'Parâmetros',        path: '/reforma/parametros' },
        // Placeholders Phase 7 (D-02):
        { label: 'Créditos IBS/CBS',  path: '/reforma/creditos',      disabled: true },
        { label: 'Reprecificação',    path: '/reforma/reprecificacao', disabled: true },
        { label: 'Ranking NCM/CFOP',  path: '/reforma/ranking',       disabled: true },
        { label: 'Split Payment',     path: '/reforma/split-payment', disabled: true },
        // Placeholders Phase 8 (D-02):
        { label: 'UF Destino',        path: '/reforma/uf-destino',    disabled: true },
    ],
},
```

**Adicionar tab no módulo config — posição após 'Alíquotas' (linha 49):**
```typescript
{ label: 'Alíquotas',            path: '/config/aliquotas' },
{ label: 'Parâmetros Reforma',   path: '/config/reforma-parametros' }, // inserir aqui
{ label: 'CFOP',                 path: '/config/cfop' },
```

**Padrão `getActiveModule` — copiar bloco de config (linha 74) e adicionar reforma (D-03):**
```typescript
// Adicionar antes do bloco config (linha 74):
if (pathname.startsWith('/reforma')) return 'reforma'
if (pathname.startsWith('/config/reforma-parametros')) return 'config'
if (pathname.startsWith('/config/')) return 'config'
```

---

### `frontend/src/App.tsx` — modificação (RFMA-07)

**Self-referential analog:** linhas 1–41 (imports) e linhas 163–182 (Routes config)

**Padrão de import de nova página — do App.tsx linhas 26–30:**
```typescript
// App.tsx linhas 26–30 (padrão de import de página admin):
import ERPBridgeConfig from './pages/ERPBridgeConfig'
import ERPBridgeLogs from './pages/ERPBridgeLogs'
import ERPBridgeCredenciais from './pages/ERPBridgeCredenciais'
import AdminUsers from './pages/AdminUsers'
import LimparDados from './pages/LimparDados'

// Adicionar import de ReformaParametros após LimparDados:
import ReformaParametros from './pages/ReformaParametros'
```

**Padrão de Route sem restrição de role — do App.tsx linha 174:**
```typescript
// App.tsx linha 174 (rota simples sem AdminRoute):
<Route path="/config/aliquotas" element={<TabelaAliquotas />} />

// Adicionar ambas as rotas (D-04 e Pitfall 7):
<Route path="/reforma/parametros"        element={<ReformaParametros />} />
<Route path="/config/reforma-parametros" element={<ReformaParametros />} />
// Nota: a página controla internamente o acesso de escrita via isAdmin (D-06).
// Não usar AdminRoute — usuários não-admin devem ver em modo leitura (não ser redirecionados).
```

---

### `frontend/public/brazil-states.json` — static asset (RFMA-08)

**Sem analog no codebase.** Arquivo TopoJSON externo para Phase 8. Ver RESEARCH.md seção RFMA-08. Obter de https://raw.githubusercontent.com/codeforamerica/click_that_hood/master/public/data/brazil-states.geojson ou equivalente no formato topojson compatível com `react-simple-maps` v3.0.0.

---

## Shared Patterns

### Autenticação e Extração de company_id
**Fonte:** `backend/handlers/rfb_credentials.go` linhas 32–43 e `backend/handlers/auth.go` linhas 209–261
**Aplicar a:** `reforma_config.go` (GetReformaParametrosHandler e PutReformaParametrosHandler)
```go
// Extração padrão — exatamente como rfb_credentials.go linhas 32–43:
claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
if !ok {
    http.Error(w, "Unauthorized", http.StatusUnauthorized)
    return
}
userID := claims["user_id"].(string)

companyID, err := GetEffectiveCompanyID(db, userID, r.Header.Get("X-Company-ID"))
if err != nil {
    http.Error(w, "Error getting company: "+err.Error(), http.StatusInternalServerError)
    return
}
```

### withAuth + AuthMiddleware (role check no middleware)
**Fonte:** `backend/main.go` linhas 358–368 e `backend/handlers/auth.go` linha 253
**Aplicar a:** registro de rotas em `main.go`
```go
// O role check "admin" acontece em AuthMiddleware ANTES do handler ser chamado.
// Handler PutReformaParametrosHandler NÃO precisa verificar role internamente.
// auth.go linha 253:
if requiredRole != "" && userRole != requiredRole && userRole != "admin" {
    http.Error(w, "Forbidden: insufficient permissions", http.StatusForbidden)
    return
}
```

### Error Handling Go (http.Error + sql.ErrNoRows)
**Fonte:** `backend/handlers/rfb_credentials.go` linhas 52–62
**Aplicar a:** `reforma_config.go`
```go
if err == sql.ErrNoRows {
    json.NewEncoder(w).Encode(map[string]interface{}{"parametros": nil})
    return
}
if err != nil {
    http.Error(w, "Error querying parametros: "+err.Error(), http.StatusInternalServerError)
    return
}
```

### useQuery + useMutation no Frontend
**Fonte:** `frontend/src/pages/ERPBridgeConfig.tsx` linhas 114–234
**Aplicar a:** `useReformaParametros.ts`, `ReformaParametros.tsx`
```typescript
// Padrão de onSuccess com toast + invalidate (ERPBridgeConfig.tsx linhas 229–234):
onSuccess: () => {
    toast.success('Configuração salva.')
    qc.invalidateQueries({ queryKey: ['reform-parametros'] })
},
onError: (e: Error) => toast.error(`Erro ao salvar: ${e.message}`),
```

### Tooltip Radix UI (Provider obrigatório)
**Fonte:** `frontend/src/pages/ConciliacaoBridgeXML.tsx` linhas 29–34 (imports) e 198–224 (uso)
**Aplicar a:** `ReformaParametros.tsx` — campo `fator_simples_pct`
**Atenção Pitfall 6:** `<Tooltip>` sem `<TooltipProvider>` causa erro runtime. Sempre envolver em `<TooltipProvider delayDuration={200}>`.

### Padrão de disabled tabs no navigation
**Fonte:** `frontend/src/lib/navigation.ts` linhas 36–39
**Aplicar a:** tabs placeholder da Reforma Tributária (D-02)
```typescript
// navigation.ts linha 36–37 (padrão de tab disabled):
{ label: 'NF-e Entradas',    path: '/apuracao/entrada/notas',  disabled: true },
{ label: 'Importar via ERP', path: '/importacoes/erp-bridge',  adminOnly: true, disabled: true },
```

### ModuleTabs renderização de disabled tabs
**Fonte:** `frontend/src/App.tsx` linhas 88–93
```typescript
// App.tsx linhas 88–93 — disabled tab renderizado como span (não Link):
return isDisabled ? (
    <span
        key={tab.path}
        className="px-3 py-1.5 text-xs rounded-md text-muted-foreground/50 cursor-not-allowed whitespace-nowrap"
    >
        {tab.label}
    </span>
) : ( ... )
// Comportamento automático — basta setar disabled: true no navigation.ts.
```

---

## No Analog Found

| Arquivo | Role | Data Flow | Motivo |
|---------|------|-----------|--------|
| `frontend/public/brazil-states.json` | static asset | — | Nenhum arquivo TopoJSON/GeoJSON existe no projeto. Obter externamente e commitar em `frontend/public/`. |

---

## Metadata

**Escopo de busca de analogs:** `backend/handlers/`, `backend/migrations/`, `backend/worker/`, `frontend/src/pages/`, `frontend/src/hooks/`, `frontend/src/lib/`, `frontend/src/App.tsx`, `backend/main.go`
**Arquivos escaneados:** ~50
**Data de extração de padrões:** 2026-05-22
