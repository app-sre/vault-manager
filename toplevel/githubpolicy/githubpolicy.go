// Package githubpolicy implements the application of a declarative
// configuration for GitHub policies mappings in Vault.
package githubpolicy

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
	toplevel.RegisterConfiguration("gh-policy-mappings", config{})
}

type entry struct {
	EntityName  string `yaml:"entity-name"`
	EntityGroup string `yaml:"entity-group"`
	GhMountName string `yaml:"gh-mount-name"`
	Policies    string `yaml:"policies"`
}

var _ vault.Item = entry{}

func (e entry) Key() string {
	path := filepath.Join("/auth", e.GhMountName, "map", e.EntityGroup, e.EntityName)
	return path
}

func (e entry) Equals(i interface{}) bool {
	entry, ok := i.(entry)
	if !ok {
		return false
	}
	ePpath := filepath.Join("/auth", e.GhMountName, "map", e.EntityGroup, e.EntityName)
	entryPpath := filepath.Join("/auth", entry.GhMountName, "map", entry.EntityGroup, entry.EntityName)
	return ePpath == entryPpath && e.Policies == entry.Policies
}

func (c config) Apply(entriesBytes []byte, dryRun bool) {
	// Unmarshal the list of configured secrets engines.
	var entries []entry
	if err := yaml.Unmarshal(entriesBytes, &entries); err != nil {
		logrus.WithError(err).Fatal("failed to decode GitHub policies configuration")
	}

	// Get existing enabled auth backends.
	existingAuthMounts, err := vault.ClientFromEnv().Sys().ListAuth()
	if err != nil {
		logrus.WithError(err).Fatal("failed to list authentication backends from Vault instance")
	}

	// Build a list of all existing GH entities
	var existingEntities []entry
	for ghMountName, backend := range existingAuthMounts {
		if backend.Type == "github" {
			// Get the current GitHub teams for a given GitHub mount.
			teamsPath := filepath.Join("/auth", ghMountName, "map/teams")
			ghTeamsList, err := vault.ClientFromEnv().Logical().List(teamsPath)
			if err != nil {
				logrus.WithError(err).WithField("path", teamsPath).Fatal("failed to read GitHub teams list from Vault instance")
			}
			if ghTeamsList != nil {
				for _, team := range ghTeamsList.Data["keys"].([]interface{}) {
					path := filepath.Join("/auth/", ghMountName, "map/teams", team.(string))
					policies, err := vault.ClientFromEnv().Logical().Read(path)
					if err != nil {
						logrus.WithError(err).WithField("path", path).Fatal("failed to read secret")
					}

					existingEntities = append(existingEntities, entry{EntityName: team.(string), EntityGroup: "teams", GhMountName: ghMountName, Policies: policies.Data["value"].(string)})
				}
			}

			// Get the current GitHub teams for a given GitHub mount.
			usersPath := filepath.Join("/auth", ghMountName, "map/users")
			ghUsersList, err := vault.ClientFromEnv().Logical().List(usersPath)
			if err != nil {
				logrus.WithError(err).WithField("path", usersPath).Fatal("failed to read GitHub users list from Vault instance")
			}
			if ghUsersList != nil {
				for _, user := range ghUsersList.Data["keys"].([]interface{}) {
					path := filepath.Join("/auth/", ghMountName, "map/users", user.(string))
					policies, err := vault.ClientFromEnv().Logical().Read(path)
					if err != nil {
						logrus.WithError(err).WithField("path", path).Fatal("failed to read secret")
					}

					existingEntities = append(existingEntities, entry{EntityName: user.(string), EntityGroup: "users", GhMountName: ghMountName, Policies: policies.Data["value"].(string)})
				}
			}
		}
	}

	// Diff the local configuration with the Vault instance.
	toBeWritten, toBeDeleted := vault.DiffItems(asItems(entries), asItems(existingEntities))

	if dryRun == true {
		for _, w := range toBeWritten {
			logrus.Infof("[Dry Run]\tpackage=github-policy\tentry to be written='%v'", w)
		}
		for _, d := range toBeDeleted {
			logrus.Infof("[Dry Run]\tpackage=github-policy\tentry to be deleted='%v'", d)
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
	path := filepath.Join("/auth", e.GhMountName, "map", e.EntityGroup, e.EntityName)
	var data = make(map[string]interface{})
	data["value"] = e.Policies

	if _, err := client.Logical().Write(path, data); err != nil {
		logrus.WithError(err).WithField("path", path).Fatal("failed to apply Vault policy to Github entity")
	}
	logrus.WithField("path", path).Info("successfully applied Vault policy to Github entity")
}

func (e entry) deleteEntity(client *api.Client) {
	path := filepath.Join("/auth", e.GhMountName, "map", e.EntityGroup, e.EntityName)
	_, err := client.Logical().Delete(path)
	if err != nil {
		logrus.WithError(err).WithField("path", path).Fatal("failed to delete GitHub entity from Vault instance")
	}
	logrus.WithField("path", path).Info("successfully deleted GitHub entity from Vault instance")
}

func asItems(xs []entry) (items []vault.Item) {
	items = make([]vault.Item, 0)
	for _, x := range xs {
		items = append(items, x)
	}

	return
}
