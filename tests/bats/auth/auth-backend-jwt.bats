#!/usr/bin/env bats

load ../helpers

@test "test vault-manager manage auth backend jwt" {
    #
    # CASE: enable auth backend jwt
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/jwt/enable_auth_backends.yaml
    run vault-manager

    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully enabled auth backend"*"path=jwt-test-1/"*"type=jwt"* ]]
    [[ "${output}" == *"successfully enabled auth backend"*"path=jwt-test-2/"*"type=jwt"* ]]

    [[ "${output}" == *"auth mount successfully configured"*"path=auth/jwt-test-1/config"*"type=jwt"* ]]
    [[ "${output}" == *"auth mount successfully configured"*"path=auth/jwt-test-2/config"*"type=jwt"* ]]

    # check auth backends created
    run vault auth list
    [ "$status" -eq 0 ]
    [[ "${output}" == *"jwt-test-1/"*"jwt-test-1 auth backend"* ]]
    [[ "${output}" == *"jwt-test-2/"*"jwt-test-2 auth backend"* ]]

    # check jwt-test-1 auth configuration
    run vault read auth/jwt-test-1/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"oidc_discovery_url"*"https://myco.auth0.com/"* ]]
    [[ "${output}" == *"oidc_discovery_ca_pem"* ]]
    [[ "${output}" == *"jwt_validation_pubkeys"*"[]"* ]]
    [[ "${output}" == *"bound_issuer"* ]]

    # check jwt-test-2 auth configuration
    run vault read auth/jwt-test-2/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"oidc_discovery_url"*"https://myco.auth0.com/"* ]]
    [[ "${output}" == *"oidc_discovery_ca_pem"* ]]
    [[ "${output}" == *"jwt_validation_pubkeys"*"[]"* ]]
    [[ "${output}" == *"bound_issuer"* ]]

    rerun_check

    #
    # CASE: update configurations
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/jwt/update_auth_backends.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"auth mount successfully configured"*"path=auth/jwt-test-1/config"*"type=jwt"* ]]
    [[ "${output}" == *"auth mount successfully configured"*"path=auth/jwt-test-2/config"*"type=jwt"* ]]

    # check jwt-test-1 auth configuration
    run vault read auth/jwt-test-1/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"bound_issuer"*"updated"* ]]

    # check jwt-test-2 auth configuration
    run vault read auth/jwt-test-2/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"bound_issuer"*"updated"* ]]



    #
    # CASE: disable auth backend jwt
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/jwt/disable_auth_backends.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully disabled auth backend"*"path=jwt-test-2/"* ]]

    # check auth backends disabled
    run vault auth list
    [ "$status" -eq 0 ]
    [[ "${output}" != *"jwt-test-2/"* ]]

    # check auth backends still enabled
    [[ "${output}" == *"jwt-test-1/"* ]]
    [[ "${output}" == *"token/"* ]]

    rerun_check
}
