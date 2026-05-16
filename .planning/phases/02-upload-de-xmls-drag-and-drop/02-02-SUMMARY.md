---
phase: 02-upload-de-xmls-drag-and-drop
plan: "02"
subsystem: backend-xml-handlers
tags: [go, xml, nfe, upload, worker, handlers, postgresql]
dependency_graph:
  requires:
    - schema/nfe_entradas.source
    - schema/nfe_saidas.source
    - schema/nfe_entradas_itens
    - schema/nfe_saidas_itens
    - schema/xml_upload_batches
    - views/vw_xml_entradas_resumo
    - views/vw_xml_saidas_resumo
    - views/vw_xml_ctes_resumo
  provides:
    - api/xml/upload (POST)
    - api/xml/upload-batches (GET)
    - api/xml/upload-batches/{id}/status (GET)
    - api/xml/painel/entradas (GET)
    - api/xml/painel/saidas (GET)
    - api/xml/painel/ctes (GET)
    - handler/XMLUploadHandler
    - handler/XMLUploadBatchesHandler
    - handler/XMLUploadBatchStatusHandler
    - handler/XMLPainelHandler
    - worker/StartXMLWorker
  affects:
    - backend/handlers/nfe_saidas.go (structs estendidos, DO UPDATE, insertNFeItens)
    - backend/handlers/nfe_entradas.go (DO UPDATE, itens, CRT)
    - backend/handlers/erp_bridge_batch.go (proteção CASE WHEN source=xml_upload)
    - backend/main.go (4 rotas + StartXMLWorker)
tech_stack:
  added:
    - archive/zip (extração de ZIP em memória)
  patterns:
    - "INSERT ... ON CONFLICT DO UPDATE com source='xml_upload' — prioridade XML>Oracle"
    - "CASE WHEN source='oracle_bridge' AND source='xml_upload' THEN preserve — proteção bidirecional"
    - "FOR UPDATE SKIP LOCKED — worker assíncrono sem race conditions"
    - "zip.NewReader(bytes.NewReader(data)) — extração de ZIP sem filesystem"
    - "xml:",any" em detICMS — captura ~30 variantes ICMS sem mapear cada uma"
key_files:
  created:
    - backend/handlers/xml_upload.go
    - backend/handlers/xml_painel.go
    - backend/worker/xml_worker.go
  modified:
    - backend/handlers/nfe_saidas.go
    - backend/handlers/nfe_entradas.go
    - backend/handlers/erp_bridge_batch.go
    - backend/main.go
decisions:
  - "ProcessXMLBatch exportado de handlers para uso pelo worker (evita duplicação de lógica)"
  - "NamedXML exportado como tipo público — worker/xml_worker.go importa handlers para reutilizar o tipo"
  - "insertNFeItens não aborta nota principal em falha de itens — degradação graciosa"
  - "xml_data BYTEA: XMLs >50 comprimidos em ZIP em memória antes de armazenar no banco"
  - "parseNFeXML estendido com wrapper fictício para XMLs sem nfeProc e stripping de prefixo nfe:"
metrics:
  duration: "~30 minutos"
  completed_date: "2026-05-16"
  tasks_completed: 3
  tasks_total: 3
  files_created: 3
  files_modified: 4
---

# Phase 02 Plan 02: Handlers Go para Upload e Parse de XMLs NF-e — Summary

Parser XML estendido com struct det[], CRT e lógica de conflito XML>Oracle; handler unificado /api/xml/upload com suporte a ZIP, batch assíncrono (>50 XMLs) e worker pool de 3 goroutines; painel com 3 endpoints sobre vw_xml_*_resumo.

## Rotas Registradas em main.go

| Rota | Handler | Método | Descrição |
|------|---------|--------|-----------|
| `/api/xml/painel/` | `XMLPainelHandler` | GET | Painel de entradas/saidas/ctes via views |
| `/api/xml/upload` | `XMLUploadHandler` | POST | Upload .xml ou .zip (inline ou async) |
| `/api/xml/upload-batches/` | `XMLUploadBatchStatusHandler` | GET | Status de batch por ID |
| `/api/xml/upload-batches` | `XMLUploadBatchesHandler` | GET | Histórico paginado de uploads |

**Ordem de registro:** `/api/xml/painel/` registrado **antes** de `/api/xml/upload-batches/` — prefixo mais específico no mux stdlib.

## Funções Exportadas por xml_upload.go

Para o frontend de 02-03 e o worker:

| Símbolo | Tipo | Descrição |
|---------|------|-----------|
| `XMLUploadHandler(db)` | `http.HandlerFunc` | POST /api/xml/upload |
| `XMLUploadBatchStatusHandler(db)` | `http.HandlerFunc` | GET /api/xml/upload-batches/{id}/status |
| `XMLUploadBatchesHandler(db)` | `http.HandlerFunc` | GET /api/xml/upload-batches |
| `ProcessXMLBatch(db, batchID, companyID, tipo, xmlFiles)` | `func` | Processa slice de NamedXML e atualiza batch |
| `NamedXML{Name, Data}` | `struct` | Arquivo XML com nome de origem |

