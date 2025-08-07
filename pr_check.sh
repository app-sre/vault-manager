#!/bin/bash

# compile sources and run unit tests
make build

# run main application tests (unit tests)
make gotest

# Note: Testcontainers integration tests are skipped in CI due to 
# container networking restrictions. Run locally with:
# make test-testcontainers-shared
