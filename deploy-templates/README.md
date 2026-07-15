# tekton-pipeline-queue

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.1.0](https://img.shields.io/badge/AppVersion-0.1.0-informational?style=flat-square)

A Helm chart for the Tekton Pipeline Queue operator

**Homepage:** <https://docs.kuberocketci.io/>

## Introduction

Tekton Pipeline Queue governs concurrency and admission ordering of Tekton `PipelineRun` resources.
It defines the `PipelineRunQueue` CRD, which selects a set of `PipelineRun`s (via a label selector),
groups them into lanes (via label-derived queue keys) and admits at most `concurrency` runs per lane
according to a configurable strategy, keeping the rest queued until a slot becomes free.

## Project Structure

- **api/v1alpha1/**: Contains the API definitions for the `PipelineRunQueue` custom resource, including
  the structure and validation of the CRD.
- **cmd/**: Hosts the main application entry point and the command-line interface setup.
- **config/**: Includes Kubernetes configuration files for deploying the controller, such as CRDs, RBAC
  rules, and sample configurations.
- **docs/**: Provides detailed documentation on the API and usage examples.
- **deploy-templates/**: Contains Helm chart templates for deploying the controller.

## Getting Started

To get started with Tekton Pipeline Queue, ensure you have Kubernetes and Tekton Pipelines installed in
your environment. Follow these steps to deploy the controller:

1. Clone the repository to your local environment.
2. Navigate to the `config/` directory.
3. Apply the CRDs to your Kubernetes cluster:

   ```bash
   kubectl apply -f config/crd/bases/
   ```

### Deploy with Helm

1. To add the Helm EPAMEDP Charts for local client, run "helm repo add":

    ```bash
    helm repo add epamedp https://epam.github.io/edp-helm-charts/stable
    ```

2. Choose available Helm chart version:

    ```bash
    helm search repo epamedp/tekton-pipeline-queue -l
    NAME                              CHART VERSION   APP VERSION     DESCRIPTION
    epamedp/tekton-pipeline-queue      0.1.0          0.1.0          A Helm chart for the Tekton Pipeline Queue operator
    ```

    _**NOTE:** It is highly recommended to use the latest released version._

3. Full chart parameters available in [deploy-templates/README.md](deploy-templates/README.md).

4. Install operator with the following command:

    ```bash
    helm install tekton-pipeline-queue epamedp/tekton-pipeline-queue --version <chart_version>
    ```

5. Check the namespace that should contain the Tekton Pipeline Queue controller in a running status.

### Deploy with cluster add-ons

1. Navigate to the forked [edp-cluster-add-ons](https://github.com/epam/edp-cluster-add-ons) repository.

2. Enable the deployment of the Tekton Pipeline Queue Helm chart by setting the
   `tekton-pipeline-queue.enable` and `tekton-pipeline-queue.createNamespace` values to `true` in the
   `clusters/core/apps/values.yaml` file.

    ```yaml title="clusters/core/apps/values.yaml"
    tekton-pipeline-queue:
      createNamespace: true
      enable: true
    ```

3. Update the `clusters/core/addons/tekton-pipeline-queue/values.yaml` file with the desired
   configuration for the Tekton Pipeline Queue Helm chart.

4. After updating the `values.yaml` file, commit the changes to the repository and apply the changes
   with Helm or Argo CD.

## Usage

Create a `PipelineRunQueue` that selects the `PipelineRun`s to govern and defines how they are grouped
into lanes:

```yaml
apiVersion: edp.epam.com/v1alpha1
kind: PipelineRunQueue
metadata:
  name: deploy-queue
spec:
  selector:
    matchLabels:
      app.edp.epam.com/pipelinetype: deploy
  queueKey:
    - app.edp.epam.com/cdpipeline
    - app.edp.epam.com/cdstage
  concurrency: 1
  strategy: Queue
```

Apply the definition to your Kubernetes cluster:

```sh
kubectl apply -f pipelinerunqueue.yaml
```

## Features

- **Lane-based Concurrency Control**: Group `PipelineRun`s into independent lanes via label-derived
  queue keys and cap the number of concurrently running runs per lane.
- **Configurable Admission Strategy**: Choose whether newer runs queue behind, replace queued, or
  cancel in-progress runs in the same lane.
- **Flexible Selection**: Use a standard Kubernetes label selector to choose which `PipelineRun`s a
  queue governs.

## Contributing

We welcome contributions in the form of issues and pull requests. Please follow the contributing
guidelines outlined in the repository.

## License

This project is licensed under the [Apache License 2.0](LICENSE.txt).

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| epmd-edp | <SupportEPMD-EDP@epam.com> | <https://solutionshub.epam.com/solution/kuberocketci> |

## Source Code

* <https://github.com/KubeRocketCI/tekton-pipeline-queue>

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` | Affinity for pod assignment |
| annotations | object | `{}` | Annotations to add to the deployment |
| extraVolumeMounts | list | `[]` | Additional volume mounts to add to the manager container |
| extraVolumes | list | `[]` | Additional volumes to add to the manager pod |
| image.registry | string | `"docker.io"` | tekton-pipeline-queue image registry |
| image.repository | string | `"epamedp/tekton-pipeline-queue"` | tekton-pipeline-queue Docker image name. The released image can be found on [Dockerhub](https://hub.docker.com/r/epamedp/tekton-pipeline-queue) |
| image.tag | string | `nil` | tekton-pipeline-queue Docker image tag. The released image can be found on [Dockerhub](https://hub.docker.com/r/epamedp/tekton-pipeline-queue). If not defined then .Chart.AppVersion is used |
| imagePullPolicy | string | `"IfNotPresent"` | Image pull policy |
| imagePullSecrets | list | `[]` | Optional array of imagePullSecrets containing private registry credentials # Ref: https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry |
| name | string | `"tekton-pipeline-queue"` | component name |
| nodeSelector | object | `{}` | Node labels for pod assignment |
| podSecurityContext | object | `{"runAsNonRoot":true,"seccompProfile":{"type":"RuntimeDefault"}}` | Pod-level security context # Ref: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/ |
| resources | object | `{"limits":{"memory":"192Mi"},"requests":{"cpu":"50m","memory":"64Mi"}}` | Resource requests/limits for the manager container. # Ref: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/ |
| securityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"readOnlyRootFilesystem":true}` | Container-level security context # Ref: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/ |
| tolerations | list | `[]` | Tolerations for pod assignment |
