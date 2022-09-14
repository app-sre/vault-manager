#!/usr/bin/env bats

load ../helpers

@test "test vault-manager manage auth backends and policy mappings" {
    #
    # CASE: enable auth backends and apply policies mappings
    #
    export GRAPHQL_QUERY_FILE=/tests/fixtures/auth/enable_auth_backends_with_policy_mappings.graphql
    run vault-manager -metrics=false
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"[Vault Auth] successfully enabled auth backend"*"instance=\"http://127.0.0.1:8200\""*"path=approle/"*"type=approle"* ]]
    [[ "${output}" == *"[Vault Auth] successfully enabled auth backend"*"instance=\"http://127.0.0.1:8200\""*"path=github/"*"type=github"* ]]
    [[ "${output}" == *"[Vault Auth] successfully enabled auth backend"*"instance=\"http://127.0.0.1:8200\""*"path=oidc/"*"type=oidc"* ]]
    [[ "${output}" == *"[Vault Auth] auth backend successfully configured"*"instance=\"http://127.0.0.1:8200\""*"path=auth/github/config"*"type=github"* ]]
    [[ "${output}" == *"[Vault Auth] auth backend successfully configured"*"instance=\"http://127.0.0.1:8200\""*"path=auth/oidc/config"*"type=oidc"* ]]
    [[ "${output}" == *"[Vault Auth] policies mapping is successfully applied"*"instance=\"http://127.0.0.1:8200\""*"path=/auth/github/map/teams/vault-app-sre"*"policies"*"app-sre-policy"* ]]
    [[ "${output}" == *"[Vault Auth] successfully enabled auth backend"*"instance=\"http://127.0.0.1:8202\""*"path=approle/"*"type=approle"* ]]
    [[ "${output}" == *"[Vault Auth] successfully enabled auth backend"*"instance=\"http://127.0.0.1:8202\""*"path=github/"*"type=github"* ]]
    [[ "${output}" == *"[Vault Auth] successfully enabled auth backend"*"instance=\"http://127.0.0.1:8202\""*"path=oidc/"*"type=oidc"* ]]
    [[ "${output}" == *"[Vault Auth] auth backend successfully configured"*"instance=\"http://127.0.0.1:8202\""*"path=auth/github/config"*"type=github"* ]]
    [[ "${output}" == *"[Vault Auth] auth backend successfully configured"*"instance=\"http://127.0.0.1:8202\""*"path=auth/oidc/config"*"type=oidc"* ]]
    [[ "${output}" == *"[Vault Auth] policies mapping is successfully applied"*"instance=\"http://127.0.0.1:8202\""*"path=/auth/github/map/teams/vault-app-sre"*"policies"*"app-sre-policy"* ]]

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

    # run same tests against secondary instance
    export VAULT_ADDR=http://127.0.0.1:8202
    
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

    export VAULT_ADDR=http://127.0.0.1:8200
    rerun_check
}
