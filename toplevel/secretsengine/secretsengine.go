// Package secretsengine implements the application of a declarative configuration
// for Vault Secrets Engines.
//
// Secrets Engines used to be referred to as "mounts".
package secretsengine

import (
	"strings"

	"github.com/app-sre/vault-manager/pkg/utils"
	"github.com/hashicorp/vault/api"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"sync"

	"github.com/app-sre/vault-manager/pkg/vault"
	"github.com/app-sre/vault-manager/toplevel"
)

type entry struct {
	Path        string            `yaml:"_path"`
	Type        string            `yaml:"type"`
	Description string            `yaml:"description"`
	Options     map[string]string `yaml:"options"`
}

var _ vault.Item = entry{}

func (e entry) Key() string {
	return e.Path
}

func (e entry) KeyForDescription() string {
	return e.Description
}

func (e entry) KeyForType() string {
	return e.Type
}

func (e entry) KeyForOptions() map[string]string {
	return e.Options
}

func (e entry) Equals(i interface{}) bool {
	entry, ok := i.(entry)
	if !ok {
		return false
	}

	return vault.EqualPathNames(e.Path, entry.Path) &&
		e.Type == entry.Type &&
		e.Description == entry.Description &&
		vault.OptionsEqual(e.AmbiguousOptions(), entry.AmbiguousOptions())
}

func (e entry) AmbiguousOptions() map[string]interface{} {
	opts := make(map[string]interface{}, len(e.Options))
	for k, v := range e.Options {
		opts[k] = v
	}
	return opts
}

type config struct{}

var _ toplevel.Configuration = config{}

func init() {
	toplevel.RegisterConfiguration("vault_secret_engines", config{})
}

// Apply ensures that an instance of Vault's secrets engine are configured
// exactly as provided.
//
// This function exits the program if an error occurs.
func (c config) Apply(entriesBytes []byte, dryRun bool, threadPoolSize int) {
	// Unmarshal the list of configured secrets engines.
	var entries []entry
	if err := yaml.Unmarshal(entriesBytes, &entries); err != nil {
		log.WithError(err).Fatal("[Vault Secrets engine] failed to decode secrets engines configuration")
	}

	// List the existing secrets engines.
	existingMounts := vault.ListSecretsEngines()

	// Build a list of all the existing entries.
	existingSecretsEngines := make([]entry, 0)

	if existingSecretsEngines != nil {

		var mutex = &sync.Mutex{}

		bwg := utils.NewBoundedWaitGroup(threadPoolSize)

		for path, engine := range existingMounts {

			bwg.Add(1)

			go func(path string, engine *api.MountOutput) {

				mutex.Lock()

				existingSecretsEngines = append(existingSecretsEngines, entry{
					Path:        path,
					Type:        engine.Type,
					Description: engine.Description,
					Options:     engine.Options,
				})

				defer bwg.Done()
				defer mutex.Unlock()
			}(path, engine)
		}
		bwg.Wait()
	}

	toBeWritten, toBeDeleted, toBeUpdated := vault.DiffItems(asItems(entries), asItems(existingSecretsEngines))

	if dryRun == true {
		for _, w := range toBeWritten {
			log.WithField("path", w.Key()).WithField("type", w.(entry).Type).Info("[Dry Run] [Vault Secrets engine] secrets-engine to be enabled")
		}
		for _, d := range toBeDeleted {
			if !isDefaultMount(d.Key()) {
				log.WithField("path", d.Key()).WithField("type", d.(entry).Type).Infof("[Dry Run] [Vault Secrets engine] secrets-engine to be disabled")
			}
		}
		for _, u := range toBeUpdated {
			log.WithField("path", u.Key()).WithField("type", u.(entry).Type).Info("[Dry Run] [Vault Secrets engine] secrets-engine to be updated")
		}
	} else {
		for _, e := range toBeWritten {
			ent := e.(entry)
			vault.EnableSecretsEngine(ent.Path, &api.MountInput{
				Type:        ent.Type,
				Description: ent.Description,
				Options:     ent.Options,
			})
		}
		for _, e := range toBeDeleted {
			ent := e.(entry)
			if !isDefaultMount(ent.Path) {
				vault.DisableSecretsEngine(ent.Path)
			}
		}
		for _, e := range toBeUpdated {
			ent := e.(entry)
			vault.UpdateSecretsEngine(ent.Path, api.MountConfigInput{
				Description: &ent.Description,
			})
		}
	}
}

func isDefaultMount(path string) bool {
	switch {
	case strings.HasPrefix(path, "cubbyhole/"),
		strings.HasPrefix(path, "identity/"),
		strings.HasPrefix(path, "secret/"),
		strings.HasPrefix(path, "sys/"):
		return true
	default:
		return false
	}
}

func asItems(xs []entry) (items []vault.Item) {
	items = make([]vault.Item, 0)
	for _, x := range xs {
		items = append(items, x)
	}

	return
}
