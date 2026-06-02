#!/usr/bin/env python3
"""
ERP Bridge Simulador — conector ISOLADO de XML de entrada + CT-e para o FB_APU04.

Por que separado do erp-bridge-aws/bridge.py:
  Aquele bridge é COMPARTILHADO (FB_APU02 / reforma tributária usa o mesmo código e a
  mesma conexão FCCORP). Para não arriscar a integração de produção do APU02, este
  conector é totalmente autônomo: lê apenas as tabelas legado Totvs `sfc_nfe_imp` e
  `sfc_cte_imp` do FCCORP (SELECT read-only), extrai o XML cru do CLOB e o envia ao
  backend do simulador via POST /api/erp-bridge/import/xml (auth X-API-Key).

  O "split" do XML em colunas é feito no backend Go (parser provado da importação
  direta) — este conector NÃO parseia XML; só transporta.

Uso (manual, parametrizado por data):
  python bridge_simulador.py --data-ini 2026-01-01 --data-fim 2026-04-30
  python bridge_simulador.py --data-ini 2026-04-01 --data-fim 2026-04-30 --tipos entradas
  python bridge_simulador.py --data-ini 2026-04-01 --data-fim 2026-04-30 --dry-run

Config (config.yaml no mesmo diretório, ou --config):
  oracle:
    dsn: "10.131.1.118:1521/FCCORP"
    usuario: "fcosta"
    senha: "***"
  fbtax:
    url: "https://simu.fcxlabs.com"
    api_key: "***"          # api_key do APU04 (resolve company_id no backend)
"""
from __future__ import annotations

import argparse
import logging
import os
import re
import sqlite3
import sys
import time
from datetime import date, datetime, timedelta, timezone
from pathlib import Path

import oracledb
import requests
import yaml

BASE_DIR = Path(__file__).resolve().parent

# ─── Logging ──────────────────────────────────────────────────────────────────
log = logging.getLogger("bridge-simulador")
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
    handlers=[logging.StreamHandler(sys.stdout)],
)

# ─── Fontes (tabelas legado Totvs no FCCORP) ───────────────────────────────────
# chave_col / xml_col = índices das colunas no SELECT.
# endpoint_tipo = valor do campo "tipo" esperado pelo backend.
FONTES = {
    "entradas": {
        "sql": """
            SELECT chave_nfe, email_xml_nfe
            FROM   sfc_nfe_imp
            WHERE  TRUNC(data_importacao) >= :data_ini
              AND  TRUNC(data_importacao) <  :data_fim
            ORDER BY chave_nfe
        """,
        "adicionar_decl": True,
        "descricao": "NF-e Entradas (sfc_nfe_imp.email_xml_nfe)",
    },
    "ctes": {
        "sql": """
            SELECT CHAVE_CTE, XML_CTE
            FROM   SFC_CTE_IMP
            WHERE  TRUNC(DATA_IMPORTACAO) >= :data_ini
              AND  TRUNC(DATA_IMPORTACAO) <  :data_fim
            ORDER BY CHAVE_CTE
        """,
        "adicionar_decl": True,
        "descricao": "CT-e Entradas (SFC_CTE_IMP.XML_CTE)",
    },
}

MAX_CONN_RETRIES = 3
CONN_RETRY_DELAY = 5

_DECL_RE = re.compile(r"<\?xml[^?]*\?>", re.IGNORECASE)
_ENCODING_RE = re.compile(r'encoding\s*=\s*["\'][^"\']*["\']', re.IGNORECASE)


def normalizar_xml(texto: str, adicionar_decl: bool = False) -> str:
    """Força encoding UTF-8 na declaração (ou injeta uma) — igual ao bridge legado."""
    texto = (texto or "").strip()
    match = _DECL_RE.match(texto)
    if match:
        nova_decl = _ENCODING_RE.sub('encoding="UTF-8"', match.group())
        texto = nova_decl + texto[match.end():]
    elif adicionar_decl:
        texto = '<?xml version="1.0" encoding="UTF-8"?>' + texto
    return texto


def clob_para_str(valor) -> str:
    if valor is None:
        return ""
    if hasattr(valor, "read"):
        return valor.read()
    return str(valor)


