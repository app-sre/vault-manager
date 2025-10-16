#!/bin/bash
set -euo pipefail

# Script to deploy vault-manager test services in a local Kubernetes cluster
# Usage: ./deploy-services.sh [namespace]
#
# Requirements:
# - kubectl or oc CLI
# - kind or minikube cluster running
# - Access to pull required container images

NAMESPACE="${1:-vault-manager-test}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "Deploying vault-manager test services to namespace: ${NAMESPACE}"

# Create namespace if it doesn't exist
kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

# Switch to the namespace
kubectl config set-context --current --namespace="${NAMESPACE}"

echo "1. Deploying Keycloak..."
kubectl apply -f "${SCRIPT_DIR}/keycloak.yaml"
echo "   Waiting for Keycloak to be ready (this may take a few minutes)..."
kubectl wait --for=condition=Ready pod -l app=keycloak --timeout=10m || {
  echo "   Keycloak didn't become ready in time. Checking status..."
  kubectl get pod -l app=keycloak
  kubectl logs -l app=keycloak --tail=20
  exit 1
}

echo "2. Configuring Keycloak..."
# Create ConfigMap from keycloak config files
kubectl create configmap keycloak-config-files \
  --from-file="${SCRIPT_DIR}/../keycloak/" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl apply -f "${SCRIPT_DIR}/keycloak-config-job.yaml"
echo "   Waiting for Keycloak configuration to complete..."
kubectl wait --for=condition=Complete job/keycloak-config --timeout=3m

echo "3. Deploying qontract-server..."
echo "   (Skipping - requires valid app-interface data bundle)"
# Note: qontract-server needs actual app-interface data.json which is too large for ConfigMap
# In Konflux, this would be available from the git repository in the ephemeral namespace
# kubectl apply -f "${SCRIPT_DIR}/qontract-server.yaml"
# kubectl wait --for=condition=Ready pod -l app=qontract-server --timeout=2m

echo "4. Deploying Vault instances..."
kubectl apply -f "${SCRIPT_DIR}/vault-primary.yaml"
kubectl apply -f "${SCRIPT_DIR}/vault-secondary.yaml"
echo "   Waiting for Vault instances to be ready..."
kubectl wait --for=condition=Ready pod -l app=primary-vault --timeout=2m
kubectl wait --for=condition=Ready pod -l app=secondary-vault --timeout=2m

echo "5. Initializing Vault secrets..."
kubectl apply -f "${SCRIPT_DIR}/vault-init-job.yaml"
kubectl wait --for=condition=Complete job/vault-init --timeout=1m

echo ""
echo "All services deployed successfully!"
echo ""
echo "Service URLs (from within cluster):"
echo "  - Keycloak:          http://keycloak:8180"
echo "  - qontract-server:   http://qontract-server:4000"
echo "  - Primary Vault:     http://primary-vault:8200"
echo "  - Secondary Vault:   http://secondary-vault:8202"
echo ""
echo "To access services locally, use port-forwarding:"
echo "  kubectl port-forward svc/keycloak 8180:8180"
echo "  kubectl port-forward svc/qontract-server 4000:4000"
echo "  kubectl port-forward svc/primary-vault 8200:8200"
echo "  kubectl port-forward svc/secondary-vault 8202:8202"
echo ""
echo "To run tests:"
echo "  kubectl apply -f ${SCRIPT_DIR}/test-runner-pod.yaml"
echo "  kubectl logs -f vault-manager-test"
