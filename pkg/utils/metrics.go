package utils

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	vaultReconcileSuccessGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vault_manager_reconcile_success",
			Help: `Whether or not last reconcile was for a specific vault instance was successful. ` +
				`A reconcile is successful if no errors occur. 0 = success. 1 = failure.`,
		},
		[]string{"address"},
	)
	executionDurationGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "vault_manager_duration_seconds",
			Help: "Execution duration of this job (reconciling all vault instances) in seconds.",
		},
	)
)

func init() {
	// metrics must be registered in order to expose
	prometheus.MustRegister(vaultReconcileSuccessGauge)
	prometheus.MustRegister(executionDurationGauge)
}

func RecordMetrics(instanceSuccesses map[string]int, execDuration time.Duration) {
	for instance, success := range instanceSuccesses {
		vaultReconcileSuccessGauge.With(
			prometheus.Labels{
				"address": instance,
			}).Set(float64(success))
	}
	executionDurationGauge.Set(execDuration.Seconds())
}
