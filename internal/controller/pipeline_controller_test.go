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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	civ1alpha1 "github.com/MateusDSouza/k8s-pipeline-controller/api/v1alpha1"
)

var _ = Describe("Pipeline Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName      = "test-resource"
			resourceNamespace = "default"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}
		pipeline := &civ1alpha1.Pipeline{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Pipeline")
			err := k8sClient.Get(ctx, typeNamespacedName, pipeline)
			if err != nil && apierrors.IsNotFound(err) {
				resource := &civ1alpha1.Pipeline{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: resourceNamespace,
					},
					Spec: civ1alpha1.PipelineSpec{
						Jobs: []civ1alpha1.PipelineJob{
							{Name: "build", TemplateRef: "build"},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &civ1alpha1.Pipeline{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Pipeline")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &PipelineReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

// These tests talk only to the API server: they check the CRD validation
// rules, not the reconciler.
var _ = Describe("Pipeline validation", func() {
	ctx := context.Background()

	// pipelineWith builds a Pipeline with a single job.
	pipelineWith := func(name, job string) *civ1alpha1.Pipeline {
		return &civ1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec: civ1alpha1.PipelineSpec{
				Jobs: []civ1alpha1.PipelineJob{{Name: job, TemplateRef: "tpl"}},
			},
		}
	}

	It("rejects a pipeline whose Job names would exceed 63 characters", func() {
		p := pipelineWith("payments-service-release-candidate-build", "integration-tests-against-staging")

		err := k8sClient.Create(ctx, p)

		Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected a validation error, got: %v", err)
		Expect(err.Error()).To(ContainSubstring("pipeline name + job name must be at most 59 characters"))
	})

	It("accepts the longest names the rule allows - edge case", func() {
		p := pipelineWith(strings.Repeat("p", 30), strings.Repeat("j", 29)) // 59 chars together

		Expect(k8sClient.Create(ctx, p)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, p)).To(Succeed()) })
	})
})
