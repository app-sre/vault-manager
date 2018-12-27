#!/usr/bin/env bats

load helpers

@test "test vault-manager manage alicloud roles" {
    #
    # CASE: create alicloud roles
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/roles/alicloud/add_roles.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully wrote role"*"path=auth/alicloud-test/role/alicloud-test-role-1"*"type=alicloud"* ]]
    [[ "${output}" == *"successfully wrote role"*"path=auth/alicloud-test/role/alicloud-test-role-2"*"type=alicloud"* ]]
    # check roles created
    run vault list auth/alicloud-test/role
    [ "$status" -eq 0 ]
    [[ "${output}" == *"alicloud-test-role-1"* ]]
    [[ "${output}" == *"alicloud-test-role-1"* ]]
    # check role config
    run vault read auth/alicloud-test/role/alicloud-test-role-1
    [ "$status" -eq 0 ]
    [[ "${output}" == *"arn"*"acs:ram::5138828231865461:role/dev-role"* ]]
    [[ "${output}" == *"bound_cidrs"*"[]"* ]]
    [[ "${output}" == *"max_ttl"*"30m"* ]]
    [[ "${output}" == *"ttl"*"30m"* ]]
    [[ "${output}" == *"period"*"0s"* ]]
    [[ "${output}" == *"policies"*"[default dev prod]"* ]]
    # check role config
    run vault read auth/alicloud-test/role/alicloud-test-role-2
    [ "$status" -eq 0 ]
    [[ "${output}" == *"arn"*"acs:ram::5138828231865461:role/dev-role"* ]]
    [[ "${output}" == *"bound_cidrs"*"[]"* ]]
    [[ "${output}" == *"max_ttl"*"30m"* ]]
    [[ "${output}" == *"ttl"*"30m"* ]]
    [[ "${output}" == *"period"*"0s"* ]]
    [[ "${output}" == *"policies"*"[default dev prod]"* ]]

    rerun_check

    #
    # CASE: update alicloud roles
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/roles/alicloud/update_roles.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully wrote role"*"path=auth/alicloud-test/role/alicloud-test-role-1"*"type=alicloud"* ]]

    # check approle config updated
    run vault read auth/alicloud-test/role/alicloud-test-role-1
    [ "$status" -eq 0 ]
    [[ "${output}" == *"arn"*"acs:ram::5138828231865461:role/dev-role"* ]]
    [[ "${output}" == *"bound_cidrs"*"[]"* ]]
    [[ "${output}" == *"max_ttl"*"11h11m11s"* ]]
    [[ "${output}" == *"ttl"*"3h30m30s"* ]]
    [[ "${output}" == *"period"*"1h1m1s"* ]]
    [[ "${output}" == *"policies"*"[default dev prod]"* ]]

    rerun_check

    #
    # CASE: remove alicloud roles
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/roles/alicloud/remove_roles.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully deleted role from Vault instance"*"path=auth/alicloud-test/role/alicloud-test-role-2"*"type=alicloud"* ]]
    # check approle removed
    run vault list auth/alicloud-test/role
    [ "$status" -eq 0 ]
    [[ "${output}" != *"test-role-2"* ]]
    # check role still exist
    [[ "${output}" == *"alicloud-test-role-1"* ]]

    rerun_check
}
