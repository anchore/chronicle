#!/usr/bin/env bash
set -eu -o pipefail

if [ -d "/path/to/dir" ]
then
    echo "fixture already exists!"
    exit 0
else
    echo "creating fixture..."
fi

git init tagged-repo

pushd tagged-repo

trap 'popd' EXIT

git commit -m 'something' --allow-empty
