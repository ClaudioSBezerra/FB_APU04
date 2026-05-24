---
phase: 09-modulos-2x-analytics-dimensional
fixed_at: 2026-05-23T00:00:00Z
review_path: .planning/phases/09-modulos-2x-analytics-dimensional/09-REVIEW.md
iteration: 1
findings_in_scope: 6
fixed: 6
skipped: 0
status: all_fixed
---

# Phase 09: Code Review Fix Report

**Fixed at:** 2026-05-23
**Source review:** `.planning/phases/09-modulos-2x-analytics-dimensional/09-REVIEW.md`
**Iteration:** 1

**Summary:**
- Findings in scope: 6 (Critical: 2, Warning: 4)
- Fixed: 6
- Skipped: 0

## Fixed Issues

### CR-01: CSV headers movidos para após todos os caminhos de erro

**Files modified:** `backend/handlers/reforma_modulo2.go`
**Commit:** `890116c`
**Applied fix:** Em `CfopAnalysisCSVHandler` e `NcmAnalysisCSVHandler`, as chamadas `w.Header().Set("Content-Type", "text/csv...")` e `w.Header().Set("Content-Disposition", ...)` foram movidas para imediatamente antes de `cw := csv.NewWriter(w)`, após todos os checks de auth, obtenção de empresa, leitura de parâmetros, execução de query e iteração de rows. Todos os caminhos de erro anteriores chamam `http.Error` sem o `Content-Disposition` presente.

---

### CR-02: Guard `userID == ""` adicionado nos handlers CSV

**Files modified:** `backend/handlers/reforma_modulo2.go`
**Commit:** `890116c`
**Applied fix:** Em ambos `CfopAnalysisCSVHandler` e `NcmAnalysisCSVHandler`, após `userID, _ := claims["user_id"].(string)`, foi adicionado:
```go
if userID == "" {
    http.Error(w, "Não autenticado", http.StatusUnauthorized)
    return
}
```
Isso garante que um JWT com `user_id` ausente retorna 401 em vez de provocar um 500 em `GetEffectiveCompanyID`.

---

### WR-01: GROUP BY NCM sem `x_prod` — LIMIT 100 aplica-se a NCMs distintos

**Files modified:** `backend/handlers/reforma_modulo2.go`
**Commit:** `890116c`
**Applied fix:** Em `NcmAnalysisHandler` e `NcmAnalysisCSVHandler`, a query foi alterada:
- SELECT: `COALESCE(nit.x_prod,'') AS x_prod` → `MAX(COALESCE(nit.x_prod,'')) AS x_prod`
- GROUP BY: removido `nit.x_prod` de ambas as queries

Agora o LIMIT 100 aplica-se a NCMs distintos, e cada NCM exibe a descrição de produto mais recente como representativa.

---

### WR-02: Correção do canal verde em `colorScale` (Reforma23UfDestino)

**Files modified:** `frontend/src/pages/Reforma23UfDestino.tsx`
**Commit:** `c11ad9a`
**Applied fix:** Corrigido o valor inicial do canal verde de `190` para `234` (valor real de `#dbeafe = rgb(219, 234, 254)`). O comentário também foi atualizado de `(219, 190, 254)` para `(219, 234, 254)`. Com 190, o mínimo renderizava roxo/lavanda; com 234 renderiza o azul claro correto.

---

### WR-03: `handleExportCSV` notifica o usuário em caso de falha via toast

**Files modified:** `frontend/src/pages/Reforma22CfopAnalysis.tsx`, `frontend/src/pages/Reforma21NcmAnalysis.tsx`
**Commit:** `edb809c`
**Applied fix:** O bloco `catch (_err) { // silent }` foi substituído por:
```ts
catch (err) {
  console.error('Falha ao exportar CSV:', err)
  toast.error('Erro ao exportar CSV')
}
```
`import { toast } from 'sonner'` adicionado no topo de ambos os arquivos.

---

### WR-04: Âncora de download inserida no DOM antes de `.click()`

**Files modified:** `frontend/src/pages/Reforma22CfopAnalysis.tsx`, `frontend/src/pages/Reforma21NcmAnalysis.tsx`
**Commit:** `edb809c`
**Applied fix:** O padrão `a.click()` simples foi substituído por:
```ts
document.body.appendChild(a)
a.click()
document.body.removeChild(a)
```
Garante que o elemento `<a>` esteja no DOM durante o clique, corrigindo o download de Blob URL no Firefox.

---

_Fixed: 2026-05-23_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
