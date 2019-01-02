#!/usr/bin/env bats

load ../helpers

@test "test vault-manager manage auth backend aws" {
    #
    # CASE: enable auth backend aws
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/aws/enable_auth_backends.yaml
    run vault-manager

    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully enabled auth backend"*"path=aws-test-1/"*"type=aws"* ]]
    [[ "${output}" == *"successfully enabled auth backend"*"path=aws-test-2/"*"type=aws"* ]]

    [[ "${output}" == *"auth mount successfully configured"*"path=auth/aws-test-1/config/client"*"type=aws"* ]]
    [[ "${output}" == *"auth mount successfully configured"*"path=auth/aws-test-2/config/client"*"type=aws"* ]]

    # check auth backends created
    run vault auth list
    [ "$status" -eq 0 ]
    [[ "${output}" == *"aws-test-1/"*"aws-test-1 auth backend"* ]]
    [[ "${output}" == *"aws-test-2/"*"aws-test-2 auth backend"* ]]

    # check aws-test-1 auth configuration
    run vault read auth/aws-test-1/config/client
    [ "$status" -eq 0 ]
    [[ "${output}" == *"access_key"*"access_key"* ]]
    [[ "${output}" == *"max_retries"*"-1"* ]]
    [[ "${output}" == *"endpoint"* ]]
    [[ "${output}" == *"iam_endpoint"* ]]
    [[ "${output}" == *"iam_server_id_header_value"* ]]
    [[ "${output}" == *"sts_endpoint"* ]]

    # check aws-test-2 auth configuration
    run vault read auth/aws-test-2/config/client
    [ "$status" -eq 0 ]
    [[ "${output}" == *"access_key"*"access_key"* ]]
    [[ "${output}" == *"max_retries"*"-1"* ]]
    [[ "${output}" == *"endpoint"* ]]
    [[ "${output}" == *"iam_endpoint"* ]]
    [[ "${output}" == *"iam_server_id_header_value"* ]]
    [[ "${output}" == *"sts_endpoint"* ]]

    rerun_check

    #
    # CASE: update configurations
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/aws/update_auth_backends.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"auth mount successfully configured"*"path=auth/aws-test-1/config/client"*"type=aws"* ]]
    [[ "${output}" == *"auth mount successfully configured"*"path=auth/aws-test-2/config/client"*"type=aws"* ]]

    # check aws-test-1 auth configuration
    run vault read auth/aws-test-1/config/client
    [ "$status" -eq 0 ]
    [[ "${output}" == *"access_key"*"access_key-updated"* ]]
    [[ "${output}" == *"max_retries"*"-1"* ]]
    [[ "${output}" == *"endpoint"* ]]

    # check aws-test-2 auth configuration
    run vault read auth/aws-test-2/config/client
    [ "$status" -eq 0 ]
    [[ "${output}" == *"access_key"*"access_key-updated"* ]]
    [[ "${output}" == *"max_retries"*"-1"* ]]
    [[ "${output}" == *"endpoint"* ]]

    #
    # CASE: disable auth backend aws
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/aws/disable_auth_backends.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully disabled auth backend"*"path=aws-test-2/"* ]]

    # check auth backends disabled
    run vault auth list
    [ "$status" -eq 0 ]
    [[ "${output}" != *"aws-test-2/"* ]]

    # check auth backends still enabled
    [[ "${output}" == *"aws-test-1/"* ]]
    [[ "${output}" == *"token/"* ]]

    rerun_check
}
