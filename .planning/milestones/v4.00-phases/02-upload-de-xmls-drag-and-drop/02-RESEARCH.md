# Phase 02: Upload de XMLs (Drag-and-Drop) — Research

**Researched:** 2026-05-16
**Domain:** NFe/CTe XML parsing (Go), drag-and-drop upload (React), PostgreSQL schema migration, background job processing
**Confidence:** HIGH — maioria das findings são VERIFIED via inspeção direta do código-fonte do projeto

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- Sistema usado por **escritórios de contabilidade** cadastrando clientes (empresas) por CNPJ.
- Tenancy: Ambiente "Servidor Senior Excelente" > Grupo > Empresas (clientes do escritório).
- Ao cadastrar empresa, identificar **regime tributário**: Lucro Real, Lucro Presumido ou Simples Nacional.
- Para Lucro Real e Presumido, importação de EFD ICMS é obrigatória.
- **Reativar** importação de XMLs de Entrada e Saída desativada no ERP_BRIDGE — criar abas separadas (Entradas, Saídas, CT-Es).
- Extrair e persistir campos de cabeçalho e itens conforme definido em CONTEXT.md (CRT, IBS/CBS if present, CCLASSTRIB if present).
- Tag `<CRT>` = 1 → alimentar tabela interna `forn_simples` (nome real no banco) / `fornecedores_simples_nacional` (nome conceitual do usuário).
- Painel **próprio para XMLs** separado do painel EFD, com abas Entradas/Saídas/CT-Es.
- Usar **VIEWs PostgreSQL** para performance nos painéis. Gerar VIEW ao importar.
- Conflito Oracle vs XML: **XML sobrescreve** campos tributários e marca `source = 'xml_upload'`.
- Relatórios: saneamento CCLASSTRIB, exportação CSV, fornecedores com classificações erradas.

### Claude's Discretion

- Estrutura exata das migrations (nomes de colunas, índices).
- Formato do drag-and-drop (biblioteca React, UX de progresso).
- Background job para XMLs grandes (Redis queue vs goroutine pool).
- Estrutura das VIEWs PostgreSQL (materializada vs regular).
- Formato do CSV de exportação.
- Ordem de implementação dos planos (waves).

### Deferred Ideas (OUT OF SCOPE)

- Watch automático de pasta.
- Multi-cliente comercial.
- Migração de stack.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| XML-01 | Tela de upload drag-and-drop (XML único ou ZIP c/ múltiplos XMLs) | Seções 7, Frontend |
| XML-02 | Validação schema NFe v4.00 (NFe + protNFe) antes da persistência | Seções 2, 3 |
| XML-03 | Parser popula nfe_entradas/nfe_saidas com PIS, COFINS, IPI, ICMS, CFOP | Seções 1, 2, 3 — tabelas já existem com colunas corretas |
| XML-04 | Conflito chave de acesso: XML sobrescreve Oracle Bridge, source='xml_upload' | Seção 8 — pattern ON CONFLICT DO UPDATE já usado no batch |
| XML-05 | Histórico de uploads no painel (quem, quando, qtd, reprocessar) | Seção 7 — nova tabela xml_upload_batches |
| XML-06 | Coluna source nas tabelas (oracle_bridge / xml_upload / manual) | Seção 8 — migration segura com ALTER TABLE ADD COLUMN |
| XML-07 | Limite por upload (100MB / 5000 XMLs) com mensagem clara | Seção 7 — validação no handler Go antes de processar |
| XML-08 | Background job para uploads >50 arquivos sem bloquear UI | Seção 7 — goroutine pool reutilizando padrão import_jobs existente |
</phase_requirements>

---

## Summary

Esta pesquisa descobre que o codebase do FB_APU04 já possui **infraestrutura XML significativa implementada**. O parser NFe/CTe em Go (`encoding/xml` nativo com structs dedicados em `backend/handlers/nfe_saidas.go`) já existe e funciona para uploads manuais. Os endpoints `/api/nfe-entradas/upload`, `/api/nfe-saidas/upload` e `/api/cte-entradas/upload` estão registrados e ativos. As tabelas `nfe_entradas`, `nfe_saidas` e `cte_entradas` existem com schema completo incluindo colunas IBS/CBS.

O que está **desativado no ERP_BRIDGE** não é o código Go — é a configuração do bridge Python: o `config-apu04.yaml` de produção usa `erp_type: sap_s4hana` (que não chama os endpoints de upload XML). O modo `oracle_xml` (que usa `/api/nfe-entradas/upload`, etc.) está implementado mas não configurado para produção APU04. A reativação requer: (1) criar abas separadas na UI do ERP Bridge por tipo (Entradas/Saídas/CT-Es) e (2) permitir configuração por servidor de quais `tipos` processar.

A Phase 2 adiciona principalmente: coluna `source`, tabela de histórico de uploads, lógica de conflito com update explícito, parser de itens NFe (tabela nova `nfe_entradas_itens`/`nfe_saidas_itens`), campo CRT→forn_simples, drag-and-drop com ZIP, background job para batches grandes, e relatórios CCLASSTRIB.

**Primary recommendation:** Reutilizar o parser XML Go existente como base, estender os structs para incluir `emit/CRT`, itens (`det`), e adicionar a lógica de conflito Oracle→XML via `ON CONFLICT DO UPDATE` nos handlers existentes.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Parser NFe/CTe XML | API (Go) | — | XML parsing é side-effect-free, pertence ao backend que valida e persiste |
| Resolução de conflito Oracle vs XML | API (Go) | — | O upsert com lógica de prioridade é uma operação de DB no backend |
| Background job (ZIP >50 XMLs) | API (Go) Worker | PostgreSQL (import_jobs) | Reutiliza o padrão existente de worker pool + tabela de jobs |
| Drag-and-drop UI + progress | Frontend (React) | — | UX client-side; ZIP expandido no frontend antes do envio ou no backend |
| Histórico de uploads | API (Go) + PostgreSQL | Frontend | Tabela nova, endpoint de leitura, render no frontend |
| VIEWs XML panel | PostgreSQL | — | Usuário locked em VIEWs (não materialized); refresh imediato |
| CRT → forn_simples | API (Go) | — | INSERT auxiliar durante o parseamento do XML |
| Relatório CCLASSTRIB | API (Go) + PostgreSQL | Frontend | Query sobre tabela de itens, exportação CSV no backend |
| ERP Bridge reativação oracle_xml | ERP Bridge (Python) | API (Go) endpoints | Configuração de tipos por servidor; endpoints já existem |

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `encoding/xml` (stdlib Go) | Go 1.26 | Parse NFe/CTe XML | Já em uso no projeto; sem dependência nova; suporte total a charsets via CharsetReader |
| `archive/zip` (stdlib Go) | Go 1.26 | Extrair XMLs de ZIP no backend | Stdlib — sem dependência nova; API simples para ler zip.Reader |
| `github.com/lib/pq` | v1.11.2 | PostgreSQL driver | Já em uso; todas as operações de DB |
| `react-dropzone` | 15.0.0 | Drag-and-drop UI no React | Biblioteca padrão do ecossistema React para upload; suporte nativo a multiple, accept, maxSize |
| `jszip` | 3.10.1 | Alternativa: expand ZIP no frontend | Apenas se preferir não enviar ZIP ao backend; ver Seção 7 |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `golang.org/x/text` | v0.14.0 | charset Windows-1252/ISO-8859-1 | Já em uso; XMLs legados com encoding não-UTF-8 |
| `sonner` | 2.0 (já presente) | Toast de progresso/conclusão | Já instalado; usar para notificar fim do background job |
| `@tanstack/react-query` | 5.90 (já presente) | Polling de status do job | Já instalado; usar para polling do status de upload batch |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `archive/zip` stdlib | go-unzip library | stdlib é suficiente; sem dependência nova |
| `react-dropzone` | HTML5 `<input>` nativo (já usado em ImportarXMLsEntrada.tsx) | dropzone melhora UX com drag-and-drop visual; input nativo é adequado para fase atual |
| Goroutine pool existente | Redis queue | Redis está provisionado mas o Go não tem cliente Redis — adotar goroutine pool reutilizando o padrão `import_jobs` evita nova dependência |

