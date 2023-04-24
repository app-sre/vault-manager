package vault

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/app-sre/vault-manager/pkg/utils"
	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/api/auth/kubernetes"
	log "github.com/sirupsen/logrus"
)

type Instance struct {
	Address string `yaml:"address"`
	Auth    auth   `yaml:"auth"`
}

type auth struct {
	Provider     string `yaml:"provider"`
	KubeRoleName string `yaml:"kubeRoleName"`
	SecretEngine string `yaml:"secretEngine"`
	RoleID       secret `yaml:"roleID"`
	SecretID     secret `yaml:"secretID"`
	Token        secret `yaml:"token"`
}

type secret struct {
	Path    string `yaml:"path"`
	Field   string `yaml:"field"`
	Version string `yaml:"version"`
}

type VaultSecret struct {
	Name    string
	Type    string
	Path    string
	Field   string
	Version string
}

type AuthBundle struct {
	KubeRoleName string
	SecretEngine string
	VaultSecrets []*VaultSecret
}

// names to assign to access attributes
const (
	ROLE_ID      = "roleID"
	SECRET_ID    = "secretID"
	TOKEN        = "token"
	APPROLE_AUTH = "approle"
	TOKEN_AUTH   = "token"
	KV_V1        = "kv_v1"
	KV_V2        = "kv_v2"
)

const (
	// How many times attempt to retry when failing
	// to retrieve a valid client token.
	defaultTokenRetryAttempts = 5

	// How long to sleep in between each retry attempt.
	defaultTokenRetrySleep = 250 * time.Millisecond
)

// global that maps instance addresses to configured vault clients
// initialization process is triggered by call to GetInstances()
// GetInstances() is called a single time within main
var vaultClients map[string]*api.Client

// errClientTokenInvalid indicates a presence of an invalid client token.
//
// There might have been an empty authentication information response
// sent back from a remote Vault server that does not contain a valid
// client token.
var errClientTokenInvalid = errors.New("client token is not valid")

// Utilized to initialize vault instance clients for use by other toplevel integrations
// returns list of instance addresses being included in reconcile
func GetInstances(entriesBytes []byte, kubeAuth bool, threadPoolSize int) []string {
	var instances []Instance
	if err := yaml.Unmarshal(entriesBytes, &instances); err != nil {
		log.WithError(err).Fatal("[Vault Instance] failed to decode instance configuration")
	}

	instanceCreds, err := processInstances(instances, kubeAuth)
	if err != nil {
		log.WithError(err).Fatal("[Vault Instance] failed to retrieve access credentials")
	}
	initClients(instanceCreds, threadPoolSize)

	// return list of addresses that clients were initialized for
	addresses := []string{}
	for address := range vaultClients {
		addresses = append(addresses, address)
	}
	return addresses
}

// generates map of instance addresses to access credentials stored in master vault
func processInstances(instances []Instance, kubeAuth bool) (map[string]AuthBundle, error) {
	instanceCreds := make(map[string]AuthBundle)
	for _, i := range instances {
		bundle := AuthBundle{}
		// conditions should only be met within cluster deployments
		if kubeAuth && len(i.Auth.KubeRoleName) > 0 {
			bundle.KubeRoleName = i.Auth.KubeRoleName
		} else {
			bundle.SecretEngine = i.Auth.SecretEngine
			switch strings.ToLower(i.Auth.Provider) {
			case APPROLE_AUTH:
				// ensure required values exist
				if i.Auth.RoleID.Field == "" || i.Auth.RoleID.Path == "" ||
					i.Auth.SecretID.Field == "" || i.Auth.SecretID.Path == "" {
					return nil, errors.New("A required approle authentication attribute is missing")
				}
				bundle.VaultSecrets = []*VaultSecret{
					{
						Name:    ROLE_ID,
						Type:    APPROLE_AUTH,
						Path:    i.Auth.RoleID.Path,
						Field:   i.Auth.RoleID.Field,
						Version: i.Auth.RoleID.Version,
					},
					{
						Name:    SECRET_ID,
						Type:    APPROLE_AUTH,
						Path:    i.Auth.SecretID.Path,
						Field:   i.Auth.SecretID.Field,
						Version: i.Auth.SecretID.Version,
					},
				}
			case TOKEN_AUTH:
				if i.Auth.Token.Field == "" || i.Auth.Token.Path == "" {
					return nil, errors.New("A required token authentication attribute is missing")
				}
				bundle.VaultSecrets = []*VaultSecret{
					{
						Name:    TOKEN,
						Type:    TOKEN_AUTH,
						Path:    i.Auth.Token.Path,
						Field:   i.Auth.Token.Field,
						Version: i.Auth.Token.Version,
					},
				}
			default:
				return nil, errors.New(fmt.Sprintf(
					"Unable to process `auth` attribute of instance definition with address %s", i.Address))
			}
		}
		instanceCreds[i.Address] = bundle
	}
	return instanceCreds, nil
}

