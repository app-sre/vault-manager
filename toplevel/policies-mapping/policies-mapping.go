package policies_mapping

import (
	"github.com/app-sre/vault-manager/pkg/vault"
	"github.com/hashicorp/vault/api"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"path/filepath"

	"github.com/app-sre/vault-manager/toplevel"
)

type config struct{}

var _ toplevel.Configuration = config{}

func init() {
	toplevel.RegisterConfiguration("policies-mapping", config{})
}

type entry struct {
	EntityName  string `yaml:"entity-name"`
	EntityGroup string `yaml:"entity-group"`
	AuthMount   string `yaml:"auth-mount"`
	Policies    string `yaml:"policies"`
}

var _ vault.Item = entry{}

func (e entry) Key() string {
	path := filepath.Join("/auth", e.AuthMount, e.EntityGroup, e.EntityName)
	return path
}

func (e entry) Equals(i interface{}) bool {
	entry, ok := i.(entry)
	if !ok {
		return false
	}
	ePpath := filepath.Join("/auth", e.AuthMount, e.EntityGroup, e.EntityName)
	entryPpath := filepath.Join("/auth", entry.AuthMount, entry.EntityGroup, entry.EntityName)
	return ePpath == entryPpath && e.Policies == entry.Policies
}

func (c config) Apply(entriesBytes []byte, dryRun bool) {
	// Unmarshal the list of configured secrets engines.
	var entries []entry
	if err := yaml.Unmarshal(entriesBytes, &entries); err != nil {
		logrus.WithError(err).Fatal("failed to decode policies mapping configuration")
	}

	// Get existing enabled auth backends.
	existingAuthMounts, err := vault.ClientFromEnv().Sys().ListAuth()
	if err != nil {
		logrus.WithError(err).Fatal("failed to list authentication backends from Vault instance")
	}

	// Build a list of all existing GH entities
	var existingEntities []entry

	for mount, backend := range existingAuthMounts {
		if backend.Type == "github" {
			for _, entityPath := range []string{"map/teams", "map/users"} {
				// Get list of entities from vault
				path := filepath.Join("/auth", mount, entityPath)
				entitiesList, err := vault.ClientFromEnv().Logical().List(path)
				if err != nil {
					logrus.WithError(err).WithField("path", path).Fatal("failed to read entities list from Vault instance")
				}
				if entitiesList != nil {
					for _, entity := range entitiesList.Data["keys"].([]interface{}) {
						path := filepath.Join("/auth/", mount, entityPath, entity.(string))
						policies, err := vault.ClientFromEnv().Logical().Read(path)
						if err != nil {
							logrus.WithError(err).WithField("path", path).Fatal("failed to read secret")
						}

						existingEntities = append(existingEntities, entry{EntityName: entity.(string), EntityGroup: entityPath, AuthMount: mount, Policies: policies.Data["value"].(string)})
					}
				}
			}
		}
	}

	// Diff the local configuration with the Vault instance.
	toBeWritten, toBeDeleted := vault.DiffItems(asItems(entries), asItems(existingEntities))

	if dryRun == true {
		for _, w := range toBeWritten {
			logrus.Infof("[Dry Run]\tpackage=policies-mapping\tentry to be written='%v'", w)
		}
		for _, d := range toBeDeleted {
			logrus.Infof("[Dry Run]\tpackage=policies-mapping\tentry to be deleted='%v'", d)
		}
	} else {
		// Write any missing gh entity to the Vault instance.
		for _, e := range toBeWritten {
			e.(entry).writeEntiry(vault.ClientFromEnv())
		}

		// Delete GH entities that are not declared in config from the Vault instance.
		for _, e := range toBeDeleted {
			e.(entry).deleteEntity(vault.ClientFromEnv())
		}
	}
}

func (e entry) writeEntiry(client *api.Client) {
	path := filepath.Join("/auth", e.AuthMount, e.EntityGroup, e.EntityName)
	var data = make(map[string]interface{})
	data["value"] = e.Policies

	if _, err := client.Logical().Write(path, data); err != nil {
		logrus.WithError(err).WithField("path", path).Fatal("failed to apply Vault policy to entity")
	}
	logrus.WithField("path", path).Info("successfully applied Vault policy to entity")
}

func (e entry) deleteEntity(client *api.Client) {
	path := filepath.Join("/auth", e.AuthMount, e.EntityGroup, e.EntityName)
	_, err := client.Logical().Delete(path)
	if err != nil {
		logrus.WithError(err).WithField("path", path).Fatal("failed to delete entity from Vault instance")
	}
	logrus.WithField("path", path).Info("successfully deleted entity from Vault instance")
}

func asItems(xs []entry) (items []vault.Item) {
	items = make([]vault.Item, 0)
	for _, x := range xs {
		items = append(items, x)
	}

	return
}