**Installation (apenas react-dropzone é nova dependência):**
```bash
# Frontend
cd frontend && npm install react-dropzone

# Backend: sem novas dependências — encoding/xml e archive/zip são stdlib
```

**Version verification:** [VERIFIED: npm registry] `react-dropzone@15.0.0` (maio 2026), `jszip@3.10.1`.

---

## Architecture Patterns

### System Architecture Diagram

```
Browser (drag-and-drop)
         │
         │  POST /api/xml/upload  (multipart: file + metadata)
         │  GET  /api/xml/upload-batches (histórico)
         │  GET  /api/xml/upload-batches/:id/status (polling)
         ▼
┌─────────────────────────────────────────────────────────┐
│  Go API Handler: XMLUploadHandler                        │
│  ┌─────────────────────────────────────────────────────┐│
│  │ 1. Validar tamanho (≤100MB / ≤5000 XMLs)           ││
│  │ 2. Se ZIP: extrair em memória (archive/zip)         ││
│  │ 3. Se ≤50 XMLs: processar inline, retornar JSON     ││
│  │ 4. Se >50 XMLs: criar xml_upload_batch (pending)   ││
│  │                 retornar batch_id + 202 Accepted    ││
│  └─────────────────────────────────────────────────────┘│
└──────────────────────────┬──────────────────────────────┘
                           │
           ┌───────────────┴───────────────┐
           │                               │
    ≤50 XMLs (síncrono)           >50 XMLs (assíncrono)
           │                               │
           ▼                               ▼
┌──────────────────────┐    ┌─────────────────────────────┐
│  parseXMLBatch()     │    │  xml_upload_batches table    │
│  Para cada XML:      │    │  status: pending→processing  │
│  - parseNFeXML()     │    │  → XML Worker Pool (3 gorout)│
│  - conflito Oracle?  │    │  → mesma lógica síncrona     │
│  - INSERT/UPDATE     │    └─────────────────────────────┘
│  - CRT → forn_simples│
│  - itens → _itens    │
└──────────────────────┘
           │
           ▼
┌────────────────────────────────────────┐
│  PostgreSQL                             │
│  nfe_entradas / nfe_saidas / cte_entradas  (com coluna source)
│  nfe_entradas_itens / nfe_saidas_itens  (nova)
│  xml_upload_batches                     (nova — histórico)
│  forn_simples                           (existente — CRT feed)
│  vw_xml_entradas / vw_xml_saidas / vw_xml_ctes  (novas VIEWs)
└────────────────────────────────────────┘
```

### Recommended Project Structure (additions only)

```
backend/
├── handlers/
│   ├── xml_upload.go           # novo: XMLUploadHandler, XMLUploadBatchesHandler
│   └── nfe_saidas.go           # modificar: adicionar CRT, itens, source
│   └── nfe_entradas.go         # modificar: adicionar CRT, itens, source
│   └── cte_entradas.go         # modificar: source
│   └── erp_bridge_batch.go     # modificar: marcar source='oracle_bridge'
├── migrations/
│   ├── 074_add_source_to_nfe_tables.sql
│   ├── 075_create_nfe_itens_tables.sql
│   ├── 076_create_xml_upload_batches.sql
│   └── 077_create_vw_xml_panels.sql
frontend/
├── src/pages/
│   ├── ImportarXMLsEntrada.tsx  # substituir: drag-and-drop + progresso + histórico
│   ├── ImportarXMLsSaida.tsx    # substituir: mesma UX
│   ├── ImportarXMLsCTe.tsx      # substituir: mesma UX
│   └── PainelXMLs.tsx           # novo: painel com abas Entradas/Saídas/CT-Es + CCLASSTRIB
```

---

## Research Findings by Question

---

## Q1: ERP_BRIDGE XML Reactivation — O Que Está Desativado e Por Quê

[VERIFIED: inspeção direta de `erp-bridge-aws/bridge.py` e `installer/aws-bridge/config.yaml.example`]

**O que NÃO está desativado (ao contrário do nome):**
- Os endpoints Go `/api/nfe-entradas/upload`, `/api/nfe-saidas/upload`, `/api/cte-entradas/upload` estão REGISTRADOS e FUNCIONAIS em `backend/main.go:559-564`.
- O handler `NfeEntradasUploadHandler`, `NfeSaidasUploadHandler`, `CteEntradasUploadHandler` estão implementados e funcionais.
- As páginas React `ImportarXMLsEntrada.tsx`, `ImportarXMLsSaida.tsx`, `ImportarXMLsCTe.tsx` existem e fazem upload manual de arquivos selecionados por pasta.

**O que está de fato desativado no ERP_BRIDGE:**
- O `config-apu04.yaml` de produção usa `erp_type: sap_s4hana`. Isso faz o daemon executar `processar_sap()` que usa `/api/erp-bridge/import/batch` (JSON) — nunca chama os endpoints de upload XML multipart.
- O modo `oracle_xml` (que chama os endpoints `/api/nfe-*/upload`) está implementado em `bridge.py:795-903` mas não está ativo na configuração APU04.
- O `FONTES` dict em `bridge.py:181-229` define as 3 fontes (nfe_saidas, nfe_entradas, cte_entradas) com SQL Oracle e endpoints corretos.
- A filtragem de `tipos` por servidor (`srv.get("tipos", list(FONTES.keys()))`) permite ativar/desativar tipos individualmente — mas na configuração atual de prod não há servidores oracle_xml configurados.

**O que requer trabalho para reativar (no ERP_BRIDGE):**
1. Adicionar à UI do ERP Bridge (`ERPBridgeConfig.tsx`) a seleção de `erp_type: oracle_xml` e configuração de servidores com seus tipos.
2. O backend já suporta `erp_type` em `erp_bridge_config.erp_type` (campo no banco, retornado em `/api/erp-bridge/credentials`).
3. A UI atual do ERP Bridge não expõe configuração de servidores individuais com `tipos` filter — isto é uma lacuna de UX.

