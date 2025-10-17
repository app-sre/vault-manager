#!/bin/bash
set -euo pipefail

# Konflux Ephemeral Namespace Test Runner
# This script runs in an ephemeral OpenShift namespace with kubectl/oc available
# Environment variables available:
# - IMAGE_URL: The built container image
# - IMAGE_DIGEST: The image digest
# - KUBECONFIG: Already configured to access the ephemeral namespace

echo "=========================================="
echo "Running vault-manager integration tests"
echo "=========================================="
echo "Testing image: ${IMAGE_URL}"
echo "Image digest: ${IMAGE_DIGEST}"
echo ""

# Get the script directory (where the K8s manifests are located)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "1. Deploying Keycloak..."
oc apply -f "${SCRIPT_DIR}/keycloak.yaml"
echo "   Waiting for Keycloak to be ready (this may take a few minutes)..."
oc wait --for=condition=Ready pod -l app=keycloak --timeout=10m

echo ""
echo "2. Configuring Keycloak..."
# Create ConfigMap from keycloak config files
oc create configmap keycloak-config-files \
  --from-file="${SCRIPT_DIR}/../keycloak/"

oc apply -f "${SCRIPT_DIR}/keycloak-config-job.yaml"
echo "   Waiting for Keycloak configuration to complete..."
oc wait --for=condition=Complete job/keycloak-config --timeout=3m || {
  echo "   Keycloak config job failed or timed out. Checking status..."
  oc get job keycloak-config
  oc get pods -l job-name=keycloak-config
  echo "   Job logs:"
  oc logs -l job-name=keycloak-config --tail=50 || echo "No logs available"
  echo "   WARNING: Keycloak configuration failed, continuing with default config..."
}

echo ""
echo "3. Deploying qontract-server..."
# Create ConfigMap with app-interface data from the cloned repository
# In Konflux, the source code is available in the workspace
if [ -f "${SCRIPT_DIR}/../app-interface/data.json" ]; then
  echo "   Found app-interface data at ${SCRIPT_DIR}/../app-interface/data.json"
  echo "   File size: $(stat -f%z "${SCRIPT_DIR}/../app-interface/data.json" 2>/dev/null || stat -c%s "${SCRIPT_DIR}/../app-interface/data.json") bytes"
  echo "   Creating app-interface-data ConfigMap from repository..."
  oc create configmap app-interface-data \
    --from-file=data.json="${SCRIPT_DIR}/../app-interface/data.json" || {
      echo "   ERROR: Failed to create ConfigMap"
      ls -lh "${SCRIPT_DIR}/../app-interface/"
      exit 1
    }

  # Deploy qontract-server using ConfigMap
  oc apply -f "${SCRIPT_DIR}/qontract-server-konflux.yaml"
  echo "   Waiting for qontract-server to be ready..."
  oc wait --for=condition=Ready pod -l app=qontract-server --timeout=3m || {
    echo "   ERROR: qontract-server failed to become ready"
    echo "   Pod status:"
    oc get pod -l app=qontract-server
    echo ""
    echo "   Pod events:"
    oc describe pod -l app=qontract-server | tail -40
    echo ""
    echo "   Container logs:"
    oc logs -l app=qontract-server --all-containers=true --tail=50 || echo "No logs available"
    echo ""
    echo "   ConfigMap info:"
    oc get configmap app-interface-data
    exit 1
  }
else
  echo "   ERROR: app-interface/data.json not found at ${SCRIPT_DIR}/../app-interface/data.json"
  echo "   Available files in source:"
  ls -la "${SCRIPT_DIR}/../" || true
  ls -la "${SCRIPT_DIR}/../app-interface/" || true
  exit 1
fi

echo ""
echo "4. Deploying Vault instances..."
oc apply -f "${SCRIPT_DIR}/vault-primary.yaml"
oc apply -f "${SCRIPT_DIR}/vault-secondary.yaml"
echo "   Waiting for Vault instances to be ready..."
oc wait --for=condition=Ready pod -l app=primary-vault --timeout=3m || {
  echo "   ERROR: primary-vault failed to become ready"
  echo "   Pod status:"
  oc get pod -l app=primary-vault
  echo ""
  echo "   Pod events:"
  oc describe pod -l app=primary-vault | tail -40
  echo ""
  echo "   Container logs:"
  oc logs -l app=primary-vault --tail=50 || echo "No logs available"
  exit 1
}
oc wait --for=condition=Ready pod -l app=secondary-vault --timeout=3m || {
  echo "   ERROR: secondary-vault failed to become ready"
  echo "   Pod status:"
  oc get pod -l app=secondary-vault
  echo ""
  echo "   Pod events:"
  oc describe pod -l app=secondary-vault | tail -40
  echo ""
  echo "   Container logs:"
  oc logs -l app=secondary-vault --tail=50 || echo "No logs available"
  exit 1
}

echo ""
echo "5. Initializing Vault secrets..."
oc apply -f "${SCRIPT_DIR}/vault-init-job.yaml"
oc wait --for=condition=Complete job/vault-init --timeout=2m

echo ""
echo "=========================================="
echo "All services deployed successfully!"
echo "=========================================="
echo ""

# Display service status
echo "Service Status:"
oc get pods,svc

echo ""
echo "Vault initialization logs:"
oc logs job/vault-init | tail -10

echo ""
echo "=========================================="
echo "Running BATS integration tests..."
echo "=========================================="
echo ""

# Deploy the test runner pod with the built image
# Replace IMAGE_PLACEHOLDER with the actual image
sed "s|IMAGE_PLACEHOLDER|${IMAGE_URL}@${IMAGE_DIGEST}|g" "${SCRIPT_DIR}/test-runner-konflux.yaml" | oc apply -f -

echo "Waiting for tests to complete (timeout: 8 minutes)..."
oc wait --for=condition=Ready pod/vault-manager-test --timeout=8m || {
  echo "Test pod failed to become ready. Checking status..."
  oc get pod vault-manager-test
  echo ""
  echo "Container logs:"
  oc logs vault-manager-test || echo "No logs available"
  echo ""
  echo "Pod details:"
  oc describe pod vault-manager-test | tail -20
  exit 1
}

echo ""
echo "Test pod is running. Streaming logs..."
oc logs -f vault-manager-test

# Check test results
TEST_EXIT_CODE=$(oc get pod vault-manager-test -o jsonpath='{.status.containerStatuses[0].state.terminated.exitCode}')

echo ""
echo "=========================================="
if [ "$TEST_EXIT_CODE" = "0" ]; then
  echo "✅ All tests passed successfully!"
  echo "=========================================="
  exit 0
else
  echo "❌ Tests failed with exit code: $TEST_EXIT_CODE"
  echo "=========================================="
  exit 1
fi
