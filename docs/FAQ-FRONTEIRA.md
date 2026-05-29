# 📘 FB Tax — Book de Principais Dúvidas (FAQ)

> Base de conhecimento do módulo **ICMS Fronteira** e funcionalidades relacionadas.
> Alimenta o **Assistente FB Tax** (modo Tutorial) e serve como referência de uso.
> Fonte espelhada em `backend/handlers/ai_ajuda_faq.go`.

---

## 1. Conceitos

**O que é a antecipação do ICMS Fronteira?**
Recolhimento antecipado do ICMS devido na entrada interestadual de mercadorias, antes da saída subsequente. Calculada sobre a base (produto + frete + outros), pela diferença entre a alíquota interna da UF e a interestadual.

**Qual a diferença entre Antecipação, ST e DIFAL?**
- **Antecipação** — compra interestadual normal para revenda (CFOP 2101/2102/2152).
- **ST (Substituição Tributária)** — o ICMS de toda a cadeia já foi retido pelo remetente (CFOP 2403/2409/2651/2652 + segmento cadastrado).
- **DIFAL** — uso/consumo ou ativo fixo (CFOP 2551/2556).

**O que são os 3 blocos (Mês Anterior / Mês Atual / Não no SPED)?**
A antecipação é devida pela **data de emissão**, mas o SPED registra por recebimento.
- **Bloco A** — notas de meses anteriores que entraram no SPED deste mês.
- **Bloco B** — notas do mês atual no SPED.
- **Bloco C** — notas que estão no XML mas ainda **não** foram lançadas no SPED.

---

## 2. UF de trabalho

**Para que serve a "UF de trabalho" no topo?**
Todo o módulo opera sobre as filiais da UF selecionada (PE/BA/CE). Troque a UF para ver as demais. Regras, segmentos e inaplicabilidades são **por UF**.

---

## 3. Flag de inaplicabilidade (simulador)

**O que faz o flag "COM / SEM inaplicabilidade"?**
É um simulador. **SEM** = cálculo padrão de hoje. **COM** = aplica as regras de inaplicabilidade **aprovadas e auto-aplicáveis** que significam "não calcular" (ex.: nota com ST já destacada `VL_ICMS_ST>0`; CST de ST 10/30/60/70; isenção 40/41/50/51), zerando o ICMS devido dessas notas. Compare os totais COM vs SEM. Quando **SEM**, não altera nada em produção.

**Liguei o flag e o total não mudou. Por quê?**
Provavelmente não há regras **aprovadas** para a UF, ou nenhuma nota casa os gatilhos seguros. Vá em **Administrativo → Inaplicabilidade**, importe e **aprove** as regras de ST/isenção da UF e teste de novo.

---

## 4. Inaplicabilidade — cadastro e aprovação

**Como cadastrar inaplicabilidade?**
Administrativo → Inaplicabilidade → **Importar planilhas** (PE/BA/CE). As regras entram como **pendentes**. Aprove ou rejeite cada uma. Só regras aprovadas (e auto-aplicáveis) entram no motor.

**O que significa o badge "auto" vs "manual"?**
- **auto** — gatilho 100% derivável do SPED (CST, CFOP, CEST, VL_ICMS_ST, NCM) → pode ser aplicado pelo motor.
- **manual** — depende de dado externo (credenciamento) ou cadastro adicional → ainda não entra automaticamente.

**Reimportar apaga as aprovações?**
Não. O reimport atualiza o conteúdo das regras mas **preserva** o status de aprovação já dado.

---

## 5. Comparativo de Planilhas

**Como uso o Comparativo de Planilhas?**
Reconciliação → **Comparativo de Planilhas** → suba 2 arquivos XLSX (Correta e Conferência) → **Comparar**. Mostra, por Bloco A/B/C, notas só em uma planilha e divergências de ICMS, com a **Causa provável**. Exporte as diferenças em Excel.

**O que é a coluna "Causa provável"?**
O sistema infere a causa pelos próprios números:
- **IPI na base** — a diferença bate com `V.IPI × alíquota interna`.
- **Base de cálculo difere** — V.Prod diferente entre as planilhas.
- **Alíquota interestadual difere** — mix 4%/12% ou mínimo SN.
- **Nota ausente** em uma das planilhas.

**Por que muitas divergências aparecem como "IPI na base"?**
Indica que uma planilha somou o IPI na base de cálculo e a outra não. Após o ajuste que remove o IPI da base (e o refresh dos dados), essas divergências tendem a desaparecer.

**Por que diferenças de centavo não aparecem?**
Há tolerância de **R$ 0,05** — arredondamentos até 5 centavos não são marcados, para destacar o que é real.

---

## 6. Cálculo

**O que é o crédito interestadual e o "mix de alíquotas"?**
O ICMS destacado na origem (4%, 7% ou 12%) é creditado. Quando uma mesma nota tem itens com alíquotas diferentes (ex.: 4% importado + 12% nacional no mesmo CFOP), o sistema **pondera o crédito por item** — não usa a maior alíquota — preservando o cálculo correto.

**Existe mínimo de 4% para fornecedor do Simples Nacional?**
Sim. Para fornecedores do Simples Nacional, o crédito interestadual considerado tem **piso de 4%** sobre o valor do produto.

**O IPI entra na base de cálculo?**
**Não.** A base da antecipação/DIFAL é produto + frete + outros (o IPI foi removido da base).

---

## 7. PRODEPE / Benefícios

**Como o PRODEPE / Central de Distribuição afeta o cálculo?**
A filial beneficiada (CNPJ + ato + vigência cadastrados em **Administrativo → PRODEPE**) é dispensada da antecipação/ST nas aquisições → o ICMS fronteira fica **zero** para as notas dentro da vigência. Aparece na aba **Incentivo**.

---

## 8. Exportação e Assistente

**Como exporto os dados?**
A maioria das abas tem botão de exportar (Excel/CSV/PDF). No assistente, o modo **Consulta de dados** também tem botão **Exportar Excel** do resultado.

**O que o Assistente FB Tax faz?**
- **Tutorial** — tira dúvidas de uso (ajuda online; sabe em que página você está).
- **Consulta de dados** — transforma sua pergunta em consulta ao sistema (**somente leitura**) e mostra a tabela, com exportação para Excel.

---

*Para ampliar este book, edite `backend/handlers/ai_ajuda_faq.go` (fonte do assistente) e replique aqui.*
