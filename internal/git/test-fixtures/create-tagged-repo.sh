#!/usr/bin/env bash
set -eux -o pipefail

if [ -d "/path/to/dir" ]
then
    echo "fixture already exists!"
    exit 0
else
    echo "creating fixture..."
fi

git init repos/tagged-repo

pushd repos/tagged-repo

git config --local user.email "nope@nope.com"
git config --local user.name "nope"

trap 'popd' EXIT

git commit -m 'something' --allow-empty
git tag v0.1.0