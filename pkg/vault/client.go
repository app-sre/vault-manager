// Package vault implements a wrapper around a Vault API client that retrieves
// credentials from the operating system environment.
package vault

import (
	"os"
	"strings"

	"github.com/hashicorp/vault/api"
	log "github.com/sirupsen/logrus"
)

var clientToken string
var client *api.Client

//  Client initializes a Vault client using the environment variables:
// VAULT_ADDR, VAULT_ROLE_ID, VAULT_SECRET_ID, VAULT_TOKEN.
func getClient() *api.Client {
	vaultCFG := api.DefaultConfig()
	vaultCFG.Address = mustGetenv("VAULT_ADDR")
	if clientToken == "" {
		var err error
		client, err = api.NewClient(vaultCFG)
		if err != nil {
			log.WithError(err).Fatal("failed to initialize Vault client")
		}

		switch authType := defaultGetenv("VAULT_AUTHTYPE", "approle"); strings.ToLower(authType) {
		case "approle":
			roleID := mustGetenv("VAULT_ROLE_ID")
			secretID := mustGetenv("VAULT_SECRET_ID")

			secret, err := client.Logical().Write("auth/approle/login", map[string]interface{}{
				"role_id":   roleID,
				"secret_id": secretID,
			})
			if err != nil {
				log.WithError(err).Fatal("[Vault Client] failed to login to Vault with AppRole")
			}
			clientToken = secret.Auth.ClientToken
		case "token":
			clientToken = mustGetenv("VAULT_TOKEN")
		default:
			log.WithField("authType", authType).Fatal("[Vault Client] unsupported auth type")
		}
	}

	client.SetToken(clientToken)
	return client
}

// write secret to vault
func WriteSecret(secretPath string, secretData map[string]interface{}) {
	if !DataInSecret(secretData, secretPath) {
		_, err := getClient().Logical().Write(secretPath, secretData)
		if err != nil {
			log.WithError(err).WithField("path", secretPath).Fatalf("[Vault Client] failed to write Vault secret ")
		}
	}
}

// read secret from vault
func ReadSecret(secretPath string) *api.Secret {
	secret, err := getClient().Logical().Read(secretPath)
	if err != nil {
		log.WithError(err).WithField("path", secretPath).Fatal("[Vault Client] failed to read Vault secret")
	}
	return secret
}

// list secrets
func ListSecrets(path string) *api.Secret {
	secretsList, err := getClient().Logical().List(path)
	if err != nil {
		log.WithError(err).WithField("path", path).Fatal("[Vault Client] failed to list Vault secrets")
	}
	return secretsList
}

// delete secret from vault
func DeleteSecret(secretPath string) {
	_, err := getClient().Logical().Delete(secretPath)
	if err != nil {
		log.WithError(err).WithField("path", secretPath).Fatal("[Vault Client] failed to delete Vault secret")
	}
}

// list existing enabled Audits Devices.
func ListAuditDevices() map[string]*api.Audit {
	enabledAuditDevices, err := getClient().Sys().ListAudit()
	if err != nil {
		log.WithError(err).Fatal("[Vault Audit] failed to list audit devices")
	}
	return enabledAuditDevices
}

// enable audit device with options
func EnableAduitDevice(path string, options *api.EnableAuditOptions) {
	if err := getClient().Sys().EnableAuditWithOptions(path, options); err != nil {
		log.WithField("path", path).Fatal("[Vault Audit] failed to enable audit device")
	}
	log.WithField("path", path).Info("[Vault Audit] audit device is successfully enabled")
}

// disable audit device
func DisableAuditDevice(path string) {
	if err := getClient().Sys().DisableAudit(path); err != nil {
		log.WithField("path", path).Fatal("[Vault Audit] failed to disable audit device")
	}
	log.WithField("path", path).Info("[Vault Audit] audit device is successfully disabled")
}

// list existing auth backends
func ListAuthBackends() map[string]*api.AuthMount {
	existingAuthMounts, err := getClient().Sys().ListAuth()
	if err != nil {
		log.WithError(err).Fatal("[Vault Auth] failed to list auth backends from Vault instance")
	}
	return existingAuthMounts
}

