package controller

import (
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

// jobState is what the controller observes for one pipeline job.
// The zero value is jobMissing, so looking up a job that was never created in a map returns "missing" -> 0
type jobState int

const (
	jobMissing jobState = iota // this is a golang concept -  iota is a counter that starts at 0 in each const block and goes up by 1 on every line.
	jobRunning
	jobSucceeded
	jobFailed
) // this enum is an int type

// stateOf reads the final conditions the Job controller sets on a Job.
func stateOf(j *batchv1.Job) jobState {
	if c := jobCondition(j, batchv1.JobComplete); c != nil && c.Status == corev1.ConditionTrue { // we always check the condition of succeeded, then if fail -> if both are false we just return that the job is still running.
		return jobSucceeded
	}
	if c := jobCondition(j, batchv1.JobFailed); c != nil && c.Status == corev1.ConditionTrue {
		return jobFailed
	}
	return jobRunning
} // because this returns an internal state of the job in the k8s API, if we will try to log this will be bad. We need to log explicit string values to know the state of the job.

func jobCondition(j *batchv1.Job, t batchv1.JobConditionType) *batchv1.JobCondition {
	for i := range j.Status.Conditions {
		if j.Status.Conditions[i].Type == t {
			return &j.Status.Conditions[i]
		}
	}
	return nil
}

type actionKind int

const (
	actionCreateJob actionKind = iota // create the Job for `index`
	actionWait                        // job `index` is running: nothing to do
	actionFail                        // job `index` failed: stop the pipeline
	actionSucceed                     // every job succeeded
)

type action struct {
	kind  actionKind
	index int
}

// decide walks the jobs in order and returns the single next action.
// Sequential and fail-fast: job N+1 is only considered after job N succeeded.
func decide(numJobs int, states map[int]jobState) action {
	for i := 0; i < numJobs; i++ {
		switch states[i] {
		case jobMissing:
			return action{kind: actionCreateJob, index: i}
		case jobRunning:
			return action{kind: actionWait, index: i}
		case jobFailed:
			return action{kind: actionFail, index: i}
		}
		// jobSucceeded: move on to the next job
	}
	return action{kind: actionSucceed}
}
