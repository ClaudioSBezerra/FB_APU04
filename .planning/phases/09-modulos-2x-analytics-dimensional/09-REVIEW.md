---
phase: 09-modulos-2x-analytics-dimensional
reviewed: 2026-05-23T00:00:00Z
depth: standard
files_reviewed: 9
files_reviewed_list:
  - backend/handlers/reforma_modulo2.go
  - backend/handlers/reforma_modulo2_test.go
  - backend/main.go
  - frontend/src/pages/Reforma22CfopAnalysis.tsx
  - frontend/src/pages/Reforma21NcmAnalysis.tsx
  - frontend/src/pages/Reforma23UfDestino.tsx
  - frontend/src/pages/Reforma24B2bB2c.tsx
  - frontend/src/lib/navigation.ts
  - frontend/src/App.tsx
findings:
  critical: 2
  warning: 4
  info: 3
  total: 9
status: issues_found
---

# Phase 09: Code Review — Módulos 2.x Analytics Dimensional

**Revisado em:** 2026-05-23
**Profundidade:** standard
**Arquivos revisados:** 9
**Status:** issues_found

## Resumo

Foram revisados 6 handlers Go (4 JSON + 2 CSV), arquivo de testes, main.go (6 novas rotas) e 4 páginas React.
O isolamento multi-tenant via `GetEffectiveCompanyID` está correto — `company_id` nunca é interpolado em SQL nem
lido do request body. Todos os parâmetros SQL são parametrizados. `rows.Err()` é chamado em todos os loops de
scan. `Content-Type` é definido antes de qualquer `WriteHeader` nos handlers JSON.

Foram encontrados **2 issues críticos** e **4 warnings** que devem ser corrigidos antes do deploy.

---

## Critical Issues

### CR-01: Handlers CSV expõem `Content-Disposition: attachment` em respostas de erro — download de arquivo com mensagem de erro ao invés de resposta HTTP legível

**Arquivo:** `backend/handlers/reforma_modulo2.go:248-292` (CfopAnalysisCSVHandler) e `:455-507` (NcmAnalysisCSVHandler)

**Issue:**
Os dois handlers CSV definem `Content-Type: text/csv` e `Content-Disposition: attachment; filename="..."` nas
linhas 248-249 / 455-456 **antes** de executar a query ao banco. Se `db.Query` ou `rows.Err()` falhar depois
dessas linhas, o caminho de erro chama `http.Error(w, ..., 500)`.

`http.Error` substitui o `Content-Type` por `text/plain`, mas **não remove** o `Content-Disposition` que já está
no mapa de headers. O browser recebe status 200→escrito pelo csv.NewWriter (se chegou lá) ou 500, mas
`Content-Disposition: attachment` está presente em todos os casos de erro pós-headers, então o browser
**força o download de um arquivo .csv contendo a mensagem de erro** em vez de exibir o status HTTP.
Isso também vaza mensagens de erro internas para o cliente.

**Fix:**
Mover as linhas de `Content-Type` e `Content-Disposition` para **depois** de toda a lógica de negócio (após
`rows.Err()` ser verificado), imediatamente antes de `cw := csv.NewWriter(w)`.

```go
// ANTES (errado): headers definidos antes da query
w.Header().Set("Content-Type", "text/csv; charset=utf-8")
w.Header().Set("Content-Disposition", `attachment; filename="analise-cfop.csv"`)

rows, err := db.Query(...)
if err != nil {
    http.Error(w, "Erro ao consultar dados", http.StatusInternalServerError) // Content-Disposition ainda presente!
    return
}
// ...
if err := rows.Err(); err != nil {
    http.Error(w, "Erro ao ler dados", http.StatusInternalServerError) // Content-Disposition ainda presente!
    return
}

// DEPOIS (correto): headers definidos apenas quando tudo está OK
w.Header().Set("Content-Type", "text/csv; charset=utf-8")
w.Header().Set("Content-Disposition", `attachment; filename="analise-cfop.csv"`)
cw := csv.NewWriter(w)
// escrever header e linhas...
```

---

### CR-02: CSV handlers não verificam `userID` vazio — retornam HTTP 500 em vez de 401 para JWTs com `user_id` ausente ou de tipo errado

**Arquivo:** `backend/handlers/reforma_modulo2.go:233` (CfopAnalysisCSVHandler) e `:440` (NcmAnalysisCSVHandler)