## Lógica de Detecção Tipo entradas/saidas/ctes

**Parâmetro de formulário** `tipo` (obrigatório no multipart):
- `tipo=entradas` → INSERT em `nfe_entradas` + itens em `nfe_entradas_itens`
- `tipo=saidas` → INSERT em `nfe_saidas` (valida tpNF=1) + itens em `nfe_saidas_itens`
- `tipo=ctes` → reservado (parser CT-e separado em cte_entradas.go, não implementado neste plan)

Qualquer outro valor retorna 400 com mensagem em PT-BR.

## Lógica de Conflito Oracle/XML

**XML sobrescreve Oracle:**
- `NfeEntradasUploadHandler` e `NfeSaidasUploadHandler`: `ON CONFLICT DO UPDATE SET ... source='xml_upload'`
- O upsert atualiza todos os campos tributários quando o XML chega

**Oracle NÃO sobrescreve XML:**
- `batchInsertNFeEntrada` e `batchInsertNFeSaida`: campos tributários protegidos com:
  ```sql
  campo = CASE WHEN EXCLUDED.source = 'oracle_bridge' AND tabela.source = 'xml_upload'
               THEN tabela.campo ELSE EXCLUDED.campo END
  ```

## Structs XML Adicionados (nfe_saidas.go)

- `emit.CRT string` — detecta Simples Nacional (CRT=1 → INSERT em `forn_simples`)
- `infNFe.Det []det` — array de itens da nota
- `det`, `prod`, `detImposto`, `detICMS`, `detICMSGrupo`, `detPIS`, `detCOFINS`, `detIPI`
- `detICMS.Grupos []detICMSGrupo \`xml:",any"\`` — captura ~30 variantes ICMS sem mapear cada uma

## Constantes de Limite

Definidas em `nfe_saidas.go` (compartilhadas pelo mesmo pacote):
```go
MaxUploadBytes      = 100 * 1024 * 1024  // 100MB
MaxXMLsPerBatch     = 5000
BatchAsyncThreshold = 50
```

## Mitigações de Segurança (Threat Model)

| Threat ID | Mitigação Implementada |
|-----------|----------------------|
| T-02-02-01 | `f.UncompressedSize64` verificado antes de abrir cada entry do ZIP |
| T-02-02-03 | `filepath.Base(f.Name)` + skip de entries com `..` |
| T-02-02-04 | Todos os campos XML persistidos via parâmetros `$N` (sem concatenação) |
| T-02-02-05 | `GetEffectiveCompanyID` valida user↔company antes de qualquer persistência |
| T-02-02-06 | `error_details` retorna apenas `{filename, motivo}` — sem stack trace |
| T-02-02-07 | `r.ContentLength > MaxUploadBytes` verificado ANTES de `r.ParseMultipartForm` |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocker] XMLPainelHandler criado antes de registrar rotas no main.go**
- **Found during:** Task 2 (ao adicionar rotas em main.go, o compilador apontou undefined: handlers.XMLPainelHandler)
- **Issue:** O plano colocava Task 3 (painel) depois de Task 2 (rotas em main.go), mas main.go referencia XMLPainelHandler. A rota `/api/xml/painel/` foi adicionada em main.go no mesmo commit de Task 2, mas o arquivo xml_painel.go precisou ser criado antes do build final.
- **Fix:** xml_painel.go criado imediatamente; rota registrada no mesmo commit do Task 2; build validado antes de commitar Task 3.
- **Files modified:** backend/handlers/xml_painel.go, backend/main.go
- **Commit:** 8d2fc60

**2. [Rule 2 - Missing Export] NamedXML e ProcessXMLBatch exportados para uso pelo worker**
- **Found during:** Task 2 (criação de xml_worker.go)
- **Issue:** O worker precisa chamar `handlers.ProcessXMLBatch` e usar `handlers.NamedXML`, mas ambos eram privados no plano original.
- **Fix:** `namedXML` renomeado para `NamedXML` (exportado); `ProcessXMLBatch` adicionado como wrapper público. Alias `namedXML = NamedXML` mantém compatibilidade interna.
- **Files modified:** backend/handlers/xml_upload.go
- **Commit:** ced16b7

## Self-Check: PASSED

Arquivos criados:
- backend/handlers/xml_upload.go — FOUND (commit ced16b7)
- backend/handlers/xml_painel.go — FOUND (commit 8d2fc60)
- backend/worker/xml_worker.go — FOUND (commit ced16b7)

Arquivos modificados:
- backend/handlers/nfe_saidas.go — FOUND (commit f21ef89)
- backend/handlers/nfe_entradas.go — FOUND (commit f21ef89)
- backend/handlers/erp_bridge_batch.go — FOUND (commit f21ef89)
- backend/main.go — FOUND (commits ced16b7, 8d2fc60)

Build: `cd backend && go build ./...` — PASSOU sem erros

Rotas no main.go:
- XMLPainelHandler — FOUND linha 563
- XMLUploadHandler — FOUND linha 564
- XMLUploadBatchStatusHandler — FOUND linha 565
- XMLUploadBatchesHandler — FOUND linha 566
- StartXMLWorker — FOUND linha 226
