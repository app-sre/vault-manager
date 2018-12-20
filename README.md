# vault-manager
vault-manager is an automation tool for configuration hashicorp vault based on [Vault GO API Client](https://github.com/hashicorp/vault/tree/master/api)


# docker run --network=host -v $(pwd)/vault-manager-prod.yaml:/vault-manager-prod.yaml --env-file ./dev-env-docker --rm -i quay.io/app-sre/vault-manager:latest -dry-run
