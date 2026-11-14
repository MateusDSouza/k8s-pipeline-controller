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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	civ1alpha1 "github.com/MateusDSouza/k8s-pipeline-controller/api/v1alpha1"

	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
func (r *PipelineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 1. Read the Pipeline that triggered this reconcile.
	var p civ1alpha1.Pipeline
	if err := r.Get(ctx, req.NamespacedName, &p); err != nil { // if we cannot get this pipeline from the k8s api, it means that it was deleted and it shouldnt do anything. This is tested as the first thing in the reconcile loop.
		// Deleted: nothing to do. Its Jobs are garbage-collected through ownerReferences.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 2. only the first job after will add sequencing and status.
	index := 0
	var tmpl civ1alpha1.PipelineJobTemplate
	key := types.NamespacedName{Namespace: p.Namespace, Name: p.Spec.Jobs[index].TemplateRef}
	if err := r.Get(ctx, key, &tmpl); err != nil {
		return ctrl.Result{}, err // e.g. template missing: requeued with backoff
	}

	// 3. Build the Job and make the Pipeline its owner
	job := buildJob(&p, index, &tmpl) // call the function in job_builder.go and create a k8s job
	if err := controllerutil.SetControllerReference(&p, job, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	// 4. Create it. "Already exists" means an earlier reconcile did it
	if err := r.Create(ctx, job); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	log.Info("created job", "job", job.Name, "template", tmpl.Name)
	return ctrl.Result{}, nil
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
