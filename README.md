# GARM Helm Chart

This chart installs [GARM](https://github.com/cloudbase/garm) and a Kubernetes controller that reconciles GARM configuration from CRDs.

> [!NOTE]
> This is an unofficial Helm chart and is not affiliated with the GARM project. Please do not open issues regarding this chart in the official GARM repository.

## Architecture

The release contains two long-running workloads:

- `garm`: the GARM API server, UI, database storage, and provider configuration.
- `garm-operator`: a controller manager that watches `garm.igrikus.dev/v1alpha1` resources and calls the GARM API.

The chart can render CRs for endpoints, credentials, organizations, repositories, enterprises, images, runner templates, and pools. The controller owns lifecycle reconciliation after install and upgrade.

## Installation

1. Create a copy of `values.yaml` (e.g., `my-values.yaml`) and customize it for your environment.

Install the published OCI chart:

```bash
helm install my-garm oci://ghcr.io/igrikus/garm \
  --namespace garm \
  --create-namespace \
  -f my-values.yaml
```

## Operator local development

1. Run `make manifests` and `make generate` in `operator/` after API changes.
2. Run `go test ./...` in `operator/`.
3. Run `helm lint .` and `helm template . --include-crds`.
4. Keep root `crds/` in sync with operator API changes.

## Contributing

Contributions are highly welcome! This project uses `semantic-release` to automate the release process, so it's important that all commit messages adhere to the [Conventional Commits specification](https://www.conventionalcommits.org/en/v1.0.0/).

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
