package entity

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/app-sre/vault-manager/pkg/utils"
	"github.com/app-sre/vault-manager/pkg/vault"
	"github.com/app-sre/vault-manager/toplevel"
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
	Name     string         `yaml:"name"`
	Service  string         `yaml:"service"`
	Instance vault.Instance `yaml:"instance"`
}

type entity struct {
	Name     string
	Id       string
	Type     string
	Metadata interface{}
	Aliases  []entityAlias
	Instance vault.Instance
}

type entityAlias struct {
	Name       string
	Id         string
	Type       string
	AuthType   string
	AccessorId string
	Instance   vault.Instance
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
		reflect.DeepEqual(e.Metadata, entry.Metadata)
}

func (e entity) CreateOrUpdate(action string) error {
	path := filepath.Join("identity", e.Type, "name", e.Name)
	config := map[string]interface{}{
		"metadata": e.Metadata,
	}
	err := vault.WriteSecret(e.Instance.Address, path, vault.KV_V1, config)
	if err != nil {
		return err
	}
	log.WithFields(log.Fields{
		"instance": e.Instance.Address,
		"path":     path,
		"type":     e.KeyForType(),
	}).Infof("[Vault Identity] entity successfully %s", action)
	return nil
}

func (e entity) Delete() error {
	path := filepath.Join("identity", e.Type, "name", e.Name)
	err := vault.DeleteSecret(e.Instance.Address, path)
	if err != nil {
		return err
	}
	log.WithFields(log.Fields{
		"instance": e.Instance.Address,
		"path":     path,
		"type":     e.KeyForType(),
	}).Info("[Vault Identity] entity successfully deleted")
	return nil
}

func (e entityAlias) Key() string {
	return e.Name
}

func (e entityAlias) KeyForType() string {
	return e.Type
}

func (e entityAlias) KeyForDescription() string {
	return e.AuthType
}

func (e entityAlias) Equals(i interface{}) bool {
	entry, ok := i.(entityAlias)
	if !ok {
		return false
	}
	return e.Name == entry.Name &&
		e.AuthType == entry.AuthType
}

func (ea entityAlias) Create(entityId string) error {
	path := filepath.Join("identity", ea.Type)
	config := map[string]interface{}{
		"name":           ea.Name,
		"canonical_id":   entityId,
		"mount_accessor": ea.AccessorId,
	}
	err := vault.WriteEntityAlias(ea.Instance.Address, path, config)
	if err != nil {
		return err
	}
	log.WithFields(log.Fields{
		"instance": ea.Instance.Address,
		"path":     filepath.Join(path, ea.Name),
		"type":     ea.AuthType,
	}).Info("[Vault Identity] entity alias successfully written")
	return nil
}

func (ea entityAlias) Update(entityId string) error {
	path := filepath.Join("identity", ea.Type, "id", ea.Id)
	config := map[string]interface{}{
		"name":           ea.Name,
		"canonical_id":   entityId,
		"mount_accessor": ea.AccessorId,
	}
	err := vault.WriteSecret(ea.Instance.Address, path, vault.KV_V1, config)
	if err != nil {
		return err
	}
	log.WithFields(log.Fields{
		"instance": ea.Instance.Address,
		"path":     filepath.Join(path, ea.Name),
		"type":     ea.AuthType,
	}).Info("[Vault Identity] entity alias successfully updated")
	return nil
}

func (ea entityAlias) Delete() error {
	path := filepath.Join("identity", ea.Type, "id", ea.Id)
	err := vault.DeleteSecret(ea.Instance.Address, path)
	if err != nil {
		return err
	}
	log.WithFields(log.Fields{
		"instance": ea.Instance.Address,
		"path":     filepath.Join(path, ea.Name),
		"type":     ea.AuthType,
	}).Info("[Vault Identity] entity alias successfully deleted")
	return nil
}

func init() {
	toplevel.RegisterConfiguration("vault_entities", config{})
}

