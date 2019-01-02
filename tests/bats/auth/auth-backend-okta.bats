#!/usr/bin/env bats

load ../helpers

@test "test vault-manager manage auth backend okta" {
    #
    # CASE: enable auth backend okta
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/okta/enable_auth_backends.yaml
    run vault-manager

    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully enabled auth backend"*"path=okta-test-1/"*"type=okta"* ]]
    [[ "${output}" == *"successfully enabled auth backend"*"path=okta-test-2/"*"type=okta"* ]]

    [[ "${output}" == *"auth mount successfully configured"*"path=auth/okta-test-1/config"*"type=okta"* ]]
    [[ "${output}" == *"auth mount successfully configured"*"path=auth/okta-test-2/config"*"type=okta"* ]]

    # check auth backends created
    run vault auth list
    [ "$status" -eq 0 ]
    [[ "${output}" == *"okta-test-1/"*"okta-test-1 auth backend"* ]]
    [[ "${output}" == *"okta-test-2/"*"okta-test-2 auth backend"* ]]

    # check okta-test-1 auth configuration
    run vault read auth/okta-test-1/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"base_url"*"okta.com"* ]]
    [[ "${output}" == *"organization"*"test-okta-org"* ]]
    [[ "${output}" == *"org_name"*"test-okta-org"* ]]
    [[ "${output}" == *"bypass_okta_mfa"*"false"* ]]
    [[ "${output}" == *"max_ttl"*"0s"* ]]
    [[ "${output}" == *"ttl"*"0s"* ]]

    # check okta-test-2 auth configuration
    run vault read auth/okta-test-2/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"base_url"*"okta.com"* ]]
    [[ "${output}" == *"organization"*"test-okta-org"* ]]
    [[ "${output}" == *"org_name"*"test-okta-org"* ]]
    [[ "${output}" == *"bypass_okta_mfa"*"false"* ]]
    [[ "${output}" == *"max_ttl"*"0s"* ]]
    [[ "${output}" == *"ttl"*"0s"* ]]


    rerun_check

    #
    # CASE: update configurations
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/okta/update_auth_backends.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"auth mount successfully configured"*"path=auth/okta-test-1/config"*"type=okta"* ]]
    [[ "${output}" == *"auth mount successfully configured"*"path=auth/okta-test-2/config"*"type=okta"* ]]

    # check okta-test-1 auth configuration
    run vault read auth/okta-test-1/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"base_url"*"okta-updated.com"* ]]
    [[ "${output}" == *"organization"*"test-okta-org-updated"* ]]
    [[ "${output}" == *"org_name"*"test-okta-org-updated"* ]]
    [[ "${output}" == *"max_ttl"*"777h"* ]]
    [[ "${output}" == *"ttl"*"12h"* ]]

    # check okta-test-2 auth configuration
    run vault read auth/okta-test-2/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"base_url"*"okta-updated.com"* ]]
    [[ "${output}" == *"organization"*"test-okta-org-updated"* ]]
    [[ "${output}" == *"org_name"*"test-okta-org-updated"* ]]
    [[ "${output}" == *"max_ttl"*"777h"* ]]
    [[ "${output}" == *"ttl"*"12h"* ]]

    #
    # CASE: disable auth backend okta
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/okta/disable_auth_backends.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully disabled auth backend"*"path=okta-test-2/"* ]]

    # check auth backends disabled
    run vault auth list
    [ "$status" -eq 0 ]
    [[ "${output}" != *"okta-test-2/"* ]]

    # check auth backends still enabled
    [[ "${output}" == *"okta-test-1/"* ]]
    [[ "${output}" == *"token/"* ]]

    rerun_check
}
