#!/usr/bin/env bats

load ../helpers

@test "test vault-manager manage auth backend cert" {
    #
    # CASE: enable auth backend cert
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/cert/enable_auth_backends.yaml
    run vault-manager

    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully enabled auth backend"*"path=cert-test-1/"*"type=cert"* ]]
    [[ "${output}" == *"successfully enabled auth backend"*"path=cert-test-2/"*"type=cert"* ]]

    # check auth backends created
    run vault auth list
    [ "$status" -eq 0 ]
    [[ "${output}" == *"cert-test-1/"* ]]
    [[ "${output}" == *"cert-test-2/"* ]]

    rerun_check

    #
    # CASE: disable auth backend cert
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/cert/disable_auth_backends.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully disabled auth backend"*"path=cert-test-2/"* ]]

    # check auth backends disabled
    run vault auth list
    [ "$status" -eq 0 ]
    [[ "${output}" != *"cert-test-2/"* ]]

    # check auth backends still enabled
    [[ "${output}" == *"cert-test-1/"* ]]
    [[ "${output}" == *"token/"* ]]

    rerun_check
}
