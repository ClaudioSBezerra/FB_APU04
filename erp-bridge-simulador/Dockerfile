FROM python:3.12-slim

# Stdout/stderr sem buffer — logs aparecem na hora em `docker logs` (essencial no
# modo --daemon, que fica idle no long-poll e geraria pouca saída pra flushar).
ENV PYTHONUNBUFFERED=1

WORKDIR /app

RUN pip install --no-cache-dir \
    pyyaml \
    requests \
    oracledb

COPY bridge_simulador.py .

# config.yaml e tracker_simulador.db montados como volume — não baked na imagem.
VOLUME ["/app/data"]

# Conector manual/on-demand: sem --daemon. O entrypoint recebe os args (datas).
# Ex.: docker run ... erp-bridge-simulador --data-ini 2026-04-01 --data-fim 2026-04-30
ENTRYPOINT ["python", "bridge_simulador.py"]
