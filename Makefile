.PHONY: test build-test-container build push gotest gobuild

IMAGE_NAME := quay.io/app-sre/vault-manager
IMAGE_TAG := $(shell git rev-parse --short=7 HEAD)
DOCKER_CONF := $(CURDIR)/.docker
GOOS := $(shell go env GOOS)
PWD := $(shell pwd)

gotest:
	CGO_ENABLED=0 GOOS=$(GOOS) go test ./...

gobuild: gotest
	CGO_ENABLED=0 GOOS=$(GOOS) go build -a -buildvcs=false -installsuffix cgo ./cmd/vault-manager

build:
	@docker build --no-cache -t $(IMAGE_NAME):$(IMAGE_TAG) .

push:
	@docker tag $(IMAGE_NAME):$(IMAGE_TAG) $(IMAGE_NAME):latest
	@docker --config=$(DOCKER_CONF) push $(IMAGE_NAME):$(IMAGE_TAG)
	@docker --config=$(DOCKER_CONF) push $(IMAGE_NAME):latest

generate:
	@helm lint helm/vault-manager
	@helm template helm/vault-manager -n vault-manager -f helm/vault-manager/values-commercial.yaml > openshift/vault-manager.template.yaml
	@helm template helm/vault-manager -n vault-manager -f helm/vault-manager/values-fedramp.yaml > openshift/vault-manager-fedramp.template.yaml

build-test-container:
	@docker build -t vault-manager-test -f tests/Dockerfile.tests .

test: build-test-container
	@docker --config=$(DOCKER_CONF) pull $(VAULT_IMAGE):$(VAULT_IMAGE_TAG)
	@docker --config=$(DOCKER_CONF) pull $(QONTRACT_SERVER_IMAGE):$(QONTRACT_SERVER_IMAGE_TAG)
	@docker --config=$(DOCKER_CONF) pull $(KEYCLOAK_IMAGE):$(KEYCLOAK_IMAGE_TAG)
	@docker --config=$(DOCKER_CONF) pull $(KEYCLOAK_CLI_IMAGE):$(KEYCLOAK_CLI_IMAGE_TAG)
	@docker run -t \
		--rm \
		--net=host \
		-v $(PWD)/.env:/tests/.env \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-e HOST_PATH=$(PWD) \
		vault-manager-test
