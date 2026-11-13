## My thought process

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

## Starting the project

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