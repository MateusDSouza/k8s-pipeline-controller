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

Create the PR to solidify this development, squash in main:

```sh
gh pr create --base main --head implement-api-types --title "feat(api-types): add Pipeline and PipelineJobTemplate types" \
  --body "define the CRD types and validation."

 gh pr merge --squash --delete-branch
```


## 3. Reconcilier

Following the template scaffolded by kubebuild and remembering that PipelineJobTemplate has the flag for the controller  = false, we only need to implement the reconcilier for the Pipeline object. The reconcilier is a function that is responsible to do API calls to the kubernetes API. What our reconcilier will do is simple: everytime something changes in a Pipeline or in a Job it owns (it looks to what we have in the status of the Pipeline and compares with the Spec) the controller calls the reconcilier and the reconcilier needs to decide the next action.

By this project, what is the logic behind our pipeline:

```sh
1. Get the Pipeline              → gone? return (nothing to do)
2. Already Succeeded/Failed?     → return (finished)
3. For each spec.jobs[i], in order:
     look for its Kubernetes Job (name: <pipeline>-<i>)
     ├─ doesn't exist      → get template, build Job, create it, phase=Running → return
     ├─ still running      → return (wait; the Job's next change triggers a new reconcile)
     ├─ failed             → phase=Failed → return
     └─ succeeded          → continue to i+1
4. All jobs succeeded            → phase=Succeeded → return
```

So, each reconcile cycle does one step and returns -> the pipeline moves forward across many api calls.

```sh
reconcile #1: no Job yet          → create build-app-0, phase=Running
reconcile #2: build-app-0 running → nothing to do
reconcile #3: build-app-0 done    → create build-app-1
reconcile #4: build-app-1 done    → phase=Succeeded
```

Now lets implement this logic inside: [text](../../internal/controller/pipeline_controller.go)

### Implementation
First we need to create a function that will be responsible for making a PipelineJobTemplate as a real kubernetes job -> [text](../../internal/controller/job_builder.go)
Following the code standards of golang imposed by kubebuilder, we have the test file in the same directory as the implementation -> [text](../../internal/controller/job_builder_test.go)
The test framework detect files with the _test.go suffix and execute them as tests. At this point we will follow a methodology where we will develop unit tests together with the development of new functions, we will use the built in test framework of golang.

The function buildJob in the job_builder.go file uses native kubernetes api libraries to execute the creation of containers and run pods based on the jobs it receives - for further documentation comments are in each line of the function.

#### Unit tests (`internal/controller/job_builder_test.go`)

| Test                            | Verifies                                                                                                            |
| ------------------------------- | ------------------------------------------------------------------------------------------------------------------- |
| `TestJobName`                   | Job name format is `<pipeline>-<index>-<job>`                                                                       |
| `TestJobName_IsDeterministic`   | Same inputs always give the same name, so the controller finds Jobs it created before                               |
| `TestJobName_UniquePerIndex`    | Different indexes in the same pipeline never share a name                                                           |
| `TestJobName_Length`            | Every name is a valid DNS-1123 label (≤ 63 chars). **Known issue:** the two long cases fail with the current format |
| `TestBuildJob_Metadata`         | Job name, namespace, and pipeline/index labels on the Job and its Pod                                               |
| `TestBuildJob_StepsOrder`       | Steps keep their order: all but the last become init containers, the last is the main container (1, 2 and 3 steps)  |
| `TestBuildJob_CopiesStepFields` | Each step's image, command, args and env are copied to its container                                                |
| `TestBuildJob_SharedWorkspace`  | One `emptyDir` volume, mounted at the same path in every container and used as the working directory                |
| `TestBuildJob_NoRetries`        | `backoffLimit: 0` and `restartPolicy: Never`, so a failing step fails the Job once                                  |
| `TestBuildJob_LabelsAreValid`   | Label values are valid, even with a 63-character pipeline name                                                      |

To execute the tests:
```sh
cd /Users/mateus.desouza/Desktop/k8s-pipeline-controller
go test -v -run 'TestJobName|TestBuildJob' ./internal/controller/
```

- Before moving forward with the development of the reconciler it is better to fix this naming length problem so that once we arrive to the execute integration tests and e2e tests we already have a problem covered. This is one huge advantage of executing unit tests before shipping.
- In this point of the development we have two main choices of design this solution:
  - rely on go coding
  - rely on kubebuilder markers
- **my decision is to rely on the kubebuilder markers because I prefer to introduce a limitation in the infrastructure rather than a workaround in the code**
```go
// I will introduce this rule on the Pipeline so that the kubernetes API output the error in when trying to apply the manifest.
// The + 4 covers the two hyphens and up to two index digits, which works because MaxItems=50 -> each Pipeline can have a maximum number of 50 PipelineJobs.
// +kubebuilder:validation:XValidation:rule="self.spec.jobs.all(j, self.metadata.name.size() + j.name.size() + 4 <= 63)",message="pipeline name + job name must be at most 59 characters"
```

