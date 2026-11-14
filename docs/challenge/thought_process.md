# My thought process

The challenge itself will test my habilities for the creation of operators for kubernetes. For this I will need to build a simple kubernetes operator that should run a sort of CICD pipeline.
We see that the operator is written in golang because it is the main language for cloud native solution - k8s, terraform... So that the technology!

Now, about the kubebuilder this is the default framework used and maintained for the creation of operators - it already gives me the scaffolding and libraries so I dont build them from scratch. Do not reinvent the wheel.

Before start coding directly it pays off to understand better what we have on the challenge markdown file:

The controller will be responsible for two CDR:

- `PipelineJobTemplate`

   - Interface to run an arbitrary number of jobs steps inside containers.

- `Pipeline`

   - A ordered composition of PipelineJobs

Bringing this to my closest reality, azure devops:

| Azure DevOps                                                           | Challenge                                                                                    |
| ---------------------------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| Pipeline (`azure-pipelines.yml`)                                       | `Pipeline` resource                                                                          |
| Job (a group of steps running on one agent)                            | One entry in `Pipeline.spec.jobs`, turned into one Kubernetes Job                            |
| Steps (`- script:` / `- task:`) run one after another in the job       | `steps` in a `PipelineJobTemplate`: init containers run in order                             |
| Job/step templates (`template: build-job.yml`) reused across pipelines | `PipelineJobTemplate`: “a reusable resource shared across pipelines”                         |
| Agent that picks up a job and runs it                                  | Controller creates the Job, and Kubernetes schedules its pod on a node                       |
| Agent workspace shared by the job’s steps ($(Pipeline.Workspace))      | An emptyDir volume shared by the init containers                                             |
| dependsOn / job order                                                  | Controller starts job N+1 only after job N succeeded                                         |
| Run status: queued / running / succeeded / failed                      | Pipeline.status.phase                                                                        |
| A failing step fails the job, and the run stops                        | A failing init container fails the Job → the Pipeline goes Failed and later jobs never start |

One difference is that in azure devops if the jobs doesnt have any dependency they can run in parallel in different agents, but in this case we have the sequential execution and will be kept like this for the simplicity and time available.

Lets start with the local development and clear up the path for the operator. Once this is done we can improve best
practices, observability, deployment, integration tests and end2end tests using kind.

## 1 - Starting the project

```sh
# Here we initiate the scaffolding with kubebuilder telling that the main module is this git repo.
kubebuilder init --domain example.com --repo github.com/MateusDSouza/k8s-pipeline-controller

# Create the Pipeline resource
kubebuilder create api --group ci --version v1alpha1 --kind Pipeline --resource --controller

# Create the PipelineJobTemplate resource - controller is false here because no controller runs this.
kubebuilder create api --group ci --version v1alpha1 --kind PipelineJobTemplate --resource --controller=false

# Execute make commands to generate CRD YAML (config/crd/bases/) and RBAC rules (config/rbac/)
make manifests generate

# Runs the test suite + download envtest binaries 
make test
```


Now, the scaffold is created I will merge this as a PR. For this I installed gh so that I can merge this from the command line.

```sh
gh pr create --base main --head scaffold --title "chore: Scaffold project with kubebuilder" \
  --body "kubebuilder init + Pipeline (with controller) and PipelineJobTemplate (no controller) APIs. make test initialized and passed."

# Using the squash to keep the main clean and organized.
gh pr merge --squash --delete-branch
```

## 2. API-Types

By analyzing the folders and files created, in the files [text](../../api/v1alpha1/pipeline_types.go) and [text](../../api/v1alpha1/pipelinejobtemplate_types.go). Each command from kubebuilder create one of those *_types.go files.
In the content of the files we can see that we have two main struct types that should be edited for the creation of our CRDs in the kubernetes.
So, first we need to implement those from the framework - controller-gen will read them and it generates the CRDs schemas. The controller of the cluster then will use those structs to read and write objects to the cluster.

- Explaining the details of what changed and why.
Those annotations for the kubebuilder that work with the makefile and so on... what is the spec and the status, the auxiliary resources created - this will be super important for the interview

- First of all to understand how the golang language works, each type xxxx struct {} represents a dataclass -> a class who only has a skeleton with the attributes (think as a datastructure - or a typed assemble of data). In this specific case each value has a tag because of kubebuilder - this is an extra info for the framework. Here we have an example of this:
```sh
Name     string     `json:"name"`
│        │          └─ tag: extra info for libraries (here: the field is called "name" in JSON/YAML)
│        └─ type of the field
└─ field name (capital letter = exported, visible outside the package)
```

