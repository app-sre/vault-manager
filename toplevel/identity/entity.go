package identity

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/app-sre/vault-manager/pkg/utils"
	"github.com/app-sre/vault-manager/pkg/vault"
	"github.com/app-sre/vault-manager/toplevel"
	"github.com/hashicorp/go-version"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type config struct{}

var _ toplevel.Configuration = config{}

type user struct {
	Name        string `yaml:"name"`
	OrgUsername string `yaml:"org_username"`
	Roles       []role `yaml:"roles"`
}

type role struct {
	Name        string           `yaml:"name"`
	Permissions []oidcPermission `yaml:"oidc_permissions"`
}

type oidcPermission struct {
	Name        string        `yaml:"name"`
	Description string        `yaml:"description"`
	Service     string        `yaml:"service"`
	Policies    []vaultPolicy `yaml:"vault_policies"`
}

type vaultPolicy struct {
	Name string `yaml:"name"`
}

type entity struct {
	Name     string
	Id       string
	Type     string
	Metadata interface{}
	Policies []string
	Aliases  []entityAlias
}

type entityAlias struct {
	Name           string
	Id             string
	Type           string
	AuthType       string
	AccessorId     string
	CustomMetadata interface{}
}

var _ vault.Item = entity{}

var _ vault.Item = entityAlias{}

func (e entity) Key() string {
	return e.Name
}

func (e entity) KeyForType() string {
	return e.Type
}

func (e entity) KeyForDescription() string {
	return fmt.Sprintf("%v", e.Metadata)
}

func (e entity) Equals(i interface{}) bool {
	entry, ok := i.(entity)
	if !ok {
		return false
	}
	return e.Name == entry.Name &&
		reflect.DeepEqual(e.Metadata, entry.Metadata) &&
		reflect.DeepEqual(e.Policies, entry.Policies)
}

func (e entity) CreateOrUpdate(action string) {
	path := filepath.Join("identity", e.Type, "name", e.Name)
	config := map[string]interface{}{
		"policies": e.Policies,
		"metadata": e.Metadata,
	}
	vault.WriteSecret(path, config)
	log.WithField("path", path).WithField("type", e.Type).Info(
		fmt.Sprintf("[Vault Identity] entity successfully %s", action))
}

func (e entity) Delete() {
	path := filepath.Join("identity", e.Type, "name", e.Name)
	vault.DeleteSecret(path)
	log.WithField("path", path).WithField("type", e.Type).Info(
		"[Vault Identity] entity successfully deleted")
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
		e.AuthType == entry.AuthType &&
		reflect.DeepEqual(e.CustomMetadata, entry.CustomMetadata)
}

func (ea entityAlias) Create(entityId string, customMetadataSupported bool) {
	path := filepath.Join("identity", ea.Type)
	config := map[string]interface{}{
		"name":           ea.Name,
		"canonical_id":   entityId,
		"mount_accessor": ea.AccessorId,
	}
	if customMetadataSupported {
		config["custom_metadata"] = ea.CustomMetadata
	}
	vault.WriteEntityAlias(path, config)
	log.WithField("path", filepath.Join(path, ea.Name)).WithField("type", ea.AuthType).Info(
		"[Vault Identity] entity alias successfully written")
}

func (ea entityAlias) Update(entityId string, customMetadataSupported bool) {
	path := filepath.Join("identity", ea.Type, "id", ea.Id)
	config := map[string]interface{}{
		"name":           ea.Name,
		"canonical_id":   entityId,
		"mount_accessor": ea.AccessorId,
	}
	if customMetadataSupported {
		config["custom_metadata"] = ea.CustomMetadata
	}
	vault.WriteSecret(path, config)
	log.WithField("path", filepath.Join(path, ea.Name)).WithField("type", ea.AuthType).Info(
		"[Vault Identity] entity alias successfully updated")
}

func (ea entityAlias) Delete() {
	path := filepath.Join("identity", ea.Type, "id", ea.Id)
	vault.DeleteSecret(path)
	log.WithField("path", filepath.Join(path, ea.Name)).WithField("type", ea.AuthType).Info(
		"[Vault Identity] entity alias successfully deleted")
}

func init() {
	toplevel.RegisterConfiguration("vault_entities", config{})
}