**Conclusão para o planner:** A "reativação" é principalmente uma questão de UI de configuração do bridge + documentação. O código Python e Go já está pronto. O que falta é a UI que permite configurar um servidor como `oracle_xml` com seleção de tipos (Entradas, Saídas, CT-Es). As novas abas separadas pedidas pelo usuário são uma melhoria da UI de configuração/disparo do bridge.

---

## Q2: NFe v4.00 XML Parsing em Go

[VERIFIED: inspeção de `backend/handlers/nfe_saidas.go`]

**O parser já existe e funciona.** Localização: `backend/handlers/nfe_saidas.go` (structs e helpers compartilhados), `backend/handlers/nfe_entradas.go` (handler de upload de entradas).

**Approach usado:** `encoding/xml` stdlib com structs Go — sem biblioteca de terceiros. [VERIFIED: `backend/go.mod` — sem dependências XML externas]

**Como funciona o namespace:**
```go
// Remove namespace para simplificar o parsing (bridge.py:174-178)
data = bytes.ReplaceAll(data,
    []byte(` xmlns="http://www.portalfiscal.inf.br/nfe"`), []byte(""))
data = bytes.ReplaceAll(data,
    []byte(` xmlns='http://www.portalfiscal.inf.br/nfe'`), []byte(""))
```
Esta abordagem (stripping do namespace antes do decode) é o padrão para XMLs SEFAZ em Go e evita os problemas de mapeamento de namespace do `encoding/xml`. [VERIFIED: código funcionando em produção]

**Structs XML existentes (em `nfe_saidas.go`):**
- `nfeProc` → `NFe` → `infNFe` (com ID attr para fallback de chave)
- `protNFe` → `infProt` → `chNFe` (chave preferencial)
- `ide` (mod, serie, nNF, dhEmi, tpNF, natOp)
- `emit` (CNPJ, xNome, enderEmit)
- `dest` (CNPJ, CPF, xNome, enderDest)
- `total` → `ICMSTot` (todos os campos vBC..vNF) + `IBSCBSTot` (reforma tributária)

**O que FALTA nos structs existentes para a Phase 2:**
- `emit.CRT` — tag para Simples Nacional [ASSUMED: posição correta é `<emit><CRT>1</CRT></emit>` conforme spec NFe v4.00]
- `det[]` — array de itens da nota (NCM, CFOP, CST, vPIS, vCOFINS, vICMS, vIPI, etc.)
- Struct para CT-e em `cte_entradas.go` (separada da NF-e)

**Charset handling:** Já existe `nfeCharsetReader` que converte Windows-1252 e ISO-8859-1 para UTF-8. [VERIFIED: `nfe_saidas.go:159-169`]

**Extração da chave de acesso:** Função `extractChave` preferencia `protNFe/infProt/chNFe` e faz fallback ao atributo `Id` de `infNFe`. [VERIFIED: `nfe_saidas.go:190-203`]

**Gotchas com XMLs SEFAZ:**
- Alguns emitentes geram XML sem `<nfeProc>` wrapper (apenas `<NFe>`). O código atual exige `nfeProc` — validação XML-02 deve aceitar ambos.
- `dhEmi` pode ser `"2026-02-26T12:00:00-03:00"`, `"2026-02-26T12:00:00Z"`, ou `"2026-02-26"` — o parser já lida com os 3 formatos. [VERIFIED: `nfe_saidas.go:206-226`]
- XMLs de cancelamento (evento 110111) têm estrutura diferente de `nfeProc` — Phase 2 não precisa tratar, só importar XMLs de autorização.

---

## Q3: Tag CRT e Simples Nacional

[VERIFIED: inspeção de `backend/migrations/040_create_forn_simples_table.sql`, `backend/handlers/forn_simples.go`]

**Localização da tag CRT no XML NFe v4.00:** [ASSUMED: baseado em conhecimento de treinamento sobre spec NFe v4.00]
```xml
<emit>
  <CNPJ>12345678000195</CNPJ>
  <xNome>Fornecedor XYZ</xNome>
  <CRT>1</CRT>  <!-- 1=Simples Nacional, 2=SN Excesso Receita Bruta, 3=Regime Normal -->
</emit>
```

**Struct Go a adicionar:**
```go
type emit struct {
    CNPJ      string    `xml:"CNPJ"`
    XNome     string    `xml:"xNome"`
    CRT       string    `xml:"CRT"`      // ADICIONAR: "1"=Simples Nacional
    EnderEmit enderEmit `xml:"enderEmit"`
}
```

**Tabela de destino:** `forn_simples` (não `fornecedores_simples_nacional`). [VERIFIED: `040_create_forn_simples_table.sql`, `handlers/forn_simples.go`]

Estrutura atual:
```sql
CREATE TABLE IF NOT EXISTS forn_simples (
    cnpj VARCHAR(14) PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
```

**Lógica de upsert após parse:**
```go
if strings.TrimSpace(inf.Emit.CRT) == "1" {
    db.Exec(`INSERT INTO forn_simples (cnpj) VALUES ($1) ON CONFLICT (cnpj) DO NOTHING`,
        strings.TrimSpace(inf.Emit.CNPJ))
}
```

**Impacto:** A tabela `forn_simples` já é usada por `mv_operacoes_simples` e `mv_compras_fornecedores` para calcular operações com fornecedores do Simples Nacional. [VERIFIED: migrations 041, 043, 055, 061]

**Importante:** `forn_simples.cnpj` é a tabela GLOBAL (sem company_id). O design original é um cadastro global de CNPJs identificados como Simples Nacional — ao detectar CRT=1 em qualquer XML, o CNPJ entra nessa lista compartilhada entre todas as empresas. O planner deve decidir se mantém esse comportamento ou adiciona escopo por company_id.

---

## Q4: IBS/CBS — Estado Atual

[VERIFIED: inspeção de `backend/migrations/067_add_ibs_cbs_to_nfe_tables.sql`, `backend/handlers/nfe_saidas.go`]

**Status:** IBS/CBS já está implementado nas tabelas e no parser. [VERIFIED: colunas `v_bc_ibs_cbs`, `v_ibs_uf`, `v_ibs_mun`, `v_ibs`, `v_cred_pres_ibs`, `v_cbs`, `v_cred_pres_cbs` em `nfe_entradas` e `nfe_saidas`]

**Structs XML existentes:**
```go
type ibsCbsTot struct {
    VBCIBSCBS string `xml:"vBCIBSCBS"`
    GIBS      gIBS   `xml:"gIBS"`
    GCBS      gCBS   `xml:"gCBS"`
}
```

**Comportamento quando ausente:**
- `nfe_entradas`: colunas IBS/CBS são `NOT NULL DEFAULT 0` — fornecedores sem as tags ficam com zero. [VERIFIED: migration 059]
- `nfe_saidas`: colunas IBS/CBS são `NULLABLE` — ficam NULL quando ausentes. [VERIFIED: migration 058]
- O helper `toDecimal(s string) float64` retorna `0` para string vazia — tratamento correto para ausência da tag.

