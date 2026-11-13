# 🧪 Kubernetes Pipeline Controller

## 🎯 Goal

In this challenge, you will design and implement a Kubernetes controller that
orchestrates the execution of pipelines, which consist of multiple jobs, and
each job comprises a series of steps.

Your task is to create a custom controller that can manage the lifecycle of
these pipelines, jobs, and steps. To complete this challenge, you will use
**kubebuilder** to scaffold your controller and **kind** to deploy, test and
validate your implementation.

---

## 🧩 Architecture

The controller shall be responsible for two custom resource definitions:

### 🔹 `PipelineJobTemplate`

- Provide a reusable resource, that can be shared across pipelines.
- Provide an interface to run an arbitrary number of job steps inside
  containers.

### 🔹 `Pipeline`

- Represents a pipeline, which is a sequence of job templates.
- On creation, triggers execution of all referenced `PipelineJobTemplate`
  resources.

---

## 📡 Observability

The controller shall have features that provide better traceability and
production-ready observability.

### 📊 Metrics

- Expose an endpoint for Prometheus-compatible real-time metrics.

### 🪵 Logging

- Use structured, contextual logging.

## 🛡️ Testing

- Include unit and integration tests where appropriate.

---

## 🧰 Technologies & Tools

| Category   | Tools                                       |
| ---------- | ------------------------------------------- |
| Language   | Go                                          |
| Framework  | [kubebuilder](https://book.kubebuilder.io/) |
| Deployment | [kind](https://kind.sigs.k8s.io/)           |

---

## 📝 Deliverables

Please submit the code back to this repository along with examples and
instructions on how to run the application.

Please do this at least one working day before the interview so that we can
review your submission and discuss it during the meeting.

### Good luck & happy coding! 💡🔧