- After changing those two templates we can see that the file [text](../../api/v1alpha1/zz_generated.deepcopy.go) has some error because of the changes, so now we run the make comands as instructed in the comments of those files.
Example of the comments:
```sh
// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
// Important: Run "make" to regenerate code after modifying this file

mateus.desouza@SDGESP-6YGQ97WG k8s-pipeline-controller % make manifests generate      
"/Users/mateus.desouza/Desktop/k8s-pipeline-controller/bin/controller-gen" rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases
"/Users/mateus.desouza/Desktop/k8s-pipeline-controller/bin/controller-gen" object:headerFile="hack/boilerplate.go.txt",year=2026 paths="./..."
mateus.desouza@SDGESP-6YGQ97WG k8s-pipeline-controller % 
```


After generating the manifests, we then test the code running 'make test' to see the results and what we got is the following error:

```sh
mateus.desouza@SDGESP-6YGQ97WG k8s-pipeline-controller % make test

"/Users/mateus.desouza/Desktop/k8s-pipeline-controller/bin/controller-gen" rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases
"/Users/mateus.desouza/Desktop/k8s-pipeline-controller/bin/controller-gen" object:headerFile="hack/boilerplate.go.txt",year=2026 paths="./..."
go fmt ./...
api/v1alpha1/pipeline_types.go
go vet ./...
Setting up envtest binaries for Kubernetes version 1.37...
/Users/mateus.desouza/Desktop/k8s-pipeline-controller/bin/k8s/1.37.0-darwin-arm64KUBEBUILDER_ASSETS="/Users/mateus.desouza/Desktop/k8s-pipeline-controller/bin/k8s/1.37.0-darwin-arm64" go test $(go list ./... | grep -v /e2e) -coverprofile cover.out
        github.com/MateusDSouza/k8s-pipeline-controller/api/v1alpha1            coverage: 0.0% of statements
        github.com/MateusDSouza/k8s-pipeline-controller/cmd             coverage: 0.0% of statements
Running Suite: Controller Suite - /Users/mateus.desouza/Desktop/k8s-pipeline-controller/internal/controller
===========================================================================================================
Random Seed: 1794604838

Will run 1 of 1 specs
------------------------------
• [FAILED] [2.008 seconds]
Pipeline Controller When reconciling a resource [BeforeEach] should successfully reconcile the resource
  [BeforeEach] /Users/mateus.desouza/Desktop/k8s-pipeline-controller/internal/controller/pipeline_controller_test.go:48
  [It] /Users/mateus.desouza/Desktop/k8s-pipeline-controller/internal/controller/pipeline_controller_test.go:72

  Timeline >>
  STEP: creating the custom resource for the Kind Pipeline @ 11/13/26 22:20:42.015
  [FAILED] in [BeforeEach] - /Users/mateus.desouza/Desktop/k8s-pipeline-controller/internal/controller/pipeline_controller_test.go:59 @ 11/13/26 22:20:44.021
  [FAILED] in [AfterEach] - /Users/mateus.desouza/Desktop/k8s-pipeline-controller/internal/controller/pipeline_controller_test.go:67 @ 11/13/26 22:20:44.023
  << Timeline

  [FAILED] Expected success, but got an error:
      <*errors.StatusError | 0xe88079c8320>: 
      Pipeline.ci.example.com "test-resource" is invalid: [spec.jobs: Required value, <nil>: Invalid value: null: some validation rules were not checked because the object was invalid; correct the existing errors to complete validation]
      {
          ErrStatus: {
              TypeMeta: {Kind: "", APIVersion: ""},
              ListMeta: {
                  SelfLink: "",
                  ResourceVersion: "",
                  Continue: "",
                  RemainingItemCount: nil,
                  ShardInfo: nil,
              },
              Status: "Failure",
              Message: "Pipeline.ci.example.com \"test-resource\" is invalid: [spec.jobs: Required value, <nil>: Invalid value: null: some validation rules were not checked because the object was invalid; correct the existing errors to complete validation]",
              Reason: "Invalid",
              Details: {
                  Name: "test-resource",
                  Group: "ci.example.com",
                  Kind: "Pipeline",
                  UID: "",
                  Causes: [
                      {
                          Type: "FieldValueRequired",
                          Message: "Required value",
                          Field: "spec.jobs",
                      },
                      {
                          Type: "FieldValueInvalid",
                          Message: "Invalid value: null: some validation rules were not checked because the object was invalid; correct the existing errors to complete validation",
                          Field: "<nil>",
                      },
                  ],
                  RetryAfterSeconds: 0,
              },
              Code: 422,
          },
      }
  In [BeforeEach] at: /Users/mateus.desouza/Desktop/k8s-pipeline-controller/internal/controller/pipeline_controller_test.go:59 @ 11/13/26 22:20:44.021

  There were additional failures detected.  To view them in detail run ginkgo -vv
------------------------------

Summarizing 1 Failure:
  [FAIL] Pipeline Controller When reconciling a resource [BeforeEach] should successfully reconcile the resource
  /Users/mateus.desouza/Desktop/k8s-pipeline-controller/internal/controller/pipeline_controller_test.go:59

Ran 1 of 1 Specs in 6.416 seconds
FAIL! -- 0 Passed | 1 Failed | 0 Pending | 0 Skipped
--- FAIL: TestControllers (6.42s)
FAIL
coverage: 0.0% of statements
FAIL    github.com/MateusDSouza/k8s-pipeline-controller/internal/controller     7.323s
        github.com/MateusDSouza/k8s-pipeline-controller/test/utils              coverage: 0.0% of statements
FAIL
make: *** [test] Error 1
mateus.desouza@SDGESP-6YGQ97WG k8s-pipeline-controller % 
```
Why this is failing -> The initial scaffolding in [text](../../internal/controller/pipeline_controller_test.go) was creating a pipeline with an empty spec for the test. But in the implementation we have that 'jobs' is a required field.
As I didnt touch any test created by the kubebuilder, so the envtest server refuses it, just like kubectl apply would.
After adding the Spec needed in the object, we have the result of the tests:

