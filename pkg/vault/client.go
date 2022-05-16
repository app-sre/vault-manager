// Package vault implements a wrapper around a Vault API client that retrieves
// credentials from the operating system environment.
package vault

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/app-sre/vault-manager/pkg/utils"
	"github.com/hashicorp/vault/api"
	log "github.com/sirupsen/logrus"
)

type AuthBundle struct {
	SecretEngine string
	VaultSecrets []*VaultSecret
}

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
	KV_V1        = "kv_v1"
	KV_V2        = "kv_v2"
)

var masterAddress string
var vaultClients map[string]*api.Client

// Called once with toplevel/instance
// Creates global map of all vault clients defined in a-i
// This allows reconciliation of multiple vault instances
func InitClients(instanceCreds map[string]AuthBundle, threadPoolSize int) {
	vaultClients = make(map[string]*api.Client)
	configureMaster(vaultClients)

	bwg := utils.NewBoundedWaitGroup(threadPoolSize)
	var mutex = &sync.Mutex{}
	// read access credentials for other vault instances and configure clients
	for addr, bundle := range instanceCreds {
		bwg.Add(1)
		go createClient(addr, bundle, &bwg, mutex)
	}
	bwg.Wait()
}

// goroutine support function for InitClients()
// initializes one vault client
func createClient(a string, b AuthBundle, bwg *utils.BoundedWaitGroup, mutex *sync.Mutex) {
	defer bwg.Done()

	accessCreds := make(map[string]string)
	for _, cred := range b.VaultSecrets {
		processedCred, err := processAccessCredential(cred, b.SecretEngine)
		if err != nil {
			log.WithError(err).Fatal()
		}
		accessCreds[cred.Name] = processedCred
	}

	// Init new client
	config := api.DefaultConfig()
	config.Address = a
	client, err := api.NewClient(config)
	if err != nil {
		log.WithError(err).Fatalf("Failed to initialize Vault client for %s", a)
	}

	// at minimum, one element will exist in secrets regardless of type
	// type is same across all VaultSecrets associated with a particular instance address
	var token string
	switch b.VaultSecrets[0].Type {
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
	// add new address/client pair to global
	mutex.Lock()
	defer mutex.Unlock()
	client.SetToken(token)
	vaultClients[a] = client
}

// attempts to read/proccess a single access credential for a particular vault instance
func processAccessCredential(cred *VaultSecret, secretEngine string) (string, error) {
	raw := ReadSecret(masterAddress, cred.Path)
	if raw == nil {
		log.Fatalf("[Vault Client] Failed to retrieve secret from master instance at path %s", cred.Path)
	}
	// vault api returns different payload depending on version
	switch secretEngine {
	case KV_V1:
		if _, exists := raw.Data[cred.Field]; !exists {
			return "", errors.New(fmt.Sprintf("[Vault Client] Field `%s` does not exist at path: `%s`", cred.Field, cred.Path))
		}
		if _, ok := raw.Data[cred.Field].(string); !ok {
			return "", errors.New(fmt.Sprintf("[Vault Client] Field `%s` cannot be converted to string", cred.Field))
		}
		return raw.Data[cred.Field].(string), nil
	case KV_V2:
		mapped, ok := raw.Data["data"].(map[string]interface{})
		if !ok {
			return "", errors.New(fmt.Sprintf("[Vault Client] Failed to process raw result at path: `%s`", cred.Path))
		}
		if len(mapped) < 1 {
			return "", errors.New(fmt.Sprintf("[Vault Client] Data does not exist at path: `%s`", cred.Path))
		}
		if _, exists := mapped[cred.Field]; !exists {
			return "", errors.New(fmt.Sprintf("[Vault Client] Field `%s` does not exist at path: `%s`", cred.Field, cred.Path))
		}
		if _, ok := mapped[cred.Field].(string); !ok {
			return "", errors.New(fmt.Sprintf("[Vault Client] Field `%s` cannot be converted to string", cred.Field))
		}
		return mapped[cred.Field].(string), nil
	default:
		return "", errors.New(fmt.Sprintf("[Vault Client] Unsupported secret engine type %s", secretEngine))
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
			log.WithError(err).WithFields(log.Fields{
				"path":     secretPath,
				"instance": instanceAddr,
			}).Fatalf("[Vault Client] failed to write Vault secret ")
		}
	}
}

// read secret from vault
func ReadSecret(instanceAddr string, secretPath string) *api.Secret {
	secret, err := getClient(instanceAddr).Logical().Read(secretPath)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"path":     secretPath,
			"instance": instanceAddr,
		}).Fatal("[Vault Client] failed to read Vault secret")
	}
	return secret
}

