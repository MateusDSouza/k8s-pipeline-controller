package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	civ1alpha1 "github.com/MateusDSouza/k8s-pipeline-controller/api/v1alpha1"
)

var _ = Describe("Pipeline scenarios", func() {
	const ns = "default"
	ctx := context.Background()
	key := func(name string) types.NamespacedName { return types.NamespacedName{Name: name, Namespace: ns} }

	BeforeEach(func() {
		// One template shared by every job: its steps never run in envtest, so their content doesn't matter.
		tmpl := &civ1alpha1.PipelineJobTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: "noop", Namespace: ns},
			Spec: civ1alpha1.PipelineJobTemplateSpec{
				Steps: []civ1alpha1.Step{{Name: "step", Image: "busybox", Command: []string{"true"}}},
			},
		}
		Expect(client.IgnoreAlreadyExists(k8sClient.Create(ctx, tmpl))).To(Succeed())
	})

	// ---------- helpers ----------

	createPipeline := func(name string, jobNames ...string) {
		jobs := make([]civ1alpha1.PipelineJob, len(jobNames))
		for i, j := range jobNames {
			jobs[i] = civ1alpha1.PipelineJob{Name: j, TemplateRef: "noop"}
		}
		p := &civ1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec:       civ1alpha1.PipelineSpec{Jobs: jobs},
		}
		Expect(k8sClient.Create(ctx, p)).To(Succeed())
	}

	reconcileOnce := func(name string) {
		r := &PipelineReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: key(name)})
		Expect(err).NotTo(HaveOccurred())
	}

	// finishJob plays Kubernetes' Job controller (envtest has none): it marks job `index` of a
	// pipeline as succeeded or failed by writing a consistent Job status.
	finishJob := func(pipeline string, index int, succeeded bool) {
		var p civ1alpha1.Pipeline
		Expect(k8sClient.Get(ctx, key(pipeline), &p)).To(Succeed())

		var job batchv1.Job
		Expect(k8sClient.Get(ctx, key(jobName(pipeline, index, p.Spec.Jobs[index].Name)), &job)).To(Succeed())

		now := metav1.Now()
		job.Status.StartTime = &now
		if succeeded {
			job.Status.Succeeded = 1
			job.Status.CompletionTime = &now
			job.Status.Conditions = []batchv1.JobCondition{
				{Type: batchv1.JobSuccessCriteriaMet, Status: corev1.ConditionTrue, Reason: "CompletionsReached", LastTransitionTime: now},
				{Type: batchv1.JobComplete, Status: corev1.ConditionTrue, Reason: "CompletionsReached", LastTransitionTime: now},
			}
		} else {
			job.Status.Failed = 1
			job.Status.Conditions = []batchv1.JobCondition{
				{Type: batchv1.JobFailureTarget, Status: corev1.ConditionTrue, Reason: "BackoffLimitExceeded", LastTransitionTime: now},
				{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Reason: "BackoffLimitExceeded", LastTransitionTime: now},
			}
		}
		Expect(k8sClient.Status().Update(ctx, &job)).To(Succeed())
	}

	// run drives one pipeline: for each outcome, a reconcile creates the next job, then the test
	// finishes it (true = succeeded, false = failed). A final reconcile reacts to the last one.
	run := func(name string, outcomes ...bool) {
		for i, ok := range outcomes {
			reconcileOnce(name)
			finishJob(name, i, ok)
		}
		reconcileOnce(name)
	}

	// expectResult checks the pipeline phase, Ready condition, completion time, each job's phase,
	// and how many Kubernetes Jobs exist (jobs after a failure must never be created).
	expectResult := func(name string, phase civ1alpha1.Phase, jobs []civ1alpha1.Phase, created int) {
		var p civ1alpha1.Pipeline
		Expect(k8sClient.Get(ctx, key(name), &p)).To(Succeed())
		Expect(p.Status.Phase).To(Equal(phase))
		Expect(p.Status.CompletionTime).NotTo(BeNil())
		Expect(meta.IsStatusConditionTrue(p.Status.Conditions, conditionReady)).To(Equal(phase == civ1alpha1.PhaseSucceeded))

		got := make([]civ1alpha1.Phase, len(p.Status.Jobs))
		for i, s := range p.Status.Jobs {
			got[i] = s.Phase
		}
		Expect(got).To(Equal(jobs))

		var list batchv1.JobList
		Expect(k8sClient.List(ctx, &list, client.InNamespace(ns),
			client.MatchingLabels{labelPipeline: name})).To(Succeed())
		Expect(list.Items).To(HaveLen(created))

		// A finished pipeline must do nothing more, however often it is reconciled.
		reconcileOnce(name)
		Expect(k8sClient.List(ctx, &list, client.InNamespace(ns),
			client.MatchingLabels{labelPipeline: name})).To(Succeed())
		Expect(list.Items).To(HaveLen(created))
	}

	const (
		ok      = civ1alpha1.PhaseSucceeded
		failed  = civ1alpha1.PhaseFailed
		skipped = civ1alpha1.PhaseSkipped
	)

	// ---------- scenarios ----------

	It("1 job that succeeds", func() {
		createPipeline("one-job-ok", "build")
		run("one-job-ok", true)
		expectResult("one-job-ok", ok, []civ1alpha1.Phase{ok}, 1)
	})

	It("1 job that fails", func() {
		createPipeline("one-job-fail", "build")
		run("one-job-fail", false)
		expectResult("one-job-fail", failed, []civ1alpha1.Phase{failed}, 1)
	})

	It("2 jobs that succeed", func() {
		createPipeline("two-jobs-ok", "build", "deploy")
		run("two-jobs-ok", true, true)
		expectResult("two-jobs-ok", ok, []civ1alpha1.Phase{ok, ok}, 2)
	})

	It("2 jobs, the first fails and the second is skipped", func() {
		createPipeline("two-jobs-first-fails", "build", "deploy")
		run("two-jobs-first-fails", false)
		expectResult("two-jobs-first-fails", failed, []civ1alpha1.Phase{failed, skipped}, 1)
	})

	It("3 jobs, the second fails and the third is skipped", func() {
		createPipeline("three-jobs-second-fails", "build", "test", "deploy")
		run("three-jobs-second-fails", true, false)
		expectResult("three-jobs-second-fails", failed, []civ1alpha1.Phase{ok, failed, skipped}, 2)
	})

	It("3 jobs that succeed", func() {
		createPipeline("three-jobs-ok", "build", "test", "deploy")
		run("three-jobs-ok", true, true, true)
		expectResult("three-jobs-ok", ok, []civ1alpha1.Phase{ok, ok, ok}, 3)
	})

	It("2 pipelines at the same time: one succeeds, the other fails in its second job", func() {
		createPipeline("parallel-a", "build", "deploy")
		createPipeline("parallel-b", "build", "deploy")

		// Interleave both pipelines step by step: neither may affect the other.
		reconcileOnce("parallel-a")
		reconcileOnce("parallel-b")
		finishJob("parallel-a", 0, true)
		finishJob("parallel-b", 0, true)
		reconcileOnce("parallel-a")
		reconcileOnce("parallel-b")
		finishJob("parallel-a", 1, true)
		finishJob("parallel-b", 1, false)
		reconcileOnce("parallel-a")
		reconcileOnce("parallel-b")

		expectResult("parallel-a", ok, []civ1alpha1.Phase{ok, ok}, 2)
		expectResult("parallel-b", failed, []civ1alpha1.Phase{ok, failed}, 2)
	})
})
