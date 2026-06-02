# ERP Bridge Simulador

Conector **isolado** que importa **NF-e de entrada + CT-e** do banco do cliente (Oracle
FCCORP, tabelas legado Totvs `sfc_nfe_imp` / `sfc_cte_imp`) para o **FB_APU04 (simulador)**,
alimentando o módulo ICMS Fronteira.

## Por que separado do `erp-bridge-aws/`

O `erp-bridge-aws/bridge.py` é **compartilhado** com o FB_APU02 (reforma tributária) e lê
as tabelas SAP (`s4i_*`) do mesmo FCCORP. Para **não arriscar a produção do APU02**, este
conector é autônomo: não importa nada daquele código, abre sua própria conexão Oracle
(SELECT read-only nas `sfc_*_imp`) e envia ao backend do simulador.

## Como funciona

```
FCCORP sfc_nfe_imp / sfc_cte_imp (CLOB XML)
  → bridge_simulador.py (lê CLOB em streaming, janela de data, lotes)
  → POST /api/erp-bridge/import/xml  (X-API-Key)
  → backend Go: pipeline assíncrono (xml_upload_batches → xml_worker)
  → parser provado (processSingleXML/processSingleCTe)
  → nfe_entradas(+itens) / cte_entradas(+refs+toma)
```

O conector **não parseia XML** — só transporta o XML cru. Todo o "split em colunas" é
feito pelo parser Go já usado na importação direta (uma fonte de verdade).

## Uso

```bash
cp config.example.yaml config.yaml   # preencha oracle.senha e fbtax.api_key

# janela de datas (inclusive nas duas pontas); importa entradas + CT-e
python bridge_simulador.py --data-ini 2026-04-01 --data-fim 2026-04-30

# só um tipo, ou conferência sem enviar
python bridge_simulador.py --data-ini 2026-04-01 --data-fim 2026-04-30 --tipos ctes
python bridge_simulador.py --data-ini 2026-04-01 --data-fim 2026-04-30 --dry-run

# re-importar uma janela já enviada (zera o dedup local)
python bridge_simulador.py --data-ini 2026-04-01 --data-fim 2026-04-30 --reset-tracker
```

### Modo fila (`--drain`) — disparo pela UI

A UI ("Importação de XMLs → Importar via ERP", admin) enfileira jobs por período.
O conector em modo `--drain` consome todos os jobs pendentes e sai (sem daemon
sempre-ligado). Rode manualmente ou via cron:

```bash
python bridge_simulador.py --config config.yaml --drain
# no servidor (container do bridge):
docker exec fbtax-bridge-apu04 python /app/bridge_simulador.py --config /app/config_simulador.yaml --drain
```

Fluxo: UI → `POST /api/erp-bridge/xml-import/trigger` (cria job pending) →
`--drain` busca em `/xml-import/pending`, executa a janela, reporta em
`/xml-import/status`. O histórico aparece em "Logs de Importação".

### Modo daemon (`--daemon`) — disparo imediato (recomendado)

Em vez de cron/drain periódico, o `--daemon` faz **long-poll**: chama
`/xml-import/pending?wait=25` e o backend segura a resposta até surgir um job —
disparo manual processado em ~1-2s, sem ficar varrendo. Conecta no Oracle só
quando há job (robusto a queda de conexão ociosa). Container recomendado:

```bash
docker run -d --name erp-xml-drain --restart unless-stopped \
  -v /home/<user>/erp-sim/config.yaml:/app/config.yaml \
  erp-bridge-simulador --config /app/config.yaml --daemon
```

A **coleta automática D-1** é agendada no backend (config por empresa em
`erp_bridge_config.xml_auto_enabled/xml_auto_hora`, ajustável na tela "Importar via
ERP"): no horário definido o backend enfileira o job de ontem e o daemon processa.

⚠️ **Volume**: a fonte tem ~450–620 mil NF-e/mês (2024–2025). Importe por janelas
controladas (ex.: por dia ou semana nos meses pesados). O backend processa de forma
assíncrona e idempotente (`ON CONFLICT` por chave); o conector mantém um dedup local
(`tracker_simulador.db`) para não reenviar o que já subiu.

## Docker

```bash
docker build -t erp-bridge-simulador .
docker run --rm -v "$PWD/data:/app/data" -v "$PWD/config.yaml:/app/config.yaml" \
  erp-bridge-simulador --data-ini 2026-04-01 --data-fim 2026-04-30
```

(monte `config.yaml` e um volume `data/` se quiser persistir o tracker entre execuções —
nesse caso ajuste o caminho do tracker, hoje fixo ao lado do script.)
```
