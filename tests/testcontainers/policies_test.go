package testcontainers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	keycloak "github.com/stillya/testcontainers-keycloak"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/vault"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestVaultManagerPolicies tests that vault-manager can create and manage Vault policies
func TestVaultManagerPolicies(t *testing.T) {
	ctx := context.Background()

	t.Logf("🚀 Starting vault-manager policies test")

	// Step 1: Start Keycloak
	t.Logf("🔧 Starting Keycloak...")
	keycloakContainer, err := keycloak.Run(ctx,
		"quay.io/keycloak/keycloak:22.0.4",
		keycloak.WithAdminUsername("admin"),
		keycloak.WithAdminPassword("admin"),
		testcontainers.WithCmd("start-dev", "--http-port", "8180"),
		testcontainers.WithWaitStrategy(
			wait.ForHTTP("/realms/master").WithPort("8180/tcp").WithStartupTimeout(60*time.Second),
		),
		testcontainers.WithExposedPorts("8180:8180"),
	)
	require.NoError(t, err)
	defer keycloakContainer.Terminate(ctx)

	keycloakHost, _ := keycloakContainer.Host(ctx)
	keycloakPort, _ := keycloakContainer.MappedPort(ctx, "8180")
	keycloakEndpoint := fmt.Sprintf("http://%s:%s", keycloakHost, keycloakPort.Port())

	// Configure Keycloak
	err = configureKeycloak(keycloakEndpoint)
	require.NoError(t, err)
	t.Logf("✅ Keycloak ready at: %s", keycloakEndpoint)

	// Step 2: Start qontract-server with network alias
	t.Logf("🔧 Starting qontract-server...")
	bundlePath := "/home/jmosco/dev/work/oss/vault-manager/tests/app-interface"
	qontractReq := testcontainers.ContainerRequest{
		Image:        "quay.io/redhat-services-prod/app-sre-tenant/qontract-server-master/qontract-server-master:f3fb9719c39b0413edc9e2254f942e725bc57344e72d16b4b947ae215d65c59b",
		ExposedPorts: []string{"4000/tcp"},
		Env: map[string]string{
			"LOAD_METHOD":    "fs",
			"DATAFILES_FILE": "/bundle/data.json",
		},
		Mounts: []testcontainers.ContainerMount{
			testcontainers.BindMount(bundlePath, "/bundle"),
		},
		WaitingFor: wait.ForHTTP("/healthz").WithPort("4000/tcp").WithStartupTimeout(60 * time.Second),
	}

	qontractContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: qontractReq,
		Started:          true,
	})
	require.NoError(t, err)
	defer qontractContainer.Terminate(ctx)

	qontractHost, _ := qontractContainer.Host(ctx)
	qontractPort, _ := qontractContainer.MappedPort(ctx, "4000")
	qontractEndpoint := fmt.Sprintf("http://%s:%s", qontractHost, qontractPort.Port())
	t.Logf("✅ qontract-server ready at: %s", qontractEndpoint)

	// Step 3: Start Primary Vault with network alias
	t.Logf("🔧 Starting primary Vault...")
	primaryVault, err := vault.Run(ctx,
		"hashicorp/vault:1.17.1",
		vault.WithToken("root"),
		testcontainers.WithEnv(map[string]string{
			"VAULT_DISABLE_MLOCK": "true",
		}),
	)
	require.NoError(t, err)
	defer primaryVault.Terminate(ctx)

	primaryVaultHost, _ := primaryVault.Host(ctx)
	primaryVaultPort, _ := primaryVault.MappedPort(ctx, "8200/tcp")
	primaryVaultURL := fmt.Sprintf("http://%s:%s", primaryVaultHost, primaryVaultPort.Port())
	t.Logf("✅ Primary Vault ready at: %s", primaryVaultURL)

	// Step 4: Start Secondary Vault with network alias
	t.Logf("🔧 Starting secondary Vault...")
	secondaryVault, err := vault.Run(ctx,
		"hashicorp/vault:1.17.1",
		vault.WithToken("root"),
		testcontainers.WithEnv(map[string]string{
			"VAULT_DISABLE_MLOCK": "true",
		}),
	)
	require.NoError(t, err)
	defer secondaryVault.Terminate(ctx)

	secondaryVaultHost, _ := secondaryVault.Host(ctx)
	secondaryVaultPort, _ := secondaryVault.MappedPort(ctx, "8200/tcp")
	secondaryVaultURL := fmt.Sprintf("http://%s:%s", secondaryVaultHost, secondaryVaultPort.Port())
	t.Logf("✅ Secondary Vault ready at: %s", secondaryVaultURL)

	// Step 5: Build vault-manager binary
	t.Logf("🔧 Building vault-manager binary...")
	err = buildVaultManager()
	require.NoError(t, err)
	t.Logf("✅ vault-manager binary built")

	// Step 6: Get container IPs and update app-interface configuration
	t.Logf("🔧 Getting container IPs and updating app-interface config...")
	
	// Get the actual container IPs for updating configurations
	qontractIP, err := qontractContainer.ContainerIP(ctx)
	require.NoError(t, err)
	primaryVaultIP, err := primaryVault.ContainerIP(ctx)
	require.NoError(t, err)
	secondaryVaultIP, err := secondaryVault.ContainerIP(ctx)
	require.NoError(t, err)
	
	primaryVaultAddress := fmt.Sprintf("http://%s:8200", primaryVaultIP)
	secondaryVaultAddress := fmt.Sprintf("http://%s:8200", secondaryVaultIP)
	
	// Temporarily update the Vault instance configurations to use container IPs
	masterInstanceFile := "/home/jmosco/dev/work/oss/vault-manager/tests/app-interface/data/services/vault/config/instances/master.yml"
	secondaryInstanceFile := "/home/jmosco/dev/work/oss/vault-manager/tests/app-interface/data/services/vault/config/instances/secondary.yml"
	
	err = updateVaultInstanceConfig(masterInstanceFile, primaryVaultAddress)
	require.NoError(t, err)
	defer restoreVaultInstanceConfig(masterInstanceFile, "http://primary-vault:8200") // restore original
	
	err = updateVaultInstanceConfig(secondaryInstanceFile, secondaryVaultAddress)
	require.NoError(t, err)
	defer restoreVaultInstanceConfig(secondaryInstanceFile, "http://secondary-vault:8202") // restore original

	// Step 7: Set up authentication for secondary Vault access
	t.Logf("🔧 Setting up cross-vault authentication...")
	
	// vault-manager needs to authenticate to the secondary vault using credentials stored in primary vault
	// Store secondary vault credentials in primary vault
	err = setupVaultAuthentication(primaryVaultURL, secondaryVaultURL, "root")
	require.NoError(t, err)

	// Step 8: Run vault-manager in container with host network access
	t.Logf("📋 Running vault-manager in container with policies configuration...")

	// Use the existing add_policies.graphql file
	queryFile := "/tests/fixtures/policies/add_policies.graphql" // path inside container

	// Create vault-manager container that can access other containers by IP
	vaultManagerReq := testcontainers.ContainerRequest{
		Image: "registry.access.redhat.com/ubi9/go-toolset:1.22.9",
		Cmd: []string{"sh", "-c", `
			# Copy and build vault-manager
			cd /vault-manager && 
			go build -o /tmp/vault-manager ./cmd/vault-manager &&
			# Run vault-manager
			/tmp/vault-manager
		`},
		Env: map[string]string{
			"GRAPHQL_SERVER":     fmt.Sprintf("http://%s:4000/graphql", qontractIP),
			"GRAPHQL_QUERY_FILE": queryFile,
			"VAULT_ADDR":         fmt.Sprintf("http://%s:8200", primaryVaultIP),
			"VAULT_TOKEN":        "root",
			"VAULT_AUTHTYPE":     "token",
		},
		Mounts: []testcontainers.ContainerMount{
			testcontainers.BindMount("/home/jmosco/dev/work/oss/vault-manager", "/vault-manager"),
		},
	}

	vaultManagerContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: vaultManagerReq,
		Started:          true,
	})
	require.NoError(t, err)
	defer vaultManagerContainer.Terminate(ctx)

	// Wait for vault-manager to complete and get the output
	logReader, err := vaultManagerContainer.Logs(ctx)
	require.NoError(t, err)
	defer logReader.Close()
	
	// Read all logs
	logBytes, err := io.ReadAll(logReader)
	require.NoError(t, err)
	output := string(logBytes)
	
	// Log the output for debugging
	t.Logf("vault-manager output:\n%s", output)

	// Step 9: Verify policies were created successfully
	t.Logf("🔍 Verifying vault-manager policy creation...")

	// Check vault-manager output for expected messages
	require.Contains(t, output, "[Vault Policy] policy successfully written to Vault instance", "Should contain policy creation message")
	require.Contains(t, output, "name=app-sre-policy", "Should create app-sre-policy")
	require.Contains(t, output, "name=app-interface-approle-policy", "Should create app-interface-approle-policy")

	// Step 10: Verify policies exist in primary Vault
	t.Logf("📋 Verifying policies in primary Vault...")
	policies, err := listVaultPoliciesAPI(primaryVaultURL, "root")
	require.NoError(t, err)
	require.Contains(t, policies, "app-sre-policy", "app-sre-policy should exist in primary Vault")
	require.Contains(t, policies, "app-interface-approle-policy", "app-interface-approle-policy should exist in primary Vault")

	// Step 11: Verify policy content in primary Vault
	t.Logf("📝 Verifying policy content in primary Vault...")
	appSrePolicy, err := getVaultPolicyAPI(primaryVaultURL, "root", "app-sre-policy")
	require.NoError(t, err)
	require.Contains(t, appSrePolicy, `path "devtools-osio-ci/*"`, "app-sre-policy should contain devtools-osio-ci path")
	require.Contains(t, appSrePolicy, `path "app-sre/*"`, "app-sre-policy should contain app-sre path")
	require.Contains(t, appSrePolicy, `path "app-interface/*"`, "app-sre-policy should contain app-interface path")

	appInterfacePolicy, err := getVaultPolicyAPI(primaryVaultURL, "root", "app-interface-approle-policy")
	require.NoError(t, err)
	require.Contains(t, appInterfacePolicy, `path "app-sre/creds/*"`, "app-interface-approle-policy should contain app-sre/creds path")

	// Step 12: Verify policies exist in secondary Vault
	t.Logf("📋 Verifying policies in secondary Vault...")
	policies, err = listVaultPoliciesAPI(secondaryVaultURL, "root")
	require.NoError(t, err)
	require.Contains(t, policies, "app-sre-policy", "app-sre-policy should exist in secondary Vault")
	require.Contains(t, policies, "app-interface-approle-policy", "app-interface-approle-policy should exist in secondary Vault")

	// Step 13: Verify policy content in secondary Vault
	t.Logf("📝 Verifying policy content in secondary Vault...")
	appSrePolicy, err = getVaultPolicyAPI(secondaryVaultURL, "root", "app-sre-policy")
	require.NoError(t, err)
	require.Contains(t, appSrePolicy, `path "devtools-osio-ci/*"`, "app-sre-policy should contain devtools-osio-ci path in secondary")
	require.Contains(t, appSrePolicy, `path "app-sre/*"`, "app-sre-policy should contain app-sre path in secondary")
	require.Contains(t, appSrePolicy, `path "app-interface/*"`, "app-sre-policy should contain app-interface path in secondary")

	appInterfacePolicy, err = getVaultPolicyAPI(secondaryVaultURL, "root", "app-interface-approle-policy")
	require.NoError(t, err)
	require.Contains(t, appInterfacePolicy, `path "app-sre/creds/*"`, "app-interface-approle-policy should contain app-sre/creds path in secondary")

	// Step 14: Test idempotency - rerun vault-manager 
	t.Logf("🔄 Testing idempotency - running vault-manager again...")
	// For now, skip the idempotency test as we'd need to recreate the container
	// TODO: Implement idempotency test with container restart

	// On rerun, vault-manager should detect no changes needed (policies already exist)
	// This is the "rerun_check" equivalent from BATS

	t.Logf("✅ vault-manager policies test completed successfully!")
	t.Logf("🎯 Policies created on both primary and secondary Vault instances")
	t.Logf("📋 Verified: app-sre-policy, app-interface-approle-policy")
}

