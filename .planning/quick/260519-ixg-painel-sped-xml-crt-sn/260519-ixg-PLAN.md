---
phase: quick-260519-ixg
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - frontend/src/components/AppSidebar.tsx
  - frontend/src/lib/navigation.ts
  - frontend/src/pages/Mercadorias.tsx
  - frontend/src/pages/MercadoriasXML.tsx
  - frontend/src/App.tsx
  - frontend/src/components/AppRail.tsx
  - backend/handlers/xml_painel.go
  - backend/main.go
  - backend/handlers/xml_upload.go
  - backend/handlers/nfe_saidas.go
autonomous: true
requirements: []

must_haves:
  truths:
    - Section title "Simulador da Reforma Tributária" appears as "Simulador da Reforma Tributária - SPED" everywhere it currently shows (sidebar, module tabs label)
    - Card "Total de Entradas" inside /mercadorias shows a new row "IPI (Informativo)" below "Total Créditos", with value pulled from XML (vw_xml_entradas_resumo.v_ipi) for the months currently selected
    - Same card shows a new row "PIS/COFINS Fornecedores Simples Nacional (Informativo)" with sum of (v_pis + v_cofins) from XML entradas WHERE forn_cnpj IS IN forn_simples for the selected competências
    - Both informative values are 0 if no XML entradas exist for the period (no error)
    - New route /mercadorias/xml renders a page titled "Simulador da Reforma Tributária - XMLs" with same UI as /mercadorias but data sourced from XML tables
    - The new page has NO "Importar SPEDs" tab/feature visible
    - Sidebar shows the new entry "Operações Comerciais (XMLs)" under the Simulador section pointing to /mercadorias/xml
    - When uploading an XML de saída, the system reads <emit><CRT> and updates the matching company.regime_tributario based on the mapping table CRT=1→simples_nacional, CRT=2→simples_nacional, CRT=3→lucro_real (fallback when not lucro_presumido)
    - If emitente CNPJ does not match any filial_apelidos.cnpj for any company, the upload completes without error and regime stays unchanged
  artifacts:
    - path: frontend/src/pages/MercadoriasXML.tsx
      provides: New page duplicated from Mercadorias.tsx, sourced from XML
      contains: "Simulador da Reforma Tributária - XMLs"
    - path: backend/handlers/xml_painel.go
      provides: Endpoint /api/xml/painel/entradas-informativos returning aggregated IPI and PIS/COFINS-SN per filial+mes_ano
    - path: backend/migrations/080_create_vw_xml_entradas_informativos.sql
      provides: View aggregating IPI and PIS/COFINS from XML entradas with forn_simples join
  key_links:
    - from: frontend/src/pages/Mercadorias.tsx
      to: /api/xml/painel/entradas-informativos
      via: useEffect fetch with X-Company-ID
      pattern: "fetch\\(.*xml/painel/entradas-informativos"
    - from: backend/handlers/xml_upload.go
      to: companies.regime_tributario UPDATE
      via: UPDATE via filial_apelidos lookup
      pattern: "UPDATE companies SET regime_tributario"
---

<objective>
Three small UX/data changes to the Simulador da Reforma Tributária module:

1. Rename the section label everywhere to distinguish the existing SPED-based panel ("- SPED") from a new XML-based clone ("- XMLs").
2. Inside the existing SPED panel (`/mercadorias`), add two informative rows in the "Total de Entradas" card: IPI from XML and PIS/COFINS of Simples Nacional fornecedores from XML — both pulled directly from the XML tables (`nfe_entradas` with `source` of `xml_upload`), not via JOIN with SPED notes.
3. Duplicate `/mercadorias` → `/mercadorias/xml` ("Simulador da Reforma Tributária - XMLs"), removing the "Importar SPEDs" tab; the new page reads only from XML tables.
4. When importing XML de saída, detect `<emit><CRT>` and auto-update `companies.regime_tributario` for the matching empresa cadastrada (silently skip if CNPJ has no match).

Purpose: Begin separating SPED-based analyses from XML-based analyses; lay the groundwork for richer Simples Nacional analyses (future, captured in `260519-ixg-deferred-items.md`).
Output: Renamed labels, new informative rows, new cloned page, automatic regime detection on XML saída import.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md

