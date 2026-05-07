# Changelog

## [4.1.1](https://github.com/igrikus/garm-helm-chart/compare/v4.1.0...v4.1.1) (2026-05-07)


### Bug Fixes

* adopt existing forge endpoints ([b6b4bc6](https://github.com/igrikus/garm-helm-chart/commit/b6b4bc69496305fde57c41a4c003f100a5f32980))

## [4.1.0](https://github.com/igrikus/garm-helm-chart/compare/v4.0.0...v4.1.0) (2026-05-07)


### Features

* operator watch scope ([7695acb](https://github.com/igrikus/garm-helm-chart/commit/7695acbcb0780da8aa55d6127d81c2c250605519))

## [4.0.0](https://github.com/igrikus/garm-helm-chart/compare/v3.0.0...v4.0.0) (2026-05-06)


### ⚠ BREAKING CHANGES

* GARM server/controller settings are now reconciled through a new
  `ServerSettings` custom resource. The chart renders this resource from the
  existing `garm.callbackUrl`, `garm.metadataUrl`, `garm.webhookUrl`, and
  `garm.url` values, so those settings are continuously enforced by the
  operator instead of being applied only during first-run database
  initialization.

### Features

* Added the `ServerSettings` CRD and operator reconciler for GARM
  server/controller settings. The operator now compares desired settings with
  GARM's `/controller-info` response and updates drift through the GARM
  controller API.
* Added `garm.minimumJobAgeBackoffSeconds` to configure GARM's minimum queued
  job age before runner allocation. The value is optional; when omitted, the
  chart leaves GARM's current/server-side default unchanged. Explicit `0` is
  supported for immediate scale-out behavior.
* Added `garm.caCertBundleSecretRef` for managing the controller CA bundle from
  a Kubernetes Secret. This field is authoritative: when it is omitted, the
  operator clears any existing GARM controller CA bundle.
* The `ServerSettings` API includes fields for GARM agent-mode settings, but
  those values are intentionally not exposed through `values.yaml` yet because
  agent mode is not fully supported by this chart. ([b90a794](https://github.com/igrikus/garm-helm-chart/commit/b90a794c7f42c0d0f7fa781d4b038c310f7c733d))

## [3.0.0](https://github.com/igrikus/garm-helm-chart/compare/v2.1.0...v3.0.0) (2026-05-06)


### ⚠ BREAKING CHANGES

This release rewrites the chart around a Kubernetes operator and the
`garm.igrikus.dev/v1alpha1` custom resources. The chart no longer uses a
post-install/post-upgrade configuration Job to drive `garm-cli` for forge
credentials, organizations, pools, and runner templates. Instead, Helm renders
custom resources and the operator continuously reconciles them against the GARM
API.

Existing `values.yaml` files that configure Gitea, GitHub, runner templates, or
pools must be migrated before upgrading to `3.0.0`.

### Added

* Added a bundled GARM operator deployment, service account, and RBAC. The
  operator image defaults to `ghcr.io/igrikus/garm-operator:<chart version>`.
* Added CRDs for GARM-managed resources:
  `Enterprise`, `Repository`, `Pool`, `Runner`, `RunnerTemplate`,
  `GiteaEndpoint`, `GiteaCredentials`, `GiteaOrganization`, `GitHubEndpoint`,
  and `GitHubCredentials`.
* Added Helm rendering for Gitea and GitHub custom resources under
  `forges.gitea.*` and `forges.github.*`.
* Added declarative GitHub repository, GitHub enterprise, and GitHub pool
  management. Previous chart versions only exposed GitHub credential setup.
* Added explicit Gitea endpoint resources, Gitea repository resources, and
  repository-scoped Gitea pools.
* Added operator controls under `operator.*`, including `operator.enabled`,
  `operator.image.*`, `operator.replicas`, `operator.leaderElection`,
  `operator.garmBaseUrl`, operator service account settings, pod metadata, and
  security contexts.
* Added `resources.operator` for the operator manager container.
* Added support for webhook configuration on organization and repository
  resources. When `webhookSecretRef` is omitted, the operator can generate a
  one-time random secret and let GARM store it encrypted.

### Changed

* Forge setup is now declarative and continuously reconciled by the operator.
  Removing or changing a rendered custom resource is the new source of truth;
  the old one-shot Job behavior is gone.
* The old `garm-operator` Helm hook Job and its init scripts were removed.
  Upgrades no longer run `garm-cli` scripts to recreate pools.
* Runner templates now render `RunnerTemplate` CRs directly. The template body
  field changed from `content` to `data`.
* Pool values now map to `Pool` CRs. Pool entries require a Kubernetes resource
  `name`; that name is also used as the GARM runner prefix.
* Secret values now use Kubernetes `SecretKeyRef`-style objects. The chart no
  longer reads endpoint URLs and tokens from mounted secret files in the
  post-upgrade Job.
* GitHub and Gitea endpoint configuration is now separated from credentials.
  Credentials reference endpoints by name.
* CRDs are installed from the chart `crds/` directory before templates. Use
  `helm install --skip-crds` if CRDs are managed separately.

### Removed

* Removed `forges.gitea.credentials[].secretName`.
* Removed `forges.gitea.credentials[].baseUrlKey`.
* Removed `forges.gitea.credentials[].apiUrlKey`.
* Removed `forges.gitea.credentials[].tokenKey`.
* Removed `forges.gitea.organizations[].credName`.
* Removed `forges.gitea.pools[].orgName`.
* Removed `runnerTemplates[].content`.
* Removed the old GitHub credential secret-key fields:
  `secretName`, `urlKey`, `tokenKey`, `appIdKey`, `appInstallationIdKey`, and
  `privateKeyKey`.

### Migration Guide

Before upgrading, rewrite your values file from the old init-Job model to the
new CR model.

1. Keep the unchanged server settings as-is.

   Most GARM server settings did not move: `garm.*`, `image.*`,
   `serviceAccount.*`, `podAnnotations`, `podLabels`, `service.*`,
   `strategyType`, `ingress.*`, `resources.garm`, `resources.substituteConfig`,
   `resources.configInit`, `extraVolumes`, `extraVolumeMounts`, `persistence`,
   `providers`, and `extraManifests` keep the same purpose.

2. Review the new operator settings.

   The operator is enabled by default:

   ```yaml
   operator:
     enabled: true
     image:
       repository: ghcr.io/igrikus/garm-operator
       tag: ""
     leaderElection: true
   ```

   If your GARM API is not reachable at the chart's internal service URL, set
   `operator.garmBaseUrl` to the URL the operator should use.

3. Migrate runner templates.

   Replace `content` with `data`:

   ```yaml
   # 2.x
   runnerTemplates:
     - name: linux-cache-server
       forgeType: gitea
       osType: linux
       content: |
         #!/bin/bash
         echo "configure runner"

   # 3.x
   runnerTemplates:
     - name: linux-cache-server
       forgeType: gitea
       osType: linux
       data: |
         #!/bin/bash
         echo "configure runner"
   ```

4. Migrate Gitea credentials into endpoint and credential resources.

   In `2.x`, a Gitea credential entry also carried the endpoint URL secret keys:

   ```yaml
   forges:
     gitea:
       credentials:
         - name: my-gitea
           secretName: garm-gitea-config
           baseUrlKey: server-url
           apiUrlKey: api-url
           tokenKey: access-token
   ```

   In `3.x`, declare the endpoint URL directly and reference only the PAT
   secret from credentials:

   ```yaml
   forges:
     gitea:
       endpoints:
         - name: my-gitea
           baseURL: https://gitea.example.com
           apiBaseURL: https://gitea.example.com/api/v1
       credentials:
         - name: my-gitea
           endpointName: my-gitea
           patSecretRef:
             name: garm-gitea-config
             key: access-token
   ```

   If your old `baseUrlKey` or `apiUrlKey` values pointed to secrets, copy
   those URL values into `baseURL` and `apiBaseURL`.

5. Migrate Gitea organizations.

   Replace `credName` with `endpointName` and `credentialsName`. The new
   `name` is the Kubernetes resource name; use `organizationName` for the
   actual forge organization name when they differ:

   ```yaml
   # 2.x
   forges:
     gitea:
       organizations:
         - name: MyGiteaOrg
           credName: my-gitea

   # 3.x
   forges:
     gitea:
       organizations:
         - name: my-gitea-org
           endpointName: my-gitea
           credentialsName: my-gitea
           organizationName: MyGiteaOrg
   ```

6. Migrate Gitea pools.

   Replace `orgName` with `organizationName`, add a pool `name`, add
   `endpointName`, and use `runnerInstallTemplateName` instead of
   `runnerInstallTemplate`:

   ```yaml
   # 2.x
   forges:
     gitea:
       pools:
         - orgName: MyGiteaOrg
           providerName: gcp
           image: ubuntu-24.04
           flavor: medium
           runnerInstallTemplate: linux-cache-server

   # 3.x
   forges:
     gitea:
       pools:
         - name: my-gitea-pool
           endpointName: my-gitea
           organizationName: my-gitea-org
           providerName: gcp
           image: ubuntu-24.04
           flavor: medium
           runnerInstallTemplateName: linux-cache-server
   ```

   `organizationName` is the Kubernetes `GiteaOrganization` resource name, not
   necessarily the literal organization name on the forge.

7. Migrate GitHub credentials.

   Add an explicit GitHub endpoint and replace secret key fields with
   `patSecretRef` or `appAuth.privateKeySecretRef`:

   ```yaml
   forges:
     github:
       endpoints:
         - name: github
           baseURL: https://github.com
       credentials:
         - name: github-pat
           endpointName: github
           authType: pat
           patSecretRef:
             name: my-github-secret
             key: token
   ```

   For GitHub App auth, use:

   ```yaml
   forges:
     github:
       credentials:
         - name: github-app
           endpointName: github
           authType: app
           appAuth:
             appID: 123
             installationID: 456
             privateKeySecretRef:
               name: my-github-secret
               key: private-key
   ```

8. Add repository and enterprise resources where needed.

   Pools now attach to explicit scope resources. For repository-scoped runners,
   create a repository resource first:

   ```yaml
   forges:
     github:
       repositories:
         - name: github-repo
           endpointName: github
           credentialsName: github-pat
           owner: my-org
           repositoryName: my-repo
       pools:
         - name: github-repo-pool
           endpointName: github
           repositoryName: github-repo
           providerName: gcp
           image: ubuntu-24.04
           flavor: medium
   ```

   For GitHub Enterprise runners, create an enterprise resource and reference
   it from the pool with `enterpriseName`.

9. Plan the first upgrade carefully.

   The old chart created and deleted some resources through one-shot
   `garm-cli` scripts. The new chart creates Kubernetes custom resources and
   the operator reconciles them. Before the first `helm upgrade`, render the
   new manifests with `helm template` and verify that every endpoint,
   credential, organization, repository, enterprise, runner template, and pool
   has the intended Kubernetes resource name and GARM scope.

### Features

* operator rewrite ([#1](https://github.com/igrikus/garm-helm-chart/issues/1)) ([6a5df95](https://github.com/igrikus/garm-helm-chart/commit/6a5df952b5a03e23b057848a20033934b82c95ce))
