---
phase: 08-cadastro-empresas-ambiente-uf
plan: "03"
subsystem: frontend-components
tags: [frontend, react, typescript, icms-fronteira, companies, uf-tabs, crud]
dependency_graph:
  requires:
    - "08-01: migrations 096/097/098 (schema companies + uf_estado + MVA ajustado)"
    - "08-02: handlers backend com 7 campos empresa + filtro uf_estado + UpdateHandler"
  provides:
    - "UI: Modal Nova Empresa com 8 campos (CNPJ, name, trade_name, regime, IE, CNAE, município, segmento)"
    - "UI: Edição inline de empresa com grid 2 colunas (5 campos novos + regime)"
    - "UI: Tabs PE/BA/CE aninhadas em RegrasTab de IcmsFronteira.tsx"
    - "UI: Fetch de regras filtrado por UF via queryKey + URL ?uf_estado"
    - "UI: Criação de regra envia uf_estado no body; upload envia uf_estado no FormData"
    - "UI: Label do módulo fronteira atualizado para 'ICMS Fronteira'"
  affects:
    - checkpoint humano Task 3 — verificação visual end-to-end
tech_stack:
  added: []
  patterns:
    - "useState por campo (sem react-hook-form) — padrão existente no GestaoAmbiente.tsx"
    - "Tabs shadcn aninhadas com selectedUF state — padrão já em uso no IcmsFronteira.tsx"
    - "queryKey com selectedUF para cache isolado por UF via TanStack Query"
    - "FormData.append('uf_estado', selectedUF) para upload de planilha por UF"
    - "JSON.stringify({ ...body, uf_estado: selectedUF }) em createMutation"
key_files:
  created: []
  modified:
    - frontend/src/pages/GestaoAmbiente.tsx
    - frontend/src/pages/IcmsFronteira.tsx
    - frontend/src/lib/navigation.ts
decisions:
  - "Tabs UF sem TabsContent replicado — o conteúdo (tabela + formulários) permanece fora dos TabsContent; o filtro é feito via selectedUF na query (abordagem simples, sem triplicar JSX)"
  - "cnae_secundario e incentivos_fiscais não expostos no formulário inline — array e JSONB requerem UX específica; deferidos para versão futura"
  - "Título do card de importação exibe UF selecionada para feedback visual imediato"
  - "handleUpdateCompany exibe mensagem de erro do backend via toast (suporta 'CNPJ deve ter 14 dígitos numéricos' do handler Go)"
metrics:
  duration: "~20 minutos"
  completed: "2026-05-23"
  tasks_completed: 2
  tasks_total: 3
  files_created: 0
  files_modified: 3
---

# Phase 08 Plan 03: Frontend — GestaoAmbiente expandido + Tabs UF em IcmsFronteira

**One-liner:** Expande GestaoAmbiente.tsx com modal de empresa de 8 campos e edição inline expandida (5 novos campos + regime) e adiciona Tabs PE/BA/CE aninhadas em IcmsFronteira.tsx com queryKey por UF, FormData com uf_estado e body de criação com uf_estado.

## Tasks Executadas

| Task | Nome | Commit | Arquivos |
|------|------|--------|----------|
| 1 | Expandir GestaoAmbiente.tsx — modal e edição inline com 7 campos | 0c92dbf | frontend/src/pages/GestaoAmbiente.tsx |
| 2 | Tabs UF em IcmsFronteira.tsx + filtros por UF + label navigation.ts | 356747a | frontend/src/pages/IcmsFronteira.tsx, frontend/src/lib/navigation.ts |
| 3 | Checkpoint humano — verificação visual end-to-end | — | aguardando aprovação humana |

## O Que Foi Construído

### Task 1 — GestaoAmbiente.tsx expandido (CADU-03)

**Interface Company:** expandida de 7 para 13 campos — 5 novos campos opcionais:
- `inscricao_estadual?: string`
- `cnae_principal?: string`
- `cnae_secundario?: string[]`
- `municipio?: string`
- `segmento_economico?: string`
- `incentivos_fiscais?: unknown`

**useState para modal Nova Empresa:** 4 novos (`newCompanyIE`, `newCompanyCNAE`, `newCompanyMunicipio`, `newCompanySegmento`), cada um com reset pós-sucesso.

**useState para edição inline:** 5 novos (`editCNPJ`, `editIE`, `editCNAE`, `editMunicipio`, `editSegmento`), inicializados a partir do objeto `company` selecionado ao clicar no botão de editar.

**handleCreateCompany:** body POST expandido com os 4 novos campos (inscricao_estadual, cnae_principal, municipio, segmento_economico).

**handleUpdateCompany (renomeado de handleUpdateCompanyRegime):** body PATCH expandido com 6 campos (regime_tributario, cnpj, inscricao_estadual, cnae_principal, municipio, segmento_economico). Exibe mensagem de erro do backend no toast (suporta validação CNPJ 14 dígitos do handler Go).

