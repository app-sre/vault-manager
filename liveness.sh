#! /bin/bash

source "$1"

if [[ "$VALUE" != "0" ]]
then
    exit 1
fi

# set back for app to update
echo "VALUE=1" > "$1"
exit 0
