// this file was not created by the scaffolding of kubebuilder, but I will let the same copyright here!
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
	"fmt"
	"strconv"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	civ1alpha1 "github.com/MateusDSouza/k8s-pipeline-controller/api/v1alpha1" // ---> importing the api types
)

const (
	labelPipeline   = "ci.example.com/pipeline"
	labelJobIndex   = "ci.example.com/job-index"
	workspaceVolume = "workspace"
	workspacePath   = "/workspace" // ---> same enum created in the Phase for the types
)

// the jobName function is deterministic, so the controller always finds a Job it created before.
func jobName(pipeline string, index int, job string) string {
	return fmt.Sprintf("%s-%d-%s", pipeline, index, job)
}

// buildJob turns job number `index` of a pipeline, plus its template, into a Kubernetes Job.
// All steps but the last become init containers: Kubernetes runs them in order and stops
// at the first failure. The last step is the main container. Steps share a /workspace volume.
func buildJob(p *civ1alpha1.Pipeline, index int, tmpl *civ1alpha1.PipelineJobTemplate) *batchv1.Job { // this function takes references for the pipeline and pipeline job template and returns a kubernetes job.
	containers := make([]corev1.Container, 0, len(tmpl.Spec.Steps)) // here we are creating a list with the size of the number of steps based on the reference of a PipelineJobTemplate -> this object knows how many steps a job has.
	for _, s := range tmpl.Spec.Steps {                             // as the step is defined as a struct, we iterate over the elements but ignoring the key and using the content of each step.
		containers = append(containers, corev1.Container{ // with the content of each step we then create the container using its attributes and store this container on the container list
			Name:         s.Name,
			Image:        s.Image,
			Command:      s.Command, // assigning values from the step to the container
			Args:         s.Args,
			Env:          s.Env,
			WorkingDir:   workspacePath,                                                           // all steps should start from this folder and will share the resources present there
			VolumeMounts: []corev1.VolumeMount{{Name: workspaceVolume, MountPath: workspacePath}}, // here we see what is mounted
		})
	}
	last := len(containers) - 1 // MinItems=1 in the CRD guarantees at least one step

	labels := map[string]string{
		labelPipeline: p.Name, // use the name of the pipeline to label this job so we know that a given executed kubernetes job belongs to that given pipeline
		labelJobIndex: strconv.Itoa(index),
	}

	return &batchv1.Job{ // now using the 3 local variables containers, last and labels we can then return a kubernetes job created with the different steps and representing a PipelineJob
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName(p.Name, index, p.Spec.Jobs[index].Name),
			Namespace: p.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: ptr.To[int32](0), // a failing step fails the job: no automatic retries
			Template: corev1.PodTemplateSpec{ // use the pod template and the local variables to create a pod and execute the jobs
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					RestartPolicy:  corev1.RestartPolicyNever,
					InitContainers: containers[:last], // this ensures that this pod will execute all the steps of the job
					Containers:     containers[last:], // the case of only 1 step per job - remember about linked lists
					Volumes: []corev1.Volume{{
						Name:         workspaceVolume, // the pod should keep a mounted volume so that all containers share this resource - they all share the same name defined on top as a constant
						VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
					}},
				},
			},
		},
	}
}
