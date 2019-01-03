#!/usr/bin/env bats

load ../helpers

@test "test vault-manager manage jwt roles" {
    #
    # CASE: create jwt roles
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/roles/jwt/add_roles.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully wrote role"*"path=auth/jwt-test/role/jwt-test-role-1"*"type=jwt"* ]]
    [[ "${output}" == *"successfully wrote role"*"path=auth/jwt-test/role/jwt-test-role-2"*"type=jwt"* ]]
    # check roles created
    run vault list auth/jwt-test/role
    [ "$status" -eq 0 ]
    [[ "${output}" == *"jwt-test-role-1"* ]]
    [[ "${output}" == *"jwt-test-role-2"* ]]
    # check role config
    run vault read auth/jwt-test/role/jwt-test-role-1
    [ "$status" -eq 0 ]
    [[ "${output}" == *"max_ttl"*"30m"* ]]
    [[ "${output}" == *"ttl"*"30m"* ]]
    [[ "${output}" == *"period"*"0s"* ]]
    [[ "${output}" == *"policies"*"[default dev prod]"* ]]
    [[ "${output}" == *"num_uses"*"0"* ]]
    [[ "${output}" == *"bound_audiences"*"[https://vault.plugin.auth.jwt.test]"* ]]
    [[ "${output}" == *"bound_cidrs"*"[]"* ]]
    [[ "${output}" == *"bound_subject"*"test@clients"* ]]
    [[ "${output}" == *"user_claim"*"https://vault/user"* ]]
    [[ "${output}" == *"groups_claim"*"https://vault/groups"* ]]

    # check role config
    run vault read auth/jwt-test/role/jwt-test-role-2
    [ "$status" -eq 0 ]
    [[ "${output}" == *"max_ttl"*"30m"* ]]
    [[ "${output}" == *"ttl"*"30m"* ]]
    [[ "${output}" == *"period"*"0s"* ]]
    [[ "${output}" == *"policies"*"[default dev prod]"* ]]
    [[ "${output}" == *"num_uses"*"0"* ]]
    [[ "${output}" == *"bound_audiences"*"[https://vault.plugin.auth.jwt.test]"* ]]
    [[ "${output}" == *"bound_cidrs"*"[]"* ]]
    [[ "${output}" == *"bound_subject"*"test@clients"* ]]
    [[ "${output}" == *"user_claim"*"https://vault/user"* ]]
    [[ "${output}" == *"groups_claim"*"https://vault/groups"* ]]

    rerun_check

    #
    # CASE: update jwt roles
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/roles/jwt/update_roles.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully wrote role"*"path=auth/jwt-test/role/jwt-test-role-1"*"type=jwt"* ]]

    # check approle config updated
    run vault read auth/jwt-test/role/jwt-test-role-1
    [ "$status" -eq 0 ]
    [[ "${output}" == *"max_ttl"*"11h11m11s"* ]]
    [[ "${output}" == *"ttl"*"3h30m30s"* ]]
    [[ "${output}" == *"period"*"1h1m1s"* ]]
    [[ "${output}" == *"policies"*"[default dev prod test]"* ]]
    [[ "${output}" == *"num_uses"*"0"* ]]
    [[ "${output}" == *"bound_audiences"*"[https://vault.plugin.auth.jwt.test]"* ]]
    [[ "${output}" == *"bound_cidrs"*"[]"* ]]
    [[ "${output}" == *"bound_subject"*"test@clients"* ]]
    [[ "${output}" == *"user_claim"*"https://vault/user"* ]]
    [[ "${output}" == *"groups_claim"*"https://vault/groups"* ]]

    rerun_check

    #
    # CASE: remove jwt roles
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/roles/jwt/remove_roles.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully deleted role from Vault instance"*"path=auth/jwt-test/role/jwt-test-role-2"*"type=jwt"* ]]
    # check approle removed
    run vault list auth/jwt-test/role
    [ "$status" -eq 0 ]
    [[ "${output}" != *"test-role-2"* ]]
    # check role still exist
    [[ "${output}" == *"jwt-test-role-1"* ]]

    rerun_check
}
