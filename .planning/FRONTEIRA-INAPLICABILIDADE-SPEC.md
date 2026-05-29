# SPEC de Entendimento — Motor de Inaplicabilidade de ICMS Fronteira

> **Status:** rascunho para revisão (usuário + contador) — **nenhum código escrito ainda**
> **Data:** 2026-05-29
> **Fonte:** 3 planilhas do contador em `/tmp/Inaplicabilidade/` (PE, BA, CE)
> **Pré-requisito de leitura:** este documento define o entendimento e a arquitetura
> proposta. Só após aprovação seguimos para planejamento de implementação.

---

## 1. Objetivo e visão

O contador produziu 3 planilhas que mapeiam, a partir do **SPED Fiscal (EFD ICMS/IPI)**,
em quais casos a **antecipação / ST / DIFAL de fronteira NÃO se aplica** (ou se aplica
por regra específica). A visão do usuário:

1. **Importar** essas regras candidatas para o sistema.
2. **Trazer para a tela** numa aba de aprovação — o contador revisa, aprova ou rejeita cada regra.
3. **Carregar a regra aprovada no motor fiscal** — o motor passa a marcar cada item como
   `NÃO CALCULAR`, `CALCULAR_OUTRO` (regime específico) ou `CALCULAR` (padrão).

Isto **substitui** a tabela `icms_fronteira_inaplicabilidades` atual (criada vazia na
migration 097, só com NCM) — insuficiente para a riqueza das regras reais.

---

## 2. O que são os 3 arquivos (factual)

Cada arquivo tem o mesmo espírito, com diferenças por UF:

| UF | Arquivo | Abas | Nº regras |
|----|---------|------|-----------|
| **PE** | `Inaplicabilidade_Antecipacao_ICMS_PE_SPED.xlsx` | MAPEAMENTO_SPED · REGRAS_INAPLICABILIDADE · REF_REGISTROS_SPED · FLUXO_DECISAO | ~42 (AP01–06, ST01–04, IS01, NAT01–09, TR01, CN01–02, CR01–15, NC01, EN01, ES01–02) |
| **BA** | `Inaplicabilidade_Antecipacao_ST_Bahia_SPED.xlsx` | COMPARATIVO_BAHIA · ANT_PARCIAL_INAPLICAB · ST_INAPLICABILIDADE_BA · FLUXO_DECISAO_BA | ~14 (AP-BA01–08 + ST-BA01–06 + ST-BA-UF) |
| **CE** | `Inaplicabilidade_Antecipacao_ST_Ceara_SPED.xlsx` | COMPARATIVO_CEARA · ANT_PROPRIA_INAPLICAB · ST_INAPLICABILIDADE_CE · FLUXO_DECISAO_CE | ~16 (AP-CE01–06 + ST-CE01–06 + ST-CE-UF) |

**Diferença estrutural importante:**
- **PE** = uma única tabela de regras (antecipação ICMS fronteira), agrupada em 10 grupos.
- **BA/CE** = DOIS institutos distintos, em abas separadas:
  - **Antecipação parcial (BA) / própria (CE)** — só para mercadoria de **revenda**.
  - **ST por antecipação** — encerra a cadeia (Anexo 1 BA / arts. 431–476 CE).
  - Regra de ouro BA/CE: se a mercadoria é **insumo industrial**, **nenhum** dos dois se aplica
    (exceto açúcar/madeira no CE, art. 767 §5º).

Cada regra traz: `ID`, `grupo/hipótese`, `tipo de verificação`, `registro SPED`, `campo`,
`valores-gatilho`, `lógica (AND/OR/N/A)`, `resultado`, `instrução`, `base legal`, `vigência`.

---

## 3. Taxonomia das verificações × viabilidade de dados (o ponto crítico)

As regras se apoiam em **8 tipos de gatilho**. A viabilidade de automatizar cada um depende
de já termos (ou não) o dado no sistema:

