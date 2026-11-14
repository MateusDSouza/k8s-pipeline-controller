package controller

import (
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	civ1alpha1 "github.com/MateusDSouza/k8s-pipeline-controller/api/v1alpha1"
)

// ---------- setPhase ----------

func TestSetPhase_Timestamps(t *testing.T) {
	tests := []struct {
		name           string
		phase          civ1alpha1.Phase
		wantStart      bool
		wantCompletion bool
	}{
		{"pending", civ1alpha1.PhasePending, false, false},
		{"running", civ1alpha1.PhaseRunning, true, false},
		{"succeeded", civ1alpha1.PhaseSucceeded, true, true},
		{"failed", civ1alpha1.PhaseFailed, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newPipeline("p", "job")
			setPhase(p, tt.phase, "Reason", "message")

			if p.Status.Phase != tt.phase {
				t.Errorf("phase = %q, want %q", p.Status.Phase, tt.phase)
			}
			if got := p.Status.StartTime != nil; got != tt.wantStart {
				t.Errorf("startTime set = %v, want %v", got, tt.wantStart)
			}
			if got := p.Status.CompletionTime != nil; got != tt.wantCompletion {
				t.Errorf("completionTime set = %v, want %v", got, tt.wantCompletion)
			}
		})
	}
}

// StartTime is set once, when the pipeline leaves Pending, and never moved.
func TestSetPhase_StartTimeNotOverwritten(t *testing.T) {
	p := newPipeline("p", "job")
	setPhase(p, civ1alpha1.PhaseRunning, "JobRunning", "")
	first := *p.Status.StartTime

	time.Sleep(time.Second) // metav1.Time has second precision
	setPhase(p, civ1alpha1.PhaseRunning, "JobRunning", "")
	setPhase(p, civ1alpha1.PhaseSucceeded, "AllJobsSucceeded", "")

	if !p.Status.StartTime.Equal(&first) {
		t.Errorf("startTime changed from %v to %v", first, p.Status.StartTime)
	}
}

// Ready is True only when the pipeline succeeded.
func TestSetPhase_ReadyCondition(t *testing.T) {
	tests := []struct {
		phase civ1alpha1.Phase
		want  metav1.ConditionStatus
	}{
		{civ1alpha1.PhasePending, metav1.ConditionFalse},
		{civ1alpha1.PhaseRunning, metav1.ConditionFalse},
		{civ1alpha1.PhaseSucceeded, metav1.ConditionTrue},
		{civ1alpha1.PhaseFailed, metav1.ConditionFalse},
	}

	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			p := newPipeline("p", "job")
			setPhase(p, tt.phase, "SomeReason", "some message")

			c := meta.FindStatusCondition(p.Status.Conditions, conditionReady)
			if c == nil {
				t.Fatal("Ready condition not set")
			}
			if c.Status != tt.want {
				t.Errorf("Ready = %q, want %q", c.Status, tt.want)
			}
		})
	}
}

// Calling setPhase again updates the condition instead of adding a second one.
func TestSetPhase_SingleConditionUpdated(t *testing.T) {
	p := newPipeline("p", "job")
	p.Generation = 3

	setPhase(p, civ1alpha1.PhaseRunning, "JobRunning", "job build is running")
	setPhase(p, civ1alpha1.PhaseFailed, "JobFailed", "job build failed")

	if len(p.Status.Conditions) != 1 {
		t.Fatalf("got %d conditions, want 1: %+v", len(p.Status.Conditions), p.Status.Conditions)
	}
	c := p.Status.Conditions[0]
	if c.Reason != "JobFailed" || c.Message != "job build failed" {
		t.Errorf("condition = %s/%q, want JobFailed/%q", c.Reason, c.Message, "job build failed")
	}
	if c.ObservedGeneration != 3 {
		t.Errorf("observedGeneration = %d, want 3", c.ObservedGeneration)
	}
}

// ---------- jobStatuses ----------

func TestJobStatuses_NoJobsYet(t *testing.T) {
	p := newPipeline("p", "build", "test")

	got := jobStatuses(p, map[int]*batchv1.Job{})

	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2", len(got))
	}
	for i, want := range []string{"build", "test"} {
		if got[i].Name != want || got[i].Phase != civ1alpha1.PhasePending || got[i].JobName != "" {
			t.Errorf("entry %d = %+v, want Pending %q with no JobName", i, got[i], want)
		}
	}
}

func TestJobStatuses_EachState(t *testing.T) {
	start := metav1.NewTime(time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC))
	end := metav1.NewTime(start.Add(2 * time.Minute))

	p := newPipeline("p", "build", "test", "deploy", "notify")
	jobs := map[int]*batchv1.Job{
		0: finishedJob("p-0-build", start, end, batchv1.JobComplete),
		1: finishedJob("p-1-test", start, end, batchv1.JobFailed),
		2: runningJob("p-2-deploy", start),
		// 3: not created yet
	}

	got := jobStatuses(p, jobs)

	tests := []struct {
		wantPhase      civ1alpha1.Phase
		wantJobName    string
		wantCompletion bool
	}{
		{civ1alpha1.PhaseSucceeded, "p-0-build", true},
		{civ1alpha1.PhaseFailed, "p-1-test", true},
		{civ1alpha1.PhaseRunning, "p-2-deploy", false},
		{civ1alpha1.PhasePending, "", false},
	}
	for i, tt := range tests {
		s := got[i]
		if s.Phase != tt.wantPhase || s.JobName != tt.wantJobName {
			t.Errorf("entry %d = %s/%q, want %s/%q", i, s.Phase, s.JobName, tt.wantPhase, tt.wantJobName)
		}
		if gotC := s.CompletionTime != nil; gotC != tt.wantCompletion {
			t.Errorf("entry %d completionTime set = %v, want %v", i, gotC, tt.wantCompletion)
		}
	}
	if got[0].CompletionTime != nil && !got[0].CompletionTime.Equal(&end) {
		t.Errorf("succeeded completionTime = %v, want %v", got[0].CompletionTime, end)
	}
}

// ---------- helpers for job states ----------

func runningJob(name string, start metav1.Time) *batchv1.Job {
	j := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: name}}
	j.Status.StartTime = &start
	j.Status.Active = 1
	return j
}

// finishedJob returns a Job that ended with the given condition (JobComplete or JobFailed).
func finishedJob(name string, start, end metav1.Time, cond batchv1.JobConditionType) *batchv1.Job {
	j := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: name}}
	j.Status.StartTime = &start
	j.Status.Conditions = []batchv1.JobCondition{{
		Type: cond, Status: corev1.ConditionTrue, LastTransitionTime: end,
	}}
	if cond == batchv1.JobComplete {
		j.Status.Succeeded = 1
		j.Status.CompletionTime = &end
	} else {
		j.Status.Failed = 1
	}
	return j
}
