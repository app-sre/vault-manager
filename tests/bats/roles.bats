#!/usr/bin/env bats

load helpers

@test "test vault-manager manage approles" {
    #
    # CASE: create approles
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/roles/approle/add_roles.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully wrote AppRole"*"path=auth/approle/role/test-role-1"* ]]
    [[ "${output}" == *"successfully wrote AppRole"*"path=auth/approle/role/test-role-2"* ]]
    [[ "${output}" == *"successfully wrote AppRole"*"path=auth/approle-test-1/role/test-role-3"* ]]
    # check approles created
    run vault list auth/approle/role
    [ "$status" -eq 0 ]
    [[ "${output}" == *"test-role-1"* ]]
    [[ "${output}" == *"test-role-2"* ]]
    # check approles created
    run vault list auth/approle-test-1/role
    [ "$status" -eq 0 ]
    [[ "${output}" == *"test-role-3"* ]]
    # check approle config
    run vault read auth/approle/role/test-role-1
    [ "$status" -eq 0 ]
    [[ "${output}" == *"token_num_uses"*"1"* ]]
    [[ "${output}" == *"token_ttl"*"30m"* ]]
    [[ "${output}" == *"token_max_ttl"*"30m"* ]]
    [[ "${output}" == *"policies"*"[default role-1]"* ]]
    [[ "${output}" == *"period"*"0s"* ]]
    [[ "${output}" == *"secret_id_ttl"*"0s"* ]]
    [[ "${output}" == *"secret_id_num_uses"*"0"* ]]
    [[ "${output}" == *"bind_secret_id"*"true"* ]]
    [[ "${output}" == *"local_secret_ids"*"false"* ]]
    [[ "${output}" == *"token_bound_cidrs"*"[]"* ]]
    [[ "${output}" == *"bound_cidr_list"*"[]"* ]]
    [[ "${output}" == *"secret_id_bound_cidrs"*"[]"* ]]
    # check approle config
    run vault read auth/approle/role/test-role-2
    [ "$status" -eq 0 ]
    [[ "${output}" == *"token_num_uses"*"1"* ]]
    [[ "${output}" == *"token_ttl"*"5h"* ]]
    [[ "${output}" == *"token_max_ttl"*"5h"* ]]
    [[ "${output}" == *"policies"*"[default role-2]"* ]]
    [[ "${output}" == *"period"*"0s"* ]]
    [[ "${output}" == *"secret_id_ttl"*"10s"* ]]
    [[ "${output}" == *"secret_id_num_uses"*"0"* ]]
    [[ "${output}" == *"bind_secret_id"*"true"* ]]
    [[ "${output}" == *"local_secret_ids"*"false"* ]]
    [[ "${output}" == *"token_bound_cidrs"*"[]"* ]]
    [[ "${output}" == *"bound_cidr_list"*"[]"* ]]
    [[ "${output}" == *"secret_id_bound_cidrs"*"[]"* ]]
    # check approle config
    run vault read auth/approle-test-1/role/test-role-3
    [ "$status" -eq 0 ]
    [[ "${output}" == *"token_num_uses"*"1"* ]]
    [[ "${output}" == *"token_ttl"*"5h"* ]]
    [[ "${output}" == *"token_max_ttl"*"5h"* ]]
    [[ "${output}" == *"policies"*"[default role-2]"* ]]
    [[ "${output}" == *"period"*"0s"* ]]
    [[ "${output}" == *"secret_id_ttl"*"10s"* ]]
    [[ "${output}" == *"secret_id_num_uses"*"0"* ]]
    [[ "${output}" == *"bind_secret_id"*"true"* ]]
    [[ "${output}" == *"local_secret_ids"*"false"* ]]
    [[ "${output}" == *"token_bound_cidrs"*"[]"* ]]
    [[ "${output}" == *"bound_cidr_list"*"[]"* ]]
    [[ "${output}" == *"secret_id_bound_cidrs"*"[]"* ]]

    rerun_check

    #
    # CASE: update approles
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/roles/approle/update_roles.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully wrote AppRole"*"path=auth/approle/role/test-role-1"* ]]
    [[ "${output}" == *"successfully wrote AppRole"*"path=auth/approle/role/test-role-2"* ]]
    [[ "${output}" == *"successfully wrote AppRole"*"path=auth/approle-test-1/role/test-role-3"* ]]

    # check approle config updated
    run vault read auth/approle/role/test-role-1
    [ "$status" -eq 0 ]
    [[ "${output}" == *"token_num_uses"*"111"* ]]
    [[ "${output}" == *"token_ttl"*"3h30m30s"* ]]
    [[ "${output}" == *"token_max_ttl"*"333h"* ]]
    [[ "${output}" == *"policies"*"[default role-1 role-2]"* ]]
    [[ "${output}" == *"period"*"1h1m1s"* ]]
    [[ "${output}" == *"secret_id_ttl"*"11h11m11s"* ]]
    [[ "${output}" == *"secret_id_num_uses"*"10"* ]]
    [[ "${output}" == *"bind_secret_id"*"true"* ]]
    [[ "${output}" == *"local_secret_ids"*"false"* ]]
    [[ "${output}" == *"token_bound_cidrs"*"[]"* ]]
    [[ "${output}" == *"bound_cidr_list"*"[]"* ]]
    [[ "${output}" == *"secret_id_bound_cidrs"*"[]"* ]]

    # check approle config updated
    run vault read auth/approle/role/test-role-2
    [ "$status" -eq 0 ]
    [[ "${output}" == *"token_num_uses"*"1"* ]]
    [[ "${output}" == *"token_ttl"*"30m"* ]]
    [[ "${output}" == *"token_max_ttl"*"1h"* ]]
    [[ "${output}" == *"policies"*"[default role-2 role-3 role-4]"* ]]
    [[ "${output}" == *"period"*"2h2m2s"* ]]
    [[ "${output}" == *"secret_id_ttl"*"22h22m22s"* ]]
    [[ "${output}" == *"secret_id_num_uses"*"10"* ]]
    [[ "${output}" == *"bind_secret_id"*"true"* ]]
    [[ "${output}" == *"local_secret_ids"*"false"* ]]
    [[ "${output}" == *"token_bound_cidrs"*"[]"* ]]
    [[ "${output}" == *"bound_cidr_list"*"[]"* ]]
    [[ "${output}" == *"secret_id_bound_cidrs"*"[]"* ]]

    # check approle config updated
    run vault read auth/approle-test-1/role/test-role-3
    [ "$status" -eq 0 ]
    [[ "${output}" == *"token_num_uses"*"1"* ]]
    [[ "${output}" == *"token_ttl"*"30m"* ]]
    [[ "${output}" == *"token_max_ttl"*"1h"* ]]
    [[ "${output}" == *"policies"*"[default role-2 role-3 role-4]"* ]]
    [[ "${output}" == *"period"*"2h2m2s"* ]]
    [[ "${output}" == *"secret_id_ttl"*"22h22m22s"* ]]
    [[ "${output}" == *"secret_id_num_uses"*"10"* ]]
    [[ "${output}" == *"bind_secret_id"*"true"* ]]
    [[ "${output}" == *"local_secret_ids"*"false"* ]]
    [[ "${output}" == *"token_bound_cidrs"*"[]"* ]]
    [[ "${output}" == *"bound_cidr_list"*"[]"* ]]
    [[ "${output}" == *"secret_id_bound_cidrs"*"[]"* ]]

    rerun_check

    #
    # CASE: remove approles
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/roles/approle/remove_roles.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully deleted AppRole from Vault instance"*"path=auth/approle/role/test-role-1"* ]]
    [[ "${output}" == *"successfully deleted AppRole from Vault instance"*"path=auth/approle-test-1/role/test-role-3"* ]]
    # check approle removed
    run vault list auth/approle/role
    [ "$status" -eq 0 ]
    [[ "${output}" != *"test-role-1"* ]]
    # check role still exist
    [[ "${output}" == *"test-role-2"* ]]

    rerun_check
}
