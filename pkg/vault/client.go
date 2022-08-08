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
var InstanceAddresses map[string]bool
var invalidInstances []string

// Called once with toplevel/instance
// Creates global map of all vault clients defined in a-i
// This allows reconciliation of multiple vault instances
func InitClients(instanceCreds map[string]AuthBundle, threadPoolSize int) {
	InstanceAddresses = make(map[string]bool)
	vaultClients = make(map[string]*api.Client)
	configureMaster()

	bwg := utils.NewBoundedWaitGroup(threadPoolSize)
	var mutex = &sync.Mutex{}
	// read access credentials for other vault instances and configure clients
	for addr, bundle := range instanceCreds {
		// client already configured separately for master
		if addr != masterAddress {
			bwg.Add(1)
			go createClient(addr, bundle, &bwg, mutex)
		}
	}
	bwg.Wait()

	// set global for reference by toplevels to determine instances for reconciliation
	for address := range vaultClients {
		InstanceAddresses[address] = true
	}
}

// goroutine support function for InitClients()
// initializes one vault client
func createClient(addr string, bundle AuthBundle, bwg *utils.BoundedWaitGroup, mutex *sync.Mutex) {
	defer bwg.Done()

	accessCreds := make(map[string]string)
	for _, cred := range bundle.VaultSecrets {
		// masterAddress hard-coded because all "child" vault access credentials must be pulled from master
		processedCred, err := GetVaultSecretField(masterAddress, cred.Path, cred.Field, bundle.SecretEngine)
		if err != nil {
			log.WithError(err).Fatal()
		}
		accessCreds[cred.Name] = processedCred
	}

	// Init new client
	config := api.DefaultConfig()
	config.Address = addr
	client, err := api.NewClient(config)
	if err != nil {
		log.WithError(err)
		fmt.Println(fmt.Sprintf("Failed to initialize Vault client for %s", addr))
		fmt.Println(fmt.Sprintf("SKIPPING ALL RECONCILIATION FOR: %s\n", addr))
		return // skip entire reconcilation for this instance
	}

	// at minimum, one element will exist in secrets regardless of type
	// type is same across all VaultSecrets associated with a particular instance address
	var token string
	switch bundle.VaultSecrets[0].Type {
	case APPROLE_AUTH:
		t, err := client.Logical().Write("auth/approle/login", map[string]interface{}{
			"role_id":   accessCreds[ROLE_ID],
			"secret_id": accessCreds[SECRET_ID],
		})
		if err != nil {
			log.WithError(err)
			fmt.Println(fmt.Sprintf("[Vault Client] failed to login to %s with AppRole credentials", addr))
			fmt.Println(fmt.Sprintf("SKIPPING ALL RECONCILIATION FOR: %s\n", addr))
			return // skip entire reconcilation for this instance
		}
		token = t.Auth.ClientToken
	case TOKEN_AUTH:
		token = accessCreds[TOKEN]
	}
	// add new address/client pair to global
	mutex.Lock()
	defer mutex.Unlock()
	client.SetToken(token)
	vaultClients[addr] = client
}

// attempts to read/proccess a single access credential for a particular vault instance
func GetVaultSecretField(instanceAddr, path, field, engineVersion string) (string, error) {
	secret, err := ReadSecret(instanceAddr, path, engineVersion)
	if err != nil {
		return "", err
	}
	if secret == nil {
		return "", errors.New(fmt.Sprintf(
			"[Vault Client] Failed to retrieve secret from %s instance at path %s", instanceAddr, path))
	}
	if _, exists := secret[field]; !exists {
		return "", errors.New(fmt.Sprintf(
			"[Vault Client] Field `%s` does not exist at path: `%s` within %s", field, path, instanceAddr))
	}
	if _, ok := secret[field].(string); !ok {
		return "", errors.New(fmt.Sprintf(
			"[Vault Client] Field `%s` cannot be converted to string", field))
	}
	return secret[field].(string), nil
}