| Tipo | Origem SPED | Temos hoje? | Viabilidade |
|------|-------------|-------------|-------------|
| **CST_ICMS** (10/30/60/70=ST; 40/41/50/51=isenção) | C170.cst_icms / C190.cst_icms | ✅ `reg_c170.cst_icms`, `reg_c190.cst_icms` | **Alta** |
| **CFOP** (devolução, remessa/retorno, ativo, etc.) | C170.cfop | ✅ `reg_c170.cfop` | **Alta** |
| **VL_ICMS_ST > 0** | C170/C190.vl_icms_st | ✅ `reg_c190.vl_icms_st` (C170 a confirmar) | **Alta** |
| **CEST** (preenchido = ST) | C170.cest ou 0200.cest | ⚠️ a confirmar (107/100/113 mexem em cest) | **Média** |
| **NCM** (listas AP01–06, têxteis, abate…) | 0200.cod_ncm | ✅ via NCM já usado na fronteira (XML/0200) | **Alta** |
| **CNAE do destinatário** (alimentação, industrial) | reg 0000.CNAE | ⚠️ no **cadastro de empresas** (`companies.cnae`), não do 0000 | **Média** (usar cadastro) |
| **Raiz CNPJ remetente** (transferência mesmo titular) | 0150.CNPJ[0:8] vs 0000.CNPJ[0:8] | ⚠️ `participants.cnpj` existe; comparar com CNPJ da empresa | **Média** |
| **CREDENCIAMENTO** (Prodepe, Proind, Mais Atacadistas…) | EXTERNO — não está no SPED | ❌ não existe | **Baixa** — exige cadastro manual de CNPJs credenciados |

**Casos que exigem cruzamento histórico (mais complexos):**
- "Industrial **fabrica** o mesmo NCM" (ST-BA01/ST-CE01) → cruzar NCM de entrada com **saídas** do contribuinte.
- "Exclusividade por produto na transferência" (ST-BA02b, desde 01/10/2024) → para cada NCM/CEST,
  verificar se **todas** as entradas do período foram por transferência.
- "Insumo industrial" (AP-BA02, ST-BA04, AP-CE01, ST-CE03) → NCM de entrada **não aparece** nas saídas.

---

## 4. Tipos de resultado

| Resultado | Significado | Efeito no motor |
|-----------|-------------|-----------------|
| `NAO_CALCULAR` | Inaplicável — não gerar antecipação/ST | Exclui o item do cálculo de fronteira |
| `CALCULAR_OUTRO` | Aplica regime específico (PE: celular, leite, gesso, cesta básica…) | Marca o item com o regime/decreto correto |
| `CALCULAR` | Padrão — nenhuma regra disparou | Calcula antecipação normal |
| `NAO_CALCULAR_PARCIAL` (BA/CE) | Antecipação parcial/própria não cabe (mas pode caber **DIFAL**) | Não gera antecipação; sinaliza DIFAL possível |
| `ST_INAPLICAVEL` (BA/CE) | ST não se aplica → reavaliar antecipação parcial/própria | Encaminha para a 2ª verificação |
| `MUDA_RESPONSAVEL` (ST-BA-UF / ST-CE-UF) | ST **não** dispensada; adquirente vira substituto | Mantém cobrança, troca o responsável |

---

## 5. Modelo de dados proposto (schema unificado)

Tabela nova `icms_fronteira_inaplic_regras` (substitui a `_inaplicabilidades` atual):

```
id              UUID PK
uf_estado       VARCHAR(2)     -- PE | BA | CE
id_regra        VARCHAR(16)    -- AP01, ST-BA02b, AP-CE01...
instituto       VARCHAR(20)    -- ANTECIPACAO | ANT_PARCIAL | ANT_PROPRIA | ST
grupo           TEXT           -- "2-SUBSTITUIÇÃO TRIBUTÁRIA"
hipotese        TEXT           -- descrição
tipo_verif      VARCHAR(20)    -- CST | CFOP | CEST | VL_ICMS_ST | NCM | CNAE | CNPJ_RAIZ | CREDENC | COMBINADA | NATUREZA
registro_sped   VARCHAR(10)
campo_sped      VARCHAR(40)
valores_gatilho TEXT           -- "10;30;60;70" ou "0201;0202..." (normalizado em array na app)
registro_sped_2 / campo_sped_2 / valores_2   -- para regras COMBINADAS
logica          VARCHAR(5)     -- AND | OR | N/A
resultado       VARCHAR(24)    -- ver seção 4
instrucao       TEXT
base_legal      TEXT
vigencia_inicio DATE
vigencia_fim    DATE
-- aprovação:
status_aprovacao VARCHAR(12)   -- pendente | aprovada | rejeitada
aprovado_por    TEXT
aprovado_em     TIMESTAMPTZ
auto_aplicavel  BOOLEAN        -- true se o gatilho é 100% SPED-derivável (seção 3)
created_at      TIMESTAMPTZ
```

Tabela auxiliar p/ credenciamentos (Grupo 7): `icms_fronteira_credenciamentos`
(`cnpj`, `uf_estado`, `sistematica`, `vigencia_inicio/fim`) — preenchida manualmente.

---

## 6. Fluxo de aprovação (a tela)

Nova sub-aba **Inaplicabilidades** em Administrativo (padrão das sub-abas atuais):

