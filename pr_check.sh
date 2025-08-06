#!/bin/bash

# compile sources and run unit tests
make build

# run e2e tests
source .env
make gobuild
