#!/usr/bin/env bats

load ../helpers

@test "test vault-manager manage groups" {
    GROUP_FIXTURES=/tests/fixtures/groups
    #
    # CASE: create groups
    #
    export GRAPHQL_QUERY_FILE=${GROUP_FIXTURES}/enable_vault_groups.graphql

    # test dry run output
    run vault-manager -dry-run
    [ "$status" -eq 0 ]

    [[ "${output}" == *"[Dry Run] [Vault Identity] 2 user(s) are in the group to be created"*"group=app-sre-vault-oidc"*"groupPolicies=\"[vault-oidc-app-sre-policy]\""*"instance=\"${PRIMARY_VAULT_URL}\""* ]]
    [[ "${output}" == *"[Dry Run] [Vault Identity] 2 user(s) are in the group to be created"*"group=app-interface-vault-oidc"*"groupPolicies=\"[vault-oidc-app-sre-policy]\""*"instance=\"${PRIMARY_VAULT_URL}\""* ]]
    [[ "${output}" == *"[Dry Run] [Vault Identity] 2 user(s) are in the group to be created"*"group=app-sre-vault-oidc-secondary"*"groupPolicies=\"[vault-oidc-app-sre-policy]\""*"instance=\"${SECONDARY_VAULT_URL}\""* ]]

    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"[Vault Identity] group successfully written"*"instance=\"${PRIMARY_VAULT_URL}\""*"path=identity/group/name/app-sre-vault-oidc"*"type=group"* ]]
    [[ "${output}" == *"[Vault Identity] group successfully written"*"instance=\"${SECONDARY_VAULT_URL}\""*"path=identity/group/name/app-sre-vault-oidc"*"type=group"* ]]

    # check groups created
    run vault list -address="${PRIMARY_VAULT_URL}" identity/group/name
    [ "$status" -eq 0 ]
    [[ "${output}" == *"app-sre-vault-oidc"* ]]

    # gather config values to test
    export VAULT_FORMAT="json"
    entity_id="$(vault read -address="${PRIMARY_VAULT_ADDR}" identity/entity/name/tester | jq -r '.["data"]."id"')"
    unset VAULT_FORMAT

    # check group config
    run vault read -address="${PRIMARY_VAULT_URL}" identity/group/name/app-sre-vault-oidc
    [ "$status" -eq 0 ]
    [[ "${output}" == *"name"*"app-sre-vault-oidc"* ]]
    [[ "${output}" == *"type"*"internal"* ]]
    [[ "${output}" == *"policies"*"[vault-oidc-app-sre-policy]"* ]]
    [[ "${output}" == *"member_entity_ids"*"$entity_id"* ]]
    [[ "${output}" == *"metadata"*"map[app-sre-vault-admin-primary:app-sre vault administrator permission]"* ]]

    # run same tests against secondary instance
    export VAULT_ADDR=${SECONDARY_VAULT_URL}

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
    [[ "${output}" == *"member_entity_ids"*"$entity_id"* ]]
    [[ "${output}" == *"metadata"*"map[app-sre-vault-admin:app-sre vault administrator permission]"* ]]

    # test that updating a policy will display the number of users affected by the change
    export VAULT_ADDR=${PRIMARY_VAULT_URL}
    export GRAPHQL_QUERY_FILE=${GROUP_FIXTURES}/vault_groups_and_policies.graphql
    run vault policy write vault-oidc-app-sre-policy ${GROUP_FIXTURES}/vault-oidc-app-sre-policy.hcl
    [ "$status" -eq 0 ]

    run vault-manager -dry-run
    [ "$status" -eq 0 ]

    [[ "${output}" == *"[Dry Run] [Vault Identity] 2 user(s) in group: 'app-sre-vault-oidc' will have policy: 'vault-oidc-app-sre-policy' updated"*"action=updated"*"group=app-sre-vault-oidc"*"instance=\"${PRIMARY_VAULT_URL}\""*"policy=vault-oidc-app-sre-policy"* ]]
    [[ "${output}" == *"[Dry Run] [Vault Identity] 2 user(s) in group: 'app-interface-vault-oidc' will have policy: 'vault-oidc-app-sre-policy' updated"*"action=updated"*"group=app-interface-vault-oidc"*"instance=\"${PRIMARY_VAULT_URL}\""*"policy=vault-oidc-app-sre-policy"* ]]

    # cleanup afterwards
    run vault-manager
    [ "$status" -eq 0 ]

    export GRAPHQL_QUERY_FILE=${GROUP_FIXTURES}/enable_vault_groups.graphql
    export VAULT_ADDR=${PRIMARY_VAULT_URL}

    rerun_check
}
