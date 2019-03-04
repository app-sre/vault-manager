#!/usr/bin/env bats

load ../helpers

@test "test vault-manager manage policies" {
    #
    # CASE: create policies
    #
    export GRAPHQL_QUERY_FILE=/tests/fixtures/policies/add_policies.graphql
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully wrote policy to Vault instance"*"name=app-sre-policy"* ]]
    [[ "${output}" == *"successfully wrote policy to Vault instance"*"name=app-interface-approle-policy"* ]]

    # check policies created
    run vault policy list
    [[ "${output}" == *"app-sre-policy"* ]]
    [[ "${output}" == *"app-interface-approle-policy"* ]]

    # check policies content
    echo $(vault policy read app-sre-policy) | grep -F -q 'path "devtools-osio-ci/*" { capabilities = ["create", "read", "update", "delete", "list"] } path "app-sre/*" { capabilities = ["create", "read", "update", "delete", "list"] } path "app-interface/*" { capabilities = ["create", "read", "update", "delete", "list"] }'
    echo $(vault policy read app-interface-approle-policy) | grep -F -q 'path "app-sre/creds/*" { capabilities = ["read"] } #app-interface secrets integration path "app-interface/*" { capabilities = ["read"] }'

    rerun_check
}
