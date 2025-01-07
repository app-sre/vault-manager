.PHONY: test build-test-container test-with-compose build push gotest gobuild

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

build-test-container:
	@$(CONTAINER_ENGINE) build -t $(IMAGE_NAME)-test -f tests/Dockerfile.tests .

test-with-compose: build-test-container
	podman-compose -f tests/compose.yml up -d --force-recreate
	bash tests/run-tests-compose.sh
