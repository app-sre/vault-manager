// Package auth implements the application of a declarative configuration
// for Vault authentication backends.
package auth

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/app-sre/vault-manager/pkg/utils"
	"github.com/app-sre/vault-manager/pkg/vault"
	"github.com/app-sre/vault-manager/toplevel"
	"github.com/hashicorp/vault/api"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type entry struct {
	Path           string                            `yaml:"_path"`
	Type           string                            `yaml:"type"`
	Description    string                            `yaml:"description"`
	Instance       vault.Instance                    `yaml:"instance"`
	Settings       map[string]map[string]interface{} `yaml:"settings"`
	PolicyMappings []policyMapping                   `yaml:"policy_mappings"`
}

type policyMapping struct {
	GithubTeam  map[string]interface{}   `yaml:"github_team"`
	Policies    []map[string]interface{} `yaml:"policies"`
	Type        string                   `yaml:"type"`
	Description string                   `yaml:"description"`
}

const toplevelName = "vault_auth_backends"

var _ vault.Item = entry{}

var _ vault.Item = policyMapping{}

func (e entry) Key() string {
	return e.Path
}

func (e entry) KeyForType() string {
	return e.Type
}

func (e entry) KeyForDescription() string {
	return e.Description
}

func (e entry) Equals(i interface{}) bool {
	entry, ok := i.(entry)
	if !ok {
		return false
	}

	return vault.EqualPathNames(e.Path, entry.Path) &&
		e.Type == entry.Type
}

func (p policyMapping) Key() string {
	return p.GithubTeam["team"].(string)
}

func (p policyMapping) KeyForType() string {
	return p.Type
}

func (p policyMapping) KeyForDescription() string {
	return p.Description
}

func (p policyMapping) Equals(i interface{}) bool {
	policyMapping, ok := i.(policyMapping)
	if !ok {
		return false
	}
	return p.GithubTeam["team"] == policyMapping.GithubTeam["team"] &&
		comparePolicies(p.Policies, policyMapping.Policies)
}

func comparePolicies(xpolicies, ypolicies []map[string]interface{}) bool {
	if len(xpolicies) != len(ypolicies) {
		return false
	}

	for i, xp := range xpolicies {
		if xp["name"].(string) != ypolicies[i]["name"].(string) {
			return false
		}
	}
	return true
}

type config struct{}

var _ toplevel.Configuration = config{}

func init() {
	toplevel.RegisterConfiguration(toplevelName, config{})
}