func (c config) Apply(address string, entriesBytes []byte, dryRun bool, threadPoolSize int) error {
	// process desired entities/aliases
	var entries []user
	if err := yaml.Unmarshal(entriesBytes, &entries); err != nil {
		log.WithError(err).Fatal("[Vault Identity] failed to decode entity configuration")
	}

	desired := getDesired(address, entries)
	populateAliasType(desired)

	// Process data on existing entities/aliases
	existingEntities, err := createBaseExistingEntities(address)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"instance": address,
		}).Info("[Vault Identity] failed to parse existing entities")
		return err
	}

	pruneNonOidcEntities(&existingEntities)

	if existingEntities != nil && len(existingEntities) > 0 {
		err := getExistingEntitiesDetails(address, existingEntities, threadPoolSize)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"instance": address,
			}).Info("[Vault Identity] failed to gather existing entity details")
			return err
		}
		populateAliasType(existingEntities)
		copyIds(desired, existingEntities)
	}

	// determine entity changes
	entitiesToBeWritten, entitiesToBeDeleted, entitiesToBeUpdated :=
		vault.DiffItems(entriesAsItems(desired), entriesAsItems(existingEntities))
	// determine entity alias changes
	aliasesToBeWritten, aliasesToBeDeleted, aliasesToBeUpdated :=
		determineAliasActions(desired, existingEntities, entitiesToBeDeleted)

	// preform actions
	if dryRun {
		entitiesDryRunOutput(address, entitiesToBeWritten, "written")
		entitiesDryRunOutput(address, entitiesToBeDeleted, "deleted")
		entitiesDryRunOutput(address, entitiesToBeUpdated, "updated")
		aliasesDryRunOutput(address, aliasesToBeWritten["id"], "written")
		aliasesDryRunOutput(address, aliasesToBeWritten["name"], "written")
		for _, alias := range aliasesToBeDeleted {
			log.WithFields(log.Fields{
				"name":     alias.Key(),
				"type":     alias.(entityAlias).AuthType,
				"instance": address,
			}).Info("[Dry Run] [Vault Identity] entity alias to be deleted")
		}
		aliasesDryRunOutput(address, aliasesToBeUpdated, "updated")
	} else {
		// TODO: make each action perform concurrently
		for _, w := range entitiesToBeWritten {
			err := w.(entity).CreateOrUpdate("written")
			if err != nil {
				return err
			}
		}
		for _, d := range entitiesToBeDeleted {
			err := d.(entity).Delete()
			if err != nil {
				return err
			}
		}
		for _, u := range entitiesToBeUpdated {
			err := u.(entity).CreateOrUpdate("update")
			if err != nil {
				return err
			}
		}
		err = performAliasReconcile(address, aliasesToBeWritten, aliasesToBeDeleted, aliasesToBeUpdated)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"instance": address,
			}).Info("[Vault Identity] error occurred during reconciliation of entity aliases")
			return err
		}
	}

	return nil
}

// getDesired accepts the yaml-marshalled result of the `vault_entities` graphql
// query and returns entity/entity-alias object slice of desired for particular instance address
func getDesired(address string, entries []user) []entity {
	desired := []entity{}
	// need to track org name
	// a user file can ref multi roles but user should only be appended once
	existing := make(map[string]bool)

	for _, u := range entries {
		for _, r := range u.Roles {
			for _, p := range r.Permissions {
				// only process first occurence of oidc ref for a user
				// and only process oidc permissions for vault service
				// and only process references to particular instance being reconciled
				if !existing[u.OrgUsername] && p.Service == "vault" && p.Instance.Address == address {
					newDesired := entity{
						Name: u.OrgUsername,
						Type: "entity",
						Aliases: []entityAlias{
							{
								Name:     u.OrgUsername,
								Type:     "entity-alias",
								AuthType: "oidc",
								Instance: p.Instance,
							},
						},
						Metadata: map[string]interface{}{
							"name": u.Name,
						},
						Instance: p.Instance,
					}
					desired = append(desired, newDesired)
					// ensure no further entities are added for this user in this instance
					existing[u.OrgUsername] = true
				}
			}
		}
	}
	return desired
}

