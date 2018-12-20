#!/usr/bin/env bats

load helpers

setup() {
  export VAULT_ADDR=http://127.0.0.1:8200
  export VAULT_TOKEN=root
  export VAULT_AUTHTYPE=token

  docker run -d --name="vault-dev-server" --cap-add=IPC_LOCK -e 'VAULT_DEV_ROOT_TOKEN_ID=root' -p 8200:8200 vault:0.11.1
  until $(curl --output /dev/null --silent --head --fail http://127.0.0.1:8200); do
    printf '.'
    sleep 1
  done
}

teardown() {
  docker rm -f vault-dev-server
}

# rerun vault-manager to ensure that nothing happens on further runs
rerun_check() {
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == "" ]]
}

@test "test vault-manager manage audit devices" {
    #
    # CASE: enable two audit devices file1/ and file2/
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/audit/enable_several_audit_devices.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"audit successfully enabled"*"path=file1/"* ]]
    [[ "${output}" == *"audit successfully enabled"*"path=file2/"* ]]
    run vault audit list --detailed
    [ "$status" -eq 0 ]
    # check file1/ is enabled
    [[ "${output}" == *"file1/"* ]]
    [[ "${output}" == *"first_logger"* ]]
    [[ "${output}" == *"file_path=/tmp/log1.log"* ]]
    [[ "${output}" == *"format=json"* ]]
    [[ "${output}" == *"log_raw=false"* ]]
    [[ "${output}" == *"mode=0600"* ]]
    # check file2/ is enabled
    [[ "${output}" == *"file2/"* ]]
    [[ "${output}" == *"second_logger"* ]]
    [[ "${output}" == *"file_path=/tmp/log2.log"* ]]
    [[ "${output}" == *"format=jsonx"* ]]
    [[ "${output}" == *"log_raw=false"* ]]
    [[ "${output}" == *"mode=0660"* ]]

    rerun_check

    #
    # CASE: disable audit file1/ device
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/audit/disable_audit_device.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"audit successfully disabled"*"path=file1/"* ]]
    # check file1/ is removed
    run vault audit list
    [ "$status" -eq 0 ]
    [[ "${output}" != *"file1/"* ]]
    [[ "${output}" != *"first_logger"* ]]
    # check file2/ is still enabled
    [[ "${output}" == *"file2/"* ]]
    [[ "${output}" == *"second_logger"* ]]

    rerun_check
}

@test "test vault-manager manage policies" {
    #
    # CASE: create policies
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/policies/add_policies.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully wrote policy to Vault instance"*"name=policy-test-1"* ]]
    [[ "${output}" == *"successfully wrote policy to Vault instance"*"name=policy-test-2"* ]]
    [[ "${output}" == *"successfully wrote policy to Vault instance"*"name=policy-test-3"* ]]
    # check policies created
    run vault policy list
    [[ "${output}" == *"policy-test-1"* ]]
    [[ "${output}" == *"policy-test-2"* ]]
    [[ "${output}" == *"policy-test-3"* ]]
    # check policies content
    echo $(vault policy read policy-test-1) | grep -F -q 'path "secret-test1/*" { capabilities = ["create", "read", "update", "delete", "list"] }'
    echo $(vault policy read policy-test-2) | grep -F -q 'path "secret-test2/*" { capabilities = ["create", "read", "update", "delete", "list"] }'
    echo $(vault policy read policy-test-3) | grep -F -q 'path "secret-test3/*" { capabilities = ["create", "read", "update", "delete", "list"] } path "secret-test3/secret/" { capabilities = ["deny"] }'

    rerun_check

    #
    # CASE: update existing policies
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/policies/update_policies.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check policies updated
    echo $(vault policy read policy-test-1) | grep -F -q 'path "secret-test1/*" { capabilities = ["read", "list"] }'
    echo $(vault policy read policy-test-2) | grep -F -q 'path "secret-test2/*" { capabilities = ["list"] }'
    echo $(vault policy read policy-test-3) | grep -F -q 'path "secret-test3/*" { capabilities = ["create", "read", "list"] } path "secret-test3/secret/" { capabilities = ["deny"] }'

    rerun_check

    #
    # CASE: remove policies
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/policies/remove_policies.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully deleted policy from Vault instance"*"name=policy-test-2"* ]]
    [[ "${output}" == *"successfully deleted policy from Vault instance"*"name=policy-test-3"* ]]
    # check policies removed
    run vault policy list
    [ "$status" -eq 0 ]
    [[ "${output}" != *"policy-test-2"* ]]
    [[ "${output}" != *"policy-test-3"* ]]
    # check policy still exist
    [[ "${output}" == *"policy-test-1"* ]]
    [[ "${output}" == *"root"* ]]
    [[ "${output}" == *"default"* ]]

    rerun_check
}

