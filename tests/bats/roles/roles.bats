#!/usr/bin/env bats

load ../helpers

@test "test vault-manager manage roles" {
    #
    # CASE: create roles
    #
    export GRAPHQL_QUERY_FILE=/tests/fixtures/roles/enable_vault_roles.graphql
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully wrote role"*"path=auth/approle/role/app-interface"*"type=approle"* ]]
    [[ "${output}" == *"successfully wrote role"*"path=auth/approle/role/vault_manager"*"type=approle"* ]]

    # check approles created
    run vault list auth/approle/role
    [ "$status" -eq 0 ]
    [[ "${output}" == *"app-interface"* ]]
    [[ "${output}" == *"vault_manager"* ]]

    # check approle config
    run vault read auth/approle/role/app-interface
    [ "$status" -eq 0 ]
    [[ "${output}" == *"token_num_uses"*"1"* ]]
    [[ "${output}" == *"token_ttl"*"30m"* ]]
    [[ "${output}" == *"token_max_ttl"*"30m"* ]]
    [[ "${output}" == *"policies"*"[app-interface-approle-policy]"* ]]
    [[ "${output}" == *"period"*"0s"* ]]
    [[ "${output}" == *"secret_id_ttl"*"0s"* ]]
    [[ "${output}" == *"secret_id_num_uses"*"0"* ]]
    [[ "${output}" == *"bind_secret_id"*"true"* ]]
    [[ "${output}" == *"local_secret_ids"*"false"* ]]
    [[ "${output}" == *"token_bound_cidrs"*"[]"* ]]
    [[ "${output}" == *"bound_cidr_list"*"[]"* ]]
    [[ "${output}" == *"secret_id_bound_cidrs"*"[]"* ]]
    [[ "${output}" == *"token_type"*"default"* ]]
    # check approle config
    run vault read auth/approle/role/vault_manager
    [ "$status" -eq 0 ]
    [[ "${output}" == *"token_num_uses"*"1"* ]]
    [[ "${output}" == *"token_ttl"*"30m"* ]]
    [[ "${output}" == *"token_max_ttl"*"30m"* ]]
    [[ "${output}" == *"policies"*"[vault-manager-policy]"* ]]
    [[ "${output}" == *"period"*"0s"* ]]
    [[ "${output}" == *"secret_id_ttl"*"0s"* ]]
    [[ "${output}" == *"secret_id_num_uses"*"0"* ]]
    [[ "${output}" == *"bind_secret_id"*"true"* ]]
    [[ "${output}" == *"local_secret_ids"*"false"* ]]
    [[ "${output}" == *"token_bound_cidrs"*"[]"* ]]
    [[ "${output}" == *"bound_cidr_list"*"[]"* ]]
    [[ "${output}" == *"secret_id_bound_cidrs"*"[]"* ]]
    [[ "${output}" == *"token_type"*"default"* ]]

    rerun_check
}
