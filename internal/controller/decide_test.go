package controller

import (
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

func TestDecide(t *testing.T) {
	tests := []struct {
		name   string
		jobs   int
		states map[int]jobState
		want   action
	}{
		{"nothing created yet", 2, nil, action{kind: actionCreateJob, index: 0}},
		{"first running", 2, map[int]jobState{0: jobRunning}, action{kind: actionWait, index: 0}},
		{"first done, second missing", 2, map[int]jobState{0: jobSucceeded}, action{kind: actionCreateJob, index: 1}},
		{"second running", 2, map[int]jobState{0: jobSucceeded, 1: jobRunning}, action{kind: actionWait, index: 1}},
		{"first failed stops the pipeline", 2, map[int]jobState{0: jobFailed}, action{kind: actionFail, index: 0}},
		{"failure after a success", 3, map[int]jobState{0: jobSucceeded, 1: jobFailed}, action{kind: actionFail, index: 1}},
		{"all done", 2, map[int]jobState{0: jobSucceeded, 1: jobSucceeded}, action{kind: actionSucceed}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decide(tt.jobs, tt.states); got != tt.want {
				t.Errorf("decide() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestStateOf(t *testing.T) {
	cond := func(ct batchv1.JobConditionType, s corev1.ConditionStatus) *batchv1.Job {
		return &batchv1.Job{Status: batchv1.JobStatus{Conditions: []batchv1.JobCondition{{Type: ct, Status: s}}}}
	}
	tests := []struct {
		name string
		job  *batchv1.Job
		want jobState
	}{
		{"no conditions yet", &batchv1.Job{}, jobRunning},
		{"complete", cond(batchv1.JobComplete, corev1.ConditionTrue), jobSucceeded},
		{"failed", cond(batchv1.JobFailed, corev1.ConditionTrue), jobFailed},
		{"complete=false is still running", cond(batchv1.JobComplete, corev1.ConditionFalse), jobRunning},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stateOf(tt.job); got != tt.want {
				t.Errorf("stateOf() = %v, want %v", got, tt.want)
			}
		})
	}
}
