# Build Stage
# Pinado em patch exato (22/10, migração de segurança) — Go 1.24 saiu de
# suporte (só 1.25/1.26 recebem patch de segurança hoje; a tag
# "golang:1.24-alpine" está congelada desde fev/2026, sem nenhum fix
# novo possível nela). Subir pra 1.26.10 corrige 15 vulnerabilidades de
# biblioteca padrão achadas via `govulncheck` (net/http, crypto/tls,
# crypto/x509, net/url) — acompanha o bump do go.mod (go 1.24.1 -> 1.26.1).
FROM golang:1.26.10-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app

# Copy go.mod and go.sum from the backend directory
COPY backend/go.mod backend/go.sum ./

# Copy vendor directory
COPY backend/vendor ./vendor

# Debug: Check vendor directory
RUN ls -la vendor && cat vendor/modules.txt

# Copy the rest of the backend source code
COPY backend/ .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -v -ldflags="-w -s" -o server .

# Run Stage
FROM alpine:latest
# postgresql-client: pg_dump usado pelo reset administrativo
# poppler-utils:    fornece pdftotext, usado para extrair NCMs de PDFs do decreto
#                   PRODEPE/PROIND (ver icms_fronteira_prodepe.go).
RUN apk add --no-cache postgresql-client poppler-utils
# Usuário sem privilégio (21/10, migração de segurança) — o backend grava
# em disco de verdade: uploads/ relativo ao WORKDIR (upload.go, worker.go,
# admin.go, efd_contribuicoes_upload.go) e /backups (admin_reset_helpers.go,
# BackupDir() — volume nomeado api_backups, path absoluto FORA do WORKDIR,
# por isso precisa do próprio mkdir+chown aqui: mesmo princípio já
# documentado no stockflow — um volume nomeado NOVO herda o dono de quem
# criou o diretório na imagem na primeira montagem).
RUN adduser -D -u 1000 appuser && \
    chown appuser:appuser /root && \
    mkdir -p /backups && chown appuser:appuser /backups
WORKDIR /root/
COPY --from=builder --chown=appuser:appuser /app/server .
COPY --from=builder --chown=appuser:appuser /app/migrations ./migrations
USER appuser
EXPOSE 8084
CMD ["./server"]
# Force Rebuild v5.5.0 - Z.AI Migration