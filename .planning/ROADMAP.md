# Roadmap: FB_APU04 — Simulador Fiscal

**Created:** 2026-05-08
**Granularity:** Coarse (5 fases largas)
**Mode:** YOLO (autonomous execution)

## Vision

Estabilizar o produto pós-incidente, entregar a nova capacidade de upload XML como segunda fonte de dados, depois evoluir incrementalmente nas frentes de qualidade, observabilidade e expansão fiscal — sem comprometer estabilidade conquistada.

## Phase Structure

### Phase 1 — Estabilização Crítica do Reset

**Goal:** Tornar impossível repetir o incidente de 2026-05-07. Qualquer operação destrutiva no banco deve ter confirmação, backup e auditoria.

**Scope:**
- Confirmação obrigatória no `ResetDatabaseHandler` (token textual digitado)
- Backup automático antes de TRUNCATE para `/backups/reset-{timestamp}.sql`
- Audit log em `admin_destructive_actions` (usuário, timestamp, scope, registros impactados)
- Restrição de role: apenas `admin` global executa reset completo; Environment Admin restrito a `ResetCompanyDataHandler`
- Rate limit (1 reset/hora/usuário)
- Tela frontend de confirmação com avisos visuais explícitos do impacto

**Requirements:** STAB-01, STAB-02, STAB-03, STAB-04, STAB-05

**Success Criteria:**
- Reset não executa sem token de confirmação correto
- Backup gerado e armazenado antes de qualquer TRUNCATE
- Audit log auditável via query SQL
- Apenas admins globais conseguem reset completo via UI e API
- Tentativas de reset em <1h retornam erro 429

**Dependencies:** Nenhuma (fundação)

**Estimated effort:** 1-2 plans

---

### Phase 2 — Importação de XMLs (Upload Manual)

**Goal:** Adicionar segunda fonte de dados (XML SEFAZ) alimentando as mesmas tabelas do ERP Bridge, com prioridade de XML em conflitos.

**Scope:**
- Tela de upload drag-and-drop (XML único ou ZIP com múltiplos)
- Validação de schema NF-e v4.00 (NFe + protNFe)
- Parser que popula `nfe_entradas` / `nfe_saidas` com PIS/COFINS/IPI/ICMS/CFOP
- Resolução de conflito: XML sobrescreve campos tributários quando chave já existe
- Coluna `source` (`oracle_bridge` | `xml_upload` | `manual`) nas tabelas afetadas
- Histórico de uploads (quem, quando, processados/rejeitados, reprocessar)
- Limite configurável (default 100MB / 5000 XMLs/ZIP) com mensagem clara
- Background job para processamentos grandes (>50 arquivos) sem bloquear UI
- Toast/notificação de progresso

**Requirements:** XML-01 a XML-08

**Success Criteria:**
- Upload de 1 XML válido aparece nas tabelas `nfe_entradas/saidas` em <5s
- Upload de ZIP com 1000 XMLs processa em <2min sem bloquear UI
- XMLs malformados rejeitados com motivo legível
- Conflito Oracle vs XML: campos tributários sobrescritos por XML, `source` atualizado
- Histórico de uploads consultável por filial/período
- Schema-validation rejeita NFes que não passem na assinatura/protocolo

**Dependencies:** Phase 1 (proteções de banco em pé antes de adicionar nova fonte de gravação)

**Estimated effort:** 3-4 plans

---

### Phase 3 — Estabilização Adicional

**Goal:** Reduzir dívida técnica em segredos, testes e resilência do bridge — pré-requisitos para escalar com confiança.

**Scope:**
- Migrar `backend/.env` (SMTP, ZAI_API_KEY) e `installer/.env` para env vars do Coolify ou secret manager
- Bootstrap de testes Go: handlers críticos (`admin.go` reset, `auth.go`, `erp_bridge_*.go`) — meta 30% cobertura
- Bootstrap de testes React: componentes-chave (Dashboard, Upload XML, Reset confirmation) com Vitest
- Retry/reconnect automático no `bridge.py` para `DPY-4011` Oracle (sem perder o run em andamento)
- Documentação de processo de gestão de segredos em `docs/`

**Requirements:** STAB-06, STAB-07, STAB-08, STAB-09

**Success Criteria:**
- Nenhum segredo real em arquivos `.env*` versionados
- Pipeline CI executa testes Go e React em cada PR
- Cobertura Go ≥30% nos pacotes `handlers/` e `services/`
- Bridge SAP sobrevive a `DPY-4011` reconectando sem intervenção humana
- Run interrompido por DPY-4011 retoma do ponto de parada via tracker

**Dependencies:** Phase 1 (audit log + reset protection já em pé permite testar com segurança)

**Estimated effort:** 3-4 plans

---

### Phase 4 — Expansão Fiscal e Conciliação

**Goal:** Aproveitar as duas fontes de dados (Bridge + XML) para gerar valor fiscal direto: conciliação, divergências, cobertura.

**Scope:**
- Conciliação automática entre dados do Oracle Bridge e XML upload — gera relatório de divergências por nota
- Dashboard de cobertura — % de NF-es com fonte XML (autêntica SEFAZ) vs apenas Oracle Bridge
- Drill-down nas divergências: ver lado-a-lado os valores de cada fonte
- Exportação do relatório para PDF/Excel para auditoria

**Requirements:** EXP-01, EXP-02

**Success Criteria:**
- Relatório de divergências gerado para qualquer período/filial em <10s
- Dashboard mostra cobertura XML por filial/mês de forma visual
- Auditor consegue exportar relatório completo de divergências
- Divergências mostram delta tributário (ex: "PIS Bridge=12.34, PIS XML=12.50, Δ=R$0.16")

**Dependencies:** Phase 2 (precisa de XML upload em produção para gerar dados)

**Estimated effort:** 2-3 plans

---

### Phase 5 — Observabilidade e Alertas

**Goal:** Aproveitar Prometheus já provisionado para ganhar visibilidade operacional — dashboards e alertas para os fluxos críticos.

**Scope:**
- Dashboards Grafana — runs do Bridge (sucesso/falha, throughput), latência API por endpoint, taxa de erro 5xx, tamanho do banco, materialized views (idade do refresh)
- Alertas críticos — DPY-4011 consecutivos no Bridge, falhas de upload XML, reset de banco executado, erro 5xx >1%
- Notificações via SMTP (já configurado) ou Slack webhook
- Documentação de runbooks para cada alerta

**Requirements:** OBS-01, OBS-02

**Success Criteria:**
- Dashboards funcionais em `monitoring.simu.fbtax.cloud` (ou subdomínio)
- Cada alerta crítico dispara em <1min após o evento
- Runbook escrito para cada tipo de alerta com passos de mitigação
- Equipe fiscal consegue ver status do Bridge sem acesso SSH

**Dependencies:** Phases 1-3 (sistema estabilizado antes de instrumentar)

**Estimated effort:** 2-3 plans

---

## Phase Dependency Graph

```
Phase 1 (Estabilização Reset)
    ↓
Phase 2 (XML Upload) ──────┐
    ↓                      │
Phase 3 (Estab. adicional) │
    ↓                      │
Phase 4 (Conciliação) ←────┘  (precisa de Phase 2)
    ↓
Phase 5 (Observabilidade)
```

## Coverage

- **v1 requirements:** 24 total
- **Mapped to phases:** 24 (100%)
- **Phases:** 5
- **Average requirements per phase:** ~5

---
*Roadmap created: 2026-05-08*