**Sobre a spec atual:** [ASSUMED: IBS/CBS são parte da Reforma Tributária (LC 214/2024). A tag `IBSCBSTot` existe na spec estendida NFe v4.01 que alguns emitentes anteciparam. Na prática, a maioria dos XMLs de 2024/2025 NÃO têm IBS/CBS — o campo ficará zerado, o que é o comportamento correto.]

**Conclusão:** Sem ação adicional necessária para IBS/CBS no parser de cabeçalho. Para itens (Q3 nos requisitos locked — campos por linha), será necessário adicionar campos `v_ibs`, `v_cbs`, `v_bc_ibs_cbs` nas tabelas de itens.

---

## Q5: VIEWs PostgreSQL vs Materialized Views para Painel XML

[VERIFIED: decisão do usuário locked em CONTEXT.md — usar VIEWs regulares, não materializadas]

**Decisão locked:** VIEWs PostgreSQL (não materializadas). Razão declarada pelo usuário: "processo similar ao do EFD ICMS já implementado" — mas EFD usa materialized views. O usuário provavelmente quer a mesma abordagem de "gerar view ao importar".

**Análise técnica:**

| Critério | VIEW Regular | Materialized VIEW |
|----------|-------------|-------------------|
| Atualização | Automática (sempre atual) | Manual (`REFRESH`) |
| Performance em alta carga | Lenta em tabelas grandes | Rápida (pré-computada) |
| Complexity | Simples | Requer UNIQUE index + REFRESH lógica |
| Uso atual do projeto | `vw_parceiros`, `vw_nfe_entradas_impostos` | `mv_mercadorias_agregada`, etc. |
| Adequado para painel XML | **Sim** — volumes menores que EFD SPED | Overkill para fase inicial |

**Recomendação para o planner (discretion):** Usar VIEWs regulares para o painel XML. O volume de XMLs (máx. 5000 por upload) é muito menor que registros SPED (milhões de linhas). VIEWs regulares são adequadas e eliminam a necessidade de REFRESH scheduling.

**Pattern das VIEWs existentes:**
- `vw_parceiros` (migration 068): VIEW simples sobre `erp_bridge_run_items` — sem agrupamento.
- `vw_nfe_entradas_impostos` (migration 072): VIEW com `GROUP BY` + `SUM(COALESCE(...))` — **este é o pattern a seguir** para o painel XML. [VERIFIED: migration 072]

**VIEWs a criar para Phase 2:**
```sql
-- vw_xml_entradas_resumo: agrega nfe_entradas por company/filial/mes
-- vw_xml_saidas_resumo:   agrega nfe_saidas por company/filial/mes
-- vw_xml_ctes_resumo:     agrega cte_entradas por company/filial/mes
-- vw_xml_itens_ncm:       agrega nfe_entradas_itens por NCM para CCLASSTRIB
```

---

## Q6: CCLASSTRIB — O Que É e Como Implementar

[ASSUMED: baseado em contexto fiscal brasileiro e análise do codebase]

**O que é CCLASSTRIB:** Código de Classificação Tributária — um campo interno do sistema (não tag XML padrão NFe) que identifica como cada item/NCM deve ser tributado (PIS/COFINS: monofásico, alíquota zero, regime geral, ST, etc.). Usado pelo sistema para calcular créditos de PIS/COFINS automaticamente. Não é uma tag SEFAZ padrão — é uma extensão do sistema.

**Onde pode aparecer no XML:** [ASSUMED] Pode ser incluído como informação complementar pelo emitente no campo `<infAdProd>` ou `<cEnq>` (código de enquadramento legal), mas mais provavelmente é um campo calculado/preenchido pelo sistema importador com base no NCM e CFOP.

**Relação com NCM:** O relatório de saneamento CCLASSTRIB compara items com o mesmo NCM e identifica divergências de classificação — i.e., fornecedores usando CST/tributação diferente para o mesmo produto (mesmo NCM). Isso é útil para identificar erros cadastrais nos fornecedores.

**Schema necessário para implementar CCLASSTRIB:**
A tabela de itens (nova — ver Q9) precisa de campos: `ncm`, `cfop`, `cst_pis`, `cst_cofins`, `cclasstrib` (nullable).

**Relatório de saneamento — lógica:**
```sql
-- Identifica NCMs com CST divergente entre fornecedores
SELECT ncm, COUNT(DISTINCT cst_pis) as variantes_cst
FROM nfe_entradas_itens
WHERE company_id = $1
GROUP BY ncm
HAVING COUNT(DISTINCT cst_pis) > 1
ORDER BY variantes_cst DESC;
```

---

## Q7: Drag-and-Drop + ZIP — Pattern Recomendado

[VERIFIED: análise de `frontend/src/pages/ImportarXMLsEntrada.tsx`; VERIFIED npm registry: react-dropzone@15.0.0]

### Situação Atual

A UI atual usa `<input type="file" webkitdirectory>` para selecionar pasta inteira. Não há drag-and-drop verdadeiro, nem suporte a ZIP, nem progresso para batch grande. [VERIFIED: `ImportarXMLsEntrada.tsx`]

### Abordagem Recomendada para Phase 2

**Frontend (react-dropzone@15.0.0):**
```tsx
import { useDropzone } from 'react-dropzone';

const { getRootProps, getInputProps, isDragActive } = useDropzone({
  accept: { 'text/xml': ['.xml'], 'application/zip': ['.zip'] },
  maxSize: 100 * 1024 * 1024, // 100MB
  onDrop: (files) => handleUpload(files),
});
```

**Estratégia para ZIP:**
- Opção A (recomendada): Enviar ZIP diretamente ao backend Go via `multipart/form-data`. O backend usa `archive/zip` para extrair em memória e processar. Mais simples, sem dependência de jszip no frontend.
- Opção B: Expandir ZIP no frontend com jszip, enviar XMLs individuais. Mais complexo, aumenta payload na rede.

**Recomendação do planner (discretion):** Opção A — backend descompacta o ZIP. Mais clean, menor superfície de ataque.

**Pattern para progress polling:**
Para batches >50 XMLs:
1. Frontend POST `/api/xml/upload` → recebe `{ batch_id: "uuid", status: "processing" }`
2. Frontend usa `useQuery` com `refetchInterval: 2000` para polling de `/api/xml/upload-batches/:id/status`
3. Backend atualiza `xml_upload_batches.processed_count` + `total_count` a cada X XMLs
4. Frontend exibe progress bar: `processed / total * 100`

**Limite de 100MB vs 5000 XMLs:**
```go
const MaxUploadBytes = 100 * 1024 * 1024 // 100MB
const MaxXMLsPerBatch = 5000

// Validação no handler ANTES de processar:
if r.ContentLength > MaxUploadBytes {
    http.Error(w, "Arquivo maior que 100MB", http.StatusRequestEntityTooLarge)
    return
}
// Após expandir ZIP:
if len(xmlFiles) > MaxXMLsPerBatch {
    http.Error(w, fmt.Sprintf("ZIP contém %d XMLs, máximo é %d", len(xmlFiles), MaxXMLsPerBatch), ...)
    return
}
```

