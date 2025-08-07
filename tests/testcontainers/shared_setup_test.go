package testcontainers

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"
)

// Shared test infrastructure
var (
	sharedPodName                = "vault-manager-shared-pod"
	sharedInternalPrimaryURL     = "http://localhost:8200"
	sharedInternalSecondaryURL   = "http://localhost:8202"
	sharedInternalQontractURL    = "http://localhost:4000"
	sharedInternalKeycloakURL    = "http://localhost:8180"
	sharedContainersReady        = false
)

// TestMain sets up shared containers for all tests
func TestMain(m *testing.M) {
	// Setup shared containers
	if err := setupSharedContainers(); err != nil {
		fmt.Printf("Failed to setup shared containers: %v\n", err)
		os.Exit(1)
	}

	// Run all tests
	code := m.Run()

	// Cleanup shared containers
	cleanupSharedContainers()

	os.Exit(code)
}

func setupSharedContainers() error {
	fmt.Printf("🚀 Setting up shared containers for all tests...\n")

	// Step 1: Create shared Podman pod
	fmt.Printf("🌐 Creating shared Podman pod...\n")
	createPodCmd := exec.Command("podman", "pod", "create",
		"--name", sharedPodName,
		"--publish", "8180:8180", // Keycloak
		"--publish", "4000:4000", // qontract-server
		"--publish", "8200:8200", // Primary Vault
		"--publish", "8202:8202", // Secondary Vault
	)
	if err := createPodCmd.Run(); err != nil {
		return fmt.Errorf("failed to create shared pod: %w", err)
	}

	// Step 2: Start Keycloak
	fmt.Printf("🔧 Starting shared Keycloak...\n")
	keycloakCmd := exec.Command("podman", "run", "-d",
		"--pod", sharedPodName,
		"--name", "keycloak-shared",
		"-e", "KEYCLOAK_ADMIN=admin",
		"-e", "KEYCLOAK_ADMIN_PASSWORD=admin",
		"quay.io/keycloak/keycloak:21.1.2",
		"start-dev", "--http-port", "8180",
	)
	if err := keycloakCmd.Run(); err != nil {
		return fmt.Errorf("failed to start shared Keycloak: %w", err)
	}

	time.Sleep(30 * time.Second)

	// Step 3: Configure Keycloak realm and client
	fmt.Printf("🔧 Configuring shared Keycloak realm...\n")
	time.Sleep(30 * time.Second) // Wait for Keycloak to be fully ready

	keycloakConfigCmd := exec.Command("podman", "run", "--rm",
		"--pod", sharedPodName,
		"docker.io/curlimages/curl:latest",
		"sh", "-c", `
			# Get admin token for Keycloak 21.x
			TOKEN=$(curl -s -X POST http://localhost:8180/realms/master/protocol/openid-connect/token \
				-H "Content-Type: application/x-www-form-urlencoded" \
				-d "username=admin&password=admin&grant_type=password&client_id=admin-cli" | \
				grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)
			
			echo "Admin token: $TOKEN"
			
			# Create test realm
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
	if err := keycloakConfigCmd.Run(); err != nil {
		fmt.Printf("⚠️ Keycloak configuration had issues: %v\n", err)
	}

	// Step 4: Start qontract-server
	fmt.Printf("🔧 Starting shared qontract-server...\n")
	bundlePath := "/home/jmosco/dev/work/oss/vault-manager/tests/app-interface"
	qontractCmd := exec.Command("podman", "run", "-d",
		"--pod", sharedPodName,
		"--name", "qontract-server-shared",
		"-v", bundlePath+":/bundle:Z",
		"-e", "LOAD_METHOD=fs",
		"-e", "DATAFILES_FILE=/bundle/data.json",
		"quay.io/redhat-services-prod/app-sre-tenant/qontract-server-master/qontract-server-master:f3fb9719c39b0413edc9e2254f942e725bc57344e72d16b4b947ae215d65c59b",
	)
	if err := qontractCmd.Run(); err != nil {
		return fmt.Errorf("failed to start shared qontract-server: %w", err)
	}

	time.Sleep(10 * time.Second)

	// Step 5: Start Primary Vault
	fmt.Printf("🔧 Starting shared primary Vault...\n")
	primaryVaultCmd := exec.Command("podman", "run", "-d",
		"--pod", sharedPodName,
		"--name", "primary-vault-shared",
		"-e", "VAULT_DISABLE_MLOCK=true",
		"-e", "VAULT_DEV_ROOT_TOKEN_ID=root",
		"-e", "VAULT_DEV_LISTEN_ADDRESS=0.0.0.0:8200",
		"docker.io/hashicorp/vault:1.17.1",
	)
	if err := primaryVaultCmd.Run(); err != nil {
		return fmt.Errorf("failed to start shared primary Vault: %w", err)
	}

	time.Sleep(5 * time.Second)

	// Step 6: Start Secondary Vault
	fmt.Printf("🔧 Starting shared secondary Vault...\n")
	secondaryVaultCmd := exec.Command("podman", "run", "-d",
		"--pod", sharedPodName,
		"--name", "secondary-vault-shared",
		"-e", "VAULT_DISABLE_MLOCK=true",
		"-e", "VAULT_DEV_ROOT_TOKEN_ID=root",
		"-e", "VAULT_DEV_LISTEN_ADDRESS=0.0.0.0:8202",
		"docker.io/hashicorp/vault:1.17.1",
		"vault", "server", "-dev", "-dev-listen-address=0.0.0.0:8202",
	)
	if err := secondaryVaultCmd.Run(); err != nil {
		return fmt.Errorf("failed to start shared secondary Vault: %w", err)
	}

	time.Sleep(5 * time.Second)

	// Step 7: Build vault-manager binary
	fmt.Printf("🔧 Building vault-manager binary...\n")
	if err := buildVaultManager(); err != nil {
		return fmt.Errorf("failed to build vault-manager: %w", err)
	}

	// Step 8: Set up base authentication secrets
	fmt.Printf("🔧 Setting up base authentication secrets...\n")
	authSetupCmd := exec.Command("podman", "run", "--rm",
		"--pod", sharedPodName,
		"docker.io/curlimages/curl:latest",
		"sh", "-c", fmt.Sprintf(`
			# Enable KV secrets engine on primary
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
				-d '{"data":{"cert":"-----BEGIN CERTIFICATE-----\\nMIIBkTCB+wIJANkNYzxzkdcZMA0GCSqGSIb3DQEBCwUAMBQxEjAQBgNVBAMMCWxv\\nY2FsaG9zdDAeFw0yMTAxMDEwMDAwMDBaFw0yMjAxMDEwMDAwMDBaMBQxEjAQBgNV\\nBAMMCWxvY2FsaG9zdDBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQC2Z2gw+9gRAYJ0\\nFPbL+o3Z2gOjM2V7l4S8Z2gOCp1XQJ2S7M9L5T8Z3gOQN2V7Z5S8M2gOCp1XQJ2S\\n7M9L5T8ZAgMBAAEwDQYJKoZIhvcNAQELBQADQQAjM2V7l4S8Z2gOCp1XQJ2S7M9L\\n5T8Z3gOQN2V7Z5S8M2gOCp1XQJ2S7M9L5T8Z3gOQN2V7Z5S8M2gOCp1XQJ2S\\n-----END CERTIFICATE-----"}}' \
				http://localhost:8202/v1/secret/data/kubernetes
			
			curl -X POST -H "X-Vault-Token: root" \
				-d '{"data":{"client-secret":"dummy-oidc-client-secret-for-testing"}}' \
				http://localhost:8202/v1/secret/data/oidc
		`, sharedInternalPrimaryURL, sharedInternalPrimaryURL, sharedInternalPrimaryURL, sharedInternalPrimaryURL, sharedInternalPrimaryURL),
	)

	if err := authSetupCmd.Run(); err != nil {
		fmt.Printf("⚠️ Base authentication setup had issues: %v\n", err)
	}

	sharedContainersReady = true
	fmt.Printf("✅ Shared containers are ready!\n")
	fmt.Printf("🌐 Primary Vault: %s\n", sharedInternalPrimaryURL)
	fmt.Printf("🌐 Secondary Vault: %s\n", sharedInternalSecondaryURL)
	fmt.Printf("🌐 qontract-server: %s\n", sharedInternalQontractURL)
	fmt.Printf("🌐 Keycloak: %s\n", sharedInternalKeycloakURL)

	return nil
}

func cleanupSharedContainers() {
	fmt.Printf("🧹 Cleaning up shared containers...\n")
	exec.Command("podman", "pod", "rm", "-f", sharedPodName).Run()
	fmt.Printf("✅ Shared containers cleaned up\n")
}

// resetVaultState resets Vault state between tests while keeping containers running
func resetVaultState(t *testing.T) {
	if !sharedContainersReady {
		t.Fatal("Shared containers are not ready")
	}

	t.Logf("🔄 Resetting Vault state for test: %s", t.Name())

	resetCmd := exec.Command("podman", "run", "--rm",
		"--pod", sharedPodName,
		"docker.io/curlimages/curl:latest",
		"sh", "-c", `
			# Reset primary Vault
			# Disable all auth backends except token
			for auth in $(curl -s -H "X-Vault-Token: root" http://localhost:8200/v1/sys/auth | grep -o '"[^"]*/"' | grep -v '"token/"' | tr -d '"'); do
				curl -s -X DELETE -H "X-Vault-Token: root" "http://localhost:8200/v1/sys/auth/$auth" || true
			done
			
			# Disable all audit devices
			for audit in $(curl -s -H "X-Vault-Token: root" http://localhost:8200/v1/sys/audit | grep -o '"[^"]*/"' | tr -d '"'); do
				curl -s -X DELETE -H "X-Vault-Token: root" "http://localhost:8200/v1/sys/audit/$audit" || true
			done
			
			# Delete all policies except default/root
			for policy in $(curl -s -H "X-Vault-Token: root" http://localhost:8200/v1/sys/policies/acl | grep -o '"[^"]*"' | grep -v -E '"(default|root)"' | tr -d '"'); do
				curl -s -X DELETE -H "X-Vault-Token: root" "http://localhost:8200/v1/sys/policies/acl/$policy" || true
			done
			
			# Delete all secret engines except secret/, sys/, identity/, cubbyhole/
			for mount in $(curl -s -H "X-Vault-Token: root" http://localhost:8200/v1/sys/mounts | grep -o '"[^"]*/"' | grep -v -E '"(secret/|sys/|identity/|cubbyhole/)"' | tr -d '"'); do
				curl -s -X DELETE -H "X-Vault-Token: root" "http://localhost:8200/v1/sys/mounts/$mount" || true
			done
			
			# Delete all entities
			curl -s -X LIST -H "X-Vault-Token: root" http://localhost:8200/v1/identity/entity/name | grep -o '"[^"]*"' | tr -d '"' | while read entity; do
				curl -s -X DELETE -H "X-Vault-Token: root" "http://localhost:8200/v1/identity/entity/name/$entity" || true
			done
			
			# Delete all groups
			curl -s -X LIST -H "X-Vault-Token: root" http://localhost:8200/v1/identity/group/name | grep -o '"[^"]*"' | tr -d '"' | while read group; do
				curl -s -X DELETE -H "X-Vault-Token: root" "http://localhost:8200/v1/identity/group/name/$group" || true
			done
			
			# Reset secondary Vault (same commands but port 8202)
			for auth in $(curl -s -H "X-Vault-Token: root" http://localhost:8202/v1/sys/auth | grep -o '"[^"]*/"' | grep -v '"token/"' | tr -d '"'); do
				curl -s -X DELETE -H "X-Vault-Token: root" "http://localhost:8202/v1/sys/auth/$auth" || true
			done
			
			for audit in $(curl -s -H "X-Vault-Token: root" http://localhost:8202/v1/sys/audit | grep -o '"[^"]*/"' | tr -d '"'); do
				curl -s -X DELETE -H "X-Vault-Token: root" "http://localhost:8202/v1/sys/audit/$audit" || true
			done
			
			for policy in $(curl -s -H "X-Vault-Token: root" http://localhost:8202/v1/sys/policies/acl | grep -o '"[^"]*"' | grep -v -E '"(default|root)"' | tr -d '"'); do
				curl -s -X DELETE -H "X-Vault-Token: root" "http://localhost:8202/v1/sys/policies/acl/$policy" || true
			done
			
			for mount in $(curl -s -H "X-Vault-Token: root" http://localhost:8202/v1/sys/mounts | grep -o '"[^"]*/"' | grep -v -E '"(secret/|sys/|identity/|cubbyhole/)"' | tr -d '"'); do
				curl -s -X DELETE -H "X-Vault-Token: root" "http://localhost:8202/v1/sys/mounts/$mount" || true
			done
			
			curl -s -X LIST -H "X-Vault-Token: root" http://localhost:8202/v1/identity/entity/name | grep -o '"[^"]*"' | tr -d '"' | while read entity; do
				curl -s -X DELETE -H "X-Vault-Token: root" "http://localhost:8202/v1/identity/entity/name/$entity" || true
			done
			
			curl -s -X LIST -H "X-Vault-Token: root" http://localhost:8202/v1/identity/group/name | grep -o '"[^"]*"' | tr -d '"' | while read group; do
				curl -s -X DELETE -H "X-Vault-Token: root" "http://localhost:8202/v1/identity/group/name/$group" || true
			done
		`,
	)

	if err := resetCmd.Run(); err != nil {
		t.Logf("⚠️ Vault state reset had issues: %v", err)
	}

	t.Logf("✅ Vault state reset completed for test: %s", t.Name())
}

// runSharedVaultManager runs vault-manager with the specified GraphQL query file in the shared pod
func runSharedVaultManager(t *testing.T, queryFile string, extraArgs ...string) ([]byte, error) {
	if !sharedContainersReady {
		return nil, fmt.Errorf("shared containers are not ready")
	}

	args := []string{
		"podman", "run", "--rm",
		"--pod", sharedPodName,
		"--name", fmt.Sprintf("vault-manager-%s-runner", t.Name()),
		"-v", "/home/jmosco/dev/work/oss/vault-manager:/src:Z",
		"-w", "/src",
		"-e", fmt.Sprintf("GRAPHQL_SERVER=%s/graphql", sharedInternalQontractURL),
		"-e", fmt.Sprintf("GRAPHQL_QUERY_FILE=%s", queryFile),
		"-e", fmt.Sprintf("VAULT_ADDR=%s", sharedInternalPrimaryURL),
		"-e", "VAULT_TOKEN=root",
		"-e", "VAULT_AUTHTYPE=token",
		"registry.access.redhat.com/ubi9/go-toolset:1.22.9",
		"sh", "-c",
	}

	// Build command with extra args
	buildCmd := "go build -buildvcs=false -o /tmp/vault-manager ./cmd/vault-manager && /tmp/vault-manager"
	if len(extraArgs) > 0 {
		buildCmd += " " + extraArgs[0] // Add flags like -dry-run
	}

	args = append(args, buildCmd)

	cmd := exec.Command(args[0], args[1:]...)
	return cmd.CombinedOutput()
}

// runVaultAPICommand runs a vault API command within the shared pod
func runVaultAPICommand(t *testing.T, curlArgs string) ([]byte, error) {
	if !sharedContainersReady {
		return nil, fmt.Errorf("shared containers are not ready")
	}

	cmd := exec.Command("podman", "run", "--rm",
		"--pod", sharedPodName,
		"docker.io/curlimages/curl:latest",
		"sh", "-c", curlArgs,
	)

	return cmd.CombinedOutput()
}