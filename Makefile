# Makefile for vault-manager
#
# Test Environments:
#   - Konflux CI: Uses .tekton/ pipelines and tests/k8s/konflux-test-runner.sh
#   - Local/Jenkins: Uses targets below (build-test-container, test-with-compose)
#
# For Konflux testing, see: .tekton/README.md and tests/k8s/README.md

.PHONY: build-test-container test-with-compose build push gotest gobuild down

COMPOSE_FILE ?= tests/compose.yml
CONTAINER_ENGINE ?= $(shell command -v podman > /dev/null 2>&1 && echo podman || echo docker )
CONTAINER_SELINUX_FLAG ?= :z
IMAGE_NAME := quay.io/app-sre/vault-manager
IMAGE_TAG := $(shell git rev-parse --short=7 HEAD)
GOOS := $(shell go env GOOS)
PWD := $(shell pwd)

ifneq (,$(wildcard $(CURDIR)/.docker))
	DOCKER_CONF := $(CURDIR)/.docker
else
	DOCKER_CONF := $(HOME)/.docker
endif

gotest:
	CGO_ENABLED=0 GOOS=$(GOOS) go test ./...

gobuild: gotest
	CGO_ENABLED=0 GOOS=$(GOOS) go build -a -buildvcs=false -installsuffix cgo ./cmd/vault-manager

build:
	@$(CONTAINER_ENGINE) build --no-cache -t $(IMAGE_NAME):$(IMAGE_TAG) .

push:
	@$(CONTAINER_ENGINE) tag $(IMAGE_NAME):$(IMAGE_TAG) $(IMAGE_NAME):latest
	@$(CONTAINER_ENGINE) --config=$(DOCKER_CONF) push $(IMAGE_NAME):$(IMAGE_TAG)
	@$(CONTAINER_ENGINE) --config=$(DOCKER_CONF) push $(IMAGE_NAME):latest

generate:
	@helm lint helm/vault-manager
	@helm template helm/vault-manager -n vault-manager -f helm/vault-manager/values-commercial.yaml > openshift/vault-manager.template.yaml
	@helm template helm/vault-manager -n vault-manager -f helm/vault-manager/values-fedramp.yaml > openshift/vault-manager-fedramp.template.yaml

# Local/Jenkins testing targets (NOT used by Konflux)
# For Konflux testing, see tests/k8s/konflux-test-runner.sh

build-test-container:
	@$(CONTAINER_ENGINE) build --target=test -t $(IMAGE_NAME)-test .

test-with-compose: build-test-container
	@podman-compose -f $(COMPOSE_FILE) up -d --force-recreate
	@podman exec vault-manager-test_vault-manager-test_1 /tests/run-tests-compose.sh
	@podman-compose -f $(COMPOSE_FILE) down --volumes --remove-orphans

down:
	@podman-compose -f $(COMPOSE_FILE) down --volumes --remove-orphans
