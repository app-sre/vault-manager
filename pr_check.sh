#!/bin/bash


DOCKER_CONF="$PWD/.docker"
mkdir -p "$DOCKER_CONF"
docker --config="$DOCKER_CONF" login -u="$QUAY_USER" -p="$QUAY_TOKEN" quay.io

# compile sources and run unit tests
make build

export QONTRACT_SERVER_NAME=qontract-server
export QONTRACT_SERVER_IMAGE=quay.io/app-sre/qontract-server
export QONTRACT_SERVER_IMAGE_TAG=64b433b

export KEYCLOAK_NAME=keycloak
export KEYCLOAK_IMAGE=quay.io/keycloak/keycloak
export KEYCLOAK_IMAGE_TAG=17.0

export KEYCLOAK_CLI_NAME=keycloak_cli
export KEYCLOAK_CLI_IMAGE=quay.io/app-sre/keycloak-config-cli
export KEYCLOAK_CLI_IMAGE_TAG=v4.9.0-rc1-17.0.0

export VAULT_NAME=vault-dev-server
export VAULT_IMAGE=quay.io/app-sre/vault
export VAULT_IMAGE_TAG=1.5.4

# run e2e tests
make test