@test "test vault-manager manage auth backends" {
    #
    # CASE: enable auth backends
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/enable_auth_backends.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully enabled auth backend"*"path=approle-1/"*"type=approle"* ]]
    [[ "${output}" == *"successfully enabled auth backend"*"path=approle-2/"*"type=approle"* ]]
    [[ "${output}" == *"successfully enabled auth backend"*"path=github-test-1/"*"type=github"* ]]
    [[ "${output}" == *"successfully enabled auth backend"*"path=github-test-2/"*"type=github"* ]]
    [[ "${output}" == *"github auth mount successfully configured"*"path=auth/github-test-1/config"* ]]
    [[ "${output}" == *"github auth mount successfully configured"*"path=auth/github-test-2/config"* ]]
    # check auth backends created
    run vault auth list
    [ "$status" -eq 0 ]
    [[ "${output}" == *"approle-1/"* ]]
    [[ "${output}" == *"approle-2/"* ]]
    [[ "${output}" == *"github-test-1/"* ]]
    [[ "${output}" == *"github-test-2/"* ]]
    [[ "${output}" == *"github-test-1 auth backend"* ]]
    [[ "${output}" == *"github-test-2 auth backend"* ]]
    # check github-test-1 auth configuration
    run vault read auth/github-test-1/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"organization"*"test-org-1"* ]]
    [[ "${output}" == *"ttl"*"72h"* ]]
    [[ "${output}" == *"max_ttl"*"72h"* ]]
    [[ "${output}" == *"base_url"*"n/a"* ]]

    rerun_check

    #
    # CASE: update github config
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/update_auth_backends.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"github auth mount successfully configured"*"path=auth/github-test-1/config"* ]]
    [[ "${output}" == *"github auth mount successfully configured"*"path=auth/github-test-2/config"* ]]
    # check github-test-1 auth configuration
    run vault read auth/github-test-1/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"ttl"*"12h57m"* ]]
    [[ "${output}" == *"max_ttl"*"777h"* ]]
    [[ "${output}" == *"organization"*"test-org-1-updated"* ]]
    [[ "${output}" == *"base_url"*"base_url_test_1"* ]]
    # check github-test-2 auth configuration
    run vault read auth/github-test-2/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"organization"*"test-org-2-updated"* ]]
    [[ "${output}" == *"ttl"*"12m57s"* ]]
    [[ "${output}" == *"max_ttl"*"12h30m26s"* ]]
    [[ "${output}" == *"base_url"*"base_url_test_2"* ]]

    rerun_check

    #
    # CASE: disable auth backends
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/disable_auth_backends.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully disabled auth backend"*"path=approle-1/"* ]]
    [[ "${output}" == *"successfully disabled auth backend"*"path=github-test-1/"* ]]
    # check auth backends disabled
    run vault auth list
    [ "$status" -eq 0 ]
    [[ "${output}" != *"approle-1/"* ]]
    [[ "${output}" != *"github-test-1/"* ]]
    # check auth backends still enabled
    [[ "${output}" == *"approle-2/"* ]]
    [[ "${output}" == *"github-test-2/"* ]]
    [[ "${output}" == *"token/"* ]]

    rerun_check
}

