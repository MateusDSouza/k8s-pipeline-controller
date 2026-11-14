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

Before moving forward it is better to document all the decision in a file called decisions.md.

#### Implementing Pipeline Sequencing

The first implementation only created the first job, which was enough to test the controller on kind.
Now a pipeline can have several jobs, which must run in order and stop at the first failure.

Every reconcile does the same four things:

| Role    | Question                                  | Where                                                  |
| ------- | ----------------------------------------- | ------------------------------------------------------ |
| OBSERVE | What exists in the cluster right now?     | `Reconcile` lists the Jobs; `stateOf()` reads each one |
| DECIDE  | Given that, what is the single next step? | `decide()` in `decide.go`                              |
| ACT     | Do it (create a Job, or nothing)          | `Reconcile` in `pipeline_controller.go`                |
| REPORT  | Tell the user what happened               | `setPhase()` and `jobStatuses()` in `status.go`        |

`decide.go` and `status.go` are pure functions: data in, data out, no calls to k8s.
That makes them easy to unit test, like `job_builder.go`. Only `pipeline_controller.go` talks to the API server.

##### decide.go — what should happen next?

- `jobState`: the four states a pipeline job can be in: missing, running, succeeded, failed.
  "Missing" is the zero value on purpose: looking up a job that was never created in a Go map
  returns the zero value, so "no Job yet" needs no special case.
- `stateOf(job)`: reads the conditions that Kubernetes' own Job controller sets on a Job:
  `Complete=True` → succeeded, `Failed=True` → failed, anything else → still running.
- `jobCondition(job, type)`: small helper that finds one condition in a Job's status.
- `decide(numJobs, states)`: walks the jobs in order and returns ONE action:
  - first job that doesn't exist yet → create it
  - first job that is running → wait
  - first job that failed → fail the pipeline (later jobs are never created: fail-fast)
  - every job succeeded → succeed the pipeline
  Because it stops at the first job that isn't done, job N+1 is only ever created after job N succeeded.

Tested in `decide_test.go` with a table of scenarios (nothing created, first running, first failed,
failure after a success, all done...), without any cluster.

##### status.go — what does the user see?

- `setPhase(pipeline, phase, reason, message)`: the ONLY place where a pipeline changes phase.
  It sets the phase, the start time (first time it leaves Pending), the completion time
  (when it reaches Succeeded or Failed) and a standard `Ready` condition with a reason and message.
  Keeping it in one place means logging and metrics can be added in one spot later.
- `jobStatuses(pipeline, jobs)`: rebuilds the per-job list in the status from the Jobs that really
  exist: name, Kubernetes Job name, phase, start and completion time.

##### pipeline_controller.go — Reconcile puts it together

1. **Read** the Pipeline. Deleted → nothing to do. Already Succeeded/Failed → nothing to do.
2. **Observe**: list the Jobs with this pipeline's label, keep only the ones it owns
   (`IsControlledBy`, to ignore leftovers from a deleted pipeline with the same name),
   and compute each one's state.
3. **Decide and act**: call `decide()`.
   - create → fetch the template, build the Job, set the owner reference, create it.
     If the template doesn't exist: condition `TemplateNotFound`, check again in 15 s.
   - wait / fail / succeed → only the phase changes.
4. **Report**: rebuild the per-job status, and write the status (only if it changed) with a patch
   on the status subresource.

Each reconcile does one step and returns. The pipeline moves forward because every change to one of
its Jobs (created, completed, failed) triggers a new reconcile (`Owns(&batchv1.Job{})`):

reconcile #1  Pipeline created   states: {}              → create job 0     phase: Running
reconcile #2  Job 0 created      states: {0: running}    → wait             phase: Running
reconcile #3  Job 0 completed    states: {0: succeeded}  → create job 1     phase: Running
reconcile #4  Job 1 created      states: {0: ok, 1: running} → wait         phase: Running
reconcile #5  Job 1 completed    states: {0: ok, 1: ok}  → succeed          phase: Succeeded

##### Samples

- New template [deploy](../../config/samples/ci_v1alpha1_pipelinejobtemplate_deploy.yaml) (added to the kustomization).
- [release-1](../../config/samples/ci_v1alpha1_pipeline.yaml) now has two jobs: build → deploy.
- [examples/fail-fast.yaml](../../examples/fail-fast.yaml): the first job fails, the second must never be created.

## 4. Observability

### Metrics

To begin with the observability lets tackle the metrics. We know that prometheus reaches the /metrics endpoint of the pod and uses a pull strategy to scrap the metrics.
An user of this controller would like to know metrics related to k8s pods, jobs, containers - but in the case of the pipelines, it would also want to know when the pipeline started and when it ends, how long did it take, how many of them succeed and how many failed, and so on...
So, if we can translate this we have:
- traffic (started)
- errors (failed)
- latency (duration)

By deciding for the simplicity we can take advantage of the controller-runtime registry that serves metrics at the same endpoint as the built-in ones - so prometheus just need to pull the metrics from the controller pod endpoint /metrics.

So the implementation of those variables that are then send to the register are in the file [text](../../internal/metrics/metrics.go).

But now we have a second problem -> the reconcile is doing several calls to check the status of a pipeline, which means that if in every cycle it tries to talk with the pipeline and increment our counters, we would have messy metrics. Thats why the way of measuring things is to always count the transition of the state of the pipeline.
We have the function setPhase that is responsible to change the phase of a run, but this is a pure function and by the best practices shouldnt be poluted, that why it is better to have another function to be able to record those values!

