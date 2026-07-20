---
title: 'Importar EFD Contribuições — enriquecer PIS/COFINS de notas por chave de acesso'
type: 'feature'
created: '2026-07-14'
status: 'in-progress'
review_loop_iteration: 0
context: []
baseline_commit: '9f559b51c01b9b1c9d5622d21386b071cbba30ea'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Notas de entrada/saída importadas via XML (ERP_BRIDGE) frequentemente chegam com PIS/COFINS zerados (`nfe_entradas.v_pis`/`v_cofins`, `nfe_saidas.v_pis`/`v_cofins`) porque nem todo fornecedor preenche essas tags no XML, impedindo o cálculo correto do impacto de IBS/CBS.

**Approach:** Nova tela/fluxo dedicado "Importar EFD Contribuições" que faz upload do arquivo oficial (fonte autoritativa de PIS/COFINS), lê apenas o registro `C100` (documento, sem ambiguidade de item), casa cada nota pela chave de acesso (`chave_nfe`, escopada por `company_id`) e atualiza `v_pis`/`v_cofins` no cabeçalho da nota já existente em `nfe_entradas`/`nfe_saidas`. Reaproveita o pipeline de jobs (`import_jobs` + worker pool) já comprovado no import de EFD ICMS/IPI.

## Boundaries & Constraints

**Always:**
- Casar exclusivamente por `(company_id, chave_nfe)` — nunca só por `chave_nfe` (guard IDOR, padrão já usado em todo o projeto).
- Ler somente o registro `C100` do arquivo (documento/cabeçalho) — sem parsing de item (`C170`/`C175`/etc).
- `IND_OPER` do C100 decide entrada (`0` → `nfe_entradas`) vs saída (`1` → `nfe_saidas`), mesma convenção do parser EFD ICMS/IPI existente.
- Sobrescrever `v_pis`/`v_cofins` sempre que a chave casar (EFD Contribuições é a fonte autoritativa) — não só quando o valor atual for zero.
- Registrar cada C100 processado numa tabela de auditoria (casado ou não) — requisito de trilha imutável já presente no PRD.
- Reaproveitar `import_jobs`/worker pool/resume-por-batch existentes — não criar fila paralela.

**Ask First:**
- ~~Posição exata dos campos `CHV_NFE`, `VL_PIS`, `VL_COFINS` no registro C100~~ — **Resolvido em 2026-07-20**, validado contra arquivo real de produção (`EFD_CONTRIB_092021_V2.txt`, ~197 mil registros C100 de ARMAZEM CORAL LTDA): `parts[9]` é chave de acesso válida (44 dígitos) em 99,4% das linhas; `parts[26]`/`parts[27]` concentram-se em ~1,65%/7,60% de `VL_MERC`, batendo com as alíquotas oficiais do regime não-cumulativo de PIS/COFINS. `parts[2]=IND_OPER  parts[9]=CHV_NFE  parts[26]=VL_PIS  parts[27]=VL_COFINS`. Também bate com o leiaute do Guia Prático oficial.
- ~~Se uma nota não casar, isso deve bloquear o job ou só ser reportado no resumo?~~ — **Resolvido em 2026-07-20**: confirmado com o usuário — não bloqueia, só reporta (`matched=false` em `efd_contribuicoes_matches`, job segue `completed`). Comportamento já implementado em `processEFDContribuicoesFile`.

**Never:**
- Parsing dos Blocos A (serviços), D (transporte) ou M (apuração/consolidação) — fora de escopo desta entrega.
- Enriquecimento a nível de item (`nfe_entradas_itens`/`nfe_saidas_itens`) — o `NUM_ITEM` do C170 do EFD Contribuições pode não alinhar 1:1 com o `nItem` do XML; risco não resolvido, fica para entrega futura.
- Reaproveitar a tela `ImportarEFD.tsx` existente — fluxo e tela são separados por decisão do usuário.
- Alterar `reg_c100`/`reg_c170` (dados nativos da EFD ICMS/IPI).

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| HAPPY_PATH | Arquivo válido, C100 com chave existente em `nfe_entradas` da mesma company | `v_pis`/`v_cofins` atualizados; job `completed`; resumo "N casadas / M não encontradas" | N/A |
| NO_MATCH | C100 com `chave_nfe` que não existe em `nfe_entradas` nem `nfe_saidas` da company | Linha `matched=false` na tabela de auditoria; job segue e finaliza `completed` | Não interrompe o job |
| CROSS_COMPANY | Chave existe, mas em `company_id` diferente | Não casa (guard `company_id`); conta como NO_MATCH | N/A |
| CORRUPTED_FILE | Arquivo truncado, sem `|9999|` final | Job `error` na validação de integridade (mesma checagem já usada no EFD ICMS/IPI) | Mensagem de erro clara em `import_jobs.message` |

