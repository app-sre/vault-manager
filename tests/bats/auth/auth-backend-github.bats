#!/usr/bin/env bats

load ../helpers

@test "test vault-manager manage auth backend github" {
    #
    # CASE: enable auth backend github
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/github/enable_auth_backends.yaml
    run vault-manager

    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully enabled auth backend"*"path=github-test-1/"*"type=github"* ]]
    [[ "${output}" == *"successfully enabled auth backend"*"path=github-test-2/"*"type=github"* ]]

    [[ "${output}" == *"auth mount successfully configured"*"path=auth/github-test-1/config"*"type=github"* ]]
    [[ "${output}" == *"auth mount successfully configured"*"path=auth/github-test-2/config"*"type=github"* ]]

    # check auth backends created
    run vault auth list
    [ "$status" -eq 0 ]
    [[ "${output}" == *"github-test-1/"*"github-test-1 auth backend"* ]]
    [[ "${output}" == *"github-test-2/"*"github-test-2 auth backend"* ]]

    # check github-test-1 auth configuration
    run vault read auth/github-test-1/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"organization"*"test-org-1"* ]]
    [[ "${output}" == *"ttl"*"72h"* ]]
    [[ "${output}" == *"max_ttl"*"72h"* ]]
    [[ "${output}" == *"base_url"*"base_url_test_1"* ]]

    # check github-test-2 auth configuration
    run vault read auth/github-test-2/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"organization"*"test-org-2"* ]]
    [[ "${output}" == *"ttl"*"72h"* ]]
    [[ "${output}" == *"max_ttl"*"72h"* ]]
    [[ "${output}" == *"base_url"*"base_url_test_2"* ]]

    rerun_check

    #
    # CASE: update configurations
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/github/update_auth_backends.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"auth mount successfully configured"*"path=auth/github-test-1/config"*"type=github"* ]]
    [[ "${output}" == *"auth mount successfully configured"*"path=auth/github-test-2/config"*"type=github"* ]]

    # check github-test-1 auth configuration
    run vault read auth/github-test-1/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"ttl"*"12h57m"* ]]
    [[ "${output}" == *"max_ttl"*"777h"* ]]
    [[ "${output}" == *"organization"*"test-org-1-updated"* ]]
    [[ "${output}" == *"base_url"*"base_url_test_1-updated"* ]]

    # check github-test-2 auth configuration
    run vault read auth/github-test-2/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"ttl"*"12h57m"* ]]
    [[ "${output}" == *"max_ttl"*"777h"* ]]
    [[ "${output}" == *"organization"*"test-org-2-updated"* ]]
    [[ "${output}" == *"base_url"*"base_url_test_2-updated"* ]]



    #
    # CASE: disable auth backend github
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/github/disable_auth_backends.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully disabled auth backend"*"path=github-test-2/"* ]]

    # check auth backends disabled
    run vault auth list
    [ "$status" -eq 0 ]
    [[ "${output}" != *"github-test-2/"* ]]

    # check auth backends still enabled
    [[ "${output}" == *"github-test-1/"* ]]
    [[ "${output}" == *"token/"* ]]

    rerun_check
}