This implementation is in [text](../../internal/controller/transitions.go).

With this modularity we just need to add this function call in the right after we have a transition in the reconcile loop.

- Now lets run it against kind using the script and see the results:

In a terminal:
```sh
cd /Users/mateus.desouza/Desktop/k8s-pipeline-controller
scripts/dev.sh up                 # cluster + CRDs
scripts/dev.sh run                # terminal 1: controller with metrics
```
In a second terminal:
```sh
cd /Users/mateus.desouza/Desktop/k8s-pipeline-controller
scripts/dev.sh metrics            # one full dry run: pulls busybox, warms everything up
scripts/dev.sh reset              # clean slate for the live demo
```

After the execution:
```sh
curl -s localhost:8080/metrics | grep '^pipeline_controller'
```

### Logging

The challenge asks for **structured, contextual logging**:
- **Structured**: each log line has a message plus separate key/value fields
  (`log.Info("created job", "job", job.Name)`), instead of values mixed into a sentence.
- **Contextual**: each line automatically says what it belongs to.

**How it works.** The code logs through **logr** (an interface) and **zap** (the library that
formats and writes the lines), set up by kubebuilder in `cmd/main.go`. Before calling `Reconcile`,
controller-runtime creates a logger that already contains the controller name, the pipeline's name
and namespace, and a unique `reconcileID`, and passes it in the `ctx`. We get it with
`logf.FromContext(ctx)`.

**What I log** (only events, not every reconcile):

| Where                                 | Message                  | Fields                    |
| ------------------------------------- | ------------------------ | ------------------------- |
| `Reconcile`, when a Job is created    | `created job`            | job, index, template      |
| `recordTransition` (`transitions.go`) | `pipeline phase changed` | from, to, reason, message |

Errors are not logged in the code: they are wrapped with context (`fmt.Errorf("create job %s: %w", ...)`)
and returned.

**JSON by default.** In `cmd/main.go` -> `Development: false`, so the output is JSON (info level,
stack traces only on errors). In production, logs are collected into systems like Grafana Loki and JSON let it filter by field.



## 5. Integration Tests

The unit tests check the pure functions (`decide`, `jobStatuses`, `buildJob`) without Kubernetes.
The integration tests check the **whole reconciler against a real API server**, using **envtest**
(a real kube-apiserver + etcd started by `make test`, with the CRDs installed).

**What envtest doesn't have:** no kubelet and no Kubernetes Job controller. My controller creates
Jobs, but nothing runs them. So in the tests, **the test plays the Job controller**: after a reconcile
creates a Job, the test writes the Job's status as succeeded or failed, then reconciles again to see
how the controller reacts. The tests call `Reconcile` directly, one step at a time, so they are
deterministic (no waiting, no timing) and take milliseconds.

Each scenario checks: the pipeline phase, the `Ready` condition, the completion time, the phase of
every job, how many Kubernetes Jobs were created (jobs after a failure must never be created), and
that reconciling a finished pipeline again changes nothing.

| #   | Scenario                     | Jobs (outcome)                      | Pipeline              | Job phases                 | Jobs created |
| --- | ---------------------------- | ----------------------------------- | --------------------- | -------------------------- | ------------ |
| 1   | 1 job succeeds               | build ✓                             | Succeeded             | Succeeded                  | 1            |
| 2   | 1 job fails                  | build ✗                             | Failed                | Failed                     | 1            |
| 3   | 2 jobs succeed               | build ✓ → deploy ✓                  | Succeeded             | Succeeded, Succeeded       | 2            |
| 4   | 2 jobs, first fails          | build ✗ → deploy (skipped)          | Failed                | Failed, Skipped            | 1            |
| 5   | 3 jobs, second fails         | build ✓ → test ✗ → deploy (skipped) | Failed                | Succeeded, Failed, Skipped | 2            |
| 6   | 3 jobs succeed               | build ✓ → test ✓ → deploy ✓         | Succeeded             | Succeeded ×3               | 3            |
| 7   | 2 pipelines at the same time | A: ✓ → ✓ / B: ✓ → ✗                 | A Succeeded, B Failed | A: S, S / B: S, F          | 2 + 2        |

**What they prove:**
- **Sequencing:** job N+1 is only created after job N succeeded (scenarios 3, 5, 6).
- **Fail-fast:** after a failure, no further Job is created, and the remaining jobs are `Skipped` (2, 4, 5).
- **Terminal state:** a finished pipeline does nothing more, however often it is reconciled (all).
- **Isolation:** two pipelines progressing at the same time don't affect each other (7). Each
  pipeline only looks at the Jobs it owns (label + ownerReference).

**What they don't prove** (covered elsewhere): that containers really run and steps execute in order
(verified on kind with `scripts/dev.sh demo`), and that the deployed controller has the right RBAC
(e2e tests deploy the real manager).

**About scenario 7:** in the tests, the two pipelines are interleaved step by step rather than truly
in parallel. That is what matters for the logic: the controller handles one reconcile per pipeline
at a time (controller-runtime's work queue guarantees it), so "at the same time" means their steps
interleave, which is exactly what the test does. On kind, the same scenario runs with real pods.


To run the integration tests:
```sh
go test ./internal/controller/ -v -ginkgo.v ## this way we run only the integration tests with more verbosity
```

To run the complete suit of test:
```sh
make test
```