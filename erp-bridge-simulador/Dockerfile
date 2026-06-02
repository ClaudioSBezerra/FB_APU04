FROM python:3.12-slim

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
