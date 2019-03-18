// Package audit implements the application of a declarative configuration
// for Vault Audit Devices.
package audit

import (
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

func (e entry) ambiguousOptions() map[string]interface{} {
	opts := make(map[string]interface{}, len(e.Options))
	for k, v := range e.Options {
		opts[k] = v
	}
	return opts
}

func (e entry) enable(client *api.Client) {
	if err := client.Sys().EnableAuditWithOptions(e.Path, &api.EnableAuditOptions{
		Type:        e.Type,
		Description: e.Description,
		Options:     e.Options,
	}); err != nil {
		log.WithField("package", "audit").WithField("path", e.Path).Fatal("failed to enable audit device")
	}
	log.WithField("package", "audit").WithField("path", e.Path).Info("audit device successfully enabled")
}

func (e entry) disable(client *api.Client) {
	if err := client.Sys().DisableAudit(e.Path); err != nil {
		log.WithField("package", "audit").WithField("path", e.Path).Fatal("failed to disable audit device")
	}
	log.WithField("package", "audit").WithField("path", e.Path).Info("audit device successfully disabled")
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
func (c config) Apply(entriesBytes []byte, dryRun bool) {
	var entries []entry
	if err := yaml.Unmarshal(entriesBytes, &entries); err != nil {
		log.WithField("package", "audit").WithError(err).Fatal("failed to decode audit device configuration")
	}

	// Get the existing enabled Audits Devices.
	enabledAudits, err := vault.Client().Sys().ListAudit()
	if err != nil {
		log.WithField("package", "audit").WithError(err).Fatal("failed to list audit devices from Vault instance")
	}

	// Build a list of all the existing entries.
	existingAudits := make([]entry, 0)
	if enabledAudits != nil {
		for _, audit := range enabledAudits {
			existingAudits = append(existingAudits, entry{
				Path:        audit.Path,
				Type:        audit.Type,
				Description: audit.Description,
				Options:     audit.Options,
			})
		}
	}

	// Diff the local configuration with the Vault instance.
	toBeWritten, toBeDeleted := vault.DiffItems(asItems(entries), asItems(existingAudits))

	if dryRun == true {
		for _, w := range toBeWritten {
			log.WithField("package", "audit").WithField("path", w.Key()).Infof("[Dry Run] audit-device to be enabled")
		}
		for _, d := range toBeDeleted {
			log.WithField("package", "audit").WithField("path", d.Key()).Infof("[Dry Run] audit device to be disabled")
		}
	} else {
		// Write any missing Audit Devices to the Vault instance.
		for _, e := range toBeWritten {
			e.(entry).enable(vault.Client())
		}

		// Delete any Audit Devices from the Vault instance.
		for _, e := range toBeDeleted {
			e.(entry).disable(vault.Client())
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
