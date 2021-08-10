.PHONY: test build-test-container build push gotest gobuild

IMAGE_NAME := quay.io/app-sre/vault-manager
IMAGE_TAG := $(shell git rev-parse --short=7 HEAD)
DOCKER_CONF := $(CURDIR)/.docker
GOOS := $(shell go env GOOS)

gotest:
	CGO_ENABLED=0 GOOS=$(GOOS) go test ./...

gobuild: gotest
	CGO_ENABLED=0 GOOS=$(GOOS) go build -a -installsuffix cgo ./cmd/vault-manager

build:
	@docker build --no-cache -t $(IMAGE_NAME):$(IMAGE_TAG) .

push:
	@docker tag $(IMAGE_NAME):$(IMAGE_TAG) $(IMAGE_NAME):latest
	@docker --config=$(DOCKER_CONF) push $(IMAGE_NAME):$(IMAGE_TAG)
	@docker --config=$(DOCKER_CONF) push $(IMAGE_NAME):latest

build-test-container:
	@docker --config=$(DOCKER_CONF) build -t vault-manager-test -f tests/Dockerfile.tests .

test: build-test-container
	@docker run -t \
	            --rm \
	            --net=host \
	            -v /var/run/docker.sock:/var/run/docker.sock \
	            -e GRAPHQL_SERVER=https://app-interface.devshift.net/graphql \
	            -e GRAPHQL_USERNAME=${USERNAME_PRODUCTION} \
	            -e GRAPHQL_PASSWORD=${PASSWORD_PRODUCTION} \
	            vault-manager-test