// processes all relevant info for entities/entity aliases from single vault api request
func createBaseExistingEntities(instanceAddr string) ([]entity, error) {
	raw, err := vault.ListEntities(instanceAddr)
	if err != nil {
		return nil, err
	}
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
					"Required `name` attribute not found on alias element for entity id: %s", id))
			}
			aliasName := vals["name"].(string)

			mountType := ""
			if _, exists := vals["mount_type"]; !exists {
				accessor, exists := vals["mount_accessor"]
				if !exists {
					return nil, errors.New(fmt.Sprintf(
						"Required `mount_accessor` attribute not found on alias element for entity id: %s", id))
				}
				// some aliases do not contain mount_type. avoid error for these as irrelevant to current reconcile
				// ex: userpass entity-aliases
				if strings.Contains(accessor.(string), "oidc") {
					return nil, errors.New(fmt.Sprintf(
						"Required `mount_type` attribute not found on alias element for entity id: %s", id))
				}
			} else {
				mountType = vals["mount_type"].(string)
			}

			processedAliases = append(processedAliases, entityAlias{
				Id:       aliasId,
				Name:     aliasName,
				AuthType: mountType,
				Instance: vault.Instance{Address: instanceAddr},
			})
		}

		processed = append(processed, entity{
			Name:     name,
			Id:       id,
			Type:     "entity", // used for reconcile and output
			Aliases:  processedAliases,
			Instance: vault.Instance{Address: instanceAddr},
		})
	}
	return processed, nil
}

// performs concurrent requests to retrieve additional details for existing entities/entity aliases
// these details require explicit requests to vault api for each entitiy/alias
func getExistingEntitiesDetails(instanceAddr string, entities []entity, threadPoolSize int) error {
	bwg := utils.NewBoundedWaitGroup(threadPoolSize)
	ch := make(chan error)

	for i := 0; i < len(entities); i++ {
		bwg.Add(1)

		go func(e *entity, ch chan<- error) {
			defer bwg.Done()

			info, err := vault.GetEntityInfo(instanceAddr, e.Name)
			if err != nil {
				ch <- err
				return
			}
			if info == nil {
				ch <- errors.New(fmt.Sprintf("No information returned for entity id: %s", e.Id))
				return
			}
			if _, exists := info["metadata"]; !exists {
				ch <- errors.New(fmt.Sprintf("Required `metadata` attribute not found for entity id: %s", e.Id))
				return
			}
			var metadata map[string]interface{}
			if info["metadata"] == nil {
				metadata = nil
			} else {
				metadata = info["metadata"].(map[string]interface{})
			}

			// TODO: make this a nested goroutine
			for j := 0; j < len(e.Aliases); j++ {
				rawAlias, err := vault.GetEntityAliasInfo(instanceAddr, e.Aliases[j].Id)
				if err != nil {
					ch <- err
					return
				}
				mountAccessor, ok := rawAlias["mount_accessor"].(string)
				if !ok {
					ch <- errors.New(fmt.Sprintf(
						"Unable to retrieve required `mount_accessor` attribute for entity-alias id: %s", e.Aliases[j].Id))
					return
				}
				e.Aliases[j].AccessorId = mountAccessor
			}

			e.Metadata = make(map[string]interface{})
			metadataMap, ok := e.Metadata.(map[string]interface{})
			if ok {
				for k, v := range metadata {
					metadataMap[k] = v
				}
			} else {
				ch <- errors.New(fmt.Sprintf("Entity Alias metatdata conversion failed"))
			}
		}(&entities[i], ch)
	}

	// do not close channel until all goroutines finish
	func() {
		bwg.Wait()
		close(ch)
	}()

	// wait indefinitely for error responses until close() is called
	for e := range ch {
		if e != nil {
			return e
		}
	}
	return nil
}

// removes existing entities that have been flagged as being connected to approles, github, etc
// from inclusion in reconcile process
func pruneNonOidcEntities(entities *[]entity) {
	var isOidc bool
	i := 0
	for _, e := range *entities {
		isOidc = true
		// ignore entire entity if a single alias is not oidc type
		for _, a := range e.Aliases {
			if a.AuthType != "oidc" {
				isOidc = false
				break
			}
		}
		if isOidc {
			(*entities)[i] = e
			i++
		}
	}
	*entities = (*entities)[:i]
}