// Apply ensures that an instance of Vault's authentication backends are
// configured exactly as provided.
func (c config) Apply(address string, entriesBytes []byte, dryRun bool, threadPoolSize int) error {
	// Unmarshal the list of configured auth backends.
	var entries []entry
	if err := yaml.Unmarshal(entriesBytes, &entries); err != nil {
		log.WithError(err).Fatal("[Vault Auth] failed to decode auth backend configuration")
	}
	// organize by instance
	instancesToDesired := make(map[string][]entry)
	for _, e := range entries {
		instancesToDesired[e.Instance.Address] = append(instancesToDesired[e.Instance.Address], e)
	}
	updateOptionalKubeDefaults(instancesToDesired[address])

	desiredItems := asItems(instancesToDesired[address])
	if unique := vault.UniqueKeys(desiredItems, toplevelName); !unique {
		return fmt.Errorf("Duplicate key value detected within %s", toplevelName)
	}

	// Get the existing auth backends
	existingAuthMounts, err := vault.ListAuthBackends(address)
	if err != nil {
		return err
	}

	// Build a array of all the existing entries.
	existingBackends := make([]entry, 0)

	if existingAuthMounts != nil {
		for path, backend := range existingAuthMounts {
			existingBackends = append(existingBackends, entry{
				Path:        path,
				Type:        backend.Type,
				Description: backend.Description,
				Instance:    vault.Instance{Address: address},
			})
		}
	}

	// perform auth reconcile
	toBeWritten, toBeDeleted, _ :=
		vault.DiffItems(desiredItems, asItems(existingBackends))
	err = enableAuth(address, toBeWritten, dryRun)
	if err != nil {
		return err
	}
	err = configureAuthMounts(address, instancesToDesired[address], dryRun)
	if err != nil {
		return err
	}
	err = disableAuth(address, toBeDeleted, dryRun)
	if err != nil {
		return err
	}

	// apply github policy mappings
	for _, e := range instancesToDesired[address] {
		if e.Type == "github" {
			//Build a array of existing policy mappings for current auth mount
			existingPolicyMappings := make([]policyMapping, 0)
			teamsList, err := vault.ListSecrets(address, filepath.Join("/auth", e.Path, "map/teams"))
			if err != nil {
				return err
			}
			if teamsList != nil {

				var mutex = &sync.Mutex{}

				teams := teamsList.Data["keys"].([]interface{})

				bwg := utils.NewBoundedWaitGroup(threadPoolSize)
				ch := make(chan error)

				// fill existing policy mappings array in parallel
				for team := range teams {
					bwg.Add(1)

					go func(team int, ch chan<- error) {
						defer bwg.Done()

						policyMappingPath := filepath.Join("/auth/", e.Path, "map/teams", teams[team].(string))

						policiesMappedToEntity, err := vault.ReadSecret(address, policyMappingPath, vault.KV_V1)
						if err != nil {
							ch <- err
							return
						}

						policies := make([]map[string]interface{}, 0)
						for _, policy := range strings.Split(policiesMappedToEntity["value"].(string), ",") {
							policies = append(policies, map[string]interface{}{"name": policy})
						}

						mutex.Lock()

						existingPolicyMappings = append(existingPolicyMappings,
							policyMapping{GithubTeam: map[string]interface{}{"team": teams[team]}, Policies: policies})

						defer mutex.Unlock()
					}(team, ch)
				}

				go func() {
					bwg.Wait()
					close(ch)
				}()

				for e := range ch {
					if e != nil {
						return e
					}
				}
			}

			// remove all gh user policy mappings from vault
			usersList, err := vault.ListSecrets(address, filepath.Join("/auth", e.Path, "map/users"))
			if usersList != nil {

				users := usersList.Data["keys"].([]interface{})

				bwg := utils.NewBoundedWaitGroup(threadPoolSize)
				// remove existing gh user policy mappings in parallel
				for user := range users {

					bwg.Add(1)

					go func(user int) {

						policyMappingPath := filepath.Join("/auth/", e.Path, "map/users", users[user].(string))

						deletePolicyMapping(address, policyMappingPath, dryRun)

						defer bwg.Done()

					}(user)
				}
				bwg.Wait()
			}

			policiesMappingsToBeApplied, policiesMappingsToBeDeleted, _ :=
				vault.DiffItems(policyMappingsAsItems(e.PolicyMappings), policyMappingsAsItems(existingPolicyMappings))

			// apply policy mappings
			for _, pm := range policiesMappingsToBeApplied {
				var policies []string
				for _, policy := range pm.(policyMapping).Policies {
					policies = append(policies, policy["name"].(string))
				}
				ghTeamName := pm.(policyMapping).GithubTeam["team"].(string)
				path := filepath.Join("/auth", e.Path, "map/teams", ghTeamName)
				data := map[string]interface{}{"key": ghTeamName, "value": strings.Join(policies, ",")}
				writePolicyMapping(address, path, data, dryRun)
			}

			// delete policy mappings
			for _, pm := range policiesMappingsToBeDeleted {
				path := filepath.Join("/auth", e.Path, "map/teams", pm.(policyMapping).GithubTeam["team"].(string))
				deletePolicyMapping(address, path, dryRun)
			}
		}
	}

	return nil
}

// updateOptionalKubeDefaults maps omitted optional attributes from desired to default values in existing
// this circumvents defining every attribute within kube auth mount definitions
func updateOptionalKubeDefaults(desired []entry) {
	defaults := map[string]interface{}{
		"disable_local_ca_jwt": false,
		"kubernetes_ca_cert":   "",
	}
	for _, auth := range desired {
		if strings.ToLower(auth.Type) == "kubernetes" {
			for _, cfg := range auth.Settings {
				for k, v := range defaults {
					// denotes that attr was not included in definition and graphql assigned nil
					// proceed with assigning default value that api would assign if attribute was omitted
					if cfg[k] == nil {
						cfg[k] = v
					}
				}
			}
		}
	}
}

