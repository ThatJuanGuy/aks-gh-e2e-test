# Cluster Health Monitor

A Kubernetes cluster health monitoring tool that checks various aspects of cluster health, including DNS resolution and pod startup time.

## Overview

Cluster Health Monitor runs as a Kubernetes deployment and exposes metrics about the health of your cluster through a Prometheus endpoint.

## Deployment

### Deploying Base Manifests

To deploy the Cluster Health Monitor using the base manifests:

```bash
kubectl apply -k manifests/base/
```

### Removing Deployment

To remove the base deployment:

```bash
kubectl delete -k manifests/base/
```

### Customizing Deployment

For custom deployments, create your own overlay in `manifests/overlays/` and change the directory to the directory containing `kustomization.yaml`, e.g., `manifests/overlays/test`.

## Testing

### Running Unit Tests

To run unit tests:

```bash
make test-unit
```

### Running End-to-End (E2E) Tests

To run E2E tests, ensure [Ginkgo](https://onsi.github.io/ginkgo/#getting-started) is installed.

There are 2 options for running E2E test:

1. Full run from setting and cleaning up the Kind cluster used for E2E:

    ```bash
    make test-e2e-full
    ```

2. Reuse existing Kind cluster:

    ```bash
    make test-e2e-reuse
    ```

    To setup Kind cluster for this, you can run:

    ```bash
    make kind-setup-e2e
    ```

    See [useful commands](#useful-commands) for other commands that can be used once the test environment is set up.

#### Customizing E2E Tests

You can customize the Kind cluster name used for E2E by setting `KIND_CLUSTER_NAME`:

```bash
KIND_CLUSTER_NAME=my-custom-cluster make kind-setup-e2e
KIND_CLUSTER_NAME=my-custom-cluster make test-e2e-reuse
```

### Local Testing with Kind

Kind (Kubernetes IN Docker) is used to create a local Kubernetes cluster for testing and development.

#### Prerequisites

- [Docker](https://www.docker.com/)
- [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)

#### Setting Up a Local Environment

To set up a local test environment with Kind:

```bash
make kind-test-local
```

This command:

1. Creates a Kind cluster.
1. Builds a Docker image for the cluster health monitor.
1. Loads the image into the Kind cluster.
1. Applies the Kubernetes manifests using the test overlay.

### Useful Commands

Once your test environment is set up, you can use the following commands:

- **Set kubectl context**:

  ```bash
  make kind-export-kubeconfig
  ```

- **Redeploy after changes**:

  ```bash
  make kind-redeploy
  ```

  This rebuilds the image, loads it into the Kind cluster, and redeploys the application.

- **Clean up**:

  ```bash
  make kind-delete-cluster
  ```

  This deletes the Kind cluster when you're done testing.
