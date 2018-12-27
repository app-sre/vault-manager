#!/bin/bash

for test in $(ls bats/*.bats); do
    bats --tap $test
done