@frontend/src/components/AppSidebar.tsx
@frontend/src/lib/navigation.ts
@frontend/src/pages/Mercadorias.tsx
@frontend/src/components/AppRail.tsx
@frontend/src/App.tsx
@frontend/src/pages/Login.tsx

@backend/handlers/xml_painel.go
@backend/handlers/xml_upload.go
@backend/handlers/nfe_saidas.go
@backend/handlers/nfe_entradas.go
@backend/migrations/078_create_vw_xml_panels.sql
@backend/migrations/077_add_regime_tributario_to_companies.sql
@backend/migrations/057_create_filial_apelidos.sql
@backend/migrations/040_create_forn_simples_table.sql
@backend/main.go

<interfaces>
<!-- Key contracts extracted from codebase. Executor should not re-explore. -->

From backend/handlers/nfe_saidas.go (parser already extracts CRT):
```go
type emit struct {
    CNPJ      string    `xml:"CNPJ"`
    XNome     string    `xml:"xNome"`
    CRT       string    `xml:"CRT"`        // "1"=Simples Nacional, "2"=SN excesso sublimite, "3"=Regime Normal
    EnderEmit enderEmit `xml:"enderEmit"`
}
```

From backend/handlers/xml_upload.go (saídas insertion happens at line 289 `case "saidas":` and CRT check at line 364-370 — the saidas branch already commits the tx before the CRT block; in saídas the emit IS the empresa cadastrada):
```go
// Around line 364, after tx.Commit() in the saidas path,
// add a regime_tributario auto-update keyed off inf.Emit.CRT and inf.Emit.CNPJ.
```

From backend/migrations/077_add_regime_tributario_to_companies.sql:
```sql
ALTER TABLE companies
  ADD COLUMN IF NOT EXISTS regime_tributario TEXT NOT NULL DEFAULT 'nao_informado';
-- CHECK constraint accepts: lucro_real, lucro_presumido, simples_nacional, nao_informado
```

From backend/migrations/057_create_filial_apelidos.sql (the join path company<->CNPJ):
```sql
CREATE TABLE filial_apelidos (
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    cnpj       VARCHAR(14) NOT NULL,
    ...
);
```

From backend/migrations/078_create_vw_xml_panels.sql (vw_xml_entradas_resumo already exposes v_ipi + v_pis + v_cofins from nfe_entradas):
```sql
CREATE VIEW vw_xml_entradas_resumo AS
SELECT company_id, forn_cnpj, forn_nome, mes_ano, source,
       SUM(COALESCE(ne.v_ipi, 0)) AS v_ipi,
       SUM(COALESCE(ne.v_pis, 0)) AS v_pis,
       SUM(COALESCE(ne.v_cofins, 0)) AS v_cofins,
       ...
FROM nfe_entradas ne
GROUP BY company_id, forn_cnpj, forn_nome, mes_ano, source;
```

From backend/migrations/040_create_forn_simples_table.sql:
```sql
CREATE TABLE forn_simples (cnpj VARCHAR(14) PRIMARY KEY);
-- 14-digit CNPJs of Simples Nacional fornecedores
```

From frontend/src/pages/Mercadorias.tsx (the IPI block already exists at lines 591-596, sourced from /api/nfe-entradas/impostos — that endpoint reads vw_nfe_entradas_impostos, which already filters nfe_entradas. We will REPLACE that data source with the new XML-only view to honor "XML, not via JOIN with SPED notes" — the existing view already reads only nfe_entradas which is XML-fed; the new endpoint will additionally produce v_pis_simples + v_cofins_simples):
```ts
const ipiNFe = nfeImpostos.filter(...).reduce((s, r) => s + r.total_ipi, 0)
// Add: const pisCofinsSimplesNFe = ...
```

From frontend/src/lib/navigation.ts (simulador module label and tabs):
```ts
simulador: {
  label: 'Simulador da Reforma Tributária',  // → rename to "...- SPED"
  tabs: [
    { label: 'Importar SPEDs', path: '/importar-efd' },
    { label: 'Operações Comerciais', path: '/mercadorias' },
    ...
  ]
}
```

