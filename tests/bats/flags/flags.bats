#!/usr/bin/env bats

load ../helpers

@test "test vault-manager dry-run flag" {
    #
    # CASE: check dry-run flag
    #
    export GRAPHQL_QUERY_FILE=/tests/fixtures/audit/enable_audit_device.graphql

    # Run in dry-run mode and assert the plan-only output for both instances.
    run vault-manager -dry-run
    [ "$status" -eq 0 ]
    [[ "${output}" == *"[Dry Run] [Vault Audit] audit device to be enabled"* ]]
    [[ "${output}" == *"instance=\"${PRIMARY_VAULT_URL}\""* ]]
    [[ "${output}" == *"instance=\"${SECONDARY_VAULT_URL}\""* ]]

}