1. Botão **Importar** as 3 planilhas → parser cria regras com `status=pendente`.
2. Lista por UF, agrupada por instituto/grupo, com: hipótese, gatilho, resultado, base legal, vigência.
3. Badge de **viabilidade** (auto-aplicável ✅ / precisa dado externo ⚠️).
4. Ações: **Aprovar** / **Rejeitar** por regra (ou em lote por grupo).
5. Só regras `aprovada` + `auto_aplicavel` entram no motor.

---

## 7. Integração no motor (ordem de precedência)

O motor já classifica por CFOP/CST (handler `motor_fiscal.go`). A ordem de avaliação segue
o FLUXO_DECISAO de cada UF (precedência importa — a 1ª regra que dispara vence):

**PE:** CFOP-natureza → NCM-específico → ST (CST/VL_ICMS_ST/CEST) → isenção (CST) → CNAE → transferência → credenciamento → NCM+credenc → energia → casos especiais → senão CALCULAR.

**BA/CE:** primeiro decide ST vs antecipação parcial/própria (CST/CEST), aplica inaplicabilidades da ST, e só então reavalia a antecipação parcial/própria. **Insumo industrial zera os dois.**

⚠️ **Exceções "sempre calcular"** (hardcode de segurança):
- PE: **camarão** (NCM 0306/0307) e contribuinte **irregular/suspenso** → ignora credenciamentos.
- CE: **açúcar** (1701) e **madeira** (cap. 44) como insumo → antecipação própria se aplica.

---

## 8. Faseamento proposto

| Fase | Escopo | Risco |
|------|--------|-------|
| **1 — Cadastro + Aprovação** | Schema novo + importador dos 3 XLSX (PE difere de BA/CE) + sub-aba de aprovação. **Não toca no cálculo.** | Baixo |
| **2 — Motor: regras SPED-deriváveis** | Aplicar no cálculo as regras de **CST, CFOP, CEST, VL_ICMS_ST, NCM** (cobre ST, isenção, natureza da operação, NCM-específico — a maioria das regras de PE e as ST de BA/CE). | Médio (mexe na MV/`fronteiraBaseQuery`) |
| **3 — Dados adicionais** | CNAE (cadastro), raiz CNPJ (transferência), cruzamento entrada×saída (insumo industrial, exclusividade por produto). | Médio-alto |
| **4 — Credenciamentos** | Cadastro manual de CNPJs credenciados + Grupo 7 (Prodepe, Proind, Mais Atacadistas…) e exceções. | Alto (regra de negócio + dado externo) |

A Fase 1 entrega exatamente o que o usuário descreveu ("trazer para aprovação na tela");
as Fases 2–4 são o "carregar no motor", incrementais por viabilidade de dados.

---

## 9. Gaps de dados / riscos a confirmar

- [ ] `reg_c170` persiste **vl_icms_st** e **cest**? (107 cria; confirmar colunas finais)
- [ ] `0200` persiste **cod_ncm** e **cest** por item, com join `cod_item`?
- [ ] **CNAE** do contribuinte: usar `companies.cnae` (cadastro) — confiável e atualizado?
- [ ] **UF do remetente**: derivar de `participants.cod_mun` (2 primeiros díg. = UF) ou de `nfe_entradas`?
- [ ] **Credenciamentos**: não há fonte automática — o cliente vai informar manualmente?
- [ ] **Cruzamento entrada×saída** (insumo industrial): custo de query/MV — viável no volume atual?
- [ ] Regras `CALCULAR_OUTRO` de PE referenciam **decretos específicos** — o motor já tem esses regimes ou só sinaliza?

---

## 10. Questões abertas para o contador

1. As **listas internas** citadas (cesta básica Dec. 26.145/2003, sistemáticas Lei 14.721/2012,
   farmacêutica Dec. 28.247/2005, insumos Anexo 8) — o contador fornece as NCMs?
2. Os **credenciamentos do cliente** (quais sistemáticas ele tem ativas) — como/quem informa?
3. Para `CALCULAR_OUTRO` (regimes específicos PE) — o sistema deve **calcular** o regime alternativo
   ou apenas **sinalizar** "tratar manualmente"?
4. **Vigências**: aplicar regra só dentro de `vigencia_inicio/fim` confrontando a data de emissão da NF?

---

## 11. Próximos passos sugeridos

1. Usuário + contador revisam esta spec (especialmente seções 3, 8, 9, 10).
2. Definir o corte da **Fase 1** (cadastro + aprovação) como primeiro entregável.
3. Escalar para `/gsd:plan-phase` (fase formal) ou `/gsd-quick --full` para a Fase 1.