</frozen-after-approval>

## Code Map

- `backend/handlers/upload.go` -- referência do padrão de upload chunked a replicar (não editar)
- `backend/worker/worker.go` -- `processFile`/`processNextJob`: adicionar dispatch por tipo de job
- `backend/handlers/job.go` -- `ListJobsHandler`: aceitar filtro por tipo
- `backend/main.go` -- registro de rotas
- `frontend/src/pages/ImportarEFD.tsx` -- referência de UX a espelhar (não editar)
- `frontend/src/App.tsx`, `frontend/src/lib/navigation.ts`, `frontend/src/components/AppRail.tsx` -- padrão de 3 arquivos p/ registrar rota+menu (skill `fb-apu04-nav-architecture`)

## Tasks & Acceptance

**Execution:**
- [x] `backend/migrations/159_add_efd_contribuicoes_support.sql` -- `ALTER TABLE import_jobs ADD COLUMN tipo_arquivo VARCHAR(30) NOT NULL DEFAULT 'efd_icms_ipi'`; `CREATE TABLE efd_contribuicoes_matches (id, job_id FK, company_id, chave_nfe, tipo_nota, matched BOOLEAN, vl_pis, vl_cofins, created_at)` -- viabiliza discriminar o job e a trilha de auditoria
- [x] `backend/handlers/efd_contribuicoes_upload.go` (novo) -- `UploadEFDContribuicoesHandler`: mesma lógica de chunking/integridade de `upload.go`, cria job com `tipo_arquivo='efd_contribuicoes'` -- reaproveita infra comprovada
- [x] `backend/worker/worker.go` -- em `processFile`, ler `tipo_arquivo` do job; se `efd_contribuicoes`, despachar para nova função `processEFDContribuicoesFile` -- mantém um único worker pool
- [x] `backend/worker/worker.go` -- `processEFDContribuicoesFile(db, jobID, filename)`: parse ISO-8859-1 linha a linha, filtra `|C100|`, extrai `IND_OPER`/`CHV_NFE`/`VL_PIS`/`VL_COFINS`, `UPDATE nfe_entradas` ou `nfe_saidas` `WHERE company_id=$ AND chave_nfe=$`, grava linha em `efd_contribuicoes_matches` -- núcleo da feature
- [x] `backend/handlers/job.go` -- `ListJobsHandler` aceita `?tipo=efd_contribuicoes` -- nova tela lista só seus próprios jobs
- [x] `backend/main.go` -- `http.HandleFunc("/api/efd-contribuicoes/upload", withAuth(handlers.UploadEFDContribuicoesHandler, ""))` -- expõe o handler
- [x] `frontend/src/pages/ImportarEFDContribuicoes.tsx` (novo) -- espelha UX de `ImportarEFD.tsx` (upload chunked, polling, badges de status) apontando para o novo endpoint; exibe resumo casadas/não encontradas ao final
- [x] `frontend/src/App.tsx` + `lib/navigation.ts` + `components/AppRail.tsx` -- registra rota `/importar-efd-contribuicoes`, novo item de menu/rail -- seguir exatamente o padrão de 3 arquivos já mapeado

**Acceptance Criteria:**
- Given um C100 cuja chave existe em `nfe_entradas` da mesma company, when o job processa essa linha, then `v_pis`/`v_cofins` são sobrescritos com os valores do EFD Contribuições.
- Given um C100 cuja chave não existe em nenhuma tabela de notas da company, when processado, then grava-se `matched=false` em `efd_contribuicoes_matches` e o job **não** falha por isso.
- Given um usuário sem persona/módulo liberado, when acessa `/importar-efd-contribuicoes`, then é bloqueado como as demais rotas protegidas por persona.

## Spec Change Log

- 2026-07-20: layout do C100 validado contra arquivo real de produção (`EFD_CONTRIB_092021_V2.txt`); item "Ask First" correspondente resolvido. Confirmado também que notas não casadas não bloqueiam o job (só reportam).

## Design Notes

~~O layout exato do registro C100 do EFD Contribuições ... deve ser conferido~~ — feito em 2026-07-20 contra arquivo real de produção (ver seção "Ask First" acima e comentário em `processEFDContribuicoesFile`, worker.go).

## Verification

**Commands:**
- `cd backend && go build ./...` -- compila sem erros
- `cd backend && go test ./worker/... ./handlers/...` -- nenhum teste existente quebra

**Manual checks (if no CLI):**
- Upload de um arquivo `.txt` sintético com 2-3 linhas `C100` (chaves mapeadas a notas de teste já existentes em `nfe_entradas`) e conferir: `v_pis`/`v_cofins` atualizados, linhas em `efd_contribuicoes_matches`, resumo exibido na tela.
