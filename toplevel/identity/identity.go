package identity

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"sync"

	"github.com/app-sre/vault-manager/pkg/utils"
	"github.com/app-sre/vault-manager/pkg/vault"
	"github.com/app-sre/vault-manager/toplevel"
	"github.com/hashicorp/go-version"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type config struct{}

var _ toplevel.Configuration = config{}

type entry struct {
	Name     string `yaml:"name"`
	Id       string
	Type     string        `yaml:"type"`
	Metadata interface{}   `yaml:"metadata"`
	Policies []string      `yaml:"policies"`
	Disabled bool          `yaml:"disabled"`
	Aliases  []entityAlias `yaml:"aliases"`
}

type entityAlias struct {
	Name           string `yaml:"name"`
	Id             string
	Type           string `yaml:"type"`
	AccessorId     string
	CustomMetadata interface{} `yaml:"custom_metadata"`
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
	var entries []entry
	if err := yaml.Unmarshal(entriesBytes, &entries); err != nil {
		log.WithError(err).Fatal("[Vault Identity] failed to decode entity configuration")
	}

	// Get the existing entities
	listEntitiesResult := vault.ListEntities()
	existingEntities, err := createBaseExistingEntities(listEntitiesResult)
	if err != nil {
		log.WithError(err).Fatal("[Vault Identity] failed to parse existing entities")
	}
	err = getExistingEntitiesDetails(existingEntities, threadPoolSize)
	if err != nil {
		log.WithError(err).Fatal("[Vault Identity] failed to retrieve details for existing entities")
	}

}

func createBaseExistingEntities(raw map[string]interface{}) ([]entry, error) {
	processed := []entry{}
	if _, exists := raw["key_info"]; !exists {
		return nil, errors.New("Required `key_info` attribute not found in response from vault.ListEntites()")
	}
	existingEntities, ok := raw["key_info"].(map[string]interface{})
	if !ok {
		return nil, errors.New(fmt.Sprintf("Failed to convert `key_info` to map[string]interface{}"))
	}

	for id, v := range existingEntities {
		values, ok := v.(map[string]interface{})
		if !ok {
			return nil, errors.New(fmt.Sprintf("Failed to convert value to map[string]interface{} for entity id: %s", id))
		}
		if _, exists := values["name"]; !exists {
			return nil, errors.New(fmt.Sprintf("Required `name` attribute not found for entity id: %s", id))
		}
		name := values["name"].(string)
		if _, exists := values["aliases"]; !exists {
			return nil, errors.New(fmt.Sprintf("Required `aliases` attribute not found for entity id: %s", id))
		}
		aliases, ok := values["aliases"].([]interface{})
		if !ok {
			return nil, errors.New(fmt.Sprintf("Failed to convert `aliases` to []interface{} for entity id: %s", id))
		}

		// process alias infos
		processedAliases := []entityAlias{}
		for _, alias := range aliases {
			vals, ok := alias.(map[string]interface{})
			if !ok {
				return nil, errors.New(fmt.Sprintf("Failed to convert element within `aliases` to map[string]interface{} for entity id: %s", id))
			}
			if _, exists := vals["id"]; !exists {
				return nil, errors.New(fmt.Sprintf("Required `id` attribute not found on alias element for entity id: %s", id))
			}
			aliasId := vals["id"].(string)
			if _, exists := vals["name"]; !exists {
				return nil, errors.New(fmt.Sprintf("Required `name` attribute not found on alias element for entity-alias id: %s", id))
			}
			aliasName := vals["name"].(string)
			if _, exists := vals["mount_type"]; !exists {
				return nil, errors.New(fmt.Sprintf("Required `mount_type` attribute not found on alias element for entity-alias id: %s", id))
			}
			mountType := vals["mount_type"].(string)
			processedAliases = append(processedAliases, entityAlias{
				Id:   aliasId,
				Name: aliasName,
				Type: mountType,
			})
		}

		processed = append(processed, entry{
			Name:    name,
			Id:      id,
			Aliases: processedAliases,
		})
	}
	return processed, nil
}

func getExistingEntitiesDetails(entities []entry, threadPoolSize int) error {
	var mutex = &sync.Mutex{}
	bwg := utils.NewBoundedWaitGroup(threadPoolSize)

	for _, entity := range entities {
		bwg.Add(1)

		go func(id string) {
			mutex.Lock()

			info := vault.GetEntityInfo(id)

			if _, exists := info["disabled"]; !exists {
				log.WithError(errors.New(fmt.Sprintf("Required `disabled` attribute not found for entity id: %s", id))).Fatal()
			}
			disabled := info["disabled"].(bool)

			if _, exists := info["policies"]; !exists {
				log.WithError(errors.New(fmt.Sprintf("Required `policies` attribute not found for entity id: %s", id))).Fatal()
			}
			rawPolicies := info["policies"].([]interface{})
			policies := []string{}
			for _, policy := range rawPolicies {
				policies = append(policies, policy.(string))
			}

		}(entity.Id)
	}
	return nil
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

// remove attributes not supported in commercial but in fedramp variant
func pruneUnsupported(roles []entry) error {
	current, err := version.NewVersion(vault.GetVaultVersion())
	if err != nil {
		return err
	}
	threshold, err := version.NewVersion("1.9.0")
	if err != nil {
		return err
	}
	if current.LessThan(threshold) {
		// TODO:
		// remove custom metadata
	}
	return nil
}
