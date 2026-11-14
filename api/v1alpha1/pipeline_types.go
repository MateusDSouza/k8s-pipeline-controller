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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// PipelineJob is one entry of the pipeline: a named reference to a template.
type PipelineJob struct {
	// name identifies the job within the pipeline and is used to name
	// the Kubernetes Job. Must be a DNS-1123 label, unique in the pipeline.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	Name string `json:"name"`

	// templateRef is the name of a PipelineJobTemplate in the same namespace.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-.a-z0-9]*[a-z0-9])?$`
	TemplateRef string `json:"templateRef"`
}

// PipelineSpec lists the jobs to run, in order.
// Each job starts only after the previous one succeeded.
type PipelineSpec struct {
	// jobs run sequentially in the order listed.
	// MaxItems keeps the index at 2 digits; the Job name length rule on Pipeline depends on it -> check internal/controller/job_builder_test.go
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=50
	// +listType=map
	// +listMapKey=name
	Jobs []PipelineJob `json:"jobs"`
}

// Phase is the lifecycle state of a pipeline or of one of its jobs.
// +kubebuilder:validation:Enum=Pending;Running;Succeeded;Failed;Skipped
type Phase string

const (
	PhasePending   Phase = "Pending"
	PhaseRunning   Phase = "Running"
	PhaseSucceeded Phase = "Succeeded"
	PhaseFailed    Phase = "Failed"
	PhaseSkipped   Phase = "Skipped"
)

// JobStatus reports the state of one pipeline job.
type JobStatus struct {
	// name matches the job name in spec.jobs.
	Name string `json:"name"`

	// jobName is the name of the Kubernetes Job created for this job.
	// +optional
	JobName string `json:"jobName,omitempty"`

	// phase is the current state of this job.
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// startTime is when the Kubernetes Job was created.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// completionTime is when the job succeeded or failed.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
}

// PipelineStatus is the observed state of a Pipeline. Written by the controller only.
type PipelineStatus struct {
	// phase summarizes the pipeline: Pending, Running, Succeeded or Failed.
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// jobs reports the state of each job, in spec order.
	// +listType=map
	// +listMapKey=name
	// +optional
	Jobs []JobStatus `json:"jobs,omitempty"`

	// startTime is when the first job was created.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// completionTime is when the pipeline reached Succeeded or Failed.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// conditions give details on the pipeline state, e.g. why it failed.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=pl
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Started",type=date,JSONPath=`.status.startTime`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:validation:XValidation:rule="self.spec.jobs.all(j, self.metadata.name.size() + j.name.size() + 4 <= 63)",message="pipeline name + job name must be at most 59 characters"

// Pipeline runs a sequence of jobs, each built from a PipelineJobTemplate.
type Pipeline struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the jobs to run. Immutable after creation.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="pipeline spec is immutable"
	Spec PipelineSpec `json:"spec"`

	// status defines the observed state of Pipeline
	// +optional
	Status PipelineStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// PipelineList contains a list of Pipeline
type PipelineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Pipeline `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &Pipeline{}, &PipelineList{})
		return nil
	})
}
