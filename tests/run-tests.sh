#!/bin/bash

for test in $(ls bats/*.bats); do
    echo "running $test"
    bats --tap $test
done

