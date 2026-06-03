#!/usr/bin/env python3
"""
backfill_receb.py — popula cte_entradas.receb_cnpj_cpf / receb_nome para CT-es
já importados.

Lê os ZIPs de XML guardados em xml_upload_batches (tipo='ctes'), extrai o bloco
<receb> de cada CT-e e atualiza cte_entradas (por company_id + chave_cte).

Pré-requisitos: migration 140 aplicada (colunas receb_*) e psql no PATH.
Uso:  DATABASE_URL=postgres://... python3 backfill_receb.py
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

RE_CHCTE = re.compile(r'Id="CTe(\d{44})"')
RE_CHCTE2 = re.compile(r"<chCTe>(\d{44})</chCTe>")
RE_RECEB = re.compile(r"<receb>(.*?)</receb>", re.DOTALL)
RE_DOC = re.compile(r"<(?:CNPJ|CPF)>(\d{11,14})</(?:CNPJ|CPF)>")
RE_NOME = re.compile(r"<xNome>([^<]*)</xNome>")


def psql(sql, tuples_only=True):
    args = ["psql", DB, "-v", "ON_ERROR_STOP=1"]
    if tuples_only:
        args += ["-t", "-A"]
    args += ["-c", sql]
    return subprocess.run(args, capture_output=True, text=True, check=True).stdout


def sql_str(s):
    return "'" + s.replace("'", "''") + "'"


def parse_cte(txt):
    """Retorna (chave_cte, receb_cnpj, receb_nome) ou None."""
    m_ch = RE_CHCTE.search(txt) or RE_CHCTE2.search(txt)
    if not m_ch:
        return None
    m_receb = RE_RECEB.search(txt)
    if not m_receb:
        return (m_ch.group(1), "", "")  # CT-e sem <receb>
    bloco = m_receb.group(1)
    doc = RE_DOC.search(bloco)
    nome = RE_NOME.search(bloco)
    return (m_ch.group(1), doc.group(1) if doc else "", nome.group(1).strip() if nome else "")


def main():
    batches = psql(
        "SELECT id, company_id FROM xml_upload_batches WHERE tipo='ctes' ORDER BY created_at;"
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

        docs = []  # textos XML individuais
        try:
            zf = zipfile.ZipFile(io.BytesIO(raw))
            for name in zf.namelist():
                try:
                    docs.append(zf.read(name).decode("utf-8", "ignore"))
                except Exception:
                    continue
        except zipfile.BadZipFile:
            # Não é ZIP — trata como um ou mais XML concatenados
            docs = [raw.decode("utf-8", "ignore")]

        triples = []  # (chave_cte, receb_cnpj, receb_nome)
        for txt in docs:
            # Um "doc" pode conter vários <cteProc> concatenados
            for part in re.split(r"(?=<cteProc)", txt):
                r = parse_cte(part)
                if r and r[1]:  # só atualiza quando há receb com CNPJ/CPF
                    triples.append(r)

        total += len(triples)
        if not triples:
            continue

        values = ",".join(
            f"({sql_str(c)},{sql_str(cnpj)},{sql_str(nome)})" for c, cnpj, nome in triples
        )
        out = psql(
            f"""WITH v(chave, cnpj, nome) AS (VALUES {values})
                UPDATE cte_entradas ce
                   SET receb_cnpj_cpf = NULLIF(v.cnpj,''),
                       receb_nome     = NULLIF(v.nome,'')
                FROM v
                WHERE ce.company_id='{company_id}' AND ce.chave_cte=v.chave
                  AND ce.receb_cnpj_cpf IS DISTINCT FROM NULLIF(v.cnpj,'');""",
            tuples_only=False,
        )
        for ln in out.splitlines():
            if ln.startswith("UPDATE "):
                updated += int(ln.split()[1])

    print(f"Backfill concluído: {total} CT-es com <receb> lidos dos ZIPs, "
          f"{updated} linhas atualizadas em cte_entradas.")


if __name__ == "__main__":
    main()