// enable auth backend
func EnableAuthWithOptions(path string, options *api.EnableAuthOptions) {
	if err := getClient().Sys().EnableAuthWithOptions(path, options); err != nil {
		log.WithError(err).WithField("path", path).WithField("type", options.Type).Fatal("[Vault Auth] failed to enable auth backend")
	}
	log.WithFields(log.Fields{
		"path": path,
		"type": options.Type,
	}).Info("[Vault Auth] successfully enabled auth backend")
}

// disable auth backend
func DisableAuth(path string) {
	if err := getClient().Sys().DisableAuth(path); err != nil {
		log.WithError(err).WithField("path", path).Fatal("[Vault Auth] failed to disable auth backend")
	}
	log.WithField("path", path).Info("[Vault Auth] successfully disabled auth backend")
}

// list vault policies
func ListVaultPolicies() []string {
	existingPolicyNames, err := getClient().Sys().ListPolicies()
	if err != nil {
		log.WithError(err).Fatal("[Vault Policy] failed to list Vault policies")
	}
	return existingPolicyNames
}

// get vault policy
func GetVaultPolicy(name string) string {
	policy, err := getClient().Sys().GetPolicy(name)
	if err != nil {
		log.WithError(err).WithField("name", name).Fatal("[Vault Policy] failed to get existing Vault policy")
	}
	return policy
}

// put vault policy
func PutVaultPolicy(name string, rules string) {
	if err := getClient().Sys().PutPolicy(name, rules); err != nil {
		log.WithError(err).WithField("name", name).Fatal("[Vault Policy] failed to write policy to Vault instance")
	}
	log.WithField("name", name).Info("[Vault Policy] policy successfully written to Vault instance")
}

// delete vault policy
func DeleteVaultPolicy(name string) {
	if err := getClient().Sys().DeletePolicy(name); err != nil {
		log.WithError(err).WithField("name", name).Fatal("[Vault Policy] failed to delete vault policy")
	}
	log.WithField("name", name).Info("[Vault Policy] successfully deleted policy from Vault instance")

}

// list secrets engines
func ListSecretsEngines() map[string]*api.MountOutput {
	existingMounts, err := getClient().Sys().ListMounts()
	if err != nil {
		log.WithError(err).Fatal("[Vault Secrets engine] failed to list Vault secrets engines")
	}
	return existingMounts
}

// enable secrets engine
func EnableSecretsEngine(path string, mount *api.MountInput) {
	if err := getClient().Sys().Mount(path, mount); err != nil {
		log.WithError(err).WithField("path", path).WithField("type", mount.Type).Fatal("[Vault Secrets engine] failed to enable secrets-engine")
	}
	log.WithField("path", path).WithField("type", mount.Type).Info("[Vault Secrets engine] successfully enabled secrets-engine")
}

// update secrets engine
func UpdateSecretsEngine(path string, config api.MountConfigInput) {
	if err := getClient().Sys().TuneMount(path, config); err != nil {
		log.WithError(err).WithField("path", path).Fatal("[Vault Secrets engine] failed to update secrets-engine")
	}
	log.WithField("path", path).Info("[Vault Secrets engine] successfully updated secrets-engine")
}

// disable secrets engine
func DisableSecretsEngine(path string) {
	if err := getClient().Sys().Unmount(path); err != nil {
		log.WithError(err).WithField("path", path).Fatal("[Vault Secrets engine] failed to disable secrets-engine")
	}
	log.WithField("path", path).Info("[Vault Secrets engine] successfully disabled secrets-engine")
}

func mustGetenv(name string) string {
	env := os.Getenv(name)
	if env == "" {
		log.WithField("env", name).Fatal("required environment variable is unset")
	}
	return env
}

func defaultGetenv(name, defaultName string) string {
	env := os.Getenv(name)
	if env == "" {
		env = defaultName
	}
	return env
}