func (c config) Apply(entriesBytes []byte, dryRun bool, threadPoolSize int) {
	// process desired entities/aliases
	var entries []user
	if err := yaml.Unmarshal(entriesBytes, &entries); err != nil {
		log.WithError(err).Fatal("[Vault Identity] failed to decode entity configuration")
	}
	desired := generateDesired(entries)
	populateAliasType(desired)
	sortPolicies(desired)

	customMetadataSupported, err := isCustomMetadataSupported()
	if err != nil {
		log.WithError(err).Fatal("[Vault Identity] failed to determine vault version")
	}

	// Process data on existing entities/aliases
	existingEntities, err := createBaseExistingEntities()
	if err != nil {
		log.WithError(err).Fatal("[Vault Identity] failed to parse existing entities")
	}
	if existingEntities != nil {
		getExistingEntitiesDetails(existingEntities, threadPoolSize, customMetadataSupported)
		if !customMetadataSupported {
			pruneUnsupported(desired)
		}
		populateAliasType(existingEntities)
		copyIds(desired, existingEntities)
		sortPolicies(existingEntities)
	}

	// determine entity changes
	entitiesToBeWritten, entitiesToBeDeleted, entitiesToBeUpdated :=
		vault.DiffItems(entriesAsItems(desired), entriesAsItems(existingEntities))
	// determine entity alias changes
	aliasesToBeWritten, aliasesToBeDeleted, aliasesToBeUpdated :=
		determineAliasActions(desired, existingEntities, entitiesToBeDeleted)

	// preform actions
	if dryRun {
		entitiesDryRunOutput(entitiesToBeWritten, "written")
		entitiesDryRunOutput(entitiesToBeDeleted, "deleted")
		entitiesDryRunOutput(entitiesToBeUpdated, "updated")
		aliasesDryRunOutput(aliasesToBeWritten["id"], "written")
		aliasesDryRunOutput(aliasesToBeWritten["name"], "written")
		for _, alias := range aliasesToBeDeleted {
			log.WithField("name", alias.Key()).WithField("type", alias.(entityAlias).AuthType).Info(
				fmt.Sprintf("[Dry Run] [Vault Identity] entity alias to be delelted"))
		}
		aliasesDryRunOutput(aliasesToBeUpdated, "updated")
	} else {
		// TODO: make each action perform concurrently
		for _, w := range entitiesToBeWritten {
			w.(entity).CreateOrUpdate("written")
		}
		for _, d := range entitiesToBeDeleted {
			d.(entity).Delete()
		}
		for _, u := range entitiesToBeUpdated {
			u.(entity).CreateOrUpdate("update")
		}
		err = performAliasReconcile(aliasesToBeWritten, aliasesToBeDeleted, aliasesToBeUpdated, customMetadataSupported)
		if err != nil {
			log.WithError(err).Fatal(
				"[Vault Identity] failed to perform entity-alias reconcile operations")
		}
	}
}

// generatedDesired accepts the yaml-marshalled result of the `vault_entities` graphql
// query and returns entity/entity-alias objects
func generateDesired(entries []user) []entity {
	desired := []entity{}

	for _, entry := range entries {
		var newDesired *entity
		var uniquePolicies map[string]bool
		if entry.Roles != nil {
			for _, role := range entry.Roles {
				if role.Permissions != nil {
					for _, permission := range role.Permissions {
						if permission.Service == "vault" {
							if newDesired == nil {
								newDesired = &entity{
									Name: entry.OrgUsername,
									Type: "entity",
									Aliases: []entityAlias{
										{
											Name:     entry.OrgUsername,
											Type:     "entity-alias",
											AuthType: "oidc",
										},
									},
									Metadata: map[string]interface{}{
										"name": entry.Name,
									},
								}
								uniquePolicies = make(map[string]bool)
							}
							for _, policy := range permission.Policies {
								// avoid adding same policy more than once that exists in multiple roles
								if _, exists := uniquePolicies[policy.Name]; !exists {
									newDesired.Policies = append(newDesired.Policies, policy.Name)
									uniquePolicies[policy.Name] = true
								}
							}
						}
					}
				}
			}
		}
		if newDesired != nil {
			desired = append(desired, *newDesired)
		}
	}
	return desired
}

