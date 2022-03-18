package entity

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/app-sre/vault-manager/pkg/utils"
	"github.com/app-sre/vault-manager/pkg/vault"
	"github.com/app-sre/vault-manager/toplevel"
)

type config struct{}

var _ toplevel.Configuration = config{}

type entry struct {
	Name     string                 `yaml:"name"`
	Type     string                 `yaml:"type"`
	Mount    string                 `yaml:"mount"`
	Metadata map[string]interface{} `yaml:"metadata"`
	Policies []string               `yaml:"policies"`
	Disabled bool                   `yaml:"disabled"`
	Aliases  []entityAlias          `yaml:"aliases"`
}

type entityAlias struct {
	Name           string                 `yaml:"name"`
	Type           string                 `yaml:"type"`
	CustomMetadata map[string]interface{} `yaml:"custom_metadata"`
}

var _ vault.Item = entry{}

var _ vault.Item = entityAlias{}

func (e entry) Key() string {
	return e.Name
}

func (e entry) KeyForType() string {
	return e.Type
}

func (e entry) KeyForDescription() string {
	return fmt.Sprintf("%v", e.Metadata)
}

func (e entry) Equals(i interface{}) bool {
	entry, ok := i.(entry)
	if !ok {
		return false
	}
	return e.Name == entry.Name &&
		e.Type == entry.Type &&
		e.Mount == entry.Mount &&
		reflect.DeepEqual(e.Metadata, entry.Metadata) &&
		reflect.DeepEqual(e.Policies, entry.Policies) &&
		e.Disabled == entry.Disabled &&
		equalAliases(e.Aliases, entry.Aliases)
}

func (e entityAlias) Key() string {
	return e.Name
}

func (e entityAlias) KeyForType() string {
	return e.Type
}

func (e entityAlias) KeyForDescription() string {
	return fmt.Sprintf("%v", e.CustomMetadata)
}

func (e entityAlias) Equals(i interface{}) bool {
	entry, ok := i.(entityAlias)
	if !ok {
		return false
	}

	return e.Name == entry.Name &&
		e.Type == entry.Type &&
		reflect.DeepEqual(e.CustomMetadata, entry.CustomMetadata)
}

func equalAliases(xaliases, yaliases []entityAlias) bool {
	if len(xaliases) != len(yaliases) {
		return false
	}

	sort.Slice(xaliases, func(i, j int) bool { return xaliases[i].Name < xaliases[j].Name })
	// Sort by type preserving name order
	sort.SliceStable(xaliases, func(i, j int) bool { return xaliases[i].Type < xaliases[j].Type })

	sort.Slice(yaliases, func(i, j int) bool { return yaliases[i].Name < yaliases[j].Name })
	// Sort by type preserving name order
	sort.SliceStable(yaliases, func(i, j int) bool { return yaliases[i].Type < yaliases[j].Type })

	for i := range xaliases {
		if !xaliases[i].Equals(yaliases[i]) {
			return false
		}
	}
	return true
}

func init() {
	toplevel.RegisterConfiguration("vault_entities", config{})
}

func (c config) Apply(entriesBytes []byte, dryRun bool, threadPoolSize int) {

}

func unmarshallMetadatas(entities []entry) error {
	for i := range entities {
		converted, err := utils.UnmarshalJsonObj("metadata", entities[i].Metadata)
		if err != nil {
			return err
		}
		entities[i].Metadata = converted
		for j := range entities[i].Aliases {
			converted, err = utils.UnmarshalJsonObj("custom_metadata", entities[i].Aliases[j].CustomMetadata)
			if err != nil {
				return err
			}
			entities[i].Aliases[j].CustomMetadata = converted
		}
	}
	return nil
}
