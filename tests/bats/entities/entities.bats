#!/usr/bin/env bats

load ../helpers

@test "test vault-manager manage entities and aliases" {
    #
    # CASE: create entities and aliases
    #
    export GRAPHQL_QUERY_FILE=/tests/fixtures/entities/enable_vault_entities_and_aliases.graphql

    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"[Vault Identity] entity successfully written"*"path=identity/entity/name/tester"*"type=entity"* ]]
    [[ "${output}" == *"[Vault Identity] entity alias successfully written"*"path=identity/entity-alias/tester"*"type=oidc"* ]]

    # check entities created
    run vault list identity/entity/name
    [ "$status" -eq 0 ]
    [[ "${output}" == *"tester"* ]]

    # check entity aliases created
    run vault list identity/entity-alias/id
    [ "$status" -eq 0 ]

    # gather config values to test
    export VAULT_FORMAT="json"
    entity_id="$(vault read identity/entity/name/tester | jq -r '.["data"]."id"')"
    alias_id="$(vault read identity/entity/name/tester | jq -r '.["data"]."aliases"[0]."id"')"
    accessor_id="$(vault auth list -detailed | jq -r '.["oidc/"]."accessor"')"
    unset VAULT_FORMAT

    # check entity config
    run vault read identity/entity/name/tester
    [ "$status" -eq 0 ]
    [[ "${output}" == *"name"*"tester"* ]]
    [[ "${output}" == *"id"*"$entity_id"* ]]
    [[ "${output}" == *"disabled"*"false"* ]]
    [[ "${output}" == *"metadata"*"map[name:The Tester]"* ]]

    # check entity alias config
    run vault read identity/entity-alias/id/$alias_id
    [ "$status" -eq 0 ]
    [[ "${output}" == *"name"*"tester"* ]]
    [[ "${output}" == *"id"*"$alias_id"* ]]
    [[ "${output}" == *"mount_type"*"oidc"* ]]
    [[ "${output}" == *"mount_accessor"*"$accessor_id"* ]]
    [[ "${output}" == *"canonical_id"*"$entity_id"* ]]

    rerun_check
}