// processes all relevant info for entities/entity aliases from single vault api request
func createBaseExistingEntities() ([]entity, error) {
	raw := vault.ListEntities()
	if raw == nil {
		return nil, nil
	}
	processed := []entity{}
	if _, exists := raw["key_info"]; !exists {
		return nil, errors.New(
			"Required `key_info` attribute not found in response from vault.ListEntites()")
	}
	existingEntities, ok := raw["key_info"].(map[string]interface{})
	if !ok {
		return nil, errors.New(fmt.Sprintf(
			"Failed to convert `key_info` to map[string]interface{}"))
	}

	for id, v := range existingEntities {
		values, ok := v.(map[string]interface{})
		if !ok {
			return nil, errors.New(fmt.Sprintf(
				"Failed to convert value to map[string]interface{} for entity id: %s", id))
		}
		if _, exists := values["name"]; !exists {
			return nil, errors.New(fmt.Sprintf(
				"Required `name` attribute not found for entity id: %s", id))
		}
		name := values["name"].(string)
		if _, exists := values["aliases"]; !exists {
			return nil, errors.New(fmt.Sprintf(
				"Required `aliases` attribute not found for entity id: %s", id))
		}
		aliases, ok := values["aliases"].([]interface{})
		if !ok {
			return nil, errors.New(fmt.Sprintf(
				"Failed to convert `aliases` to []interface{} for entity id: %s", id))
		}

		// process alias infos
		processedAliases := []entityAlias{}
		for _, alias := range aliases {
			vals, ok := alias.(map[string]interface{})
			if !ok {
				return nil, errors.New(fmt.Sprintf(
					"Failed to convert element within `aliases` to map[string]interface{} for entity id: %s", id))
			}
			if _, exists := vals["id"]; !exists {
				return nil, errors.New(fmt.Sprintf(
					"Required `id` attribute not found on alias element for entity id: %s", id))
			}
			aliasId := vals["id"].(string)
			if _, exists := vals["name"]; !exists {
				return nil, errors.New(fmt.Sprintf(
					"Required `name` attribute not found on alias element for entity-alias id: %s", id))
			}
			aliasName := vals["name"].(string)
			if _, exists := vals["mount_type"]; !exists {
				return nil, errors.New(fmt.Sprintf(
					"Required `mount_type` attribute not found on alias element for entity-alias id: %s", id))
			}
			mountType := vals["mount_type"].(string)
			processedAliases = append(processedAliases, entityAlias{
				Id:       aliasId,
				Name:     aliasName,
				AuthType: mountType,
			})
		}

		processed = append(processed, entity{
			Name:    name,
			Id:      id,
			Type:    "entity", // used for reconcile and output
			Aliases: processedAliases,
		})
	}
	return processed, nil
}

// performs concurrent requests to retrieve additional details for existing entities/entity aliases
// these details require explicit requests to vault api for each entitiy/alias
func getExistingEntitiesDetails(entities []entity, threadPoolSize int, customMetadataSupported bool) {
	bwg := utils.NewBoundedWaitGroup(threadPoolSize)

	for i := 0; i < len(entities); i++ {
		bwg.Add(1)

		go func(entity *entity) {
			defer bwg.Done()
			info := vault.GetEntityInfo(entity.Name)

			if _, exists := info["metadata"]; !exists {
				log.WithError(errors.New(fmt.Sprintf(
					"Required `metadata` attribute not found for entity id: %s", entity.Id))).Fatal()
			}
			metadata := info["metadata"].(map[string]interface{})
			if _, exists := info["policies"]; !exists {
				log.WithError(errors.New(fmt.Sprintf(
					"Required `policies` attribute not found for entity id: %s", entity.Id))).Fatal()
			}
			rawPolicies := info["policies"].([]interface{})
			policies := []string{}
			for _, policy := range rawPolicies {
				policies = append(policies, policy.(string))
			}

			// TODO: make this a nested goroutine
			for j := 0; j < len(entity.Aliases); j++ {
				rawAlias := vault.GetEntityAliasInfo(entity.Aliases[j].Id)
				if _, exists := rawAlias["mount_accessor"]; !exists {
					log.WithError(errors.New(fmt.Sprintf(
						"Required `mount_accessor` attribute not found for entity-alias id: %s", entity.Aliases[j].Id))).Fatal()
				}
				entity.Aliases[j].AccessorId = rawAlias["mount_accessor"].(string)
				if customMetadataSupported {
					// deviate from norm and do not fail on missing custom_metadata
					if _, exists := rawAlias["custom_metadata"]; exists {
						entity.Aliases[j].CustomMetadata = make(map[string]interface{})
						aliasMap, ok := entity.Aliases[j].CustomMetadata.(map[string]interface{})
						for k, v := range rawAlias["custom_metadata"].(map[string]interface{}) {
							if ok {
								aliasMap[k] = v
							}
						}
					}
				}
			}

			entity.Policies = policies
			entity.Metadata = make(map[string]interface{})
			metadataMap, ok := entity.Metadata.(map[string]interface{})
			if ok {
				for k, v := range metadata {
					metadataMap[k] = v
				}
			}
		}(&entities[i])
	}
	bwg.Wait()
}

