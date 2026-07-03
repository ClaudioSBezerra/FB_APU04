# Milestones

## v6.00 Módulo Teste Pacote Fiscal (Shipped: 2026-07-03)

**Phases completed:** 2 phases (11-12), 9 plans, 18 tasks

**Key accomplishments:**

- go-ora v2.9.0 instalado e vendorizado; `openFiscalOracleConn`/`FiscalOraclePingHandler` portados para `backend/handlers/fiscal_oracle_conn.go`; rota admin `POST /api/fiscal/oracle-ping` registrada e testada de ponta a ponta contra o Oracle FCCORP real (10.131.1.118:1521), confirmando alcançabilidade de rede + protocolo via handshake TNS completo (ORA-01017, não erro de rede).
- Migration 146 adiciona v_desc/v_outro (NUMERIC(15,2) DEFAULT 0) a nfe_saidas_itens e nfe_entradas_itens; struct `prod` passa a parsear `vOutro` item-level e `insertNFeItens` grava/atualiza ambos os campos via ON CONFLICT DO UPDATE — fecha o único gap de dados de entrada identificado para os 23 parâmetros IN do pacote fiscal (`pDesconto`/`pDespesas`).
- Lookup de grupo fiscal via Oracle prod/PRODB com erro explícito para filial não mapeada, e tabela fiscal_execution_items no modelo híbrido (colunas típicas + JSONB) com colunas IBS/CBS para a Reforma Tributária.
- `backend/services/oracle_fiscal.go` portado verbatim do FB_TESTESFC: 23 parâmetros IN / 88 campos OUT do `PKG_FISCAL_FCTAX.calcula_imposto_produto` mapeados via duas tabelas de metadados fixas, bloco PL/SQL anônimo gerado 100% por reflection sobre essas tabelas (zero concatenação de input), e `CallFiscalPackage` exportada com bind seguro (`sql.Named` + `go_ora.Out{Size:4000}`).
- `POST /api/fiscal/execute` costura conexão Oracle dedicada + lookup de grupo fiscal + chamada do pacote `PKG_FISCAL_FCTAX` + persistência em `fiscal_execution_items`, com fan-out de goroutines (semáforo cap 5), timeout de 15s por item, isolamento de panic e upsert por item — nunca uma transação única do lote.
- Três handlers HTTP admin-gated (busca por número/chave, leitura da comparação item a item, export CSV) que expõem `fiscal_execution_items` (Fase 11) comparado a `nfe_saidas_itens`, com o 4º estado "nunca executado" e a soma de IBS resolvidos em SQL único e reutilizado.
- Tela React "Comparação Fiscal" completa — busca NF-e por número/chave (combobox debounced server-side), dispara `POST /api/fiscal/execute` e recarrega automaticamente 6 impostos (ICMS/ICMS-ST/PIS/COFINS/IBS/CBS) com tolerância zero, 4 estados de badge, filtro "só divergentes", resumo agregado (4 cards + 6 chips) e exportação Excel/CSV.
- Item de navegação "Teste Pacote Fiscal → Comparação Fiscal" com gate `adminOnly: true` (3 arquivos, reuso verbatim do padrão "malha"), fluxo completo busca→executar→comparar aprovado pelo usuário nos 10 passos de verificação manual.

**Dívida conhecida ao fechar (ver STATE.md § Deferred Items):** 2 cenários UAT da Fase 11 pendentes de credenciais Oracle reais; gaps de verificação humana das Fases 08/09/11 (inclui achado de segurança CR-02 na Fase 08 — regras fiscais globais BA/CE editáveis por qualquer usuário autenticado).

---
