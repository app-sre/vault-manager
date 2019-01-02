#!/usr/bin/env bats

load ../helpers

@test "test vault-manager manage auth backend ldap" {
    #
    # CASE: enable auth backend ldap
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/ldap/enable_auth_backends.yaml
    run vault-manager

    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully enabled auth backend"*"path=ldap-test-1/"*"type=ldap"* ]]
    [[ "${output}" == *"successfully enabled auth backend"*"path=ldap-test-2/"*"type=ldap"* ]]

    [[ "${output}" == *"auth mount successfully configured"*"path=auth/ldap-test-1/config"*"type=ldap"* ]]
    [[ "${output}" == *"auth mount successfully configured"*"path=auth/ldap-test-2/config"*"type=ldap"* ]]

    # check auth backends created
    run vault auth list
    [ "$status" -eq 0 ]
    [[ "${output}" == *"ldap-test-1/"*"ldap-test-1 auth backend"* ]]
    [[ "${output}" == *"ldap-test-2/"*"ldap-test-2 auth backend"* ]]

    # check ldap-test-1 auth configuration
    run vault read auth/ldap-test-1/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"binddn"*"cn=vault,ou=users,dc=example,dc=com"* ]]
    [[ "${output}" == *"case_sensitive_names"*"false"* ]]
    [[ "${output}" == *"certificate"* ]]
    [[ "${output}" == *"deny_null_bind"*"true"* ]]
    [[ "${output}" == *"discoverdn"*"false"* ]]
    [[ "${output}" == *"groupattr"*"memberOf"* ]]
    [[ "${output}" == *"groupdn"*"ou=Users,dc=example,dc=com"* ]]
    [[ "${output}" == *"groupfilter"*"(&(objectClass=person)(uid={{.Username}}))"* ]]
    [[ "${output}" == *"insecure_tls"*"false"* ]]
    [[ "${output}" == *"starttls"*"true"* ]]
    [[ "${output}" == *"tls_max_version"*"tls12"* ]]
    [[ "${output}" == *"tls_min_version"*"tls12"* ]]
    [[ "${output}" == *"upndomain"* ]]
    [[ "${output}" == *"url"*"ldap://ldap.example.com"* ]]
    [[ "${output}" == *"use_token_groups"*"false"* ]]
    [[ "${output}" == *"userattr"*"samaccountname"* ]]
    [[ "${output}" == *"userdn"*"ou=Users,dc=example,dc=com"* ]]

    # check ldap-test-2 auth configuration
    run vault read auth/ldap-test-2/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"binddn"*"cn=vault,ou=users,dc=example,dc=com"* ]]
    [[ "${output}" == *"case_sensitive_names"*"false"* ]]
    [[ "${output}" == *"certificate"* ]]
    [[ "${output}" == *"deny_null_bind"*"true"* ]]
    [[ "${output}" == *"discoverdn"*"false"* ]]
    [[ "${output}" == *"groupattr"*"memberOf"* ]]
    [[ "${output}" == *"groupdn"*"ou=Users,dc=example,dc=com"* ]]
    [[ "${output}" == *"groupfilter"*"(&(objectClass=person)(uid={{.Username}}))"* ]]
    [[ "${output}" == *"insecure_tls"*"false"* ]]
    [[ "${output}" == *"starttls"*"true"* ]]
    [[ "${output}" == *"tls_max_version"*"tls12"* ]]
    [[ "${output}" == *"tls_min_version"*"tls12"* ]]
    [[ "${output}" == *"upndomain"* ]]
    [[ "${output}" == *"url"*"ldap://ldap.example.com"* ]]
    [[ "${output}" == *"use_token_groups"*"false"* ]]
    [[ "${output}" == *"userattr"*"samaccountname"* ]]
    [[ "${output}" == *"userdn"*"ou=Users,dc=example,dc=com"* ]]

    rerun_check

    #
    # CASE: update configurations
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/ldap/update_auth_backends.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"auth mount successfully configured"*"path=auth/ldap-test-1/config"*"type=ldap"* ]]
    [[ "${output}" == *"auth mount successfully configured"*"path=auth/ldap-test-2/config"*"type=ldap"* ]]

    # check ldap-test-1 auth configuration
    run vault read auth/ldap-test-1/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"binddn"*"cn=vault-updated,ou=users,dc=example,dc=com"* ]]
    [[ "${output}" == *"url"*"ldap://ldap.example-updated.com"* ]]
    [[ "${output}" == *"use_token_groups"*"false"* ]]
    [[ "${output}" == *"userattr"*"samaccountname"* ]]
    [[ "${output}" == *"userdn"*"ou=Users,dc=example-updated,dc=com"* ]]

    # check ldap-test-2 auth configuration
    run vault read auth/ldap-test-2/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"binddn"*"cn=vault-updated,ou=users,dc=example,dc=com"* ]]
    [[ "${output}" == *"url"*"ldap://ldap.example-updated.com"* ]]
    [[ "${output}" == *"use_token_groups"*"false"* ]]
    [[ "${output}" == *"userattr"*"samaccountname"* ]]
    [[ "${output}" == *"userdn"*"ou=Users,dc=example-updated,dc=com"* ]]

    #
    # CASE: disable auth backend ldap
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/ldap/disable_auth_backends.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully disabled auth backend"*"path=ldap-test-2/"* ]]

    # check auth backends disabled
    run vault auth list
    [ "$status" -eq 0 ]
    [[ "${output}" != *"ldap-test-2/"* ]]

    # check auth backends still enabled
    [[ "${output}" == *"ldap-test-1/"* ]]
    [[ "${output}" == *"token/"* ]]

    rerun_check
}
