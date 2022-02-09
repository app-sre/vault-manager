// Package auth implements the application of a declarative configuration
// for Vault authentication backends.
package auth

import (
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
	Settings       map[string]map[string]interface{} `yaml:"settings"`
	PolicyMappings []policyMapping                   `yaml:"policy_mappings"`
}

type policyMapping struct {
	GithubTeam map[string]interface{}   `yaml:"github_team"`
	Policies   []map[string]interface{} `yaml:"policies"`
	Type       string                   `yaml:"type"`
	// Description string                   `yaml:"description"`
}

var _ vault.Item = entry{}

var _ vault.Item = policyMapping{}

func (e entry) Key() string {
	return e.Path
}

func (e entry) KeyForType() string {
	return e.Type
}

// func (e entry) KeyForDescription() string {
// 	return e.Description
// }

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

// func (p policyMapping) KeyForDescription() string {
// 	return p.Description
// }

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
	toplevel.RegisterConfiguration("vault_auth_backends", config{})
}

// Apply ensures that an instance of Vault's authentication backends are
// configured exactly as provided.
//
// This function exits the program if an error occurs.
func (c config) Apply(entriesBytes []byte, dryRun bool, threadPoolSize int) {
	// Unmarshal the list of configured auth backends.
	var entries []entry
	if err := yaml.Unmarshal(entriesBytes, &entries); err != nil {
		log.WithError(err).Fatal("[Vault Auth] failed to decode auth backend configuration")
	}

	// Get the existing auth backends
	existingAuthMounts := vault.ListAuthBackends()

	// Build a array of all the existing entries.
	existingBackends := make([]entry, 0)

	if existingAuthMounts != nil {

		var mutex = &sync.Mutex{}

		bwg := utils.NewBoundedWaitGroup(threadPoolSize)

		for path, backend := range existingAuthMounts {
			bwg.Add(1)

			go func(path string, backend *api.AuthMount) {

				mutex.Lock()

				existingBackends = append(existingBackends, entry{
					Path:        path,
					Type:        backend.Type,
					Description: backend.Description,
				})

				defer bwg.Done()
				defer mutex.Unlock()
			}(path, backend)
		}
		bwg.Wait()
	}

	toBeWritten, toBeDeleted, _ := vault.DiffItems(entriesAsItems(entries), entriesAsItems(existingBackends))

	enableAuth(toBeWritten, dryRun)

	configureAuthMounts(entries, dryRun)

	disableAuth(toBeDeleted, dryRun)

	// apply policy mappings
	for _, e := range entries {
		if e.Type == "github" {
			//Build a array of existing policy mappings for current auth mount
			existingPolicyMappings := make([]policyMapping, 0)
			teamsList := vault.ListSecrets(filepath.Join("/auth", e.Path, "map/teams"))
			if teamsList != nil {

				var mutex = &sync.Mutex{}

				teams := teamsList.Data["keys"].([]interface{})

				bwg := utils.NewBoundedWaitGroup(threadPoolSize)

				// fill existing policy mappings array in parallel
				for team := range teams {

					bwg.Add(1)

					go func(team int) {

						policyMappingPath := filepath.Join("/auth/", e.Path, "map/teams", teams[team].(string))

						policiesMappedToEntity := vault.ReadSecret(policyMappingPath).Data["value"].(string)

						policies := make([]map[string]interface{}, 0)

						for _, policy := range strings.Split(policiesMappedToEntity, ",") {
							policies = append(policies, map[string]interface{}{"name": policy})
						}

						mutex.Lock()

						existingPolicyMappings = append(existingPolicyMappings,
							policyMapping{GithubTeam: map[string]interface{}{"team": teams[team]}, Policies: policies})

						defer bwg.Done()
						defer mutex.Unlock()
					}(team)
				}
				bwg.Wait()
			}

			// remove all gh user policy mappings from vault
			usersList := vault.ListSecrets(filepath.Join("/auth", e.Path, "map/users"))
			if usersList != nil {

				users := usersList.Data["keys"].([]interface{})

				bwg := utils.NewBoundedWaitGroup(threadPoolSize)
				// remove existing gh user policy mappings in parallel
				for user := range users {

					bwg.Add(1)

					go func(user int) {

						policyMappingPath := filepath.Join("/auth/", e.Path, "map/users", users[user].(string))

						deletePolicyMapping(policyMappingPath, dryRun)

						defer bwg.Done()

					}(user)
				}
				bwg.Wait()
			}

			policiesMappingsToBeApplied, policiesMappingsToBeDeleted, _ := vault.DiffItems(policyMappingsAsItems(e.PolicyMappings), policyMappingsAsItems(existingPolicyMappings))

			// apply policy mappings
			for _, pm := range policiesMappingsToBeApplied {
				var policies []string
				for _, policy := range pm.(policyMapping).Policies {
					policies = append(policies, policy["name"].(string))
				}
				ghTeamName := pm.(policyMapping).GithubTeam["team"].(string)
				path := filepath.Join("/auth", e.Path, "map/teams", ghTeamName)
				data := map[string]interface{}{"key": ghTeamName, "value": strings.Join(policies, ",")}
				writePolicyMapping(path, data, dryRun)
			}

			// delete policy mappings
			for _, pm := range policiesMappingsToBeDeleted {
				path := filepath.Join("/auth", e.Path, "map/teams", pm.(policyMapping).GithubTeam["team"].(string))
				deletePolicyMapping(path, dryRun)
			}
		}
	}
}

