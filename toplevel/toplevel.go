// Package toplevel implements a collection of top-level configuration blocks
// used to declarative manage a Vault instance.
package toplevel

import (
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

var (
	configs  = make(map[string]Configuration)
	configsM sync.RWMutex
	// add a new map variable here. the keys will be policy names and the values will be `delete` or `update`
	// your discretion for how to store delete/update value
	// could be an enum?
)

// Configuration represents a block of declarative configuration data that can
// be applied to a service.
//
// If an error occurs applying a configuration, the process should exit.
type Configuration interface {
	Apply(string, []byte, bool, int) error
}

// RegisterConfiguration makes a Configuration available by the provided name.
//
// If called twice with the same name, the name is blank, or if the provided
// Extractor is nil, this function panics.
func RegisterConfiguration(name string, c Configuration) {
	configsM.Lock()
	defer configsM.Unlock()

	if name == "" {
		panic("toplevel: could not register a Configuration with an empty name")
	}

	if c == nil {
		panic("toplevel: could not register a nil Configuration")
	}

	name = strings.ToLower(name)

	if _, dup := configs[name]; dup {
		panic("toplevel: RegisterConfiguration called twice for " + name)
	}

	configs[name] = c
}

// Apply looks up registered top-level configuration by name and applies it an
// instance of Vault.
func Apply(name string, address string, cfg []byte, dryRun bool, threadPoolSize int) error {
	configsM.RLock()
	defer configsM.RUnlock()
	c, ok := configs[name]
	if !ok {
		log.WithField("name", name).Fatal("failed to find top-level configuration")
	}
	return c.Apply(address, cfg, dryRun, threadPoolSize)
}

// a function to update the package level polices variable
// this will need to accept written / deleted policies from the policy toplevel Apply function
// those objects are of type vault.Item (defined within pkg/reconcile.go)
// this func needs to iterate over both lists and populate the package level variable

// a simple get function that returns the package level policies variable

// a function to clear the package level policies variable
// recall that vault-manager reconciles multiple vault instances
// changed policies needs to be cleared before the next instance is reconciled
