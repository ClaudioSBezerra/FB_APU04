FROM python:3.12-slim

WORKDIR /app

RUN pip install --no-cache-dir \
    pyyaml \
    requests \
    prometheus_client \
    oracledb \
    python-dateutil

COPY bridge.py .

# config.yaml montado como volume no servidor — não baked na imagem
VOLUME ["/app/data"]

CMD ["python", "bridge.py", "--daemon"]
