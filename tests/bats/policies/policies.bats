#!/usr/bin/env bats

load ../helpers

@test "test vault-manager manage policies" {
    #
    # CASE: create policies
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/policies/add_policies.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully wrote policy to Vault instance"*"name=policy-test-1"* ]]
    [[ "${output}" == *"successfully wrote policy to Vault instance"*"name=policy-test-2"* ]]
    [[ "${output}" == *"successfully wrote policy to Vault instance"*"name=policy-test-3"* ]]
    # check policies created
    run vault policy list
    [[ "${output}" == *"policy-test-1"* ]]
    [[ "${output}" == *"policy-test-2"* ]]
    [[ "${output}" == *"policy-test-3"* ]]
    # check policies content
    echo $(vault policy read policy-test-1) | grep -F -q 'path "secret-test1/*" { capabilities = ["create", "read", "update", "delete", "list"] }'
    echo $(vault policy read policy-test-2) | grep -F -q 'path "secret-test2/*" { capabilities = ["create", "read", "update", "delete", "list"] }'
    echo $(vault policy read policy-test-3) | grep -F -q 'path "secret-test3/*" { capabilities = ["create", "read", "update", "delete", "list"] } path "secret-test3/secret/" { capabilities = ["deny"] }'

    rerun_check

    #
    # CASE: update existing policies
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/policies/update_policies.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check policies updated
    echo $(vault policy read policy-test-1) | grep -F -q 'path "secret-test1/*" { capabilities = ["read", "list"] }'
    echo $(vault policy read policy-test-2) | grep -F -q 'path "secret-test2/*" { capabilities = ["list"] }'
    echo $(vault policy read policy-test-3) | grep -F -q 'path "secret-test3/*" { capabilities = ["create", "read", "list"] } path "secret-test3/secret/" { capabilities = ["deny"] }'

    rerun_check

    #
    # CASE: remove policies
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/policies/remove_policies.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully deleted policy from Vault instance"*"name=policy-test-2"* ]]
    [[ "${output}" == *"successfully deleted policy from Vault instance"*"name=policy-test-3"* ]]
    # check policies removed
    run vault policy list
    [ "$status" -eq 0 ]
    [[ "${output}" != *"policy-test-2"* ]]
    [[ "${output}" != *"policy-test-3"* ]]
    # check policy still exist
    [[ "${output}" == *"policy-test-1"* ]]
    [[ "${output}" == *"root"* ]]
    [[ "${output}" == *"default"* ]]

    rerun_check
}
