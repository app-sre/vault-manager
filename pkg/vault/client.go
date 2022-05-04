// Package vault implements a wrapper around a Vault API client that retrieves
// credentials from the operating system environment.
package vault

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/app-sre/vault-manager/pkg/utils"
	"github.com/hashicorp/vault/api"
	log "github.com/sirupsen/logrus"
)

type VaultSecret struct {
	Name    string
	Type    string
	Path    string
	Field   string
	Version string
}

// names to assign to access attributes
const (
	ROLE_ID      = "roleID"
	SECRET_ID    = "secretID"
	TOKEN        = "token"
	APPROLE_AUTH = "approle"
	TOKEN_AUTH   = "token"
)

var masterAddress string
var vaultClients map[string]*api.Client

func InitClients(instanceCreds map[string][]*VaultSecret, threadPoolSize int) {
	vaultClients = make(map[string]*api.Client)

	configureMaster(vaultClients)

	bwg := utils.NewBoundedWaitGroup(threadPoolSize)
	var mutex = &sync.Mutex{}

	// read access credentials for other vault instances and configure clients
	for addr, secrets := range instanceCreds {
		bwg.Add(1)

		go func(a string, s []*VaultSecret) {
			defer bwg.Done()

			accessCreds := make(map[string]string)
			for _, cred := range s {
				raw := ReadSecret(masterAddress, cred.Path)
				mapped, ok := raw.Data["data"].(map[string]interface{})
				if !ok {
					log.Fatalf("[Vault Client] Failed to process raw result at path: `%s`", cred.Path)
				}
				if len(mapped) < 1 {
					log.Fatalf("[Vault Client] Data does not exist at path: `%s`", cred.Path)
				}
				if _, exists := mapped[cred.Field]; !exists {
					log.Fatalf("[Vault Client] Field `%s` does not exist at path: `%s`", cred.Field, cred.Path)
				}
				if _, ok := mapped[cred.Field].(string); !ok {
					log.Fatalf("[Vault Client] Field `%s` cannot be converted to string", cred.Field)
				}
				accessCreds[cred.Name] = mapped[cred.Field].(string)
			}

			config := api.DefaultConfig()
			config.Address = a
			client, err := api.NewClient(config)
			if err != nil {
				log.WithError(err).Fatalf("Failed to initialize Vault client for %s", a)
			}

			// at minimum, one element will exist in secrets regardless of type
			// type is same across all VaultSecrets associated with a particular instance address
			var token string
			switch s[0].Type {
			case APPROLE_AUTH:
				t, err := client.Logical().Write("auth/approle/login", map[string]interface{}{
					"role_id":   accessCreds[ROLE_ID],
					"secret_id": accessCreds[SECRET_ID],
				})
				if err != nil {
					log.WithError(err).Fatal("[Vault Client] failed to login to master Vault with AppRole")
				}
				token = t.Auth.ClientToken
			case TOKEN_AUTH:
				token = accessCreds[TOKEN]
			}

			mutex.Lock()
			defer mutex.Unlock()
			client.SetToken(token)
			vaultClients[a] = client
		}(addr, secrets)
	}
}

// configureMaster initializes vault client for the master instance
// This is the only client configured using environment variables
// env vars: VAULT_ADDR, VAULT_AUTHTYPE, VAULT_ROLE_ID, VAULT_SECRET_ID, VAULT_TOKEN
func configureMaster(instanceCreds map[string]*api.Client) {
	masterVaultCFG := api.DefaultConfig()
	masterVaultCFG.Address = mustGetenv("VAULT_ADDR")

	client, err := api.NewClient(masterVaultCFG)
	if err != nil {
		log.WithError(err).Fatal("failed to initialize master Vault client")
	}

	var clientToken string
	switch authType := defaultGetenv("VAULT_AUTHTYPE", "approle"); strings.ToLower(authType) {
	case APPROLE_AUTH:
		roleID := mustGetenv("VAULT_ROLE_ID")
		secretID := mustGetenv("VAULT_SECRET_ID")

		secret, err := client.Logical().Write("auth/approle/login", map[string]interface{}{
			"role_id":   roleID,
			"secret_id": secretID,
		})
		if err != nil {
			log.WithError(err).Fatal("[Vault Client] failed to login to master Vault with AppRole")
		}
		clientToken = secret.Auth.ClientToken
	case TOKEN_AUTH:
		clientToken = mustGetenv("VAULT_TOKEN")
	default:
		log.WithField("authType", authType).Fatal("[Vault Client] unsupported auth type")
	}

	client.SetToken(clientToken)
	vaultClients[masterVaultCFG.Address] = client
	masterAddress = masterVaultCFG.Address
}

