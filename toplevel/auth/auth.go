// Package auth implements the application of a declarative configuration
// for Vault authentication backends.
package auth

import (
	"log"
	"path/filepath"
	"strings"

	"github.com/hashicorp/vault/api"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/app-sre/vault-manager/pkg/vault"
	"github.com/app-sre/vault-manager/toplevel"
)

type entry struct {
	Path           string                            `yaml:"_path"`
	Type           string                            `yaml:"type"`
	Description    string                            `yaml:"description"`
	Settings       map[string]map[string]interface{} `yaml:"settings"`
	PolicyMappings []PolicyMapping                   `yaml:"policy_mappings"`
}

type PolicyMapping struct {
	GithubTeam map[string]interface{}   `yaml:"github_team"`
	Policies   []map[string]interface{} `yaml:"policies"`
}

var _ vault.Item = entry{}

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

func (e entry) enable(client *api.Client) {
	if err := client.Sys().EnableAuthWithOptions(e.Path, &api.EnableAuthOptions{
		Type:        e.Type,
		Description: e.Description,
	}); err != nil {
		logrus.WithError(err).WithField("path", e.Path).Fatal("failed to enable auth backend")
	}
	logrus.WithFields(logrus.Fields{
		"path": e.Path,
		"type": e.Type,
	}).Info("successfully enabled auth backend")
}

func (e entry) disable(client *api.Client) {
	if err := client.Sys().DisableAuth(e.Path); err != nil {
		logrus.WithError(err).WithField("path", e.Path).Fatal("failed to disable auth backend")
	}
	logrus.WithField("path", e.Path).Info("successfully disabled auth backend")
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
		logrus.WithError(err).Fatal("failed to decode authentication backend configuration")
	}

	// Get the existing enabled auth backends.
	existingAuthMounts, err := vault.ClientFromEnv().Sys().ListAuth()
	if err != nil {
		logrus.WithError(err).Fatal("failed to list authentication backends from Vault instance")
	}

	// Build a list of all the existing entries.
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

	toBeWritten, toBeDeleted := vault.DiffItems(asItems(entries), asItems(existingBackends))

	enableAuth(toBeWritten, dryRun)

	configureAuthMounts(entries, dryRun)

	disableAuth(toBeDeleted, dryRun)

	// apply policy mappings
	for _, e := range entries {
		if e.PolicyMappings != nil {
			for _, policyMapping := range e.PolicyMappings {
				var policies []string
				for _, policy := range policyMapping.Policies {
					policies = append(policies, policy["name"].(string))
				}
				path := filepath.Join("/auth", e.Path, "map/teams", policyMapping.GithubTeam["team"].(string))
				data := map[string]interface{}{"key": policyMapping.GithubTeam["team"], "value": strings.Join(policies, ",")}
				writeMapping(path, data, dryRun)
			}
		}
	}
}

func enableAuth(toBeWritten []vault.Item, dryRun bool) {
	// TODO(riuvshin): implement auth tuning
	for _, e := range toBeWritten {
		if dryRun == true {
			logrus.Infof("[Dry Run]\tpackage=auth\tauth to be enabled='%v'", e.(entry))
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
				if !vault.DataInSecret(cfg, path, vault.ClientFromEnv()) {
					if dryRun == true {
						logrus.Infof("[Dry Run]\tpackage=auth\tauth config to be written path='%v' config='%v'", path, e.Settings)
					} else {
						_, err := vault.ClientFromEnv().Logical().Write(path, cfg)
						if err != nil {
							log.Fatal(err)
						}
						logrus.WithField("path", path).WithField("type", e.Type).Info("auth mount successfully configured")
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
			logrus.Infof("[Dry Run]\tpackage=auth\tauth to be disabled='%v'", ent.Path)
		} else {
			ent.disable(vault.ClientFromEnv())
		}
	}
}

func writeMapping(path string, data map[string]interface{}, dryRun bool) {
	if !vault.DataInSecret(data, path, vault.ClientFromEnv()) {
		if dryRun == true {
			logrus.Infof("[Dry Run]\tpackage=auth\tpolicies mapping to be written path='%v' policies='%v'", path, data["value"])
		} else {
			_, err := vault.ClientFromEnv().Logical().Write(path, data)
			if err != nil {
				logrus.Fatal(err)
			}
			logrus.WithField("path", path).WithField("policies", data["value"]).Info("policy mapping is successfully written")
		}
	}
}

func asItems(xs []entry) (items []vault.Item) {
	items = make([]vault.Item, 0)
	for _, x := range xs {
		items = append(items, x)
	}

	return
}