// configureMaster initializes vault client for the master instance
// This is the only client configured using environment variables
// env vars: VAULT_ADDR, VAULT_AUTHTYPE, VAULT_ROLE_ID, VAULT_SECRET_ID, VAULT_TOKEN
func configureMaster() {
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

// AddInvalild is called by toplevel packages when an error is encountered while reconciling
// The invalid instance is appended to a global and then processed within RemoveInstanceFromReconcile
func AddInvalid(instanceAddr string) {
	invalidInstances = append(invalidInstances, instanceAddr)
}

// Removes an instance from the global slice utilized by toplevels to target instances for reconciliation
func RemoveInstanceFromReconciliation() {
	for _, invalid := range invalidInstances {
		if _, exists := InstanceAddresses[invalid]; exists {
			delete(InstanceAddresses, invalid)
			fmt.Println(fmt.Sprintf("SKIPPING REMAINING RECONCILIATION FOR %s", invalid))
		}
	}
	// clear invalid
	invalidInstances = nil
}

// return proper secret path format based upon kv version
// kv v2 api inserts /data/ between the root engine name and remaining path
func FormatSecretPath(secret string, secretEngine string) string {
	if secretEngine == KV_V2 {
		sliced := strings.SplitN(secret, "/", 2)
		if len(sliced) < 2 {
			log.Fatal("[Vault Instance] Error processessing kv_v2 secret path")
		}
		return fmt.Sprintf("%s/data/%s", sliced[0], sliced[1])
	} else {
		return secret
	}
}

// write secret to vault
func WriteSecret(instanceAddr string, secretPath string, secretData map[string]interface{}) error {
	dataExists, err := DataInSecret(instanceAddr, secretData, secretPath)
	if err != nil {
		return err
	}
	if !dataExists {
		_, err := getClient(instanceAddr).Logical().Write(secretPath, secretData)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"path":     secretPath,
				"instance": instanceAddr,
			}).Info("[Vault Client] failed to write Vault secret")
			return errors.New("failed to write secret")
		}
	}
	return nil
}

// read secret from vault and return the secret map
func ReadSecret(instanceAddr, secretPath, engineVersion string) (map[string]interface{}, error) {
	// vault manager does not support reverting and should always reference latest data within a-i
	// therefore, secret version is not specified for KV V2 secrets
	raw, err := getClient(instanceAddr).Logical().Read(secretPath)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"path":          secretPath,
			"instance":      instanceAddr,
			"engineVersion": engineVersion,
		}).Fatal("[Vault Client] failed to read Vault secret")
	}
	if raw == nil {
		return nil, nil
	}
	// vault api returns different payload depending on version
	switch engineVersion {
	case KV_V1:
		return raw.Data, nil
	case KV_V2:
		if len(raw.Data) == 0 {
			return nil, nil
		}
		mapped, ok := raw.Data["data"].(map[string]interface{})
		if !ok {
			log.WithError(err).WithFields(log.Fields{
				"path":          secretPath,
				"instance":      instanceAddr,
				"engineVersion": engineVersion,
			}).Info("[Vault Client] failed to process `data` from result of read")
			return nil, errors.New("failed to convert `data` to map")
		}
		return mapped, nil
	default:
		log.WithError(err).WithFields(log.Fields{
			"path":          secretPath,
			"instance":      instanceAddr,
			"engineVersion": engineVersion,
		}).Info("[Vault Client] unsupported KV engine version passed to ReadSecret()")
		return nil, errors.New("unsupported engine version specified")
	}
}

// list secrets
func ListSecrets(instanceAddr string, path string) (*api.Secret, error) {
	secretsList, err := getClient(instanceAddr).Logical().List(path)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"path":     path,
			"instance": instanceAddr,
		}).Info("[Vault Client] failed to list Vault secrets")
		return nil, errors.New("failed to list secrets")
	}
	return secretsList, nil
}

// delete secret from vault
func DeleteSecret(instanceAddr string, secretPath string) error {
	_, err := getClient(instanceAddr).Logical().Delete(secretPath)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"path":     secretPath,
			"instance": instanceAddr,
		}).Info("[Vault Client] failed to delete Vault secret")
		return errors.New("failed to delete secret")
	}
	return nil
}

