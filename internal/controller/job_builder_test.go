package controller

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	civ1alpha1 "github.com/MateusDSouza/k8s-pipeline-controller/api/v1alpha1" // ---> importing the api types
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

// jobNameCases is shared by the jobName tests.
var jobNameCases = []struct {
	name     string
	pipeline string
	index    int
	job      string
	want     string
	valid    bool
}{
	{"first job", "build-app", 0, "compile", "build-app-0-compile", true},
	{"later job", "build-app", 3, "test", "build-app-3-test", true},
	{"double digit index", "release", 12, "push", "release-12-push", true},

	// Boundary: pipeline + job = 59 chars, highest index → exactly 63 chars.
	{"longest names the CRD allows",
		strings.Repeat("p", 30), 49, strings.Repeat("j", 29),
		strings.Repeat("p", 30) + "-49-" + strings.Repeat("j", 29), true},

	// Rejected by the CRD: the name would exceed 63 chars.
	{"long but realistic names (rejected by CRD)",
		"payments-service-release-candidate-build", 7, "integration-tests-against-staging",
		"payments-service-release-candidate-build-7-integration-tests-against-staging", false},
	{"max job name with short pipeline (rejected by CRD)",
		"build-app", 49, strings.Repeat("a", 63),
		"build-app-49-" + strings.Repeat("a", 63), false},
}

func TestJobName(t *testing.T) {
	for _, tt := range jobNameCases {
		t.Run(tt.name, func(t *testing.T) {
			got := jobName(tt.pipeline, tt.index, tt.job)
			if got != tt.want {
				t.Errorf("jobName(%q, %d, %q) = %q, want %q",
					tt.pipeline, tt.index, tt.job, got, tt.want)
			}
		})
	}
}

// The controller relies on finding the Job it created in an earlier reconcile.
func TestJobName_IsDeterministic(t *testing.T) {
	first := jobName("build-app", 1, "test")
	second := jobName("build-app", 1, "test")
	if first != second {
		t.Fatalf("same inputs gave different names: %q and %q", first, second)
	}
}

// Two jobs of the same pipeline must never share a Job name.
func TestJobName_UniquePerIndex(t *testing.T) {
	a := jobName("build-app", 0, "test")
	b := jobName("build-app", 1, "test")
	if a == b {
		t.Fatalf("different indexes gave the same name: %q", a)
	}
}

// The Job controller copies the Job name into the Pod label
// batch.kubernetes.io/job-name, so it must be a valid DNS-1123 label (≤ 63 chars).
// jobName does not truncate: inputs that would give a longer name are
// rejected by the CEL rule on Pipeline before the controller sees them.
func TestJobName_Length(t *testing.T) {
	for _, tt := range jobNameCases {
		t.Run(tt.name, func(t *testing.T) {
			got := jobName(tt.pipeline, tt.index, tt.job)
			errs := validation.IsDNS1123Label(got)

			switch {
			case tt.valid && len(errs) > 0:
				t.Errorf("%q (%d chars) should be a valid Job name: %v", got, len(got), errs)
			case !tt.valid && len(errs) == 0:
				t.Errorf("%q (%d chars) should be invalid (> 63 chars), so the CRD rule must reject this input",
					got, len(got))
			}
		})
	}
}

// ---------- buildJob tests ----------

//it is better to let here on top the main tested functions and the helpers bellow - personal preference

// The Job gets the deterministic name, the Pipeline's namespace, and labels
// that link it back to its Pipeline. The Pod gets the same labels.
func TestBuildJob_Metadata(t *testing.T) {
	p := newPipeline("build-app", "compile", "test")

	job := buildJob(p, 1, newTemplate(step("run")))

	if want := jobName("build-app", 1, "test"); job.Name != want {
		t.Errorf("name = %q, want %q", job.Name, want)
	}
	if job.Namespace != "ci" {
		t.Errorf("namespace = %q, want %q", job.Namespace, "ci")
	}

	wantLabels := map[string]string{labelPipeline: "build-app", labelJobIndex: "1"}
	if !reflect.DeepEqual(job.Labels, wantLabels) {
		t.Errorf("job labels = %v, want %v", job.Labels, wantLabels)
	}
	if !reflect.DeepEqual(job.Spec.Template.Labels, wantLabels) {
		t.Errorf("pod labels = %v, want %v", job.Spec.Template.Labels, wantLabels)
	}
}

// Steps run in order: all but the last become init containers,
// the last one is the main container.
func TestBuildJob_StepsOrder(t *testing.T) {
	tests := []struct {
		name     string
		steps    []string
		wantInit []string
		wantMain string
	}{
		{"single step", []string{"run"}, nil, "run"},
		{"two steps", []string{"build", "test"}, []string{"build"}, "test"},
		{"three steps", []string{"checkout", "build", "test"}, []string{"checkout", "build"}, "test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			steps := make([]civ1alpha1.Step, 0, len(tt.steps))
			for _, s := range tt.steps {
				steps = append(steps, step(s))
			}

			pod := buildJob(newPipeline("p", "job"), 0, newTemplate(steps...)).Spec.Template.Spec

			var gotInit []string
			for _, c := range pod.InitContainers {
				gotInit = append(gotInit, c.Name)
			}
			if !reflect.DeepEqual(gotInit, tt.wantInit) {
				t.Errorf("init containers = %v, want %v", gotInit, tt.wantInit)
			}
			if len(pod.Containers) != 1 || pod.Containers[0].Name != tt.wantMain {
				t.Errorf("main containers = %v, want exactly [%s]", pod.Containers, tt.wantMain)
			}
		})
	}
}

