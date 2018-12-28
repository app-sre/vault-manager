#!/usr/bin/env bats

load helpers

@test "test vault-manager manage kubernetes roles" {
    #
    # CASE: create kubernetes roles
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/roles/kubernetes/add_roles.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully wrote role"*"path=auth/kubernetes-test/role/kubernetes-test-role-1"*"type=kubernetes"* ]]
    [[ "${output}" == *"successfully wrote role"*"path=auth/kubernetes-test/role/kubernetes-test-role-2"*"type=kubernetes"* ]]
    # check roles created
    run vault list auth/kubernetes-test/role
    [ "$status" -eq 0 ]
    [[ "${output}" == *"kubernetes-test-role-1"* ]]
    [[ "${output}" == *"kubernetes-test-role-2"* ]]
    # check role config
    run vault read auth/kubernetes-test/role/kubernetes-test-role-1
    [ "$status" -eq 0 ]
    [[ "${output}" == *"max_ttl"*"30m"* ]]
    [[ "${output}" == *"ttl"*"30m"* ]]
    [[ "${output}" == *"period"*"0s"* ]]
    [[ "${output}" == *"policies"*"[default dev prod]"* ]]
    [[ "${output}" == *"num_uses"*"0"* ]]
    [[ "${output}" == *"bound_cidrs"*"[]"* ]]
    [[ "${output}" == *"bound_service_account_names"*"[vault-auth]"* ]]
    [[ "${output}" == *"bound_service_account_namespaces"*"[default]"* ]]

    # check role config
    run vault read auth/kubernetes-test/role/kubernetes-test-role-2
    [ "$status" -eq 0 ]
    [[ "${output}" == *"max_ttl"*"30m"* ]]
    [[ "${output}" == *"ttl"*"30m"* ]]
    [[ "${output}" == *"period"*"0s"* ]]
    [[ "${output}" == *"policies"*"[default dev prod]"* ]]
    [[ "${output}" == *"num_uses"*"0"* ]]
    [[ "${output}" == *"bound_cidrs"*"[]"* ]]
    [[ "${output}" == *"bound_service_account_names"*"[vault-auth]"* ]]
    [[ "${output}" == *"bound_service_account_namespaces"*"[default]"* ]]

    rerun_check

    #
    # CASE: update kubernetes roles
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/roles/kubernetes/update_roles.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully wrote role"*"path=auth/kubernetes-test/role/kubernetes-test-role-1"*"type=kubernetes"* ]]

    # check approle config updated
    run vault read auth/kubernetes-test/role/kubernetes-test-role-1
    [ "$status" -eq 0 ]
    [[ "${output}" == *"max_ttl"*"11h11m11s"* ]]
    [[ "${output}" == *"ttl"*"3h30m30s"* ]]
    [[ "${output}" == *"period"*"1h1m1s"* ]]
    [[ "${output}" == *"policies"*"[default dev prod test]"* ]]
    [[ "${output}" == *"num_uses"*"0"* ]]
    [[ "${output}" == *"bound_cidrs"*"[]"* ]]
    [[ "${output}" == *"bound_cidrs"*"[]"* ]]
    [[ "${output}" == *"bound_service_account_names"*"[vault-auth]"* ]]
    [[ "${output}" == *"bound_service_account_namespaces"*"[default]"* ]]

    rerun_check

    #
    # CASE: remove kubernetes roles
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/roles/kubernetes/remove_roles.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully deleted role from Vault instance"*"path=auth/kubernetes-test/role/kubernetes-test-role-2"*"type=kubernetes"* ]]
    # check approle removed
    run vault list auth/kubernetes-test/role
    [ "$status" -eq 0 ]
    [[ "${output}" != *"test-role-2"* ]]
    # check role still exist
    [[ "${output}" == *"kubernetes-test-role-1"* ]]

    rerun_check
}