```sh
mateus.desouza@SDGESP-6YGQ97WG k8s-pipeline-controller % make test

"/Users/mateus.desouza/Desktop/k8s-pipeline-controller/bin/controller-gen" rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases
"/Users/mateus.desouza/Desktop/k8s-pipeline-controller/bin/controller-gen" object:headerFile="hack/boilerplate.go.txt",year=2026 paths="./..."
go fmt ./...
go vet ./...
Setting up envtest binaries for Kubernetes version 1.37...
/Users/mateus.desouza/Desktop/k8s-pipeline-controller/bin/k8s/1.37.0-darwin-arm64KUBEBUILDER_ASSETS="/Users/mateus.desouza/Desktop/k8s-pipeline-controller/bin/k8s/1.37.0-darwin-arm64" go test $(go list ./... | grep -v /e2e) -coverprofile cover.out
        github.com/MateusDSouza/k8s-pipeline-controller/api/v1alpha1            coverage: 0.0% of statements
        github.com/MateusDSouza/k8s-pipeline-controller/cmd             coverage: 0.0% of statements
ok      github.com/MateusDSouza/k8s-pipeline-controller/internal/controller     6.748s  coverage: 66.7% of statements
        github.com/MateusDSouza/k8s-pipeline-controller/test/utils              coverage: 0.0% of statements
mateus.desouza@SDGESP-6YGQ97WG k8s-pipeline-controller % 
```

Those are base tests for the implementation of the API types.

### Changes applied in the kubebuilder generated files

The first thing done is to work on the file [text](../../api/v1alpha1/pipelinejobtemplate_types.go), where we need to create a template for the pipeline job. We need to understand that for this implementation, we do not care about the _Status struct. Why this?
When we run from the framework `kubebuilder create api --group ci --version v1alpha1 --kind PipelineJobTemplate --resource --controller=false` this flag `controller=false`makes that this type will not be carrying a status inside the k8s controller, saying so, this resource will not be managed the same way a Pipeline type would.
To clarify this, lets think about how the kubernetes controller works: the controller check 2 things:
   - What is the actual state of the objects that k8s is managing? This is done by controlling the "Status".
   - What the kubernetes administrator is setting for this same object? This is done by controlling the "Spec".
So, we can see that the controller works by having his `Reconcile loop` checking what we have deployed in the infrastructure against what was declared in the manifest - so thats why when we change the Spec and apply those changes, the controller check this against what is present in the cluster and apply the changes with pertinent.