// list existing enabled Audits Devices.
func ListAuditDevices(instanceAddr string) (map[string]*api.Audit, error) {
	enabledAuditDevices, err := getClient(instanceAddr).Sys().ListAudit()
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"instance": instanceAddr,
		}).Info("[Vault Audit] failed to list audit devices")
		return nil, errors.New("failed to list audit devices")
	}
	return enabledAuditDevices, nil
}

// enable audit device with options
func EnableAuditDevice(instanceAddr, path string, options *api.EnableAuditOptions) error {
	if err := getClient(instanceAddr).Sys().EnableAuditWithOptions(path, options); err != nil {
		log.WithFields(log.Fields{
			"path":     path,
			"instance": instanceAddr,
		}).Info("[Vault Audit] failed to enable audit device")
		return errors.New("failed to enable audit device")
	}
	log.WithFields(log.Fields{
		"path":     path,
		"instance": instanceAddr,
	}).Info("[Vault Audit] audit device is successfully enabled")
	return nil
}

// disable audit device
func DisableAuditDevice(instanceAddr string, path string) error {
	if err := getClient(instanceAddr).Sys().DisableAudit(path); err != nil {
		log.WithFields(log.Fields{
			"path":     path,
			"instance": instanceAddr,
		}).Info("[Vault Audit] failed to disable audit device")
		return errors.New("failed to disable audit device")
	}
	log.WithFields(log.Fields{
		"path":     path,
		"instance": instanceAddr,
	}).Info("[Vault Audit] audit device is successfully disabled")
	return nil
}

// list existing auth backends
func ListAuthBackends(instanceAddr string) (map[string]*api.AuthMount, error) {
	existingAuthMounts, err := getClient(instanceAddr).Sys().ListAuth()
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"instance": instanceAddr,
		}).Info("[Vault Auth] failed to list auth backends")
		return nil, errors.New("failed to list auth backends")
	}
	return existingAuthMounts, nil
}

// enable auth backend
func EnableAuthWithOptions(instanceAddr string, path string, options *api.EnableAuthOptions) error {
	if err := getClient(instanceAddr).Sys().EnableAuthWithOptions(path, options); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"path":     path,
			"type":     options.Type,
			"instance": instanceAddr,
		}).Info("[Vault Auth] failed to enable auth backend")
		return errors.New("failed to enable auth backend")
	}
	log.WithFields(log.Fields{
		"path":     path,
		"type":     options.Type,
		"instance": instanceAddr,
	}).Info("[Vault Auth] successfully enabled auth backend")
	return nil
}

// disable auth backend
func DisableAuth(instanceAddr string, path string) error {
	if err := getClient(instanceAddr).Sys().DisableAuth(path); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"path":     path,
			"instance": instanceAddr,
		}).Info("[Vault Auth] failed to disable auth backend")
		return errors.New("failed to disable auth backend")
	}
	log.WithFields(log.Fields{
		"path":     path,
		"instance": instanceAddr,
	}).Info("[Vault Auth] successfully disabled auth backend")
	return nil
}

// returns a list of existing policy names for a specific instance
func ListVaultPolicies(instanceAddr string) ([]string, error) {
	existingPolicyNames, err := getClient(instanceAddr).Sys().ListPolicies()
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"instance": instanceAddr,
		}).Info("[Vault Policy] failed to list existing policies")
		return nil, errors.New("[Vault Policy] failed to list existing policies")
	}
	return existingPolicyNames, nil
}

// get vault policy name
func GetVaultPolicy(instanceAddr string, name string) (string, error) {
	policy, err := getClient(instanceAddr).Sys().GetPolicy(name)
	if err != nil {
		log.WithError(err).WithFields(
			log.Fields{
				"name":     name,
				"instance": instanceAddr,
			}).Info("[Vault Policy] failed to get existing Vault policy")
		return "", err
	}
	return policy, nil
}

// put vault policy
func PutVaultPolicy(instanceAddr string, name string, rules string) error {
	if err := getClient(instanceAddr).Sys().PutPolicy(name, rules); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"name":     name,
			"instance": instanceAddr,
		}).Info("[Vault Policy] failed to write policy to Vault instance")
		return err
	}
	log.WithFields(log.Fields{
		"name":     name,
		"instance": instanceAddr,
	}).Info("[Vault Policy] policy successfully written to Vault instance")
	return nil
}