// list secrets
func ListSecrets(instanceAddr string, path string) *api.Secret {
	secretsList, err := getClient(instanceAddr).Logical().List(path)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"path":     path,
			"instance": instanceAddr,
		}).Fatal("[Vault Client] failed to list Vault secrets")
	}
	return secretsList
}

// delete secret from vault
func DeleteSecret(instanceAddr string, secretPath string) {
	_, err := getClient(instanceAddr).Logical().Delete(secretPath)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"path":     secretPath,
			"instance": instanceAddr,
		}).Fatal("[Vault Client] failed to delete Vault secret")
	}
}

// list existing enabled Audits Devices.
func ListAuditDevices(instanceAddr string) map[string]*api.Audit {
	enabledAuditDevices, err := getClient(instanceAddr).Sys().ListAudit()
	if err != nil {
		log.WithError(err).WithField("instance", instanceAddr).Fatal(
			"[Vault Audit] failed to list audit devices")
	}
	return enabledAuditDevices
}

// enable audit device with options
func EnableAduitDevice(instanceAddr string, path string, options *api.EnableAuditOptions) {
	if err := getClient(instanceAddr).Sys().EnableAuditWithOptions(path, options); err != nil {
		log.WithFields(log.Fields{
			"path":     path,
			"instance": instanceAddr,
		}).Fatal("[Vault Audit] failed to enable audit device")
	}
	log.WithFields(log.Fields{
		"path":     path,
		"instance": instanceAddr,
	}).Info("[Vault Audit] audit device is successfully enabled")
}

// disable audit device
func DisableAuditDevice(instanceAddr string, path string) {
	if err := getClient(instanceAddr).Sys().DisableAudit(path); err != nil {
		log.WithField("path", path).Fatal("[Vault Audit] failed to disable audit device")
	}
	log.WithFields(log.Fields{
		"path":     path,
		"instance": instanceAddr,
	}).Info("[Vault Audit] audit device is successfully disabled")
}

// list existing auth backends
func ListAuthBackends(instanceAddr string) map[string]*api.AuthMount {
	existingAuthMounts, err := getClient(instanceAddr).Sys().ListAuth()
	if err != nil {
		log.WithError(err).WithField("instance", instanceAddr).Fatal(
			"[Vault Auth] failed to list auth backends from Vault instance")
	}
	return existingAuthMounts
}

// enable auth backend
func EnableAuthWithOptions(instanceAddr string, path string, options *api.EnableAuthOptions) {
	if err := getClient(instanceAddr).Sys().EnableAuthWithOptions(path, options); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"path":     path,
			"type":     options.Type,
			"instance": instanceAddr,
		}).Fatal("[Vault Auth] failed to enable auth backend")
	}
	log.WithFields(log.Fields{
		"path":     path,
		"type":     options.Type,
		"instance": instanceAddr,
	}).Info("[Vault Auth] successfully enabled auth backend")
}

// disable auth backend
func DisableAuth(instanceAddr string, path string) {
	if err := getClient(instanceAddr).Sys().DisableAuth(path); err != nil {
		log.WithError(err).WithField("path", path).Fatal("[Vault Auth] failed to disable auth backend")
	}
	log.WithField("path", path).WithField("instance", instanceAddr).Info(
		"[Vault Auth] successfully disabled auth backend")
}

// list vault policies
func ListVaultPolicies(instanceAddr string) []string {
	existingPolicyNames, err := getClient(instanceAddr).Sys().ListPolicies()
	if err != nil {
		log.WithError(err).WithField("instance", instanceAddr).Fatal(
			"[Vault Policy] failed to list Vault policies")
	}
	return existingPolicyNames
}

// get vault policy
func GetVaultPolicy(instanceAddr string, name string) string {
	policy, err := getClient(instanceAddr).Sys().GetPolicy(name)
	if err != nil {
		log.WithError(err).WithField("name", name).WithField("instance", instanceAddr).Fatal(
			"[Vault Policy] failed to get existing Vault policy")
	}
	return policy
}

// put vault policy
func PutVaultPolicy(instanceAddr string, name string, rules string) {
	if err := getClient(instanceAddr).Sys().PutPolicy(name, rules); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"name":     name,
			"instance": instanceAddr,
		}).Fatal("[Vault Policy] failed to write policy to Vault instance")
	}
	log.WithFields(log.Fields{
		"name":     name,
		"instance": instanceAddr,
	}).Info("[Vault Policy] policy successfully written to Vault instance")
}

// delete vault policy
func DeleteVaultPolicy(instanceAddr string, name string) {
	if err := getClient(instanceAddr).Sys().DeletePolicy(name); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"name":     name,
			"instance": instanceAddr,
		}).Fatal("[Vault Policy] failed to delete vault policy")
	}
	log.WithFields(log.Fields{
		"name":     name,
		"instance": instanceAddr,
	}).Info("[Vault Policy] successfully deleted policy from Vault instance")

}

