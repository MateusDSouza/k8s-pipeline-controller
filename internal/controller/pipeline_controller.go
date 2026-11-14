/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"strconv"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	civ1alpha1 "github.com/MateusDSouza/k8s-pipeline-controller/api/v1alpha1"
)

// PipelineReconciler reconciles a Pipeline object
type PipelineReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=ci.example.com,resources=pipelines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ci.example.com,resources=pipelines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ci.example.com,resources=pipelines/finalizers,verbs=update
// +kubebuilder:rbac:groups=ci.example.com,resources=pipelinejobtemplates,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;delete

// every r.Get, r.List and r.Create in this function is a request to the API server made as your operator ServiceAccount. RBAC denies everything by default, so each kind of call needs a rule.
func (r *PipelineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) { // this is a function from the PipelineReconciler type
	log := logf.FromContext(ctx)

	// 1. Read the pipeline.
	var p civ1alpha1.Pipeline
	if err := r.Get(ctx, req.NamespacedName, &p); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if p.Status.Phase == civ1alpha1.PhaseSucceeded || p.Status.Phase == civ1alpha1.PhaseFailed { //In case the pipeline fail returns
		return ctrl.Result{}, nil // finished: nothing more to do
	}
	before := p.DeepCopy()

	// 2. Observe: the Jobs this pipeline owns, by index.
	var jobList batchv1.JobList
	if err := r.List(ctx, &jobList, client.InNamespace(p.Namespace),
		client.MatchingLabels{labelPipeline: p.Name}); err != nil {
		return ctrl.Result{}, fmt.Errorf("list jobs: %w", err) // we delegate
	}
	jobs := map[int]*batchv1.Job{}
	states := map[int]jobState{}
	for i := range jobList.Items {
		j := &jobList.Items[i]
		idx, err := strconv.Atoi(j.Labels[labelJobIndex])
		if err != nil || !metav1.IsControlledBy(j, &p) {
			continue // not ours, e.g. left over from a deleted pipeline with the same name
		}
		jobs[idx] = j
		states[idx] = stateOf(j)
	}

	// 3. Decide and act.
	result := ctrl.Result{}
	next := decide(len(p.Spec.Jobs), states)
	switch next.kind {
	case actionCreateJob:
		pj := p.Spec.Jobs[next.index]
		var tmpl civ1alpha1.PipelineJobTemplate
		err := r.Get(ctx, types.NamespacedName{Namespace: p.Namespace, Name: pj.TemplateRef}, &tmpl)
		switch {
		case apierrors.IsNotFound(err):
			phase := p.Status.Phase
			if phase == "" {
				phase = civ1alpha1.PhasePending
			}
			setPhase(&p, phase, "TemplateNotFound",
				fmt.Sprintf("template %q for job %q not found", pj.TemplateRef, pj.Name))
			result = ctrl.Result{RequeueAfter: 15 * time.Second} // check again later
		case err != nil:
			return ctrl.Result{}, fmt.Errorf("get template %q: %w", pj.TemplateRef, err)
		default:
			job := buildJob(&p, next.index, &tmpl)
			if err := controllerutil.SetControllerReference(&p, job, r.Scheme); err != nil {
				return ctrl.Result{}, fmt.Errorf("set owner reference on job %s: %w", job.Name, err)
			}
			if err := r.Create(ctx, job); err != nil {
				if !apierrors.IsAlreadyExists(err) {
					return ctrl.Result{}, fmt.Errorf("create job %s: %w", job.Name, err)
				}
			} else {
				log.Info("created job", "job", job.Name, "index", next.index, "template", tmpl.Name)
			}
			setPhase(&p, civ1alpha1.PhaseRunning, "Running", fmt.Sprintf("job %q started", pj.Name))
		}
	case actionWait:
		setPhase(&p, civ1alpha1.PhaseRunning, "Running",
			fmt.Sprintf("job %q running", p.Spec.Jobs[next.index].Name))
	case actionFail:
		setPhase(&p, civ1alpha1.PhaseFailed, "JobFailed",
			fmt.Sprintf("job %q failed", p.Spec.Jobs[next.index].Name))
	case actionSucceed:
		setPhase(&p, civ1alpha1.PhaseSucceeded, "Succeeded", "all jobs succeeded")
	}

	// 4. Report: write status only if it changed, then log/count the transition once it's saved.
	p.Status.Jobs = jobStatuses(&p, jobs)
	if !equality.Semantic.DeepEqual(before.Status, p.Status) {
		if err := r.Status().Patch(ctx, &p, client.MergeFrom(before)); err != nil {
			return ctrl.Result{}, fmt.Errorf("update status: %w", err)
		}
		recordTransition(log, &p, before.Status.Phase) // in this point after a transition occurs we send the record to the variables created in metrics/metrics.go!
	}
	return result, nil
}

// This function connects the reconciler to the manager and sets when Reconcile should run. It uses the builder pattern: each method adds a setting and returns the builder
// SetupWithManager sets up the controller with the Manager.
func (r *PipelineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr). // start building a controller in this manager
							For(&civ1alpha1.Pipeline{}). // main resource
							Owns(&batchv1.Job{}).        // resources it creates
							Named("pipeline").           // controller name
							Complete(r)                  // build it, with r as the reconciler
}
