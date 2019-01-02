#!/usr/bin/env bats

load ../helpers

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
