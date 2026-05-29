# Relatório de varredura noturna — 27/05/2026

**Para ler ao acordar.** Localhost está pronto para a carga geral. Sem pendências críticas. Detalhes abaixo.

---

## TL;DR

- **5 frentes auditadas** (motor, validação ao vivo, importações, limpeza, build/coverage). Tudo passou.
- **Localhost** rodando código de `4456d04` (último commit pushed para `main`). Base PE intacta, `company_segmentos` vazia (pronto para você marcar amanhã).
- **Único item de atenção:** o bundle servido no PRD pode ainda ser o antigo se o deploy não terminou — me confirme ao acordar e eu ajudo.

---

## 1) Motor de cálculo (Segmento × UF × NCM)

Auditei lado a lado os dois caminhos de cálculo:

- **Bloco A/B (`icms_fronteira.go`):** NFs do SPED. ST = regra tem `segmento_codigo` E `company_segmentos.uf = j.uf` (UF da filial no SPED).
- **Bloco C (`icms_fronteira_nao_sped.go`):** NFs só com XML. ST = regra tem `segmento_codigo` E `company_segmentos.uf = m.dest_uf` (UF do destinatário no XML).

**Veredito:** a lógica de ST × Segmento × UF está **idêntica** nos dois caminhos. CFOPs corretos:
- DIFAL: 2551/2556
- ST condicional: 2403/2409/2651/2652 (vira antecipação se segmento não casa)
- Antecipação pura: 2101/2102/2152

**Fallback de NCM do SPED ativo** (`reg_0200` via `reg_c170.cod_item`): notas sem XML vinculado conseguem casar regra por NCM. Sem o fallback, eram só 4/54 com regra; com o fallback, 54/54 têm NCM disponível.

**Diferença menor (pré-existente, não bloqueia):** o bloco C só lê `mva_original`/`mva_ajustado_12pct`, enquanto A/B seleciona 4/7/12% pela alíquota interestadual. Funciona, mas pode subestimar MVA no bloco C se houver MVAs 4% ou 7% cadastrados. Posso uniformizar depois da sua carga.

## 2) Validação ponta-a-ponta ao vivo (PE 04/2026)

Login: `claudio_bezerra@hotmail.com` / admin / empresa MASTER (`0fce5c57-...`).

**Snapshot ANTES** (`company_segmentos` vazia):
```
ANTECIPACAO: 245 notas, ICMS devido R$ 880.993,85
DIFAL:         1 nota,  ICMS devido R$   1.920,95
ST: 0 (nada com segmento marcado)
```

Marquei segmento 8 (Autopeças) via API:
```
PUT /api/icms-fronteira/company-segmentos {"uf":"PE","codigos":[8]}
→ saved: 1
```

**Snapshot DEPOIS**:
```
ANTECIPACAO: 245 notas, ICMS devido R$ 879.051,50  (-R$ 1.942,35)
DIFAL:         1 nota,  ICMS devido R$   1.920,95
ST:            2 notas, ICMS devido R$   3.693,38  ← apareceu!
```

✅ O motor reconciliou na hora. As 2 notas que migraram para ST são as de NCM 8482 (rolamentos), única regra cadastrada que casa com a base atual.

**Limpei depois** (`company_segmentos = []`) para você começar amanhã do zero.

## 3) Importações (segmentos + regras NCM)

Testei 5 casos críticos via API:

| Cenário | Resultado |
|---|---|
| CSV com BOM + `;` + aspas (com vírgula dentro) | 3 importados ✅ |
| Upsert de código existente (atualiza descrição) | OK ✅ |
| Linhas inválidas (vazia, código não numérico, descrição vazia) | listadas em `errors`, count `skipped` correto ✅ |
| Regra PE com `segmento_codigo` que existe em **outra UF** | rejeita com 400 "não existe para a UF PE" ✅ |
| Regra **sem** `segmento_codigo` | rejeita com 400 "segmento_codigo é obrigatório" ✅ |