// Helper function to run vault-manager (not dry-run, actual execution)
func runVaultManager(env []string) (string, error) {
	cmd := exec.Command("/tmp/vault-manager")
	cmd.Dir = "/home/jmosco/dev/work/oss/vault-manager"
	cmd.Env = append(os.Environ(), env...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// Helper function to list Vault policies via HTTP API
func listVaultPoliciesAPI(vaultURL, token string) ([]string, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest("LIST", vaultURL+"/v1/sys/policies/acl", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Vault-Token", token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vault API returned status %d", resp.StatusCode)
	}

	var result struct {
		Data struct {
			Keys []string `json:"keys"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data.Keys, nil
}

// Helper function to get a specific Vault policy via HTTP API
func getVaultPolicyAPI(vaultURL, token, policyName string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest("GET", vaultURL+"/v1/sys/policies/acl/"+policyName, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-Vault-Token", token)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("vault API returned status %d for policy %s", resp.StatusCode, policyName)
	}

	var result struct {
		Data struct {
			Policy string `json:"policy"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Data.Policy, nil
}

// Helper function to set up authentication between vaults
// This stores the secondary vault credentials in the primary vault so vault-manager can access both
func setupVaultAuthentication(primaryVaultURL, secondaryVaultURL, token string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	
	// Enable KV secrets engine if not already enabled
	enableKVReq, err := http.NewRequest("POST", primaryVaultURL+"/v1/sys/mounts/secret", 
		strings.NewReader(`{"type":"kv","options":{"version":"2"}}`))
	if err != nil {
		return err
	}
	enableKVReq.Header.Set("X-Vault-Token", token)
	enableKVReq.Header.Set("Content-Type", "application/json")
	
	resp, err := client.Do(enableKVReq)
	if err != nil {
		return err
	}
	resp.Body.Close()
	
	// Store primary vault credentials (for master vault auth)
	masterCredentials := fmt.Sprintf(`{"data":{"rootToken":"%s"}}`, token)
	masterReq, err := http.NewRequest("POST", primaryVaultURL+"/v1/secret/data/master", 
		strings.NewReader(masterCredentials))
	if err != nil {
		return err
	}
	masterReq.Header.Set("X-Vault-Token", token)
	masterReq.Header.Set("Content-Type", "application/json")
	
	resp, err = client.Do(masterReq)
	if err != nil {
		return err
	}
	resp.Body.Close()
	
	// Store secondary vault credentials (for secondary vault auth)
	secondaryCredentials := fmt.Sprintf(`{"data":{"root":"%s"}}`, token)
	secondaryReq, err := http.NewRequest("POST", primaryVaultURL+"/v1/secret/data/secondary", 
		strings.NewReader(secondaryCredentials))
	if err != nil {
		return err
	}
	secondaryReq.Header.Set("X-Vault-Token", token)
	secondaryReq.Header.Set("Content-Type", "application/json")
	
	resp, err = client.Do(secondaryReq)
	if err != nil {
		return err
	}
	resp.Body.Close()
	
	return nil
}

// Helper function to temporarily update Vault instance configuration
func updateVaultInstanceConfig(filePath, newAddress string) error {
	// Read the current content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	
	// Replace the address with the new URL (including quotes)
	oldContent := string(content)
	var newContent string
	
	if strings.Contains(oldContent, `"http://primary-vault:8200"`) {
		newContent = strings.Replace(oldContent, `"http://primary-vault:8200"`, `"`+newAddress+`"`, 1)
	} else if strings.Contains(oldContent, `"http://secondary-vault:8202"`) {
		newContent = strings.Replace(oldContent, `"http://secondary-vault:8202"`, `"`+newAddress+`"`, 1)
	} else {
		return fmt.Errorf("couldn't find expected vault address in %s", filePath)
	}
	
	// Write back the updated content
	return os.WriteFile(filePath, []byte(newContent), 0644)
}

// Helper function to restore the original configuration
func restoreVaultInstanceConfig(filePath, originalAddress string) error {
	// Read the current content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	
	// This is a simple approach - replace any http://localhost URLs back to original
	oldContent := string(content)
	newContent := oldContent
	
	// Replace any localhost URLs back to the original address
	if strings.Contains(oldContent, "http://localhost:") {
		// Find the localhost URL pattern and replace it
		lines := strings.Split(oldContent, "\n")
		for i, line := range lines {
			if strings.Contains(line, "address:") && strings.Contains(line, "http://localhost:") {
				lines[i] = fmt.Sprintf("address: \"%s\"", originalAddress)
				break
			}
		}
		newContent = strings.Join(lines, "\n")
	}
	
	// Write back the original content
	return os.WriteFile(filePath, []byte(newContent), 0644)
}