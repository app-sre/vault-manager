#!/usr/bin/env bats

load helpers

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
    [[ "${output}" == *"auth mount successfully configured"*"path=auth/github-test-1/config"* ]]
    [[ "${output}" == *"auth mount successfully configured"*"path=auth/github-test-2/config"* ]]
    # check auth backends created
    run vault auth list
    [ "$status" -eq 0 ]
    [[ "${output}" == *"approle-1/"* ]]
    [[ "${output}" == *"approle-2/"* ]]
    [[ "${output}" == *"github-test-1/"*"github-test-1 auth backend"* ]]
    [[ "${output}" == *"github-test-2/"*"github-test-2 auth backend"* ]]
    [[ "${output}" == *"alicloud-test-1/"*"alicloud-test-1 auth backend"* ]]
    [[ "${output}" == *"aws-test-1/"*"aws-test-1 auth backend"* ]]
    [[ "${output}" == *"azure-test-1/"*"azure-test-1 auth backend"* ]]
    [[ "${output}" == *"cert-test-1/"*"cert-test-1 auth backend"* ]]
    [[ "${output}" == *"gcp-test-1/"*"gcp-test-1 auth backend"* ]]
    [[ "${output}" == *"jwt-test-1/"*"jwt-test-1 auth backend"* ]]
    [[ "${output}" == *"kubernetes-test-1/"*"kubernetes-test-1 auth backend"* ]]
    [[ "${output}" == *"ldap-test-1/"*"ldap-test-1 auth backend"* ]]
    [[ "${output}" == *"okta-test-1/"*"okta-test-1 auth backend"* ]]
    [[ "${output}" == *"radius-test-1/"*"radius-test-1 auth backend"* ]]
    [[ "${output}" == *"userpass-test-1/"*"userpass-test-1 auth backend"* ]]
    # check github-test-1 auth configuration
    run vault read auth/github-test-1/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"organization"*"test-org-1"* ]]
    [[ "${output}" == *"ttl"*"72h"* ]]
    [[ "${output}" == *"max_ttl"*"72h"* ]]
    [[ "${output}" == *"base_url"*"n/a"* ]]
    # check kubernetes-test-1 auth configuration
    run vault read auth/kubernetes-test-1/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"kubernetes_host"*"https://192.168.99.100:8443"* ]]
    [[ "${output}" == *"pem_keys"*"[]"* ]]
    [[ "${output}" == *"kubernetes_ca_cert"*"DTALBgNVBAsMBHRlc3QxDTALBgNVBAMMBHRlc3QxEzARBgkqhkiG9w0BCQEWBHRl"* ]]
    [[ "${output}" == *"kubernetes_ca_cert"*"c3QwgZ8wDQYJKoZIhvcNAQEBBQADgY0AMIGJAoGBAMkMxHQJ8MaO4WbjdIZHotsy"* ]]
    [[ "${output}" == *"kubernetes_ca_cert"*"KOZDA5EuA7v8azaFogTla6p+LnaqwvkjYLe0dgmoPOKZDSSU+Of9PsDc7eTKt88M"* ]]
    # check azure-test-1 auth configuration
    run vault read auth/azure-test-1/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"tenant_id"*"azure-test-tenant-id"* ]]
    [[ "${output}" == *"resource"*"https://vault.hashicorp.com"* ]]
    [[ "${output}" == *"client_id"*"azure-test-client-id"* ]]
    # check jwt-test-1 auth configuration
    run vault read auth/jwt-test-1/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"bound_issuer"* ]]
    [[ "${output}" == *"jwt_validation_pubkeys"*"[]"* ]]
    [[ "${output}" == *"oidc_discovery_ca_pem"* ]]
    [[ "${output}" == *"oidc_discovery_url"*"https://myco.auth0.com/"* ]]
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
    # check okta-test-1 auth configuration
    run vault read auth/okta-test-1/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"base_url"*"okta.com"* ]]
    [[ "${output}" == *"organization"*"test-okta-org"* ]]
    [[ "${output}" == *"org_name"*"test-okta-org"* ]]
    [[ "${output}" == *"bypass_okta_mfa"*"false"* ]]
    [[ "${output}" == *"max_ttl"*"0s"* ]]
    [[ "${output}" == *"ttl"*"0s"* ]]

    rerun_check

    #
    # CASE: update github config
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/update_auth_backends.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"auth mount successfully configured"*"path=auth/github-test-1/config"* ]]
    [[ "${output}" == *"auth mount successfully configured"*"path=auth/github-test-2/config"* ]]
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
