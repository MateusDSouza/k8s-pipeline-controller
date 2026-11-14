package controller

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus/testutil"

	civ1alpha1 "github.com/MateusDSouza/k8s-pipeline-controller/api/v1alpha1"
	"github.com/MateusDSouza/k8s-pipeline-controller/internal/metrics"
)

func TestRecordTransition_CountsOnlyChanges(t *testing.T) {
	before := testutil.ToFloat64(metrics.PipelinesStarted)
	p := &civ1alpha1.Pipeline{Status: civ1alpha1.PipelineStatus{Phase: civ1alpha1.PhaseRunning}}

	recordTransition(logr.Discard(), p, "")                      // "" -> Running: counted
	recordTransition(logr.Discard(), p, civ1alpha1.PhaseRunning) // Running -> Running: ignored

	if got := testutil.ToFloat64(metrics.PipelinesStarted) - before; got != 1 {
		t.Errorf("started counter increased by %v, want 1", got)
	}
}
