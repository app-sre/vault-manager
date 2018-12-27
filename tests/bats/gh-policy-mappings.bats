#!/usr/bin/env bats

load helpers

@test "test vault-manager gh policy mappings" {
    #
    # CASE: map policy to gh users / teams
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/gh-policy-mappings/add_gh_policy_mappings.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully applied Vault policy to Github entity"*"path=/auth/github-test-1/map/teams/test-team-1"* ]]
    [[ "${output}" == *"successfully applied Vault policy to Github entity"*"path=/auth/github-test-1/map/teams/test-team-2"* ]]
    [[ "${output}" == *"successfully applied Vault policy to Github entity"*"path=/auth/github-test-1/map/users/test-user-1"* ]]
    [[ "${output}" == *"successfully applied Vault policy to Github entity"*"path=/auth/github-test-1/map/users/test-user-2"* ]]
    [[ "${output}" == *"successfully applied Vault policy to Github entity"*"path=/auth/github-test-2/map/teams/test-team-3"* ]]
    [[ "${output}" == *"successfully applied Vault policy to Github entity"*"path=/auth/github-test-2/map/users/test-user-3"* ]]

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
    # CASE: update gh entities policies
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/gh-policy-mappings/update_gh_policy_mappings.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully applied Vault policy to Github entity"*"path=/auth/github-test-1/map/teams/test-team-1"* ]]
    [[ "${output}" == *"successfully applied Vault policy to Github entity"*"path=/auth/github-test-1/map/users/test-user-1"* ]]
    [[ "${output}" == *"successfully applied Vault policy to Github entity"*"path=/auth/github-test-2/map/users/test-user-3"* ]]

    # check policies updated
    check_vault_secret "read" "auth/github-test-1/map/teams/test-team-1" "policy-team-1-updated"
    check_vault_secret "read" "auth/github-test-1/map/users/test-user-1" "policy-user-updated"
    check_vault_secret "read" "auth/github-test-2/map/users/test-user-3" "policy-user-3-updated"

    rerun_check

    #
    # CASE: remove gh entities from vault
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/gh-policy-mappings/remove_gh_policy_mappings.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully deleted GitHub entity from Vault instance"*"path=/auth/github-test-1/map/teams/test-team-1"* ]]
    [[ "${output}" == *"successfully deleted GitHub entity from Vault instance"*"path=/auth/github-test-1/map/users/test-user-2"* ]]
    [[ "${output}" == *"successfully deleted GitHub entity from Vault instance"*"path=/auth/github-test-2/map/teams/test-team-3"* ]]
    [[ "${output}" == *"successfully deleted GitHub entity from Vault instance"*"path=/auth/github-test-2/map/users/test-user-3"* ]]

    # check entities removed
    check_vault_secret_not_exist "list" "auth/github-test-1/map/teams" "test-team-1"
    check_vault_secret_not_exist "list" "auth/github-test-1/map/users" "test-team-2"

    # check entities still exist
    check_vault_secret "list" "auth/github-test-1/map/teams" "test-team-2"
    check_vault_secret "list" "auth/github-test-1/map/users" "test-user-1"

    rerun_check
}
