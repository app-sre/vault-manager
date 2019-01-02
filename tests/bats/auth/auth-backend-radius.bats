#!/usr/bin/env bats

load ../helpers

@test "test vault-manager manage auth backend radius" {
    #
    # CASE: enable auth backend radius
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/radius/enable_auth_backends.yaml
    run vault-manager

    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully enabled auth backend"*"path=radius-test-1/"*"type=radius"* ]]
    [[ "${output}" == *"successfully enabled auth backend"*"path=radius-test-2/"*"type=radius"* ]]

    # check auth backends created
    run vault auth list
    [ "$status" -eq 0 ]
    [[ "${output}" == *"radius-test-1/"* ]]
    [[ "${output}" == *"radius-test-2/"* ]]

    rerun_check

    #
    # CASE: disable auth backend radius
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/radius/disable_auth_backends.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully disabled auth backend"*"path=radius-test-2/"* ]]

    # check auth backends disabled
    run vault auth list
    [ "$status" -eq 0 ]
    [[ "${output}" != *"radius-test-2/"* ]]

    # check auth backends still enabled
    [[ "${output}" == *"radius-test-1/"* ]]
    [[ "${output}" == *"token/"* ]]

    rerun_check
}