@test "test vault-manager manage approles" {
    #
    # CASE: create approles
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/approle/add_approles.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully wrote AppRole"*"path=auth/approle/role/test-role-1"* ]]
    [[ "${output}" == *"successfully wrote AppRole"*"path=auth/approle/role/test-role-2"* ]]
    # check approles created
    run vault list auth/approle/role
    [ "$status" -eq 0 ]
    [[ "${output}" == *"test-role-1"* ]]
    [[ "${output}" == *"test-role-2"* ]]
    # check approle config
    run vault read auth/approle/role/test-role-1
    [ "$status" -eq 0 ]
    [[ "${output}" == *"token_num_uses"*"1"* ]]
    [[ "${output}" == *"token_ttl"*"30m"* ]]
    [[ "${output}" == *"token_max_ttl"*"30m"* ]]
    [[ "${output}" == *"policies"*"[default role-1]"* ]]
    [[ "${output}" == *"period"*"0s"* ]]
    [[ "${output}" == *"secret_id_ttl"*"0s"* ]]
    [[ "${output}" == *"secret_id_num_uses"*"0"* ]]
    [[ "${output}" == *"bind_secret_id"*"true"* ]]
    [[ "${output}" == *"local_secret_ids"*"false"* ]]
    [[ "${output}" == *"token_bound_cidrs"*"[]"* ]]
    [[ "${output}" == *"bound_cidr_list"*"[]"* ]]
    [[ "${output}" == *"secret_id_bound_cidrs"*"[]"* ]]
    # check approle config
    run vault read auth/approle/role/test-role-2
    [ "$status" -eq 0 ]
    [[ "${output}" == *"token_num_uses"*"1"* ]]
    [[ "${output}" == *"token_ttl"*"5h"* ]]
    [[ "${output}" == *"token_max_ttl"*"5h"* ]]
    [[ "${output}" == *"policies"*"[default role-2]"* ]]
    [[ "${output}" == *"period"*"0s"* ]]
    [[ "${output}" == *"secret_id_ttl"*"10s"* ]]
    [[ "${output}" == *"secret_id_num_uses"*"0"* ]]
    [[ "${output}" == *"bind_secret_id"*"true"* ]]
    [[ "${output}" == *"local_secret_ids"*"false"* ]]
    [[ "${output}" == *"token_bound_cidrs"*"[]"* ]]
    [[ "${output}" == *"bound_cidr_list"*"[]"* ]]
    [[ "${output}" == *"secret_id_bound_cidrs"*"[]"* ]]

    rerun_check

    #
    # CASE: update approles
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/approle/update_approles.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully wrote AppRole"*"path=auth/approle/role/test-role-1"* ]]
    [[ "${output}" == *"successfully wrote AppRole"*"path=auth/approle/role/test-role-2"* ]]

    # check approle config updated
    run vault read auth/approle/role/test-role-1
    [ "$status" -eq 0 ]
    [[ "${output}" == *"token_num_uses"*"111"* ]]
    [[ "${output}" == *"token_ttl"*"3h30m30s"* ]]
    [[ "${output}" == *"token_max_ttl"*"333h"* ]]
    [[ "${output}" == *"policies"*"[default role-1 role-2]"* ]]
    [[ "${output}" == *"period"*"1h1m1s"* ]]
    [[ "${output}" == *"secret_id_ttl"*"11h11m11s"* ]]
    [[ "${output}" == *"secret_id_num_uses"*"10"* ]]
    [[ "${output}" == *"bind_secret_id"*"true"* ]]
    [[ "${output}" == *"local_secret_ids"*"false"* ]]
    [[ "${output}" == *"token_bound_cidrs"*"[]"* ]]
    [[ "${output}" == *"bound_cidr_list"*"[]"* ]]
    [[ "${output}" == *"secret_id_bound_cidrs"*"[]"* ]]

    # check approle config updated
    run vault read auth/approle/role/test-role-2
    [ "$status" -eq 0 ]
    [[ "${output}" == *"token_num_uses"*"1"* ]]
    [[ "${output}" == *"token_ttl"*"30m"* ]]
    [[ "${output}" == *"token_max_ttl"*"1h"* ]]
    [[ "${output}" == *"policies"*"[default role-2 role-3 role-4]"* ]]
    [[ "${output}" == *"period"*"2h2m2s"* ]]
    [[ "${output}" == *"secret_id_ttl"*"22h22m22s"* ]]
    [[ "${output}" == *"secret_id_num_uses"*"10"* ]]
    [[ "${output}" == *"bind_secret_id"*"true"* ]]
    [[ "${output}" == *"local_secret_ids"*"false"* ]]
    [[ "${output}" == *"token_bound_cidrs"*"[]"* ]]
    [[ "${output}" == *"bound_cidr_list"*"[]"* ]]
    [[ "${output}" == *"secret_id_bound_cidrs"*"[]"* ]]

    rerun_check

    #
    # CASE: remove approles
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/approle/remove_approles.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully deleted AppRole from Vault instance"*"path=auth/approle/role/test-role-1"* ]]
    # check approle removed
    run vault list auth/approle/role
    [ "$status" -eq 0 ]
    [[ "${output}" != *"test-role-1"* ]]
    # check role still exist
    [[ "${output}" == *"test-role-2"* ]]

    rerun_check
}

