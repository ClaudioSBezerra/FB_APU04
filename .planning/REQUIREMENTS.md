# Requirements: FB_APU04

**Defined:** 2026-05-08
**Core Value:** Escrituração fiscal completa e auditável — todos os valores tributários (PIS, COFINS, IPI, ICMS) corretos por nota, com rastreabilidade até o documento original (XML ou ERP), pronta para fiscalização da Receita Federal.

## v1 Requirements

Requisitos do ciclo atual, organizados pelas frentes do PROJECT.md em ordem de prioridade.

### Estabilização (prioridade 1) — Pós-incidente APU02 + bug de cache

- [x] **STAB-01**: `ResetDatabaseHandler` exige confirmação obrigatória (token de confirmação ou prompt textual `DELETE-FB_APU04` digitado pelo usuário) antes de executar TRUNCATE
- [x] **STAB-02**: Backup automático do estado atual antes de qualquer execução de `ResetDatabaseHandler` (dump das tabelas afetadas para `/backups/reset-{timestamp}.sql`)
- [x] **STAB-03**: Audit log de toda execução de reset (usuário, timestamp, registros impactados, scope) gravado em tabela `admin_destructive_actions`
- [x] **STAB-04**: Restrição de role — apenas role `admin` global (não Environment Admin) pode invocar reset de banco completo; Environment Admin restrito a `ResetCompanyDataHandler`
- [x] **STAB-05**: Rate limit no endpoint de reset (1 execução por hora por usuário) para evitar reset acidental em loop
- [x] **STAB-10**: Resolver bug de cache em `simu.fcxlabs.com/login` — primeira visita mostra página de login do app anterior (FC Bots) ao invés do FB_APU04; usuários precisam dar `Ctrl+Shift+R` para ver a página correta. Investigar service worker stale do app anterior, cache do Traefik/Coolify, e cache do nginx; aplicar correção que invalida SW antigo e força reload da página correta na primeira visita

### Importação XML (prioridade 2) — Nova fonte de dados

- [x] **XML-01**: Tela de upload com drag-and-drop aceita arquivos XML único ou ZIP contendo múltiplos XMLs de NF-e
- [x] **XML-02**: Validação de schema NF-e v4.00 (NFe + protNFe) antes da persistência; XMLs inválidos rejeitados com motivo claro
- [x] **XML-03**: Parser extrai e popula as tabelas `nfe_entradas` / `nfe_saidas` (mesmas tabelas do ERP Bridge) com PIS, COFINS, IPI, ICMS, CFOP, valores
- [x] **XML-04**: Resolução de conflito por chave de acesso — quando NF-e já existe via Oracle Bridge, **XML sobrescreve** os campos tributários e marca origem como `xml_upload`
- [x] **XML-05**: Histórico de uploads visível no painel — quem subiu, quando, quantos XMLs processados/rejeitados, link para reprocessar
- [x] **XML-06**: Coluna `source` nas tabelas (`oracle_bridge` | `xml_upload` | `manual`) para auditoria de origem
- [x] **XML-07**: Limite de tamanho por upload (configurável, default 100MB ou 5000 XMLs por ZIP) com mensagem clara quando excedido
- [x] **XML-08**: Background job processa XMLs grandes (>50 arquivos) sem bloquear a UI; usuário recebe status via toast/notificação

### Estabilização adicional (prioridade 3)

- [ ] **STAB-06**: Tirar credenciais hardcoded de `backend/.env` (SMTP password, ZAI_API_KEY) e `installer/.env` — migrar para Coolify env vars ou secret manager
- [ ] **STAB-07**: Bootstrap de testes Go — testes unitários para handlers críticos (`admin.go` reset, `auth.go`, `erp_bridge_*.go`) com cobertura mínima 30%
- [x] **STAB-08**: Bootstrap de testes React — testes para componentes-chave (Dashboard, Upload XML, Reset Database confirmation) com Vitest
- [ ] **STAB-09**: Retry/reconnect automático no Bridge Python para erros DPY-4011 — detectar perda de conexão Oracle e reconectar transparentemente sem perder o run

### Expansão fiscal (prioridade 4)

- [x] **EXP-01**: Conciliação automática entre dados do ERP Bridge e XML upload — relatório de divergências de valores tributários
- [x] **EXP-02**: Dashboard de cobertura — % de NF-es com fonte XML (autêntica) vs apenas Oracle Bridge

### Observabilidade (prioridade 5)

- [x] **OBS-01**: Dashboards Grafana dedicados (Prometheus já provisionado em prod) — runs do Bridge, latência API, taxa de erro, ocupação do banco
- [x] **OBS-02**: Alertas para falhas críticas — erros DPY-4011 consecutivos, falhas de upload XML, reset de banco executado

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Modernização

- **MOD-01**: Avaliar substituir materialized views por refresh on-demand ou cache em Redis (Redis hoje provisionado mas não usado)
- **MOD-02**: Pasta watch / S3 bucket para upload XML automatizado (alternativa ao manual)
- **MOD-03**: Multi-cliente comercial agressivo (vendas externas para outras empresas) — tenancy lógico já existe, falta apenas processo de onboarding e billing

### Compliance avançado

- **COMP-01**: Integração ativa com Receita Federal API CBS/IBS (URLs já em template, mas não implementado)
- **COMP-02**: Geração automática de SPED em layouts adicionais (além do 020)

## Out of Scope

| Feature | Reason |
|---------|--------|
| Reescrita do Bridge em Go | Python+oracledb funciona; complexidade está na lógica fiscal, não na linguagem |
| Migração de stack (Go→outro / React→outro / Postgres→outro) | Produto em produção e estável; mudar fundações destruiria valor |
| Reset/limpeza por API sem UI | Pós-incidente, qualquer destrutivo requer fluxo UI explícito |
| Multi-cliente comercial agressivo neste ciclo | Tenancy lógico cobre caso interno; venda externa é decisão comercial separada |
| Suporte a NFC-e (consumidor final) e NFS-e | Foco é NF-e (mercantil); NFS-e foi removida em commit anterior |

## Traceability

Mapeamento requisito → fase. Atualizado quando o ROADMAP.md for criado.

| Requirement | Phase | Status |
|-------------|-------|--------|
| STAB-01 a STAB-05, STAB-10 | Phase 1 | Complete (2026-05-08) |
| XML-01 a XML-08 | Phase 2 | Complete (2026-05-16) |
| STAB-06 a STAB-09 | Phase 3 | Complete (2026-05-16) |
| EXP-01 a EXP-02 | Phase 4 | Complete (2026-05-16) |
| OBS-01 a OBS-02 | Phase 5 | Complete (2026-05-17) |

**Coverage:**
- v1 requirements: 25 total
- Mapped to phases: 25
- Unmapped: 0 ✓

---
*Requirements defined: 2026-05-08*
*Last updated: 2026-05-08 after initialization*
