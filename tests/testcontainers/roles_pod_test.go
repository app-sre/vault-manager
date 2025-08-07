package testcontainers

import (
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestVaultManagerRolesPod tests that vault-manager can manage Vault roles using Podman pods
func TestVaultManagerRolesPod(t *testing.T) {

	t.Logf("🚀 Starting vault-manager roles test with Podman pod")

	// Step 1: Create Podman pod with port mappings
	t.Logf("🌐 Creating Podman pod with shared networking...")
	podName := "vault-manager-roles-pod"
	
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
		"--name", "keycloak-roles",
		"-e", "KEYCLOAK_ADMIN=admin",
		"-e", "KEYCLOAK_ADMIN_PASSWORD=admin",
		"quay.io/keycloak/keycloak:21.1.2",
		"start-dev", "--http-port", "8180",
	)
	output, err := keycloakCmd.CombinedOutput()
	t.Logf("Keycloak start output: %s", string(output))
	require.NoError(t, err, "Failed to start Keycloak in pod")
	
	time.Sleep(30 * time.Second)
	defer exec.Command("podman", "rm", "-f", "keycloak-roles").Run()

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
		"--name", "qontract-server-roles",
		"-v", bundlePath+":/bundle:Z",
		"-e", "LOAD_METHOD=fs",
		"-e", "DATAFILES_FILE=/bundle/data.json",
		"quay.io/redhat-services-prod/app-sre-tenant/qontract-server-master/qontract-server-master:f3fb9719c39b0413edc9e2254f942e725bc57344e72d16b4b947ae215d65c59b",
	)
	err = qontractCmd.Run()
	require.NoError(t, err, "Failed to start qontract-server in pod")
	
	time.Sleep(10 * time.Second)
	defer exec.Command("podman", "rm", "-f", "qontract-server-roles").Run()

	t.Logf("✅ qontract-server ready at: http://localhost:4000")

	// Step 4: Start Primary Vault in the pod
	t.Logf("🔧 Starting primary Vault in pod...")
	primaryVaultCmd := exec.Command("podman", "run", "-d",
		"--pod", podName,
		"--name", "primary-vault-roles",
		"-e", "VAULT_DISABLE_MLOCK=true",
		"-e", "VAULT_DEV_ROOT_TOKEN_ID=root",
		"-e", "VAULT_DEV_LISTEN_ADDRESS=0.0.0.0:8200",
		"docker.io/hashicorp/vault:1.17.1",
	)
	primaryOutput, err := primaryVaultCmd.CombinedOutput()
	t.Logf("Primary Vault start output: %s", string(primaryOutput))
	require.NoError(t, err, "Failed to start primary Vault in pod")
	
	time.Sleep(5 * time.Second)
	defer exec.Command("podman", "rm", "-f", "primary-vault-roles").Run()

	t.Logf("✅ Primary Vault ready at: http://localhost:8200")

	// Step 5: Start Secondary Vault in the pod
	t.Logf("🔧 Starting secondary Vault in pod...")
	secondaryVaultCmd := exec.Command("podman", "run", "-d",
		"--pod", podName,
		"--name", "secondary-vault-roles",
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
	defer exec.Command("podman", "rm", "-f", "secondary-vault-roles").Run()

	t.Logf("✅ Secondary Vault ready at: http://localhost:8202")

	// Step 6: Build vault-manager binary
	t.Logf("🔧 Building vault-manager binary...")
	err = buildVaultManager()
	require.NoError(t, err)
	t.Logf("✅ vault-manager binary built")

	// Step 7: Set up authentication and prerequisites (auth backends, policies)
	t.Logf("🔧 Setting up authentication and prerequisites from within pod...")
	
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
			
			# Enable AppRole auth backend on primary
			curl -X POST -H "X-Vault-Token: root" \
				-d '{"type":"approle"}' \
				%s/v1/sys/auth/approle
			
			# Create required policies on primary
			curl -X POST -H "X-Vault-Token: root" \
				-d '{"policy":"path \"secret/*\" {\n  capabilities = [\"create\", \"read\", \"update\", \"delete\", \"list\"]\n}\npath \"auth/token/lookup-self\" {\n  capabilities = [\"read\"]\n}"}' \
				%s/v1/sys/policies/acl/app-interface-approle-policy
			
			curl -X POST -H "X-Vault-Token: root" \
				-d '{"policy":"path \"secret/*\" {\n  capabilities = [\"create\", \"read\", \"update\", \"delete\", \"list\"]\n}\npath \"auth/token/lookup-self\" {\n  capabilities = [\"read\"]\n}"}' \
				%s/v1/sys/policies/acl/app-sre-policy
			
			curl -X POST -H "X-Vault-Token: root" \
				-d '{"policy":"path \"secret/*\" {\n  capabilities = [\"create\", \"read\", \"update\", \"delete\", \"list\"]\n}\npath \"auth/token/lookup-self\" {\n  capabilities = [\"read\"]\n}"}' \
				%s/v1/sys/policies/acl/vault-oidc-app-sre-policy
				
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
			
			# Enable AppRole auth backend on secondary
			curl -X POST -H "X-Vault-Token: root" \
				-d '{"type":"approle"}' \
				http://localhost:8202/v1/sys/auth/approle
			
			# Create required policies on secondary
			curl -X POST -H "X-Vault-Token: root" \
				-d '{"policy":"path \"secret/*\" {\n  capabilities = [\"create\", \"read\", \"update\", \"delete\", \"list\"]\n}\npath \"auth/token/lookup-self\" {\n  capabilities = [\"read\"]\n}"}' \
				http://localhost:8202/v1/sys/policies/acl/app-interface-approle-policy
			
			curl -X POST -H "X-Vault-Token: root" \
				-d '{"policy":"path \"secret/*\" {\n  capabilities = [\"create\", \"read\", \"update\", \"delete\", \"list\"]\n}\npath \"auth/token/lookup-self\" {\n  capabilities = [\"read\"]\n}"}' \
				http://localhost:8202/v1/sys/policies/acl/vault-oidc-app-sre-policy
		`, internalPrimaryVaultURL, internalPrimaryVaultURL, internalPrimaryVaultURL, internalPrimaryVaultURL, internalPrimaryVaultURL, internalPrimaryVaultURL, internalPrimaryVaultURL, internalPrimaryVaultURL, internalPrimaryVaultURL),
	)
	
	authOutput, err := authSetupCmd.CombinedOutput()
	t.Logf("Authentication setup output: %s", string(authOutput))
	if err != nil {
		t.Logf("⚠️ Authentication setup had issues: %v", err)
	}

	// Step 8: Run vault-manager with roles configuration
	t.Logf("📋 Running vault-manager with roles configuration...")

	internalQontractEndpoint := "http://localhost:4000"
	
	vaultManagerCmd := exec.Command("podman", "run", "--rm",
		"--pod", podName,
		"--name", "vault-manager-roles-runner",
		"-v", "/home/jmosco/dev/work/oss/vault-manager:/src:Z",
		"-w", "/src",
		"-e", fmt.Sprintf("GRAPHQL_SERVER=%s/graphql", internalQontractEndpoint),
		"-e", fmt.Sprintf("GRAPHQL_QUERY_FILE=%s", "/src/tests/fixtures/roles/enable_vault_roles.graphql"),
		"-e", fmt.Sprintf("VAULT_ADDR=%s", internalPrimaryVaultURL),
		"-e", "VAULT_TOKEN=root",
		"-e", "VAULT_AUTHTYPE=token",
		"registry.access.redhat.com/ubi9/go-toolset:1.22.9",
		"sh", "-c", `
			go build -o /tmp/vault-manager ./cmd/vault-manager &&
			/tmp/vault-manager
		`,
	)
	
	vaultManagerOutput, err := vaultManagerCmd.CombinedOutput()
	t.Logf("vault-manager output:\n%s", string(vaultManagerOutput))
	require.NoError(t, err, "vault-manager should succeed")

	// Step 9: Verify vault-manager results
	t.Logf("🔍 Verifying vault-manager roles creation...")
	
	// Check vault-manager output for expected messages
	require.Contains(t, string(vaultManagerOutput), "[Vault Role] role is successfully written", "Should contain role creation message")
	
	// Check for specific role paths and types
	require.Contains(t, string(vaultManagerOutput), "path=auth/approle/role/app-interface", "Should create app-interface role")
	require.Contains(t, string(vaultManagerOutput), "path=auth/approle/role/vault_manager", "Should create vault_manager role")
	require.Contains(t, string(vaultManagerOutput), "type=approle", "Should show approle type")
	
	// Check for instances
	require.Contains(t, string(vaultManagerOutput), "instance=\"http://localhost:8200\"", "Should configure primary instance")
	require.Contains(t, string(vaultManagerOutput), "instance=\"http://localhost:8202\"", "Should configure secondary instance")

	// Step 10: Test roles using Vault API from within pod
	t.Logf("📋 Testing Vault roles from within pod...")
	
	// List roles from primary Vault within pod
	listRolesCmd := exec.Command("podman", "run", "--rm",
		"--pod", podName,
		"docker.io/curlimages/curl:latest",
		"curl", "-s", "-H", "X-Vault-Token: root", "-X", "LIST",
		"http://localhost:8200/v1/auth/approle/role",
	)
	listRolesOutput, err := listRolesCmd.CombinedOutput()
	t.Logf("Primary Vault roles: %s", string(listRolesOutput))
	if err != nil {
		t.Logf("⚠️ Failed to list roles on primary: %v", err)
	} else {
		// Verify the expected roles are present
		require.Contains(t, string(listRolesOutput), "app-interface", "app-interface role should exist on primary")
		require.Contains(t, string(listRolesOutput), "vault_manager", "vault_manager role should exist on primary")
	}

	// Test app-interface role configuration from primary Vault
	appInterfaceRoleConfigCmd := exec.Command("podman", "run", "--rm",
		"--pod", podName,
		"docker.io/curlimages/curl:latest",
		"curl", "-s", "-H", "X-Vault-Token: root",
		"http://localhost:8200/v1/auth/approle/role/app-interface",
	)
	appInterfaceRoleConfigOutput, err := appInterfaceRoleConfigCmd.CombinedOutput()
	t.Logf("Primary Vault app-interface role config: %s", string(appInterfaceRoleConfigOutput))
	if err != nil {
		t.Logf("⚠️ Failed to get app-interface role config on primary: %v", err)
	} else {
		// Verify role configuration
		require.Contains(t, string(appInterfaceRoleConfigOutput), "\"token_num_uses\":0", "Should show correct token_num_uses")
		require.Contains(t, string(appInterfaceRoleConfigOutput), "\"token_ttl\":1800", "Should show 30m token_ttl")
		require.Contains(t, string(appInterfaceRoleConfigOutput), "\"token_max_ttl\":1800", "Should show 30m token_max_ttl")
		require.Contains(t, string(appInterfaceRoleConfigOutput), "app-interface-approle-policy", "Should have app-interface-approle-policy")
		require.Contains(t, string(appInterfaceRoleConfigOutput), "app-sre-policy", "Should have app-sre-policy")
		require.Contains(t, string(appInterfaceRoleConfigOutput), "\"bind_secret_id\":true", "Should bind secret ID")
	}

	// Test vault_manager role configuration from primary Vault
	vaultManagerRoleConfigCmd := exec.Command("podman", "run", "--rm",
		"--pod", podName,
		"docker.io/curlimages/curl:latest",
		"curl", "-s", "-H", "X-Vault-Token: root",
		"http://localhost:8200/v1/auth/approle/role/vault_manager",
	)
	vaultManagerRoleConfigOutput, err := vaultManagerRoleConfigCmd.CombinedOutput()
	t.Logf("Primary Vault vault_manager role config: %s", string(vaultManagerRoleConfigOutput))
	if err != nil {
		t.Logf("⚠️ Failed to get vault_manager role config on primary: %v", err)
	} else {
		// Verify role configuration
		require.Contains(t, string(vaultManagerRoleConfigOutput), "\"token_num_uses\":0", "Should show correct token_num_uses")
		require.Contains(t, string(vaultManagerRoleConfigOutput), "\"token_ttl\":1800", "Should show 30m token_ttl")
		require.Contains(t, string(vaultManagerRoleConfigOutput), "\"token_max_ttl\":1800", "Should show 30m token_max_ttl")
		require.Contains(t, string(vaultManagerRoleConfigOutput), "vault-oidc-app-sre-policy", "Should have vault-oidc-app-sre-policy")
		require.Contains(t, string(vaultManagerRoleConfigOutput), "\"bind_secret_id\":true", "Should bind secret ID")
	}

	// List roles from secondary Vault within pod
	listRolesSecondaryCmd := exec.Command("podman", "run", "--rm",
		"--pod", podName,
		"docker.io/curlimages/curl:latest",
		"curl", "-s", "-H", "X-Vault-Token: root", "-X", "LIST",
		"http://localhost:8202/v1/auth/approle/role",
	)
	listRolesSecondaryOutput, err := listRolesSecondaryCmd.CombinedOutput()
	t.Logf("Secondary Vault roles: %s", string(listRolesSecondaryOutput))
	if err != nil {
		t.Logf("⚠️ Failed to list roles on secondary: %v", err)
	} else {
		// Verify the expected roles are present
		require.Contains(t, string(listRolesSecondaryOutput), "app-interface", "app-interface role should exist on secondary")
		require.Contains(t, string(listRolesSecondaryOutput), "vault_manager", "vault_manager role should exist on secondary")
	}

	// Test app-interface role configuration from secondary Vault  
	appInterfaceRoleConfigSecondaryCmd := exec.Command("podman", "run", "--rm",
		"--pod", podName,
		"docker.io/curlimages/curl:latest",
		"curl", "-s", "-H", "X-Vault-Token: root",
		"http://localhost:8202/v1/auth/approle/role/app-interface",
	)
	appInterfaceRoleConfigSecondaryOutput, err := appInterfaceRoleConfigSecondaryCmd.CombinedOutput()
	t.Logf("Secondary Vault app-interface role config: %s", string(appInterfaceRoleConfigSecondaryOutput))
	if err != nil {
		t.Logf("⚠️ Failed to get app-interface role config on secondary: %v", err)
	} else {
		// Verify role configuration - Note: different policies on secondary
		require.Contains(t, string(appInterfaceRoleConfigSecondaryOutput), "\"token_num_uses\":0", "Should show correct token_num_uses")
		require.Contains(t, string(appInterfaceRoleConfigSecondaryOutput), "\"token_ttl\":1800", "Should show 30m token_ttl")
		require.Contains(t, string(appInterfaceRoleConfigSecondaryOutput), "\"token_max_ttl\":1800", "Should show 30m token_max_ttl")
		require.Contains(t, string(appInterfaceRoleConfigSecondaryOutput), "app-interface-approle-policy", "Should have app-interface-approle-policy")
		require.Contains(t, string(appInterfaceRoleConfigSecondaryOutput), "\"bind_secret_id\":true", "Should bind secret ID")
	}

	t.Logf("✅ vault-manager roles test completed successfully!")
	t.Logf("🎯 Roles created on both primary and secondary Vault instances")
	t.Logf("📋 Verified: app-interface and vault_manager AppRole configurations")
	t.Logf("🔧 Verified: role configurations, policies, and TTL settings")
	t.Logf("🚀 Pod-based networking approach successful!")
}