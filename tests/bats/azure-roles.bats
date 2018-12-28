#!/usr/bin/env bats

load helpers

@test "test vault-manager manage azure roles" {
    #
    # CASE: create azure roles
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/roles/azure/add_roles.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully wrote role"*"path=auth/azure-test/role/azure-test-role-1"*"type=azure"* ]]
    [[ "${output}" == *"successfully wrote role"*"path=auth/azure-test/role/azure-test-role-2"*"type=azure"* ]]
    # check roles created
    run vault list auth/azure-test/role
    [ "$status" -eq 0 ]
    [[ "${output}" == *"azure-test-role-1"* ]]
    [[ "${output}" == *"azure-test-role-1"* ]]
    # check role config
    run vault read auth/azure-test/role/azure-test-role-1
    [ "$status" -eq 0 ]
    [[ "${output}" == *"max_ttl"*"30m"* ]]
    [[ "${output}" == *"ttl"*"30m"* ]]
    [[ "${output}" == *"period"*"0s"* ]]
    [[ "${output}" == *"policies"*"[default dev prod]"* ]]
    [[ "${output}" == *"num_uses"*"0"* ]]
    [[ "${output}" == *"bound_service_principal_ids"*"[]"* ]]
    [[ "${output}" == *"bound_group_ids"*"[group-id]"* ]]
    [[ "${output}" == *"bound_subscription_ids"*"[subscription-id]"* ]]
    [[ "${output}" == *"bound_scale_sets"*"[]"* ]]
    [[ "${output}" == *"bound_locations"*"[]"* ]]
    [[ "${output}" == *"bound_resource_groups"*"[]"* ]]

    # check role config
    run vault read auth/azure-test/role/azure-test-role-2
    [ "$status" -eq 0 ]
    [[ "${output}" == *"max_ttl"*"30m"* ]]
    [[ "${output}" == *"ttl"*"30m"* ]]
    [[ "${output}" == *"period"*"0s"* ]]
    [[ "${output}" == *"policies"*"[default dev prod]"* ]]
    [[ "${output}" == *"num_uses"*"0"* ]]
    [[ "${output}" == *"bound_service_principal_ids"*"[]"* ]]
    [[ "${output}" == *"bound_group_ids"*"[group-id]"* ]]
    [[ "${output}" == *"bound_subscription_ids"*"[subscription-id]"* ]]
    [[ "${output}" == *"bound_scale_sets"*"[]"* ]]
    [[ "${output}" == *"bound_locations"*"[]"* ]]
    [[ "${output}" == *"bound_resource_groups"*"[]"* ]]

    rerun_check

    #
    # CASE: update azure roles
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/roles/azure/update_roles.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully wrote role"*"path=auth/azure-test/role/azure-test-role-1"*"type=azure"* ]]

    # check approle config updated
    run vault read auth/azure-test/role/azure-test-role-1
    [ "$status" -eq 0 ]
    [[ "${output}" == *"max_ttl"*"11h11m11s"* ]]
    [[ "${output}" == *"ttl"*"3h30m30s"* ]]
    [[ "${output}" == *"period"*"1h1m1s"* ]]
    [[ "${output}" == *"policies"*"[default dev prod]"* ]]
    [[ "${output}" == *"num_uses"*"0"* ]]
    [[ "${output}" == *"bound_service_principal_ids"*"[]"* ]]
    [[ "${output}" == *"bound_group_ids"*"[group-id]"* ]]
    [[ "${output}" == *"bound_subscription_ids"*"[subscription-id]"* ]]
    [[ "${output}" == *"bound_scale_sets"*"[]"* ]]
    [[ "${output}" == *"bound_locations"*"[]"* ]]
    [[ "${output}" == *"bound_resource_groups"*"[]"* ]]

    rerun_check

    #
    # CASE: remove azure roles
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/roles/azure/remove_roles.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully deleted role from Vault instance"*"path=auth/azure-test/role/azure-test-role-2"*"type=azure"* ]]
    # check approle removed
    run vault list auth/azure-test/role
    [ "$status" -eq 0 ]
    [[ "${output}" != *"test-role-2"* ]]
    # check role still exist
    [[ "${output}" == *"azure-test-role-1"* ]]

    rerun_check
}
