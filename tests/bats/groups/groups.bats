#!/usr/bin/env bats

load ../helpers

@test "test vault-manager manage groups" {
    #
    # CASE: create groups
    #
    export GRAPHQL_QUERY_FILE=/tests/fixtures/groups/enable_vault_groups.graphql
    run vault-manager -metrics=false
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"[Vault Identity] group successfully written"*"instance=\"http://127.0.0.1:8200\""*"path=identity/group/name/app-sre-vault-oidc"*"type=group"* ]]
    [[ "${output}" == *"[Vault Identity] group successfully written"*"instance=\"http://127.0.0.1:8202\""*"path=identity/group/name/app-sre-vault-oidc"*"type=group"* ]]

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

    # run same tests against secondary instance
    export VAULT_ADDR=http://127.0.0.1:8202

    # check groups created
    run vault list identity/group/name
    [ "$status" -eq 0 ]
    [[ "${output}" == *"app-sre-vault-oidc-secondary"* ]]

    # gather config values to test
    export VAULT_FORMAT="json"
    entity_id="$(vault read identity/entity/name/tester | jq -r '.["data"]."id"')"
    unset VAULT_FORMAT

    # check group config
    run vault read identity/group/name/app-sre-vault-oidc-secondary
    [ "$status" -eq 0 ]
    [[ "${output}" == *"name"*"app-sre-vault-oidc-secondary"* ]]
    [[ "${output}" == *"type"*"internal"* ]]
    [[ "${output}" == *"policies"*"[vault-oidc-app-sre-policy]"* ]]
    [[ "${output}" == *"member_entity_ids"*"[$entity_id]"* ]]
    [[ "${output}" == *"metadata"*"map[app-sre-vault-admin:app-sre vault administrator permission]"* ]]

    export VAULT_ADDR=http://127.0.0.1:8200
    rerun_check
}