**Background Job Pattern (>50 XMLs):**
Reutilizar exatamente o padrão da tabela `import_jobs` existente. [VERIFIED: `backend/worker/worker.go`]

```sql
-- Nova tabela xml_upload_batches (análoga a import_jobs)
CREATE TABLE xml_upload_batches (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id     UUID NOT NULL REFERENCES companies(id),
    uploaded_by    UUID,           -- user_id
    total_count    INT NOT NULL DEFAULT 0,
    processed_count INT NOT NULL DEFAULT 0,
    imported_count INT NOT NULL DEFAULT 0,
    rejected_count INT NOT NULL DEFAULT 0,
    status         TEXT NOT NULL DEFAULT 'pending', -- pending/processing/done/failed
    error_details  JSONB,
    created_at     TIMESTAMPTZ DEFAULT NOW(),
    completed_at   TIMESTAMPTZ
);
```

O worker existente (`backend/worker/worker.go`) usa `FOR UPDATE SKIP LOCKED` e pool de 3 goroutines. O XML worker pode ser um segundo pool dedicado ou expandir o worker.go existente para suportar um segundo job type. [VERIFIED: `worker.go` usa `import_jobs.status` como discriminador]

**Decisão recomendada (discretion):** Adicionar type de job `xml_batch` à tabela `import_jobs` existente OU criar tabela separada `xml_upload_batches` com seu próprio worker. Tabela separada é mais limpo — evita misturar SPED jobs com XML jobs.

---

## Q8: Migração da Coluna `source` — Estratégia Segura

[VERIFIED: inspeção das tabelas nfe_entradas, nfe_saidas, cte_entradas]

**A coluna `source` NÃO existe** nas tabelas atuais. [VERIFIED: migrations 058, 059, 060, 066, 067, 070 — nenhuma menciona coluna `source`]

**Migration segura com ALTER TABLE ADD COLUMN:**
```sql
-- 074_add_source_to_nfe_tables.sql
-- Adiciona coluna source com DEFAULT para compatibilidade com dados históricos.
-- Dados existentes do ERP Bridge ficam como 'oracle_bridge'.
-- Dados futuros do upload XML ficam como 'xml_upload'.

ALTER TABLE nfe_saidas
  ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'oracle_bridge';

ALTER TABLE nfe_entradas
  ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'oracle_bridge';

ALTER TABLE cte_entradas
  ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'oracle_bridge';

-- Índices para filtrar por fonte (útil para relatório de cobertura - Phase 4)
CREATE INDEX IF NOT EXISTS idx_nfe_saidas_source   ON nfe_saidas(company_id, source);
CREATE INDEX IF NOT EXISTS idx_nfe_entradas_source ON nfe_entradas(company_id, source);
CREATE INDEX IF NOT EXISTS idx_cte_entradas_source ON cte_entradas(company_id, source);
```

**Risco:** Praticamente zero. `ADD COLUMN IF NOT EXISTS` com `DEFAULT` não bloqueia leituras em PostgreSQL 15. O PostgreSQL usa um valor default "virtual" para rows existentes sem reescrever dados. [ASSUMED: comportamento padrão PostgreSQL desde v11]

**Constraint check:** Os valores válidos são `oracle_bridge`, `xml_upload`, `manual`. Pode-se adicionar CHECK constraint:
```sql
ADD CONSTRAINT chk_nfe_entradas_source CHECK (source IN ('oracle_bridge', 'xml_upload', 'manual'))
```

**Onde atualizar o source:**
1. `NfeEntradasUploadHandler`: adicionar `source = 'xml_upload'` no INSERT e no ON CONFLICT DO UPDATE.
2. `batchInsertNFeEntrada` / `batchInsertNFeSaida` em `erp_bridge_batch.go`: adicionar `source = 'oracle_bridge'` nos INSERTs e preservar no DO UPDATE (não sobrescrever se já for xml_upload).
3. Regra de conflito Oracle vs XML: ver Q4 conflito abaixo.

---

## Q9: Schema Atual das Tabelas — Resumo Completo

[VERIFIED: inspeção direta de migrations 058, 059, 060, 066, 067, 070]

### `nfe_entradas` (migration 058 + 066 + 067 + 070)

| Coluna | Tipo | Fonte |
|--------|------|-------|
| id, company_id, chave_nfe, modelo, serie, numero_nfe | PK/FK/identif. | ambas |
| data_emissao, data_autorizacao, mes_ano, nat_op | temporal | ambas |
| forn_cnpj, forn_nome, forn_uf, forn_municipio | emitente | XML |
| dest_cnpj_cpf, dest_nome, dest_uf, dest_c_mun | destinatário | XML |
| cancelado, cfop, tipo_cfop | status | bridge |
| v_bc..v_nf (ICMSTot XML) | impostos cabeçalho XML | XML |
| base_icms, icms, icms_st, ipi, base_pis, pis, base_cofins, cofins | impostos bridge | bridge |
| base_partilha, icms_partilha | partilha ICMS | bridge |
| v_bc_ibs_cbs, v_ibs_uf, v_ibs_mun, v_ibs, v_cred_pres_ibs, v_cbs, v_cred_pres_cbs | reforma tributária | ambas |
| created_at | auditoria | — |

**Colunas que FALTAM para Phase 2:**
- `source TEXT NOT NULL DEFAULT 'oracle_bridge'`
- NOTA: campos de itens (NCM, CFOP, CST por linha) ficam numa tabela separada `nfe_entradas_itens`

### `nfe_saidas` (migration 058 + 066 + 067 + 070)

Mesma estrutura que `nfe_entradas` com nomenclatura diferente:
- `emit_cnpj/nome/uf/municipio` (não `forn_*`)
- `dest_c_mun` existe

**Colunas que FALTAM:**
- `source TEXT NOT NULL DEFAULT 'oracle_bridge'`

### `cte_entradas` (migration 060 + 066)

Estrutura mais simples: identificação CT-e, emitente (transportadora), remetente, destinatário, `v_prest`, `v_rec`, `v_carga`, `v_bc_icms`, `v_icms`, IBS/CBS nullable, `cancelado`, `data_autorizacao`.

**Colunas que FALTAM:**
- `source TEXT NOT NULL DEFAULT 'oracle_bridge'`

### Tabelas NOVAS a criar em Phase 2

