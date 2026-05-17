# Runbook: XMLUploadFalha

**Severidade:** warning
**Dispara quando:** `increase(xml_upload_errors_total[5m]) > 0`
**Detectado em:** até 30 segundos após o primeiro erro (scrape 15s + eval 15s + for 0s)
**Alerta Prometheus:** XMLUploadFalha
**Métrica:** `xml_upload_errors_total` (Counter, api:8084/metrics)

---

## Sintomas

- Email de alerta `[FB_APU04][WARNING] XMLUploadFalha` chegou na caixa
- Usuário relata que o upload de XML/ZIP foi rejeitado ou falhou
- Interface exibe mensagem de erro ao fazer upload no painel
- Tabela `xml_upload_batches` contém registros com `status != 'success'`
- Logs do backend Go mostram erro no handler XMLUploadHandler

## Causa Mais Provável

1. **Arquivo ZIP corrompido** — ZIP enviado está truncado, com checksum inválido ou não é um ZIP
2. **NF-e fora do schema v4.00** — XMLs usam versão anterior do leiaute SEFAZ (v3.10 ou inferior)
3. **Arquivo muito grande** — ZIP com mais de 100 MB ou com mais de 5.000 XMLs (limites do sistema)
4. **NCM não cadastrado** — código NCM dos itens não existe na tabela `ncm_cclasstrib`
5. **CT-e com estrutura inválida** — XML de CT-e com tags obrigatórias ausentes
6. **Encoding incorreto** — arquivo XML não está em UTF-8 (SEFAZ exige UTF-8)
7. **NF-e de saída em vez de entrada** — sistema está configurado para entradas; NF-e saídas são ignoradas com aviso

## Passos de Mitigação

1. **Identificar o batch com falha (últimos 60 minutos):**
   ```sql
   SELECT id, file_name, status, error_msg, total_xmls, created_at
   FROM xml_upload_batches
   WHERE created_at > NOW() - INTERVAL '1 hour'
     AND status != 'success'
   ORDER BY created_at DESC LIMIT 20;
   ```

2. **Ler o `error_msg` do batch mais recente:**
   ```sql
   SELECT error_msg, status, total_xmls, xmls_ok, xmls_error
   FROM xml_upload_batches
   WHERE id = '<id_do_batch>';
   ```

3. **Se erro for de schema/estrutura XML:**
   - Baixar o ZIP do cliente
   - Validar o XML manualmente com ferramenta SEFAZ ou online
   - Verificar se a tag `<nfeProc>` ou `<NFe>` está presente com `versao="4.00"`
   - Verificar se o XML tem assinatura digital válida (`<Signature>` no final)

4. **Se arquivo muito grande:**
   - Orientar o usuário a dividir o ZIP em partes de no máximo 5.000 XMLs ou 80 MB
   - Usar comando zip: `split -b 80m arquivo.zip parte_`

5. **Se NCM não encontrado:**
   ```sql
   SELECT codigo_ncm
   FROM ncm_cclasstrib
   WHERE codigo_ncm = '<ncm_do_item>';
   ```
   Se ausente, cadastrar na tabela `ncm_cclasstrib` conforme tabela TIPI/SEFAZ.

6. **Verificar logs do backend para contexto adicional:**
   ```
   docker compose logs --tail 100 api | grep -i "xml\|upload\|batch\|error"
   ```

7. **Tentar novo upload após correção:**
   - Corrigir o arquivo apontado no `error_msg`
   - Fazer upload novamente pelo painel
   - Confirmar que o novo batch tem `status=success`

## Verificação Pós-Mitigação

1. Novo upload bem-sucedido cria registro em `xml_upload_batches` com `status='success'`:
   ```sql
   SELECT status, total_xmls, xmls_ok, created_at
   FROM xml_upload_batches
   ORDER BY created_at DESC LIMIT 3;
   ```

2. Counter `xml_upload_errors_total` não deve crescer após a correção:
   ```
   curl -s 'http://localhost:9090/api/v1/query?query=increase(xml_upload_errors_total[5m])'
   ```

3. Os XMLs importados devem aparecer nas views `vw_xml_nfe_entradas` ou `vw_xml_nfe_saidas`.

4. O alerta Prometheus resolve automaticamente quando nenhum novo erro ocorre por 5 minutos.

## Escalar Para

- **Usuário que fez o upload:** Para corrigir o arquivo conforme instruções do `error_msg`
- **claudio.bezerra@ferreiracosta.com.br:** Para questões de NCM não cadastrado ou problema de schema recorrente
- **Suporte SEFAZ/Receita Federal:** Para validar XMLs com schema NF-e específico

## Histórico de Incidentes

| Data | Descrição | Resolução |
|------|-----------|-----------|
| — | Sem incidentes registrados nesta categoria | — |

---
*Runbook interno FB_APU04 — equipe fiscal Ferreira Costa*
