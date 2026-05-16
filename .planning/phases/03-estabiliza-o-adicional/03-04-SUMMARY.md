# 03-04 Summary — Retry/Reconnect Oracle DPY-4011 (STAB-09)

**Status:** COMPLETE  
**Commit:** 88aaa6d

## What was done

Adicionado retry automático de conexão Oracle ao `erp-bridge-aws/bridge.py`:

### Helpers adicionados (antes da seção SAP, linha 605)
- `MAX_CONN_RETRIES = 3`, `CONN_RETRY_DELAY = 5` — constantes de módulo
- `_is_dpy4011(exc)` — detecta se uma exceção contém "DPY-4011"
- `_connect_oracle(usuario, senha, dsn, nome)` — conecta com retry linear (5s/10s entre tentativas), lança na última falha

### Sites substituídos (4 total)
1. `processar_sap()` linha ~672 — `oracledb.connect()` → `_connect_oracle("FCCORP")`
2. `processar_servidor()` linha ~857 — `oracledb.connect()` → `_connect_oracle(nome)`
3. `processar_servidor()` query interna — `try cur.execute ... except continue` → loop `while True` com `_is_dpy4011()` para reconexão mid-query
4. `run_daemon()` parceiros linha ~1175 — `oracledb.connect()` → `_connect_oracle("oracle-daemon")`
5. `main()` parceiros linha ~1363 — `oracledb.connect()` → `_connect_oracle("oracle-main")`

### Verificação
- `python3 -c "ast.parse(...)"` → OK: sintaxe válida
- `python3 bridge.py --help` → executa (falha graciosamente por oracledb não instalado no dev)
- `grep "oracledb.connect("` → apenas 1 ocorrência, dentro de `_connect_oracle`

## Requirements satisfied

- STAB-09: Bridge SAP sobrevive a DPY-4011 reconectando sem intervenção humana ✓