@test "test vault-manager manage sercrets engines" {
    #
    # CASE: enable secrets engines
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/secrets-engines/enable_secrets_engines.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully enabled mount"*"path=secrets-test-1/"* ]]
    [[ "${output}" == *"successfully enabled mount"*"path=secrets-test-2/"* ]]
    # check secrets engines enabled
    run vault secrets  list -detailed
    [ "$status" -eq 0 ]
    [[ "${output}" == *"secrets-test-1/"* ]]
    [[ "${output}" == *"this is first secrets engine"* ]]
    [[ "${output}" == *"secrets-test-2/"* ]]
    [[ "${output}" == *"map[version:2]"*"this is second secrets engine"* ]]

    rerun_check

    #
    # CASE: disable secrets engines
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/secrets-engines/disable_secrets_engines.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully disabled mount"*"path=secrets-test-1/"* ]]
    run vault secrets  list -detailed
    [ "$status" -eq 0 ]
    [[ "${output}" != *"secrets-test-1/"* ]]
    [[ "${output}" != *"this is first secrets engine"* ]]
    # check secrets engine still exist
    [[ "${output}" == *"secrets-test-2/"* ]]
    [[ "${output}" == *"sys/"* ]]
    [[ "${output}" == *"secret/"* ]]
    [[ "${output}" == *"identity/"* ]]
    [[ "${output}" == *"cubbyhole/"* ]]

    rerun_check
}

@test "test vault-manager gh policy mappings" {
    #
    # CASE: map policy to gh users / teams
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/gh-policy-mappings/add_gh_policy_mappings.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully applied Vault policy to Github entity"*"path=/auth/github-test-1/map/teams/test-team-1"* ]]
    [[ "${output}" == *"successfully applied Vault policy to Github entity"*"path=/auth/github-test-1/map/teams/test-team-2"* ]]
    [[ "${output}" == *"successfully applied Vault policy to Github entity"*"path=/auth/github-test-1/map/users/test-user-1"* ]]
    [[ "${output}" == *"successfully applied Vault policy to Github entity"*"path=/auth/github-test-1/map/users/test-user-2"* ]]
    [[ "${output}" == *"successfully applied Vault policy to Github entity"*"path=/auth/github-test-2/map/teams/test-team-3"* ]]
    [[ "${output}" == *"successfully applied Vault policy to Github entity"*"path=/auth/github-test-2/map/users/test-user-3"* ]]

    # check mappings applied
    check_vault_secret "list" "auth/github-test-1/map/teams" "test-team-1"
    check_vault_secret "list" "auth/github-test-1/map/teams" "test-team-2"
    check_vault_secret "list" "auth/github-test-1/map/users" "test-user-1"
    check_vault_secret "list" "auth/github-test-1/map/users" "test-user-2"
    check_vault_secret "list" "auth/github-test-2/map/teams" "test-team-3"
    check_vault_secret "list" "auth/github-test-2/map/users" "test-user-3"

    # check applied
    check_vault_secret "read" "auth/github-test-1/map/teams/test-team-1" "policy-team-1-1,policy-team-1-2"
    check_vault_secret "read" "auth/github-test-1/map/teams/test-team-2" "policy-team-2-1,policy-team-2-2"
    check_vault_secret "read" "auth/github-test-1/map/users/test-user-1" "policy-user-1-1,policy-user-1-2"
    check_vault_secret "read" "auth/github-test-1/map/users/test-user-2" "policy-user-2-1,policy-user-2-2"
    check_vault_secret "read" "auth/github-test-2/map/teams/test-team-3" "policy-team-3-1,policy-team-3-2"
    check_vault_secret "read" "auth/github-test-2/map/users/test-user-3" "policy-user-3-1,policy-user-3-2"

    rerun_check

    #
    # CASE: update gh entities policies
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/gh-policy-mappings/update_gh_policy_mappings.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully applied Vault policy to Github entity"*"path=/auth/github-test-1/map/teams/test-team-1"* ]]
    [[ "${output}" == *"successfully applied Vault policy to Github entity"*"path=/auth/github-test-1/map/users/test-user-1"* ]]
    [[ "${output}" == *"successfully applied Vault policy to Github entity"*"path=/auth/github-test-2/map/users/test-user-3"* ]]

    # check policies updated
    check_vault_secret "read" "auth/github-test-1/map/teams/test-team-1" "policy-team-1-updated"
    check_vault_secret "read" "auth/github-test-1/map/users/test-user-1" "policy-user-updated"
    check_vault_secret "read" "auth/github-test-2/map/users/test-user-3" "policy-user-3-updated"

    rerun_check

    #
    # CASE: remove gh entities from vault
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/gh-policy-mappings/remove_gh_policy_mappings.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully deleted GitHub entity from Vault instance"*"path=/auth/github-test-1/map/teams/test-team-1"* ]]
    [[ "${output}" == *"successfully deleted GitHub entity from Vault instance"*"path=/auth/github-test-1/map/users/test-user-2"* ]]
    [[ "${output}" == *"successfully deleted GitHub entity from Vault instance"*"path=/auth/github-test-2/map/teams/test-team-3"* ]]
    [[ "${output}" == *"successfully deleted GitHub entity from Vault instance"*"path=/auth/github-test-2/map/users/test-user-3"* ]]

    # check entities removed
    check_vault_secret_not_exist "list" "auth/github-test-1/map/teams" "test-team-1"
    check_vault_secret_not_exist "list" "auth/github-test-1/map/users" "test-team-2"

    # check entities still exist
    check_vault_secret "list" "auth/github-test-1/map/teams" "test-team-2"
    check_vault_secret "list" "auth/github-test-1/map/users" "test-user-1"

    rerun_check
}

