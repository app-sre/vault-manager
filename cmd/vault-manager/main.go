package main

import (
	"flag"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"os"
	"sort"

	"github.com/app-sre/vault-manager/toplevel"

	// Register top-level configurations.
	_ "github.com/app-sre/vault-manager/toplevel/audit"
	_ "github.com/app-sre/vault-manager/toplevel/auth"
	_ "github.com/app-sre/vault-manager/toplevel/policies-mapping"
	_ "github.com/app-sre/vault-manager/toplevel/policy"
	_ "github.com/app-sre/vault-manager/toplevel/role"
	_ "github.com/app-sre/vault-manager/toplevel/secretsengine"
)

type TopLevelConfig struct {
	Name     string
	Priority int
}

type ByPriority []TopLevelConfig

func (a ByPriority) Len() int {
	return len(a)
}
func (a ByPriority) Less(i, j int) bool {
	return a[i].Priority < a[j].Priority
}
func (a ByPriority) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func main() {
	var dryRun bool
	var force bool
	flag.BoolVar(&dryRun, "dry-run", false, "If true, will only print planned actions")
	flag.BoolVar(&force, "force", false, "If true, will force potentially unsafe write actions")
	flag.Parse()

	cfg, err := readConfig()
	if err != nil {
		logrus.WithError(err).Fatal("failed to parse config")
	}

	for t, e := range cfg {
		if force == false {
			if e == nil {
				logrus.Fatalf("top-level configuration key '%v' does not have any entry, this will lead to removing all existing configurations of this top-level key in vault, use -force to force this operation", t)
			}
		} else {
			if e == nil {
				logrus.Warningf("top-level configuration key '%v' does not have any entry, this will lead to removing all existing configurations of this top-level key in vault", t)
			}
		}
	}

	topLevelConfigs := []TopLevelConfig{}

	for key := range cfg {
		c := TopLevelConfig{key, resolveConfigPriority(key)}
		topLevelConfigs = append(topLevelConfigs, c)
	}

	// sort configs by priority
	sort.Sort(ByPriority(topLevelConfigs))

	for _, config := range topLevelConfigs {
		// Marshal the contents of this object back into bytes so that it can be
		// unmarshaled into a specific type in the application.
		dataBytes, err := yaml.Marshal(cfg[config.Name])
		if err != nil {
			logrus.WithField("name", config.Name).Fatal("failed to remarshal configuration")
		}
		toplevel.Apply(config.Name, dataBytes, dryRun)
	}
}

type configFile map[string]interface{}

func readConfig() (configFile, error) {
	configPath := os.Getenv("VAULT_MANAGER_CONFIG_FILE")
	if configPath == "" {
		configPath = "vault-manager.yaml"
	}

	f, err := os.Open(os.ExpandEnv(configPath))
	if err != nil {
		return configFile{}, errors.Wrap(err, "failed to open configuration file")
	}
	defer f.Close()

	var cfg configFile
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return configFile{}, errors.Wrap(err, "failed to decode configuration file")
	}

	return cfg, nil
}

func resolveConfigPriority(s string) int {
	var priority int
	switch s {
	case "policies":
		priority = 1
	case "audit":
		priority = 2
	case "secrets-engines":
		priority = 3
	case "auth":
		priority = 4
	case "roles":
		priority = 5
	case "policies-mapping":
		priority = 6
	default:
		priority = 0
	}
	return priority
}
