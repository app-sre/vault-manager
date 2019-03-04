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
    [[ "${output}" == *"[Dry Run]"*"package=audit"*"entry to be written="*"file/"* ]]

    run vault audit list --detailed
    [ "$status" -eq 0 ]
    # check that no audit devices enabled
    [[ "${output}" != *"file/"* ]]

    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"audit successfully enabled"*"path=file/"* ]]
}
