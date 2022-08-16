// Package vault implements a wrapper around a Vault API client that retrieves
// credentials from the operating system environment.
package vault

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/vault/api"
	log "github.com/sirupsen/logrus"
)

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
		log.WithError(err).WithFields(log.Fields{
			"path":     secretPath,
			"instance": instanceAddr,
		}).Info("[Vault Client] failed to write Vault secret")
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
		log.WithError(err).WithFields(log.Fields{
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
		log.WithError(err).WithFields(log.Fields{
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
func UpdateSecretsEngine(instanceAddr string, path string, config api.MountConfigInput) error {
	if err := getClient(instanceAddr).Sys().TuneMount(path, config); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"path":     path,
			"instance": instanceAddr,
		}).Info("[Vault Secrets engine] failed to update secrets-engine")
		return err
	}
	log.WithFields(log.Fields{
		"path":     path,
		"instance": instanceAddr,
	}).Info("[Vault Secrets engine] successfully updated secrets-engine")
	return nil
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

func ListGroups(instanceAddr string) (map[string]interface{}, error) {
	existingGroups, err := getClient(instanceAddr).Logical().List("identity/group/id")
	if err != nil {
		log.WithError(err).WithField("instance", instanceAddr).Info(
			"[Vault Group] failed to list Vault groups")
		return nil, err
	}
	if existingGroups == nil {
		return nil, nil
	}
	return existingGroups.Data, nil
}

func GetGroupInfo(instanceAddr string, name string) (map[string]interface{}, error) {
	entity, err := getClient(instanceAddr).Logical().Read(fmt.Sprintf("identity/group/name/%s", name))
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"instance": instanceAddr,
			"name":     name,
		}).Info("[Vault Group] failed to get info for group")
		return nil, err
	}
	if entity == nil {
		return nil, nil
	}
	return entity.Data, nil
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