def _is_dpy4011(exc: Exception) -> bool:
    return "DPY-4011" in str(exc)


# ─── Tracker (dedup / watermark local) ─────────────────────────────────────────
def init_tracker(path: Path) -> sqlite3.Connection:
    conn = sqlite3.connect(str(path))
    conn.execute(
        """CREATE TABLE IF NOT EXISTS enviados (
               tipo   TEXT NOT NULL,
               chave  TEXT NOT NULL,
               status TEXT,
               ts     TEXT,
               PRIMARY KEY (tipo, chave)
           )"""
    )
    conn.commit()
    return conn


def ja_enviado(tracker: sqlite3.Connection, tipo: str, chave: str) -> bool:
    cur = tracker.execute(
        "SELECT 1 FROM enviados WHERE tipo=? AND chave=? AND status='ok'", (tipo, chave)
    )
    return cur.fetchone() is not None


def marcar(tracker: sqlite3.Connection, tipo: str, chave: str, status: str) -> None:
    tracker.execute(
        "INSERT OR REPLACE INTO enviados (tipo, chave, status, ts) VALUES (?,?,?,?)",
        (tipo, chave, status, datetime.now(timezone.utc).isoformat()),
    )


# ─── Oracle ─────────────────────────────────────────────────────────────────--
def conectar_oracle(o: dict) -> oracledb.Connection:
    # fetch_lobs=False → CLOBs vêm direto como str (sem round-trip por linha),
    # essencial para volume alto.
    oracledb.defaults.fetch_lobs = False
    last = None
    for tent in range(1, MAX_CONN_RETRIES + 1):
        try:
            conn = oracledb.connect(
                user=o["usuario"], password=o["senha"], dsn=o["dsn"], expire_time=2
            )
            log.info("Conectado ao FCCORP %s (thin mode)", o["dsn"])
            return conn
        except Exception as exc:  # noqa: BLE001
            last = exc
            log.warning("Falha conexão (tentativa %d/%d): %s", tent, MAX_CONN_RETRIES, exc)
            time.sleep(CONN_RETRY_DELAY * tent)
    raise last  # type: ignore[misc]


# ─── Cliente FBTax ─────────────────────────────────────────────────────────────
class FBTax:
    def __init__(self, cfg: dict):
        self.base_url = cfg["url"].rstrip("/")
        self.api_key = cfg["api_key"]

    def enviar_xml_batch(self, tipo: str, xmls: list, competencia: str = "", job_id: str = "") -> dict:
        resp = requests.post(
            f"{self.base_url}/api/erp-bridge/import/xml",
            headers={"X-API-Key": self.api_key, "Content-Type": "application/json"},
            json={"tipo": tipo, "competencia": competencia, "job_id": job_id, "xmls": xmls},
            timeout=180,
        )
        if not resp.ok:
            raise RuntimeError(f"HTTP {resp.status_code}: {resp.text[:300]}")
        return resp.json()

    # ── Fila de jobs (modo --drain) ────────────────────────────────────────────
    def get_pending_job(self) -> dict | None:
        """Reivindica o próximo job pendente da empresa (ou None)."""
        resp = requests.get(
            f"{self.base_url}/api/erp-bridge/xml-import/pending",
            headers={"X-API-Key": self.api_key}, timeout=60,
        )
        if not resp.ok:
            raise RuntimeError(f"pending HTTP {resp.status_code}: {resp.text[:200]}")
        data = resp.json()
        return data if data and data.get("id") else None

    def report_job(self, job_id: str, status: str, enviados: int, erros: int,
                   error_message: str = "") -> None:
        try:
            requests.post(
                f"{self.base_url}/api/erp-bridge/xml-import/status",
                headers={"X-API-Key": self.api_key, "Content-Type": "application/json"},
                json={"job_id": job_id, "status": status, "total_enviados": enviados,
                      "total_erros": erros, "error_message": error_message},
                timeout=60,
            )
        except Exception as exc:  # noqa: BLE001
            log.warning("Falha ao reportar status do job %s: %s", job_id, exc)


