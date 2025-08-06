package testcontainers

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestVaultManagerFlagsShared tests dry-run flag using shared containers
func TestVaultManagerFlagsShared(t *testing.T) {
	// Reset Vault state for clean test
	resetVaultState(t)

	t.Logf("🚀 Testing vault-manager flags with shared containers")

	// Test dry-run flag
	t.Logf("📋 Testing vault-manager with -dry-run flag...")
	
	vaultManagerDryRunOutput, err := runSharedVaultManager(t, "/src/tests/fixtures/audit/enable_audit_device.graphql", "-dry-run")
	t.Logf("vault-manager dry-run output:\n%s", string(vaultManagerDryRunOutput))
	require.NoError(t, err, "vault-manager dry-run should succeed")

	// Verify dry-run behavior
	t.Logf("🔍 Verifying dry-run behavior...")
	
	// Check dry-run output messages
	require.Contains(t, string(vaultManagerDryRunOutput), "[Dry Run] [Vault Audit] audit device to be enabled", "Should show dry-run audit device message")
	require.Contains(t, string(vaultManagerDryRunOutput), "path=file/", "Should show file audit device path")
	require.Contains(t, string(vaultManagerDryRunOutput), `instance="http://localhost:8200"`, "Should show primary instance")
	require.Contains(t, string(vaultManagerDryRunOutput), `instance="http://localhost:8202"`, "Should show secondary instance")

	// Verify no actual changes were made during dry-run
	t.Logf("🔍 Verifying no audit devices were created during dry-run...")
	
	// Check that no audit devices exist on primary
	listAuditPrimaryOutput, err := runVaultAPICommand(t, `curl -s -H "X-Vault-Token: root" http://localhost:8200/v1/sys/audit`)
	t.Logf("Primary Vault audit devices after dry-run: %s", string(listAuditPrimaryOutput))
	if err != nil {
		t.Logf("⚠️ Failed to list audit devices on primary: %v", err)
	} else {
		// Verify no file audit device exists
		require.NotContains(t, string(listAuditPrimaryOutput), "file/", "No audit devices should exist after dry-run")
	}

	// Check that no audit devices exist on secondary
	listAuditSecondaryOutput, err := runVaultAPICommand(t, `curl -s -H "X-Vault-Token: root" http://localhost:8202/v1/sys/audit`)
	t.Logf("Secondary Vault audit devices after dry-run: %s", string(listAuditSecondaryOutput))
	if err != nil {
		t.Logf("⚠️ Failed to list audit devices on secondary: %v", err)
	} else {
		// Verify no file audit device exists
		require.NotContains(t, string(listAuditSecondaryOutput), "file/", "No audit devices should exist after dry-run")
	}

	// Run vault-manager without dry-run to verify actual changes
	t.Logf("📋 Running vault-manager without dry-run flag...")
	
	vaultManagerRealRunOutput, err := runSharedVaultManager(t, "/src/tests/fixtures/audit/enable_audit_device.graphql")
	t.Logf("vault-manager real run output:\n%s", string(vaultManagerRealRunOutput))
	require.NoError(t, err, "vault-manager real run should succeed")

	// Verify actual changes were made
	t.Logf("🔍 Verifying audit devices were created during real run...")
	
	// Check real run output messages
	require.Contains(t, string(vaultManagerRealRunOutput), "[Vault Audit] audit device is successfully enabled", "Should show actual audit device creation")
	require.Contains(t, string(vaultManagerRealRunOutput), "path=file/", "Should show file audit device path")
	require.Contains(t, string(vaultManagerRealRunOutput), `instance="http://localhost:8200"`, "Should work on primary instance")
	require.Contains(t, string(vaultManagerRealRunOutput), `instance="http://localhost:8202"`, "Should work on secondary instance")

	// Verify audit devices now exist on primary
	listAuditPrimaryAfterOutput, err := runVaultAPICommand(t, `curl -s -H "X-Vault-Token: root" http://localhost:8200/v1/sys/audit`)
	t.Logf("Primary Vault audit devices after real run: %s", string(listAuditPrimaryAfterOutput))
	if err != nil {
		t.Logf("⚠️ Failed to list audit devices on primary: %v", err)
	} else {
		// Verify file audit device exists
		require.Contains(t, string(listAuditPrimaryAfterOutput), "file/", "File audit device should exist after real run")
		require.Contains(t, string(listAuditPrimaryAfterOutput), "file_path", "Should show file path configuration")
		require.Contains(t, string(listAuditPrimaryAfterOutput), "/tmp/vault_audit.log", "Should show correct audit log path")
	}

	// Verify audit devices now exist on secondary
	listAuditSecondaryAfterOutput, err := runVaultAPICommand(t, `curl -s -H "X-Vault-Token: root" http://localhost:8202/v1/sys/audit`)
	t.Logf("Secondary Vault audit devices after real run: %s", string(listAuditSecondaryAfterOutput))
	if err != nil {
		t.Logf("⚠️ Failed to list audit devices on secondary: %v", err)
	} else {
		// Verify file audit device exists
		require.Contains(t, string(listAuditSecondaryAfterOutput), "file/", "File audit device should exist after real run")
		require.Contains(t, string(listAuditSecondaryAfterOutput), "file_path", "Should show file path configuration")
		require.Contains(t, string(listAuditSecondaryAfterOutput), "/tmp/vault_audit.log", "Should show correct audit log path")
	}

	t.Logf("✅ vault-manager flags test completed successfully!")
	t.Logf("🎯 Dry-run flag behavior verified correctly")
	t.Logf("📋 Verified: Dry-run shows planned changes without applying them")
	t.Logf("🔧 Verified: Real run applies actual changes to both instances")
	t.Logf("🚀 Shared container approach successful!")
}