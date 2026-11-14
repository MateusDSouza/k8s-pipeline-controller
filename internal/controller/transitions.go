package controller

import (
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"

	civ1alpha1 "github.com/MateusDSouza/k8s-pipeline-controller/api/v1alpha1"
	"github.com/MateusDSouza/k8s-pipeline-controller/internal/metrics"
)

// recordTransition logs and counts a phase change. It's called once, after the new status was saved,
// so a failed status write (and the retry that follows) can't count the same transition twice.
func recordTransition(log logr.Logger, p *civ1alpha1.Pipeline, from civ1alpha1.Phase) {
	to := p.Status.Phase
	if from == to {
		return
	}

	reason, message := "", ""
	if c := meta.FindStatusCondition(p.Status.Conditions, conditionReady); c != nil {
		reason, message = c.Reason, c.Message
	}
	log.Info("pipeline phase changed", "from", from, "to", to, "reason", reason, "message", message)

	switch to {
	case civ1alpha1.PhaseRunning:
		metrics.PipelinesStarted.Inc()
	case civ1alpha1.PhaseSucceeded, civ1alpha1.PhaseFailed:
		metrics.PipelinesFinished.WithLabelValues(string(to)).Inc()
		if p.Status.StartTime != nil && p.Status.CompletionTime != nil {
			d := p.Status.CompletionTime.Sub(p.Status.StartTime.Time).Seconds()
			metrics.PipelineDuration.WithLabelValues(string(to)).Observe(d)
		}
	}
}