From frontend/src/components/AppSidebar.tsx (line 80-91):
```ts
{
  id: "simulador",
  title: "Simulador da Reforma Tributária",  // → rename to "...- SPED"
  items: [
    { title: "Importar SPEDs", url: "/importar-efd", ... },
    { title: "Operações Comerciais", url: "/mercadorias", ... },
    ...
  ]
}
```
</interfaces>
</context>

<tasks>

<task type="auto">
  <name>Task 1: Rename label, add XML informative rows in /mercadorias (Painel Simulador RT - SPED)</name>
  <files>
    frontend/src/components/AppSidebar.tsx,
    frontend/src/lib/navigation.ts,
    frontend/src/components/AppRail.tsx,
    frontend/src/pages/Login.tsx,
    frontend/src/App.tsx,
    frontend/src/pages/Mercadorias.tsx,
    backend/migrations/080_create_vw_xml_entradas_informativos.sql,
    backend/handlers/xml_painel.go,
    backend/main.go
  </files>
  <action>
    PART A — Rename label only (NOT routes, NOT component names, NOT module ID `simulador`):
    - AppSidebar.tsx line 81: change `title: "Simulador da Reforma Tributária"` to `title: "Simulador da Reforma Tributária - SPED"`.
    - AppSidebar.tsx line 206 (the visible heading inside the sidebar group): change "Simulador da Reforma Tributária" to "Simulador da Reforma Tributária - SPED".
    - navigation.ts line 16: change `label: 'Simulador da Reforma Tributária'` to `label: 'Simulador da Reforma Tributária - SPED'`.
    - AppRail.tsx line 34: keep `label: 'Simulador RT'` BUT replace with `label: 'Simulador RT - SPED'` (the short label that appears on the icon-rail tooltip).
    - Login.tsx line 111: change "Simulador da Reforma Tributária" to "Simulador da Reforma Tributária - SPED".
    - App.tsx line 148 comment and line 190 console.log: append " - SPED" so logs and code-comments stay consistent.
    - Do NOT touch module IDs (`simulador`), route paths, file names, or component class names.

    PART B — Backend: create XML-only informativos view + endpoint:
    - Create `backend/migrations/080_create_vw_xml_entradas_informativos.sql`:
      ```sql
      CREATE OR REPLACE VIEW vw_xml_entradas_informativos AS
      SELECT
        ne.company_id,
        ne.mes_ano,
        SUM(COALESCE(ne.v_ipi, 0))                                                                AS total_ipi,
        SUM(CASE WHEN fs.cnpj IS NOT NULL THEN COALESCE(ne.v_pis, 0)    ELSE 0 END)               AS total_pis_simples,
        SUM(CASE WHEN fs.cnpj IS NOT NULL THEN COALESCE(ne.v_cofins, 0) ELSE 0 END)               AS total_cofins_simples,
        COUNT(*)                                                                                  AS qtd_notas
      FROM nfe_entradas ne
      LEFT JOIN forn_simples fs ON fs.cnpj = REGEXP_REPLACE(ne.forn_cnpj, '[^0-9]', '', 'g')
      WHERE ne.source = 'xml_upload'           -- XML-only, NOT JOIN with SPED
      GROUP BY ne.company_id, ne.mes_ano;
      ```
      Note: filtering `source = 'xml_upload'` makes this an independent query from SPED tables — fulfills the constraint "from XML tables, NOT via JOIN with SPED notes". The view aggregates per competência (mes_ano), no filial_cnpj since the spec asks for values "same competência as the SPED".
    - In `backend/handlers/xml_painel.go`, add a new handler `XMLEntradasInformativosHandler(db *sql.DB) http.HandlerFunc` that:
      - Auth: identical pattern to existing XMLPainelHandler in the same file (JWT claims, GetEffectiveCompanyID).
      - Query: `SELECT mes_ano, total_ipi, total_pis_simples, total_cofins_simples, qtd_notas FROM vw_xml_entradas_informativos WHERE company_id = $1 ORDER BY mes_ano`.
      - Response shape:
        ```json
        { "total": N, "items": [{ "mes_ano": "MM/YYYY", "total_ipi": 0, "total_pis_simples": 0, "total_cofins_simples": 0, "qtd_notas": 0 }] }
        ```
    - In `backend/main.go` register the route alongside the other `/api/xml/painel/*` routes:
      `http.HandleFunc("/api/xml/painel/entradas-informativos", withAuth(handlers.XMLEntradasInformativosHandler(db), ""))`

    PART C — Frontend: render the two new informative rows in Mercadorias.tsx "Total de Entradas" card:
    - At the existing `nfeImpostos` useEffect (line 121-129), add a SECOND useEffect that fetches `/api/xml/painel/entradas-informativos` and stores items in a new state `xmlInformativos` (typed `{ mes_ano: string; total_ipi: number; total_pis_simples: number; total_cofins_simples: number; qtd_notas: number }[]`).
    - Compute below the existing `ipiNFe` (line 379) two new sums respecting the current `selectedMonth` filter:
      ```ts
      const ipiXML = xmlInformativos
        .filter(r => selectedMonth === 'all' || r.mes_ano === selectedMonth)
        .reduce((s, r) => s + r.total_ipi, 0)
      const pisCofinsSimplesXML = xmlInformativos
        .filter(r => selectedMonth === 'all' || r.mes_ano === selectedMonth)
        .reduce((s, r) => s + r.total_pis_simples + r.total_cofins_simples, 0)
      ```
    - In the "Total de Entradas" card (lines 575-619), AFTER the existing "Total Créditos:" row (line 613-616) — i.e., BELOW it — add:
      ```tsx
      <div className="flex justify-between pt-2">
        <span className="text-gray-500">IPI (Informativo):</span>
        <span className="font-medium text-gray-400">{formatCurrency(ipiXML)}</span>
      </div>
      <div className="flex justify-between">
        <span className="text-gray-500">PIS/COFINS Fornecedores Simples Nacional (Informativo):</span>
        <span className="font-medium text-gray-400">{formatCurrency(pisCofinsSimplesXML)}</span>
      </div>
      ```
      Keep the existing `ipiNFe` row (lines 591-596) untouched — it sources from /api/nfe-entradas/impostos which mixes Bridge+XML; the NEW rows are the XML-only informativos.
    - If the fetch fails or returns no items, the values stay 0 — render the rows anyway (zero is a valid informativo).
  </action>
  <verify>
    <automated>
      cd backend && go build ./... &&
      cd ../frontend && npm run build &&
      grep -c "Simulador da Reforma Tributária - SPED" frontend/src/components/AppSidebar.tsx frontend/src/lib/navigation.ts frontend/src/pages/Login.tsx &&
      grep -c "vw_xml_entradas_informativos" backend/migrations/080_create_vw_xml_entradas_informativos.sql backend/handlers/xml_painel.go &&
      grep -c "/api/xml/painel/entradas-informativos" backend/main.go frontend/src/pages/Mercadorias.tsx &&
      grep -c "IPI (Informativo)\|PIS/COFINS Fornecedores Simples Nacional (Informativo)" frontend/src/pages/Mercadorias.tsx
    </automated>
  </verify>
  <done>
    - Backend compiles, frontend builds.
    - Sidebar, module-tabs, login screen all show "Simulador da Reforma Tributária - SPED".
    - Migration 080 file exists and the view aggregates IPI + PIS/COFINS-SN from `nfe_entradas WHERE source='xml_upload'` only.
    - New endpoint `/api/xml/painel/entradas-informativos` registered in `main.go`.
    - "Total de Entradas" card on `/mercadorias` displays two extra informative rows beneath "Total Créditos:".
    - With no XML data: rows render with R$ 0,00 (no console error, no crash).
  </done>
