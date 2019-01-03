#!/usr/bin/env bats

load ../helpers

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
