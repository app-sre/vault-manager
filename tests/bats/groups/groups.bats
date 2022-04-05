#!/usr/bin/env bats

load ../helpers

@test "test vault-manager manage groups" {
    #
    # CASE: create groups
    #
    export GRAPHQL_QUERY_FILE=/tests/fixtures/groups/enable_vault_groups.graphql
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"[Vault Identity] group successfully written"*"path=identity/group/name/app-sre-vault-oidc"*"type=group"* ]]

    # check groups created
    run vault list identity/group/name
    [ "$status" -eq 0 ]
    [[ "${output}" == *"app-sre-vault-oidc"* ]]

    # gather config values to test
    export VAULT_FORMAT="json"
    entity_id="$(vault read identity/entity/name/tester | jq -r '.["data"]."id"')"
    unset VAULT_FORMAT

    # check group config
    run vault read identity/group/name/app-sre-vault-oidc
    [ "$status" -eq 0 ]
    [[ "${output}" == *"name"*"app-sre-vault-oidc"* ]]
    [[ "${output}" == *"type"*"internal"* ]]
    [[ "${output}" == *"policies"*"[vault-oidc-app-sre-policy]"* ]]
    [[ "${output}" == *"member_entity_ids"*"[$entity_id]"* ]]
    [[ "${output}" == *"metadata"*"map[app-sre-vault-admin:app-sre vault administrator permission]"* ]]


    rerun_check
}
