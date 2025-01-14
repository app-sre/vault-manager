# vault-manager integration testing

This project uses BATS (Bash Automated Testing System) for integration testing.

Upon commit, a build is triggered in the CI pipeline which runs the tests.
The build sets up necessary resources to run the tests, then executes `pr_check.sh`.

## Bats Core

[Bats Core](https://bats-core.readthedocs.io/en/stable/docker-usage.html) is a fork of Bats with the goal of providing a more maintainable and extensible platform for development of testing tools. Bats is a TAP-compliant testing framework for Bash. It provides a simple way to verify that the UNIX programs you write behave as expected.

This project uses Bats Core for integration testing.

## Building images and running tests locally

From the top level directory of the project, run:

```bash
make test-with-compose
```

This first runs the target `build-test-container` which builds the test container.

```make
build-test-container:
	@docker build -t vault-manager-test -f tests/Dockerfile.tests .
```

Then, the target `test-with-compose` is executed, which runs the container `vault-manager-test` built from the `build-test-container` target.

```make
test-with-compose: build-test-container
	@podman-compose -f tests/docker-compose.yml up --force-recreate
```

The test container image WORKDIR is set to `/tests` and the entrypoint is set to `/tests/run-tests-compose.sh`.

Refer to the script `run-tests-compose.sh` for the commands executed within the container.

## Test cases

The following test cases are executed:
* audit-devices.bats
* auth-backends-with-policies.bats
* entities.bats
* errors.bats
* flags.bats
* groups.bats
* policies.bats
* roles.bats
* secret-engines.bats