// Calls vault.DiffItems for existing/desired list of aliases, within each exisitng/desired entity
func determineAliasActions(entries, existingEntities []entity, entitiesToBeDeleted []vault.Item) (map[string]map[string][]vault.Item,
	[]vault.Item, map[string][]vault.Item) {

	existingEntityToAliases := make(map[string][]entityAlias)
	for _, entity := range existingEntities {
		existingEntityToAliases[entity.Name] = append(existingEntityToAliases[entity.Name], entity.Aliases...)
	}
	aliasesToBeWritten := make(map[string]map[string][]vault.Item)
	aliasesToBeDeleted := make([]vault.Item, 0)
	aliasesToBeUpdated := make(map[string][]vault.Item)

	for _, entry := range entries {
		w, d, u := vault.DiffItems(aliasesAsItems(entry.Aliases), aliasesAsItems(existingEntityToAliases[entry.Name]))
		// new entities will not have an id.. need to differentiate organization for alias to be written
		// by id for existing entity receiving new alias OR new entity with new aliases
		if entry.Id == "" {
			if len(aliasesToBeWritten["name"]) == 0 {
				aliasesToBeWritten["name"] = make(map[string][]vault.Item)
			}
			aliasesToBeWritten["name"][entry.Name] = append(aliasesToBeWritten["name"][entry.Name], w...)
		} else {
			if len(aliasesToBeWritten["id"]) == 0 {
				aliasesToBeWritten["id"] = make(map[string][]vault.Item)
			}
			aliasesToBeWritten["id"][entry.Id] = append(aliasesToBeWritten["id"][entry.Id], w...)
		}
		aliasesToBeDeleted = append(aliasesToBeDeleted, d...)
		aliasesToBeUpdated[entry.Id] = append(aliasesToBeUpdated[entry.Id], u...)
	}

	// the parent existing entity DNE in desired (to be deleted)
	// treat this as deletion of all affiliated aliases
	for _, e := range entitiesToBeDeleted {
		if _, exists := existingEntityToAliases[e.(entity).Name]; exists {
			aliasesToBeDeleted = append(aliasesToBeDeleted, aliasesAsItems(e.(entity).Aliases)...)
		}
	}
	return aliasesToBeWritten, aliasesToBeDeleted, aliasesToBeUpdated
}

// writes, deletes, and/or updates entity aliases
func performAliasReconcile(aliasesToBeWritten map[string]map[string][]vault.Item,
	aliasesToBeDeleted []vault.Item, aliasesToBeUpdated map[string][]vault.Item, customMetadataSupported bool) error {
	var accessorIds map[string]string
	// extra work (vault api request) required to organize accessor ids
	if len(aliasesToBeWritten) > 0 {
		accessorIds = make(map[string]string)
		authBackends := vault.ListAuthBackends()
		for k, v := range authBackends {
			accessorIds[strings.TrimRight(k, "/")] = v.Accessor
		}
	}
	if _, exists := aliasesToBeWritten["id"]; exists {
		for id, ws := range aliasesToBeWritten["id"] {
			for _, w := range ws {
				a := w.(entityAlias)
				a.AccessorId = accessorIds[a.AuthType]
				a.Create(id, customMetadataSupported)
			}
		}
	}
	if _, exists := aliasesToBeWritten["name"]; exists {
		for name, ws := range aliasesToBeWritten["name"] {
			for _, w := range ws {
				a := w.(entityAlias)
				a.AccessorId = accessorIds[a.AuthType]
				newEntity := vault.GetEntityInfo(name)
				if newEntity == nil {
					return errors.New(fmt.Sprintf(
						"[Vault Identity] failed to get info for newly created entity: %s", name))
				}
				a.Create(newEntity["id"].(string), customMetadataSupported)
			}
		}
	}
	for _, d := range aliasesToBeDeleted {
		d.(entityAlias).Delete()
	}
	for id, us := range aliasesToBeUpdated {
		for _, u := range us {
			u.(entityAlias).Update(id, customMetadataSupported)
		}
	}
	return nil
}

