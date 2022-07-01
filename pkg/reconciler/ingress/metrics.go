package ingress

import (
	"time"

	"github.com/kuadrant/kcp-glbc/pkg/metrics"
	"github.com/kuadrant/kcp-glbc/pkg/tls"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	issuerLabel          = "issuer"
	resultLabel          = "result"
	resultLabelSucceeded = "succeeded"
	resultLabelFailed    = "failed"
)

var (
	ingressObjectTimeToAdmission = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name: "glbc_ingress_managed_object_time_to_admission",
			Help: "Duration of the ingress object admission",
			Buckets: []float64{
				1 * time.Second.Seconds(),
				5 * time.Second.Seconds(),
				10 * time.Second.Seconds(),
				15 * time.Second.Seconds(),
				30 * time.Second.Seconds(),
				45 * time.Second.Seconds(),
				1 * time.Minute.Seconds(),
				2 * time.Minute.Seconds(),
				5 * time.Minute.Seconds(),
			},
		})

	ingressObjectTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "glbc_ingress_managed_object_total",
			Help: "Total number of managed ingress object",
		})

	tlsCertificateRequestCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "glbc_tls_certificate_pending_request_count",
			Help: "GLBC TLS certificate pending request count",
		},
		[]string{
			issuerLabel,
		},
	)

	// tlsCertificateRequestTotal is a prometheus counter metrics which holds the total
	// number of TLS certificate requests.
	tlsCertificateRequestTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "glbc_tls_certificate_request_total",
			Help: "GLBC TLS certificate total number of requests",
		},
		[]string{
			issuerLabel,
			resultLabel,
		},
	)

	// tlsCertificateIssuanceDuration is a prometheus metric which records the duration
	// of TLS certificate issuance.
	tlsCertificateIssuanceDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "glbc_tls_certificate_issuance_duration_seconds",
			Help: "GLBC TLS certificate issuance duration",
			Buckets: []float64{
				1 * time.Second.Seconds(),
				5 * time.Second.Seconds(),
				10 * time.Second.Seconds(),
				15 * time.Second.Seconds(),
				30 * time.Second.Seconds(),
				45 * time.Second.Seconds(),
				1 * time.Minute.Seconds(),
				2 * time.Minute.Seconds(),
				5 * time.Minute.Seconds(),
			},
		},
		[]string{
			issuerLabel,
			resultLabel,
		},
	)

	// tlsCertificateRequestErrors is a prometheus counter metrics which holds the total
	// number of failed TLS certificate requests.
	tlsCertificateRequestErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "glbc_tls_certificate_request_errors_total",
			Help: "GLBC TLS certificate total number of request errors",
		},
		// TODO: check if it's possible to add an error/code label
		[]string{
			issuerLabel,
		},
	)
	// tlsCertificateSecretCount is a prometheus metric which holds the number of
	// TLS certificates currently managed.
	tlsCertificateSecretCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "glbc_tls_certificate_secret_count",
			Help: "GLBC TLS certificate secret count",
		},
		[]string{
			issuerLabel,
		},
	)
)

func init() {
	// Register metrics with the global prometheus registry
	metrics.Registry.MustRegister(
		ingressObjectTimeToAdmission,
		ingressObjectTotal,
		tlsCertificateRequestCount,
		tlsCertificateRequestErrors,
		tlsCertificateRequestTotal,
		tlsCertificateIssuanceDuration,
		tlsCertificateSecretCount,
	)
}

func InitMetrics(provider tls.Provider) {
	issuer := provider.IssuerID()
	tlsCertificateRequestCount.WithLabelValues(issuer).Set(0)
	tlsCertificateRequestTotal.WithLabelValues(issuer, resultLabelSucceeded).Add(0)
	tlsCertificateRequestTotal.WithLabelValues(issuer, resultLabelFailed).Add(0)
	tlsCertificateRequestErrors.WithLabelValues(issuer).Add(0)
	tlsCertificateSecretCount.WithLabelValues(issuer).Set(0)
}
