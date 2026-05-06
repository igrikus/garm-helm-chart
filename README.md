# GARM Helm Chart

This chart installs [GARM](https://github.com/cloudbase/garm) and an operator that reconciles GARM configuration from CRDs.

> [!NOTE]
> This is an unofficial Helm chart and is not affiliated with the GARM project. Please do not open issues regarding this chart in the official GARM repository.

## Architecture

The release contains two long-running workloads:

- `garm`: the GARM API server, UI, database storage, and provider configuration.
- `garm-operator`: watches `garm.igrikus.dev/v1alpha1` resources and calls the GARM API.

The chart can render CRs for endpoints, credentials, organizations, repositories, enterprises, runner templates, and pools. The operator owns lifecycle reconciliation after install and upgrade.

## Installation

1. Create a copy of `values.yaml` (e.g., `my-values.yaml`) and customize it for your environment.

2. Install the published OCI chart:

```bash
helm install my-garm oci://ghcr.io/igrikus/garm \
  --namespace garm \
  --create-namespace \
  -f my-values.yaml
```

## Upgrading

Helm does not update CRDs on upgrade ([by design](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/#some-caveats-and-explanations)). When upgrading to a version with CRD changes, apply the CRDs manually before running `helm upgrade`:

```bash
export GARM_CHART_VERSION=<version>
helm pull oci://ghcr.io/igrikus/garm --version "${GARM_CHART_VERSION}" --untar
kubectl apply -f garm/crds/
```

Then upgrade the release:

```bash
helm upgrade my-garm oci://ghcr.io/igrikus/garm \
  --version <version> \
  --namespace garm \
  -f my-values.yaml
```

## Operator local development

1. Run `make generate manifests` in `operator/` after API type or marker changes.
2. Run `make check-generated` in `operator/` to verify generated CRDs, RBAC, and DeepCopy methods are committed.
3. Run `go test ./...` in `operator/`.
4. Run `helm lint .` and `helm template . --include-crds`.

CI runs the same generated-artifact drift check and fails if `crds/`, RBAC, or DeepCopy output is stale. The release workflow does not regenerate CRDs; published chart tags must already contain the generated CRDs.

## Contributing

Contributions are highly welcome! This project uses [Release Please](https://github.com/googleapis/release-please) to automate releases, so it's important that all commit messages adhere to the Conventional Commits specification.

> [!NOTE]
> Do not edit `version:` in `Chart.yaml` by hand for normal releases. Let Release Please update it in the release pull request.

When you create a pull request, please make sure your commit messages are structured as follows:

```
<type>([optional scope]): <description>

[optional body]
```

**Common commit types:**

- **fix**: A bug fix. (Triggers a `patch` version release)
- **feat**: A new feature. (Triggers a `minor` version release)
- **docs**: Documentation only changes. (No release)
- **refactor**: A code change that neither fixes a bug nor adds a feature. (No release)
- **test**: Adding missing tests or correcting existing tests. (No release)
- **chore**: Changes to the build process or auxiliary tools and libraries such as documentation generation. (No release)

To indicate a **breaking change** (which triggers a `major` version release), add a `!` after the type (e.g., `feat!:`) or include `BREAKING CHANGE:` in the commit message footer.

**Examples:**

No release

```
chore: change build process
```

`patch` release

```
fix: added icon
```

`minor` release

```
feat(server): Add option to configure liveness probe
```

`major` release

```
feat!: breaking change for pool creation logic
```

Following this convention is crucial as it directly controls the versioning and release notes for the chart.
