package controller

import (
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	civ1alpha1 "github.com/MateusDSouza/k8s-pipeline-controller/api/v1alpha1"
)

const conditionReady = "Ready"

// setPhase is the ONE place where a pipeline's phase changes.
func setPhase(p *civ1alpha1.Pipeline, phase civ1alpha1.Phase, reason, message string) {
	now := metav1.Now()
	if p.Status.StartTime == nil && phase != civ1alpha1.PhasePending {
		p.Status.StartTime = &now
	}
	if (phase == civ1alpha1.PhaseSucceeded || phase == civ1alpha1.PhaseFailed) && p.Status.CompletionTime == nil {
		p.Status.CompletionTime = &now
	}
	p.Status.Phase = phase

	ready := metav1.ConditionFalse
	if phase == civ1alpha1.PhaseSucceeded {
		ready = metav1.ConditionTrue
	}
	meta.SetStatusCondition(&p.Status.Conditions, metav1.Condition{
		Type:               conditionReady,
		Status:             ready,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: p.Generation,
	})
}

// jobStatuses rebuilds the per-job status from the Jobs that actually exist.
func jobStatuses(p *civ1alpha1.Pipeline, jobs map[int]*batchv1.Job) []civ1alpha1.JobStatus {
	out := make([]civ1alpha1.JobStatus, len(p.Spec.Jobs))
	for i, pj := range p.Spec.Jobs {
		s := civ1alpha1.JobStatus{Name: pj.Name, Phase: civ1alpha1.PhasePending}
		if j, ok := jobs[i]; ok {
			s.JobName = j.Name
			s.StartTime = j.Status.StartTime
			switch stateOf(j) {
			case jobSucceeded:
				s.Phase = civ1alpha1.PhaseSucceeded
				s.CompletionTime = j.Status.CompletionTime
			case jobFailed:
				s.Phase = civ1alpha1.PhaseFailed
				if c := jobCondition(j, batchv1.JobFailed); c != nil {
					t := c.LastTransitionTime
					s.CompletionTime = &t
				}
			default:
				s.Phase = civ1alpha1.PhaseRunning
			}
		}
		out[i] = s
	}
	return out
}
