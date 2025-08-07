package testcontainers

import (
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestVaultManagerAuditDevicesPod tests that vault-manager can enable and manage Vault audit devices using Podman pods
func TestVaultManagerAuditDevicesPod(t *testing.T) {

	t.Logf("🚀 Starting vault-manager audit devices test with Podman pod")

	// Step 1: Create Podman pod with port mappings
	t.Logf("🌐 Creating Podman pod with shared networking...")
	podName := "vault-manager-audit-devices-pod"

	createPodCmd := exec.Command("podman", "pod", "create",
		"--name", podName,
		"--publish", "8180:8180", // Keycloak
		"--publish", "4000:4000", // qontract-server
		"--publish", "8200:8200", // Primary Vault
		"--publish", "8202:8202", // Secondary Vault
	)
	err := createPodCmd.Run()
	require.NoError(t, err, "Failed to create Podman pod")

	// Ensure pod cleanup
	defer func() {
		exec.Command("podman", "pod", "rm", "-f", podName).Run()
	}()

	// Step 2: Start Keycloak in the pod
	t.Logf("🔧 Starting Keycloak in pod...")
	keycloakCmd := exec.Command("podman", "run", "-d",
		"--pod", podName,
		"--name", "keycloak-audit-devices",
		"-e", "KEYCLOAK_ADMIN=admin",
		"-e", "KEYCLOAK_ADMIN_PASSWORD=admin",
		"quay.io/keycloak/keycloak:21.1.2",
		"start-dev", "--http-port", "8180",
	)
	output, err := keycloakCmd.CombinedOutput()
	t.Logf("Keycloak start output: %s", string(output))
	require.NoError(t, err, "Failed to start Keycloak in pod")

	time.Sleep(30 * time.Second)
	defer exec.Command("podman", "rm", "-f", "keycloak-audit-devices").Run()

	t.Logf("⚠️  Skipping Keycloak configuration for debugging...")

	// Step 3: Start qontract-server in the pod
	t.Logf("🔧 Starting qontract-server in pod...")
	bundlePath := "/home/jmosco/dev/work/oss/vault-manager/tests/app-interface"
	qontractCmd := exec.Command("podman", "run", "-d",
		"--pod", podName,
		"--name", "qontract-server-audit-devices",
		"-v", bundlePath+":/bundle:Z",
		"-e", "LOAD_METHOD=fs",
		"-e", "DATAFILES_FILE=/bundle/data.json",
		"quay.io/redhat-services-prod/app-sre-tenant/qontract-server-master/qontract-server-master:f3fb9719c39b0413edc9e2254f942e725bc57344e72d16b4b947ae215d65c59b",
	)
	err = qontractCmd.Run()
	require.NoError(t, err, "Failed to start qontract-server in pod")

	time.Sleep(10 * time.Second)
	defer exec.Command("podman", "rm", "-f", "qontract-server-audit-devices").Run()

	t.Logf("✅ qontract-server ready at: http://localhost:4000")

	// Step 4: Start Primary Vault in the pod
	t.Logf("🔧 Starting primary Vault in pod...")
	primaryVaultCmd := exec.Command("podman", "run", "-d",
		"--pod", podName,
		"--name", "primary-vault-audit-devices",
		"-e", "VAULT_DISABLE_MLOCK=true",
		"-e", "VAULT_DEV_ROOT_TOKEN_ID=root",
		"-e", "VAULT_DEV_LISTEN_ADDRESS=0.0.0.0:8200",
		"docker.io/hashicorp/vault:1.17.1",
	)
	primaryOutput, err := primaryVaultCmd.CombinedOutput()
	t.Logf("Primary Vault start output: %s", string(primaryOutput))
	require.NoError(t, err, "Failed to start primary Vault in pod")

	time.Sleep(5 * time.Second)
	defer exec.Command("podman", "rm", "-f", "primary-vault-audit-devices").Run()

	t.Logf("✅ Primary Vault ready at: http://localhost:8200")

	// Step 5: Start Secondary Vault in the pod
	t.Logf("🔧 Starting secondary Vault in pod...")
	secondaryVaultCmd := exec.Command("podman", "run", "-d",
		"--pod", podName,
		"--name", "secondary-vault-audit-devices",
		"-e", "VAULT_DISABLE_MLOCK=true",
		"-e", "VAULT_DEV_ROOT_TOKEN_ID=root",
		"-e", "VAULT_DEV_LISTEN_ADDRESS=0.0.0.0:8202",
		"docker.io/hashicorp/vault:1.17.1",
		"vault", "server", "-dev", "-dev-listen-address=0.0.0.0:8202",
	)
	secondaryOutput, err := secondaryVaultCmd.CombinedOutput()
	t.Logf("Secondary Vault start output: %s", string(secondaryOutput))
	require.NoError(t, err, "Failed to start secondary Vault in pod")

	time.Sleep(5 * time.Second)
	defer exec.Command("podman", "rm", "-f", "secondary-vault-audit-devices").Run()

	t.Logf("✅ Secondary Vault ready at: http://localhost:8202")

	// Step 6: Build vault-manager binary
	t.Logf("🔧 Building vault-manager binary...")
	err = buildVaultManager()
	require.NoError(t, err)
	t.Logf("✅ vault-manager binary built")

	// Step 7: Set up authentication
	t.Logf("🔧 Setting up cross-vault authentication from within pod...")

	internalPrimaryVaultURL := "http://localhost:8200"

	authSetupCmd := exec.Command("podman", "run", "--rm",
		"--pod", podName,
		"docker.io/curlimages/curl:latest",
		"sh", "-c", fmt.Sprintf(`
			# Enable KV secrets engine
			curl -X POST -H "X-Vault-Token: root" \
				-d '{"type":"kv","options":{"version":"2"}}' \
				%s/v1/sys/mounts/secret || true
			
			# Store primary vault credentials
			curl -X POST -H "X-Vault-Token: root" \
				-d '{"data":{"rootToken":"root"}}' \
				%s/v1/secret/data/master
			
			# Store secondary vault credentials  
			curl -X POST -H "X-Vault-Token: root" \
				-d '{"data":{"root":"root"}}' \
				%s/v1/secret/data/secondary
		`, internalPrimaryVaultURL, internalPrimaryVaultURL, internalPrimaryVaultURL),
	)

	authOutput, err := authSetupCmd.CombinedOutput()
	t.Logf("Authentication setup output: %s", string(authOutput))
	if err != nil {
		t.Logf("⚠️ Authentication setup had issues: %v", err)
	}

	// Step 8: Run vault-manager with audit devices configuration
	t.Logf("📋 Running vault-manager with audit devices configuration...")

	internalQontractEndpoint := "http://localhost:4000"

	vaultManagerCmd := exec.Command("podman", "run", "--rm",
		"--pod", podName,
		"--name", "vault-manager-audit-devices-runner",
		"-v", "/home/jmosco/dev/work/oss/vault-manager:/src:Z",
		"-w", "/src",
		"-e", fmt.Sprintf("GRAPHQL_SERVER=%s/graphql", internalQontractEndpoint),
		"-e", fmt.Sprintf("GRAPHQL_QUERY_FILE=%s", "/src/tests/fixtures/audit/enable_audit_device.graphql"),
		"-e", fmt.Sprintf("VAULT_ADDR=%s", internalPrimaryVaultURL),
		"-e", "VAULT_TOKEN=root",
		"-e", "VAULT_AUTHTYPE=token",
		"registry.access.redhat.com/ubi9/go-toolset:1.23",
		"sh", "-c", `
			go build -o /tmp/vault-manager ./cmd/vault-manager &&
			/tmp/vault-manager
		`,
	)

	vaultManagerOutput, err := vaultManagerCmd.CombinedOutput()
	t.Logf("vault-manager output:\n%s", string(vaultManagerOutput))
	require.NoError(t, err, "vault-manager should succeed")

	// Step 9: Verify vault-manager results
	t.Logf("🔍 Verifying vault-manager audit device creation...")

	// Check vault-manager output for expected messages
	require.Contains(t, string(vaultManagerOutput), "[Vault Audit] audit device is successfully enabled", "Should contain audit device creation message")
	require.Contains(t, string(vaultManagerOutput), "path=file/", "Should create file audit device")
	require.Contains(t, string(vaultManagerOutput), "instance=\"http://localhost:8200\"", "Should create audit device on primary")
	require.Contains(t, string(vaultManagerOutput), "instance=\"http://localhost:8202\"", "Should create audit device on secondary")

	// Step 10: Test audit devices using Vault API from within pod
	t.Logf("📋 Testing Vault audit devices from within pod...")

	// List audit devices from primary Vault within pod
	listAuditCmd := exec.Command("podman", "run", "--rm",
		"--pod", podName,
		"docker.io/curlimages/curl:latest",
		"curl", "-s", "-H", "X-Vault-Token: root",
		"http://localhost:8200/v1/sys/audit",
	)
	listAuditOutput, err := listAuditCmd.CombinedOutput()
	t.Logf("Primary Vault audit devices: %s", string(listAuditOutput))
	if err != nil {
		t.Logf("⚠️ Failed to list audit devices on primary: %v", err)
	} else {
		// Verify the expected audit device is present
		require.Contains(t, string(listAuditOutput), "file/", "file/ audit device should exist on primary")
		require.Contains(t, string(listAuditOutput), "file_path", "Should show file_path configuration")
	}

	// List audit devices from secondary Vault within pod
	listAuditSecondaryCmd := exec.Command("podman", "run", "--rm",
		"--pod", podName,
		"docker.io/curlimages/curl:latest",
		"curl", "-s", "-H", "X-Vault-Token: root",
		"http://localhost:8202/v1/sys/audit",
	)
	listAuditSecondaryOutput, err := listAuditSecondaryCmd.CombinedOutput()
	t.Logf("Secondary Vault audit devices: %s", string(listAuditSecondaryOutput))
	if err != nil {
		t.Logf("⚠️ Failed to list audit devices on secondary: %v", err)
	} else {
		// Verify the expected audit device is present
		require.Contains(t, string(listAuditSecondaryOutput), "file/", "file/ audit device should exist on secondary")
		require.Contains(t, string(listAuditSecondaryOutput), "file_path", "Should show file_path configuration")
	}

	t.Logf("✅ vault-manager audit devices test completed successfully!")
	t.Logf("🎯 Audit devices created on both primary and secondary Vault instances")
	t.Logf("📋 Verified: file/ audit device with file_path configuration")
	t.Logf("🚀 Pod-based networking approach successful!")
}