// calls vault.DiffItems for existing/desired list of aliases, within each exisitng/desired entity
// vault.DiffItem cannot adequately handle reconcile of aliases in "top level" diffItem of entities
// this logic goes a layer deeper and compares aliases of a entities one at a time
func determineAliasActions(entries, existingEntities []entity, entitiesToBeDeleted []vault.Item) (map[string]map[string][]vault.Item,
	[]vault.Item, map[string][]vault.Item) {

	// ds to quickly pull applicable aliases for diff against desired
	// using existing entites, map entity name to list of associated aliases
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
	// this is redundant "to be certain" logic as vault should remove associated aliases when entity is deleted
	for _, e := range entitiesToBeDeleted {
		if _, exists := existingEntityToAliases[e.(entity).Name]; exists {
			aliasesToBeDeleted = append(aliasesToBeDeleted, aliasesAsItems(e.(entity).Aliases)...)
		}
	}
	return aliasesToBeWritten, aliasesToBeDeleted, aliasesToBeUpdated
}

// writes, deletes, and/or updates entity aliases
func performAliasReconcile(instanceAddr string, aliasesToBeWritten map[string]map[string][]vault.Item,
	aliasesToBeDeleted []vault.Item, aliasesToBeUpdated map[string][]vault.Item) error {
	var accessorIds map[string]string
	// extra work (vault api request) required to organize accessor ids
	if len(aliasesToBeWritten) > 0 {
		accessorIds = make(map[string]string)
		authBackends, err := vault.ListAuthBackends(instanceAddr)
		if err != nil {
			return err
		}
		for k, v := range authBackends {
			accessorIds[strings.TrimRight(k, "/")] = v.Accessor
		}
	}
	if _, exists := aliasesToBeWritten["id"]; exists {
		for id, ws := range aliasesToBeWritten["id"] {
			for _, w := range ws {
				a := w.(entityAlias)
				a.AccessorId = accessorIds[a.AuthType]
				err := a.Create(id)
				if err != nil {
					return err
				}
			}
		}
	}
	// recall, a new entity was created for entries in this ds
	// therefore, additional call is required to find the id of the new entity
	// in order to associate the new aliases
	if _, exists := aliasesToBeWritten["name"]; exists {
		for name, ws := range aliasesToBeWritten["name"] {
			for _, w := range ws {
				a := w.(entityAlias)
				a.AccessorId = accessorIds[a.AuthType]
				newEntity, err := vault.GetEntityInfo(instanceAddr, name)
				if err != nil {
					return err
				}
				if newEntity == nil {
					return errors.New(fmt.Sprintf(
						"[Vault Identity] failed to get info for newly created entity: %s", name))
				}
				a.Create(newEntity["id"].(string))
			}
		}
	}
	for _, d := range aliasesToBeDeleted {
		d.(entityAlias).Delete()
	}
	for id, us := range aliasesToBeUpdated {
		for _, u := range us {
			u.(entityAlias).Update(id)
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
	}
	return nil
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
func entitiesDryRunOutput(instanceAddr string, entities []vault.Item, action string) {
	for _, e := range entities {
		log.WithFields(log.Fields{
			"name":     e.Key(),
			"type":     e.KeyForType(),
			"instance": instanceAddr,
		}).Infof("[Dry Run] [Vault Identity] entity to be %s", action)
	}
}

// reusable func to output updates on writes, deletes, and updates for entity aliases
func aliasesDryRunOutput(instanceAddr string, idsToAliases map[string][]vault.Item, action string) {
	for _, aliases := range idsToAliases {
		for _, alias := range aliases {
			log.WithFields(log.Fields{
				"name":     alias.Key(),
				"type":     alias.(entityAlias).AuthType,
				"instance": instanceAddr,
			}).Infof("[Dry Run] [Vault Identity] entity alias to be %s", action)
		}
	}
}