// list secrets engines
func ListSecretsEngines(instanceAddr string) map[string]*api.MountOutput {
	existingMounts, err := getClient(instanceAddr).Sys().ListMounts()
	if err != nil {
		log.WithError(err).WithField("instance", instanceAddr).Fatal(
			"[Vault Secrets engine] failed to list Vault secrets engines")
	}
	return existingMounts
}

// enable secrets engine
func EnableSecretsEngine(instanceAddr string, path string, mount *api.MountInput) {
	if err := getClient(instanceAddr).Sys().Mount(path, mount); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"path":     path,
			"type":     mount.Type,
			"instance": instanceAddr,
		}).Fatal("[Vault Secrets engine] failed to enable secrets-engine")
	}
	log.WithFields(log.Fields{
		"path":     path,
		"type":     mount.Type,
		"instance": instanceAddr,
	}).Info("[Vault Secrets engine] successfully enabled secrets-engine")
}

// update secrets engine
func UpdateSecretsEngine(instanceAddr string, path string, config api.MountConfigInput) {
	if err := getClient(instanceAddr).Sys().TuneMount(path, config); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"path":     path,
			"instance": instanceAddr,
		}).Fatal("[Vault Secrets engine] failed to update secrets-engine")
	}
	log.WithFields(log.Fields{
		"path":     path,
		"instance": instanceAddr,
	}).Info("[Vault Secrets engine] successfully updated secrets-engine")
}

// disable secrets engine
func DisableSecretsEngine(instanceAddr string, path string) {
	if err := getClient(instanceAddr).Sys().Unmount(path); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"path":     path,
			"instance": instanceAddr,
		}).Fatal("[Vault Secrets engine] failed to disable secrets-engine")
	}
	log.WithFields(log.Fields{
		"path":     path,
		"instance": instanceAddr,
	}).Info("[Vault Secrets engine] successfully disabled secrets-engine")
}

// GetVaultVersion returns the vault server version
func GetVaultVersion(instanceAddr string) string {
	info, err := getClient(instanceAddr).Sys().Health()
	if err != nil {
		log.WithError(err).WithField("instance", instanceAddr).Fatal(
			"[Vault System] failed to retrieve vault system information")
	}
	return info.Version
}

func ListEntities(instanceAddr string) map[string]interface{} {
	existingEntities, err := getClient(instanceAddr).Logical().List("identity/entity/id")
	if err != nil {
		log.WithError(err).WithField("instance", instanceAddr).Fatal(
			"[Vault Identity] failed to list Vault entities")
	}
	if existingEntities == nil {
		return nil
	}
	return existingEntities.Data
}

func GetEntityInfo(instanceAddr string, name string) map[string]interface{} {
	entity, err := getClient(instanceAddr).Logical().Read(fmt.Sprintf("identity/entity/name/%s", name))
	if err != nil {
		log.WithError(err).WithField("instance", instanceAddr).Fatalf(
			"[Vault Identity] failed to get info for entity: %s", name)
	}
	if entity == nil {
		return nil
	}
	return entity.Data
}

func GetEntityAliasInfo(instanceAddr string, id string) map[string]interface{} {
	entityAlias, err := getClient(instanceAddr).Logical().Read(fmt.Sprintf("identity/entity-alias/id/%s", id))
	if err != nil {
		log.WithError(err).WithField("instance", instanceAddr).Fatalf(
			"[Vault Identity] failed to get info for entity alias: %s", id)
	}
	return entityAlias.Data
}

func WriteEntityAlias(instanceAddr string, secretPath string, secretData map[string]interface{}) {
	_, err := getClient(instanceAddr).Logical().Write(secretPath, secretData)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"path":     secretPath,
			"instance": instanceAddr,
		}).Fatal("[Vault Client] failed to write entity-alias secret")
	}
}

func ListGroups(instanceAddr string) map[string]interface{} {
	existingGroups, err := getClient(instanceAddr).Logical().List("identity/group/id")
	if err != nil {
		log.WithError(err).WithField("instance", instanceAddr).Fatal(
			"[Vault Group] failed to list Vault groups")
	}
	if existingGroups == nil {
		return nil
	}
	return existingGroups.Data
}

func GetGroupInfo(instanceAddr string, name string) map[string]interface{} {
	entity, err := getClient(instanceAddr).Logical().Read(fmt.Sprintf("identity/group/name/%s", name))
	if err != nil {
		log.WithError(err).WithField("instance", instanceAddr).Fatalf(
			"[Vault Group] failed to get info for group: %s", name)
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
