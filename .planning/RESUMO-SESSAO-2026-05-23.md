# Resumo da Sessão — 2026-05-23 (noite)

## O que foi entregue hoje

### 1. Phase 9 completa (Módulos 2.x — Analytics Dimensional)
- Backend `reforma_modulo2.go`: 4 handlers JSON + 2 CSV (CFOP, NCM, UF-destino, B2B/B2C) + 6 rotas
- Frontend: 4 páginas (Reforma21/22/23/24) + mapa coroplético + 4 tabs habilitadas
- Code review: 6 findings corrigidos (2 blockers + 4 warnings)
- Verificação: 10/10 must-haves
- **Deployado** (commits até `cf626dd`, fixes CI threshold 26% + package-lock)

### 2. ICMS Fronteira — 5 bugs de cálculo (G1-G5) — DEPLOYADO
- G1: filtro `uf_estado` na regra NCM (BA/CE usavam regra de PE)
- G2: MVA ajustado (Convênio 110/07) em vez de MVA original
- G3: `reducao_bc_pct` aplicado no cálculo
- G4: `v_frete` (frete CIF) na base de cálculo
- G5: gross-up PE não aplicado a fornecedor Simples Nacional
- Commit `6f202a3`

### 3. ICMS Fronteira — seed BA/CE + totalizações (G13-G14) — DEPLOYADO
- G13: BA/CE expandidos de 7 → 40 NCMs cada (migration 099)
- G14: window functions COUNT/SUM corrigem totais com paginação
- Commit `e553916`

### 4. Edição de nome de usuário + diagnóstico — DEPLOYADO
- PromoteUserHandler aceita `full_name`; input "Nome" no AdminUsers
- Endpoint `/api/admin/diagnostic` + botão "Diagnóstico de Dados"
- Commits `dac12fe`, `df51944`

### 5. **CORREÇÃO PRINCIPAL — Fronteira lia do XML, agora lê do SPED — DEPLOYADO**
- **Causa raiz dos relatórios vazios**: queries liam `nfe_entradas` (XML), onde o
  CFOP fica no item e na perspectiva do FORNECEDOR (saída 6102/6152) e o cabeçalho
  fica vazio. O Fronteira classifica por CFOP de ENTRADA (2102/2152/2403/2652/2551),
  que só existe no SPED `reg_c190`.
- Reescritas 3 queries (`icms_fronteira.go`, `_itens.go`, `_divergencias.go`) para
  ler do SPED + join XML por chave para detalhe de NCM.
- Commit `6e31fdf`

---

## Estado atual de TODOS os relatórios (testado local com base ROLIMEC)

| Relatório | Status | Observação |
|-----------|--------|------------|
| Reforma 1.1 créditos | 0 linhas — **CORRETO** | base não tem ST retida nem CST 51 |
| Reforma 1.2 reprecificação | 190 linhas ✓ | |
| Reforma 1.3 ranking | 4 linhas ✓ | |
| Reforma 1.4 split payment | OK ✓ | |
| Reforma 2.1 NCM | 23 linhas ✓ | |
| Reforma 2.2 CFOP | 1 linha ✓ | |
| Reforma 2.3 UF-destino | 7 linhas ✓ | |
| Reforma 2.4 B2B/B2C | 2 linhas ✓ | |
| Fronteira Resumo | 3 regimes, R$106.089 ✓ | |
| Fronteira Antecipação | 98 linhas, R$99.776 ✓ | |
| Fronteira ST | 8 linhas, R$0 | base sem ST retida; NCMs sem MVA |
| Fronteira DIFAL | 1 nota, R$6.312 ✓ | |
| Fronteira Itens | 210 itens, R$73.362 ✓ | só notas com XML casado (68 chaves) |
| Fronteira Mensal | R$106.089 ✓ | |
| Fronteira Divergências | 64 linhas ✓ | SEFAZ=0 (falta importar extrato) |
| Fronteira Regras BA | 40 NCMs ✓ | |

---

## PENDÊNCIAS PARA AMANHÃ (em ordem de prioridade)

1. **Cadastrar regras NCM para rolamentos (NCM 8482)** — produto PRINCIPAL da ROLIMEC.
   Hoje caem no default 20,5% sem MVA (tratados como antecipação simples). Precisa
   da MVA e alíquota interna corretas (decisão de negócio — confirmar com o contador).
   Via aba "Regras NCM" ou seed.

2. **Importar extrato SEFAZ** para a aba Divergências comparar calculado × cobrado.

3. **ST nota-nível** — refinar: notas ST sem XML casado não têm NCM → sem MVA.
   Avaliar usar reg_c190.vl_icms_st quando disponível, ou item-nível sempre.

4. **Gaps maiores do Fronteira (do FRONTEIRA-GAP-ANALYSIS.md)** ainda abertos:
   - G6/G7: Inaplicabilidades (PRODEPE/PROIND, industrial-insumo)
   - G8: CT-e para frete autônomo (cte_entradas existe, sem JOIN)
   - G9: Comparação SPED × XML (notas sobrando/faltando)
   - G10: Classificação por IA das notas sem SPED
   - G11: Upload de tabela MVA em PDF
   - G12: Seleção de regra por CNAE da empresa

5. **companies não tem coluna `uf`** — o lookup de regra usa `dest_uf` do XML
   (fallback PE). Para notas SPED sem XML, a UF fica imprecisa. Considerar
   adicionar `companies.uf`.

---

## COMO RETOMAR O AMBIENTE LOCAL AMANHÃ

Stack Docker `fb_apu04` (já configurado):
- Frontend: **http://localhost:3003**
- Backend: http://localhost:8084
- DB: container `fb_apu04-db` / `fiscal_apu04_db`
- Login teste: `claudio_bezerra@hotmail.com` / senha `Teste@123` (resetada p/ teste)
- Empresa com dados: MASTER (`0fce5c57-e0f1-4554-9785-8fb8bcd040ff`)

Se os containers estiverem parados:
```bash
cd /home/claudiobezerra/projetos/FB_APU04
docker network create coolify 2>/dev/null
docker compose up -d api web db redis
```
(o `docker-compose.override.yml` local mapeia a porta 3003 do frontend)

Após mudar código backend:
```bash
docker compose build api && docker compose up -d api
```

Arquivos locais (gitignored, não commitar): `docker-compose.override.yml`, `backend/.env` (tem JWT_SECRET local).

---

## DOMÍNIO DE PRODUÇÃO
`https://simulador.fbtax.cloud` (NÃO é fcxlabs.com)

---
*Tudo que está marcado "DEPLOYADO" já foi para produção via GitHub Actions → Coolify.*
*Ambiente local rodando e pronto para continuar os testes.*