# ─── Processamento ──────────────────────────────────────────────────────────--
def processar(
    fonte_key: str,
    conn_ora: oracledb.Connection,
    o: dict,
    fbtax: FBTax,
    tracker: sqlite3.Connection,
    data_ini: date,
    data_fim: date,
    batch_size: int,
    dry_run: bool,
    job_id: str = "",
) -> dict:
    fonte = FONTES[fonte_key]
    stats = {"lidos": 0, "enviados": 0, "ignorados": 0, "erros": 0}
    log.info("-" * 60)
    log.info("Consultando %s | %s -> %s", fonte["descricao"], data_ini, data_fim)

    cur = conn_ora.cursor()
    cur.arraysize = 500
    try:
        cur.execute(fonte["sql"], data_ini=data_ini, data_fim=data_fim)
    except Exception as exc:  # noqa: BLE001
        if _is_dpy4011(exc):
            log.warning("DPY-4011 na query %s — reconectando e repetindo...", fonte_key)
            conn_ora = conectar_oracle(o)
            cur = conn_ora.cursor()
            cur.arraysize = 500
            cur.execute(fonte["sql"], data_ini=data_ini, data_fim=data_fim)
        else:
            raise

    lote: list[dict] = []
    prog_ts = time.monotonic()

    def flush() -> None:
        nonlocal lote
        if not lote:
            return
        chaves = [d["name"] for d in lote]
        if dry_run:
            stats["enviados"] += len(lote)
            log.info("  [dry-run] enviaria %d XMLs", len(lote))
            lote = []
            return
        try:
            fbtax.enviar_xml_batch(fonte_key, lote, job_id=job_id)
            for ch in chaves:
                marcar(tracker, fonte_key, ch, "ok")
            tracker.commit()
            stats["enviados"] += len(lote)
        except Exception as exc:  # noqa: BLE001
            log.error("  Erro ao enviar lote (%d XMLs): %s", len(lote), exc)
            stats["erros"] += len(lote)
        lote = []

    for raw in cur:
        stats["lidos"] += 1
        chave = str(raw[0]).strip()
        xml_str = clob_para_str(raw[1])
        if not xml_str:
            stats["ignorados"] += 1
            continue
        if ja_enviado(tracker, fonte_key, chave):
            stats["ignorados"] += 1
            continue
        # nome com .xml: o worker do backend extrai do ZIP só entradas .xml.
        lote.append({"name": f"{chave}.xml", "content": normalizar_xml(xml_str, fonte["adicionar_decl"])})
        if len(lote) >= batch_size:
            flush()
        if time.monotonic() - prog_ts >= 60:
            log.info(
                "  [progresso] lidos=%d enviados=%d ignorados=%d erros=%d",
                stats["lidos"], stats["enviados"], stats["ignorados"], stats["erros"],
            )
            prog_ts = time.monotonic()

    flush()
    cur.close()
    log.info(
        "%s concluído: lidos=%d enviados=%d ignorados=%d erros=%d",
        fonte_key, stats["lidos"], stats["enviados"], stats["ignorados"], stats["erros"],
    )
    return stats


def parse_data(s: str) -> date:
    return datetime.strptime(s, "%Y-%m-%d").date()