func enableAuth(toBeWritten []vault.Item, dryRun bool) {
	// TODO(riuvshin): implement auth tuning
	for _, e := range toBeWritten {
		ent := e.(entry)
		if dryRun == true {
			log.WithField("path", ent.Path).WithField("type", ent.Type).Info("[Dry Run] [Vault Auth] auth backend to be enabled")
		} else {
			vault.EnableAuthWithOptions(ent.Path, &api.EnableAuthOptions{
				Type:        ent.Type,
				Description: ent.Description,
			})
		}
	}
}

func configureAuthMounts(entries []entry, dryRun bool) {
	// configure auth mounts
	for _, e := range entries {
		if e.Settings != nil {
			for name, cfg := range e.Settings {
				path := filepath.Join("auth", e.Path, name)
				if !vault.DataInSecret(cfg, path) {
					if dryRun == true {
						log.WithField("path", path).WithField("type", e.Type).Info("[Dry Run] [Vault Auth] auth backend configuration to be written")
					} else {
						vault.WriteSecret(path, cfg)
						log.WithField("path", path).WithField("type", e.Type).Info("[Vault Auth] auth backend successfully configured")
					}
				}
			}
		}
	}
}

func disableAuth(toBeDeleted []vault.Item, dryRun bool) {
	for _, e := range toBeDeleted {
		ent := e.(entry)
		if strings.HasPrefix(ent.Path, "token/") {
			continue
		}
		if dryRun == true {
			log.WithField("path", ent.Path).WithField("type", ent.Type).Info("[Dry Run] [Vault Auth] auth backend to be disabled")
		} else {
			vault.DisableAuth(ent.Path)
		}
	}
}

func writePolicyMapping(path string, data map[string]interface{}, dryRun bool) {
	if dryRun == true {
		log.WithField("path", path).WithField("policies", data["value"]).Info("[Dry Run] [Vault Auth] policies mapping to be applied")
	} else {
		vault.WriteSecret(path, data)
		log.WithField("path", path).WithField("policies", data["value"]).Info("[Vault Auth] policies mapping is successfully applied")
	}
}
func entriesAsItems(xs []entry) (items []vault.Item) {
	items = make([]vault.Item, 0)
	for _, x := range xs {
		items = append(items, x)
	}

	return
}

func deletePolicyMapping(path string, dryRun bool) {
	if dryRun == true {
		log.WithField("path", path).Info("[Dry Run] [Vault Auth] policies mapping to be deleted")
	} else {
		vault.DeleteSecret(path)
		log.WithField("path", path).Info("[Vault Auth] policies mapping is successfully deleted")
	}
}

func policyMappingsAsItems(xs []policyMapping) (items []vault.Item) {
	items = make([]vault.Item, 0)
	for _, x := range xs {
		items = append(items, x)
	}

	return
}
