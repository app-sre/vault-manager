#!/usr/bin/env bats

load ../helpers

@test "test vault-manager manage roles" {
    #
    # CASE: create roles
    #
    export GRAPHQL_QUERY_FILE=/tests/fixtures/roles/enable_vault_roles.graphql
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"[Vault Role] role is successfully written"*"path=auth/approle/role/app-interface"*"type=approle"* ]]
    [[ "${output}" == *"[Vault Role] role is successfully written"*"path=auth/approle/role/vault_manager"*"type=approle"* ]]
    [[ "${output}" == *"[Vault Role] role is successfully written"*"path=auth/oidc/role/reader"*"type=oidc"* ]]

    # check approles created
    run vault list auth/approle/role
    [ "$status" -eq 0 ]
    [[ "${output}" == *"app-interface"* ]]
    [[ "${output}" == *"vault_manager"* ]]
    
    # check oidc roles created
    run vault list auth/oidc/role
    [[ "${output}" == *"default"* ]]
    [[ "${output}" == *"minimal"* ]]

    # check approle config
    run vault read auth/approle/role/app-interface
    [ "$status" -eq 0 ]
    [[ "${output}" == *"token_num_uses"*"0"* ]]
    [[ "${output}" == *"token_ttl"*"30m"* ]]
    [[ "${output}" == *"token_max_ttl"*"30m"* ]]
    [[ "${output}" == *"policies"*"[app-interface-approle-policy]"* ]]
    [[ "${output}" == *"period"*"0s"* ]]
    [[ "${output}" == *"secret_id_ttl"*"0s"* ]]
    [[ "${output}" == *"secret_id_num_uses"*"0"* ]]
    [[ "${output}" == *"bind_secret_id"*"true"* ]]
    [[ "${output}" == *"local_secret_ids"*"false"* ]]
    [[ "${output}" == *"token_bound_cidrs"*"[]"* ]]
    [[ "${output}" == *"secret_id_bound_cidrs"*"[]"* ]]
    [[ "${output}" == *"token_type"*"default"* ]]
    # check approle config
    run vault read auth/approle/role/vault_manager
    [ "$status" -eq 0 ]
    [[ "${output}" == *"token_num_uses"*"0"* ]]
    [[ "${output}" == *"token_ttl"*"30m"* ]]
    [[ "${output}" == *"token_max_ttl"*"30m"* ]]
    [[ "${output}" == *"policies"*"[vault-manager-policy]"* ]]
    [[ "${output}" == *"period"*"0s"* ]]
    [[ "${output}" == *"secret_id_ttl"*"0s"* ]]
    [[ "${output}" == *"secret_id_num_uses"*"0"* ]]
    [[ "${output}" == *"bind_secret_id"*"true"* ]]
    [[ "${output}" == *"local_secret_ids"*"false"* ]]
    [[ "${output}" == *"token_bound_cidrs"*"[]"* ]]
    [[ "${output}" == *"secret_id_bound_cidrs"*"[]"* ]]
    [[ "${output}" == *"token_type"*"default"* ]]

    # check oidc role config with all attributes explicitly defined
    run vault read auth/oidc/role/default
    [ "$status" -eq 0 ]
    [[ "${output}" == *"allowed_redirect_uris"*"[http://localhost:8200/ui/vault/auth/oidc/oidc/callback]"* ]]
    [[ "${output}" == *"bound_audiences"*"[]"* ]]
    [[ "${output}" == *"bound_claims"*"map[foo:bar hello:world]"* ]]
    [[ "${output}" == *"bound_claims_type"*"string"* ]]
    [[ "${output}" == *"bound_subject"*"n/a"* ]]
    [[ "${output}" == *"claim_mappings"*"map[foo:bar]"* ]]
    [[ "${output}" == *"clock_skew_leeway"*"30"* ]]
    [[ "${output}" == *"expiration_leeway"*"20"* ]]
    [[ "${output}" == *"groups_claim"*"n/a"* ]]
    [[ "${output}" == *"not_before_leeway"*"10"* ]]
    [[ "${output}" == *"oidc_scopes"*"[]"* ]]
    [[ "${output}" == *"role_type"*"oidc"* ]]
    [[ "${output}" == *"token_bound_cidrs"*"[]"* ]]
    [[ "${output}" == *"token_explicit_max_ttl"*"0s"* ]]
    [[ "${output}" == *"token_max_ttl"*"30m"* ]]
    [[ "${output}" == *"token_no_default_policy"*"false"* ]]
    [[ "${output}" == *"token_num_uses"*"0"* ]]
    [[ "${output}" == *"token_period"*"0s"* ]]
    [[ "${output}" == *"token_policies"*"[]"* ]]
    [[ "${output}" == *"token_ttl"*"30m"* ]]
    [[ "${output}" == *"token_type"*"default"* ]]
    [[ "${output}" == *"user_claim"*"email"* ]]
    [[ "${output}" == *"verbose_oidc_logging"*"true"* ]]

    # check oidc role config with all optional attributes omitted from definition
    run vault read auth/oidc/role/minimal
    [ "$status" -eq 0 ]
    [[ "${output}" == *"allowed_redirect_uris"*"[http://localhost:8200/ui/vault/auth/oidc/oidc/callback]"* ]]
    [[ "${output}" == *"bound_audiences"*"[]"* ]]
    [[ "${output}" == *"bound_claims"*"<nil>"* ]]
    [[ "${output}" == *"bound_claims_type"*"string"* ]]
    [[ "${output}" == *"bound_subject"*"n/a"* ]]
    [[ "${output}" == *"claim_mappings"*"<nil>"* ]]
    [[ "${output}" == *"clock_skew_leeway"*"0"* ]]
    [[ "${output}" == *"expiration_leeway"*"0"* ]]
    [[ "${output}" == *"groups_claim"*"n/a"* ]]
    [[ "${output}" == *"not_before_leeway"*"0"* ]]
    [[ "${output}" == *"oidc_scopes"*"[]"* ]]
    [[ "${output}" == *"role_type"*"oidc"* ]]
    [[ "${output}" == *"token_bound_cidrs"*"[]"* ]]
    [[ "${output}" == *"token_explicit_max_ttl"*"0s"* ]]
    [[ "${output}" == *"token_max_ttl"*"1m"* ]]
    [[ "${output}" == *"token_no_default_policy"*"false"* ]]
    [[ "${output}" == *"token_num_uses"*"0"* ]]
    [[ "${output}" == *"token_period"*"0s"* ]]
    [[ "${output}" == *"token_policies"*"[]"* ]]
    [[ "${output}" == *"token_ttl"*"1m"* ]]
    [[ "${output}" == *"token_type"*"default"* ]]
    [[ "${output}" == *"user_claim"*"email"* ]]
    [[ "${output}" == *"verbose_oidc_logging"*"false"* ]]

    rerun_check
}