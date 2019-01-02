#!/usr/bin/env bats

load ../helpers

@test "test vault-manager manage auth backend alicloud" {
    #
    # CASE: enable auth backend alicloud
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/alicloud/enable_auth_backends.yaml
    run vault-manager

    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully enabled auth backend"*"path=alicloud-test-1/"*"type=alicloud"* ]]
    [[ "${output}" == *"successfully enabled auth backend"*"path=alicloud-test-2/"*"type=alicloud"* ]]

    # check auth backends created
    run vault auth list
    [ "$status" -eq 0 ]
    [[ "${output}" == *"alicloud-test-1/"*"alicloud-test-1 auth backend"* ]]
    [[ "${output}" == *"alicloud-test-2/"*"alicloud-test-2 auth backend"* ]]

    rerun_check

    #
    # CASE: disable auth backend alicloud
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/alicloud/disable_auth_backends.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully disabled auth backend"*"path=alicloud-test-2/"* ]]

    # check auth backends disabled
    run vault auth list
    [ "$status" -eq 0 ]
    [[ "${output}" != *"alicloud-test-2/"* ]]

    # check auth backends still enabled
    [[ "${output}" == *"alicloud-test-1/"* ]]
    [[ "${output}" == *"token/"* ]]

    rerun_check
}
