#!/usr/bin/env bats

load ../helpers

@test "test vault-manager policies mapping radius" {
    #
    # CASE: map policies to radius users / teams
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/policies-mapping/radius/add_policies_mapping.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully applied Vault policy to entity"*"path=/auth/radius-test-1/users/test-user-1"* ]]
    [[ "${output}" == *"successfully applied Vault policy to entity"*"path=/auth/radius-test-1/users/test-user-2"* ]]

    # check mappings applied
    check_vault_secret "list" "auth/radius-test-1/users" "test-user-1"
    check_vault_secret "list" "auth/radius-test-1/users" "test-user-2"

    # check applied
    check_vault_secret "read" "auth/radius-test-1/users/test-user-1" "policy-user-1-1 policy-user-1-2"
    check_vault_secret "read" "auth/radius-test-1/users/test-user-2" "policy-user-2-1 policy-user-2-2"

    rerun_check

    #
    # CASE: update radius entities policies
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/policies-mapping/radius/update_policies_mapping.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully applied Vault policy to entity"*"path=/auth/radius-test-1/users/test-user-1"* ]]
    [[ "${output}" == *"successfully applied Vault policy to entity"*"path=/auth/radius-test-1/users/test-user-2"* ]]

    # check policies updated
    check_vault_secret "read" "auth/radius-test-1/users/test-user-1" "policy-user-1-1-updated policy-user-1-2-updated"
    check_vault_secret "read" "auth/radius-test-1/users/test-user-2" "policy-user-2-1-updated policy-user-2-2-updated"

    rerun_check

    #
    # CASE: remove radius entities from vault
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/policies-mapping/radius/remove_policies_mapping.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully deleted entity from Vault instance"*"path=/auth/radius-test-1/users/test-user-2"* ]]

    # check entities removed
    check_vault_secret_not_exist "list" "auth/radius-test-1/users" "test-user-2"

    # check entities still exist
    check_vault_secret "list" "auth/radius-test-1/users" "test-user-1"

    rerun_check
}