func enableAuth(instanceAddr string, toBeWritten []vault.Item, dryRun bool) error {
	// TODO(riuvshin): implement auth tuning
	for _, e := range toBeWritten {
		ent := e.(entry)
		if dryRun == true {
			log.WithFields(log.Fields{
				"path":     ent.Path,
				"type":     ent.Type,
				"instance": instanceAddr,
			}).Info("[Dry Run] [Vault Auth] auth backend to be enabled")
		} else {
			err := vault.EnableAuthWithOptions(instanceAddr, ent.Path,
				&api.EnableAuthOptions{
					Type:        ent.Type,
					Description: ent.Description,
				})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func configureAuthMounts(instanceAddr string, entries []entry, dryRun bool) error {
	// configure auth mounts
	for _, e := range entries {
		if e.Settings != nil {
			if e.Type == "oidc" {
				err := setOidcClientSecret(instanceAddr, e.Settings)
				if err != nil {
					return err
				}
			} else if e.Type == "kubernetes" {
				err := setKubeCaCert(instanceAddr, e.Settings)
				if err != nil {
					return err
				}
			}
			for name, cfg := range e.Settings {
				path := filepath.Join("auth", e.Path, name)
				dataExists, err := vault.DataInSecret(instanceAddr, cfg, path, vault.KV_V1)
				if err != nil {
					return err
				}
				if !dataExists {
					if dryRun == true {
						log.WithField("path", path).WithField("type", e.Type).WithField("instance", instanceAddr).Info(
							"[Dry Run] [Vault Auth] auth backend configuration to be written")
					} else {
						err := vault.WriteSecret(instanceAddr, path, vault.KV_V1, cfg)
						if err != nil {
							return err
						}
						log.WithField("path", path).WithField("type", e.Type).WithField("instance", instanceAddr).Info(
							"[Vault Auth] auth backend successfully configured")
					}
				}
			}
		}
	}
	return nil
}

func disableAuth(instanceAddr string, toBeDeleted []vault.Item, dryRun bool) error {
	for _, e := range toBeDeleted {
		ent := e.(entry)
		if strings.HasPrefix(ent.Path, "token/") {
			continue
		}
		if dryRun == true {
			log.WithField("path", ent.Path).WithField("type", ent.Type).WithField("instance", instanceAddr).Info(
				"[Dry Run] [Vault Auth] auth backend to be disabled")
		} else {
			err := vault.DisableAuth(instanceAddr, ent.Path)
			if err != nil {
				return err
			}
			log.WithField("path", ent.Path).WithField("type", ent.Type).WithField("instance", instanceAddr).Info(
				"[Vault Auth] auth backend disabled")
		}
	}
	return nil
}

func writePolicyMapping(instanceAddr string, path string, data map[string]interface{}, dryRun bool) error {
	if dryRun == true {
		log.WithField("path", path).WithField("policies", data["value"]).WithField("instance", instanceAddr).Info(
			"[Dry Run] [Vault Auth] policies mapping to be applied")
	} else {
		err := vault.WriteSecret(instanceAddr, path, vault.KV_V1, data)
		if err != nil {
			return err
		}
		log.WithField("path", path).WithField("policies", data["value"]).WithField("instance", instanceAddr).Info(
			"[Vault Auth] policies mapping is successfully applied")
	}
	return nil
}

func asItems(xs []entry) (items []vault.Item) {
	items = make([]vault.Item, 0)
	for _, x := range xs {
		items = append(items, x)
	}
	return items
}

func deletePolicyMapping(instanceAddr string, path string, dryRun bool) {
	if dryRun == true {
		log.WithField("path", path).WithField("instance", instanceAddr).Info(
			"[Dry Run] [Vault Auth] policies mapping to be deleted")
	} else {
		vault.DeleteSecret(instanceAddr, path)
		log.WithField("path", path).WithField("instance", instanceAddr).Info(
			"[Vault Auth] policies mapping is successfully deleted")
	}
}

func policyMappingsAsItems(xs []policyMapping) (items []vault.Item) {
	items = make([]vault.Item, 0)
	for _, x := range xs {
		items = append(items, x)
	}
	return items
}

// retrieves client secret at vault location specified in oidc auth definition
// and overwrites oidc_client_secret within desired object's settings
func setOidcClientSecret(instanceAddr string, settings map[string]map[string]interface{}) error {
	// logic to check existence of keys before referencing is unnecessary due to schema validation
	cfg := settings["config"]
	engineVersion := cfg[vault.OIDC_CLIENT_SECRET_KV_VER].(string)
	location := cfg[vault.OIDC_CLIENT_SECRET].(map[interface{}]interface{})
	path := location["path"].(string)
	field := location["field"].(string)
	secret, err := vault.GetVaultSecretField(instanceAddr, path, field, engineVersion)
	if err != nil {
		return errors.New(fmt.Sprintf(
			"[Vault Auth] failed to retrieve `oidc_client_secret` for %s", instanceAddr))
	}
	cfg[vault.OIDC_CLIENT_SECRET] = secret
	delete(cfg, vault.OIDC_CLIENT_SECRET_KV_VER) // only used to obtain secret. do not include in reconcile
	return nil
}

// retrieves client secret from vault location specified in kubernetes auth definition
// and overwrites kubernetes_ca_cert within desired object's settings
func setKubeCaCert(instanceAddr string, settings map[string]map[string]interface{}) error {
	cfg := settings["config"]
	// ca cert is optional within kube auth config
	// if omitted from definition, proceeding assertion will fail
	location, ok := cfg[vault.KUBERNETES_CA_CERT].(map[interface{}]interface{})
	if !ok {
		return nil
	}
	engineVersion, exists := cfg[vault.KUBERNETES_CA_CERT_KV_VER].(string)
	// default to v2 when not specified
	if !exists {
		engineVersion = vault.KV_V2
	}
	path := location["path"].(string)
	field := location["field"].(string)
	cert, err := vault.GetVaultSecretField(instanceAddr, path, field, engineVersion)
	if err != nil {
		return errors.New(fmt.Sprintf(
			"[Vault Auth] failed to retrieve `kubernetes_ca_cert` for %s", instanceAddr))
	}
	cfg[vault.KUBERNETES_CA_CERT] = cert
	delete(cfg, vault.KUBERNETES_CA_CERT_KV_VER) // only used to obtain secret. do not include in reconcile
	return nil
}
