package handlers

// metrics.go — OBS-01: Instrumentação Prometheus para o backend Go.
// Exporta MetricsMiddleware, contadores de eventos críticos e histograma de latência HTTP.
// Padrão de threat model T-05-01-01: normalizePath impede cardinality explosion via UUIDs/IDs em labels.
// Padrão de threat model T-05-01-05: UUIDs e segmentos numéricos são substituídos por :id e :n.

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ─── Regex para normalização de paths (T-05-01-05) ───────────────────────────

var (
	// metricsReUUID substitui UUIDs por :id no path (ex: /api/runs/abc123-... → /api/runs/:id)
	// Nomeado com prefixo metrics_ para evitar conflito com reUUID em admin.go (mesmo pacote).
	metricsReUUID = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

	// metricsReNum substitui segmentos puramente numéricos por :n (ex: /api/foo/123 → /api/foo/:n)
	metricsReNum = regexp.MustCompile(`/\d+(/|$)`)
)

// normalizePath substitui partes dinâmicas do path (UUIDs, números) por placeholders
// para evitar cardinalidade alta no Prometheus (Pitfall 3 do RESEARCH.md).
func normalizePath(path string) string {
	// Substituir UUIDs por :id
	path = metricsReUUID.ReplaceAllString(path, ":id")

	// Substituir segmentos puramente numéricos por :n, preservando separador final
	path = metricsReNum.ReplaceAllStringFunc(path, func(m string) string {
		if strings.HasSuffix(m, "/") {
			return "/:n/"
		}
		return "/:n"
	})

	// Remover trailing slash exceto para raiz
	if len(path) > 1 && strings.HasSuffix(path, "/") {
		path = strings.TrimRight(path, "/")
	}

	return path
}

// ─── Métricas HTTP (histograma de latência + counter de requisições) ─────────

var (
	// HTTPRequestDuration mede a latência de cada requisição HTTP em segundos.
	// Labels: method (GET/POST/...), path (normalizado), status (código HTTP).
	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Latência das requisições HTTP em segundos",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path", "status"})

	// HTTPRequestsTotal conta o total de requisições HTTP por method/path/status.
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total de requisições HTTP recebidas",
	}, []string{"method", "path", "status"})
)

// ─── Contadores de eventos críticos (OBS-01 / OBS-02) ────────────────────────

var (
	// BridgeRunErrorsTotal conta runs do ERP Bridge finalizados com status=error.
	// Incrementado em erp_bridge.go quando PATCH /api/erp-bridge/runs/{id} recebe status=error.
	BridgeRunErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bridge_run_errors_total",
		Help: "Total de runs do Bridge com status=error",
	})

	// XMLUploadErrorsTotal conta uploads XML rejeitados ou com erro de parse.
	// Incrementado em xml_upload.go quando o processamento de um batch falha/rejeita.
	XMLUploadErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "xml_upload_errors_total",
		Help: "Total de uploads XML rejeitados ou com erro de parse",
	})

	// DatabaseResetTotal conta execuções bem-sucedidas do ResetDatabaseHandler.
	// Incrementado em admin.go APÓS o commit bem-sucedido (não em early returns de erro).
	DatabaseResetTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "database_reset_total",
		Help: "Total de execuções de ResetDatabaseHandler concluídas com sucesso",
	})
)

// ─── statusRecorder ───────────────────────────────────────────────────────────

// statusRecorder envolve http.ResponseWriter para capturar o status HTTP retornado.
// Necessário porque http.ResponseWriter não expõe o código de status após WriteHeader.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

// ─── MetricsMiddleware ────────────────────────────────────────────────────────

// MetricsMiddleware instrumenta todas as requisições HTTP com histograma de latência
// e counter de total. Deve ser encadeado por FORA do SecurityMiddleware para medir
// inclusive requisições bloqueadas por CORS (detecção de ataques — T-05-01-01).
//
// Uso em main.go:
//
//	Handler: handlers.MetricsMiddleware(handlers.SecurityMiddleware(http.DefaultServeMux, origins))
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		sr := &statusRecorder{
			ResponseWriter: w,
			status:         200, // default: se WriteHeader nunca for chamado, assume 200
		}

		next.ServeHTTP(sr, r)

		duration := time.Since(start)
		method := r.Method
		path := normalizePath(r.URL.Path)
		status := strconv.Itoa(sr.status)

		HTTPRequestDuration.WithLabelValues(method, path, status).Observe(duration.Seconds())
		HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
	})
}