def main() -> int:
    p = argparse.ArgumentParser(description="ERP Bridge Simulador — XML entradas + CT-e (FCCORP → FB_APU04)")
    p.add_argument("--config", default=str(BASE_DIR / "config.yaml"))
    p.add_argument("--data-ini", help="YYYY-MM-DD (inclusive). Não usar com --drain")
    p.add_argument("--data-fim", help="YYYY-MM-DD (inclusive). Não usar com --drain")
    p.add_argument("--tipos", default="entradas,ctes", help="entradas,ctes (padrão ambos)")
    p.add_argument("--batch", type=int, default=300, help="XMLs por POST (padrão 300)")
    p.add_argument("--dry-run", action="store_true", help="lê e conta, sem enviar")
    p.add_argument("--reset-tracker", action="store_true", help="zera o tracker antes (re-importa janela)")
    p.add_argument("--drain", action="store_true",
                   help="consome jobs pendentes da fila (enfileirados pela UI) e sai")
    args = p.parse_args()

    if not os.path.exists(args.config):
        log.error("config não encontrado: %s", args.config)
        return 1
    with open(args.config) as f:
        cfg = yaml.safe_load(f)

    fbtax = FBTax(cfg["fbtax"])
    o = cfg["oracle"]

    tracker_path = BASE_DIR / "tracker_simulador.db"
    if args.reset_tracker and tracker_path.exists():
        tracker_path.unlink()
        log.info("Tracker resetado.")
    tracker = init_tracker(tracker_path)

    def executar_janela(conn_ora, tipos, data_ini, data_fim_excl, job_id="") -> dict:
        grand = {"lidos": 0, "enviados": 0, "ignorados": 0, "erros": 0}
        for tipo in tipos:
            st = processar(tipo, conn_ora, o, fbtax, tracker, data_ini, data_fim_excl,
                           args.batch, args.dry_run, job_id)
            for k in grand:
                grand[k] += st[k]
        return grand

    # ── Modo DRAIN: consome jobs pendentes enfileirados pela UI ────────────────
    if args.drain:
        log.info("=" * 60)
        log.info("ERP Bridge Simulador — MODO DRAIN → %s", fbtax.base_url)
        log.info("=" * 60)
        conn_ora = conectar_oracle(o)
        n_jobs = 0
        try:
            while True:
                try:
                    job = fbtax.get_pending_job()
                except Exception as exc:  # noqa: BLE001
                    log.error("Erro ao buscar job pendente: %s", exc)
                    return 1
                if not job:
                    log.info("Sem jobs pendentes. %d job(s) processado(s).", n_jobs)
                    break
                n_jobs += 1
                jid = job["id"]
                tipos = [t.strip() for t in str(job.get("tipos", "")).split(",") if t.strip() in FONTES]
                if not tipos:
                    tipos = ["entradas", "ctes"]
                try:
                    di = parse_data(job["data_ini"])
                    df_excl = parse_data(job["data_fim"]) + timedelta(days=1)
                except Exception as exc:  # noqa: BLE001
                    log.error("Job %s com datas inválidas: %s", jid, exc)
                    fbtax.report_job(jid, "error", 0, 0, f"datas inválidas: {exc}")
                    continue
                log.info("► Job %s | %s a %s | tipos: %s", jid, job["data_ini"], job["data_fim"], ",".join(tipos))
                try:
                    g = executar_janela(conn_ora, tipos, di, df_excl, jid)
                    status = "done" if g["erros"] == 0 else "error"
                    fbtax.report_job(jid, status, g["enviados"], g["erros"])
                    log.info("◄ Job %s %s — enviados=%d erros=%d", jid, status, g["enviados"], g["erros"])
                except Exception as exc:  # noqa: BLE001
                    log.error("Job %s falhou: %s", jid, exc)
                    fbtax.report_job(jid, "error", 0, 0, str(exc)[:500])
        finally:
            conn_ora.close()
            tracker.close()
        return 0

    # ── Modo janela única (CLI manual) ─────────────────────────────────────────
    if not args.data_ini or not args.data_fim:
        log.error("Informe --data-ini e --data-fim (ou use --drain)")
        return 1
    data_ini = parse_data(args.data_ini)
    data_fim_excl = parse_data(args.data_fim) + timedelta(days=1)  # query usa < data_fim
    tipos = [t.strip() for t in args.tipos.split(",") if t.strip() in FONTES]
    if not tipos:
        log.error("Nenhum tipo válido em --tipos (use entradas,ctes)")
        return 1

    log.info("=" * 60)
    log.info("ERP Bridge Simulador — FCCORP → %s", fbtax.base_url)
    log.info("Período: %s a %s | tipos: %s | batch: %d%s",
             data_ini, args.data_fim, ",".join(tipos), args.batch,
             " | DRY-RUN" if args.dry_run else "")
    log.info("=" * 60)

    conn_ora = conectar_oracle(o)
    try:
        grand = executar_janela(conn_ora, tipos, data_ini, data_fim_excl)
    finally:
        conn_ora.close()
        tracker.close()

    log.info("=" * 60)
    log.info("TOTAL: lidos=%d enviados=%d ignorados=%d erros=%d",
             grand["lidos"], grand["enviados"], grand["ignorados"], grand["erros"])
    log.info("=" * 60)
    return 0 if grand["erros"] == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
