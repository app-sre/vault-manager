#!/usr/bin/env bats

load ../helpers

@test "test vault-manager manage auth backend userpass" {
    #
    # CASE: enable auth backend userpass
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/userpass/enable_auth_backends.yaml
    run vault-manager

    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully enabled auth backend"*"path=userpass-test-1/"*"type=userpass"* ]]
    [[ "${output}" == *"successfully enabled auth backend"*"path=userpass-test-2/"*"type=userpass"* ]]

    # check auth backends created
    run vault auth list
    [ "$status" -eq 0 ]
    [[ "${output}" == *"userpass-test-1/"* ]]
    [[ "${output}" == *"userpass-test-2/"* ]]

    rerun_check

    #
    # CASE: disable auth backend userpass
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/userpass/disable_auth_backends.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully disabled auth backend"*"path=userpass-test-2/"* ]]

    # check auth backends disabled
    run vault auth list
    [ "$status" -eq 0 ]
    [[ "${output}" != *"userpass-test-2/"* ]]

    # check auth backends still enabled
    [[ "${output}" == *"userpass-test-1/"* ]]
    [[ "${output}" == *"token/"* ]]

    rerun_check
}