**Invariante Regras × Segmento × UF está blindada.**

## 4) Limpeza de base (grupo "fronteira")

Testei o DELETE no grupo `fronteira` com snapshot completo + restore:

**Antes:** 33 segmentos_uf, 155 regras_empresa, 1 company_segmento
**Depois do DELETE:** 0, 0, 0 — **clean slate confirmado** (apaga catálogo global, regras globais e por empresa, segmentos da empresa).

Restaurei tudo do `pg_dump` para deixar como você reimportou.

## 5) Build, testes, tsc, bundle

| Item | Status |
|---|---|
| `go build ./...` | OK |
| `go test ./handlers/...` | OK, 14.5s |
| Cobertura | **23.7%** (limite CI: 23%) ✅ |
| `npx tsc --noEmit` (exit code real) | exit 0 ✅ |
| `npm run build` (Vite) | OK, bundle `index-DuONhgcB.js` |
| Bundle servido no localhost contém: import de segmentos, V. Frete CT-e (bloco C), observação "vinculados às" | ✅ |

**Containers:** todos running, sem erros recentes (10 min de logs limpos, zero HTTP 5xx).

## 6) Migrations no banco

`schema_migrations` em dia: 120 (`company_segmentos`), 121 (`segmento_codigo` em regras), 122 (`remove_seed_fronteira_global`) todas aplicadas em 2026-05-26.

---

## Estado para amanhã

**Localhost (3003/8084) — pronto para carga:**

| Tabela | Estado |
|---|---|
| `segmentos_uf` | 33 (catálogo carregado: 21 PE + 12 BA) |
| `icms_fronteira_regras_ncm` (empresa) | 155 (PE carregadas) |
| `icms_fronteira_regras_ncm` (global) | 0 |
| `company_segmentos` | 0 ← **você marca amanhã** |
| Notas SPED + XML + CT-e PE 04/2026 | intactas |

**Fluxo recomendado pela manhã:**
1. Ir em **Fronteira → Administrativo → Segmentos ST**, selecionar UF **PE**.
2. Importar/marcar os segmentos que o contador mandou (CSV ou clique a clique).
3. Clicar **"Salvar seleção"** (popula `company_segmentos` — chave para ST aparecer).
4. Voltar ao relatório e clicar **Recalcular** (ou refresh — é tudo ao vivo, não precisa reimportar SPED/XML).
5. Repetir para **BA** se aplicável.

**Carga de PRD:** quando rodar a mesma carga em PRD, a migration 122 já vai ter rodado (zerou os seeds antigos). Você só precisa importar segmentos + regras pela tela e marcar `company_segmentos`.

---

## Commits recentes (todos pushed, irão para PRD no deploy)

```
4456d04 fix(segmentos): importa AlertTriangle — tela Segmentos ST abortava em runtime
80d7ae0 fix(fronteira): NCM do SPED como fallback no cálculo + ajustes na tela de segmentos
adedce8 fix(segmentos): importação de CSV via upload server-side (parsing robusto)
919d873 feat(fronteira): restaura frete CT-e no Bloco C + limpeza clean-slate
527fa9c feat(fronteira): segmento obrigatório nas regras NCM — Regras × Segmento × UF
```

## Pendências menores (não bloqueiam a carga)

1. **Bloco C MVA 4/7/12%:** o bloco C ainda só lê `mva_original` ou `mva_ajustado_12pct`. Se houver regras com MVA 4% ou 7% específico, o bloco C pode subestimar. Bloqueio: nenhum agora porque você não tem regras MVA 4/7% para esses NCMs.
2. **Cobertura no limite:** 23.7% vs threshold 23%. Margem fina. Se mexer em código novo amanhã, vale rodar `go test -coverprofile=coverage.out` antes de pushar.
3. **Job órfão antigo de teste** (`DADOS_TESTE.txt`, 05/05) sem `company_id`. Não atrapalha (motor filtra por empresa). Posso limpar amanhã se quiser.

Bom descanso. 🌙
