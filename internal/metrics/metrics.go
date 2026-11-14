// Package metrics defines the controller's custom Prometheus metrics. They are registered with
// controller-runtime's registry, so they're served on the same /metrics endpoint as the built-in ones.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	PipelinesStarted = prometheus.NewCounter(prometheus.CounterOpts{ // this is a counter that export the number of pipelines that have being started
		Name: "pipeline_controller_pipelines_started_total",
		Help: "Pipelines that started running.",
	})

	PipelinesFinished = prometheus.NewCounterVec(prometheus.CounterOpts{ //this is also a counter but grouped by the phase where they finish -> succeeded or failed
		Name: "pipeline_controller_pipelines_finished_total",
		Help: "Pipelines that finished, by final phase.",
	}, []string{"phase"})

	PipelineDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{ // here we have a histogram also with the pipelines grouped by phase -> succeeded or failed
		Name:    "pipeline_controller_pipeline_duration_seconds",
		Help:    "Time from start to completion, by final phase.",
		Buckets: prometheus.ExponentialBuckets(1, 2, 12), // 1s, 2s, 4s ... ~34 min
	}, []string{"phase"})
)

func init() { // this is a special go function that runs before main is called and automatically register those variables and let them be called
	ctrlmetrics.Registry.MustRegister(PipelinesStarted, PipelinesFinished, PipelineDuration)
}
