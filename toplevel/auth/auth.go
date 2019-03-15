// Package auth implements the application of a declarative configuration
// for Vault authentication backends.
package auth

import (
	"path/filepath"
	"strings"

	"github.com/hashicorp/vault/api"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/app-sre/vault-manager/pkg/vault"
	"github.com/app-sre/vault-manager/toplevel"
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
}

var _ vault.Item = entry{}

var _ vault.Item = policyMapping{}

func (e entry) Key() string {
	return e.Path
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

func (e entry) enable(client *api.Client) {
	if err := client.Sys().EnableAuthWithOptions(e.Path, &api.EnableAuthOptions{
		Type:        e.Type,
		Description: e.Description,
	}); err != nil {
		log.WithField("package", "auth").WithError(err).WithField("path", e.Path).WithField("type", e.Type).Fatal("failed to enable auth backend")
	}
	log.WithField("package", "auth").WithFields(log.Fields{
		"path": e.Path,
		"type": e.Type,
	}).Info("successfully enabled auth backend")
}

func (e entry) disable(client *api.Client) {
	if err := client.Sys().DisableAuth(e.Path); err != nil {
		log.WithField("package", "auth").WithError(err).WithField("path", e.Path).WithField("type", e.Type).Fatal("failed to disable auth backend")
	}
	log.WithField("package", "auth").WithField("path", e.Path).WithField("type", e.Type).Info("successfully disabled auth backend")
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
func (c config) Apply(entriesBytes []byte, dryRun bool) {
	// Unmarshal the list of configured auth backends.
	var entries []entry
	if err := yaml.Unmarshal(entriesBytes, &entries); err != nil {
		log.WithField("package", "auth").WithError(err).Fatal("failed to decode auth backend configuration")
	}

	// Get the existing enabled auth backends.
	existingAuthMounts, err := vault.ClientFromEnv().Sys().ListAuth()
	if err != nil {
		log.WithField("package", "auth").WithError(err).Fatal("failed to list auth backends from Vault instance")
	}

	// Build a array of all the existing entries.
	existingBackends := make([]entry, 0)
	if existingAuthMounts != nil {
		for path, backend := range existingAuthMounts {
			existingBackends = append(existingBackends, entry{
				Path:        path,
				Type:        backend.Type,
				Description: backend.Description,
			})
		}
	}

	toBeWritten, toBeDeleted := vault.DiffItems(entriesAsItems(entries), entriesAsItems(existingBackends))

	enableAuth(toBeWritten, dryRun)

	configureAuthMounts(entries, dryRun)

	disableAuth(toBeDeleted, dryRun)

	// apply policy mappings
	for _, e := range entries {
		if e.Type == "github" {
			//Build a array of existing policy mappings for current auth mount
			existingPolicyMappings := make([]policyMapping, 0)
			entitiesList := vault.ListSecrets(filepath.Join("/auth", e.Path, "map/teams"))
			if entitiesList != nil {
				for _, entity := range entitiesList.Data["keys"].([]interface{}) {
					policyMappingPath := filepath.Join("/auth/", e.Path, "map/teams", entity.(string))
					policiesMappedToEntity := vault.ReadSecret(policyMappingPath).Data["value"].(string)
					policies := make([]map[string]interface{}, 0)
					for _, policy := range strings.Split(policiesMappedToEntity, ",") {
						policies = append(policies, map[string]interface{}{"name": policy})
					}
					existingPolicyMappings = append(existingPolicyMappings,
						policyMapping{GithubTeam: map[string]interface{}{"team": entity}, Policies: policies})
				}
			}

			policiesMappingsToBeApplied, policiesMappingsToBeDeleted := vault.DiffItems(policyMappingsAsItems(e.PolicyMappings), policyMappingsAsItems(existingPolicyMappings))
			// apply policy mappings
			for _, pm := range policiesMappingsToBeApplied {
				var policies []string
				for _, policy := range pm.(policyMapping).Policies {
					policies = append(policies, policy["name"].(string))
				}
				ghTeamName := pm.(policyMapping).GithubTeam["team"].(string)
				path := filepath.Join("/auth", e.Path, "map/teams", ghTeamName)
				data := map[string]interface{}{"key": ghTeamName, "value": strings.Join(policies, ",")}
				writeMapping(path, data, dryRun)
			}

			// delete policy mappings
			for _, pm := range policiesMappingsToBeDeleted {
				path := filepath.Join("/auth", e.Path, "map/teams", pm.(policyMapping).GithubTeam["team"].(string))
				deleteMapping(path, dryRun)
			}
		}
	}
}

func enableAuth(toBeWritten []vault.Item, dryRun bool) {
	// TODO(riuvshin): implement auth tuning
	for _, e := range toBeWritten {
		if dryRun == true {
			log.WithField("package", "auth").WithField("path", e.(entry).Path).WithField("type", e.(entry).Type).Info("[Dry Run] auth backend to be enabled")
		} else {
			e.(entry).enable(vault.ClientFromEnv())
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
						log.WithField("package", "auth").WithField("path", path).WithField("type", e.Type).Info("[Dry Run] auth backend configuration to be written")
					} else {
						vault.WriteSecret(path, cfg)
						log.WithField("package", "auth").WithField("path", path).WithField("type", e.Type).Info("auth backend successfully configured")
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
			log.WithField("package", "auth").WithField("path", ent.Path).WithField("type", ent.Type).Info("[Dry Run] auth backend to be disabled")
		} else {
			ent.disable(vault.ClientFromEnv())
		}
	}
}

func writeMapping(path string, data map[string]interface{}, dryRun bool) {
	if !vault.DataInSecret(data, path) {
		if dryRun == true {
			log.WithField("package", "auth").WithField("path", path).WithField("policies", data["value"]).Info("[Dry Run] policies mapping to be applied")
		} else {
			vault.WriteSecret(path, data)
			log.WithField("package", "auth").WithField("path", path).WithField("policies", data["value"]).Info("policies mapping is successfully applied")
		}
	}
}

func deleteMapping(path string, dryRun bool) {
	if dryRun == true {
		log.WithField("package", "auth").WithField("path", path).Info("[Dry Run] policies mapping to be deleted")
	} else {
		vault.DeleteSecret(path)
		log.WithField("package", "auth").WithField("path", path).Info("policies mapping is successfully deleted")
	}
}
func entriesAsItems(xs []entry) (items []vault.Item) {
	items = make([]vault.Item, 0)
	for _, x := range xs {
		items = append(items, x)
	}

	return
}

func policyMappingsAsItems(xs []policyMapping) (items []vault.Item) {
	items = make([]vault.Item, 0)
	for _, x := range xs {
		items = append(items, x)
	}

	return
}
