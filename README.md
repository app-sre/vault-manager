# vault-manager
vault-manager is an automation tool for managing [hashicorp vault](https://github.com/hashicorp/vault) configurations based on [Vault GO API Client](https://github.com/hashicorp/vault/tree/master/api)

# how to use
```bash
docker run --rm -t \
           -v <PATH_TO_VAULT_MANAGER_CONFIG>:/vault-devshift-net-config.yaml \
           -e VAULT_MANAGER_CONFIG_FILE=vault-devshift-net-config.yaml \
           -e VAULT_ADDR=<VAULT_INSTANCE_URL> \
           -e VAULT_AUTHTYPE=approle \
           -e VAULT_ROLE_ID=<APPROLE_ROLE_ID> \
           -e VAULT_SECRET_ID=<APPROLE_SECRET_ID> \
           quay.io/app-sre/vault-manager:latest -dry-run
```
Note that running vault-manager with -dry-run flag will only print planned actions,
remove this flag to make changes enter into effect

## Flags
- `-dry-run`, default=false<br>
runs vault-manager in dry-run mode and only print planned actions
- `-force`, default=false<br>
By default vault-manager will fail if any top-level configuration entry is empty since it leads to removing all appropriate existing config entries in vault. So this flag can be used if you are really want to remove all top-level configuration entries.

# Example vault-manager configuration file
<details><summary>vault-manager.yaml</summary>
<p>

```yaml

---
audit:
  - path: "file1/"
    type: "file"
    description: "first_logger"
    options:
      file_path: "/tmp/log1.log"
      log_raw: "false"
      mode: "0600"
      format: "json"
  - path: "approle-1/"
    type: "approle"
    description: "approle-1 auth backend"
  - path: "github-test-1/"
    type: "github"
    description: "github-test-1 auth backend"
    github-config:
      organization: "test-org-1"
      base_url: ""
      max_ttl: "72h"
      ttl: "72h"
approle:
  - name: "test-role-1"
    options:
      local_secret_ids: "false"
      token_bound_cidrs: []
      bound_cidr_list: []
      secret_id_bound_cidrs: []
      secret_id_num_uses: "0"
      bind_secret_id: "true"
      period: "0s"
      secret_id_ttl: "0s"
      token_num_uses: "1"
      token_ttl: "30m"
      token_max_ttl: "30m"
      policies:
        - policy-test-1
        - policy-test-2
secrets-engines:
  - path: "secrets-test-1/"
    type: "kv"
    description: "this is first kv secrets engine"
  - path: "secrets-test-2/"
    type: "kv"
    description: "this is second kv v2 secrets engine"
    options:
      version: "2"
policies:
  - name: "policy-test-1"
    rules: |
      path "secret-test1/*" {
        capabilities = ["create", "read", "update", "delete", "list"]
      }
  - name: "policy-test-2"
    rules: |
      path "secret-test2/*" {
        capabilities = ["create", "read", "update", "delete", "list"]
      }
gh-policy-mappings:
  - entity-name: "test-team-1"
    gh-mount-name: "github"
    entity-group: "teams"
    policies: "policy-test-1,policy-test-2"
  - entity-name: "test-user-1"
    gh-mount-name: "github"
    entity-group: "users"
    policies: "policy-test-1"
```
</p>
</details>

More examples can be found [here](https://github.com/app-sre/vault-manager/tree/master/tests/fixtures)
