package utils

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

const (
	JOB                      = "vault-manager"
	RECONCILE_SUCCESS_METRIC = "vault_manager_reconcile_success"
	DURATION_METRIC          = "vault_manager_duration_seconds"
)

// push new values to vault_manager_reconcile_success for each instance reconciled
func PushInstanceReconcileMetric(pushGatewayUrl string, instanceSuccess map[string]int) error {
	vaultReconcileSuccessGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: RECONCILE_SUCCESS_METRIC,
			Help: `Whether or not last reconcile was successful. ` +
				`A reconcile is successful if no errors occur. ` +
				`0 = success. 1 = failure.`,
		},
	)

	for instance, success := range instanceSuccess {
		vaultReconcileSuccessGauge.Set(float64(success))
		err := push.New(pushGatewayUrl, JOB).
			Grouping("vault_instance", instance). // label
			Collector(vaultReconcileSuccessGauge).
			Push()
		if err != nil {
			return err
		}
	}
	return nil
}

// push new total execution time metric
func PushExecutionDurationMetric(pushGatewayUrl string, duration time.Duration) error {
	executionDurationGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: DURATION_METRIC,
			Help: "Execution duration of this job in seconds.",
		},
	)
	executionDurationGauge.Set(duration.Seconds())
	err := push.New(pushGatewayUrl, JOB).Collector(executionDurationGauge).Push()
	return err
}