**Issue:**
Os handlers JSON (`CfopAnalysisHandler`, `NcmAnalysisHandler`, `UfDestinoHandler`, `B2bB2cHandler`) verificam
corretamente que `userID` não é vazio:
```go
userID, ok2 := claims["user_id"].(string)
if !ok2 || userID == "" {
    jsonErr(w, http.StatusUnauthorized, "Unauthorized")
    return
}
```

Já os dois handlers CSV usam:
```go
userID, _ := claims["user_id"].(string)
```

Se o claim `user_id` estiver ausente ou não for `string`, `userID` será `""`.
`GetEffectiveCompanyID(db, "", ...)` consulta `WHERE owner_id = ''`, não encontra nenhuma empresa, e retorna
`sql.ErrNoRows` — o handler responde com **HTTP 500** (Erro ao obter empresa).
O comportamento correto é **401 Unauthorized**.
Um token JWT forjado sem `user_id` obtém um 500 que pode ser explorado como oráculo.

**Fix:**
```go
// Adicionar após a extração do claim em ambos os CSV handlers:
userID, _ := claims["user_id"].(string)
if userID == "" {
    http.Error(w, "Não autenticado", http.StatusUnauthorized)
    return
}
```

---

## Warnings

### WR-01: Query NCM agrupa por `nit.x_prod` — mesmo NCM pode gerar múltiplas linhas; LIMIT 100 aplica-se a pares (ncm, x_prod), não a NCMs distintos

**Arquivo:** `backend/handlers/reforma_modulo2.go:379` e `:478`

**Issue:**
```sql
GROUP BY nit.ncm, nit.x_prod, ncmr.ibs_reducao_pct, ncmr.cbs_reducao_pct, ncmr.cclasstrib
ORDER BY vl_prod DESC
LIMIT 100
```

O campo `x_prod` é a descrição do produto **no item da NF-e**. O mesmo NCM pode ter dezenas de descrições
diferentes em notas distintas, gerando uma linha por combinação `(ncm, x_prod)`.
O frontend exibe "Limitado aos 100 NCMs de maior volume", mas o LIMIT 100 corta em pares `(ncm, x_prod)`.
Uma empresa com 40 NCMs e 3+ descrições por NCM jamais veria todos os NCMs.
O volume consolidado por NCM fica fragmentado, distorcendo a análise fiscal.

**Fix:**
Agregar apenas por NCM (e dados de redução IBS/CBS) usando `MIN(x_prod)` ou `MAX(x_prod)` para preservar uma descrição representativa:
```sql
SELECT
  nit.ncm,
  MAX(nit.x_prod) AS x_prod,          -- descrição mais recente como representativa
  SUM(nit.v_prod) AS vl_prod,
  SUM(nit.v_icms) AS vl_icms,
  COALESCE(ncmr.ibs_reducao_pct, 0) AS ibs_reducao_pct,
  COALESCE(ncmr.cbs_reducao_pct, 0) AS cbs_reducao_pct,
  CASE WHEN ncmr.cclasstrib IS NOT NULL THEN true ELSE false END AS is_flag
FROM ...
GROUP BY nit.ncm, ncmr.ibs_reducao_pct, ncmr.cbs_reducao_pct, ncmr.cclasstrib
ORDER BY vl_prod DESC
LIMIT 100
```

---

### WR-02: `colorScale` em `Reforma23UfDestino.tsx` usa valor de `g` errado — cor mínima renderiza roxo em vez de azul claro

**Arquivo:** `frontend/src/pages/Reforma23UfDestino.tsx:49-53`

**Issue:**
O comentário documenta que a interpolação vai de `#dbeafe` a `#1d4ed8`, mas o código usa valor errado para o
canal verde no ponto de partida:

```ts
// Comentário: from: #dbeafe (219, 190, 254)  <- ERRADO
// #dbeafe real = rgb(219, 234, 254)           <- g=234, não 190
const g = Math.round(190 + (78 - 190) * t)    // usa 190, deveria ser 234
```

O valor correto de `#dbeafe` é `rgb(219, 234, 254)` (g=234). Com g=190, o valor mínimo renderiza
`rgb(219, 190, 254)` — um tom roxo/lavanda —em vez do azul claro esperado (`#dbeafe`).
O mapa coroplético mostrará cores incorretas para estados com menor volume.

**Fix:**
```ts
// from: #dbeafe (219, 234, 254) to: #1d4ed8 (29, 78, 216)
const r = Math.round(219 + (29 - 219) * t)
const g = Math.round(234 + (78 - 234) * t)  // 234, não 190
const b = Math.round(254 + (216 - 254) * t)
```

---

### WR-03: `handleExportCSV` ignora erros silenciosamente — usuário não recebe feedback em caso de falha