// Creates global map of all vault clients defined in a-i
// This allows reconciliation of multiple vault instances
func initClients(instanceCreds map[string]AuthBundle, threadPoolSize int) {
	vaultClients = make(map[string]*api.Client) // THIS IS THE GLOBAL
	masterAddress := configureMaster(instanceCreds)
	bwg := utils.NewBoundedWaitGroup(threadPoolSize)
	var mutex = &sync.Mutex{}
	// read access credentials for other vault instances and configure clients
	for addr, bundle := range instanceCreds {
		// client already configured separately for master
		if addr != masterAddress {
			bwg.Add(1)
			go createClient(addr, masterAddress, bundle, &bwg, mutex)
		}
	}
	bwg.Wait()
}

// configureMaster initializes vault client for the master instance
// This is the only client that can be configured using environment variables
// env vars: VAULT_ADDR, VAULT_AUTHTYPE, VAULT_ROLE_ID, VAULT_SECRET_ID, VAULT_TOKEN
func configureMaster(instanceCreds map[string]AuthBundle) string {
	masterVaultCFG := api.DefaultConfig()
	masterVaultCFG.Address = mustGetenv("VAULT_ADDR")

	client, err := api.NewClient(masterVaultCFG)
	if err != nil {
		log.WithError(err).Fatal("failed to initialize master Vault client")
	}

	masterAuthBundle := instanceCreds[masterVaultCFG.Address]
	// indicates kube auth should be utilized
	if len(masterAuthBundle.KubeRoleName) > 0 {
		err := configureKubeAuthClient(client, masterAuthBundle)
		if err != nil {
			log.WithError(err).Fatal("[Vault Client] failed to configure master client using kube auth")
		}
	} else {
		var clientToken string
		switch authType := defaultGetenv("VAULT_AUTHTYPE", "approle"); strings.ToLower(authType) {
		case APPROLE_AUTH:
			roleID := mustGetenv("VAULT_ROLE_ID")
			secretID := mustGetenv("VAULT_SECRET_ID")

			err := utils.Retry(defaultTokenRetryAttempts, defaultTokenRetrySleep, func() error {
				secret, err := client.Logical().Write("auth/approle/login", map[string]interface{}{
					"role_id":   roleID,
					"secret_id": secretID,
				})
				if err != nil {
					return utils.RetryStop(err)
				}
				// Writing to the AppRole mount endpoint to "login" to obtain a new
				// token might return an empty response without necessarily failing.
				if secret.Auth == nil || secret.Auth.ClientToken == "" {
					log.Warn("[Vault Client] received empty authentication information. Retrying...")
					return errClientTokenInvalid
				}
				clientToken = secret.Auth.ClientToken
				return nil
			})
			if err != nil {
				log.WithError(err).Fatal("[Vault Client] failed to login to master Vault with AppRole")
			}
		case TOKEN_AUTH:
			clientToken = mustGetenv("VAULT_TOKEN")
		default:
			log.WithField("authType", authType).Fatal("[Vault Client] unsupported auth type")
		}
		client.SetToken(clientToken)
	}

	vaultClients[masterVaultCFG.Address] = client
	return masterVaultCFG.Address
}

