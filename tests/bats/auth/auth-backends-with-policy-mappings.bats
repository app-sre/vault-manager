#!/usr/bin/env bats

load ../helpers

@test "test vault-manager manage auth backends and policy mappings" {
    #
    # CASE: enable auth backends and apply policies mappings
    #
    # export VAULT_ADDR="${PRIMARY_VAULT_URL}"
    export GRAPHQL_QUERY_FILE=/tests/fixtures/auth/enable_auth_backends_with_policy_mappings.graphql
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"[Vault Auth] successfully enabled auth backend"*"instance=\"${PRIMARY_VAULT_URL}\""*"path=approle/"*"type=approle"* ]]
    [[ "${output}" == *"[Vault Auth] successfully enabled auth backend"*"instance=\"${PRIMARY_VAULT_URL}\""*"path=github/"*"type=github"* ]]
    [[ "${output}" == *"[Vault Auth] successfully enabled auth backend"*"instance=\"${PRIMARY_VAULT_URL}\""*"path=oidc/"*"type=oidc"* ]]
    [[ "${output}" == *"[Vault Auth] successfully enabled auth backend"*"instance=\"${PRIMARY_VAULT_URL}\""*"path=kubernetes-main/"*"type=kubernetes"* ]]
    [[ "${output}" == *"[Vault Auth] auth backend successfully configured"*"instance=\"${PRIMARY_VAULT_URL}\""*"path=auth/github/config"*"type=github"* ]]
    [[ "${output}" == *"[Vault Auth] auth backend successfully configured"*"instance=\"${PRIMARY_VAULT_URL}\""*"path=auth/oidc/config"*"type=oidc"* ]]
    [[ "${output}" == *"[Vault Auth] auth backend successfully configured"*"instance=\"${PRIMARY_VAULT_URL}\""*"path=auth/kubernetes-main/config"*"type=kubernetes"* ]]
    [[ "${output}" == *"[Vault Auth] policies mapping is successfully applied"*"instance=\"${PRIMARY_VAULT_URL}\""*"path=/auth/github/map/teams/vault-app-sre"*"policies"*"app-sre-policy"* ]]
    [[ "${output}" == *"[Vault Auth] successfully enabled auth backend"*"instance=\"${SECONDARY_VAULT_URL}\""*"path=approle/"*"type=approle"* ]]
    [[ "${output}" == *"[Vault Auth] successfully enabled auth backend"*"instance=\"${SECONDARY_VAULT_URL}\""*"path=github/"*"type=github"* ]]
    [[ "${output}" == *"[Vault Auth] successfully enabled auth backend"*"instance=\"${SECONDARY_VAULT_URL}\""*"path=oidc/"*"type=oidc"* ]]
    [[ "${output}" == *"[Vault Auth] successfully enabled auth backend"*"instance=\"${SECONDARY_VAULT_URL}\""*"path=kubernetes-secondary/"*"type=kubernetes"* ]]
    [[ "${output}" == *"[Vault Auth] auth backend successfully configured"*"instance=\"${SECONDARY_VAULT_URL}\""*"path=auth/github/config"*"type=github"* ]]
    [[ "${output}" == *"[Vault Auth] auth backend successfully configured"*"instance=\"${SECONDARY_VAULT_URL}\""*"path=auth/oidc/config"*"type=oidc"* ]]
    [[ "${output}" == *"[Vault Auth] auth backend successfully configured"*"instance=\"${SECONDARY_VAULT_URL}\""*"path=auth/kubernetes-secondary/config"*"type=kubernetes"* ]]
    [[ "${output}" == *"[Vault Auth] policies mapping is successfully applied"*"instance=\"${SECONDARY_VAULT_URL}\""*"path=/auth/github/map/teams/vault-app-sre"*"policies"*"app-sre-policy"* ]]

    export VAULT_ADDR=${PRIMARY_VAULT_URL}
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
    [[ "${output}" == *"oidc_discovery_url"*"http://keycloak:8180/auth/realms/test"* ]]
    [[ "${output}" == *"oidc_client_id"*"vault"* ]]
    [[ "${output}" == *"default_role"*"default"* ]]

    # check policy mappings
    run vault read auth/github/map/teams/vault-app-sre
    [ "$status" -eq 0 ]
    [[ "${output}" == *"key"*"vault-app-sre"* ]]
    [[ "${output}" == *"value"*"app-sre-policy"* ]]

    # run same tests against secondary instance
    export VAULT_ADDR=${SECONDARY_VAULT_URL}

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
    [[ "${output}" == *"oidc_discovery_url"*"${KEYCLOAK_URL}/auth/realms/test"* ]]
    [[ "${output}" == *"oidc_client_id"*"vault"* ]]
    [[ "${output}" == *"default_role"*"default"* ]]

    # check policy mappings
    run vault read auth/github/map/teams/vault-app-sre
    [ "$status" -eq 0 ]
    [[ "${output}" == *"key"*"vault-app-sre"* ]]
    [[ "${output}" == *"value"*"app-sre-policy"* ]]

    export VAULT_ADDR=${PRIMARY_VAULT_URL}
    rerun_check
}