Now we need to generate the manifests once again to get the changes.
Good also to test this against the envtest server - integration.

```sh
cd /Users/mateus.desouza/Desktop/k8s-pipeline-controller
make generate manifests
make test
```

The unit tests cases in [text](../../internal/controller/job_builder_test.go) were adjusted to cover the different scenarios and respecting the limit of 63 characters per kubernetes job names.
As the solution was implemented in the infrastructure side, it is good to also implement some integration tests agains the envtest server using the ginkgo - the default integration test framework used by kubebuilder.
Tests implemented in [text](../../internal/controller/pipeline_controller_test.go).

- Now that we have the job_builder implemented, tested unitary and with a couple of integration tests to ensure that the naming length is respected, time to go for the reconcilier implementation. Following the kubebuilder guide, in the file [text](../../internal/controller/pipeline_controller.go) we have the Reconcile() function with our TODO's.

Without the proper RBAC markers our operator would be able to communicate with the kubernetes API - Every r.Get, r.List and r.Create in this function is a request to the API server made as your operator’s ServiceAccount. RBAC denies everything by default, so each kind of call needs a rule.


Lets try to run everything on a local kind cluster now:

```sh
##kind create cluster --name pipeline-dev # create another kind cluster
cd /Users/mateus.desouza/Desktop/k8s-pipeline-controller
make install # install cdrs in the cluster
make run # runs the controller

#make sure that the correct context is being used
kubectl config use-context kind-pipeline-dev 
kubectl config current-context
```
Result:

```sh
2026-11-14T14:02:33+01:00       INFO    setup   Starting manager
2026-11-14T14:02:33+01:00       INFO    starting server {"name": "health probe", "addr": "[::]:8081"}
2026-11-14T14:02:33+01:00       INFO    Starting EventSource    {"controller": "pipeline", "controllerGroup": "ci.example.com", "controllerKind": "Pipeline", "source": "kind source: *v1alpha1.Pipeline"}
2026-11-14T14:02:33+01:00       INFO    Starting EventSource    {"controller": "pipeline", "controllerGroup": "ci.example.com", "controllerKind": "Pipeline", "source": "kind source: *v1.Job"}
2026-11-14T14:02:33+01:00       INFO    Starting Controller     {"controller": "pipeline", "controllerGroup": "ci.example.com", "controllerKind": "Pipeline"}
2026-11-14T14:02:33+01:00       INFO    Starting workers        {"controller": "pipeline", "controllerGroup": "ci.example.com", "controllerKind": "Pipeline", "worker count": 1}
```
Now, we need to prepare the sample files to test our operator -> [text](../../config/samples/ci_v1alpha1_pipeline.yaml) and [text](../../config/samples/ci_v1alpha1_pipelinejobtemplate.yaml).

For testing we will use the dummy job - 1 job with 2 steps the image used is the 
```yaml
apiVersion: ci.example.com/v1alpha1
kind: PipelineJobTemplate
metadata:
  name: build
spec:
  steps:
    - name: checkout
      image: busybox #tiny default image with linux just for testing
      command: ["sh", "-c", "echo cloning && sleep 2"]
    - name: test
      image: busybox
      command: ["sh", "-c", "echo testing && sleep 2"]
```

And the dummy pipeline referencing the same job by name - build:
```yaml
apiVersion: ci.example.com/v1alpha1
kind: Pipeline
metadata:
  name: release-1
spec:
  jobs:
    - name: build
      templateRef: build
```

In a second terminal apply the sample manifest generated by kubebuilder:
```bash
mateus.desouza@SDGESP-6YGQ97WG k8s-pipeline-controller % kubectl apply -k config/samples
sleep 10
kubectl get jobs
kubectl get pods --show-labels
---------------------------------------
pipeline.ci.example.com/release-1 created
pipelinejobtemplate.ci.example.com/build unchanged
NAME                STATUS     COMPLETIONS   DURATION   AGE
release-1-0-build   Complete   1/1           8s         10s
NAME                      READY   STATUS      RESTARTS   AGE   LABELS
release-1-0-build-5fj7j   0/1     Completed   0          10s   batch.kubernetes.io/controller-uid=7a0149e1-7342-4a85-887a-7127ecea7927,batch.kubernetes.io/job-name=release-1-0-build,ci.example.com/job-index=0,ci.example.com/pipeline=release-1,controller-uid=7a0149e1-7342-4a85-887a-7127ecea7927,job-name=release-1-0-build
mateus.desouza@SDGESP-6YGQ97WG k8s-pipeline-controller % kubectl logs -l ci.example.com/pipeline=release-1 -c checkout
kubectl logs -l ci.example.com/pipeline=release-1 -c test
cloning
testing
```

After execution delete the pipeline:
```sh
kubectl delete pl release-1
kubectl get jobs
kubectl get pods --show-labels
```
Result:
```bash
 *  Executing task: docs/challenge/thought_process.md: kubectl-delete 
pipeline.ci.example.com "release-1" deleted from default namespace
No resources found in default namespace.
No resources found in default namespace.
```