@test "test vault-manager dry-run flag" {
    #
    # CASE: check dry-run flag
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/audit/enable_several_audit_devices.yaml
    run vault-manager -dry-run
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"[Dry Run]"*"package=audit"*"entry to be written="*"file1/"* ]]
    [[ "${output}" == *"[Dry Run]"*"package=audit"*"entry to be written="*"file2/"* ]]

    run vault audit list --detailed
    [ "$status" -eq 0 ]
    # check that no audit devices enabled
    [[ "${output}" != *"file1/"* ]]
    [[ "${output}" != *"file2/"* ]]

    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"audit successfully enabled"*"path=file1/"* ]]
    [[ "${output}" == *"audit successfully enabled"*"path=file2/"* ]]

    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/audit/disable_audit_device.yaml
    run vault-manager -dry-run
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"[Dry Run]"*"package=audit"*"entry to be deleted="*"file1/"* ]]

    run vault audit list --detailed
    [ "$status" -eq 0 ]
    # check that audit devices are still enabled
    [[ "${output}" == *"file1/"* ]]
    [[ "${output}" == *"file2/"* ]]
}

@test "test vault-manager force flag" {
    #
    # CASE: check force flag
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/force/force_prepare.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    # check vault-manager output
    [[ "${output}" == *"successfully wrote policy to Vault instance"*"name=policy-test-1"* ]]
    [[ "${output}" == *"successfully wrote policy to Vault instance"*"name=policy-test-2"* ]]
    [[ "${output}" == *"successfully wrote policy to Vault instance"*"name=policy-test-3"* ]]

    # check policies created
    run vault policy list
    [ "$status" -eq 0 ]
    [[ "${output}" == *"policy-test-1"* ]]
    [[ "${output}" == *"policy-test-2"* ]]
    [[ "${output}" == *"policy-test-3"* ]]

    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/force/test_force.yaml
    run vault-manager
    [ "$status" -eq 1 ]
    # check vault-manager output
    [[ "${output}" == *"top-level configuration key 'policies' does not have any entry, this will lead to removing all existing configurations of this top-level key in vault, use -force to force this operation"* ]]

    # check policies still exist
    run vault policy list
    [ "$status" -eq 0 ]
    [[ "${output}" == *"policy-test-1"* ]]
    [[ "${output}" == *"policy-test-2"* ]]
    [[ "${output}" == *"policy-test-3"* ]]

    run vault-manager -force
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"top-level configuration key 'policies' does not have any entry, this will lead to removing all existing configurations of this top-level key in vault"* ]]
    [[ "${output}" == *"successfully deleted policy from Vault instance"*"name=policy-test-1"* ]]
    [[ "${output}" == *"successfully deleted policy from Vault instance"*"name=policy-test-2"* ]]
    [[ "${output}" == *"successfully deleted policy from Vault instance"*"name=policy-test-3"* ]]

    # check all policies removed
    run vault policy list
    [ "$status" -eq 0 ]
    [[ "${output}" != *"policy-test-1"* ]]
    [[ "${output}" != *"policy-test-2"* ]]
    [[ "${output}" != *"policy-test-3"* ]]
}
