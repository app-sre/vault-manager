#!/usr/bin/env bats

load ../helpers

@test "test vault-manager error handling - part 1" {
    #
    # CASE: invalid login credentials for instance
    #
    export GRAPHQL_QUERY_FILE=/tests/fixtures/errors/missing_oidc_secret.graphql

    # update credential utilized to configure secondary vault client with invalid data
    # export VAULT_ADDR=${PRIMARY_VAULT_URL}
    vault kv put -address="${PRIMARTY_VAULT_URL}" secret/secondary root=badroot

    # remove an existing auth in order to trigger reconciliation output for the valid instance
    # goal is to test that even with an error occurring on one instance,
    # reconcile can continue on the other w/out issue
    vault auth disable -address="${PRIMARY_VAULT_URL}" oidc/

    # deletion of oidc with dependent entities still existing is not a normal order of operations
    # manually deleting the "orphaned" entity
    # we test for creation of this entity to further validate error handling on per instance basis
    vault delete -address="${PRIMARY_VAULT_URL}" identity/entity/name/tester
    vault delete -address="${PRIMARY_VAULT_URL}" identity/entity/name/tester2

    run vault-manager
    [[ "${output}" == *"SKIPPING ALL RECONCILIATION FOR: ${SECONDARY_VAULT_URL}"* ]]
    [[ "${output}" == *"[Vault Auth] successfully enabled auth backend"*"instance=\"${PRIMARY_VAULT_URL}\""*"path=oidc/"*"type=oidc"* ]]
    [[ "${output}" == *"[Vault Identity] entity successfully written"*"instance=\"${PRIMARY_VAULT_URL}\""*"path=identity/entity/name/tester"*"type=entity"* ]]
    [[ "${output}" == *"[Vault Identity] entity alias successfully written"*"instance=\"${PRIMARY_VAULT_URL}\""*"path=identity/entity-alias/tester"*"type=oidc"* ]]
}

@test "test vault-manager error handling - part 2" {
    #
    # CASE: missing oidc secret
    #
    export GRAPHQL_QUERY_FILE=/tests/fixtures/errors/missing_oidc_secret.graphql

    # fix credential from prior test
    # export VAULT_ADDR=${PRIMARY_VAULT_URL}
    vault kv put -address="${PRIMARY_VAULT_URL}" secret/secondary root=root

    # remove a dependency for auth reconciliation (intentionally cause an error)
    vault kv delete -address="${SECONDARY_VAULT_URL}" secret/oidc

    # remove an existing entity in order to trigger reconcile output
    # goal is to test that even with an error occurring on one instance,
    # reconcile can continue on the other w/out issue
    vault delete -address="${PRIMARY_VAULT_URL}" identity/entity/name/tester
    vault delete -address="${PRIMARY_VAULT_URL}" identity/entity/name/tester2

    run vault-manager
    [[ "${output}" == *"SKIPPING REMAINING RECONCILIATION FOR ${SECONDARY_VAULT_URL}"* ]]
    [[ "${output}" == *"[Vault Identity] entity successfully written"*"instance=\"${PRIMARY_VAULT_URL}\""*"path=identity/entity/name/tester"*"type=entity"* ]]
    [[ "${output}" == *"[Vault Identity] entity alias successfully written"*"instance=\"${PRIMARY_VAULT_URL}\""*"path=identity/entity-alias/tester"*"type=oidc"* ]]
}
