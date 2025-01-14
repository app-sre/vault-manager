#!/usr/bin/env bats

# This test verifies that vault-manager can correctly manage and enable
# secret engines in a Vault setup. It ensures that the secret engines are
# successfully configured on both the primary and secondary Vault instances,
# and that they behave as expected when queried.

# The test performs the following steps:
# 1. Executes vault-manager with a GraphQL query that enables specific secret engines.
# 2. Verifies that  vault-manager outputs success messages for enabling secret engines
#    on the primary and secondary Vault instances.
# 3. Confirms that the secret engines ("app-interface/" and "app-sre/") are listed in the
#    enabled secrets when queried from the primary Vault instance.
# 4. Repeats the verification step for the secondary Vault instance to ensure consistency.
# 5. Checks that the secret engines are correctly configured with version 2.

load ../helpers

@test "test vault-manager manage secret engines" {
    #
    # CASE: enable secrets engines
    #
    export GRAPHQL_QUERY_FILE=/tests/fixtures/secret-engines/enable_secrets_engines.graphql
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"[Vault Secrets engine] successfully enabled secrets-engine"*"instance=\"${PRIMARY_VAULT_URL}\""*"path=app-interface/"* ]]
    [[ "${output}" == *"[Vault Secrets engine] successfully enabled secrets-engine"*"instance=\"${PRIMARY_VAULT_URL}\""*"path=app-sre/"* ]]
    [[ "${output}" == *"[Vault Secrets engine] successfully enabled secrets-engine"*"instance=\"${SECONDARY_VAULT_URL}\""*"path=app-interface/"* ]]
    [[ "${output}" == *"[Vault Secrets engine] successfully enabled secrets-engine"*"instance=\"${SECONDARY_VAULT_URL}\""*"path=app-sre/"* ]]

    # verify secrets engines enabled
    run vault secrets  list -detailed
    [ "$status" -eq 0 ]
    [[ "${output}" == *"app-interface/"* ]]
    [[ "${output}" == *"app-sre/"* ]]
    [[ "${output}" == *"map[version:2]"* ]]

    # repeat tests against secondary instance
    export VAULT_ADDR=${SECONDARY_VAULT_URL}

    # verify secrets engines are enabled
    run vault secrets list -detailed
    [ "$status" -eq 0 ]
    [[ "${output}" == *"app-interface/"* ]]
    [[ "${output}" == *"app-sre/"* ]]
    [[ "${output}" == *"map[version:2]"* ]]

    export VAULT_ADDR=${PRIMARY_VAULT_URL}
    rerun_check
}