// returns the vault client associated with instance address
func getClient(instanceAddr string) *api.Client {
	if vaultClients[instanceAddr] == nil {
		log.Fatalf("[Vault Client] client does not exist for address: %s", instanceAddr)
	}
	return vaultClients[instanceAddr]
}

// write secret to vault
func WriteSecret(instanceAddr string, secretPath string, secretData map[string]interface{}) {
	if !DataInSecret(instanceAddr, secretData, secretPath) {
		_, err := getClient(instanceAddr).Logical().Write(secretPath, secretData)
		if err != nil {
			log.WithError(err).WithField("path", secretPath).Fatalf("[Vault Client] failed to write Vault secret ")
		}
	}
}

// read secret from vault
func ReadSecret(instanceAddr string, secretPath string) *api.Secret {
	secret, err := getClient(instanceAddr).Logical().Read(secretPath)
	if err != nil {
		log.WithError(err).WithField("path", secretPath).Fatal("[Vault Client] failed to read Vault secret")
	}
	return secret
}

// list secrets
func ListSecrets(instanceAddr string, path string) *api.Secret {
	secretsList, err := getClient(instanceAddr).Logical().List(path)
	if err != nil {
		log.WithError(err).WithField("path", path).Fatal("[Vault Client] failed to list Vault secrets")
	}
	return secretsList
}

// delete secret from vault
func DeleteSecret(instanceAddr string, secretPath string) {
	_, err := getClient(instanceAddr).Logical().Delete(secretPath)
	if err != nil {
		log.WithError(err).WithField("path", secretPath).Fatal("[Vault Client] failed to delete Vault secret")
	}
}

// list existing enabled Audits Devices.
func ListAuditDevices(instanceAddr string) map[string]*api.Audit {
	enabledAuditDevices, err := getClient(instanceAddr).Sys().ListAudit()
	if err != nil {
		log.WithError(err).Fatal("[Vault Audit] failed to list audit devices")
	}
	return enabledAuditDevices
}

// enable audit device with options
func EnableAduitDevice(instanceAddr string, path string, options *api.EnableAuditOptions) {
	if err := getClient(instanceAddr).Sys().EnableAuditWithOptions(path, options); err != nil {
		log.WithField("path", path).Fatal("[Vault Audit] failed to enable audit device")
	}
	log.WithField("path", path).Info("[Vault Audit] audit device is successfully enabled")
}

// disable audit device
func DisableAuditDevice(instanceAddr string, path string) {
	if err := getClient(instanceAddr).Sys().DisableAudit(path); err != nil {
		log.WithField("path", path).Fatal("[Vault Audit] failed to disable audit device")
	}
	log.WithField("path", path).Info("[Vault Audit] audit device is successfully disabled")
}

// list existing auth backends
func ListAuthBackends(instanceAddr string) map[string]*api.AuthMount {
	existingAuthMounts, err := getClient(instanceAddr).Sys().ListAuth()
	if err != nil {
		log.WithError(err).Fatal("[Vault Auth] failed to list auth backends from Vault instance")
	}
	return existingAuthMounts
}

// enable auth backend
func EnableAuthWithOptions(instanceAddr string, path string, options *api.EnableAuthOptions) {
	if err := getClient(instanceAddr).Sys().EnableAuthWithOptions(path, options); err != nil {
		log.WithError(err).WithField("path", path).WithField("type", options.Type).Fatal("[Vault Auth] failed to enable auth backend")
	}
	log.WithFields(log.Fields{
		"path": path,
		"type": options.Type,
	}).Info("[Vault Auth] successfully enabled auth backend")
}

// disable auth backend
func DisableAuth(instanceAddr string, path string) {
	if err := getClient(instanceAddr).Sys().DisableAuth(path); err != nil {
		log.WithError(err).WithField("path", path).Fatal("[Vault Auth] failed to disable auth backend")
	}
	log.WithField("path", path).Info("[Vault Auth] successfully disabled auth backend")
}

// list vault policies
func ListVaultPolicies(instanceAddr string) []string {
	existingPolicyNames, err := getClient(instanceAddr).Sys().ListPolicies()
	if err != nil {
		log.WithError(err).Fatal("[Vault Policy] failed to list Vault policies")
	}
	return existingPolicyNames
}

// get vault policy
func GetVaultPolicy(instanceAddr string, name string) string {
	policy, err := getClient(instanceAddr).Sys().GetPolicy(name)
	if err != nil {
		log.WithError(err).WithField("name", name).Fatal("[Vault Policy] failed to get existing Vault policy")
	}
	return policy
}

