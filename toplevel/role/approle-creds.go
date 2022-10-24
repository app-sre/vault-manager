package role

import (
	"errors"
	"fmt"
	"strings"

	"github.com/app-sre/vault-manager/pkg/vault"
	log "github.com/sirupsen/logrus"
)

func populateApproleCreds(address string, roles []entry, dryRun bool) error {
	kvVersions, err := getKvEngineVersions(address)
	if err != nil {
		return err
	}

	for _, role := range roles {
		if strings.ToLower(role.Type) == "approle" && len(role.OutputPath) > 0 {
			// assumed output path format: /engine-name/foo/bar
			// root of secret path is name of the secret engine
			pathSegments := strings.Split(role.OutputPath, "/")
			if len(pathSegments) < 2 {
				log.WithFields(log.Fields{
					"name":     role.Name,
					"path":     role.OutputPath,
					"instance": address,
				}).Info("[Vault Approle] Invalid output_path length")
				return errors.New("output_path must contain at least two segments")
			}
			pathRoot := strings.Split(role.OutputPath, "/")[1] // skip root /
			formattedPath := strings.Join(pathSegments[1:], "/")
			fmt.Println("PATH ROOT")
			fmt.Println(fmt.Sprint(pathRoot, "/"))
			fmt.Println(kvVersions)
			if _, exists := kvVersions[fmt.Sprint(pathRoot, "/")]; !exists {
				log.WithFields(log.Fields{
					"name":     role.Name,
					"path":     role.OutputPath,
					"instance": address,
				}).Info("[Vault Approle] Specified output path does not match any existing KV engines")
				return errors.New("approle creds invalid output path")
			}

			// determine if data already exists at desired output path and skip if exists
			// first determine KV version of desired output
			var version string
			switch kvVersions[fmt.Sprint(pathRoot, "/")] {
			case "1":
				version = vault.KV_V1
			case "2":
				version = vault.KV_V2
			default:
				log.WithFields(log.Fields{
					"name":       role.Name,
					"path":       role.OutputPath,
					"kv_version": kvVersions[fmt.Sprint(pathRoot, "/")],
					"instance":   address,
				}).Info("[Vault Approle] Retrieved KV version is not supported")
				return errors.New("approle creds unsupported KV version")
			}
			secret, err := vault.ReadSecret(address, formattedPath, version)
			if err != nil {
				log.WithFields(log.Fields{
					"name":       role.Name,
					"path":       role.OutputPath,
					"kv_version": kvVersions[fmt.Sprint(pathRoot, "/")],
					"instance":   address,
				}).Info("[Vault Approle] Unable to read desired output path")
				return err
			}
			if secret != nil {
				continue
			}

			if dryRun {
				log.WithFields(log.Fields{
					"name":       role.Name,
					"path":       role.OutputPath,
					"kv_version": kvVersions[fmt.Sprint(pathRoot, "/")],
					"instance":   address,
				}).Info("[DRY RUN][Vault Approle] Credentials written to desired path")
			} else {
				creds, err := generatePayload(address, role)
				if err != nil {
					return err
				}
				// write creds to desired output
				err = vault.WriteSecret(address, formattedPath, version, creds)
				if err != nil {
					return err
				}
				log.WithFields(log.Fields{
					"name":       role.Name,
					"path":       role.OutputPath,
					"kv_version": kvVersions[fmt.Sprint(pathRoot, "/")],
					"instance":   address,
				}).Info("[Vault Approle] Credentials written to desired path")
			}
		}
	}
	return nil
}

// Returns map of kv engine names to their kv versions
// KV v1 and v2 require different path formats for rw
func getKvEngineVersions(address string) (map[string]string, error) {
	secretEngines, err := vault.ListSecretsEngines(address)
	if err != nil {
		return nil, err
	}
	kvVersions := make(map[string]string)
	for name, config := range secretEngines {
		if config.Type != "kv" {
			continue
		}

		if v, exists := config.Options["version"]; exists {
			kvVersions[name] = v
		} else {
			log.WithFields(log.Fields{
				"name":     name,
				"instance": address,
			}).Info("Unable to determine KV version")
			continue
		}
	}
	return kvVersions, nil
}

// returns a map containing the role_id, secret_id, and secret_id_accessor for an approle
func generatePayload(address string, role entry) (map[string]interface{}, error) {
	creds := make(map[string]interface{})
	roleSecret, err := vault.ReadSecret(
		address,
		fmt.Sprintf("auth/approle/role/%s/role-id", role.Name),
		vault.KV_V1, // vault internally stored approle data within KV v1
	)
	if err != nil {
		return nil, err
	}
	if _, exists := roleSecret["role_id"]; !exists {
		log.WithFields(log.Fields{
			"name":     role.Name,
			"instance": address,
		}).Info("[Vault Approle] Unable to retrieve role_id")
		return nil, errors.New("role_id retrieval failed")
	}
	creds["role_id"] = roleSecret["role_id"]
	secretIdResult, err := vault.GenerateApproleSecretID(
		address,
		fmt.Sprintf("auth/approle/role/%s/secret-id", role.Name),
	)
	if err != nil {
		return nil, err
	}
	if _, exists := secretIdResult.Data["secret_id"]; !exists {
		log.WithFields(log.Fields{
			"name":     role.Name,
			"instance": address,
		}).Info("[Vault Approle] Unable to retrieve secret_id")
		return nil, errors.New("secret_id retrieval failed")
	}
	creds["secret_id"] = secretIdResult.Data["secret_id"]
	if _, exists := secretIdResult.Data["secret_id_accessor"]; !exists {
		log.WithFields(log.Fields{
			"name":     role.Name,
			"instance": address,
		}).Info("[Vault Approle] Unable to retrieve secret_id_accessor")
		return nil, errors.New("secret_id_accessor retrieval failed")
	}
	creds["secret_id_accessor"] = secretIdResult.Data["secret_id_accessor"]
	return creds, nil
}
