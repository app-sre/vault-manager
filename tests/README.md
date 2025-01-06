# vault-manager testing

This project uses BATS (Bash Automated Testing System) for integration testing.

Upon commit, a build is triggered in the CI pipeline which runs the tests.
The build sets up necessary resources to run the tests, then executes `pr_check.sh`.

## Bats testing

https://bats-core.readthedocs.io/en/stable/docker-usage.html

## Building images and running tests locally

From the top level directory of the project, run:

```bash
make test
```

This first runs the target `build-test-container` which builds the test container.

```make
build-test-container:
	@docker build -t vault-manager-test -f tests/Dockerfile.tests .
```

Then, the target `run-tests` is executed, which runs the container `vault-manager-test` built from the `build-test-container` target.

```make
	@docker run -t \
		--rm \
		--net=host \
		-v $(PWD)/.env:/tests/.env \
		-e HOST_PATH=$(PWD) \
		vault-manager-test
```

The test container images WORKDIR is set to `/tests` and the entrypoint is set to `/tests/run-tests.sh`.

Refer to the script `run-tests.sh` for the commands executed within the container.

At a high level, the following steps are executed:

First, the following 4 images are pulled:
* vault
* qontract-server
* keycloak
* keycloak-cli

