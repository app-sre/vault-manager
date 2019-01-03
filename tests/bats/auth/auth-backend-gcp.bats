#!/usr/bin/env bats

load ../helpers

@test "test vault-manager manage auth backend gcp" {
    #
    # CASE: enable auth backend gcp
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/gcp/enable_auth_backends.yaml
    run vault-manager

    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully enabled auth backend"*"path=gcp-test-1/"*"type=gcp"* ]]
    [[ "${output}" == *"successfully enabled auth backend"*"path=gcp-test-2/"*"type=gcp"* ]]

    # check auth backends created
    run vault auth list
    [ "$status" -eq 0 ]
    [[ "${output}" == *"gcp-test-1/"* ]]
    [[ "${output}" == *"gcp-test-2/"* ]]

    rerun_check

    #
    # CASE: disable auth backend gcp
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/gcp/disable_auth_backends.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully disabled auth backend"*"path=gcp-test-2/"* ]]

    # check auth backends disabled
    run vault auth list
    [ "$status" -eq 0 ]
    [[ "${output}" != *"gcp-test-2/"* ]]

    # check auth backends still enabled
    [[ "${output}" == *"gcp-test-1/"* ]]
    [[ "${output}" == *"token/"* ]]

    rerun_check
}
