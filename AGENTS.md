## What this is

A Helm chart that deploys [GARM](https://github.com/cloudbase/garm) (GitHub/Gitea Actions Runner Manager) alongside a custom Kubernetes operator. The operator watches `garm.igrikus.dev/v1alpha1` CRDs and reconciles them against the GARM API.

## Two-component architecture

- **Helm chart** (root): templates, values, CRDs. Published as an OCI chart to `ghcr.io/igrikus/garm`.
- **Operator** (`operator/`): a kubebuilder-based Go controller manager. Built as a Docker image (`ghcr.io/igrikus/garm-operator`). Has its own `go.mod`, `Makefile`, tests, and CI.

CRD YAMLs in `crds/` are generated from the operator's Go API types. The chart installs them; the operator reconciles them.

## Common commands

### Helm chart (run from repo root)

```bash
helm lint .
helm template . --include-crds
helm template . -f test-values.yaml
```

### Operator (run from `operator/`)

```bash
make manifests      # regenerate CRDs (output goes to ../crds/) and RBAC
make generate       # regenerate DeepCopy methods
make test           # unit tests (uses envtest)
make lint           # golangci-lint (uses custom plugin config in .custom-gcl.yml)
make build          # build the manager binary
go test ./...       # run tests without regeneration steps
```

### Running a single operator test

```bash
cd operator && go test ./internal/controller/ -run TestMyController -v
```

## CRD resource types

API group: `garm.igrikus.dev/v1alpha1`. Types defined in `operator/api/v1alpha1/`:

Enterprise, Repository, Pool, Runner, RunnerTemplate, GiteaEndpoint, GiteaCredentials, GiteaOrganization, GitHubEndpoint, GitHubCredentials

## Key conventions

- **Commit messages**: Conventional Commits required. `semantic-release` on `main` auto-bumps `Chart.yaml` version and publishes. Do not edit `version:` in `Chart.yaml` by hand.
- **CRD sync**: after changing Go types in `operator/api/`, run `make manifests` and `make generate` from `operator/` to keep `crds/` and deepcopy in sync.
- **Values structure**: forge-specific resources (endpoints, credentials, orgs, repos, pools) live under `forges.gitea.*` and `forges.github.*`. Provider configs live under `providers[]`.
- **Template naming**: Helm templates for the operator use `operator-` prefix (e.g., `operator-deployment.yaml`, `operator-rbac.yaml`).
