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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	civ1alpha1 "github.com/MateusDSouza/k8s-pipeline-controller/api/v1alpha1"
)

var _ = Describe("Pipeline with a missing template", func() {
	It("waits with TemplateNotFound, creates no Job, and runs once the template exists", func() {
		r := &PipelineReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		key := types.NamespacedName{Name: "missing-template", Namespace: "default"}
		req := reconcile.Request{NamespacedName: key}

		p := &civ1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace},
			Spec: civ1alpha1.PipelineSpec{
				Jobs: []civ1alpha1.PipelineJob{{Name: "build", TemplateRef: "created-later"}},
			},
		}
		Expect(k8sClient.Create(ctx, p)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, p) })

		jobs := func() []batchv1.Job {
			var list batchv1.JobList
			Expect(k8sClient.List(ctx, &list, client.InNamespace(key.Namespace),
				client.MatchingLabels{labelPipeline: key.Name})).To(Succeed())
			return list.Items
		}

		By("waiting while the template is missing")
		res, err := r.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred()) // a missing template is not an error: no exponential backoff
		Expect(res.RequeueAfter).To(Equal(15 * time.Second))

		Expect(k8sClient.Get(ctx, key, p)).To(Succeed())
		Expect(p.Status.Phase).To(Equal(civ1alpha1.PhasePending))
		ready := meta.FindStatusCondition(p.Status.Conditions, conditionReady)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Reason).To(Equal("TemplateNotFound"))
		Expect(jobs()).To(BeEmpty())

		By("creating the template")
		tmpl := &civ1alpha1.PipelineJobTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: "created-later", Namespace: key.Namespace},
			Spec: civ1alpha1.PipelineJobTemplateSpec{
				Steps: []civ1alpha1.Step{{Name: "noop", Image: "busybox:1.37"}},
			},
		}
		Expect(k8sClient.Create(ctx, tmpl)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, tmpl) })

		By("the next reconcile (the 15 s requeue) creates the Job")
		_, err = r.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())
		Expect(jobs()).To(HaveLen(1))

		_, err = r.Reconcile(ctx, req) // the Job exists and hasn't finished -> Running
		Expect(err).NotTo(HaveOccurred())
		Expect(k8sClient.Get(ctx, key, p)).To(Succeed())
		Expect(p.Status.Phase).To(Equal(civ1alpha1.PhaseRunning))
	})
})
