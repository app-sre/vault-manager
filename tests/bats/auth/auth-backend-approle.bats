#!/usr/bin/env bats

load ../helpers

@test "test vault-manager manage auth backend approle" {
    #
    # CASE: enable auth backend approle
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/approle/enable_auth_backends.yaml
    run vault-manager

    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully enabled auth backend"*"path=approle-test-1/"*"type=approle"* ]]
    [[ "${output}" == *"successfully enabled auth backend"*"path=approle-test-2/"*"type=approle"* ]]

    # check auth backends created
    run vault auth list
    [ "$status" -eq 0 ]
    [[ "${output}" == *"approle-test-1/"* ]]
    [[ "${output}" == *"approle-test-2/"* ]]

    rerun_check

    #
    # CASE: disable auth backend approle
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/approle/disable_auth_backends.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully disabled auth backend"*"path=approle-test-2/"* ]]

    # check auth backends disabled
    run vault auth list
    [ "$status" -eq 0 ]
    [[ "${output}" != *"approle-test-2/"* ]]

    # check auth backends still enabled
    [[ "${output}" == *"approle-test-1/"* ]]
    [[ "${output}" == *"token/"* ]]

    rerun_check
}
