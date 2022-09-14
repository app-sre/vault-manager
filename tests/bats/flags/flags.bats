#!/usr/bin/env bats

load ../helpers

@test "test vault-manager dry-run flag" {
    #
    # CASE: check dry-run flag
    #
    export GRAPHQL_QUERY_FILE=/tests/fixtures/audit/enable_audit_device.graphql
    run vault-manager -dry-run
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"[Dry Run] [Vault Audit] audit device to be enabled"*"instance=\"http://127.0.0.1:8200\""*"path=file/"* ]]
    [[ "${output}" == *"[Dry Run] [Vault Audit] audit device to be enabled"*"instance=\"http://127.0.0.1:8202\""*"path=file/"* ]]

    run vault audit list --detailed
    [ "$status" -eq 2 ]
    # check that no audit devices enabled
    [[ "${output}" != *"file/"* ]]

    run vault-manager -metrics=false
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"[Vault Audit] audit device is successfully enabled"*"instance=\"http://127.0.0.1:8200\""*"path=file/"* ]]
    [[ "${output}" == *"[Vault Audit] audit device is successfully enabled"*"instance=\"http://127.0.0.1:8202\""*"path=file/"* ]]

    run vault audit list --detailed
    [ "$status" -eq 0 ]
    # check file/ is enabled
    [[ "${output}" == *"file/"* ]]
    [[ "${output}" == *"file_path=/var/log/vault/vault_audit.log"* ]]

    # run same tests against secondary instance
    export VAULT_ADDR=http://127.0.0.1:8202

    run vault audit list --detailed
    [ "$status" -eq 0 ]
    # check file/ is enabled
    [[ "${output}" == *"file/"* ]]
    [[ "${output}" == *"file_path=/var/log/vault/vault_audit.log"* ]]

    export VAULT_ADDR=http://127.0.0.1:8200
}
