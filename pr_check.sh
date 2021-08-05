#!/bin/bash

docker login quay.io -u ${QUAY_USER} -p ${QUAY_TOKEN}

# compile sources and run unit tests
make build

# run e2e tests
make test
