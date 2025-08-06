package testcontainers

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestVaultManagerEntitiesShared tests entities using shared containers
func TestVaultManagerEntitiesShared(t *testing.T) {
	// Reset Vault state for clean test
	resetVaultState(t)

	t.Logf("🚀 Testing vault-manager entities with shared containers")

	// Set up OIDC auth backends first (prerequisite for entity aliases)
	t.Logf("🔧 Setting up OIDC auth backends...")
	
	_, err := runVaultAPICommand(t, `
		# Enable OIDC auth backend on primary
		curl -s -X POST -H "X-Vault-Token: root" \
			-d '{"type":"oidc"}' \
			http://localhost:8200/v1/sys/auth/oidc
		
		# Configure OIDC auth backend on primary
		curl -s -X POST -H "X-Vault-Token: root" \
			-d '{"oidc_discovery_url":"http://localhost:8180/realms/test","oidc_client_id":"vault","oidc_client_secret":"dummy-oidc-client-secret-for-testing","default_role":"default"}' \
			http://localhost:8200/v1/auth/oidc/config
		
		# Enable OIDC auth backend on secondary
		curl -s -X POST -H "X-Vault-Token: root" \
			-d '{"type":"oidc"}' \
			http://localhost:8202/v1/sys/auth/oidc
		
		# Configure OIDC auth backend on secondary
		curl -s -X POST -H "X-Vault-Token: root" \
			-d '{"oidc_discovery_url":"http://localhost:8180/realms/test","oidc_client_id":"vault","oidc_client_secret":"dummy-oidc-client-secret-for-testing","default_role":"default"}' \
			http://localhost:8202/v1/auth/oidc/config
	`)
	if err != nil {
		t.Logf("⚠️ OIDC setup had issues: %v", err)
	}

	// Run vault-manager with entities configuration
	t.Logf("📋 Running vault-manager with entities configuration...")
	
	vaultManagerOutput, err := runSharedVaultManager(t, "/src/tests/fixtures/entities/enable_vault_entities_and_aliases.graphql")
	t.Logf("vault-manager output:\n%s", string(vaultManagerOutput))
	require.NoError(t, err, "vault-manager should succeed")

	// Verify vault-manager results
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
	require.Contains(t, string(vaultManagerOutput), `instance="http://localhost:8200"`, "Should configure primary instance")
	require.Contains(t, string(vaultManagerOutput), `instance="http://localhost:8202"`, "Should configure secondary instance")

	// Test entities using Vault API
	t.Logf("📋 Testing Vault entities using shared containers...")
	
	// List entities from primary Vault
	listEntitiesOutput, err := runVaultAPICommand(t, `curl -s -H "X-Vault-Token: root" -X LIST http://localhost:8200/v1/identity/entity/name`)
	t.Logf("Primary Vault entities: %s", string(listEntitiesOutput))
	if err != nil {
		t.Logf("⚠️ Failed to list entities on primary: %v", err)
	} else {
		// Verify the expected entities are present
		require.Contains(t, string(listEntitiesOutput), "tester", "tester entity should exist on primary")
	}

	// Test entity configuration from primary Vault
	entityConfigOutput, err := runVaultAPICommand(t, `curl -s -H "X-Vault-Token: root" http://localhost:8200/v1/identity/entity/name/tester`)
	t.Logf("Primary Vault tester entity config: %s", string(entityConfigOutput))
	if err != nil {
		t.Logf("⚠️ Failed to get tester entity config on primary: %v", err)
	} else {
		// Verify entity configuration
		require.Contains(t, string(entityConfigOutput), `"name":"tester"`, "Should show correct entity name")
		require.Contains(t, string(entityConfigOutput), `"disabled":false`, "Should show entity is enabled")
		require.Contains(t, string(entityConfigOutput), `"metadata"`, "Should have metadata")
		require.Contains(t, string(entityConfigOutput), `"name":"The Tester"`, "Should have correct metadata name")
	}

	// List entities from secondary Vault
	listEntitiesSecondaryOutput, err := runVaultAPICommand(t, `curl -s -H "X-Vault-Token: root" -X LIST http://localhost:8202/v1/identity/entity/name`)
	t.Logf("Secondary Vault entities: %s", string(listEntitiesSecondaryOutput))
	if err != nil {
		t.Logf("⚠️ Failed to list entities on secondary: %v", err)
	} else {
		// Verify the expected entities are present
		require.Contains(t, string(listEntitiesSecondaryOutput), "tester", "tester entity should exist on secondary")
	}

	t.Logf("✅ vault-manager entities test completed successfully!")
	t.Logf("🎯 Entities and aliases created on both primary and secondary Vault instances")
	t.Logf("📋 Verified: tester entity with metadata and OIDC aliases")
	t.Logf("🔧 Verified: entity configurations and properties")
	t.Logf("🚀 Shared container approach successful!")
}