// Package policy implements the application of a declarative configuration
// for Vault policies.
package policy

import (
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/app-sre/vault-manager/pkg/vault"
	"github.com/app-sre/vault-manager/toplevel"
)

type config struct{}

var _ toplevel.Configuration = config{}

func init() {
	toplevel.RegisterConfiguration("policies", config{})
}

type entry struct {
	Name  string `yaml:"name"`
	Rules string `yaml:"rules"`
}

var _ vault.Item = entry{}

func (e entry) Key() string {
	return e.Name
}

func (e entry) Equals(i interface{}) bool {
	entry, ok := i.(entry)
	if !ok {
		return false
	}

	return e.Name == entry.Name && e.Rules == entry.Rules
}

func (c config) Apply(entriesBytes []byte, dryRun bool) {
	// Unmarshal the list of configured secrets engines.
	var entries []entry
	if err := yaml.Unmarshal(entriesBytes, &entries); err != nil {
		logrus.WithError(err).Fatal("failed to decode policies configuration")
	}

	// List the existing policies.
	existingPolicyNames, err := vault.ClientFromEnv().Sys().ListPolicies()
	if err != nil {
		logrus.WithError(err).Fatal("failed to list policies from Vault instance")
	}

	// Build a list of all the existing entries.
	existingPolicies := make([]entry, 0)
	if existingPolicies != nil {
		for _, name := range existingPolicyNames {
			policy, err := vault.ClientFromEnv().Sys().GetPolicy(name)
			if err != nil {
				logrus.WithError(err).WithField("name", name).Fatal("failed to get existing policy from Vault instance")
			}
			existingPolicies = append(existingPolicies, entry{Name: name, Rules: policy})
		}
	}

	// Diff the local configuration with the Vault instance.
	toBeWritten, toBeDeleted := vault.DiffItems(asItems(entries), asItems(existingPolicies))

	if dryRun == true {
		for _, w := range toBeWritten {
			logrus.Infof("[Dry Run]\tpackage=policy\tentry to be written='%v'", w)
		}
		for _, d := range toBeDeleted {
			if isDefaultPolicy(d.Key()) {
				continue
			}

			logrus.Infof("[Dry Run]\tpackage=policy\tentry to be deleted='%v'", d)
		}
	} else {
		// Write any missing policies to the Vault instance.
		for _, e := range toBeWritten {
			ent := e.(entry)
			if err := vault.ClientFromEnv().Sys().PutPolicy(ent.Name, ent.Rules); err != nil {
				logrus.WithError(err).WithField("name", ent.Name).Fatal("failed to write policy to Vault instance")
			}
			logrus.WithField("name", ent.Name).Info("successfully wrote policy to Vault instance")
		}

		// Delete any policies from the Vault instance.
		for _, e := range toBeDeleted {
			ent := e.(entry)
			if isDefaultPolicy(ent.Name) {
				continue
			}

			if err := vault.ClientFromEnv().Sys().DeletePolicy(ent.Name); err != nil {
				logrus.WithError(err).WithField("name", ent.Name).Fatal("failed to delete policy from Vault instance")
			}
			logrus.WithField("name", ent.Name).Info("successfully deleted policy from Vault instance")
		}
	}
}

func isDefaultPolicy(name string) bool {
	return name == "root" || name == "default"
}

func asItems(xs []entry) (items []vault.Item) {
	items = make([]vault.Item, 0)
	for _, x := range xs {
		items = append(items, x)
	}

	return
}