**`nfe_entradas_itens`** — itens por linha de NF-e de entrada:
```sql
CREATE TABLE nfe_entradas_itens (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nfe_id          UUID NOT NULL REFERENCES nfe_entradas(id) ON DELETE CASCADE,
    company_id      UUID NOT NULL,  -- desnormalizado para queries sem JOIN
    n_item          SMALLINT NOT NULL,    -- número do item (1..N)
    c_prod          VARCHAR(60),          -- <cProd> código do produto
    x_prod          VARCHAR(120) NOT NULL,-- <xProd> descrição
    ncm             VARCHAR(8),           -- <NCM>
    cfop            VARCHAR(4),           -- <CFOP>
    cst_icms        VARCHAR(3),           -- <ICMS><CST> ou <CSOSN>
    cst_pis         VARCHAR(2),           -- <PIS><CST>
    cst_cofins      VARCHAR(2),           -- <COFINS><CST>
    v_prod          NUMERIC(15,2) DEFAULT 0,
    v_total_item    NUMERIC(15,2) DEFAULT 0,
    v_bc_icms       NUMERIC(15,2) DEFAULT 0,
    v_icms          NUMERIC(15,2) DEFAULT 0,
    v_ipi           NUMERIC(15,2) DEFAULT 0,
    v_bc_pis        NUMERIC(15,2) DEFAULT 0,
    v_pis           NUMERIC(15,2) DEFAULT 0,
    v_bc_cofins     NUMERIC(15,2) DEFAULT 0,
    v_cofins        NUMERIC(15,2) DEFAULT 0,
    v_ibs           NUMERIC(15,2) DEFAULT 0,
    v_cbs           NUMERIC(15,2) DEFAULT 0,
    cclasstrib      VARCHAR(20),          -- classificação tributária (nullable)
    UNIQUE (nfe_id, n_item)
);
```

**`nfe_saidas_itens`** — mesma estrutura, referência para `nfe_saidas`.

**`xml_upload_batches`** — histórico de uploads:
```sql
CREATE TABLE xml_upload_batches (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id      UUID NOT NULL REFERENCES companies(id),
    uploaded_by     UUID,
    tipo            TEXT NOT NULL,  -- 'entradas' | 'saidas' | 'ctes'
    filename        TEXT,           -- nome do arquivo original
    total_count     INT NOT NULL DEFAULT 0,
    processed_count INT NOT NULL DEFAULT 0,
    imported_count  INT NOT NULL DEFAULT 0,
    rejected_count  INT NOT NULL DEFAULT 0,
    status          TEXT NOT NULL DEFAULT 'pending',
    error_details   JSONB,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    completed_at    TIMESTAMPTZ
);
```

---

## Resolução de Conflito Oracle vs XML (XML-04)

[VERIFIED: inspeção de `erp_bridge_batch.go` — padrão ON CONFLICT DO UPDATE já existe]

**Situação atual:** `batchInsertNFeEntrada` já usa `ON CONFLICT ON CONSTRAINT uq_nfe_entradas_company_chave DO UPDATE` para atualizar campos quando registro existe. O mesmo conflito acontece no handler de upload XML — que usa `ON CONFLICT DO NOTHING`.

**O que precisa mudar:** O `NfeEntradasUploadHandler` (upload XML) deve usar `DO UPDATE` ao invés de `DO NOTHING` para sobrescrever campos tributários quando o registro já existe via Oracle Bridge. E deve marcar `source = 'xml_upload'`.

**Regra de prioridade:**
- XML sobrescreve Oracle Bridge: campos `v_bc`, `v_icms`, `v_pis`, `v_cofins`, `v_ipi`, `v_bc_ibs_cbs`, `v_ibs`, `v_cbs`, `source`
- Oracle Bridge **não** sobrescreve XML: `DO UPDATE SET source = CASE WHEN EXCLUDED.source = 'xml_upload' THEN 'xml_upload' ELSE nfe_entradas.source END`

**Pattern SQL para conflito com prioridade:**
```sql
INSERT INTO nfe_entradas (company_id, chave_nfe, ..., source)
VALUES ($1, $2, ..., 'xml_upload')
ON CONFLICT ON CONSTRAINT uq_nfe_entradas_company_chave
DO UPDATE SET
    -- Campos tributários: XML sempre sobrescreve
    v_bc    = EXCLUDED.v_bc,
    v_icms  = EXCLUDED.v_icms,
    v_pis   = EXCLUDED.v_pis,
    v_cofins = EXCLUDED.v_cofins,
    source  = 'xml_upload',
    -- Campos de identidade: preservar o que já existe se XML não tiver
    forn_nome = COALESCE(NULLIF(EXCLUDED.forn_nome,''), nfe_entradas.forn_nome),
    -- etc.
```

Para que o Oracle Bridge NÃO sobrescreva XMLs: em `batchInsertNFeEntrada`, adicionar condição:
```sql
DO UPDATE SET
    cancelado = EXCLUDED.cancelado,
    base_icms = CASE WHEN nfe_entradas.source = 'xml_upload' THEN nfe_entradas.base_icms ELSE EXCLUDED.base_icms END,
    -- etc.
```

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Drag-and-drop UI | implementação nativa HTML5 custom | `react-dropzone` | Lida com edge cases de browser compatibility, mobile, multiple files |
| ZIP extraction (Go) | loop manual de bytes | `archive/zip` (stdlib) | stdlib já testa todos edge cases de ZIP64, encoding de filenames |
| ZIP extraction (frontend) | loop de bytes JS | `jszip` ou backend Go | Ver Q7 — backend Go é recomendado |
| NFe XML validation | parser custom com regras | `encoding/xml` existente + validação de campos obrigatórios | O projeto já tem o parser; adicionar verificação de `chave_nfe len=44` e `mod IN ('55','65')` |
| Charset conversion | iconv-go | `golang.org/x/text` existente | Já vendorizado |
| Background job queue | Redis Streams / custom | padrão `import_jobs` existente (PostgreSQL FOR UPDATE SKIP LOCKED) | Redis Go client não está instalado; padrão existente funciona e é testado |
| Progress polling | WebSocket | `useQuery` com `refetchInterval` (@tanstack/react-query) | Já instalado; polling a cada 2s é suficiente para este volume |

**Key insight:** O risco principal desta fase não é falta de bibliotecas mas sim complexidade de parsing de itens NFe (struct `det[]` aninhada) e lógica de conflito Oracle/XML. Ambos são resolvidos com código Go direto, sem libs externas.

---

## Common Pitfalls

### Pitfall 1: XML sem wrapper `<nfeProc>`
**What goes wrong:** XMLs gerados por alguns emitentes têm apenas `<NFe>` sem `<nfeProc>` wrapper. O parser atual exige `nfeProc`.
**Why it happens:** A SEFAZ aceita ambos; alguns sistemas omitem o wrapper.
**How to avoid:** Testar se o XML tem `<nfeProc>` ou `<NFe>` como raiz. Se `<NFe>`, criar struct alternativo ou normalizar adicionando wrapper fictício.
**Warning signs:** Parse error "expected element type 'nfeProc' but have 'NFe'".

### Pitfall 2: Namespace em múltiplos formatos
**What goes wrong:** `xmlns="http://www.portalfiscal.inf.br/nfe"` pode vir em aspas duplas ou simples, com ou sem espaço antes.
**Why it happens:** Diferentes emissores/sistemas.
**How to avoid:** O código atual faz `bytes.ReplaceAll` para os 2 casos — adicionar também variante com prefixo de namespace (`nfe:NFe`).
**Warning signs:** Parse error em XML com `xmlns='...'` (aspas simples — já tratado) ou com `nfe:` prefix (não tratado ainda).

