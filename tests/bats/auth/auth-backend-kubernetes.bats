#!/usr/bin/env bats

load ../helpers

@test "test vault-manager manage auth backend kubernetes" {
    #
    # CASE: enable auth backend kubernetes
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/kubernetes/enable_auth_backends.yaml
    run vault-manager

    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully enabled auth backend"*"path=kubernetes-test-1/"*"type=kubernetes"* ]]
    [[ "${output}" == *"successfully enabled auth backend"*"path=kubernetes-test-2/"*"type=kubernetes"* ]]

    [[ "${output}" == *"auth mount successfully configured"*"path=auth/kubernetes-test-1/config"*"type=kubernetes"* ]]
    [[ "${output}" == *"auth mount successfully configured"*"path=auth/kubernetes-test-2/config"*"type=kubernetes"* ]]

    # check auth backends created
    run vault auth list
    [ "$status" -eq 0 ]
    [[ "${output}" == *"kubernetes-test-1/"*"kubernetes-test-1 auth backend"* ]]
    [[ "${output}" == *"kubernetes-test-2/"*"kubernetes-test-2 auth backend"* ]]

    # check kubernetes-test-1 auth configuration
    run vault read auth/kubernetes-test-1/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"kubernetes_host"*"https://192.168.99.100:8443"* ]]
    [[ "${output}" == *"pem_keys"*"[]"* ]]
    [[ "${output}" == *"kubernetes_ca_cert"*"dummy-cert"* ]]

    # check kubernetes-test-2 auth configuration
    run vault read auth/kubernetes-test-2/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"kubernetes_host"*"https://192.168.99.100:8443"* ]]
    [[ "${output}" == *"pem_keys"*"[]"* ]]
    [[ "${output}" == *"kubernetes_ca_cert"*"dummy-cert"* ]]

    rerun_check

    #
    # CASE: update configurations
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/kubernetes/update_auth_backends.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"auth mount successfully configured"*"path=auth/kubernetes-test-1/config"*"type=kubernetes"* ]]
    [[ "${output}" == *"auth mount successfully configured"*"path=auth/kubernetes-test-2/config"*"type=kubernetes"* ]]

    # check kubernetes-test-1 auth configuration
    run vault read auth/kubernetes-test-1/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"kubernetes_host"*"https://192.168.99.100-updated:8443"* ]]
    [[ "${output}" == *"kubernetes_ca_cert"*"dummy-cert-updated"* ]]

    # check kubernetes-test-2 auth configuration
    run vault read auth/kubernetes-test-2/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"kubernetes_host"*"https://192.168.99.100-updated:8443"* ]]
    [[ "${output}" == *"kubernetes_ca_cert"*"dummy-cert-updated"* ]]


    #
    # CASE: disable auth backend kubernetes
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/kubernetes/disable_auth_backends.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully disabled auth backend"*"path=kubernetes-test-2/"* ]]

    # check auth backends disabled
    run vault auth list
    [ "$status" -eq 0 ]
    [[ "${output}" != *"kubernetes-test-2/"* ]]

    # check auth backends still enabled
    [[ "${output}" == *"kubernetes-test-1/"* ]]
    [[ "${output}" == *"token/"* ]]

    rerun_check
}