</task>

<task type="auto">
  <name>Task 2: Duplicate Painel into /mercadorias/xml (Simulador da Reforma Tributária - XMLs)</name>
  <files>
    frontend/src/pages/MercadoriasXML.tsx,
    frontend/src/App.tsx,
    frontend/src/components/AppSidebar.tsx,
    frontend/src/lib/navigation.ts,
    backend/handlers/xml_painel.go,
    backend/handlers/xml_reports.go,
    backend/main.go,
    backend/migrations/081_create_vw_xml_operacoes_resumo.sql
  </files>
  <action>
    PART A — Backend: aggregate XML data the same shape as `/api/reports/mercadorias`:
    - Create migration `backend/migrations/081_create_vw_xml_operacoes_resumo.sql` with a view `vw_xml_operacoes_resumo` exposing the same columns the frontend `AggregatedData` interface expects (filial_cnpj, filial_nome, mes_ano, valor, icms, vl_ipi, vl_pis, vl_cofins, vl_icms_projetado=0, vl_ibs_projetado=0, vl_cbs_projetado=0, tipo IN ('ENTRADA','SAIDA'), tipo_cfop, origem, tipo_operacao):
      - Entradas: from `nfe_entradas WHERE source='xml_upload'`. Map filial_cnpj=`dest_cnpj_cpf` (own filial), filial_nome=`dest_nome`. tipo='ENTRADA', tipo_operacao='Entrada_XML' (single bucket — XML doesn't have CFOP-tipo classification like SPED).
      - Saídas: from `nfe_saidas WHERE source='xml_upload'`. filial_cnpj=`emit_cnpj`, filial_nome=`emit_nome`. tipo='SAIDA', tipo_operacao='Saida_XML'.
      - Leave vl_ibs_projetado / vl_cbs_projetado as the actual XML values when present (`v_ibs`, `v_cbs`) so the simulator math still works against currently imported XML data; ICMS projetado stays 0 since the view does not run the SPED projection logic.
    - Add a new handler `MercadoriasXMLReportHandler` in `backend/handlers/xml_reports.go` (file already exists in repo) that returns the rows from `vw_xml_operacoes_resumo` for the current company in the same JSON array shape as `/api/reports/mercadorias`. Accept `target_year` and `tipo_operacao` query params for parity but treat `tipo_operacao=todos` as no filter.
    - Register route in main.go: `http.HandleFunc("/api/xml/reports/mercadorias", withAuth(handlers.MercadoriasXMLReportHandler(db), ""))`.

    PART B — Frontend: duplicate Mercadorias.tsx → MercadoriasXML.tsx:
    - `cp frontend/src/pages/Mercadorias.tsx frontend/src/pages/MercadoriasXML.tsx` then:
      - Rename the exported component from `Mercadorias` to `MercadoriasXML`.
      - Replace the h1 (line 490) text "Comparativo de impostos atuais com IBS e CBS" with "Simulador da Reforma Tributária - XMLs". Keep the layout, cards, filters, chart unchanged.
      - Replace fetch URL `/api/reports/mercadorias?...` (line 137) with `/api/xml/reports/mercadorias?...`.
      - Replace the secondary fetch `/api/nfe-entradas/impostos` (line 123) and `/api/xml/painel/entradas-informativos` (added in Task 1) — combine them so the page consumes ONLY the XML-only informativos endpoint added in Task 1 (drop the nfe-entradas/impostos call entirely from this new page).
      - Remove the "Reconstruir Painel" button (lines 517-529): the XML page has no materialized views to refresh.
      - Keep the IPI and PIS/COFINS-SN informativos rows from Task 1 (they're already XML-sourced).
    - App.tsx: add a new route inside the "Simulador da Reforma Tributária" Routes block (around line 150):
      `<Route path="/mercadorias/xml" element={<MercadoriasXML />} />`
      Import `MercadoriasXML` from `./pages/MercadoriasXML` at the top of App.tsx with the other page imports.
    - AppSidebar.tsx: add a new menu item inside the `simulador` section items[] (after "Operações Comerciais", line 85):
      `{ title: "Operações Comerciais (XMLs)", url: "/mercadorias/xml", icon: ShoppingCart },`
    - navigation.ts: add a new tab inside `modules.simulador.tabs` after "Operações Comerciais":
      `{ label: 'Operações Comerciais (XMLs)', path: '/mercadorias/xml' },`
    - Do NOT add any "Importar SPEDs" reference in any new code — the new page must not have/imply that tab. The `simulador` module tabs list will still contain "Importar SPEDs" because the SPED panel still uses it, but the new page itself has no internal tab/feature referencing SPED upload.

    PART C — Module identity:
    - `getActiveModule` in `navigation.ts` already maps `/mercadorias` → `simulador` (line 62: `pathname.startsWith('/mercadorias')` covers `/mercadorias/xml` too). No change needed there.
  </action>
  <verify>
    <automated>
      cd backend && go build ./... &&
      cd ../frontend && npm run build &&
      test -f frontend/src/pages/MercadoriasXML.tsx &&
      grep -c "Simulador da Reforma Tributária - XMLs" frontend/src/pages/MercadoriasXML.tsx &&
      grep -c "MercadoriasXML" frontend/src/App.tsx &&
      grep -c "/mercadorias/xml" frontend/src/components/AppSidebar.tsx frontend/src/lib/navigation.ts &&
      grep -c "vw_xml_operacoes_resumo" backend/migrations/081_create_vw_xml_operacoes_resumo.sql backend/handlers/xml_reports.go &&
      grep -c "/api/xml/reports/mercadorias" backend/main.go frontend/src/pages/MercadoriasXML.tsx &&
      ! grep -q "Importar SPED\|importar-efd\|Reconstruir Painel" frontend/src/pages/MercadoriasXML.tsx
    </automated>
  </verify>
  <done>
    - `/mercadorias/xml` renders the new page with h1 "Simulador da Reforma Tributária - XMLs".
    - Sidebar shows the new "Operações Comerciais (XMLs)" entry; module-tabs row shows the same.
    - Page has no "Importar SPEDs" tab/button, no "Reconstruir Painel" button, no SPED endpoint calls.
    - Backend endpoint `/api/xml/reports/mercadorias` returns rows aggregated from XML tables only.
    - Existing `/mercadorias` page continues to work unchanged.
  </done>
</task>

<task type="auto">
  <name>Task 3: Auto-detect CRT from XML saída and update companies.regime_tributario</name>
  <files>
    backend/handlers/xml_upload.go,
    backend/handlers/nfe_saidas.go,
    backend/handlers/admin.go,
    backend/handlers/xml_upload_test.go
  </files>
  <action>
    Helper function: add `updateCompanyRegimeFromCRT(db *sql.DB, companyID, emitCNPJ, crt string)` to a shared spot — recommend adding it at the end of `backend/handlers/xml_upload.go` (since both single-XML and batch import paths live in that file). Behavior:
    - Trim whitespace from emitCNPJ and crt; if either is empty → return silently (no error).
    - Map CRT codes to regime values (CHECK constraint of `companies.regime_tributario` accepts: lucro_real, lucro_presumido, simples_nacional, nao_informado):
      - "1" → "simples_nacional"
      - "2" → "simples_nacional"  (SN excesso de sublimite still é Simples)
      - "3" → "lucro_real"        (CRT=3 covers Regime Normal — LR/LP; default to lucro_real per spec; the user can manually change to lucro_presumido in GestaoAmbiente if appropriate. Document this in a code comment.)
      - any other value → return silently.
    - Find the matching company via filial_apelidos (the only table that joins companies to CNPJs):
      ```sql
      UPDATE companies
      SET regime_tributario = $1, updated_at = NOW()
      WHERE id IN (
        SELECT DISTINCT company_id
        FROM filial_apelidos
        WHERE cnpj = $2
      )
      AND regime_tributario IS DISTINCT FROM $1;
      ```
      Use `db.Exec` and DROP the err (`_, _ = db.Exec(...)`) — failures must not block XML import. Log a single line on success (number of rows affected) and on real DB error: `log.Printf("[CRT-Detect] emit=%s crt=%s err=%v", emitCNPJ, crt, err)`.
    - The constraint "if CNPJ not found in empresa table, silently skip" is naturally satisfied — the UPDATE matches zero rows when filial_apelidos has no entry for the CNPJ.

    Hook points (call the helper only on the SAÍDA path, never entradas):
    1. `backend/handlers/xml_upload.go` `processSingleXML` — inside `case "saidas":` branch, AFTER the existing `inf.Emit.CRT == "1"` forn_simples block (around line 364-370) but BEFORE `return tx.Commit()`. Add:
       ```go
       // Auto-detectar regime tributário da empresa a partir do CRT do XML de saída.
       // Em saídas, o emitente é a própria empresa (filial cadastrada).
       updateCompanyRegimeFromCRT(db, companyID, inf.Emit.CNPJ, inf.Emit.CRT)
       ```
       Place this AFTER `tx.Commit()` (do not run inside the transaction; if the update fails it must not roll back the nota).
    2. `backend/handlers/nfe_saidas.go` — the batch import path at line 631-636 (`if strings.TrimSpace(inf.Emit.CRT) == "1"` for forn_simples). Add the call to `updateCompanyRegimeFromCRT` immediately after that block, AFTER `tx.Commit()` succeeds (around line 638-640). This covers the legacy synchronous batch upload that doesn't go through `processSingleXML`.
    3. Do NOT add the call to entrada (`processSingleXML` `case "entradas"`) — the emitente of an entrada is the fornecedor, not the empresa cadastrada.

    Test: add `backend/handlers/xml_upload_test.go` with a unit test `TestUpdateCompanyRegimeFromCRT` that uses a stub DB (testify/sqlmock if already in go.mod; otherwise write a minimal table-driven test that just asserts the SQL string is what we expect, e.g., by structuring the helper to return the (sql, args) pair from an internal `buildRegimeUpdate` function called by the public helper). Cases:
    - CRT="1" → expects regime "simples_nacional".
    - CRT="2" → expects "simples_nacional".
    - CRT="3" → expects "lucro_real".
    - CRT="" / CRT="9" / CNPJ empty → helper returns without invoking DB.
    Check `go list -m github.com/DATA-DOG/go-sqlmock` first; if absent, use the build-string-only approach (no new deps).
  </action>
  <verify>
    <automated>
      cd backend && go build ./... &&
      go test ./handlers/ -run TestUpdateCompanyRegimeFromCRT -v &&
      grep -c "updateCompanyRegimeFromCRT" backend/handlers/xml_upload.go backend/handlers/nfe_saidas.go &&
      grep -c "UPDATE companies" backend/handlers/xml_upload.go &&
      grep -c "filial_apelidos" backend/handlers/xml_upload.go &&
      ! grep -c "updateCompanyRegimeFromCRT" backend/handlers/nfe_entradas.go
    </automated>
  </verify>
  <done>
    - `go build` passes; new unit test passes.
    - Saída XML upload triggers the helper on both async (`processSingleXML`) and sync (`nfe_saidas.go`) paths.
    - Entrada XML upload does NOT trigger the helper.
    - If `inf.Emit.CNPJ` is not present in any `filial_apelidos` row, UPDATE matches 0 rows — no error surfaced to caller, no panic.
    - Existing forn_simples insertion (CRT=1 path) still fires alongside the new regime update.
  </done>
</task>

</tasks>

<verification>
- Build: `cd backend && go build ./...` AND `cd frontend && npm run build` both succeed.
- Migrations: 080 and 081 apply cleanly to a fresh DB (`psql -f`). They are CREATE OR REPLACE / CREATE IF NOT EXISTS so re-running is idempotent.
- Manual smoke (one minute each):
  1. Open `/mercadorias` — verify "Total de Entradas" card shows the two extra rows; values are 0 if no XML data is present.
  2. Open `/mercadorias/xml` — verify h1 "Simulador da Reforma Tributária - XMLs", no "Importar SPEDs" or "Reconstruir Painel" buttons.
  3. Upload one XML de saída with CRT=1 from an emitente whose CNPJ is in `filial_apelidos`; confirm `SELECT regime_tributario FROM companies WHERE id=<that company>` returns `simples_nacional`.
  4. Upload one XML de saída with an emitente CNPJ NOT in `filial_apelidos`; confirm import succeeds and no company row was changed.
- Regression: existing `/mercadorias` panel still loads SPED data; existing `/painel/xmls` still loads.
</verification>

<success_criteria>
- All three tasks pass automated verify.
- Sidebar/module-tabs reflect the "- SPED" rename.
- `/mercadorias/xml` is reachable, named correctly, and SPED-free.
- CRT detection writes to `companies.regime_tributario` only when CNPJ matches an `filial_apelidos` row.
- No existing endpoint, route, or component name was renamed.
- Item 4 (Simples Nacional analysis ideas) lives only in `260519-ixg-deferred-items.md` — no implementation code was written for it.
</success_criteria>

<output>
Create `.planning/quick/260519-ixg-painel-sped-xml-crt-sn/260519-ixg-SUMMARY.md` when done.
</output>