**Arquivo:** `frontend/src/pages/Reforma22CfopAnalysis.tsx:93-95` e `Reforma21NcmAnalysis.tsx:95-97`

**Issue:**
```ts
} catch (_err) {
  // silent
}
```

Se o download CSV falhar (rede, servidor 500, etc.), o usuário não recebe nenhuma mensagem.
O botão volta ao estado "Exportar CSV" sem indicar que houve erro.
Para uma ferramenta fiscal com dados sensíveis, falhas silenciosas são inaceitáveis — o usuário pode
assumir que o arquivo foi gerado corretamente.

**Fix:**
```ts
} catch (err) {
  // Usar toast, alert ou state de erro para notificar o usuário
  console.error('Falha ao exportar CSV:', err)
  // Exemplo com toast (padrão do projeto):
  // toast.error('Falha ao exportar CSV. Tente novamente.')
}
```

---

### WR-04: Âncora de download criada fora do DOM pode não funcionar no Firefox

**Arquivo:** `frontend/src/pages/Reforma22CfopAnalysis.tsx:88-92` e `Reforma21NcmAnalysis.tsx:90-94`

**Issue:**
```ts
const a = document.createElement('a')
a.href = url
a.download = 'analise-cfop.csv'
a.click()
URL.revokeObjectURL(url)
```

O elemento `<a>` é criado e clicado sem ser inserido no DOM.
Navegadores baseados em Chromium aceitam isso, mas **Firefox requer que o elemento esteja no DOM**
para que `click()` dispare o download via `Blob URL`.
O projeto já usa esse padrão em outros lugares (Reforma11, etc.), portanto é um problema de projeto, mas
como este é código novo, deve ser corrigido aqui.

**Fix:**
```ts
document.body.appendChild(a)
a.click()
document.body.removeChild(a)
URL.revokeObjectURL(url)
```

---

## Info

### IN-01: `console.log` de versão em `App.tsx` permanece em produção

**Arquivo:** `frontend/src/App.tsx:228`

**Issue:**
```ts
console.log('App Version: 1.0.0 — FB_APU04 Simulador da Reforma Tributária - SPED')
```

Este `console.log` executa a cada re-render do componente `App` (frequente em HMR/dev e no mount inicial
em produção). Expõe informação da versão nos devtools do browser.

**Fix:** Remover ou mover para uma variável de ambiente checada apenas em dev:
```ts
if (import.meta.env.DEV) {
  console.log('App Version: ...')
}
```

---

### IN-02: Testes não cobrem cenário de `Unauthorized` (claims ausentes) para os handlers CSV

**Arquivo:** `backend/handlers/reforma_modulo2_test.go`

**Issue:**
Os testes validam apenas criação de handler (`nil` DB) e rejeição de método HTTP não permitido (POST).
Nenhum teste cobre o cenário de request sem JWT (sem claims no contexto) para os handlers CSV.
O bug CR-02 não seria detectado por esta suite.

**Fix (sugestão):**
Adicionar testes como:
```go
func TestCfopAnalysisCSVHandler_Unauthorized(t *testing.T) {
    h := CfopAnalysisCSVHandler(nil)
    req := httptest.NewRequest(http.MethodGet, "/api/reforma/modulo2/cfop/csv", nil)
    // request sem ClaimsKey no contexto
    rr := httptest.NewRecorder()
    h(rr, req)
    if rr.Code != http.StatusUnauthorized {
        t.Errorf("got %d, want 401", rr.Code)
    }
}
```

---

### IN-03: Módulos 2.3 (UF Destino) e 2.4 (B2B/B2C) não possuem exportação CSV — inconsistência com os demais módulos

**Arquivo:** `backend/main.go` (ausência de rotas), `frontend/src/pages/Reforma23UfDestino.tsx`, `frontend/src/pages/Reforma24B2bB2c.tsx`

**Issue:**
Os módulos 2.2 (CFOP) e 2.1 (NCM) têm handlers CSV registrados e botão de exportação nas páginas.
Os módulos 2.3 e 2.4 não possuem handler CSV no backend nem botão de exportação nas páginas front-end.
A inconsistência pode causar confusão ao usuário e foi provavelmente um item esquecido no escopo da fase.

**Fix:**
Implementar `UfDestinoCSVHandler` e `B2bB2cCSVHandler` no backend seguindo o padrão dos handlers existentes,
registrá-los em `main.go` e adicionar o botão "Exportar CSV" nos respectivos componentes React.

---

_Revisado em: 2026-05-23_
_Revisor: Claude (gsd-code-reviewer)_
_Profundidade: standard_
