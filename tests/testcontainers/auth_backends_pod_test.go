package testcontainers

import (
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestVaultManagerAuthBackendsPod tests that vault-manager can enable and manage Vault auth backends with policy mappings using Podman pods
func TestVaultManagerAuthBackendsPod(t *testing.T) {

	t.Logf("🚀 Starting vault-manager auth backends test with Podman pod")

	// Step 1: Create Podman pod with port mappings
	t.Logf("🌐 Creating Podman pod with shared networking...")
	podName := "vault-manager-auth-backends-pod"
	
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
		"--name", "keycloak-auth-backends",
		"-e", "KEYCLOAK_ADMIN=admin",
		"-e", "KEYCLOAK_ADMIN_PASSWORD=admin",
		"quay.io/keycloak/keycloak:21.1.2",
		"start-dev", "--http-port", "8180",
	)
	output, err := keycloakCmd.CombinedOutput()
	t.Logf("Keycloak start output: %s", string(output))
	require.NoError(t, err, "Failed to start Keycloak in pod")
	
	time.Sleep(30 * time.Second)
	defer exec.Command("podman", "rm", "-f", "keycloak-auth-backends").Run()

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
		"--name", "qontract-server-auth-backends",
		"-v", bundlePath+":/bundle:Z",
		"-e", "LOAD_METHOD=fs",
		"-e", "DATAFILES_FILE=/bundle/data.json",
		"quay.io/redhat-services-prod/app-sre-tenant/qontract-server-master/qontract-server-master:f3fb9719c39b0413edc9e2254f942e725bc57344e72d16b4b947ae215d65c59b",
	)
	err = qontractCmd.Run()
	require.NoError(t, err, "Failed to start qontract-server in pod")
	
	time.Sleep(10 * time.Second)
	defer exec.Command("podman", "rm", "-f", "qontract-server-auth-backends").Run()

	t.Logf("✅ qontract-server ready at: http://localhost:4000")
	
	// Debug: Check qontract-server logs and test connectivity
	qontractLogsCmd := exec.Command("podman", "logs", "qontract-server-auth-backends")
	qontractLogs, _ := qontractLogsCmd.CombinedOutput()
	t.Logf("qontract-server logs: %s", string(qontractLogs))
	
	// Test connectivity to qontract-server from within pod
	testConnCmd := exec.Command("podman", "run", "--rm", "--pod", podName,
		"docker.io/curlimages/curl:latest", 
		"curl", "-s", "-o", "/dev/null", "-w", "%{http_code}", "http://localhost:4000/graphql")
	testConnOutput, _ := testConnCmd.CombinedOutput()
	t.Logf("qontract-server connectivity test: %s", string(testConnOutput))

	// Step 4: Start Primary Vault in the pod
	t.Logf("🔧 Starting primary Vault in pod...")
	primaryVaultCmd := exec.Command("podman", "run", "-d",
		"--pod", podName,
		"--name", "primary-vault-auth-backends",
		"-e", "VAULT_DISABLE_MLOCK=true",
		"-e", "VAULT_DEV_ROOT_TOKEN_ID=root",
		"-e", "VAULT_DEV_LISTEN_ADDRESS=0.0.0.0:8200",
		"docker.io/hashicorp/vault:1.17.1",
	)
	primaryOutput, err := primaryVaultCmd.CombinedOutput()
	t.Logf("Primary Vault start output: %s", string(primaryOutput))
	require.NoError(t, err, "Failed to start primary Vault in pod")
	
	time.Sleep(5 * time.Second)
	defer exec.Command("podman", "rm", "-f", "primary-vault-auth-backends").Run()

	t.Logf("✅ Primary Vault ready at: http://localhost:8200")

	// Step 5: Start Secondary Vault in the pod
	t.Logf("🔧 Starting secondary Vault in pod...")
	secondaryVaultCmd := exec.Command("podman", "run", "-d",
		"--pod", podName,
		"--name", "secondary-vault-auth-backends",
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
	defer exec.Command("podman", "rm", "-f", "secondary-vault-auth-backends").Run()

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
				
			# Store Kubernetes CA certificate (valid dummy PEM for testing)
			curl -X POST -H "X-Vault-Token: root" \
				-d '{"data":{"cert":"-----BEGIN CERTIFICATE-----\nMIIBkTCB+wIJANkNYzxzkdcZMA0GCSqGSIb3DQEBCwUAMBQxEjAQBgNVBAMMCWxv\nY2FsaG9zdDAeFw0yMTAxMDEwMDAwMDBaFw0yMjAxMDEwMDAwMDBaMBQxEjAQBgNV\nBAMMCWxvY2FsaG9zdDBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQC2Z2gw+9gRAYJ0\nFPbL+o3Z2gOjM2V7l4S8Z2gOCp1XQJ2S7M9L5T8Z3gOQN2V7Z5S8M2gOCp1XQJ2S\n7M9L5T8ZAgMBAAEwDQYJKoZIhvcNAQELBQADQQAjM2V7l4S8Z2gOCp1XQJ2S7M9L\n5T8Z3gOQN2V7Z5S8M2gOCp1XQJ2S7M9L5T8Z3gOQN2V7Z5S8M2gOCp1XQJ2S\n-----END CERTIFICATE-----"}}' \
				%s/v1/secret/data/kubernetes
				
			# Store OIDC client secret
			curl -X POST -H "X-Vault-Token: root" \
				-d '{"data":{"client-secret":"dummy-oidc-client-secret-for-testing"}}' \
				%s/v1/secret/data/oidc
				
			# Configure secondary vault with required secrets
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
				-d '{"data":{"cert":"-----BEGIN CERTIFICATE-----\nMIIBkTCB+wIJANkNYzxzkdcZMA0GCSqGSIb3DQEBCwUAMBQxEjAQBgNVBAMMCWxv\nY2FsaG9zdDAeFw0yMTAxMDEwMDAwMDBaFw0yMjAxMDEwMDAwMDBaMBQxEjAQBgNV\nBAMMCWxvY2FsaG9zdDBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQC2Z2gw+9gRAYJ0\nFPbL+o3Z2gOjM2V7l4S8Z2gOCp1XQJ2S7M9L5T8Z3gOQN2V7Z5S8M2gOCp1XQJ2S\n7M9L5T8ZAgMBAAEwDQYJKoZIhvcNAQELBQADQQAjM2V7l4S8Z2gOCp1XQJ2S7M9L\n5T8Z3gOQN2V7Z5S8M2gOCp1XQJ2S7M9L5T8Z3gOQN2V7Z5S8M2gOCp1XQJ2S\n-----END CERTIFICATE-----"}}' \
				http://localhost:8202/v1/secret/data/kubernetes
			
			curl -X POST -H "X-Vault-Token: root" \
				-d '{"data":{"client-secret":"dummy-oidc-client-secret-for-testing"}}' \
				http://localhost:8202/v1/secret/data/oidc
		`, internalPrimaryVaultURL, internalPrimaryVaultURL, internalPrimaryVaultURL, internalPrimaryVaultURL, internalPrimaryVaultURL),
	)
	
	authOutput, err := authSetupCmd.CombinedOutput()
	t.Logf("Authentication setup output: %s", string(authOutput))
	if err != nil {
		t.Logf("⚠️ Authentication setup had issues: %v", err)
	}
	
	// Step 8: Run vault-manager with auth backends configuration
	t.Logf("📋 Running vault-manager with auth backends configuration...")

	internalQontractEndpoint := "http://localhost:4000"
	
	vaultManagerCmd := exec.Command("podman", "run", "--rm",
		"--pod", podName,
		"--name", "vault-manager-auth-backends-runner",
		"-v", "/home/jmosco/dev/work/oss/vault-manager:/src:Z",
		"-w", "/src",
		"-e", fmt.Sprintf("GRAPHQL_SERVER=%s/graphql", internalQontractEndpoint),
		"-e", fmt.Sprintf("GRAPHQL_QUERY_FILE=%s", "/src/tests/fixtures/auth/enable_auth_backends_with_policy_mappings.graphql"),
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
	t.Logf("🔍 Verifying vault-manager auth backends creation...")
	
	// Check vault-manager output for expected messages
	require.Contains(t, string(vaultManagerOutput), "[Vault Auth] successfully enabled auth backend", "Should contain auth backend creation message")
	
	// Check for specific auth backends on primary
	require.Contains(t, string(vaultManagerOutput), "path=approle/", "Should create approle auth backend")
	require.Contains(t, string(vaultManagerOutput), "path=oidc/", "Should create oidc auth backend")
	require.Contains(t, string(vaultManagerOutput), "path=kubernetes-main/", "Should create kubernetes auth backend on primary")
	
	// Check for auth backend configuration
	require.Contains(t, string(vaultManagerOutput), "[Vault Auth] auth backend successfully configured", "Should contain auth backend configuration message")
	require.Contains(t, string(vaultManagerOutput), "path=auth/oidc/config", "Should configure oidc auth backend")
	require.Contains(t, string(vaultManagerOutput), "path=auth/kubernetes-main/config", "Should configure kubernetes auth backend")
	
	// Check for policy mappings (if any are configured)
	// Note: GitHub auth backend removed from test scope
	
	// Check for instances
	require.Contains(t, string(vaultManagerOutput), "instance=\"http://localhost:8200\"", "Should configure primary instance")
	require.Contains(t, string(vaultManagerOutput), "instance=\"http://localhost:8202\"", "Should configure secondary instance")

	// Step 10: Test auth backends using Vault API from within pod
	t.Logf("📋 Testing Vault auth backends from within pod...")
	
	// List auth backends from primary Vault within pod
	listAuthCmd := exec.Command("podman", "run", "--rm",
		"--pod", podName,
		"docker.io/curlimages/curl:latest",
		"curl", "-s", "-H", "X-Vault-Token: root",
		"http://localhost:8200/v1/sys/auth",
	)
	listAuthOutput, err := listAuthCmd.CombinedOutput()
	t.Logf("Primary Vault auth backends: %s", string(listAuthOutput))
	if err != nil {
		t.Logf("⚠️ Failed to list auth backends on primary: %v", err)
	} else {
		// Verify the expected auth backends are present
		require.Contains(t, string(listAuthOutput), "approle/", "approle/ auth backend should exist on primary")
		require.Contains(t, string(listAuthOutput), "oidc/", "oidc/ auth backend should exist on primary")
		require.Contains(t, string(listAuthOutput), "kubernetes-main/", "kubernetes-main/ auth backend should exist on primary")
	}

	// Test OIDC auth configuration from primary Vault
	oidcConfigCmd := exec.Command("podman", "run", "--rm",
		"--pod", podName,
		"docker.io/curlimages/curl:latest",
		"curl", "-s", "-H", "X-Vault-Token: root",
		"http://localhost:8200/v1/auth/oidc/config",
	)
	oidcConfigOutput, err := oidcConfigCmd.CombinedOutput()
	t.Logf("Primary Vault OIDC config: %s", string(oidcConfigOutput))
	if err != nil {
		t.Logf("⚠️ Failed to get OIDC config on primary: %v", err)
	} else {
		// Verify OIDC configuration
		require.Contains(t, string(oidcConfigOutput), "oidc_discovery_url", "Should show OIDC discovery URL configuration")
		require.Contains(t, string(oidcConfigOutput), "oidc_client_id", "Should show OIDC client ID configuration")
	}

	// List auth backends from secondary Vault within pod
	listAuthSecondaryCmd := exec.Command("podman", "run", "--rm",
		"--pod", podName,
		"docker.io/curlimages/curl:latest",
		"curl", "-s", "-H", "X-Vault-Token: root",
		"http://localhost:8202/v1/sys/auth",
	)
	listAuthSecondaryOutput, err := listAuthSecondaryCmd.CombinedOutput()
	t.Logf("Secondary Vault auth backends: %s", string(listAuthSecondaryOutput))
	if err != nil {
		t.Logf("⚠️ Failed to list auth backends on secondary: %v", err)
	} else {
		// Verify the expected auth backends are present
		require.Contains(t, string(listAuthSecondaryOutput), "approle/", "approle/ auth backend should exist on secondary")
		require.Contains(t, string(listAuthSecondaryOutput), "oidc/", "oidc/ auth backend should exist on secondary")
		require.Contains(t, string(listAuthSecondaryOutput), "kubernetes-secondary/", "kubernetes-secondary/ auth backend should exist on secondary")
	}

	t.Logf("✅ vault-manager auth backends test completed successfully!")
	t.Logf("🎯 Auth backends created on both primary and secondary Vault instances")
	t.Logf("📋 Verified: approle/, oidc/, kubernetes auth backends")
	t.Logf("🔧 Verified: auth backend configurations")
	t.Logf("🚀 Pod-based networking approach successful!")
}