# Kubernetes Test Manifests for vault-manager

This directory contains Kubernetes manifests for running vault-manager integration tests in a local or remote Kubernetes cluster.

## Prerequisites

- A running Kubernetes cluster (kind, minikube, or remote cluster)
- `kubectl` CLI configured to access the cluster
- Ability to pull container images from quay.io

## Important: Container Images

This setup uses publicly available images where possible:
- **Keycloak**: `quay.io/keycloak/keycloak:22.0.4` (public)
- **Vault**:
  - Local/DockerHub: `hashicorp/vault:1.17.1` (public)
  - Red Hat Catalog: `registry.connect.redhat.com/hashicorp/vault:1.17.1` (certified)
  - App-SRE (Konflux): `quay.io/app-sre/vault:1.17.1` (internal, used in production)
- **qontract-server**: `quay.io/app-sre/qontract-server:ed1f3d5` (requires app-sre access)
- **keycloak-config-cli**: `quay.io/app-sre/keycloak-config-cli:5.11.0-22.0.4` (requires app-sre access)

### Image Selection by Environment

**Local testing (kind/minikube):**
- Use `hashicorp/vault:1.17.1` from DockerHub (no auth needed)
- qontract-server won't work without app-sre access (use placeholder)

**Konflux/Production:**
- Use `quay.io/app-sre/vault:1.17.1` (available in Konflux environment)
- All app-sre images will be accessible

If you have access to the app-sre Quay organization, you can authenticate:
```bash
docker login quay.io
kubectl create secret docker-registry quay-auth \
  --docker-server=quay.io \
  --docker-username=YOUR_USERNAME \
  --docker-password=YOUR_PASSWORD \
  -n vault-manager-test
```

Then add `imagePullSecrets` to the pod specs.

## Quick Start

### 1. Create a local kind cluster (optional)

```bash
kind create cluster --name vault-manager-test
```

### 2. Deploy all test services

```bash
./tests/k8s/deploy-services.sh
```

This will:
- Create a namespace `vault-manager-test`
- Deploy Keycloak, qontract-server, and two Vault instances
- Initialize Vault with required secrets
- Wait for all services to be ready

### 3. Run the tests

```bash
kubectl apply -f tests/k8s/test-runner-pod.yaml
kubectl logs -f vault-manager-test
```

## Manual Deployment

If you prefer to deploy services individually:

```bash
# Set namespace
kubectl create namespace vault-manager-test
kubectl config set-context --current --namespace=vault-manager-test

# Deploy Keycloak
kubectl apply -f tests/k8s/keycloak.yaml
kubectl wait --for=condition=Ready pod/keycloak --timeout=5m

# Configure Keycloak (after updating configmap with your keycloak configs)
kubectl create configmap keycloak-config-files --from-file=tests/keycloak/
kubectl apply -f tests/k8s/keycloak-config-job.yaml

# Deploy qontract-server
kubectl create configmap app-interface-data --from-file=data.json=tests/app-interface/data.json
kubectl apply -f tests/k8s/qontract-server.yaml

# Deploy Vault instances
kubectl apply -f tests/k8s/vault-primary.yaml
kubectl apply -f tests/k8s/vault-secondary.yaml

# Initialize Vault secrets
kubectl apply -f tests/k8s/vault-init-job.yaml
```

## Port Forwarding for Local Access

To access services from your local machine:

```bash
# Keycloak
kubectl port-forward svc/keycloak 8180:8180 &

# qontract-server
kubectl port-forward svc/qontract-server 4000:4000 &

# Primary Vault
kubectl port-forward svc/primary-vault 8200:8200 &

# Secondary Vault
kubectl port-forward svc/secondary-vault 8202:8202 &
```

## Testing with Your Local Build

To test with a locally built image:

```bash
# Build and load into kind
docker build -t vault-manager-test:local -f tests/Dockerfile.tests .
kind load docker-image vault-manager-test:local --name vault-manager-test

# Update test-runner-pod.yaml to use vault-manager-test:local
# Then run:
kubectl apply -f tests/k8s/test-runner-pod.yaml
kubectl logs -f vault-manager-test
```

## Cleanup

```bash
kubectl delete namespace vault-manager-test

# Or if using kind:
kind delete cluster --name vault-manager-test
```

## Differences from podman-compose

- Uses Kubernetes Pods instead of compose services
- Uses ConfigMaps for configuration data
- Uses Jobs for one-time initialization tasks
- Uses Services for inter-pod networking
- Health checks use Kubernetes probes (livenessProbe/readinessProbe)

## Troubleshooting

### Pods not starting

```bash
kubectl get pods
kubectl describe pod <pod-name>
kubectl logs <pod-name>
```

### Services not accessible

```bash
kubectl get svc
kubectl describe svc <service-name>
```

### View job logs

```bash
kubectl logs job/keycloak-config
kubectl logs job/vault-init
```

### Debug in a pod

```bash
kubectl exec -it keycloak -- bash
kubectl exec -it primary-vault -- sh
```
