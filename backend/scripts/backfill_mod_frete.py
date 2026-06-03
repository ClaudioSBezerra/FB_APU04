#!/usr/bin/env python3
"""
backfill_mod_frete.py — popula nfe_entradas.mod_frete para NFs já importadas.

Lê os ZIPs de XML guardados em xml_upload_batches (tipo='entradas'), extrai o
<modFrete> de cada NF-e e atualiza nfe_entradas (por company_id + chave_nfe).

Pré-requisitos: migration 139 aplicada (coluna mod_frete) e psql no PATH.
Uso:  DATABASE_URL=postgres://... python3 backfill_mod_frete.py
"""
import base64
import io
import os
import re
import subprocess
import sys
import zipfile

DB = os.environ.get("DATABASE_URL")
if not DB:
    sys.exit("DATABASE_URL não definida")

RE_CHAVE = re.compile(r'Id="NFe(\d{44})"')
RE_CHNFE = re.compile(r"<chNFe>(\d{44})</chNFe>")
RE_MODFRETE = re.compile(r"<modFrete>\s*(\d)\s*</modFrete>")


def psql(sql, tuples_only=True):
    args = ["psql", DB, "-v", "ON_ERROR_STOP=1"]
    if tuples_only:
        args += ["-t", "-A"]
    args += ["-c", sql]
    return subprocess.run(args, capture_output=True, text=True, check=True).stdout


def main():
    batches = psql(
        "SELECT id, company_id FROM xml_upload_batches WHERE tipo='entradas' ORDER BY created_at;"
    ).strip().splitlines()

    total, updated = 0, 0
    for line in batches:
        if not line.strip():
            continue
        bid, company_id = line.split("|")
        b64 = psql(
            f"SELECT encode(xml_data,'base64') FROM xml_upload_batches WHERE id='{bid}';"
        ).strip().replace("\n", "")
        raw = base64.b64decode(b64)
        try:
            zf = zipfile.ZipFile(io.BytesIO(raw))
        except Exception as e:
            print(f"  batch {bid}: ZIP inválido ({e}) — pulando")
            continue

        pairs = []  # (chave, modFrete)
        for name in zf.namelist():
            try:
                txt = zf.read(name).decode("utf-8", "ignore")
            except Exception:
                continue
            m_ch = RE_CHAVE.search(txt) or RE_CHNFE.search(txt)
            m_mf = RE_MODFRETE.search(txt)
            if m_ch and m_mf:
                pairs.append((m_ch.group(1), m_mf.group(1)))

        total += len(pairs)
        if not pairs:
            continue

        # UPDATE em lote via VALUES
        values = ",".join(f"('{c}',{mf})" for c, mf in pairs)
        out = psql(
            f"""WITH v(chave, mf) AS (VALUES {values})
                UPDATE nfe_entradas ne SET mod_frete = v.mf::smallint
                FROM v WHERE ne.company_id='{company_id}' AND ne.chave_nfe=v.chave
                  AND ne.mod_frete IS DISTINCT FROM v.mf::smallint;""",
            tuples_only=False,
        )
        # psql imprime "UPDATE n"
        for ln in out.splitlines():
            if ln.startswith("UPDATE "):
                updated += int(ln.split()[1])

    print(f"Backfill concluído: {total} NF-e lidas dos ZIPs, {updated} linhas atualizadas em nfe_entradas.")


if __name__ == "__main__":
    main()
