#!/usr/bin/env bats

load ../helpers

@test "test vault-manager manage audit device" {
    #
    # CASE: enable audit device
    #
    export GRAPHQL_QUERY_FILE=/tests/fixtures/audit/enable_audit_device.graphql
    run vault-manager
    decho ${output}
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"audit successfully enabled"*"path=file/"* ]]
    run vault audit list --detailed
    [ "$status" -eq 0 ]
    # check file1/ is enabled
    [[ "${output}" == *"file/"* ]]
    [[ "${output}" == *"file_path=/var/log/vault/vault_audit.log"* ]]

    rerun_check
}
