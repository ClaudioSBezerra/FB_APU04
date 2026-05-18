# Phase 02: Upload de XMLs (Drag-and-Drop) — Context

**Gathered:** 2026-05-16
**Status:** Ready for planning
**Source:** User context provided at /gsd-plan-phase 2

---

## Phase Boundary

**Goal:** Adicionar segunda fonte de dados (XML SEFAZ) alimentando as mesmas tabelas do ERP Bridge, com prioridade do XML em conflitos por chave de acesso — e reativar a importação de XMLs que estava desativada no ERP_BRIDGE.

**In scope:** Parser NFe/CTe v4.00, upload drag-and-drop, reativação do fluxo ERP_BRIDGE XML, painéis de visualização, relatórios de saneamento CCLASSTRIB.
**Out of scope:** Migração de stack, watch automático de pasta, multi-cliente comercial.

---

## Locked Decisions

Estas decisões foram fornecidas pelo usuário e devem ser honradas no plano — não estão abertas para negociação pelo planner.

### Usuários e contexto de uso

**LOCKED:** O sistema será usado por **escritórios de contabilidade** que cadastram seus clientes (empresas) por CNPJ.

**LOCKED:** Estrutura de tenancy para escritórios:
- Ambiente: "Servidor Senior Excelente"
- Grupo: "Grupo Senior Excelente"
- Empresas: Clientes do escritório

**LOCKED:** Ao cadastrar empresa, identificar o **regime tributário**: Lucro Real, Lucro Presumido ou Simples Nacional.

**LOCKED:** Para empresas do **Lucro Real e Presumido**, a importação de EFD ICMS é obrigatória.

### Reativação ERP_BRIDGE

**LOCKED:** Reativar a importação de XMLs de Entrada e Saída que está **desativada no ERP_BRIDGE** — criar abas separadas por tipo de importação (Entradas, Saídas, CT-Es).

### Dados de XMLs de Entradas e CT-Es

**LOCKED:** Extrair e persistir os seguintes dados de cada nota de entrada/CT-e:

**Cabeçalho:**
- Dados do Emitente (Fornecedor): CNPJ, Razão Social
- Dados do Destinatário: reconhecer como filial da empresa cadastrada
- Dados da Nota: número, série, modelo, chave eletrônica, data emissão
- Identificação Simples Nacional: Tag `<CRT>` = 1 → alimentar tabela interna de fornecedores do Simples Nacional

**Itens (por linha da nota):**
- Código, Descrição, NCM, CFOP, CST
- VLR TOTAL, BASE ICMS, VLR ICMS, VLR PIS, VLR COFINS, VLR IPI
- BASE IBS/CBS (se houver), VLR IBS (se houver), VLR CBS (se houver)
- CCLASSTRIB (se houver)

### Dados de XMLs de Saídas

**LOCKED:** Extrair e persistir para notas de saída:

**Cabeçalho:**
- Dados do Emitente: reconhecer como filial da empresa cadastrada
- Dados do Destinatário: reconhecer como cliente
- Dados da Nota: número, série, modelo, chave eletrônica, data emissão
- Identificação Simples Nacional: Tag `<CRT>` = 1 → alimentar tabela interna

**Itens (mesmos campos das entradas):** Código, Descrição, NCM, CFOP, CST, VLR TOTAL, BASE ICMS, VLR ICMS, VLR PIS, VLR COFINS, VLR IPI, BASE IBS/CBS, VLR IBS, VLR CBS, CCLASSTRIB

### Painéis de visualização

**LOCKED:** Criar painel **próprio para XMLs importados** (separado do painel EFD), com abas para Entradas, Saídas e CT-Es — similar ao painel de Mercadorias existente.

**LOCKED:** Usar **VIEWs PostgreSQL** para performance no painel. Ao importar os dados, gerar a VIEW automaticamente — processo similar ao do EFD ICMS já implementado.

### Conflito Oracle vs XML

**LOCKED (do ROADMAP original):** Quando NF-e já existe via Oracle Bridge, **XML sobrescreve** os campos tributários e atualiza `source = 'xml_upload'`.

### Relatórios de saneamento

**LOCKED:** Criar os seguintes relatórios:

1. **Relatório de saneamento CCLASSTRIB:** comparar NCMs dos itens vendidos e identificar divergências de classificação tributária
2. **Exportação CSV:** arquivo para a empresa importar e fazer saneamento automático do cadastro (preenchimento de CCLASSTRIB faltantes)
3. **Relatório de fornecedores com classificações erradas:** identificar fornecedores cujas NF-es têm CCLASSTRIB incorreto ou ausente

### Simples Nacional

**LOCKED:** Tag `<CRT>` (Código de Regime Tributário): valor `1` = Simples Nacional. Quando identificado, alimentar tabela interna `fornecedores_simples_nacional` para uso em rotinas existentes de cálculo diferenciado.

---

## Claude's Discretion

As seguintes decisões técnicas ficam a cargo do planner, respeitando as restrições acima:

- Estrutura exata das migrations (nomes de colunas, índices)
- Formato do drag-and-drop (biblioteca React, UX de progresso)
- Background job para XMLs grandes (Redis queue vs goroutine pool)
- Estrutura das VIEWs PostgreSQL (materializada vs regular)
- Formato do CSV de exportação
- Ordem de implementação dos planos (waves)

---

## Canonical References

- `.planning/ROADMAP.md` — Phase 2 goals e success criteria originais
- `.planning/REQUIREMENTS.md` — XML-01 a XML-08
- `.planning/codebase/ARCHITECTURE.md` — arquitetura 3-tier + bridge
- `.planning/codebase/STRUCTURE.md` — estrutura de arquivos existente
- `.planning/codebase/STACK.md` — stack técnico (Go 1.22, React 18, PostgreSQL 15, Redis)
- `.planning/phases/01-estabiliza-o-cr-tica-reset-cache/01-01-SUMMARY.md` — contexto Phase 1 backend

---

*Context created: 2026-05-16*