### Pitfall 3: `ON CONFLICT DO NOTHING` perde dados XML
**What goes wrong:** Handler atual de upload usa `DO NOTHING` — se nota já existe via Bridge, os campos XML mais ricos (nome completo do fornecedor, endereço) são perdidos.
**Why it happens:** Design original era cautioso.
**How to avoid:** Mudar para `DO UPDATE SET` explícito por campo, preservando a regra de prioridade XML > Oracle.
**Warning signs:** Usuário importa XML, valores fiscais permanecem os do Bridge.

### Pitfall 4: ZIP com estrutura de subpastas
**What goes wrong:** Usuário zipa uma pasta e o ZIP contém `pasta/2026/nota.xml`. A lógica que itera `zip.File` precisa ignorar diretórios e extrair apenas `.xml`.
**Why it happens:** Windows cria ZIPs com estrutura de pastas.
**How to avoid:** `for _, f := range r.File { if f.FileInfo().IsDir() || !strings.HasSuffix(strings.ToLower(f.Name), ".xml") { continue } }`.
**Warning signs:** 0 XMLs processados de um ZIP que o usuário sabe que contém XMLs.

### Pitfall 5: `forn_simples` sem company_id scope
**What goes wrong:** A tabela `forn_simples` não tem `company_id` — é global. Um CNPJ identificado como Simples Nacional em qualquer empresa fica visível para todas. Isso pode ser intencional ou uma limitação.
**Why it happens:** Design original sem multi-tenancy no cadastro de regime tributário.
**How to avoid:** Documentar comportamento para o usuário. Se necessário, adicionar `company_id` na tabela — mas isso quebra as materialized views existentes (mv_operacoes_simples).
**Warning signs:** CNPJ aparece como Simples Nacional em empresa que não importou XMLs desse fornecedor.

### Pitfall 6: Items NFe — struct `det[]` aninhada
**What goes wrong:** A tag `<det>` contém sub-structs aninhadas para impostos (`<ICMS><ICMS00>`, `<ICMS><ICMSSN102>`, etc.) com múltiplos regimes. Tentar mapear todos os sub-tipos é complexo demais.
**Why it happens:** Spec NFe v4.00 tem ~30 variantes de grupo ICMS.
**How to avoid:** Para a Phase 2, extrair apenas: `cProd`, `xProd`, `NCM`, `CFOP`, `vProd`, `vTotItem` do cabeçalho do item, e os totais de impostos (não os grupos aninhados). Os totais por item podem ser capturados via `PIS/vPIS`, `COFINS/vCOFINS`, `IPI/vIPI`, e ICMS calculado de CST.
**Warning signs:** Código com 500 linhas de structs XML para variantes ICMS.

---

## Code Examples

### Adicionar CRT ao struct emit existente
```go
// Source: backend/handlers/nfe_saidas.go (modificar struct existente)
type emit struct {
    CNPJ      string    `xml:"CNPJ"`
    XNome     string    `xml:"xNome"`
    CRT       string    `xml:"CRT"`       // NOVO: 1=Simples Nacional
    EnderEmit enderEmit `xml:"enderEmit"`
}
```

### Struct para itens NFe
```go
// Source: novo — adicionar a nfe_saidas.go (compartilhado)
type det struct {
    NItem  string  `xml:"nItem,attr"`
    Prod   prod    `xml:"prod"`
    Imposto detImposto `xml:"imposto"`
}

type prod struct {
    CProd  string `xml:"cProd"`
    XProd  string `xml:"xProd"`
    NCM    string `xml:"NCM"`
    CFOP   string `xml:"CFOP"`
    VProd  string `xml:"vProd"`
}

type detImposto struct {
    ICMS   detICMS   `xml:"ICMS"`
    PIS    detPIS    `xml:"PIS"`
    COFINS detCOFINS `xml:"COFINS"`
    IPI    detIPI    `xml:"IPI"`
}

// Simplificação: capturar CST do primeiro grupo presente
// (evitar mapear todas as ~30 variantes de grupo ICMS)
type detICMS struct {
    Grupos []detICMSGrupo `xml:",any"` // captura qualquer sub-elemento
}
```

### Extração ZIP em Go (stdlib)
```go
// Source: archive/zip stdlib — sem dependência nova
import "archive/zip"

func extractXMLsFromZip(data []byte) ([][]byte, error) {
    r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
    if err != nil {
        return nil, err
    }
    var xmls [][]byte
    for _, f := range r.File {
        if f.FileInfo().IsDir() {
            continue
        }
        if !strings.HasSuffix(strings.ToLower(f.Name), ".xml") {
            continue
        }
        rc, err := f.Open()
        if err != nil {
            continue
        }
        content, err := io.ReadAll(rc)
        rc.Close()
        if err != nil {
            continue
        }
        xmls = append(xmls, content)
    }
    return xmls, nil
}
```

### react-dropzone com accept XML/ZIP
```tsx
// Source: react-dropzone@15.0.0 docs
import { useDropzone } from 'react-dropzone';

const { getRootProps, getInputProps, isDragActive } = useDropzone({
  accept: {
    'text/xml': ['.xml'],
    'application/zip': ['.zip'],
    'application/x-zip-compressed': ['.zip'],
  },
  maxSize: 100 * 1024 * 1024, // 100MB
  multiple: true,
  onDrop: (accepted, rejected) => {
    if (rejected.length > 0) {
      toast.error(`${rejected.length} arquivo(s) rejeitados. Apenas XML e ZIP até 100MB.`);
    }
    handleUpload(accepted);
  },
});
```

