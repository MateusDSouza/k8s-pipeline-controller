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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus/testutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	civ1alpha1 "github.com/MateusDSouza/k8s-pipeline-controller/api/v1alpha1"
	"github.com/MateusDSouza/k8s-pipeline-controller/internal/metrics"
)

// staleClient returns an old copy of the Pipeline on Get, like an informer cache that hasn't
// received the latest status update yet. Everything else goes to the real API server.
type staleClient struct {
	client.Client
	stale *civ1alpha1.Pipeline
}

func (c staleClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if p, ok := obj.(*civ1alpha1.Pipeline); ok {
		c.stale.DeepCopyInto(p)
		return nil
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

var _ = Describe("Pipeline read from a stale cache", func() {
	It("does not record the same transition twice", func() {
		key := types.NamespacedName{Name: "stale-cache", Namespace: "default"}
		req := reconcile.Request{NamespacedName: key}

		tmpl := &civ1alpha1.PipelineJobTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: "stale-cache-noop", Namespace: key.Namespace},
			Spec: civ1alpha1.PipelineJobTemplateSpec{
				Steps: []civ1alpha1.Step{{Name: "noop", Image: "busybox:1.37"}},
			},
		}
		Expect(k8sClient.Create(ctx, tmpl)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, tmpl) })

		p := &civ1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace},
			Spec: civ1alpha1.PipelineSpec{
				Jobs: []civ1alpha1.PipelineJob{{Name: "build", TemplateRef: tmpl.Name}},
			},
		}
		Expect(k8sClient.Create(ctx, p)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, p) })

		By("keeping a copy from before the first reconcile (phase empty, old resourceVersion)")
		stale := &civ1alpha1.Pipeline{}
		Expect(k8sClient.Get(ctx, key, stale)).To(Succeed())

		By("the first reconcile creates the Job and moves the pipeline to Running")
		started := testutil.ToFloat64(metrics.PipelinesStarted)
		fresh := &PipelineReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := fresh.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())
		Expect(testutil.ToFloat64(metrics.PipelinesStarted) - started).To(Equal(1.0))

		By("a second reconcile that still sees the old copy gets a conflict and records nothing")
		behind := &PipelineReconciler{Client: staleClient{Client: k8sClient, stale: stale}, Scheme: k8sClient.Scheme()}
		res, err := behind.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())                  // a conflict is expected, not an error
		Expect(res.RequeueAfter).To(BeNumerically(">", 0)) // it comes back with fresh data
		Expect(testutil.ToFloat64(metrics.PipelinesStarted) - started).To(Equal(1.0))
	})
})
