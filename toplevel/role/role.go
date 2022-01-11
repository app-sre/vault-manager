// Package role implements the application of a declarative configuration
// for Vault App Roles.
package role

import (
	"path/filepath"
	"sync"

	"github.com/app-sre/vault-manager/pkg/utils"
	"github.com/app-sre/vault-manager/pkg/vault"
	"github.com/app-sre/vault-manager/toplevel"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type entry struct {
	Name        string                 `yaml:"name"`
	Type        string                 `yaml:"type"`
	Mount       string                 `yaml:"mount"`
	Description string                 `yaml:"description"`
	Options     map[string]interface{} `yaml:"options"`
}

var _ vault.Item = entry{}

func (e entry) KeyForDescription() string {
	return e.Description
}

func (e entry) Key() string {
	return e.Name
}

func (e entry) Equals(i interface{}) bool {
	entry, ok := i.(entry)
	if !ok {
		return false
	}

	return e.Name == entry.Name &&
		e.Type == entry.Type &&
		e.Mount == entry.Mount &&
		vault.OptionsEqual(e.Options, entry.Options)
}

func (e entry) Save() {
	path := filepath.Join("auth", e.Mount, "role", e.Name)
	options := make(map[string]interface{})
	for k, v := range e.Options {
		// local_secret_ids can not be changed after creation so we skip this option
		if k == "local_secret_ids" {
			continue
		}
		options[k] = v
	}
	vault.WriteSecret(path, options)
	log.WithField("path", path).WithField("type", e.Type).Info("[Vault Role] role is successfully written")
}

func (e entry) Delete() {
	path := filepath.Join("auth", e.Mount, "role", e.Name)
	vault.DeleteSecret(path)
	log.WithField("path", path).WithField("type", e.Type).Info("[Vault Role] role is successfully deleted from Vault instance")
}

type config struct{}

var _ toplevel.Configuration = config{}

func init() {
	toplevel.RegisterConfiguration("vault_roles", config{})
}

// Apply ensures that an instance of Vault's roles are configured exactly
// as provided.
//
// This function exits the program if an error occurs.
func (c config) Apply(entriesBytes []byte, dryRun bool, threadPoolSize int) {
	var entries []entry
	if err := yaml.Unmarshal(entriesBytes, &entries); err != nil {
		log.WithError(err).Fatal("[Vault Role] failed to decode role configuration")
	}

	// Get the existing auth backends
	existingAuthBackends := vault.ListAuthBackends()

	existingRoles := make([]entry, 0)

	if existingAuthBackends != nil {
		for authBackend := range existingAuthBackends {
			// Get the secret with the existing App Roles.
			path := filepath.Join("auth", authBackend, "role")
			secret := vault.ListSecrets(path)
			if secret != nil {
				roles := secret.Data["keys"].([]interface{})

				var mutex = &sync.Mutex{}

				bwg := utils.NewBoundedWaitGroup(threadPoolSize)

				// fill existing policies array in parallel
				for i := range roles {

					bwg.Add(1)

					go func(i int) {

						path := filepath.Join("auth", authBackend, "role", roles[i].(string))

						mutex.Lock()

						existingRoles = append(existingRoles, entry{
							Name:    roles[i].(string),
							Type:    existingAuthBackends[authBackend].Type,
							Mount:   authBackend,
							Options: vault.ReadSecret(path).Data,
						})

						defer bwg.Done()
						defer mutex.Unlock()
					}(i)
				}
				bwg.Wait()
			}
		}
	}

	// Diff the local configuration with the Vault instance.
	entriesToBeWritten, entriesToBeDeleted, _ := vault.DiffItems(asItems(entries), asItems(existingRoles))

	if dryRun == true {
		for _, w := range entriesToBeWritten {
			log.WithField("name", w.Key()).WithField("type", w.(entry).Type).Info("[Dry Run] [Vault Role] role to be written")
		}
		for _, d := range entriesToBeDeleted {
			log.WithField("name", d.Key()).WithField("type", d.(entry).Type).Info("[Dry Run] [Vault Role] role to be deleted")
		}
	} else {
		// Write any missing App Roles to the Vault instance.
		for _, e := range entriesToBeWritten {
			e.(entry).Save()
		}

		// Delete any App Roles from the Vault instance.
		for _, e := range entriesToBeDeleted {
			e.(entry).Delete()
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