func configureKubeAuthClient(client *api.Client, bundle AuthBundle) error {
	mount := mustGetenv("KUBE_AUTH_MOUNT")
	kubeSATokenPath := mustGetenv("KUBE_SA_TOKEN_PATH")
	kubeAuth, err := kubernetes.NewKubernetesAuth(
		bundle.KubeRoleName,
		kubernetes.WithServiceAccountTokenPath(kubeSATokenPath),
		kubernetes.WithMountPath(mount),
	)
	if err != nil {
		return err
	}
	ctxTimeout, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	authInfo, err := client.Auth().Login(ctxTimeout, kubeAuth)
	if err != nil {
		return err
	}
	if authInfo == nil {
		return errors.New("[Vault Client] no auth info was returned after kube login")
	}
	return nil
}

// goroutine support function for initClients()
// initializes one vault client
func createClient(addr string,
	masterAddress string,
	bundle AuthBundle,
	bwg *utils.BoundedWaitGroup,
	mutex *sync.Mutex) {

	defer bwg.Done()

	config := api.DefaultConfig()
	config.Address = addr
	client, err := api.NewClient(config)
	if err != nil {
		log.WithError(err).Errorf("[Vault Client] failed to initialize Vault client for %s", addr)
		log.Warnf("SKIPPING ALL RECONCILIATION FOR: %s\n", addr)
		return // skip entire reconcilation for this instance
	}

	// indicates kube auth should be utilized
	if len(bundle.KubeRoleName) > 0 {
		err := configureKubeAuthClient(client, bundle)
		if err != nil {
			log.WithError(err).Errorf("[Vault Client] failed to login to %s with kube sa token", addr)
			log.Warnf("SKIPPING ALL RECONCILIATION FOR: %s\n", addr)
			return // skip entire reconcilation for this instance
		}
	} else {
		accessCreds := make(map[string]string)
		for _, cred := range bundle.VaultSecrets {
			// masterAddress hard-coded because all "child" vault access credentials must be pulled from master
			processedCred, err := GetVaultSecretField(masterAddress, cred.Path, cred.Field, bundle.SecretEngine)
			if err != nil {
				log.WithError(err).Fatal()
			}
			accessCreds[cred.Name] = processedCred
		}

		// at minimum, one element will exist in secrets regardless of type
		// type is same across all VaultSecrets associated with a particular instance address
		var clientToken string
		switch bundle.VaultSecrets[0].Type {
		case APPROLE_AUTH:
			err := utils.Retry(defaultTokenRetryAttempts, defaultTokenRetrySleep, func() error {
				secret, err := client.Logical().Write("auth/approle/login", map[string]interface{}{
					"role_id":   accessCreds[ROLE_ID],
					"secret_id": accessCreds[SECRET_ID],
				})
				if err != nil {
					return utils.RetryStop(err)
				}
				// Writing to the AppRole mount endpoint to "login" to obtain a new
				// token might return an empty response without necessarily failing.
				if secret.Auth == nil || secret.Auth.ClientToken == "" {
					log.Warn("[Vault Client] received empty authentication information. Retrying...")
					return errClientTokenInvalid
				}
				clientToken = secret.Auth.ClientToken
				return nil
			})
			if err != nil {
				log.WithError(err).Errorf("[Vault Client] failed to login to %s with AppRole credentials", addr)
				log.Warnf("SKIPPING ALL RECONCILIATION FOR: %s\n", addr)
				return // Skip entire reconciliation for this instance.
			}
		case TOKEN_AUTH:
			clientToken = accessCreds[TOKEN]
		}
		client.SetToken(clientToken)
	}

	// test client
	_, err = client.Sys().ListAuth()
	if err != nil {
		log.WithError(err).Errorf("[Vault Client] failed to login to %s", addr)
		log.Warnf("SKIPPING ALL RECONCILIATION FOR: %s\n", addr)
		return
	}

	// add new address/client pair to global
	mutex.Lock()
	defer mutex.Unlock()
	vaultClients[addr] = client
}

// returns the vault client associated with instance address
func getClient(instanceAddr string) *api.Client {
	if vaultClients[instanceAddr] == nil {
		log.Fatalf("[Vault Client] client does not exist for address: %s", instanceAddr)
	}
	return vaultClients[instanceAddr]
}
