package controller

import (
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	civ1alpha1 "github.com/MateusDSouza/k8s-pipeline-controller/api/v1alpha1"
)

func finishedJob(ct batchv1.JobConditionType) *batchv1.Job {
	return &batchv1.Job{Status: batchv1.JobStatus{
		Conditions: []batchv1.JobCondition{{Type: ct, Status: corev1.ConditionTrue}},
	}}
}

func TestJobStatuses(t *testing.T) {
	var (
		ok      = finishedJob(batchv1.JobComplete)
		failed  = finishedJob(batchv1.JobFailed)
		running = &batchv1.Job{} // no final condition yet
	)
	tests := []struct {
		name  string
		phase civ1alpha1.Phase // pipeline phase, as set by setPhase before jobStatuses runs
		jobs  map[int]*batchv1.Job
		want  []civ1alpha1.Phase
	}{
		{"running: later jobs still pending", civ1alpha1.PhaseRunning,
			map[int]*batchv1.Job{0: ok, 1: running},
			[]civ1alpha1.Phase{civ1alpha1.PhaseSucceeded, civ1alpha1.PhaseRunning, civ1alpha1.PhasePending}},
		{"second job failed: third skipped", civ1alpha1.PhaseFailed,
			map[int]*batchv1.Job{0: ok, 1: failed},
			[]civ1alpha1.Phase{civ1alpha1.PhaseSucceeded, civ1alpha1.PhaseFailed, civ1alpha1.PhaseSkipped}},
		{"first job failed: rest skipped", civ1alpha1.PhaseFailed,
			map[int]*batchv1.Job{0: failed},
			[]civ1alpha1.Phase{civ1alpha1.PhaseFailed, civ1alpha1.PhaseSkipped, civ1alpha1.PhaseSkipped}},
		{"all succeeded", civ1alpha1.PhaseSucceeded,
			map[int]*batchv1.Job{0: ok, 1: ok, 2: ok},
			[]civ1alpha1.Phase{civ1alpha1.PhaseSucceeded, civ1alpha1.PhaseSucceeded, civ1alpha1.PhaseSucceeded}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &civ1alpha1.Pipeline{
				Spec: civ1alpha1.PipelineSpec{Jobs: []civ1alpha1.PipelineJob{
					{Name: "build", TemplateRef: "t"},
					{Name: "test", TemplateRef: "t"},
					{Name: "deploy", TemplateRef: "t"},
				}},
				Status: civ1alpha1.PipelineStatus{Phase: tt.phase},
			}
			got := jobStatuses(p, tt.jobs)
			for i, want := range tt.want {
				if got[i].Phase != want {
					t.Errorf("job %d (%s): phase = %s, want %s", i, got[i].Name, got[i].Phase, want)
				}
			}
		})
	}
}
