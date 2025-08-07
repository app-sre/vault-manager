#!/bin/bash

# compile sources and run unit tests
make build

# run integration tests
source .env
# make gobuild
make test-testcontainers-shared
