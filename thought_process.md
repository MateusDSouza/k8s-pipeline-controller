Before start coding directly it pays off to understand better what we have on the challenge markdown file:

The controller will be responsible for two CDR:
- `PipelineJobTemplate`
  - Interface to run an arbitrary number of jobs steps inside containers.
- `Pipeline`
  - A ordered composition of PipelineJobs

Bringing this to my closest reality, azure devops:

| Azure DevOps | Challenge |
|---|---|
| Pipeline (`azure-pipelines.yml`) | `Pipeline` resource |
| Job (a group of steps running on one agent) | One entry in `Pipeline.spec.jobs`, turned into one Kubernetes Job |
| Steps (`- script:` / `- task:`) run one after another in the job | `steps` in a `PipelineJobTemplate`: init containers run in order |
| Job/step templates (`template: build-job.yml`) reused across pipelines | `PipelineJobTemplate`: “a reusable resource shared across pipelines” |
| Agent that picks up a job and runs it | Controller creates the Job, and Kubernetes schedules its pod on a node |
| Agent workspace shared by the job’s steps ($(Pipeline.Workspace)) | An emptyDir volume shared by the init containers |
| dependsOn / job order | Controller starts job N+1 only after job N succeeded |
| Run status: queued / running / succeeded / failed | Pipeline.status.phase |
| A failing step fails the job, and the run stops | A failing init container fails the Job → the Pipeline goes Failed and later jobs never start |

## My thought process
The challenge itself will test my habilities for the creation of operators for kubernetes. For this I will need to build a simple kubernetes operator that should run a sort of CICD pipeline.
We see that the operator is written in golang because it is the main language for cloud native solution - k8s, terraform... So that the technology!

Now, about the kubebuilder this is the default framework used and maintained for the creation of operators - it already gives me the scaffolding and libraries so I dont build them from scratch. Do not reinvent the wheel.