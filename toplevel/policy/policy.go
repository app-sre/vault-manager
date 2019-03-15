// Package policy implements the application of a declarative configuration
// for Vault policies.
package policy

import (
	"fmt"
	"github.com/app-sre/vault-manager/pkg/vault"
	"github.com/app-sre/vault-manager/toplevel"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"time"
)

type config struct{}

var _ toplevel.Configuration = config{}

func init() {
	toplevel.RegisterConfiguration("vault_policies", config{})
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
	fmt.Println("policy start:",time.Now().Format(time.Stamp))
	// Unmarshal the list of configured secrets engines.
	var entries []entry
	if err := yaml.Unmarshal(entriesBytes, &entries); err != nil {
		log.WithField("package", "policy").WithError(err).Fatal("failed to decode policies configuration")
	}
	fmt.Println("policy after unmarshal:",time.Now().Format(time.Stamp))
	// List the existing policies.
	existingPolicyNames, err := vault.Client().Sys().ListPolicies()
	if err != nil {
		log.WithField("package", "policy").WithError(err).Fatal("failed to list policies from Vault instance")
	}
	fmt.Println("policy after existing policies:",time.Now().Format(time.Stamp))
	// Build a list of all the existing entries.
	existingPolicies := make([]entry, 0)
	if existingPolicies != nil {
		for _, name := range existingPolicyNames {
			policy, err := vault.Client().Sys().GetPolicy(name)
			if err != nil {
				log.WithField("package", "policy").WithError(err).WithField("name", name).Fatal("failed to get existing policy from Vault instance")
			}
			existingPolicies = append(existingPolicies, entry{Name: name, Rules: policy})
		}
	}
	fmt.Println("policy after build existing policies list:",time.Now().Format(time.Stamp))
	// Diff the local configuration with the Vault instance.
	toBeWritten, toBeDeleted := vault.DiffItems(asItems(entries), asItems(existingPolicies))
	fmt.Println("policy after diff:",time.Now().Format(time.Stamp))
	if dryRun == true {
		for _, w := range toBeWritten {
			log.WithField("package", "policy").Infof("[Dry Run] policy to be written='%v'", w.Key())
		}
		for _, d := range toBeDeleted {
			if isDefaultPolicy(d.Key()) {
				continue
			}

			log.WithField("package", "policy").Infof("[Dry Run] policy to be deleted='%v'", d.Key())
		}
	} else {
		// Write any missing policies to the Vault instance.
		for _, e := range toBeWritten {
			ent := e.(entry)
			if err := vault.Client().Sys().PutPolicy(ent.Name, ent.Rules); err != nil {
				log.WithField("package", "policy").WithError(err).WithField("name", ent.Name).Fatal("failed to write policy to Vault instance")
			}
			log.WithField("package", "policy").WithField("name", ent.Name).Info("policy successfully written to Vault instance")
		}

		// Delete any policies from the Vault instance.
		for _, e := range toBeDeleted {
			ent := e.(entry)
			if isDefaultPolicy(ent.Name) {
				continue
			}

			if err := vault.Client().Sys().DeletePolicy(ent.Name); err != nil {
				log.WithField("package", "policy").WithError(err).WithField("name", ent.Name).Fatal("failed to delete policy from Vault instance")
			}
			log.WithField("package", "policy").WithField("name", ent.Name).Info("successfully deleted policy from Vault instance")
		}
	}
	fmt.Println("policy finish:",time.Now().Format(time.Stamp))
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
