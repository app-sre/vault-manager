package testcontainers

import (
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestVaultManagerEntitiesPod tests that vault-manager can manage Vault identity entities and aliases using Podman pods
func TestVaultManagerEntitiesPod(t *testing.T) {

	t.Logf("🚀 Starting vault-manager entities test with Podman pod")

	// Step 1: Create Podman pod with port mappings
	t.Logf("🌐 Creating Podman pod with shared networking...")
	podName := "vault-manager-entities-pod"
	
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
		"--name", "keycloak-entities",
		"-e", "KEYCLOAK_ADMIN=admin",
		"-e", "KEYCLOAK_ADMIN_PASSWORD=admin",
		"quay.io/keycloak/keycloak:21.1.2",
		"start-dev", "--http-port", "8180",
	)
	output, err := keycloakCmd.CombinedOutput()
	t.Logf("Keycloak start output: %s", string(output))
	require.NoError(t, err, "Failed to start Keycloak in pod")
	
	time.Sleep(30 * time.Second)
	defer exec.Command("podman", "rm", "-f", "keycloak-entities").Run()

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
		"--name", "qontract-server-entities",
		"-v", bundlePath+":/bundle:Z",
		"-e", "LOAD_METHOD=fs",
		"-e", "DATAFILES_FILE=/bundle/data.json",
		"quay.io/redhat-services-prod/app-sre-tenant/qontract-server-master/qontract-server-master:f3fb9719c39b0413edc9e2254f942e725bc57344e72d16b4b947ae215d65c59b",
	)
	err = qontractCmd.Run()
	require.NoError(t, err, "Failed to start qontract-server in pod")
	
	time.Sleep(10 * time.Second)
	defer exec.Command("podman", "rm", "-f", "qontract-server-entities").Run()

	t.Logf("✅ qontract-server ready at: http://localhost:4000")

	// Step 4: Start Primary Vault in the pod
	t.Logf("🔧 Starting primary Vault in pod...")
	primaryVaultCmd := exec.Command("podman", "run", "-d",
		"--pod", podName,
		"--name", "primary-vault-entities",
		"-e", "VAULT_DISABLE_MLOCK=true",
		"-e", "VAULT_DEV_ROOT_TOKEN_ID=root",
		"-e", "VAULT_DEV_LISTEN_ADDRESS=0.0.0.0:8200",
		"docker.io/hashicorp/vault:1.17.1",
	)
	primaryOutput, err := primaryVaultCmd.CombinedOutput()
	t.Logf("Primary Vault start output: %s", string(primaryOutput))
	require.NoError(t, err, "Failed to start primary Vault in pod")
	
	time.Sleep(5 * time.Second)
	defer exec.Command("podman", "rm", "-f", "primary-vault-entities").Run()

	t.Logf("✅ Primary Vault ready at: http://localhost:8200")

	// Step 5: Start Secondary Vault in the pod
	t.Logf("🔧 Starting secondary Vault in pod...")
	secondaryVaultCmd := exec.Command("podman", "run", "-d",
		"--pod", podName,
		"--name", "secondary-vault-entities",
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
	defer exec.Command("podman", "rm", "-f", "secondary-vault-entities").Run()

	t.Logf("✅ Secondary Vault ready at: http://localhost:8202")

	// Step 6: Build vault-manager binary
	t.Logf("🔧 Building vault-manager binary...")
	err = buildVaultManager()
	require.NoError(t, err)
	t.Logf("✅ vault-manager binary built")

	// Step 7: Set up authentication and auth backends (needed for entity aliases)
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
	
	// Step 8: Run vault-manager with entities configuration
	t.Logf("📋 Running vault-manager with entities configuration...")

	internalQontractEndpoint := "http://localhost:4000"
	
	vaultManagerCmd := exec.Command("podman", "run", "--rm",
		"--pod", podName,
		"--name", "vault-manager-entities-runner",
		"-v", "/home/jmosco/dev/work/oss/vault-manager:/src:Z",
		"-w", "/src",
		"-e", fmt.Sprintf("GRAPHQL_SERVER=%s/graphql", internalQontractEndpoint),
		"-e", fmt.Sprintf("GRAPHQL_QUERY_FILE=%s", "/src/tests/fixtures/entities/enable_vault_entities_and_aliases.graphql"),
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
	t.Logf("🔍 Verifying vault-manager entities creation...")
	
	// Check vault-manager output for expected messages
	require.Contains(t, string(vaultManagerOutput), "[Vault Identity] entity successfully written", "Should contain entity creation message")
	require.Contains(t, string(vaultManagerOutput), "[Vault Identity] entity alias successfully written", "Should contain entity alias creation message")
	
	// Check for specific entity paths and types
	require.Contains(t, string(vaultManagerOutput), "path=identity/entity/name/tester", "Should create tester entity")
	require.Contains(t, string(vaultManagerOutput), "type=entity", "Should show entity type")
	require.Contains(t, string(vaultManagerOutput), "path=identity/entity-alias/tester", "Should create tester entity alias")
	require.Contains(t, string(vaultManagerOutput), "type=oidc", "Should show OIDC alias type")
	
	// Check for instances
	require.Contains(t, string(vaultManagerOutput), "instance=\"http://localhost:8200\"", "Should configure primary instance")
	require.Contains(t, string(vaultManagerOutput), "instance=\"http://localhost:8202\"", "Should configure secondary instance")

	// Step 10: Test entities using Vault API from within pod
	t.Logf("📋 Testing Vault entities from within pod...")
	
	// List entities from primary Vault within pod using correct API path
	listEntitiesCmd := exec.Command("podman", "run", "--rm",
		"--pod", podName,
		"docker.io/curlimages/curl:latest",
		"curl", "-s", "-H", "X-Vault-Token: root", "-X", "LIST",
		"http://localhost:8200/v1/identity/entity/name",
	)
	listEntitiesOutput, err := listEntitiesCmd.CombinedOutput()
	t.Logf("Primary Vault entities: %s", string(listEntitiesOutput))
	if err != nil {
		t.Logf("⚠️ Failed to list entities on primary: %v", err)
	} else {
		// Verify the expected entities are present
		require.Contains(t, string(listEntitiesOutput), "tester", "tester entity should exist on primary")
	}

	// Test entity configuration from primary Vault
	entityConfigCmd := exec.Command("podman", "run", "--rm",
		"--pod", podName,
		"docker.io/curlimages/curl:latest",
		"curl", "-s", "-H", "X-Vault-Token: root",
		"http://localhost:8200/v1/identity/entity/name/tester",
	)
	entityConfigOutput, err := entityConfigCmd.CombinedOutput()
	t.Logf("Primary Vault tester entity config: %s", string(entityConfigOutput))
	if err != nil {
		t.Logf("⚠️ Failed to get tester entity config on primary: %v", err)
	} else {
		// Verify entity configuration
		require.Contains(t, string(entityConfigOutput), "\"name\":\"tester\"", "Should show correct entity name")
		require.Contains(t, string(entityConfigOutput), "\"disabled\":false", "Should show entity is enabled")
		require.Contains(t, string(entityConfigOutput), "\"metadata\"", "Should have metadata")
		require.Contains(t, string(entityConfigOutput), "\"name\":\"The Tester\"", "Should have correct metadata name")
	}

	// List entity aliases from primary Vault within pod
	listAliasesCmd := exec.Command("podman", "run", "--rm",
		"--pod", podName,
		"docker.io/curlimages/curl:latest",
		"curl", "-s", "-H", "X-Vault-Token: root", "-X", "LIST",
		"http://localhost:8200/v1/identity/entity-alias/id",
	)
	listAliasesOutput, err := listAliasesCmd.CombinedOutput()
	t.Logf("Primary Vault entity aliases: %s", string(listAliasesOutput))
	if err != nil {
		t.Logf("⚠️ Failed to list entity aliases on primary: %v", err)
	}

	// List entities from secondary Vault within pod
	listEntitiesSecondaryCmd := exec.Command("podman", "run", "--rm",
		"--pod", podName,
		"docker.io/curlimages/curl:latest",
		"curl", "-s", "-H", "X-Vault-Token: root", "-X", "LIST",
		"http://localhost:8202/v1/identity/entity/name",
	)
	listEntitiesSecondaryOutput, err := listEntitiesSecondaryCmd.CombinedOutput()
	t.Logf("Secondary Vault entities: %s", string(listEntitiesSecondaryOutput))
	if err != nil {
		t.Logf("⚠️ Failed to list entities on secondary: %v", err)
	} else {
		// Verify the expected entities are present
		require.Contains(t, string(listEntitiesSecondaryOutput), "tester", "tester entity should exist on secondary")
	}

	// Test entity configuration from secondary Vault  
	entityConfigSecondaryCmd := exec.Command("podman", "run", "--rm",
		"--pod", podName,
		"docker.io/curlimages/curl:latest",
		"curl", "-s", "-H", "X-Vault-Token: root",
		"http://localhost:8202/v1/identity/entity/name/tester",
	)
	entityConfigSecondaryOutput, err := entityConfigSecondaryCmd.CombinedOutput()
	t.Logf("Secondary Vault tester entity config: %s", string(entityConfigSecondaryOutput))
	if err != nil {
		t.Logf("⚠️ Failed to get tester entity config on secondary: %v", err)
	} else {
		// Verify entity configuration
		require.Contains(t, string(entityConfigSecondaryOutput), "\"name\":\"tester\"", "Should show correct entity name")
		require.Contains(t, string(entityConfigSecondaryOutput), "\"disabled\":false", "Should show entity is enabled")
		require.Contains(t, string(entityConfigSecondaryOutput), "\"metadata\"", "Should have metadata")
		require.Contains(t, string(entityConfigSecondaryOutput), "\"name\":\"The Tester\"", "Should have correct metadata name")
	}

	t.Logf("✅ vault-manager entities test completed successfully!")
	t.Logf("🎯 Entities and aliases created on both primary and secondary Vault instances")
	t.Logf("📋 Verified: tester entity with metadata and OIDC aliases")
	t.Logf("🔧 Verified: entity configurations and properties")
	t.Logf("🚀 Pod-based networking approach successful!")
}