// Package audit implements the application of a declarative configuration
// for Vault Audit Devices.
package audit

import (
	"sync"

	"github.com/app-sre/vault-manager/pkg/utils"
	"github.com/app-sre/vault-manager/pkg/vault"
	"github.com/app-sre/vault-manager/toplevel"
	"github.com/hashicorp/vault/api"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
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

func (e entry) Equals(i interface{}) bool {
	entry, ok := i.(entry)
	if !ok {
		return false
	}

	return vault.EqualPathNames(e.Path, entry.Path) &&
		e.Type == entry.Type &&
		e.Description == entry.Description &&
		vault.OptionsEqual(e.ambiguousOptions(), entry.ambiguousOptions())
}

func (e entry) KeyForDescription() string {
	return e.Description
}

func (e entry) ambiguousOptions() map[string]interface{} {
	opts := make(map[string]interface{}, len(e.Options))
	for k, v := range e.Options {
		opts[k] = v
	}
	return opts
}

type config struct{}

var _ toplevel.Configuration = config{}

func init() {
	toplevel.RegisterConfiguration("vault_audit_backends", config{})
}

// Apply ensures that an instance of Vault's Audit Devices are configured
// exactly as provided.
//
// This function exits the program if an error occurs.
func (c config) Apply(entriesBytes []byte, dryRun bool, threadPoolSize int) {
	var entries []entry
	if err := yaml.Unmarshal(entriesBytes, &entries); err != nil {
		log.WithError(err).Fatal("[Vault Audit] failed to decode audit device configuration")
	}

	enabledAudits := vault.ListAuditDevices()

	// Build a list of all the existing entries.
	existingAudits := make([]entry, 0)

	if enabledAudits != nil {

		var mutex = &sync.Mutex{}

		bwg := utils.NewBoundedWaitGroup(threadPoolSize)

		// fill existing audits array in parallel
		for k := range enabledAudits {

			bwg.Add(1)

			go func(k string) {

				mutex.Lock()

				existingAudits = append(existingAudits, entry{
					Path:        enabledAudits[k].Path,
					Type:        enabledAudits[k].Type,
					Description: enabledAudits[k].Description,
					Options:     enabledAudits[k].Options,
				})

				defer bwg.Done()
				defer mutex.Unlock()
			}(k)
		}
		bwg.Wait()
	}

	// Diff the local configuration with the Vault instance.
	toBeWritten, toBeDeleted, _ := vault.DiffItems(asItems(entries), asItems(existingAudits))

	if dryRun == true {
		for _, w := range toBeWritten {
			log.WithField("path", w.Key()).Infof("[Dry Run] [Vault Audit] audit device to be enabled")
		}
		for _, d := range toBeDeleted {
			log.WithField("path", d.Key()).Infof("[Dry Run] [Vault Audit] audit device to be disabled")
		}
	} else {
		// Write any missing Audit Devices to the Vault instance.
		for _, e := range toBeWritten {
			ent := e.(entry)
			vault.EnableAduitDevice(ent.Path, &api.EnableAuditOptions{
				Type:        ent.Type,
				Description: ent.Description,
				Options:     ent.Options,
			})
		}

		// Delete any Audit Devices from the Vault instance.
		for _, e := range toBeDeleted {
			vault.DisableAuditDevice(e.(entry).Path)
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
