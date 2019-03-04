#!/usr/bin/env bats

load ../helpers

@test "test vault-manager manage secret engines" {
    #
    # CASE: enable secrets engines
    #
    export GRAPHQL_QUERY_FILE=/tests/fixtures/secret-engines/enable_secrets_engines.graphql
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully enabled mount"*"path=app-interface/"* ]]
    [[ "${output}" == *"successfully enabled mount"*"path=app-sre/"* ]]
    # check secrets engines enabled
    run vault secrets  list -detailed
    [ "$status" -eq 0 ]
    [[ "${output}" == *"app-interface/"* ]]
    [[ "${output}" == *"app-sre/"* ]]
    [[ "${output}" == *"map[version:2]"* ]]

    rerun_check
}
