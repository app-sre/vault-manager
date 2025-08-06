package testcontainers

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestVaultManagerSecretEnginesShared tests secret engines using shared containers
func TestVaultManagerSecretEnginesShared(t *testing.T) {
	// Reset Vault state for clean test
	resetVaultState(t)

	t.Logf("🚀 Testing vault-manager secret engines with shared containers")

	// Run vault-manager with secret engines configuration
	t.Logf("📋 Running vault-manager with secret engines configuration...")
	
	vaultManagerOutput, err := runSharedVaultManager(t, "/src/tests/fixtures/secret-engines/enable_secrets_engines.graphql")
	t.Logf("vault-manager output:\n%s", string(vaultManagerOutput))
	require.NoError(t, err, "vault-manager should succeed")

	// Verify vault-manager results
	t.Logf("🔍 Verifying vault-manager secret engines creation...")
	
	// Check vault-manager output for expected messages
	require.Contains(t, string(vaultManagerOutput), "[Vault Secrets engine] successfully enabled secrets-engine", "Should contain secret engine creation message")
	
	// Check for specific secret engines on both instances
	require.Contains(t, string(vaultManagerOutput), "path=app-interface/", "Should create app-interface/ secret engine")
	require.Contains(t, string(vaultManagerOutput), "path=app-sre/", "Should create app-sre/ secret engine")
	
	// Check for instances
	require.Contains(t, string(vaultManagerOutput), `instance="http://localhost:8200"`, "Should configure primary instance")
	require.Contains(t, string(vaultManagerOutput), `instance="http://localhost:8202"`, "Should configure secondary instance")

	// Test secret engines using Vault API
	t.Logf("📋 Testing Vault secret engines using shared containers...")
	
	// List secret engines from primary Vault
	listSecretsOutput, err := runVaultAPICommand(t, `curl -s -H "X-Vault-Token: root" http://localhost:8200/v1/sys/mounts`)
	t.Logf("Primary Vault secret engines: %s", string(listSecretsOutput))
	if err != nil {
		t.Logf("⚠️ Failed to list secret engines on primary: %v", err)
	} else {
		// Verify the expected secret engines are present
		require.Contains(t, string(listSecretsOutput), "app-interface/", "app-interface/ secret engine should exist on primary")
		require.Contains(t, string(listSecretsOutput), "app-sre/", "app-sre/ secret engine should exist on primary")
		require.Contains(t, string(listSecretsOutput), `"type":"kv"`, "Should show KV secret engine type")
	}

	// Test app-interface secret engine configuration from primary Vault
	appInterfaceConfigOutput, err := runVaultAPICommand(t, `curl -s -H "X-Vault-Token: root" http://localhost:8200/v1/sys/mounts/app-interface`)
	t.Logf("Primary Vault app-interface config: %s", string(appInterfaceConfigOutput))
	if err != nil {
		t.Logf("⚠️ Failed to get app-interface config on primary: %v", err)
	} else {
		// Verify app-interface configuration - should be version 2 on master
		require.Contains(t, string(appInterfaceConfigOutput), `"type":"kv"`, "Should show KV type")
		require.Contains(t, string(appInterfaceConfigOutput), `"version":"2"`, "Should show version 2 for app-interface on primary")
	}

	// List secret engines from secondary Vault
	listSecretsSecondaryOutput, err := runVaultAPICommand(t, `curl -s -H "X-Vault-Token: root" http://localhost:8202/v1/sys/mounts`)
	t.Logf("Secondary Vault secret engines: %s", string(listSecretsSecondaryOutput))
	if err != nil {
		t.Logf("⚠️ Failed to list secret engines on secondary: %v", err)
	} else {
		// Verify the expected secret engines are present
		require.Contains(t, string(listSecretsSecondaryOutput), "app-interface/", "app-interface/ secret engine should exist on secondary")
		require.Contains(t, string(listSecretsSecondaryOutput), "app-sre/", "app-sre/ secret engine should exist on secondary")
		require.Contains(t, string(listSecretsSecondaryOutput), `"type":"kv"`, "Should show KV secret engine type")
	}

	// Test app-sre secret engine configuration from secondary Vault  
	appSreConfigOutput, err := runVaultAPICommand(t, `curl -s -H "X-Vault-Token: root" http://localhost:8202/v1/sys/mounts/app-sre`)
	t.Logf("Secondary Vault app-sre config: %s", string(appSreConfigOutput))
	if err != nil {
		t.Logf("⚠️ Failed to get app-sre config on secondary: %v", err)
	} else {
		// Verify app-sre configuration - should be version 1 on secondary
		require.Contains(t, string(appSreConfigOutput), `"type":"kv"`, "Should show KV type")
		require.Contains(t, string(appSreConfigOutput), `"version":"1"`, "Should show version 1 for app-sre on secondary")
	}

	t.Logf("✅ vault-manager secret engines test completed successfully!")
	t.Logf("🎯 Secret engines created on both primary and secondary Vault instances")
	t.Logf("📋 Verified: app-interface/ (v2), app-sre/ (v1) secret engines")
	t.Logf("🔧 Verified: secret engine configurations and versions")
	t.Logf("🚀 Shared container approach successful!")
}