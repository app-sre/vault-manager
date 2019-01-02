#!/usr/bin/env bats

load ../helpers

@test "test vault-manager manage auth backend azure" {
    #
    # CASE: enable auth backend azure
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/azure/enable_auth_backends.yaml
    run vault-manager

    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully enabled auth backend"*"path=azure-test-1/"*"type=azure"* ]]
    [[ "${output}" == *"successfully enabled auth backend"*"path=azure-test-2/"*"type=azure"* ]]

    [[ "${output}" == *"auth mount successfully configured"*"path=auth/azure-test-1/config"*"type=azure"* ]]
    [[ "${output}" == *"auth mount successfully configured"*"path=auth/azure-test-2/config"*"type=azure"* ]]

    # check auth backends created
    run vault auth list
    [ "$status" -eq 0 ]
    [[ "${output}" == *"azure-test-1/"*"azure-test-1 auth backend"* ]]
    [[ "${output}" == *"azure-test-2/"*"azure-test-2 auth backend"* ]]

    # check azure-test-1 auth configuration
    run vault read auth/azure-test-1/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"tenant_id"*"azure-test-tenant-id"* ]]
    [[ "${output}" == *"resource"*"https://vault.hashicorp.com"* ]]
    [[ "${output}" == *"client_id"*"azure-test-client-id"* ]]
    [[ "${output}" == *"environment"* ]]

    # check azure-test-2 auth configuration
    run vault read auth/azure-test-2/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"tenant_id"*"azure-test-tenant-id"* ]]
    [[ "${output}" == *"resource"*"https://vault.hashicorp.com"* ]]
    [[ "${output}" == *"client_id"*"azure-test-client-id"* ]]
    [[ "${output}" == *"environment"* ]]

    rerun_check

    #
    # CASE: update configurations
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/azure/update_auth_backends.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"auth mount successfully configured"*"path=auth/azure-test-1/config"*"type=azure"* ]]
    [[ "${output}" == *"auth mount successfully configured"*"path=auth/azure-test-2/config"*"type=azure"* ]]

    # check configurations
    # check azure-test-1 auth configuration
    run vault read auth/azure-test-1/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"tenant_id"*"azure-test-tenant-id-updated"* ]]
    [[ "${output}" == *"resource"*"https://vault.hashicorp-updated.com"* ]]
    [[ "${output}" == *"client_id"*"azure-test-client-id-updated"* ]]

    # check configurations
    # check azure-test-2 auth configuration
    run vault read auth/azure-test-2/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"tenant_id"*"azure-test-tenant-id-updated"* ]]
    [[ "${output}" == *"resource"*"https://vault.hashicorp-updated.com"* ]]
    [[ "${output}" == *"client_id"*"azure-test-client-id-updated"* ]]


    #
    # CASE: disable auth backend azure
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/azure/disable_auth_backends.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully disabled auth backend"*"path=azure-test-2/"* ]]

    # check auth backends disabled
    run vault auth list
    [ "$status" -eq 0 ]
    [[ "${output}" != *"azure-test-2/"* ]]

    # check auth backends still enabled
    [[ "${output}" == *"azure-test-1/"* ]]
    [[ "${output}" == *"token/"* ]]

    rerun_check
}
