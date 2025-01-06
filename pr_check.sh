#!/bin/bash

# DOCKER_CONF="$PWD/.docker"
# mkdir -p "$DOCKER_CONF"
# docker --config="$DOCKER_CONF" login -u="$QUAY_USER" -p="$QUAY_TOKEN" quay.io

# compile sources and run unit tests
make build

# run e2e tests
source .env
make test-with-compose
