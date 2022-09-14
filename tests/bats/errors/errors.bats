#!/usr/bin/env bats

load ../helpers

@test "test vault-manager error handling - part 1" {
    #
    # CASE: invalid login credentials for instance
    #
    export GRAPHQL_QUERY_FILE=/tests/fixtures/errors/missing_oidc_secret.graphql

    # update credential utilized to configure secondary vault client with invalid data
    export VAULT_ADDR=http://127.0.0.1:8200
    vault kv put secret/secondary root=badroot

    # remove an existing auth in order to trigger reconciliation output for the valid instance
    # goal is to test that even with an error occurring on one instance,
    # reconcile can continue on the other w/out issue
    vault auth disable oidc/

    # deletion of oidc with dependent entities still existing is not a normal order of operations
    # manually deleting the "orphaned" entity
    # we test for creation of this entity to further validate error handling on per instance basis
    vault delete identity/entity/name/tester

    run vault-manager -metrics=false
    [[ "${output}" == *"SKIPPING ALL RECONCILIATION FOR: http://127.0.0.1:8202"* ]]
    [[ "${output}" == *"[Vault Auth] successfully enabled auth backend"*"instance=\"http://127.0.0.1:8200\""*"path=oidc/"*"type=oidc"* ]]
    [[ "${output}" == *"[Vault Identity] entity successfully written"*"instance=\"http://127.0.0.1:8200\""*"path=identity/entity/name/tester"*"type=entity"* ]]
    [[ "${output}" == *"[Vault Identity] entity alias successfully written"*"instance=\"http://127.0.0.1:8200\""*"path=identity/entity-alias/tester"*"type=oidc"* ]]
}

@test "test vault-manager error handling - part 2" {
    #
    # CASE: missing oidc secret
    #
    export GRAPHQL_QUERY_FILE=/tests/fixtures/errors/missing_oidc_secret.graphql

    # fix credential from prior test
    export VAULT_ADDR=http://127.0.0.1:8200
    vault kv put secret/secondary root=root

    # remove a dependency for auth reconciliation (intentionally cause an error)
    export VAULT_ADDR=http://127.0.0.1:8202
    vault kv delete secret/oidc

    # remove an existing entity in order to trigger reconcile output
    # goal is to test that even with an error occurring on one instance,
    # reconcile can continue on the other w/out issue
    export VAULT_ADDR=http://127.0.0.1:8200
    vault delete identity/entity/name/tester

    run vault-manager -metrics=false
    [[ "${output}" == *"SKIPPING REMAINING RECONCILIATION FOR http://127.0.0.1:8202"* ]]
    [[ "${output}" == *"[Vault Identity] entity successfully written"*"instance=\"http://127.0.0.1:8200\""*"path=identity/entity/name/tester"*"type=entity"* ]]
    [[ "${output}" == *"[Vault Identity] entity alias successfully written"*"instance=\"http://127.0.0.1:8200\""*"path=identity/entity-alias/tester"*"type=oidc"* ]]
}