// Every field of a Step is copied to its container.
func TestBuildJob_CopiesStepFields(t *testing.T) {
	s := civ1alpha1.Step{
		Name:    "build",
		Image:   "golang:1.24",
		Command: []string{"go"},
		Args:    []string{"build", "./..."},
		Env:     []corev1.EnvVar{{Name: "CGO_ENABLED", Value: "0"}},
	}

	c := buildJob(newPipeline("p", "job"), 0, newTemplate(s)).Spec.Template.Spec.Containers[0]

	if c.Name != s.Name || c.Image != s.Image {
		t.Errorf("name/image = %q/%q, want %q/%q", c.Name, c.Image, s.Name, s.Image)
	}
	if !reflect.DeepEqual(c.Command, s.Command) {
		t.Errorf("command = %v, want %v", c.Command, s.Command)
	}
	if !reflect.DeepEqual(c.Args, s.Args) {
		t.Errorf("args = %v, want %v", c.Args, s.Args)
	}
	if !reflect.DeepEqual(c.Env, s.Env) {
		t.Errorf("env = %v, want %v", c.Env, s.Env)
	}
}

// All steps share one emptyDir volume, mounted at the same path,
// and start in that folder.
func TestBuildJob_SharedWorkspace(t *testing.T) {
	pod := buildJob(newPipeline("p", "job"), 0,
		newTemplate(step("checkout"), step("build"), step("test"))).Spec.Template.Spec

	if len(pod.Volumes) != 1 || pod.Volumes[0].Name != workspaceVolume || pod.Volumes[0].EmptyDir == nil {
		t.Fatalf("volumes = %+v, want one emptyDir named %q", pod.Volumes, workspaceVolume)
	}

	for _, c := range allContainers(pod) {
		if c.WorkingDir != workspacePath {
			t.Errorf("%s: workingDir = %q, want %q", c.Name, c.WorkingDir, workspacePath)
		}
		want := []corev1.VolumeMount{{Name: workspaceVolume, MountPath: workspacePath}}
		if !reflect.DeepEqual(c.VolumeMounts, want) {
			t.Errorf("%s: volumeMounts = %v, want %v", c.Name, c.VolumeMounts, want)
		}
	}
}

// A failing step must fail the Job once: no Pod restarts, no Job retries.
func TestBuildJob_NoRetries(t *testing.T) {
	job := buildJob(newPipeline("p", "job"), 0, newTemplate(step("run")))

	if job.Spec.BackoffLimit == nil || *job.Spec.BackoffLimit != 0 {
		t.Errorf("backoffLimit = %v, want 0", job.Spec.BackoffLimit)
	}
	if rp := job.Spec.Template.Spec.RestartPolicy; rp != corev1.RestartPolicyNever {
		t.Errorf("restartPolicy = %q, want %q", rp, corev1.RestartPolicyNever)
	}
}

// Label values must also be valid, or the API server rejects the Job.
func TestBuildJob_LabelsAreValid(t *testing.T) {
	job := buildJob(newPipeline(strings.Repeat("p", 63), "job"), 0, newTemplate(step("run")))

	for k, v := range job.Labels {
		if errs := validation.IsValidLabelValue(v); len(errs) > 0 {
			t.Errorf("label %s=%q is invalid: %v", k, v, errs)
		}
	}
	if job.Labels[labelJobIndex] != strconv.Itoa(0) {
		t.Errorf("job index label = %q, want %q", job.Labels[labelJobIndex], "0")
	}
}

// ---------- helpers for buildJob ----------

// newPipeline returns a Pipeline with one PipelineJob per given job name.
func newPipeline(name string, jobNames ...string) *civ1alpha1.Pipeline {
	jobs := make([]civ1alpha1.PipelineJob, 0, len(jobNames))
	for _, j := range jobNames {
		jobs = append(jobs, civ1alpha1.PipelineJob{Name: j, TemplateRef: j + "-tpl"})
	}
	return &civ1alpha1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ci"},
		Spec:       civ1alpha1.PipelineSpec{Jobs: jobs},
	}
}

// newTemplate returns a template with the given steps.
func newTemplate(steps ...civ1alpha1.Step) *civ1alpha1.PipelineJobTemplate {
	return &civ1alpha1.PipelineJobTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "tpl", Namespace: "ci"},
		Spec:       civ1alpha1.PipelineJobTemplateSpec{Steps: steps},
	}
}

func step(name string) civ1alpha1.Step {
	return civ1alpha1.Step{Name: name, Image: "busybox:1.36"}
}

// allContainers returns init containers followed by regular containers,
// i.e. the steps in execution order.
func allContainers(spec corev1.PodSpec) []corev1.Container {
	return append(append([]corev1.Container{}, spec.InitContainers...), spec.Containers...)
}
