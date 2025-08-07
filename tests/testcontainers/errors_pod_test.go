package testcontainers

import (
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestVaultManagerErrorsPod tests that vault-manager handles errors gracefully using Podman pods
func TestVaultManagerErrorsPod(t *testing.T) {

	t.Logf("🚀 Starting vault-manager errors test with Podman pod")

	// Step 1: Create Podman pod with port mappings
	t.Logf("🌐 Creating Podman pod with shared networking...")
	podName := "vault-manager-errors-pod"
	
	createPodCmd := exec.Command("podman", "pod", "create", 
		"--name", podName,
		"--publish", "8180:8180",  // Keycloak
		"--publish", "4000:4000",  // qontract-server  
		"--publish", "8200:8200",  // Primary Vault
		"--publish", "8202:8202",  // Secondary Vault
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
		"--name", "keycloak-errors",
		"-e", "KEYCLOAK_ADMIN=admin",
		"-e", "KEYCLOAK_ADMIN_PASSWORD=admin",
		"quay.io/keycloak/keycloak:21.1.2",
		"start-dev", "--http-port", "8180",
	)
	output, err := keycloakCmd.CombinedOutput()
	t.Logf("Keycloak start output: %s", string(output))
	require.NoError(t, err, "Failed to start Keycloak in pod")
	
	time.Sleep(30 * time.Second)
	defer exec.Command("podman", "rm", "-f", "keycloak-errors").Run()

	// Step 2.5: Configure Keycloak realm and client
	t.Logf("🔧 Configuring Keycloak realm and client...")
	
	// Wait a bit more for Keycloak to be fully ready
	time.Sleep(30 * time.Second)
	
	// Configure Keycloak realm and OIDC client
	keycloakConfigCmd := exec.Command("podman", "run", "--rm",
		"--pod", podName,
		"docker.io/curlimages/curl:latest",
		"sh", "-c", `
			# Get admin token for Keycloak 21.x
			TOKEN=$(curl -s -X POST http://localhost:8180/realms/master/protocol/openid-connect/token \
				-H "Content-Type: application/x-www-form-urlencoded" \
				-d "username=admin&password=admin&grant_type=password&client_id=admin-cli" | \
				grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)
			
			echo "Admin token: $TOKEN"
			
			# Create test realm using Keycloak 21.x API paths
			curl -s -X POST http://localhost:8180/admin/realms \
				-H "Authorization: Bearer $TOKEN" \
				-H "Content-Type: application/json" \
				-d '{"realm":"test","enabled":true}'
			
			# Create vault client
			curl -s -X POST http://localhost:8180/admin/realms/test/clients \
				-H "Authorization: Bearer $TOKEN" \
				-H "Content-Type: application/json" \
				-d '{"clientId":"vault","enabled":true,"clientAuthenticatorType":"client-secret","secret":"dummy-oidc-client-secret-for-testing","redirectUris":["*"],"webOrigins":["*"],"publicClient":false}'
			
			# Verify the realm exists
			curl -s -X GET http://localhost:8180/realms/test/.well-known/openid_configuration
		`,
	)
	keycloakConfigOutput, err := keycloakConfigCmd.CombinedOutput()
	t.Logf("Keycloak configuration output: %s", string(keycloakConfigOutput))
	if err != nil {
		t.Logf("⚠️ Keycloak configuration had issues: %v", err)
	}

	// Step 3: Start qontract-server in the pod
	t.Logf("🔧 Starting qontract-server in pod...")
	bundlePath := "/home/jmosco/dev/work/oss/vault-manager/tests/app-interface"
	qontractCmd := exec.Command("podman", "run", "-d",
		"--pod", podName,
		"--name", "qontract-server-errors",
		"-v", bundlePath+":/bundle:Z",
		"-e", "LOAD_METHOD=fs",
		"-e", "DATAFILES_FILE=/bundle/data.json",
		"quay.io/redhat-services-prod/app-sre-tenant/qontract-server-master/qontract-server-master:f3fb9719c39b0413edc9e2254f942e725bc57344e72d16b4b947ae215d65c59b",
	)
	err = qontractCmd.Run()
	require.NoError(t, err, "Failed to start qontract-server in pod")
	
	time.Sleep(10 * time.Second)
	defer exec.Command("podman", "rm", "-f", "qontract-server-errors").Run()

	t.Logf("✅ qontract-server ready at: http://localhost:4000")

	// Step 4: Start Primary Vault in the pod
	t.Logf("🔧 Starting primary Vault in pod...")
	primaryVaultCmd := exec.Command("podman", "run", "-d",
		"--pod", podName,
		"--name", "primary-vault-errors",
		"-e", "VAULT_DISABLE_MLOCK=true",
		"-e", "VAULT_DEV_ROOT_TOKEN_ID=root",
		"-e", "VAULT_DEV_LISTEN_ADDRESS=0.0.0.0:8200",
		"docker.io/hashicorp/vault:1.17.1",
	)
	primaryOutput, err := primaryVaultCmd.CombinedOutput()
	t.Logf("Primary Vault start output: %s", string(primaryOutput))
	require.NoError(t, err, "Failed to start primary Vault in pod")
	
	time.Sleep(5 * time.Second)
	defer exec.Command("podman", "rm", "-f", "primary-vault-errors").Run()

	t.Logf("✅ Primary Vault ready at: http://localhost:8200")

	// Step 5: Start Secondary Vault in the pod
	t.Logf("🔧 Starting secondary Vault in pod...")
	secondaryVaultCmd := exec.Command("podman", "run", "-d",
		"--pod", podName,
		"--name", "secondary-vault-errors",
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
	defer exec.Command("podman", "rm", "-f", "secondary-vault-errors").Run()

	t.Logf("✅ Secondary Vault ready at: http://localhost:8202")

	// Step 6: Build vault-manager binary
	t.Logf("🔧 Building vault-manager binary...")
	err = buildVaultManager()
	require.NoError(t, err)
	t.Logf("✅ vault-manager binary built")

	// Step 7: Set up authentication and OIDC auth backends
	t.Logf("🔧 Setting up authentication and OIDC auth backends from within pod...")
	
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
			
			# Store Kubernetes CA certificate
			curl -X POST -H "X-Vault-Token: root" \
				-d '{"data":{"cert":"-----BEGIN CERTIFICATE-----\\nMIIBkTCB+wIJANkNYzxzkdcZMA0GCSqGSIb3DQEBCwUAMBQxEjAQBgNVBAMMCWxv\\nY2FsaG9zdDAeFw0yMTAxMDEwMDAwMDBaFw0yMjAxMDEwMDAwMDBaMBQxEjAQBgNV\\nBAMMCWxvY2FsaG9zdDBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQC2Z2gw+9gRAYJ0\\nFPbL+o3Z2gOjM2V7l4S8Z2gOCp1XQJ2S7M9L5T8Z3gOQN2V7Z5S8M2gOCp1XQJ2S\\n7M9L5T8ZAgMBAAEwDQYJKoZIhvcNAQELBQADQQAjM2V7l4S8Z2gOCp1XQJ2S7M9L\\n5T8Z3gOQN2V7Z5S8M2gOCp1XQJ2S7M9L5T8Z3gOQN2V7Z5S8M2gOCp1XQJ2S\\n-----END CERTIFICATE-----"}}' \
				%s/v1/secret/data/kubernetes
			
			# Store OIDC client secret
			curl -X POST -H "X-Vault-Token: root" \
				-d '{"data":{"client-secret":"dummy-oidc-client-secret-for-testing"}}' \
				%s/v1/secret/data/oidc
			
			# Enable OIDC auth backend on primary
			curl -X POST -H "X-Vault-Token: root" \
				-d '{"type":"oidc"}' \
				%s/v1/sys/auth/oidc
			
			# Configure OIDC auth backend on primary
			curl -X POST -H "X-Vault-Token: root" \
				-d '{"oidc_discovery_url":"http://localhost:8180/realms/test","oidc_client_id":"vault","oidc_client_secret":"dummy-oidc-client-secret-for-testing","default_role":"default"}' \
				%s/v1/auth/oidc/config
				
			# Configure secondary vault with required secrets and auth
			curl -X POST -H "X-Vault-Token: root" \
				-d '{"type":"kv","options":{"version":"2"}}' \
				http://localhost:8202/v1/sys/mounts/secret || true
			
			curl -X POST -H "X-Vault-Token: root" \
				-d '{"data":{"rootToken":"root"}}' \
				http://localhost:8202/v1/secret/data/master
			
			curl -X POST -H "X-Vault-Token: root" \
				-d '{"data":{"root":"root"}}' \
				http://localhost:8202/v1/secret/data/secondary
				
			curl -X POST -H "X-Vault-Token: root" \
				-d '{"data":{"cert":"-----BEGIN CERTIFICATE-----\\nMIIBkTCB+wIJANkNYzxzkdcZMA0GCSqGSIb3DQEBCwUAMBQxEjAQBgNVBAMMCWxv\\nY2FsaG9zdDAeFw0yMTAxMDEwMDAwMDBaFw0yMjAxMDEwMDAwMDBaMBQxEjAQBgNV\\nBAMMCWxvY2FsaG9zdDBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQC2Z2gw+9gRAYJ0\\nFPbL+o3Z2gOjM2V7l4S8Z2gOCp1XQJ2S7M9L5T8Z3gOQN2V7Z5S8M2gOCp1XQJ2S\\n7M9L5T8ZAgMBAAEwDQYJKoZIhvcNAQELBQADQQAjM2V7l4S8Z2gOCp1XQJ2S7M9L\\n5T8Z3gOQN2V7Z5S8M2gOCp1XQJ2S7M9L5T8Z3gOQN2V7Z5S8M2gOCp1XQJ2S\\n-----END CERTIFICATE-----"}}' \
				http://localhost:8202/v1/secret/data/kubernetes
			
			curl -X POST -H "X-Vault-Token: root" \
				-d '{"data":{"client-secret":"dummy-oidc-client-secret-for-testing"}}' \
				http://localhost:8202/v1/secret/data/oidc
			
			# Enable OIDC auth backend on secondary
			curl -X POST -H "X-Vault-Token: root" \
				-d '{"type":"oidc"}' \
				http://localhost:8202/v1/sys/auth/oidc
			
			# Configure OIDC auth backend on secondary
			curl -X POST -H "X-Vault-Token: root" \
				-d '{"oidc_discovery_url":"http://localhost:8180/realms/test","oidc_client_id":"vault","oidc_client_secret":"dummy-oidc-client-secret-for-testing","default_role":"default"}' \
				http://localhost:8202/v1/auth/oidc/config
		`, internalPrimaryVaultURL, internalPrimaryVaultURL, internalPrimaryVaultURL, internalPrimaryVaultURL, internalPrimaryVaultURL, internalPrimaryVaultURL, internalPrimaryVaultURL),
	)
	
	authOutput, err := authSetupCmd.CombinedOutput()
	t.Logf("Authentication setup output: %s", string(authOutput))
	if err != nil {
		t.Logf("⚠️ Authentication setup had issues: %v", err)
	}
	
	// Step 8: Test Error Handling - Part 1: Invalid Secondary Credentials
	t.Logf("📋 Testing error handling - Part 1: Invalid secondary credentials...")

	// Corrupt secondary vault credentials
	corruptCredsCmd := exec.Command("podman", "run", "--rm",
		"--pod", podName,
		"docker.io/curlimages/curl:latest",
		"curl", "-s", "-X", "POST", "-H", "X-Vault-Token: root",
		"-d", `{"data":{"root":"badroot"}}`,
		"http://localhost:8200/v1/secret/data/secondary",
	)
	corruptCredsOutput, err := corruptCredsCmd.CombinedOutput()
	t.Logf("Corrupt credentials output: %s", string(corruptCredsOutput))
	if err != nil {
		t.Logf("⚠️ Failed to corrupt secondary credentials: %v", err)
	}

	// Disable OIDC to trigger reconciliation on primary
	disableOIDCCmd := exec.Command("podman", "run", "--rm",
		"--pod", podName,
		"docker.io/curlimages/curl:latest",
		"curl", "-s", "-X", "DELETE", "-H", "X-Vault-Token: root",
		"http://localhost:8200/v1/sys/auth/oidc",
	)
	disableOIDCOutput, err := disableOIDCCmd.CombinedOutput()
	t.Logf("Disable OIDC output: %s", string(disableOIDCOutput))
	if err != nil {
		t.Logf("⚠️ Failed to disable OIDC: %v", err)
	}

	// Delete entities to trigger entity recreation
	deleteEntitiesCmd := exec.Command("podman", "run", "--rm",
		"--pod", podName,
		"docker.io/curlimages/curl:latest",
		"sh", "-c", `
			curl -s -X DELETE -H "X-Vault-Token: root" http://localhost:8200/v1/identity/entity/name/tester
			curl -s -X DELETE -H "X-Vault-Token: root" http://localhost:8200/v1/identity/entity/name/tester2
		`,
	)
	deleteEntitiesOutput, err := deleteEntitiesCmd.CombinedOutput()
	t.Logf("Delete entities output: %s", string(deleteEntitiesOutput))
	if err != nil {
		t.Logf("⚠️ Failed to delete entities: %v", err)
	}

	internalQontractEndpoint := "http://localhost:4000"
	
	// Run vault-manager with missing OIDC secret configuration (should cause error on secondary)
	vaultManagerErrorCmd := exec.Command("podman", "run", "--rm",
		"--pod", podName,
		"--name", "vault-manager-errors-runner-1",
		"-v", "/home/jmosco/dev/work/oss/vault-manager:/src:Z",
		"-w", "/src",
		"-e", fmt.Sprintf("GRAPHQL_SERVER=%s/graphql", internalQontractEndpoint),
		"-e", fmt.Sprintf("GRAPHQL_QUERY_FILE=%s", "/src/tests/fixtures/errors/missing_oidc_secret.graphql"),
		"-e", fmt.Sprintf("VAULT_ADDR=%s", internalPrimaryVaultURL),
		"-e", "VAULT_TOKEN=root",
		"-e", "VAULT_AUTHTYPE=token",
		"registry.access.redhat.com/ubi9/go-toolset:1.23",
		"sh", "-c", `
			go build -o /tmp/vault-manager ./cmd/vault-manager &&
			/tmp/vault-manager
		`,
	)
	
	vaultManagerErrorOutput, err := vaultManagerErrorCmd.CombinedOutput()
	t.Logf("vault-manager error test output:\n%s", string(vaultManagerErrorOutput))
	// Note: We expect this to succeed but with error messages about secondary
	require.NoError(t, err, "vault-manager should handle errors gracefully")

	// Step 9: Verify error handling behavior
	t.Logf("🔍 Verifying error handling behavior...")
	
	// Check that secondary instance is skipped due to bad credentials
	require.Contains(t, string(vaultManagerErrorOutput), "SKIPPING ALL RECONCILIATION FOR: http://localhost:8202", "Should skip secondary due to bad credentials")
	
	// Check that primary instance continues to work
	require.Contains(t, string(vaultManagerErrorOutput), "[Vault Auth] successfully enabled auth backend", "Should enable auth backend on primary")
	require.Contains(t, string(vaultManagerErrorOutput), "path=oidc/", "Should enable OIDC on primary")
	require.Contains(t, string(vaultManagerErrorOutput), "[Vault Identity] entity successfully written", "Should create entities on primary")
	require.Contains(t, string(vaultManagerErrorOutput), "instance=\"http://localhost:8200\"", "Should work on primary instance")

	// Step 10: Test Error Handling - Part 2: Missing OIDC Secret
	t.Logf("📋 Testing error handling - Part 2: Missing OIDC secret dependency...")

	// Fix secondary credentials
	fixCredsCmd := exec.Command("podman", "run", "--rm",
		"--pod", podName,
		"docker.io/curlimages/curl:latest",
		"curl", "-s", "-X", "POST", "-H", "X-Vault-Token: root",
		"-d", `{"data":{"root":"root"}}`,
		"http://localhost:8200/v1/secret/data/secondary",
	)
	fixCredsOutput, err := fixCredsCmd.CombinedOutput()
	t.Logf("Fix credentials output: %s", string(fixCredsOutput))
	if err != nil {
		t.Logf("⚠️ Failed to fix secondary credentials: %v", err)
	}

	// Remove OIDC secret dependency from secondary
	removeOIDCSecretCmd := exec.Command("podman", "run", "--rm",
		"--pod", podName,
		"docker.io/curlimages/curl:latest",
		"curl", "-s", "-X", "DELETE", "-H", "X-Vault-Token: root",
		"http://localhost:8202/v1/secret/metadata/oidc",
	)
	removeOIDCSecretOutput, err := removeOIDCSecretCmd.CombinedOutput()
	t.Logf("Remove OIDC secret output: %s", string(removeOIDCSecretOutput))
	if err != nil {
		t.Logf("⚠️ Failed to remove OIDC secret: %v", err)
	}

	// Delete entities again to trigger reconcile
	deleteEntitiesCmd2 := exec.Command("podman", "run", "--rm",
		"--pod", podName,
		"docker.io/curlimages/curl:latest",
		"sh", "-c", `
			curl -s -X DELETE -H "X-Vault-Token: root" http://localhost:8200/v1/identity/entity/name/tester
			curl -s -X DELETE -H "X-Vault-Token: root" http://localhost:8200/v1/identity/entity/name/tester2
		`,
	)
	deleteEntitiesOutput2, err := deleteEntitiesCmd2.CombinedOutput()
	t.Logf("Delete entities output 2: %s", string(deleteEntitiesOutput2))
	if err != nil {
		t.Logf("⚠️ Failed to delete entities: %v", err)
	}

	// Run vault-manager again
	vaultManagerError2Cmd := exec.Command("podman", "run", "--rm",
		"--pod", podName,
		"--name", "vault-manager-errors-runner-2",
		"-v", "/home/jmosco/dev/work/oss/vault-manager:/src:Z",
		"-w", "/src",
		"-e", fmt.Sprintf("GRAPHQL_SERVER=%s/graphql", internalQontractEndpoint),
		"-e", fmt.Sprintf("GRAPHQL_QUERY_FILE=%s", "/src/tests/fixtures/errors/missing_oidc_secret.graphql"),
		"-e", fmt.Sprintf("VAULT_ADDR=%s", internalPrimaryVaultURL),
		"-e", "VAULT_TOKEN=root",
		"-e", "VAULT_AUTHTYPE=token",
		"registry.access.redhat.com/ubi9/go-toolset:1.23",
		"sh", "-c", `
			go build -o /tmp/vault-manager ./cmd/vault-manager &&
			/tmp/vault-manager
		`,
	)
	
	vaultManagerError2Output, err := vaultManagerError2Cmd.CombinedOutput()
	t.Logf("vault-manager error test 2 output:\n%s", string(vaultManagerError2Output))
	require.NoError(t, err, "vault-manager should handle errors gracefully")

	// Step 11: Verify second error scenario
	t.Logf("🔍 Verifying second error handling behavior...")
	
	// Check that secondary instance reconciliation is partially skipped due to missing OIDC secret
	require.Contains(t, string(vaultManagerError2Output), "SKIPPING REMAINING RECONCILIATION FOR http://localhost:8202", "Should skip remaining reconciliation on secondary due to missing OIDC secret")
	
	// Check that primary instance continues to work
	require.Contains(t, string(vaultManagerError2Output), "[Vault Identity] entity successfully written", "Should create entities on primary")
	require.Contains(t, string(vaultManagerError2Output), "instance=\"http://localhost:8200\"", "Should work on primary instance")

	t.Logf("✅ vault-manager errors test completed successfully!")
	t.Logf("🎯 Error handling verified on both primary and secondary instances")
	t.Logf("📋 Verified: Graceful failure with instance isolation")
	t.Logf("🔧 Verified: Primary continues working when secondary fails")
	t.Logf("🚀 Pod-based networking approach successful!")
}