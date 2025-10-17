# Konflux Integration Summary

This document summarizes the Konflux integration for vault-manager using shared-pipelines.

## What's Been Created

### 1. Tekton Pipeline Configurations (`.tekton/`)

- **vault-manager-master-push.yaml** - Triggers on master branch pushes
- **vault-manager-pull-request.yaml** - Triggers on pull requests
- **README.md** - Complete Konflux documentation

### 2. Kubernetes Test Manifests (`tests/k8s/`)

- **keycloak.yaml** - Keycloak authentication server
- **keycloak-config-job.yaml** - Keycloak realm configuration
- **qontract-server.yaml** - GraphQL API server
- **vault-primary.yaml** - Primary Vault instance
- **vault-secondary.yaml** - Secondary Vault instance
- **vault-init-job.yaml** - Vault secret initialization
- **deploy-services.sh** - Automated deployment script
- **cleanup.sh** - Environment cleanup script
- **README.md** - Local testing documentation

### 3. Test Runner Script

- **konflux-test-runner.sh** - Deploys infrastructure and runs BATS tests in Konflux ephemeral namespaces

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Konflux Shared Pipeline                       │
│  (https://github.com/app-sre/shared-pipelines)                  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │ Referenced via Git Resolver
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│              vault-manager PipelineRun                           │
│  (.tekton/vault-manager-master-push.yaml)                       │
│                                                                  │
│  Parameters:                                                     │
│  - ephemeral-namespace-run-script: tests/k8s/konflux-...sh     │
│  - ephemeral-pod-wait-timeout: 10m                              │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │ Executes in Ephemeral Namespace
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│           Ephemeral Kubernetes Namespace                         │
│                                                                  │
│  1. Deploy Keycloak (auth server)                               │
│  2. Configure Keycloak realm                                    │
│  3. Deploy qontract-server (GraphQL)                            │
│  4. Deploy Primary & Secondary Vault                            │
│  5. Initialize Vault secrets                                    │
│  6. Run BATS test suite                                         │
└─────────────────────────────────────────────────────────────────┘
```

## Test Flow

1. **Trigger**: Git push or PR creation
2. **Build**: Shared pipeline builds container image
3. **Provision**: Ephemeral namespace created
4. **Deploy**: `konflux-test-runner.sh` deploys test infrastructure
5. **Test**: BATS tests run against deployed services
6. **Report**: Results sent back to pipeline
7. **Cleanup**: Ephemeral namespace automatically deleted

## Key Differences: Local vs Konflux

| Aspect | Local (kind) | Konflux |
|--------|--------------|---------|
| **Images** | Public images (hashicorp/vault) | app-sre images (quay.io/app-sre/) |
| **Data Access** | Limited (ConfigMap size) | Full git repo available |
| **Networking** | ClusterIP services | ClusterIP services |
| **Persistence** | Manual cleanup needed | Auto-deleted after tests |
| **Execution** | `./deploy-services.sh` | `konflux-test-runner.sh` |

## Local Testing Validated

Successfully tested locally with kind:

```bash
✅ Keycloak - Running
✅ Keycloak Config - Completed
✅ Primary Vault - Running
✅ Secondary Vault - Running
✅ Vault Init - Completed
```

**Services Deployed:**
- keycloak:8180
- primary-vault:8200
- secondary-vault:8202

## Next Steps

### To Deploy in Konflux:

1. **Commit the changes:**
   ```bash
   git add .tekton/ tests/k8s/
   git commit -m "Add Konflux integration with shared-pipelines"
   git push origin master
   ```

2. **Configure Konflux:**
   - Ensure namespace `app-sre-tenant` exists
   - Service account `build-pipeline-vault-manager-master` has correct permissions
   - Git auth secret is configured
   - Quay.io push access is set up

3. **Create a test PR:**
   - Pipeline will automatically trigger
   - Watch progress in Konflux UI
   - Ephemeral namespace tests will run

4. **Monitor:**
   ```bash
   oc get pipelineruns -n app-sre-tenant | grep vault-manager
   oc logs <pipelinerun-pod> -n app-sre-tenant -f
   ```

## Troubleshooting

### Issue: qontract-server not starting in local tests

**Solution:** qontract-server requires actual app-interface data which is too large for ConfigMaps in local testing. In Konflux, the git repo is cloned so the file is available.

### Issue: Keycloak takes too long to start

**Solution:** Increase `ephemeral-pod-wait-timeout` parameter:
```yaml
- name: ephemeral-pod-wait-timeout
  value: "15m"
```

### Issue: Tests timeout

**Check:**
1. Service readiness probes
2. Network connectivity between pods
3. Resource limits (memory/CPU)

## References

- **Shared Pipelines**: https://github.com/app-sre/shared-pipelines
- **Konflux Docs**: https://konflux.pages.redhat.com/docs/
- **Local Testing**: `tests/k8s/README.md`
- **Pipeline Config**: `.tekton/README.md`

