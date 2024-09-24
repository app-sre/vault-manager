#!/bin/bash

PODMAN_CONF="$PWD/.config/containers"
mkdir -p "$PODMAN_CONF"


podman login -u="$QUAY_USER" -p="$QUAY_TOKEN" quay.io

# compile sources and run unit tests
make build-podman

# run e2e tests
source .env
make test-podman