### Polling de status com react-query
```tsx
// Source: @tanstack/react-query@5 docs — já instalado
const { data: batchStatus } = useQuery({
  queryKey: ['xml-batch', batchId],
  queryFn: () => fetch(`/api/xml/upload-batches/${batchId}/status`, { headers: authHeaders }).then(r => r.json()),
  enabled: !!batchId && batchStatus?.status !== 'done' && batchStatus?.status !== 'failed',
  refetchInterval: batchId ? 2000 : false,
});
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Upload XML via pasta selecionada | Drag-and-drop + ZIP | Phase 2 | UX muito melhor para importação em lote |
| `ON CONFLICT DO NOTHING` no upload XML | `ON CONFLICT DO UPDATE` com prioridade XML>Oracle | Phase 2 | Garante que XML sobrescreve Bridge |
| Sem coluna source | `source` em nfe_entradas/saidas/cte_entradas | Phase 2 | Rastreabilidade de origem dos dados |
| Sem itens por linha | tabelas `nfe_entradas_itens`/`nfe_saidas_itens` | Phase 2 | Habilita CCLASSTRIB e relatórios por NCM |
| Upload XML individual (handler existe) | Upload em lote + background job | Phase 2 | Suporte a ZIPs com 5000 XMLs |

**Deprecated/outdated:**
- `ImportarXMLsEntrada.tsx` atual (input folder): será substituído pelo drag-and-drop — pode ser refatorado in-place.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Tag `<CRT>` está em `<emit><CRT>` no XML NFe v4.00 | Q3 | Struct Go incorreto → CRT nunca detectado → forn_simples nunca alimentado |
| A2 | IBS/CBS são parte de spec estendida NFe v4.01, maioria dos XMLs não têm | Q4 | Sem impacto — código já trata ausência com DEFAULT 0 |
| A3 | CCLASSTRIB não é tag XML padrão SEFAZ — é campo interno do sistema | Q6 | Se CCLASSTRIB for tag XML real, precisa de struct adicional no parser |
| A4 | `ADD COLUMN IF NOT EXISTS` com DEFAULT não bloqueia reads em PG15 | Q8 | Migration pode causar lock — verificar em janela de manutenção |
| A5 | Volume de XMLs (até 5000) é pequeno o suficiente para VIEWs regulares | Q5 | Se crescer >100k registros, considerar materializar |
| A6 | `forn_simples` sem company_id é comportamento intencional | Q3 | Dados Simples Nacional de um cliente visíveis para outros |

---

## Open Questions

1. **CRT e forn_simples — escopo company_id**
   - What we know: `forn_simples` tem apenas `cnpj` como PK, sem company_id. As MVs existentes (`mv_operacoes_simples`) dependem de JOIN sem company_id.
   - What's unclear: Usuário quer isso global (qualquer empresa que identificar o fornecedor como SN beneficia todos) ou por empresa?
   - Recommendation: Manter global por ora — mudança requereria refactor das MVs existentes.

2. **ERP Bridge — abas separadas Entradas/Saídas/CT-Es**
   - What we know: O contexto diz "criar abas separadas por tipo de importação no ERP Bridge".
   - What's unclear: Isso é na UI de disparo de run (trigger Entradas/Saídas/CT-Es separadamente) ou na configuração de servidores (cada servidor tem seus tipos)?
   - Recommendation: Implementar como filtro de tipo na UI de trigger + campo `tipos` na configuração de servidor. O daemon já suporta `srv.tipos` em `bridge.py:803`.

3. **Items NFe — profundidade do parser**
   - What we know: Spec NFe v4.00 tem ~30 variantes de grupo ICMS por item. Capturar tudo é muito código.
   - What's unclear: O usuário precisa de CST por item ou apenas NCM + valores totais?
   - Recommendation: Phase 2 captura: cProd, xProd, NCM, CFOP, vProd, vICMS(total), vPIS, vCOFINS, vIPI, e CST quando disponível sem distinguir variante.

4. **upload_batches vs import_jobs — mesmo worker ou separado?**
   - What we know: Worker existente processa `import_jobs` (SPED files). XML batch é similar mas tipo diferente.
   - What's unclear: Vale a pena estender o worker.go existente ou criar xml_worker.go separado?
   - Recommendation: Tabela separada `xml_upload_batches` + novo `xml_worker.go` que reutiliza as funções `parseNFeXML`, `extractXMLsFromZip`. Evita contaminar lógica SPED com lógica XML.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Backend XML worker | ✓ | go1.26.0 | — |
| Node.js 18 | Frontend build | ✓ | v18.19.1 | — |
| PostgreSQL | DB migrations | ✓ | 16.13 | — |
| Redis CLI | (info only) | ✓ | 7.0.15 | — |
| `archive/zip` | ZIP extraction | ✓ | stdlib | — |
| `encoding/xml` | XML parsing | ✓ | stdlib | — |
| `react-dropzone` | Drag-and-drop UI | ✗ (não instalado) | 15.0.0 | `<input webkitdirectory>` já funcional |

**Missing dependencies com fallback:**
- `react-dropzone@15.0.0`: não instalado. A UI atual já funciona para seleção de pasta. Instalar com `npm install react-dropzone` antes de refatorar as páginas de upload.

---

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V5 Input Validation | yes | Validar chave 44 dígitos, modelo IN ('55','65','57'), tamanho do arquivo ≤100MB |
| V4 Access Control | yes | JWT obrigatório + companyID validation (padrão existente withAuth) |
| V13 API/File Upload | yes | Limite de tamanho, validação de tipo MIME, não executar conteúdo do XML |
| V6 Cryptography | no | Não há criptografia nova nesta fase |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| XML bomb / billion laughs | Denial of Service | `xml.Decoder` stdlib Go NÃO expande entidades externas por default — safe |
| ZIP bomb | Denial of Service | Validar tamanho ANTES de extrair (`f.UncompressedSize64 > maxBytes`) |
| Path traversal em ZIP | Tampering | Ignorar filenames com `..` ou `/`; usar `filepath.Base(f.Name)` |
| CNPJ injection via XML | Tampering | Usar parâmetros $1,$2 em todas as queries (já no padrão do projeto) |
| Upload de XML malicioso | Tampering | O parser apenas lê valores de tags conhecidas; não executa conteúdo |

---

## Sources

### Primary (HIGH confidence)
- `backend/handlers/nfe_saidas.go` — parser XML existente, structs, helpers
- `backend/handlers/nfe_entradas.go` — handler de upload e INSERT
- `backend/handlers/erp_bridge_batch.go` — padrão ON CONFLICT com prioridade
- `backend/migrations/058-060, 066-067, 070, 072` — schema atual das tabelas
- `backend/migrations/040-044` — `forn_simples` e MVs de Simples Nacional
- `backend/worker/worker.go` — padrão de background job (import_jobs)
- `erp-bridge-aws/bridge.py` — modo oracle_xml, FONTES, processar_servidor
- `frontend/src/pages/ImportarXMLsEntrada.tsx` — UI atual de upload
- `backend/go.mod` — dependências Go existentes

### Secondary (MEDIUM confidence)
- npm registry: react-dropzone@15.0.0, jszip@3.10.1 — versões verificadas
- Observação indireta da spec NFe v4.00 via structs existentes no código

### Tertiary (LOW confidence)
- Posição da tag `<CRT>` no XML (A1): baseado em treinamento, não verificado via spec oficial
- Natureza de CCLASSTRIB (A3): inferido do contexto fiscal, não verificado via spec SEFAZ

---

## Metadata

**Confidence breakdown:**
- Schema das tabelas: HIGH — inspeção direta das migrations
- Parser XML existente: HIGH — inspeção direta do código
- ERP Bridge reactivation: HIGH — inspeção direta do bridge.py
- CRT tag structure: MEDIUM — inferido de código existente + treinamento
- IBS/CBS status: HIGH — colunas verificadas no schema
- react-dropzone API: HIGH — versão verificada no npm registry
- CCLASSTRIB definition: LOW — conceito inferido, não verificado via spec SEFAZ

**Research date:** 2026-05-16
**Valid until:** 2026-06-16 (stack estável, principais riscos são A1/A3 acima)
