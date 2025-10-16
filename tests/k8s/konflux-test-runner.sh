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
  echo "   Creating app-interface-data ConfigMap from repository..."
  oc create configmap app-interface-data \
    --from-file=data.json="${SCRIPT_DIR}/../app-interface/data.json"

  # Update qontract-server manifest to use ConfigMap instead of emptyDir
  cat > /tmp/qontract-server-konflux.yaml <<EOF
---
apiVersion: v1
kind: Pod
metadata:
  name: qontract-server
  labels:
    app: qontract-server
spec:
  containers:
  - name: qontract-server
    image: quay.io/app-sre/qontract-server:latest
    ports:
    - containerPort: 4000
      name: http
    env:
    - name: LOAD_METHOD
      value: fs
    - name: DATAFILES_FILE
      value: /bundle/data.json
    volumeMounts:
    - name: data
      mountPath: /bundle
      readOnly: true
    livenessProbe:
      httpGet:
        path: /healthz
        port: 4000
      initialDelaySeconds: 10
      periodSeconds: 10
    readinessProbe:
      httpGet:
        path: /healthz
        port: 4000
      initialDelaySeconds: 5
      periodSeconds: 5
    resources:
      requests:
        memory: "256Mi"
        cpu: "100m"
      limits:
        memory: "512Mi"
        cpu: "200m"
  volumes:
  - name: data
    configMap:
      name: app-interface-data
---
apiVersion: v1
kind: Service
metadata:
  name: qontract-server
  labels:
    app: qontract-server
spec:
  selector:
    app: qontract-server
  ports:
  - name: http
    port: 4000
    targetPort: 4000
    protocol: TCP
  type: ClusterIP
EOF

  oc apply -f /tmp/qontract-server-konflux.yaml
  echo "   Waiting for qontract-server to be ready..."
  oc wait --for=condition=Ready pod -l app=qontract-server --timeout=2m || {
    echo "   qontract-server failed to become ready. Checking status..."
    oc get pod -l app=qontract-server
    oc describe pod -l app=qontract-server | tail -30
    echo "   Pod logs:"
    oc logs -l app=qontract-server --tail=30 || echo "No logs available"
    echo "   WARNING: qontract-server failed, tests may not have GraphQL data available"
  }
else
  echo "   (Skipping - app-interface data not found)"
  echo "   WARNING: Tests will not have qontract-server available"
fi

echo ""
echo "4. Deploying Vault instances..."
oc apply -f "${SCRIPT_DIR}/vault-primary.yaml"
oc apply -f "${SCRIPT_DIR}/vault-secondary.yaml"
echo "   Waiting for Vault instances to be ready..."
oc wait --for=condition=Ready pod -l app=primary-vault --timeout=2m
oc wait --for=condition=Ready pod -l app=secondary-vault --timeout=2m

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
cat > /tmp/test-runner-konflux.yaml <<EOF
---
apiVersion: v1
kind: Pod
metadata:
  name: vault-manager-test
  labels:
    app: vault-manager-test
spec:
  restartPolicy: Never
  containers:
  - name: test
    image: ${IMAGE_URL}@${IMAGE_DIGEST}
    env:
    - name: PRIMARY_VAULT_URL
      value: http://primary-vault:8200
    - name: SECONDARY_VAULT_URL
      value: http://secondary-vault:8202
    - name: KEYCLOAK_URL
      value: http://keycloak:8180
    - name: GRAPHQL_SERVER
      value: http://qontract-server:4000/graphql
    - name: PODMAN_IGNORE_CGROUPSV1_WARNING
      value: "1"
    command: ["/bin/bash", "-c"]
    args:
    - |
      set -e

      echo "Waiting for services to be ready..."
      for url in "\${KEYCLOAK_URL}/realms/master" \
                 "http://qontract-server:4000/healthz" \
                 "\${PRIMARY_VAULT_URL}/v1/sys/health" \
                 "\${SECONDARY_VAULT_URL}/v1/sys/health"; do
        until curl -s -f -o /dev/null "\$url"; do
          echo "Waiting for \$url..."
          sleep 5
        done
      done

      echo "All services ready. Running BATS tests..."
      cd /tests

      # Run test suite
      for test in \$(find /tests/bats/ -type f | grep '\\.bats' | grep -vE 'roles|entities|groups|errors'); do
        echo "Running \$test"
        bats --tap "\$test"
      done

      echo "Running roles tests"
      bats --tap /tests/bats/roles/roles.bats

      echo "Running entities tests"
      bats --tap /tests/bats/entities/entities.bats

      echo "Running groups tests"
      bats --tap /tests/bats/groups/groups.bats

      echo "Running error handling tests"
      bats --tap /tests/bats/errors/errors.bats

      echo "All tests completed successfully!"
    resources:
      requests:
        memory: "512Mi"
        cpu: "200m"
      limits:
        memory: "1Gi"
        cpu: "500m"
EOF

oc apply -f /tmp/test-runner-konflux.yaml

echo "Waiting for tests to complete (timeout: 8 minutes)..."
oc wait --for=condition=Ready pod/vault-manager-test --timeout=8m || {
  echo "Test pod failed to become ready. Checking status..."
  oc get pod vault-manager-test
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
