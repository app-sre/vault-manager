#!/usr/bin/env bats

load ../helpers

@test "test vault-manager manage audit device" {
    #
    # CASE: enable audit device
    #
    export GRAPHQL_QUERY_FILE=/tests/fixtures/audit/enable_audit_device.graphql
    run vault-manager
    [ "$status" -eq 0 ]
    echo "${output}"

    # check vault-manager output
    [[ "${output}" == *"[Vault Audit] audit device is successfully enabled"*"instance=\"${PRIMARY_VAULT_URL}\""*"path=file/"* ]]
    [[ "${output}" == *"[Vault Audit] audit device is successfully enabled"*"instance=\"${SECONDARY_VAULT_URL}\""*"path=file/"* ]]

    run vault audit list -address="${PRIMARY_VAULT_URL}" -detailed
    [ "$status" -eq 0 ]
    # check file/ is enabled
    [[ "${output}" == *"file/"* ]]
    [[ "${output}" == *"file_path=/var/log/vault/vault_audit.log"* ]]

    # run same tests against secondary instance
    # export VAULT_ADDR=${SECONDARY_VAULT_URL}

    run vault audit list -address="${SECONDARY_VAULT_URL}" -detailed
    [ "$status" -eq 0 ]
    # check file/ is enabled
    [[ "${output}" == *"file/"* ]]
    [[ "${output}" == *"file_path=/var/log/vault/vault_audit.log"* ]]

    rerun_check
}