// put vault policy
func PutVaultPolicy(instanceAddr string, name string, rules string) {
	if err := getClient(instanceAddr).Sys().PutPolicy(name, rules); err != nil {
		log.WithError(err).WithField("name", name).Fatal("[Vault Policy] failed to write policy to Vault instance")
	}
	log.WithField("name", name).Info("[Vault Policy] policy successfully written to Vault instance")
}

// delete vault policy
func DeleteVaultPolicy(instanceAddr string, name string) {
	if err := getClient(instanceAddr).Sys().DeletePolicy(name); err != nil {
		log.WithError(err).WithField("name", name).Fatal("[Vault Policy] failed to delete vault policy")
	}
	log.WithField("name", name).Info("[Vault Policy] successfully deleted policy from Vault instance")

}

// list secrets engines
func ListSecretsEngines(instanceAddr string) map[string]*api.MountOutput {
	existingMounts, err := getClient(instanceAddr).Sys().ListMounts()
	if err != nil {
		log.WithError(err).Fatal("[Vault Secrets engine] failed to list Vault secrets engines")
	}
	return existingMounts
}

// enable secrets engine
func EnableSecretsEngine(instanceAddr string, path string, mount *api.MountInput) {
	if err := getClient(instanceAddr).Sys().Mount(path, mount); err != nil {
		log.WithError(err).WithField("path", path).WithField("type", mount.Type).Fatal("[Vault Secrets engine] failed to enable secrets-engine")
	}
	log.WithField("path", path).WithField("type", mount.Type).Info("[Vault Secrets engine] successfully enabled secrets-engine")
}

// update secrets engine
func UpdateSecretsEngine(instanceAddr string, path string, config api.MountConfigInput) {
	if err := getClient(instanceAddr).Sys().TuneMount(path, config); err != nil {
		log.WithError(err).WithField("path", path).Fatal("[Vault Secrets engine] failed to update secrets-engine")
	}
	log.WithField("path", path).Info("[Vault Secrets engine] successfully updated secrets-engine")
}

// disable secrets engine
func DisableSecretsEngine(instanceAddr string, path string) {
	if err := getClient(instanceAddr).Sys().Unmount(path); err != nil {
		log.WithError(err).WithField("path", path).Fatal("[Vault Secrets engine] failed to disable secrets-engine")
	}
	log.WithField("path", path).Info("[Vault Secrets engine] successfully disabled secrets-engine")
}

// GetVaultVersion returns the vault server version
func GetVaultVersion(instanceAddr string) string {
	info, err := getClient(instanceAddr).Sys().Health()
	if err != nil {
		log.WithError(err).Fatal("[Vault System] failed to retrieve vault system information")
	}
	return info.Version
}

func ListEntities(instanceAddr string) map[string]interface{} {
	existingEntities, err := getClient(instanceAddr).Logical().List("identity/entity/id")
	if err != nil {
		log.WithError(err).Fatal("[Vault Identity] failed to list Vault entities")
	}
	if existingEntities == nil {
		return nil
	}
	return existingEntities.Data
}

func GetEntityInfo(instanceAddr string, name string) map[string]interface{} {
	entity, err := getClient(instanceAddr).Logical().Read(fmt.Sprintf("identity/entity/name/%s", name))
	if err != nil {
		log.WithError(err).Fatalf("[Vault Identity] failed to get info for entity: %s", name)
	}
	if entity == nil {
		return nil
	}
	return entity.Data
}

func GetEntityAliasInfo(instanceAddr string, id string) map[string]interface{} {
	entityAlias, err := getClient(instanceAddr).Logical().Read(fmt.Sprintf("identity/entity-alias/id/%s", id))
	if err != nil {
		log.WithError(err).Fatalf("[Vault Identity] failed to get info for entity alias: %s", id)
	}
	return entityAlias.Data
}

func WriteEntityAlias(instanceAddr string, secretPath string, secretData map[string]interface{}) {
	_, err := getClient(instanceAddr).Logical().Write(secretPath, secretData)
	if err != nil {
		log.WithError(err).WithField("path", secretPath).Fatal("[Vault Client] failed to write entity-alias secret")
	}
}

func ListGroups(instanceAddr string) map[string]interface{} {
	existingGroups, err := getClient(instanceAddr).Logical().List("identity/group/id")
	if err != nil {
		log.WithError(err).Fatal("[Vault Group] failed to list Vault groups")
	}
	if existingGroups == nil {
		return nil
	}
	return existingGroups.Data
}

func GetGroupInfo(instanceAddr string, name string) map[string]interface{} {
	entity, err := getClient(instanceAddr).Logical().Read(fmt.Sprintf("identity/group/name/%s", name))
	if err != nil {
		log.WithError(err).Fatalf("[Vault Group] failed to get info for group: %s", name)
	}
	if entity == nil {
		return nil
	}
	return entity.Data
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
