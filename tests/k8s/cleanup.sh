#!/bin/bash
set -euo pipefail

# Script to cleanup vault-manager test environment
# Usage: ./cleanup.sh [namespace]

NAMESPACE="${1:-vault-manager-test}"

echo "Cleaning up namespace: ${NAMESPACE}"

# Delete the namespace (this will delete everything in it)
kubectl delete namespace "${NAMESPACE}" --ignore-not-found=true

echo "Cleanup complete!"
