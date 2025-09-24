# GARM Helm Chart

This Helm chart deploys [GARM (GitHub/Gitea Actions Runner Manager)](https://github.com/cloudbase/garm) on a Kubernetes cluster.

> **Note:** This is an unofficial Helm chart and is not affiliated with the GARM project. Please do not open issues regarding this chart in the official GARM repository.

## Chart Status & Expectations

- This chart was initially created for a specific use case (Gitea with a GCP provider). For other scenarios, it is provided on a best-effort basis. Pull requests are highly encouraged and welcome.
- The chart relies on unreleased GARM features (e.g., WebUI, Gitea forge support). Right now it uses a pinned `nightly` image hash that has been tested. Using other image versions or tags may result in partial or complete failure.

## Architecture

This chart uses an operator-like pattern, launching two pods:

- **GARM Server (`Deployment`):** The main GARM application that runs continuously.
- **Operator (`Job`):** A one-time job that runs during `helm install` or `helm upgrade` to apply declarative configurations for forges, providers, and runner pools from your `values.yaml`.

## Installation

> This chart is not yet available on a public chart repository like Artifact Hub.

1.  Clone this repository:
    ```bash
    git clone <repository-url>
    cd garm-helm-chart
    ```
2.  Create a copy of `values.yaml` (e.g., `my-values.yaml`) and customize it for your environment.
3.  Install the chart:
    ```bash
    helm install -f my-values.yaml my-garm .
    ```

## Configuration

All configuration is managed through the `values.yaml` file, which serves as the single source of truth for your GARM deployment. Please refer to the comments within `values.yaml` for detailed information on all available options.

### Pool Management

This chart provides a convenient way to manage GARM pools directly from your `values.yaml` file.

**Important Notes:**

- **Gitea Exclusive:** Currently, this declarative pool management feature is only implemented for Gitea forges.
- **Pool Recreation:** Pools are recreated on every `helm upgrade` to ensure they are always synchronized with the state defined in `values.yaml`. Any manual changes made to the pools via the GARM UI or API will be lost on chart upgrade
