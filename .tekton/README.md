# Konflux Pipeline Configuration

This directory contains Tekton PipelineRun configurations for building and testing vault-manager in Konflux using the [app-sre/shared-pipelines](https://github.com/app-sre/shared-pipelines).

## Pipeline Files

### vault-manager-master-push.yaml
Triggered on pushes to the `master` branch:
- Builds the vault-manager container image
- Tags with the git commit SHA
- Runs integration tests in an ephemeral namespace
- Pushes to `quay.io/app-sre/vault-manager:<commit-sha>`

### vault-manager-pull-request.yaml
Triggered on pull requests targeting `master`:
- Builds a test image
- Runs full integration test suite
- Tags with `on-pr-<commit-sha>` and expires after 5 days
- Pushes to `quay.io/redhat-user-workloads/app-sre-tenant/vault-manager-master/vault-manager-master:on-pr-<commit-sha>`

## Integration Testing

Both pipelines use the `ephemeral-namespace-run-script` parameter to run tests in a Kubernetes ephemeral namespace:

```yaml
- name: ephemeral-namespace-run-script
  value: tests/k8s/konflux-test-runner.sh
```

### Test Infrastructure

The test runner (`tests/k8s/konflux-test-runner.sh`) deploys the following services:

1. **Keycloak** - Authentication service with test realm configuration
2. **qontract-server** - GraphQL API server with app-interface data
3. **Primary Vault** - First Vault instance for testing
4. **Secondary Vault** - Second Vault instance for multi-instance scenarios
5. **Vault Init Job** - Populates Vaults with required secrets

After infrastructure setup, it runs the BATS test suite from the built container image.

### Test Execution Flow

```
1. Deploy Keycloak → Wait for ready → Configure realm
2. Deploy qontract-server with app-interface data
3. Deploy both Vault instances → Wait for ready
4. Initialize Vault secrets
5. Deploy test runner pod with built image
6. Run BATS integration tests
7. Report results
```

## Kubernetes Manifests

Test infrastructure manifests are located in `tests/k8s/`:

- `keycloak.yaml` - Keycloak pod and service
- `keycloak-config-job.yaml` - Keycloak realm configuration job
- `qontract-server.yaml` - GraphQL server pod and service
- `vault-primary.yaml` - Primary Vault instance
- `vault-secondary.yaml` - Secondary Vault instance
- `vault-init-job.yaml` - Vault secret initialization job

## Local Testing

You can test the Kubernetes manifests locally with kind:

```bash
# Create kind cluster
kind create cluster --name vault-manager-test

# Load required images (if using app-sre images)
docker pull quay.io/app-sre/keycloak-config-cli:5.11.0-22.0.4
kind load docker-image quay.io/app-sre/keycloak-config-cli:5.11.0-22.0.4 --name vault-manager-test

# Deploy test infrastructure
./tests/k8s/deploy-services.sh

# Check status
kubectl get all -n vault-manager-test

# Cleanup
./tests/k8s/cleanup.sh
```

## Pipeline Parameters

Key parameters used in the pipelines:

| Parameter | Description | Example |
|-----------|-------------|---------|
| `git-url` | Source repository URL | Provided by PipelinesAsCode |
| `revision` | Git commit SHA | Provided by PipelinesAsCode |
| `output-image` | Container image to build | `quay.io/app-sre/vault-manager:<sha>` |
| `dockerfile` | Path to Dockerfile | `Dockerfile` |
| `ephemeral-namespace-run-script` | Test script path | `tests/k8s/konflux-test-runner.sh` |
| `ephemeral-pod-wait-timeout` | Test timeout | `10m` |

## Shared Pipeline Integration

These PipelineRuns reference the shared pipeline via git resolver:

```yaml
pipelineRef:
  params:
  - name: url
    value: https://github.com/app-sre/shared-pipelines
  - name: revision
    value: main
  - name: pathInRepo
    value: pipelines/multi-arch-build-pipeline.yaml
  resolver: git
```

The shared pipeline provides:
- Container image building with buildah
- Security scans (Clair, Snyk, ClamAV)
- SBOM generation
- Image signing via Sigstore
- Ephemeral namespace provisioning for testing

## Requirements

- Namespace: `app-sre-tenant`
- Service Account: `build-pipeline-vault-manager-master`
- Git auth secret: Provided by Konflux
- Image registry: `quay.io/app-sre/` (requires push access)

## Monitoring Pipeline Runs

View pipeline runs in Konflux UI:
```
https://konflux-ui.apps.stone-prd-rh01.pg1f.p1.openshiftapps.com/ns/app-sre-tenant/applications/vault-manager-master
```

Or via CLI:
```bash
oc get pipelineruns -n app-sre-tenant -l appstudio.openshift.io/component=vault-manager-master
```

## Troubleshooting

### Pipeline fails at test stage

Check ephemeral namespace logs:
```bash
oc get pipelinerun -n app-sre-tenant | grep vault-manager
oc logs <pipelinerun-pod> -n app-sre-tenant -c step-run-custom-script-in-ephemeral-namespace
```

### Services not starting in ephemeral namespace

The test runner includes detailed logging. Check:
1. Keycloak startup (can take 1-2 minutes)
2. Vault initialization completion
3. Network connectivity between services

### Test timeouts

Default timeout is 10 minutes. Increase if needed:
```yaml
- name: ephemeral-pod-wait-timeout
  value: "15m"
```

## References

- [Konflux Documentation](https://konflux.pages.redhat.com/docs/)
- [shared-pipelines Repository](https://github.com/app-sre/shared-pipelines)
- [Tekton Pipelines](https://tekton.dev/docs/pipelines/)
- [PipelinesAsCode](https://pipelinesascode.com/)
