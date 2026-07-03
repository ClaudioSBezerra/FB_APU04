# Phase 11: Motor de Execução do Pacote Fiscal (Backend) - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-03
**Phase:** 11-motor-de-execu-o-do-pacote-fiscal-backend
**Areas discussed:** Conexão Oracle dedicada, Schema da tabela fiscal_execution_items

---

## Conexão Oracle dedicada

| Option | Description | Selected |
|--------|-------------|----------|
| Reusar erp_bridge_config | Backend Go lê credenciais já salvas em erp_bridge_config por company_id e abre sua própria conexão go-ora direta e síncrona (não passa pelo bridge Python) | ✓ |
| Conexão dedicada separada (env vars) | Credenciais em variáveis de ambiente próprias, como o FB_TESTESFC fazia (single-tenant) | |

**User's choice:** Reusar erp_bridge_config (recomendado)
**Notes:** Reaproveita armazenamento de credencial já criptografado e existente por empresa; evita duplicar cadastro. Diferença técnica importante: é a primeira vez que o backend Go abre uma conexão Oracle síncrona em tempo de requisição (uso atual de erp_bridge_config é só leitura assíncrona pelo bridge Python externo).

---

## Schema da tabela fiscal_execution_items

| Option | Description | Selected |
|--------|-------------|----------|
| Padrão simples de nfe_saidas_itens | Schema public, FK ON DELETE CASCADE, UNIQUE em nfe_item_id para upsert, sem particionamento | ✓ |
| Quero decidir os detalhes agora | Discutir índices/retenção/estrutura específica | |

**User's choice:** Padrão simples de nfe_saidas_itens (recomendado)
**Notes:** Volume esperado baixo (uso administrativo de validação, não o fluxo de todas as notas) — não justifica particionamento.

---

## Claude's Discretion

- Nome exato das colunas da migration de `fiscal_execution_items` (mapear 1:1 com `RDADOS_FISCAIS_PRODUTO`).
- Estrutura interna do pool de conexão Oracle (compartilhado vs. por-request).

## Confirmado por inspeção de código (não foi pergunta ao usuário)

- **TPF-02 confirmado necessário**: `insertNFeItens` já parseia `VDesc` na struct `det` mas não persiste; `VOutro` nem está na struct. Escopo: adicionar `v_desc`/`v_outro` na struct, tabela e INSERT.

## Deferred Ideas

- Tela "Comparação Fiscal" (TPF-06/07/08) — Fase 12.
- Sistema de permissão granular por módulo — milestone futura.
- Otimização de pool de conexão Oracle — futuro, se necessário.
