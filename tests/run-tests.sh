#!/bin/bash

set -e

for test in $(find bats -type f | grep bats); do
    echo "running $test"
    bats --tap $test
done

