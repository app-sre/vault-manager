#!/usr/bin/env bats

load ../helpers

@test "test vault-manager policies mapping github" {
    #
    # CASE: map policies to github users / teams
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/policies-mapping/github/add_policies_mapping.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully applied Vault policy to entity"*"path=/auth/github-test-1/map/teams/test-team-1"* ]]
    [[ "${output}" == *"successfully applied Vault policy to entity"*"path=/auth/github-test-1/map/teams/test-team-2"* ]]
    [[ "${output}" == *"successfully applied Vault policy to entity"*"path=/auth/github-test-1/map/users/test-user-1"* ]]
    [[ "${output}" == *"successfully applied Vault policy to entity"*"path=/auth/github-test-1/map/users/test-user-2"* ]]
    [[ "${output}" == *"successfully applied Vault policy to entity"*"path=/auth/github-test-2/map/teams/test-team-3"* ]]
    [[ "${output}" == *"successfully applied Vault policy to entity"*"path=/auth/github-test-2/map/users/test-user-3"* ]]

    # check mappings applied
    check_vault_secret "list" "auth/github-test-1/map/teams" "test-team-1"
    check_vault_secret "list" "auth/github-test-1/map/teams" "test-team-2"
    check_vault_secret "list" "auth/github-test-1/map/users" "test-user-1"
    check_vault_secret "list" "auth/github-test-1/map/users" "test-user-2"
    check_vault_secret "list" "auth/github-test-2/map/teams" "test-team-3"
    check_vault_secret "list" "auth/github-test-2/map/users" "test-user-3"

    # check applied
    check_vault_secret "read" "auth/github-test-1/map/teams/test-team-1" "policy-team-1-1,policy-team-1-2"
    check_vault_secret "read" "auth/github-test-1/map/teams/test-team-2" "policy-team-2-1,policy-team-2-2"
    check_vault_secret "read" "auth/github-test-1/map/users/test-user-1" "policy-user-1-1,policy-user-1-2"
    check_vault_secret "read" "auth/github-test-1/map/users/test-user-2" "policy-user-2-1,policy-user-2-2"
    check_vault_secret "read" "auth/github-test-2/map/teams/test-team-3" "policy-team-3-1,policy-team-3-2"
    check_vault_secret "read" "auth/github-test-2/map/users/test-user-3" "policy-user-3-1,policy-user-3-2"

    rerun_check

    #
    # CASE: update github entities policies
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/policies-mapping/github/update_policies_mapping.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully applied Vault policy to entity"*"path=/auth/github-test-1/map/teams/test-team-1"* ]]
    [[ "${output}" == *"successfully applied Vault policy to entity"*"path=/auth/github-test-1/map/users/test-user-1"* ]]
    [[ "${output}" == *"successfully applied Vault policy to entity"*"path=/auth/github-test-2/map/users/test-user-3"* ]]

    # check policies updated
    check_vault_secret "read" "auth/github-test-1/map/teams/test-team-1" "policy-team-1-updated"
    check_vault_secret "read" "auth/github-test-1/map/users/test-user-1" "policy-user-updated"
    check_vault_secret "read" "auth/github-test-2/map/users/test-user-3" "policy-user-3-updated"

    rerun_check

    #
    # CASE: remove github entities from vault
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/policies-mapping/github/remove_policies_mapping.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully deleted entity from Vault instance"*"path=/auth/github-test-1/map/teams/test-team-1"* ]]
    [[ "${output}" == *"successfully deleted entity from Vault instance"*"path=/auth/github-test-1/map/users/test-user-2"* ]]
    [[ "${output}" == *"successfully deleted entity from Vault instance"*"path=/auth/github-test-2/map/teams/test-team-3"* ]]
    [[ "${output}" == *"successfully deleted entity from Vault instance"*"path=/auth/github-test-2/map/users/test-user-3"* ]]

    # check entities removed
    check_vault_secret_not_exist "list" "auth/github-test-1/map/teams" "test-team-1"
    check_vault_secret_not_exist "list" "auth/github-test-1/map/users" "test-team-2"

    # check entities still exist
    check_vault_secret "list" "auth/github-test-1/map/teams" "test-team-2"
    check_vault_secret "list" "auth/github-test-1/map/users" "test-user-1"

    rerun_check
}
