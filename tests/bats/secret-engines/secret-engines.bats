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
    [[ "${output}" == *"[Vault Secrets engine] successfully enabled secrets-engine"*instance=\"${PRIMARY_VAULT_URL}\"*path=app-interface/* ]]
    [[ "${output}" == *"[Vault Secrets engine] successfully enabled secrets-engine"*instance=\"${PRIMARY_VAULT_URL}\"*path=app-sre/* ]]
    [[ "${output}" == *"[Vault Secrets engine] successfully enabled secrets-engine"*instance=\"${SECONDARY_VAULT_URL}\"*path=app-interface/* ]]
    [[ "${output}" == *"[Vault Secrets engine] successfully enabled secrets-engine"*instance=\"${SECONDARY_VAULT_URL}\"*path=app-sre/* ]]
    # check secrets engines enabled
    run vault secrets  list -detailed
    [ "$status" -eq 0 ]
    [[ "${output}" == *"app-interface/"* ]]
    [[ "${output}" == *"app-sre/"* ]]
    [[ "${output}" == *"map[version:2]"* ]]

    # run same tests against secondary instance
    export VAULT_ADDR=${SECONDARY_VAULT_URL}

    # check secrets engines enabled
    run vault secrets list -detailed
    [ "$status" -eq 0 ]
    [[ "${output}" == *"app-interface/"* ]]
    [[ "${output}" == *"app-sre/"* ]]
    [[ "${output}" == *"map[version:2]"* ]]

    export VAULT_ADDR=${PRIMARY_VAULT_URL}
    rerun_check
}
