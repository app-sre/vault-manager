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
    [[ "${output}" == *"successfully enabled auth backend"*"package=auth"*"path=approle/"*"type=approle"* ]]
    [[ "${output}" == *"successfully enabled auth backend"*"package=auth"*"path=github/"*"type=github"* ]]
    [[ "${output}" == *"auth backend successfully configured"*"package=auth"*"path=auth/github/config"*"type=github"* ]]
    [[ "${output}" == *"policies mapping is successfully applied"*"package=auth"*"path=/auth/github/map/teams/vault-app-sre"*"policies=app-sre-policy"* ]]

    # check auth backends created
    run vault auth list
    [ "$status" -eq 0 ]
    [[ "${output}" == *"approle/"* ]]
    [[ "${output}" == *"github/"* ]]

    # check github auth configuration
    run vault read auth/github/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"organization"*"app-sre"* ]]
    [[ "${output}" == *"ttl"*"120h"* ]]
    [[ "${output}" == *"max_ttl"*"360h"* ]]
    [[ "${output}" == *"base_url"*""* ]]

    # check policy mappings
    run vault read auth/github/map/teams/vault-app-sre
    [ "$status" -eq 0 ]
    [[ "${output}" == *"key"*"vault-app-sre"* ]]
    [[ "${output}" == *"value"*"app-sre-policy"* ]]

    rerun_check
}