// delete vault policy
func DeleteVaultPolicy(instanceAddr string, name string) error {
	if err := getClient(instanceAddr).Sys().DeletePolicy(name); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"name":     name,
			"instance": instanceAddr,
		}).Info("[Vault Policy] failed to delete vault policy")
		return err
	}
	log.WithFields(log.Fields{
		"name":     name,
		"instance": instanceAddr,
	}).Info("[Vault Policy] successfully deleted policy from Vault instance")
	return nil
}

// return secret engines
func ListSecretsEngines(instanceAddr string) (map[string]*api.MountOutput, error) {
	existingMounts, err := getClient(instanceAddr).Sys().ListMounts()
	if err != nil {
		log.WithError(err).WithField("instance", instanceAddr).Info(
			"[Vault Secrets engine] failed to list Vault secrets engines")
		return nil, err
	}
	return existingMounts, nil
}

// enable secrets engine
func EnableSecretsEngine(instanceAddr string, path string, mount *api.MountInput) error {
	if err := getClient(instanceAddr).Sys().Mount(path, mount); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"path":     path,
			"type":     mount.Type,
			"instance": instanceAddr,
		}).Info("[Vault Secrets engine] failed to enable secrets-engine")
		return err
	}
	log.WithFields(log.Fields{
		"path":     path,
		"type":     mount.Type,
		"instance": instanceAddr,
	}).Info("[Vault Secrets engine] successfully enabled secrets-engine")
	return nil
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
func DisableSecretsEngine(instanceAddr string, path string) error {
	if err := getClient(instanceAddr).Sys().Unmount(path); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"path":     path,
			"instance": instanceAddr,
		}).Info("[Vault Secrets engine] failed to disable secrets-engine")
		return err
	}
	log.WithFields(log.Fields{
		"path":     path,
		"instance": instanceAddr,
	}).Info("[Vault Secrets engine] successfully disabled secrets-engine")
	return nil
}

// GetVaultVersion returns the vault server version
func GetVaultVersion(instanceAddr string) (string, error) {
	info, err := getClient(instanceAddr).Sys().Health()
	if err != nil {
		log.WithError(err).WithField("instance", instanceAddr).Info(
			"[Vault System] failed to retrieve vault system information")
		return "", err
	}
	return info.Version, nil
}

func ListEntities(instanceAddr string) (map[string]interface{}, error) {
	existingEntities, err := getClient(instanceAddr).Logical().List("identity/entity/id")
	if err != nil {
		log.WithError(err).WithField("instance", instanceAddr).Info(
			"[Vault Identity] failed to list Vault entities")
	}
	if existingEntities == nil {
		return nil, err
	}
	return existingEntities.Data, nil
}

func GetEntityInfo(instanceAddr string, name string) (map[string]interface{}, error) {
	entity, err := getClient(instanceAddr).Logical().Read(fmt.Sprintf("identity/entity/name/%s", name))
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"instance": instanceAddr,
			"name":     name,
		}).Info("[Vault Identity] failed to get entity info")
		return nil, err
	}
	if entity == nil {
		return nil, nil
	}
	return entity.Data, nil
}

func GetEntityAliasInfo(instanceAddr string, id string) (map[string]interface{}, error) {
	entityAlias, err := getClient(instanceAddr).Logical().Read(fmt.Sprintf("identity/entity-alias/id/%s", id))
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"instance": instanceAddr,
			"id":       id,
		}).Info("[Vault Identity] failed to get info for entity alias")
		return nil, err
	}
	return entityAlias.Data, nil
}

func WriteEntityAlias(instanceAddr string, secretPath string, secretData map[string]interface{}) error {
	_, err := getClient(instanceAddr).Logical().Write(secretPath, secretData)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"path":     secretPath,
			"instance": instanceAddr,
		}).Info("[Vault Client] failed to write entity-alias secret")
		return err
	}
	return nil
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