Going back to our situation for this lab, we can see that for PipelineJobTemplate, we need a Job object that is composed by different stages, but the object that would be managed by the kubernetes is not this small job part, but the whole Pipeline object composed by different Jobs. Hence, the PipelineJobTemplete needs a Spec, but we delegate the status check for the Pipeline!

Other important point to observe is that because we are using this framework - kubebuilder - we have several markers that are similar to comments, but rather they are used by the tool controller-gen to read the changes that we apply in the _types.go files and add a new functionalities related to the object that they have right below. So, the comments matters and the sign + identifies the markers. Those can be related to annotations in Spring or decorators in python frameworks.

Why it is important to use those markers rather than use go functions and create logics to check the right usage of those types when templating???? This design enforced by the tool controller-gen, allow us to validate the objects in the API server of kubernetes before the controller even sees the object. Imagine that the specs now generated from those types contains strict rules that the API will validate before reaching the controller - this allow us to enforce that Jobs is never empty (avoid the creatio of an empty pipeline) and that all steps are unique (avoid creating repetition of sequencial tasks). Marks are useful and support us by delegating this safety seat belt to the controller-gen creating the CRDs.


Going back for our implementation:

This is the definition of a step. Remember, Jobs are composed by steps. Pay attention on the markers and the fields - every step needs to have an name and an image in which it runs - those fields in the yaml have a primitive go type - in this case a string.
To support the usage of different environments for the step we use the library `corev1`.
```go
// Step is one container. Steps run in order; the first failure stops the job.
type Step struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// +optional
	Command []string `json:"command,omitempty"`

	// +optional
	Args []string `json:"args,omitempty"`

	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`
}
```
As you can see bellow, the PipelineJobTemplateSpec becomes composable and clear - it is composed by an 'array' of steps that were defined above. As those are in the same file, there is no need for import of different modules.
Most of the work was done already by the template. As `controller=false` we can see that PipelineJobTemplateStatus is implemented for the API of this resource, but this object will not have a reconcilier and wont be registered in the main.

```go
// PipelineJobTemplateSpec is a reusable list of steps that pipelines reference.
type PipelineJobTemplateSpec struct {
	// +kubebuilder:validation:MinItems=1
	// +listType=map
	// +listMapKey=name
	Steps []Step `json:"steps"`
}
```

Moving forward, now we can see that we have the PipelineJobTemplate resource definition, already generated by the kubebuilder:
```go
type PipelineJobTemplate struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the steps of the job
	// +required
	Spec PipelineJobTemplateSpec `json:"spec"`
}
```

Interesting to remark that `// +kubebuilder:resource:shortName=pjt` allows us to reach from this resource using `kubectl get`.

Also we have in the end of the file a List type for PipelineJobTemplate object. Why this? Because when we get all the objects from the k8s api for the type of a PipelineJobTemplate we want to get a view of them as a list of objects. This comes by default from kubebuilder.


- Looking now to the main file of this section [text](../../api/v1alpha1/pipeline_types.go), here the same analogy was applied but also we need to have a Status object to kept the track by the controller. Kubebuilder already gave us the skeleton of PipelineStatus and PipelineSpec - just need to implement it.
A pipeline job is an implementation of a PipelineJobTemplate, which is composed by different unique steps, all developed inside PipelineJobTemplate_types.go. So inside of a PipelineSpec, what we expect is to have a sequence of PipelineJobs.
Each Pipeline has its own status - this is achieved by the enum construction in golang -> const + Phase.
For every job that runs in the pipeline we have a JobStatus that gives information about the job at the moment of that execution, that why we have the class JobStatus.
The last bit is the Pipeline status that has a JobStatus at it. Look that JobStatus and PipelineStatus have a Phase type - because during the execution of a Pipeline we can have a job in Pending meanwhile the pipeline is in Running.

With those two implemented we can sketch their relationship with the following graphic:
```sh
Pipeline
├── spec: PipelineSpec                  ← what you want
│   └── jobs: []PipelineJob             ← "run these, in this order"
│         ├── name                      ── matched by name ──┐
│         └── templateRef ── points to ──▶ PipelineJobTemplate
│                                             └── spec.steps  ← the actual "job spec"
│
└── status: PipelineStatus              ← what happened
    ├── phase: Phase                     │
    └── jobs: []JobStatus                │
          ├── name  ◀────────────────────┘
          ├── jobName ── points to ──▶ Kubernetes Job (created by the controller)
          └── phase, startTime, completionTime
```