#!/bin/bash

if [ -t 1 ]; then
  BATS_OPTIONS="--pretty"
else
  BATS_OPTIONS="--tap"
fi

for test in $(ls bats/*.bats); do
    bats $BATS_OPTIONS $test
done