// due to yaml unmarshal limitation, nested objects are initially unmarshalled as json strings
// unmarshallMetadatas targets nested object attributes defined in entity schema and properly unmarshalls
func unmarshallMetadatas(entries []entity) error {
	for i := range entries {
		converted, err := utils.UnmarshalJsonObj("metadata", entries[i].Metadata)
		if err != nil {
			return err
		}
		entries[i].Metadata = converted
		for j := range entries[i].Aliases {
			converted, err = utils.UnmarshalJsonObj("custom_metadata", entries[i].Aliases[j].CustomMetadata)
			if err != nil {
				return err
			}
			entries[i].Aliases[j].CustomMetadata = converted
		}
	}
	return nil
}

// return flag indicating if vault version supports alias custom_metadata attribute
func isCustomMetadataSupported() (bool, error) {
	current, err := version.NewVersion(vault.GetVaultVersion())
	if err != nil {
		return false, err
	}
	threshold, err := version.NewVersion("1.9.0")
	if err != nil {
		return false, err
	}
	if current.LessThan(threshold) {
		return false, nil
	}
	return true, nil
}

// remove unsupported attributes that may be supported in one version of vault but not another
// (commercial v fedramp)
func pruneUnsupported(entries []entity) {
	for _, entry := range entries {
		for _, alias := range entry.Aliases {
			alias.CustomMetadata = nil
		}
	}
}

func entriesAsItems(entries []entity) []vault.Item {
	items := make([]vault.Item, 0)
	for _, entry := range entries {
		items = append(items, entry)
	}
	return items
}

func aliasesAsItems(aliases []entityAlias) []vault.Item {
	items := make([]vault.Item, 0)
	for _, entry := range aliases {
		items = append(items, entry)
	}
	return items
}

// sets Type field for use in vault.DiffItem()
func populateAliasType(entries []entity) {
	for _, entry := range entries {
		for i := 0; i < len(entry.Aliases); i++ {
			entry.Aliases[i].Type = "entity-alias"
		}
	}
}

// sorts policies within entity based upon policy name
func sortPolicies(entries []entity) {
	for _, entry := range entries {
		sort.Strings(entry.Policies)
	}
}

// copy entity and alias Ids off existing entities/aliases to matching desired
// desired entries do not include Ids but ID is required for various operations
func copyIds(entries, existingEntities []entity) {
	existingEntityIds := make(map[string]string)
	// entityName: aliasName: aliasID
	existingEntityAliasIds := make(map[string]map[string]string)
	for _, entity := range existingEntities {
		existingEntityIds[entity.Name] = entity.Id
		existingEntityAliasIds[entity.Name] = make(map[string]string)
		for _, alias := range entity.Aliases {
			existingEntityAliasIds[entity.Name][alias.Name] = alias.Id
		}
	}
	// update entity ids
	for i := 0; i < len(entries); i++ {
		if _, exists := existingEntityIds[entries[i].Name]; exists {
			entries[i].Id = existingEntityIds[entries[i].Name]
		}
	}
	// update aliases ids
	for _, entity := range entries {
		if _, exists := existingEntityAliasIds[entity.Name]; exists {
			for i := 0; i < len(entity.Aliases); i++ {
				if _, exists := existingEntityAliasIds[entity.Name][entity.Aliases[i].Name]; exists {
					entity.Aliases[i].Id = existingEntityAliasIds[entity.Name][entity.Aliases[i].Name]
				}
			}
		}
	}
}

// reusable func to output updates on writes, deletes, and updates for entities
func entitiesDryRunOutput(entities []vault.Item, action string) {
	for _, e := range entities {
		log.WithField("name", e.Key()).WithField("type", e.(entity).Type).Info(
			fmt.Sprintf("[Dry Run] [Vault Identity] entity to be %s", action))
	}
}

// reusable func to output updates on writes, deletes, and updates for entity aliases
func aliasesDryRunOutput(idsToAliases map[string][]vault.Item, action string) {
	for _, aliases := range idsToAliases {
		for _, alias := range aliases {
			log.WithField("name", alias.Key()).WithField("type", alias.(entityAlias).AuthType).Info(
				fmt.Sprintf("[Dry Run] [Vault Identity] entity alias to be %s", action))
		}
	}
}
