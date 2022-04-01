#!/usr/bin/env bats

load ../helpers

@test "test vault-manager manage auth backends and policy mappings" {
    #
    # CASE: enable auth backends and apply policies mappings
    #
    export GRAPHQL_QUERY_FILE=/tests/fixtures/auth/enable_auth_backends_with_policy_mappings.graphql
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"[Vault Auth] successfully enabled auth backend"*"path=approle/"*"type=approle"* ]]
    [[ "${output}" == *"[Vault Auth] successfully enabled auth backend"*"path=github/"*"type=github"* ]]
    [[ "${output}" == *"[Vault Auth] successfully enabled auth backend"*"path=oidc/"*"type=oidc"* ]]
    [[ "${output}" == *"[Vault Auth] auth backend successfully configured"*"path=auth/github/config"*"type=github"* ]]
    [[ "${output}" == *"[Vault Auth] auth backend successfully configured"*"path=auth/oidc/config"*"type=oidc"* ]]
    [[ "${output}" == *"[Vault Auth] policies mapping is successfully applied"*"path=/auth/github/map/teams/vault-app-sre"*"policies"*"app-sre-policy"* ]]

    # check auth backends created
    run vault auth list
    [ "$status" -eq 0 ]
    [[ "${output}" == *"approle/"* ]]
    [[ "${output}" == *"github/"* ]]
    [[ "${output}" == *"oidc/"* ]]

    # check github auth configuration
    run vault read auth/github/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"organization"*"app-sre"* ]]
    [[ "${output}" == *"ttl"*"12h"* ]]
    [[ "${output}" == *"max_ttl"*"24h"* ]]
    [[ "${output}" == *"base_url"*""* ]]

    # check oidc auth configuration
    run vault read auth/oidc/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"oidc_discovery_url"*"http://localhost:8180/auth/realms/test"* ]]
    [[ "${output}" == *"oidc_client_id"*"vault"* ]]
    [[ "${output}" == *"default_role"*"default"* ]]

    # check policy mappings
    run vault read auth/github/map/teams/vault-app-sre
    [ "$status" -eq 0 ]
    [[ "${output}" == *"key"*"vault-app-sre"* ]]
    [[ "${output}" == *"value"*"app-sre-policy"* ]]

    rerun_check
}