**Modal Nova Empresa:** 8 inputs total (CNPJ, Razão Social, Nome Fantasia, Regime Tributário, Inscrição Estadual, CNAE Principal, Município, Segmento Econômico).

**Painel inline de edição:** substituído Select de regime por layout `grid grid-cols-2 gap-2` com 6 fields (CNPJ, IE, CNAE, Município, Segmento + Select regime ocupando linha inteira).

### Task 2 — IcmsFronteira.tsx + navigation.ts (CADU-07)

**Interface RegraNCM:** expandida com 4 campos novos (`mva_ajustado_4pct`, `mva_ajustado_7pct`, `mva_ajustado_12pct: number | null` e `uf_estado: string`).

**selectedUF state:** `useState<'PE' | 'BA' | 'CE'>('PE')` adicionado no topo de `RegrasTab`.

**useQuery:** `queryKey` alterado para `['icms-fronteira/regras', selectedUF]`; URL alterada para template literal `` `/api/icms-fronteira/regras?uf_estado=${selectedUF}` ``. A troca de UF invalida o cache da UF anterior e busca dados frescos da nova UF.

**createMutation:** body expandido com `{ ...body, uf_estado: selectedUF }`; `onSuccess` invalida `['icms-fronteira/regras', selectedUF]`.

**deleteMutation:** `onSuccess` invalida `['icms-fronteira/regras', selectedUF]`.

**handleImport:** `fd.append('uf_estado', selectedUF)` adicionado antes do fetch; `onSuccess` invalida a query com selectedUF.

**Tabs aninhadas:** `<Tabs value={selectedUF} onValueChange={...}>` com 3 `<TabsTrigger>` (PE/BA/CE) inseridas antes do Card de importação. Título do Card exibe UF ativa.

**navigation.ts:** label `'ICMS Fronteira — PE'` → `'ICMS Fronteira'`.

### Task 3 — Checkpoint humano (pendente)

Verificação visual end-to-end aguardando aprovação do usuário. Veja detalhes no checkpoint.

## Deviations from Plan

None - plan executed exactly as written.

## Known Stubs

Nenhum. Todos os campos exibidos no formulário são reais e serão enviados ao backend. A tabela de regras exibe dados reais filtrados por UF via API. Os campos `cnae_secundario` e `incentivos_fiscais` foram intencionalmente excluídos do formulário (conforme instrução do plano — UX complexa; deferido para versão futura), mas estão presentes na interface TypeScript.

## Threat Surface Scan

Nenhuma nova superfície de rede introduzida além do mapeado no threat model do plano:

- T-08-12 (uf_estado manipulável no client): frontend é cooperativo; backend aplica whitelist independentemente
- T-08-14 (XSS via campos de texto): React escapa todo conteúdo em JSX; sem dangerouslySetInnerHTML
- T-08-15 (CNPJ inválido): maxLength=14 no Input limita digitação; backend retorna 400 com mensagem exibida via toast
- T-08-SC (npm installs): nenhuma dependência nova — Tabs shadcn, useQuery, useState já em uso

## Self-Check: PASSED

- [x] `frontend/src/pages/GestaoAmbiente.tsx` existe — confirmado (modificado)
- [x] `frontend/src/pages/IcmsFronteira.tsx` existe — confirmado (modificado)
- [x] `frontend/src/lib/navigation.ts` existe — confirmado (modificado)
- [x] Commit 0c92dbf existe — confirmado (Task 1)
- [x] Commit 356747a existe — confirmado (Task 2)
- [x] `cd frontend && npx tsc --noEmit` — saída vazia (código 0, sem erros de tipo)
- [x] Interface Company contém 5+ campos novos opcionais — confirmado (`grep -c` retornou 5)
- [x] 4 useState newCompany* novos — confirmado (linhas 103-106 GestaoAmbiente.tsx)
- [x] 5 useState edit* novos — confirmado (linhas 108-112 GestaoAmbiente.tsx)
- [x] handleCreateCompany com inscricao_estadual: newCompanyIE — confirmado
- [x] handleUpdateCompany com 5 campos editáveis no body PATCH — confirmado (5 matches)
- [x] Modal Nova Empresa com 4 Inputs novos — confirmado (4 matches)
- [x] Edição inline com 5 inputs novos — confirmado (5 matches)
- [x] selectedUF useState em IcmsFronteira.tsx — confirmado
- [x] queryKey inclui selectedUF — confirmado
- [x] URL fetch com ?uf_estado=${selectedUF} — confirmado
- [x] fd.append('uf_estado', selectedUF) — confirmado
- [x] 3 TabsTrigger PE/BA/CE — confirmado
- [x] mva_ajustado_4/7/12pct na interface RegraNCM — confirmado (3 matches)
- [x] uf_estado: string na interface RegraNCM — confirmado
- [x] navigation.ts label 'ICMS Fronteira' — confirmado
- [x] navigation.ts sem 'ICMS Fronteira — PE' — confirmado (0 matches)
