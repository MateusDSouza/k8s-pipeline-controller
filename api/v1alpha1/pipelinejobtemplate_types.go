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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Step is one container. Steps run in order; the first failure stops the job.
type Step struct {
	// name identifies the step and becomes the init container name.
	// kubernetes requires container names to be lowercase and alphanumeric, so we delegate to the CRD to validate this before the controller tries to create a container with a wrong name and gets rejected by the API.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	Name string `json:"name"`

	// image is the container image that runs this step.
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// command overrides the image entrypoint.
	// +optional
	Command []string `json:"command,omitempty"`

	// args are passed to the command.
	// +optional
	Args []string `json:"args,omitempty"`

	// env sets environment variables for this step.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`
}

// PipelineJobTemplateSpec is a reusable list of steps that pipelines reference.
type PipelineJobTemplateSpec struct {
	// steps run in order inside a single Kubernetes Job.
	// +kubebuilder:validation:MinItems=1
	// +listType=map
	// +listMapKey=name
	Steps []Step `json:"steps"`
}

// PipelineJobTemplateStatus defines the observed state of PipelineJobTemplate.
type PipelineJobTemplateStatus struct {
	// conditions represent the current state of the PipelineJobTemplate resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=pjt

// PipelineJobTemplate is a reusable job definition shared across pipelines.
type PipelineJobTemplate struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the steps of the job
	// +required
	Spec PipelineJobTemplateSpec `json:"spec"`

	// status defines the observed state of PipelineJobTemplate
	// +optional
	Status PipelineJobTemplateStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// PipelineJobTemplateList contains a list of PipelineJobTemplate
type PipelineJobTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []PipelineJobTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &PipelineJobTemplate{}, &PipelineJobTemplateList{})
		return nil
	})
}
