#!/bin/bash

set -e
source dev-env
vault server -dev -dev-root-token-id="root"