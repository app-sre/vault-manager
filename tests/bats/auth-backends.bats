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
    [[ "${output}" == *"successfully enabled auth backend"*"path=approle-test-1/"*"type=approle"* ]]
    [[ "${output}" == *"successfully enabled auth backend"*"path=alicloud-test-1/"*"type=alicloud"* ]]
    [[ "${output}" == *"successfully enabled auth backend"*"path=azure-test-1/"*"type=azure"* ]]
    [[ "${output}" == *"successfully enabled auth backend"*"path=github-test-1/"*"type=github"* ]]
    [[ "${output}" == *"successfully enabled auth backend"*"path=gcp-test-1/"*"type=gcp"* ]]
    [[ "${output}" == *"successfully enabled auth backend"*"path=jwt-test-1/"*"type=jwt"* ]]
    [[ "${output}" == *"successfully enabled auth backend"*"path=kubernetes-test-1/"*"type=kubernetes"* ]]
    [[ "${output}" == *"successfully enabled auth backend"*"path=ldap-test-1/"*"type=ldap"* ]]
    [[ "${output}" == *"successfully enabled auth backend"*"path=okta-test-1/"*"type=okta"* ]]
    [[ "${output}" == *"successfully enabled auth backend"*"path=radius-test-1/"*"type=radius"* ]]
    [[ "${output}" == *"successfully enabled auth backend"*"path=cert-test-1/"*"type=cert"* ]]
    [[ "${output}" == *"successfully enabled auth backend"*"path=userpass-test-1/"*"type=userpass"* ]]
    [[ "${output}" == *"successfully enabled auth backend"*"path=aws-test-1/"*"type=aws"* ]]

    [[ "${output}" == *"auth mount successfully configured"*"path=auth/azure-test-1/config"*"type=azure"* ]]
    [[ "${output}" == *"auth mount successfully configured"*"path=auth/github-test-1/config"*"type=github"* ]]
    [[ "${output}" == *"auth mount successfully configured"*"path=auth/jwt-test-1/config"*"type=jwt"* ]]
    [[ "${output}" == *"auth mount successfully configured"*"path=auth/kubernetes-test-1/config"*"type=kubernetes"* ]]
    [[ "${output}" == *"auth mount successfully configured"*"path=auth/ldap-test-1/config"*"type=ldap"* ]]
    [[ "${output}" == *"auth mount successfully configured"*"path=auth/okta-test-1/config"*"type=okta"* ]]
    [[ "${output}" == *"auth mount successfully configured"*"path=auth/aws-test-1/config/client"*"type=aws"* ]]

    # check auth backends created
    run vault auth list
    [ "$status" -eq 0 ]
    [[ "${output}" == *"approle-test-1/"* ]]
    [[ "${output}" == *"github-test-1/"*"github-test-1 auth backend"* ]]
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

    # check configurations
    # check azure-test-1 auth configuration
    run vault read auth/azure-test-1/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"tenant_id"*"azure-test-tenant-id"* ]]
    [[ "${output}" == *"resource"*"https://vault.hashicorp.com"* ]]
    [[ "${output}" == *"client_id"*"azure-test-client-id"* ]]
    [[ "${output}" == *"environment"* ]]

    # check github-test-1 auth configuration
    run vault read auth/github-test-1/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"organization"*"test-org-1"* ]]
    [[ "${output}" == *"ttl"*"72h"* ]]
    [[ "${output}" == *"max_ttl"*"72h"* ]]
    [[ "${output}" == *"base_url"*"base_url_test_1"* ]]

    # check jwt-test-1 auth configuration
    run vault read auth/jwt-test-1/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"oidc_discovery_url"*"https://myco.auth0.com/"* ]]
    [[ "${output}" == *"oidc_discovery_ca_pem"* ]]
    [[ "${output}" == *"jwt_validation_pubkeys"*"[]"* ]]
    [[ "${output}" == *"bound_issuer"* ]]

    # check kubernetes-test-1 auth configuration
    run vault read auth/kubernetes-test-1/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"kubernetes_host"*"https://192.168.99.100:8443"* ]]
    [[ "${output}" == *"pem_keys"*"[]"* ]]
    [[ "${output}" == *"kubernetes_ca_cert"*"dummy-cert"* ]]

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

    # check aws-test-1 auth configuration
    run vault read auth/aws-test-1/config/client
    [ "$status" -eq 0 ]
    [[ "${output}" == *"access_key"*"access_key"* ]]
    [[ "${output}" == *"max_retries"*"-1"* ]]
    [[ "${output}" == *"endpoint"* ]]
    [[ "${output}" == *"iam_endpoint"* ]]
    [[ "${output}" == *"iam_server_id_header_value"* ]]
    [[ "${output}" == *"sts_endpoint"* ]]

    rerun_check

    #
    # CASE: update configurations
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/update_auth_backends.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"auth mount successfully configured"*"path=auth/github-test-1/config"* ]]

    # check configurations
    # check azure-test-1 auth configuration
    run vault read auth/azure-test-1/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"tenant_id"*"azure-test-tenant-id-updated"* ]]
    [[ "${output}" == *"resource"*"https://vault.hashicorp-updated.com"* ]]
    [[ "${output}" == *"client_id"*"azure-test-client-id-updated"* ]]

    # check github-test-1 auth configuration
    run vault read auth/github-test-1/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"ttl"*"12h57m"* ]]
    [[ "${output}" == *"max_ttl"*"777h"* ]]
    [[ "${output}" == *"organization"*"test-org-1-updated"* ]]
    [[ "${output}" == *"base_url"*"base_url_test_1-updated"* ]]

    # check jwt-test-1 auth configuration
    run vault read auth/jwt-test-1/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"bound_issuer"*"updated"* ]]

    # check kubernetes-test-1 auth configuration
    run vault read auth/kubernetes-test-1/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"kubernetes_host"*"https://192.168.99.100-updated:8443"* ]]
    [[ "${output}" == *"kubernetes_ca_cert"*"dummy-cert-updated"* ]]

    # check ldap-test-1 auth configuration
    run vault read auth/ldap-test-1/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"binddn"*"cn=vault-updated,ou=users,dc=example,dc=com"* ]]
    [[ "${output}" == *"url"*"ldap://ldap.example-updated.com"* ]]
    [[ "${output}" == *"use_token_groups"*"false"* ]]
    [[ "${output}" == *"userattr"*"samaccountname"* ]]
    [[ "${output}" == *"userdn"*"ou=Users,dc=example-updated,dc=com"* ]]

    # check okta-test-1 auth configuration
    run vault read auth/okta-test-1/config
    [ "$status" -eq 0 ]
    [[ "${output}" == *"base_url"*"okta-updated.com"* ]]
    [[ "${output}" == *"organization"*"test-okta-org-updated"* ]]
    [[ "${output}" == *"org_name"*"test-okta-org-updated"* ]]
    [[ "${output}" == *"max_ttl"*"777h"* ]]
    [[ "${output}" == *"ttl"*"12h"* ]]

    # check aws-test-1 auth configuration
    run vault read auth/aws-test-1/config/client
    [ "$status" -eq 0 ]
    [[ "${output}" == *"access_key"*"access_key-updated"* ]]
    [[ "${output}" == *"max_retries"*"-1"* ]]
    [[ "${output}" == *"endpoint"* ]]

    rerun_check

    #
    # CASE: disable auth backends
    #
    export VAULT_MANAGER_CONFIG_FILE=/tests/fixtures/auth/disable_auth_backends.yaml
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == *"successfully disabled auth backend"*"path=approle-test-1/"* ]]
    [[ "${output}" == *"successfully disabled auth backend"*"path=alicloud-test-1/"* ]]
    [[ "${output}" == *"successfully disabled auth backend"*"path=aws-test-1/"* ]]
    [[ "${output}" == *"successfully disabled auth backend"*"path=azure-test-1/"* ]]
    [[ "${output}" == *"successfully disabled auth backend"*"path=cert-test-1/"* ]]
    [[ "${output}" == *"successfully disabled auth backend"*"path=gcp-test-1/"* ]]
    [[ "${output}" == *"successfully disabled auth backend"*"path=jwt-test-1/"* ]]
    [[ "${output}" == *"successfully disabled auth backend"*"path=kubernetes-test-1/"* ]]
    [[ "${output}" == *"successfully disabled auth backend"*"path=ldap-test-1/"* ]]
    [[ "${output}" == *"successfully disabled auth backend"*"path=okta-test-1/"* ]]
    [[ "${output}" == *"successfully disabled auth backend"*"path=radius-test-1/"* ]]
    [[ "${output}" == *"successfully disabled auth backend"*"path=userpass-test-1/"* ]]

    # check auth backends disabled
    run vault auth list
    [ "$status" -eq 0 ]
    [[ "${output}" != *"approle-test-1/"* ]]
    [[ "${output}" != *"alicloud-test-1/"* ]]
    [[ "${output}" != *"aws-test-1/"* ]]
    [[ "${output}" != *"azure-test-1/"* ]]
    [[ "${output}" != *"cert-test-1/"* ]]
    [[ "${output}" != *"gcp-test-1/"* ]]
    [[ "${output}" != *"jwt-test-1/"* ]]
    [[ "${output}" != *"kubernetes-test-1/"* ]]
    [[ "${output}" != *"ldap-test-1/"* ]]
    [[ "${output}" != *"okta-test-1/"* ]]
    [[ "${output}" != *"radius-test-1/"* ]]
    [[ "${output}" != *"userpass-test-1/"* ]]

    # check auth backends still enabled
    [[ "${output}" == *"github-test-1/"* ]]
    [[ "${output}" == *"token/"* ]]

    rerun_check
}
