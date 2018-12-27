#!/usr/bin/env bats

load helpers

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
