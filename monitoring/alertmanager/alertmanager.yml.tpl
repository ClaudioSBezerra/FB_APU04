# Alertmanager — FB_APU04 Simulador Fiscal
# Plan 02 (OBS-02) — Configuração SMTP com roteamento por severidade
#
# MECANISMO DE EXPANSÃO DE VARIÁVEIS:
# O Alertmanager v0.27 NÃO expande variáveis de ambiente diretamente no YAML.
# Este arquivo é um TEMPLATE (.tpl) que é processado por `awk` no entrypoint
# do container (docker-compose.yml), gerando /tmp/alertmanager.yml antes do binário
# iniciar. As variáveis ${SMTP_*} são substituídas pelos valores reais em runtime via
# ENVIRON["VAR"] do awk (disponível no Busybox da imagem prom/alertmanager).
# Os valores reais ficam no Coolify secrets (nunca versionados no repo).
# Nota: envsubst não está disponível na imagem prom/alertmanager:v0.27.0 (sem apk).
#
# CONFIGURAÇÃO TLS/SMTP:
# O backend Go (services/email.go) usa porta 465 com SSL implícito (sendMailSSL).
# smtp_require_tls: false é o padrão correto para SSL implícito na porta 465.
# Se SMTP_PORT for 587 com STARTTLS, alterar para: smtp_require_tls: true

global:
  resolve_timeout: 5m

  # SMTP — variáveis expandidas em runtime pelo envsubst no entrypoint
  smtp_smarthost: "${SMTP_HOST}:${SMTP_PORT}"
  smtp_from: "${SMTP_FROM}"
  smtp_auth_username: "${SMTP_USER}"
  smtp_auth_password: "${SMTP_PASSWORD}"

  # SSL implícito (porta 465 — padrão do Hostinger/email.go).
  # Para STARTTLS na porta 587, alterar para: smtp_require_tls: true
  smtp_require_tls: false

# ── Roteamento principal ───────────────────────────────────────────────────
route:
  # Agrupamento por alertname + severity para deduplicação eficiente
  group_by: ["alertname", "severity"]
  # Aguardar 10s antes de enviar o primeiro alerta do grupo
  group_wait: 10s
  # Aguardar 5m para agregar novos alertas do mesmo grupo
  group_interval: 5m
  # Repetir alertas não resolvidos a cada 4h (padrão para warnings)
  repeat_interval: 4h
  receiver: "fiscal-team-default"

  # Sub-rotas por severidade (processadas em ordem; primeira match vence)
  routes:
    # Critical: menor latência, repetição mais frequente
    - matchers:
        - severity = "critical"
      receiver: "fiscal-team-critical"
      group_wait: 5s
      repeat_interval: 1h

    # Warning: menos urgente, menor volume de email
    - matchers:
        - severity = "warning"
      receiver: "fiscal-team-warning"
      repeat_interval: 12h

    # Fallback explícito para alerts sem severity label (segurança)
    - matchers:
        - team = "fiscal"
      receiver: "fiscal-team-default"

# ── Inibição de alertas correlatos ────────────────────────────────────────
# Quando o Bridge está completamente down (BridgeDaemonDown), suprimir DPY-4011
# porque a causa raiz é o daemon, não a conexão Oracle isolada.
inhibit_rules:
  - source_matchers:
      - severity = "critical"
      - alertname = "BridgeDaemonDown"
    target_matchers:
      - severity = "critical"
      - alertname = "BridgeDPY4011Consecutivos"
    # Só inibe se for o mesmo team (evita inibição cruzada futura)
    equal: ["team"]

# ── Receivers ─────────────────────────────────────────────────────────────
receivers:

  # Receiver padrão (alerts sem rota específica)
  - name: "fiscal-team-default"
    email_configs:
      - to: "claudio.bezerra@ferreiracosta.com.br"
        send_resolved: true
        headers:
          Subject: "[FB_APU04][{{ .GroupLabels.severity }}] {{ .GroupLabels.alertname }}"
        html: |
          <h2>{{ .GroupLabels.alertname }}</h2>
          {{ range .Alerts }}
          <p><b>{{ .Annotations.summary }}</b></p>
          <p>{{ .Annotations.description }}</p>
          <p>Severity: <strong>{{ .Labels.severity }}</strong></p>
          <p>Runbook: <a href="{{ .Annotations.runbook_url }}">{{ .Annotations.runbook_url }}</a></p>
          <hr>
          {{ end }}

  # Receiver para alertas CRITICAL — email urgente com destaque visual
  - name: "fiscal-team-critical"
    email_configs:
      - to: "claudio.bezerra@ferreiracosta.com.br"
        send_resolved: true
        headers:
          Subject: "[FB_APU04][CRITICAL] {{ .GroupLabels.alertname }}"
        html: |
          <h1 style="color:red">CRITICAL: {{ .GroupLabels.alertname }}</h1>
          {{ range .Alerts }}
          <p><b>{{ .Annotations.summary }}</b></p>
          <p>{{ .Annotations.description }}</p>
          <p><a href="{{ .Annotations.runbook_url }}" style="color:red;font-weight:bold">RUNBOOK — Clique aqui para passos de mitigação</a></p>
          <hr>
          {{ end }}

  # Receiver para alertas WARNING — menos urgente, sem cores vermelhas
  - name: "fiscal-team-warning"
    email_configs:
      - to: "claudio.bezerra@ferreiracosta.com.br"
        send_resolved: false
        headers:
          Subject: "[FB_APU04][WARNING] {{ .GroupLabels.alertname }}"
        html: |
          <h2>WARNING: {{ .GroupLabels.alertname }}</h2>
          {{ range .Alerts }}
          <p>{{ .Annotations.description }}</p>
          <p><a href="{{ .Annotations.runbook_url }}">Runbook: {{ .Annotations.runbook_url }}</a></p>
          <hr>
          {{ end }}
